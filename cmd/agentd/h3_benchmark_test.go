package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/benchmark"
)

func TestRunRoutesH3BenchmarkWithFrozenFlags(t *testing.T) {
	datasetPath := filepath.Join("..", "..", "benchmarks", "h3")
	calls := 0
	var got h3ExecutionRequest
	exec := func(_ context.Context, req h3ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		got = req
		return benchmark.RunResult{RunID: req.RunID, RunDir: filepath.Join(req.RunsRoot, req.RunID)}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithAllBenchmarkExecutors([]string{"council", "benchmark", "h3", "--dataset", datasetPath}, &stdout, &stderr, nil, nil, exec)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("executor calls=%d", calls)
	}
	if got.DatasetPath != datasetPath || got.RunsRoot != ".council/runs" {
		t.Fatalf("unexpected H3 request: %+v", got)
	}
	if len(got.RunID) < 4 || got.RunID[:3] != "h3-" {
		t.Fatalf("run id=%q does not use h3 prefix", got.RunID)
	}
	if len(got.RunID) != 32 {
		t.Fatalf("run id=%q length=%d want 32", got.RunID, len(got.RunID))
	}
}
func TestRunH3BenchmarkRejectsFrozenPolicyOverride(t *testing.T) {
	calls := 0
	exec := func(_ context.Context, _ h3ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithAllBenchmarkExecutors(
		[]string{"council", "benchmark", "h3", "--material-worse-delta", "5"},
		&stdout, &stderr, nil, nil, exec,
	)
	if code == 0 {
		t.Fatal("H3 accepted frozen-policy override")
	}
	if calls != 0 {
		t.Fatalf("executor calls=%d", calls)
	}
}
