package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCouncilRunCreatesRunAndPrintsJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	problem := filepath.Join(root, "problem.md")
	if err := os.WriteFile(problem, []byte("# Problem\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runs := filepath.Join(root, "runs")

	var stdout, stderr bytes.Buffer
	code := run([]string{"council", "run", "--runs-dir", runs, problem}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit = %d, stderr = %s", code, stderr.String())
	}

	var out struct {
		RunID  string `json:"run_id"`
		RunDir string `json:"run_dir"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not JSON: %v: %q", err, stdout.String())
	}
	if out.RunID == "" {
		t.Fatal("run_id is empty")
	}
	if out.RunDir != filepath.Join(runs, out.RunID) {
		t.Fatalf("run_dir = %q", out.RunDir)
	}
	if _, err := os.Stat(filepath.Join(out.RunDir, "manifest.json")); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"council", "unknown"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("run() accepted unknown command")
	}
}
