package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimeMaxAttemptsOneDisablesSameAdapterRetry(t *testing.T) {
	t.Parallel()
	runner := &fakeProcessRunner{results: []processResult{
		{Stdout: "Logged in using ChatGPT\n", ExitCode: 0},
		{Stderr: "temporary crash", ExitCode: 2, Err: errors.New("exit status 2")},
		{Stdout: "must-not-run", ExitCode: 0},
	}}
	environ := codexFileAuthEnvironmentForTest(t)
	rt := newCodexCLI("codex", runner, func() []string { return environ })
	req := isolatedRequest(t, "x")
	req.MaxAttempts = 1

	resp, err := rt.Run(context.Background(), req)
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureProcess {
		t.Fatalf("Run() error=%v want process failure", err)
	}
	if resp.Attempts != 1 {
		t.Fatalf("attempts=%d want 1", resp.Attempts)
	}
	if len(runner.specs) != 2 {
		t.Fatalf("process calls=%d want auth + one execution", len(runner.specs))
	}
}

func TestRuntimeZeroMaxAttemptsPreservesLegacyRetry(t *testing.T) {
	t.Parallel()
	runner := &fakeProcessRunner{results: []processResult{
		{Stdout: "Logged in using ChatGPT\n", ExitCode: 0},
		{Stderr: "temporary crash", ExitCode: 2, Err: errors.New("exit status 2")},
		{Stdout: "recovered", ExitCode: 0},
	}}
	environ := codexFileAuthEnvironmentForTest(t)
	rt := newCodexCLI("codex", runner, func() []string { return environ })
	req := isolatedRequest(t, "x")

	resp, err := rt.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Attempts != 2 || resp.Stdout != "recovered" {
		t.Fatalf("unexpected legacy response: %+v", resp)
	}
	if len(runner.specs) != 3 {
		t.Fatalf("process calls=%d want auth + two executions", len(runner.specs))
	}
}
