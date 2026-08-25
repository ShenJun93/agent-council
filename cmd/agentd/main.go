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
	default:
		fmt.Fprintf(stderr, "unknown council command %q\n", args[1])
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
		fmt.Fprintln(stderr, "agentd council run requires exactly one problem file")
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
		fmt.Fprintf(stderr, "create council run: %v\n", err)
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
		fmt.Fprintf(stderr, "write output: %v\n", err)
		return 1
	}
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: agentd council run [flags] <problem.md>")
}
