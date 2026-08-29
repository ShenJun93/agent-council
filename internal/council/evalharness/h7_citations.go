package evalharness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/modeloutput"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	"github.com/ShenJun93/agent-council/internal/council/visibility"
)

type CitationOccurrenceKey struct {
	ArtifactID string `json:"artifact_id"`
	Locator    string `json:"locator"`
	Claim      string `json:"claim"`
}

type H7CitationCheck struct {
	Reference CitationOccurrenceKey `json:"reference"`
	Status    string                `json:"status"`
	Note      string                `json:"note"`
}

type H7JudgeArtifact struct {
	OverallScore      float64                 `json:"overall_score"`
	Dimensions        map[string]float64      `json:"dimensions"`
	CitationChecks    []H7CitationCheck       `json:"citation_checks"`
	ReliedOnCitations []CitationOccurrenceKey `json:"relied_on_citations"`
	CriticalErrors    []string                `json:"critical_errors"`
	Strengths         []string                `json:"strengths"`
	Weaknesses        []string                `json:"weaknesses"`
	Confidence        float64                 `json:"confidence"`
}

const h7JudgeInstruction = `EVALUATION_JUDGE
Score exactly one masked candidate against the normalized problem, frozen rubric, and frozen reference set. Do not infer the candidate arm, provider, other candidates, consensus, or another judge's verdict. Verify every citation against the visible artifact it names before relying on that citation. A citation that cannot be verified must not support the score. Return JSON only with exactly these keys: \"overall_score\" (0..100), \"dimensions\" (object mapping every rubric dimension id to 0..100, no extras), \"citation_checks\" ({reference:{\"artifact_id\":string,\"locator\":string,\"claim\":string},status,note}[]), \"relied_on_citations\" ({\"artifact_id\":string,\"locator\":string,\"claim\":string}[]), \"critical_errors\" (string[]), \"strengths\" (string[]), \"weaknesses\" (string[]), \"confidence\" (0..1). Every citation reference must copy artifact_id, locator, and claim exactly from candidate.citations. Two candidate citations may share artifact_id and locator with different claims; treat each full tuple as a distinct citation occurrence. Do not join, reformat, paraphrase, deduplicate by source location, or merge claims. Use status \"verified\" only when the cited claim matches the visible artifact.`

func decodeH7Judge(raw string, dimensions []string, candidate MaskedCandidate) (JudgeArtifact, error) {
	var wire H7JudgeArtifact
	if err := modeloutput.DecodeStrict(raw, &wire); err != nil {
		return JudgeArtifact{}, malformedEval(fmt.Errorf("decode judge JSON: %w", err))
	}
	if err := validateH7CitationReferences(wire, candidate); err != nil {
		return JudgeArtifact{}, malformedEval(err)
	}
	artifact, err := canonicalizeH7JudgeArtifact(wire)
	if err != nil {
		return JudgeArtifact{}, malformedEval(err)
	}
	if err := validateJudgeArtifact(artifact, dimensions); err != nil {
		return JudgeArtifact{}, malformedEval(err)
	}
	return artifact, nil
}

func validateH7CitationReferences(wire H7JudgeArtifact, candidate MaskedCandidate) error {
	candidateRefs := make(map[CitationOccurrenceKey]struct{}, len(candidate.Citations))
	for _, citation := range candidate.Citations {
		candidateRefs[evidenceRefToOccurrenceKey(citation)] = struct{}{}
	}
	checks := make(map[CitationOccurrenceKey]string, len(wire.CitationChecks))
	for _, check := range wire.CitationChecks {
		_, member := candidateRefs[check.Reference]
		if !member {
			if err := validateCitationOccurrenceKey(check.Reference); err != nil {
				return fmt.Errorf("citation check: %w", err)
			}
		}
		if strings.TrimSpace(check.Status) == "" {
			return fmt.Errorf("citation check status is required")
		}
		if !member {
			return fmt.Errorf("citation check %q is not present in candidate", canonicalOccurrenceString(check.Reference))
		}
		if _, duplicate := checks[check.Reference]; duplicate {
			return fmt.Errorf("duplicate citation check %q", canonicalOccurrenceString(check.Reference))
		}
		checks[check.Reference] = strings.ToLower(strings.TrimSpace(check.Status))
	}
	seen := make(map[CitationOccurrenceKey]struct{}, len(wire.ReliedOnCitations))
	for _, reference := range wire.ReliedOnCitations {
		if _, ok := candidateRefs[reference]; !ok {
			if err := validateCitationOccurrenceKey(reference); err != nil {
				return fmt.Errorf("relied-on citation: %w", err)
			}
			return fmt.Errorf("relied-on citation %q is not present in candidate", canonicalOccurrenceString(reference))
		}
		if _, duplicate := seen[reference]; duplicate {
			return fmt.Errorf("duplicate relied-on citation %q", canonicalOccurrenceString(reference))
		}
		seen[reference] = struct{}{}
		if checks[reference] != "verified" {
			return fmt.Errorf("relied-on citation %q is not verified", canonicalOccurrenceString(reference))
		}
	}
	return nil
}

func validateCitationOccurrenceKey(key CitationOccurrenceKey) error {
	if strings.TrimSpace(key.ArtifactID) == "" || strings.TrimSpace(key.Locator) == "" || strings.TrimSpace(key.Claim) == "" {
		return fmt.Errorf("artifact_id, locator, and claim are required")
	}
	if strings.TrimSpace(key.ArtifactID) != key.ArtifactID || strings.TrimSpace(key.Locator) != key.Locator || strings.TrimSpace(key.Claim) != key.Claim {
		return fmt.Errorf("artifact_id, locator, and claim must copy exactly without surrounding whitespace")
	}
	return nil
}

func canonicalizeH7JudgeArtifact(wire H7JudgeArtifact) (JudgeArtifact, error) {
	checks := make([]protocol.CitationCheck, len(wire.CitationChecks))
	for i, check := range wire.CitationChecks {
		reference, err := canonicalOccurrenceJSON(check.Reference)
		if err != nil {
			return JudgeArtifact{}, err
		}
		checks[i] = protocol.CitationCheck{Reference: reference, Status: check.Status, Note: check.Note}
	}
	relied := make([]string, len(wire.ReliedOnCitations))
	for i, reference := range wire.ReliedOnCitations {
		encoded, err := canonicalOccurrenceJSON(reference)
		if err != nil {
			return JudgeArtifact{}, err
		}
		relied[i] = encoded
	}
	return JudgeArtifact{
		OverallScore: wire.OverallScore, Dimensions: wire.Dimensions,
		CitationChecks: checks, ReliedOnCitations: relied,
		CriticalErrors: wire.CriticalErrors, Strengths: wire.Strengths,
		Weaknesses: wire.Weaknesses, Confidence: wire.Confidence,
	}, nil
}

func canonicalOccurrenceJSON(key CitationOccurrenceKey) (string, error) {
	encoded, err := json.Marshal(key)
	if err != nil {
		return "", fmt.Errorf("marshal citation occurrence key: %w", err)
	}
	return string(encoded), nil
}

func canonicalOccurrenceString(key CitationOccurrenceKey) string {
	encoded, err := canonicalOccurrenceJSON(key)
	if err != nil {
		return key.ArtifactID + ":" + key.Locator + ":" + key.Claim
	}
	return encoded
}

func evidenceRefToOccurrenceKey(ref protocol.EvidenceRef) CitationOccurrenceKey {
	return CitationOccurrenceKey{ArtifactID: ref.ArtifactID, Locator: ref.Locator, Claim: ref.Claim}
}

func renderH7JudgePrompt(workspace visibility.Workspace, artifacts []visibility.Artifact) (string, error) {
	byID := make(map[string]visibility.Artifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	order := []string{"problem", "rubric", "reference-set", "candidate"}
	var b strings.Builder
	b.WriteString(h7JudgeInstruction)
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
