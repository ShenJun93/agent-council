package evalharness

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func h7DuplicateSourceCandidate(t *testing.T) (ProblemRequest, preparedProblem, preparedArm) {
	t.Helper()
	candidate := MaskedCandidate{Decision: "ship", Citations: []protocol.EvidenceRef{
		{ArtifactID: "problem", Locator: "constraints[1]", Claim: "claim one"},
		{ArtifactID: "problem", Locator: "constraints[1]", Claim: "claim two"},
	}}
	content, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	problem := []byte(`{"constraints":["must be safe","no committed write may be silently lost"]}`)
	rubric := []byte(`{"dimensions":[{"id":"correctness"}]}`)
	reference := []byte(`{"facts":["reference"]}`)
	prepared := preparedProblem{problem: problem, rubric: rubric, referenceSet: reference, dimensions: []string{"correctness"}, problemHash: digestHex(problem), rubricHash: digestHex(rubric), referenceHash: digestHex(reference)}
	arm := preparedArm{arm: baseline.ArmAClaudeSingle, candidate: candidate, content: content}
	req := ProblemRequest{ProblemID: "p1", RunID: "h7-test", RunRoot: t.TempDir()}
	return req, prepared, arm
}

func TestH6RejectsDuplicateSourceDifferentClaimAsDuplicate(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]"},"status":"verified","note":"matched claim one"},{"reference":{"artifact_id":"problem","locator":"constraints[1]"},"status":"verified","note":"matched claim two"}],"relied_on_citations":[{"artifact_id":"problem","locator":"constraints[1]"}],"critical_errors":[],"strengths":["clear"],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV1, TempRoot: t.TempDir()}
	_, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err == nil || !strings.Contains(err.Error(), "duplicate citation check") {
		t.Fatalf("expected H6 V1 to reject the second occurrence as a duplicate source key, got %v", err)
	}
}

func TestH7AcceptsBothClaimDistinctCitationsAndSurvivesSerialization(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},"status":"verified","note":"matched"},{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim two"},"status":"verified","note":"matched"}],"relied_on_citations":[{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},{"artifact_id":"problem","locator":"constraints[1]","claim":"claim two"}],"critical_errors":[],"strengths":["clear"],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV2, TempRoot: t.TempDir()}
	score, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err != nil {
		t.Fatalf("evaluateCandidate() error = %v", err)
	}

	wantOne, err := json.Marshal(CitationOccurrenceKey{ArtifactID: "problem", Locator: "constraints[1]", Claim: "claim one"})
	if err != nil {
		t.Fatal(err)
	}
	wantTwo, err := json.Marshal(CitationOccurrenceKey{ArtifactID: "problem", Locator: "constraints[1]", Claim: "claim two"})
	if err != nil {
		t.Fatal(err)
	}

	gotChecks := score.Artifact.CitationChecks
	if len(gotChecks) != 2 {
		t.Fatalf("citation checks = %+v, want 2 distinct entries", gotChecks)
	}
	if gotChecks[0].Reference != string(wantOne) || gotChecks[1].Reference != string(wantTwo) {
		t.Fatalf("citation checks = %+v, want canonical tuples %s and %s", gotChecks, wantOne, wantTwo)
	}

	gotRelied := score.Artifact.ReliedOnCitations
	if len(gotRelied) != 2 || gotRelied[0] != string(wantOne) || gotRelied[1] != string(wantTwo) {
		t.Fatalf("relied-on citations = %v, want %s and %s", gotRelied, wantOne, wantTwo)
	}

	calls := rt.snapshot()
	if len(calls) != 1 || !strings.Contains(calls[0].Prompt, `"claim"`) {
		t.Fatalf("H7 prompt missing claim field in citation contract: %q", calls[0].Prompt)
	}
}

func TestH7RejectsUnknownClaimWithValidArtifactAndLocator(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim never stated by candidate"},"status":"verified","note":"hallucinated claim"}],"relied_on_citations":[],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV2, TempRoot: t.TempDir()}
	_, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err == nil || !strings.Contains(err.Error(), "citation check") || !strings.Contains(err.Error(), "not present in candidate") {
		t.Fatalf("expected unknown-claim citation-check rejection, got %v", err)
	}
}

func TestH7RejectsDuplicateIdenticalFullTupleInCitationChecks(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},"status":"verified","note":"matched"},{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},"status":"verified","note":"matched again"}],"relied_on_citations":[],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV2, TempRoot: t.TempDir()}
	_, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err == nil || !strings.Contains(err.Error(), "duplicate citation check") {
		t.Fatalf("expected duplicate full-tuple citation-check rejection, got %v", err)
	}
}

func TestH7RejectsDuplicateIdenticalFullTupleInReliedOnCitations(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},"status":"verified","note":"matched"}],"relied_on_citations":[{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"}],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV2, TempRoot: t.TempDir()}
	_, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err == nil || !strings.Contains(err.Error(), "duplicate relied-on citation") {
		t.Fatalf("expected duplicate full-tuple relied-on rejection, got %v", err)
	}
}

func TestH7RejectsReliedOnTupleWithoutMatchingVerifiedCheck(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},"status":"unverified","note":"could not confirm"}],"relied_on_citations":[{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"}],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV2, TempRoot: t.TempDir()}
	_, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err == nil || !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("expected unverified relied-on rejection, got %v", err)
	}
}

func TestH7RejectsWhitespaceMutatedClaim(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":" claim one"},"status":"verified","note":"mutated"}],"relied_on_citations":[],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV2, TempRoot: t.TempDir()}
	_, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err == nil || !strings.Contains(err.Error(), "copy exactly") {
		t.Fatalf("expected exact-tuple rejection for whitespace-mutated claim, got %v", err)
	}
}

func TestH7RejectsEmptyClaim(t *testing.T) {
	req, prepared, arm := h7DuplicateSourceCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":""},"status":"verified","note":"empty"}],"relied_on_citations":[],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV2, TempRoot: t.TempDir()}
	_, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected required-field rejection for empty claim, got %v", err)
	}
}
