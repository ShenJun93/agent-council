package protocol

import (
	"context"
	"fmt"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/visibility"
)

// FullInfoEngine is the Phase F comparator for the blind protocol. It keeps
// the same providers, phases, and call budget as Engine while exposing all
// already-produced peer artifacts that are appropriate for the current phase.
type FullInfoEngine struct {
	Engine
}

func (e FullInfoEngine) Run(ctx context.Context, req RunRequest) (Result, error) {
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

	fullResearch := []string{"problem", "research-1", "research-2"}
	reviews, err := parallel2(ctx,
		func(callCtx context.Context) (ReviewArtifact, error) {
			var out ReviewArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeReviewer1(), "reviewer-1", "reviewer", PhaseReview, artifacts, fullResearch, reviewInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (ReviewArtifact, error) {
			var out ReviewArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeReviewer2(), "reviewer-2", "reviewer", PhaseReview, artifacts, fullResearch, reviewInstruction(), &out)
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
	challengeInputs := []string{"problem", "research-1", "research-2", "review-1", "review-2"}
	var challenge ChallengeArtifact
	if err := e.invokeJSON(
		ctx,
		req,
		challengeRuntime,
		"challenger",
		"challenger",
		PhaseChallenge,
		artifacts,
		challengeInputs,
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

	fullRebuttal := []string{"problem", "research-1", "research-2", "review-1", "review-2", "challenge"}
	rebuttals, err := parallel2(ctx,
		func(callCtx context.Context) (RebuttalArtifact, error) {
			var out RebuttalArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeResearcher1(), "researcher-1", "researcher", PhaseRebuttal, artifacts, fullRebuttal, rebuttalInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (RebuttalArtifact, error) {
			var out RebuttalArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeResearcher2(), "researcher-2", "researcher", PhaseRebuttal, artifacts, fullRebuttal, rebuttalInstruction(), &out)
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

	judgeInputs := []string{
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
			err := e.invokeJSON(callCtx, req, e.runtimeJudge1(), "judge-1", "judge", PhaseJudge, artifacts, judgeInputs, judgeInstruction(), &out)
			return out, err
		},
		func(callCtx context.Context) (JudgeArtifact, error) {
			var out JudgeArtifact
			err := e.invokeJSON(callCtx, req, e.runtimeJudge2(), "judge-2", "judge", PhaseJudge, artifacts, judgeInputs, judgeInstruction(), &out)
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
