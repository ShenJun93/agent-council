package runtime

import (
	"context"
	"testing"
)

func TestCodexIsolationRuntimeDisablesLocalAndPeerCapabilitySurfaces(t *testing.T) {
	t.Parallel()

	runner := &fakeProcessRunner{results: []processResult{
		{Stderr: "Logged in using ChatGPT\n", ExitCode: 0},
		{Stdout: "answer\n", ExitCode: 0},
	}}
	rt := newCodexCLI("codex", runner, func() []string { return []string{"PATH=/bin", "HOME=/home/test"} })

	if _, err := rt.Run(context.Background(), isolatedRequest(t, "review")); err != nil {
		t.Fatal(err)
	}
	if len(runner.specs) != 2 {
		t.Fatalf("process calls = %d, want auth + execution", len(runner.specs))
	}

	args := runner.specs[1].Args
	for _, feature := range []string{
		"shell_tool",
		"code_mode",
		"code_mode_host",
		"apps",
		"plugins",
		"multi_agent",
		"tool_suggest",
	} {
		if !hasArgPair(args, "--disable", feature) {
			t.Fatalf("Codex execution args do not disable %q: %#v", feature, args)
		}
	}
}

func hasArgPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}
