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

func TestCouncilBrokerShowPrintsPasteablePrompt(t *testing.T) {
	root := t.TempDir()
	rt := &humanbroker.Runtime{WaitTimeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond, NewRequestID: func() (string, error) { return "req-show", nil }, NewNonce: func() (string, error) { return "nonce-show", nil }}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := rt.Run(ctx, councilruntime.AgentRequest{RunID: "h5", RunRoot: root, SlotID: "reviewer-1", Participant: "reviewer-1", Role: "reviewer", Phase: "cross-review", Prompt: "REVIEW ME", OutputSchema: json.RawMessage(`{"type":"object"}`)})
		done <- err
	}()
	waitForBrokerRequest(t, filepath.Join(root, "human-broker", "req-show", "request.json"))
	var stdout, stderr bytes.Buffer
	code := runWithH5BenchmarkExecutors([]string{"council", "broker", "show", "--run-root", root, "--request-id", "req-show"}, &stdout, &stderr, nil, nil, nil, nil, nil)
	if code != 0 || !bytes.Contains(stdout.Bytes(), []byte("REVIEW ME")) || !bytes.Contains(stdout.Bytes(), []byte("TRANSPORT_OUTPUT_SCHEMA_BEGIN")) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	cancel()
	<-done
}

func TestCouncilBrokerPendingListsOnlyUnansweredRequests(t *testing.T) {
	root := t.TempDir()
	makePendingBrokerRequest(t, root, "req-pending", "nonce-pending")
	makePendingBrokerRequest(t, root, "req-done", "nonce-done")
	if err := humanbroker.SubmitResponse(root, humanbroker.Submission{RequestID: "req-done", Nonce: "nonce-done", FreshSession: true, RawResponse: `{"ok":true}`}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runWithH5BenchmarkExecutors([]string{"council", "broker", "pending", "--run-root", root}, &stdout, &stderr, nil, nil, nil, nil, nil)
	if code != 0 || !bytes.Contains(stdout.Bytes(), []byte("req-pending")) || bytes.Contains(stdout.Bytes(), []byte("req-done")) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func waitForBrokerRequest(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("broker request not created: %s", path)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func makePendingBrokerRequest(t *testing.T, root, requestID, nonce string) {
	t.Helper()
	rt := &humanbroker.Runtime{WaitTimeout: 30 * time.Millisecond, PollInterval: 5 * time.Millisecond, NewRequestID: func() (string, error) { return requestID, nil }, NewNonce: func() (string, error) { return nonce, nil }}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := rt.Run(ctx, councilruntime.AgentRequest{RunID: "h5", RunRoot: root, SlotID: "reviewer-1", Participant: "reviewer-1", Role: "reviewer", Phase: "cross-review", Prompt: "REVIEW", OutputSchema: json.RawMessage(`{"type":"object"}`)})
		done <- err
	}()
	waitForBrokerRequest(t, filepath.Join(root, "human-broker", requestID, "request.json"))
	cancel()
	<-done
}
