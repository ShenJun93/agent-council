package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

type countingH5ArmExecutor struct{ count *int }

func (f countingH5ArmExecutor) RunArm(_ context.Context, _ baseline.RunRequest, arm baseline.Arm) (baseline.ArmResult, error) {
	(*f.count)++
	return baseline.ArmResult{Arm: arm, InvocationCount: 1}, nil
}

func TestH5RunnerCompletesWithVersionedFinalArtifacts(t *testing.T) {
	dataset, err := LoadH5(writeValidH5Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	armCalls, evalCalls := 0, 0
	runner := H5Runner{
		NewBaseline: func(Case) (H5BaselineExecutor, error) {
			return countingH5ArmExecutor{count: &armCalls}, nil
		},
		Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
			evalCalls++
			return scoredProblem(req.ProblemID), nil
		}},
		Now: func() time.Time { return time.Date(2026, 8, 27, 8, 30, 0, 0, time.UTC) },
		CollectAdapterSummary: func(context.Context, string, string) (H5AdapterSummary, error) {
			return H5AdapterSummary{SchemaVersion: H5AdapterSummarySchemaVersion, SuccessfulInvocations: H5ExpectedSuccessfulInvocations, SuccessesByAdapter: map[string]int{}, SuccessesByProvider: map[string]int{}, AttemptsByAdapter: map[string]int{}, AvailabilityFailuresByAdapter: map[string]int{}, SuccessesBySlot: map[string]map[string]int{}}, nil
		},
	}
	runsRoot := t.TempDir()
	result, err := runner.Run(context.Background(), RunRequest{Dataset: dataset, RunsRoot: runsRoot, RunID: "h5-runner-test"})
	if err != nil {
		t.Fatal(err)
	}
	if armCalls != H1CaseCount*6 || evalCalls != H1CaseCount {
		t.Fatalf("calls arms=%d eval=%d", armCalls, evalCalls)
	}
	if result.RunID != "h5-runner-test" || result.Summary.ProblemCount != H1CaseCount {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, rel := range []string{
		filepath.Join("baseline", dataset.Cases[0].ID, "arm-A.json"),
		filepath.Join("baseline", dataset.Cases[H1CaseCount-1].ID, "arm-F.json"),
		filepath.Join("eval", "batch-summary.json"),
		"adapter-summary.json",
		"h5-result.json",
	} {
		if _, err := os.Stat(filepath.Join(result.RunDir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

func TestH5RunnerRejectsIncompleteAdapterSummary(t *testing.T) {
	dataset, err := LoadH5(writeValidH5Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	runner := H5Runner{
		NewBaseline: func(Case) (H5BaselineExecutor, error) { count := 0; return countingH5ArmExecutor{count: &count}, nil },
		Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
			return scoredProblem(req.ProblemID), nil
		}},
		CollectAdapterSummary: func(context.Context, string, string) (H5AdapterSummary, error) {
			return H5AdapterSummary{SchemaVersion: H5AdapterSummarySchemaVersion, SuccessfulInvocations: H5ExpectedSuccessfulInvocations - 1}, nil
		},
	}
	_, err = runner.Run(context.Background(), RunRequest{Dataset: dataset, RunsRoot: t.TempDir(), RunID: "h5-incomplete"})
	if err == nil {
		t.Fatal("incomplete adapter summary accepted")
	}
}
