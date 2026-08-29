package structuredoutput

import (
	"encoding/json"
	"fmt"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

type SchemaProfile uint8

const (
	SchemaProfileLegacy SchemaProfile = iota
	SchemaProfileH6
)

var citationKeySchema = compactSchema("h6-citation-key", `{
  "type":"object",
  "properties":{
    "artifact_id":{"type":"string"},
    "locator":{"type":"string"}
  },
  "required":["artifact_id","locator"],
  "additionalProperties":false
}`)
var h6CitationCheckSchema = compactSchema("h6-citation-check", `{
  "type":"object",
  "properties":{
    "reference":`+string(citationKeySchema)+`,
    "status":{"type":"string"},
    "note":{"type":"string"}
  },
  "required":["reference","status","note"],
  "additionalProperties":false
}`)

var h6EvalJudgeSchema = compactSchema("h6-eval-judge", `{
  "type":"object",
  "properties":{
    "overall_score":{"type":"number"},
    "dimensions":{"type":"object","properties":{
      "correctness_soundness":{"type":"number"},
      "evidence_use":{"type":"number"},
      "risk_handling":{"type":"number"},
      "actionability":{"type":"number"},
      "calibration":{"type":"number"}
    },"required":["correctness_soundness","evidence_use","risk_handling","actionability","calibration"],"additionalProperties":false},
    "citation_checks":{"type":"array","items":`+string(h6CitationCheckSchema)+`},
    "relied_on_citations":{"type":"array","items":`+string(citationKeySchema)+`},
    "critical_errors":{"type":"array","items":{"type":"string"}},
    "strengths":{"type":"array","items":{"type":"string"}},
    "weaknesses":{"type":"array","items":{"type":"string"}},
    "confidence":{"type":"number"}
  },
  "required":["overall_score","dimensions","citation_checks","relied_on_citations","critical_errors","strengths","weaknesses","confidence"],
  "additionalProperties":false
}`)

func SchemaForProfile(role, phase string, profile SchemaProfile) (json.RawMessage, error) {
	switch profile {
	case SchemaProfileLegacy:
		return SchemaFor(role, phase)
	case SchemaProfileH6:
		if role == "judge" && phase == evalharness.PhaseEvalJudge {
			return append(json.RawMessage(nil), h6EvalJudgeSchema...), nil
		}
		return SchemaFor(role, phase)
	default:
		return nil, fmt.Errorf("unsupported structured-output schema profile %d", profile)
	}
}
