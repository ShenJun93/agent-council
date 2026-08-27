package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/benchmark"
)

func TestRunH4BenchmarkRejectsMeteredFallbackBeforeExecution(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "council.yaml")
	configBody := "runs:\n  root: .council/runs\nbilling:\n  mode: subscription_only\n  fail_closed: true\n  allow_metered_fallback: true\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	exec := func(_ context.Context, _ h4ExecutionRequest) (benchmark.RunResult, error) {
		calls++
		return benchmark.RunResult{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runWithAllBenchmarkExecutors([]string{"council", "benchmark", "h4", "--config", configPath}, &stdout, &stderr, nil, nil, nil, exec)
	if code == 0 {
		t.Fatal("H4 accepted metered fallback config")
	}
	if calls != 0 {
		t.Fatalf("executor calls=%d", calls)
	}
}
