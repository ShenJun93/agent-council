package doctor

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type fakeRuntime struct {
	provider councilruntime.Provider
	leak     bool
	err      error
	secret   string
	requests []councilruntime.AgentRequest
}

func (f *fakeRuntime) Run(_ context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return councilruntime.AgentResponse{}, f.err
	}

	match := regexp.MustCompile(`(?m)^EXTERNAL_PATH: (.+)$`).FindStringSubmatch(req.Prompt)
	if len(match) != 2 {
		return councilruntime.AgentResponse{}, errors.New("probe path missing")
	}
	secret, err := os.ReadFile(strings.TrimSpace(match[1]))
	if err != nil {
		return councilruntime.AgentResponse{}, err
	}
	f.secret = string(secret)

	stdout := probeOK + "\nACCESS_DENIED\n"
	if f.leak {
		stdout = probeOK + "\n" + f.secret
	}

	return councilruntime.AgentResponse{Provider: f.provider, Stdout: stdout, ExitCode: 0, Attempts: 1}, nil
}

func TestIsolationDoctorPassesWhenProvidersCannotReadExternalSecret(t *testing.T) {
	t.Parallel()

	claude := &fakeRuntime{provider: councilruntime.ProviderClaude}
	codex := &fakeRuntime{provider: councilruntime.ProviderCodex}
	report, err := RunIsolation(context.Background(), []Probe{
		{Name: "claude", Runtime: claude},
		{Name: "codex", Runtime: codex},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Gate != GatePass {
		t.Fatalf("gate = %q, want %q: %+v", report.Gate, GatePass, report)
	}
	if len(report.Providers) != 2 || !report.Providers[0].Pass || !report.Providers[1].Pass {
		t.Fatalf("unexpected provider report: %+v", report.Providers)
	}

	for _, f := range []*fakeRuntime{claude, codex} {
		if len(f.requests) != 1 {
			t.Fatalf("runtime requests = %d, want 1", len(f.requests))
		}
		if f.requests[0].Workdir == "" || f.requests[0].RunRoot == "" {
			t.Fatalf("probe request missing isolation roots: %+v", f.requests[0])
		}
		if f.secret == "" {
			t.Fatal("fake runtime did not observe the external sentinel")
		}
		if strings.Contains(f.requests[0].Prompt, f.secret) {
			t.Fatal("probe prompt contained the secret sentinel")
		}
	}
}

func TestIsolationDoctorFailsClosedOnSecretLeak(t *testing.T) {
	t.Parallel()

	leaking := &fakeRuntime{provider: councilruntime.ProviderCodex, leak: true}
	report, err := RunIsolation(context.Background(), []Probe{{Name: "codex", Runtime: leaking}})
	if err == nil {
		t.Fatal("RunIsolation() returned nil error for a secret leak")
	}
	if report.Gate != GateFail || len(report.Providers) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !report.Providers[0].SecretLeak || report.Providers[0].Pass {
		t.Fatalf("leak was not recorded: %+v", report.Providers[0])
	}
}

func TestIsolationDoctorFailsClosedWhenRuntimeCannotCompleteProbe(t *testing.T) {
	t.Parallel()

	broken := &fakeRuntime{provider: councilruntime.ProviderClaude, err: errors.New("auth failed")}
	report, err := RunIsolation(context.Background(), []Probe{{Name: "claude", Runtime: broken}})
	if err == nil {
		t.Fatal("RunIsolation() returned nil error for a broken runtime")
	}
	if report.Gate != GateFail || report.Providers[0].Pass {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Providers[0].Error == "" {
		t.Fatal("provider error was not recorded")
	}
}
