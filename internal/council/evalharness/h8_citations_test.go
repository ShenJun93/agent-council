package evalharness

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestH8RejectsUnverifiedReliedOnCitation(t *testing.T) {
	_, _, arm := h7DuplicateSourceCandidate(t)
	key := CitationOccurrenceKey{ArtifactID: "problem", Locator: "constraints[1]", Claim: "claim one"}
	wire := H8JudgeArtifact{CitationChecks: []H8CitationCheck{{Reference: key, Status: "unverified", ReliedOn: true, Note: "partially supported inference"}}}
	if err := validateH8CitationReferences(wire, arm.candidate); err == nil || !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("expected contradictory score reliance to fail closed, got %v", err)
	}
}

func TestH8DerivesReliedOnOnlyFromVerifiedCheck(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},"status":"verified","relied_on":true,"note":"direct support"}],"critical_errors":[],"strengths":["clear"],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{output: out}
	h := Harness{CitationContract: CitationContractStructuredV3, TempRoot: t.TempDir()}
	score, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err != nil {
		t.Fatalf("evaluateCandidate() error = %v", err)
	}
	want, _ := json.Marshal(CitationOccurrenceKey{ArtifactID: "problem", Locator: "constraints[1]", Claim: "claim one"})
	if len(score.Artifact.ReliedOnCitations) != 1 || score.Artifact.ReliedOnCitations[0] != string(want) {
		t.Fatalf("relied-on citations = %v, want %s", score.Artifact.ReliedOnCitations, want)
	}
}

func TestH8UnverifiedNotReliedOnIsScoreable(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":55,"dimensions":{"correctness":55},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},"status":"unverified","relied_on":false,"note":"partially supported; full claim is inference"}],"critical_errors":[],"strengths":[],"weaknesses":["overclaims evidence"],"confidence":0.7}`
	rt := &fakeJudgeRuntime{output: out}
	h := Harness{CitationContract: CitationContractStructuredV3, TempRoot: t.TempDir()}
	score, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err != nil {
		t.Fatalf("evaluateCandidate() error = %v", err)
	}
	if len(score.Artifact.ReliedOnCitations) != 0 {
		t.Fatalf("unverified citation unexpectedly became score reliance: %v", score.Artifact.ReliedOnCitations)
	}
}

func TestH8PreservesClaimDistinctChecks(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},"status":"verified","relied_on":true,"note":"matched"},{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim two"},"status":"verified","relied_on":false,"note":"matched"}],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{output: out}
	h := Harness{CitationContract: CitationContractStructuredV3, TempRoot: t.TempDir()}
	score, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err != nil {
		t.Fatalf("evaluateCandidate() error = %v", err)
	}
	if len(score.Artifact.CitationChecks) != 2 {
		t.Fatalf("citation checks = %+v, want two claim-distinct occurrences", score.Artifact.CitationChecks)
	}
}

func TestH8PromptDefinesJudgeScoreReliance(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{output: out}
	h := Harness{CitationContract: CitationContractStructuredV3, TempRoot: t.TempDir()}
	if _, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt); err != nil {
		t.Fatalf("evaluateCandidate() error = %v", err)
	}
	calls := rt.snapshot()
	if len(calls) != 1 {
		t.Fatalf("runtime calls = %d, want 1", len(calls))
	}
	prompt := calls[0].Prompt
	for _, phrase := range []string{"evaluation judge", `"verified" or "unverified"`, `"relied_on"`, "partially supported"} {
		if !strings.Contains(strings.ToLower(prompt), strings.ToLower(phrase)) {
			t.Fatalf("H8 prompt missing %q: %q", phrase, prompt)
		}
	}
}
