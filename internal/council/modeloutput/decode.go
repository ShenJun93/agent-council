package modeloutput

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Normalize accepts exactly one JSON object, either raw or wrapped in one
// untagged/json Markdown fence. It never strips prose or repairs JSON.
func Normalize(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("model output is empty")
	}
	if strings.HasPrefix(trimmed, "{") {
		if !json.Valid([]byte(trimmed)) {
			return "", fmt.Errorf("raw model output is not exactly one valid JSON object")
		}
		return trimmed, nil
	}
	if !strings.HasPrefix(trimmed, "```") {
		return "", fmt.Errorf("model output must be a JSON object or a single JSON fence")
	}
	return normalizeFence(trimmed)
}
func normalizeFence(raw string) (string, error) {
	lines := strings.Split(raw, "\n")
	if len(lines) < 3 {
		return "", fmt.Errorf("fenced model output is incomplete")
	}
	if lines[0] != "```" && lines[0] != "```json" {
		return "", fmt.Errorf("unsupported model output fence %q", lines[0])
	}
	if lines[len(lines)-1] != "```" {
		return "", fmt.Errorf("fenced model output must end at the closing fence")
	}
	body := strings.Join(lines[1:len(lines)-1], "\n")
	if strings.Contains(body, "```") {
		return "", fmt.Errorf("nested model output fence is not allowed")
	}
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "{") {
		return "", fmt.Errorf("fenced model output must contain a JSON object")
	}
	if !json.Valid([]byte(body)) {
		return "", fmt.Errorf("fenced model output is not exactly one valid JSON object")
	}
	return body, nil
}

// DecodeStrict normalizes transport and rejects unknown object fields.
func DecodeStrict(raw string, out any) error {
	normalized, err := Normalize(raw)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(normalized))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode JSON output: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values in output")
		}
		return fmt.Errorf("trailing JSON output: %w", err)
	}
	return nil
}
