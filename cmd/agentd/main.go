package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ShenJun93/agent-council/internal/council/app"
	"github.com/ShenJun93/agent-council/internal/council/doctor"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[0] != "council" {
		printUsage(stderr)
		return 2
	}

	switch args[1] {
	case "run":
		return runCouncilRun(args[2:], stdout, stderr)
	case "doctor":
		return runCouncilDoctor(args[2:], stdout, stderr)
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
	_, _ = fmt.Fprintln(w, "  agentd council doctor isolation [--claude-bin claude] [--codex-bin codex]")
}
