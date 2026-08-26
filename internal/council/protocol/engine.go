package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/visibility"
)

const defaultHighConfidenceThreshold = 0.9

type pairResult[T any] struct {
	index int
	value T
	err   error
}

func (e Engine) Run(ctx context.Context, req RunRequest) (Result, error) {
	problem, err := validateNormalizedProblem(req.NormalizedProblem)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(req.RunID) == "" {
		return Result{}, fmt.Errorf("run id is required")
	}
	if strings.TrimSpace(req.RunRoot) == "" {
		return Result{}, fmt.Errorf("run root is required")
	}
	if strings.TrimSpace(e.TempRoot) == "" {
		return Result{}, fmt.Errorf("protocol temp root is required")
	}
	if e.Claude == nil || e.Codex == nil {
		return Result{}, fmt.Errorf("both Claude and Codex runtimes are required")
	}
	if e.ChallengerProvider != councilruntime.ProviderClaude && e.ChallengerProvider != councilruntime.ProviderCodex {
		return Result{}, fmt.Errorf("challenger provider must be explicitly set to claude or codex")
	}

	problemArtifact := visibility.Artifact{
		ID:           "problem",
		RelativePath: "context/problem.json",
		Content:      problem,
	}
	artifacts := []visibility.Artifact{problemArtifact}

	researchPair, err := parallel2(ctx,
		func(callCtx context.Context) (ResearchArtifact, error) {
			var out ResearchArtifact
			err := e.invokeJSON(callCtx, req, e.Claude, "researcher-1", "researcher", PhaseResearch, artifacts, []string{"problem"}, researchInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (ResearchArtifact, error) {
			var out ResearchArtifact
			err := e.invokeJSON(callCtx, req, e.Codex, "researcher-2", "researcher", PhaseResearch, artifacts, []string{"problem"}, researchInstruction(), &out)
			return out, err
		},
	)
	if err != nil {
		return Result{}, fmt.Errorf("research phase: %w", err)
	}
	for i := range researchPair {
		if err := validateResearch(researchPair[i]); err != nil {
			return Result{}, malformed("research", err)
		}
	}

	research1Artifact, err := visibilityJSONArtifact("research-1", "context/research-1.json", researchPair[0])
	if err != nil {
		return Result{}, err
	}
	research2Artifact, err := visibilityJSONArtifact("research-2", "context/research-2.json", researchPair[1])
	if err != nil {
		return Result{}, err
	}
	artifacts = append(artifacts, research1Artifact, research2Artifact)

	reviewPair, err := parallel2(ctx,
		func(callCtx context.Context) (ReviewArtifact, error) {
			var out ReviewArtifact
			err := e.invokeJSON(callCtx, req, e.Claude, "reviewer-1", "reviewer", PhaseReview, artifacts, []string{"problem", "research-2"}, reviewInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (ReviewArtifact, error) {
			var out ReviewArtifact
			err := e.invokeJSON(callCtx, req, e.Codex, "reviewer-2", "reviewer", PhaseReview, artifacts, []string{"problem", "research-1"}, reviewInstruction(), &out)
			return out, err
		},
	)
	if err != nil {
		return Result{}, fmt.Errorf("review phase: %w", err)
	}
	for i := range reviewPair {
		if err := validateReview(reviewPair[i]); err != nil {
			return Result{}, malformed("review", err)
		}
	}

	review1Artifact, err := visibilityJSONArtifact("review-1", "context/review-1.json", reviewPair[0])
	if err != nil {
		return Result{}, err
	}
	review2Artifact, err := visibilityJSONArtifact("review-2", "context/review-2.json", reviewPair[1])
	if err != nil {
		return Result{}, err
	}
	artifacts = append(artifacts, review1Artifact, review2Artifact)

	challengeDecision := e.challengeDecision(researchPair)
	challengeRuntime := e.Claude
	if e.ChallengerProvider == councilruntime.ProviderCodex {
		challengeRuntime = e.Codex
	}
	var challenge ChallengeArtifact
	if err := e.invokeJSON(
		ctx,
		req,
		challengeRuntime,
		"challenger",
		"challenger",
		PhaseChallenge,
		artifacts,
		[]string{"problem", "research-1", "research-2", "review-1", "review-2"},
		challengeInstruction(challengeDecision.Mode),
		&challenge,
	); err != nil {
		return Result{}, fmt.Errorf("challenge phase: %w", err)
	}
	if err := validateChallenge(challenge); err != nil {
		return Result{}, malformed("challenge", err)
	}
	challengeArtifact, err := visibilityJSONArtifact("challenge", "context/challenge.json", challenge)
	if err != nil {
		return Result{}, err
	}
	artifacts = append(artifacts, challengeArtifact)

	rebuttalPair, err := parallel2(ctx,
		func(callCtx context.Context) (RebuttalArtifact, error) {
			var out RebuttalArtifact
			err := e.invokeJSON(callCtx, req, e.Claude, "researcher-1", "researcher", PhaseRebuttal, artifacts, []string{"problem", "research-1", "review-2", "challenge"}, rebuttalInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (RebuttalArtifact, error) {
			var out RebuttalArtifact
			err := e.invokeJSON(callCtx, req, e.Codex, "researcher-2", "researcher", PhaseRebuttal, artifacts, []string{"problem", "research-2", "review-1", "challenge"}, rebuttalInstruction(), &out)
			return out, err
		},
	)
	if err != nil {
		return Result{}, fmt.Errorf("rebuttal phase: %w", err)
	}
	for i := range rebuttalPair {
		if err := validateRebuttal(rebuttalPair[i]); err != nil {
			return Result{}, malformed("rebuttal", err)
		}
	}

	rebuttal1Artifact, err := visibilityJSONArtifact("rebuttal-1", "context/rebuttal-1.json", rebuttalPair[0])
	if err != nil {
		return Result{}, err
	}
	rebuttal2Artifact, err := visibilityJSONArtifact("rebuttal-2", "context/rebuttal-2.json", rebuttalPair[1])
	if err != nil {
		return Result{}, err
	}
	artifacts = append(artifacts, rebuttal1Artifact, rebuttal2Artifact)

	judgeAllowed := []string{
		"problem",
		"research-1",
		"research-2",
		"review-1",
		"review-2",
		"challenge",
		"rebuttal-1",
		"rebuttal-2",
	}
	judgePair, err := parallel2(ctx,
		func(callCtx context.Context) (JudgeArtifact, error) {
			var out JudgeArtifact
			err := e.invokeJSON(callCtx, req, e.Claude, "judge-1", "judge", PhaseJudge, artifacts, judgeAllowed, judgeInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (JudgeArtifact, error) {
			var out JudgeArtifact
			err := e.invokeJSON(callCtx, req, e.Codex, "judge-2", "judge", PhaseJudge, artifacts, judgeAllowed, judgeInstruction(), &out)
			return out, err
		},
	)
	if err != nil {
		return Result{}, fmt.Errorf("judge phase: %w", err)
	}
	for i := range judgePair {
		if err := validateJudge(judgePair[i]); err != nil {
			return Result{}, malformed("judge", err)
		}
	}

	decision := buildDecision(judgePair)
	return Result{
		Research: []ResearchRecord{
			{ID: "research-1", Artifact: researchPair[0]},
			{ID: "research-2", Artifact: researchPair[1]},
		},
		Reviews: []ReviewRecord{
			{ID: "review-1", TargetID: "research-2", Artifact: reviewPair[0]},
			{ID: "review-2", TargetID: "research-1", Artifact: reviewPair[1]},
		},
		ChallengeDecision: challengeDecision,
		Challenge:         ChallengeRecord{ID: "challenge", Artifact: challenge},
		Rebuttals: []RebuttalRecord{
			{ID: "rebuttal-1", TargetID: "research-1", Artifact: rebuttalPair[0]},
			{ID: "rebuttal-2", TargetID: "research-2", Artifact: rebuttalPair[1]},
		},
		Judges: []JudgeRecord{
			{ID: "judge-1", Artifact: judgePair[0]},
			{ID: "judge-2", Artifact: judgePair[1]},
		},
		Decision: decision,
	}, nil
}

func (e Engine) invokeJSON(
	ctx context.Context,
	req RunRequest,
	rt councilruntime.AgentRuntime,
	participant string,
	role string,
	phase string,
	artifacts []visibility.Artifact,
	allowedIDs []string,
	instruction string,
	out any,
) error {
	grants := make([]visibility.Grant, 0, len(allowedIDs))
	for _, id := range allowedIDs {
		grants = append(grants, visibility.Grant{Participant: participant, Phase: phase, ArtifactID: id})
	}
	workspace, err := visibility.Materialize(visibility.Request{
		RunRoot:   req.RunRoot,
		TempRoot:  e.TempRoot,
		Viewer:    visibility.Viewer{Participant: participant, Phase: phase},
		Artifacts: append([]visibility.Artifact(nil), artifacts...),
		Policy: visibility.Policy{
			Grants:               grants,
			MaskProviderIdentity: true,
		},
	})
	if err != nil {
		return &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: err}
	}

	prompt, renderErr := renderPrompt(workspace, artifacts, allowedIDs, instruction)
	if renderErr != nil {
		cleanupErr := workspace.Cleanup()
		if cleanupErr != nil {
			return errors.Join(renderErr, fmt.Errorf("clean isolated workspace: %w", cleanupErr))
		}
		return renderErr
	}

	response, runErr := rt.Run(ctx, councilruntime.AgentRequest{
		RunID:       req.RunID,
		RunRoot:     req.RunRoot,
		Participant: participant,
		Role:        role,
		Phase:       phase,
		Prompt:      prompt,
		Workdir:     workspace.Root,
	})
	cleanupErr := workspace.Cleanup()
	if runErr != nil {
		if cleanupErr != nil {
			return errors.Join(runErr, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: cleanupErr})
		}
		return runErr
	}
	if cleanupErr != nil {
		return &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: fmt.Errorf("clean isolated workspace: %w", cleanupErr)}
	}

	decoder := json.NewDecoder(strings.NewReader(response.Stdout))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return malformed(phase, fmt.Errorf("decode JSON output: %w", err))
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return malformed(phase, fmt.Errorf("multiple JSON values in output"))
		}
		return malformed(phase, fmt.Errorf("trailing output: %w", err))
	}
	return nil
}

func renderPrompt(workspace visibility.Workspace, artifacts []visibility.Artifact, allowedIDs []string, instruction string) (string, error) {
	byID := make(map[string]visibility.Artifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	allowed := append([]string(nil), allowedIDs...)
	sort.Strings(allowed)

	var b strings.Builder
	b.WriteString(instruction)
	b.WriteString("\n\nVISIBLE_ARTIFACTS_BEGIN\n")
	for _, id := range allowed {
		artifact, ok := byID[id]
		if !ok {
			return "", fmt.Errorf("allowed artifact %q is missing", id)
		}
		path := filepath.Join(workspace.Root, artifact.RelativePath)
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read materialized artifact %q: %w", id, err)
		}
		b.WriteString("--- artifact: ")
		b.WriteString(id)
		b.WriteString(" ---\n")
		b.Write(content)
		b.WriteString("\n")
	}
	b.WriteString("VISIBLE_ARTIFACTS_END\n")
	return b.String(), nil
}

func validateNormalizedProblem(raw json.RawMessage) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("normalized problem is required")
	}
	var object map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("normalized problem must be a JSON object: %w", err)
	}
	if object == nil || len(object) == 0 {
		return nil, fmt.Errorf("normalized problem must be a non-empty JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("normalized problem must contain exactly one JSON object")
	}
	compact := new(bytes.Buffer)
	if err := json.Compact(compact, raw); err != nil {
		return nil, fmt.Errorf("compact normalized problem: %w", err)
	}
	return compact.Bytes(), nil
}

func visibilityJSONArtifact(id, path string, value any) (visibility.Artifact, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return visibility.Artifact{}, fmt.Errorf("marshal artifact %q: %w", id, err)
	}
	return visibility.Artifact{ID: id, RelativePath: path, Content: data}, nil
}

func parallel2[T any](ctx context.Context, first, second func(context.Context) (T, error)) ([2]T, error) {
	var values [2]T
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan pairResult[T], 2)
	go func() {
		value, err := first(childCtx)
		results <- pairResult[T]{index: 0, value: value, err: err}
	}()
	go func() {
		value, err := second(childCtx)
		results <- pairResult[T]{index: 1, value: value, err: err}
	}()

	var firstErr error
	for range 2 {
		result := <-results
		values[result.index] = result.value
		if result.err != nil && firstErr == nil {
			firstErr = result.err
			cancel()
		}
	}
	return values, firstErr
}

func (e Engine) challengeDecision(research [2]ResearchArtifact) ChallengeDecision {
	threshold := e.ChallengePolicy.HighConfidenceThreshold
	if threshold <= 0 || threshold > 1 {
		threshold = defaultHighConfidenceThreshold
	}
	agreement := canonicalText(research[0].Recommendation) == canonicalText(research[1].Recommendation)
	mode := ChallengeFull
	if e.ChallengePolicy.AllowAbbreviated && agreement && research[0].Confidence >= threshold && research[1].Confidence >= threshold {
		mode = ChallengeAbbreviated
	}
	return ChallengeDecision{
		Mode:                    mode,
		MaterialAgreement:       agreement,
		HighConfidenceThreshold: threshold,
		ResearchConfidences:     [2]float64{research[0].Confidence, research[1].Confidence},
	}
}

func buildDecision(judges [2]JudgeArtifact) DecisionRecord {
	agreement := canonicalText(judges[0].Decision) == canonicalText(judges[1].Decision)
	status := DecisionAgreed
	if !agreement {
		status = DecisionJudgeDisagreement
	}
	minority := append([]string(nil), judges[0].Minority...)
	minority = append(minority, judges[1].Minority...)
	unresolved := append([]string(nil), judges[0].Unresolved...)
	unresolved = append(unresolved, judges[1].Unresolved...)
	next := append([]string(nil), judges[0].NextValidation...)
	next = append(next, judges[1].NextValidation...)
	return DecisionRecord{
		Status:          status,
		JudgeAgreement:  agreement,
		JudgeDecisions:  [2]string{judges[0].Decision, judges[1].Decision},
		MinorityReport:  minority,
		Unresolved:      unresolved,
		NextValidations: next,
	}
}

func canonicalText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func validateResearch(value ResearchArtifact) error {
	if strings.TrimSpace(value.Recommendation) == "" {
		return fmt.Errorf("recommendation is required")
	}
	return validateConfidence(value.Confidence)
}

func validateReview(value ReviewArtifact) error {
	return validateConfidence(value.Confidence)
}

func validateChallenge(value ChallengeArtifact) error {
	return validateConfidence(value.Confidence)
}

func validateRebuttal(value RebuttalArtifact) error {
	if strings.TrimSpace(value.UpdatedRecommendation) == "" {
		return fmt.Errorf("updated_recommendation is required")
	}
	return validateConfidence(value.UpdatedConfidence)
}

func validateJudge(value JudgeArtifact) error {
	if strings.TrimSpace(value.Decision) == "" {
		return fmt.Errorf("decision is required")
	}
	if len(value.CitationChecks) == 0 {
		return fmt.Errorf("citation_checks is required")
	}
	return validateConfidence(value.Confidence)
}

func validateConfidence(value float64) error {
	if value < 0 || value > 1 {
		return fmt.Errorf("confidence %.4f must be between 0 and 1", value)
	}
	return nil
}

func malformed(phase string, err error) error {
	return &councilruntime.RunError{Class: councilruntime.FailureMalformedOutput, Err: fmt.Errorf("%s artifact: %w", phase, err)}
}

func researchInstruction() string {
	return `RESEARCH_PHASE
Work independently. Do not infer or seek peer output. Return JSON only with exactly these fields: recommendation (string), reasoning (string[]), considerations (string[]), assumptions (string[]), risks (string[]), evidence_needed (string[]), citations ({artifact_id,locator,claim}[]), confidence (0..1). Citations may reference only visible artifact IDs.`
}

func reviewInstruction() string {
	return `BLIND_CROSS_REVIEW_PHASE
Review only the single target research report visible below. Do not infer author/provider identity, peer review, or consensus. Return JSON only with exactly these fields: strengths, weaknesses, unsupported, missing, incorrect_assumptions, critical_risks, recommended_changes (all string[]), confidence (0..1).`
}

func challengeInstruction(mode ChallengeMode) string {
	if mode == ChallengeAbbreviated {
		return `ABBREVIATED_CHALLENGE
The two independent researchers materially agree at the configured high-confidence threshold. Do not skip challenge. Produce a short falsification-oriented challenge identifying the strongest attack, decisive falsifier, and evidence gap. Return JSON only with exactly: attacks (string[]), falsifiers (string[]), evidence_gaps (string[]), confidence (0..1).`
	}
	return `FULL_CHALLENGE
Attack the emerging recommendations rather than seeking consensus. Look for falsifiers, unsupported assumptions, and decisive evidence gaps. Return JSON only with exactly: attacks (string[]), falsifiers (string[]), evidence_gaps (string[]), confidence (0..1).`
}

func rebuttalInstruction() string {
	return `REBUTTAL_PHASE
Reassess your own report using only criticism relevant to it. Accept or reject criticism based on evidence, not social pressure. Return JSON only with exactly: accepted_criticisms (string[]), rejected_criticisms (string[]), changed_position (bool), position_correct_before_flip (bool), updated_recommendation (string), updated_confidence (0..1), reasons (string[]). position_correct_before_flip must state whether your pre-change position was actually correct on the visible evidence so later evaluation can detect an incorrect concession.`
}

func judgeInstruction() string {
	return `FRESH_BLIND_JUDGE_PHASE
Decide independently. Provider identities are intentionally hidden and no other judge verdict is visible. Do not optimize for consensus. Before relying on any researcher citation or evidence reference, verify the cited claim against the actual visible artifact contents; missing, mismatched, or unverifiable citations must not receive evidentiary credit. Return JSON only with exactly: decision (string), confidence (0..1), action (string), reasons (string[]), evidence (string[]), rejected_alternatives (string[]), minority (string[]), unresolved (string[]), assumptions (string[]), change_conditions (string[]), next_validation (string[]), citation_checks ({reference,status,note}[]).`
}
