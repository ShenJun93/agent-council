package evalharness

import (
	"time"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

const PhaseEvalJudge = "eval-judge"

type Comparator string

const ComparatorBestSingle Comparator = "best_single"

type RiskPolicy struct {
	Comparator         Comparator `json:"comparator"`
	MaterialWorseDelta float64    `json:"material_worse_delta"`
}

type RubricDocument struct {
	Dimensions []RubricDimension `json:"dimensions"`
}

type RubricDimension struct {
	ID string `json:"id"`
}

type MaskedCandidate struct {
	Decision        string                 `json:"decision"`
	Action          string                 `json:"action,omitempty"`
	Reasons         []string               `json:"reasons"`
	Assumptions     []string               `json:"assumptions"`
	Risks           []string               `json:"risks"`
	Citations       []protocol.EvidenceRef `json:"citations"`
	Evidence        []string               `json:"evidence"`
	Minority        []string               `json:"minority"`
	Unresolved      []string               `json:"unresolved"`
	NextValidations []string               `json:"next_validations"`
}

type JudgeArtifact struct {
	OverallScore       float64                  `json:"overall_score"`
	Dimensions         map[string]float64       `json:"dimensions"`
	CitationChecks     []protocol.CitationCheck `json:"citation_checks"`
	ReliedOnCitations  []string                 `json:"relied_on_citations"`
	CriticalErrors     []string                 `json:"critical_errors"`
	Strengths          []string                 `json:"strengths"`
	Weaknesses         []string                 `json:"weaknesses"`
	Confidence         float64                  `json:"confidence"`
}

type JudgeScore struct {
	Slot          string                  `json:"slot"`
	Provider      councilruntime.Provider `json:"provider"`
	Artifact      JudgeArtifact           `json:"artifact"`
	InputHashes   map[string]string       `json:"input_hashes"`
	OutputSHA256  string                  `json:"output_sha256"`
	StartedAt     time.Time               `json:"started_at"`
	FinishedAt    time.Time               `json:"finished_at"`
}

type ArmScore struct {
	Arm         baseline.Arm `json:"arm"`
	Judges      [2]JudgeScore `json:"judges"`
	MeanScore   float64       `json:"mean_score"`
	JudgeSpread float64       `json:"judge_spread"`
}

type ProblemResult struct {
	ProblemID          string     `json:"problem_id"`
	RubricSHA256       string     `json:"rubric_sha256"`
	ReferenceSetSHA256 string     `json:"reference_set_sha256"`
	RiskPolicy         RiskPolicy `json:"risk_policy"`
	Arms               []ArmScore `json:"arms"`
}
