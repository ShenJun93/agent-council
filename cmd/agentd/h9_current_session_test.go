package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/adapterpool"
	"github.com/ShenJun93/agent-council/internal/council/benchmark"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/humanbroker"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func TestH9BrokerUsesCurrentOrchestratorSession(t *testing.T) {
	root := t.TempDir()
	policy := benchmark.H5AdapterPolicy{
		Adapters: []benchmark.H5AdapterDescriptor{{ID: humanbroker.DefaultAdapterID, ProviderFamily: "chatgpt", Transport: "human-chatgpt-session", AuthClass: "chatgpt-subscription", Interaction: "human-broker"}},
		Slots:    map[string][]string{"eval-judge-1": {humanbroker.DefaultAdapterID}, "eval-judge-2": {humanbroker.DefaultAdapterID}},
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
			Participant: "judge-1", Role: "judge", Phase: evalharness.PhaseEvalJudge, Prompt: "JUDGE",
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
	if !strings.Contains(joined, "current chatgpt web orchestration session") || strings.Contains(joined, "open a new chat in chatgpt with no prior context") {
		t.Fatalf("instructions=%v", packet.Instructions)
	}

	record := map[string]any{
		"schema_version":  humanbroker.ResponseSchemaVersion,
		"request_id":      packet.RequestID,
		"nonce":           packet.Nonce,
		"fresh_session":   false,
		"current_session": true,
		"model_label":     "GPT-5.6 Sol",
		"raw_response":    `{"ok":true}`,
		"submitted_at":    time.Now().UTC(),
	}
	responseData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	responsePath := filepath.Join(root, "human-broker", packet.RequestID, "response.json")
	if err := os.WriteFile(responsePath, responseData, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("H9 current-session runtime did not resume")
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
