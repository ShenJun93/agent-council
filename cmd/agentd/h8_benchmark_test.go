package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/adapterpool"
	"github.com/ShenJun93/agent-council/internal/council/benchmark"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/structuredoutput"
)

type h8NoopRuntime struct{}

func (h8NoopRuntime) Run(context.Context, councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	return councilruntime.AgentResponse{}, nil
}

func TestRunRoutesH8BenchmarkWithFrozenFlags(t *testing.T) {
	datasetPath := filepath.Join("..", "..", "benchmarks", "h8")
	calls := 0
	var got h8ExecutionRequest
	exec := func(_ context.Context, req h8ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		got = req
		return benchmark.RunResult{RunID: req.RunID, RunDir: filepath.Join(req.RunsRoot, req.RunID)}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH8BenchmarkExecutors([]string{"council", "benchmark", "h8", "--dataset", datasetPath}, &stdout, &stderr, nil, nil, nil, nil, nil, nil, nil, exec)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if got.DatasetPath != datasetPath || got.RunsRoot != ".council/runs" {
		t.Fatalf("request=%+v", got)
	}
	if len(got.RunID) < 4 || got.RunID[:3] != "h8-" {
		t.Fatalf("run id=%q", got.RunID)
	}
}

func TestNewH8RunIDIsFreshAtSameTimestamp(t *testing.T) {
	now := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	first, err := newH8RunID(now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newH8RunID(now)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("run ids collided: %q", first)
	}
	if first[:3] != "h8-" || second[:3] != "h8-" {
		t.Fatalf("run ids=%q %q", first, second)
	}
}

func TestH8ExecutionUsesH8SchemaProfileAndV3Evaluator(t *testing.T) {
	wrapped := wrapH8Adapter(h8NoopRuntime{}, "codex-chatgpt", councilruntime.ProviderCodex)
	profiled, ok := wrapped.(*structuredoutput.Runtime)
	if !ok {
		t.Fatalf("wrapped runtime type=%T", wrapped)
	}
	if profiled.Profile != structuredoutput.SchemaProfileH8 {
		t.Fatalf("profile=%v want H8", profiled.Profile)
	}
	id := adapterpool.AdapterID("codex-chatgpt")
	registry := h8Registry{id: {ID: id, Provider: councilruntime.ProviderCodex, Runtime: wrapped}}
	policy := benchmark.H5AdapterPolicy{Slots: map[string][]string{"eval-judge-1": {string(id)}, "eval-judge-2": {string(id)}}}
	evaluator, err := newH8Evaluator(registry, policy, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, ok := evaluator.(evalharness.Harness)
	if !ok {
		t.Fatalf("evaluator type=%T", evaluator)
	}
	if h.CitationContract != evalharness.CitationContractStructuredV3 {
		t.Fatalf("citation contract=%v want V3", h.CitationContract)
	}
}

func TestH8DoesNotChangeH7EvaluatorContractOrProfile(t *testing.T) {
	wrapped := wrapH7Adapter(h8NoopRuntime{}, "codex-chatgpt", councilruntime.ProviderCodex)
	profiled, ok := wrapped.(*structuredoutput.Runtime)
	if !ok {
		t.Fatalf("H7 wrapped runtime type=%T", wrapped)
	}
	if profiled.Profile != structuredoutput.SchemaProfileH7 {
		t.Fatalf("H7 profile=%v want H7", profiled.Profile)
	}
	id := adapterpool.AdapterID("codex-chatgpt")
	registry := h7Registry{id: {ID: id, Provider: councilruntime.ProviderCodex, Runtime: wrapped}}
	policy := benchmark.H5AdapterPolicy{Slots: map[string][]string{"eval-judge-1": {string(id)}, "eval-judge-2": {string(id)}}}
	evaluator, err := newH7Evaluator(registry, policy, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, ok := evaluator.(evalharness.Harness)
	if !ok {
		t.Fatalf("H7 evaluator type=%T", evaluator)
	}
	if h.CitationContract != evalharness.CitationContractStructuredV2 {
		t.Fatalf("H7 citation contract=%v want V2", h.CitationContract)
	}
}
