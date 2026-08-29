package benchmark

import (
	"context"
	"encoding/json"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateH6RunUsesH6IdentityAndFiles(t *testing.T) {
	dataset, err := LoadH6(writeValidH6Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	runRoot, manifest, err := CreateH6Run(context.Background(), t.TempDir(), "h6-test", dataset, time.Unix(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != H6RunSchemaVersion || manifest.BenchmarkID != H6BenchmarkID {
		t.Fatalf("manifest=%+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h6-run.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h5-run.json")); !os.IsNotExist(err) {
		t.Fatalf("H5 marker present: %v", err)
	}
}

func TestWriteH6FinalResultUsesH6Identity(t *testing.T) {
	runRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runRoot, "eval"), 0o750); err != nil {
		t.Fatal(err)
	}
	summary := evalharness.BatchSummary{ProblemCount: H1CaseCount}
	data, _ := json.Marshal(summary)
	if err := os.WriteFile(filepath.Join(runRoot, "eval", "batch-summary.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := H5AdapterSummary{SchemaVersion: H5AdapterSummarySchemaVersion, SuccessfulInvocations: H5ExpectedSuccessfulInvocations, AttemptsByAdapter: map[string]int{}, SuccessesByAdapter: map[string]int{}, SuccessesByProvider: map[string]int{}, AvailabilityFailuresByAdapter: map[string]int{}, SuccessesBySlot: map[string]map[string]int{}}
	ah, err := WriteH5AdapterSummary(context.Background(), runRoot, adapter)
	if err != nil {
		t.Fatal(err)
	}
	got, err := WriteH6FinalResult(context.Background(), runRoot, "h6-test", summary, adapter, ah)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != H6ResultSchemaVersion || got.BenchmarkID != H6BenchmarkID {
		t.Fatalf("result=%+v", got)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h6-result.json")); err != nil {
		t.Fatal(err)
	}
}
