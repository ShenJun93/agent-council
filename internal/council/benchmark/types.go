package benchmark

import (
	"encoding/json"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

const (
	H1BenchmarkID          = "h1"
	H1DatasetSchemaVersion = "council.h1-dataset.v0"
	H1CasesSchemaVersion   = "council.h1-cases.v0"
	H1RubricSchemaVersion  = "council.h1-rubric.v0"
	H1CaseCount            = 20
	H1TechnicalCount       = 10
	H1ProductCount         = 10
)

var H1RiskPolicy = evalharness.RiskPolicy{
	Comparator:         evalharness.ComparatorBestSingle,
	MaterialWorseDelta: 10.0,
}

var H1ChallengePolicy = protocol.ChallengePolicy{
	AllowAbbreviated:        false,
	HighConfidenceThreshold: 1.0,
}

type Manifest struct {
	SchemaVersion       string                 `json:"schema_version"`
	BenchmarkID         string                 `json:"benchmark_id"`
	CaseCount           int                    `json:"case_count"`
	CategoryCounts      map[string]int         `json:"category_counts"`
	CaseIDs             []string               `json:"case_ids"`
	RubricSHA256        string                 `json:"rubric_sha256"`
	CasesSHA256         string                 `json:"cases_sha256"`
	AdapterPolicySHA256 string                 `json:"adapter_policy_sha256,omitempty"`
	Comparator          evalharness.Comparator `json:"comparator"`
	MaterialWorseDelta  float64                `json:"material_worse_delta"`
}

type Case struct {
	ID                 string
	Category           string
	ChallengerProvider councilruntime.Provider
	Problem            json.RawMessage
	ProblemSHA256      string
	ReferenceSet       json.RawMessage
	ReferenceSetSHA256 string
}

type Dataset struct {
	Root                string
	Manifest            Manifest
	ManifestBytes       []byte
	Rubric              json.RawMessage
	RubricSHA256        string
	CasesBytes          []byte
	Cases               []Case
	AdapterPolicyBytes  []byte
	AdapterPolicySHA256 string
	AdapterPolicy       *H5AdapterPolicy
}
