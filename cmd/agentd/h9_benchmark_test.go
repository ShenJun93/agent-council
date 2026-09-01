package main

import (
	"bytes"
	"context"
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
	"github.com/ShenJun93/agent-council/internal/council/structuredoutput"
)

type h9NoopRuntime struct{}

func (h9NoopRuntime) Run(context.Context, councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	return councilruntime.AgentResponse{}, nil
}

func TestRunRoutesH9BenchmarkWithFrozenFlags(t *testing.T) {
	datasetPath := filepath.Join("..", "..", "benchmarks", "h9")
	calls := 0
	var got h9ExecutionRequest
	exec := func(_ context.Context, req h9ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		got = req
		return benchmark.RunResult{RunID: req.RunID, RunDir: filepath.Join(req.RunsRoot, req.RunID)}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH9BenchmarkExecutors([]string{"council", "benchmark", "h9", "--dataset", datasetPath}, &stdout, &stderr, nil, nil, nil, nil, nil, nil, nil, nil, exec)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if got.DatasetPath != datasetPath || got.RunsRoot != ".council/runs" {
		t.Fatalf("request=%+v", got)
	}
	if len(got.RunID) < 4 || got.RunID[:3] != "h9-" {
		t.Fatalf("run id=%q", got.RunID)
	}
}

func TestNewH9RunIDIsFreshAtSameTimestamp(t *testing.T) {
	now := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	first, err := newH9RunID(now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newH9RunID(now)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("run ids collided: %q", first)
	}
	if first[:3] != "h9-" || second[:3] != "h9-" {
		t.Fatalf("run ids=%q %q", first, second)
	}
}

func TestNewH9RegistryContainsOnlyHumanChatGPT(t *testing.T) {
	policy := h9PolicyForTest()
	registry, err := newH9Registry(h9ExecutionRequest{Dataset: benchmark.Dataset{AdapterPolicy: &policy}})
	if err != nil {
		t.Fatal(err)
	}
	if len(registry) != 1 {
		t.Fatalf("registry size=%d", len(registry))
	}
	id := adapterpool.AdapterID(humanbroker.DefaultAdapterID)
	adapter, ok := registry[id]
	if !ok {
		t.Fatalf("missing %s", id)
	}
	if adapter.Provider != councilruntime.ProviderChatGPT {
		t.Fatalf("provider=%s", adapter.Provider)
	}
	profiled, ok := adapter.Runtime.(*structuredoutput.Runtime)
	if !ok {
		t.Fatalf("runtime type=%T", adapter.Runtime)
	}
	if profiled.Profile != structuredoutput.SchemaProfileH8 {
		t.Fatalf("profile=%v want H8", profiled.Profile)
	}
}

func TestH9RegistryUsesCurrentOrchestratorSessionBroker(t *testing.T) {
	policy := h9PolicyForTest()
	registry, err := newH9Registry(h9ExecutionRequest{Dataset: benchmark.Dataset{AdapterPolicy: &policy}})
	if err != nil {
		t.Fatal(err)
	}
	id := adapterpool.AdapterID(humanbroker.DefaultAdapterID)
	adapter := registry[id]
	runRoot := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, _ = adapter.Runtime.Run(ctx, councilruntime.AgentRequest{
		RunID:       "h9-current-session-test",
		RunRoot:     runRoot,
		SlotID:      "eval-judge-1",
		AdapterID:   string(id),
		Participant: "judge-1",
		Role:        "judge",
		Phase:       "eval",
		Prompt:      "test prompt",
	})
	entries, err := os.ReadDir(filepath.Join(runRoot, "human-broker"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("broker requests=%d", len(entries))
	}
	packet, err := humanbroker.LoadRequest(runRoot, entries[0].Name())
	if err != nil {
		t.Fatal(err)
	}
	if packet.RequireFreshSession {
		t.Fatal("H9 broker must not require a fresh ChatGPT session")
	}
	if !packet.RequireCurrentSession {
		t.Fatal("H9 broker must require the current orchestrating ChatGPT session")
	}
	if !strings.Contains(strings.Join(packet.Instructions, "\n"), "current orchestrating ChatGPT conversation") {
		t.Fatalf("instructions=%q", packet.Instructions)
	}
	if err := humanbroker.SubmitResponse(runRoot, humanbroker.Submission{
		RequestID:      packet.RequestID,
		Nonce:          packet.Nonce,
		CurrentSession: true,
		RawResponse:    `{}`,
	}); err != nil {
		t.Fatalf("submit current-session response: %v", err)
	}
}

func TestH9EvaluatorReusesH8V3Contract(t *testing.T) {
	wrapped := wrapH9Adapter(h9NoopRuntime{}, humanbroker.DefaultAdapterID, councilruntime.ProviderChatGPT)
	id := adapterpool.AdapterID(humanbroker.DefaultAdapterID)
	registry := h9Registry{id: {ID: id, Provider: councilruntime.ProviderChatGPT, Runtime: wrapped}}
	policy := benchmark.H5AdapterPolicy{Slots: map[string][]string{"eval-judge-1": {string(id)}, "eval-judge-2": {string(id)}}}
	evaluator, err := newH9Evaluator(registry, policy, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, ok := evaluator.(evalharness.H8Harness)
	if !ok {
		t.Fatalf("evaluator type=%T, want evalharness.H8Harness", evaluator)
	}
	if h.CitationContract != evalharness.CitationContractStructuredV3 {
		t.Fatalf("citation contract=%v want V3", h.CitationContract)
	}
}

func TestH9CommandDoesNotAcceptLegacyProviderBinaryFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCouncilBenchmarkH9([]string{"--claude-bin", "claude"}, &stdout, &stderr, func(context.Context, h9ExecutionRequest) (benchmark.RunResult, error) {
		return benchmark.RunResult{}, nil
	})
	if code != 2 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
}

func TestH9DoesNotChangeH8EvaluatorContractOrProfile(t *testing.T) {
	wrapped := wrapH8Adapter(h9NoopRuntime{}, "codex-chatgpt", councilruntime.ProviderCodex)
	profiled, ok := wrapped.(*structuredoutput.Runtime)
	if !ok {
		t.Fatalf("H8 wrapped runtime type=%T", wrapped)
	}
	if profiled.Profile != structuredoutput.SchemaProfileH8 {
		t.Fatalf("H8 profile=%v", profiled.Profile)
	}
	id := adapterpool.AdapterID("codex-chatgpt")
	registry := h8Registry{id: {ID: id, Provider: councilruntime.ProviderCodex, Runtime: wrapped}}
	policy := benchmark.H5AdapterPolicy{Slots: map[string][]string{"eval-judge-1": {string(id)}, "eval-judge-2": {string(id)}}}
	evaluator, err := newH8Evaluator(registry, policy, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, ok := evaluator.(evalharness.H8Harness)
	if !ok {
		t.Fatalf("H8 evaluator type=%T", evaluator)
	}
	if h.CitationContract != evalharness.CitationContractStructuredV3 {
		t.Fatalf("H8 citation contract=%v", h.CitationContract)
	}
}

func h9PolicyForTest() benchmark.H5AdapterPolicy {
	return benchmark.H5AdapterPolicy{
		SchemaVersion: benchmark.H9AdapterPolicySchemaVersion,
		Adapters:      []benchmark.H5AdapterDescriptor{{ID: humanbroker.DefaultAdapterID, ProviderFamily: "chatgpt", Transport: "human-chatgpt-session", AuthClass: "chatgpt-subscription", Interaction: "human-broker"}},
	}
}
