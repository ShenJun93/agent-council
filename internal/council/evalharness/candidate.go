package evalharness

import (
	"fmt"
	"math"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

func NormalizeCandidate(result baseline.ArmResult) (MaskedCandidate, error) {
	switch result.Arm {
	case baseline.ArmAClaudeSingle, baseline.ArmBCodexSingle, baseline.ArmCClaudeSelfReview, baseline.ArmDCodexSelfReview:
		if result.Answer == nil || result.Protocol != nil {
			return MaskedCandidate{}, fmt.Errorf("arm %s must contain exactly one answer artifact", result.Arm)
		}
		answer := result.Answer
		if strings.TrimSpace(answer.Decision) == "" {
			return MaskedCandidate{}, fmt.Errorf("arm %s answer decision is required", result.Arm)
		}
		return MaskedCandidate{
			Decision:    answer.Decision,
			Action:      answer.Action,
			Reasons:     cloneStrings(answer.Reasons),
			Assumptions: cloneStrings(answer.Assumptions),
			Risks:       cloneStrings(answer.Risks),
			Citations:   cloneEvidenceRefs(answer.Citations),
			Evidence:    []string{},
			Minority:    []string{},
			Unresolved:  []string{},
			NextValidations: []string{},
		}, nil
	case baseline.ArmEFullInfo, baseline.ArmFBlindCouncil:
		if result.Protocol == nil || result.Answer != nil {
			return MaskedCandidate{}, fmt.Errorf("arm %s must contain exactly one protocol result", result.Arm)
		}
		return normalizeProtocolCandidate(*result.Protocol)
	default:
		return MaskedCandidate{}, fmt.Errorf("unknown baseline arm %q", result.Arm)
	}
}

func normalizeProtocolCandidate(result protocol.Result) (MaskedCandidate, error) {
	if len(result.Judges) != 2 {
		return MaskedCandidate{}, fmt.Errorf("protocol result must contain exactly two judges")
	}

	decision := protocolCandidateDecision(result.Decision)
	if strings.TrimSpace(decision) == "" {
		return MaskedCandidate{}, fmt.Errorf("protocol decision is required")
	}

	var actions []string
	var reasons []string
	var assumptions []string
	var evidence []string
	for _, judge := range result.Judges {
		actions = appendUniqueStrings(actions, judge.Artifact.Action)
		reasons = appendUniqueStrings(reasons, judge.Artifact.Reasons...)
		assumptions = appendUniqueStrings(assumptions, judge.Artifact.Assumptions...)
		evidence = appendUniqueStrings(evidence, judge.Artifact.Evidence...)
	}

	var risks []string
	var citations []protocol.EvidenceRef
	for _, research := range result.Research {
		risks = appendUniqueStrings(risks, research.Artifact.Risks...)
		citations = appendUniqueEvidenceRefs(citations, research.Artifact.Citations...)
	}

	return MaskedCandidate{
		Decision:        decision,
		Action:          strings.Join(actions, " | "),
		Reasons:         reasons,
		Assumptions:     assumptions,
		Risks:           risks,
		Citations:       citations,
		Evidence:        evidence,
		Minority:        cloneStrings(result.Decision.MinorityReport),
		Unresolved:      cloneStrings(result.Decision.Unresolved),
		NextValidations: cloneStrings(result.Decision.NextValidations),
	}, nil
}

func protocolCandidateDecision(record protocol.DecisionRecord) string {
	first := strings.TrimSpace(record.JudgeDecisions[0])
	second := strings.TrimSpace(record.JudgeDecisions[1])
	if record.JudgeAgreement && first != "" && first == second {
		return first
	}
	parts := make([]string, 0, 2)
	if first != "" {
		parts = append(parts, first)
	}
	if second != "" && second != first {
		parts = append(parts, second)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " | ")
	}
	return strings.TrimSpace(record.Status)
}

func validateRiskPolicy(policy RiskPolicy) error {
	if policy.Comparator != ComparatorBestSingle {
		return fmt.Errorf("unsupported comparator %q", policy.Comparator)
	}
	if policy.MaterialWorseDelta <= 0 || math.IsNaN(policy.MaterialWorseDelta) || math.IsInf(policy.MaterialWorseDelta, 0) {
		return fmt.Errorf("material_worse_delta must be a finite positive value")
	}
	return nil
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func cloneEvidenceRefs(values []protocol.EvidenceRef) []protocol.EvidenceRef {
	if len(values) == 0 {
		return []protocol.EvidenceRef{}
	}
	return append([]protocol.EvidenceRef(nil), values...)
}

func appendUniqueStrings(dst []string, values ...string) []string {
	seen := make(map[string]struct{}, len(dst)+len(values))
	for _, value := range dst {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		dst = append(dst, value)
	}
	return dst
}

func appendUniqueEvidenceRefs(dst []protocol.EvidenceRef, values ...protocol.EvidenceRef) []protocol.EvidenceRef {
	seen := make(map[string]struct{}, len(dst)+len(values))
	for _, value := range dst {
		seen[evidenceRefKey(value)] = struct{}{}
	}
	for _, value := range values {
		key := evidenceRefKey(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dst = append(dst, value)
	}
	return dst
}

func evidenceRefKey(value protocol.EvidenceRef) string {
	return value.ArtifactID + "\x00" + value.Locator + "\x00" + value.Claim
}
