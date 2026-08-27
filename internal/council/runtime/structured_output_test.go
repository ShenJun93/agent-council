package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

var structuredTestSchema = json.RawMessage(`{
  "type": "object",
  "properties": {"answer": {"type": "string"}},
  "required": ["answer"],
  "additionalProperties": false
}`)

func argValue(args []string, flag string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}
func TestClaudeRuntimePassesInlineStructuredOutputSchema(t *testing.T) {
	runner := &fakeProcessRunner{results: []processResult{
		{Stdout: `{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty","subscriptionType":"pro"}`, ExitCode: 0},
		{Stdout: `{"answer":"ok"}`, ExitCode: 0},
	}}
	rt := newClaudeCLI("claude", runner, func() []string { return []string{"PATH=/bin", "HOME=/home/test"} })
	req := isolatedRequest(t, "structured")
	req.OutputSchema = structuredTestSchema

	if _, err := rt.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(runner.specs) != 2 {
		t.Fatalf("process calls=%d want 2", len(runner.specs))
	}
	got, ok := argValue(runner.specs[1].Args, "--json-schema")
	if !ok {
		t.Fatalf("Claude run args missing --json-schema: %#v", runner.specs[1].Args)
	}
	want := `{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`
	if got != want {
		t.Fatalf("schema=%q want %q", got, want)
	}
	format, ok := argValue(runner.specs[1].Args, "--output-format")
	if !ok || format != "json" {
		t.Fatalf("Claude structured output format=%q present=%v want json", format, ok)
	}
}

type schemaInspectRunner struct {
	t             *testing.T
	results       []processResult
	specs         []processSpec
	schemaPaths   []string
	schemaContent [][]byte
}

func (r *schemaInspectRunner) Run(_ context.Context, spec processSpec) processResult {
	r.specs = append(r.specs, spec)
	if path, ok := argValue(spec.Args, "--output-schema"); ok {
		data, err := os.ReadFile(path)
		if err != nil {
			r.t.Fatalf("read live output schema: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			r.t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			r.t.Fatalf("schema mode=%#o want 0600", info.Mode().Perm())
		}
		r.schemaPaths = append(r.schemaPaths, path)
		r.schemaContent = append(r.schemaContent, data)
	}
	if len(r.results) == 0 {
		return processResult{ExitCode: 0}
	}
	out := r.results[0]
	r.results = r.results[1:]
	return out
}
func TestCodexRuntimePassesTemporaryStructuredOutputSchemaAndCleansIt(t *testing.T) {
	runner := &schemaInspectRunner{t: t, results: []processResult{
		{Stdout: "Logged in using ChatGPT\n", ExitCode: 0},
		{Stdout: `{"answer":"ok"}`, ExitCode: 0},
	}}
	rt := newCodexCLI("codex", runner, func() []string { return codexFileAuthEnvironmentForTest(t) })
	req := isolatedRequest(t, "structured")
	req.OutputSchema = structuredTestSchema

	if _, err := rt.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(runner.schemaPaths) != 1 {
		t.Fatalf("schema uses=%d want 1", len(runner.schemaPaths))
	}
	want := []byte(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`)
	if !reflect.DeepEqual(runner.schemaContent[0], want) {
		t.Fatalf("schema=%s want %s", runner.schemaContent[0], want)
	}
	if _, err := os.Stat(runner.schemaPaths[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("schema temp file survived Run: %v", err)
	}
}
func TestRuntimeRejectsInvalidStructuredOutputSchemaBeforeSpawning(t *testing.T) {
	for _, schema := range []json.RawMessage{json.RawMessage(`[`), json.RawMessage(`[1]`)} {
		runner := &fakeProcessRunner{}
		rt := newClaudeCLI("claude", runner, func() []string { return []string{"PATH=/bin", "HOME=/home/test"} })
		req := isolatedRequest(t, "x")
		req.OutputSchema = schema
		if _, err := rt.Run(context.Background(), req); err == nil {
			t.Fatalf("Run accepted schema %q", schema)
		}
		if len(runner.specs) != 0 {
			t.Fatalf("schema %q spawned %d processes", schema, len(runner.specs))
		}
	}
}

func TestCodexRuntimeCleansStructuredOutputSchemaAfterProcessFailure(t *testing.T) {
	runner := &schemaInspectRunner{t: t, results: []processResult{
		{Stdout: "Logged in using ChatGPT\n", ExitCode: 0},
		{Stderr: "usage limit reached", ExitCode: 1, Err: errors.New("exit status 1")},
	}}
	rt := newCodexCLI("codex", runner, func() []string { return codexFileAuthEnvironmentForTest(t) })
	req := isolatedRequest(t, "structured")
	req.OutputSchema = structuredTestSchema
	if _, err := rt.Run(context.Background(), req); err == nil {
		t.Fatal("expected failure")
	}
	if len(runner.schemaPaths) != 1 {
		t.Fatalf("schema uses=%d want 1", len(runner.schemaPaths))
	}
	if _, err := os.Stat(runner.schemaPaths[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("schema temp file survived failed Run: %v", err)
	}
}
