package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/benchmark"
)

func TestRunRoutesH4BenchmarkWithFrozenFlags(t *testing.T) {
	datasetPath := filepath.Join("..", "..", "benchmarks", "h4")
	calls := 0
	var got h4ExecutionRequest
	exec := func(_ context.Context, req h4ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		got = req
		return benchmark.RunResult{RunID: req.RunID, RunDir: filepath.Join(req.RunsRoot, req.RunID)}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithAllBenchmarkExecutors([]string{"council", "benchmark", "h4", "--dataset", datasetPath}, &stdout, &stderr, nil, nil, nil, exec)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("executor calls=%d", calls)
	}
	if got.DatasetPath != datasetPath || got.RunsRoot != ".council/runs" {
		t.Fatalf("unexpected H4 request: %+v", got)
	}
	if len(got.RunID) < 4 || got.RunID[:3] != "h4-" {
		t.Fatalf("run id=%q does not use h4 prefix", got.RunID)
	}
	if len(got.RunID) != 32 {
		t.Fatalf("run id=%q length=%d want 32", got.RunID, len(got.RunID))
	}
}
func TestRunH4BenchmarkRejectsFrozenPolicyOverride(t *testing.T) {
	calls := 0
	exec := func(_ context.Context, _ h4ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithAllBenchmarkExecutors(
		[]string{"council", "benchmark", "h4", "--material-worse-delta", "5"},
		&stdout, &stderr, nil, nil, nil, exec,
	)
	if code == 0 {
		t.Fatal("H4 accepted frozen-policy override")
	}
	if calls != 0 {
		t.Fatalf("executor calls=%d", calls)
	}
}
