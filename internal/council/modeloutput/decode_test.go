package modeloutput

import (
	"strings"
	"testing"
)

type sample struct {
	Value string `json:"value"`
}

func TestDecodeStrictAcceptedTransportForms(t *testing.T) {
	t.Parallel()
	cases := []string{
		`{"value":"x"}`,
		"```json\n{\"value\":\"x\"}\n```",
		"```\n{\"value\":\"x\"}\n```",
		" \n```json\n{\"value\":\"x\"}\n```\n ",
	}
	for _, raw := range cases {
		raw := raw
		t.Run(strings.ReplaceAll(raw, "\n", "\\n"), func(t *testing.T) {
			var got sample
			if err := DecodeStrict(raw, &got); err != nil {
				t.Fatalf("DecodeStrict() error = %v", err)
			}
			if got.Value != "x" {
				t.Fatalf("value = %q, want x", got.Value)
			}
		})
	}
}

func TestNormalizeRejectsInvalidTransportForms(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"prose prefix":        "answer:\n{\"value\":\"x\"}",
		"prose suffix":        "{\"value\":\"x\"}\ndone",
		"unsupported tag":     "```javascript\n{\"value\":\"x\"}\n```",
		"nested fence":        "```json\n{\"value\":\"```\"}\n```",
		"multiple fences":     "```json\n{\"value\":\"x\"}\n```\n```json\n{\"value\":\"y\"}\n```",
		"multiple values":     "{\"value\":\"x\"}\n{\"value\":\"y\"}",
		"array top level":     `["x"]`,
		"null top level":      `null`,
		"string top level":    `"x"`,
		"malformed json":      `{"value":`,
		"fence trailing text": "```json\n{\"value\":\"x\"}\n```\ntrailing",
	}
	for name, raw := range cases {
		raw := raw
		t.Run(name, func(t *testing.T) {
			if _, err := Normalize(raw); err == nil {
				t.Fatalf("Normalize(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

func TestDecodeStrictRejectsUnknownField(t *testing.T) {
	t.Parallel()
	var got sample
	if err := DecodeStrict(`{"value":"x","extra":true}`, &got); err == nil {
		t.Fatal("DecodeStrict() unexpectedly accepted unknown field")
	}
}

func TestDecodeStrictRejectsMultipleJSONValuesInsideFence(t *testing.T) {
	t.Parallel()
	var got sample
	raw := "```json\n{\"value\":\"x\"}\n{\"value\":\"y\"}\n```"
	if err := DecodeStrict(raw, &got); err == nil {
		t.Fatal("DecodeStrict() unexpectedly accepted multiple JSON values")
	}
}
