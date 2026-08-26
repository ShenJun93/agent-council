package evalharness

import (
	"fmt"
	"math"
	"sort"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
)

func SummarizeBatch(problems []ProblemResult, policy RiskPolicy) (BatchSummary, error) {
	if err := validateRiskPolicy(policy); err != nil {
		return BatchSummary{}, err
	}
	if len(problems) == 0 {
		return BatchSummary{}, fmt.Errorf("evaluation batch must contain at least one problem")
	}

	values := make(map[baseline.Arm][]float64, len(frozenEvalArms))
	spreads := make(map[baseline.Arm][]float64, len(frozenEvalArms))
	for _, arm := range frozenEvalArms {
		values[arm] = make([]float64, 0, len(problems))
		spreads[arm] = make([]float64, 0, len(problems))
	}

	deltas := make([]float64, 0, len(problems))
	materiallyWorseIDs := make([]string, 0)
	seenProblems := make(map[string]struct{}, len(problems))
	for _, problem := range problems {
		if !safeID(problem.ProblemID) || problem.ProblemID == "" {
			return BatchSummary{}, fmt.Errorf("unsafe or empty problem id %q", problem.ProblemID)
		}
		if _, duplicate := seenProblems[problem.ProblemID]; duplicate {
			return BatchSummary{}, fmt.Errorf("duplicate problem id %q", problem.ProblemID)
		}
		seenProblems[problem.ProblemID] = struct{}{}
		if problem.RiskPolicy != policy {
			return BatchSummary{}, fmt.Errorf("problem %q risk policy differs from frozen batch policy", problem.ProblemID)
		}

		byArm, err := validateProblemArmScores(problem.Arms)
		if err != nil {
			return BatchSummary{}, fmt.Errorf("problem %q: %w", problem.ProblemID, err)
		}
		for _, arm := range frozenEvalArms {
			score := byArm[arm]
			values[arm] = append(values[arm], score.MeanScore)
			spreads[arm] = append(spreads[arm], score.JudgeSpread)
		}

		bestSingle := math.Max(byArm[baseline.ArmAClaudeSingle].MeanScore, byArm[baseline.ArmBCodexSingle].MeanScore)
		delta := byArm[baseline.ArmFBlindCouncil].MeanScore - bestSingle
		deltas = append(deltas, delta)
		if delta <= -policy.MaterialWorseDelta {
			materiallyWorseIDs = append(materiallyWorseIDs, problem.ProblemID)
		}
	}

	armSummaries := make(map[baseline.Arm]DistributionSummary, len(frozenEvalArms))
	for _, arm := range frozenEvalArms {
		dist, err := distribution(values[arm], spreads[arm])
		if err != nil {
			return BatchSummary{}, fmt.Errorf("arm %s distribution: %w", arm, err)
		}
		armSummaries[arm] = dist
	}

	deltaMean, deltaVariance := meanAndPopulationVariance(deltas)
	sortedDeltas := sortedCopy(deltas)
	sort.Strings(materiallyWorseIDs)
	tail := CouncilTailSummary{
		Comparator:                policy.Comparator,
		MaterialWorseDelta:        policy.MaterialWorseDelta,
		MeanDelta:                 deltaMean,
		DeltaVariance:             deltaVariance,
		MinDelta:                  sortedDeltas[0],
		P10Delta:                  nearestRank(sortedDeltas, 0.10),
		MateriallyWorseCount:      len(materiallyWorseIDs),
		MateriallyWorseRate:       float64(len(materiallyWorseIDs)) / float64(len(problems)),
		MateriallyWorseProblemIDs: materiallyWorseIDs,
	}

	return BatchSummary{
		ProblemCount:        len(problems),
		Arms:                armSummaries,
		CouncilVsBestSingle: tail,
	}, nil
}

func distribution(values, judgeSpreads []float64) (DistributionSummary, error) {
	if len(values) == 0 {
		return DistributionSummary{}, fmt.Errorf("distribution requires at least one value")
	}
	if len(values) != len(judgeSpreads) {
		return DistributionSummary{}, fmt.Errorf("values/judge spread count mismatch")
	}
	for i, value := range values {
		if !validScore(value) {
			return DistributionSummary{}, fmt.Errorf("value %d %.4f must be between 0 and 100", i, value)
		}
	}
	for i, spread := range judgeSpreads {
		if spread < 0 || spread > 100 || math.IsNaN(spread) || math.IsInf(spread, 0) {
			return DistributionSummary{}, fmt.Errorf("judge spread %d %.4f must be between 0 and 100", i, spread)
		}
	}

	sorted := sortedCopy(values)
	mean, variance := meanAndPopulationVariance(values)
	spreadMean, _ := meanAndPopulationVariance(judgeSpreads)
	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	return DistributionSummary{
		Count:           len(values),
		Mean:            mean,
		Variance:        variance,
		Min:             sorted[0],
		Max:             sorted[len(sorted)-1],
		Median:          median,
		P10:             nearestRank(sorted, 0.10),
		P90:             nearestRank(sorted, 0.90),
		MeanJudgeSpread: spreadMean,
	}, nil
}

func validateProblemArmScores(scores []ArmScore) (map[baseline.Arm]ArmScore, error) {
	if len(scores) != len(frozenEvalArms) {
		return nil, fmt.Errorf("expected exactly six arm scores, got %d", len(scores))
	}
	allowed := make(map[baseline.Arm]struct{}, len(frozenEvalArms))
	for _, arm := range frozenEvalArms {
		allowed[arm] = struct{}{}
	}
	byArm := make(map[baseline.Arm]ArmScore, len(scores))
	for _, score := range scores {
		if _, ok := allowed[score.Arm]; !ok {
			return nil, fmt.Errorf("unknown arm score %q", score.Arm)
		}
		if _, duplicate := byArm[score.Arm]; duplicate {
			return nil, fmt.Errorf("duplicate arm score %q", score.Arm)
		}
		if !validScore(score.MeanScore) {
			return nil, fmt.Errorf("arm %s mean score %.4f must be between 0 and 100", score.Arm, score.MeanScore)
		}
		if score.JudgeSpread < 0 || score.JudgeSpread > 100 || math.IsNaN(score.JudgeSpread) || math.IsInf(score.JudgeSpread, 0) {
			return nil, fmt.Errorf("arm %s judge spread %.4f must be between 0 and 100", score.Arm, score.JudgeSpread)
		}
		byArm[score.Arm] = score
	}
	for _, arm := range frozenEvalArms {
		if _, ok := byArm[arm]; !ok {
			return nil, fmt.Errorf("missing arm score %s", arm)
		}
	}
	return byArm, nil
}

func meanAndPopulationVariance(values []float64) (float64, float64) {
	var sum float64
	for _, value := range values {
		sum += value
	}
	mean := sum / float64(len(values))
	var squared float64
	for _, value := range values {
		delta := value - mean
		squared += delta * delta
	}
	return mean, squared / float64(len(values))
}

func sortedCopy(values []float64) []float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return sorted
}

func nearestRank(sorted []float64, percentile float64) float64 {
	rank := int(math.Ceil(percentile * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}
