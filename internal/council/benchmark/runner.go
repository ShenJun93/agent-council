package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type BaselineExecutor interface {
	RunAll(context.Context, baseline.RunRequest) ([]baseline.ArmResult, error)
}

type EvalExecutor interface {
	EvaluateProblem(context.Context, evalharness.ProblemRequest) (evalharness.ProblemResult, error)
}

type Runner struct {
	NewBaseline func(councilruntime.Provider) BaselineExecutor
	Evaluator   EvalExecutor
	Now         func() time.Time
}

type RunRequest struct {
	Dataset  Dataset
	RunsRoot string
	RunID    string
}

type RunResult struct {
	RunID          string                   `json:"run_id"`
	RunDir         string                   `json:"run_dir"`
	Summary        evalharness.BatchSummary `json:"batch_summary"`
	AdapterSummary *H5AdapterSummary        `json:"adapter_summary,omitempty"`
}

func (r Runner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	if r.NewBaseline == nil {
		return RunResult{}, fmt.Errorf("H1 baseline factory is required")
	}
	if r.Evaluator == nil {
		return RunResult{}, fmt.Errorf("H1 evaluator is required")
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}

	runRoot, _, err := CreateRun(ctx, req.RunsRoot, req.RunID, req.Dataset, now())
	if err != nil {
		return RunResult{}, fmt.Errorf("create H1 run: %w", err)
	}

	problems := make([]evalharness.ProblemResult, 0, len(req.Dataset.Cases))
	for index, problem := range req.Dataset.Cases {
		if err := ctx.Err(); err != nil {
			return RunResult{}, err
		}
		executor := r.NewBaseline(problem.ChallengerProvider)
		if executor == nil {
			return RunResult{}, fmt.Errorf("problem %q: baseline factory returned nil executor", problem.ID)
		}
		arms, err := executor.RunAll(ctx, baseline.RunRequest{
			RunID:             req.RunID,
			RunRoot:           runRoot,
			NormalizedProblem: problem.Problem,
		})
		if err != nil {
			return RunResult{}, fmt.Errorf("problem %d %q baseline: %w", index+1, problem.ID, err)
		}
		if err := WriteBaselineResults(ctx, runRoot, problem.ID, arms); err != nil {
			return RunResult{}, fmt.Errorf("problem %d %q baseline artifacts: %w", index+1, problem.ID, err)
		}

		evaluated, err := r.Evaluator.EvaluateProblem(ctx, evalharness.ProblemRequest{
			ProblemID:          problem.ID,
			RunID:              req.RunID,
			RunRoot:            runRoot,
			NormalizedProblem:  problem.Problem,
			Rubric:             req.Dataset.Rubric,
			RubricSHA256:       req.Dataset.RubricSHA256,
			ReferenceSet:       problem.ReferenceSet,
			ReferenceSetSHA256: problem.ReferenceSetSHA256,
			Arms:               arms,
			RiskPolicy:         H1RiskPolicy,
		})
		if err != nil {
			return RunResult{}, fmt.Errorf("problem %d %q evaluation: %w", index+1, problem.ID, err)
		}
		if evaluated.ProblemID != problem.ID {
			return RunResult{}, fmt.Errorf("problem %d %q evaluation returned problem id %q", index+1, problem.ID, evaluated.ProblemID)
		}
		if evaluated.RiskPolicy != H1RiskPolicy {
			return RunResult{}, fmt.Errorf("problem %d %q evaluation returned non-H1 risk policy", index+1, problem.ID)
		}
		problems = append(problems, evaluated)
	}

	if len(problems) != H1CaseCount {
		return RunResult{}, fmt.Errorf("H1 evaluated %d problems, want %d", len(problems), H1CaseCount)
	}
	summary, err := evalharness.SummarizeBatch(problems, H1RiskPolicy)
	if err != nil {
		return RunResult{}, fmt.Errorf("summarize H1 evaluation: %w", err)
	}
	if err := evalharness.WriteEvaluation(ctx, evalharness.WriteRequest{
		Root:     runRoot,
		Policy:   H1RiskPolicy,
		Problems: problems,
		Summary:  summary,
	}); err != nil {
		return RunResult{}, fmt.Errorf("persist H1 evaluation: %w", err)
	}
	if _, err := WriteFinalResult(ctx, runRoot, req.RunID, summary); err != nil {
		return RunResult{}, fmt.Errorf("persist H1 final result: %w", err)
	}

	return RunResult{RunID: req.RunID, RunDir: runRoot, Summary: summary}, nil
}
