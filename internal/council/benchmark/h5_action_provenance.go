package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/invocationlog"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

const H5ActionProvenanceSchemaVersion = "council.h5-action-provenance.v0"

type H5EvidenceSnapshot map[string]struct{}

type H5InvocationEvidenceRef struct {
	Path            string                      `json:"path"`
	SHA256          string                      `json:"sha256"`
	SlotID          string                      `json:"slot_id"`
	AdapterID       string                      `json:"adapter_id"`
	ProviderFamily  councilruntime.Provider     `json:"provider_family"`
	FailoverIndex   int                         `json:"failover_index"`
	FailoverTrigger councilruntime.FailureClass `json:"failover_trigger,omitempty"`
	FailureClass    councilruntime.FailureClass `json:"failure_class,omitempty"`
}

type H5ActionProvenance struct {
	SchemaVersion                 string                    `json:"schema_version"`
	RunID                         string                    `json:"run_id"`
	ProblemID                     string                    `json:"problem_id"`
	Scope                         string                    `json:"scope"`
	Arm                           baseline.Arm              `json:"arm,omitempty"`
	ExpectedSuccessfulInvocations int                       `json:"expected_successful_invocations"`
	TotalAttempts                 int                       `json:"total_attempts"`
	SuccessfulInvocations         int                       `json:"successful_invocations"`
	AvailabilityFailures          int                       `json:"availability_failures"`
	TotalAvailabilityFailovers    int                       `json:"total_availability_failovers"`
	Evidence                      []H5InvocationEvidenceRef `json:"evidence"`
}

type H5ActionRecorder interface {
	Snapshot(context.Context) (H5EvidenceSnapshot, error)
	RecordArm(context.Context, H5EvidenceSnapshot, string, baseline.Arm) error
	RecordEvaluation(context.Context, H5EvidenceSnapshot, string) error
}

type H5ActionProvenanceRecorder struct{ runRoot, runID string }

func NewH5ActionProvenanceRecorder(runRoot, runID string) *H5ActionProvenanceRecorder {
	return &H5ActionProvenanceRecorder{runRoot: runRoot, runID: runID}
}

type h5EvidenceRecord struct {
	rel      string
	sha      string
	evidence invocationlog.AdapterEvidence
}

func (r *H5ActionProvenanceRecorder) Snapshot(ctx context.Context) (H5EvidenceSnapshot, error) {
	records, err := scanH5Evidence(ctx, r.runRoot, r.runID, true)
	if err != nil {
		return nil, err
	}
	out := make(H5EvidenceSnapshot, len(records))
	for rel := range records {
		out[rel] = struct{}{}
	}
	return out, nil
}

func (r *H5ActionProvenanceRecorder) RecordArm(ctx context.Context, before H5EvidenceSnapshot, problemID string, arm baseline.Arm) error {
	expected, ok := map[baseline.Arm]int{
		baseline.ArmAClaudeSingle: 1, baseline.ArmBCodexSingle: 1,
		baseline.ArmCClaudeSelfReview: 2, baseline.ArmDCodexSelfReview: 2,
		baseline.ArmEFullInfo: 9, baseline.ArmFBlindCouncil: 9,
	}[arm]
	if !ok {
		return fmt.Errorf("unknown H5 arm %q", arm)
	}
	return r.record(ctx, before, problemID, "baseline-arm", arm, expected, filepath.Join("adapter-provenance", "problems", problemID, "arm-"+string(arm)+".json"))
}

func (r *H5ActionProvenanceRecorder) RecordEvaluation(ctx context.Context, before H5EvidenceSnapshot, problemID string) error {
	return r.record(ctx, before, problemID, "evaluation", "", 12, filepath.Join("adapter-provenance", "problems", problemID, "evaluation.json"))
}

func (r *H5ActionProvenanceRecorder) record(ctx context.Context, before H5EvidenceSnapshot, problemID, scope string, arm baseline.Arm, expected int, rel string) error {
	if !safeDatasetID(problemID) {
		return fmt.Errorf("unsafe H5 problem id %q", problemID)
	}
	records, err := scanH5Evidence(ctx, r.runRoot, r.runID, false)
	if err != nil {
		return err
	}
	paths := make([]string, 0)
	for path := range records {
		if _, seen := before[path]; !seen {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	provenance := H5ActionProvenance{SchemaVersion: H5ActionProvenanceSchemaVersion, RunID: r.runID, ProblemID: problemID, Scope: scope, Arm: arm, ExpectedSuccessfulInvocations: expected, Evidence: make([]H5InvocationEvidenceRef, 0, len(paths))}

	for _, path := range paths {
		record := records[path]
		e := record.evidence
		ref := H5InvocationEvidenceRef{Path: path, SHA256: record.sha, SlotID: e.SlotID, AdapterID: e.AdapterID, ProviderFamily: e.ProviderFamily, FailoverIndex: e.FailoverIndex, FailoverTrigger: e.FailoverTrigger, FailureClass: e.FailureClass}
		provenance.Evidence = append(provenance.Evidence, ref)
		provenance.TotalAttempts++
		if e.FailureClass == "" {
			if e.FailoverIndex > 0 && !h5AvailabilityClass(e.FailoverTrigger) {
				return fmt.Errorf("H5 action %s success has invalid failover trigger %q", scope, e.FailoverTrigger)
			}
			provenance.SuccessfulInvocations++
		} else if h5AvailabilityClass(e.FailureClass) {
			provenance.AvailabilityFailures++
		} else {
			return fmt.Errorf("H5 action %s contains terminal failure %q in %s", scope, e.FailureClass, path)
		}
	}
	provenance.TotalAvailabilityFailovers = provenance.AvailabilityFailures
	if provenance.SuccessfulInvocations != expected {
		return fmt.Errorf("H5 %s successful invocations %d, want %d", scope, provenance.SuccessfulInvocations, expected)
	}
	data, err := json.Marshal(provenance)
	if err != nil {
		return fmt.Errorf("marshal H5 action provenance: %w", err)
	}
	if err := writeH1Specs(ctx, r.runRoot, []h1FileSpec{{rel: rel, data: data}}); err != nil {
		return fmt.Errorf("write H5 action provenance: %w", err)
	}
	return nil
}

func scanH5Evidence(ctx context.Context, runRoot, runID string, allowMissing bool) (map[string]h5EvidenceRecord, error) {
	out := map[string]h5EvidenceRecord{}
	invRoot := filepath.Join(runRoot, "invocations")
	info, err := os.Lstat(invRoot)
	if errors.Is(err, os.ErrNotExist) && allowMissing {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat H5 invocation root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("H5 invocation root must be a real directory")
	}
	err = filepath.WalkDir(invRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("H5 invocation evidence contains symlink %q", path)
		}
		if d.IsDir() {
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			return err
		}
		if !fileInfo.Mode().IsRegular() || filepath.Ext(path) != ".json" {
			return fmt.Errorf("unexpected H5 invocation artifact %q", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var e invocationlog.AdapterEvidence
		if err := decodeStrict(path, data, &e); err != nil {
			return err
		}
		if e.SchemaVersion != invocationlog.AdapterSchemaVersion || e.RunID != runID || e.SlotID == "" || e.AdapterID == "" || e.ProviderFamily == "" || e.FailoverIndex < 0 {
			return fmt.Errorf("invalid H5 adapter evidence %q", path)
		}
		rel, err := filepath.Rel(runRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		out[rel] = h5EvidenceRecord{rel: rel, sha: sha256Hex(data), evidence: e}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
