package evalharness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/modeloutput"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	"github.com/ShenJun93/agent-council/internal/council/visibility"
)

type CitationContract uint8

const (
	CitationContractLegacy CitationContract = iota
	CitationContractStructuredV1
)

type CitationKey struct {
	ArtifactID string `json:"artifact_id"`
	Locator    string `json:"locator"`
}

type H6CitationCheck struct {
	Reference CitationKey `json:"reference"`
	Status    string      `json:"status"`
	Note      string      `json:"note"`
}
type H6JudgeArtifact struct {
	OverallScore      float64            `json:"overall_score"`
	Dimensions        map[string]float64 `json:"dimensions"`
	CitationChecks    []H6CitationCheck  `json:"citation_checks"`
	ReliedOnCitations []CitationKey      `json:"relied_on_citations"`
	CriticalErrors    []string           `json:"critical_errors"`
	Strengths         []string           `json:"strengths"`
	Weaknesses        []string           `json:"weaknesses"`
	Confidence        float64            `json:"confidence"`
}

const h6JudgeInstruction = `EVALUATION_JUDGE
Score exactly one masked candidate against the normalized problem, frozen rubric, and frozen reference set. Do not infer the candidate arm, provider, other candidates, consensus, or another judge's verdict. Verify every citation against the visible artifact it names before relying on that citation. A citation that cannot be verified must not support the score. Return JSON only with exactly these keys: \"overall\u005fscore\" (0..100), \"dimensions\" (object mapping every rubric dimension id to 0..100, no extras), \"citation\u005fchecks\" ({reference:{\"artifact_id\":string,\"locator\":string},status,note}[]), \"relied\u005fon\u005fcitations\" ({\"artifact_id\":string,\"locator\":string}[]), \"critical\u005ferrors\" (string[]), \"strengths\" (string[]), \"weaknesses\" (string[]), \"confidence\" (0..1). Every citation reference must copy artifact_id and locator exactly from candidate.citations. Do not join, reformat, or paraphrase citation keys. Use status \"verified\" only when the cited claim matches the visible artifact.`

func validateCitationContract(contract CitationContract) error {
	switch contract {
	case CitationContractLegacy, CitationContractStructuredV1:
		return nil
	default:
		return fmt.Errorf("unsupported citation contract %d", contract)
	}
}

func decodeJudgeForContract(raw string, contract CitationContract, dimensions []string, candidate MaskedCandidate) (JudgeArtifact, error) {
	if contract == CitationContractLegacy {
		var artifact JudgeArtifact
		if err := decodeStrictJudgeJSON(raw, &artifact); err != nil {
			return JudgeArtifact{}, err
		}
		if err := validateJudgeArtifact(artifact, dimensions); err != nil {
			return JudgeArtifact{}, malformedEval(err)
		}
		if err := validateReliedCitations(artifact, candidate); err != nil {
			return JudgeArtifact{}, malformedEval(err)
		}
		return artifact, nil
	}
	var wire H6JudgeArtifact
	if err := modeloutput.DecodeStrict(raw, &wire); err != nil {
		return JudgeArtifact{}, malformedEval(fmt.Errorf("decode judge JSON: %w", err))
	}
	if err := validateH6CitationReferences(wire, candidate); err != nil {
		return JudgeArtifact{}, malformedEval(err)
	}
	artifact := canonicalizeH6JudgeArtifact(wire)
	if err := validateJudgeArtifact(artifact, dimensions); err != nil {
		return JudgeArtifact{}, malformedEval(err)
	}
	return artifact, nil
}
func validateH6CitationReferences(wire H6JudgeArtifact, candidate MaskedCandidate) error {
	candidateRefs := make(map[CitationKey]struct{}, len(candidate.Citations))
	for _, citation := range candidate.Citations {
		candidateRefs[CitationKey{ArtifactID: citation.ArtifactID, Locator: citation.Locator}] = struct{}{}
	}
	checks := make(map[CitationKey]string, len(wire.CitationChecks))
	for _, check := range wire.CitationChecks {
		if err := validateCitationKey(check.Reference); err != nil {
			return fmt.Errorf("citation check: %w", err)
		}
		if strings.TrimSpace(check.Status) == "" {
			return fmt.Errorf("citation check status is required")
		}
		if _, ok := candidateRefs[check.Reference]; !ok {
			return fmt.Errorf("citation check %q is not present in candidate", canonicalCitationKey(check.Reference))
		}
		if _, duplicate := checks[check.Reference]; duplicate {
			return fmt.Errorf("duplicate citation check %q", canonicalCitationKey(check.Reference))
		}
		checks[check.Reference] = strings.ToLower(strings.TrimSpace(check.Status))
	}
	seen := make(map[CitationKey]struct{}, len(wire.ReliedOnCitations))
	for _, reference := range wire.ReliedOnCitations {
		if err := validateCitationKey(reference); err != nil {
			return fmt.Errorf("relied-on citation: %w", err)
		}
		if _, duplicate := seen[reference]; duplicate {
			return fmt.Errorf("duplicate relied-on citation %q", canonicalCitationKey(reference))
		}
		seen[reference] = struct{}{}
		if _, ok := candidateRefs[reference]; !ok {
			return fmt.Errorf("relied-on citation %q is not present in candidate", canonicalCitationKey(reference))
		}
		if checks[reference] != "verified" {
			return fmt.Errorf("relied-on citation %q is not verified", canonicalCitationKey(reference))
		}
	}
	return nil
}

func validateCitationKey(key CitationKey) error {
	if strings.TrimSpace(key.ArtifactID) == "" || strings.TrimSpace(key.Locator) == "" {
		return fmt.Errorf("artifact_id and locator are required")
	}
	if strings.TrimSpace(key.ArtifactID) != key.ArtifactID || strings.TrimSpace(key.Locator) != key.Locator {
		return fmt.Errorf("artifact_id and locator must copy exactly without surrounding whitespace")
	}
	return nil
}
func canonicalizeH6JudgeArtifact(wire H6JudgeArtifact) JudgeArtifact {
	checks := make([]protocol.CitationCheck, len(wire.CitationChecks))
	for i, check := range wire.CitationChecks {
		checks[i] = protocol.CitationCheck{Reference: canonicalCitationKey(check.Reference), Status: check.Status, Note: check.Note}
	}
	relied := make([]string, len(wire.ReliedOnCitations))
	for i, reference := range wire.ReliedOnCitations {
		relied[i] = canonicalCitationKey(reference)
	}
	return JudgeArtifact{
		OverallScore: wire.OverallScore, Dimensions: wire.Dimensions,
		CitationChecks: checks, ReliedOnCitations: relied,
		CriticalErrors: wire.CriticalErrors, Strengths: wire.Strengths,
		Weaknesses: wire.Weaknesses, Confidence: wire.Confidence,
	}
}

func canonicalCitationKey(key CitationKey) string {
	return key.ArtifactID + ":" + key.Locator
}
func renderH6JudgePrompt(workspace visibility.Workspace, artifacts []visibility.Artifact) (string, error) {
	byID := make(map[string]visibility.Artifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	order := []string{"problem", "rubric", "reference-set", "candidate"}
	var b strings.Builder
	b.WriteString(h6JudgeInstruction)
	b.WriteString("\n\nVISIBLE_ARTIFACTS_BEGIN\n")
	for _, id := range order {
		artifact := byID[id]
		content, err := os.ReadFile(filepath.Join(workspace.Root, artifact.RelativePath))
		if err != nil {
			return "", fmt.Errorf("read materialized eval artifact %q: %w", id, err)
		}
		b.WriteString("--- artifact: ")
		b.WriteString(id)
		b.WriteString(" ---\n")
		b.Write(content)
		b.WriteByte('\n')
	}
	b.WriteString("VISIBLE_ARTIFACTS_END\n")
	return b.String(), nil
}
