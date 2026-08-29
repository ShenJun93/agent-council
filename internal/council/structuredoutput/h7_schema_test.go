package structuredoutput

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func TestH7EvalJudgeSchemaUsesThreeFieldCitationTuples(t *testing.T) {
	schema, err := SchemaForProfile("judge", evalharness.PhaseEvalJudge, SchemaProfileH7)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"artifact_id", "claim", "locator"}

	checks := propertyShape(t, schema, "citation_checks")
	check := decodeSchemaShape(t, checks.Items)
	reference := decodeSchemaShape(t, check.Properties["reference"])
	if reference.Type != "object" {
		t.Fatalf("reference type=%q want object", reference.Type)
	}
	if got := sortedKeys(reference.Properties); !equalStrings(got, want) {
		t.Fatalf("reference properties=%v want %v", got, want)
	}
	if got := sortedCopy(reference.Required); !equalStrings(got, want) {
		t.Fatalf("reference required=%v want %v", got, want)
	}
	if string(reference.AdditionalProperties) != "false" {
		t.Fatalf("reference additionalProperties=%s want false", reference.AdditionalProperties)
	}

	relied := propertyShape(t, schema, "relied_on_citations")
	item := decodeSchemaShape(t, relied.Items)
	if item.Type != "object" {
		t.Fatalf("relied item type=%q want object", item.Type)
	}
	if got := sortedKeys(item.Properties); !equalStrings(got, want) {
		t.Fatalf("relied properties=%v want %v", got, want)
	}
	if got := sortedCopy(item.Required); !equalStrings(got, want) {
		t.Fatalf("relied required=%v want %v", got, want)
	}
	if string(item.AdditionalProperties) != "false" {
		t.Fatalf("relied additionalProperties=%s want false", item.AdditionalProperties)
	}
}

func TestH7ProfileLeavesH6EvalSchemaByteIdentical(t *testing.T) {
	h6Eval, err := SchemaForProfile("judge", evalharness.PhaseEvalJudge, SchemaProfileH6)
	if err != nil {
		t.Fatal(err)
	}
	h7Eval, err := SchemaForProfile("judge", evalharness.PhaseEvalJudge, SchemaProfileH7)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(h6Eval, h7Eval) {
		t.Fatal("H7 eval schema must differ from H6 eval schema")
	}

	h6EvalAgain, err := SchemaForProfile("judge", evalharness.PhaseEvalJudge, SchemaProfileH6)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(h6Eval, h6EvalAgain) {
		t.Fatal("H6 eval schema bytes changed after H7 was introduced")
	}
}

func TestH7NonEvalSchemasAreByteIdenticalToH6(t *testing.T) {
	cases := []struct {
		name  string
		role  string
		phase string
	}{
		{"baseline-draft", "baseline", "baseline-draft"},
		{"baseline-final", "baseline", "baseline-final"},
		{"research", "researcher", protocol.PhaseResearch},
		{"review", "reviewer", protocol.PhaseReview},
		{"challenge", "challenger", protocol.PhaseChallenge},
		{"rebuttal", "researcher", protocol.PhaseRebuttal},
		{"protocol-judge", "judge", protocol.PhaseJudge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h6Schema, err := SchemaForProfile(tc.role, tc.phase, SchemaProfileH6)
			if err != nil {
				t.Fatal(err)
			}
			h7Schema, err := SchemaForProfile(tc.role, tc.phase, SchemaProfileH7)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(h6Schema, h7Schema) {
				t.Fatalf("H7 %s/%s schema diverged from H6:\nh6=%s\nh7=%s", tc.role, tc.phase, h6Schema, h7Schema)
			}
		})
	}
}

func TestWrapProfileH7InjectsTypedEvalSchemaAndRejectsCallerSchema(t *testing.T) {
	inner := &captureRuntime{resp: councilruntime.AgentResponse{Provider: councilruntime.ProviderCodex, Stdout: `{}`}}
	wrapped := WrapProfile(inner, SchemaProfileH7)
	_, err := wrapped.Run(context.Background(), councilruntime.AgentRequest{Role: "judge", Phase: evalharness.PhaseEvalJudge})
	if err != nil {
		t.Fatal(err)
	}
	want, err := SchemaForProfile("judge", evalharness.PhaseEvalJudge, SchemaProfileH7)
	if err != nil {
		t.Fatal(err)
	}
	if len(inner.reqs) != 1 || !bytes.Equal(inner.reqs[0].OutputSchema, want) {
		t.Fatalf("injected schema=%s want=%s", inner.reqs[0].OutputSchema, want)
	}

	inner2 := &captureRuntime{}
	_, err = WrapProfile(inner2, SchemaProfileH7).Run(context.Background(), councilruntime.AgentRequest{Role: "judge", Phase: evalharness.PhaseEvalJudge, OutputSchema: json.RawMessage(`{"type":"object"}`)})
	if err == nil || len(inner2.reqs) != 0 {
		t.Fatalf("caller schema injection err=%v calls=%d", err, len(inner2.reqs))
	}
}
