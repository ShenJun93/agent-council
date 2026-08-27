package evalharness

import "testing"

func TestDecodeStrictJudgeJSONAcceptsFencedOutput(t *testing.T) {
	var got JudgeArtifact
	raw := "```json\n{\"overall_score\":90,\"confidence\":0.8}\n```"
	if err := decodeStrictJudgeJSON(raw, &got); err != nil {
		t.Fatalf("decodeStrictJudgeJSON() error = %v", err)
	}
	if got.OverallScore != 90 {
		t.Fatalf("overall_score = %v, want 90", got.OverallScore)
	}
}
