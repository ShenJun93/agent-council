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

	"github.com/ShenJun93/agent-council/internal/council/modeloutput"
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
	if err := e.validateRuntimes(); err != nil {
		return Result{}, err
	}

	artifacts := []visibility.Artifact{{
		ID:           "problem",
		RelativePath: "context/problem.json",
		Content:      problem,
	}}

	research, err := parallel2(ctx,
		func(callCtx context.Context) (ResearchArtifact, error) {
			var out ResearchArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeResearcher1(), "researcher-1", "researcher", PhaseResearch, artifacts, []string{"problem"}, researchInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (ResearchArtifact, error) {
			var out ResearchArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeResearcher2(), "researcher-2", "researcher", PhaseResearch, artifacts, []string{"problem"}, researchInstruction(), &out)
			return out, err
		},
	)
	if err != nil {
		return Result{}, fmt.Errorf("research phase: %w", err)
	}
	for i := range research {
		if err := validateResearch(research[i]); err != nil {
			return Result{}, malformed("research", err)
		}
	}
	artifacts, err = appendJSONArtifacts(artifacts,
		jsonArtifactSpec{id: "research-1", path: "context/research-1.json", value: research[0]},
		jsonArtifactSpec{id: "research-2", path: "context/research-2.json", value: research[1]},
	)
	if err != nil {
		return Result{}, err
	}

	reviews, err := parallel2(ctx,
		func(callCtx context.Context) (ReviewArtifact, error) {
			var out ReviewArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeReviewer1(), "reviewer-1", "reviewer", PhaseReview, artifacts, []string{"problem", "research-2"}, reviewInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (ReviewArtifact, error) {
			var out ReviewArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeReviewer2(), "reviewer-2", "reviewer", PhaseReview, artifacts, []string{"problem", "research-1"}, reviewInstruction(), &out)
			return out, err
		},
	)
	if err != nil {
		return Result{}, fmt.Errorf("review phase: %w", err)
	}
	for i := range reviews {
		if err := validateReview(reviews[i]); err != nil {
			return Result{}, malformed("review", err)
		}
	}
	artifacts, err = appendJSONArtifacts(artifacts,
		jsonArtifactSpec{id: "review-1", path: "context/review-1.json", value: reviews[0]},
		jsonArtifactSpec{id: "review-2", path: "context/review-2.json", value: reviews[1]},
	)
	if err != nil {
		return Result{}, err
	}

	challengeDecision := e.challengeDecision(research)
	challengeRuntime := e.runtimeChallenger()
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
	artifacts, err = appendJSONArtifacts(artifacts,
		jsonArtifactSpec{id: "challenge", path: "context/challenge.json", value: challenge},
	)
	if err != nil {
		return Result{}, err
	}

	rebuttals, err := parallel2(ctx,
		func(callCtx context.Context) (RebuttalArtifact, error) {
			var out RebuttalArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeResearcher1(), "researcher-1", "researcher", PhaseRebuttal, artifacts, []string{"problem", "research-1", "review-2", "challenge"}, rebuttalInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (RebuttalArtifact, error) {
			var out RebuttalArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeResearcher2(), "researcher-2", "researcher", PhaseRebuttal, artifacts, []string{"problem", "research-2", "review-1", "challenge"}, rebuttalInstruction(), &out)
			return out, err
		},
	)
	if err != nil {
		return Result{}, fmt.Errorf("rebuttal phase: %w", err)
	}
	for i := range rebuttals {
		if err := validateRebuttal(rebuttals[i]); err != nil {
			return Result{}, malformed("rebuttal", err)
		}
	}
	artifacts, err = appendJSONArtifacts(artifacts,
		jsonArtifactSpec{id: "rebuttal-1", path: "context/rebuttal-1.json", value: rebuttals[0]},
		jsonArtifactSpec{id: "rebuttal-2", path: "context/rebuttal-2.json", value: rebuttals[1]},
	)
	if err != nil {
		return Result{}, err
	}

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
	judges, err := parallel2(ctx,
		func(callCtx context.Context) (JudgeArtifact, error) {
			var out JudgeArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeJudge1(), "judge-1", "judge", PhaseJudge, artifacts, judgeAllowed, judgeInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (JudgeArtifact, error) {
			var out JudgeArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeJudge2(), "judge-2", "judge", PhaseJudge, artifacts, judgeAllowed, judgeInstruction(), &out)
			return out, err
		},
	)
	if err != nil {
		return Result{}, fmt.Errorf("judge phase: %w", err)
	}
	for i := range judges {
		if err := validateJudge(judges[i]); err != nil {
			return Result{}, malformed("judge", err)
		}
	}

	return Result{
		Research: []ResearchRecord{
			{ID: "research-1", Artifact: research[0]},
			{ID: "research-2", Artifact: research[1]},
		},
		Reviews: []ReviewRecord{
			{ID: "review-1", TargetID: "research-2", Artifact: reviews[0]},
			{ID: "review-2", TargetID: "research-1", Artifact: reviews[1]},
		},
		ChallengeDecision: challengeDecision,
		Challenge:         ChallengeRecord{ID: "challenge", Artifact: challenge},
		Rebuttals: []RebuttalRecord{
			{ID: "rebuttal-1", TargetID: "research-1", Artifact: rebuttals[0]},
			{ID: "rebuttal-2", TargetID: "research-2", Artifact: rebuttals[1]},
		},
		Judges: []JudgeRecord{
			{ID: "judge-1", Artifact: judges[0]},
			{ID: "judge-2", Artifact: judges[1]},
		},
		Decision: buildDecision(judges),
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
	return decodeStrictJSON(phase, response.Stdout, out)
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
		content, err := os.ReadFile(filepath.Join(workspace.Root, artifact.RelativePath))
		if err != nil {
			return "", fmt.Errorf("read materialized artifact %q: %w", id, err)
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

func decodeStrictJSON(phase, raw string, out any) error {
	if err := modeloutput.DecodeStrict(raw, out); err != nil {
		return malformed(phase, err)
	}
	return nil
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
	if len(object) == 0 {
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

type jsonArtifactSpec struct {
	id    string
	path  string
	value any
}

func appendJSONArtifacts(artifacts []visibility.Artifact, specs ...jsonArtifactSpec) ([]visibility.Artifact, error) {
	for _, spec := range specs {
		data, err := json.Marshal(spec.value)
		if err != nil {
			return nil, fmt.Errorf("marshal artifact %q: %w", spec.id, err)
		}
		artifacts = append(artifacts, visibility.Artifact{ID: spec.id, RelativePath: spec.path, Content: data})
	}
	return artifacts, nil
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
	for i := 0; i < 2; i++ {
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
