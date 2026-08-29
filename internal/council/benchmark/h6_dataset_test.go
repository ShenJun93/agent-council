package benchmark

import (
	"bytes"
	"path/filepath"
	"testing"
)

func writeValidH6Fixture(t *testing.T) string {
	t.Helper()
	root := writeValidH5Fixture(t)
	var rubric rubricDocument
	readJSONFile(t, filepath.Join(root, "rubric.json"), &rubric)
	rubric.SchemaVersion = H6RubricSchemaVersion
	rubricBytes := writeJSONFile(t, filepath.Join(root, "rubric.json"), rubric)
	doc := readFixtureCases(t, root)
	doc.SchemaVersion = H6CasesSchemaVersion
	casesBytes := writeJSONFile(t, filepath.Join(root, "cases.json"), doc)
	manifest := readFixtureManifest(t, root)
	manifest.SchemaVersion = H6DatasetSchemaVersion
	manifest.BenchmarkID = H6BenchmarkID
	manifest.RubricSHA256 = digestBytes(rubricBytes)
	manifest.CasesSHA256 = digestBytes(casesBytes)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
	return root
}

func TestLoadH6AcceptsVersionedAdaptiveDataset(t *testing.T) {
	dataset, err := LoadH6(writeValidH6Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if dataset.Manifest.BenchmarkID != H6BenchmarkID || dataset.AdapterPolicy == nil {
		t.Fatalf("dataset=%+v", dataset.Manifest)
	}
	for _, c := range dataset.Cases {
		if c.ChallengerProvider != "" {
			t.Fatalf("case %s retained provider", c.ID)
		}
	}
}

func TestCommittedH6PreservesH5PayloadsAndPolicyBytes(t *testing.T) {
	h5, err := LoadH5(filepath.Join("..", "..", "..", "benchmarks", "h5"))
	if err != nil {
		t.Fatal(err)
	}
	h6, err := LoadH6(filepath.Join("..", "..", "..", "benchmarks", "h6"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(h5.AdapterPolicyBytes, h6.AdapterPolicyBytes) {
		t.Fatal("adapter policy drift")
	}
	if len(h5.Cases) != len(h6.Cases) {
		t.Fatal("case count drift")
	}
	for i := range h5.Cases {
		a, b := h5.Cases[i], h6.Cases[i]
		if a.ID != b.ID || a.Category != b.Category || !bytes.Equal(a.Problem, b.Problem) || !bytes.Equal(a.ReferenceSet, b.ReferenceSet) {
			t.Fatalf("case %d semantic drift", i)
		}
	}
}
