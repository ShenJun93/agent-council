package benchmark

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureManifest struct {
	SchemaVersion      string         `json:"schema_version"`
	BenchmarkID        string         `json:"benchmark_id"`
	CaseCount          int            `json:"case_count"`
	CategoryCounts     map[string]int `json:"category_counts"`
	CaseIDs            []string       `json:"case_ids"`
	RubricSHA256       string         `json:"rubric_sha256"`
	CasesSHA256        string         `json:"cases_sha256"`
	Comparator         string         `json:"comparator"`
	MaterialWorseDelta float64        `json:"material_worse_delta"`
}

type fixtureCasesDocument struct {
	SchemaVersion string        `json:"schema_version"`
	Cases         []fixtureCase `json:"cases"`
}

type fixtureCase struct {
	ID                 string          `json:"id"`
	Category           string          `json:"category"`
	ChallengerProvider string          `json:"challenger_provider"`
	Problem            json.RawMessage `json:"problem"`
	ProblemSHA256      string          `json:"problem_sha256"`
	ReferenceSet       json.RawMessage `json:"reference_set"`
	ReferenceSetSHA256 string          `json:"reference_set_sha256"`
}

var fixtureCaseIDs = []string{
	"tech-01-db-cutover",
	"tech-02-api-rate-limits",
	"tech-03-cache-stampede",
	"tech-04-token-rotation",
	"tech-05-queue-ordering",
	"tech-06-backup-retention",
	"tech-07-deploy-rollback",
	"tech-08-observability-sampling",
	"tech-09-search-build-buy",
	"tech-10-data-reconciliation",
	"product-01-pricing-tiers",
	"product-02-onboarding-friction",
	"product-03-notification-launch",
	"product-04-enterprise-sso",
	"product-05-marketplace-moderation",
	"product-06-feature-deprecation",
	"product-07-regional-expansion",
	"product-08-experiment-guardrails",
	"product-09-support-automation",
	"product-10-roadmap-retention",
}

func TestLoadH1AcceptsFrozenShape(t *testing.T) {
	root := writeValidH1Fixture(t)

	dataset, err := LoadH1(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(dataset.Cases); got != 20 {
		t.Fatalf("case count = %d, want 20", got)
	}
	if dataset.Manifest.BenchmarkID != "h1" {
		t.Fatalf("benchmark id = %q, want h1", dataset.Manifest.BenchmarkID)
	}
	if dataset.Cases[0].ID != fixtureCaseIDs[0] || dataset.Cases[19].ID != fixtureCaseIDs[19] {
		t.Fatalf("unexpected frozen case order: first=%q last=%q", dataset.Cases[0].ID, dataset.Cases[19].ID)
	}
}

func TestLoadH1RejectsManifestCasesHashMismatch(t *testing.T) {
	root := writeValidH1Fixture(t)
	manifest := readFixtureManifest(t, root)
	manifest.CasesSHA256 = strings.Repeat("0", 64)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)

	_, err := LoadH1(root)
	if err == nil || !strings.Contains(err.Error(), "cases") || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("expected cases hash mismatch, got %v", err)
	}
}

func TestLoadH1RejectsWrongChallengerSchedule(t *testing.T) {
	root := writeValidH1Fixture(t)
	doc := readFixtureCases(t, root)
	doc.Cases[0].ChallengerProvider = "codex"
	rewriteFixtureCasesAndManifest(t, root, doc)

	_, err := LoadH1(root)
	if err == nil || !strings.Contains(err.Error(), "challenger") {
		t.Fatalf("expected challenger schedule rejection, got %v", err)
	}
}

func TestLoadH1RejectsReferenceEvidenceNotVisibleToCandidate(t *testing.T) {
	root := writeValidH1Fixture(t)
	doc := readFixtureCases(t, root)
	doc.Cases[0].ReferenceSet = json.RawMessage(`{"evidence":[{"id":"hidden","claim":"secret","evaluation_note":"not candidate-visible"}]}`)
	doc.Cases[0].ReferenceSetSHA256 = digestCompact(t, doc.Cases[0].ReferenceSet)
	rewriteFixtureCasesAndManifest(t, root, doc)

	_, err := LoadH1(root)
	if err == nil || !strings.Contains(err.Error(), "reference evidence") {
		t.Fatalf("expected hidden reference evidence rejection, got %v", err)
	}
}

func TestLoadH1RejectsWrongCategorySplit(t *testing.T) {
	root := writeValidH1Fixture(t)
	doc := readFixtureCases(t, root)
	doc.Cases[9].Category = "product"
	rewriteFixtureCasesAndManifest(t, root, doc)

	_, err := LoadH1(root)
	if err == nil || !strings.Contains(err.Error(), "category") {
		t.Fatalf("expected category split rejection, got %v", err)
	}
}

func writeValidH1Fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	rubric := []byte(`{"schema_version":"council.h1-rubric.v0","overall_score_rule":"overall_score is the arithmetic mean of the five equally weighted dimension scores","dimensions":[{"id":"correctness_soundness","weight":1,"description":"sound"},{"id":"evidence_use","weight":1,"description":"evidence"},{"id":"risk_handling","weight":1,"description":"risk"},{"id":"actionability","weight":1,"description":"action"},{"id":"calibration","weight":1,"description":"calibration"}]}`)
	if err := os.WriteFile(filepath.Join(root, "rubric.json"), rubric, 0o600); err != nil {
		t.Fatal(err)
	}

	doc := fixtureCasesDocument{SchemaVersion: "council.h1-cases.v0", Cases: make([]fixtureCase, 0, 20)}
	for i, id := range fixtureCaseIDs {
		category := "technical"
		if i >= 10 {
			category = "product"
		}
		challenger := "claude"
		if (i+1)%2 == 0 {
			challenger = "codex"
		}
		problem := mustJSON(t, map[string]any{
			"title":       "Fixture " + id,
			"decision":    "Choose the best option.",
			"context":     []string{"Synthetic fixture context."},
			"constraints": []string{"Constraint one.", "Constraint two."},
			"options":     []string{"Option A", "Option B"},
			"evidence": []map[string]string{
				{"id": "e1", "fact": "Evidence one."},
				{"id": "e2", "fact": "Evidence two."},
			},
		})
		referenceSet := mustJSON(t, map[string]any{
			"evidence": []map[string]string{
				{"id": "e1", "claim": "Evidence one.", "evaluation_note": "Treat as verified."},
				{"id": "e2", "claim": "Evidence two.", "evaluation_note": "Treat as verified."},
			},
		})
		doc.Cases = append(doc.Cases, fixtureCase{
			ID:                 id,
			Category:           category,
			ChallengerProvider: challenger,
			Problem:            problem,
			ProblemSHA256:      digestCompact(t, problem),
			ReferenceSet:       referenceSet,
			ReferenceSetSHA256: digestCompact(t, referenceSet),
		})
	}

	casesBytes := writeJSONFile(t, filepath.Join(root, "cases.json"), doc)
	manifest := fixtureManifest{
		SchemaVersion:      "council.h1-dataset.v0",
		BenchmarkID:        "h1",
		CaseCount:          20,
		CategoryCounts:     map[string]int{"technical": 10, "product": 10},
		CaseIDs:            append([]string(nil), fixtureCaseIDs...),
		RubricSHA256:       digestBytes(rubric),
		CasesSHA256:        digestBytes(casesBytes),
		Comparator:         "best_single",
		MaterialWorseDelta: 10.0,
	}
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
	return root
}

func readFixtureManifest(t *testing.T, root string) fixtureManifest {
	t.Helper()
	var manifest fixtureManifest
	readJSONFile(t, filepath.Join(root, "manifest.json"), &manifest)
	return manifest
}

func readFixtureCases(t *testing.T, root string) fixtureCasesDocument {
	t.Helper()
	var doc fixtureCasesDocument
	readJSONFile(t, filepath.Join(root, "cases.json"), &doc)
	return doc
}

func rewriteFixtureCasesAndManifest(t *testing.T, root string, doc fixtureCasesDocument) {
	t.Helper()
	casesBytes := writeJSONFile(t, filepath.Join(root, "cases.json"), doc)
	manifest := readFixtureManifest(t, root)
	manifest.CasesSHA256 = digestBytes(casesBytes)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
}

func writeJSONFile(t *testing.T, path string, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return data
}

func readJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func digestCompact(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		t.Fatal(err)
	}
	return digestBytes(compact.Bytes())
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
