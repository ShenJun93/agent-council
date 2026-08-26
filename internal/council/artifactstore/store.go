package artifactstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

const (
	IndexSchemaVersion      = "council.artifact-index.v0"
	ProvenanceSchemaVersion = "council.provenance.v0"
)

type WriteRequest struct {
	RunID              string
	RunRoot            string
	Result             protocol.Result
	ChallengerProvider councilruntime.Provider
	Now                func() time.Time
}

type Index struct {
	SchemaVersion string       `json:"schema_version"`
	RunID         string       `json:"run_id"`
	Entries       []Provenance `json:"entries"`
}

type Provenance struct {
	SchemaVersion string                  `json:"schema_version"`
	RunID         string                  `json:"run_id"`
	ArtifactID    string                  `json:"artifact_id"`
	Phase         string                  `json:"phase"`
	Participant   string                  `json:"participant"`
	Role          string                  `json:"role"`
	Provider      councilruntime.Provider `json:"provider,omitempty"`
	Inputs        []string                `json:"inputs"`
	Path          string                  `json:"path"`
	SHA256        string                  `json:"sha256"`
	RecordedAt    string                  `json:"recorded_at"`
}

type artifactSpec struct {
	id          string
	phase       string
	participant string
	role        string
	provider    councilruntime.Provider
	inputs      []string
	relPath     string
	value       any
}

func WriteProtocolResult(ctx context.Context, req WriteRequest) (Index, error) {
	if err := ctx.Err(); err != nil {
		return Index{}, err
	}
	if strings.TrimSpace(req.RunID) == "" {
		return Index{}, fmt.Errorf("run id is required")
	}
	if strings.TrimSpace(req.RunRoot) == "" {
		return Index{}, fmt.Errorf("run root is required")
	}
	if req.ChallengerProvider != councilruntime.ProviderClaude && req.ChallengerProvider != councilruntime.ProviderCodex {
		return Index{}, fmt.Errorf("challenger provider must be claude or codex")
	}
	if err := validateResultShape(req.Result); err != nil {
		return Index{}, err
	}

	artifactRoot, err := prepareArtifactRoot(req.RunRoot)
	if err != nil {
		return Index{}, err
	}
	provenancePath := filepath.Join(artifactRoot, "provenance.jsonl")
	if _, err := os.Lstat(provenancePath); err == nil {
		return Index{}, fmt.Errorf("artifact provenance already exists: %s", provenancePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Index{}, fmt.Errorf("inspect provenance path: %w", err)
	}

	now := time.Now
	if req.Now != nil {
		now = req.Now
	}
	recordedAt := now().UTC().Format(time.RFC3339Nano)

	specs := protocolArtifactSpecs(req.Result, req.ChallengerProvider)
	entries := make([]Provenance, 0, len(specs))
	created := make([]string, 0, len(specs)+1)
	committed := false
	defer func() {
		if committed {
			return
		}
		for i := len(created) - 1; i >= 0; i-- {
			_ = os.Remove(created[i])
		}
	}()

	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			return Index{}, err
		}
		if err := validateArtifactID(spec.id); err != nil {
			return Index{}, err
		}
		data, err := json.Marshal(spec.value)
		if err != nil {
			return Index{}, fmt.Errorf("marshal artifact %q: %w", spec.id, err)
		}
		path := filepath.Join(req.RunRoot, filepath.FromSlash(spec.relPath))
		if err := ensureSafeParent(req.RunRoot, path); err != nil {
			return Index{}, fmt.Errorf("prepare artifact %q: %w", spec.id, err)
		}
		if err := writeExclusive(path, data); err != nil {
			if errors.Is(err, os.ErrExist) {
				return Index{}, fmt.Errorf("artifact %q already exists: %w", spec.id, err)
			}
			return Index{}, fmt.Errorf("write artifact %q: %w", spec.id, err)
		}
		created = append(created, path)

		digest := sha256.Sum256(data)
		entries = append(entries, Provenance{
			SchemaVersion: ProvenanceSchemaVersion,
			RunID:         req.RunID,
			ArtifactID:    spec.id,
			Phase:         spec.phase,
			Participant:   spec.participant,
			Role:          spec.role,
			Provider:      spec.provider,
			Inputs:        append([]string(nil), spec.inputs...),
			Path:          filepath.ToSlash(spec.relPath),
			SHA256:        hex.EncodeToString(digest[:]),
			RecordedAt:    recordedAt,
		})
	}

	provenanceBytes, err := marshalJSONLines(entries)
	if err != nil {
		return Index{}, err
	}
	if err := ensureSafeParent(req.RunRoot, provenancePath); err != nil {
		return Index{}, fmt.Errorf("prepare provenance log: %w", err)
	}
	if err := writeExclusive(provenancePath, provenanceBytes); err != nil {
		if errors.Is(err, os.ErrExist) {
			return Index{}, fmt.Errorf("artifact provenance already exists: %w", err)
		}
		return Index{}, fmt.Errorf("write provenance log: %w", err)
	}
	created = append(created, provenancePath)
	committed = true

	return Index{
		SchemaVersion: IndexSchemaVersion,
		RunID:         req.RunID,
		Entries:       entries,
	}, nil
}

func protocolArtifactSpecs(result protocol.Result, challenger councilruntime.Provider) []artifactSpec {
	judgeInputs := []string{
		"problem",
		"research-1",
		"research-2",
		"review-1",
		"review-2",
		"challenge",
		"rebuttal-1",
		"rebuttal-2",
	}
	return []artifactSpec{
		{id: "research-1", phase: protocol.PhaseResearch, participant: "researcher-1", role: "researcher", provider: councilruntime.ProviderClaude, inputs: []string{"problem"}, relPath: "artifacts/research/research-1.json", value: result.Research[0].Artifact},
		{id: "research-2", phase: protocol.PhaseResearch, participant: "researcher-2", role: "researcher", provider: councilruntime.ProviderCodex, inputs: []string{"problem"}, relPath: "artifacts/research/research-2.json", value: result.Research[1].Artifact},
		{id: "review-1", phase: protocol.PhaseReview, participant: "reviewer-1", role: "reviewer", provider: councilruntime.ProviderClaude, inputs: []string{"problem", "research-2"}, relPath: "artifacts/reviews/review-1.json", value: result.Reviews[0].Artifact},
		{id: "review-2", phase: protocol.PhaseReview, participant: "reviewer-2", role: "reviewer", provider: councilruntime.ProviderCodex, inputs: []string{"problem", "research-1"}, relPath: "artifacts/reviews/review-2.json", value: result.Reviews[1].Artifact},
		{id: "challenge", phase: protocol.PhaseChallenge, participant: "challenger", role: "challenger", provider: challenger, inputs: []string{"problem", "research-1", "research-2", "review-1", "review-2"}, relPath: "artifacts/challenge/challenge.json", value: result.Challenge.Artifact},
		{id: "rebuttal-1", phase: protocol.PhaseRebuttal, participant: "researcher-1", role: "researcher", provider: councilruntime.ProviderClaude, inputs: []string{"problem", "research-1", "review-2", "challenge"}, relPath: "artifacts/rebuttals/rebuttal-1.json", value: result.Rebuttals[0].Artifact},
		{id: "rebuttal-2", phase: protocol.PhaseRebuttal, participant: "researcher-2", role: "researcher", provider: councilruntime.ProviderCodex, inputs: []string{"problem", "research-2", "review-1", "challenge"}, relPath: "artifacts/rebuttals/rebuttal-2.json", value: result.Rebuttals[1].Artifact},
		{id: "judge-1", phase: protocol.PhaseJudge, participant: "judge-1", role: "judge", provider: councilruntime.ProviderClaude, inputs: judgeInputs, relPath: "artifacts/judges/judge-1.json", value: result.Judges[0].Artifact},
		{id: "judge-2", phase: protocol.PhaseJudge, participant: "judge-2", role: "judge", provider: councilruntime.ProviderCodex, inputs: judgeInputs, relPath: "artifacts/judges/judge-2.json", value: result.Judges[1].Artifact},
		{id: "challenge-decision", phase: "challenge-routing", participant: "engine", role: "engine", inputs: []string{"research-1", "research-2"}, relPath: "artifacts/decision/challenge-decision.json", value: result.ChallengeDecision},
		{id: "decision-record", phase: "decision", participant: "engine", role: "engine", inputs: []string{"judge-1", "judge-2"}, relPath: "artifacts/decision/decision-record.json", value: result.Decision},
	}
}

func validateResultShape(result protocol.Result) error {
	checks := []struct {
		kind string
		got  []string
		want []string
	}{
		{kind: "research", got: researchIDs(result.Research), want: []string{"research-1", "research-2"}},
		{kind: "review", got: reviewIDs(result.Reviews), want: []string{"review-1", "review-2"}},
		{kind: "rebuttal", got: rebuttalIDs(result.Rebuttals), want: []string{"rebuttal-1", "rebuttal-2"}},
		{kind: "judge", got: judgeIDs(result.Judges), want: []string{"judge-1", "judge-2"}},
	}
	for _, check := range checks {
		if strings.Join(check.got, ",") != strings.Join(check.want, ",") {
			for _, id := range check.got {
				if err := validateArtifactID(id); err != nil {
					return err
				}
			}
			return fmt.Errorf("unexpected %s artifact ids: got %v want %v", check.kind, check.got, check.want)
		}
	}
	if err := validateArtifactID(result.Challenge.ID); err != nil {
		return err
	}
	if result.Challenge.ID != "challenge" {
		return fmt.Errorf("unexpected challenge artifact id %q", result.Challenge.ID)
	}
	return nil
}

func validateArtifactID(id string) error {
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("unsafe artifact id %q", id)
	}
	return nil
}

func researchIDs(records []protocol.ResearchRecord) []string {
	ids := make([]string, len(records))
	for i := range records {
		ids[i] = records[i].ID
	}
	return ids
}

func reviewIDs(records []protocol.ReviewRecord) []string {
	ids := make([]string, len(records))
	for i := range records {
		ids[i] = records[i].ID
	}
	return ids
}

func rebuttalIDs(records []protocol.RebuttalRecord) []string {
	ids := make([]string, len(records))
	for i := range records {
		ids[i] = records[i].ID
	}
	return ids
}

func judgeIDs(records []protocol.JudgeRecord) []string {
	ids := make([]string, len(records))
	for i := range records {
		ids[i] = records[i].ID
	}
	return ids
}

func prepareArtifactRoot(runRoot string) (string, error) {
	info, err := os.Stat(runRoot)
	if err != nil {
		return "", fmt.Errorf("stat run root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("run root is not a directory: %s", runRoot)
	}
	artifactRoot := filepath.Join(runRoot, "artifacts")
	if err := os.MkdirAll(artifactRoot, 0o750); err != nil {
		return "", fmt.Errorf("create artifact root: %w", err)
	}
	if err := ensureExistingWithin(runRoot, artifactRoot); err != nil {
		return "", fmt.Errorf("validate artifact root: %w", err)
	}
	return artifactRoot, nil
}

func ensureSafeParent(runRoot, path string) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	return ensureExistingWithin(runRoot, parent)
}

func ensureExistingWithin(root, candidate string) error {
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
		return fmt.Errorf("path %q escapes run root %q", candidate, root)
	}
	return nil
}

func writeExclusive(path string, data []byte) error {
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

func marshalJSONLines(entries []Provenance) ([]byte, error) {
	var b strings.Builder
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("marshal provenance for %q: %w", entry.ArtifactID, err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}
