package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/invocationlog"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

const (
	H5AdapterSummarySchemaVersion   = "council.h5-adapter-summary.v0"
	H5ExpectedSuccessfulInvocations = H1CaseCount * 36
)

type H5AdapterSummary struct {
	SchemaVersion              string `json:"schema_version"`
	TotalAttempts              int    `json:"total_attempts"`
	SuccessfulInvocations      int    `json:"successful_invocations"`
	AvailabilityFailures       int    `json:"availability_failures"`
	TotalAvailabilityFailovers int    `json:"total_availability_failovers"`

	HumanBrokerInvocations        int                       `json:"human_broker_invocations"`
	EffectiveAdapterDiversity     int                       `json:"effective_adapter_diversity"`
	EffectiveProviderDiversity    int                       `json:"effective_provider_diversity"`
	AttemptsByAdapter             map[string]int            `json:"attempts_by_adapter"`
	SuccessesByAdapter            map[string]int            `json:"successes_by_adapter"`
	SuccessesByProvider           map[string]int            `json:"successes_by_provider"`
	AvailabilityFailuresByAdapter map[string]int            `json:"availability_failures_by_adapter"`
	SuccessesBySlot               map[string]map[string]int `json:"successes_by_slot"`
}

func CollectH5AdapterSummary(ctx context.Context, runRoot, runID string) (H5AdapterSummary, error) {
	out := H5AdapterSummary{SchemaVersion: H5AdapterSummarySchemaVersion, AttemptsByAdapter: map[string]int{}, SuccessesByAdapter: map[string]int{}, SuccessesByProvider: map[string]int{}, AvailabilityFailuresByAdapter: map[string]int{}, SuccessesBySlot: map[string]map[string]int{}}
	adapters, providers := map[string]struct{}{}, map[string]struct{}{}
	invRoot := filepath.Join(runRoot, "invocations")
	if err := requireRealDirectory(invRoot); err != nil {
		return H5AdapterSummary{}, fmt.Errorf("validate H5 invocation root: %w", err)
	}
	err := filepath.WalkDir(invRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
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
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || strings.ToLower(filepath.Ext(path)) != ".json" {
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

		out.TotalAttempts++
		out.AttemptsByAdapter[e.AdapterID]++
		if e.FailureClass == "" {
			if e.FailoverIndex > 0 && !h5AvailabilityClass(e.FailoverTrigger) {
				return fmt.Errorf("successful failover evidence %q has invalid trigger %q", path, e.FailoverTrigger)
			}
			out.SuccessfulInvocations++
			out.SuccessesByAdapter[e.AdapterID]++
			out.SuccessesByProvider[string(e.ProviderFamily)]++
			if out.SuccessesBySlot[e.SlotID] == nil {
				out.SuccessesBySlot[e.SlotID] = map[string]int{}
			}
			out.SuccessesBySlot[e.SlotID][e.AdapterID]++
			adapters[e.AdapterID] = struct{}{}
			providers[string(e.ProviderFamily)] = struct{}{}
			if e.AdapterID == "human-chatgpt-session" {
				out.HumanBrokerInvocations++
			}
			return nil
		}
		if !h5AvailabilityClass(e.FailureClass) {
			return fmt.Errorf("terminal H5 failure evidence %q class %q cannot appear in final adapter summary", path, e.FailureClass)
		}
		out.AvailabilityFailures++
		out.AvailabilityFailuresByAdapter[e.AdapterID]++
		return nil
	})
	if err != nil {
		return H5AdapterSummary{}, err
	}
	out.TotalAvailabilityFailovers = out.AvailabilityFailures
	out.EffectiveAdapterDiversity = len(adapters)
	out.EffectiveProviderDiversity = len(providers)
	return out, nil
}

func h5AvailabilityClass(class councilruntime.FailureClass) bool {
	switch class {
	case councilruntime.FailureQuotaExhausted, councilruntime.FailureAuth, councilruntime.FailureAdapterUnavailable:
		return true
	default:
		return false
	}
}

func WriteH5AdapterSummary(ctx context.Context, runRoot string, summary H5AdapterSummary) (string, error) {
	if summary.SchemaVersion != H5AdapterSummarySchemaVersion {
		return "", fmt.Errorf("invalid H5 adapter summary schema %q", summary.SchemaVersion)
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return "", fmt.Errorf("marshal H5 adapter summary: %w", err)
	}
	if err := writeH1Specs(ctx, runRoot, []h1FileSpec{{rel: "adapter-summary.json", data: data}}); err != nil {
		return "", fmt.Errorf("write H5 adapter summary: %w", err)
	}
	return sha256Hex(data), nil
}
