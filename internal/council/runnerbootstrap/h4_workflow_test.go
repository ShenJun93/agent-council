package runnerbootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestH4FrozenWorkflowIsManualOnlyAndPinned(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", ".github", "workflows", "h4-frozen-execution.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"workflow_dispatch:",
		"github.event_name == 'workflow_dispatch'",
		"ref: 375be888a49e261667362063e8ec03a2c42e152f",
		"- self-hosted",
		"- linux",
		"- h4-benchmark",
		"1286bbaa9bc630308f2cf81ac0811f11dc084c1d3092810b54ae3301eab0cad0",
		"6439c683279e3e7997bcfa19e42b8a42d1d354414c16d1e7a5cb6bd4141d6b39",
		"1ec5d7aa3d36efbeffc53d4455143bb2a542326a954d77a77c006f8cbe77cfa8",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("H4 workflow missing %q", required)
		}
	}
	if strings.Contains(text, "workflow_dispatch:\n    inputs:") {
		t.Fatal("H4 workflow dispatch must not expose inputs")
	}
	if strings.Contains(text, "\n  push:") || strings.Contains(text, "\n  pull_request:") {
		t.Fatal("H4 frozen workflow must be manual-only")
	}
	command := "go run ./cmd/agentd council benchmark h4"
	if strings.Count(text, command) != 1 {
		t.Fatalf("H4 benchmark command count=%d want 1", strings.Count(text, command))
	}
	for _, required := range []string{
		"--dataset benchmarks/h4",
		"if: always()",
		".h4-audit",
		".council/runs",
		"h4-run.json",
		"eval/batch-summary.json",
		"h4-result.json",
		"retention-days: 90",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("H4 workflow missing evidence contract %q", required)
		}
	}
}
