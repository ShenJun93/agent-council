package benchmark

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const h1OverallScoreRule = "overall_score is the arithmetic mean of the five equally weighted dimension scores"

var h1CaseIDs = []string{
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

var h1RubricDimensionIDs = []string{
	"correctness_soundness",
	"evidence_use",
	"risk_handling",
	"actionability",
	"calibration",
}

type casesDocument struct {
	SchemaVersion string         `json:"schema_version"`
	Cases         []caseEnvelope `json:"cases"`
}

type caseEnvelope struct {
	ID                 string          `json:"id"`
	Category           string          `json:"category"`
	ChallengerProvider string          `json:"challenger_provider"`
	Problem            json.RawMessage `json:"problem"`
	ProblemSHA256      string          `json:"problem_sha256"`
	ReferenceSet       json.RawMessage `json:"reference_set"`
	ReferenceSetSHA256 string          `json:"reference_set_sha256"`
}

type problemDocument struct {
	Title       string            `json:"title"`
	Decision    string            `json:"decision"`
	Context     []string          `json:"context"`
	Constraints []string          `json:"constraints"`
	Options     []string          `json:"options"`
	Evidence    []problemEvidence `json:"evidence"`
}

type problemEvidence struct {
	ID   string `json:"id"`
	Fact string `json:"fact"`
}

type referenceDocument struct {
	Evidence []referenceEvidence `json:"evidence"`
}

type referenceEvidence struct {
	ID             string `json:"id"`
	Claim          string `json:"claim"`
	EvaluationNote string `json:"evaluation_note"`
}

type rubricDocument struct {
	SchemaVersion    string            `json:"schema_version"`
	OverallScoreRule string            `json:"overall_score_rule"`
	Dimensions       []rubricDimension `json:"dimensions"`
}

type rubricDimension struct {
	ID          string  `json:"id"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

func LoadH1(root string) (Dataset, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" || root == "." && strings.TrimSpace(root) == "" {
		return Dataset{}, fmt.Errorf("H1 dataset root is required")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return Dataset{}, fmt.Errorf("stat H1 dataset root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Dataset{}, fmt.Errorf("H1 dataset root must be a real directory")
	}

	manifestBytes, err := readDatasetFile(root, "manifest.json")
	if err != nil {
		return Dataset{}, err
	}
	rubricBytes, err := readDatasetFile(root, "rubric.json")
	if err != nil {
		return Dataset{}, err
	}
	casesBytes, err := readDatasetFile(root, "cases.json")
	if err != nil {
		return Dataset{}, err
	}

	var manifest Manifest
	if err := decodeStrict("manifest.json", manifestBytes, &manifest); err != nil {
		return Dataset{}, err
	}
	if err := validateManifest(manifest, rubricBytes, casesBytes); err != nil {
		return Dataset{}, err
	}
	if err := validateRubric(rubricBytes); err != nil {
		return Dataset{}, err
	}

	var document casesDocument
	if err := decodeStrict("cases.json", casesBytes, &document); err != nil {
		return Dataset{}, err
	}
	if document.SchemaVersion != H1CasesSchemaVersion {
		return Dataset{}, fmt.Errorf("cases schema_version %q, want %q", document.SchemaVersion, H1CasesSchemaVersion)
	}
	if len(document.Cases) != H1CaseCount {
		return Dataset{}, fmt.Errorf("cases count %d, want %d", len(document.Cases), H1CaseCount)
	}

	cases := make([]Case, 0, H1CaseCount)
	technical := 0
	product := 0
	seen := make(map[string]struct{}, H1CaseCount)
	for index, envelope := range document.Cases {
		wantID := h1CaseIDs[index]
		if envelope.ID != wantID {
			return Dataset{}, fmt.Errorf("case order mismatch at global index %d: got %q want %q", index+1, envelope.ID, wantID)
		}
		if !safeDatasetID(envelope.ID) {
			return Dataset{}, fmt.Errorf("unsafe case id %q", envelope.ID)
		}
		if _, duplicate := seen[envelope.ID]; duplicate {
			return Dataset{}, fmt.Errorf("duplicate case id %q", envelope.ID)
		}
		seen[envelope.ID] = struct{}{}

		wantCategory := "technical"
		if index >= H1TechnicalCount {
			wantCategory = "product"
		}
		if envelope.Category != wantCategory {
			return Dataset{}, fmt.Errorf("case %q category %q, want %q", envelope.ID, envelope.Category, wantCategory)
		}
		if envelope.Category == "technical" {
			technical++
		} else {
			product++
		}

		wantChallenger := "claude"
		if (index+1)%2 == 0 {
			wantChallenger = "codex"
		}
		if envelope.ChallengerProvider != wantChallenger {
			return Dataset{}, fmt.Errorf("case %q challenger %q, want %q for global index %d", envelope.ID, envelope.ChallengerProvider, wantChallenger, index+1)
		}

		problem, problemEvidenceIDs, err := validateProblem(envelope.ID, envelope.Problem)
		if err != nil {
			return Dataset{}, err
		}
		if err := verifyDigest(envelope.ID+" problem", problem, envelope.ProblemSHA256); err != nil {
			return Dataset{}, err
		}
		referenceSet, err := validateReferenceSet(envelope.ID, envelope.ReferenceSet, problemEvidenceIDs)
		if err != nil {
			return Dataset{}, err
		}
		if err := verifyDigest(envelope.ID+" reference set", referenceSet, envelope.ReferenceSetSHA256); err != nil {
			return Dataset{}, err
		}

		cases = append(cases, Case{
			ID:                 envelope.ID,
			Category:           envelope.Category,
			ChallengerProvider: providerFromString(envelope.ChallengerProvider),
			Problem:            append(json.RawMessage(nil), problem...),
			ProblemSHA256:      strings.ToLower(envelope.ProblemSHA256),
			ReferenceSet:       append(json.RawMessage(nil), referenceSet...),
			ReferenceSetSHA256: strings.ToLower(envelope.ReferenceSetSHA256),
		})
	}
	if technical != H1TechnicalCount || product != H1ProductCount {
		return Dataset{}, fmt.Errorf("category split technical=%d product=%d, want %d/%d", technical, product, H1TechnicalCount, H1ProductCount)
	}

	return Dataset{
		Root:          root,
		Manifest:      manifest,
		ManifestBytes: append([]byte(nil), manifestBytes...),
		Rubric:        append(json.RawMessage(nil), rubricBytes...),
		RubricSHA256:  strings.ToLower(manifest.RubricSHA256),
		CasesBytes:    append([]byte(nil), casesBytes...),
		Cases:         cases,
	}, nil
}

func validateManifest(manifest Manifest, rubricBytes, casesBytes []byte) error {
	if manifest.SchemaVersion != H1DatasetSchemaVersion {
		return fmt.Errorf("manifest schema_version %q, want %q", manifest.SchemaVersion, H1DatasetSchemaVersion)
	}
	if manifest.BenchmarkID != H1BenchmarkID {
		return fmt.Errorf("manifest benchmark_id %q, want %q", manifest.BenchmarkID, H1BenchmarkID)
	}
	if manifest.CaseCount != H1CaseCount {
		return fmt.Errorf("manifest case_count %d, want %d", manifest.CaseCount, H1CaseCount)
	}
	if len(manifest.CategoryCounts) != 2 || manifest.CategoryCounts["technical"] != H1TechnicalCount || manifest.CategoryCounts["product"] != H1ProductCount {
		return fmt.Errorf("manifest category_counts must be exactly technical=%d product=%d", H1TechnicalCount, H1ProductCount)
	}
	if len(manifest.CaseIDs) != len(h1CaseIDs) {
		return fmt.Errorf("manifest case_ids count %d, want %d", len(manifest.CaseIDs), len(h1CaseIDs))
	}
	for i, id := range h1CaseIDs {
		if manifest.CaseIDs[i] != id {
			return fmt.Errorf("manifest case order mismatch at global index %d: got %q want %q", i+1, manifest.CaseIDs[i], id)
		}
	}
	if manifest.Comparator != H1RiskPolicy.Comparator {
		return fmt.Errorf("manifest comparator %q, want %q", manifest.Comparator, H1RiskPolicy.Comparator)
	}
	if manifest.MaterialWorseDelta != H1RiskPolicy.MaterialWorseDelta {
		return fmt.Errorf("manifest material_worse_delta %.4f, want %.4f", manifest.MaterialWorseDelta, H1RiskPolicy.MaterialWorseDelta)
	}
	if err := verifyDigest("rubric file", rubricBytes, manifest.RubricSHA256); err != nil {
		return err
	}
	if err := verifyDigest("cases file", casesBytes, manifest.CasesSHA256); err != nil {
		return err
	}
	return nil
}

func validateRubric(raw []byte) error {
	var rubric rubricDocument
	if err := decodeStrict("rubric.json", raw, &rubric); err != nil {
		return err
	}
	if rubric.SchemaVersion != H1RubricSchemaVersion {
		return fmt.Errorf("rubric schema_version %q, want %q", rubric.SchemaVersion, H1RubricSchemaVersion)
	}
	if rubric.OverallScoreRule != h1OverallScoreRule {
		return fmt.Errorf("rubric overall_score_rule differs from frozen H1 rule")
	}
	if len(rubric.Dimensions) != len(h1RubricDimensionIDs) {
		return fmt.Errorf("rubric dimension count %d, want %d", len(rubric.Dimensions), len(h1RubricDimensionIDs))
	}
	for i, dimension := range rubric.Dimensions {
		if dimension.ID != h1RubricDimensionIDs[i] {
			return fmt.Errorf("rubric dimension %d id %q, want %q", i, dimension.ID, h1RubricDimensionIDs[i])
		}
		if dimension.Weight != 1 {
			return fmt.Errorf("rubric dimension %q weight %.4f, want 1", dimension.ID, dimension.Weight)
		}
		if strings.TrimSpace(dimension.Description) == "" {
			return fmt.Errorf("rubric dimension %q description is required", dimension.ID)
		}
	}
	return nil
}

func validateProblem(caseID string, raw json.RawMessage) ([]byte, map[string]struct{}, error) {
	compact, err := compactJSONObject(caseID+" problem", raw)
	if err != nil {
		return nil, nil, err
	}
	var problem problemDocument
	if err := decodeStrict(caseID+" problem", compact, &problem); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(problem.Title) == "" || strings.TrimSpace(problem.Decision) == "" {
		return nil, nil, fmt.Errorf("case %q problem title and decision are required", caseID)
	}
	if len(problem.Context) == 0 {
		return nil, nil, fmt.Errorf("case %q problem context is required", caseID)
	}
	if len(problem.Constraints) < 2 {
		return nil, nil, fmt.Errorf("case %q problem requires at least two constraints", caseID)
	}
	if len(problem.Options) < 2 {
		return nil, nil, fmt.Errorf("case %q problem requires at least two options", caseID)
	}
	if len(problem.Evidence) == 0 {
		return nil, nil, fmt.Errorf("case %q problem evidence is required", caseID)
	}

	ids := make(map[string]struct{}, len(problem.Evidence))
	for _, evidence := range problem.Evidence {
		if !safeDatasetID(evidence.ID) || strings.TrimSpace(evidence.Fact) == "" {
			return nil, nil, fmt.Errorf("case %q has invalid problem evidence %q", caseID, evidence.ID)
		}
		if _, duplicate := ids[evidence.ID]; duplicate {
			return nil, nil, fmt.Errorf("case %q has duplicate problem evidence id %q", caseID, evidence.ID)
		}
		ids[evidence.ID] = struct{}{}
	}
	return compact, ids, nil
}

func validateReferenceSet(caseID string, raw json.RawMessage, problemEvidenceIDs map[string]struct{}) ([]byte, error) {
	compact, err := compactJSONObject(caseID+" reference set", raw)
	if err != nil {
		return nil, err
	}
	var reference referenceDocument
	if err := decodeStrict(caseID+" reference set", compact, &reference); err != nil {
		return nil, err
	}
	if len(reference.Evidence) != len(problemEvidenceIDs) {
		return nil, fmt.Errorf("case %q reference evidence count %d does not mirror %d problem evidence ids", caseID, len(reference.Evidence), len(problemEvidenceIDs))
	}
	seen := make(map[string]struct{}, len(reference.Evidence))
	for _, evidence := range reference.Evidence {
		if _, visible := problemEvidenceIDs[evidence.ID]; !visible {
			return nil, fmt.Errorf("case %q reference evidence %q is not visible in candidate problem evidence", caseID, evidence.ID)
		}
		if _, duplicate := seen[evidence.ID]; duplicate {
			return nil, fmt.Errorf("case %q has duplicate reference evidence id %q", caseID, evidence.ID)
		}
		if strings.TrimSpace(evidence.Claim) == "" || strings.TrimSpace(evidence.EvaluationNote) == "" {
			return nil, fmt.Errorf("case %q reference evidence %q requires claim and evaluation_note", caseID, evidence.ID)
		}
		seen[evidence.ID] = struct{}{}
	}
	for id := range problemEvidenceIDs {
		if _, ok := seen[id]; !ok {
			return nil, fmt.Errorf("case %q reference set is missing problem evidence id %q", caseID, id)
		}
	}
	return compact, nil
}

func compactJSONObject(label string, raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("%s is required", label)
	}
	var object map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", label, err)
	}
	if len(object) == 0 {
		return nil, fmt.Errorf("%s must be a non-empty JSON object", label)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s must contain exactly one JSON object", label)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, fmt.Errorf("compact %s: %w", label, err)
	}
	return compact.Bytes(), nil
}

func decodeStrict(label string, raw []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode %s: multiple JSON values", label)
		}
		return fmt.Errorf("decode %s trailing data: %w", label, err)
	}
	return nil
}

func readDatasetFile(root, name string) ([]byte, error) {
	path := filepath.Join(root, name)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("stat H1 dataset file %q: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("H1 dataset file %q must be a regular non-symlink file", name)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read H1 dataset file %q: %w", name, err)
	}
	return data, nil
}

func verifyDigest(label string, data []byte, expected string) error {
	if len(expected) != 64 {
		return fmt.Errorf("%s hash must be a 64-character SHA-256 digest", label)
	}
	actual := sha256Hex(data)
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("%s hash mismatch: got %s want %s", label, actual, strings.ToLower(expected))
	}
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func safeDatasetID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func providerFromString(provider string) councilruntime.Provider {
	if provider == "claude" {
		return councilruntime.ProviderClaude
	}
	return councilruntime.ProviderCodex
}
