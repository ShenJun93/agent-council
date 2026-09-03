package benchmark

import (
	"fmt"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

type PhaseHOutcome string

const (
	PhaseHOutcomePass PhaseHOutcome = "PASS"
	PhaseHOutcomeFail PhaseHOutcome = "FAIL"
)

type PhaseHCaseDelta struct {
	ProblemID    string  `json:"problem_id"`
	CouncilDelta float64 `json:"council_delta"`
}

type PhaseHValueSummary struct {
	OverallMeanCouncilDelta   float64           `json:"overall_mean_council_delta"`
	WinCount                  int               `json:"win_count"`
	TieCount                  int               `json:"tie_count"`
	LossCount                 int               `json:"loss_count"`
	MateriallyWorseCount      int               `json:"materially_worse_count"`
	MateriallyWorseRate       float64           `json:"materially_worse_rate"`
	MateriallyWorseProblemIDs []string          `json:"materially_worse_problem_ids"`
	MeanJudgeDisagreement     float64           `json:"mean_judge_disagreement"`
	MaxJudgeDisagreement      float64           `json:"max_judge_disagreement"`
	PerCaseDeltas             []PhaseHCaseDelta `json:"per_case_deltas"`
}

func ClassifyPhaseHValue(summary PhaseHValueSummary) PhaseHOutcome {
	if summary.OverallMeanCouncilDelta > 0 && summary.MateriallyWorseCount == 0 {
		return PhaseHOutcomePass
	}
	return PhaseHOutcomeFail
}

func SummarizePhaseHValue(problems []evalharness.ProblemResult) (PhaseHValueSummary, error) {
	if len(problems) != PhaseHReplayCaseCount {
		return PhaseHValueSummary{}, fmt.Errorf("phase H value summary problems=%d, want %d", len(problems), PhaseHReplayCaseCount)
	}
	summary := PhaseHValueSummary{
		MateriallyWorseProblemIDs: make([]string, 0),
		PerCaseDeltas:             make([]PhaseHCaseDelta, 0, PhaseHReplayCaseCount),
	}
	var deltaTotal float64
	var spreadTotal float64
	var spreadCount int
	for i, problem := range problems {
		if problem.ProblemID != phaseHReplayCaseIDs[i] {
			return PhaseHValueSummary{}, fmt.Errorf("phase H problem %d id=%q, want %q", i+1, problem.ProblemID, phaseHReplayCaseIDs[i])
		}
		if problem.RiskPolicy != PhaseHRiskPolicy {
			return PhaseHValueSummary{}, fmt.Errorf("phase H problem %q risk policy mismatch", problem.ProblemID)
		}
		delta, spreads, err := phaseHProblemMetrics(problem)
		if err != nil {
			return PhaseHValueSummary{}, err
		}
		deltaTotal += delta
		summary.PerCaseDeltas = append(summary.PerCaseDeltas, PhaseHCaseDelta{ProblemID: problem.ProblemID, CouncilDelta: delta})
		switch {
		case delta > 0:
			summary.WinCount++
		case delta < 0:
			summary.LossCount++
		default:
			summary.TieCount++
		}
		if delta <= -PhaseHRiskPolicy.MaterialWorseDelta {
			summary.MateriallyWorseCount++
			summary.MateriallyWorseProblemIDs = append(summary.MateriallyWorseProblemIDs, problem.ProblemID)
		}
		for _, spread := range spreads {
			spreadTotal += spread
			spreadCount++
			if spread > summary.MaxJudgeDisagreement {
				summary.MaxJudgeDisagreement = spread
			}
		}
	}
	summary.OverallMeanCouncilDelta = deltaTotal / float64(PhaseHReplayCaseCount)
	summary.MateriallyWorseRate = float64(summary.MateriallyWorseCount) / float64(PhaseHReplayCaseCount)
	if spreadCount > 0 {
		summary.MeanJudgeDisagreement = spreadTotal / float64(spreadCount)
	}
	return summary, nil
}

func phaseHProblemMetrics(problem evalharness.ProblemResult) (float64, []float64, error) {
	if len(problem.Arms) != len(phaseHReplayArms) {
		return 0, nil, fmt.Errorf("phase H problem %q arms=%d, want %d", problem.ProblemID, len(problem.Arms), len(phaseHReplayArms))
	}
	means := make(map[baseline.Arm]float64, len(problem.Arms))
	spreads := make([]float64, 0, len(problem.Arms))
	for i, armScore := range problem.Arms {
		if armScore.Arm != phaseHReplayArms[i] {
			return 0, nil, fmt.Errorf("phase H problem %q arm %d=%q, want %q", problem.ProblemID, i+1, armScore.Arm, phaseHReplayArms[i])
		}
		if armScore.JudgeSpread < 0 {
			return 0, nil, fmt.Errorf("phase H problem %q arm %s has negative judge spread", problem.ProblemID, armScore.Arm)
		}
		means[armScore.Arm] = armScore.MeanScore
		spreads = append(spreads, armScore.JudgeSpread)
	}
	bestSingle := means[baseline.ArmAClaudeSingle]
	if means[baseline.ArmBCodexSingle] > bestSingle {
		bestSingle = means[baseline.ArmBCodexSingle]
	}
	return means[baseline.ArmFBlindCouncil] - bestSingle, spreads, nil
}
