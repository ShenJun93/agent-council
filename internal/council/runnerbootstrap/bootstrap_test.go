package runnerbootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestH1RunnerBootstrapPreflightAcceptsSubscriptionCLIs(t *testing.T) {
	t.Parallel()

	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", `#!/bin/sh
if [ "$1 $2" = "auth status" ]; then exit 0; fi
exit 1
`)
	writeFakeCLI(t, bin, "claude", `#!/bin/sh
if [ "$1" = "--version" ]; then echo "Claude Code v2.1.165"; exit 0; fi
if [ "$1 $2" = "auth status" ]; then
  echo '{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty","subscriptionType":"pro"}'
  exit 0
fi
exit 1
`)
	writeFakeCLI(t, bin, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then echo "codex-cli 0.149.0"; exit 0; fi
if [ "$1 $2" = "login status" ]; then echo "Logged in using ChatGPT"; exit 0; fi
exit 1
`)

	cmd := exec.Command("/bin/bash", bootstrapScript(t), "--preflight-only")
	cmd.Env = []string{
		"PATH=" + bin + ":/usr/bin:/bin",
		"HOME=" + t.TempDir(),
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("preflight failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "H1 runner preflight OK") {
		t.Fatalf("preflight output missing success marker: %s", output)
	}
}

func TestH1RunnerBootstrapPreflightRejectsMeteredAPIEnvironment(t *testing.T) {
	t.Parallel()

	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", "#!/bin/sh\nexit 0\n")
	writeFakeCLI(t, bin, "claude", "#!/bin/sh\nexit 0\n")
	writeFakeCLI(t, bin, "codex", "#!/bin/sh\nexit 0\n")

	cmd := exec.Command("/bin/bash", bootstrapScript(t), "--preflight-only")
	cmd.Env = []string{
		"PATH=" + bin + ":/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"OPENAI_API_KEY=forbidden",
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("preflight accepted OPENAI_API_KEY: %s", output)
	}
	if !strings.Contains(string(output), "metered API credentials are forbidden") {
		t.Fatalf("unexpected rejection: %s", output)
	}
}

func TestH1RunnerBootstrapPinsEphemeralH1Label(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(bootstrapScript(t))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"actions/runners/registration-token",
		"--ephemeral",
		"h1-benchmark",
		"./run.sh",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("bootstrap script missing %q", required)
		}
	}
}

func bootstrapScript(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "scripts", "bootstrap-h1-runner.sh")
}

func writeFakeCLI(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}
