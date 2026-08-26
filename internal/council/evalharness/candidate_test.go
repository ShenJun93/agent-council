package evalharness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

func TestNormalizeCandidateMasksArmAndProviderMetadata(t *testing.T) {
	t.Parallel()

	answer := baseline.AnswerArtifact{
		Decision:    "ship",
		Action:      "deploy",
		Reasons:     []string{"reason"},
		Assumptions: []string{"assumption"},
		Risks:       []string{"risk"},
		Citations: []protocol.EvidenceRef{{
			ArtifactID: "problem",
			Locator:    "problem",
			Claim:      "supports ship",
		}},
		Confidence: 0.8,
	}
	candidate, err := NormalizeCandidate(baseline.ArmResult{
		Arm:    baseline.ArmAClaudeSingle,
		Answer: &answer,
	})
	if err != nil {
		t.Fatalf("NormalizeCandidate() error = %v", err)
	}
	if candidate.Decision != "ship" || candidate.Action != "deploy" {
		t.Fatalf("candidate = %+v", candidate)
	}
	encoded, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("marshal candidate: %v", err)
	}
	text := strings.ToLower(string(encoded))
	if strings.Contains(text, `"arm"`) || strings.Contains(text, `"provider"`) {
		t.Fatalf("masked candidate leaked harness identity metadata: %s", encoded)
	}
}

func TestNormalizeCandidateProtocolCarriesDecisionEvidenceAndMinority(t *testing.T) {
	t.Parallel()

	result := protocol.Result{
		Research: []protocol.ResearchRecord{
			{ID: "research-1", Artifact: protocol.ResearchArtifact{Risks: []string{"r1"}, Citations: []protocol.EvidenceRef{{ArtifactID: "problem", Locator: "problem", Claim: "claim-1"}}}},
			{ID: "research-2", Artifact: protocol.ResearchArtifact{Risks: []string{"r2"}, Citations: []protocol.EvidenceRef{{ArtifactID: "problem", Locator: "problem", Claim: "claim-2"}}}},
		},
		Judges: []protocol.JudgeRecord{
			{ID: "judge-1", Artifact: protocol.JudgeArtifact{Action: "act", Reasons: []string{"j1 reason"}, Evidence: []string{"e1"}, Assumptions: []string{"a1"}}},
			{ID: "judge-2", Artifact: protocol.JudgeArtifact{Action: "act", Reasons: []string{"j2 reason"}, Evidence: []string{"e2"}, Assumptions: []string{"a2"}}},
		},
		Decision: protocol.DecisionRecord{
			Status:          protocol.DecisionAgreed,
			JudgeAgreement:  true,
			JudgeDecisions:  [2]string{"ship", "ship"},
			MinorityReport:  []string{"minority"},
			Unresolved:      []string{"open question"},
			NextValidations: []string{"validate next"},
		},
	}
	candidate, err := NormalizeCandidate(baseline.ArmResult{
		Arm:      baseline.ArmFBlindCouncil,
		Protocol: &result,
	})
	if err != nil {
		t.Fatalf("NormalizeCandidate() error = %v", err)
	}
	if candidate.Decision != "ship" || candidate.Action != "act" {
		t.Fatalf("candidate decision/action = %+v", candidate)
	}
	if strings.Join(candidate.Reasons, ",") != "j1 reason,j2 reason" {
		t.Fatalf("candidate reasons = %v", candidate.Reasons)
	}
	if strings.Join(candidate.Evidence, ",") != "e1,e2" {
		t.Fatalf("candidate evidence = %v", candidate.Evidence)
	}
	if strings.Join(candidate.Risks, ",") != "r1,r2" {
		t.Fatalf("candidate risks = %v", candidate.Risks)
	}
	if strings.Join(candidate.Minority, ",") != "minority" || strings.Join(candidate.Unresolved, ",") != "open question" || strings.Join(candidate.NextValidations, ",") != "validate next" {
		t.Fatalf("candidate decision metadata = %+v", candidate)
	}
	if len(candidate.Citations) != 2 {
		t.Fatalf("candidate citations = %v", candidate.Citations)
	}
}

func TestNormalizeCandidateRejectsWrongArmResultShape(t *testing.T) {
	t.Parallel()

	answer := baseline.AnswerArtifact{Decision: "x"}
	protocolResult := protocol.Result{}
	for _, result := range []baseline.ArmResult{
		{Arm: baseline.ArmAClaudeSingle},
		{Arm: baseline.ArmAClaudeSingle, Answer: &answer, Protocol: &protocolResult},
		{Arm: baseline.ArmFBlindCouncil, Answer: &answer},
		{Arm: baseline.Arm("Z"), Answer: &answer},
	} {
		if _, err := NormalizeCandidate(result); err == nil {
			t.Fatalf("NormalizeCandidate(%+v) unexpectedly succeeded", result)
		}
	}
}

func TestValidateRiskPolicyRequiresFrozenPositiveBestSingleDelta(t *testing.T) {
	t.Parallel()

	if err := validateRiskPolicy(RiskPolicy{Comparator: ComparatorBestSingle, MaterialWorseDelta: 5}); err != nil {
		t.Fatalf("valid policy rejected: %v", err)
	}
	for _, policy := range []RiskPolicy{
		{},
		{Comparator: Comparator("mean_single"), MaterialWorseDelta: 5},
		{Comparator: ComparatorBestSingle, MaterialWorseDelta: 0},
		{Comparator: ComparatorBestSingle, MaterialWorseDelta: -1},
	} {
		if err := validateRiskPolicy(policy); err == nil {
			t.Fatalf("invalid policy %+v accepted", policy)
		}
	}
}
