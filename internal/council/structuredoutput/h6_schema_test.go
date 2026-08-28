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

func TestH6EvalJudgeSchemaUsesTypedCitationKeys(t *testing.T) {
	schema, err := SchemaForProfile("judge", evalharness.PhaseEvalJudge, SchemaProfileH6)
	if err != nil {
		t.Fatal(err)
	}
	checks := propertyShape(t, schema, "citation_checks")
	check := decodeSchemaShape(t, checks.Items)
	reference := decodeSchemaShape(t, check.Properties["reference"])
	if reference.Type != "object" {
		t.Fatalf("reference type=%q want object", reference.Type)
	}
	if got := sortedKeys(reference.Properties); !equalStrings(got, []string{"artifact_id", "locator"}) {
		t.Fatalf("reference properties=%v", got)
	}
	if got := sortedCopy(reference.Required); !equalStrings(got, []string{"artifact_id", "locator"}) {
		t.Fatalf("reference required=%v", got)
	}
	if string(reference.AdditionalProperties) != "false" {
		t.Fatalf("reference additionalProperties=%s", reference.AdditionalProperties)
	}
	relied := propertyShape(t, schema, "relied_on_citations")
	item := decodeSchemaShape(t, relied.Items)
	if item.Type != "object" {
		t.Fatalf("relied item type=%q want object", item.Type)
	}
}
func TestH6ProfileLeavesLegacyAndNonEvalSchemasByteIdentical(t *testing.T) {
	legacyEval, err := SchemaFor("judge", evalharness.PhaseEvalJudge)
	if err != nil {
		t.Fatal(err)
	}
	profileLegacy, err := SchemaForProfile("judge", evalharness.PhaseEvalJudge, SchemaProfileLegacy)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacyEval, profileLegacy) {
		t.Fatal("legacy eval schema changed under profile API")
	}
	legacyRelied := propertyShape(t, legacyEval, "relied_on_citations")
	if got := decodeSchemaShape(t, legacyRelied.Items).Type; got != "string" {
		t.Fatalf("legacy relied item type=%q want string", got)
	}

	legacyResearch, err := SchemaFor("researcher", protocol.PhaseResearch)
	if err != nil {
		t.Fatal(err)
	}
	h6Research, err := SchemaForProfile("researcher", protocol.PhaseResearch, SchemaProfileH6)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacyResearch, h6Research) {
		t.Fatal("H6 changed non-eval schema bytes")
	}
}

func TestWrapProfileH6InjectsTypedEvalSchemaAndRejectsCallerSchema(t *testing.T) {
	inner := &captureRuntime{resp: councilruntime.AgentResponse{Provider: councilruntime.ProviderCodex, Stdout: `{}`}}
	wrapped := WrapProfile(inner, SchemaProfileH6)
	_, err := wrapped.Run(context.Background(), councilruntime.AgentRequest{Role: "judge", Phase: evalharness.PhaseEvalJudge})
	if err != nil {
		t.Fatal(err)
	}
	want, err := SchemaForProfile("judge", evalharness.PhaseEvalJudge, SchemaProfileH6)
	if err != nil {
		t.Fatal(err)
	}
	if len(inner.reqs) != 1 || !bytes.Equal(inner.reqs[0].OutputSchema, want) {
		t.Fatalf("injected schema=%s want=%s", inner.reqs[0].OutputSchema, want)
	}

	inner2 := &captureRuntime{}
	_, err = WrapProfile(inner2, SchemaProfileH6).Run(context.Background(), councilruntime.AgentRequest{Role: "judge", Phase: evalharness.PhaseEvalJudge, OutputSchema: json.RawMessage(`{"type":"object"}`)})
	if err == nil || len(inner2.reqs) != 0 {
		t.Fatalf("caller schema injection err=%v calls=%d", err, len(inner2.reqs))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
