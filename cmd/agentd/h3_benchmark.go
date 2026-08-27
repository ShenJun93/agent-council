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

type h3ExecutionRequest struct {
	Dataset     benchmark.Dataset
	DatasetPath string
	RunsRoot    string
	RunID       string
	TempRoot    string
	ClaudeBin   string
	CodexBin    string
}

type h3Executor func(context.Context, h3ExecutionRequest) (benchmark.RunResult, error)

func runCouncilBenchmarkH3(args []string, stdout, stderr io.Writer, execute h3Executor) int {
	fs := flag.NewFlagSet("agentd council benchmark h3", flag.ContinueOnError)
	fs.SetOutput(stderr)
	datasetPath := fs.String("dataset", filepath.FromSlash("benchmarks/h3"), "path to frozen H3 dataset")
	runsDir := fs.String("runs-dir", "", "override H3 run artifact root")
	configPath := fs.String("config", "", "path to council.yaml")
	tempRoot := fs.String("temp-root", os.TempDir(), "temporary workspace root")
	claudeBin := fs.String("claude-bin", "claude", "Claude Code CLI binary")
	codexBin := fs.String("codex-bin", "codex", "Codex CLI binary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "agentd council benchmark h3 does not accept positional arguments")
		return 2
	}
	if execute == nil {
		_, _ = fmt.Fprintln(stderr, "H3 benchmark executor is required")
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H3 config: %v\n", err)
		return 1
	}
	dataset, err := benchmark.LoadH3(*datasetPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H3 dataset: %v\n", err)
		return 1
	}
	resolvedRunsRoot := *runsDir
	if resolvedRunsRoot == "" {
		resolvedRunsRoot = cfg.Runs.Root
	}
	runID, err := newH3RunID(time.Now().UTC())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate H3 run id: %v\n", err)
		return 1
	}
	result, err := execute(context.Background(), h3ExecutionRequest{
		Dataset: dataset, DatasetPath: *datasetPath, RunsRoot: resolvedRunsRoot,
		RunID: runID, TempRoot: *tempRoot, ClaudeBin: *claudeBin, CodexBin: *codexBin,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "run H3 benchmark: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		_, _ = fmt.Fprintf(stderr, "write H3 benchmark output: %v\n", err)
		return 1
	}
	return 0
}

func newH3Baseline(claude, codex councilruntime.AgentRuntime, tempRoot string, provider councilruntime.Provider) benchmark.H3BaselineExecutor {
	return baseline.Runner{
		Claude: claude, Codex: codex, TempRoot: tempRoot,
		ChallengerProvider: provider, ChallengePolicy: benchmark.H3ChallengePolicy,
		CitationAuthority: baseline.CitationAuthorityProblemOnlyFinal,
	}
}

func executeH3Benchmark(ctx context.Context, req h3ExecutionRequest) (benchmark.RunResult, error) {
	claude := invocationlog.Wrap(councilruntime.NewClaudeCLI(req.ClaudeBin), councilruntime.ProviderClaude)
	codex := invocationlog.Wrap(councilruntime.NewCodexCLI(req.CodexBin), councilruntime.ProviderCodex)
	evaluator := evalharness.Harness{Claude: claude, Codex: codex, TempRoot: req.TempRoot}
	runner := benchmark.H3Runner{
		NewBaseline: func(provider councilruntime.Provider) benchmark.H3BaselineExecutor {
			return newH3Baseline(claude, codex, req.TempRoot, provider)
		},
		Evaluator: evaluator,
	}
	return runner.Run(ctx, benchmark.RunRequest{Dataset: req.Dataset, RunsRoot: req.RunsRoot, RunID: req.RunID})
}

func newH3RunID(now time.Time) (string, error) {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("random suffix: %w", err)
	}
	return "h3-" + now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix[:]), nil
}
