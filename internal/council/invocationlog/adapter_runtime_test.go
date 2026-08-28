package invocationlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/adapterpool"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/structuredoutput"
)

func TestWrapAdapterPersistsZeroResponseAvailabilityFailure(t *testing.T) {
	root := t.TempDir()
	inner := &fakeRuntime{err: &councilruntime.RunError{Class: councilruntime.FailureAuth, Err: errors.New("not logged in")}}
	wrapped := WrapAdapter(inner, AdapterMetadata{ID: "claude-max", Provider: councilruntime.ProviderClaude})
	req := councilruntime.AgentRequest{
		RunID: "h5-run", RunRoot: root, Participant: "reviewer-1", Role: "reviewer", Phase: "review",
		Prompt: "prompt", SlotID: "reviewer-1", AdapterID: "claude-max", FailoverIndex: 0,
	}
	if _, err := wrapped.Run(context.Background(), req); err == nil {
		t.Fatal("expected auth failure")
	}

	path := filepath.Join(root, "invocations", "claude-max", "reviewer-1", "review", "000001.json")
	got := readAdapterEvidence(t, path)
	if got.SchemaVersion != AdapterSchemaVersion || got.AdapterID != "claude-max" || got.ProviderFamily != councilruntime.ProviderClaude {
		t.Fatalf("identity evidence=%+v", got)
	}
	if got.SlotID != "reviewer-1" || got.FailureClass != councilruntime.FailureAuth {
		t.Fatalf("slot/failure evidence=%+v", got)
	}
	if got.ExitCode != -1 || got.Attempts != 0 || got.StartedAt.IsZero() || got.FinishedAt.IsZero() {
		t.Fatalf("zero-response evidence=%+v", got)
	}
}

func TestWrapAdapterPreservesRawResponseAndFailoverMetadata(t *testing.T) {
	root := t.TempDir()
	raw := `{"type":"result","structured_output":{"decision":"ship"}}`
	inner := &fakeRuntime{response: councilruntime.AgentResponse{
		Provider: councilruntime.ProviderClaude, Stdout: raw, Stderr: "raw-stderr", ExitCode: 0, Attempts: 1,
		StartedAt: time.Unix(10, 0).UTC(), FinishedAt: time.Unix(11, 0).UTC(),
	}}
	wrapped := WrapAdapter(inner, AdapterMetadata{ID: "claude-max", Provider: councilruntime.ProviderClaude})
	schema := json.RawMessage(`{"type":"object","properties":{},"required":[],"additionalProperties":false}`)
	req := councilruntime.AgentRequest{
		RunID: "h5-run", RunRoot: root, Participant: "judge-1", Role: "judge", Phase: "judge", Prompt: "prompt",
		OutputSchema: schema, SlotID: "judge-1", AdapterID: "claude-max", FailoverIndex: 1,
		FailoverTrigger: councilruntime.FailureQuotaExhausted,
	}
	resp, err := wrapped.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Stdout != raw {
		t.Fatalf("wrapper changed raw stdout: %q", resp.Stdout)
	}
	path := filepath.Join(root, "invocations", "claude-max", "judge-1", "judge", "000001.json")
	got := readAdapterEvidence(t, path)
	if got.Stdout != raw || got.Stderr != "raw-stderr" || got.FailoverIndex != 1 || got.FailoverTrigger != councilruntime.FailureQuotaExhausted {
		t.Fatalf("raw/failover evidence=%+v", got)
	}
	promptHash := sha256.Sum256([]byte("prompt"))
	if got.PromptSHA256 != hex.EncodeToString(promptHash[:]) {
		t.Fatalf("prompt hash=%q", got.PromptSHA256)
	}
	schemaHash := sha256.Sum256(schema)
	if got.OutputSchemaSHA256 != hex.EncodeToString(schemaHash[:]) {
		t.Fatalf("schema hash=%q", got.OutputSchemaSHA256)
	}
}

func TestAdapterPoolLogsUnavailableThenSuccessfulStructuredAttempts(t *testing.T) {
	root := t.TempDir()
	primaryRaw := &fakeRuntime{response: councilruntime.AgentResponse{
		Provider: councilruntime.ProviderClaude, Stderr: "usage limit reached", ExitCode: 1, Attempts: 1,
		StartedAt: time.Unix(10, 0).UTC(), FinishedAt: time.Unix(11, 0).UTC(),
	}, err: &councilruntime.RunError{Class: councilruntime.FailureQuotaExhausted, Err: errors.New("quota")}}
	secondaryRaw := &fakeRuntime{response: councilruntime.AgentResponse{
		Provider: councilruntime.ProviderCodex, Stdout: `{"decision":"ship"}`, ExitCode: 0, Attempts: 1,
		StartedAt: time.Unix(12, 0).UTC(), FinishedAt: time.Unix(13, 0).UTC(),
	}}
	primary := structuredoutput.Wrap(WrapAdapter(primaryRaw, AdapterMetadata{ID: "claude-max", Provider: councilruntime.ProviderClaude}))
	secondary := structuredoutput.Wrap(WrapAdapter(secondaryRaw, AdapterMetadata{ID: "codex-chatgpt", Provider: councilruntime.ProviderCodex}))
	pool, err := adapterpool.New(map[adapterpool.AdapterID]adapterpool.Adapter{
		"claude-max":    {ID: "claude-max", Provider: councilruntime.ProviderClaude, Runtime: primary},
		"codex-chatgpt": {ID: "codex-chatgpt", Provider: councilruntime.ProviderCodex, Runtime: secondary},
	}, adapterpool.Policy{Slot: "baseline-a", Chain: []adapterpool.AdapterID{"claude-max", "codex-chatgpt"}})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := pool.Run(context.Background(), councilruntime.AgentRequest{
		RunID: "h5-run", RunRoot: root, Participant: "baseline-a", Role: "baseline", Phase: "baseline-final", Prompt: "prompt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Provider != councilruntime.ProviderCodex || resp.AdapterID != "codex-chatgpt" {
		t.Fatalf("fallback response=%+v", resp)
	}
	first := readAdapterEvidence(t, filepath.Join(root, "invocations", "claude-max", "baseline-a", "baseline-final", "000001.json"))
	second := readAdapterEvidence(t, filepath.Join(root, "invocations", "codex-chatgpt", "baseline-a", "baseline-final", "000001.json"))
	if first.FailureClass != councilruntime.FailureQuotaExhausted || first.OutputSchemaSHA256 == "" {
		t.Fatalf("primary evidence=%+v", first)
	}
	if second.FailureClass != "" || second.OutputSchemaSHA256 == "" || second.FailoverIndex != 1 {
		t.Fatalf("secondary evidence=%+v", second)
	}
}

func TestWrapAdapterRejectsMetadataMismatchBeforeInvocation(t *testing.T) {
	root := t.TempDir()
	inner := &fakeRuntime{response: modelResponse()}
	wrapped := WrapAdapter(inner, AdapterMetadata{ID: "claude-max", Provider: councilruntime.ProviderClaude})
	req := councilruntime.AgentRequest{
		RunID: "h5-run", RunRoot: root, Participant: "p", Role: "r", Phase: "phase",
		SlotID: "reviewer-1", AdapterID: "codex-chatgpt",
	}
	if _, err := wrapped.Run(context.Background(), req); err == nil {
		t.Fatal("expected metadata mismatch")
	}
	if inner.calls != 0 {
		t.Fatalf("inner calls=%d want 0", inner.calls)
	}
}

func readAdapterEvidence(t *testing.T, path string) AdapterEvidence {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got AdapterEvidence
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got
}
