package structuredoutput

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type Runtime struct {
	Inner councilruntime.AgentRuntime
}

func Wrap(inner councilruntime.AgentRuntime) councilruntime.AgentRuntime {
	return &Runtime{Inner: inner}
}

func (r *Runtime) Run(ctx context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	if r == nil || r.Inner == nil {
		return councilruntime.AgentResponse{}, isolation(errors.New("inner runtime is required"))
	}
	if len(bytes.TrimSpace(req.OutputSchema)) != 0 {
		return councilruntime.AgentResponse{}, isolation(errors.New("output schema must be injected by H4 runtime"))
	}
	schema, err := SchemaFor(req.Role, req.Phase)
	if err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	req.OutputSchema = schema

	resp, runErr := r.Inner.Run(ctx, req)
	if runErr != nil {
		return resp, runErr
	}
	switch resp.Provider {
	case councilruntime.ProviderCodex, councilruntime.ProviderChatGPT:
		return resp, nil
	case councilruntime.ProviderAntigravity:
		payload, err := extractAntigravityStructuredOutput(resp.Stdout)
		if err != nil {
			return resp, malformed(err)
		}
		resp.Stdout = payload
		return resp, nil
	case councilruntime.ProviderClaude:
		payload, err := extractClaudeStructuredOutput(resp.Stdout)
		if err != nil {
			return resp, malformed(err)
		}
		resp.Stdout = payload
		return resp, nil
	default:
		return resp, isolation(fmt.Errorf("unsupported provider %q", resp.Provider))
	}
}

func extractAntigravityStructuredOutput(stdout string) (string, error) {
	var envelope struct {
		Status           string          `json:"status"`
		Error            string          `json:"error"`
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		return "", fmt.Errorf("decode Antigravity structured output envelope: %w", err)
	}
	if envelope.Status != "SUCCESS" {
		return "", fmt.Errorf("antigravity structured output status %q: %s", envelope.Status, envelope.Error)
	}
	trimmed := bytes.TrimSpace(envelope.StructuredOutput)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", errors.New("antigravity structured output envelope missing structured_output")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil || object == nil {
		return "", errors.New("antigravity structured_output must be one JSON object")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return "", fmt.Errorf("compact Antigravity structured_output: %w", err)
	}
	return compact.String(), nil
}

func extractClaudeStructuredOutput(stdout string) (string, error) {
	var envelope struct {
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		return "", fmt.Errorf("decode Claude structured output envelope: %w", err)
	}
	trimmed := bytes.TrimSpace(envelope.StructuredOutput)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", errors.New("claude structured output envelope missing structured_output")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil || object == nil {
		if err == nil {
			err = errors.New("structured_output must be one JSON object")
		}
		return "", fmt.Errorf("decode Claude structured_output: %w", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return "", fmt.Errorf("compact Claude structured_output: %w", err)
	}
	return compact.String(), nil
}

func isolation(err error) error {
	return &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: err}
}

func malformed(err error) error {
	return &councilruntime.RunError{Class: councilruntime.FailureMalformedOutput, Err: err}
}
