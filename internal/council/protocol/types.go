package protocol

import (
	"encoding/json"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

const (
	PhaseResearch  = "research"
	PhaseReview    = "review"
	PhaseChallenge = "challenge"
	PhaseRebuttal  = "rebuttal"
	PhaseJudge     = "judge"
)

type ChallengeMode string

const (
	ChallengeFull        ChallengeMode = "full"
	ChallengeAbbreviated ChallengeMode = "abbreviated"
)

const (
	DecisionAgreed            = "agreed"
	DecisionJudgeDisagreement = "judge_disagreement"
)

type EvidenceRef struct {
	ArtifactID string `json:"artifact_id"`
	Locator    string `json:"locator"`
	Claim      string `json:"claim"`
}

type ResearchArtifact struct {
	Recommendation string        `json:"recommendation"`
	Reasoning      []string      `json:"reasoning"`
	Considerations []string      `json:"considerations"`
	Assumptions    []string      `json:"assumptions"`
	Risks          []string      `json:"risks"`
	EvidenceNeeded []string      `json:"evidence_needed"`
	Citations      []EvidenceRef `json:"citations"`
	Confidence     float64       `json:"confidence"`
}

type ReviewArtifact struct {
	Strengths            []string `json:"strengths"`
	Weaknesses           []string `json:"weaknesses"`
	Unsupported          []string `json:"unsupported"`
	Missing              []string `json:"missing"`
	IncorrectAssumptions []string `json:"incorrect_assumptions"`
	CriticalRisks        []string `json:"critical_risks"`
	RecommendedChanges   []string `json:"recommended_changes"`
	Confidence           float64  `json:"confidence"`
}

type ChallengeArtifact struct {
	Attacks      []string `json:"attacks"`
	Falsifiers   []string `json:"falsifiers"`
	EvidenceGaps []string `json:"evidence_gaps"`
	Confidence   float64  `json:"confidence"`
}

type RebuttalArtifact struct {
	AcceptedCriticisms        []string `json:"accepted_criticisms"`
	RejectedCriticisms        []string `json:"rejected_criticisms"`
	ChangedPosition           bool     `json:"changed_position"`
	PositionCorrectBeforeFlip bool     `json:"position_correct_before_flip"`
	UpdatedRecommendation     string   `json:"updated_recommendation"`
	UpdatedConfidence         float64  `json:"updated_confidence"`
	Reasons                   []string `json:"reasons"`
}

type CitationCheck struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Note      string `json:"note"`
}

type JudgeArtifact struct {
	Decision             string          `json:"decision"`
	Confidence           float64         `json:"confidence"`
	Action               string          `json:"action"`
	Reasons              []string        `json:"reasons"`
	Evidence             []string        `json:"evidence"`
	RejectedAlternatives []string        `json:"rejected_alternatives"`
	Minority             []string        `json:"minority"`
	Unresolved           []string        `json:"unresolved"`
	Assumptions          []string        `json:"assumptions"`
	ChangeConditions     []string        `json:"change_conditions"`
	NextValidation       []string        `json:"next_validation"`
	CitationChecks       []CitationCheck `json:"citation_checks"`
}

type ResearchRecord struct {
	ID       string           `json:"id"`
	Artifact ResearchArtifact `json:"artifact"`
}

type ReviewRecord struct {
	ID       string         `json:"id"`
	TargetID string         `json:"target_id"`
	Artifact ReviewArtifact `json:"artifact"`
}

type ChallengeRecord struct {
	ID       string            `json:"id"`
	Artifact ChallengeArtifact `json:"artifact"`
}

type RebuttalRecord struct {
	ID       string           `json:"id"`
	TargetID string           `json:"target_id"`
	Artifact RebuttalArtifact `json:"artifact"`
}

type JudgeRecord struct {
	ID       string        `json:"id"`
	Artifact JudgeArtifact `json:"artifact"`
}

type ChallengeDecision struct {
	Mode                    ChallengeMode `json:"mode"`
	MaterialAgreement       bool          `json:"material_agreement"`
	HighConfidenceThreshold float64       `json:"high_confidence_threshold"`
	ResearchConfidences     [2]float64    `json:"research_confidences"`
}

type DecisionRecord struct {
	Status          string    `json:"status"`
	JudgeAgreement  bool      `json:"judge_agreement"`
	JudgeDecisions  [2]string `json:"judge_decisions"`
	MinorityReport  []string  `json:"minority_report,omitempty"`
	Unresolved      []string  `json:"unresolved,omitempty"`
	NextValidations []string  `json:"next_validations,omitempty"`
}

type Result struct {
	Research          []ResearchRecord  `json:"research"`
	Reviews           []ReviewRecord    `json:"reviews"`
	ChallengeDecision ChallengeDecision `json:"challenge_decision"`
	Challenge         ChallengeRecord   `json:"challenge"`
	Rebuttals         []RebuttalRecord  `json:"rebuttals"`
	Judges            []JudgeRecord     `json:"judges"`
	Decision          DecisionRecord    `json:"decision"`
}

type ChallengePolicy struct {
	AllowAbbreviated        bool    `json:"allow_abbreviated"`
	HighConfidenceThreshold float64 `json:"high_confidence_threshold"`
}

type RunRequest struct {
	RunID             string
	RunRoot           string
	NormalizedProblem json.RawMessage
}

type Engine struct {
	Claude             councilruntime.AgentRuntime
	Codex              councilruntime.AgentRuntime
	TempRoot           string
	ChallengerProvider councilruntime.Provider
	ChallengePolicy    ChallengePolicy
}
