package evalharness

import (
	"github.com/ShenJun93/agent-council/internal/council/protocol"
)

type Comparator string

const ComparatorBestSingle Comparator = "best_single"

type RiskPolicy struct {
	Comparator         Comparator `json:"comparator"`
	MaterialWorseDelta float64    `json:"material_worse_delta"`
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
