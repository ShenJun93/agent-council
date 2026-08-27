package benchmark

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

func TestCreateH2RunWritesVersionedIdentity(t *testing.T) {
	dataset, err := LoadH2(writeValidH2Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	runsRoot := t.TempDir()
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	runRoot, manifest, err := CreateH2Run(context.Background(), runsRoot, "h2-test", dataset, now)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.BenchmarkID != H2BenchmarkID || manifest.SchemaVersion != H2RunSchemaVersion {
		t.Fatalf("unexpected H2 manifest: %+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h2-run.json")); err != nil {
		t.Fatalf("missing h2-run.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h1-run.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected H1 manifest in H2 run: %v", err)
	}
}
func TestWriteH2FinalResultWritesH2Identity(t *testing.T) {
	runRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runRoot, "eval"), 0o750); err != nil {
		t.Fatal(err)
	}
	summary := evalharness.BatchSummary{ProblemCount: H1CaseCount}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runRoot, "eval", "batch-summary.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	manifest, err := WriteH2FinalResult(context.Background(), runRoot, "h2-test", summary)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.BenchmarkID != H2BenchmarkID || manifest.SchemaVersion != H2ResultSchemaVersion {
		t.Fatalf("unexpected H2 result manifest: %+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h2-result.json")); err != nil {
		t.Fatalf("missing h2-result.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h1-result.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected H1 result in H2 run: %v", err)
	}
}
