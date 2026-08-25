package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type fakeProcessRunner struct {
	results []processResult
	specs   []processSpec
}

func (f *fakeProcessRunner) Run(_ context.Context, spec processSpec) processResult {
	f.specs = append(f.specs, spec)
	if len(f.results) == 0 {
		return processResult{ExitCode: 0}
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result
}

func TestClaudeRuntimePreflightsThenRunsHeadless(t *testing.T) {
	t.Parallel()

	runner := &fakeProcessRunner{results: []processResult{
		{Stdout: `{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty","subscriptionType":"pro"}`, ExitCode: 0},
		{Stdout: "answer\n", ExitCode: 0},
	}}
	rt := newClaudeCLI("claude", runner, func() []string { return []string{"PATH=/bin", "HOME=/home/test"} })

	resp, err := rt.Run(context.Background(), AgentRequest{
		RunID:       "run-1",
		Participant: "researcher-a",
		Role:        "researcher",
		Phase:       "research",
		Prompt:      "analyze independently",
		Workdir:     t.TempDir(),
		Timeout:     time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Provider != ProviderClaude || resp.Stdout != "answer\n" || resp.Attempts != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(runner.specs) != 2 {
		t.Fatalf("process calls = %d, want 2", len(runner.specs))
	}
	if got, want := runner.specs[0].Args, []string{"auth", "status", "--json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("auth args = %#v, want %#v", got, want)
	}
	if got, want := runner.specs[1].Args, []string{"-p", "analyze independently", "--output-format", "text", "--permission-mode", "plan"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("run args = %#v, want %#v", got, want)
	}
}

func TestCodexRuntimeRequiresChatGPTAndUsesReadOnlyEphemeralExec(t *testing.T) {
	t.Parallel()

	runner := &fakeProcessRunner{results: []processResult{
		{Stdout: "Logged in using ChatGPT\n", ExitCode: 0},
		{Stdout: "answer\n", ExitCode: 0},
	}}
	rt := newCodexCLI("codex", runner, func() []string { return []string{"PATH=/bin", "HOME=/home/test"} })
	workdir := t.TempDir()

	_, err := rt.Run(context.Background(), AgentRequest{Prompt: "review", Workdir: workdir, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.specs) != 2 {
		t.Fatalf("process calls = %d, want 2", len(runner.specs))
	}
	if got, want := runner.specs[0].Args, []string{"login", "status"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("auth args = %#v, want %#v", got, want)
	}
	if got, want := runner.specs[1].Args, []string{"exec", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", "review"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("run args = %#v, want %#v", got, want)
	}
	if runner.specs[1].Dir != workdir {
		t.Fatalf("run dir = %q, want %q", runner.specs[1].Dir, workdir)
	}
}

func TestRuntimeFailsClosedBeforeAuthWhenMeteredCredentialExists(t *testing.T) {
	t.Parallel()

	runner := &fakeProcessRunner{}
	rt := newClaudeCLI("claude", runner, func() []string {
		return []string{"PATH=/bin", "ANTHROPIC_API_KEY=secret"}
	})

	_, err := rt.Run(context.Background(), AgentRequest{Prompt: "x", Workdir: t.TempDir()})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureBillingPolicyViolation {
		t.Fatalf("Run() error = %v, want billing policy violation", err)
	}
	if len(runner.specs) != 0 {
		t.Fatalf("spawned %d processes despite billing violation", len(runner.specs))
	}
}

func TestRuntimeRejectsWrongAuthMode(t *testing.T) {
	t.Parallel()

	runner := &fakeProcessRunner{results: []processResult{{Stdout: "Logged in using an API key\n", ExitCode: 0}}}
	rt := newCodexCLI("codex", runner, func() []string { return []string{"PATH=/bin"} })

	_, err := rt.Run(context.Background(), AgentRequest{Prompt: "x", Workdir: t.TempDir()})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureAuth {
		t.Fatalf("Run() error = %v, want auth failure", err)
	}
	if len(runner.specs) != 1 {
		t.Fatalf("process calls = %d, want auth check only", len(runner.specs))
	}
}

func TestRuntimeRetriesProcessFailureOnce(t *testing.T) {
	t.Parallel()

	runner := &fakeProcessRunner{results: []processResult{
		{Stdout: "Logged in using ChatGPT\n", ExitCode: 0},
		{Stderr: "temporary crash", ExitCode: 2, Err: errors.New("exit status 2")},
		{Stdout: "recovered", ExitCode: 0},
	}}
	rt := newCodexCLI("codex", runner, func() []string { return []string{"PATH=/bin"} })

	resp, err := rt.Run(context.Background(), AgentRequest{Prompt: "x", Workdir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Attempts != 2 || resp.Stdout != "recovered" {
		t.Fatalf("unexpected retry response: %+v", resp)
	}
	if len(runner.specs) != 3 {
		t.Fatalf("process calls = %d, want auth + 2 attempts", len(runner.specs))
	}
}

func TestRuntimeDoesNotRetryQuotaFailure(t *testing.T) {
	t.Parallel()

	runner := &fakeProcessRunner{results: []processResult{
		{Stdout: "Logged in using ChatGPT\n", ExitCode: 0},
		{Stderr: "usage limit reached", ExitCode: 1, Err: errors.New("exit status 1")},
	}}
	rt := newCodexCLI("codex", runner, func() []string { return []string{"PATH=/bin"} })

	_, err := rt.Run(context.Background(), AgentRequest{Prompt: "x", Workdir: t.TempDir()})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureQuotaExhausted {
		t.Fatalf("Run() error = %v, want quota exhausted", err)
	}
	if len(runner.specs) != 2 {
		t.Fatalf("process calls = %d, want auth + one attempt", len(runner.specs))
	}
}

func TestRuntimeRejectsEmptyWorkdirBeforeSpawning(t *testing.T) {
	t.Parallel()

	runner := &fakeProcessRunner{}
	rt := newCodexCLI("codex", runner, func() []string { return []string{"PATH=/bin"} })

	_, err := rt.Run(context.Background(), AgentRequest{Prompt: "x"})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureIsolation {
		t.Fatalf("Run() error = %v, want isolation failure", err)
	}
	if len(runner.specs) != 0 {
		t.Fatalf("spawned %d processes with empty workdir", len(runner.specs))
	}
}

func TestRuntimeRejectsWorkdirInsideFullRunRoot(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	runRoot := filepath.Join(base, "run")
	workdir := filepath.Join(runRoot, "participant")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}

	runner := &fakeProcessRunner{}
	rt := newClaudeCLI("claude", runner, func() []string { return []string{"PATH=/bin"} })
	_, err := rt.Run(context.Background(), AgentRequest{Prompt: "x", RunRoot: runRoot, Workdir: workdir})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureIsolation {
		t.Fatalf("Run() error = %v, want isolation failure", err)
	}
	if len(runner.specs) != 0 {
		t.Fatalf("spawned %d processes inside full run root", len(runner.specs))
	}
}

func TestClassifyFailure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		stdout string
		stderr string
		want   FailureClass
	}{
		{name: "timeout", err: context.DeadlineExceeded, want: FailureTimeout},
		{name: "quota", err: errors.New("exit"), stderr: "quota exhausted", want: FailureQuotaExhausted},
		{name: "auth", err: errors.New("exit"), stderr: "401 unauthorized; please login", want: FailureAuth},
		{name: "process", err: errors.New("exit"), stderr: "segmentation fault", want: FailureProcess},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyFailure(tc.err, tc.stdout, tc.stderr); got != tc.want {
				t.Fatalf("classifyFailure() = %q, want %q", got, tc.want)
			}
		})
	}
}
