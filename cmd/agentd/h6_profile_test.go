package main

import (
	"context"
	"github.com/ShenJun93/agent-council/internal/council/adapterpool"
	"github.com/ShenJun93/agent-council/internal/council/benchmark"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"testing"
)

type h6NoopRuntime struct{ provider councilruntime.Provider }

func (r h6NoopRuntime) Run(context.Context, councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	return councilruntime.AgentResponse{Provider: r.provider, Stdout: "{}", ExitCode: 0, Attempts: 1}, nil
}

func TestNewH6EvaluatorSelectsTypedCitationContract(t *testing.T) {
	registry := h6Registry{
		adapterpool.AdapterID("claude-max"):    {ID: adapterpool.AdapterID("claude-max"), Provider: councilruntime.ProviderClaude, Runtime: h6NoopRuntime{councilruntime.ProviderClaude}},
		adapterpool.AdapterID("codex-chatgpt"): {ID: adapterpool.AdapterID("codex-chatgpt"), Provider: councilruntime.ProviderCodex, Runtime: h6NoopRuntime{councilruntime.ProviderCodex}},
	}
	policy := benchmark.H5AdapterPolicy{Slots: map[string][]string{"eval-judge-1": {"claude-max"}, "eval-judge-2": {"codex-chatgpt"}}}
	got, err := newH6Evaluator(registry, policy, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, ok := got.(evalharness.Harness)
	if !ok {
		t.Fatalf("type=%T", got)
	}
	if h.CitationContract != evalharness.CitationContractStructuredV1 {
		t.Fatalf("contract=%v", h.CitationContract)
	}
}
