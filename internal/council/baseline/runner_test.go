package baseline

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type fakeRuntime struct {
	provider       councilruntime.Provider
	recommendation string
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
	case "baseline-draft":
		output = AnswerArtifact{
			Decision:   "draft " + f.recommendation,
			Action:     "draft action",
			Reasons:    []string{"draft reason"},
			Assumptions: []string{"draft assumption"},
			Risks:      []string{"draft risk"},
			Citations: []protocol.EvidenceRef{{
				ArtifactID: "problem",
				Locator:    "problem",
				Claim:      "draft claim",
			}},
			Confidence: 0.7,
		}
	case "baseline-final":
		output = AnswerArtifact{
			Decision:   "final " + f.recommendation,
			Action:     "final action",
			Reasons:    []string{"final reason"},
			Assumptions: []string{"final assumption"},
			Risks:      []string{"final risk"},
			Citations: []protocol.EvidenceRef{{
				ArtifactID: "problem",
				Locator:    "problem",
				Claim:      "final claim",
			}},
			Confidence: 0.8,
		}
	case protocol.PhaseResearch:
		output = protocol.ResearchArtifact{
			Recommendation: f.recommendation,
			Reasoning:      []string{"reason"},
			Considerations: []string{"consideration"},
			Assumptions:    []string{"assumption"},
			Risks:          []string{"risk"},
			EvidenceNeeded: []string{"evidence"},
			Citations: []protocol.EvidenceRef{{
				ArtifactID: "problem",
				Locator:    "problem",
				Claim:      "research claim",
			}},
			Confidence: 0.8,
		}
	case protocol.PhaseReview:
		output = protocol.ReviewArtifact{
			Strengths:            []string{"strength"},
			Weaknesses:           []string{"weakness"},
			Unsupported:          []string{"unsupported"},
			Missing:              []string{"missing"},
			IncorrectAssumptions: []string{"incorrect assumption"},
			CriticalRisks:        []string{"critical risk"},
			RecommendedChanges:   []string{"change"},
			Confidence:           0.8,
		}
	case protocol.PhaseChallenge:
		output = protocol.ChallengeArtifact{
			Attacks:      []string{"attack"},
			Falsifiers:   []string{"falsifier"},
			EvidenceGaps: []string{"gap"},
			Confidence:   0.8,
		}
	case protocol.PhaseRebuttal:
		output = protocol.RebuttalArtifact{
			AcceptedCriticisms:        []string{"accepted"},
			RejectedCriticisms:        []string{"rejected"},
			ChangedPosition:           false,
			PositionCorrectBeforeFlip: true,
			UpdatedRecommendation:     f.recommendation + " updated",
			UpdatedConfidence:         0.8,
			Reasons:                   []string{"rebuttal reason"},
		}
	case protocol.PhaseJudge:
		output = protocol.JudgeArtifact{
			Decision:             f.judgeDecision,
			Confidence:           0.8,
			Action:               "act",
			Reasons:              []string{"judge reason"},
			Evidence:             []string{"verified evidence"},
			RejectedAlternatives: []string{"alternative"},
			Minority:             []string{"minority"},
			Unresolved:           []string{"unresolved"},
			Assumptions:          []string{"judge assumption"},
			ChangeConditions:     []string{"change condition"},
			NextValidation:       []string{"validate"},
			CitationChecks: []protocol.CitationCheck{{
				Reference: "problem:problem",
				Status:    "verified",
				Note:      "checked",
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

func newRunner(t *testing.T) (Runner, *fakeRuntime, *fakeRuntime) {
	t.Helper()
	claude := &fakeRuntime{
		provider:       councilruntime.ProviderClaude,
		recommendation: "alpha recommendation",
		judgeDecision:  "choose alpha",
	}
	codex := &fakeRuntime{
		provider:       councilruntime.ProviderCodex,
		recommendation: "beta recommendation",
		judgeDecision:  "choose beta",
	}
	return Runner{
		Claude:             claude,
		Codex:              codex,
		TempRoot:           t.TempDir(),
		ChallengerProvider: councilruntime.ProviderClaude,
	}, claude, codex
}

func runRequest(t *testing.T) RunRequest {
	t.Helper()
	return RunRequest{
		RunID:             "baseline-test",
		RunRoot:           t.TempDir(),
		NormalizedProblem: json.RawMessage(`{"problem":"choose a safe implementation"}`),
	}
}

func TestRunnerExactArmCallBudgetsAndProviderRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		arm        Arm
		claudeWant int
		codexWant  int
		answer     bool
		council    bool
	}{
		{name: "A Claude single", arm: ArmAClaudeSingle, claudeWant: 1, answer: true},
		{name: "B Codex single", arm: ArmBCodexSingle, codexWant: 1, answer: true},
		{name: "C Claude self-review", arm: ArmCClaudeSelfReview, claudeWant: 2, answer: true},
		{name: "D Codex self-review", arm: ArmDCodexSelfReview, codexWant: 2, answer: true},
		{name: "E full-info", arm: ArmEFullInfo, claudeWant: 5, codexWant: 4, council: true},
		{name: "F blind Council", arm: ArmFBlindCouncil, claudeWant: 5, codexWant: 4, council: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runner, claude, codex := newRunner(t)
			req := runRequest(t)
			result, err := runner.RunArm(context.Background(), req, tt.arm)
			if err != nil {
				t.Fatalf("RunArm() error = %v", err)
			}
			if result.Arm != tt.arm {
				t.Fatalf("result arm = %q, want %q", result.Arm, tt.arm)
			}
			if got := len(claude.snapshot()); got != tt.claudeWant {
				t.Fatalf("Claude calls = %d, want %d", got, tt.claudeWant)
			}
			if got := len(codex.snapshot()); got != tt.codexWant {
				t.Fatalf("Codex calls = %d, want %d", got, tt.codexWant)
			}
			if result.InvocationCount != tt.claudeWant+tt.codexWant {
				t.Fatalf("invocation_count = %d, want %d", result.InvocationCount, tt.claudeWant+tt.codexWant)
			}
			if (result.Answer != nil) != tt.answer || (result.Protocol != nil) != tt.council {
				t.Fatalf("result shape answer=%v protocol=%v", result.Answer != nil, result.Protocol != nil)
			}

			seen := map[string]struct{}{}
			for _, call := range append(claude.snapshot(), codex.snapshot()...) {
				rel, relErr := filepath.Rel(req.RunRoot, call.Workdir)
				if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					t.Fatalf("workdir %q is inside run root %q", call.Workdir, req.RunRoot)
				}
				if _, duplicate := seen[call.Workdir]; duplicate {
					t.Fatalf("workdir reused: %q", call.Workdir)
				}
				seen[call.Workdir] = struct{}{}
			}
		})
	}
}

func TestRunnerSelfReviewUsesFreshInvocationAndOwnDraft(t *testing.T) {
	t.Parallel()

	runner, claude, _ := newRunner(t)
	_, err := runner.RunArm(context.Background(), runRequest(t), ArmCClaudeSelfReview)
	if err != nil {
		t.Fatalf("RunArm() error = %v", err)
	}
	calls := claude.snapshot()
	if len(calls) != 2 {
		t.Fatalf("Claude calls = %d, want 2", len(calls))
	}
	if calls[0].Phase != "baseline-draft" || calls[1].Phase != "baseline-final" {
		t.Fatalf("phases = %q, %q", calls[0].Phase, calls[1].Phase)
	}
	if calls[0].Workdir == calls[1].Workdir {
		t.Fatalf("self-review reused workdir %q", calls[0].Workdir)
	}
	if !strings.Contains(calls[1].Prompt, "draft alpha recommendation") {
		t.Fatalf("self-review prompt missing own draft: %q", calls[1].Prompt)
	}
}

func TestRunnerFullInfoAndBlindCouncilDifferAtReviewVisibilityOnly(t *testing.T) {
	t.Parallel()

	fullRunner, fullClaude, fullCodex := newRunner(t)
	if _, err := fullRunner.RunArm(context.Background(), runRequest(t), ArmEFullInfo); err != nil {
		t.Fatalf("full-info RunArm() error = %v", err)
	}
	blindRunner, blindClaude, blindCodex := newRunner(t)
	if _, err := blindRunner.RunArm(context.Background(), runRequest(t), ArmFBlindCouncil); err != nil {
		t.Fatalf("blind RunArm() error = %v", err)
	}

	assertReviewVisibility := func(t *testing.T, calls []councilruntime.AgentRequest, full bool) {
		t.Helper()
		count := 0
		for _, call := range calls {
			if call.Phase != protocol.PhaseReview {
				continue
			}
			count++
			hasAlpha := strings.Contains(call.Prompt, "alpha recommendation")
			hasBeta := strings.Contains(call.Prompt, "beta recommendation")
			if full && (!hasAlpha || !hasBeta) {
				t.Fatalf("full-info reviewer missing peer artifacts: %q", call.Prompt)
			}
			if !full && hasAlpha == hasBeta {
				t.Fatalf("blind reviewer must see exactly one research report: %q", call.Prompt)
			}
		}
		if count != 2 {
			t.Fatalf("review calls = %d, want 2", count)
		}
	}

	assertReviewVisibility(t, append(fullClaude.snapshot(), fullCodex.snapshot()...), true)
	assertReviewVisibility(t, append(blindClaude.snapshot(), blindCodex.snapshot()...), false)
}

func TestRunAllReturnsFrozenArmOrder(t *testing.T) {
	t.Parallel()

	runner, _, _ := newRunner(t)
	results, err := runner.RunAll(context.Background(), runRequest(t))
	if err != nil {
		t.Fatalf("RunAll() error = %v", err)
	}
	want := []Arm{
		ArmAClaudeSingle,
		ArmBCodexSingle,
		ArmCClaudeSelfReview,
		ArmDCodexSelfReview,
		ArmEFullInfo,
		ArmFBlindCouncil,
	}
	if len(results) != len(want) {
		t.Fatalf("results = %d, want %d", len(results), len(want))
	}
	for i := range want {
		if results[i].Arm != want[i] {
			t.Fatalf("result[%d].Arm = %q, want %q", i, results[i].Arm, want[i])
		}
	}
}
