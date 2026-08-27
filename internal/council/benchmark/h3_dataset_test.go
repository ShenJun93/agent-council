package benchmark

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadH3AcceptsVersionedShape(t *testing.T) {
	root := writeValidH3Fixture(t)
	dataset, err := LoadH3(root)
	if err != nil {
		t.Fatal(err)
	}
	if dataset.Manifest.BenchmarkID != H3BenchmarkID {
		t.Fatalf("benchmark id=%q want %q", dataset.Manifest.BenchmarkID, H3BenchmarkID)
	}
	if len(dataset.Cases) != H1CaseCount {
		t.Fatalf("case count=%d want %d", len(dataset.Cases), H1CaseCount)
	}
}

func TestLoadH3RejectsH2Identity(t *testing.T) {
	if _, err := LoadH3(writeValidH2Fixture(t)); err == nil {
		t.Fatal("LoadH3 unexpectedly accepted H2 identity")
	}
}
func writeValidH3Fixture(t *testing.T) string {
	t.Helper()
	root := writeValidH2Fixture(t)
	var rubric rubricDocument
	readJSONFile(t, filepath.Join(root, "rubric.json"), &rubric)
	rubric.SchemaVersion = H3RubricSchemaVersion
	rubricBytes := writeJSONFile(t, filepath.Join(root, "rubric.json"), rubric)
	doc := readFixtureCases(t, root)
	doc.SchemaVersion = H3CasesSchemaVersion
	casesBytes := writeJSONFile(t, filepath.Join(root, "cases.json"), doc)
	manifest := readFixtureManifest(t, root)
	manifest.SchemaVersion = H3DatasetSchemaVersion
	manifest.BenchmarkID = H3BenchmarkID
	manifest.RubricSHA256 = digestBytes(rubricBytes)
	manifest.CasesSHA256 = digestBytes(casesBytes)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
	return root
}

func TestCommittedH3MatchesH2SemanticContent(t *testing.T) {
	h2Root := filepath.Join("..", "..", "..", "benchmarks", "h2")
	h3Root := filepath.Join("..", "..", "..", "benchmarks", "h3")
	h2, err := LoadH2(h2Root)
	if err != nil {
		t.Fatal(err)
	}
	h3, err := LoadH3(h3Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(h2.Cases) != len(h3.Cases) {
		t.Fatalf("case count differs")
	}
	for i := range h2.Cases {
		a, b := h2.Cases[i], h3.Cases[i]
		if a.ID != b.ID || a.Category != b.Category || a.ChallengerProvider != b.ChallengerProvider {
			t.Fatalf("case %d identity drift", i)
		}
		if !bytes.Equal(a.Problem, b.Problem) || !bytes.Equal(a.ReferenceSet, b.ReferenceSet) {
			t.Fatalf("case %s semantic payload drift", a.ID)
		}
		if a.ProblemSHA256 != b.ProblemSHA256 || a.ReferenceSetSHA256 != b.ReferenceSetSHA256 {
			t.Fatalf("case %s payload hash drift", a.ID)
		}
	}
	var r2, r3 rubricDocument
	for path, out := range map[string]*rubricDocument{
		filepath.Join(h2Root, "rubric.json"): &r2,
		filepath.Join(h3Root, "rubric.json"): &r3,
	} {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatal(err)
		}
	}
	r2.SchemaVersion, r3.SchemaVersion = "", ""
	if !reflect.DeepEqual(r2, r3) {
		t.Fatalf("rubric semantic drift: h2=%+v h3=%+v", r2, r3)
	}
}
