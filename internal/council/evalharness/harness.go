package evalharness

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/modeloutput"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/visibility"
)

var frozenEvalArms = []baseline.Arm{
	baseline.ArmAClaudeSingle,
	baseline.ArmBCodexSingle,
	baseline.ArmCClaudeSelfReview,
	baseline.ArmDCodexSelfReview,
	baseline.ArmEFullInfo,
	baseline.ArmFBlindCouncil,
}

type AdaptiveJudgeRuntimes struct {
	Judge1 councilruntime.AgentRuntime
	Judge2 councilruntime.AgentRuntime
}

type Harness struct {
	Claude           councilruntime.AgentRuntime
	Codex            councilruntime.AgentRuntime
	Adaptive         *AdaptiveJudgeRuntimes
	CitationContract CitationContract
	TempRoot         string
}

func (h Harness) validateJudgeRuntimes() error {
	if h.Adaptive != nil {
		if h.Adaptive.Judge1 == nil || h.Adaptive.Judge2 == nil {
			return fmt.Errorf("both adaptive judge runtimes are required")
		}
		return nil
	}
	if h.Claude == nil || h.Codex == nil {
		return fmt.Errorf("both fixed judge runtimes are required")
	}
	return nil
}

func (h Harness) judgeRuntimes() (councilruntime.AgentRuntime, councilruntime.AgentRuntime, councilruntime.Provider, councilruntime.Provider, bool) {
	if h.Adaptive != nil {
		return h.Adaptive.Judge1, h.Adaptive.Judge2, "", "", false
	}
	return h.Claude, h.Codex, councilruntime.ProviderClaude, councilruntime.ProviderCodex, true
}

type ProblemRequest struct {
	ProblemID          string
	RunID              string
	RunRoot            string
	NormalizedProblem  json.RawMessage
	Rubric             json.RawMessage
	RubricSHA256       string
	ReferenceSet       json.RawMessage
	ReferenceSetSHA256 string
	Arms               []baseline.ArmResult
	RiskPolicy         RiskPolicy
}

type preparedArm struct {
	arm       baseline.Arm
	candidate MaskedCandidate
	content   []byte
}

type preparedProblem struct {
	problem       []byte
	rubric        []byte
	referenceSet  []byte
	dimensions    []string
	arms          []preparedArm
	problemHash   string
	rubricHash    string
	referenceHash string
}

func (h Harness) EvaluateProblem(ctx context.Context, req ProblemRequest) (ProblemResult, error) {
	prepared, err := h.prepare(req)
	if err != nil {
		return ProblemResult{}, err
	}

	judge1, judge2, expected1, expected2, enforce := h.judgeRuntimes()
	armScores := make([]ArmScore, 0, len(prepared.arms))
	for _, arm := range prepared.arms {
		score1, err := h.evaluateCandidate(ctx, req, prepared, arm, "judge-1", "eval-judge-1", expected1, enforce, judge1)
		if err != nil {
			return ProblemResult{}, fmt.Errorf("arm %s judge-1: %w", arm.arm, err)
		}
		score2, err := h.evaluateCandidate(ctx, req, prepared, arm, "judge-2", "eval-judge-2", expected2, enforce, judge2)
		if err != nil {
			return ProblemResult{}, fmt.Errorf("arm %s judge-2: %w", arm.arm, err)
		}
		mean := (score1.Artifact.OverallScore + score2.Artifact.OverallScore) / 2
		spread := math.Abs(score1.Artifact.OverallScore - score2.Artifact.OverallScore)
		armScores = append(armScores, ArmScore{Arm: arm.arm, Judges: [2]JudgeScore{score1, score2}, MeanScore: mean, JudgeSpread: spread})
	}

	return ProblemResult{
		ProblemID:          req.ProblemID,
		RubricSHA256:       prepared.rubricHash,
		ReferenceSetSHA256: prepared.referenceHash,
		RiskPolicy:         req.RiskPolicy,
		Arms:               armScores,
	}, nil
}

func (h Harness) prepare(req ProblemRequest) (preparedProblem, error) {
	if strings.TrimSpace(req.ProblemID) == "" || !safeID(req.ProblemID) {
		return preparedProblem{}, fmt.Errorf("safe problem id is required")
	}
	if strings.TrimSpace(req.RunID) == "" {
		return preparedProblem{}, fmt.Errorf("run id is required")
	}
	if strings.TrimSpace(req.RunRoot) == "" {
		return preparedProblem{}, fmt.Errorf("run root is required")
	}
	if strings.TrimSpace(h.TempRoot) == "" {
		return preparedProblem{}, fmt.Errorf("eval temp root is required")
	}
	if err := h.validateJudgeRuntimes(); err != nil {
		return preparedProblem{}, err
	}
	if err := validateCitationContract(h.CitationContract); err != nil {
		return preparedProblem{}, err
	}
	if err := validateRiskPolicy(req.RiskPolicy); err != nil {
		return preparedProblem{}, err
	}

	rubricHash, err := verifySHA256("rubric", req.Rubric, req.RubricSHA256)
	if err != nil {
		return preparedProblem{}, err
	}
	referenceHash, err := verifySHA256("reference set", req.ReferenceSet, req.ReferenceSetSHA256)
	if err != nil {
		return preparedProblem{}, err
	}

	problem, err := normalizeJSONObject("normalized problem", req.NormalizedProblem)
	if err != nil {
		return preparedProblem{}, err
	}
	if !json.Valid(req.Rubric) {
		return preparedProblem{}, fmt.Errorf("rubric must be valid JSON")
	}
	if !json.Valid(req.ReferenceSet) {
		return preparedProblem{}, fmt.Errorf("reference set must be valid JSON")
	}
	var rubric RubricDocument
	if err := json.Unmarshal(req.Rubric, &rubric); err != nil {
		return preparedProblem{}, fmt.Errorf("decode rubric: %w", err)
	}
	dimensions, err := validateRubricDimensions(rubric)
	if err != nil {
		return preparedProblem{}, err
	}

	byArm := make(map[baseline.Arm]baseline.ArmResult, len(req.Arms))
	for _, result := range req.Arms {
		if _, duplicate := byArm[result.Arm]; duplicate {
			return preparedProblem{}, fmt.Errorf("duplicate baseline arm %q", result.Arm)
		}
		byArm[result.Arm] = result
	}
	if len(byArm) != len(frozenEvalArms) {
		return preparedProblem{}, fmt.Errorf("evaluation requires exactly six frozen baseline arms")
	}

	preparedArms := make([]preparedArm, 0, len(frozenEvalArms))
	for _, arm := range frozenEvalArms {
		result, ok := byArm[arm]
		if !ok {
			return preparedProblem{}, fmt.Errorf("missing baseline arm %s", arm)
		}
		candidate, err := NormalizeCandidate(result)
		if err != nil {
			return preparedProblem{}, fmt.Errorf("normalize arm %s: %w", arm, err)
		}
		if err := validateCandidateCitations(candidate); err != nil {
			return preparedProblem{}, fmt.Errorf("arm %s citations: %w", arm, err)
		}
		content, err := json.Marshal(candidate)
		if err != nil {
			return preparedProblem{}, fmt.Errorf("marshal arm %s candidate: %w", arm, err)
		}
		preparedArms = append(preparedArms, preparedArm{arm: arm, candidate: candidate, content: content})
	}

	return preparedProblem{
		problem:       problem,
		rubric:        append([]byte(nil), req.Rubric...),
		referenceSet:  append([]byte(nil), req.ReferenceSet...),
		dimensions:    dimensions,
		arms:          preparedArms,
		problemHash:   digestHex(problem),
		rubricHash:    rubricHash,
		referenceHash: referenceHash,
	}, nil
}

func (h Harness) evaluateCandidate(
	ctx context.Context,
	req ProblemRequest,
	prepared preparedProblem,
	arm preparedArm,
	slot string,
	participant string,
	expectedProvider councilruntime.Provider,
	enforceProvider bool,
	rt councilruntime.AgentRuntime,
) (JudgeScore, error) {
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
		RunRoot:   req.RunRoot,
		TempRoot:  h.TempRoot,
		Viewer:    visibility.Viewer{Participant: participant, Phase: PhaseEvalJudge},
		Artifacts: artifacts,
		Policy: visibility.Policy{
			Grants:               grants,
			MaskProviderIdentity: true,
		},
	})
	if err != nil {
		return JudgeScore{}, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: err}
	}

	prompt, renderErr := renderJudgePrompt(workspace, artifacts)
	if h.CitationContract == CitationContractStructuredV1 {
		prompt, renderErr = renderH6JudgePrompt(workspace, artifacts)
	}
	if renderErr != nil {
		cleanupErr := workspace.Cleanup()
		if cleanupErr != nil {
			return JudgeScore{}, errors.Join(renderErr, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: cleanupErr})
		}
		return JudgeScore{}, renderErr
	}

	response, runErr := rt.Run(ctx, councilruntime.AgentRequest{
		RunID:       req.RunID,
		RunRoot:     req.RunRoot,
		Participant: participant,
		Role:        "judge",
		Phase:       PhaseEvalJudge,
		Prompt:      prompt,
		Workdir:     workspace.Root,
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

	artifact, err := decodeJudgeForContract(response.Stdout, h.CitationContract, prepared.dimensions, arm.candidate)
	if err != nil {
		return JudgeScore{}, err
	}

	return JudgeScore{
		Slot:            slot,
		Provider:        response.Provider,
		AdapterID:       response.AdapterID,
		FailoverIndex:   response.FailoverIndex,
		FailoverTrigger: response.FailoverTrigger,
		Artifact:        artifact,
		InputHashes: map[string]string{
			"problem":       prepared.problemHash,
			"rubric":        prepared.rubricHash,
			"reference-set": prepared.referenceHash,
			"candidate":     digestHex(arm.content),
		},
		OutputSHA256: digestHex([]byte(response.Stdout)),
		StartedAt:    response.StartedAt,
		FinishedAt:   response.FinishedAt,
	}, nil
}

func renderJudgePrompt(workspace visibility.Workspace, artifacts []visibility.Artifact) (string, error) {
	const instruction = `EVALUATION_JUDGE
Score exactly one masked candidate against the normalized problem, frozen rubric, and frozen reference set. Do not infer the candidate arm, provider, other candidates, consensus, or another judge's verdict. Verify every citation against the visible artifact it names before relying on that citation. A citation that cannot be verified must not support the score. Return JSON only with exactly these keys: "overall\u005fscore" (0..100), "dimensions" (object mapping every rubric dimension id to 0..100, no extras), "citation\u005fchecks" ({reference,status,note}[]), "relied\u005fon\u005fcitations" (string[]), "critical\u005ferrors" (string[]), "strengths" (string[]), "weaknesses" (string[]), "confidence" (0..1). Use status "verified" only when the cited claim matches the visible artifact.`

	byID := make(map[string]visibility.Artifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	order := []string{"problem", "rubric", "reference-set", "candidate"}
	var b strings.Builder
	b.WriteString(instruction)
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

func verifySHA256(label string, data []byte, expected string) (string, error) {
	actual := digestHex(data)
	if len(expected) != 64 {
		return "", fmt.Errorf("%s hash must be a 64-character SHA-256 digest", label)
	}
	if !strings.EqualFold(actual, expected) {
		return "", fmt.Errorf("%s hash mismatch: got %s", label, actual)
	}
	return actual, nil
}

func normalizeJSONObject(label string, raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("%s is required", label)
	}
	var object map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", label, err)
	}
	if len(object) == 0 {
		return nil, fmt.Errorf("%s must be a non-empty JSON object", label)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s must contain exactly one JSON object", label)
	}
	buffer := new(bytes.Buffer)
	if err := json.Compact(buffer, raw); err != nil {
		return nil, fmt.Errorf("compact %s: %w", label, err)
	}
	return buffer.Bytes(), nil
}

func validateRubricDimensions(rubric RubricDocument) ([]string, error) {
	if len(rubric.Dimensions) == 0 {
		return nil, fmt.Errorf("rubric must define at least one dimension")
	}
	seen := make(map[string]struct{}, len(rubric.Dimensions))
	dimensions := make([]string, 0, len(rubric.Dimensions))
	for _, dimension := range rubric.Dimensions {
		id := strings.TrimSpace(dimension.ID)
		if id == "" {
			return nil, fmt.Errorf("rubric dimension id is required")
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("duplicate rubric dimension %q", id)
		}
		seen[id] = struct{}{}
		dimensions = append(dimensions, id)
	}
	return dimensions, nil
}

func validateCandidateCitations(candidate MaskedCandidate) error {
	for _, citation := range candidate.Citations {
		if citation.ArtifactID != "problem" && citation.ArtifactID != "reference-set" {
			return fmt.Errorf("citation artifact %q is not visible to eval judges", citation.ArtifactID)
		}
		if strings.TrimSpace(citation.Locator) == "" {
			return fmt.Errorf("citation locator is required")
		}
	}
	return nil
}

func decodeStrictJudgeJSON(raw string, out *JudgeArtifact) error {
	if err := modeloutput.DecodeStrict(raw, out); err != nil {
		return malformedEval(fmt.Errorf("decode judge JSON: %w", err))
	}
	return nil
}

func validateJudgeArtifact(artifact JudgeArtifact, dimensions []string) error {
	if !validScore(artifact.OverallScore) {
		return fmt.Errorf("overall score %.4f must be between 0 and 100", artifact.OverallScore)
	}
	if artifact.Confidence < 0 || artifact.Confidence > 1 || math.IsNaN(artifact.Confidence) || math.IsInf(artifact.Confidence, 0) {
		return fmt.Errorf("judge confidence %.4f must be between 0 and 1", artifact.Confidence)
	}
	if len(artifact.Dimensions) != len(dimensions) {
		return fmt.Errorf("judge dimensions count = %d want %d", len(artifact.Dimensions), len(dimensions))
	}
	for _, id := range dimensions {
		score, ok := artifact.Dimensions[id]
		if !ok {
			return fmt.Errorf("judge missing rubric dimension %q", id)
		}
		if !validScore(score) {
			return fmt.Errorf("rubric dimension %q score %.4f must be between 0 and 100", id, score)
		}
	}
	allowed := make(map[string]struct{}, len(dimensions))
	for _, id := range dimensions {
		allowed[id] = struct{}{}
	}
	for id := range artifact.Dimensions {
		if _, ok := allowed[id]; !ok {
			return fmt.Errorf("judge returned unknown rubric dimension %q", id)
		}
	}

	checks := make(map[string]struct{}, len(artifact.CitationChecks))
	for _, check := range artifact.CitationChecks {
		ref := strings.TrimSpace(check.Reference)
		if ref == "" || strings.TrimSpace(check.Status) == "" {
			return fmt.Errorf("citation check reference and status are required")
		}
		if _, duplicate := checks[ref]; duplicate {
			return fmt.Errorf("duplicate citation check %q", ref)
		}
		checks[ref] = struct{}{}
	}
	return nil
}

func validateReliedCitations(artifact JudgeArtifact, candidate MaskedCandidate) error {
	candidateRefs := make(map[string]struct{}, len(candidate.Citations))
	for _, citation := range candidate.Citations {
		candidateRefs[citation.ArtifactID+":"+citation.Locator] = struct{}{}
	}
	checks := make(map[string]string, len(artifact.CitationChecks))
	for _, check := range artifact.CitationChecks {
		checks[check.Reference] = strings.ToLower(strings.TrimSpace(check.Status))
	}
	seen := make(map[string]struct{}, len(artifact.ReliedOnCitations))
	for _, reference := range artifact.ReliedOnCitations {
		if _, duplicate := seen[reference]; duplicate {
			return fmt.Errorf("duplicate relied-on citation %q", reference)
		}
		seen[reference] = struct{}{}
		if _, ok := candidateRefs[reference]; !ok {
			return fmt.Errorf("relied-on citation %q is not present in candidate", reference)
		}
		if checks[reference] != "verified" {
			return fmt.Errorf("relied-on citation %q is not verified", reference)
		}
	}
	return nil
}

func validScore(value float64) bool {
	return value >= 0 && value <= 100 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func malformedEval(err error) error {
	return &councilruntime.RunError{Class: councilruntime.FailureMalformedOutput, Err: fmt.Errorf("eval judge artifact: %w", err)}
}

func digestHex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func safeID(value string) bool {
	return value != "." && value != ".." && filepath.Base(value) == value && !strings.ContainsAny(value, `/\\`)
}
