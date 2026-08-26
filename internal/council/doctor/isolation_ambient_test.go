package doctor

import (
	"context"
	"testing"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type ambientContextRuntime struct{}

func (ambientContextRuntime) Run(_ context.Context, _ councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	return councilruntime.AgentResponse{
		Provider: councilruntime.ProviderCodex,
		Stdout:   probeOK + "\n" + accessDenied + "\n",
		Stderr:   `failed to load skill C:\Users\example\.agents\skills\ambient\SKILL.md`,
		ExitCode: 0,
		Attempts: 1,
	}, nil
}

func TestIsolationDoctorFailsClosedOnAmbientHostContext(t *testing.T) {
	t.Parallel()

	report, err := RunIsolation(context.Background(), []Probe{{Name: "codex", Runtime: ambientContextRuntime{}}})
	if err == nil {
		t.Fatal("RunIsolation() returned nil error for ambient host context")
	}
	if report.Gate != GateFail || len(report.Providers) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	provider := report.Providers[0]
	if !provider.AmbientContext || provider.Pass {
		t.Fatalf("ambient host context was not fail-closed: %+v", provider)
	}
}
