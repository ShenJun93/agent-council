package benchmark

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

// h7DuplicateSourceRefs mirrors the real H6 arm-F failure evidence: two candidate
// citation occurrences that share artifact_id/locator but carry different claims.
var h7DuplicateSourceRefs = []protocol.EvidenceRef{
	{ArtifactID: "problem", Locator: "constraints[1]", Claim: "No committed write may be silently lost, the constraint dual-write most directly endangers."},
	{ArtifactID: "problem", Locator: "constraints[1]", Claim: "No committed write may be silently lost."},
}

type h7DuplicateSourceArmExecutor struct{}

func (h7DuplicateSourceArmExecutor) RunArm(_ context.Context, _ baseline.RunRequest, arm baseline.Arm) (baseline.ArmResult, error) {
	refs := append([]protocol.EvidenceRef(nil), h7DuplicateSourceRefs...)
	if arm == baseline.ArmAClaudeSingle || arm == baseline.ArmBCodexSingle || arm == baseline.ArmCClaudeSelfReview || arm == baseline.ArmDCodexSelfReview {
		a := &baseline.AnswerArtifact{Decision: "ship", Action: "act", Reasons: []string{"reason"}, Assumptions: []string{}, Risks: []string{}, Citations: refs, Confidence: .8}
		return baseline.ArmResult{Arm: arm, InvocationCount: 1, Answer: a}, nil
	}
	pr := &protocol.Result{
		Research: []protocol.ResearchRecord{
			{ID: "research-1", Artifact: protocol.ResearchArtifact{Risks: []string{"risk"}, Citations: []protocol.EvidenceRef{refs[0]}}},
			{ID: "research-2", Artifact: protocol.ResearchArtifact{Risks: []string{"risk"}, Citations: []protocol.EvidenceRef{refs[1]}}},
		},
		Judges:   []protocol.JudgeRecord{{ID: "judge-1", Artifact: protocol.JudgeArtifact{Action: "act", Reasons: []string{"reason"}, Evidence: []string{"evidence"}}}, {ID: "judge-2", Artifact: protocol.JudgeArtifact{Action: "act", Reasons: []string{"reason"}, Evidence: []string{"evidence"}}}},
		Decision: protocol.DecisionRecord{Status: protocol.DecisionAgreed, JudgeAgreement: true, JudgeDecisions: [2]string{"ship", "ship"}},
	}
	return baseline.ArmResult{Arm: arm, InvocationCount: 1, Protocol: pr}, nil
}

type h7DuplicateSourceJudgeRuntime struct{ provider councilruntime.Provider }

func (r h7DuplicateSourceJudgeRuntime) Run(_ context.Context, _ councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	out := `{"overall_score":80,"dimensions":{"correctness_soundness":80,"evidence_use":80,"risk_handling":80,"actionability":80,"calibration":80},"citation_checks":[` +
		`{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"No committed write may be silently lost, the constraint dual-write most directly endangers."},"status":"verified","note":"matches first occurrence"},` +
		`{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"No committed write may be silently lost."},"status":"verified","note":"matches second occurrence"}` +
		`],"relied_on_citations":[` +
		`{"artifact_id":"problem","locator":"constraints[1]","claim":"No committed write may be silently lost, the constraint dual-write most directly endangers."},` +
		`{"artifact_id":"problem","locator":"constraints[1]","claim":"No committed write may be silently lost."}` +
		`],"critical_errors":[],"strengths":["clear"],"weaknesses":[],"confidence":0.8}`
	return councilruntime.AgentResponse{Provider: r.provider, Stdout: out, ExitCode: 0, Attempts: 1}, nil
}

func TestH7RunnerAcceptsDuplicateSourceDifferentClaimCitationsThroughActualEvaluator(t *testing.T) {
	dataset, err := LoadH7(writeValidH7Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	evaluator := evalharness.Harness{
		Adaptive:         &evalharness.AdaptiveJudgeRuntimes{Judge1: h7DuplicateSourceJudgeRuntime{councilruntime.ProviderClaude}, Judge2: h7DuplicateSourceJudgeRuntime{councilruntime.ProviderCodex}},
		TempRoot:         t.TempDir(),
		CitationContract: evalharness.CitationContractStructuredV2,
	}

	result, err := runH7Problem(context.Background(), h7DuplicateSourceArmExecutor{}, evaluator, "h7-claim-aware-integration", t.TempDir(), dataset, dataset.Cases[0])
	if err != nil {
		t.Fatalf("H7 evaluation of duplicate-source/different-claim citations failed: %v", err)
	}

	wantOne, err := json.Marshal(evalharness.CitationOccurrenceKey{ArtifactID: "problem", Locator: "constraints[1]", Claim: h7DuplicateSourceRefs[0].Claim})
	if err != nil {
		t.Fatal(err)
	}
	wantTwo, err := json.Marshal(evalharness.CitationOccurrenceKey{ArtifactID: "problem", Locator: "constraints[1]", Claim: h7DuplicateSourceRefs[1].Claim})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Arms) != 6 {
		t.Fatalf("arm count=%d want 6", len(result.Arms))
	}
	for _, arm := range result.Arms {
		for _, judge := range arm.Judges {
			if judge.FailoverIndex != 0 || judge.FailoverTrigger != "" {
				t.Fatalf("arm %s judge %s unexpectedly failed over: index=%d trigger=%q", arm.Arm, judge.Slot, judge.FailoverIndex, judge.FailoverTrigger)
			}
			checks := judge.Artifact.CitationChecks
			if len(checks) != 2 {
				t.Fatalf("arm %s judge %s citation checks=%+v, want 2 distinct entries", arm.Arm, judge.Slot, checks)
			}
			if checks[0].Reference != string(wantOne) || checks[1].Reference != string(wantTwo) {
				t.Fatalf("arm %s judge %s citation checks=%+v, want canonical tuples %s and %s", arm.Arm, judge.Slot, checks, wantOne, wantTwo)
			}
			if len(judge.Artifact.ReliedOnCitations) != 2 {
				t.Fatalf("arm %s judge %s relied-on citations=%v, want 2 distinct entries", arm.Arm, judge.Slot, judge.Artifact.ReliedOnCitations)
			}
		}
	}
}
