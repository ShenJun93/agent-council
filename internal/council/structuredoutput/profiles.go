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
	SchemaProfileH7
	SchemaProfileH8
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

var citationOccurrenceKeySchema = compactSchema("h7-citation-occurrence-key", `{
  "type":"object",
  "properties":{
    "artifact_id":{"type":"string"},
    "locator":{"type":"string"},
    "claim":{"type":"string"}
  },
  "required":["artifact_id","locator","claim"],
  "additionalProperties":false
}`)
var h7CitationCheckSchema = compactSchema("h7-citation-check", `{
  "type":"object",
  "properties":{
    "reference":`+string(citationOccurrenceKeySchema)+`,
    "status":{"type":"string"},
    "note":{"type":"string"}
  },
  "required":["reference","status","note"],
  "additionalProperties":false
}`)

var h7EvalJudgeSchema = compactSchema("h7-eval-judge", `{
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
    "citation_checks":{"type":"array","items":`+string(h7CitationCheckSchema)+`},
    "relied_on_citations":{"type":"array","items":`+string(citationOccurrenceKeySchema)+`},
    "critical_errors":{"type":"array","items":{"type":"string"}},
    "strengths":{"type":"array","items":{"type":"string"}},
    "weaknesses":{"type":"array","items":{"type":"string"}},
    "confidence":{"type":"number"}
  },
  "required":["overall_score","dimensions","citation_checks","relied_on_citations","critical_errors","strengths","weaknesses","confidence"],
  "additionalProperties":false
}`)

var h8CitationCheckSchema = compactSchema("h8-citation-check", `{
  "type":"object",
  "properties":{
    "reference":`+string(citationOccurrenceKeySchema)+`,
    "status":{"type":"string","enum":["verified","unverified"]},
    "relied_on":{"type":"boolean"},
    "note":{"type":"string"}
  },
  "required":["reference","status","relied_on","note"],
  "additionalProperties":false
}`)

var h8EvalJudgeSchema = compactSchema("h8-eval-judge", `{
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
    "citation_checks":{"type":"array","items":`+string(h8CitationCheckSchema)+`},
    "critical_errors":{"type":"array","items":{"type":"string"}},
    "strengths":{"type":"array","items":{"type":"string"}},
    "weaknesses":{"type":"array","items":{"type":"string"}},
    "confidence":{"type":"number"}
  },
  "required":["overall_score","dimensions","citation_checks","critical_errors","strengths","weaknesses","confidence"],
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
	case SchemaProfileH7:
		if role == "judge" && phase == evalharness.PhaseEvalJudge {
			return append(json.RawMessage(nil), h7EvalJudgeSchema...), nil
		}
		return SchemaFor(role, phase)
	case SchemaProfileH8:
		if role == "judge" && phase == evalharness.PhaseEvalJudge {
			return append(json.RawMessage(nil), h8EvalJudgeSchema...), nil
		}
		return SchemaForProfile(role, phase, SchemaProfileH7)
	default:
		return nil, fmt.Errorf("unsupported structured-output schema profile %d", profile)
	}
}
