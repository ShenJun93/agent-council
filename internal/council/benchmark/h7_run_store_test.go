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

func TestCreateH7RunUsesH7IdentityAndFiles(t *testing.T) {
	dataset, err := LoadH7(writeValidH7Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	runRoot, manifest, err := CreateH7Run(context.Background(), t.TempDir(), "h7-test", dataset, time.Unix(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != H7RunSchemaVersion || manifest.BenchmarkID != H7BenchmarkID {
		t.Fatalf("manifest=%+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h7-run.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h6-run.json")); !os.IsNotExist(err) {
		t.Fatalf("H6 marker present: %v", err)
	}
}

func TestWriteH7FinalResultUsesH7Identity(t *testing.T) {
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
	got, err := WriteH7FinalResult(context.Background(), runRoot, "h7-test", summary, adapter, ah)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != H7ResultSchemaVersion || got.BenchmarkID != H7BenchmarkID {
		t.Fatalf("result=%+v", got)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h7-result.json")); err != nil {
		t.Fatal(err)
	}
}
