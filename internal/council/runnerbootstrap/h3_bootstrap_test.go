package runnerbootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestH3RunnerBootstrapPreflightAcceptsSubscriptionCLIs(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", `#!/bin/sh
if [ "$1 $2" = "auth status" ]; then exit 0; fi
if [ "$1 $2 $3 $4" = "api -X POST repos/ShenJun93/agent-council/actions/runners/registration-token" ]; then
  echo '{"token":"test-registration-token"}'
  exit 0
fi
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
	cmd := exec.Command("/bin/bash", h3BootstrapScript(t), "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir()}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("preflight failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "H3 runner preflight OK") {
		t.Fatalf("missing H3 success marker: %s", output)
	}
}

func TestH3RunnerBootstrapRejectsMeteredAPIEnvironment(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", "#!/bin/sh\nexit 0\n")
	writeFakeCLI(t, bin, "claude", "#!/bin/sh\nexit 0\n")
	writeFakeCLI(t, bin, "codex", "#!/bin/sh\nexit 0\n")
	cmd := exec.Command("/bin/bash", h3BootstrapScript(t), "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir(), "ANTHROPIC_API_KEY=forbidden"}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("preflight accepted metered credential: %s", output)
	}
	if !strings.Contains(string(output), "metered API credentials are forbidden") {
		t.Fatalf("unexpected rejection: %s", output)
	}
}

func TestH3RunnerBootstrapPinsEphemeralH3Label(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(h3BootstrapScript(t))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"actions/runners/registration-token",
		"--ephemeral",
		"h3-benchmark",
		"./run.sh",
		"sha256:",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("H3 bootstrap missing %q", required)
		}
	}
}

func h3BootstrapScript(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "scripts", "bootstrap-h3-runner.sh")
}
