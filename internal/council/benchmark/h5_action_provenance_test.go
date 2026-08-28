package benchmark

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/invocationlog"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func TestH5ActionRecorderWritesArmSidecarFromNewEvidence(t *testing.T) {
	root := t.TempDir()
	recorder := NewH5ActionProvenanceRecorder(root, "h5-test")
	before, err := recorder.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	failed := invocationlog.AdapterEvidence{SchemaVersion: invocationlog.AdapterSchemaVersion, RunID: "h5-test", SlotID: "baseline-a", AdapterID: "claude-max", ProviderFamily: councilruntime.ProviderClaude, FailureClass: councilruntime.FailureQuotaExhausted}
	writeH5Evidence(t, root, "claude/a.json", failed)
	success := failed
	success.AdapterID = "codex-chatgpt"
	success.ProviderFamily = councilruntime.ProviderCodex
	success.FailureClass = ""
	success.FailoverIndex = 1
	success.FailoverTrigger = councilruntime.FailureQuotaExhausted
	writeH5Evidence(t, root, "codex/a.json", success)
	if err := recorder.RecordArm(context.Background(), before, "tech-01-db-cutover", baseline.ArmAClaudeSingle); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "adapter-provenance", "problems", "tech-01-db-cutover", "arm-A.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got H5ActionProvenance
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Scope != "baseline-arm" || got.Arm != baseline.ArmAClaudeSingle || got.ExpectedSuccessfulInvocations != 1 || got.SuccessfulInvocations != 1 || got.AvailabilityFailures != 1 || got.TotalAttempts != 2 {
		t.Fatalf("provenance=%+v", got)
	}
	if len(got.Evidence) != 2 || got.Evidence[0].SHA256 == "" || got.Evidence[1].SHA256 == "" {
		t.Fatalf("evidence=%+v", got.Evidence)
	}
	if got.Evidence[1].AdapterID != "codex-chatgpt" || got.Evidence[1].FailoverIndex != 1 {
		t.Fatalf("success ref=%+v", got.Evidence[1])
	}
}

func TestH5ActionRecorderRejectsTerminalFailureForCompletedAction(t *testing.T) {
	root := t.TempDir()
	recorder := NewH5ActionProvenanceRecorder(root, "h5-test")
	before, err := recorder.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	bad := invocationlog.AdapterEvidence{SchemaVersion: invocationlog.AdapterSchemaVersion, RunID: "h5-test", SlotID: "baseline-a", AdapterID: "codex-chatgpt", ProviderFamily: councilruntime.ProviderCodex, FailureClass: councilruntime.FailureMalformedOutput}
	writeH5Evidence(t, root, "bad/1.json", bad)
	if err := recorder.RecordArm(context.Background(), before, "tech-01-db-cutover", baseline.ArmAClaudeSingle); err == nil {
		t.Fatal("terminal failure accepted")
	}
}

func TestH5ActionRecorderWritesEvaluationSidecarWithTwelveSuccesses(t *testing.T) {
	root := t.TempDir()
	recorder := NewH5ActionProvenanceRecorder(root, "h5-test")
	before, err := recorder.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		e := invocationlog.AdapterEvidence{SchemaVersion: invocationlog.AdapterSchemaVersion, RunID: "h5-test", SlotID: "eval-judge-1", AdapterID: "claude-max", ProviderFamily: councilruntime.ProviderClaude}
		writeH5Evidence(t, root, filepath.Join("eval", string(rune('a'+i))+".json"), e)
	}
	if err := recorder.RecordEvaluation(context.Background(), before, "tech-01-db-cutover"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "adapter-provenance", "problems", "tech-01-db-cutover", "evaluation.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got H5ActionProvenance
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Scope != "evaluation" || got.ExpectedSuccessfulInvocations != 12 || got.SuccessfulInvocations != 12 || got.TotalAttempts != 12 {
		t.Fatalf("provenance=%+v", got)
	}
}

func TestH5ActionRecorderRefusesOverwriteAndSymlinkParent(t *testing.T) {
	root := t.TempDir()
	recorder := NewH5ActionProvenanceRecorder(root, "h5-test")
	before, err := recorder.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	e := invocationlog.AdapterEvidence{SchemaVersion: invocationlog.AdapterSchemaVersion, RunID: "h5-test", SlotID: "baseline-a", AdapterID: "claude-max", ProviderFamily: councilruntime.ProviderClaude}
	writeH5Evidence(t, root, "a/1.json", e)
	if err := recorder.RecordArm(context.Background(), before, "tech-01-db-cutover", baseline.ArmAClaudeSingle); err != nil {
		t.Fatal(err)
	}
	if err := recorder.RecordArm(context.Background(), before, "tech-01-db-cutover", baseline.ArmAClaudeSingle); err == nil {
		t.Fatal("overwrite accepted")
	}

	root2 := t.TempDir()
	recorder2 := NewH5ActionProvenanceRecorder(root2, "h5-test")
	before2, err := recorder2.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	writeH5Evidence(t, root2, "a/1.json", e)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root2, "adapter-provenance")); err != nil {
		t.Fatal(err)
	}
	if err := recorder2.RecordArm(context.Background(), before2, "tech-01-db-cutover", baseline.ArmAClaudeSingle); err == nil {
		t.Fatal("symlink parent accepted")
	}
}
