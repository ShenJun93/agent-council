package doctor

import (
	"context"
	"strings"
	"testing"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type diagnosticRuntime struct {
	stdout string
	stderr string
}

func (d diagnosticRuntime) Run(_ context.Context, _ councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	return councilruntime.AgentResponse{
		Provider: councilruntime.ProviderCodex,
		Stdout:   d.stdout,
		Stderr:   d.stderr,
		ExitCode: 0,
		Attempts: 1,
	}, nil
}

func TestIsolationDoctorIncludesSafeFailureOutputPreviews(t *testing.T) {
	t.Parallel()

	runtime := diagnosticRuntime{
		stdout: "I cannot access local files because no shell tool is available.",
		stderr: "codex diagnostic stream",
	}
	report, err := RunIsolation(context.Background(), []Probe{{Name: "codex", Runtime: runtime}})
	if err == nil {
		t.Fatal("RunIsolation() returned nil error without probe acknowledgements")
	}
	if len(report.Providers) != 1 {
		t.Fatalf("providers = %d, want 1", len(report.Providers))
	}
	provider := report.Providers[0]
	if provider.StdoutPreview != runtime.stdout {
		t.Fatalf("stdout preview = %q, want %q", provider.StdoutPreview, runtime.stdout)
	}
	if provider.StderrPreview != runtime.stderr {
		t.Fatalf("stderr preview = %q, want %q", provider.StderrPreview, runtime.stderr)
	}
}

func TestIsolationPromptDefinesExactDeniedOutputContract(t *testing.T) {
	t.Parallel()

	prompt := isolationPrompt("C:/outside/secret.txt")
	for _, want := range []string{
		"Your final response must be exactly one of these two forms",
		"PROBE_OK\\nACCESS_DENIED",
		"If no local capability is available",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("isolation prompt missing %q: %s", want, prompt)
		}
	}
}
