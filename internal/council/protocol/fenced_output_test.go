package protocol

import "testing"

func TestDecodeStrictJSONAcceptsFencedOutput(t *testing.T) {
	var got struct {
		Decision string `json:"decision"`
	}
	raw := "```\n{\"decision\":\"ship\"}\n```"
	if err := decodeStrictJSON("research", raw, &got); err != nil {
		t.Fatalf("decodeStrictJSON() error = %v", err)
	}
	if got.Decision != "ship" {
		t.Fatalf("decision = %q, want ship", got.Decision)
	}
}
