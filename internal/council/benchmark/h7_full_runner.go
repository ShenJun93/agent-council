package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

type H7Runner struct {
	NewBaseline           func(Case) (H7BaselineExecutor, error)
	Evaluator             EvalExecutor
	Now                   func() time.Time
	CollectAdapterSummary func(context.Context, string, string) (H5AdapterSummary, error)
}

func (r H7Runner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	if r.NewBaseline == nil {
		return RunResult{}, fmt.Errorf("H7 baseline factory is required")
	}
	if r.Evaluator == nil {
		return RunResult{}, fmt.Errorf("H7 evaluator is required")
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}

	runRoot, _, err := CreateH7Run(ctx, req.RunsRoot, req.RunID, req.Dataset, now())
	if err != nil {
		return RunResult{}, fmt.Errorf("create H7 run: %w", err)
	}
	problems := make([]evalharness.ProblemResult, 0, len(req.Dataset.Cases))
	for index, problem := range req.Dataset.Cases {
		if err := ctx.Err(); err != nil {
			return RunResult{}, err
		}
		executor, factoryErr := r.NewBaseline(problem)
		if factoryErr != nil {
			return RunResult{}, fmt.Errorf("problem %q: H7 baseline factory: %w", problem.ID, factoryErr)
		}
		if executor == nil {
			return RunResult{}, fmt.Errorf("problem %q: H7 baseline factory returned nil executor", problem.ID)
		}
		evaluated, err := runH7Problem(ctx, executor, r.Evaluator, req.RunID, runRoot, req.Dataset, problem)
		if err != nil {
			return RunResult{}, fmt.Errorf("problem %d %q: %w", index+1, problem.ID, err)
		}
		problems = append(problems, evaluated)
	}
	if len(problems) != H1CaseCount {
		return RunResult{}, fmt.Errorf("H7 evaluated %d problems, want %d", len(problems), H1CaseCount)
	}
	summary, err := evalharness.SummarizeBatch(problems, H7RiskPolicy)
	if err != nil {
		return RunResult{}, fmt.Errorf("summarize H7 evaluation: %w", err)
	}
	if err := evalharness.WriteEvaluation(ctx, evalharness.WriteRequest{
		Root: runRoot, Policy: H7RiskPolicy, Problems: problems, Summary: summary,
	}); err != nil {
		return RunResult{}, fmt.Errorf("persist H7 evaluation: %w", err)
	}
	collector := CollectH5AdapterSummary
	if r.CollectAdapterSummary != nil {
		collector = r.CollectAdapterSummary
	}
	adapterSummary, err := collector(ctx, runRoot, req.RunID)
	if err != nil {
		return RunResult{}, fmt.Errorf("collect H7 adapter summary: %w", err)
	}
	if adapterSummary.SuccessfulInvocations != H5ExpectedSuccessfulInvocations {
		return RunResult{}, fmt.Errorf("H7 successful logical invocations %d, want %d", adapterSummary.SuccessfulInvocations, H5ExpectedSuccessfulInvocations)
	}
	adapterHash, err := WriteH5AdapterSummary(ctx, runRoot, adapterSummary)
	if err != nil {
		return RunResult{}, fmt.Errorf("persist H7 adapter summary: %w", err)
	}
	if _, err := WriteH7FinalResult(ctx, runRoot, req.RunID, summary, adapterSummary, adapterHash); err != nil {
		return RunResult{}, fmt.Errorf("persist H7 final result: %w", err)
	}
	return RunResult{RunID: req.RunID, RunDir: runRoot, Summary: summary, AdapterSummary: &adapterSummary}, nil
}
