package main

import (
	"context"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/adapterpool"
	"github.com/ShenJun93/agent-council/internal/council/benchmark"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/structuredoutput"
)

type h7NoopRuntime struct{ provider councilruntime.Provider }

func (r h7NoopRuntime) Run(context.Context, councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	return councilruntime.AgentResponse{Provider: r.provider, Stdout: "{}", ExitCode: 0, Attempts: 1}, nil
}

func TestNewH7EvaluatorSelectsClaimAwareCitationContract(t *testing.T) {
	registry := h7Registry{
		adapterpool.AdapterID("claude-max"):    {ID: adapterpool.AdapterID("claude-max"), Provider: councilruntime.ProviderClaude, Runtime: h7NoopRuntime{councilruntime.ProviderClaude}},
		adapterpool.AdapterID("codex-chatgpt"): {ID: adapterpool.AdapterID("codex-chatgpt"), Provider: councilruntime.ProviderCodex, Runtime: h7NoopRuntime{councilruntime.ProviderCodex}},
	}
	policy := benchmark.H5AdapterPolicy{Slots: map[string][]string{"eval-judge-1": {"claude-max"}, "eval-judge-2": {"codex-chatgpt"}}}
	got, err := newH7Evaluator(registry, policy, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, ok := got.(evalharness.Harness)
	if !ok {
		t.Fatalf("type=%T", got)
	}
	if h.CitationContract != evalharness.CitationContractStructuredV2 {
		t.Fatalf("contract=%v", h.CitationContract)
	}
}

func TestWrapH7AdapterSelectsH7SchemaProfile(t *testing.T) {
	wrapped := wrapH7Adapter(h7NoopRuntime{councilruntime.ProviderCodex}, "codex-chatgpt", councilruntime.ProviderCodex)
	rt, ok := wrapped.(*structuredoutput.Runtime)
	if !ok {
		t.Fatalf("wrapper type=%T want *structuredoutput.Runtime", wrapped)
	}
	if rt.Profile != structuredoutput.SchemaProfileH7 {
		t.Fatalf("profile=%v want H7", rt.Profile)
	}
}
