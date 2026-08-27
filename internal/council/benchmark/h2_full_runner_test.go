package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type countingH2ArmExecutor struct{ count *int }

func (f countingH2ArmExecutor) RunArm(_ context.Context, _ baseline.RunRequest, arm baseline.Arm) (baseline.ArmResult, error) {
	(*f.count)++
	return baseline.ArmResult{Arm: arm, InvocationCount: 1}, nil
}

func TestH2RunnerCompletesWithVersionedFinalArtifacts(t *testing.T) {
	dataset, err := LoadH2(writeValidH2Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	armCalls, evalCalls := 0, 0
	runner := H2Runner{
		NewBaseline: func(councilruntime.Provider) H2BaselineExecutor {
			return countingH2ArmExecutor{count: &armCalls}
		},
		Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
			evalCalls++
			return scoredProblem(req.ProblemID), nil
		}},
		Now: func() time.Time { return time.Date(2026, 8, 27, 8, 30, 0, 0, time.UTC) },
	}
	runsRoot := t.TempDir()
	result, err := runner.Run(context.Background(), RunRequest{Dataset: dataset, RunsRoot: runsRoot, RunID: "h2-runner-test"})
	if err != nil {
		t.Fatal(err)
	}
	if armCalls != H1CaseCount*6 || evalCalls != H1CaseCount {
		t.Fatalf("calls arms=%d eval=%d", armCalls, evalCalls)
	}
	if result.RunID != "h2-runner-test" || result.Summary.ProblemCount != H1CaseCount {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, rel := range []string{
		filepath.Join("baseline", dataset.Cases[0].ID, "arm-A.json"),
		filepath.Join("baseline", dataset.Cases[H1CaseCount-1].ID, "arm-F.json"),
		filepath.Join("eval", "batch-summary.json"),
		"h2-result.json",
	} {
		if _, err := os.Stat(filepath.Join(result.RunDir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}
