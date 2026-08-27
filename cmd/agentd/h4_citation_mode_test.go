package main

import (
	"context"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/invocationlog"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/structuredoutput"
)

func TestNewH4BaselineEnablesProblemOnlyFinalCitationAuthority(t *testing.T) {
	executor := newH4Baseline(nil, nil, t.TempDir(), councilruntime.ProviderClaude)
	runner, ok := executor.(baseline.Runner)
	if !ok {
		t.Fatalf("H4 baseline type=%T want baseline.Runner", executor)
	}
	if runner.CitationAuthority != baseline.CitationAuthorityProblemOnlyFinal {
		t.Fatalf("citation authority=%d want problem-only-final", runner.CitationAuthority)
	}
}

type h4NoopRuntime struct{}

func (h4NoopRuntime) Run(context.Context, councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	return councilruntime.AgentResponse{}, nil
}

func TestWrapH4RuntimeOrdersSchemaOutsideInvocationLog(t *testing.T) {
	inner := h4NoopRuntime{}
	wrapped := wrapH4Runtime(inner, councilruntime.ProviderClaude)
	schemaRuntime, ok := wrapped.(*structuredoutput.Runtime)
	if !ok {
		t.Fatalf("outer runtime type=%T want *structuredoutput.Runtime", wrapped)
	}
	logged, ok := schemaRuntime.Inner.(*invocationlog.Runtime)
	if !ok {
		t.Fatalf("inner runtime type=%T want *invocationlog.Runtime", schemaRuntime.Inner)
	}
	if logged.Provider != councilruntime.ProviderClaude {
		t.Fatalf("logged provider=%q", logged.Provider)
	}
	if _, ok := logged.Inner.(h4NoopRuntime); !ok {
		t.Fatalf("leaf runtime type=%T want h4NoopRuntime", logged.Inner)
	}
}
