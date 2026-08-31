package benchmark

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeValidH8Fixture(t *testing.T) string {
	t.Helper()
	root := writeValidH7Fixture(t)
	var rubric rubricDocument
	readJSONFile(t, filepath.Join(root, "rubric.json"), &rubric)
	rubric.SchemaVersion = H8RubricSchemaVersion
	rubricBytes := writeJSONFile(t, filepath.Join(root, "rubric.json"), rubric)
	doc := readFixtureCases(t, root)
	doc.SchemaVersion = H8CasesSchemaVersion
	casesBytes := writeJSONFile(t, filepath.Join(root, "cases.json"), doc)
	manifest := readFixtureManifest(t, root)
	manifest.SchemaVersion = H8DatasetSchemaVersion
	manifest.BenchmarkID = H8BenchmarkID
	manifest.RubricSHA256 = digestBytes(rubricBytes)
	manifest.CasesSHA256 = digestBytes(casesBytes)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
	return root
}

func TestLoadH8AcceptsVersionedDataset(t *testing.T) {
	dataset, err := LoadH8(writeValidH8Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if dataset.Manifest.BenchmarkID != H8BenchmarkID || dataset.AdapterPolicy == nil {
		t.Fatalf("dataset=%+v", dataset.Manifest)
	}
	if len(dataset.Cases) != 20 {
		t.Fatalf("case count=%d want 20", len(dataset.Cases))
	}
	for _, c := range dataset.Cases {
		if c.ChallengerProvider != "" {
			t.Fatalf("case %s retained provider", c.ID)
		}
	}
}

func TestH8PoliciesEqualH7(t *testing.T) {
	if !reflect.DeepEqual(H8RiskPolicy, H7RiskPolicy) {
		t.Fatalf("risk policy drift: h7=%+v h8=%+v", H7RiskPolicy, H8RiskPolicy)
	}
	if !reflect.DeepEqual(H8ChallengePolicy, H7ChallengePolicy) {
		t.Fatalf("challenge policy drift: h7=%+v h8=%+v", H7ChallengePolicy, H8ChallengePolicy)
	}
}

func TestCommittedH8PreservesH7PayloadsAndPolicyBytes(t *testing.T) {
	h7, err := LoadH7(filepath.Join("..", "..", "..", "benchmarks", "h7"))
	if err != nil {
		t.Fatal(err)
	}
	h8, err := LoadH8(filepath.Join("..", "..", "..", "benchmarks", "h8"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(h7.AdapterPolicyBytes, h8.AdapterPolicyBytes) || h7.AdapterPolicySHA256 != h8.AdapterPolicySHA256 {
		t.Fatal("adapter policy drift")
	}
	normalizedH8Rubric := bytes.Replace(h8.Rubric, []byte(H8RubricSchemaVersion), []byte(H7RubricSchemaVersion), 1)
	if !bytes.Equal(h7.Rubric, normalizedH8Rubric) {
		t.Fatal("rubric semantic drift beyond schema_version")
	}
	if len(h7.Cases) != len(h8.Cases) {
		t.Fatal("case count drift")
	}
	for i := range h7.Cases {
		a, b := h7.Cases[i], h8.Cases[i]
		if a.ID != b.ID || a.Category != b.Category || !bytes.Equal(a.Problem, b.Problem) || !bytes.Equal(a.ReferenceSet, b.ReferenceSet) {
			t.Fatalf("case %d semantic drift", i)
		}
	}
}

func TestCommittedH8AdapterPolicyIsByteIdenticalToH7(t *testing.T) {
	h7, err := os.ReadFile(filepath.Join("..", "..", "..", "benchmarks", "h7", "adapter-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	h8, err := os.ReadFile(filepath.Join("..", "..", "..", "benchmarks", "h8", "adapter-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(h7, h8) {
		t.Fatal("benchmarks/h8/adapter-policy.json must be byte-identical to benchmarks/h7/adapter-policy.json")
	}
}
