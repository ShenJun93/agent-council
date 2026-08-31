package evalharness

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/modeloutput"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/visibility"
)

type H8CitationCheck struct {
	Reference CitationOccurrenceKey `json:"reference"`
	Status    string                `json:"status"`
	ReliedOn  *bool                 `json:"relied_on"`
	Note      string                `json:"note"`
}

type H8JudgeArtifact struct {
	OverallScore   float64            `json:"overall_score"`
	Dimensions     map[string]float64 `json:"dimensions"`
	CitationChecks []H8CitationCheck  `json:"citation_checks"`
	CriticalErrors []string           `json:"critical_errors"`
	Strengths      []string           `json:"strengths"`
	Weaknesses     []string           `json:"weaknesses"`
	Confidence     float64            `json:"confidence"`
}

const h8JudgeInstruction = `EVALUATION_JUDGE
Score exactly one masked candidate against the normalized problem, frozen rubric, and frozen reference set. Do not infer the candidate arm, provider, other candidates, consensus, or another judge's verdict. Verify candidate citations only against the visible artifact each names. Return JSON only with exactly these keys: "overall_score" (0..100), "dimensions" (object mapping every rubric dimension id to 0..100, no extras), "citation_checks" ({reference:{"artifact_id":string,"locator":string,"claim":string},status,relied_on,note}[]), "critical_errors" (string[]), "strengths" (string[]), "weaknesses" (string[]), "confidence" (0..1). Every citation reference must copy artifact_id, locator, and claim exactly from candidate.citations. Two candidate citations may share artifact_id and locator with different claims; treat each full tuple as a distinct citation occurrence. Do not join, reformat, paraphrase, deduplicate by source location, or merge claims. Citation status must be exactly "verified" or "unverified". Use "verified" only when the full cited claim matches the visible artifact. "relied_on" means you, the evaluation judge, use this exact citation occurrence as evidentiary support for your numeric score or dimension scores; it does not mean the candidate relied on the citation. Only a "verified" citation may have "relied_on":true. A partially supported, inferred, over-broad, contradicted, or unsupported full claim is "unverified", MUST have "relied_on":false, and may be discussed as a weakness or critical error and reflected by a lower score.`

type H8Harness struct {
	Harness
}

func (h H8Harness) EvaluateProblem(ctx context.Context, req ProblemRequest) (ProblemResult, error) {
	if h.CitationContract != CitationContractStructuredV3 {
		return ProblemResult{}, fmt.Errorf("H8 evaluator requires citation contract V3")
	}
	prepared, err := h.prepare(req)
	if err != nil {
		return ProblemResult{}, err
	}
	judge1, judge2, expected1, expected2, enforce := h.judgeRuntimes()
	armScores := make([]ArmScore, 0, len(prepared.arms))
	for _, arm := range prepared.arms {
		score1, err := h.evaluateCandidateH8(ctx, req, prepared, arm, "judge-1", "eval-judge-1", expected1, enforce, judge1)
		if err != nil {
			return ProblemResult{}, fmt.Errorf("arm %s judge-1: %w", arm.arm, err)
		}
		score2, err := h.evaluateCandidateH8(ctx, req, prepared, arm, "judge-2", "eval-judge-2", expected2, enforce, judge2)
		if err != nil {
			return ProblemResult{}, fmt.Errorf("arm %s judge-2: %w", arm.arm, err)
		}
		mean := (score1.Artifact.OverallScore + score2.Artifact.OverallScore) / 2
		spread := math.Abs(score1.Artifact.OverallScore - score2.Artifact.OverallScore)
		armScores = append(armScores, ArmScore{Arm: arm.arm, Judges: [2]JudgeScore{score1, score2}, MeanScore: mean, JudgeSpread: spread})
	}
	return ProblemResult{
		ProblemID: req.ProblemID, RubricSHA256: prepared.rubricHash,
		ReferenceSetSHA256: prepared.referenceHash, RiskPolicy: req.RiskPolicy, Arms: armScores,
	}, nil
}

func (h H8Harness) evaluateCandidateH8(ctx context.Context, req ProblemRequest, prepared preparedProblem, arm preparedArm, slot, participant string, expectedProvider councilruntime.Provider, enforceProvider bool, rt councilruntime.AgentRuntime) (JudgeScore, error) {
	artifacts := []visibility.Artifact{
		{ID: "problem", RelativePath: "context/problem.json", Content: prepared.problem},
		{ID: "rubric", RelativePath: "context/rubric.json", Content: prepared.rubric},
		{ID: "reference-set", RelativePath: "context/reference-set.json", Content: prepared.referenceSet},
		{ID: "candidate", RelativePath: "context/candidate.json", Content: arm.content},
	}
	grants := make([]visibility.Grant, 0, len(artifacts))
	for _, artifact := range artifacts {
		grants = append(grants, visibility.Grant{Participant: participant, Phase: PhaseEvalJudge, ArtifactID: artifact.ID})
	}
	workspace, err := visibility.Materialize(visibility.Request{
		RunRoot: req.RunRoot, TempRoot: h.TempRoot,
		Viewer: visibility.Viewer{Participant: participant, Phase: PhaseEvalJudge}, Artifacts: artifacts,
		Policy: visibility.Policy{Grants: grants, MaskProviderIdentity: true},
	})
	if err != nil {
		return JudgeScore{}, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: err}
	}
	prompt, renderErr := renderH8JudgePrompt(workspace, artifacts)
	if renderErr != nil {
		cleanupErr := workspace.Cleanup()
		if cleanupErr != nil {
			return JudgeScore{}, errors.Join(renderErr, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: cleanupErr})
		}
		return JudgeScore{}, renderErr
	}
	response, runErr := rt.Run(ctx, councilruntime.AgentRequest{
		RunID: req.RunID, RunRoot: req.RunRoot, Participant: participant,
		Role: "judge", Phase: PhaseEvalJudge, Prompt: prompt, Workdir: workspace.Root,
	})
	cleanupErr := workspace.Cleanup()
	if runErr != nil {
		if cleanupErr != nil {
			return JudgeScore{}, errors.Join(runErr, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: cleanupErr})
		}
		return JudgeScore{}, runErr
	}
	if cleanupErr != nil {
		return JudgeScore{}, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: fmt.Errorf("clean eval workspace: %w", cleanupErr)}
	}
	if enforceProvider && response.Provider != expectedProvider {
		return JudgeScore{}, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: fmt.Errorf("judge provider substitution: got %q want %q", response.Provider, expectedProvider)}
	}
	artifact, err := decodeH8Judge(response.Stdout, prepared.dimensions, arm.candidate)
	if err != nil {
		return JudgeScore{}, err
	}
	return JudgeScore{
		Slot: slot, Provider: response.Provider, AdapterID: response.AdapterID,
		FailoverIndex: response.FailoverIndex, FailoverTrigger: response.FailoverTrigger, Artifact: artifact,
		InputHashes:  map[string]string{"problem": prepared.problemHash, "rubric": prepared.rubricHash, "reference-set": prepared.referenceHash, "candidate": digestHex(arm.content)},
		OutputSHA256: digestHex([]byte(response.Stdout)), StartedAt: response.StartedAt, FinishedAt: response.FinishedAt,
	}, nil
}

func decodeH8Judge(raw string, dimensions []string, candidate MaskedCandidate) (JudgeArtifact, error) {
	var wire H8JudgeArtifact
	if err := modeloutput.DecodeStrict(raw, &wire); err != nil {
		return JudgeArtifact{}, malformedEval(fmt.Errorf("decode judge JSON: %w", err))
	}
	if err := validateH8CitationReferences(wire, candidate); err != nil {
		return JudgeArtifact{}, malformedEval(err)
	}
	artifact, err := canonicalizeH8JudgeArtifact(wire)
	if err != nil {
		return JudgeArtifact{}, malformedEval(err)
	}
	if err := validateJudgeArtifact(artifact, dimensions); err != nil {
		return JudgeArtifact{}, malformedEval(err)
	}
	return artifact, nil
}

func validateH8CitationReferences(wire H8JudgeArtifact, candidate MaskedCandidate) error {
	candidateRefs := make(map[CitationOccurrenceKey]struct{}, len(candidate.Citations))
	for _, citation := range candidate.Citations {
		candidateRefs[evidenceRefToOccurrenceKey(citation)] = struct{}{}
	}
	seen := make(map[CitationOccurrenceKey]struct{}, len(wire.CitationChecks))
	for _, check := range wire.CitationChecks {
		if _, member := candidateRefs[check.Reference]; !member {
			if err := validateCitationOccurrenceKey(check.Reference); err != nil {
				return fmt.Errorf("citation check: %w", err)
			}
			return fmt.Errorf("citation check %q is not present in candidate", canonicalOccurrenceString(check.Reference))
		}
		if _, duplicate := seen[check.Reference]; duplicate {
			return fmt.Errorf("duplicate citation check %q", canonicalOccurrenceString(check.Reference))
		}
		seen[check.Reference] = struct{}{}
		if check.Status != "verified" && check.Status != "unverified" {
			return fmt.Errorf("citation check status %q is not supported", check.Status)
		}
		if check.ReliedOn == nil {
			return fmt.Errorf("citation check relied_on is required")
		}
		if *check.ReliedOn && check.Status != "verified" {
			return fmt.Errorf("relied-on citation %q is not verified", canonicalOccurrenceString(check.Reference))
		}
	}
	return nil
}

func canonicalizeH8JudgeArtifact(wire H8JudgeArtifact) (JudgeArtifact, error) {
	checks := make([]protocol.CitationCheck, len(wire.CitationChecks))
	relied := make([]string, 0, len(wire.CitationChecks))
	for i, check := range wire.CitationChecks {
		reference, err := canonicalOccurrenceJSON(check.Reference)
		if err != nil {
			return JudgeArtifact{}, err
		}
		checks[i] = protocol.CitationCheck{Reference: reference, Status: check.Status, Note: check.Note}
		if check.ReliedOn != nil && *check.ReliedOn {
			relied = append(relied, reference)
		}
	}
	return JudgeArtifact{
		OverallScore: wire.OverallScore, Dimensions: wire.Dimensions,
		CitationChecks: checks, ReliedOnCitations: relied,
		CriticalErrors: wire.CriticalErrors, Strengths: wire.Strengths,
		Weaknesses: wire.Weaknesses, Confidence: wire.Confidence,
	}, nil
}

func renderH8JudgePrompt(workspace visibility.Workspace, artifacts []visibility.Artifact) (string, error) {
	byID := make(map[string]visibility.Artifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	order := []string{"problem", "rubric", "reference-set", "candidate"}
	var b strings.Builder
	b.WriteString(h8JudgeInstruction)
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
