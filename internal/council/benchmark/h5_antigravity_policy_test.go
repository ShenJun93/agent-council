package benchmark

import (
	"path/filepath"
	"testing"
)

func TestCommittedH5PolicyIncludesAntigravityBeforeHumanBroker(t *testing.T) {
	dataset, err := LoadH5(filepath.Join("..", "..", "..", "benchmarks", "h5"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, adapter := range dataset.AdapterPolicy.Adapters {
		if adapter.ID != "antigravity-subscription" {
			continue
		}
		found = true
		if adapter.ProviderFamily != "antigravity" || adapter.Transport != "agy-cli" || adapter.AuthClass != "subscription" || adapter.Interaction != "automated" || adapter.Model != "gemini-3.1-pro-high" {
			t.Fatalf("antigravity adapter=%+v", adapter)
		}
	}
	if !found {
		t.Fatal("antigravity-subscription adapter missing")
	}

	for slot, chain := range dataset.AdapterPolicy.Slots {
		assertAntigravityBeforeHuman(t, slot, chain)
	}
	for caseID, chain := range dataset.AdapterPolicy.ChallengerByCase {
		assertAntigravityBeforeHuman(t, "challenger:"+caseID, chain)
	}
}

func assertAntigravityBeforeHuman(t *testing.T, label string, chain []string) {
	t.Helper()
	agy, human := -1, -1
	for i, id := range chain {
		switch id {
		case "antigravity-subscription":
			agy = i
		case "human-chatgpt-session":
			human = i
		}
	}
	if agy < 0 || human < 0 || agy >= human {
		t.Fatalf("chain %s=%v must place antigravity before human broker", label, chain)
	}
}
