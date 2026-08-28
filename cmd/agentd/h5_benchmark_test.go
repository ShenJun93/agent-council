package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/benchmark"
)

func TestRunRoutesH5BenchmarkWithFrozenFlags(t *testing.T) {
	datasetPath := filepath.Join("..", "..", "benchmarks", "h5")
	calls := 0
	var got h5ExecutionRequest
	exec := func(_ context.Context, req h5ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		got = req
		return benchmark.RunResult{RunID: req.RunID, RunDir: filepath.Join(req.RunsRoot, req.RunID)}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH5BenchmarkExecutors([]string{"council", "benchmark", "h5", "--dataset", datasetPath}, &stdout, &stderr, nil, nil, nil, nil, exec)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("executor calls=%d", calls)
	}
	if got.DatasetPath != datasetPath || got.RunsRoot != ".council/runs" {
		t.Fatalf("unexpected H5 request: %+v", got)
	}
	if len(got.RunID) < 4 || got.RunID[:3] != "h5-" {
		t.Fatalf("run id=%q does not use h5 prefix", got.RunID)
	}
	if len(got.RunID) != 32 {
		t.Fatalf("run id=%q length=%d want 32", got.RunID, len(got.RunID))
	}
}
func TestRunH5BenchmarkRejectsFrozenPolicyOverride(t *testing.T) {
	calls := 0
	exec := func(_ context.Context, _ h5ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH5BenchmarkExecutors(
		[]string{"council", "benchmark", "h5", "--material-worse-delta", "5"},
		&stdout, &stderr, nil, nil, nil, nil, exec,
	)
	if code == 0 {
		t.Fatal("H5 accepted frozen-policy override")
	}
	if calls != 0 {
		t.Fatalf("executor calls=%d", calls)
	}
}
