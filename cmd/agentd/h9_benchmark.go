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

type h9ExecutionRequest struct {
	Dataset       benchmark.Dataset
	ReplayDataset benchmark.H9ReplayDataset
	DatasetPath   string
	RunsRoot      string
	RunID         string
	TempRoot      string
}

type h9Executor func(context.Context, h9ExecutionRequest) (benchmark.RunResult, error)

func runCouncilBenchmarkH9(args []string, stdout, stderr io.Writer, execute h9Executor) int {
	fs := flag.NewFlagSet("agentd council benchmark h9", flag.ContinueOnError)
	fs.SetOutput(stderr)
	datasetPath := fs.String("dataset", filepath.FromSlash("benchmarks/h9"), "path to frozen H9 replay dataset")
	runsDir := fs.String("runs-dir", "", "override H9 run artifact root")
	configPath := fs.String("config", "", "path to council.yaml")
	tempRoot := fs.String("temp-root", os.TempDir(), "temporary workspace root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "agentd council benchmark h9 does not accept positional arguments")
		return 2
	}
	if execute == nil {
		_, _ = fmt.Fprintln(stderr, "H9 benchmark executor is required")
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H9 config: %v\n", err)
		return 1
	}
	replay, err := benchmark.LoadH9Replay(*datasetPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H9 replay dataset: %v\n", err)
		return 1
	}
	resolvedRunsRoot := *runsDir
	if resolvedRunsRoot == "" {
		resolvedRunsRoot = cfg.Runs.Root
	}
	runID, err := newH9RunID(time.Now().UTC())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate H9 run id: %v\n", err)
		return 1
	}
	policy := benchmark.H5AdapterPolicy{
		SchemaVersion: replay.AdapterPolicy.SchemaVersion,
		Adapters:      replay.AdapterPolicy.Adapters,
		Slots:         replay.AdapterPolicy.Slots,
	}
	result, err := execute(context.Background(), h9ExecutionRequest{
		Dataset:       benchmark.Dataset{AdapterPolicy: &policy},
		ReplayDataset: replay,
		DatasetPath:   *datasetPath,
		RunsRoot:      resolvedRunsRoot,
		RunID:         runID,
		TempRoot:      *tempRoot,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "run H9 benchmark: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		_, _ = fmt.Fprintf(stderr, "write H9 benchmark output: %v\n", err)
		return 1
	}
	return 0
}

func wrapH9Adapter(inner councilruntime.AgentRuntime, id string, provider councilruntime.Provider) councilruntime.AgentRuntime {
	return structuredoutput.WrapProfile(
		invocationlog.WrapAdapter(inner, invocationlog.AdapterMetadata{ID: id, Provider: provider}),
		structuredoutput.SchemaProfileH8,
	)
}

type h9Registry map[adapterpool.AdapterID]adapterpool.Adapter

func newH9Registry(req h9ExecutionRequest) (h9Registry, error) {
	if req.Dataset.AdapterPolicy == nil {
		return nil, fmt.Errorf("H9 adapter policy is required")
	}
	policy := req.Dataset.AdapterPolicy
	if len(policy.Adapters) != 1 {
		return nil, fmt.Errorf("H9 registry requires exactly one adapter")
	}
	desc := policy.Adapters[0]
	if desc.ID != humanbroker.DefaultAdapterID ||
		desc.ProviderFamily != string(councilruntime.ProviderChatGPT) ||
		desc.Transport != "human-chatgpt-session" ||
		desc.AuthClass != "chatgpt-subscription" ||
		desc.Interaction != "human-broker" ||
		strings.TrimSpace(desc.Model) != "" {
		return nil, fmt.Errorf("H9 registry accepts only the ChatGPT web human broker")
	}
	id := adapterpool.AdapterID(desc.ID)
	provider := councilruntime.ProviderChatGPT
	return h9Registry{
		id: {
			ID:       id,
			Provider: provider,
			Runtime:  wrapH9Adapter(&humanbroker.Runtime{CurrentSession: true}, desc.ID, provider),
		},
	}, nil
}

func newH9Pool(registry h9Registry, slot string, chain []string) (councilruntime.AgentRuntime, error) {
	ids := make([]adapterpool.AdapterID, len(chain))
	for i, id := range chain {
		ids[i] = adapterpool.AdapterID(id)
	}
	return adapterpool.New(registry, adapterpool.Policy{Slot: adapterpool.SlotID(slot), Chain: ids})
}

func newH9Evaluator(registry h9Registry, policy benchmark.H5AdapterPolicy, tempRoot string) (benchmark.EvalExecutor, error) {
	j1, err := newH9Pool(registry, "eval-judge-1", policy.Slots["eval-judge-1"])
	if err != nil {
		return nil, err
	}
	j2, err := newH9Pool(registry, "eval-judge-2", policy.Slots["eval-judge-2"])
	if err != nil {
		return nil, err
	}
	return evalharness.H8Harness{
		Harness: evalharness.Harness{
			Adaptive:         &evalharness.AdaptiveJudgeRuntimes{Judge1: j1, Judge2: j2},
			TempRoot:         tempRoot,
			CitationContract: evalharness.CitationContractStructuredV3,
		},
	}, nil
}

func executeH9Benchmark(ctx context.Context, req h9ExecutionRequest) (benchmark.RunResult, error) {
	if req.Dataset.AdapterPolicy == nil {
		return benchmark.RunResult{}, fmt.Errorf("H9 adapter policy is required")
	}
	registry, err := newH9Registry(req)
	if err != nil {
		return benchmark.RunResult{}, err
	}
	evaluator, err := newH9Evaluator(registry, *req.Dataset.AdapterPolicy, req.TempRoot)
	if err != nil {
		return benchmark.RunResult{}, err
	}
	runner := benchmark.H9ReplayRunner{Evaluator: evaluator}
	return runner.Run(ctx, benchmark.H9ReplayRunRequest{
		Dataset:  req.ReplayDataset,
		RunsRoot: req.RunsRoot,
		RunID:    req.RunID,
	})
}

func newH9RunID(now time.Time) (string, error) {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("random suffix: %w", err)
	}
	return "h9-" + now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix[:]), nil
}
