package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/benchmark"
)

func TestRunH5AcceptsAntigravityBinaryWithoutProviderPolicyOverride(t *testing.T) {
	datasetPath := filepath.Join("..", "..", "benchmarks", "h5")
	var got h5ExecutionRequest
	exec := func(_ context.Context, req h5ExecutionRequest) (benchmark.RunResult, error) {
		got = req
		return benchmark.RunResult{RunID: req.RunID}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithH5BenchmarkExecutors([]string{"council", "benchmark", "h5", "--dataset", datasetPath, "--antigravity-bin", "agy-test"}, &stdout, &stderr, nil, nil, nil, nil, exec)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if got.AntigravityBin != "agy-test" {
		t.Fatalf("antigravity bin=%q", got.AntigravityBin)
	}
}
