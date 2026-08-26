package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type fakeRuntime struct {
	provider       councilruntime.Provider
	recommendation string
	confidence     float64
	judgeDecision  string

	mu    sync.Mutex
	calls []councilruntime.AgentRequest
}

func (f *fakeRuntime) Run(_ context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	f.mu.Unlock()

	var output any
	switch req.Phase {
	case PhaseResearch:
		output = ResearchArtifact{
			Recommendation: f.recommendation,
			Reasoning:      []string{"reason"},
			Considerations: []string{"consideration"},
			Assumptions:    []string{"assumption"},
			Risks:          []string{"risk"},
			EvidenceNeeded: []string{"evidence"},
			Citations: []EvidenceRef{{
				ArtifactID: "problem",
				Locator:    "problem",
				Claim:      "problem supports the recommendation",
			}},
			Confidence: f.confidence,
		}
	case PhaseReview:
		output = ReviewArtifact{
			Strengths:            []string{"strength"},
			Weaknesses:           []string{"weakness"},
			Unsupported:          []string{"unsupported"},
			Missing:              []string{"missing"},
			IncorrectAssumptions: []string{"incorrect assumption"},
			CriticalRisks:        []string{"critical risk"},
			RecommendedChanges:   []string{"change"},
			Confidence:           0.8,
		}
	case PhaseChallenge:
		output = ChallengeArtifact{
			Attacks:      []string{"attack"},
			Falsifiers:   []string{"falsifier"},
			EvidenceGaps: []string{"gap"},
			Confidence:   0.8,
		}
	case PhaseRebuttal:
		output = RebuttalArtifact{
			AcceptedCriticisms:        []string{"accepted"},
			RejectedCriticisms:        []string{"rejected"},
			ChangedPosition:            true,
			PositionCorrectBeforeFlip: true,
			UpdatedRecommendation:      f.recommendation + " updated",
			UpdatedConfidence:          0.75,
			Reasons:                    []string{"evidence changed the position"},
		}
	case PhaseJudge:
		output = JudgeArtifact{
			Decision:             f.judgeDecision,
			Confidence:           0.85,
			Action:               "act",
			Reasons:              []string{"judge reason"},
			Evidence:             []string{"verified evidence"},
			RejectedAlternatives: []string{"alternative"},
			Minority:             []string{"minority"},
			Unresolved:           []string{"unresolved"},
			Assumptions:          []string{"judge assumption"},
			ChangeConditions:     []string{"new evidence"},
			NextValidation:       []string{"validate"},
			CitationChecks: []CitationCheck{{
				Reference: "problem:problem",
				Status:    "verified",
				Note:      "matched allowed artifact",
			}},
		}
	default:
		return councilruntime.AgentResponse{}, fmt.Errorf("unexpected phase %q", req.Phase)
	}

	data, err := json.Marshal(output)
	if err != nil {
		return councilruntime.AgentResponse{}, err
	}
	return councilruntime.AgentResponse{Provider: f.provider, Stdout: string(data), ExitCode: 0, Attempts: 1}, nil
}

func (f *fakeRuntime) snapshot() []councilruntime.AgentRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]councilruntime.AgentRequest(nil), f.calls...)
}

func TestEngineRunsBlindProtocolAndDualFreshJudges(t *testing.T) {
	t.Parallel()

	runRoot := t.TempDir()
	tempRoot := t.TempDir()
	claude := &fakeRuntime{
		provider:       councilruntime.ProviderClaude,
		recommendation: "alpha recommendation",
		confidence:     0.92,
		judgeDecision:  "choose alpha",
	}
	codex := &fakeRuntime{
		provider:       councilruntime.ProviderCodex,
		recommendation: "beta recommendation",
		confidence:     0.91,
		judgeDecision:  "choose beta",
	}

	engine := Engine{
		Claude:             claude,
		Codex:              codex,
		TempRoot:           tempRoot,
		ChallengerProvider: councilruntime.ProviderClaude,
	}
	result, err := engine.Run(context.Background(), RunRequest{
		RunID:             "phase-d-test",
		RunRoot:           runRoot,
		NormalizedProblem: json.RawMessage(`{"problem":"choose a safe implementation"}`),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ChallengeDecision.Mode != ChallengeFull {
		t.Fatalf("challenge mode = %q, want %q", result.ChallengeDecision.Mode, ChallengeFull)
	}
	if len(result.Research) != 2 || len(result.Reviews) != 2 || len(result.Rebuttals) != 2 || len(result.Judges) != 2 {
		t.Fatalf("unexpected artifact counts: research=%d reviews=%d rebuttals=%d judges=%d", len(result.Research), len(result.Reviews), len(result.Rebuttals), len(result.Judges))
	}
	if !result.Rebuttals[0].Artifact.PositionCorrectBeforeFlip || !result.Rebuttals[1].Artifact.PositionCorrectBeforeFlip {
		t.Fatal("rebuttal position_correct_before_flip was not preserved")
	}
	if result.Decision.Status != DecisionJudgeDisagreement || result.Decision.JudgeAgreement {
		t.Fatalf("decision = %+v, want judge disagreement", result.Decision)
	}

	allCalls := append(claude.snapshot(), codex.snapshot()...)
	if len(allCalls) != 9 {
		t.Fatalf("runtime calls = %d, want 9", len(allCalls))
	}

	var reviewPrompts []string
	var judgePrompts []string
	seenWorkdirs := map[string]struct{}{}
	for _, call := range allCalls {
		inside, err := filepath.Rel(runRoot, call.Workdir)
		if err == nil && inside != ".." && !strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
			t.Fatalf("workdir %q is inside run root %q", call.Workdir, runRoot)
		}
		if _, duplicate := seenWorkdirs[call.Workdir]; duplicate {
			t.Fatalf("workdir reused across fresh invocations: %q", call.Workdir)
		}
		seenWorkdirs[call.Workdir] = struct{}{}

		switch call.Phase {
		case PhaseReview:
			reviewPrompts = append(reviewPrompts, call.Prompt)
		case PhaseJudge:
			judgePrompts = append(judgePrompts, call.Prompt)
		}
	}
	if len(reviewPrompts) != 2 {
		t.Fatalf("review prompts = %d, want 2", len(reviewPrompts))
	}
	for _, prompt := range reviewPrompts {
		containsAlpha := strings.Contains(prompt, "alpha recommendation")
		containsBeta := strings.Contains(prompt, "beta recommendation")
		if containsAlpha == containsBeta {
			t.Fatalf("blind reviewer prompt must contain exactly one target report: %q", prompt)
		}
	}
	for _, prompt := range judgePrompts {
		lower := strings.ToLower(prompt)
		if strings.Contains(lower, `"provider":"claude"`) || strings.Contains(lower, `"provider":"codex"`) {
			t.Fatalf("judge prompt leaked provider identity: %q", prompt)
		}
		if !strings.Contains(lower, "verify") || !strings.Contains(lower, "citation") {
			t.Fatalf("judge prompt does not require citation verification: %q", prompt)
		}
	}
}

func TestEngineUsesAuditedAbbreviatedChallengeOnHighConfidenceAgreement(t *testing.T) {
	t.Parallel()

	claude := &fakeRuntime{
		provider:       councilruntime.ProviderClaude,
		recommendation: "same recommendation",
		confidence:     0.95,
		judgeDecision:  "same decision",
	}
	codex := &fakeRuntime{
		provider:       councilruntime.ProviderCodex,
		recommendation: "  SAME   recommendation ",
		confidence:     0.96,
		judgeDecision:  "same decision",
	}

	engine := Engine{
		Claude:             claude,
		Codex:              codex,
		TempRoot:           t.TempDir(),
		ChallengerProvider: councilruntime.ProviderCodex,
		ChallengePolicy: ChallengePolicy{
			AllowAbbreviated:       true,
			HighConfidenceThreshold: 0.9,
		},
	}
	result, err := engine.Run(context.Background(), RunRequest{
		RunID:             "abbreviated-test",
		RunRoot:           t.TempDir(),
		NormalizedProblem: json.RawMessage(`{"problem":"test shortcut"}`),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	decision := result.ChallengeDecision
	if decision.Mode != ChallengeAbbreviated || !decision.MaterialAgreement {
		t.Fatalf("challenge decision = %+v, want audited abbreviated challenge", decision)
	}
	if decision.HighConfidenceThreshold != 0.9 || decision.ResearchConfidences != [2]float64{0.95, 0.96} {
		t.Fatalf("challenge trigger values not logged: %+v", decision)
	}

	challengeCalls := 0
	for _, call := range append(claude.snapshot(), codex.snapshot()...) {
		if call.Phase == PhaseChallenge {
			challengeCalls++
			if !strings.Contains(call.Prompt, "ABBREVIATED_CHALLENGE") {
				t.Fatalf("abbreviated challenge was not explicitly logged in prompt: %q", call.Prompt)
			}
		}
	}
	if challengeCalls != 1 {
		t.Fatalf("challenge calls = %d, want 1; shortcut must not silently omit challenge artifact", challengeCalls)
	}
	if result.Challenge.Artifact.Attacks == nil {
		t.Fatal("abbreviated challenge artifact missing")
	}
}

func TestEngineRejectsMalformedNormalizedProblem(t *testing.T) {
	t.Parallel()

	engine := Engine{
		Claude:             &fakeRuntime{provider: councilruntime.ProviderClaude},
		Codex:              &fakeRuntime{provider: councilruntime.ProviderCodex},
		TempRoot:           t.TempDir(),
		ChallengerProvider: councilruntime.ProviderClaude,
	}
	_, err := engine.Run(context.Background(), RunRequest{
		RunID:             "bad-problem",
		RunRoot:           t.TempDir(),
		NormalizedProblem: json.RawMessage(`[]`),
	})
	if err == nil || !strings.Contains(err.Error(), "normalized problem") {
		t.Fatalf("Run() error = %v, want normalized problem validation error", err)
	}
}
