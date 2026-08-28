package adapterpool

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

var safeID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type Runtime struct {
	registry map[AdapterID]Adapter
	policy   Policy
}

func New(registry map[AdapterID]Adapter, policy Policy) (*Runtime, error) {
	if !safeID.MatchString(string(policy.Slot)) {
		return nil, fmt.Errorf("invalid slot id %q", policy.Slot)
	}
	if len(policy.Chain) == 0 {
		return nil, errors.New("adapter chain is required")
	}
	copied := make(map[AdapterID]Adapter, len(registry))
	for id, adapter := range registry {
		if !safeID.MatchString(string(id)) || id != adapter.ID {
			return nil, fmt.Errorf("invalid adapter registry entry %q", id)
		}
		if adapter.Provider == "" || adapter.Runtime == nil {
			return nil, fmt.Errorf("adapter %q requires provider and runtime", id)
		}
		copied[id] = adapter
	}
	seen := make(map[AdapterID]struct{}, len(policy.Chain))
	chain := make([]AdapterID, len(policy.Chain))
	copy(chain, policy.Chain)
	for _, id := range chain {
		if !safeID.MatchString(string(id)) {
			return nil, fmt.Errorf("invalid adapter id %q", id)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("duplicate adapter %q", id)
		}
		if _, ok := copied[id]; !ok {
			return nil, fmt.Errorf("unknown adapter %q", id)
		}
		seen[id] = struct{}{}
	}
	return &Runtime{registry: copied, policy: Policy{Slot: policy.Slot, Chain: chain}}, nil
}

func (r *Runtime) Run(ctx context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	if r == nil {
		return councilruntime.AgentResponse{}, &councilruntime.RunError{
			Class: councilruntime.FailureIsolation, Err: errors.New("adapter pool is required"),
		}
	}

	var failures []attemptFailure
	var trigger councilruntime.FailureClass
	var last councilruntime.AgentResponse
	for index, id := range r.policy.Chain {
		adapter := r.registry[id]
		attemptReq := req
		attemptReq.MaxAttempts = 1
		attemptReq.SlotID = string(r.policy.Slot)
		attemptReq.AdapterID = string(id)
		attemptReq.FailoverIndex = index
		attemptReq.FailoverTrigger = trigger

		response, err := adapter.Runtime.Run(ctx, attemptReq)
		response.AdapterID = string(id)
		response.SlotID = string(r.policy.Slot)
		response.FailoverIndex = index
		response.FailoverTrigger = trigger
		last = response
		if err == nil {
			if response.Provider != adapter.Provider {
				return response, &councilruntime.RunError{
					Class: councilruntime.FailureIsolation,
					Err:   fmt.Errorf("adapter %q provider mismatch: got %q want %q", id, response.Provider, adapter.Provider),
				}
			}
			return response, nil
		}

		classes := failureClasses(err)
		if !allAvailability(classes) {
			return response, err
		}
		failures = append(failures, attemptFailure{id: id, classes: classes, err: err})
		trigger = classes[0]
	}

	return last, &councilruntime.RunError{
		Class: councilruntime.FailureAdapterPoolExhausted,
		Err:   errors.New(formatFailures(failures)),
	}
}

type attemptFailure struct {
	id      AdapterID
	classes []councilruntime.FailureClass
	err     error
}

func allAvailability(classes []councilruntime.FailureClass) bool {
	if len(classes) == 0 {
		return false
	}
	for _, class := range classes {
		switch class {
		case councilruntime.FailureQuotaExhausted,
			councilruntime.FailureAuth,
			councilruntime.FailureAdapterUnavailable:
		default:
			return false
		}
	}
	return true
}
func failureClasses(err error) []councilruntime.FailureClass {
	if err == nil {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var out []councilruntime.FailureClass
		for _, child := range joined.Unwrap() {
			out = append(out, failureClasses(child)...)
		}
		return out
	}
	var run *councilruntime.RunError
	if errors.As(err, &run) {
		return []councilruntime.FailureClass{run.Class}
	}
	return nil
}

func formatFailures(failures []attemptFailure) string {
	if len(failures) == 0 {
		return "adapter pool exhausted"
	}
	parts := make([]string, 0, len(failures))
	for _, failure := range failures {
		classes := make([]string, len(failure.classes))
		for i, class := range failure.classes {
			classes[i] = string(class)
		}
		parts = append(parts, fmt.Sprintf("%s[%s]: %v", failure.id, strings.Join(classes, "+"), failure.err))
	}
	return "adapter pool exhausted: " + strings.Join(parts, "; ")
}
