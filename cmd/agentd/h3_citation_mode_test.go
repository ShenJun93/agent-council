package main

import (
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func TestNewH3BaselineEnablesProblemOnlyFinalCitationAuthority(t *testing.T) {
	executor := newH3Baseline(nil, nil, t.TempDir(), councilruntime.ProviderClaude)
	runner, ok := executor.(baseline.Runner)
	if !ok {
		t.Fatalf("H3 baseline type=%T want baseline.Runner", executor)
	}
	if runner.CitationAuthority != baseline.CitationAuthorityProblemOnlyFinal {
		t.Fatalf("citation authority=%d want problem-only-final", runner.CitationAuthority)
	}
}
