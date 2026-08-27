package benchmark

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

type fakeH2ArmExecutor struct {
	calls []baseline.Arm
	fail  baseline.Arm
}

func (f *fakeH2ArmExecutor) RunArm(_ context.Context, _ baseline.RunRequest, arm baseline.Arm) (baseline.ArmResult, error) {
	f.calls = append(f.calls, arm)
	if arm == f.fail {
		return baseline.ArmResult{}, errors.New("synthetic arm failure")
	}
	return baseline.ArmResult{Arm: arm, InvocationCount: 1}, nil
}

func TestRunH2ProblemPreservesCompletedArmsOnFailure(t *testing.T) {
	runRoot := t.TempDir()
	executor := &fakeH2ArmExecutor{fail: baseline.ArmCClaudeSelfReview}
	evalCalls := 0
	evaluator := fakeEvalExecutor{evaluate: func(context.Context, evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
		evalCalls++
		return evalharness.ProblemResult{}, nil
	}}
	problem := Case{ID: "tech-01-db-cutover", Problem: []byte(`{"id":"p"}`)}
	dataset := Dataset{Rubric: []byte(`{"dimensions":[]}`), RubricSHA256: "rubric"}

	_, err := runH2Problem(context.Background(), executor, evaluator, "h2-run", runRoot, dataset, problem)
	if err == nil || !stringsContains(err.Error(), "synthetic arm failure") {
		t.Fatalf("expected arm failure, got %v", err)
	}
	if evalCalls != 0 {
		t.Fatalf("eval calls = %d, want 0", evalCalls)
	}
	if len(executor.calls) != 3 {
		t.Fatalf("arm calls = %v, want A-C", executor.calls)
	}
	for _, arm := range []baseline.Arm{baseline.ArmAClaudeSingle, baseline.ArmBCodexSingle} {
		path := filepath.Join(runRoot, "baseline", problem.ID, "arm-"+string(arm)+".json")
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("completed arm %s not persisted: %v", arm, statErr)
		}
	}
	for _, arm := range []baseline.Arm{baseline.ArmCClaudeSelfReview, baseline.ArmDCodexSelfReview, baseline.ArmEFullInfo, baseline.ArmFBlindCouncil} {
		path := filepath.Join(runRoot, "baseline", problem.ID, "arm-"+string(arm)+".json")
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("unexpected arm %s artifact after failure: %v", arm, statErr)
		}
	}
	for _, rel := range []string{filepath.Join("eval", "batch-summary.json"), "h2-result.json"} {
		if _, statErr := os.Stat(filepath.Join(runRoot, rel)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("unexpected final artifact %s: %v", rel, statErr)
		}
	}
}
