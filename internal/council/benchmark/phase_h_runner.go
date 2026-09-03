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

type PhaseHRunResult struct {
	RunID          string                   `json:"run_id"`
	RunDir         string                   `json:"run_dir"`
	Summary        evalharness.BatchSummary `json:"batch_summary"`
	AdapterSummary H5AdapterSummary         `json:"adapter_summary"`
	ValueSummary   PhaseHValueSummary       `json:"value_summary"`
	Outcome        PhaseHOutcome            `json:"outcome"`
}

type PhaseHReplayRunner struct {
	Evaluator             EvalExecutor
	Now                   func() time.Time
	CollectAdapterSummary func(context.Context, string, string) (H5AdapterSummary, error)
}

func CreatePhaseHReplayRun(ctx context.Context, runsRoot, runID string, dataset PhaseHReplayDataset, now time.Time) (string, PhaseHRunManifest, error) {
	if err := ctx.Err(); err != nil {
		return "", PhaseHRunManifest{}, err
	}
	if strings.TrimSpace(runsRoot) == "" {
		return "", PhaseHRunManifest{}, fmt.Errorf("runs root is required")
	}
	if !safeDatasetID(runID) {
		return "", PhaseHRunManifest{}, fmt.Errorf("invalid Phase H run id %q", runID)
	}
	if len(dataset.Cases) != PhaseHReplayCaseCount || dataset.Manifest.BenchmarkID != PhaseHBenchmarkID {
		return "", PhaseHRunManifest{}, fmt.Errorf("dataset is not the frozen Phase H replay dataset")
	}
	if err := os.MkdirAll(runsRoot, 0o750); err != nil {
		return "", PhaseHRunManifest{}, fmt.Errorf("create Phase H runs root: %w", err)
	}
	if err := requireRealDirectory(runsRoot); err != nil {
		return "", PhaseHRunManifest{}, fmt.Errorf("validate Phase H runs root: %w", err)
	}
	runRoot := filepath.Join(runsRoot, runID)
	if err := os.Mkdir(runRoot, 0o750); err != nil {
		return "", PhaseHRunManifest{}, fmt.Errorf("create Phase H run directory: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(runRoot)
		}
	}()
	manifest := PhaseHRunManifest{
		SchemaVersion: PhaseHRunSchemaVersion, BenchmarkID: PhaseHBenchmarkID,
		RunID: runID, CreatedAt: now.UTC().Format(time.RFC3339Nano),
		ReplayManifestSHA256: sha256Hex(dataset.ManifestBytes),
		RubricSHA256:         dataset.RubricSHA256, AdapterPolicySHA256: dataset.AdapterPolicySHA256,
		SourceWorkflowRunID: PhaseHSourceWorkflowRunID, SourceFrozenSHA: PhaseHSourceFrozenSHA,
		SourceH8RunID: PhaseHSourceH8RunID, SourceArtifactDigest: PhaseHSourceH8ArtifactDigest,
	}
	runBytes, err := json.Marshal(manifest)
	if err != nil {
		return "", PhaseHRunManifest{}, err
	}
	specs := []h1FileSpec{
		{rel: "phase-h-run.json", data: runBytes},
		{rel: filepath.Join("inputs", "replay-manifest.json"), data: dataset.ManifestBytes},
		{rel: filepath.Join("inputs", "rubric.json"), data: dataset.Rubric},
		{rel: filepath.Join("inputs", "adapter-policy.json"), data: dataset.AdapterPolicyBytes},
	}
	for _, c := range dataset.Cases {
		base := filepath.Join("inputs", "cases", c.ID)
		specs = append(specs,
			h1FileSpec{rel: filepath.Join(base, "problem.json"), data: c.Problem},
			h1FileSpec{rel: filepath.Join(base, "reference-set.json"), data: c.ReferenceSet},
		)
		for _, arm := range phaseHReplayArms {
			specs = append(specs, h1FileSpec{rel: filepath.Join(base, "arm-"+string(arm)+".json"), data: c.ArmBytes[arm]})
		}
	}
	if err := writeH1Specs(ctx, runRoot, specs); err != nil {
		return "", PhaseHRunManifest{}, fmt.Errorf("freeze Phase H replay inputs: %w", err)
	}
	committed = true
	return runRoot, manifest, nil
}

func (r PhaseHReplayRunner) Run(ctx context.Context, req PhaseHReplayRunRequest) (PhaseHRunResult, error) {
	if err := ctx.Err(); err != nil {
		return PhaseHRunResult{}, err
	}
	if r.Evaluator == nil {
		return PhaseHRunResult{}, fmt.Errorf("phase H evaluator is required")
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	runRoot, _, err := CreatePhaseHReplayRun(ctx, req.RunsRoot, req.RunID, req.Dataset, now())
	if err != nil {
		return PhaseHRunResult{}, fmt.Errorf("create Phase H run: %w", err)
	}
	problems := make([]evalharness.ProblemResult, 0, PhaseHReplayCaseCount)
	for _, c := range req.Dataset.Cases {
		result, err := r.Evaluator.EvaluateProblem(ctx, evalharness.ProblemRequest{
			ProblemID: c.ID, RunID: req.RunID, RunRoot: runRoot,
			NormalizedProblem: c.Problem, Rubric: req.Dataset.Rubric,
			RubricSHA256: req.Dataset.RubricSHA256, ReferenceSet: c.ReferenceSet,
			ReferenceSetSHA256: c.ReferenceSetSHA256, Arms: c.Arms, RiskPolicy: PhaseHRiskPolicy,
		})
		if err != nil {
			return PhaseHRunResult{}, fmt.Errorf("phase H replay case %q evaluation: %w", c.ID, err)
		}
		if result.ProblemID != c.ID || result.RiskPolicy != PhaseHRiskPolicy {
			return PhaseHRunResult{}, fmt.Errorf("phase H replay case %q returned invalid evaluation identity/policy", c.ID)
		}
		problems = append(problems, result)
	}
	if len(problems) != PhaseHReplayCaseCount {
		return PhaseHRunResult{}, fmt.Errorf("phase H evaluated %d replay cases, want %d", len(problems), PhaseHReplayCaseCount)
	}
	summary, err := evalharness.SummarizeBatch(problems, PhaseHRiskPolicy)
	if err != nil {
		return PhaseHRunResult{}, fmt.Errorf("summarize Phase H evaluation: %w", err)
	}
	valueSummary, err := SummarizePhaseHValue(problems)
	if err != nil {
		return PhaseHRunResult{}, fmt.Errorf("summarize Phase H value: %w", err)
	}
	outcome := ClassifyPhaseHValue(valueSummary)
	if err := evalharness.WriteEvaluation(ctx, evalharness.WriteRequest{
		Root: runRoot, Policy: PhaseHRiskPolicy, Problems: problems, Summary: summary,
	}); err != nil {
		return PhaseHRunResult{}, fmt.Errorf("persist Phase H evaluation: %w", err)
	}
	collector := CollectH5AdapterSummary
	if r.CollectAdapterSummary != nil {
		collector = r.CollectAdapterSummary
	}
	adapterSummary, err := collector(ctx, runRoot, req.RunID)
	if err != nil {
		return PhaseHRunResult{}, fmt.Errorf("collect Phase H adapter summary: %w", err)
	}
	if err := validatePhaseHAdapterSummary(adapterSummary); err != nil {
		return PhaseHRunResult{}, err
	}
	adapterHash, err := WriteH5AdapterSummary(ctx, runRoot, adapterSummary)
	if err != nil {
		return PhaseHRunResult{}, fmt.Errorf("persist Phase H adapter summary: %w", err)
	}
	if _, err := WritePhaseHFinalResult(ctx, runRoot, req.RunID, req.Dataset, summary, valueSummary, outcome, adapterSummary, adapterHash); err != nil {
		return PhaseHRunResult{}, fmt.Errorf("persist Phase H final result: %w", err)
	}
	return PhaseHRunResult{RunID: req.RunID, RunDir: runRoot, Summary: summary,
		AdapterSummary: adapterSummary, ValueSummary: valueSummary, Outcome: outcome}, nil
}
func validatePhaseHAdapterSummary(s H5AdapterSummary) error {
	if s.SchemaVersion != H5AdapterSummarySchemaVersion {
		return fmt.Errorf("phase H adapter summary schema %q is invalid", s.SchemaVersion)
	}
	if s.SuccessfulInvocations != PhaseHExpectedSuccessfulInvocations {
		return fmt.Errorf("phase H successful invocations %d, want %d", s.SuccessfulInvocations, PhaseHExpectedSuccessfulInvocations)
	}
	if s.HumanBrokerInvocations != PhaseHExpectedSuccessfulInvocations {
		return fmt.Errorf("phase H human broker invocations %d, want %d", s.HumanBrokerInvocations, PhaseHExpectedSuccessfulInvocations)
	}
	if s.EffectiveAdapterDiversity != 1 || s.EffectiveProviderDiversity != 1 {
		return fmt.Errorf("phase H effective adapter/provider diversity must be 1/1")
	}
	if s.TotalAvailabilityFailovers != 0 || s.AvailabilityFailures != 0 {
		return fmt.Errorf("phase H must not contain availability failover/failure evidence")
	}
	return nil
}

func WritePhaseHFinalResult(ctx context.Context, runRoot, runID string, dataset PhaseHReplayDataset,
	summary evalharness.BatchSummary, valueSummary PhaseHValueSummary, outcome PhaseHOutcome,
	adapterSummary H5AdapterSummary, adapterSummarySHA256 string) (PhaseHResultManifest, error) {
	if err := ctx.Err(); err != nil {
		return PhaseHResultManifest{}, err
	}
	if err := requireRealDirectory(runRoot); err != nil {
		return PhaseHResultManifest{}, err
	}
	summaryBytes, err := json.Marshal(summary)
	if err != nil {
		return PhaseHResultManifest{}, err
	}
	summaryHash := sha256Hex(summaryBytes)
	batchPath := filepath.Join(runRoot, "eval", "batch-summary.json")
	if err := requireContainedRegularFile(runRoot, batchPath); err != nil {
		return PhaseHResultManifest{}, fmt.Errorf("validate Phase H batch summary: %w", err)
	}
	stored, err := os.ReadFile(batchPath)
	if err != nil {
		return PhaseHResultManifest{}, err
	}
	if sha256Hex(stored) != summaryHash {
		return PhaseHResultManifest{}, fmt.Errorf("phase H batch summary hash mismatch")
	}
	adapterPath := filepath.Join(runRoot, "adapter-summary.json")
	if err := requireContainedRegularFile(runRoot, adapterPath); err != nil {
		return PhaseHResultManifest{}, fmt.Errorf("validate Phase H adapter summary: %w", err)
	}
	adapterBytes, err := os.ReadFile(adapterPath)
	if err != nil {
		return PhaseHResultManifest{}, err
	}
	if sha256Hex(adapterBytes) != strings.ToLower(adapterSummarySHA256) {
		return PhaseHResultManifest{}, fmt.Errorf("phase H adapter summary hash mismatch")
	}
	manifest := PhaseHResultManifest{
		SchemaVersion: PhaseHResultSchemaVersion, BenchmarkID: PhaseHBenchmarkID,
		Mode: PhaseHReplayMode, RunID: runID, ReplayCaseCount: summary.ProblemCount,
		ExpectedSuccessfulCalls: PhaseHExpectedSuccessfulInvocations,
		BatchSummarySHA256:      summaryHash, AdapterSummarySHA256: strings.ToLower(adapterSummarySHA256),
		EffectiveProviderDiversity: adapterSummary.EffectiveProviderDiversity,
		TotalAvailabilityFailovers: adapterSummary.TotalAvailabilityFailovers,
		HumanBrokerInvocations:     adapterSummary.HumanBrokerInvocations,
		SourceWorkflowRunID:        PhaseHSourceWorkflowRunID, SourceH8RunID: PhaseHSourceH8RunID,
		SourceH8ArtifactDigest: PhaseHSourceH8ArtifactDigest,
		ReplayManifestSHA256:   sha256Hex(dataset.ManifestBytes), RubricSHA256: dataset.RubricSHA256,
		AdapterPolicySHA256: dataset.AdapterPolicySHA256, Outcome: outcome, ValueSummary: valueSummary,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return PhaseHResultManifest{}, err
	}
	if err := writeH1Specs(ctx, runRoot, []h1FileSpec{{rel: "phase-h-result.json", data: data}}); err != nil {
		return PhaseHResultManifest{}, fmt.Errorf("write Phase H result: %w", err)
	}
	return manifest, nil
}
