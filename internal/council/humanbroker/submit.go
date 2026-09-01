package humanbroker

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/safestore"
)

func LoadRequest(runRoot, requestID string) (RequestPacket, error) {
	if !safeRequestID.MatchString(requestID) {
		return RequestPacket{}, fmt.Errorf("invalid request id")
	}
	requestPath := filepath.Join(runRoot, "human-broker", requestID, "request.json")
	data, found, err := readRegular(requestPath)
	if err != nil {
		return RequestPacket{}, fmt.Errorf("read broker request: %w", err)
	}
	if !found {
		return RequestPacket{}, fmt.Errorf("broker request %q does not exist", requestID)
	}
	var packet RequestPacket
	if err := decodeStrict(data, &packet); err != nil {
		return RequestPacket{}, fmt.Errorf("decode broker request: %w", err)
	}
	if packet.SchemaVersion != RequestSchemaVersion || packet.RequestID != requestID || packet.Nonce == "" || packet.AdapterID != DefaultAdapterID || packet.ProviderFamily != "chatgpt" {
		return RequestPacket{}, fmt.Errorf("invalid broker request %q", requestID)
	}
	if packet.RequireFreshSession == packet.RequireCurrentSession {
		return RequestPacket{}, fmt.Errorf("broker request %q must require exactly one ChatGPT session mode", requestID)
	}
	return packet, nil
}

func SubmitResponse(runRoot string, submission Submission) error {
	if !safeRequestID.MatchString(submission.RequestID) {
		return fmt.Errorf("invalid request id")
	}
	packet, err := LoadRequest(runRoot, submission.RequestID)
	if err != nil {
		return err
	}
	if packet.RequestID != submission.RequestID || packet.Nonce != submission.Nonce {
		return fmt.Errorf("broker request identity mismatch")
	}
	if err := validateSessionAttestation(packet.RequireFreshSession, packet.RequireCurrentSession, submission.FreshSession, submission.CurrentSession); err != nil {
		return err
	}
	if strings.TrimSpace(submission.RawResponse) == "" {
		return fmt.Errorf("raw ChatGPT response is required")
	}
	record := ResponseRecord{SchemaVersion: ResponseSchemaVersion, Submission: submission, SubmittedAt: time.Now().UTC()}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode broker response: %w", err)
	}
	_, err = safestore.WriteExclusive(runRoot, filepath.Join("human-broker", submission.RequestID, "response.json"), encoded)
	if err != nil {
		return fmt.Errorf("write broker response: %w", err)
	}
	return nil
}

func validateRecord(record ResponseRecord, requestID, nonce string, requireFresh, requireCurrent bool) error {
	if record.SchemaVersion != ResponseSchemaVersion {
		return fmt.Errorf("response schema_version %q, want %q", record.SchemaVersion, ResponseSchemaVersion)
	}
	if record.RequestID != requestID || record.Nonce != nonce {
		return fmt.Errorf("broker response identity mismatch")
	}
	if err := validateSessionAttestation(requireFresh, requireCurrent, record.FreshSession, record.CurrentSession); err != nil {
		return err
	}
	if strings.TrimSpace(record.RawResponse) == "" {
		return fmt.Errorf("raw ChatGPT response is required")
	}
	return nil
}

func validateSessionAttestation(requireFresh, requireCurrent, fresh, current bool) error {
	if requireFresh == requireCurrent {
		return fmt.Errorf("broker session mode is invalid")
	}
	if requireFresh {
		if !fresh || current {
			return fmt.Errorf("fresh ChatGPT session attestation is required")
		}
		return nil
	}
	if !current || fresh {
		return fmt.Errorf("current orchestrating ChatGPT session attestation is required")
	}
	return nil
}
