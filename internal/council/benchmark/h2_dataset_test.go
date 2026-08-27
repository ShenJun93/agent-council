package benchmark

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadH2AcceptsVersionedShape(t *testing.T) {
	root := writeValidH2Fixture(t)
	dataset, err := LoadH2(root)
	if err != nil {
		t.Fatal(err)
	}
	if dataset.Manifest.BenchmarkID != H2BenchmarkID {
		t.Fatalf("benchmark id = %q want %q", dataset.Manifest.BenchmarkID, H2BenchmarkID)
	}
	if len(dataset.Cases) != H1CaseCount {
		t.Fatalf("case count = %d want %d", len(dataset.Cases), H1CaseCount)
	}
}

func TestLoadH2RejectsH1Identity(t *testing.T) {
	root := writeValidH1Fixture(t)
	if _, err := LoadH2(root); err == nil {
		t.Fatal("LoadH2 unexpectedly accepted H1 identity")
	}
}
func writeValidH2Fixture(t *testing.T) string {
	t.Helper()
	root := writeValidH1Fixture(t)

	var rubric rubricDocument
	readJSONFile(t, filepath.Join(root, "rubric.json"), &rubric)
	rubric.SchemaVersion = H2RubricSchemaVersion
	rubricBytes := writeJSONFile(t, filepath.Join(root, "rubric.json"), rubric)

	doc := readFixtureCases(t, root)
	doc.SchemaVersion = H2CasesSchemaVersion
	casesBytes := writeJSONFile(t, filepath.Join(root, "cases.json"), doc)

	manifest := readFixtureManifest(t, root)
	manifest.SchemaVersion = H2DatasetSchemaVersion
	manifest.BenchmarkID = H2BenchmarkID
	manifest.RubricSHA256 = digestBytes(rubricBytes)
	manifest.CasesSHA256 = digestBytes(casesBytes)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
	return root
}

func TestCommittedH2MatchesH1SemanticContent(t *testing.T) {
	h1Root := filepath.Join("..", "..", "..", "benchmarks", "h1")
	h2Root := filepath.Join("..", "..", "..", "benchmarks", "h2")
	h1, err := LoadH1(h1Root)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := LoadH2(h2Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(h1.Cases) != len(h2.Cases) {
		t.Fatalf("case count differs: h1=%d h2=%d", len(h1.Cases), len(h2.Cases))
	}
	for i := range h1.Cases {
		a, b := h1.Cases[i], h2.Cases[i]
		if a.ID != b.ID || a.Category != b.Category || a.ChallengerProvider != b.ChallengerProvider {
			t.Fatalf("case %d identity drift: h1=%+v h2=%+v", i, a, b)
		}
		if !bytes.Equal(a.Problem, b.Problem) || !bytes.Equal(a.ReferenceSet, b.ReferenceSet) {
			t.Fatalf("case %s semantic payload drift", a.ID)
		}
		if a.ProblemSHA256 != b.ProblemSHA256 || a.ReferenceSetSHA256 != b.ReferenceSetSHA256 {
			t.Fatalf("case %s payload hash drift", a.ID)
		}
	}

	var r1, r2 rubricDocument
	mustReadRubric := func(path string, out *rubricDocument) {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if decodeErr := json.Unmarshal(data, out); decodeErr != nil {
			t.Fatal(decodeErr)
		}
	}
	mustReadRubric(filepath.Join(h1Root, "rubric.json"), &r1)
	mustReadRubric(filepath.Join(h2Root, "rubric.json"), &r2)
	r1.SchemaVersion, r2.SchemaVersion = "", ""
	if !reflect.DeepEqual(r1, r2) {
		t.Fatalf("rubric semantic drift:\nh1=%+v\nh2=%+v", r1, r2)
	}
}
