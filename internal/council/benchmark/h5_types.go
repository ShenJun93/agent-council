package benchmark

import (
	"fmt"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

const (
	H5BenchmarkID                = "h5"
	H5DatasetSchemaVersion       = "council.h5-dataset.v0"
	H5CasesSchemaVersion         = "council.h5-cases.v0"
	H5RubricSchemaVersion        = "council.h5-rubric.v0"
	H5AdapterPolicySchemaVersion = "council.h5-adapter-policy.v0"
	H5RunSchemaVersion           = "council.h5-run.v0"
	H5ResultSchemaVersion        = "council.h5-result.v0"
)

var H5RiskPolicy = evalharness.RiskPolicy{Comparator: evalharness.ComparatorBestSingle, MaterialWorseDelta: 10.0}
var H5ChallengePolicy = protocol.ChallengePolicy{AllowAbbreviated: false, HighConfidenceThreshold: 1.0}

type H5AdapterDescriptor struct {
	ID             string `json:"id"`
	ProviderFamily string `json:"provider_family"`
	Transport      string `json:"transport"`
	AuthClass      string `json:"auth_class"`
	Interaction    string `json:"interaction"`
	Model          string `json:"model,omitempty"`
}

type H5AdapterPolicy struct {
	SchemaVersion    string                `json:"schema_version"`
	Adapters         []H5AdapterDescriptor `json:"adapters"`
	Slots            map[string][]string   `json:"slots"`
	ChallengerByCase map[string][]string   `json:"challenger_by_case"`
}

type H5ResultManifest struct {
	SchemaVersion              string `json:"schema_version"`
	BenchmarkID                string `json:"benchmark_id"`
	RunID                      string `json:"run_id"`
	ProblemCount               int    `json:"problem_count"`
	BatchSummarySHA256         string `json:"batch_summary_sha256"`
	AdapterSummarySHA256       string `json:"adapter_summary_sha256"`
	EffectiveProviderDiversity int    `json:"effective_provider_diversity"`
	TotalAvailabilityFailovers int    `json:"total_availability_failovers"`
	HumanBrokerInvocations     int    `json:"human_broker_invocations"`
}

var h5RequiredSlots = []string{
	"baseline-a", "baseline-b", "researcher-1", "researcher-2", "reviewer-1", "reviewer-2",
	"judge-1", "judge-2", "eval-judge-1", "eval-judge-2",
}

func validateH5AdapterPolicy(policy H5AdapterPolicy) error {
	if policy.SchemaVersion != H5AdapterPolicySchemaVersion {
		return fmt.Errorf("adapter policy schema_version %q, want %q", policy.SchemaVersion, H5AdapterPolicySchemaVersion)
	}
	if len(policy.Adapters) == 0 {
		return fmt.Errorf("adapter policy must define at least one adapter")
	}
	known := make(map[string]struct{}, len(policy.Adapters))
	for _, adapter := range policy.Adapters {
		if !safeDatasetID(adapter.ID) {
			return fmt.Errorf("unsafe adapter id %q", adapter.ID)
		}
		if _, duplicate := known[adapter.ID]; duplicate {
			return fmt.Errorf("duplicate adapter id %q", adapter.ID)
		}
		known[adapter.ID] = struct{}{}
		switch adapter.ProviderFamily {
		case "claude", "codex", "chatgpt", "antigravity":
		default:
			return fmt.Errorf("adapter %q provider_family %q is unsupported", adapter.ID, adapter.ProviderFamily)
		}
		if adapter.ProviderFamily == "antigravity" && strings.TrimSpace(adapter.Model) == "" {
			return fmt.Errorf("adapter %q model is required for antigravity", adapter.ID)
		}
		if strings.TrimSpace(adapter.Transport) == "" || strings.TrimSpace(adapter.AuthClass) == "" {
			return fmt.Errorf("adapter %q transport and auth_class are required", adapter.ID)
		}
		switch adapter.Interaction {
		case "automated", "human-broker":
		default:
			return fmt.Errorf("adapter %q interaction %q is unsupported", adapter.ID, adapter.Interaction)
		}
	}
	if len(policy.Slots) != len(h5RequiredSlots) {
		return fmt.Errorf("adapter policy slots count %d, want %d", len(policy.Slots), len(h5RequiredSlots))
	}
	for _, slot := range h5RequiredSlots {
		chain, ok := policy.Slots[slot]
		if !ok {
			return fmt.Errorf("adapter policy missing slot %q", slot)
		}
		if err := validateH5AdapterChain(slot, chain, known); err != nil {
			return err
		}
	}
	if len(policy.ChallengerByCase) != len(h1CaseIDs) {
		return fmt.Errorf("challenger_by_case count %d, want %d", len(policy.ChallengerByCase), len(h1CaseIDs))
	}
	for _, id := range h1CaseIDs {
		chain, ok := policy.ChallengerByCase[id]
		if !ok {
			return fmt.Errorf("challenger_by_case missing %q", id)
		}
		if err := validateH5AdapterChain("challenger:"+id, chain, known); err != nil {
			return err
		}
	}
	return nil
}

func validateH5AdapterChain(label string, chain []string, known map[string]struct{}) error {
	if len(chain) == 0 {
		return fmt.Errorf("adapter chain %q is empty", label)
	}
	seen := make(map[string]struct{}, len(chain))
	for _, id := range chain {
		if _, ok := known[id]; !ok {
			return fmt.Errorf("adapter chain %q references unknown adapter %q", label, id)
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("adapter chain %q repeats adapter %q", label, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}
