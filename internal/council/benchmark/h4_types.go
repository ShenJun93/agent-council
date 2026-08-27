package benchmark

import (
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

const (
	H4BenchmarkID          = "h4"
	H4DatasetSchemaVersion = "council.h4-dataset.v0"
	H4CasesSchemaVersion   = "council.h4-cases.v0"
	H4RubricSchemaVersion  = "council.h4-rubric.v0"
	H4RunSchemaVersion     = "council.h4-run.v0"
	H4ResultSchemaVersion  = "council.h4-result.v0"
)

var H4RiskPolicy = evalharness.RiskPolicy{
	Comparator:         evalharness.ComparatorBestSingle,
	MaterialWorseDelta: 10.0,
}

var H4ChallengePolicy = protocol.ChallengePolicy{
	AllowAbbreviated:        false,
	HighConfidenceThreshold: 1.0,
}
