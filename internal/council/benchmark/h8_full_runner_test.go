package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

func TestH8RunnerCompletesWithH8FinalArtifacts(t *testing.T) {
	dataset, err := LoadH8(writeValidH8Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	armCalls, evalCalls := 0, 0
	runner := H8Runner{NewBaseline: func(Case) (H8BaselineExecutor, error) { return countingH8ArmExecutor{&armCalls}, nil }, Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
		evalCalls++
		return scoredProblem(req.ProblemID), nil
	}}, CollectAdapterSummary: func(context.Context, string, string) (H5AdapterSummary, error) {
		return H5AdapterSummary{SchemaVersion: H5AdapterSummarySchemaVersion, SuccessfulInvocations: H5ExpectedSuccessfulInvocations, AttemptsByAdapter: map[string]int{}, SuccessesByAdapter: map[string]int{}, SuccessesByProvider: map[string]int{}, AvailabilityFailuresByAdapter: map[string]int{}, SuccessesBySlot: map[string]map[string]int{}}, nil
	}}
	result, err := runner.Run(context.Background(), RunRequest{Dataset: dataset, RunsRoot: t.TempDir(), RunID: "h8-runner-test"})
	if err != nil {
		t.Fatal(err)
	}
	if armCalls != H1CaseCount*6 || evalCalls != H1CaseCount {
		t.Fatalf("calls arms=%d eval=%d", armCalls, evalCalls)
	}
	for _, rel := range []string{filepath.Join("eval", "batch-summary.json"), "adapter-summary.json", "h8-result.json"} {
		if _, err := os.Stat(filepath.Join(result.RunDir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

func TestH8RunnerRejectsWrongProblemCountFromEvaluator(t *testing.T) {
	dataset, err := LoadH8(writeValidH8Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	runner := H8Runner{NewBaseline: func(Case) (H8BaselineExecutor, error) { return countingH8ArmExecutor{new(int)}, nil }, Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
		return evalharness.ProblemResult{}, context.DeadlineExceeded
	}}}
	if _, err := runner.Run(context.Background(), RunRequest{Dataset: dataset, RunsRoot: t.TempDir(), RunID: "h8-runner-fail"}); err == nil {
		t.Fatal("expected failure to propagate from evaluator")
	}
}
