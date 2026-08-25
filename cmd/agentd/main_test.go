package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/doctor"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type doctorFakeRuntime struct {
	provider councilruntime.Provider
}

func (f doctorFakeRuntime) Run(_ context.Context, _ councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	return councilruntime.AgentResponse{
		Provider: f.provider,
		Stdout:   "PROBE_OK\nACCESS_DENIED\n",
		ExitCode: 0,
		Attempts: 1,
	}, nil
}

func TestRunCouncilRunCreatesRunAndPrintsJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	problem := filepath.Join(root, "problem.md")
	if err := os.WriteFile(problem, []byte("# Problem\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runs := filepath.Join(root, "runs")

	var stdout, stderr bytes.Buffer
	code := run([]string{"council", "run", "--runs-dir", runs, problem}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit = %d, stderr = %s", code, stderr.String())
	}

	var out struct {
		RunID  string `json:"run_id"`
		RunDir string `json:"run_dir"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not JSON: %v: %q", err, stdout.String())
	}
	if out.RunID == "" {
		t.Fatal("run_id is empty")
	}
	if out.RunDir != filepath.Join(runs, out.RunID) {
		t.Fatalf("run_dir = %q", out.RunDir)
	}
	if _, err := os.Stat(filepath.Join(out.RunDir, "manifest.json")); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
}

func TestRunCouncilDoctorIsolationPrintsPassJSON(t *testing.T) {
	t.Parallel()

	probes := []doctor.Probe{
		{Name: "claude", Runtime: doctorFakeRuntime{provider: councilruntime.ProviderClaude}},
		{Name: "codex", Runtime: doctorFakeRuntime{provider: councilruntime.ProviderCodex}},
	}

	var stdout, stderr bytes.Buffer
	code := runCouncilDoctorIsolationWithProbes(context.Background(), probes, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("doctor exit = %d, stderr = %s", code, stderr.String())
	}

	var report doctor.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v: %q", err, stdout.String())
	}
	if report.Gate != doctor.GatePass {
		t.Fatalf("gate = %q, want %q: %+v", report.Gate, doctor.GatePass, report)
	}
	if len(report.Providers) != 2 || !report.Providers[0].Pass || !report.Providers[1].Pass {
		t.Fatalf("unexpected provider report: %+v", report.Providers)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"council", "unknown"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("run() accepted unknown command")
	}
}
