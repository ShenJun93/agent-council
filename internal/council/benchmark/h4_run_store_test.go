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

func TestCreateH4RunUsesH4IdentityAndFiles(t *testing.T) {
	dataset, err := LoadH4(writeValidH4Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	runsRoot := t.TempDir()
	runRoot, manifest, err := CreateH4Run(context.Background(), runsRoot, "h4-test", dataset, time.Unix(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != H4RunSchemaVersion || manifest.BenchmarkID != H4BenchmarkID {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h4-run.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h3-run.json")); !os.IsNotExist(err) {
		t.Fatalf("H3 run marker present in H4 run: %v", err)
	}
}
func TestWriteH4FinalResultUsesH4IdentityAndFile(t *testing.T) {
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
	manifest, err := WriteH4FinalResult(context.Background(), runRoot, "h4-test", summary)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != H4ResultSchemaVersion || manifest.BenchmarkID != H4BenchmarkID {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h4-result.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h3-result.json")); !os.IsNotExist(err) {
		t.Fatalf("H3 result marker present in H4 run: %v", err)
	}
}
