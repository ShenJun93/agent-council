package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCodexRuntimeFailsClosedOnNativeWindows(t *testing.T) {
	t.Parallel()

	environ := codexFileAuthEnvironmentForTest(t)
	runner := &fakeProcessRunner{}
	rt := newCodexCLI("codex", runner, func() []string { return environ })
	cli, ok := rt.(*cliRuntime)
	if !ok {
		t.Fatalf("runtime type = %T, want *cliRuntime", rt)
	}
	cli.goos = "windows"

	_, err := rt.Run(context.Background(), isolatedRequest(t, "review"))
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureIsolation {
		t.Fatalf("Run() error = %v, want isolation failure", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "wsl2") {
		t.Fatalf("Run() error = %v, want WSL2 remediation", err)
	}
	if len(runner.specs) != 0 {
		t.Fatalf("spawned %d processes on unsupported native Windows Codex path", len(runner.specs))
	}
}
