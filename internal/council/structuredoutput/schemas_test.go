package structuredoutput

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

type schemaShape struct {
	Type                 string                     `json:"type"`
	Properties           map[string]json.RawMessage `json:"properties"`
	Required             []string                   `json:"required"`
	AdditionalProperties json.RawMessage            `json:"additionalProperties"`
	Items                json.RawMessage            `json:"items"`
}

func decodeSchemaShape(t *testing.T, raw json.RawMessage) schemaShape {
	t.Helper()
	var got schemaShape
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode schema: %v\n%s", err, raw)
	}
	return got
}

func structJSONFields(t reflect.Type) []string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	fields := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			continue
		}
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func assertClosedStructSchema(t *testing.T, raw json.RawMessage, typ reflect.Type) {
	t.Helper()
	shape := decodeSchemaShape(t, raw)
	if shape.Type != "object" {
		t.Fatalf("schema type=%q want object", shape.Type)
	}
	want := structJSONFields(typ)
	if got := sortedKeys(shape.Properties); !reflect.DeepEqual(got, want) {
		t.Fatalf("properties=%v want %v", got, want)
	}
	if got := sortedCopy(shape.Required); !reflect.DeepEqual(got, want) {
		t.Fatalf("required=%v want %v", got, want)
	}
	if string(shape.AdditionalProperties) != "false" {
		t.Fatalf("additionalProperties=%s want false", shape.AdditionalProperties)
	}
}

func propertyShape(t *testing.T, raw json.RawMessage, name string) schemaShape {
	t.Helper()
	shape := decodeSchemaShape(t, raw)
	property, ok := shape.Properties[name]
	if !ok {
		t.Fatalf("missing property %q", name)
	}
	return decodeSchemaShape(t, property)
}
func TestFrozenSchemasMatchArtifactStructs(t *testing.T) {
	cases := []struct {
		name  string
		role  string
		phase string
		typ   reflect.Type
	}{
		{"baseline-draft", "baseline", "baseline-draft", reflect.TypeOf(baseline.AnswerArtifact{})},
		{"baseline-final", "baseline", "baseline-final", reflect.TypeOf(baseline.AnswerArtifact{})},
		{"research", "researcher", protocol.PhaseResearch, reflect.TypeOf(protocol.ResearchArtifact{})},
		{"review", "reviewer", protocol.PhaseReview, reflect.TypeOf(protocol.ReviewArtifact{})},
		{"challenge", "challenger", protocol.PhaseChallenge, reflect.TypeOf(protocol.ChallengeArtifact{})},
		{"rebuttal", "researcher", protocol.PhaseRebuttal, reflect.TypeOf(protocol.RebuttalArtifact{})},
		{"protocol-judge", "judge", protocol.PhaseJudge, reflect.TypeOf(protocol.JudgeArtifact{})},
		{"eval-judge", "judge", evalharness.PhaseEvalJudge, reflect.TypeOf(evalharness.JudgeArtifact{})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema, err := SchemaFor(tc.role, tc.phase)
			if err != nil {
				t.Fatal(err)
			}
			assertClosedStructSchema(t, schema, tc.typ)
			var compact bytes.Buffer
			if err := json.Compact(&compact, schema); err != nil || compact.String() != string(schema) {
				t.Fatalf("schema is not compact: err=%v schema=%q", err, schema)
			}
		})
	}
}

func TestFrozenSchemasCloseNestedCitationObjects(t *testing.T) {
	for _, tc := range []struct {
		role, phase, property string
		typ                   reflect.Type
	}{
		{"baseline", "baseline-draft", "citations", reflect.TypeOf(protocol.EvidenceRef{})},
		{"researcher", protocol.PhaseResearch, "citations", reflect.TypeOf(protocol.EvidenceRef{})},
		{"judge", protocol.PhaseJudge, "citation_checks", reflect.TypeOf(protocol.CitationCheck{})},
		{"judge", evalharness.PhaseEvalJudge, "citation_checks", reflect.TypeOf(protocol.CitationCheck{})},
	} {
		schema, err := SchemaFor(tc.role, tc.phase)
		if err != nil {
			t.Fatal(err)
		}
		array := propertyShape(t, schema, tc.property)
		if array.Type != "array" {
			t.Fatalf("%s/%s %s type=%q want array", tc.role, tc.phase, tc.property, array.Type)
		}
		assertClosedStructSchema(t, array.Items, tc.typ)
	}
}

func TestEvalDimensionsSchemaUsesFrozenRubricIDs(t *testing.T) {
	schema, err := SchemaFor("judge", evalharness.PhaseEvalJudge)
	if err != nil {
		t.Fatal(err)
	}
	dimensions := propertyShape(t, schema, "dimensions")
	want := []string{"actionability", "calibration", "correctness_soundness", "evidence_use", "risk_handling"}
	if got := sortedKeys(dimensions.Properties); !reflect.DeepEqual(got, want) {
		t.Fatalf("dimension properties=%v want %v", got, want)
	}
	if got := sortedCopy(dimensions.Required); !reflect.DeepEqual(got, want) {
		t.Fatalf("dimension required=%v want %v", got, want)
	}
	if string(dimensions.AdditionalProperties) != "false" {
		t.Fatalf("dimension additionalProperties=%s want false", dimensions.AdditionalProperties)
	}
	for _, raw := range dimensions.Properties {
		if got := decodeSchemaShape(t, raw).Type; got != "number" {
			t.Fatalf("dimension type=%q want number", got)
		}
	}
}
