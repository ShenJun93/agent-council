package benchmark

import (
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

const (
	H3BenchmarkID          = "h3"
	H3DatasetSchemaVersion = "council.h3-dataset.v0"
	H3CasesSchemaVersion   = "council.h3-cases.v0"
	H3RubricSchemaVersion  = "council.h3-rubric.v0"
	H3RunSchemaVersion     = "council.h3-run.v0"
	H3ResultSchemaVersion  = "council.h3-result.v0"
)

var H3RiskPolicy = evalharness.RiskPolicy{
	Comparator:         evalharness.ComparatorBestSingle,
	MaterialWorseDelta: 10.0,
}

var H3ChallengePolicy = protocol.ChallengePolicy{
	AllowAbbreviated:        false,
	HighConfidenceThreshold: 1.0,
}
