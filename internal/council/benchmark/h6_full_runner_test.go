package benchmark

import (
	"context"
	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"os"
	"path/filepath"
	"testing"
)

type countingH6ArmExecutor struct{ count *int }

func (f countingH6ArmExecutor) RunArm(_ context.Context, _ baseline.RunRequest, arm baseline.Arm) (baseline.ArmResult, error) {
	(*f.count)++
	return baseline.ArmResult{Arm: arm, InvocationCount: 1}, nil
}

func TestH6RunnerCompletesWithH6FinalArtifacts(t *testing.T) {
	dataset, err := LoadH6(writeValidH6Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	armCalls, evalCalls := 0, 0
	runner := H6Runner{NewBaseline: func(Case) (H6BaselineExecutor, error) { return countingH6ArmExecutor{&armCalls}, nil }, Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
		evalCalls++
		return scoredProblem(req.ProblemID), nil
	}}, CollectAdapterSummary: func(context.Context, string, string) (H5AdapterSummary, error) {
		return H5AdapterSummary{SchemaVersion: H5AdapterSummarySchemaVersion, SuccessfulInvocations: H5ExpectedSuccessfulInvocations, AttemptsByAdapter: map[string]int{}, SuccessesByAdapter: map[string]int{}, SuccessesByProvider: map[string]int{}, AvailabilityFailuresByAdapter: map[string]int{}, SuccessesBySlot: map[string]map[string]int{}}, nil
	}}
	result, err := runner.Run(context.Background(), RunRequest{Dataset: dataset, RunsRoot: t.TempDir(), RunID: "h6-runner-test"})
	if err != nil {
		t.Fatal(err)
	}
	if armCalls != H1CaseCount*6 || evalCalls != H1CaseCount {
		t.Fatalf("calls arms=%d eval=%d", armCalls, evalCalls)
	}
	for _, rel := range []string{filepath.Join("eval", "batch-summary.json"), "adapter-summary.json", "h6-result.json"} {
		if _, err := os.Stat(filepath.Join(result.RunDir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}
