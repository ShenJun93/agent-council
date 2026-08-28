package invocationlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/safestore"
)

const AdapterSchemaVersion = "council.invocation-evidence.v2"

type AdapterMetadata struct {
	ID       string
	Provider councilruntime.Provider
}

type AdapterEvidence struct {
	SchemaVersion      string                      `json:"schema_version"`
	RunID              string                      `json:"run_id"`
	SlotID             string                      `json:"slot_id"`
	AdapterID          string                      `json:"adapter_id"`
	ProviderFamily     councilruntime.Provider     `json:"provider_family"`
	Participant        string                      `json:"participant"`
	Role               string                      `json:"role"`
	Phase              string                      `json:"phase"`
	FailoverIndex      int                         `json:"failover_index"`
	FailoverTrigger    councilruntime.FailureClass `json:"failover_trigger,omitempty"`
	FailureClass       councilruntime.FailureClass `json:"failure_class,omitempty"`
	PromptSHA256       string                      `json:"prompt_sha256"`
	OutputSchemaSHA256 string                      `json:"output_schema_sha256,omitempty"`
	Stdout             string                      `json:"stdout"`
	Stderr             string                      `json:"stderr"`
	ExitCode           int                         `json:"exit_code"`
	Attempts           int                         `json:"attempts"`
	StartedAt          time.Time                   `json:"started_at"`
	FinishedAt         time.Time                   `json:"finished_at"`
}

type AdapterRuntime struct {
	Inner    councilruntime.AgentRuntime
	Metadata AdapterMetadata
	seq      atomic.Uint64
}

func WrapAdapter(inner councilruntime.AgentRuntime, metadata AdapterMetadata) councilruntime.AgentRuntime {
	return &AdapterRuntime{Inner: inner, Metadata: metadata}
}

func (r *AdapterRuntime) Run(ctx context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	if r == nil || r.Inner == nil {
		return councilruntime.AgentResponse{}, isolation(errors.New("inner runtime is required"))
	}
	if err := validateComponent("adapter", r.Metadata.ID); err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	if err := validateComponent("provider family", string(r.Metadata.Provider)); err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	if req.AdapterID != r.Metadata.ID {
		return councilruntime.AgentResponse{}, isolation(fmt.Errorf("request adapter %q does not match wrapper %q", req.AdapterID, r.Metadata.ID))
	}
	if err := validateComponent("slot", req.SlotID); err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	if err := validateComponent("participant", req.Participant); err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	if err := validateComponent("phase", req.Phase); err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	if req.FailoverIndex < 0 {
		return councilruntime.AgentResponse{}, isolation(errors.New("failover index cannot be negative"))
	}

	wrapperStarted := time.Now().UTC()
	response, runErr := r.Inner.Run(ctx, req)
	wrapperFinished := time.Now().UTC()
	started := response.StartedAt
	if started.IsZero() {
		started = wrapperStarted
	}
	finished := response.FinishedAt
	if finished.IsZero() {
		finished = wrapperFinished
	}
	exitCode := response.ExitCode
	if response.StartedAt.IsZero() && response.Attempts == 0 && runErr != nil && exitCode == 0 {
		exitCode = -1
	}

	promptDigest := sha256.Sum256([]byte(req.Prompt))
	schemaDigest, err := compactSchemaDigest(req.OutputSchema)
	if err != nil {
		return response, errors.Join(runErr, isolation(fmt.Errorf("hash output schema: %w", err)))
	}
	evidence := AdapterEvidence{
		SchemaVersion: AdapterSchemaVersion, RunID: req.RunID, SlotID: req.SlotID,
		AdapterID: r.Metadata.ID, ProviderFamily: r.Metadata.Provider,
		Participant: req.Participant, Role: req.Role, Phase: req.Phase,
		FailoverIndex: req.FailoverIndex, FailoverTrigger: req.FailoverTrigger,
		FailureClass: firstFailureClass(runErr),
		PromptSHA256: hex.EncodeToString(promptDigest[:]), OutputSchemaSHA256: schemaDigest,
		Stdout: response.Stdout, Stderr: response.Stderr, ExitCode: exitCode, Attempts: response.Attempts,
		StartedAt: started, FinishedAt: finished,
	}
	data, err := json.Marshal(evidence)
	if err != nil {
		return response, errors.Join(runErr, isolation(fmt.Errorf("marshal adapter evidence: %w", err)))
	}
	sequence := r.seq.Add(1)
	rel := filepath.Join("invocations", r.Metadata.ID, req.Participant, req.Phase, fmt.Sprintf("%06d.json", sequence))
	if _, err := safestore.WriteExclusive(req.RunRoot, rel, data); err != nil {
		writeErr := isolation(fmt.Errorf("persist adapter evidence: %w", err))
		if runErr != nil {
			return response, errors.Join(writeErr, runErr)
		}
		return response, writeErr
	}
	return response, runErr
}

func firstFailureClass(err error) councilruntime.FailureClass {
	if err == nil {
		return ""
	}
	var run *councilruntime.RunError
	if errors.As(err, &run) {
		return run.Class
	}
	return ""
}
