package baseline

import (
	"context"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func adaptiveCouncilSlots() *protocol.SlotRuntimes {
	return &protocol.SlotRuntimes{
		Researcher1: &fakeRuntime{provider: councilruntime.ProviderCodex, recommendation: "r1", judgeDecision: "j1"},
		Researcher2: &fakeRuntime{provider: councilruntime.ProviderClaude, recommendation: "r2", judgeDecision: "j2"},
		Reviewer1:   &fakeRuntime{provider: councilruntime.ProviderCodex}, Reviewer2: &fakeRuntime{provider: councilruntime.ProviderClaude},
		Challenger: &fakeRuntime{provider: councilruntime.ProviderCodex},
		Judge1:     &fakeRuntime{provider: councilruntime.ProviderCodex, judgeDecision: "j1"},
		Judge2:     &fakeRuntime{provider: councilruntime.ProviderClaude, judgeDecision: "j2"},
	}
}

func TestAdaptiveBaselineUsesLogicalSidesNotProviders(t *testing.T) {
	t.Parallel()
	slotA := &fakeRuntime{provider: councilruntime.ProviderCodex, recommendation: "slot-a", judgeDecision: "a"}
	slotB := &fakeRuntime{provider: councilruntime.ProviderClaude, recommendation: "slot-b", judgeDecision: "b"}
	runner := Runner{SlotA: slotA, SlotB: slotB, CouncilSlots: adaptiveCouncilSlots(), TempRoot: t.TempDir(), CitationAuthority: CitationAuthorityProblemOnlyFinal}
	for _, arm := range []Arm{ArmAClaudeSingle, ArmCClaudeSelfReview, ArmBCodexSingle, ArmDCodexSelfReview} {
		if _, err := runner.RunArm(context.Background(), runRequest(t), arm); err != nil {
			t.Fatalf("arm %s: %v", arm, err)
		}
	}
	if got := len(slotA.snapshot()); got != 3 {
		t.Fatalf("slot A calls=%d want=3", got)
	}
	if got := len(slotB.snapshot()); got != 3 {
		t.Fatalf("slot B calls=%d want=3", got)
	}
}

func TestAdaptiveBaselineRequiresCompleteTopology(t *testing.T) {
	t.Parallel()
	runner := Runner{SlotA: &fakeRuntime{}, TempRoot: t.TempDir(), CitationAuthority: CitationAuthorityProblemOnlyFinal}
	_, err := runner.RunArm(context.Background(), runRequest(t), ArmAClaudeSingle)
	if err == nil {
		t.Fatal("expected incomplete adaptive topology rejection")
	}
}
