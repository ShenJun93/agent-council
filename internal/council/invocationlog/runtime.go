package invocationlog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/safestore"
)

const SchemaVersion = "council.invocation-evidence.v1"

type Evidence struct {
	SchemaVersion      string                  `json:"schema_version"`
	RunID              string                  `json:"run_id"`
	Provider           councilruntime.Provider `json:"provider"`
	Participant        string                  `json:"participant"`
	Role               string                  `json:"role"`
	Phase              string                  `json:"phase"`
	PromptSHA256       string                  `json:"prompt_sha256"`
	OutputSchemaSHA256 string                  `json:"output_schema_sha256,omitempty"`
	Stdout             string                  `json:"stdout"`
	Stderr             string                  `json:"stderr"`
	ExitCode           int                     `json:"exit_code"`
	Attempts           int                     `json:"attempts"`
	StartedAt          time.Time               `json:"started_at"`
	FinishedAt         time.Time               `json:"finished_at"`
}

type Runtime struct {
	Inner    councilruntime.AgentRuntime
	Provider councilruntime.Provider
	seq      atomic.Uint64
}

func Wrap(inner councilruntime.AgentRuntime, provider councilruntime.Provider) councilruntime.AgentRuntime {
	return &Runtime{Inner: inner, Provider: provider}
}

func (r *Runtime) Run(ctx context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	if r == nil || r.Inner == nil {
		return councilruntime.AgentResponse{}, isolation(errors.New("inner runtime is required"))
	}
	if err := validateComponent("provider", string(r.Provider)); err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	if err := validateComponent("participant", req.Participant); err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	if err := validateComponent("phase", req.Phase); err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	response, runErr := r.Inner.Run(ctx, req)
	if response.StartedAt.IsZero() && response.Attempts == 0 {
		return response, runErr
	}

	digest := sha256.Sum256([]byte(req.Prompt))
	outputSchemaDigest, digestErr := compactSchemaDigest(req.OutputSchema)
	if digestErr != nil {
		return response, errors.Join(runErr, isolation(fmt.Errorf("hash output schema: %w", digestErr)))
	}
	evidence := Evidence{
		SchemaVersion:      SchemaVersion,
		RunID:              req.RunID,
		Provider:           r.Provider,
		Participant:        req.Participant,
		Role:               req.Role,
		Phase:              req.Phase,
		PromptSHA256:       hex.EncodeToString(digest[:]),
		OutputSchemaSHA256: outputSchemaDigest,
		Stdout:             response.Stdout,
		Stderr:             response.Stderr,
		ExitCode:           response.ExitCode,
		Attempts:           response.Attempts,
		StartedAt:          response.StartedAt,
		FinishedAt:         response.FinishedAt,
	}
	data, err := json.Marshal(evidence)
	if err != nil {
		return response, errors.Join(runErr, isolation(fmt.Errorf("marshal invocation evidence: %w", err)))
	}
	sequence := r.seq.Add(1)
	rel := filepath.Join(
		"invocations",
		string(r.Provider),
		req.Participant,
		req.Phase,
		fmt.Sprintf("%06d.json", sequence),
	)
	if _, err := safestore.WriteExclusive(req.RunRoot, rel, data); err != nil {
		writeErr := isolation(fmt.Errorf("persist invocation evidence: %w", err))
		if runErr != nil {
			return response, errors.Join(writeErr, runErr)
		}
		return response, writeErr
	}
	return response, runErr
}

func validateComponent(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if value == "." || value == ".." || filepath.Base(value) != value || strings.ContainsAny(value, `/\\`) {
		return fmt.Errorf("unsafe %s %q", name, value)
	}
	return nil
}

func isolation(err error) error {
	return &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: err}
}

func compactSchemaDigest(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return "", err
	}
	digest := sha256.Sum256(compact.Bytes())
	return hex.EncodeToString(digest[:]), nil
}
