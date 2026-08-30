package structuredoutput

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

type enumSchemaShape struct {
	Type string   `json:"type"`
	Enum []string `json:"enum"`
}

func TestH8EvalJudgeSchemaCoLocatesVerificationAndReliance(t *testing.T) {
	schema, err := SchemaForProfile("judge", evalharness.PhaseEvalJudge, SchemaProfileH8)
	if err != nil {
		t.Fatal(err)
	}
	top := decodeSchemaShape(t, schema)
	if _, exists := top.Properties["relied_on_citations"]; exists {
		t.Fatal("H8 eval schema must not expose model-authored relied_on_citations")
	}
	checks := propertyShape(t, schema, "citation_checks")
	check := decodeSchemaShape(t, checks.Items)
	wantFields := []string{"note", "reference", "relied_on", "status"}
	if got := sortedKeys(check.Properties); !reflect.DeepEqual(got, wantFields) {
		t.Fatalf("citation check properties=%v want %v", got, wantFields)
	}
	if got := sortedCopy(check.Required); !reflect.DeepEqual(got, wantFields) {
		t.Fatalf("citation check required=%v want %v", got, wantFields)
	}
	if got := decodeSchemaShape(t, check.Properties["relied_on"]).Type; got != "boolean" {
		t.Fatalf("relied_on type=%q want boolean", got)
	}
	var status enumSchemaShape
	if err := json.Unmarshal(check.Properties["status"], &status); err != nil {
		t.Fatal(err)
	}
	if status.Type != "string" || !reflect.DeepEqual(status.Enum, []string{"verified", "unverified"}) {
		t.Fatalf("status schema=%+v", status)
	}
	reference := decodeSchemaShape(t, check.Properties["reference"])
	wantReference := []string{"artifact_id", "claim", "locator"}
	if got := sortedKeys(reference.Properties); !reflect.DeepEqual(got, wantReference) {
		t.Fatalf("reference properties=%v want %v", got, wantReference)
	}
}

func TestH8NonEvalSchemasAreByteIdenticalToH7(t *testing.T) {
	cases := []struct{ role, phase string }{
		{"baseline", "baseline-draft"},
		{"baseline", "baseline-final"},
		{"researcher", protocol.PhaseResearch},
		{"reviewer", protocol.PhaseReview},
		{"challenger", protocol.PhaseChallenge},
		{"researcher", protocol.PhaseRebuttal},
		{"judge", protocol.PhaseJudge},
	}
	for _, tc := range cases {
		h7, err := SchemaForProfile(tc.role, tc.phase, SchemaProfileH7)
		if err != nil {
			t.Fatal(err)
		}
		h8, err := SchemaForProfile(tc.role, tc.phase, SchemaProfileH8)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(h7, h8) {
			t.Fatalf("H8 non-eval schema diverged for %s/%s", tc.role, tc.phase)
		}
	}
}
