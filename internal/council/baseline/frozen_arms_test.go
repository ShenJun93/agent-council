package baseline

import "testing"

func TestFrozenArmsReturnsDefensiveCopy(t *testing.T) {
	first := FrozenArms()
	want := []Arm{ArmAClaudeSingle, ArmBCodexSingle, ArmCClaudeSelfReview, ArmDCodexSelfReview, ArmEFullInfo, ArmFBlindCouncil}
	if len(first) != len(want) {
		t.Fatalf("FrozenArms() length = %d, want %d", len(first), len(want))
	}
	for i := range want {
		if first[i] != want[i] {
			t.Fatalf("FrozenArms()[%d] = %q, want %q", i, first[i], want[i])
		}
	}
	first[0] = Arm("mutated")
	second := FrozenArms()
	if second[0] != ArmAClaudeSingle {
		t.Fatalf("FrozenArms() leaked mutable backing array: %v", second)
	}
}
