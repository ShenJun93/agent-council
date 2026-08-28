package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/humanbroker"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func TestCouncilBrokerSubmitReadsNonceAndResumesHumanRuntime(t *testing.T) {
	root := t.TempDir()
	rt := &humanbroker.Runtime{WaitTimeout: time.Second, PollInterval: 5 * time.Millisecond, NewRequestID: func() (string, error) { return "req-cli", nil }, NewNonce: func() (string, error) { return "nonce-cli", nil }}
	req := councilruntime.AgentRequest{RunID: "h5-test", RunRoot: root, SlotID: "reviewer-1", AdapterID: humanbroker.DefaultAdapterID, Participant: "reviewer-1", Role: "reviewer", Phase: "cross-review", Prompt: "REVIEW", OutputSchema: json.RawMessage(`{"type":"object"}`)}
	done := make(chan error, 1)
	go func() { _, err := rt.Run(context.Background(), req); done <- err }()
	requestPath := filepath.Join(root, "human-broker", "req-cli", "request.json")
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(requestPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("request not created")
		}
		time.Sleep(5 * time.Millisecond)
	}
	responseFile := filepath.Join(t.TempDir(), "response.txt")
	if err := os.WriteFile(responseFile, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runWithH5BenchmarkExecutors([]string{"council", "broker", "submit", "--run-root", root, "--request-id", "req-cli", "--response-file", responseFile, "--fresh-session", "--model-label", "ChatGPT"}, &stdout, &stderr, nil, nil, nil, nil, nil)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("human runtime did not resume")
	}
	var out map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["request_id"] != "req-cli" || out["status"] != "submitted" {
		t.Fatalf("output=%v", out)
	}
}
