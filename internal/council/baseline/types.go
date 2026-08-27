package baseline

import (
	"encoding/json"

	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

type Arm string

const (
	ArmAClaudeSingle     Arm = "A"
	ArmBCodexSingle      Arm = "B"
	ArmCClaudeSelfReview Arm = "C"
	ArmDCodexSelfReview  Arm = "D"
	ArmEFullInfo         Arm = "E"
	ArmFBlindCouncil     Arm = "F"
)

type AnswerArtifact struct {
	Decision    string                 `json:"decision"`
	Action      string                 `json:"action"`
	Reasons     []string               `json:"reasons"`
	Assumptions []string               `json:"assumptions"`
	Risks       []string               `json:"risks"`
	Citations   []protocol.EvidenceRef `json:"citations"`
	Confidence  float64                `json:"confidence"`
}

type ArmResult struct {
	Arm             Arm              `json:"arm"`
	InvocationCount int              `json:"invocation_count"`
	Answer          *AnswerArtifact  `json:"answer,omitempty"`
	Protocol        *protocol.Result `json:"protocol,omitempty"`
}

type RunRequest struct {
	RunID             string
	RunRoot           string
	NormalizedProblem json.RawMessage
}

type CitationAuthority uint8

const (
	CitationAuthorityVisibleArtifacts CitationAuthority = iota
	CitationAuthorityProblemOnlyFinal
)

type Runner struct {
	Claude             councilruntime.AgentRuntime
	Codex              councilruntime.AgentRuntime
	TempRoot           string
	ChallengerProvider councilruntime.Provider
	ChallengePolicy    protocol.ChallengePolicy
	CitationAuthority  CitationAuthority
}
