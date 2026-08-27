package structuredoutput

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/invocationlog"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type captureRuntime struct {
	reqs []councilruntime.AgentRequest
	resp councilruntime.AgentResponse
	err  error
}

func (r *captureRuntime) Run(_ context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	r.reqs = append(r.reqs, req)
	return r.resp, r.err
}

func TestWrapInjectsSchemaAndExtractsClaudeStructuredOutput(t *testing.T) {
	inner := &captureRuntime{resp: councilruntime.AgentResponse{
		Provider: councilruntime.ProviderClaude,
		Stdout:   `{"type":"result","structured_output":{"decision":"ship"},"result":"ignored"}`,
	}}
	wrapped := Wrap(inner)

	resp, err := wrapped.Run(context.Background(), councilruntime.AgentRequest{
		Role:  "baseline",
		Phase: "baseline-draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(inner.reqs) != 1 || len(inner.reqs[0].OutputSchema) == 0 {
		t.Fatalf("inner requests=%d schema=%q", len(inner.reqs), inner.reqs[0].OutputSchema)
	}
	wantSchema, err := SchemaFor("baseline", "baseline-draft")
	if err != nil {
		t.Fatal(err)
	}
	if string(inner.reqs[0].OutputSchema) != string(wantSchema) {
		t.Fatalf("schema=%s want %s", inner.reqs[0].OutputSchema, wantSchema)
	}
	if resp.Stdout != `{"decision":"ship"}` {
		t.Fatalf("stdout=%q", resp.Stdout)
	}
}

func TestWrapLeavesCodexStructuredOutputAsPayload(t *testing.T) {
	inner := &captureRuntime{resp: councilruntime.AgentResponse{
		Provider: councilruntime.ProviderCodex,
		Stdout:   `{"decision":"ship"}`,
	}}
	wrapped := Wrap(inner)
	resp, err := wrapped.Run(context.Background(), councilruntime.AgentRequest{
		Role: "researcher", Phase: "research",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Stdout != `{"decision":"ship"}` {
		t.Fatalf("stdout=%q", resp.Stdout)
	}
	if len(inner.reqs) != 1 || len(inner.reqs[0].OutputSchema) == 0 {
		t.Fatal("schema was not injected")
	}
}

func TestWrapRejectsUnknownRolePhaseBeforeInnerRuntime(t *testing.T) {
	inner := &captureRuntime{}
	_, err := Wrap(inner).Run(context.Background(), councilruntime.AgentRequest{
		Role: "researcher", Phase: "unknown",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(inner.reqs) != 0 {
		t.Fatalf("inner runtime called %d times", len(inner.reqs))
	}
}

func TestWrapRejectsPrepopulatedSchema(t *testing.T) {
	inner := &captureRuntime{}
	_, err := Wrap(inner).Run(context.Background(), councilruntime.AgentRequest{
		Role: "baseline", Phase: "baseline-final",
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(inner.reqs) != 0 {
		t.Fatalf("inner runtime called %d times", len(inner.reqs))
	}
}

func TestWrapRejectsMalformedClaudeStructuredOutputEnvelope(t *testing.T) {
	inner := &captureRuntime{resp: councilruntime.AgentResponse{
		Provider: councilruntime.ProviderClaude,
		Stdout:   `{"type":"result","result":"missing structured output"}`,
	}}
	resp, err := Wrap(inner).Run(context.Background(), councilruntime.AgentRequest{
		Role: "judge", Phase: "judge",
	})
	if err == nil {
		t.Fatal("expected malformed output error")
	}
	var runErr *councilruntime.RunError
	if !errors.As(err, &runErr) || runErr.Class != councilruntime.FailureMalformedOutput {
		t.Fatalf("error=%v", err)
	}
	if resp.Stdout != inner.resp.Stdout {
		t.Fatalf("raw stdout changed on envelope failure: %q", resp.Stdout)
	}
}

func TestWrapLogsRawClaudeEnvelopeBeforeExtraction(t *testing.T) {
	root := t.TempDir()
	raw := `{"type":"result","structured_output":{"decision":"ship"},"result":"ignored"}`
	inner := &captureRuntime{resp: councilruntime.AgentResponse{
		Provider:   councilruntime.ProviderClaude,
		Stdout:     raw,
		ExitCode:   0,
		Attempts:   1,
		StartedAt:  time.Unix(10, 0).UTC(),
		FinishedAt: time.Unix(11, 0).UTC(),
	}}
	wrapped := Wrap(invocationlog.Wrap(inner, councilruntime.ProviderClaude))
	resp, err := wrapped.Run(context.Background(), councilruntime.AgentRequest{
		RunID: "h4-run", RunRoot: root, Participant: "baseline-a",
		Role: "baseline", Phase: "baseline-draft", Prompt: "prompt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Stdout != `{"decision":"ship"}` {
		t.Fatalf("returned stdout=%q", resp.Stdout)
	}
	path := filepath.Join(root, "invocations", "claude", "baseline-a", "baseline-draft", "000001.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Stdout             string `json:"stdout"`
		OutputSchemaSHA256 string `json:"output_schema_sha256"`
	}
	if err := json.Unmarshal(data, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Stdout != raw {
		t.Fatalf("logged stdout=%q want raw envelope %q", evidence.Stdout, raw)
	}
	if evidence.OutputSchemaSHA256 == "" {
		t.Fatal("missing output schema digest")
	}
}
