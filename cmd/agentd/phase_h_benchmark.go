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
	"strings"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/adapterpool"
	"github.com/ShenJun93/agent-council/internal/council/benchmark"
	"github.com/ShenJun93/agent-council/internal/council/config"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/humanbroker"
	"github.com/ShenJun93/agent-council/internal/council/invocationlog"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/structuredoutput"
)

type phaseHExecutionRequest struct {
	Dataset       benchmark.Dataset
	ReplayDataset benchmark.PhaseHReplayDataset
	DatasetPath   string
	RunsRoot      string
	RunID         string
	TempRoot      string
}

type phaseHExecutor func(context.Context, phaseHExecutionRequest) (benchmark.PhaseHRunResult, error)

func runCouncilBenchmarkPhaseH(args []string, stdout, stderr io.Writer, execute phaseHExecutor) int {
	fs := flag.NewFlagSet("agentd council benchmark phase-h", flag.ContinueOnError)
	fs.SetOutput(stderr)
	datasetPath := fs.String("dataset", filepath.FromSlash("benchmarks/phase-h"), "path to frozen Phase H replay dataset")
	runsDir := fs.String("runs-dir", "", "override Phase H run artifact root")
	configPath := fs.String("config", "", "path to council.yaml")
	tempRoot := fs.String("temp-root", os.TempDir(), "temporary workspace root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "agentd council benchmark phase-h does not accept positional arguments")
		return 2
	}
	if execute == nil {
		_, _ = fmt.Fprintln(stderr, "Phase H benchmark executor is required")
		return 1
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load Phase H config: %v\n", err)
		return 1
	}
	replay, err := benchmark.LoadPhaseHReplay(*datasetPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load Phase H replay dataset: %v\n", err)
		return 1
	}
	resolvedRunsRoot := *runsDir
	if resolvedRunsRoot == "" {
		resolvedRunsRoot = cfg.Runs.Root
	}
	runID, err := newPhaseHRunID(time.Now().UTC())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate Phase H run id: %v\n", err)
		return 1
	}
	policy := benchmark.H5AdapterPolicy{
		SchemaVersion: replay.AdapterPolicy.SchemaVersion,
		Adapters:      replay.AdapterPolicy.Adapters,
		Slots:         replay.AdapterPolicy.Slots,
	}
	result, err := execute(context.Background(), phaseHExecutionRequest{
		Dataset: benchmark.Dataset{AdapterPolicy: &policy}, ReplayDataset: replay,
		DatasetPath: *datasetPath, RunsRoot: resolvedRunsRoot, RunID: runID, TempRoot: *tempRoot,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "run Phase H benchmark: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		_, _ = fmt.Fprintf(stderr, "write Phase H benchmark output: %v\n", err)
		return 1
	}
	return 0
}
func wrapPhaseHAdapter(inner councilruntime.AgentRuntime, id string, provider councilruntime.Provider) councilruntime.AgentRuntime {
	return structuredoutput.WrapProfile(
		invocationlog.WrapAdapter(inner, invocationlog.AdapterMetadata{ID: id, Provider: provider}),
		structuredoutput.SchemaProfileH8,
	)
}

type phaseHRegistry map[adapterpool.AdapterID]adapterpool.Adapter

func newPhaseHRegistry(req phaseHExecutionRequest) (phaseHRegistry, error) {
	if req.Dataset.AdapterPolicy == nil {
		return nil, fmt.Errorf("phase H adapter policy is required")
	}
	policy := req.Dataset.AdapterPolicy
	if len(policy.Adapters) != 1 {
		return nil, fmt.Errorf("phase H registry requires exactly one adapter")
	}
	desc := policy.Adapters[0]
	if desc.ID != humanbroker.DefaultAdapterID ||
		desc.ProviderFamily != string(councilruntime.ProviderChatGPT) ||
		desc.Transport != "human-chatgpt-session" ||
		desc.AuthClass != "chatgpt-subscription" ||
		desc.Interaction != "human-broker" || strings.TrimSpace(desc.Model) != "" {
		return nil, fmt.Errorf("phase H registry accepts only the current ChatGPT web session broker")
	}
	id := adapterpool.AdapterID(desc.ID)
	provider := councilruntime.ProviderChatGPT
	return phaseHRegistry{
		id: {
			ID: id, Provider: provider,
			Runtime: wrapPhaseHAdapter(&humanbroker.Runtime{UseCurrentSession: true}, desc.ID, provider),
		},
	}, nil
}

func newPhaseHPool(registry phaseHRegistry, slot string, chain []string) (councilruntime.AgentRuntime, error) {
	ids := make([]adapterpool.AdapterID, len(chain))
	for i, id := range chain {
		ids[i] = adapterpool.AdapterID(id)
	}
	return adapterpool.New(registry, adapterpool.Policy{Slot: adapterpool.SlotID(slot), Chain: ids})
}

func newPhaseHEvaluator(registry phaseHRegistry, policy benchmark.H5AdapterPolicy, tempRoot string) (benchmark.EvalExecutor, error) {
	j1, err := newPhaseHPool(registry, "eval-judge-1", policy.Slots["eval-judge-1"])
	if err != nil {
		return nil, err
	}
	j2, err := newPhaseHPool(registry, "eval-judge-2", policy.Slots["eval-judge-2"])
	if err != nil {
		return nil, err
	}
	return evalharness.H8Harness{
		Harness: evalharness.Harness{
			Adaptive: &evalharness.AdaptiveJudgeRuntimes{Judge1: j1, Judge2: j2},
			TempRoot: tempRoot, CitationContract: evalharness.CitationContractStructuredV3,
		},
	}, nil
}
func executePhaseHBenchmark(ctx context.Context, req phaseHExecutionRequest) (benchmark.PhaseHRunResult, error) {
	if req.Dataset.AdapterPolicy == nil {
		return benchmark.PhaseHRunResult{}, fmt.Errorf("phase H adapter policy is required")
	}
	registry, err := newPhaseHRegistry(req)
	if err != nil {
		return benchmark.PhaseHRunResult{}, err
	}
	evaluator, err := newPhaseHEvaluator(registry, *req.Dataset.AdapterPolicy, req.TempRoot)
	if err != nil {
		return benchmark.PhaseHRunResult{}, err
	}
	runner := benchmark.PhaseHReplayRunner{Evaluator: evaluator}
	return runner.Run(ctx, benchmark.PhaseHReplayRunRequest{
		Dataset: req.ReplayDataset, RunsRoot: req.RunsRoot, RunID: req.RunID,
	})
}

func newPhaseHRunID(now time.Time) (string, error) {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("random suffix: %w", err)
	}
	return "phase-h-" + now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix[:]), nil
}
