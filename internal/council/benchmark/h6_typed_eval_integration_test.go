package benchmark

import (
	"context"
	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"testing"
)

type h6TypedArmExecutor struct{}

func (h6TypedArmExecutor) RunArm(_ context.Context, _ baseline.RunRequest, arm baseline.Arm) (baseline.ArmResult, error) {
	ref := protocol.EvidenceRef{ArtifactID: "problem", Locator: "constraints[0]", Claim: "constraint"}
	if arm == baseline.ArmAClaudeSingle || arm == baseline.ArmBCodexSingle || arm == baseline.ArmCClaudeSelfReview || arm == baseline.ArmDCodexSelfReview {
		a := &baseline.AnswerArtifact{Decision: "ship", Action: "act", Reasons: []string{"reason"}, Assumptions: []string{}, Risks: []string{}, Citations: []protocol.EvidenceRef{ref}, Confidence: .8}
		return baseline.ArmResult{Arm: arm, InvocationCount: 1, Answer: a}, nil
	}
	pr := &protocol.Result{Research: []protocol.ResearchRecord{{ID: "research-1", Artifact: protocol.ResearchArtifact{Risks: []string{"risk"}, Citations: []protocol.EvidenceRef{ref}}}, {ID: "research-2", Artifact: protocol.ResearchArtifact{Risks: []string{"risk"}, Citations: []protocol.EvidenceRef{ref}}}}, Judges: []protocol.JudgeRecord{{ID: "judge-1", Artifact: protocol.JudgeArtifact{Action: "act", Reasons: []string{"reason"}, Evidence: []string{"evidence"}}}, {ID: "judge-2", Artifact: protocol.JudgeArtifact{Action: "act", Reasons: []string{"reason"}, Evidence: []string{"evidence"}}}}, Decision: protocol.DecisionRecord{Status: protocol.DecisionAgreed, JudgeAgreement: true, JudgeDecisions: [2]string{"ship", "ship"}}}
	return baseline.ArmResult{Arm: arm, InvocationCount: 1, Protocol: pr}, nil
}

type h6TypedJudgeRuntime struct{ provider councilruntime.Provider }

func (r h6TypedJudgeRuntime) Run(_ context.Context, _ councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	out := `{"overall_score":80,"dimensions":{"correctness_soundness":80,"evidence_use":80,"risk_handling":80,"actionability":80,"calibration":80},"citation_checks":[{"reference":{"artifact_id":"problem","locator":"constraints[0]"},"status":"verified","note":"matched"}],"relied_on_citations":[{"artifact_id":"problem","locator":"constraints[0]"}],"critical_errors":[],"strengths":["clear"],"weaknesses":[],"confidence":0.8}`
	return councilruntime.AgentResponse{Provider: r.provider, Stdout: out, ExitCode: 0, Attempts: 1}, nil
}

func TestH6RunnerAcceptsPriorH5FailureCitationShapeThroughActualEvaluator(t *testing.T) {
	dataset, err := LoadH6(writeValidH6Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	evaluator := evalharness.Harness{Adaptive: &evalharness.AdaptiveJudgeRuntimes{Judge1: h6TypedJudgeRuntime{councilruntime.ProviderClaude}, Judge2: h6TypedJudgeRuntime{councilruntime.ProviderCodex}}, TempRoot: t.TempDir(), CitationContract: evalharness.CitationContractStructuredV1}
	runner := H6Runner{NewBaseline: func(Case) (H6BaselineExecutor, error) { return h6TypedArmExecutor{}, nil }, Evaluator: evaluator, CollectAdapterSummary: func(context.Context, string, string) (H5AdapterSummary, error) {
		return H5AdapterSummary{SchemaVersion: H5AdapterSummarySchemaVersion, SuccessfulInvocations: H5ExpectedSuccessfulInvocations, AttemptsByAdapter: map[string]int{}, SuccessesByAdapter: map[string]int{}, SuccessesByProvider: map[string]int{}, AvailabilityFailuresByAdapter: map[string]int{}, SuccessesBySlot: map[string]map[string]int{}}, nil
	}}
	result, err := runner.Run(context.Background(), RunRequest{Dataset: dataset, RunsRoot: t.TempDir(), RunID: "h6-typed-integration"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.ProblemCount != H1CaseCount {
		t.Fatalf("problem count=%d", result.Summary.ProblemCount)
	}
}
