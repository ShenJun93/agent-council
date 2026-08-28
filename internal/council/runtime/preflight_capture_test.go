package runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRuntimeCanExposeRawAuthFailureForAdapterEvidence(t *testing.T) {
	t.Parallel()
	started := time.Unix(20, 0).UTC()
	finished := time.Unix(21, 0).UTC()
	runner := &fakeProcessRunner{results: []processResult{{
		Stderr: "usage limit reached", ExitCode: 1, Err: errors.New("exit status 1"),
		StartedAt: started, FinishedAt: finished,
	}}}
	environ := codexFileAuthEnvironmentForTest(t)
	rt := newCodexCLI("codex", runner, func() []string { return environ })
	req := isolatedRequest(t, "x")
	req.CapturePreflightFailure = true

	resp, err := rt.Run(context.Background(), req)
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureQuotaExhausted {
		t.Fatalf("Run() error=%v want quota exhausted", err)
	}
	if resp.Provider != ProviderCodex || resp.Stderr != "usage limit reached" || resp.ExitCode != 1 {
		t.Fatalf("raw auth response lost: %+v", resp)
	}
	if !resp.StartedAt.Equal(started) || !resp.FinishedAt.Equal(finished) || resp.Attempts != 0 {
		t.Fatalf("auth timing/attempt metadata=%+v", resp)
	}
}

func TestRuntimeLegacyAuthFailureStillReturnsZeroResponse(t *testing.T) {
	t.Parallel()
	runner := &fakeProcessRunner{results: []processResult{{
		Stderr: "usage limit reached", ExitCode: 1, Err: errors.New("exit status 1"),
		StartedAt: time.Unix(20, 0).UTC(), FinishedAt: time.Unix(21, 0).UTC(),
	}}}
	environ := codexFileAuthEnvironmentForTest(t)
	rt := newCodexCLI("codex", runner, func() []string { return environ })
	resp, err := rt.Run(context.Background(), isolatedRequest(t, "x"))
	if err == nil {
		t.Fatal("expected quota failure")
	}
	if resp != (AgentResponse{}) {
		t.Fatalf("legacy response changed: %+v", resp)
	}
}
