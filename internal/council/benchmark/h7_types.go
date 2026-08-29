package benchmark

import (
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

const (
	H7BenchmarkID          = "h7"
	H7DatasetSchemaVersion = "council.h7-dataset.v0"
	H7CasesSchemaVersion   = "council.h7-cases.v0"
	H7RubricSchemaVersion  = "council.h7-rubric.v0"
	H7RunSchemaVersion     = "council.h7-run.v0"
	H7ResultSchemaVersion  = "council.h7-result.v0"
)

var H7RiskPolicy = evalharness.RiskPolicy{Comparator: evalharness.ComparatorBestSingle, MaterialWorseDelta: 10.0}
var H7ChallengePolicy = protocol.ChallengePolicy{AllowAbbreviated: false, HighConfidenceThreshold: 1.0}

type H7ResultManifest struct {
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
