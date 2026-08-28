package humanbroker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type PendingRequest struct {
	RequestID    string `json:"request_id"`
	SlotID       string `json:"slot_id"`
	Role         string `json:"role"`
	Phase        string `json:"phase"`
	PromptSHA256 string `json:"prompt_sha256"`
	CreatedAt    string `json:"created_at"`
}

func ListPending(runRoot string) ([]PendingRequest, error) {
	root := filepath.Join(runRoot, "human-broker")
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return []PendingRequest{}, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("invalid human broker root")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read human broker root: %w", err)
	}
	pending := make([]PendingRequest, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !safeRequestID.MatchString(entry.Name()) {
			continue
		}
		packet, err := LoadRequest(runRoot, entry.Name())
		if err != nil {
			return nil, err
		}
		_, answered, err := readRegular(filepath.Join(root, entry.Name(), "response.json"))
		if err != nil {
			return nil, err
		}
		if answered {
			continue
		}
		pending = append(pending, PendingRequest{
			RequestID: packet.RequestID, SlotID: packet.SlotID, Role: packet.Role,
			Phase: packet.Phase, PromptSHA256: packet.PromptSHA256,
			CreatedAt: packet.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z"),
		})
	}
	return pending, nil
}
