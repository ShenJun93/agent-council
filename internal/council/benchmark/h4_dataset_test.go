package benchmark

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadH4AcceptsVersionedShape(t *testing.T) {
	root := writeValidH4Fixture(t)
	dataset, err := LoadH4(root)
	if err != nil {
		t.Fatal(err)
	}
	if dataset.Manifest.BenchmarkID != H4BenchmarkID {
		t.Fatalf("benchmark id=%q want %q", dataset.Manifest.BenchmarkID, H4BenchmarkID)
	}
	if len(dataset.Cases) != H1CaseCount {
		t.Fatalf("case count=%d want %d", len(dataset.Cases), H1CaseCount)
	}
}

func TestLoadH4RejectsH3Identity(t *testing.T) {
	if _, err := LoadH4(writeValidH3Fixture(t)); err == nil {
		t.Fatal("LoadH4 unexpectedly accepted H3 identity")
	}
}
func writeValidH4Fixture(t *testing.T) string {
	t.Helper()
	root := writeValidH3Fixture(t)
	var rubric rubricDocument
	readJSONFile(t, filepath.Join(root, "rubric.json"), &rubric)
	rubric.SchemaVersion = H4RubricSchemaVersion
	rubricBytes := writeJSONFile(t, filepath.Join(root, "rubric.json"), rubric)
	doc := readFixtureCases(t, root)
	doc.SchemaVersion = H4CasesSchemaVersion
	casesBytes := writeJSONFile(t, filepath.Join(root, "cases.json"), doc)
	manifest := readFixtureManifest(t, root)
	manifest.SchemaVersion = H4DatasetSchemaVersion
	manifest.BenchmarkID = H4BenchmarkID
	manifest.RubricSHA256 = digestBytes(rubricBytes)
	manifest.CasesSHA256 = digestBytes(casesBytes)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
	return root
}

func TestCommittedH4MatchesH3SemanticContent(t *testing.T) {
	h3Root := filepath.Join("..", "..", "..", "benchmarks", "h3")
	h4Root := filepath.Join("..", "..", "..", "benchmarks", "h4")
	h3, err := LoadH3(h3Root)
	if err != nil {
		t.Fatal(err)
	}
	h4, err := LoadH4(h4Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(h3.Cases) != len(h4.Cases) {
		t.Fatalf("case count differs")
	}
	for i := range h3.Cases {
		a, b := h3.Cases[i], h4.Cases[i]
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
		filepath.Join(h3Root, "rubric.json"): &r2,
		filepath.Join(h4Root, "rubric.json"): &r3,
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
		t.Fatalf("rubric semantic drift: h3=%+v h4=%+v", r2, r3)
	}
}
