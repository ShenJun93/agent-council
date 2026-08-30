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

	"github.com/ShenJun93/agent-council/internal/council/adapterpool"
	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/benchmark"
	"github.com/ShenJun93/agent-council/internal/council/config"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/humanbroker"
	"github.com/ShenJun93/agent-council/internal/council/invocationlog"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/structuredoutput"
)

type h8ExecutionRequest struct {
	Dataset        benchmark.Dataset
	DatasetPath    string
	RunsRoot       string
	RunID          string
	TempRoot       string
	ClaudeBin      string
	CodexBin       string
	AntigravityBin string
}

type h8Executor func(context.Context, h8ExecutionRequest) (benchmark.RunResult, error)

func runCouncilBenchmarkH8(args []string, stdout, stderr io.Writer, execute h8Executor) int {
	fs := flag.NewFlagSet("agentd council benchmark h8", flag.ContinueOnError)
	fs.SetOutput(stderr)
	datasetPath := fs.String("dataset", filepath.FromSlash("benchmarks/h8"), "path to frozen H8 dataset")
	runsDir := fs.String("runs-dir", "", "override H8 run artifact root")
	configPath := fs.String("config", "", "path to council.yaml")
	tempRoot := fs.String("temp-root", os.TempDir(), "temporary workspace root")
	claudeBin := fs.String("claude-bin", "claude", "Claude Code CLI binary")
	codexBin := fs.String("codex-bin", "codex", "Codex CLI binary")
	antigravityBin := fs.String("antigravity-bin", "agy", "Google Antigravity CLI binary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "agentd council benchmark h8 does not accept positional arguments")
		return 2
	}
	if execute == nil {
		_, _ = fmt.Fprintln(stderr, "H8 benchmark executor is required")
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H8 config: %v\n", err)
		return 1
	}
	dataset, err := benchmark.LoadH8(*datasetPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H8 dataset: %v\n", err)
		return 1
	}
	resolvedRunsRoot := *runsDir
	if resolvedRunsRoot == "" {
		resolvedRunsRoot = cfg.Runs.Root
	}
	runID, err := newH8RunID(time.Now().UTC())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate H8 run id: %v\n", err)
		return 1
	}
	result, err := execute(context.Background(), h8ExecutionRequest{
		Dataset: dataset, DatasetPath: *datasetPath, RunsRoot: resolvedRunsRoot,
		RunID: runID, TempRoot: *tempRoot, ClaudeBin: *claudeBin, CodexBin: *codexBin, AntigravityBin: *antigravityBin,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "run H8 benchmark: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		_, _ = fmt.Fprintf(stderr, "write H8 benchmark output: %v\n", err)
		return 1
	}
	return 0
}

func wrapH8Adapter(inner councilruntime.AgentRuntime, id string, provider councilruntime.Provider) councilruntime.AgentRuntime {
	return structuredoutput.WrapProfile(invocationlog.WrapAdapter(inner, invocationlog.AdapterMetadata{ID: id, Provider: provider}), structuredoutput.SchemaProfileH8)
}

type h8Registry map[adapterpool.AdapterID]adapterpool.Adapter

func newH8Registry(req h8ExecutionRequest) (h8Registry, error) {
	if req.Dataset.AdapterPolicy == nil {
		return nil, fmt.Errorf("H8 adapter policy is required")
	}
	registry := h8Registry{}
	for _, desc := range req.Dataset.AdapterPolicy.Adapters {
		var provider councilruntime.Provider
		var inner councilruntime.AgentRuntime
		switch desc.ID {
		case "claude-max":
			provider, inner = councilruntime.ProviderClaude, councilruntime.NewClaudeCLI(req.ClaudeBin)
		case "codex-chatgpt":
			provider, inner = councilruntime.ProviderCodex, councilruntime.NewCodexCLI(req.CodexBin)
		case "antigravity-subscription":
			provider, inner = councilruntime.ProviderAntigravity, councilruntime.NewAntigravityCLI(req.AntigravityBin, desc.Model)
		case humanbroker.DefaultAdapterID:
			provider, inner = councilruntime.ProviderChatGPT, &humanbroker.Runtime{}
		default:
			return nil, fmt.Errorf("unsupported H8 adapter %q", desc.ID)
		}
		if string(provider) != desc.ProviderFamily {
			return nil, fmt.Errorf("adapter %q provider family %q does not match %q", desc.ID, desc.ProviderFamily, provider)
		}
		id := adapterpool.AdapterID(desc.ID)
		registry[id] = adapterpool.Adapter{ID: id, Provider: provider, Runtime: wrapH8Adapter(inner, desc.ID, provider)}
	}
	return registry, nil
}

func newH8Pool(registry h8Registry, slot string, chain []string) (councilruntime.AgentRuntime, error) {
	ids := make([]adapterpool.AdapterID, len(chain))
	for i, id := range chain {
		ids[i] = adapterpool.AdapterID(id)
	}
	return adapterpool.New(registry, adapterpool.Policy{Slot: adapterpool.SlotID(slot), Chain: ids})
}

func newH8Baseline(registry h8Registry, policy benchmark.H5AdapterPolicy, tempRoot string, problem benchmark.Case) (benchmark.H8BaselineExecutor, error) {
	pool := func(slot string, chain []string) (councilruntime.AgentRuntime, error) {
		return newH8Pool(registry, slot, chain)
	}
	a, err := pool("baseline-a", policy.Slots["baseline-a"])
	if err != nil {
		return nil, err
	}
	b, err := pool("baseline-b", policy.Slots["baseline-b"])
	if err != nil {
		return nil, err
	}
	r1, err := pool("researcher-1", policy.Slots["researcher-1"])
	if err != nil {
		return nil, err
	}
	r2, err := pool("researcher-2", policy.Slots["researcher-2"])
	if err != nil {
		return nil, err
	}
	rv1, err := pool("reviewer-1", policy.Slots["reviewer-1"])
	if err != nil {
		return nil, err
	}
	rv2, err := pool("reviewer-2", policy.Slots["reviewer-2"])
	if err != nil {
		return nil, err
	}
	ch, err := pool("challenger", policy.ChallengerByCase[problem.ID])
	if err != nil {
		return nil, err
	}
	j1, err := pool("judge-1", policy.Slots["judge-1"])
	if err != nil {
		return nil, err
	}
	j2, err := pool("judge-2", policy.Slots["judge-2"])
	if err != nil {
		return nil, err
	}
	slots := &protocol.SlotRuntimes{Researcher1: r1, Researcher2: r2, Reviewer1: rv1, Reviewer2: rv2, Challenger: ch, Judge1: j1, Judge2: j2}
	return baseline.Runner{SlotA: a, SlotB: b, CouncilSlots: slots, TempRoot: tempRoot, ChallengePolicy: benchmark.H8ChallengePolicy, CitationAuthority: baseline.CitationAuthorityProblemOnlyFinal}, nil
}

func newH8Evaluator(registry h8Registry, policy benchmark.H5AdapterPolicy, tempRoot string) (benchmark.EvalExecutor, error) {
	j1, err := newH8Pool(registry, "eval-judge-1", policy.Slots["eval-judge-1"])
	if err != nil {
		return nil, err
	}
	j2, err := newH8Pool(registry, "eval-judge-2", policy.Slots["eval-judge-2"])
	if err != nil {
		return nil, err
	}
	return evalharness.Harness{Adaptive: &evalharness.AdaptiveJudgeRuntimes{Judge1: j1, Judge2: j2}, TempRoot: tempRoot, CitationContract: evalharness.CitationContractStructuredV3}, nil
}

func executeH8Benchmark(ctx context.Context, req h8ExecutionRequest) (benchmark.RunResult, error) {
	registry, err := newH8Registry(req)
	if err != nil {
		return benchmark.RunResult{}, err
	}
	policy := *req.Dataset.AdapterPolicy
	evaluator, err := newH8Evaluator(registry, policy, req.TempRoot)
	if err != nil {
		return benchmark.RunResult{}, err
	}
	runner := benchmark.H8Runner{NewBaseline: func(problem benchmark.Case) (benchmark.H8BaselineExecutor, error) {
		return newH8Baseline(registry, policy, req.TempRoot, problem)
	}, Evaluator: evaluator}
	return runner.Run(ctx, benchmark.RunRequest{Dataset: req.Dataset, RunsRoot: req.RunsRoot, RunID: req.RunID})
}

func newH8RunID(now time.Time) (string, error) {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("random suffix: %w", err)
	}
	return "h8-" + now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix[:]), nil
}
