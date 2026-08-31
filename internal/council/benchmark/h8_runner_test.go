package benchmark

import (
	"context"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

type countingH8ArmExecutor struct{ count *int }

func (f countingH8ArmExecutor) RunArm(_ context.Context, _ baseline.RunRequest, arm baseline.Arm) (baseline.ArmResult, error) {
	(*f.count)++
	return baseline.ArmResult{Arm: arm, InvocationCount: 1}, nil
}

func TestRunH8ProblemRequiresExecutorAndEvaluator(t *testing.T) {
	dataset, err := LoadH8(writeValidH8Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	evaluator := fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
		return scoredProblem(req.ProblemID), nil
	}}
	if _, err := runH8Problem(context.Background(), nil, evaluator, "h8-runner-test", t.TempDir(), dataset, dataset.Cases[0]); err == nil {
		t.Fatal("expected error for nil baseline executor")
	}
	armCount := 0
	if _, err := runH8Problem(context.Background(), countingH8ArmExecutor{&armCount}, nil, "h8-runner-test", t.TempDir(), dataset, dataset.Cases[0]); err == nil {
		t.Fatal("expected error for nil evaluator")
	}
}

func TestRunH8ProblemUsesH8RiskPolicyAndSixArms(t *testing.T) {
	dataset, err := LoadH8(writeValidH8Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	armCount := 0
	var gotPolicy evalharness.RiskPolicy
	evaluator := fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
		gotPolicy = req.RiskPolicy
		return scoredProblem(req.ProblemID), nil
	}}
	result, err := runH8Problem(context.Background(), countingH8ArmExecutor{&armCount}, evaluator, "h8-runner-test", t.TempDir(), dataset, dataset.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	if armCount != 6 {
		t.Fatalf("arm calls=%d, want 6", armCount)
	}
	if gotPolicy != H8RiskPolicy {
		t.Fatalf("risk policy=%+v want %+v", gotPolicy, H8RiskPolicy)
	}
	if result.ProblemID != dataset.Cases[0].ID {
		t.Fatalf("problem id=%q want %q", result.ProblemID, dataset.Cases[0].ID)
	}
}
