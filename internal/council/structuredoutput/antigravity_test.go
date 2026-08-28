package structuredoutput

import (
	"context"
	"errors"
	"testing"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func TestWrapExtractsAntigravityStructuredOutputEnvelope(t *testing.T) {
	raw := `{"conversation_id":"fresh","status":"SUCCESS","response":"{\"decision\":\"ship\"}\n","structured_output":{"decision":"ship"}}`
	inner := &captureRuntime{resp: councilruntime.AgentResponse{Provider: councilruntime.ProviderAntigravity, Stdout: raw}}
	resp, err := Wrap(inner).Run(context.Background(), councilruntime.AgentRequest{Role: "reviewer", Phase: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Stdout != `{"decision":"ship"}` {
		t.Fatalf("stdout=%q", resp.Stdout)
	}
}

func TestWrapRejectsNonSuccessAntigravityEnvelope(t *testing.T) {
	raw := `{"status":"ERROR","error":"quota exceeded","structured_output":null}`
	inner := &captureRuntime{resp: councilruntime.AgentResponse{Provider: councilruntime.ProviderAntigravity, Stdout: raw}}
	_, err := Wrap(inner).Run(context.Background(), councilruntime.AgentRequest{Role: "reviewer", Phase: "review"})
	var runErr *councilruntime.RunError
	if !errors.As(err, &runErr) || runErr.Class != councilruntime.FailureMalformedOutput {
		t.Fatalf("error=%v", err)
	}
}
