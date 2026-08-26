package evalharness

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
)

func testWriteRequest(t *testing.T, root string) WriteRequest {
	t.Helper()
	policy := RiskPolicy{Comparator: ComparatorBestSingle, MaterialWorseDelta: 5}
	problems := []ProblemResult{
		metricProblem("problem-1", 80, 70, 75, 74, 78, 72, policy),
		metricProblem("problem-2", 60, 65, 62, 63, 66, 64, policy),
	}
	summary, err := SummarizeBatch(problems, policy)
	if err != nil {
		t.Fatalf("SummarizeBatch() fixture error = %v", err)
	}
	return WriteRequest{Root: root, Policy: policy, Problems: problems, Summary: summary}
}

func TestWriteEvaluationPersistsImmutableContainedArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	req := testWriteRequest(t, root)
	if err := WriteEvaluation(context.Background(), req); err != nil {
		t.Fatalf("WriteEvaluation() error = %v", err)
	}

	for _, path := range []string{
		"eval/eval-policy.json",
		"eval/batch-summary.json",
		"eval/problems/problem-1/problem-summary.json",
		"eval/problems/problem-2/problem-summary.json",
		"eval/provenance.jsonl",
	} {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("artifact %q missing or not regular: info=%v err=%v", path, info, err)
		}
	}
	for _, problemID := range []string{"problem-1", "problem-2"} {
		for _, arm := range []baseline.Arm{
			baseline.ArmAClaudeSingle,
			baseline.ArmBCodexSingle,
			baseline.ArmCClaudeSelfReview,
			baseline.ArmDCodexSelfReview,
			baseline.ArmEFullInfo,
			baseline.ArmFBlindCouncil,
		} {
			path := filepath.Join(root, "eval", "problems", problemID, "arm-"+string(arm)+".json")
			if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
				t.Fatalf("arm artifact %q missing or not regular: info=%v err=%v", path, info, err)
			}
		}
	}

	policyBytes, err := os.ReadFile(filepath.Join(root, "eval", "eval-policy.json"))
	if err != nil {
		t.Fatalf("read eval policy: %v", err)
	}
	var storedPolicy RiskPolicy
	if err := json.Unmarshal(policyBytes, &storedPolicy); err != nil {
		t.Fatalf("decode eval policy: %v", err)
	}
	if storedPolicy != req.Policy {
		t.Fatalf("stored policy = %+v, want %+v", storedPolicy, req.Policy)
	}

	provenanceFile, err := os.Open(filepath.Join(root, "eval", "provenance.jsonl"))
	if err != nil {
		t.Fatalf("open provenance: %v", err)
	}
	defer provenanceFile.Close()
	var entries []EvalProvenance
	scanner := bufio.NewScanner(provenanceFile)
	for scanner.Scan() {
		var entry EvalProvenance
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("decode provenance: %v", err)
		}
		if entry.Path == "" || len(entry.SHA256) != 64 {
			t.Fatalf("invalid provenance entry: %+v", entry)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan provenance: %v", err)
	}
	if len(entries) != 16 {
		t.Fatalf("provenance entries = %d, want 16", len(entries))
	}

	if err := WriteEvaluation(context.Background(), req); err == nil {
		t.Fatal("second WriteEvaluation() unexpectedly overwrote immutable artifacts")
	}
}

func TestWriteEvaluationRejectsInconsistentSummaryBeforeWriting(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	req := testWriteRequest(t, root)
	req.Summary.ProblemCount++
	if err := WriteEvaluation(context.Background(), req); err == nil || !strings.Contains(strings.ToLower(err.Error()), "summary") {
		t.Fatalf("WriteEvaluation() error = %v, want summary mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(root, "eval")); !os.IsNotExist(err) {
		t.Fatalf("eval root created before validation: %v", err)
	}
}

func TestWriteEvaluationRejectsUnsafeProblemID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	req := testWriteRequest(t, root)
	req.Problems[0].ProblemID = "../escape"
	if err := WriteEvaluation(context.Background(), req); err == nil {
		t.Fatal("unsafe problem ID unexpectedly accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
		t.Fatalf("unsafe write escaped eval root: %v", err)
	}
}

func TestWriteEvaluationRejectsSymlinkEvalRootWithoutWritingOutside(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "eval")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	req := testWriteRequest(t, root)
	if err := WriteEvaluation(context.Background(), req); err == nil {
		t.Fatal("symlinked eval root unexpectedly accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatalf("read outside dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("writer escaped through symlink: %v", entries)
	}
}

func TestProblemSummaryDerivationUsesBestSingleBoundary(t *testing.T) {
	t.Parallel()

	policy := RiskPolicy{Comparator: ComparatorBestSingle, MaterialWorseDelta: 5}
	problem := metricProblem("boundary", 80, 70, 75, 74, 78, 75, policy)
	summary, err := summarizeProblem(problem, policy)
	if err != nil {
		t.Fatalf("summarizeProblem() error = %v", err)
	}
	want := ProblemSummary{
		ProblemID:       "boundary",
		BestSingleScore: 80,
		CouncilScore:    75,
		CouncilDelta:    -5,
		MateriallyWorse: true,
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("problem summary = %+v, want %+v", summary, want)
	}
}
