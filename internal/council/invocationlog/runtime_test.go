package invocationlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type fakeRuntime struct {
	response councilruntime.AgentResponse
	err      error
	mu       sync.Mutex
	calls    int
}

func (f *fakeRuntime) Run(_ context.Context, _ councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.response, f.err
}

func modelResponse() councilruntime.AgentResponse {
	return councilruntime.AgentResponse{Provider: councilruntime.ProviderClaude, Stdout: "raw-out", Stderr: "raw-err", ExitCode: 0, Attempts: 1, StartedAt: time.Unix(10, 0).UTC(), FinishedAt: time.Unix(11, 0).UTC()}
}

func TestWrapPersistsSuccessfulInvocation(t *testing.T) {
	root := t.TempDir()
	inner := &fakeRuntime{response: modelResponse()}
	wrapped := Wrap(inner, councilruntime.ProviderClaude)
	req := councilruntime.AgentRequest{RunID: "h2-run", RunRoot: root, Participant: "baseline-a", Role: "baseline", Phase: "baseline-final", Prompt: "prompt"}
	if _, err := wrapped.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "invocations", "claude", "baseline-a", "baseline-final", "000001.json")
	evidence := readEvidence(t, path)
	if evidence.Stdout != "raw-out" || evidence.Stderr != "raw-err" {
		t.Fatalf("raw response not preserved: %+v", evidence)
	}
	digest := sha256.Sum256([]byte("prompt"))
	if evidence.PromptSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("prompt hash = %q", evidence.PromptSHA256)
	}
	if evidence.SchemaVersion != SchemaVersion || evidence.RunID != "h2-run" || evidence.Attempts != 1 {
		t.Fatalf("unexpected evidence: %+v", evidence)
	}
}

func TestWrapPersistsFailedModelInvocation(t *testing.T) {
	root := t.TempDir()
	inner := &fakeRuntime{response: modelResponse(), err: &councilruntime.RunError{Class: councilruntime.FailureQuotaExhausted, Err: errors.New("quota")}}
	wrapped := Wrap(inner, councilruntime.ProviderClaude)
	req := councilruntime.AgentRequest{RunID: "h2-run", RunRoot: root, Participant: "p", Role: "r", Phase: "phase", Prompt: "prompt"}
	if _, err := wrapped.Run(context.Background(), req); err == nil {
		t.Fatal("expected provider error")
	}
	if _, err := os.Stat(filepath.Join(root, "invocations", "claude", "p", "phase", "000001.json")); err != nil {
		t.Fatal(err)
	}
}

func TestWrapSkipsEvidenceForPreModelFailure(t *testing.T) {
	root := t.TempDir()
	inner := &fakeRuntime{err: &councilruntime.RunError{Class: councilruntime.FailureAuth, Err: errors.New("auth")}}
	wrapped := Wrap(inner, councilruntime.ProviderClaude)
	req := councilruntime.AgentRequest{RunID: "h2-run", RunRoot: root, Participant: "p", Role: "r", Phase: "phase", Prompt: "prompt"}
	if _, err := wrapped.Run(context.Background(), req); err == nil {
		t.Fatal("expected auth error")
	}
	if _, err := os.Stat(filepath.Join(root, "invocations")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected invocation evidence: %v", err)
	}
}

func TestWrapFailsClosedWhenEvidenceCannotBeWritten(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "invocations"), []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	wrapped := Wrap(&fakeRuntime{response: modelResponse()}, councilruntime.ProviderClaude)
	req := councilruntime.AgentRequest{RunID: "h2-run", RunRoot: root, Participant: "p", Role: "r", Phase: "phase", Prompt: "prompt"}
	response, err := wrapped.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected isolation error")
	}
	if response.Stdout != "raw-out" {
		t.Fatalf("response lost: %+v", response)
	}
	var runErr *councilruntime.RunError
	if !errors.As(err, &runErr) || runErr.Class != councilruntime.FailureIsolation {
		t.Fatalf("error = %v", err)
	}
}

func TestWrapRejectsUnsafePathComponentsBeforeInvocation(t *testing.T) {
	for _, req := range []councilruntime.AgentRequest{
		{RunID: "h2-run", RunRoot: t.TempDir(), Participant: "../escape", Phase: "phase"},
		{RunID: "h2-run", RunRoot: t.TempDir(), Participant: "p", Phase: "a/b"},
	} {
		inner := &fakeRuntime{response: modelResponse()}
		if _, err := Wrap(inner, councilruntime.ProviderClaude).Run(context.Background(), req); err == nil {
			t.Fatal("expected unsafe component error")
		}
		if inner.calls != 0 {
			t.Fatalf("inner called %d times", inner.calls)
		}
	}
}

func TestWrapUsesUniqueSequenceForConcurrentInvocations(t *testing.T) {
	root := t.TempDir()
	wrapped := Wrap(&fakeRuntime{response: modelResponse()}, councilruntime.ProviderClaude)
	const count = 16
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := wrapped.Run(context.Background(), councilruntime.AgentRequest{RunID: "h2-run", RunRoot: root, Participant: "p", Role: "r", Phase: "phase", Prompt: "prompt"})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	files, err := filepath.Glob(filepath.Join(root, "invocations", "claude", "p", "phase", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != count {
		t.Fatalf("files = %d want %d: %v", len(files), count, files)
	}
}

func readEvidence(t *testing.T, path string) Evidence {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Evidence
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestWrapPersistsOutputSchemaDigestWhenPresent(t *testing.T) {
	root := t.TempDir()
	wrapped := Wrap(&fakeRuntime{response: modelResponse()}, councilruntime.ProviderClaude)
	schema := json.RawMessage(`{"type":"object","properties":{},"required":[],"additionalProperties":false}`)
	req := councilruntime.AgentRequest{
		RunID: "h4-run", RunRoot: root, Participant: "p", Role: "baseline",
		Phase: "baseline-final", Prompt: "prompt", OutputSchema: schema,
	}
	if _, err := wrapped.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "invocations", "claude", "p", "baseline-final", "000001.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := json.Unmarshal(raw["output_schema_sha256"], &got); err != nil {
		t.Fatalf("missing output_schema_sha256: %v; evidence=%s", err, data)
	}
	digest := sha256.Sum256(schema)
	if want := hex.EncodeToString(digest[:]); got != want {
		t.Fatalf("schema hash=%q want %q", got, want)
	}
}

func TestWrapOmitsOutputSchemaDigestForLegacyRequest(t *testing.T) {
	root := t.TempDir()
	wrapped := Wrap(&fakeRuntime{response: modelResponse()}, councilruntime.ProviderClaude)
	req := councilruntime.AgentRequest{
		RunID: "h3-run", RunRoot: root, Participant: "p", Role: "baseline",
		Phase: "baseline-final", Prompt: "prompt",
	}
	if _, err := wrapped.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "invocations", "claude", "p", "baseline-final", "000001.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, exists := raw["output_schema_sha256"]; exists {
		t.Fatalf("legacy evidence unexpectedly includes output_schema_sha256: %s", data)
	}
}
