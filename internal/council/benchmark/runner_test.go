package benchmark

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type fakeBaselineExecutor struct {
	run func(context.Context, baseline.RunRequest) ([]baseline.ArmResult, error)
}

func (f fakeBaselineExecutor) RunAll(ctx context.Context, req baseline.RunRequest) ([]baseline.ArmResult, error) {
	return f.run(ctx, req)
}

type fakeEvalExecutor struct {
	evaluate func(context.Context, evalharness.ProblemRequest) (evalharness.ProblemResult, error)
}

func (f fakeEvalExecutor) EvaluateProblem(ctx context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
	return f.evaluate(ctx, req)
}

func TestRunnerExecutesFrozenH1InManifestOrder(t *testing.T) {
	datasetRoot := writeValidH1Fixture(t)
	dataset, err := LoadH1(datasetRoot)
	if err != nil {
		t.Fatal(err)
	}

	var providers []councilruntime.Provider
	var baselineRequests []baseline.RunRequest
	var evalRequests []evalharness.ProblemRequest
	runner := Runner{
		NewBaseline: func(provider councilruntime.Provider) BaselineExecutor {
			providers = append(providers, provider)
			return fakeBaselineExecutor{run: func(_ context.Context, req baseline.RunRequest) ([]baseline.ArmResult, error) {
				baselineRequests = append(baselineRequests, req)
				return sixArmResults(), nil
			}}
		},
		Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
			evalRequests = append(evalRequests, req)
			return scoredProblem(req.ProblemID), nil
		}},
		Now: func() time.Time { return time.Date(2026, 8, 27, 2, 3, 4, 0, time.UTC) },
	}

	result, err := runner.Run(context.Background(), RunRequest{
		Dataset: dataset,
		RunsRoot: t.TempDir(),
		RunID:    "h1-runner-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RunID != "h1-runner-test" || result.Summary.ProblemCount != H1CaseCount {
		t.Fatalf("unexpected run result: %#v", result)
	}
	if len(providers) != H1CaseCount || len(baselineRequests) != H1CaseCount || len(evalRequests) != H1CaseCount {
		t.Fatalf("call counts providers=%d baseline=%d eval=%d", len(providers), len(baselineRequests), len(evalRequests))
	}

	for i, c := range dataset.Cases {
		if providers[i] != c.ChallengerProvider {
			t.Fatalf("case %d provider=%q want %q", i+1, providers[i], c.ChallengerProvider)
		}
		if baselineRequests[i].RunID != "h1-runner-test" || baselineRequests[i].RunRoot != result.RunDir {
			t.Fatalf("case %d baseline run identity mismatch: %#v", i+1, baselineRequests[i])
		}
		if !bytes.Equal(baselineRequests[i].NormalizedProblem, c.Problem) {
			t.Fatalf("case %d baseline problem bytes differ", i+1)
		}

		evalReq := evalRequests[i]
		if evalReq.ProblemID != c.ID || evalReq.RunID != "h1-runner-test" || evalReq.RunRoot != result.RunDir {
			t.Fatalf("case %d eval identity mismatch: %#v", i+1, evalReq)
		}
		if !bytes.Equal(evalReq.NormalizedProblem, c.Problem) || !bytes.Equal(evalReq.Rubric, dataset.Rubric) || !bytes.Equal(evalReq.ReferenceSet, c.ReferenceSet) {
			t.Fatalf("case %d eval input bytes differ", i+1)
		}
		if evalReq.RubricSHA256 != dataset.RubricSHA256 || evalReq.ReferenceSetSHA256 != c.ReferenceSetSHA256 || evalReq.RiskPolicy != H1RiskPolicy {
			t.Fatalf("case %d eval frozen policy/hash mismatch", i+1)
		}
		if len(evalReq.Arms) != 6 {
			t.Fatalf("case %d eval arm count=%d", i+1, len(evalReq.Arms))
		}
	}

	for _, rel := range []string{
		filepath.Join("baseline", dataset.Cases[0].ID, "arm-A.json"),
		filepath.Join("baseline", dataset.Cases[H1CaseCount-1].ID, "arm-F.json"),
		filepath.Join("eval", "batch-summary.json"),
		"h1-result.json",
	} {
		if _, err := os.Stat(filepath.Join(result.RunDir, rel)); err != nil {
			t.Fatalf("missing final artifact %s: %v", rel, err)
		}
	}
}

func TestRunnerStopsAfterEvaluationFailureWithoutFinalSummary(t *testing.T) {
	datasetRoot := writeValidH1Fixture(t)
	dataset, err := LoadH1(datasetRoot)
	if err != nil {
		t.Fatal(err)
	}

	baselineCalls := 0
	evalCalls := 0
	runner := Runner{
		NewBaseline: func(councilruntime.Provider) BaselineExecutor {
			return fakeBaselineExecutor{run: func(_ context.Context, _ baseline.RunRequest) ([]baseline.ArmResult, error) {
				baselineCalls++
				return sixArmResults(), nil
			}}
		},
		Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
			evalCalls++
			if evalCalls == 3 {
				return evalharness.ProblemResult{}, errors.New("synthetic evaluator failure")
			}
			return scoredProblem(req.ProblemID), nil
		}},
		Now: time.Now,
	}
	runsRoot := t.TempDir()

	_, err = runner.Run(context.Background(), RunRequest{Dataset: dataset, RunsRoot: runsRoot, RunID: "h1-failure-test"})
	if err == nil || !stringsContains(err.Error(), "synthetic evaluator failure") {
		t.Fatalf("expected evaluator failure, got %v", err)
	}
	if baselineCalls != 3 || evalCalls != 3 {
		t.Fatalf("calls after failure baseline=%d eval=%d, want 3/3", baselineCalls, evalCalls)
	}
	runRoot := filepath.Join(runsRoot, "h1-failure-test")
	if _, err := os.Stat(filepath.Join(runRoot, "h1-result.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("final H1 result exists after partial failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "eval", "batch-summary.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("batch summary exists after partial failure: %v", err)
	}
}

func scoredProblem(problemID string) evalharness.ProblemResult {
	means := []float64{70, 72, 71, 73, 74, 76}
	arms := make([]evalharness.ArmScore, 0, len(h1BaselineArms))
	for i, arm := range h1BaselineArms {
		arms = append(arms, evalharness.ArmScore{Arm: arm, MeanScore: means[i], JudgeSpread: 1})
	}
	return evalharness.ProblemResult{
		ProblemID:  problemID,
		RiskPolicy: H1RiskPolicy,
		Arms:       arms,
	}
}

func stringsContains(value, needle string) bool {
	return bytes.Contains([]byte(value), []byte(needle))
}
