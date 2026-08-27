package runnerbootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestH1FrozenWorkflowAllowsFreshManualDispatchWithoutPolicyInputs(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", ".github", "workflows", "h1-frozen-execution.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	for _, required := range []string{
		"workflow_dispatch:",
		"github.event_name == 'workflow_dispatch'",
		"ref: 5f22d664495e40bd7460588c938191f500a8aa65",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("H1 workflow missing %q", required)
		}
	}
	if strings.Contains(text, "workflow_dispatch:\n    inputs:") {
		t.Fatal("H1 workflow dispatch must not expose policy/runtime override inputs")
	}
}
