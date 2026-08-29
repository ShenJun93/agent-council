package benchmark

import (
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

const (
	H6BenchmarkID          = "h6"
	H6DatasetSchemaVersion = "council.h6-dataset.v0"
	H6CasesSchemaVersion   = "council.h6-cases.v0"
	H6RubricSchemaVersion  = "council.h6-rubric.v0"
	H6RunSchemaVersion     = "council.h6-run.v0"
	H6ResultSchemaVersion  = "council.h6-result.v0"
)

var H6RiskPolicy = evalharness.RiskPolicy{Comparator: evalharness.ComparatorBestSingle, MaterialWorseDelta: 10.0}
var H6ChallengePolicy = protocol.ChallengePolicy{AllowAbbreviated: false, HighConfidenceThreshold: 1.0}

type H6ResultManifest struct {
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
