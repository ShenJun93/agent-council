package benchmark

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/invocationlog"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func writeH5Evidence(t *testing.T, root, rel string, e invocationlog.AdapterEvidence) {
	t.Helper()
	path := filepath.Join(root, "invocations", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCollectH5AdapterSummaryCountsRealizedBindings(t *testing.T) {
	root := t.TempDir()
	base := invocationlog.AdapterEvidence{SchemaVersion: invocationlog.AdapterSchemaVersion, RunID: "h5-test", SlotID: "reviewer-1", AdapterID: "claude-max", ProviderFamily: councilruntime.ProviderClaude}
	writeH5Evidence(t, root, "a/1.json", base)
	failed := base
	failed.AdapterID = "claude-max"
	failed.FailureClass = councilruntime.FailureQuotaExhausted
	writeH5Evidence(t, root, "a/2.json", failed)
	second := base
	second.SlotID = "reviewer-2"
	second.AdapterID = "codex-chatgpt"
	second.ProviderFamily = councilruntime.ProviderCodex
	second.FailoverIndex = 1
	second.FailoverTrigger = councilruntime.FailureQuotaExhausted
	writeH5Evidence(t, root, "b/1.json", second)
	failed2 := second
	failed2.FailureClass = councilruntime.FailureAuth
	writeH5Evidence(t, root, "b/2.json", failed2)
	human := base
	human.SlotID = "judge-1"
	human.AdapterID = "human-chatgpt-session"
	human.ProviderFamily = councilruntime.ProviderChatGPT
	human.FailoverIndex = 2
	human.FailoverTrigger = councilruntime.FailureAuth
	writeH5Evidence(t, root, "c/1.json", human)

	got, err := CollectH5AdapterSummary(context.Background(), root, "h5-test")
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalAttempts != 5 || got.SuccessfulInvocations != 3 || got.AvailabilityFailures != 2 || got.TotalAvailabilityFailovers != 2 {
		t.Fatalf("summary=%+v", got)
	}
	if got.EffectiveAdapterDiversity != 3 || got.EffectiveProviderDiversity != 3 || got.HumanBrokerInvocations != 1 {
		t.Fatalf("diversity=%+v", got)
	}
	if got.SuccessesByAdapter["claude-max"] != 1 || got.SuccessesByAdapter["codex-chatgpt"] != 1 || got.SuccessesByAdapter["human-chatgpt-session"] != 1 {
		t.Fatalf("adapters=%v", got.SuccessesByAdapter)
	}
	if got.SuccessesBySlot["judge-1"]["human-chatgpt-session"] != 1 {
		t.Fatalf("slots=%v", got.SuccessesBySlot)
	}
}

func TestCollectH5AdapterSummaryRejectsTerminalFailureEvidence(t *testing.T) {
	root := t.TempDir()
	e := invocationlog.AdapterEvidence{SchemaVersion: invocationlog.AdapterSchemaVersion, RunID: "h5-test", SlotID: "judge-1", AdapterID: "codex-chatgpt", ProviderFamily: councilruntime.ProviderCodex, FailureClass: councilruntime.FailureMalformedOutput}
	writeH5Evidence(t, root, "bad/1.json", e)
	if _, err := CollectH5AdapterSummary(context.Background(), root, "h5-test"); err == nil {
		t.Fatal("terminal failure evidence accepted")
	}
}

func TestCollectH5AdapterSummaryRejectsWrongRunAndLegacySchema(t *testing.T) {
	tests := []struct {
		name     string
		evidence invocationlog.AdapterEvidence
	}{
		{"wrong-run", invocationlog.AdapterEvidence{SchemaVersion: invocationlog.AdapterSchemaVersion, RunID: "other", SlotID: "judge-1", AdapterID: "codex-chatgpt", ProviderFamily: councilruntime.ProviderCodex}},
		{"legacy-schema", invocationlog.AdapterEvidence{SchemaVersion: "council.invocation-evidence.v1", RunID: "h5-test", SlotID: "judge-1", AdapterID: "codex-chatgpt", ProviderFamily: councilruntime.ProviderCodex}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeH5Evidence(t, root, "bad/1.json", tc.evidence)
			if _, err := CollectH5AdapterSummary(context.Background(), root, "h5-test"); err == nil {
				t.Fatal("invalid evidence accepted")
			}
		})
	}
}

func TestCollectH5AdapterSummaryRejectsSymlinkEvidence(t *testing.T) {
	root := t.TempDir()
	inv := filepath.Join(root, "invocations")
	if err := os.MkdirAll(inv, 0o750); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(inv, "link.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := CollectH5AdapterSummary(context.Background(), root, "h5-test"); err == nil {
		t.Fatal("symlink evidence accepted")
	}
}
