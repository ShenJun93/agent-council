package protocol

import (
	"context"
	"encoding/json"
	"testing"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func adaptiveProtocolSlots() (*SlotRuntimes, map[string]*fakeRuntime) {
	makeRT := func(provider councilruntime.Provider, recommendation, judge string) *fakeRuntime {
		return &fakeRuntime{provider: provider, recommendation: recommendation, confidence: 0.8, judgeDecision: judge}
	}
	runtimes := map[string]*fakeRuntime{
		"researcher-1": makeRT(councilruntime.ProviderCodex, "r1", ""),
		"researcher-2": makeRT(councilruntime.ProviderClaude, "r2", ""),
		"reviewer-1":   makeRT(councilruntime.ProviderCodex, "", ""),
		"reviewer-2":   makeRT(councilruntime.ProviderClaude, "", ""),
		"challenger":   makeRT(councilruntime.ProviderCodex, "", ""),
		"judge-1":      makeRT(councilruntime.ProviderCodex, "", "choose-one"),
		"judge-2":      makeRT(councilruntime.ProviderClaude, "", "choose-two"),
	}
	return &SlotRuntimes{
		Researcher1: runtimes["researcher-1"], Researcher2: runtimes["researcher-2"],
		Reviewer1: runtimes["reviewer-1"], Reviewer2: runtimes["reviewer-2"],
		Challenger: runtimes["challenger"], Judge1: runtimes["judge-1"], Judge2: runtimes["judge-2"],
	}, runtimes
}

func TestAdaptiveSlotsRunWithoutProviderBindings(t *testing.T) {
	t.Parallel()
	slots, runtimes := adaptiveProtocolSlots()
	engine := Engine{Slots: slots, TempRoot: t.TempDir(), ChallengePolicy: ChallengePolicy{AllowAbbreviated: false, HighConfidenceThreshold: 1}}
	_, err := engine.Run(context.Background(), RunRequest{
		RunID: "adaptive", RunRoot: t.TempDir(),
		NormalizedProblem: json.RawMessage(`{"problem":"choose"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := map[string]int{"researcher-1": 2, "researcher-2": 2, "reviewer-1": 1, "reviewer-2": 1, "challenger": 1, "judge-1": 1, "judge-2": 1}
	for name, want := range wantCalls {
		if got := len(runtimes[name].snapshot()); got != want {
			t.Fatalf("%s calls=%d want=%d", name, got, want)
		}
	}
	if got := runtimes["reviewer-1"].provider; got != councilruntime.ProviderCodex {
		t.Fatalf("reviewer-1 provider=%q", got)
	}
	if got := runtimes["judge-2"].provider; got != councilruntime.ProviderClaude {
		t.Fatalf("judge-2 provider=%q", got)
	}
}

func TestAdaptiveSlotsAlsoDriveFullInfo(t *testing.T) {
	t.Parallel()
	slots, _ := adaptiveProtocolSlots()
	engine := FullInfoEngine{Engine: Engine{Slots: slots, TempRoot: t.TempDir(), ChallengePolicy: ChallengePolicy{AllowAbbreviated: false, HighConfidenceThreshold: 1}}}
	if _, err := engine.Run(context.Background(), RunRequest{RunID: "adaptive-full", RunRoot: t.TempDir(), NormalizedProblem: json.RawMessage(`{"problem":"choose"}`)}); err != nil {
		t.Fatal(err)
	}
}

func TestAdaptiveSlotsRejectIncompleteTopology(t *testing.T) {
	t.Parallel()
	engine := Engine{Slots: &SlotRuntimes{Researcher1: &fakeRuntime{}}, TempRoot: t.TempDir()}
	_, err := engine.Run(context.Background(), RunRequest{RunID: "bad", RunRoot: t.TempDir(), NormalizedProblem: json.RawMessage(`{"problem":"choose"}`)})
	if err == nil {
		t.Fatal("expected incomplete slot topology rejection")
	}
}
