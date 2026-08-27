package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/safestore"
)

func WriteBaselineArmResult(ctx context.Context, runRoot, problemID string, result baseline.ArmResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !safeDatasetID(problemID) || strings.TrimSpace(problemID) == "" {
		return fmt.Errorf("invalid H2 problem id %q", problemID)
	}
	if !isFrozenBaselineArm(result.Arm) {
		return fmt.Errorf("invalid H2 baseline arm %q", result.Arm)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal H2 baseline arm %s: %w", result.Arm, err)
	}
	rel := filepath.Join("baseline", problemID, "arm-"+string(result.Arm)+".json")
	if _, err := safestore.WriteExclusive(runRoot, rel, data); err != nil {
		return fmt.Errorf("write H2 baseline arm %s for %q: %w", result.Arm, problemID, err)
	}
	return nil
}

func isFrozenBaselineArm(arm baseline.Arm) bool {
	for _, candidate := range baseline.FrozenArms() {
		if arm == candidate {
			return true
		}
	}
	return false
}
