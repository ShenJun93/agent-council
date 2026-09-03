package benchmark

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

const (
	PhaseHBenchmarkID                         = "phase-h"
	PhaseHReplayDatasetSchemaVersion          = "council.phase-h-replay-dataset.v0"
	PhaseHAdapterPolicySchemaVersion          = "council.phase-h-adapter-policy.v0"
	PhaseHRunSchemaVersion                    = "council.phase-h-run.v0"
	PhaseHResultSchemaVersion                 = "council.phase-h-result.v0"
	PhaseHReplayMode                          = "technical_value_validation_replay"
	PhaseHHumanAdapterID                      = "human-chatgpt-session"
	PhaseHReplayCaseCount                     = 10
	PhaseHExpectedSuccessfulInvocations       = PhaseHReplayCaseCount * 6 * 2
	PhaseHSourceWorkflowRunID           int64 = 33349114073
	PhaseHSourceFrozenSHA                     = "8d13f0a82758f5ea6286409d55123b97929dbce4"
	PhaseHSourceH8RunID                       = "h8-20260831T015809Z-e8ab51986046"
	PhaseHSourceH8ArtifactID            int64 = 9745340503
	PhaseHSourceH8ArtifactDigest              = "sha256:c37cbab9cb4d1a8c5b4f6cc2535268c681456d2b6380587fb178396ae9dedfd6"
)

var PhaseHRiskPolicy = H8RiskPolicy

var phaseHReplayCaseIDs = []string{
	"tech-01-db-cutover", "tech-02-api-rate-limits", "tech-03-cache-stampede", "tech-04-token-rotation", "tech-05-queue-ordering",
	"tech-06-backup-retention", "tech-07-deploy-rollback", "tech-08-observability-sampling", "tech-09-search-build-buy", "tech-10-data-reconciliation",
}
var phaseHReplayArms = []baseline.Arm{
	baseline.ArmAClaudeSingle,
	baseline.ArmBCodexSingle,
	baseline.ArmCClaudeSelfReview,
	baseline.ArmDCodexSelfReview,
	baseline.ArmEFullInfo,
	baseline.ArmFBlindCouncil,
}

type PhaseHReplayArmManifest struct {
	Arm    string `json:"arm"`
	SHA256 string `json:"sha256"`
}

type PhaseHReplayCaseManifest struct {
	ID                 string                    `json:"id"`
	ProblemSHA256      string                    `json:"problem_sha256"`
	ReferenceSetSHA256 string                    `json:"reference_set_sha256"`
	Arms               []PhaseHReplayArmManifest `json:"arms"`
}

type PhaseHReplayManifest struct {
	SchemaVersion                 string                     `json:"schema_version"`
	BenchmarkID                   string                     `json:"benchmark_id"`
	Mode                          string                     `json:"mode"`
	SourceWorkflowRunID           int64                      `json:"source_workflow_run_id"`
	SourceFrozenSHA               string                     `json:"source_frozen_sha"`
	SourceH8RunID                 string                     `json:"source_h8_run_id"`
	SourceH8ArtifactID            int64                      `json:"source_h8_artifact_id"`
	SourceH8ArtifactDigest        string                     `json:"source_h8_artifact_digest"`
	ExpectedSuccessfulInvocations int                        `json:"expected_successful_invocations"`
	RequireCurrentSession         bool                       `json:"require_current_session"`
	RequireFreshSession           bool                       `json:"require_fresh_session"`
	RubricSHA256                  string                     `json:"rubric_sha256"`
	AdapterPolicySHA256           string                     `json:"adapter_policy_sha256"`
	Cases                         []PhaseHReplayCaseManifest `json:"cases"`
}

type PhaseHAdapterPolicy struct {
	SchemaVersion string                `json:"schema_version"`
	Adapters      []H5AdapterDescriptor `json:"adapters"`
	Slots         map[string][]string   `json:"slots"`
}

type PhaseHReplayCase struct {
	ID                 string
	Problem            json.RawMessage
	ProblemSHA256      string
	ReferenceSet       json.RawMessage
	ReferenceSetSHA256 string
	Arms               []baseline.ArmResult
	ArmBytes           map[baseline.Arm][]byte
}

type PhaseHReplayDataset struct {
	Root                string
	Manifest            PhaseHReplayManifest
	ManifestBytes       []byte
	Rubric              json.RawMessage
	RubricSHA256        string
	AdapterPolicy       PhaseHAdapterPolicy
	AdapterPolicyBytes  []byte
	AdapterPolicySHA256 string
	Cases               []PhaseHReplayCase
}

type PhaseHReplayRunRequest struct {
	Dataset  PhaseHReplayDataset
	RunsRoot string
	RunID    string
}

type PhaseHRunManifest struct {
	SchemaVersion        string `json:"schema_version"`
	BenchmarkID          string `json:"benchmark_id"`
	RunID                string `json:"run_id"`
	CreatedAt            string `json:"created_at"`
	ReplayManifestSHA256 string `json:"replay_manifest_sha256"`
	RubricSHA256         string `json:"rubric_sha256"`
	AdapterPolicySHA256  string `json:"adapter_policy_sha256"`
	SourceWorkflowRunID  int64  `json:"source_workflow_run_id"`
	SourceFrozenSHA      string `json:"source_frozen_sha"`
	SourceH8RunID        string `json:"source_h8_run_id"`
	SourceArtifactDigest string `json:"source_h8_artifact_digest"`
}

type PhaseHResultManifest struct {
	SchemaVersion              string             `json:"schema_version"`
	BenchmarkID                string             `json:"benchmark_id"`
	Mode                       string             `json:"mode"`
	RunID                      string             `json:"run_id"`
	ReplayCaseCount            int                `json:"replay_case_count"`
	ExpectedSuccessfulCalls    int                `json:"expected_successful_invocations"`
	BatchSummarySHA256         string             `json:"batch_summary_sha256"`
	AdapterSummarySHA256       string             `json:"adapter_summary_sha256"`
	ReplayManifestSHA256       string             `json:"replay_manifest_sha256"`
	RubricSHA256               string             `json:"rubric_sha256"`
	AdapterPolicySHA256        string             `json:"adapter_policy_sha256"`
	Outcome                    PhaseHOutcome      `json:"outcome"`
	ValueSummary               PhaseHValueSummary `json:"value_summary"`
	EffectiveProviderDiversity int                `json:"effective_provider_diversity"`
	TotalAvailabilityFailovers int                `json:"total_availability_failovers"`
	HumanBrokerInvocations     int                `json:"human_broker_invocations"`
	SourceWorkflowRunID        int64              `json:"source_workflow_run_id"`
	SourceH8RunID              string             `json:"source_h8_run_id"`
	SourceH8ArtifactDigest     string             `json:"source_h8_artifact_digest"`
}

func LoadPhaseHReplay(root string) (PhaseHReplayDataset, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "." || strings.TrimSpace(root) == "" {
		return PhaseHReplayDataset{}, fmt.Errorf("PhaseH replay dataset root is required")
	}
	if err := requireRealDirectory(root); err != nil {
		return PhaseHReplayDataset{}, fmt.Errorf("validate PhaseH replay dataset root: %w", err)
	}
	manifestBytes, err := readDatasetFile(root, "manifest.json")
	if err != nil {
		return PhaseHReplayDataset{}, err
	}
	var manifest PhaseHReplayManifest
	if err := decodeStrict("manifest.json", manifestBytes, &manifest); err != nil {
		return PhaseHReplayDataset{}, err
	}
	if err := validatePhaseHReplayManifest(manifest); err != nil {
		return PhaseHReplayDataset{}, err
	}

	rubric, err := readDatasetFile(root, "rubric.json")
	if err != nil {
		return PhaseHReplayDataset{}, err
	}
	if err := verifyDigest("PhaseH rubric", rubric, manifest.RubricSHA256); err != nil {
		return PhaseHReplayDataset{}, err
	}
	if !json.Valid(rubric) {
		return PhaseHReplayDataset{}, fmt.Errorf("PhaseH rubric must be valid JSON")
	}

	policyBytes, err := readDatasetFile(root, "adapter-policy.json")
	if err != nil {
		return PhaseHReplayDataset{}, err
	}
	if err := verifyDigest("PhaseH adapter policy", policyBytes, manifest.AdapterPolicySHA256); err != nil {
		return PhaseHReplayDataset{}, err
	}
	var policy PhaseHAdapterPolicy
	if err := decodeStrict("adapter-policy.json", policyBytes, &policy); err != nil {
		return PhaseHReplayDataset{}, err
	}
	if err := validatePhaseHAdapterPolicy(policy); err != nil {
		return PhaseHReplayDataset{}, err
	}

	cases := make([]PhaseHReplayCase, 0, PhaseHReplayCaseCount)
	for index, cm := range manifest.Cases {
		if cm.ID != phaseHReplayCaseIDs[index] {
			return PhaseHReplayDataset{}, fmt.Errorf("PhaseH case %d id %q, want %q", index+1, cm.ID, phaseHReplayCaseIDs[index])
		}
		base := filepath.Join("cases", cm.ID)
		problem, err := readDatasetFile(root, filepath.Join(base, "problem.json"))
		if err != nil {
			return PhaseHReplayDataset{}, err
		}
		if err := verifyDigest(cm.ID+" problem", problem, cm.ProblemSHA256); err != nil {
			return PhaseHReplayDataset{}, err
		}
		if !json.Valid(problem) {
			return PhaseHReplayDataset{}, fmt.Errorf("PhaseH case %q problem must be valid JSON", cm.ID)
		}
		referenceSet, err := readDatasetFile(root, filepath.Join(base, "reference-set.json"))
		if err != nil {
			return PhaseHReplayDataset{}, err
		}
		if err := verifyDigest(cm.ID+" reference set", referenceSet, cm.ReferenceSetSHA256); err != nil {
			return PhaseHReplayDataset{}, err
		}
		if !json.Valid(referenceSet) {
			return PhaseHReplayDataset{}, fmt.Errorf("PhaseH case %q reference set must be valid JSON", cm.ID)
		}

		arms := make([]baseline.ArmResult, 0, len(phaseHReplayArms))
		armBytes := make(map[baseline.Arm][]byte, len(phaseHReplayArms))
		for armIndex, arm := range phaseHReplayArms {
			am := cm.Arms[armIndex]
			if am.Arm != string(arm) {
				return PhaseHReplayDataset{}, fmt.Errorf("PhaseH case %q arm %d label %q, want %q", cm.ID, armIndex+1, am.Arm, arm)
			}
			raw, err := readDatasetFile(root, filepath.Join(base, "arm-"+string(arm)+".json"))
			if err != nil {
				return PhaseHReplayDataset{}, err
			}
			if err := verifyDigest(cm.ID+" arm "+string(arm), raw, am.SHA256); err != nil {
				return PhaseHReplayDataset{}, err
			}
			var result baseline.ArmResult
			if err := decodeStrict(cm.ID+" arm "+string(arm), raw, &result); err != nil {
				return PhaseHReplayDataset{}, err
			}
			if result.Arm != arm {
				return PhaseHReplayDataset{}, fmt.Errorf("PhaseH case %q replay arm file says %q, want %q", cm.ID, result.Arm, arm)
			}
			if result.InvocationCount <= 0 {
				return PhaseHReplayDataset{}, fmt.Errorf("PhaseH case %q arm %s has invalid source invocation count %d", cm.ID, arm, result.InvocationCount)
			}
			if _, err := evalharness.NormalizeCandidate(result); err != nil {
				return PhaseHReplayDataset{}, fmt.Errorf("PhaseH case %q arm %s candidate: %w", cm.ID, arm, err)
			}
			arms = append(arms, result)
			armBytes[arm] = append([]byte(nil), raw...)
		}
		cases = append(cases, PhaseHReplayCase{ID: cm.ID, Problem: append(json.RawMessage(nil), problem...), ProblemSHA256: strings.ToLower(cm.ProblemSHA256), ReferenceSet: append(json.RawMessage(nil), referenceSet...), ReferenceSetSHA256: strings.ToLower(cm.ReferenceSetSHA256), Arms: arms, ArmBytes: armBytes})
	}
	return PhaseHReplayDataset{Root: root, Manifest: manifest, ManifestBytes: append([]byte(nil), manifestBytes...), Rubric: append(json.RawMessage(nil), rubric...), RubricSHA256: strings.ToLower(manifest.RubricSHA256), AdapterPolicy: policy, AdapterPolicyBytes: append([]byte(nil), policyBytes...), AdapterPolicySHA256: strings.ToLower(manifest.AdapterPolicySHA256), Cases: cases}, nil
}

func validatePhaseHReplayManifest(m PhaseHReplayManifest) error {
	if m.SchemaVersion != PhaseHReplayDatasetSchemaVersion {
		return fmt.Errorf("PhaseH manifest schema_version %q, want %q", m.SchemaVersion, PhaseHReplayDatasetSchemaVersion)
	}
	if m.BenchmarkID != PhaseHBenchmarkID || m.Mode != PhaseHReplayMode {
		return fmt.Errorf("PhaseH manifest benchmark/mode mismatch")
	}
	if m.SourceWorkflowRunID != PhaseHSourceWorkflowRunID || m.SourceFrozenSHA != PhaseHSourceFrozenSHA || m.SourceH8RunID != PhaseHSourceH8RunID || m.SourceH8ArtifactID != PhaseHSourceH8ArtifactID || m.SourceH8ArtifactDigest != PhaseHSourceH8ArtifactDigest {
		return fmt.Errorf("phase H manifest source provenance mismatch")
	}
	if m.ExpectedSuccessfulInvocations != PhaseHExpectedSuccessfulInvocations || !m.RequireCurrentSession || m.RequireFreshSession {
		return fmt.Errorf("phase H manifest invocation/session policy mismatch")
	}
	if len(m.Cases) != PhaseHReplayCaseCount {
		return fmt.Errorf("PhaseH manifest cases count %d, want %d", len(m.Cases), PhaseHReplayCaseCount)
	}
	for i, c := range m.Cases {
		if !safeDatasetID(c.ID) || c.ID != phaseHReplayCaseIDs[i] {
			return fmt.Errorf("PhaseH manifest unexpected case %q", c.ID)
		}
		if len(c.Arms) != len(phaseHReplayArms) {
			return fmt.Errorf("PhaseH case %q arms count %d, want %d", c.ID, len(c.Arms), len(phaseHReplayArms))
		}
		if len(c.ProblemSHA256) != 64 || len(c.ReferenceSetSHA256) != 64 {
			return fmt.Errorf("PhaseH case %q hashes must be SHA-256", c.ID)
		}
		for j, a := range c.Arms {
			if a.Arm != string(phaseHReplayArms[j]) || len(a.SHA256) != 64 {
				return fmt.Errorf("PhaseH case %q invalid arm manifest at %d", c.ID, j+1)
			}
		}
	}
	return nil
}

func validatePhaseHAdapterPolicy(policy PhaseHAdapterPolicy) error {
	if policy.SchemaVersion != PhaseHAdapterPolicySchemaVersion {
		return fmt.Errorf("PhaseH adapter policy schema_version %q, want %q", policy.SchemaVersion, PhaseHAdapterPolicySchemaVersion)
	}
	if len(policy.Adapters) != 1 {
		return fmt.Errorf("PhaseH adapter policy must define exactly one adapter")
	}
	a := policy.Adapters[0]
	if a.ID != PhaseHHumanAdapterID || a.ProviderFamily != "chatgpt" || a.Transport != "human-chatgpt-session" || a.AuthClass != "chatgpt-subscription" || a.Interaction != "human-broker" || strings.TrimSpace(a.Model) != "" {
		return fmt.Errorf("PhaseH adapter policy must use only the ChatGPT web human broker")
	}
	if len(policy.Slots) != 2 {
		return fmt.Errorf("PhaseH adapter policy slots count %d, want 2", len(policy.Slots))
	}
	for _, slot := range []string{"eval-judge-1", "eval-judge-2"} {
		chain, ok := policy.Slots[slot]
		if !ok || len(chain) != 1 || chain[0] != PhaseHHumanAdapterID {
			return fmt.Errorf("PhaseH slot %q must be exactly [%q]", slot, PhaseHHumanAdapterID)
		}
	}
	return nil
}
