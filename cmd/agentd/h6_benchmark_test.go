package main

import (
	"bytes"
	"context"
	"github.com/ShenJun93/agent-council/internal/council/benchmark"
	"path/filepath"
	"testing"
)

func TestRunRoutesH6BenchmarkWithFrozenFlags(t *testing.T) {
	datasetPath := filepath.Join("..", "..", "benchmarks", "h6")
	calls := 0
	var got h6ExecutionRequest
	exec := func(_ context.Context, req h6ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		got = req
		return benchmark.RunResult{RunID: req.RunID, RunDir: filepath.Join(req.RunsRoot, req.RunID)}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH6BenchmarkExecutors([]string{"council", "benchmark", "h6", "--dataset", datasetPath}, &stdout, &stderr, nil, nil, nil, nil, nil, exec)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if got.DatasetPath != datasetPath || got.RunsRoot != ".council/runs" {
		t.Fatalf("request=%+v", got)
	}
	if len(got.RunID) < 4 || got.RunID[:3] != "h6-" {
		t.Fatalf("run id=%q", got.RunID)
	}
}

func TestRunH6BenchmarkRejectsFrozenPolicyOverride(t *testing.T) {
	calls := 0
	exec := func(_ context.Context, _ h6ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH6BenchmarkExecutors([]string{"council", "benchmark", "h6", "--material-worse-delta", "5"}, &stdout, &stderr, nil, nil, nil, nil, nil, exec)
	if code == 0 || calls != 0 {
		t.Fatalf("exit=%d calls=%d", code, calls)
	}
}
