package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateRunFreezesInputsAndWritesManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	problem := filepath.Join(root, "problem.md")
	configPath := filepath.Join(root, "council.yaml")
	prompts := filepath.Join(root, "prompts")
	reference := filepath.Join(root, "reference-set.json")
	runs := filepath.Join(root, "runs")

	mustWrite := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(problem, "# Decide\nChoose A or B.\n")
	mustWrite(configPath, "billing:\n  mode: subscription_only\n  fail_closed: true\n  allow_metered_fallback: false\n")
	mustWrite(filepath.Join(prompts, "research.md"), "Research independently.\n")
	mustWrite(filepath.Join(prompts, "judge.md"), "Judge independently.\n")
	mustWrite(reference, "{\"must_cover\":[\"risk\"]}\n")

	createdAt := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	manifest, err := CreateRun(context.Background(), CreateRunRequest{
		ProblemPath:      problem,
		ConfigPath:       configPath,
		PromptsDir:       prompts,
		ReferenceSetPath: reference,
		RunsRoot:         runs,
		RunID:            "run-test",
		Now:              func() time.Time { return createdAt },
	})
	if err != nil {
		t.Fatal(err)
	}

	if manifest.RunID != "run-test" {
		t.Fatalf("RunID = %q", manifest.RunID)
	}
	if manifest.SchemaVersion != "council.run.v0" {
		t.Fatalf("SchemaVersion = %q", manifest.SchemaVersion)
	}
	if manifest.CreatedAt != createdAt.Format(time.RFC3339Nano) {
		t.Fatalf("CreatedAt = %q", manifest.CreatedAt)
	}
	for name, digest := range map[string]string{
		"problem":       manifest.Inputs.Problem.SHA256,
		"config":        manifest.Inputs.Config.SHA256,
		"prompt_bundle": manifest.Inputs.PromptBundle.SHA256,
		"reference_set": manifest.Inputs.ReferenceSet.SHA256,
	} {
		if len(digest) != 64 {
			t.Fatalf("%s digest = %q, want SHA-256 hex", name, digest)
		}
	}

	runDir := filepath.Join(runs, "run-test")
	for _, path := range []string{
		filepath.Join(runDir, "manifest.json"),
		filepath.Join(runDir, "events.jsonl"),
		filepath.Join(runDir, "artifacts"),
		filepath.Join(runDir, "inputs", "problem.md"),
		filepath.Join(runDir, "inputs", "council.yaml"),
		filepath.Join(runDir, "inputs", "reference-set.json"),
		filepath.Join(runDir, "inputs", "prompts", "research.md"),
		filepath.Join(runDir, "inputs", "prompts", "judge.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}

	manifestBytes, err := os.ReadFile(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted Manifest
	if err := json.Unmarshal(manifestBytes, &persisted); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if persisted.RunID != manifest.RunID {
		t.Fatalf("persisted RunID = %q", persisted.RunID)
	}

	logBytes, err := os.ReadFile(filepath.Join(runDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(logBytes, &event); err != nil {
		t.Fatalf("events.jsonl first line is not JSON: %v", err)
	}
	if event["event"] != "run_created" {
		t.Fatalf("event = %v", event["event"])
	}
}

func TestCreateRunRefusesExistingRunDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	problem := filepath.Join(root, "problem.md")
	if err := os.WriteFile(problem, []byte("problem"), 0o600); err != nil {
		t.Fatal(err)
	}
	runs := filepath.Join(root, "runs")
	if err := os.MkdirAll(filepath.Join(runs, "same-id"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := CreateRun(context.Background(), CreateRunRequest{
		ProblemPath: problem,
		RunsRoot:    runs,
		RunID:       "same-id",
	})
	if err == nil {
		t.Fatal("CreateRun() overwrote an existing run directory")
	}
}
