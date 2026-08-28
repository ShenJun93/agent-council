package runtime

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestAntigravityRuntimeUsesHeadlessStructuredOutputWithoutAuthProbe(t *testing.T) {
	runner := &fakeProcessRunner{results: []processResult{{Stdout: `{"status":"SUCCESS","structured_output":{"answer":"ok"}}`, ExitCode: 0}}}
	rt := newAntigravityCLI("agy", "gemini-3.1-pro-high", runner, func() []string { return []string{"PATH=/bin", "HOME=/home/test"} })
	req := isolatedRequest(t, "review independently")
	req.OutputSchema = structuredTestSchema

	resp, err := rt.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Provider != ProviderAntigravity {
		t.Fatalf("provider=%q", resp.Provider)
	}
	if len(runner.specs) != 1 {
		t.Fatalf("process calls=%d want 1", len(runner.specs))
	}

	wantSchema := `{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`
	want := []string{"--print", "review independently", "--output-format", "json", "--json-schema", wantSchema, "--model", "gemini-3.1-pro-high", "--mode", "plan", "--sandbox", "--disable-slash-commands"}
	if !reflect.DeepEqual(runner.specs[0].Args, want) {
		t.Fatalf("args=%#v want %#v", runner.specs[0].Args, want)
	}
}

func TestAntigravityRuntimeClassifiesAuthenticationRequired(t *testing.T) {
	runner := &fakeProcessRunner{results: []processResult{{Stderr: "authentication required", ExitCode: 1, Err: errors.New("exit status 1")}}}
	rt := newAntigravityCLI("agy", "gemini-3.1-pro-high", runner, func() []string { return []string{"PATH=/bin", "HOME=/home/test"} })
	_, err := rt.Run(context.Background(), isolatedRequest(t, "x"))
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureAuth {
		t.Fatalf("error=%v want auth", err)
	}
	if len(runner.specs) != 1 {
		t.Fatalf("process calls=%d want 1", len(runner.specs))
	}
}

func TestAntigravityRuntimeClassifiesQuotaWithoutSameAdapterRetry(t *testing.T) {
	runner := &fakeProcessRunner{results: []processResult{{Stderr: "quota exceeded", ExitCode: 1, Err: errors.New("exit status 1")}}}
	rt := newAntigravityCLI("agy", "gemini-3.1-pro-high", runner, func() []string { return []string{"PATH=/bin", "HOME=/home/test"} })
	req := isolatedRequest(t, "x")
	req.MaxAttempts = 1
	_, err := rt.Run(context.Background(), req)
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Class != FailureQuotaExhausted {
		t.Fatalf("error=%v want quota", err)
	}
	if len(runner.specs) != 1 {
		t.Fatalf("process calls=%d want 1", len(runner.specs))
	}
}
