package benchmark

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeValidH7Fixture(t *testing.T) string {
	t.Helper()
	root := writeValidH6Fixture(t)
	var rubric rubricDocument
	readJSONFile(t, filepath.Join(root, "rubric.json"), &rubric)
	rubric.SchemaVersion = H7RubricSchemaVersion
	rubricBytes := writeJSONFile(t, filepath.Join(root, "rubric.json"), rubric)
	doc := readFixtureCases(t, root)
	doc.SchemaVersion = H7CasesSchemaVersion
	casesBytes := writeJSONFile(t, filepath.Join(root, "cases.json"), doc)
	manifest := readFixtureManifest(t, root)
	manifest.SchemaVersion = H7DatasetSchemaVersion
	manifest.BenchmarkID = H7BenchmarkID
	manifest.RubricSHA256 = digestBytes(rubricBytes)
	manifest.CasesSHA256 = digestBytes(casesBytes)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
	return root
}

func TestLoadH7AcceptsVersionedClaimAwareDataset(t *testing.T) {
	dataset, err := LoadH7(writeValidH7Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if dataset.Manifest.BenchmarkID != H7BenchmarkID || dataset.AdapterPolicy == nil {
		t.Fatalf("dataset=%+v", dataset.Manifest)
	}
	for _, c := range dataset.Cases {
		if c.ChallengerProvider != "" {
			t.Fatalf("case %s retained provider", c.ID)
		}
	}
}

func TestCommittedH7PreservesH6PayloadsAndPolicyBytes(t *testing.T) {
	h6, err := LoadH6(filepath.Join("..", "..", "..", "benchmarks", "h6"))
	if err != nil {
		t.Fatal(err)
	}
	h7, err := LoadH7(filepath.Join("..", "..", "..", "benchmarks", "h7"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(h6.AdapterPolicyBytes, h7.AdapterPolicyBytes) {
		t.Fatal("adapter policy drift")
	}
	if h6.AdapterPolicySHA256 != h7.AdapterPolicySHA256 {
		t.Fatal("adapter policy hash drift")
	}
	if len(h6.Cases) != len(h7.Cases) {
		t.Fatal("case count drift")
	}
	for i := range h6.Cases {
		a, b := h6.Cases[i], h7.Cases[i]
		if a.ID != b.ID || a.Category != b.Category || !bytes.Equal(a.Problem, b.Problem) || !bytes.Equal(a.ReferenceSet, b.ReferenceSet) {
			t.Fatalf("case %d semantic drift", i)
		}
	}
}

func TestCommittedH7AdapterPolicyIsByteIdenticalToH6(t *testing.T) {
	h6, err := os.ReadFile(filepath.Join("..", "..", "..", "benchmarks", "h6", "adapter-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	h7, err := os.ReadFile(filepath.Join("..", "..", "..", "benchmarks", "h7", "adapter-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(h6, h7) {
		t.Fatal("benchmarks/h7/adapter-policy.json must be byte-identical to benchmarks/h6/adapter-policy.json")
	}
}
