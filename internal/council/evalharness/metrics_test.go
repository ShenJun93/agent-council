package evalharness

import (
	"math"
	"reflect"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
)

func scoreArm(arm baseline.Arm, mean, spread float64) ArmScore {
	return ArmScore{Arm: arm, MeanScore: mean, JudgeSpread: spread}
}

func metricProblem(id string, a, b, c, d, e, f float64, policy RiskPolicy) ProblemResult {
	return ProblemResult{
		ProblemID:  id,
		RiskPolicy: policy,
		Arms: []ArmScore{
			scoreArm(baseline.ArmAClaudeSingle, a, 2),
			scoreArm(baseline.ArmBCodexSingle, b, 4),
			scoreArm(baseline.ArmCClaudeSelfReview, c, 6),
			scoreArm(baseline.ArmDCodexSelfReview, d, 8),
			scoreArm(baseline.ArmEFullInfo, e, 10),
			scoreArm(baseline.ArmFBlindCouncil, f, 12),
		},
	}
}

func TestSummarizeBatchReportsDistributionAndCouncilTailRisk(t *testing.T) {
	t.Parallel()

	policy := RiskPolicy{Comparator: ComparatorBestSingle, MaterialWorseDelta: 5}
	problems := []ProblemResult{
		metricProblem("p2", 60, 75, 68, 69, 73, 71, policy),
		metricProblem("p1", 80, 70, 76, 75, 78, 74, policy),
		metricProblem("p3", 90, 85, 88, 87, 86, 80, policy),
	}

	summary, err := SummarizeBatch(problems, policy)
	if err != nil {
		t.Fatalf("SummarizeBatch() error = %v", err)
	}
	if summary.ProblemCount != 3 {
		t.Fatalf("problem count = %d, want 3", summary.ProblemCount)
	}

	a := summary.Arms[baseline.ArmAClaudeSingle]
	assertClose(t, "A mean", a.Mean, 76.66666666666667)
	assertClose(t, "A variance", a.Variance, 155.55555555555557)
	if a.Count != 3 || a.Min != 60 || a.Max != 90 || a.Median != 80 || a.P10 != 60 || a.P90 != 90 || a.MeanJudgeSpread != 2 {
		t.Fatalf("A distribution = %+v", a)
	}

	f := summary.Arms[baseline.ArmFBlindCouncil]
	if f.Count != 3 || f.Mean != 75 || f.Variance != 14 || f.Min != 71 || f.Max != 80 || f.Median != 74 || f.P10 != 71 || f.P90 != 80 || f.MeanJudgeSpread != 12 {
		t.Fatalf("F distribution = %+v", f)
	}

	tail := summary.CouncilVsBestSingle
	assertClose(t, "tail mean delta", tail.MeanDelta, -6.666666666666667)
	assertClose(t, "tail variance", tail.DeltaVariance, 6.222222222222222)
	if tail.MinDelta != -10 || tail.P10Delta != -10 {
		t.Fatalf("tail min/p10 = %.2f/%.2f", tail.MinDelta, tail.P10Delta)
	}
	if tail.MateriallyWorseCount != 2 || tail.MateriallyWorseRate != 2.0/3.0 {
		t.Fatalf("tail worse count/rate = %d/%f", tail.MateriallyWorseCount, tail.MateriallyWorseRate)
	}
	if !reflect.DeepEqual(tail.MateriallyWorseProblemIDs, []string{"p1", "p3"}) {
		t.Fatalf("tail problem ids = %v", tail.MateriallyWorseProblemIDs)
	}
}

func TestSummarizeBatchMaterialWorseBoundaryIsInclusive(t *testing.T) {
	t.Parallel()

	policy := RiskPolicy{Comparator: ComparatorBestSingle, MaterialWorseDelta: 5}
	problem := metricProblem("boundary", 80, 70, 75, 75, 76, 75, policy)
	summary, err := SummarizeBatch([]ProblemResult{problem}, policy)
	if err != nil {
		t.Fatalf("SummarizeBatch() error = %v", err)
	}
	if summary.CouncilVsBestSingle.MateriallyWorseCount != 1 {
		t.Fatalf("boundary delta -5 must count as materially worse: %+v", summary.CouncilVsBestSingle)
	}
}

func TestDistributionUsesNearestRankPercentilesAndEvenMedian(t *testing.T) {
	t.Parallel()

	got, err := distribution([]float64{10, 20, 30, 40}, []float64{2, 4, 6, 8})
	if err != nil {
		t.Fatalf("distribution() error = %v", err)
	}
	if got.Count != 4 || got.Mean != 25 || got.Variance != 125 || got.Min != 10 || got.Max != 40 || got.Median != 25 || got.P10 != 10 || got.P90 != 40 || got.MeanJudgeSpread != 5 {
		t.Fatalf("distribution = %+v", got)
	}
}

func TestSummarizeBatchRejectsInvalidOrMixedInputs(t *testing.T) {
	t.Parallel()

	validPolicy := RiskPolicy{Comparator: ComparatorBestSingle, MaterialWorseDelta: 5}
	if _, err := SummarizeBatch(nil, validPolicy); err == nil {
		t.Fatal("empty batch unexpectedly accepted")
	}
	if _, err := SummarizeBatch([]ProblemResult{metricProblem("p", 1, 2, 3, 4, 5, 6, validPolicy)}, RiskPolicy{}); err == nil {
		t.Fatal("invalid risk policy unexpectedly accepted")
	}

	mixed := metricProblem("mixed", 1, 2, 3, 4, 5, 6, RiskPolicy{Comparator: ComparatorBestSingle, MaterialWorseDelta: 7})
	if _, err := SummarizeBatch([]ProblemResult{mixed}, validPolicy); err == nil {
		t.Fatal("mixed risk policy unexpectedly accepted")
	}

	missingArm := metricProblem("missing", 1, 2, 3, 4, 5, 6, validPolicy)
	missingArm.Arms = missingArm.Arms[:5]
	if _, err := SummarizeBatch([]ProblemResult{missingArm}, validPolicy); err == nil {
		t.Fatal("problem missing an arm unexpectedly accepted")
	}
}

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %.12f, want %.12f", name, got, want)
	}
}
