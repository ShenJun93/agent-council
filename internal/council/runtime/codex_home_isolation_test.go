package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type codexHomeInspectRunner struct {
	t                 *testing.T
	calls             int
	sourceUserHome    string
	sourceCodexHome   string
	isolatedCodexHome string
}

func (r *codexHomeInspectRunner) Run(_ context.Context, spec processSpec) processResult {
	r.calls++
	if r.calls == 1 {
		return processResult{Stderr: "Logged in using ChatGPT\n", ExitCode: 0}
	}

	env := environmentMapForTest(spec.Env)
	isolatedCodexHome := env["CODEX_HOME"]
	if isolatedCodexHome == "" {
		r.t.Fatal("Codex execution environment is missing CODEX_HOME")
	}
	if samePathForTest(isolatedCodexHome, r.sourceCodexHome) {
		r.t.Fatalf("Codex execution reused ambient CODEX_HOME %q", isolatedCodexHome)
	}
	r.isolatedCodexHome = isolatedCodexHome

	for _, key := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		value := env[key]
		if value == "" {
			continue
		}
		if strings.Contains(strings.ToLower(filepath.Clean(value)), strings.ToLower(filepath.Clean(r.sourceUserHome))) {
			r.t.Fatalf("Codex execution inherited ambient %s=%q", key, value)
		}
	}

	authPath := filepath.Join(isolatedCodexHome, "auth.json")
	auth, err := os.ReadFile(authPath)
	if err != nil {
		r.t.Fatalf("isolated Codex auth is unavailable during execution: %v", err)
	}
	if !strings.Contains(string(auth), `"auth_mode":"chatgpt"`) {
		r.t.Fatalf("isolated auth does not preserve ChatGPT mode: %s", auth)
	}
	for _, forbidden := range []string{"AGENTS.md", "AGENTS.override.md", "config.toml", "skills", "plugins"} {
		if _, err := os.Stat(filepath.Join(isolatedCodexHome, forbidden)); err == nil || !os.IsNotExist(err) {
			r.t.Fatalf("ambient Codex artifact %q exists in isolated home (err=%v)", forbidden, err)
		}
	}

	return processResult{Stdout: "answer\n", ExitCode: 0}
}

func TestCodexExecutionUsesEphemeralSanitizedHomeWithOnlyChatGPTAuth(t *testing.T) {
	t.Parallel()

	sourceUserHome := t.TempDir()
	sourceCodexHome := filepath.Join(sourceUserHome, ".codex")
	if err := os.MkdirAll(sourceCodexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceCodexHome, "auth.json"), []byte(`{"auth_mode":"chatgpt","OPENAI_API_KEY":null,"tokens":{"access_token":"test-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceCodexHome, "AGENTS.md"), []byte("ambient instructions"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceCodexHome, "config.toml"), []byte("model = \"ambient\""), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(sourceUserHome, ".agents", "skills", "ambient"), 0o700); err != nil {
		t.Fatal(err)
	}

	runner := &codexHomeInspectRunner{t: t, sourceUserHome: sourceUserHome, sourceCodexHome: sourceCodexHome}
	rt := newCodexCLI("codex", runner, func() []string {
		return []string{
			"PATH=/bin",
			"HOME=" + sourceUserHome,
			"USERPROFILE=" + sourceUserHome,
			"APPDATA=" + filepath.Join(sourceUserHome, "AppData", "Roaming"),
			"LOCALAPPDATA=" + filepath.Join(sourceUserHome, "AppData", "Local"),
			"CODEX_HOME=" + sourceCodexHome,
		}
	})

	if _, err := rt.Run(context.Background(), isolatedRequest(t, "review")); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 2 {
		t.Fatalf("process calls = %d, want auth + execution", runner.calls)
	}
	if runner.isolatedCodexHome == "" {
		t.Fatal("execution did not observe an isolated Codex home")
	}
	if _, err := os.Stat(runner.isolatedCodexHome); !os.IsNotExist(err) {
		t.Fatalf("isolated Codex home was not cleaned up: %v", err)
	}
}

func TestCodexExecutionFailsClosedWhenChatGPTAuthCannotBeIsolated(t *testing.T) {
	t.Parallel()

	sourceUserHome := t.TempDir()
	sourceCodexHome := filepath.Join(sourceUserHome, ".codex")
	if err := os.MkdirAll(sourceCodexHome, 0o700); err != nil {
		t.Fatal(err)
	}

	runner := &fakeProcessRunner{results: []processResult{{Stderr: "Logged in using ChatGPT\n", ExitCode: 0}}}
	rt := newCodexCLI("codex", runner, func() []string {
		return []string{"PATH=/bin", "HOME=" + sourceUserHome, "USERPROFILE=" + sourceUserHome, "CODEX_HOME=" + sourceCodexHome}
	})

	_, err := rt.Run(context.Background(), isolatedRequest(t, "review"))
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureIsolation {
		t.Fatalf("Run() error = %v, want isolation failure", err)
	}
	if len(runner.specs) != 1 {
		t.Fatalf("process calls = %d, want auth preflight only", len(runner.specs))
	}
}

func TestCodexExecutionDisablesAmbientInstructionSources(t *testing.T) {
	t.Parallel()

	environ := codexFileAuthEnvironmentForTest(t)
	runner := &fakeProcessRunner{results: []processResult{
		{Stderr: "Logged in using ChatGPT\n", ExitCode: 0},
		{Stdout: "answer\n", ExitCode: 0},
	}}
	rt := newCodexCLI("codex", runner, func() []string { return environ })

	if _, err := rt.Run(context.Background(), isolatedRequest(t, "review")); err != nil {
		t.Fatal(err)
	}
	args := runner.specs[1].Args
	for _, override := range []string{
		"skills.include_instructions=false",
		"skills.bundled.enabled=false",
		"project_doc_max_bytes=0",
	} {
		if !hasArgPair(args, "-c", override) {
			t.Fatalf("Codex execution args do not set %q: %#v", override, args)
		}
	}
}

func codexFileAuthEnvironmentForTest(t *testing.T) []string {
	t.Helper()
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte(`{"auth_mode":"chatgpt","OPENAI_API_KEY":null,"tokens":{"access_token":"test-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=/bin", "HOME=" + home, "USERPROFILE=" + home, "CODEX_HOME=" + codexHome}
}

func environmentMapForTest(environ []string) map[string]string {
	values := make(map[string]string)
	for _, entry := range environ {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			values[strings.ToUpper(parts[0])] = parts[1]
		}
	}
	return values
}

func samePathForTest(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
