package runnerbootstrap

import (
	"os/exec"
	"strings"
	"testing"
)

func TestGenericBootstrapH9DoesNotRequireModelCLIs(t *testing.T) {
	bin := t.TempDir()
	writeFakeCLI(t, bin, "gh", `#!/bin/sh
if [ "$1 $2" = "auth status" ]; then exit 0; fi
if [ "$1 $2 $3 $4" = "api -X POST repos/ShenJun93/agent-council/actions/runners/registration-token" ]; then
  echo '{"token":"test-registration-token"}'; exit 0
fi
exit 1
`)
	cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h9", "--preflight-only")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir()}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("H9 human-broker preflight must not require Claude/Codex CLIs: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "H9 runner preflight OK") {
		t.Fatalf("missing H9 success marker: %s", output)
	}
}

func TestGenericBootstrapH9RejectsExtendedMeteredAPIEnvironment(t *testing.T) {
	for _, key := range []string{"GEMINI_API_KEY", "GOOGLE_API_KEY", "AZURE_OPENAI_API_KEY"} {
		t.Run(key, func(t *testing.T) {
			bin := t.TempDir()
			writeFakeCLI(t, bin, "gh", "#!/bin/sh\nexit 0\n")
			cmd := exec.Command("/bin/bash", genericBootstrapScript(t), "--benchmark", "h9", "--preflight-only")
			cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir(), key + "=forbidden"}
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("H9 accepted metered credential %s: %s", key, output)
			}
			if !strings.Contains(string(output), "metered API credentials are forbidden for H9") {
				t.Fatalf("H9 rejected %s for the wrong reason: %s", key, output)
			}
		})
	}
}
