package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestCodexRuntimeFailsClosedWhenLinuxResolvesWindowsCodexShim(t *testing.T) {
	t.Parallel()

	runner := &fakeProcessRunner{}
	rt := newCodexCLI("codex", runner, func() []string { return []string{"PATH=/bin"} }).(*cliRuntime)
	rt.goos = "linux"
	rt.lookPath = func(string) (string, error) {
		return `/mnt/c/Users/example/AppData/Roaming/npm/codex`, nil
	}

	_, err := rt.Run(context.Background(), isolatedRequest(t, "review"))
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureIsolation {
		t.Fatalf("Run() error = %v, want isolation failure", err)
	}
	if len(runner.specs) != 0 {
		t.Fatalf("process calls = %d, want zero before rejecting Windows Codex shim", len(runner.specs))
	}
}
