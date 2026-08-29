package runnerbootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericBootstrapPreflightAcceptsSubscriptionCLIs(t *testing.T) {
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", `#!/bin/sh
if [ "$1 $2" = "auth status" ]; then exit 0; fi
if [ "$1 $2 $3 $4" = "api -X POST repos/ShenJun93/agent-council/actions/runners/registration-token" ]; then
  echo '{"token":"test-registration-token"}'; exit 0
fi
exit 1
`)
	writeFakeCLI(t, bin, "claude", `#!/bin/sh
if [ "$1" = "--version" ]; then echo "Claude Code v2.1.245"; exit 0; fi
if [ "$1 $2" = "auth status" ]; then
  echo '{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty","subscriptionType":"max"}'; exit 0
fi
exit 1
`)
	writeFakeCLI(t, bin, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then echo "codex-cli 0.149.1"; exit 0; fi
if [ "$1 $2" = "login status" ]; then echo "Logged in using ChatGPT"; exit 0; fi
exit 1
`)
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h5", "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir()}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("preflight failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "H5 runner preflight OK") {
		t.Fatalf("missing success marker: %s", output)
	}
}

func TestGenericBootstrapRejectsUnsafeBenchmarkID(t *testing.T) {
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "../h5", "--preflight-only")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("unsafe benchmark id accepted: %s", output)
	}
	if !strings.Contains(string(output), "invalid benchmark id") {
		t.Fatalf("unexpected rejection: %s", output)
	}
}
func TestGenericBootstrapRejectsMeteredAPIEnvironment(t *testing.T) {
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", "#!/bin/sh\nexit 0\n")
	writeFakeCLI(t, bin, "claude", "#!/bin/sh\nexit 0\n")
	writeFakeCLI(t, bin, "codex", "#!/bin/sh\nexit 0\n")
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h5", "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir(), "OPENAI_API_KEY=forbidden"}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("metered credential accepted: %s", output)
	}
	if !strings.Contains(string(output), "metered API credentials are forbidden") {
		t.Fatalf("unexpected rejection: %s", output)
	}
}

func TestGenericBootstrapDerivesEphemeralLabel(t *testing.T) {
	data, err := os.ReadFile(genericBootstrapScript(t))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{"--ephemeral", `RUNNER_LABEL="${BENCHMARK}-benchmark"`, `runner_name="${BENCHMARK}-${host_name}-$$"`, "./run.sh", "sha256:"} {
		if !strings.Contains(text, required) {
			t.Fatalf("generic bootstrap missing %q", required)
		}
	}
}
func genericBootstrapScript(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "scripts", "bootstrap-benchmark-runner.sh")
}
func TestGenericBootstrapRejectsNonLinuxHost(t *testing.T) {
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", "#!/bin/sh\nif [ \"$1 $2\" = \"auth status\" ]; then exit 0; fi\nif [ \"$1\" = \"api\" ]; then echo '{\"token\":\"token\"}'; exit 0; fi\nexit 1\n")
	writeFakeCLI(t, bin, "claude", "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo v; exit 0; fi\nif [ \"$1 $2\" = \"auth status\" ]; then echo '{\"loggedIn\":true,\"authMethod\":\"claude.ai\",\"apiProvider\":\"firstParty\",\"subscriptionType\":\"max\"}'; exit 0; fi\nexit 1\n")
	writeFakeCLI(t, bin, "codex", "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo v; exit 0; fi\nif [ \"$1 $2\" = \"login status\" ]; then echo 'Logged in using ChatGPT'; exit 0; fi\nexit 1\n")
	writeFakeCLI(t, bin, "uname", "#!/bin/sh\nif [ \"$1\" = \"-s\" ]; then echo Darwin; else echo x86_64; fi\n")
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h5", "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir()}
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "requires a Linux self-hosted runner") {
		t.Fatalf("non-Linux host not rejected: err=%v output=%s", err, output)
	}
}

func TestGenericBootstrapH5AcceptsCodexWhenClaudeUnavailable(t *testing.T) {
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", `#!/bin/sh
if [ "$1 $2" = "auth status" ]; then exit 0; fi
if [ "$1 $2 $3 $4" = "api -X POST repos/ShenJun93/agent-council/actions/runners/registration-token" ]; then echo '{"token":"test-registration-token"}'; exit 0; fi
exit 1
`)
	writeFakeCLI(t, bin, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then echo "codex-cli 0.149.1"; exit 0; fi
if [ "$1 $2" = "login status" ]; then echo "Logged in using ChatGPT"; exit 0; fi
exit 1
`)
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h5", "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir()}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("H5 should accept a usable Codex adapter without Claude: %v\n%s", err, output)
	}
}

func TestGenericBootstrapH5RejectsGoogleMeteredAPIEnvironment(t *testing.T) {
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", "#!/bin/sh\nexit 0\n")
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h5", "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir(), "GEMINI_API_KEY=forbidden"}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("metered Google credential accepted: %s", output)
	}
	if !strings.Contains(string(output), "metered API credentials are forbidden") {
		t.Fatalf("unexpected rejection: %s", output)
	}
}

func TestGenericBootstrapH6AcceptsCodexWhenClaudeUnavailable(t *testing.T) {
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", `#!/bin/sh
if [ "$1 $2" = "auth status" ]; then exit 0; fi
if [ "$1 $2 $3 $4" = "api -X POST repos/ShenJun93/agent-council/actions/runners/registration-token" ]; then echo '{"token":"test-registration-token"}'; exit 0; fi
exit 1
`)
	writeFakeCLI(t, bin, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then echo "codex-cli"; exit 0; fi
if [ "$1 $2" = "login status" ]; then echo "Logged in using ChatGPT"; exit 0; fi
exit 1
`)
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h6", "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir()}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("H6 should accept Codex without Claude: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "H6 runner preflight OK") {
		t.Fatalf("missing H6 success marker: %s", output)
	}
}

func TestGenericBootstrapH6RejectsGoogleMeteredAPIEnvironment(t *testing.T) {
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", "#!/bin/sh\nexit 0\n")
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h6", "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir(), "GOOGLE_API_KEY=forbidden"}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("metered Google credential accepted: %s", output)
	}
	if !strings.Contains(string(output), "metered API credentials are forbidden") {
		t.Fatalf("unexpected rejection: %s", output)
	}
}

func TestGenericBootstrapH7AcceptsCodexWhenClaudeUnavailable(t *testing.T) {
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", `#!/bin/sh
if [ "$1 $2" = "auth status" ]; then exit 0; fi
if [ "$1 $2 $3 $4" = "api -X POST repos/ShenJun93/agent-council/actions/runners/registration-token" ]; then echo '{"token":"test-registration-token"}'; exit 0; fi
exit 1
`)
	writeFakeCLI(t, bin, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then echo "codex-cli"; exit 0; fi
if [ "$1 $2" = "login status" ]; then echo "Logged in using ChatGPT"; exit 0; fi
exit 1
`)
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h7", "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir()}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("H7 should accept Codex without Claude: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "H7 runner preflight OK") {
		t.Fatalf("missing H7 success marker: %s", output)
	}
}

func TestGenericBootstrapH7RejectsGoogleMeteredAPIEnvironment(t *testing.T) {
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", "#!/bin/sh\nexit 0\n")
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h7", "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir(), "GOOGLE_API_KEY=forbidden"}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("metered Google credential accepted: %s", output)
	}
	if !strings.Contains(string(output), "metered API credentials are forbidden") {
		t.Fatalf("unexpected rejection: %s", output)
	}
}
