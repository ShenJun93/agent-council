package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/benchmark"
)

func TestRunRoutesH7BenchmarkWithFrozenFlags(t *testing.T) {
	datasetPath := filepath.Join("..", "..", "benchmarks", "h7")
	calls := 0
	var got h7ExecutionRequest
	exec := func(_ context.Context, req h7ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		got = req
		return benchmark.RunResult{RunID: req.RunID, RunDir: filepath.Join(req.RunsRoot, req.RunID)}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH7BenchmarkExecutors([]string{"council", "benchmark", "h7", "--dataset", datasetPath}, &stdout, &stderr, nil, nil, nil, nil, nil, nil, exec)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if got.DatasetPath != datasetPath || got.RunsRoot != ".council/runs" {
		t.Fatalf("request=%+v", got)
	}
	if len(got.RunID) < 4 || got.RunID[:3] != "h7-" {
		t.Fatalf("run id=%q", got.RunID)
	}
}

func TestNewH7RunIDIsFreshAtSameTimestamp(t *testing.T) {
	now := time.Date(2026, 8, 29, 8, 0, 0, 0, time.UTC)
	first, err := newH7RunID(now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newH7RunID(now)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("run ids collided: %q", first)
	}
	if first[:3] != "h7-" || second[:3] != "h7-" {
		t.Fatalf("run ids=%q %q", first, second)
	}
}

func TestRunH7BenchmarkRejectsFrozenPolicyOverride(t *testing.T) {
	calls := 0
	exec := func(_ context.Context, _ h7ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH7BenchmarkExecutors([]string{"council", "benchmark", "h7", "--material-worse-delta", "5"}, &stdout, &stderr, nil, nil, nil, nil, nil, nil, exec)
	if code == 0 || calls != 0 {
		t.Fatalf("exit=%d calls=%d", code, calls)
	}
}

func TestRunH7BenchmarkRejectsPositionalArgs(t *testing.T) {
	calls := 0
	exec := func(_ context.Context, _ h7ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH7BenchmarkExecutors([]string{"council", "benchmark", "h7", "unexpected"}, &stdout, &stderr, nil, nil, nil, nil, nil, nil, exec)
	if code == 0 || calls != 0 {
		t.Fatalf("exit=%d calls=%d", code, calls)
	}
}
