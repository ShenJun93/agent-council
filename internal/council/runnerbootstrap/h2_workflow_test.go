package runnerbootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const h2FrozenSHA = "96e3c056bc92bf32d52dc159cd743fe5f7d21d32"

func TestH2FrozenWorkflowContract(t *testing.T) {
	path := filepath.Join("..", "..", "..", ".github", "workflows", "h2-frozen-execution.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"workflow_dispatch:",
		"self-hosted", "linux", "h2-benchmark",
		"ref: " + h2FrozenSHA,
		"council benchmark h2",
		"--dataset benchmarks/h2",
		"if: always()",
		"actions/upload-artifact@v4",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("H2 workflow missing %q", required)
		}
	}
	if strings.Contains(text, "workflow_dispatch:\n    inputs:") {
		t.Fatal("H2 workflow dispatch must not expose inputs")
	}
	if got := strings.Count(text, "council benchmark h2"); got != 1 {
		t.Fatalf("H2 benchmark command count=%d want 1", got)
	}
	for _, required := range []string{"OPENAI_API_KEY", "CODEX_API_KEY", "ANTHROPIC_API_KEY", "metered API credentials are forbidden"} {
		if !strings.Contains(text, required) {
			t.Fatalf("H2 workflow missing fail-closed check %q", required)
		}
	}
	if strings.Contains(text, "material-worse-delta") {
		t.Fatal("H2 workflow exposes policy override")
	}
}

func TestH2RunnerBootstrapPinsEphemeralLabel(t *testing.T) {
	path := filepath.Join("..", "..", "..", "scripts", "bootstrap-h2-runner.sh")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"actions/runners/registration-token", "--ephemeral", "h2-benchmark", "./run.sh",
		"claude auth status", "codex login status",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("H2 bootstrap missing %q", required)
		}
	}
	if strings.Contains(text, "h1-benchmark") {
		t.Fatal("H2 bootstrap still pins H1 label")
	}
}
