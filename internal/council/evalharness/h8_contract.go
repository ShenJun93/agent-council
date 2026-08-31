package evalharness

import "fmt"

const CitationContractStructuredV3 CitationContract = CitationContractStructuredV2 + 1

func (h H8Harness) prepare(req ProblemRequest) (preparedProblem, error) {
	if h.CitationContract != CitationContractStructuredV3 {
		return preparedProblem{}, fmt.Errorf("H8 evaluator requires citation contract V3")
	}
	base := h.Harness
	base.CitationContract = CitationContractStructuredV2
	return base.prepare(req)
}
