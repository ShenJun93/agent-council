package runnerbootstrap

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const h4FrozenSHA = "375be888a49e261667362063e8ec03a2c42e152f"

func TestGenericWorkflowRendererEmbedsFrozenContract(t *testing.T) {
	out := filepath.Join(t.TempDir(), "h4.yml")
	renderWorkflow(t, "h4", h4FrozenSHA, out)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"name: H4 Frozen Execution",
		"workflow_dispatch:",
		"ref: " + h4FrozenSHA,
		"- h4-benchmark",
		"1286bbaa9bc630308f2cf81ac0811f11dc084c1d3092810b54ae3301eab0cad0",
		"6439c683279e3e7997bcfa19e42b8a42d1d354414c16d1e7a5cb6bd4141d6b39",
		"1ec5d7aa3d36efbeffc53d4455143bb2a542326a954d77a77c006f8cbe77cfa8",
		"--dataset benchmarks/h4",
		".h4-audit",
		"h4-run.json",
		"eval/batch-summary.json",
		"h4-result.json",
		"if: always()",
		"retention-days: 90",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("rendered workflow missing %q", required)
		}
	}
	if strings.Contains(text, "\n  push:") || strings.Contains(text, "\n  pull_request:") {
		t.Fatal("rendered workflow must be manual-only")
	}
	if strings.Contains(text, "workflow_dispatch:\n    inputs:") {
		t.Fatal("rendered workflow must not expose inputs")
	}
	command := "go run ./cmd/agentd council benchmark h4"
	if got := strings.Count(text, command); got != 1 {
		t.Fatalf("benchmark command count=%d want 1", got)
	}
}
func TestGenericWorkflowRendererIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.yml")
	second := filepath.Join(dir, "second.yml")
	renderWorkflow(t, "h4", h4FrozenSHA, first)
	renderWorkflow(t, "h4", h4FrozenSHA, second)
	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("renderer output is not deterministic")
	}
}

func TestGenericWorkflowRendererRejectsUnsafeInputs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		benchmark string
		sha       string
	}{
		{"unsafe benchmark", "../h5", h4FrozenSHA},
		{"invalid sha", "h4", "deadbeef"},
		{"missing dataset", "h99", h4FrozenSHA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out.yml")
			cmd := exec.Command("/bin/bash", workflowRendererScript(t), "--benchmark", tc.benchmark, "--frozen-sha", tc.sha, "--output", out)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("unsafe render succeeded: %s", output)
			}
		})
	}
}

func TestGenericWorkflowRendererRejectsExistingOutput(t *testing.T) {
	out := filepath.Join(t.TempDir(), "existing.yml")
	if err := os.WriteFile(out, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/bash", workflowRendererScript(t), "--benchmark", "h4", "--frozen-sha", h4FrozenSHA, "--output", out)
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("existing output overwritten: %s", output)
	}
	data, _ := os.ReadFile(out)
	if string(data) != "keep" {
		t.Fatalf("existing output changed: %q", data)
	}
}
func renderWorkflow(t *testing.T, benchmark, sha, out string) {
	t.Helper()
	cmd := exec.Command("/bin/bash", workflowRendererScript(t), "--benchmark", benchmark, "--frozen-sha", sha, "--output", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render failed: %v\n%s", err, output)
	}
}

func workflowRendererScript(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "scripts", "render-frozen-benchmark-workflow.sh")
}
func TestGenericWorkflowRendererCheckDetectsDrift(t *testing.T) {
	out := filepath.Join(t.TempDir(), "h4.yml")
	renderWorkflow(t, "h4", h4FrozenSHA, out)
	check := exec.Command("/bin/bash", workflowRendererScript(t), "--benchmark", "h4", "--frozen-sha", h4FrozenSHA, "--output", out, "--check")
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("check rejected matching workflow: %v\n%s", err, output)
	}
	if err := os.WriteFile(out, []byte("drift\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	check = exec.Command("/bin/bash", workflowRendererScript(t), "--benchmark", "h4", "--frozen-sha", h4FrozenSHA, "--output", out, "--check")
	if output, err := check.CombinedOutput(); err == nil {
		t.Fatalf("check accepted drifted workflow: %s", output)
	}
}

func TestGenericWorkflowRendererH5EmbedsAdapterPolicyHash(t *testing.T) {
	out := filepath.Join(t.TempDir(), "h5.yml")
	renderWorkflow(t, "h5", "5df2f40af9535d61c30ab56f89bff4dd4d5f2de7", out)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"e12e67dac8af5f7cba704a36f2b3030a898ae869bac4ce4573421b2e2a93d890",
		"benchmarks/h5/adapter-policy.json",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("rendered H5 workflow missing %q", required)
		}
	}
}

func TestGenericWorkflowRendererH5UsesAvailabilityPreflight(t *testing.T) {
	out := filepath.Join(t.TempDir(), "h5.yml")
	renderWorkflow(t, "h5", "5df2f40af9535d61c30ab56f89bff4dd4d5f2de7", out)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{"Verify subscription adapter availability", "human-chatgpt-session: frozen final availability fallback", "GEMINI_API_KEY", "GOOGLE_API_KEY"} {
		if !strings.Contains(text, required) {
			t.Fatalf("rendered H5 workflow missing %q", required)
		}
	}
	if strings.Contains(text, "command -v claude | tee") || strings.Contains(text, "command -v codex | tee") {
		t.Fatal("H5 workflow must not require Claude or Codex individually")
	}
}

func TestGenericWorkflowRendererH5HashesAdapterPolicyInAuditList(t *testing.T) {
	out := filepath.Join(t.TempDir(), "h5.yml")
	renderWorkflow(t, "h5", "5df2f40af9535d61c30ab56f89bff4dd4d5f2de7", out)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "benchmarks/h5/cases.json benchmarks/h5/adapter-policy.json") {
		t.Fatal("H5 audit hash list must include adapter-policy.json")
	}
}
