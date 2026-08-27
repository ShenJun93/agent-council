package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/benchmark"
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

func TestRunRoutesH1BenchmarkWithFrozenDefaults(t *testing.T) {
	t.Parallel()

	datasetPath := filepath.Join("..", "..", "benchmarks", "h1")
	calls := 0
	var got h1ExecutionRequest
	exec := func(_ context.Context, req h1ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		got = req
		return benchmark.RunResult{RunID: req.RunID, RunDir: filepath.Join(req.RunsRoot, req.RunID)}, nil
	}

	var stdout, stderr bytes.Buffer
	code := runWithH1Executor([]string{"council", "benchmark", "h1", "--dataset", datasetPath}, &stdout, &stderr, exec)
	if code != 0 {
		t.Fatalf("benchmark exit=%d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("executor calls=%d, want 1", calls)
	}
	if got.DatasetPath != datasetPath {
		t.Fatalf("dataset path=%q want %q", got.DatasetPath, datasetPath)
	}
	if got.RunsRoot != ".council/runs" {
		t.Fatalf("runs root=%q want .council/runs", got.RunsRoot)
	}
	if got.TempRoot != os.TempDir() || got.ClaudeBin != "claude" || got.CodexBin != "codex" {
		t.Fatalf("unexpected defaults: %+v", got)
	}
	if len(got.RunID) <= len("h1-") || got.RunID[:3] != "h1-" {
		t.Fatalf("run id=%q does not use h1 prefix", got.RunID)
	}

	var out benchmark.RunResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not JSON: %v: %q", err, stdout.String())
	}
	if out.RunID != got.RunID {
		t.Fatalf("output run id=%q want %q", out.RunID, got.RunID)
	}
}

func TestRunH1BenchmarkRejectsMeteredFallbackConfigBeforeExecution(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, "council.yaml")
	configBody := "runs:\n  root: .council/runs\nbilling:\n  mode: subscription_only\n  fail_closed: true\n  allow_metered_fallback: true\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	calls := 0
	exec := func(_ context.Context, _ h1ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH1Executor([]string{"council", "benchmark", "h1", "--config", configPath}, &stdout, &stderr, exec)
	if code == 0 {
		t.Fatal("benchmark accepted metered fallback config")
	}
	if calls != 0 {
		t.Fatalf("executor called %d times before config rejection", calls)
	}
}

func TestRunH1BenchmarkRejectsFrozenPolicyOverrideFlag(t *testing.T) {
	t.Parallel()

	calls := 0
	exec := func(_ context.Context, _ h1ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH1Executor([]string{"council", "benchmark", "h1", "--material-worse-delta", "5"}, &stdout, &stderr, exec)
	if code == 0 {
		t.Fatal("benchmark accepted frozen-policy override flag")
	}
	if calls != 0 {
		t.Fatalf("executor called %d times after unknown flag", calls)
	}
}

func TestRunH1BenchmarkRejectsUnknownSubcommand(t *testing.T) {
	t.Parallel()

	calls := 0
	exec := func(_ context.Context, _ h1ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH1Executor([]string{"council", "benchmark", "h2"}, &stdout, &stderr, exec)
	if code == 0 {
		t.Fatal("benchmark accepted unknown benchmark version")
	}
	if calls != 0 {
		t.Fatalf("executor called %d times for unknown benchmark", calls)
	}
}
