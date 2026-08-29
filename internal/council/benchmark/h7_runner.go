package benchmark

import (
	"context"
	"fmt"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

type H7BaselineExecutor interface {
	RunArm(context.Context, baseline.RunRequest, baseline.Arm) (baseline.ArmResult, error)
}

func runH7Problem(
	ctx context.Context,
	executor H7BaselineExecutor,
	evaluator EvalExecutor,
	runID string,
	runRoot string,
	dataset Dataset,
	problem Case,
) (evalharness.ProblemResult, error) {
	if executor == nil {
		return evalharness.ProblemResult{}, fmt.Errorf("H7 baseline executor is required")
	}
	if evaluator == nil {
		return evalharness.ProblemResult{}, fmt.Errorf("H7 evaluator is required")
	}
	arms := make([]baseline.ArmResult, 0, len(baseline.FrozenArms()))
	request := baseline.RunRequest{
		RunID:             runID,
		RunRoot:           runRoot,
		NormalizedProblem: problem.Problem,
	}
	for _, arm := range baseline.FrozenArms() {
		if err := ctx.Err(); err != nil {
			return evalharness.ProblemResult{}, err
		}
		result, err := executor.RunArm(ctx, request, arm)
		if err != nil {
			return evalharness.ProblemResult{}, fmt.Errorf("baseline arm %s: %w", arm, err)
		}
		if result.Arm != arm {
			return evalharness.ProblemResult{}, fmt.Errorf("baseline arm %s returned arm %s", arm, result.Arm)
		}
		if err := WriteBaselineArmResult(ctx, runRoot, problem.ID, result); err != nil {
			return evalharness.ProblemResult{}, fmt.Errorf("persist baseline arm %s: %w", arm, err)
		}
		arms = append(arms, result)
	}

	evaluated, err := evaluator.EvaluateProblem(ctx, evalharness.ProblemRequest{
		ProblemID:          problem.ID,
		RunID:              runID,
		RunRoot:            runRoot,
		NormalizedProblem:  problem.Problem,
		Rubric:             dataset.Rubric,
		RubricSHA256:       dataset.RubricSHA256,
		ReferenceSet:       problem.ReferenceSet,
		ReferenceSetSHA256: problem.ReferenceSetSHA256,
		Arms:               arms,
		RiskPolicy:         H7RiskPolicy,
	})
	if err != nil {
		return evalharness.ProblemResult{}, fmt.Errorf("evaluation: %w", err)
	}
	if evaluated.ProblemID != problem.ID {
		return evalharness.ProblemResult{}, fmt.Errorf("evaluation returned problem id %q want %q", evaluated.ProblemID, problem.ID)
	}
	if evaluated.RiskPolicy != H7RiskPolicy {
		return evalharness.ProblemResult{}, fmt.Errorf("evaluation returned non-H7 risk policy")
	}
	return evaluated, nil
}
