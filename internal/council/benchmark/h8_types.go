package benchmark

import (
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

const (
	H8BenchmarkID          = "h8"
	H8DatasetSchemaVersion = "council.h8-dataset.v0"
	H8CasesSchemaVersion   = "council.h8-cases.v0"
	H8RubricSchemaVersion  = "council.h8-rubric.v0"
	H8RunSchemaVersion     = "council.h8-run.v0"
	H8ResultSchemaVersion  = "council.h8-result.v0"
)

var H8RiskPolicy = evalharness.RiskPolicy{Comparator: evalharness.ComparatorBestSingle, MaterialWorseDelta: 10.0}
var H8ChallengePolicy = protocol.ChallengePolicy{AllowAbbreviated: false, HighConfidenceThreshold: 1.0}

type H8ResultManifest struct {
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
