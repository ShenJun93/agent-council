package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

func TestPhaseHReplayRunnerPersistsCompletePass(t *testing.T) {
	ds := loadCommittedPhaseH(t)
	calls := 0
	runner := PhaseHReplayRunner{
		Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
			calls++
			if len(req.Arms) != 6 || req.RiskPolicy != PhaseHRiskPolicy {
				t.Fatalf("bad request arms=%d policy=%+v", len(req.Arms), req.RiskPolicy)
			}
			result := scoredProblem(req.ProblemID)
			result.RiskPolicy = PhaseHRiskPolicy
			return result, nil
		}},
		CollectAdapterSummary: successfulPhaseHAdapterSummary,
	}
	result, err := runner.Run(context.Background(), PhaseHReplayRunRequest{
		Dataset: ds, RunsRoot: t.TempDir(), RunID: "phase-h-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != PhaseHReplayCaseCount || result.Summary.ProblemCount != PhaseHReplayCaseCount {
		t.Fatalf("calls=%d problems=%d", calls, result.Summary.ProblemCount)
	}
	if result.Outcome != PhaseHOutcomePass || result.ValueSummary.OverallMeanCouncilDelta <= 0 {
		t.Fatalf("outcome=%q value=%+v", result.Outcome, result.ValueSummary)
	}
	data, err := os.ReadFile(filepath.Join(result.RunDir, "phase-h-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest PhaseHResultManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Outcome != PhaseHOutcomePass || manifest.ExpectedSuccessfulCalls != 120 {
		t.Fatalf("manifest=%+v", manifest)
	}
	if manifest.SourceH8ArtifactDigest != PhaseHSourceH8ArtifactDigest || manifest.ReplayManifestSHA256 == "" {
		t.Fatalf("source/hash missing: %+v", manifest)
	}
}
func TestPhaseHReplayRunnerFailsClosedBeforeFinalResult(t *testing.T) {
	ds := loadCommittedPhaseH(t)
	calls := 0
	runsRoot := t.TempDir()
	runner := PhaseHReplayRunner{
		Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
			calls++
			if calls == 3 {
				return evalharness.ProblemResult{}, errors.New("judge failed")
			}
			result := scoredProblem(req.ProblemID)
			result.RiskPolicy = PhaseHRiskPolicy
			return result, nil
		}},
		CollectAdapterSummary: successfulPhaseHAdapterSummary,
	}
	_, err := runner.Run(context.Background(), PhaseHReplayRunRequest{
		Dataset: ds, RunsRoot: runsRoot, RunID: "phase-h-partial",
	})
	if err == nil {
		t.Fatal("expected evaluator failure")
	}
	if calls != 3 {
		t.Fatalf("calls=%d", calls)
	}
	runRoot := filepath.Join(runsRoot, "phase-h-partial")
	if _, err := os.Stat(filepath.Join(runRoot, "phase-h-result.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("final result exists after failure: %v", err)
	}
}
func TestValidatePhaseHAdapterSummaryRejectsFailover(t *testing.T) {
	summary, err := successfulPhaseHAdapterSummary(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	summary.TotalAvailabilityFailovers = 1
	if err := validatePhaseHAdapterSummary(summary); err == nil {
		t.Fatal("expected failover rejection")
	}
}

func loadCommittedPhaseH(t *testing.T) PhaseHReplayDataset {
	t.Helper()
	ds, err := LoadPhaseHReplay(filepath.Join("..", "..", "..", "benchmarks", "phase-h"))
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

func successfulPhaseHAdapterSummary(context.Context, string, string) (H5AdapterSummary, error) {
	return H5AdapterSummary{
		SchemaVersion:             H5AdapterSummarySchemaVersion,
		SuccessfulInvocations:     PhaseHExpectedSuccessfulInvocations,
		EffectiveAdapterDiversity: 1, EffectiveProviderDiversity: 1,
		HumanBrokerInvocations: PhaseHExpectedSuccessfulInvocations,
		AttemptsByAdapter:      map[string]int{}, SuccessesByAdapter: map[string]int{},
		SuccessesByProvider: map[string]int{}, AvailabilityFailuresByAdapter: map[string]int{},
		SuccessesBySlot: map[string]map[string]int{},
	}, nil
}
