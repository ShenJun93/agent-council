package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type H2Runner struct {
	NewBaseline func(councilruntime.Provider) H2BaselineExecutor
	Evaluator   EvalExecutor
	Now         func() time.Time
}

func (r H2Runner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	if r.NewBaseline == nil {
		return RunResult{}, fmt.Errorf("H2 baseline factory is required")
	}
	if r.Evaluator == nil {
		return RunResult{}, fmt.Errorf("H2 evaluator is required")
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}

	runRoot, _, err := CreateH2Run(ctx, req.RunsRoot, req.RunID, req.Dataset, now())
	if err != nil {
		return RunResult{}, fmt.Errorf("create H2 run: %w", err)
	}
	problems := make([]evalharness.ProblemResult, 0, len(req.Dataset.Cases))
	for index, problem := range req.Dataset.Cases {
		if err := ctx.Err(); err != nil {
			return RunResult{}, err
		}
		executor := r.NewBaseline(problem.ChallengerProvider)
		if executor == nil {
			return RunResult{}, fmt.Errorf("problem %q: H2 baseline factory returned nil executor", problem.ID)
		}
		evaluated, err := runH2Problem(ctx, executor, r.Evaluator, req.RunID, runRoot, req.Dataset, problem)
		if err != nil {
			return RunResult{}, fmt.Errorf("problem %d %q: %w", index+1, problem.ID, err)
		}
		problems = append(problems, evaluated)
	}
	if len(problems) != H1CaseCount {
		return RunResult{}, fmt.Errorf("H2 evaluated %d problems, want %d", len(problems), H1CaseCount)
	}
	summary, err := evalharness.SummarizeBatch(problems, H2RiskPolicy)
	if err != nil {
		return RunResult{}, fmt.Errorf("summarize H2 evaluation: %w", err)
	}
	if err := evalharness.WriteEvaluation(ctx, evalharness.WriteRequest{
		Root: runRoot, Policy: H2RiskPolicy, Problems: problems, Summary: summary,
	}); err != nil {
		return RunResult{}, fmt.Errorf("persist H2 evaluation: %w", err)
	}
	if _, err := WriteH2FinalResult(ctx, runRoot, req.RunID, summary); err != nil {
		return RunResult{}, fmt.Errorf("persist H2 final result: %w", err)
	}
	return RunResult{RunID: req.RunID, RunDir: runRoot, Summary: summary}, nil
}
