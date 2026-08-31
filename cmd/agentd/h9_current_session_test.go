package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/adapterpool"
	"github.com/ShenJun93/agent-council/internal/council/benchmark"
	"github.com/ShenJun93/agent-council/internal/council/humanbroker"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func TestH9BrokerUsesCurrentOrchestratorSession(t *testing.T) {
	root := t.TempDir()
	policy := benchmark.H5AdapterPolicy{
		Adapters: []benchmark.H5AdapterDescriptor{{ID: humanbroker.DefaultAdapterID, ProviderFamily: "chatgpt", Transport: "human-chatgpt-session", AuthClass: "chatgpt-subscription", Interaction: "human-broker"}},
		Slots: map[string][]string{"eval-judge-1": {humanbroker.DefaultAdapterID}, "eval-judge-2": {humanbroker.DefaultAdapterID}},
	}
	registry, err := newH9Registry(h9ExecutionRequest{Dataset: benchmark.Dataset{AdapterPolicy: &policy}})
	if err != nil {
		t.Fatal(err)
	}
	adapter, ok := registry[adapterpool.AdapterID(humanbroker.DefaultAdapterID)]
	if !ok {
		t.Fatal("H9 human broker adapter missing")
	}
	done := make(chan error, 1)
	go func() {
		_, err := adapter.Runtime.Run(context.Background(), councilruntime.AgentRequest{
			RunID: "h9-test", RunRoot: root, SlotID: "eval-judge-1", AdapterID: humanbroker.DefaultAdapterID,
			Participant: "judge-1", Role: "eval-judge", Phase: "eval", Prompt: "JUDGE",
			OutputSchema: json.RawMessage(`{"type":"object"}`),
		})
		done <- err
	}()

	requestPath := waitForH9BrokerRequest(t, root)
	data, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet humanbroker.RequestPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.RequireFreshSession {
		t.Fatal("H9 must not require a fresh ChatGPT session")
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["require_current_session"] != true {
		t.Fatalf("request=%s", data)
	}
	joined := strings.ToLower(strings.Join(packet.Instructions, " "))
	if !strings.Contains(joined, "current") || strings.Contains(joined, "open a new chat") {
		t.Fatalf("instructions=%v", packet.Instructions)
	}

	responseFile := filepath.Join(t.TempDir(), "response.txt")
	if err := os.WriteFile(responseFile, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runWithH9BenchmarkExecutors(
		[]string{"council", "broker", "submit", "--run-root", root, "--request-id", packet.RequestID, "--response-file", responseFile, "--current-session", "--model-label", "GPT-5.6 Sol"},
		&stdout, &stderr, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("H9 current-session runtime did not resume")
	}
	responseData, err := os.ReadFile(filepath.Join(root, "human-broker", packet.RequestID, "response.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(responseData, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["current_session"] != true || raw["fresh_session"] != false {
		t.Fatalf("response=%s", responseData)
	}
}

func waitForH9BrokerRequest(t *testing.T, root string) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		matches, _ := filepath.Glob(filepath.Join(root, "human-broker", "*", "request.json"))
		if len(matches) == 1 {
			return matches[0]
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("H9 broker request not created")
	return ""
}
