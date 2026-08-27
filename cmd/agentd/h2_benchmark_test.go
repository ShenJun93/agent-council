package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/benchmark"
)

func TestRunRoutesH2BenchmarkWithFrozenFlags(t *testing.T) {
	datasetPath := filepath.Join("..", "..", "benchmarks", "h2")
	calls := 0
	var got h2ExecutionRequest
	exec := func(_ context.Context, req h2ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		got = req
		return benchmark.RunResult{RunID: req.RunID, RunDir: filepath.Join(req.RunsRoot, req.RunID)}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithBenchmarkExecutors([]string{"council", "benchmark", "h2", "--dataset", datasetPath}, &stdout, &stderr, nil, exec)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("executor calls=%d", calls)
	}
	if got.DatasetPath != datasetPath || got.RunsRoot != ".council/runs" {
		t.Fatalf("unexpected H2 request: %+v", got)
	}
	if len(got.RunID) < 4 || got.RunID[:3] != "h2-" {
		t.Fatalf("run id=%q does not use h2 prefix", got.RunID)
	}
	if len(got.RunID) != 32 {
		t.Fatalf("run id=%q length=%d want 32", got.RunID, len(got.RunID))
	}
}
func TestRunH2BenchmarkRejectsFrozenPolicyOverride(t *testing.T) {
	calls := 0
	exec := func(_ context.Context, _ h2ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithBenchmarkExecutors(
		[]string{"council", "benchmark", "h2", "--material-worse-delta", "5"},
		&stdout, &stderr, nil, exec,
	)
	if code == 0 {
		t.Fatal("H2 accepted frozen-policy override")
	}
	if calls != 0 {
		t.Fatalf("executor calls=%d", calls)
	}
}
