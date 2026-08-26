package benchmark

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

func TestCreateRunWritesFrozenInputs(t *testing.T) {
	datasetRoot := writeValidH1Fixture(t)
	dataset, err := LoadH1(datasetRoot)
	if err != nil {
		t.Fatal(err)
	}
	runsRoot := t.TempDir()
	now := time.Date(2026, 8, 27, 1, 2, 3, 4, time.UTC)

	runRoot, manifest, err := CreateRun(context.Background(), runsRoot, "h1-test", dataset, now)
	if err != nil {
		t.Fatal(err)
	}
	if runRoot != filepath.Join(runsRoot, "h1-test") {
		t.Fatalf("run root = %q", runRoot)
	}
	if manifest.BenchmarkID != H1BenchmarkID || manifest.RunID != "h1-test" {
		t.Fatalf("unexpected run manifest: %#v", manifest)
	}
	if manifest.CreatedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("created_at = %q", manifest.CreatedAt)
	}

	wantFiles := []string{
		"h1-run.json",
		filepath.Join("inputs", "benchmark-manifest.json"),
		filepath.Join("inputs", "rubric.json"),
	}
	for _, c := range dataset.Cases {
		wantFiles = append(wantFiles,
			filepath.Join("inputs", "cases", c.ID, "problem.json"),
			filepath.Join("inputs", "cases", c.ID, "reference-set.json"),
		)
	}
	for _, rel := range wantFiles {
		info, err := os.Stat(filepath.Join(runRoot, rel))
		if err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("%s is not a regular file", rel)
		}
	}
}

func TestCreateRunRejectsDuplicateRun(t *testing.T) {
	datasetRoot := writeValidH1Fixture(t)
	dataset, err := LoadH1(datasetRoot)
	if err != nil {
		t.Fatal(err)
	}
	runsRoot := t.TempDir()
	now := time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC)

	if _, _, err := CreateRun(context.Background(), runsRoot, "h1-test", dataset, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := CreateRun(context.Background(), runsRoot, "h1-test", dataset, now); err == nil {
		t.Fatal("expected duplicate run rejection")
	}
}

func TestWriteBaselineResultsRefusesOverwrite(t *testing.T) {
	runRoot := t.TempDir()
	results := sixArmResults()
	if err := WriteBaselineResults(context.Background(), runRoot, "tech-01-db-cutover", results); err != nil {
		t.Fatal(err)
	}
	if err := WriteBaselineResults(context.Background(), runRoot, "tech-01-db-cutover", results); err == nil {
		t.Fatal("expected immutable overwrite rejection")
	}
}

func TestWriteBaselineResultsRejectsWrongArmSet(t *testing.T) {
	results := sixArmResults()
	results[len(results)-1].Arm = baseline.ArmEFullInfo

	err := WriteBaselineResults(context.Background(), t.TempDir(), "tech-01-db-cutover", results)
	if err == nil || !strings.Contains(err.Error(), "arm") {
		t.Fatalf("expected A-F arm-set rejection, got %v", err)
	}
}

func TestWriteBaselineResultsRejectsUnsafeProblemID(t *testing.T) {
	err := WriteBaselineResults(context.Background(), t.TempDir(), "../escape", sixArmResults())
	if err == nil || !strings.Contains(err.Error(), "problem") {
		t.Fatalf("expected unsafe problem id rejection, got %v", err)
	}
}

func TestWriteBaselineResultsRejectsSymlinkEscape(t *testing.T) {
	runRoot := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(runRoot, "baseline")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := WriteBaselineResults(context.Background(), runRoot, "tech-01-db-cutover", sixArmResults())
	if err == nil {
		t.Fatal("expected symlink escape rejection")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink escape wrote outside run root: %v", entries)
	}
}

func TestWriteFinalResultRejectsMissingBatchSummary(t *testing.T) {
	_, err := WriteFinalResult(context.Background(), t.TempDir(), "h1-test", evalharness.BatchSummary{})
	if err == nil || !strings.Contains(err.Error(), "batch-summary") {
		t.Fatalf("expected missing batch-summary rejection, got %v", err)
	}
}

func TestWriteFinalResultHashesExactBatchSummaryBytes(t *testing.T) {
	runRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runRoot, "eval"), 0o750); err != nil {
		t.Fatal(err)
	}
	summary := evalharness.BatchSummary{ProblemCount: 20}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runRoot, "eval", "batch-summary.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	manifest, err := WriteFinalResult(context.Background(), runRoot, "h1-test", summary)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.RunID != "h1-test" || manifest.ProblemCount != 20 || len(manifest.BatchSummarySHA256) != 64 {
		t.Fatalf("unexpected result manifest: %#v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h1-result.json")); err != nil {
		t.Fatal(err)
	}
}

func sixArmResults() []baseline.ArmResult {
	return []baseline.ArmResult{
		{Arm: baseline.ArmAClaudeSingle},
		{Arm: baseline.ArmBCodexSingle},
		{Arm: baseline.ArmCClaudeSelfReview},
		{Arm: baseline.ArmDCodexSelfReview},
		{Arm: baseline.ArmEFullInfo},
		{Arm: baseline.ArmFBlindCouncil},
	}
}
