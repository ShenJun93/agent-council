package benchmark

import (
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

const (
	H2BenchmarkID          = "h2"
	H2DatasetSchemaVersion = "council.h2-dataset.v0"
	H2CasesSchemaVersion   = "council.h2-cases.v0"
	H2RubricSchemaVersion  = "council.h2-rubric.v0"
	H2RunSchemaVersion     = "council.h2-run.v0"
	H2ResultSchemaVersion  = "council.h2-result.v0"
)

var H2RiskPolicy = evalharness.RiskPolicy{
	Comparator:         evalharness.ComparatorBestSingle,
	MaterialWorseDelta: 10.0,
}

var H2ChallengePolicy = protocol.ChallengePolicy{
	AllowAbbreviated:        false,
	HighConfidenceThreshold: 1.0,
}
