package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
)

func TestWriteBaselineArmResultCreatesExactImmutablePath(t *testing.T) {
	root := t.TempDir()
	result := baseline.ArmResult{Arm: baseline.ArmAClaudeSingle, InvocationCount: 1}
	if err := WriteBaselineArmResult(context.Background(), root, "tech-01-db-cutover", result); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "baseline", "tech-01-db-cutover", "arm-A.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("missing arm artifact: %v", err)
	}
	if err := WriteBaselineArmResult(context.Background(), root, "tech-01-db-cutover", result); err == nil {
		t.Fatal("expected duplicate arm write rejection")
	}
}

func TestWriteBaselineArmResultRejectsUnsafeProblemAndUnknownArm(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		problem string
		result  baseline.ArmResult
	}{
		{problem: "../escape", result: baseline.ArmResult{Arm: baseline.ArmAClaudeSingle}},
		{problem: "tech-01-db-cutover", result: baseline.ArmResult{Arm: baseline.Arm("Z")}},
	}
	for _, tc := range cases {
		if err := WriteBaselineArmResult(context.Background(), root, tc.problem, tc.result); err == nil {
			t.Fatalf("unexpected success: problem=%q arm=%q", tc.problem, tc.result.Arm)
		}
	}
}

func TestWriteBaselineArmResultRejectsSymlinkedBaselineParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "baseline")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	result := baseline.ArmResult{Arm: baseline.ArmAClaudeSingle, InvocationCount: 1}
	if err := WriteBaselineArmResult(context.Background(), root, "tech-01-db-cutover", result); err == nil {
		t.Fatal("expected symlink rejection")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink escape wrote outside root: %v", entries)
	}
}
