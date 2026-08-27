package baseline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type citationRuntime struct {
	provider      councilruntime.Provider
	finalArtifact string
	mu            sync.Mutex
	calls         []councilruntime.AgentRequest
}

func (r *citationRuntime) Run(_ context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	r.mu.Lock()
	r.calls = append(r.calls, req)
	r.mu.Unlock()

	artifactID := "problem"
	if req.Phase == "baseline-final" && r.finalArtifact != "" {
		artifactID = r.finalArtifact
	}
	answer := AnswerArtifact{
		Decision:    "self-contained recommendation",
		Action:      "execute safely",
		Reasons:     []string{"reason"},
		Assumptions: []string{"assumption"},
		Risks:       []string{"risk"},
		Citations: []protocol.EvidenceRef{{
			ArtifactID: artifactID,
			Locator:    "e1",
			Claim:      "supporting claim",
		}},
		Confidence: 0.8,
	}
	data, err := json.Marshal(answer)
	if err != nil {
		return councilruntime.AgentResponse{}, err
	}
	return councilruntime.AgentResponse{Provider: r.provider, Stdout: string(data), ExitCode: 0, Attempts: 1}, nil
}

func (r *citationRuntime) snapshot() []councilruntime.AgentRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]councilruntime.AgentRequest(nil), r.calls...)
}

func citationRunner(t *testing.T, authority CitationAuthority, finalArtifact string) (*Runner, *citationRuntime) {
	t.Helper()
	claude := &citationRuntime{provider: councilruntime.ProviderClaude, finalArtifact: finalArtifact}
	codex := &citationRuntime{provider: councilruntime.ProviderCodex, finalArtifact: finalArtifact}
	runner := &Runner{
		Claude:             claude,
		Codex:              codex,
		TempRoot:           t.TempDir(),
		ChallengerProvider: councilruntime.ProviderClaude,
		CitationAuthority:  authority,
	}
	return runner, claude
}

func TestLegacySelfReviewMayCiteVisibleDraft(t *testing.T) {
	runner, _ := citationRunner(t, CitationAuthorityVisibleArtifacts, "draft")
	result, err := runner.RunArm(context.Background(), runRequest(t), ArmCClaudeSelfReview)
	if err != nil {
		t.Fatalf("legacy self-review rejected visible draft citation: %v", err)
	}
	if result.Answer == nil || len(result.Answer.Citations) != 1 || result.Answer.Citations[0].ArtifactID != "draft" {
		t.Fatalf("unexpected legacy result: %#v", result.Answer)
	}
}

func TestH3SelfReviewSeesDraftButOnlyProblemIsCitable(t *testing.T) {
	runner, claude := citationRunner(t, CitationAuthorityProblemOnlyFinal, "problem")
	_, err := runner.RunArm(context.Background(), runRequest(t), ArmCClaudeSelfReview)
	if err != nil {
		t.Fatal(err)
	}
	calls := claude.snapshot()
	if len(calls) != 2 {
		t.Fatalf("calls=%d want 2", len(calls))
	}
	finalPrompt := calls[1].Prompt
	for _, required := range []string{
		"--- artifact: draft ---",
		"self-contained recommendation",
		"CITABLE_ARTIFACT_IDS: problem",
		"self-contained final answer",
	} {
		if !strings.Contains(finalPrompt, required) {
			t.Fatalf("H3 self-review prompt missing %q: %s", required, finalPrompt)
		}
	}
	if strings.Contains(finalPrompt, "CITABLE_ARTIFACT_IDS: draft") || strings.Contains(finalPrompt, "CITABLE_ARTIFACT_IDS: draft,problem") {
		t.Fatalf("draft leaked into H3 citable allowlist: %s", finalPrompt)
	}
}

func TestH3SelfReviewRejectsDraftCitationAsMalformed(t *testing.T) {
	runner, claude := citationRunner(t, CitationAuthorityProblemOnlyFinal, "draft")
	_, err := runner.RunArm(context.Background(), runRequest(t), ArmCClaudeSelfReview)
	if err == nil {
		t.Fatal("H3 self-review accepted private draft citation")
	}
	var runErr *councilruntime.RunError
	if !errors.As(err, &runErr) || runErr.Class != councilruntime.FailureMalformedOutput {
		t.Fatalf("error=%v, want malformed output", err)
	}
	if got := len(claude.snapshot()); got != 2 {
		t.Fatalf("model calls=%d want 2", got)
	}
}

func TestH3SelfReviewAcceptsProblemCitation(t *testing.T) {
	runner, _ := citationRunner(t, CitationAuthorityProblemOnlyFinal, "problem")
	result, err := runner.RunArm(context.Background(), runRequest(t), ArmCClaudeSelfReview)
	if err != nil {
		t.Fatalf("H3 self-review rejected problem citation: %v", err)
	}
	if result.Answer == nil || result.Answer.Citations[0].ArtifactID != "problem" {
		t.Fatalf("unexpected H3 result: %#v", result.Answer)
	}
}
