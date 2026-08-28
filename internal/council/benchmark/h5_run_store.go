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

func CreateH5Run(ctx context.Context, runsRoot, runID string, dataset Dataset, now time.Time) (string, RunManifest, error) {
	if err := ctx.Err(); err != nil {
		return "", RunManifest{}, err
	}
	if strings.TrimSpace(runsRoot) == "" {
		return "", RunManifest{}, fmt.Errorf("runs root is required")
	}
	if !safeDatasetID(runID) || strings.TrimSpace(runID) == "" {
		return "", RunManifest{}, fmt.Errorf("invalid H5 run id %q", runID)
	}
	if err := validateH5DatasetForStore(dataset); err != nil {
		return "", RunManifest{}, err
	}
	if err := os.MkdirAll(runsRoot, 0o750); err != nil {
		return "", RunManifest{}, fmt.Errorf("create H5 runs root: %w", err)
	}
	if err := requireRealDirectory(runsRoot); err != nil {
		return "", RunManifest{}, fmt.Errorf("validate H5 runs root: %w", err)
	}
	runRoot := filepath.Join(runsRoot, runID)
	if err := os.Mkdir(runRoot, 0o750); err != nil {
		return "", RunManifest{}, fmt.Errorf("create H5 run directory %q: %w", runRoot, err)
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
		SchemaVersion:         H5RunSchemaVersion,
		BenchmarkID:           H5BenchmarkID,
		RunID:                 runID,
		CreatedAt:             now.UTC().Format(time.RFC3339Nano),
		DatasetManifestSHA256: sha256Hex(dataset.ManifestBytes),
		RubricSHA256:          dataset.RubricSHA256,
		CasesSHA256:           strings.ToLower(dataset.Manifest.CasesSHA256),
		AdapterPolicySHA256:   dataset.AdapterPolicySHA256,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return "", RunManifest{}, fmt.Errorf("marshal H5 run manifest: %w", err)
	}
	specs := []h1FileSpec{
		{rel: "h5-run.json", data: manifestBytes},
		{rel: filepath.Join("inputs", "benchmark-manifest.json"), data: dataset.ManifestBytes},
		{rel: filepath.Join("inputs", "rubric.json"), data: dataset.Rubric},
		{rel: filepath.Join("inputs", "adapter-policy.json"), data: dataset.AdapterPolicyBytes},
	}
	for _, c := range dataset.Cases {
		specs = append(specs,
			h1FileSpec{rel: filepath.Join("inputs", "cases", c.ID, "problem.json"), data: c.Problem},
			h1FileSpec{rel: filepath.Join("inputs", "cases", c.ID, "reference-set.json"), data: c.ReferenceSet},
		)
	}
	if err := writeH1Specs(ctx, runRoot, specs); err != nil {
		return "", RunManifest{}, fmt.Errorf("freeze H5 run inputs: %w", err)
	}
	committed = true
	return runRoot, manifest, nil
}

func WriteH5FinalResult(ctx context.Context, runRoot, runID string, summary evalharness.BatchSummary, adapterSummary H5AdapterSummary, adapterSummarySHA256 string) (H5ResultManifest, error) {
	if err := ctx.Err(); err != nil {
		return H5ResultManifest{}, err
	}
	if err := requireRealDirectory(runRoot); err != nil {
		return H5ResultManifest{}, fmt.Errorf("validate H5 run root: %w", err)
	}
	if !safeDatasetID(runID) || strings.TrimSpace(runID) == "" {
		return H5ResultManifest{}, fmt.Errorf("invalid H5 run id %q", runID)
	}
	summaryBytes, err := json.Marshal(summary)
	if err != nil {
		return H5ResultManifest{}, fmt.Errorf("marshal H5 batch summary: %w", err)
	}
	expectedHash := sha256Hex(summaryBytes)
	batchPath := filepath.Join(runRoot, "eval", "batch-summary.json")
	if err := requireContainedRegularFile(runRoot, batchPath); err != nil {
		return H5ResultManifest{}, fmt.Errorf("validate eval/batch-summary.json: %w", err)
	}
	storedBytes, err := os.ReadFile(batchPath)
	if err != nil {
		return H5ResultManifest{}, fmt.Errorf("read eval/batch-summary.json: %w", err)
	}
	if storedHash := sha256Hex(storedBytes); storedHash != expectedHash {
		return H5ResultManifest{}, fmt.Errorf("eval/batch-summary.json hash mismatch: got %s want %s", storedHash, expectedHash)
	}
	adapterPath := filepath.Join(runRoot, "adapter-summary.json")
	if err := requireContainedRegularFile(runRoot, adapterPath); err != nil {
		return H5ResultManifest{}, fmt.Errorf("validate adapter-summary.json: %w", err)
	}
	adapterBytes, err := os.ReadFile(adapterPath)
	if err != nil {
		return H5ResultManifest{}, fmt.Errorf("read adapter-summary.json: %w", err)
	}
	if got := sha256Hex(adapterBytes); got != strings.ToLower(adapterSummarySHA256) {
		return H5ResultManifest{}, fmt.Errorf("adapter-summary.json hash mismatch: got %s want %s", got, adapterSummarySHA256)
	}
	manifest := H5ResultManifest{SchemaVersion: H5ResultSchemaVersion, BenchmarkID: H5BenchmarkID, RunID: runID, ProblemCount: summary.ProblemCount, BatchSummarySHA256: expectedHash, AdapterSummarySHA256: strings.ToLower(adapterSummarySHA256), EffectiveProviderDiversity: adapterSummary.EffectiveProviderDiversity, TotalAvailabilityFailovers: adapterSummary.TotalAvailabilityFailovers, HumanBrokerInvocations: adapterSummary.HumanBrokerInvocations}
	data, err := json.Marshal(manifest)
	if err != nil {
		return H5ResultManifest{}, fmt.Errorf("marshal H5 result manifest: %w", err)
	}
	if err := writeH1Specs(ctx, runRoot, []h1FileSpec{{rel: "h5-result.json", data: data}}); err != nil {
		return H5ResultManifest{}, fmt.Errorf("write H5 result manifest: %w", err)
	}
	return manifest, nil
}

func validateH5DatasetForStore(dataset Dataset) error {
	if dataset.Manifest.BenchmarkID != H5BenchmarkID || dataset.Manifest.SchemaVersion != H5DatasetSchemaVersion {
		return fmt.Errorf("dataset is not the frozen H5 dataset")
	}
	if len(dataset.Cases) != H1CaseCount {
		return fmt.Errorf("dataset has %d cases, want %d", len(dataset.Cases), H1CaseCount)
	}
	if len(dataset.ManifestBytes) == 0 || len(dataset.Rubric) == 0 || len(dataset.CasesBytes) == 0 || len(dataset.AdapterPolicyBytes) == 0 {
		return fmt.Errorf("dataset frozen bytes are incomplete")
	}
	manifestRubricHash := strings.ToLower(dataset.Manifest.RubricSHA256)
	if sha256Hex(dataset.Rubric) != manifestRubricHash || dataset.RubricSHA256 != manifestRubricHash {
		return fmt.Errorf("dataset rubric hash differs from frozen manifest")
	}
	if sha256Hex(dataset.CasesBytes) != strings.ToLower(dataset.Manifest.CasesSHA256) {
		return fmt.Errorf("dataset cases hash differs from frozen manifest")
	}
	if sha256Hex(dataset.AdapterPolicyBytes) != strings.ToLower(dataset.Manifest.AdapterPolicySHA256) || dataset.AdapterPolicySHA256 != strings.ToLower(dataset.Manifest.AdapterPolicySHA256) {
		return fmt.Errorf("dataset adapter policy hash differs from frozen manifest")
	}
	return nil
}
