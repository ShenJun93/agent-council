package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type H4Runner struct {
	NewBaseline func(councilruntime.Provider) H4BaselineExecutor
	Evaluator   EvalExecutor
	Now         func() time.Time
}

func (r H4Runner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	if r.NewBaseline == nil {
		return RunResult{}, fmt.Errorf("H4 baseline factory is required")
	}
	if r.Evaluator == nil {
		return RunResult{}, fmt.Errorf("H4 evaluator is required")
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}

	runRoot, _, err := CreateH4Run(ctx, req.RunsRoot, req.RunID, req.Dataset, now())
	if err != nil {
		return RunResult{}, fmt.Errorf("create H4 run: %w", err)
	}
	problems := make([]evalharness.ProblemResult, 0, len(req.Dataset.Cases))
	for index, problem := range req.Dataset.Cases {
		if err := ctx.Err(); err != nil {
			return RunResult{}, err
		}
		executor := r.NewBaseline(problem.ChallengerProvider)
		if executor == nil {
			return RunResult{}, fmt.Errorf("problem %q: H4 baseline factory returned nil executor", problem.ID)
		}
		evaluated, err := runH4Problem(ctx, executor, r.Evaluator, req.RunID, runRoot, req.Dataset, problem)
		if err != nil {
			return RunResult{}, fmt.Errorf("problem %d %q: %w", index+1, problem.ID, err)
		}
		problems = append(problems, evaluated)
	}
	if len(problems) != H1CaseCount {
		return RunResult{}, fmt.Errorf("H4 evaluated %d problems, want %d", len(problems), H1CaseCount)
	}
	summary, err := evalharness.SummarizeBatch(problems, H4RiskPolicy)
	if err != nil {
		return RunResult{}, fmt.Errorf("summarize H4 evaluation: %w", err)
	}
	if err := evalharness.WriteEvaluation(ctx, evalharness.WriteRequest{
		Root: runRoot, Policy: H4RiskPolicy, Problems: problems, Summary: summary,
	}); err != nil {
		return RunResult{}, fmt.Errorf("persist H4 evaluation: %w", err)
	}
	if _, err := WriteH4FinalResult(ctx, runRoot, req.RunID, summary); err != nil {
		return RunResult{}, fmt.Errorf("persist H4 final result: %w", err)
	}
	return RunResult{RunID: req.RunID, RunDir: runRoot, Summary: summary}, nil
}
