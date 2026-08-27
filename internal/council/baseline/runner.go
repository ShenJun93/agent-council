package baseline

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
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/visibility"
)

const protocolInvocationCount = 9

var frozenArms = []Arm{
	ArmAClaudeSingle,
	ArmBCodexSingle,
	ArmCClaudeSelfReview,
	ArmDCodexSelfReview,
	ArmEFullInfo,
	ArmFBlindCouncil,
}

func (r Runner) RunAll(ctx context.Context, req RunRequest) ([]ArmResult, error) {
	results := make([]ArmResult, 0, len(frozenArms))
	for _, arm := range frozenArms {
		result, err := r.RunArm(ctx, req, arm)
		if err != nil {
			return nil, fmt.Errorf("baseline arm %s: %w", arm, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func (r Runner) RunArm(ctx context.Context, req RunRequest, arm Arm) (ArmResult, error) {
	problem, err := validateNormalizedProblem(req.NormalizedProblem)
	if err != nil {
		return ArmResult{}, err
	}
	if strings.TrimSpace(req.RunID) == "" {
		return ArmResult{}, fmt.Errorf("run id is required")
	}
	if strings.TrimSpace(req.RunRoot) == "" {
		return ArmResult{}, fmt.Errorf("run root is required")
	}
	if strings.TrimSpace(r.TempRoot) == "" {
		return ArmResult{}, fmt.Errorf("baseline temp root is required")
	}
	if r.Claude == nil || r.Codex == nil {
		return ArmResult{}, fmt.Errorf("both Claude and Codex runtimes are required")
	}
	if r.ChallengerProvider != councilruntime.ProviderClaude && r.ChallengerProvider != councilruntime.ProviderCodex {
		return ArmResult{}, fmt.Errorf("challenger provider must be explicitly set to claude or codex")
	}

	normalized := RunRequest{RunID: req.RunID, RunRoot: req.RunRoot, NormalizedProblem: problem}
	switch arm {
	case ArmAClaudeSingle:
		answer, err := r.runSingle(ctx, normalized, r.Claude, "baseline-a")
		if err != nil {
			return ArmResult{}, err
		}
		return ArmResult{Arm: arm, InvocationCount: 1, Answer: &answer}, nil
	case ArmBCodexSingle:
		answer, err := r.runSingle(ctx, normalized, r.Codex, "baseline-b")
		if err != nil {
			return ArmResult{}, err
		}
		return ArmResult{Arm: arm, InvocationCount: 1, Answer: &answer}, nil
	case ArmCClaudeSelfReview:
		answer, err := r.runSelfReview(ctx, normalized, r.Claude, "baseline-c")
		if err != nil {
			return ArmResult{}, err
		}
		return ArmResult{Arm: arm, InvocationCount: 2, Answer: &answer}, nil
	case ArmDCodexSelfReview:
		answer, err := r.runSelfReview(ctx, normalized, r.Codex, "baseline-d")
		if err != nil {
			return ArmResult{}, err
		}
		return ArmResult{Arm: arm, InvocationCount: 2, Answer: &answer}, nil
	case ArmEFullInfo:
		result, err := (protocol.FullInfoEngine{Engine: r.protocolEngine()}).Run(ctx, protocolRequest(normalized))
		if err != nil {
			return ArmResult{}, err
		}
		return ArmResult{Arm: arm, InvocationCount: protocolInvocationCount, Protocol: &result}, nil
	case ArmFBlindCouncil:
		result, err := r.protocolEngine().Run(ctx, protocolRequest(normalized))
		if err != nil {
			return ArmResult{}, err
		}
		return ArmResult{Arm: arm, InvocationCount: protocolInvocationCount, Protocol: &result}, nil
	default:
		return ArmResult{}, fmt.Errorf("unknown baseline arm %q", arm)
	}
}

func (r Runner) protocolEngine() protocol.Engine {
	return protocol.Engine{
		Claude:             r.Claude,
		Codex:              r.Codex,
		TempRoot:           r.TempRoot,
		ChallengerProvider: r.ChallengerProvider,
		ChallengePolicy:    r.ChallengePolicy,
	}
}

func protocolRequest(req RunRequest) protocol.RunRequest {
	return protocol.RunRequest{RunID: req.RunID, RunRoot: req.RunRoot, NormalizedProblem: req.NormalizedProblem}
}

func (r Runner) runSingle(ctx context.Context, req RunRequest, rt councilruntime.AgentRuntime, participant string) (AnswerArtifact, error) {
	problem := visibility.Artifact{ID: "problem", RelativePath: "context/problem.json", Content: req.NormalizedProblem}
	return r.invokeAnswer(ctx, req, rt, participant, "baseline-final", []visibility.Artifact{problem}, []string{"problem"}, finalInstruction())
}

func (r Runner) runSelfReview(ctx context.Context, req RunRequest, rt councilruntime.AgentRuntime, participant string) (AnswerArtifact, error) {
	problem := visibility.Artifact{ID: "problem", RelativePath: "context/problem.json", Content: req.NormalizedProblem}
	draft, err := r.invokeAnswer(ctx, req, rt, participant, "baseline-draft", []visibility.Artifact{problem}, []string{"problem"}, draftInstruction())
	if err != nil {
		return AnswerArtifact{}, err
	}
	draftBytes, err := json.Marshal(draft)
	if err != nil {
		return AnswerArtifact{}, fmt.Errorf("marshal self-review draft: %w", err)
	}
	artifacts := []visibility.Artifact{
		problem,
		{ID: "draft", RelativePath: "context/draft.json", Content: draftBytes},
	}
	return r.invokeAnswer(ctx, req, rt, participant, "baseline-final", artifacts, []string{"problem", "draft"}, selfReviewInstruction())
}

func (r Runner) invokeAnswer(
	ctx context.Context,
	req RunRequest,
	rt councilruntime.AgentRuntime,
	participant string,
	phase string,
	artifacts []visibility.Artifact,
	allowedIDs []string,
	instruction string,
) (AnswerArtifact, error) {
	grants := make([]visibility.Grant, 0, len(allowedIDs))
	for _, id := range allowedIDs {
		grants = append(grants, visibility.Grant{Participant: participant, Phase: phase, ArtifactID: id})
	}
	workspace, err := visibility.Materialize(visibility.Request{
		RunRoot:   req.RunRoot,
		TempRoot:  r.TempRoot,
		Viewer:    visibility.Viewer{Participant: participant, Phase: phase},
		Artifacts: append([]visibility.Artifact(nil), artifacts...),
		Policy:    visibility.Policy{Grants: grants, MaskProviderIdentity: true},
	})
	if err != nil {
		return AnswerArtifact{}, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: err}
	}

	prompt, renderErr := renderPrompt(workspace, artifacts, allowedIDs, instruction)
	if renderErr != nil {
		cleanupErr := workspace.Cleanup()
		if cleanupErr != nil {
			return AnswerArtifact{}, errors.Join(renderErr, fmt.Errorf("clean isolated workspace: %w", cleanupErr))
		}
		return AnswerArtifact{}, renderErr
	}

	response, runErr := rt.Run(ctx, councilruntime.AgentRequest{
		RunID:       req.RunID,
		RunRoot:     req.RunRoot,
		Participant: participant,
		Role:        "baseline",
		Phase:       phase,
		Prompt:      prompt,
		Workdir:     workspace.Root,
	})
	cleanupErr := workspace.Cleanup()
	if runErr != nil {
		if cleanupErr != nil {
			return AnswerArtifact{}, errors.Join(runErr, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: cleanupErr})
		}
		return AnswerArtifact{}, runErr
	}
	if cleanupErr != nil {
		return AnswerArtifact{}, &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: fmt.Errorf("clean isolated workspace: %w", cleanupErr)}
	}

	var answer AnswerArtifact
	if err := decodeStrictJSON(response.Stdout, &answer); err != nil {
		return AnswerArtifact{}, err
	}
	if strings.TrimSpace(answer.Decision) == "" {
		return AnswerArtifact{}, malformed(fmt.Errorf("decision is required"))
	}
	if answer.Confidence < 0 || answer.Confidence > 1 {
		return AnswerArtifact{}, malformed(fmt.Errorf("confidence %.4f must be between 0 and 1", answer.Confidence))
	}
	return answer, nil
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

func validateNormalizedProblem(raw json.RawMessage) (json.RawMessage, error) {
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
	return json.RawMessage(compact.Bytes()), nil
}

func decodeStrictJSON(raw string, out any) error {
	if err := modeloutput.DecodeStrict(raw, out); err != nil {
		return malformed(err)
	}
	return nil
}

func malformed(err error) error {
	return &councilruntime.RunError{Class: councilruntime.FailureMalformedOutput, Err: fmt.Errorf("baseline artifact: %w", err)}
}

func draftInstruction() string {
	return `BASELINE_DRAFT
Work independently and answer the normalized problem. Return JSON only with exactly: decision (string), action (string), reasons (string[]), assumptions (string[]), risks (string[]), citations ({artifact_id,locator,claim}[]), confidence (0..1). Citations may reference only visible artifact IDs.`
}

func finalInstruction() string {
	return `BASELINE_FINAL
Work independently and answer the normalized problem without peer review. Return JSON only with exactly: decision (string), action (string), reasons (string[]), assumptions (string[]), risks (string[]), citations ({artifact_id,locator,claim}[]), confidence (0..1). Citations may reference only visible artifact IDs.`
}

func selfReviewInstruction() string {
	return `BASELINE_SELF_REVIEW
Review your own draft against the normalized problem, correct errors if warranted, and produce the final answer. Do not infer peer output or consensus. Return JSON only with exactly: decision (string), action (string), reasons (string[]), assumptions (string[]), risks (string[]), citations ({artifact_id,locator,claim}[]), confidence (0..1). Citations may reference only visible artifact IDs.`
}
