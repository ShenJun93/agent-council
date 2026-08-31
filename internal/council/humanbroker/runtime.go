package humanbroker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
	"github.com/ShenJun93/agent-council/internal/council/safestore"
)

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,127}$`)

const defaultWaitTimeout = 12 * time.Hour
const defaultPollInterval = 2 * time.Second

type Runtime struct {
	WaitTimeout    time.Duration
	PollInterval   time.Duration
	CurrentSession bool
	NewRequestID   func() (string, error)
	NewNonce       func() (string, error)
	Now            func() time.Time
}

func (r *Runtime) Run(ctx context.Context, req councilruntime.AgentRequest) (councilruntime.AgentResponse, error) {
	started := r.now()
	requestID, err := r.newRequestID()
	if err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	nonce, err := r.newNonce()
	if err != nil {
		return councilruntime.AgentResponse{}, isolation(err)
	}
	if !safeRequestID.MatchString(requestID) || !safeRequestID.MatchString(nonce) {
		return councilruntime.AgentResponse{}, isolation(fmt.Errorf("unsafe broker request identity"))
	}
	adapterID := strings.TrimSpace(req.AdapterID)
	if adapterID == "" {
		adapterID = DefaultAdapterID
	}
	currentSession := r != nil && r.CurrentSession
	instructions := []string{
		"Open a New Chat in ChatGPT with no prior context.",
		"Paste only pasteable_prompt into that fresh session; do not add files, tools, browsing, or prior conversation context.",
		"Return the raw assistant response unchanged and attest fresh_session=true when submitting.",
	}
	if currentSession {
		instructions = []string{
			"Use the current ChatGPT web orchestration session; do not open a New Chat or switch providers.",
			"Treat pasteable_prompt as the complete benchmark task input for this role and do not incorporate unrelated prior conversation context.",
			"Return the raw assistant response unchanged and attest current_session=true and fresh_session=false when submitting.",
		}
	}
	pasteable := buildPasteablePrompt(req.Prompt, req.OutputSchema)
	packet := RequestPacket{
		SchemaVersion: RequestSchemaVersion, RequestID: requestID, Nonce: nonce,
		RunID: req.RunID, SlotID: req.SlotID, AdapterID: adapterID,
		ProviderFamily: string(councilruntime.ProviderChatGPT), Participant: req.Participant,
		FailoverIndex: req.FailoverIndex, FailoverTrigger: req.FailoverTrigger,
		Instructions: instructions,
		Role:         req.Role, Phase: req.Phase, Prompt: req.Prompt,
		OutputSchema: append(json.RawMessage(nil), req.OutputSchema...),
		PromptSHA256: digest([]byte(req.Prompt)), OutputSchemaSHA256: digest(req.OutputSchema),
		PasteablePrompt: pasteable, PasteablePromptSHA256: digest([]byte(pasteable)),
		RequireFreshSession: !currentSession, RequireCurrentSession: currentSession, CreatedAt: started,
	}
	relDir := filepath.Join("human-broker", requestID)
	if _, err := safestore.WriteExclusive(req.RunRoot, filepath.Join(relDir, "prompt.txt"), []byte(pasteable)); err != nil {
		return councilruntime.AgentResponse{}, isolation(fmt.Errorf("write broker prompt: %w", err))
	}
	data, err := json.Marshal(packet)
	if err != nil {
		return councilruntime.AgentResponse{}, isolation(fmt.Errorf("encode broker packet: %w", err))
	}
	if _, err := safestore.WriteExclusive(req.RunRoot, filepath.Join(relDir, "request.json"), data); err != nil {
		return councilruntime.AgentResponse{}, isolation(fmt.Errorf("write broker packet: %w", err))
	}

	record, err := r.wait(ctx, req.RunRoot, requestID, nonce, !currentSession, currentSession)
	if err != nil {
		return councilruntime.AgentResponse{}, err
	}
	return councilruntime.AgentResponse{
		Provider: councilruntime.ProviderChatGPT, AdapterID: adapterID, SlotID: req.SlotID,
		FailoverIndex: req.FailoverIndex, FailoverTrigger: req.FailoverTrigger,
		Stdout: record.RawResponse, ExitCode: 0, Attempts: 1,
		StartedAt: started, FinishedAt: record.SubmittedAt,
	}, nil
}

func (r *Runtime) wait(ctx context.Context, root, requestID, nonce string, requireFresh, requireCurrent bool) (ResponseRecord, error) {
	timeout := r.WaitTimeout
	if timeout <= 0 {
		timeout = defaultWaitTimeout
	}
	poll := r.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		record, found, err := readResponse(root, requestID)
		if err != nil {
			return ResponseRecord{}, isolation(err)
		}
		if found {
			if err := validateRecord(record, requestID, nonce, requireFresh, requireCurrent); err != nil {
				return ResponseRecord{}, &councilruntime.RunError{Class: councilruntime.FailureMalformedOutput, Err: err}
			}
			return record, nil
		}
		select {
		case <-ctx.Done():
			return ResponseRecord{}, timeoutError(ctx.Err())
		case <-deadline.C:
			return ResponseRecord{}, timeoutError(errors.New("human ChatGPT response wait expired"))
		case <-ticker.C:
		}
	}
}

func buildPasteablePrompt(prompt string, schema json.RawMessage) string {
	if len(schema) == 0 {
		return prompt
	}
	return prompt + "\n\nTRANSPORT_OUTPUT_SCHEMA_BEGIN\nReturn exactly one JSON object matching this schema:\n" + string(schema) + "\nTRANSPORT_OUTPUT_SCHEMA_END"
}

func (r *Runtime) now() time.Time {
	if r != nil && r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}
func (r *Runtime) newRequestID() (string, error) {
	if r != nil && r.NewRequestID != nil {
		return r.NewRequestID()
	}
	return randomID("req-")
}
func (r *Runtime) newNonce() (string, error) {
	if r != nil && r.NewNonce != nil {
		return r.NewNonce()
	}
	return randomID("nonce-")
}
func randomID(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func isolation(err error) error {
	return &councilruntime.RunError{Class: councilruntime.FailureIsolation, Err: err}
}
func timeoutError(err error) error {
	return &councilruntime.RunError{Class: councilruntime.FailureTimeout, Err: err}
}

func readResponse(root, requestID string) (ResponseRecord, bool, error) {
	path := filepath.Join(root, "human-broker", requestID, "response.json")
	data, found, err := readRegular(path)
	if err != nil || !found {
		return ResponseRecord{}, found, err
	}
	var record ResponseRecord
	if err := decodeStrict(data, &record); err != nil {
		return ResponseRecord{}, true, fmt.Errorf("decode broker response: %w", err)
	}
	return record, true, nil
}

func readRegular(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("broker file is not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}
func decodeStrict(data []byte, out any) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("expected exactly one JSON object")
	}
	return nil
}
