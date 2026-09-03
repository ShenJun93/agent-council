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

type phaseHNoopRuntime struct{}

func (phaseHNoopRuntime) Run(context.Context, councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	return councilruntime.AgentResponse{}, nil
}

func TestRunRoutesPhaseHBenchmarkWithFrozenFlags(t *testing.T) {
	datasetPath := filepath.Join("..", "..", "benchmarks", "phase-h")
	calls := 0
	var got phaseHExecutionRequest
	exec := func(_ context.Context, req phaseHExecutionRequest) (benchmark.PhaseHRunResult, error) {
		calls++
		got = req
		return benchmark.PhaseHRunResult{RunID: req.RunID, RunDir: filepath.Join(req.RunsRoot, req.RunID)}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithPhaseHBenchmarkExecutor([]string{"council", "benchmark", "phase-h", "--dataset", datasetPath}, &stdout, &stderr, exec)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if got.DatasetPath != datasetPath || got.RunsRoot != ".council/runs" {
		t.Fatalf("request=%+v", got)
	}
	if !strings.HasPrefix(got.RunID, "phase-h-") {
		t.Fatalf("run id=%q", got.RunID)
	}
}

func TestNewPhaseHRunIDIsFreshAtSameTimestamp(t *testing.T) {
	now := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	first, err := newPhaseHRunID(now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newPhaseHRunID(now)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "phase-h-") || !strings.HasPrefix(second, "phase-h-") {
		t.Fatalf("run ids=%q %q", first, second)
	}
}
func TestNewPhaseHRegistryContainsOnlyHumanChatGPT(t *testing.T) {
	policy := phaseHPolicyForTest()
	registry, err := newPhaseHRegistry(phaseHExecutionRequest{Dataset: benchmark.Dataset{AdapterPolicy: &policy}})
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

func TestPhaseHRegistryUsesCurrentOrchestratorSessionBroker(t *testing.T) {
	policy := phaseHPolicyForTest()
	registry, err := newPhaseHRegistry(phaseHExecutionRequest{Dataset: benchmark.Dataset{AdapterPolicy: &policy}})
	if err != nil {
		t.Fatal(err)
	}
	id := adapterpool.AdapterID(humanbroker.DefaultAdapterID)
	profiled, ok := registry[id].Runtime.(*structuredoutput.Runtime)
	if !ok {
		t.Fatalf("runtime type=%T", registry[id].Runtime)
	}
	runRoot := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, _ = profiled.Inner.Run(ctx, councilruntime.AgentRequest{
		RunID: "phase-h-current-session-test", RunRoot: runRoot,
		SlotID: "eval-judge-1", AdapterID: string(id), Participant: "judge-1",
		Role: "judge", Phase: "eval", Prompt: "test prompt",
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
	if packet.RequireFreshSession || !packet.RequireCurrentSession {
		t.Fatalf("session flags fresh=%v current=%v", packet.RequireFreshSession, packet.RequireCurrentSession)
	}
	if !strings.Contains(strings.Join(packet.Instructions, "\n"), "current orchestrating ChatGPT conversation") {
		t.Fatalf("instructions=%q", packet.Instructions)
	}
}

func TestPhaseHEvaluatorReusesH8V3Contract(t *testing.T) {
	wrapped := wrapPhaseHAdapter(phaseHNoopRuntime{}, humanbroker.DefaultAdapterID, councilruntime.ProviderChatGPT)
	id := adapterpool.AdapterID(humanbroker.DefaultAdapterID)
	registry := phaseHRegistry{id: {ID: id, Provider: councilruntime.ProviderChatGPT, Runtime: wrapped}}
	policy := benchmark.H5AdapterPolicy{Slots: map[string][]string{"eval-judge-1": {string(id)}, "eval-judge-2": {string(id)}}}
	evaluator, err := newPhaseHEvaluator(registry, policy, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, ok := evaluator.(evalharness.H8Harness)
	if !ok {
		t.Fatalf("evaluator type=%T", evaluator)
	}
	if h.CitationContract != evalharness.CitationContractStructuredV3 {
		t.Fatalf("citation contract=%v want V3", h.CitationContract)
	}
}
func TestPhaseHCommandDoesNotAcceptLegacyProviderBinaryFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCouncilBenchmarkPhaseH([]string{"--claude-bin", "claude"}, &stdout, &stderr,
		func(context.Context, phaseHExecutionRequest) (benchmark.PhaseHRunResult, error) {
			return benchmark.PhaseHRunResult{}, nil
		})
	if code != 2 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
}

func phaseHPolicyForTest() benchmark.H5AdapterPolicy {
	return benchmark.H5AdapterPolicy{
		SchemaVersion: benchmark.PhaseHAdapterPolicySchemaVersion,
		Adapters: []benchmark.H5AdapterDescriptor{{
			ID: humanbroker.DefaultAdapterID, ProviderFamily: "chatgpt",
			Transport: "human-chatgpt-session", AuthClass: "chatgpt-subscription",
			Interaction: "human-broker",
		}},
		Slots: map[string][]string{
			"eval-judge-1": {humanbroker.DefaultAdapterID},
			"eval-judge-2": {humanbroker.DefaultAdapterID},
		},
	}
}
