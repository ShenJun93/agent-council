package baseline

import "testing"

func TestDecodeStrictJSONAcceptsFencedOutput(t *testing.T) {
	var got AnswerArtifact
	raw := "```json\n{\"decision\":\"ship\"}\n```"
	if err := decodeStrictJSON(raw, &got); err != nil {
		t.Fatalf("decodeStrictJSON() error = %v", err)
	}
	if got.Decision != "ship" {
		t.Fatalf("decision = %q, want ship", got.Decision)
	}
}
