package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

func CreateH2Run(ctx context.Context, runsRoot, runID string, dataset Dataset, now time.Time) (string, RunManifest, error) {
	if err := ctx.Err(); err != nil {
		return "", RunManifest{}, err
	}
	if strings.TrimSpace(runsRoot) == "" {
		return "", RunManifest{}, fmt.Errorf("runs root is required")
	}
	if !safeDatasetID(runID) || strings.TrimSpace(runID) == "" {
		return "", RunManifest{}, fmt.Errorf("invalid H2 run id %q", runID)
	}
	if err := validateH2DatasetForStore(dataset); err != nil {
		return "", RunManifest{}, err
	}
	if err := os.MkdirAll(runsRoot, 0o750); err != nil {
		return "", RunManifest{}, fmt.Errorf("create H2 runs root: %w", err)
	}
	if err := requireRealDirectory(runsRoot); err != nil {
		return "", RunManifest{}, fmt.Errorf("validate H2 runs root: %w", err)
	}
	runRoot := filepath.Join(runsRoot, runID)
	if err := os.Mkdir(runRoot, 0o750); err != nil {
		return "", RunManifest{}, fmt.Errorf("create H2 run directory %q: %w", runRoot, err)
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
		SchemaVersion:         H2RunSchemaVersion,
		BenchmarkID:           H2BenchmarkID,
		RunID:                 runID,
		CreatedAt:             now.UTC().Format(time.RFC3339Nano),
		DatasetManifestSHA256: sha256Hex(dataset.ManifestBytes),
		RubricSHA256:          dataset.RubricSHA256,
		CasesSHA256:           strings.ToLower(dataset.Manifest.CasesSHA256),
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return "", RunManifest{}, fmt.Errorf("marshal H2 run manifest: %w", err)
	}
	specs := []h1FileSpec{
		{rel: "h2-run.json", data: manifestBytes},
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
		return "", RunManifest{}, fmt.Errorf("freeze H2 run inputs: %w", err)
	}
	committed = true
	return runRoot, manifest, nil
}

func WriteH2FinalResult(ctx context.Context, runRoot, runID string, summary evalharness.BatchSummary) (ResultManifest, error) {
	if err := ctx.Err(); err != nil {
		return ResultManifest{}, err
	}
	if err := requireRealDirectory(runRoot); err != nil {
		return ResultManifest{}, fmt.Errorf("validate H2 run root: %w", err)
	}
	if !safeDatasetID(runID) || strings.TrimSpace(runID) == "" {
		return ResultManifest{}, fmt.Errorf("invalid H2 run id %q", runID)
	}
	summaryBytes, err := json.Marshal(summary)
	if err != nil {
		return ResultManifest{}, fmt.Errorf("marshal H2 batch summary: %w", err)
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
	if storedHash := sha256Hex(storedBytes); storedHash != expectedHash {
		return ResultManifest{}, fmt.Errorf("eval/batch-summary.json hash mismatch: got %s want %s", storedHash, expectedHash)
	}
	manifest := ResultManifest{
		SchemaVersion:      H2ResultSchemaVersion,
		BenchmarkID:        H2BenchmarkID,
		RunID:              runID,
		ProblemCount:       summary.ProblemCount,
		BatchSummarySHA256: expectedHash,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return ResultManifest{}, fmt.Errorf("marshal H2 result manifest: %w", err)
	}
	if err := writeH1Specs(ctx, runRoot, []h1FileSpec{{rel: "h2-result.json", data: data}}); err != nil {
		return ResultManifest{}, fmt.Errorf("write H2 result manifest: %w", err)
	}
	return manifest, nil
}

func validateH2DatasetForStore(dataset Dataset) error {
	if dataset.Manifest.BenchmarkID != H2BenchmarkID || dataset.Manifest.SchemaVersion != H2DatasetSchemaVersion {
		return fmt.Errorf("dataset is not the frozen H2 dataset")
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
