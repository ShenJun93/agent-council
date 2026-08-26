package protocol

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func TestFullInfoEngineKeepsCallBudgetAndShowsPeerArtifacts(t *testing.T) {
	t.Parallel()

	runRoot := t.TempDir()
	claude := &fakeRuntime{
		provider:       councilruntime.ProviderClaude,
		recommendation: "alpha recommendation",
		confidence:     0.82,
		judgeDecision:  "choose alpha",
	}
	codex := &fakeRuntime{
		provider:       councilruntime.ProviderCodex,
		recommendation: "beta recommendation",
		confidence:     0.81,
		judgeDecision:  "choose beta",
	}

	engine := FullInfoEngine{Engine: Engine{
		Claude:             claude,
		Codex:              codex,
		TempRoot:           t.TempDir(),
		ChallengerProvider: councilruntime.ProviderClaude,
	}}
	_, err := engine.Run(context.Background(), RunRequest{
		RunID:             "full-info-test",
		RunRoot:           runRoot,
		NormalizedProblem: json.RawMessage(`{"problem":"compare visibility policies"}`),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	allCalls := append(claude.snapshot(), codex.snapshot()...)
	if len(allCalls) != 9 {
		t.Fatalf("runtime calls = %d, want 9", len(allCalls))
	}

	seenWorkdirs := map[string]struct{}{}
	reviewCount := 0
	rebuttalCount := 0
	for _, call := range allCalls {
		rel, err := filepath.Rel(runRoot, call.Workdir)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("workdir %q is inside run root %q", call.Workdir, runRoot)
		}
		if _, duplicate := seenWorkdirs[call.Workdir]; duplicate {
			t.Fatalf("workdir reused across invocations: %q", call.Workdir)
		}
		seenWorkdirs[call.Workdir] = struct{}{}

		switch call.Phase {
		case PhaseReview:
			reviewCount++
			if !strings.Contains(call.Prompt, "alpha recommendation") || !strings.Contains(call.Prompt, "beta recommendation") {
				t.Fatalf("full-info reviewer did not receive both research reports: %q", call.Prompt)
			}
		case PhaseRebuttal:
			rebuttalCount++
			for _, marker := range []string{"alpha recommendation", "beta recommendation", "strength", "weakness"} {
				if !strings.Contains(call.Prompt, marker) {
					t.Fatalf("full-info rebuttal missing prior artifact marker %q: %q", marker, call.Prompt)
				}
			}
		}
	}
	if reviewCount != 2 || rebuttalCount != 2 {
		t.Fatalf("review/rebuttal calls = %d/%d, want 2/2", reviewCount, rebuttalCount)
	}
}
