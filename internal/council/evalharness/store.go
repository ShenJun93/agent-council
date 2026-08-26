package evalharness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

const EvalProvenanceSchemaVersion = "council.eval-provenance.v0"

type WriteRequest struct {
	Root     string
	Policy   RiskPolicy
	Problems []ProblemResult
	Summary  BatchSummary
}

type ProblemSummary struct {
	ProblemID       string  `json:"problem_id"`
	BestSingleScore float64 `json:"best_single_score"`
	CouncilScore    float64 `json:"council_score"`
	CouncilDelta    float64 `json:"council_delta"`
	MateriallyWorse bool    `json:"materially_worse"`
}

type EvalJudgeProvenance struct {
	Slot         string                  `json:"slot"`
	Provider     councilruntime.Provider `json:"provider"`
	InputHashes  map[string]string       `json:"input_hashes"`
	OutputSHA256 string                  `json:"output_sha256"`
}

type EvalProvenance struct {
	SchemaVersion string                `json:"schema_version"`
	Path          string                `json:"path"`
	SHA256        string                `json:"sha256"`
	ProblemID     string                `json:"problem_id,omitempty"`
	Arm           baseline.Arm          `json:"arm,omitempty"`
	Judges        []EvalJudgeProvenance `json:"judges,omitempty"`
}

type evalArtifactSpec struct {
	relPath   string
	value     any
	problemID string
	arm       baseline.Arm
	judges    [2]JudgeScore
	hasJudges bool
}

func WriteEvaluation(ctx context.Context, req WriteRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(req.Root) == "" {
		return fmt.Errorf("evaluation root is required")
	}
	rootInfo, err := os.Stat(req.Root)
	if err != nil {
		return fmt.Errorf("stat evaluation root: %w", err)
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("evaluation root is not a directory: %s", req.Root)
	}
	if err := validateRiskPolicy(req.Policy); err != nil {
		return err
	}
	computedSummary, err := SummarizeBatch(req.Problems, req.Policy)
	if err != nil {
		return fmt.Errorf("validate evaluation problems: %w", err)
	}
	if !reflect.DeepEqual(computedSummary, req.Summary) {
		return fmt.Errorf("evaluation summary does not match frozen problem results")
	}

	specs, err := evaluationArtifactSpecs(req)
	if err != nil {
		return err
	}

	evalRoot := filepath.Join(req.Root, "eval")
	if _, err := os.Lstat(evalRoot); err == nil {
		return fmt.Errorf("evaluation artifacts already exist: %s", evalRoot)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect evaluation artifact root: %w", err)
	}
	if err := os.Mkdir(evalRoot, 0o750); err != nil {
		return fmt.Errorf("create evaluation artifact root: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		info, statErr := os.Lstat(evalRoot)
		if statErr == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			_ = os.RemoveAll(evalRoot)
		}
	}()
	if err := ensureContainedDirectory(req.Root, evalRoot); err != nil {
		return fmt.Errorf("validate evaluation artifact root: %w", err)
	}

	entries := make([]EvalProvenance, 0, len(specs))
	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := json.Marshal(spec.value)
		if err != nil {
			return fmt.Errorf("marshal evaluation artifact %q: %w", spec.relPath, err)
		}
		path := filepath.Join(req.Root, filepath.FromSlash(spec.relPath))
		if err := prepareContainedParent(evalRoot, path); err != nil {
			return fmt.Errorf("prepare evaluation artifact %q: %w", spec.relPath, err)
		}
		if err := writeEvalExclusive(path, data); err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("evaluation artifact %q already exists: %w", spec.relPath, err)
			}
			return fmt.Errorf("write evaluation artifact %q: %w", spec.relPath, err)
		}

		digest := sha256.Sum256(data)
		entry := EvalProvenance{
			SchemaVersion: EvalProvenanceSchemaVersion,
			Path:          filepath.ToSlash(spec.relPath),
			SHA256:        hex.EncodeToString(digest[:]),
			ProblemID:     spec.problemID,
			Arm:           spec.arm,
		}
		if spec.hasJudges {
			entry.Judges = []EvalJudgeProvenance{
				judgeProvenance(spec.judges[0]),
				judgeProvenance(spec.judges[1]),
			}
		}
		entries = append(entries, entry)
	}

	provenanceBytes, err := marshalEvalProvenance(entries)
	if err != nil {
		return err
	}
	provenancePath := filepath.Join(evalRoot, "provenance.jsonl")
	if err := prepareContainedParent(evalRoot, provenancePath); err != nil {
		return fmt.Errorf("prepare evaluation provenance: %w", err)
	}
	if err := writeEvalExclusive(provenancePath, provenanceBytes); err != nil {
		return fmt.Errorf("write evaluation provenance: %w", err)
	}

	committed = true
	return nil
}

func evaluationArtifactSpecs(req WriteRequest) ([]evalArtifactSpec, error) {
	specs := []evalArtifactSpec{
		{relPath: "eval/eval-policy.json", value: req.Policy},
		{relPath: "eval/batch-summary.json", value: req.Summary},
	}
	for _, problem := range req.Problems {
		if !safeID(problem.ProblemID) || strings.TrimSpace(problem.ProblemID) == "" {
			return nil, fmt.Errorf("unsafe or empty problem id %q", problem.ProblemID)
		}
		byArm, err := validateProblemArmScores(problem.Arms)
		if err != nil {
			return nil, fmt.Errorf("problem %q: %w", problem.ProblemID, err)
		}
		for _, arm := range frozenEvalArms {
			score := byArm[arm]
			specs = append(specs, evalArtifactSpec{
				relPath:   filepath.ToSlash(filepath.Join("eval", "problems", problem.ProblemID, "arm-"+string(arm)+".json")),
				value:     score,
				problemID: problem.ProblemID,
				arm:       arm,
				judges:    score.Judges,
				hasJudges: true,
			})
		}
		summary, err := summarizeProblem(problem, req.Policy)
		if err != nil {
			return nil, fmt.Errorf("problem %q summary: %w", problem.ProblemID, err)
		}
		specs = append(specs, evalArtifactSpec{
			relPath:   filepath.ToSlash(filepath.Join("eval", "problems", problem.ProblemID, "problem-summary.json")),
			value:     summary,
			problemID: problem.ProblemID,
		})
	}
	return specs, nil
}

func summarizeProblem(problem ProblemResult, policy RiskPolicy) (ProblemSummary, error) {
	if err := validateRiskPolicy(policy); err != nil {
		return ProblemSummary{}, err
	}
	if problem.RiskPolicy != policy {
		return ProblemSummary{}, fmt.Errorf("problem risk policy differs from frozen evaluation policy")
	}
	if !safeID(problem.ProblemID) || strings.TrimSpace(problem.ProblemID) == "" {
		return ProblemSummary{}, fmt.Errorf("unsafe or empty problem id %q", problem.ProblemID)
	}
	byArm, err := validateProblemArmScores(problem.Arms)
	if err != nil {
		return ProblemSummary{}, err
	}
	bestSingle := math.Max(
		byArm[baseline.ArmAClaudeSingle].MeanScore,
		byArm[baseline.ArmBCodexSingle].MeanScore,
	)
	council := byArm[baseline.ArmFBlindCouncil].MeanScore
	delta := council - bestSingle
	return ProblemSummary{
		ProblemID:       problem.ProblemID,
		BestSingleScore: bestSingle,
		CouncilScore:    council,
		CouncilDelta:    delta,
		MateriallyWorse: delta <= -policy.MaterialWorseDelta,
	}, nil
}

func judgeProvenance(score JudgeScore) EvalJudgeProvenance {
	inputs := make(map[string]string, len(score.InputHashes))
	for key, value := range score.InputHashes {
		inputs[key] = value
	}
	return EvalJudgeProvenance{
		Slot:         score.Slot,
		Provider:     score.Provider,
		InputHashes:  inputs,
		OutputSHA256: score.OutputSHA256,
	}
}

func ensureContainedDirectory(root, candidate string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return fmt.Errorf("resolve candidate: %w", err)
	}
	rootAbs, err := filepath.Abs(resolvedRoot)
	if err != nil {
		return fmt.Errorf("absolute root: %w", err)
	}
	candidateAbs, err := filepath.Abs(resolvedCandidate)
	if err != nil {
		return fmt.Errorf("absolute candidate: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return fmt.Errorf("relativize candidate: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path %q escapes root %q", candidate, root)
	}
	return nil
}

func prepareContainedParent(evalRoot, path string) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	return ensureContainedDirectory(evalRoot, parent)
}

func writeEvalExclusive(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func marshalEvalProvenance(entries []EvalProvenance) ([]byte, error) {
	var b strings.Builder
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("marshal evaluation provenance for %q: %w", entry.Path, err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}
