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
	"github.com/ShenJun93/agent-council/internal/council/structuredoutput"
)

type h4ExecutionRequest struct {
	Dataset     benchmark.Dataset
	DatasetPath string
	RunsRoot    string
	RunID       string
	TempRoot    string
	ClaudeBin   string
	CodexBin    string
}

type h4Executor func(context.Context, h4ExecutionRequest) (benchmark.RunResult, error)

func runCouncilBenchmarkH4(args []string, stdout, stderr io.Writer, execute h4Executor) int {
	fs := flag.NewFlagSet("agentd council benchmark h4", flag.ContinueOnError)
	fs.SetOutput(stderr)
	datasetPath := fs.String("dataset", filepath.FromSlash("benchmarks/h4"), "path to frozen H4 dataset")
	runsDir := fs.String("runs-dir", "", "override H4 run artifact root")
	configPath := fs.String("config", "", "path to council.yaml")
	tempRoot := fs.String("temp-root", os.TempDir(), "temporary workspace root")
	claudeBin := fs.String("claude-bin", "claude", "Claude Code CLI binary")
	codexBin := fs.String("codex-bin", "codex", "Codex CLI binary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "agentd council benchmark h4 does not accept positional arguments")
		return 2
	}
	if execute == nil {
		_, _ = fmt.Fprintln(stderr, "H4 benchmark executor is required")
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H4 config: %v\n", err)
		return 1
	}
	dataset, err := benchmark.LoadH4(*datasetPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H4 dataset: %v\n", err)
		return 1
	}
	resolvedRunsRoot := *runsDir
	if resolvedRunsRoot == "" {
		resolvedRunsRoot = cfg.Runs.Root
	}
	runID, err := newH4RunID(time.Now().UTC())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate H4 run id: %v\n", err)
		return 1
	}
	result, err := execute(context.Background(), h4ExecutionRequest{
		Dataset: dataset, DatasetPath: *datasetPath, RunsRoot: resolvedRunsRoot,
		RunID: runID, TempRoot: *tempRoot, ClaudeBin: *claudeBin, CodexBin: *codexBin,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "run H4 benchmark: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		_, _ = fmt.Fprintf(stderr, "write H4 benchmark output: %v\n", err)
		return 1
	}
	return 0
}

func newH4Baseline(claude, codex councilruntime.AgentRuntime, tempRoot string, provider councilruntime.Provider) benchmark.H4BaselineExecutor {
	return baseline.Runner{
		Claude: claude, Codex: codex, TempRoot: tempRoot,
		ChallengerProvider: provider, ChallengePolicy: benchmark.H4ChallengePolicy,
		CitationAuthority: baseline.CitationAuthorityProblemOnlyFinal,
	}
}

func wrapH4Runtime(inner councilruntime.AgentRuntime, provider councilruntime.Provider) councilruntime.AgentRuntime {
	return structuredoutput.Wrap(invocationlog.Wrap(inner, provider))
}

func executeH4Benchmark(ctx context.Context, req h4ExecutionRequest) (benchmark.RunResult, error) {
	claude := wrapH4Runtime(councilruntime.NewClaudeCLI(req.ClaudeBin), councilruntime.ProviderClaude)
	codex := wrapH4Runtime(councilruntime.NewCodexCLI(req.CodexBin), councilruntime.ProviderCodex)
	evaluator := evalharness.Harness{Claude: claude, Codex: codex, TempRoot: req.TempRoot}
	runner := benchmark.H4Runner{
		NewBaseline: func(provider councilruntime.Provider) benchmark.H4BaselineExecutor {
			return newH4Baseline(claude, codex, req.TempRoot, provider)
		},
		Evaluator: evaluator,
	}
	return runner.Run(ctx, benchmark.RunRequest{Dataset: req.Dataset, RunsRoot: req.RunsRoot, RunID: req.RunID})
}

func newH4RunID(now time.Time) (string, error) {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("random suffix: %w", err)
	}
	return "h4-" + now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix[:]), nil
}
