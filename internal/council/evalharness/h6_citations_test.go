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

func h6PreparedCandidate(t *testing.T) (ProblemRequest, preparedProblem, preparedArm) {
	t.Helper()
	candidate := MaskedCandidate{Decision: "ship", Citations: []protocol.EvidenceRef{{ArtifactID: "problem", Locator: "constraints[0]", Claim: "constraint"}}}
	content, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	problem := []byte(`{"constraints":["must be safe"]}`)
	rubric := []byte(`{"dimensions":[{"id":"correctness"}]}`)
	reference := []byte(`{"facts":["reference"]}`)
	prepared := preparedProblem{problem: problem, rubric: rubric, referenceSet: reference, dimensions: []string{"correctness"}, problemHash: digestHex(problem), rubricHash: digestHex(rubric), referenceHash: digestHex(reference)}
	arm := preparedArm{arm: baseline.ArmAClaudeSingle, candidate: candidate, content: content}
	req := ProblemRequest{ProblemID: "p1", RunID: "h6-test", RunRoot: t.TempDir()}
	return req, prepared, arm
}
func TestH6TypedCitationKeysAcceptCanonicalCandidateReference(t *testing.T) {
	req, prepared, arm := h6PreparedCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[0]"},"status":"verified","note":"matched"}],"relied_on_citations":[{"artifact_id":"problem","locator":"constraints[0]"}],"critical_errors":[],"strengths":["clear"],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV1, TempRoot: t.TempDir()}
	score, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err != nil {
		t.Fatalf("evaluateCandidate() error = %v", err)
	}
	if got := score.Artifact.ReliedOnCitations; len(got) != 1 || got[0] != "problem:constraints[0]" {
		t.Fatalf("relied citations = %v", got)
	}
	if got := score.Artifact.CitationChecks; len(got) != 1 || got[0].Reference != "problem:constraints[0]" {
		t.Fatalf("citation checks = %+v", got)
	}
	calls := rt.snapshot()
	if len(calls) != 1 || !strings.Contains(calls[0].Prompt, `"artifact_id"`) || !strings.Contains(calls[0].Prompt, `"locator"`) {
		t.Fatalf("H6 prompt missing typed citation contract: %q", calls[0].Prompt)
	}
}

func TestH6RejectsLegacyFreeFormCitationStrings(t *testing.T) {
	req, prepared, arm := h6PreparedCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":"problem:constraints[0]","status":"verified","note":"matched"}],"relied_on_citations":["problem:constraints[0]"],"critical_errors":[],"strengths":["clear"],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV1, TempRoot: t.TempDir()}
	_, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err == nil || !strings.Contains(err.Error(), "decode judge JSON") {
		t.Fatalf("expected typed decode rejection, got %v", err)
	}
}

func TestH6RejectsCitationCheckOutsideCandidateSet(t *testing.T) {
	req, prepared, arm := h6PreparedCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[1]"},"status":"verified","note":"wrong candidate key"}],"relied_on_citations":[],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV1, TempRoot: t.TempDir()}
	_, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err == nil || !strings.Contains(err.Error(), "citation check") || !strings.Contains(err.Error(), "not present in candidate") {
		t.Fatalf("expected unknown citation-check rejection, got %v", err)
	}
}

func TestH6RejectsWhitespaceMutatedCitationKey(t *testing.T) {
	req, prepared, arm := h6PreparedCandidate(t)
	out := `{"overall_score":80,"dimensions":{"correctness":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":" constraints[0]"},"status":"verified","note":"mutated"}],"relied_on_citations":[],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	rt := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: out}
	h := Harness{CitationContract: CitationContractStructuredV1, TempRoot: t.TempDir()}
	_, err := h.evaluateCandidate(context.Background(), req, prepared, arm, "judge-1", "eval-judge-1", "", false, rt)
	if err == nil || !strings.Contains(err.Error(), "copy exactly") {
		t.Fatalf("expected exact-key rejection, got %v", err)
	}
}
