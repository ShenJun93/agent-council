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

	"github.com/ShenJun93/agent-council/internal/council/app"
	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/benchmark"
	"github.com/ShenJun93/agent-council/internal/council/config"
	"github.com/ShenJun93/agent-council/internal/council/doctor"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/humanbroker"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type h1ExecutionRequest struct {
	Dataset     benchmark.Dataset
	DatasetPath string
	RunsRoot    string
	RunID       string
	TempRoot    string
	ClaudeBin   string
	CodexBin    string
}

type h1Executor func(context.Context, h1ExecutionRequest) (benchmark.RunResult, error)

func run(args []string, stdout, stderr io.Writer) int {
	return runWithPhaseHBenchmarkExecutors(args, stdout, stderr, executeH1Benchmark, executeH2Benchmark, executeH3Benchmark, executeH4Benchmark, executeH5Benchmark, executeH6Benchmark, executeH7Benchmark, executeH8Benchmark, executeH9Benchmark, executePhaseHBenchmark)
}

func runWithPhaseHBenchmarkExecutor(args []string, stdout, stderr io.Writer, executePhaseH phaseHExecutor) int {
	return runWithPhaseHBenchmarkExecutors(args, stdout, stderr, nil, nil, nil, nil, nil, nil, nil, nil, nil, executePhaseH)
}

func runWithPhaseHBenchmarkExecutors(args []string, stdout, stderr io.Writer, executeH1 h1Executor, executeH2 h2Executor, executeH3 h3Executor, executeH4 h4Executor, executeH5 h5Executor, executeH6 h6Executor, executeH7 h7Executor, executeH8 h8Executor, executeH9 h9Executor, executePhaseH phaseHExecutor) int {
	if len(args) >= 3 && args[0] == "council" && args[1] == "benchmark" && args[2] == "phase-h" {
		return runCouncilBenchmarkPhaseH(args[3:], stdout, stderr, executePhaseH)
	}
	return runWithH9BenchmarkExecutors(args, stdout, stderr, executeH1, executeH2, executeH3, executeH4, executeH5, executeH6, executeH7, executeH8, executeH9)
}

func runWithH9BenchmarkExecutors(args []string, stdout, stderr io.Writer, executeH1 h1Executor, executeH2 h2Executor, executeH3 h3Executor, executeH4 h4Executor, executeH5 h5Executor, executeH6 h6Executor, executeH7 h7Executor, executeH8 h8Executor, executeH9 h9Executor) int {
	if len(args) >= 3 && args[0] == "council" && args[1] == "benchmark" && args[2] == "h9" {
		return runCouncilBenchmarkH9(args[3:], stdout, stderr, executeH9)
	}
	return runWithH8BenchmarkExecutors(args, stdout, stderr, executeH1, executeH2, executeH3, executeH4, executeH5, executeH6, executeH7, executeH8)
}

func runWithH8BenchmarkExecutors(args []string, stdout, stderr io.Writer, executeH1 h1Executor, executeH2 h2Executor, executeH3 h3Executor, executeH4 h4Executor, executeH5 h5Executor, executeH6 h6Executor, executeH7 h7Executor, executeH8 h8Executor) int {
	if len(args) >= 3 && args[0] == "council" && args[1] == "benchmark" && args[2] == "h8" {
		return runCouncilBenchmarkH8(args[3:], stdout, stderr, executeH8)
	}
	return runWithH7BenchmarkExecutors(args, stdout, stderr, executeH1, executeH2, executeH3, executeH4, executeH5, executeH6, executeH7)
}

func runWithH7BenchmarkExecutors(args []string, stdout, stderr io.Writer, executeH1 h1Executor, executeH2 h2Executor, executeH3 h3Executor, executeH4 h4Executor, executeH5 h5Executor, executeH6 h6Executor, executeH7 h7Executor) int {
	if len(args) >= 3 && args[0] == "council" && args[1] == "benchmark" && args[2] == "h7" {
		return runCouncilBenchmarkH7(args[3:], stdout, stderr, executeH7)
	}
	return runWithH6BenchmarkExecutors(args, stdout, stderr, executeH1, executeH2, executeH3, executeH4, executeH5, executeH6)
}

func runWithH6BenchmarkExecutors(args []string, stdout, stderr io.Writer, executeH1 h1Executor, executeH2 h2Executor, executeH3 h3Executor, executeH4 h4Executor, executeH5 h5Executor, executeH6 h6Executor) int {
	if len(args) >= 3 && args[0] == "council" && args[1] == "benchmark" && args[2] == "h6" {
		return runCouncilBenchmarkH6(args[3:], stdout, stderr, executeH6)
	}
	return runWithH5BenchmarkExecutors(args, stdout, stderr, executeH1, executeH2, executeH3, executeH4, executeH5)
}

func runWithH5BenchmarkExecutors(args []string, stdout, stderr io.Writer, executeH1 h1Executor, executeH2 h2Executor, executeH3 h3Executor, executeH4 h4Executor, executeH5 h5Executor) int {
	if len(args) >= 2 && args[0] == "council" && args[1] == "broker" {
		return runCouncilBroker(args[2:], stdout, stderr)
	}
	if len(args) >= 3 && args[0] == "council" && args[1] == "benchmark" && args[2] == "h5" {
		return runCouncilBenchmarkH5(args[3:], stdout, stderr, executeH5)
	}
	return runWithAllBenchmarkExecutors(args, stdout, stderr, executeH1, executeH2, executeH3, executeH4)
}

func runCouncilBroker(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "agentd council broker requires pending, show, or submit")
		return 2
	}
	if args[0] == "pending" {
		return runCouncilBrokerPending(args[1:], stdout, stderr)
	}
	if args[0] == "show" {
		return runCouncilBrokerShow(args[1:], stdout, stderr)
	}
	if args[0] != "submit" {
		_, _ = fmt.Fprintln(stderr, "agentd council broker requires pending, show, or submit")
		return 2
	}
	fs := flag.NewFlagSet("agentd council broker submit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runRoot := fs.String("run-root", "", "run root containing the human broker request")
	requestID := fs.String("request-id", "", "human broker request id")
	responseFile := fs.String("response-file", "", "file containing raw ChatGPT response")
	fresh := fs.Bool("fresh-session", false, "attest that the response came from a brand-new ChatGPT New Chat")
	current := fs.Bool("current-session", false, "attest that the response came from the current orchestrating ChatGPT conversation")
	modelLabel := fs.String("model-label", "", "optional ChatGPT model label shown in the UI")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 || *runRoot == "" || *requestID == "" || *responseFile == "" {
		_, _ = fmt.Fprintln(stderr, "broker submit requires --run-root, --request-id, and --response-file")
		return 2
	}
	packet, err := humanbroker.LoadRequest(*runRoot, *requestID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load broker request: %v\n", err)
		return 1
	}
	if *fresh == *current || packet.RequireFreshSession != *fresh || packet.RequireCurrentSession != *current {
		_, _ = fmt.Fprintln(stderr, "broker submit session attestation does not match request")
		return 2
	}
	info, err := os.Lstat(*responseFile)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		_, _ = fmt.Fprintln(stderr, "response-file must be a real regular file")
		return 1
	}
	raw, err := os.ReadFile(*responseFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read broker response: %v\n", err)
		return 1
	}
	if err := humanbroker.SubmitResponse(*runRoot, humanbroker.Submission{RequestID: *requestID, Nonce: packet.Nonce, FreshSession: *fresh, CurrentSession: *current, ModelLabel: *modelLabel, RawResponse: string(raw)}); err != nil {
		_, _ = fmt.Fprintf(stderr, "submit broker response: %v\n", err)
		return 1
	}
	return encodeBrokerSubmission(stdout, stderr, *requestID)
}

func runCouncilBrokerPending(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agentd council broker pending", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runRoot := fs.String("run-root", "", "H5 run root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || *runRoot == "" {
		_, _ = fmt.Fprintln(stderr, "broker pending requires --run-root")
		return 2
	}
	pending, err := humanbroker.ListPending(*runRoot)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "list broker requests: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(pending); err != nil {
		_, _ = fmt.Fprintf(stderr, "write broker pending output: %v\n", err)
		return 1
	}
	return 0
}

func runCouncilBrokerShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agentd council broker show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runRoot := fs.String("run-root", "", "H5 run root")
	requestID := fs.String("request-id", "", "human broker request id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || *runRoot == "" || *requestID == "" {
		_, _ = fmt.Fprintln(stderr, "broker show requires --run-root and --request-id")
		return 2
	}
	packet, err := humanbroker.LoadRequest(*runRoot, *requestID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load broker request: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, packet.PasteablePrompt)
	return 0
}

func encodeBrokerSubmission(stdout, stderr io.Writer, requestID string) int {
	out := struct {
		RequestID string `json:"request_id"`
		Status    string `json:"status"`
	}{RequestID: requestID, Status: "submitted"}
	if err := json.NewEncoder(stdout).Encode(out); err != nil {
		_, _ = fmt.Fprintf(stderr, "write broker output: %v\n", err)
		return 1
	}
	return 0
}

func runWithH1Executor(args []string, stdout, stderr io.Writer, execute h1Executor) int {
	return runWithAllBenchmarkExecutors(args, stdout, stderr, execute, nil, nil, nil)
}

func runWithBenchmarkExecutors(args []string, stdout, stderr io.Writer, executeH1 h1Executor, executeH2 h2Executor) int {
	return runWithAllBenchmarkExecutors(args, stdout, stderr, executeH1, executeH2, nil, nil)
}

func runWithAllBenchmarkExecutors(args []string, stdout, stderr io.Writer, executeH1 h1Executor, executeH2 h2Executor, executeH3 h3Executor, executeH4 ...h4Executor) int {
	if len(args) < 2 || args[0] != "council" {
		printUsage(stderr)
		return 2
	}

	switch args[1] {
	case "run":
		return runCouncilRun(args[2:], stdout, stderr)
	case "doctor":
		return runCouncilDoctor(args[2:], stdout, stderr)
	case "benchmark":
		return runCouncilBenchmarkAll(args[2:], stdout, stderr, executeH1, executeH2, executeH3, executeH4...)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown council command %q\n", args[1])
		printUsage(stderr)
		return 2
	}
}

func runCouncilRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agentd council run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to council.yaml")
	promptsDir := fs.String("prompts-dir", "", "directory containing prompt templates")
	referenceSet := fs.String("reference-set", "", "path to frozen reference-set.json")
	runsDir := fs.String("runs-dir", "", "override run artifact root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "agentd council run requires exactly one problem file")
		return 2
	}

	manifest, err := app.CreateRun(context.Background(), app.CreateRunRequest{
		ProblemPath:      fs.Arg(0),
		ConfigPath:       *configPath,
		PromptsDir:       *promptsDir,
		ReferenceSetPath: *referenceSet,
		RunsRoot:         *runsDir,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "create council run: %v\n", err)
		return 1
	}

	out := struct {
		RunID  string `json:"run_id"`
		RunDir string `json:"run_dir"`
	}{
		RunID:  manifest.RunID,
		RunDir: filepath.Clean(manifest.RunDir),
	}
	if err := json.NewEncoder(stdout).Encode(out); err != nil {
		_, _ = fmt.Fprintf(stderr, "write output: %v\n", err)
		return 1
	}
	return 0
}

func runCouncilBenchmarkAll(args []string, stdout, stderr io.Writer, executeH1 h1Executor, executeH2 h2Executor, executeH3 h3Executor, executeH4 ...h4Executor) int {
	var h4 h4Executor
	if len(executeH4) > 0 {
		h4 = executeH4[0]
	}
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "agentd council benchmark requires the h1, h2, h3, or h4 subcommand")
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "h1":
		return runCouncilBenchmarkH1(args[1:], stdout, stderr, executeH1)
	case "h2":
		return runCouncilBenchmarkH2(args[1:], stdout, stderr, executeH2)
	case "h3":
		return runCouncilBenchmarkH3(args[1:], stdout, stderr, executeH3)
	case "h4":
		return runCouncilBenchmarkH4(args[1:], stdout, stderr, h4)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown benchmark version %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runCouncilBenchmarkH1(args []string, stdout, stderr io.Writer, execute h1Executor) int {
	fs := flag.NewFlagSet("agentd council benchmark h1", flag.ContinueOnError)
	fs.SetOutput(stderr)
	datasetPath := fs.String("dataset", filepath.FromSlash("benchmarks/h1"), "path to frozen H1 dataset")
	runsDir := fs.String("runs-dir", "", "override H1 run artifact root")
	configPath := fs.String("config", "", "path to council.yaml")
	tempRoot := fs.String("temp-root", os.TempDir(), "temporary workspace root")
	claudeBin := fs.String("claude-bin", "claude", "Claude Code CLI binary")
	codexBin := fs.String("codex-bin", "codex", "Codex CLI binary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "agentd council benchmark h1 does not accept positional arguments")
		return 2
	}
	if execute == nil {
		_, _ = fmt.Fprintln(stderr, "H1 benchmark executor is required")
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H1 config: %v\n", err)
		return 1
	}
	dataset, err := benchmark.LoadH1(*datasetPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load H1 dataset: %v\n", err)
		return 1
	}
	resolvedRunsRoot := *runsDir
	if resolvedRunsRoot == "" {
		resolvedRunsRoot = cfg.Runs.Root
	}
	runID, err := newH1RunID(time.Now().UTC())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate H1 run id: %v\n", err)
		return 1
	}

	result, err := execute(context.Background(), h1ExecutionRequest{
		Dataset:     dataset,
		DatasetPath: *datasetPath,
		RunsRoot:    resolvedRunsRoot,
		RunID:       runID,
		TempRoot:    *tempRoot,
		ClaudeBin:   *claudeBin,
		CodexBin:    *codexBin,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "run H1 benchmark: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		_, _ = fmt.Fprintf(stderr, "write H1 benchmark output: %v\n", err)
		return 1
	}
	return 0
}

func executeH1Benchmark(ctx context.Context, req h1ExecutionRequest) (benchmark.RunResult, error) {
	claude := councilruntime.NewClaudeCLI(req.ClaudeBin)
	codex := councilruntime.NewCodexCLI(req.CodexBin)
	evaluator := evalharness.Harness{Claude: claude, Codex: codex, TempRoot: req.TempRoot}
	runner := benchmark.Runner{
		NewBaseline: func(provider councilruntime.Provider) benchmark.BaselineExecutor {
			return baseline.Runner{
				Claude:             claude,
				Codex:              codex,
				TempRoot:           req.TempRoot,
				ChallengerProvider: provider,
				ChallengePolicy:    benchmark.H1ChallengePolicy,
			}
		},
		Evaluator: evaluator,
	}
	return runner.Run(ctx, benchmark.RunRequest{
		Dataset:  req.Dataset,
		RunsRoot: req.RunsRoot,
		RunID:    req.RunID,
	})
}

func newH1RunID(now time.Time) (string, error) {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("random suffix: %w", err)
	}
	return "h1-" + now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix[:]), nil
}

func runCouncilDoctor(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "isolation" {
		_, _ = fmt.Fprintln(stderr, "agentd council doctor requires the isolation subcommand")
		printUsage(stderr)
		return 2
	}
	return runCouncilDoctorIsolation(args[1:], stdout, stderr)
}

func runCouncilDoctorIsolation(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agentd council doctor isolation", flag.ContinueOnError)
	fs.SetOutput(stderr)
	claudeBin := fs.String("claude-bin", "claude", "Claude Code CLI binary")
	codexBin := fs.String("codex-bin", "codex", "Codex CLI binary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "agentd council doctor isolation does not accept positional arguments")
		return 2
	}

	probes := []doctor.Probe{
		{Name: "claude", Runtime: councilruntime.NewClaudeCLI(*claudeBin)},
		{Name: "codex", Runtime: councilruntime.NewCodexCLI(*codexBin)},
	}
	return runCouncilDoctorIsolationWithProbes(context.Background(), probes, stdout, stderr)
}

func runCouncilDoctorIsolationWithProbes(ctx context.Context, probes []doctor.Probe, stdout, stderr io.Writer) int {
	report, err := doctor.RunIsolation(ctx, probes)
	if encodeErr := json.NewEncoder(stdout).Encode(report); encodeErr != nil {
		_, _ = fmt.Fprintf(stderr, "write isolation doctor report: %v\n", encodeErr)
		return 1
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "isolation doctor failed: %v\n", err)
		return 1
	}
	return 0
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage:")
	_, _ = fmt.Fprintln(w, "  agentd council run [flags] <problem.md>")
	_, _ = fmt.Fprintln(w, "  agentd council benchmark h1 [--dataset benchmarks/h1] [--runs-dir .council/runs] [--config council.yaml] [--temp-root TMP] [--claude-bin claude] [--codex-bin codex]")
	_, _ = fmt.Fprintln(w, "  agentd council benchmark h2 [--dataset benchmarks/h2] [--runs-dir .council/runs] [--config council.yaml] [--temp-root TMP] [--claude-bin claude] [--codex-bin codex]")
	_, _ = fmt.Fprintln(w, "  agentd council benchmark h3 [--dataset benchmarks/h3] [--runs-dir .council/runs] [--config council.yaml] [--temp-root TMP] [--claude-bin claude] [--codex-bin codex]")
	_, _ = fmt.Fprintln(w, "  agentd council benchmark h4 [--dataset benchmarks/h4] [--runs-dir .council/runs] [--config council.yaml] [--temp-root TMP] [--claude-bin claude] [--codex-bin codex]")
	_, _ = fmt.Fprintln(w, "  agentd council benchmark h5 [--dataset benchmarks/h5] [--runs-dir .council/runs] [--config council.yaml] [--temp-root TMP] [--claude-bin claude] [--codex-bin codex]")
	_, _ = fmt.Fprintln(w, "  agentd council benchmark phase-h [--dataset benchmarks/phase-h] [--runs-dir .council/runs] [--config council.yaml] [--temp-root TMP]")
	_, _ = fmt.Fprintln(w, "  agentd council broker submit --run-root RUN --request-id REQ --response-file FILE (--fresh-session|--current-session) [--model-label LABEL]")
	_, _ = fmt.Fprintln(w, "  agentd council doctor isolation [--claude-bin claude] [--codex-bin codex]")
}
