package humanbroker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedRuntime() *Runtime {
	return &Runtime{
		WaitTimeout:  2 * time.Second,
		PollInterval: 5 * time.Millisecond,
		NewRequestID: func() (string, error) { return "req-001", nil },
		NewNonce:     func() (string, error) { return "nonce-001", nil },
	}
}

func testRequest(t *testing.T) councilruntime.AgentRequest {
	t.Helper()
	return councilruntime.AgentRequest{
		RunID: "h5-test", RunRoot: t.TempDir(), SlotID: "reviewer-1",
		AdapterID: "human-chatgpt-session", Participant: "reviewer-1",
		FailoverIndex: 2, FailoverTrigger: councilruntime.FailureAuth,
		Role: "reviewer", Phase: "cross-review", Prompt: "REVIEW THIS CANDIDATE",
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`),
	}
}

func TestRuntimeWritesPacketAndReturnsFreshSubmittedResponse(t *testing.T) {
	t.Parallel()
	req := testRequest(t)
	rt := fixedRuntime()
	type result struct {
		resp councilruntime.AgentResponse
		err  error
	}
	done := make(chan result, 1)
	go func() { resp, err := rt.Run(context.Background(), req); done <- result{resp, err} }()
	var packet RequestPacket
	waitJSON(t, filepath.Join(req.RunRoot, "human-broker", "req-001", "request.json"), &packet)
	if packet.RequestID != "req-001" || packet.Nonce != "nonce-001" || !packet.RequireFreshSession {
		t.Fatalf("packet=%+v", packet)
	}
	if packet.ProviderFamily != "chatgpt" || packet.AdapterID != "human-chatgpt-session" {
		t.Fatalf("provenance=%+v", packet)
	}
	if packet.FailoverIndex != 2 || packet.FailoverTrigger != councilruntime.FailureAuth {
		t.Fatalf("failover=%+v", packet)
	}
	joined := strings.ToLower(strings.Join(packet.Instructions, " "))
	if !strings.Contains(joined, "new chat") || !strings.Contains(joined, "prior context") {
		t.Fatalf("instructions=%v", packet.Instructions)
	}
	ph := sha256.Sum256([]byte(req.Prompt))
	sh := sha256.Sum256(req.OutputSchema)
	if packet.PromptSHA256 != hex.EncodeToString(ph[:]) || packet.OutputSchemaSHA256 != hex.EncodeToString(sh[:]) {
		t.Fatalf("hashes=%+v", packet)
	}
	if packet.Prompt != req.Prompt || string(packet.OutputSchema) != string(req.OutputSchema) {
		t.Fatal("packet content drift")
	}
	if err := SubmitResponse(req.RunRoot, Submission{RequestID: "req-001", Nonce: "nonce-001", FreshSession: true, ModelLabel: "ChatGPT", RawResponse: `{"ok":true}`}); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.resp.Provider != councilruntime.ProviderChatGPT || got.resp.Stdout != `{"ok":true}` || got.resp.Attempts != 1 {
		t.Fatalf("response=%+v", got.resp)
	}
}

func TestSubmitResponseRejectsWrongNonceNonFreshAndOverwrite(t *testing.T) {
	t.Parallel()
	req := testRequest(t)
	rt := fixedRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := rt.Run(ctx, req); done <- err }()
	var packet RequestPacket
	waitJSON(t, filepath.Join(req.RunRoot, "human-broker", "req-001", "request.json"), &packet)
	bad := Submission{RequestID: "req-001", Nonce: "wrong", FreshSession: true, RawResponse: `{"ok":true}`}
	if err := SubmitResponse(req.RunRoot, bad); err == nil {
		t.Fatal("wrong nonce accepted")
	}
	bad.Nonce = "nonce-001"
	bad.FreshSession = false
	if err := SubmitResponse(req.RunRoot, bad); err == nil {
		t.Fatal("non-fresh session accepted")
	}
	good := Submission{RequestID: "req-001", Nonce: "nonce-001", FreshSession: true, RawResponse: `{"ok":true}`}
	if err := SubmitResponse(req.RunRoot, good); err != nil {
		t.Fatal(err)
	}
	if err := SubmitResponse(req.RunRoot, good); err == nil {
		t.Fatal("response overwrite accepted")
	}
	<-done
}

func TestRuntimeCancellationAndTimeoutAreTerminal(t *testing.T) {
	t.Parallel()
	t.Run("cancel", func(t *testing.T) {
		req := testRequest(t)
		rt := fixedRuntime()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := rt.Run(ctx, req)
		assertTimeout(t, err)
	})
	t.Run("timeout", func(t *testing.T) {
		req := testRequest(t)
		rt := fixedRuntime()
		rt.WaitTimeout = 20 * time.Millisecond
		_, err := rt.Run(context.Background(), req)
		assertTimeout(t, err)
	})
}

func waitJSON(t *testing.T, path string, out any) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(data, out); err != nil {
				t.Fatal(err)
			}
			return
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}
func assertTimeout(t *testing.T, err error) {
	t.Helper()
	var run *councilruntime.RunError
	if !errors.As(err, &run) || run.Class != councilruntime.FailureTimeout {
		t.Fatalf("error=%v want timeout", err)
	}
}
