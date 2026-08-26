package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadH1RejectsCaseOrderMismatch(t *testing.T) {
	root := writeValidH1Fixture(t)
	doc := readFixtureCases(t, root)
	doc.Cases[0], doc.Cases[1] = doc.Cases[1], doc.Cases[0]
	rewriteFixtureCasesAndManifest(t, root, doc)

	_, err := LoadH1(root)
	if err == nil || !strings.Contains(err.Error(), "order") {
		t.Fatalf("expected case order rejection, got %v", err)
	}
}

func TestLoadH1RejectsUnknownManifestField(t *testing.T) {
	root := writeValidH1Fixture(t)
	path := filepath.Join(root, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	value["unexpected"] = true
	mutated, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, mutated, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = LoadH1(root)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected unknown field rejection, got %v", err)
	}
}

func TestLoadH1RejectsRubricHashMismatch(t *testing.T) {
	root := writeValidH1Fixture(t)
	manifest := readFixtureManifest(t, root)
	manifest.RubricSHA256 = strings.Repeat("0", 64)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)

	_, err := LoadH1(root)
	if err == nil || !strings.Contains(err.Error(), "rubric") || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("expected rubric hash mismatch, got %v", err)
	}
}

func TestLoadH1RejectsRiskPolicyDrift(t *testing.T) {
	root := writeValidH1Fixture(t)
	manifest := readFixtureManifest(t, root)
	manifest.MaterialWorseDelta = 9
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)

	_, err := LoadH1(root)
	if err == nil || !strings.Contains(err.Error(), "material_worse_delta") {
		t.Fatalf("expected frozen delta rejection, got %v", err)
	}
}

func TestLoadH1RejectsProblemHashMismatch(t *testing.T) {
	root := writeValidH1Fixture(t)
	doc := readFixtureCases(t, root)
	doc.Cases[0].ProblemSHA256 = strings.Repeat("f", 64)
	rewriteFixtureCasesAndManifest(t, root, doc)

	_, err := LoadH1(root)
	if err == nil || !strings.Contains(err.Error(), "problem") || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("expected problem hash mismatch, got %v", err)
	}
}
