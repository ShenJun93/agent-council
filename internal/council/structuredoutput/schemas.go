package structuredoutput

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

var evidenceRefSchema = compactSchema("evidence-ref", `{
  "type":"object",
  "properties":{
    "artifact_id":{"type":"string"},
    "locator":{"type":"string"},
    "claim":{"type":"string"}
  },
  "required":["artifact_id","locator","claim"],
  "additionalProperties":false
}`)

var citationCheckSchema = compactSchema("citation-check", `{
  "type":"object",
  "properties":{
    "reference":{"type":"string"},
    "status":{"type":"string"},
    "note":{"type":"string"}
  },
  "required":["reference","status","note"],
  "additionalProperties":false
}`)

var answerSchema = compactSchema("answer", `{
  "type":"object",
  "properties":{
    "decision":{"type":"string"},
    "action":{"type":"string"},
    "reasons":{"type":"array","items":{"type":"string"}},
    "assumptions":{"type":"array","items":{"type":"string"}},
    "risks":{"type":"array","items":{"type":"string"}},
    "citations":{"type":"array","items":`+string(evidenceRefSchema)+`},
    "confidence":{"type":"number"}
  },
  "required":["decision","action","reasons","assumptions","risks","citations","confidence"],
  "additionalProperties":false
}`)

var researchSchema = compactSchema("research", `{
  "type":"object",
  "properties":{
    "recommendation":{"type":"string"},
    "reasoning":{"type":"array","items":{"type":"string"}},
    "considerations":{"type":"array","items":{"type":"string"}},
    "assumptions":{"type":"array","items":{"type":"string"}},
    "risks":{"type":"array","items":{"type":"string"}},
    "evidence_needed":{"type":"array","items":{"type":"string"}},
    "citations":{"type":"array","items":`+string(evidenceRefSchema)+`},
    "confidence":{"type":"number"}
  },
  "required":["recommendation","reasoning","considerations","assumptions","risks","evidence_needed","citations","confidence"],
  "additionalProperties":false
}`)

var reviewSchema = compactSchema("review", `{
  "type":"object",
  "properties":{
    "strengths":{"type":"array","items":{"type":"string"}},
    "weaknesses":{"type":"array","items":{"type":"string"}},
    "unsupported":{"type":"array","items":{"type":"string"}},
    "missing":{"type":"array","items":{"type":"string"}},
    "incorrect_assumptions":{"type":"array","items":{"type":"string"}},
    "critical_risks":{"type":"array","items":{"type":"string"}},
    "recommended_changes":{"type":"array","items":{"type":"string"}},
    "confidence":{"type":"number"}
  },
  "required":["strengths","weaknesses","unsupported","missing","incorrect_assumptions","critical_risks","recommended_changes","confidence"],
  "additionalProperties":false
}`)

var challengeSchema = compactSchema("challenge", `{
  "type":"object",
  "properties":{
    "attacks":{"type":"array","items":{"type":"string"}},
    "falsifiers":{"type":"array","items":{"type":"string"}},
    "evidence_gaps":{"type":"array","items":{"type":"string"}},
    "confidence":{"type":"number"}
  },
  "required":["attacks","falsifiers","evidence_gaps","confidence"],
  "additionalProperties":false
}`)

var rebuttalSchema = compactSchema("rebuttal", `{
  "type":"object",
  "properties":{
    "accepted_criticisms":{"type":"array","items":{"type":"string"}},
    "rejected_criticisms":{"type":"array","items":{"type":"string"}},
    "changed_position":{"type":"boolean"},
    "position_correct_before_flip":{"type":"boolean"},
    "updated_recommendation":{"type":"string"},
    "updated_confidence":{"type":"number"},
    "reasons":{"type":"array","items":{"type":"string"}}
  },
  "required":["accepted_criticisms","rejected_criticisms","changed_position","position_correct_before_flip","updated_recommendation","updated_confidence","reasons"],
  "additionalProperties":false
}`)

var protocolJudgeSchema = compactSchema("protocol-judge", `{
  "type":"object",
  "properties":{
    "decision":{"type":"string"},
    "confidence":{"type":"number"},
    "action":{"type":"string"},
    "reasons":{"type":"array","items":{"type":"string"}},
    "evidence":{"type":"array","items":{"type":"string"}},
    "rejected_alternatives":{"type":"array","items":{"type":"string"}},
    "minority":{"type":"array","items":{"type":"string"}},
    "unresolved":{"type":"array","items":{"type":"string"}},
    "assumptions":{"type":"array","items":{"type":"string"}},
    "change_conditions":{"type":"array","items":{"type":"string"}},
    "next_validation":{"type":"array","items":{"type":"string"}},
    "citation_checks":{"type":"array","items":`+string(citationCheckSchema)+`}
  },
  "required":["decision","confidence","action","reasons","evidence","rejected_alternatives","minority","unresolved","assumptions","change_conditions","next_validation","citation_checks"],
  "additionalProperties":false
}`)

var evalJudgeSchema = compactSchema("eval-judge", `{
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
    "citation_checks":{"type":"array","items":`+string(citationCheckSchema)+`},
    "relied_on_citations":{"type":"array","items":{"type":"string"}},
    "critical_errors":{"type":"array","items":{"type":"string"}},
    "strengths":{"type":"array","items":{"type":"string"}},
    "weaknesses":{"type":"array","items":{"type":"string"}},
    "confidence":{"type":"number"}
  },
  "required":["overall_score","dimensions","citation_checks","relied_on_citations","critical_errors","strengths","weaknesses","confidence"],
  "additionalProperties":false
}`)

func SchemaFor(role, phase string) (json.RawMessage, error) {
	var schema json.RawMessage
	switch {
	case role == "baseline" && (phase == "baseline-draft" || phase == "baseline-final"):
		schema = answerSchema
	case role == "researcher" && phase == protocol.PhaseResearch:
		schema = researchSchema
	case role == "reviewer" && phase == protocol.PhaseReview:
		schema = reviewSchema
	case role == "challenger" && phase == protocol.PhaseChallenge:
		schema = challengeSchema
	case role == "researcher" && phase == protocol.PhaseRebuttal:
		schema = rebuttalSchema
	case role == "judge" && phase == protocol.PhaseJudge:
		schema = protocolJudgeSchema
	case role == "judge" && phase == evalharness.PhaseEvalJudge:
		schema = evalJudgeSchema
	default:
		return nil, fmt.Errorf("no frozen H4 output schema for role=%q phase=%q", role, phase)
	}
	return append(json.RawMessage(nil), schema...), nil
}

func compactSchema(name, raw string) json.RawMessage {
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(raw)); err != nil {
		panic(fmt.Sprintf("invalid frozen %s schema: %v", name, err))
	}
	return json.RawMessage(compact.Bytes())
}
