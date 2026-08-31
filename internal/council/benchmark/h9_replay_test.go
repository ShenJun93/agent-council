package benchmark

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

func TestLoadH9ReplayAcceptsCommittedFrozenReplay(t *testing.T) {
	ds, err := LoadH9Replay(filepath.Join("..", "..", "..", "benchmarks", "h9"))
	if err != nil {
		t.Fatal(err)
	}
	if ds.Manifest.BenchmarkID != H9BenchmarkID {
		t.Fatalf("benchmark=%q", ds.Manifest.BenchmarkID)
	}
	if ds.Manifest.SourceH8RunID != H9SourceH8RunID || ds.Manifest.SourceH8ArtifactDigest != H9SourceH8ArtifactDigest {
		t.Fatalf("source=%+v", ds.Manifest)
	}
	if len(ds.Cases) != H9ReplayCaseCount {
		t.Fatalf("cases=%d", len(ds.Cases))
	}
	for _, c := range ds.Cases {
		if len(c.Arms) != 6 {
			t.Fatalf("case %s arms=%d", c.ID, len(c.Arms))
		}
	}
	if err := validateH9AdapterPolicy(ds.AdapterPolicy); err != nil {
		t.Fatal(err)
	}
}

func TestValidateH9AdapterPolicyRejectsNonWebAdapter(t *testing.T) {
	policy := validH9AdapterPolicyForTest()
	policy.Adapters = append(policy.Adapters, H5AdapterDescriptor{ID: "codex-chatgpt", ProviderFamily: "codex", Transport: "codex-cli", AuthClass: "chatgpt-subscription", Interaction: "automated"})
	if err := validateH9AdapterPolicy(policy); err == nil {
		t.Fatal("expected non-web adapter rejection")
	}
}

func TestH9ReplayRunnerEvaluatesOnlyFrozenReplayCases(t *testing.T) {
	ds, err := LoadH9Replay(filepath.Join("..", "..", "..", "benchmarks", "h9"))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	runner := H9ReplayRunner{
		Evaluator: fakeEvalExecutor{evaluate: func(_ context.Context, req evalharness.ProblemRequest) (evalharness.ProblemResult, error) {
			calls++
			if len(req.Arms) != 6 {
				t.Fatalf("arms=%d", len(req.Arms))
			}
			result := scoredProblem(req.ProblemID)
			result.RiskPolicy = H9RiskPolicy
			return result, nil
		}},
		CollectAdapterSummary: func(context.Context, string, string) (H5AdapterSummary, error) {
			return H5AdapterSummary{SchemaVersion: H5AdapterSummarySchemaVersion, SuccessfulInvocations: H9ExpectedSuccessfulInvocations, EffectiveAdapterDiversity: 1, EffectiveProviderDiversity: 1, HumanBrokerInvocations: H9ExpectedSuccessfulInvocations, AttemptsByAdapter: map[string]int{}, SuccessesByAdapter: map[string]int{}, SuccessesByProvider: map[string]int{}, AvailabilityFailuresByAdapter: map[string]int{}, SuccessesBySlot: map[string]map[string]int{}}, nil
		},
	}
	result, err := runner.Run(context.Background(), H9ReplayRunRequest{Dataset: ds, RunsRoot: t.TempDir(), RunID: "h9-test"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != H9ReplayCaseCount {
		t.Fatalf("eval calls=%d", calls)
	}
	if result.Summary.ProblemCount != H9ReplayCaseCount {
		t.Fatalf("problem count=%d", result.Summary.ProblemCount)
	}
}

func validH9AdapterPolicyForTest() H9AdapterPolicy {
	return H9AdapterPolicy{SchemaVersion: H9AdapterPolicySchemaVersion, Adapters: []H5AdapterDescriptor{{ID: H9HumanAdapterID, ProviderFamily: "chatgpt", Transport: "human-chatgpt-session", AuthClass: "chatgpt-subscription", Interaction: "human-broker"}}, Slots: map[string][]string{"eval-judge-1": {H9HumanAdapterID}, "eval-judge-2": {H9HumanAdapterID}}}
}
