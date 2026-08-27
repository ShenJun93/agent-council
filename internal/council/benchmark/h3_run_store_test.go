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

func TestCreateH3RunUsesH3IdentityAndFiles(t *testing.T) {
	dataset, err := LoadH3(writeValidH3Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	runsRoot := t.TempDir()
	runRoot, manifest, err := CreateH3Run(context.Background(), runsRoot, "h3-test", dataset, time.Unix(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != H3RunSchemaVersion || manifest.BenchmarkID != H3BenchmarkID {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h3-run.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h2-run.json")); !os.IsNotExist(err) {
		t.Fatalf("H2 run marker present in H3 run: %v", err)
	}
}
func TestWriteH3FinalResultUsesH3IdentityAndFile(t *testing.T) {
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
	manifest, err := WriteH3FinalResult(context.Background(), runRoot, "h3-test", summary)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != H3ResultSchemaVersion || manifest.BenchmarkID != H3BenchmarkID {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h3-result.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h2-result.json")); !os.IsNotExist(err) {
		t.Fatalf("H2 result marker present in H3 run: %v", err)
	}
}
