package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

const (
	H1RunSchemaVersion    = "council.h1-run.v0"
	H1ResultSchemaVersion = "council.h1-result.v0"
)

var h1BaselineArms = []baseline.Arm{
	baseline.ArmAClaudeSingle,
	baseline.ArmBCodexSingle,
	baseline.ArmCClaudeSelfReview,
	baseline.ArmDCodexSelfReview,
	baseline.ArmEFullInfo,
	baseline.ArmFBlindCouncil,
}

type RunManifest struct {
	SchemaVersion         string `json:"schema_version"`
	BenchmarkID           string `json:"benchmark_id"`
	RunID                 string `json:"run_id"`
	CreatedAt             string `json:"created_at"`
	DatasetManifestSHA256 string `json:"dataset_manifest_sha256"`
	RubricSHA256          string `json:"rubric_sha256"`
	CasesSHA256           string `json:"cases_sha256"`
	AdapterPolicySHA256   string `json:"adapter_policy_sha256,omitempty"`
}

type ResultManifest struct {
	SchemaVersion      string `json:"schema_version"`
	BenchmarkID        string `json:"benchmark_id"`
	RunID              string `json:"run_id"`
	ProblemCount       int    `json:"problem_count"`
	BatchSummarySHA256 string `json:"batch_summary_sha256"`
}

type h1FileSpec struct {
	rel  string
	data []byte
}

func CreateRun(ctx context.Context, runsRoot, runID string, dataset Dataset, now time.Time) (string, RunManifest, error) {
	if err := ctx.Err(); err != nil {
		return "", RunManifest{}, err
	}
	if strings.TrimSpace(runsRoot) == "" {
		return "", RunManifest{}, fmt.Errorf("runs root is required")
	}
	if !safeDatasetID(runID) || strings.TrimSpace(runID) == "" {
		return "", RunManifest{}, fmt.Errorf("invalid H1 run id %q", runID)
	}
	if err := validateDatasetForStore(dataset); err != nil {
		return "", RunManifest{}, err
	}
	if err := os.MkdirAll(runsRoot, 0o750); err != nil {
		return "", RunManifest{}, fmt.Errorf("create H1 runs root: %w", err)
	}
	if err := requireRealDirectory(runsRoot); err != nil {
		return "", RunManifest{}, fmt.Errorf("validate H1 runs root: %w", err)
	}

	runRoot := filepath.Join(runsRoot, runID)
	if err := os.Mkdir(runRoot, 0o750); err != nil {
		return "", RunManifest{}, fmt.Errorf("create H1 run directory %q: %w", runRoot, err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		info, err := os.Lstat(runRoot)
		if err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			_ = os.RemoveAll(runRoot)
		}
	}()

	manifest := RunManifest{
		SchemaVersion:         H1RunSchemaVersion,
		BenchmarkID:           H1BenchmarkID,
		RunID:                 runID,
		CreatedAt:             now.UTC().Format(time.RFC3339Nano),
		DatasetManifestSHA256: sha256Hex(dataset.ManifestBytes),
		RubricSHA256:          dataset.RubricSHA256,
		CasesSHA256:           strings.ToLower(dataset.Manifest.CasesSHA256),
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return "", RunManifest{}, fmt.Errorf("marshal H1 run manifest: %w", err)
	}

	specs := []h1FileSpec{
		{rel: "h1-run.json", data: manifestBytes},
		{rel: filepath.Join("inputs", "benchmark-manifest.json"), data: dataset.ManifestBytes},
		{rel: filepath.Join("inputs", "rubric.json"), data: dataset.Rubric},
	}
	for _, c := range dataset.Cases {
		specs = append(specs,
			h1FileSpec{rel: filepath.Join("inputs", "cases", c.ID, "problem.json"), data: c.Problem},
			h1FileSpec{rel: filepath.Join("inputs", "cases", c.ID, "reference-set.json"), data: c.ReferenceSet},
		)
	}
	if err := writeH1Specs(ctx, runRoot, specs); err != nil {
		return "", RunManifest{}, fmt.Errorf("freeze H1 run inputs: %w", err)
	}

	committed = true
	return runRoot, manifest, nil
}

func WriteBaselineResults(ctx context.Context, runRoot, problemID string, results []baseline.ArmResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireRealDirectory(runRoot); err != nil {
		return fmt.Errorf("validate H1 run root: %w", err)
	}
	if !safeDatasetID(problemID) || strings.TrimSpace(problemID) == "" {
		return fmt.Errorf("invalid H1 problem id %q", problemID)
	}

	byArm := make(map[baseline.Arm]baseline.ArmResult, len(results))
	for _, result := range results {
		if _, duplicate := byArm[result.Arm]; duplicate {
			return fmt.Errorf("duplicate baseline arm %q", result.Arm)
		}
		byArm[result.Arm] = result
	}
	if len(byArm) != len(h1BaselineArms) {
		return fmt.Errorf("baseline arm set has %d arms, want A-F", len(byArm))
	}

	specs := make([]h1FileSpec, 0, len(h1BaselineArms))
	for _, arm := range h1BaselineArms {
		result, ok := byArm[arm]
		if !ok {
			return fmt.Errorf("baseline arm set is missing arm %s", arm)
		}
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal baseline arm %s: %w", arm, err)
		}
		specs = append(specs, h1FileSpec{
			rel:  filepath.Join("baseline", problemID, "arm-"+string(arm)+".json"),
			data: data,
		})
	}
	if err := writeH1Specs(ctx, runRoot, specs); err != nil {
		return fmt.Errorf("write H1 baseline results for %q: %w", problemID, err)
	}
	return nil
}

func WriteFinalResult(ctx context.Context, runRoot, runID string, summary evalharness.BatchSummary) (ResultManifest, error) {
	if err := ctx.Err(); err != nil {
		return ResultManifest{}, err
	}
	if err := requireRealDirectory(runRoot); err != nil {
		return ResultManifest{}, fmt.Errorf("validate H1 run root: %w", err)
	}
	if !safeDatasetID(runID) || strings.TrimSpace(runID) == "" {
		return ResultManifest{}, fmt.Errorf("invalid H1 run id %q", runID)
	}

	summaryBytes, err := json.Marshal(summary)
	if err != nil {
		return ResultManifest{}, fmt.Errorf("marshal H1 batch summary: %w", err)
	}
	expectedHash := sha256Hex(summaryBytes)
	batchPath := filepath.Join(runRoot, "eval", "batch-summary.json")
	if err := requireContainedRegularFile(runRoot, batchPath); err != nil {
		return ResultManifest{}, fmt.Errorf("validate eval/batch-summary.json: %w", err)
	}
	storedBytes, err := os.ReadFile(batchPath)
	if err != nil {
		return ResultManifest{}, fmt.Errorf("read eval/batch-summary.json: %w", err)
	}
	storedHash := sha256Hex(storedBytes)
	if storedHash != expectedHash {
		return ResultManifest{}, fmt.Errorf("eval/batch-summary.json hash mismatch: got %s want %s", storedHash, expectedHash)
	}

	manifest := ResultManifest{
		SchemaVersion:      H1ResultSchemaVersion,
		BenchmarkID:        H1BenchmarkID,
		RunID:              runID,
		ProblemCount:       summary.ProblemCount,
		BatchSummarySHA256: expectedHash,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return ResultManifest{}, fmt.Errorf("marshal H1 result manifest: %w", err)
	}
	if err := writeH1Specs(ctx, runRoot, []h1FileSpec{{rel: "h1-result.json", data: data}}); err != nil {
		return ResultManifest{}, fmt.Errorf("write H1 result manifest: %w", err)
	}
	return manifest, nil
}

func validateDatasetForStore(dataset Dataset) error {
	if dataset.Manifest.BenchmarkID != H1BenchmarkID || dataset.Manifest.SchemaVersion != H1DatasetSchemaVersion {
		return fmt.Errorf("dataset is not the frozen H1 dataset")
	}
	if len(dataset.Cases) != H1CaseCount {
		return fmt.Errorf("dataset has %d cases, want %d", len(dataset.Cases), H1CaseCount)
	}
	if len(dataset.ManifestBytes) == 0 || len(dataset.Rubric) == 0 || len(dataset.CasesBytes) == 0 {
		return fmt.Errorf("dataset frozen bytes are incomplete")
	}
	manifestRubricHash := strings.ToLower(dataset.Manifest.RubricSHA256)
	if sha256Hex(dataset.Rubric) != manifestRubricHash || dataset.RubricSHA256 != manifestRubricHash {
		return fmt.Errorf("dataset rubric hash differs from frozen manifest")
	}
	if sha256Hex(dataset.CasesBytes) != strings.ToLower(dataset.Manifest.CasesSHA256) {
		return fmt.Errorf("dataset cases hash differs from frozen manifest")
	}
	return nil
}

func writeH1Specs(ctx context.Context, root string, specs []h1FileSpec) error {
	if err := requireRealDirectory(root); err != nil {
		return err
	}

	paths := make([]string, len(specs))
	for i, spec := range specs {
		if err := ctx.Err(); err != nil {
			return err
		}
		clean := filepath.Clean(spec.rel)
		if filepath.IsAbs(spec.rel) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe relative artifact path %q", spec.rel)
		}
		path := filepath.Join(root, clean)
		if err := mkdirContained(root, filepath.Dir(path)); err != nil {
			return fmt.Errorf("prepare artifact %q: %w", spec.rel, err)
		}
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("artifact %q already exists", spec.rel)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect artifact %q: %w", spec.rel, err)
		}
		paths[i] = path
	}

	written := make([]string, 0, len(specs))
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, path := range written {
			_ = os.Remove(path)
		}
	}()
	for i, spec := range specs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeH1Exclusive(paths[i], spec.data); err != nil {
			return fmt.Errorf("write artifact %q: %w", spec.rel, err)
		}
		written = append(written, paths[i])
	}
	committed = true
	return nil
}

func requireRealDirectory(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("directory path is required")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%q must be a real directory", path)
	}
	return nil
}

func mkdirContained(root, parent string) error {
	if err := requireRealDirectory(root); err != nil {
		return fmt.Errorf("validate root: %w", err)
	}
	rootAbs, rel, err := relativeInside(root, parent)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}

	current := rootAbs
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		switch {
		case statErr == nil:
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("path component %q is not a real directory", current)
			}
		case errors.Is(statErr, os.ErrNotExist):
			if err := os.Mkdir(current, 0o750); err != nil {
				return fmt.Errorf("create directory %q: %w", current, err)
			}
		default:
			return fmt.Errorf("inspect directory %q: %w", current, statErr)
		}
	}
	return ensureResolvedInside(root, parent)
}

func requireContainedRegularFile(root, path string) error {
	if err := requireRealDirectory(root); err != nil {
		return err
	}
	_, rel, err := relativeInside(root, path)
	if err != nil {
		return err
	}
	current := root
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path component %q is a symlink", current)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("path component %q is not a directory", current)
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q must be a regular file", path)
	}
	return ensureResolvedInside(root, path)
}

func relativeInside(root, candidate string) (string, string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("absolute root: %w", err)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", "", fmt.Errorf("absolute candidate: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return "", "", fmt.Errorf("relativize candidate: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("path %q escapes root %q", candidate, root)
	}
	return rootAbs, rel, nil
}

func ensureResolvedInside(root, candidate string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return fmt.Errorf("resolve candidate: %w", err)
	}
	_, _, err = relativeInside(resolvedRoot, resolvedCandidate)
	return err
}

func writeH1Exclusive(path string, data []byte) error {
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
