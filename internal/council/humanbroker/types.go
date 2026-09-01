package humanbroker

import (
	"encoding/json"
	"time"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

const (
	RequestSchemaVersion  = "council.human-broker-request.v0"
	ResponseSchemaVersion = "council.human-broker-response.v0"
	DefaultAdapterID      = "human-chatgpt-session"
)

type RequestPacket struct {
	SchemaVersion          string                      `json:"schema_version"`
	RequestID              string                      `json:"request_id"`
	Nonce                  string                      `json:"nonce"`
	RunID                  string                      `json:"run_id"`
	SlotID                 string                      `json:"slot_id"`
	AdapterID              string                      `json:"adapter_id"`
	ProviderFamily         string                      `json:"provider_family"`
	FailoverIndex          int                         `json:"failover_index"`
	FailoverTrigger        councilruntime.FailureClass `json:"failover_trigger,omitempty"`
	Instructions           []string                    `json:"instructions"`
	Participant            string                      `json:"participant"`
	Role                   string                      `json:"role"`
	Phase                  string                      `json:"phase"`
	Prompt                 string                      `json:"prompt"`
	OutputSchema           json.RawMessage             `json:"output_schema"`
	PromptSHA256           string                      `json:"prompt_sha256"`
	OutputSchemaSHA256     string                      `json:"output_schema_sha256"`
	PasteablePrompt        string                      `json:"pasteable_prompt"`
	PasteablePromptSHA256  string                      `json:"pasteable_prompt_sha256"`
	RequireFreshSession    bool                        `json:"require_fresh_session"`
	RequireCurrentSession  bool                        `json:"require_current_session,omitempty"`
	CreatedAt              time.Time                   `json:"created_at"`
}

type Submission struct {
	RequestID      string `json:"request_id"`
	Nonce          string `json:"nonce"`
	FreshSession   bool   `json:"fresh_session"`
	CurrentSession bool   `json:"current_session,omitempty"`
	ModelLabel     string `json:"model_label,omitempty"`
	RawResponse    string `json:"raw_response"`
}

type ResponseRecord struct {
	SchemaVersion string `json:"schema_version"`
	Submission
	SubmittedAt time.Time `json:"submitted_at"`
}
