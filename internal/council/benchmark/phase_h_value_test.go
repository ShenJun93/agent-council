package benchmark

import (
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

func TestClassifyPhaseHValuePass(t *testing.T) {
	got := ClassifyPhaseHValue(PhaseHValueSummary{OverallMeanCouncilDelta: 0.01, MateriallyWorseCount: 0})
	if got != PhaseHOutcomePass {
		t.Fatalf("outcome=%q", got)
	}
}

func TestClassifyPhaseHValueZeroDeltaFails(t *testing.T) {
	got := ClassifyPhaseHValue(PhaseHValueSummary{OverallMeanCouncilDelta: 0, MateriallyWorseCount: 0})
	if got != PhaseHOutcomeFail {
		t.Fatalf("outcome=%q", got)
	}
}

func TestClassifyPhaseHValueMateriallyWorseFails(t *testing.T) {
	got := ClassifyPhaseHValue(PhaseHValueSummary{OverallMeanCouncilDelta: 5, MateriallyWorseCount: 1})
	if got != PhaseHOutcomeFail {
		t.Fatalf("outcome=%q", got)
	}
}

func TestSummarizePhaseHValue(t *testing.T) {
	deltas := []float64{5, 0, -2, 3, 4, 1, -1, 2, 6, 2}
	problems := make([]evalharness.ProblemResult, 0, len(deltas))
	for i, delta := range deltas {
		problems = append(problems, phaseHScoredProblem(phaseHReplayCaseIDs[i], delta, 1))
	}
	summary, err := SummarizePhaseHValue(problems)
	if err != nil {
		t.Fatal(err)
	}
	if summary.OverallMeanCouncilDelta != 2 {
		t.Fatalf("mean delta=%v", summary.OverallMeanCouncilDelta)
	}
	if summary.WinCount != 7 || summary.TieCount != 1 || summary.LossCount != 2 {
		t.Fatalf("win/tie/loss=%d/%d/%d", summary.WinCount, summary.TieCount, summary.LossCount)
	}
	if summary.MateriallyWorseCount != 0 || summary.MateriallyWorseRate != 0 {
		t.Fatalf("materially worse=%d rate=%v", summary.MateriallyWorseCount, summary.MateriallyWorseRate)
	}
	if summary.MeanJudgeDisagreement != 1 || summary.MaxJudgeDisagreement != 1 {
		t.Fatalf("judge disagreement mean/max=%v/%v", summary.MeanJudgeDisagreement, summary.MaxJudgeDisagreement)
	}
	if len(summary.PerCaseDeltas) != PhaseHReplayCaseCount || summary.PerCaseDeltas[0].ProblemID != phaseHReplayCaseIDs[0] {
		t.Fatalf("per-case deltas=%+v", summary.PerCaseDeltas)
	}
}

func TestSummarizePhaseHValueMarksExactMinusTenMateriallyWorse(t *testing.T) {
	problems := make([]evalharness.ProblemResult, 0, PhaseHReplayCaseCount)
	for i, id := range phaseHReplayCaseIDs {
		delta := float64(1)
		if i == 3 {
			delta = -10
		}
		problems = append(problems, phaseHScoredProblem(id, delta, 2))
	}
	summary, err := SummarizePhaseHValue(problems)
	if err != nil {
		t.Fatal(err)
	}
	if summary.MateriallyWorseCount != 1 || len(summary.MateriallyWorseProblemIDs) != 1 || summary.MateriallyWorseProblemIDs[0] != phaseHReplayCaseIDs[3] {
		t.Fatalf("materially worse=%+v", summary)
	}
}

func phaseHScoredProblem(problemID string, delta, judgeSpread float64) evalharness.ProblemResult {
	arms := make([]evalharness.ArmScore, 0, len(phaseHReplayArms))
	for _, arm := range phaseHReplayArms {
		mean := float64(75)
		if string(arm) == "A" {
			mean = 80
		}
		if string(arm) == "B" {
			mean = 78
		}
		if string(arm) == "F" {
			mean = 80 + delta
		}
		arms = append(arms, evalharness.ArmScore{Arm: arm, MeanScore: mean, JudgeSpread: judgeSpread})
	}
	return evalharness.ProblemResult{ProblemID: problemID, RiskPolicy: PhaseHRiskPolicy, Arms: arms}
}
