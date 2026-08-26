package evalharness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type fakeJudgeRuntime struct {
	provider councilruntime.Provider
	output   string

	mu    sync.Mutex
	calls []councilruntime.AgentRequest
}

func (f *fakeJudgeRuntime) Run(_ context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	f.mu.Unlock()

	output := f.output
	if output == "" {
		score := 80.0
		if f.provider == councilruntime.ProviderCodex {
			score = 70
		}
		artifact := JudgeArtifact{
			OverallScore: score,
			Dimensions: map[string]float64{
				"correctness": score,
				"safety":      score,
			},
			CitationChecks: []protocol.CitationCheck{{
				Reference: "problem:problem",
				Status:    "verified",
				Note:      "matched visible problem artifact",
			}},
			ReliedOnCitations: []string{"problem:problem"},
			CriticalErrors:    []string{},
			Strengths:         []string{"clear"},
			Weaknesses:        []string{},
			Confidence:        0.8,
		}
		data, err := json.Marshal(artifact)
		if err != nil {
			return councilruntime.AgentResponse{}, err
		}
		output = string(data)
	}

	return councilruntime.AgentResponse{
		Provider: f.provider,
		Stdout:   output,
		ExitCode: 0,
		Attempts: 1,
	}, nil
}

func (f *fakeJudgeRuntime) snapshot() []councilruntime.AgentRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]councilruntime.AgentRequest(nil), f.calls...)
}

func testEvalArms() []baseline.ArmResult {
	answer := func() *baseline.AnswerArtifact {
		return &baseline.AnswerArtifact{
			Decision:    "ship candidate",
			Action:      "deploy",
			Reasons:     []string{"reason"},
			Assumptions: []string{"assumption"},
			Risks:       []string{"risk"},
			Citations: []protocol.EvidenceRef{{
				ArtifactID: "problem",
				Locator:    "problem",
				Claim:      "supports candidate",
			}},
			Confidence: 0.8,
		}
	}
	protocolResult := func() *protocol.Result {
		return &protocol.Result{
			Research: []protocol.ResearchRecord{
				{ID: "research-1", Artifact: protocol.ResearchArtifact{Risks: []string{"risk"}, Citations: []protocol.EvidenceRef{{ArtifactID: "problem", Locator: "problem", Claim: "supports candidate"}}}},
			},
			Judges: []protocol.JudgeRecord{
				{ID: "judge-1", Artifact: protocol.JudgeArtifact{Action: "deploy", Reasons: []string{"reason"}, Evidence: []string{"evidence"}}},
				{ID: "judge-2", Artifact: protocol.JudgeArtifact{Action: "deploy", Reasons: []string{"reason"}, Evidence: []string{"evidence"}}},
			},
			Decision: protocol.DecisionRecord{
				Status:         protocol.DecisionAgreed,
				JudgeAgreement: true,
				JudgeDecisions: [2]string{"ship candidate", "ship candidate"},
			},
		}
	}
	return []baseline.ArmResult{
		{Arm: baseline.ArmAClaudeSingle, Answer: answer()},
		{Arm: baseline.ArmBCodexSingle, Answer: answer()},
		{Arm: baseline.ArmCClaudeSelfReview, Answer: answer()},
		{Arm: baseline.ArmDCodexSelfReview, Answer: answer()},
		{Arm: baseline.ArmEFullInfo, Protocol: protocolResult()},
		{Arm: baseline.ArmFBlindCouncil, Protocol: protocolResult()},
	}
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func testProblemRequest(t *testing.T) ProblemRequest {
	t.Helper()
	problem := json.RawMessage(`{"problem":"choose a safe implementation"}`)
	rubric := json.RawMessage(`{"dimensions":[{"id":"correctness"},{"id":"safety"}]}`)
	reference := json.RawMessage(`{"facts":["reference fact"]}`)
	return ProblemRequest{
		ProblemID:          "problem-1",
		RunID:              "eval-run",
		RunRoot:            t.TempDir(),
		NormalizedProblem:  problem,
		Rubric:             rubric,
		RubricSHA256:       sha256Hex(rubric),
		ReferenceSet:       reference,
		ReferenceSetSHA256: sha256Hex(reference),
		Arms:               testEvalArms(),
		RiskPolicy: RiskPolicy{
			Comparator:         ComparatorBestSingle,
			MaterialWorseDelta: 5,
		},
	}
}

func TestEvaluateProblemUsesFixedFreshMaskedJudgesForEveryArm(t *testing.T) {
	t.Parallel()

	claude := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude}
	codex := &fakeJudgeRuntime{provider: councilruntime.ProviderCodex}
	harness := Harness{Claude: claude, Codex: codex, TempRoot: t.TempDir()}
	req := testProblemRequest(t)

	result, err := harness.EvaluateProblem(context.Background(), req)
	if err != nil {
		t.Fatalf("EvaluateProblem() error = %v", err)
	}
	if result.ProblemID != req.ProblemID || len(result.Arms) != 6 {
		t.Fatalf("result = %+v", result)
	}
	if len(claude.snapshot()) != 6 || len(codex.snapshot()) != 6 {
		t.Fatalf("judge calls claude/codex = %d/%d, want 6/6", len(claude.snapshot()), len(codex.snapshot()))
	}

	seenWorkdirs := map[string]struct{}{}
	for provider, calls := range map[string][]councilruntime.AgentRequest{
		"claude": claude.snapshot(),
		"codex":  codex.snapshot(),
	} {
		for _, call := range calls {
			if call.Phase != PhaseEvalJudge {
				t.Fatalf("%s phase = %q, want %q", provider, call.Phase, PhaseEvalJudge)
			}
			wantParticipant := "eval-judge-1"
			if provider == "codex" {
				wantParticipant = "eval-judge-2"
			}
			if call.Participant != wantParticipant {
				t.Fatalf("%s participant = %q, want %q", provider, call.Participant, wantParticipant)
			}
			rel, relErr := filepath.Rel(req.RunRoot, call.Workdir)
			if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Fatalf("judge workdir %q is inside run root %q", call.Workdir, req.RunRoot)
			}
			if _, duplicate := seenWorkdirs[call.Workdir]; duplicate {
				t.Fatalf("judge workdir reused: %q", call.Workdir)
			}
			seenWorkdirs[call.Workdir] = struct{}{}

			lower := strings.ToLower(call.Prompt)
			for _, required := range []string{"correctness", "safety", "reference fact", "ship candidate", "verify", "citation"} {
				if !strings.Contains(lower, required) {
					t.Fatalf("judge prompt missing %q: %q", required, call.Prompt)
				}
			}
			for _, forbidden := range []string{`"arm":"a"`, `"arm":"b"`, `"arm":"c"`, `"arm":"d"`, `"arm":"e"`, `"arm":"f"`, `"provider"`, `overall_score`} {
				if strings.Contains(lower, forbidden) {
					t.Fatalf("judge prompt leaked %q: %q", forbidden, call.Prompt)
				}
			}
		}
	}
	if len(seenWorkdirs) != 12 {
		t.Fatalf("unique judge workdirs = %d, want 12", len(seenWorkdirs))
	}

	for _, armScore := range result.Arms {
		if armScore.Judges[0].Provider != councilruntime.ProviderClaude || armScore.Judges[1].Provider != councilruntime.ProviderCodex {
			t.Fatalf("arm %s judge providers = %v/%v", armScore.Arm, armScore.Judges[0].Provider, armScore.Judges[1].Provider)
		}
		if armScore.MeanScore != 75 || armScore.JudgeSpread != 10 {
			t.Fatalf("arm %s aggregate = mean %.2f spread %.2f", armScore.Arm, armScore.MeanScore, armScore.JudgeSpread)
		}
	}
}

func TestEvaluateProblemRejectsHashMismatchBeforeRuntime(t *testing.T) {
	t.Parallel()

	claude := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude}
	codex := &fakeJudgeRuntime{provider: councilruntime.ProviderCodex}
	harness := Harness{Claude: claude, Codex: codex, TempRoot: t.TempDir()}
	req := testProblemRequest(t)
	req.RubricSHA256 = strings.Repeat("0", 64)

	_, err := harness.EvaluateProblem(context.Background(), req)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "rubric") || !strings.Contains(strings.ToLower(err.Error()), "hash") {
		t.Fatalf("EvaluateProblem() error = %v, want rubric hash failure", err)
	}
	if len(claude.snapshot()) != 0 || len(codex.snapshot()) != 0 {
		t.Fatalf("runtime called before hash validation: claude=%d codex=%d", len(claude.snapshot()), len(codex.snapshot()))
	}
}

func TestEvaluateProblemRejectsUnverifiedReliedCitation(t *testing.T) {
	t.Parallel()

	badArtifact := `{"overall_score":80,"dimensions":{"correctness":80,"safety":80},"citation_checks":[{"reference":"problem:problem","status":"unsupported","note":"not verified"}],"relied_on_citations":["problem:problem"],"critical_errors":[],"strengths":[],"weaknesses":[],"confidence":0.8}`
	claude := &fakeJudgeRuntime{provider: councilruntime.ProviderClaude, output: badArtifact}
	codex := &fakeJudgeRuntime{provider: councilruntime.ProviderCodex}
	harness := Harness{Claude: claude, Codex: codex, TempRoot: t.TempDir()}

	_, err := harness.EvaluateProblem(context.Background(), testProblemRequest(t))
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "citation") {
		t.Fatalf("EvaluateProblem() error = %v, want citation verification failure", err)
	}
}

func TestEvaluateProblemRejectsWrongArmSet(t *testing.T) {
	t.Parallel()

	harness := Harness{
		Claude:   &fakeJudgeRuntime{provider: councilruntime.ProviderClaude},
		Codex:    &fakeJudgeRuntime{provider: councilruntime.ProviderCodex},
		TempRoot: t.TempDir(),
	}
	req := testProblemRequest(t)
	req.Arms = append(req.Arms[:5], req.Arms[0])
	_, err := harness.EvaluateProblem(context.Background(), req)
	if err == nil {
		t.Fatal("EvaluateProblem() unexpectedly accepted duplicate/missing arm set")
	}
}

func TestValidateJudgeArtifactRejectsMissingExtraOrOutOfRangeDimensions(t *testing.T) {
	t.Parallel()

	dimensions := []string{"correctness", "safety"}
	valid := JudgeArtifact{
		OverallScore: 80,
		Dimensions: map[string]float64{
			"correctness": 80,
			"safety":      70,
		},
		CitationChecks: []protocol.CitationCheck{{Reference: "problem:problem", Status: "verified", Note: "ok"}},
		Confidence:     0.8,
	}
	if err := validateJudgeArtifact(valid, dimensions); err != nil {
		t.Fatalf("valid judge artifact rejected: %v", err)
	}

	cases := []JudgeArtifact{
		func() JudgeArtifact { value := valid; value.OverallScore = 101; return value }(),
		func() JudgeArtifact { value := valid; value.Dimensions = map[string]float64{"correctness": 80}; return value }(),
		func() JudgeArtifact { value := valid; value.Dimensions = map[string]float64{"correctness": 80, "safety": 70, "extra": 60}; return value }(),
		func() JudgeArtifact { value := valid; value.Dimensions = map[string]float64{"correctness": -1, "safety": 70}; return value }(),
		func() JudgeArtifact { value := valid; value.Confidence = 2; return value }(),
	}
	for i, artifact := range cases {
		if err := validateJudgeArtifact(artifact, dimensions); err == nil {
			t.Fatalf("case %d unexpectedly accepted: %+v", i, artifact)
		}
	}
}

func TestProblemRequestFixtureHasStableHashes(t *testing.T) {
	t.Parallel()

	req := testProblemRequest(t)
	if got := sha256Hex(req.Rubric); got != req.RubricSHA256 {
		t.Fatalf("rubric hash = %s want %s", got, req.RubricSHA256)
	}
	if got := sha256Hex(req.ReferenceSet); got != req.ReferenceSetSHA256 {
		t.Fatalf("reference hash = %s want %s", got, req.ReferenceSetSHA256)
	}
	if fmt.Sprint(req.RiskPolicy.Comparator) != "best_single" {
		t.Fatalf("unexpected comparator %q", req.RiskPolicy.Comparator)
	}
}
