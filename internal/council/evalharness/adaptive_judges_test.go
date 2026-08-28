package evalharness

import (
	"context"
	"testing"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func TestAdaptiveJudgesRecordActualProviders(t *testing.T) {
	t.Parallel()
	judge1 := &fakeJudgeRuntime{provider: councilruntime.ProviderCodex}
	judge2 := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude}
	harness := Harness{Adaptive: &AdaptiveJudgeRuntimes{Judge1: judge1, Judge2: judge2}, TempRoot: t.TempDir()}
	result, err := harness.EvaluateProblem(context.Background(), testProblemRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(judge1.snapshot()) != 6 || len(judge2.snapshot()) != 6 {
		t.Fatalf("adaptive calls=%d/%d", len(judge1.snapshot()), len(judge2.snapshot()))
	}
	for _, arm := range result.Arms {
		if arm.Judges[0].Provider != councilruntime.ProviderCodex || arm.Judges[1].Provider != councilruntime.ProviderClaude {
			t.Fatalf("arm %s providers=%q/%q", arm.Arm, arm.Judges[0].Provider, arm.Judges[1].Provider)
		}
	}
}

func TestAdaptiveJudgesRequireBothSlots(t *testing.T) {
	t.Parallel()
	harness := Harness{Adaptive: &AdaptiveJudgeRuntimes{Judge1: &fakeJudgeRuntime{provider: councilruntime.ProviderCodex}}, TempRoot: t.TempDir()}
	if _, err := harness.EvaluateProblem(context.Background(), testProblemRequest(t)); err == nil {
		t.Fatal("expected incomplete adaptive judges rejection")
	}
}

func TestLegacyJudgesStillRejectProviderSubstitution(t *testing.T) {
	t.Parallel()
	wrongClaude := &fakeJudgeRuntime{provider: councilruntime.ProviderCodex}
	codex := &fakeJudgeRuntime{provider: councilruntime.ProviderCodex}
	harness := Harness{Claude: wrongClaude, Codex: codex, TempRoot: t.TempDir()}
	_, err := harness.EvaluateProblem(context.Background(), testProblemRequest(t))
	if err == nil {
		t.Fatal("expected legacy provider substitution rejection")
	}
}
