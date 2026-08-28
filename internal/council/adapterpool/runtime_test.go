package adapterpool

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type fakeStep struct {
	response councilruntime.AgentResponse
	err      error
}

type fakeRuntime struct {
	steps    []fakeStep
	requests []councilruntime.AgentRequest
}

func (f *fakeRuntime) Run(_ context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.steps) == 0 {
		return councilruntime.AgentResponse{}, errors.New("unexpected call")
	}
	step := f.steps[0]
	f.steps = f.steps[1:]
	return step.response, step.err
}

func runErr(class councilruntime.FailureClass) error {
	return &councilruntime.RunError{Class: class, Err: errors.New(string(class))}
}

func testRegistry(primary, secondary councilruntime.AgentRuntime) map[AdapterID]Adapter {
	return map[AdapterID]Adapter{
		"claude-max": {
			ID: "claude-max", Provider: councilruntime.ProviderClaude, Runtime: primary,
		},
		"codex-chatgpt": {
			ID: "codex-chatgpt", Provider: councilruntime.ProviderCodex, Runtime: secondary,
		},
	}
}

func response(provider councilruntime.Provider) councilruntime.AgentResponse {
	return councilruntime.AgentResponse{
		Provider: provider, Stdout: `{"ok":true}`, ExitCode: 0, Attempts: 1,
		StartedAt: time.Unix(10, 0).UTC(), FinishedAt: time.Unix(11, 0).UTC(),
	}
}

func TestPoolFailsOverAvailabilityFailureToSecondary(t *testing.T) {
	for _, class := range []councilruntime.FailureClass{
		councilruntime.FailureQuotaExhausted,
		councilruntime.FailureAuth,
		councilruntime.FailureAdapterUnavailable,
	} {
		t.Run(string(class), func(t *testing.T) {
			primary := &fakeRuntime{steps: []fakeStep{{response: response(councilruntime.ProviderClaude), err: runErr(class)}}}
			secondary := &fakeRuntime{steps: []fakeStep{{response: response(councilruntime.ProviderCodex)}}}
			rt, err := New(testRegistry(primary, secondary), Policy{
				Slot: "reviewer-1", Chain: []AdapterID{"claude-max", "codex-chatgpt"},
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := rt.Run(context.Background(), councilruntime.AgentRequest{Participant: "reviewer-1"})
			if err != nil {
				t.Fatalf("fallback failed: %v", err)
			}
			if got.Provider != councilruntime.ProviderCodex || got.AdapterID != "codex-chatgpt" {
				t.Fatalf("got provider=%q adapter=%q", got.Provider, got.AdapterID)
			}
			if got.SlotID != "reviewer-1" || got.FailoverIndex != 1 || got.FailoverTrigger != class {
				t.Fatalf("metadata=%+v want slot reviewer-1 index 1 trigger %s", got, class)
			}
			if len(primary.requests) != 1 || len(secondary.requests) != 1 {
				t.Fatalf("calls primary=%d secondary=%d", len(primary.requests), len(secondary.requests))
			}
			for i, req := range []councilruntime.AgentRequest{primary.requests[0], secondary.requests[0]} {
				if req.MaxAttempts != 1 || req.SlotID != "reviewer-1" || req.FailoverIndex != i {
					t.Fatalf("request %d metadata=%+v", i, req)
				}
			}
			if secondary.requests[0].FailoverTrigger != class {
				t.Fatalf("secondary trigger=%q want %q", secondary.requests[0].FailoverTrigger, class)
			}
		})
	}
}

func TestPoolDoesNotFailOverTerminalFailures(t *testing.T) {
	classes := []councilruntime.FailureClass{
		councilruntime.FailureMalformedOutput, councilruntime.FailureProcess,
		councilruntime.FailureIsolation, councilruntime.FailureBillingPolicyViolation,
		councilruntime.FailureTimeout,
	}
	for _, class := range classes {
		t.Run(string(class), func(t *testing.T) {
			primary := &fakeRuntime{steps: []fakeStep{{response: response(councilruntime.ProviderClaude), err: runErr(class)}}}
			secondary := &fakeRuntime{steps: []fakeStep{{response: response(councilruntime.ProviderCodex)}}}
			rt, err := New(testRegistry(primary, secondary), Policy{
				Slot: "reviewer-1", Chain: []AdapterID{"claude-max", "codex-chatgpt"},
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = rt.Run(context.Background(), councilruntime.AgentRequest{Participant: "reviewer-1"})
			if err == nil || !strings.Contains(err.Error(), string(class)) {
				t.Fatalf("error=%v want class %s", err, class)
			}
			if len(secondary.requests) != 0 {
				t.Fatalf("secondary calls=%d want 0", len(secondary.requests))
			}
		})
	}
}

func TestPoolJoinedErrorsRequireAllClassesAvailable(t *testing.T) {
	t.Run("all availability falls through", func(t *testing.T) {
		primary := &fakeRuntime{steps: []fakeStep{{err: errors.Join(
			runErr(councilruntime.FailureQuotaExhausted),
			runErr(councilruntime.FailureAuth),
		)}}}
		secondary := &fakeRuntime{steps: []fakeStep{{response: response(councilruntime.ProviderCodex)}}}
		rt, err := New(testRegistry(primary, secondary), Policy{Slot: "judge-1", Chain: []AdapterID{"claude-max", "codex-chatgpt"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rt.Run(context.Background(), councilruntime.AgentRequest{}); err != nil {
			t.Fatal(err)
		}
		if len(secondary.requests) != 1 {
			t.Fatalf("secondary calls=%d want 1", len(secondary.requests))
		}
	})

	t.Run("mixed availability and isolation stops", func(t *testing.T) {
		primary := &fakeRuntime{steps: []fakeStep{{err: errors.Join(
			runErr(councilruntime.FailureQuotaExhausted),
			runErr(councilruntime.FailureIsolation),
		)}}}
		secondary := &fakeRuntime{steps: []fakeStep{{response: response(councilruntime.ProviderCodex)}}}
		rt, err := New(testRegistry(primary, secondary), Policy{Slot: "judge-1", Chain: []AdapterID{"claude-max", "codex-chatgpt"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rt.Run(context.Background(), councilruntime.AgentRequest{}); err == nil {
			t.Fatal("mixed error unexpectedly succeeded")
		}
		if len(secondary.requests) != 0 {
			t.Fatalf("secondary calls=%d want 0", len(secondary.requests))
		}
	})
}

func TestPoolExhaustionPreservesOrderedAvailabilityFailures(t *testing.T) {
	primary := &fakeRuntime{steps: []fakeStep{{err: runErr(councilruntime.FailureQuotaExhausted)}}}
	secondary := &fakeRuntime{steps: []fakeStep{{err: runErr(councilruntime.FailureAuth)}}}
	rt, err := New(testRegistry(primary, secondary), Policy{
		Slot: "judge-1", Chain: []AdapterID{"claude-max", "codex-chatgpt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.Run(context.Background(), councilruntime.AgentRequest{})
	var run *councilruntime.RunError
	if !errors.As(err, &run) || run.Class != councilruntime.FailureAdapterPoolExhausted {
		t.Fatalf("error=%v want adapter pool exhausted", err)
	}
	text := err.Error()
	first := strings.Index(text, "claude-max")
	second := strings.Index(text, "codex-chatgpt")
	if first < 0 || second < 0 || first >= second {
		t.Fatalf("unordered exhaustion error: %s", text)
	}
	if !strings.Contains(text, "quota_exhausted") || !strings.Contains(text, "auth_failure") {
		t.Fatalf("missing causes: %s", text)
	}
}

func TestPoolRejectsProviderMismatchWithoutFallback(t *testing.T) {
	primary := &fakeRuntime{steps: []fakeStep{{response: response(councilruntime.ProviderCodex)}}}
	secondary := &fakeRuntime{steps: []fakeStep{{response: response(councilruntime.ProviderCodex)}}}
	rt, err := New(testRegistry(primary, secondary), Policy{
		Slot: "researcher-1", Chain: []AdapterID{"claude-max", "codex-chatgpt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.Run(context.Background(), councilruntime.AgentRequest{})
	var run *councilruntime.RunError
	if !errors.As(err, &run) || run.Class != councilruntime.FailureIsolation {
		t.Fatalf("error=%v want isolation", err)
	}
	if len(secondary.requests) != 0 {
		t.Fatalf("secondary calls=%d want 0", len(secondary.requests))
	}
}

func TestNewRejectsInvalidPolicy(t *testing.T) {
	good := &fakeRuntime{}
	registry := map[AdapterID]Adapter{"claude-max": {ID: "claude-max", Provider: councilruntime.ProviderClaude, Runtime: good}}
	tests := []Policy{
		{Slot: "../reviewer", Chain: []AdapterID{"claude-max"}},
		{Slot: "reviewer-1"},
		{Slot: "reviewer-1", Chain: []AdapterID{"missing"}},
		{Slot: "reviewer-1", Chain: []AdapterID{"claude-max", "claude-max"}},
	}
	for _, policy := range tests {
		if _, err := New(registry, policy); err == nil {
			t.Fatalf("policy %+v accepted", policy)
		}
	}
}

func TestPoolFallsThroughTwoUnavailableAdaptersToHumanChatGPT(t *testing.T) {
	claude := &fakeRuntime{steps: []fakeStep{{err: runErr(councilruntime.FailureQuotaExhausted)}}}
	codex := &fakeRuntime{steps: []fakeStep{{err: runErr(councilruntime.FailureAuth)}}}
	human := &fakeRuntime{steps: []fakeStep{{response: response(councilruntime.ProviderChatGPT)}}}
	registry := testRegistry(claude, codex)
	registry["human-chatgpt-session"] = Adapter{ID: "human-chatgpt-session", Provider: councilruntime.ProviderChatGPT, Runtime: human}
	rt, err := New(registry, Policy{Slot: "reviewer-1", Chain: []AdapterID{"claude-max", "codex-chatgpt", "human-chatgpt-session"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := rt.Run(context.Background(), councilruntime.AgentRequest{Participant: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != councilruntime.ProviderChatGPT || got.AdapterID != "human-chatgpt-session" || got.FailoverIndex != 2 || got.FailoverTrigger != councilruntime.FailureAuth {
		t.Fatalf("got=%+v", got)
	}
	if len(claude.requests) != 1 || len(codex.requests) != 1 || len(human.requests) != 1 {
		t.Fatalf("calls=%d/%d/%d", len(claude.requests), len(codex.requests), len(human.requests))
	}
}
