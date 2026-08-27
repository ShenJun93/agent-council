package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/benchmark"
	"github.com/ShenJun93/agent-council/internal/council/config"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/invocationlog"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type h2ExecutionRequest struct {
	Dataset     benchmark.Dataset
	DatasetPath string
	RunsRoot    string
	RunID       string
	TempRoot    string
	ClaudeBin   string
	CodexBin    string
}

type h2Executor func(context.Context, h2ExecutionRequest) (benchmark.RunResult, error)

func runCouncilBenchmarkH2(args []string, stdout, stderr io.Writer, execute h2Executor) int {
	fs := flag.NewFlagSet("agentd council benchmark h2", flag.ContinueOnError)
	fs.SetOutput(stderr)
	datasetPath := fs.String("dataset", filepath.FromSlash("benchmarks/h2"), "path to frozen H2 dataset")
	runsDir := fs.String("runs-dir", "", "override H2 run artifact root")
	configPath := fs.String("config", "", "path to council.yaml")
	tempRoot := fs.String("temp-root", os.TempDir(), "temporary workspace root")
	claudeBin := fs.String("claude-bin", "claude", "Claude Code CLI binary")
	codexBin := fs.String("codex-bin", "codex", "Codex CLI binary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "agentd council benchmark h2 does not accept positional arguments")
		return 2
	}
	if execute == nil {
		_, _ = fmt.Fprintln(stderr, "H2 benchmark executor is required")
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H2 config: %v\n", err)
		return 1
	}
	dataset, err := benchmark.LoadH2(*datasetPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H2 dataset: %v\n", err)
		return 1
	}
	resolvedRunsRoot := *runsDir
	if resolvedRunsRoot == "" {
		resolvedRunsRoot = cfg.Runs.Root
	}
	runID, err := newH2RunID(time.Now().UTC())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate H2 run id: %v\n", err)
		return 1
	}
	result, err := execute(context.Background(), h2ExecutionRequest{
		Dataset: dataset, DatasetPath: *datasetPath, RunsRoot: resolvedRunsRoot,
		RunID: runID, TempRoot: *tempRoot, ClaudeBin: *claudeBin, CodexBin: *codexBin,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "run H2 benchmark: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		_, _ = fmt.Fprintf(stderr, "write H2 benchmark output: %v\n", err)
		return 1
	}
	return 0
}

func executeH2Benchmark(ctx context.Context, req h2ExecutionRequest) (benchmark.RunResult, error) {
	claude := invocationlog.Wrap(councilruntime.NewClaudeCLI(req.ClaudeBin), councilruntime.ProviderClaude)
	codex := invocationlog.Wrap(councilruntime.NewCodexCLI(req.CodexBin), councilruntime.ProviderCodex)
	evaluator := evalharness.Harness{Claude: claude, Codex: codex, TempRoot: req.TempRoot}
	runner := benchmark.H2Runner{
		NewBaseline: func(provider councilruntime.Provider) benchmark.H2BaselineExecutor {
			return baseline.Runner{
				Claude: claude, Codex: codex, TempRoot: req.TempRoot,
				ChallengerProvider: provider, ChallengePolicy: benchmark.H2ChallengePolicy,
			}
		},
		Evaluator: evaluator,
	}
	return runner.Run(ctx, benchmark.RunRequest{Dataset: req.Dataset, RunsRoot: req.RunsRoot, RunID: req.RunID})
}

func newH2RunID(now time.Time) (string, error) {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("random suffix: %w", err)
	}
	return "h2-" + now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix[:]), nil
}
