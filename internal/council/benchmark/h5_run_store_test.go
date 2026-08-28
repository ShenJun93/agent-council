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

func TestCreateH5RunUsesH5IdentityAndFiles(t *testing.T) {
	dataset, err := LoadH5(writeValidH5Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	runsRoot := t.TempDir()
	runRoot, manifest, err := CreateH5Run(context.Background(), runsRoot, "h5-test", dataset, time.Unix(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != H5RunSchemaVersion || manifest.BenchmarkID != H5BenchmarkID {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h5-run.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h3-run.json")); !os.IsNotExist(err) {
		t.Fatalf("H3 run marker present in H5 run: %v", err)
	}
}
func TestWriteH5FinalResultUsesH5IdentityAndFile(t *testing.T) {
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
	adapterSummary := H5AdapterSummary{SchemaVersion: H5AdapterSummarySchemaVersion, SuccessfulInvocations: H5ExpectedSuccessfulInvocations, EffectiveProviderDiversity: 2, TotalAvailabilityFailovers: 1, HumanBrokerInvocations: 1, AttemptsByAdapter: map[string]int{}, SuccessesByAdapter: map[string]int{}, SuccessesByProvider: map[string]int{}, AvailabilityFailuresByAdapter: map[string]int{}, SuccessesBySlot: map[string]map[string]int{}}
	adapterHash, err := WriteH5AdapterSummary(context.Background(), runRoot, adapterSummary)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := WriteH5FinalResult(context.Background(), runRoot, "h5-test", summary, adapterSummary, adapterHash)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != H5ResultSchemaVersion || manifest.BenchmarkID != H5BenchmarkID || manifest.AdapterSummarySHA256 != adapterHash || manifest.EffectiveProviderDiversity != 2 || manifest.TotalAvailabilityFailovers != 1 || manifest.HumanBrokerInvocations != 1 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h5-result.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runRoot, "h3-result.json")); !os.IsNotExist(err) {
		t.Fatalf("H3 result marker present in H5 run: %v", err)
	}
}
