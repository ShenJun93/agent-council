package runnerbootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestH3FrozenWorkflowIsManualOnlyAndPinned(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", ".github", "workflows", "h3-frozen-execution.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"workflow_dispatch:",
		"github.event_name == 'workflow_dispatch'",
		"ref: 25076d3ca8b71c95ede2dbc8d220b1fb592d1110",
		"- self-hosted",
		"- linux",
		"- h3-benchmark",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("H3 workflow missing %q", required)
		}
	}
	if strings.Contains(text, "workflow_dispatch:\n    inputs:") {
		t.Fatal("H3 workflow dispatch must not expose inputs")
	}
	if strings.Contains(text, "\n  push:") || strings.Contains(text, "\n  pull_request:") {
		t.Fatal("H3 frozen workflow must be manual-only")
	}
	command := "go run ./cmd/agentd council benchmark h3"
	if strings.Count(text, command) != 1 {
		t.Fatalf("H3 benchmark command count=%d want 1", strings.Count(text, command))
	}
	for _, required := range []string{
		"--dataset benchmarks/h3",
		"if: always()",
		".h3-audit",
		".council/runs",
		"h3-run.json",
		"eval/batch-summary.json",
		"h3-result.json",
		"retention-days: 90",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("H3 workflow missing evidence contract %q", required)
		}
	}
}
