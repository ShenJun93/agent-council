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

func TestCreateH8RunUsesH8IdentityAndFiles(t *testing.T) {
	dataset, err := LoadH8(writeValidH8Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	runRoot, manifest, err := CreateH8Run(context.Background(), t.TempDir(), "h8-test", dataset, time.Unix(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != H8RunSchemaVersion || manifest.BenchmarkID != H8BenchmarkID {
		t.Fatalf("manifest=%+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h8-run.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h7-run.json")); !os.IsNotExist(err) {
		t.Fatalf("H7 marker present: %v", err)
	}
}

func TestWriteH8FinalResultUsesH8Identity(t *testing.T) {
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
	got, err := WriteH8FinalResult(context.Background(), runRoot, "h8-test", summary, adapter, ah)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != H8ResultSchemaVersion || got.BenchmarkID != H8BenchmarkID {
		t.Fatalf("result=%+v", got)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h8-result.json")); err != nil {
		t.Fatal(err)
	}
}
