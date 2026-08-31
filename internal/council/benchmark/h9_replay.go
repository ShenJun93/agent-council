package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/baseline"
	"github.com/ShenJun93/agent-council/internal/council/evalharness"
)

const (
	H9BenchmarkID                   = "h9"
	H9ReplayDatasetSchemaVersion    = "council.h9-replay-dataset.v0"
	H9AdapterPolicySchemaVersion    = "council.h9-adapter-policy.v0"
	H9RunSchemaVersion              = "council.h9-run.v0"
	H9ResultSchemaVersion           = "council.h9-result.v0"
	H9ReplayMode                    = "focused_replay_regression"
	H9HumanAdapterID                = "human-chatgpt-session"
	H9ReplayCaseCount               = 2
	H9ExpectedSuccessfulInvocations = H9ReplayCaseCount * 6 * 2
	H9SourceWorkflowRunID           = 33349114073
	H9SourceFrozenSHA               = "8d13f0a82758f5ea6286409d55123b97929dbce4"
	H9SourceH8RunID                 = "h8-20260831T015809Z-e8ab51986046"
	H9SourceH8ArtifactDigest        = "sha256:c37cbab9cb4d1a8c5b4f6cc2535268c681456d2b6380587fb178396ae9dedfd6"
)

var H9RiskPolicy = H8RiskPolicy

var h9ReplayCaseIDs = []string{"tech-01-db-cutover", "tech-03-cache-stampede"}
var h9ReplayArms = []baseline.Arm{
	baseline.ArmAClaudeSingle,
	baseline.ArmBCodexSingle,
	baseline.ArmCClaudeSelfReview,
	baseline.ArmDCodexSelfReview,
	baseline.ArmEFullInfo,
	baseline.ArmFBlindCouncil,
}

type H9ReplayArmManifest struct {
	Arm    string `json:"arm"`
	SHA256 string `json:"sha256"`
}

type H9ReplayCaseManifest struct {
	ID                 string                `json:"id"`
	ProblemSHA256      string                `json:"problem_sha256"`
	ReferenceSetSHA256 string                `json:"reference_set_sha256"`
	Arms               []H9ReplayArmManifest `json:"arms"`
}

type H9ReplayManifest struct {
	SchemaVersion          string                 `json:"schema_version"`
	BenchmarkID            string                 `json:"benchmark_id"`
	Mode                   string                 `json:"mode"`
	SourceWorkflowRunID    int64                  `json:"source_workflow_run_id"`
	SourceFrozenSHA        string                 `json:"source_frozen_sha"`
	SourceH8RunID          string                 `json:"source_h8_run_id"`
	SourceH8ArtifactDigest string                 `json:"source_h8_artifact_digest"`
	RubricSHA256           string                 `json:"rubric_sha256"`
	AdapterPolicySHA256    string                 `json:"adapter_policy_sha256"`
	Cases                  []H9ReplayCaseManifest `json:"cases"`
}

type H9AdapterPolicy struct {
	SchemaVersion string                `json:"schema_version"`
	Adapters      []H5AdapterDescriptor `json:"adapters"`
	Slots         map[string][]string   `json:"slots"`
}

type H9ReplayCase struct {
	ID                 string
	Problem            json.RawMessage
	ProblemSHA256      string
	ReferenceSet       json.RawMessage
	ReferenceSetSHA256 string
	Arms               []baseline.ArmResult
	ArmBytes           map[baseline.Arm][]byte
}

type H9ReplayDataset struct {
	Root                string
	Manifest            H9ReplayManifest
	ManifestBytes       []byte
	Rubric              json.RawMessage
	RubricSHA256        string
	AdapterPolicy       H9AdapterPolicy
	AdapterPolicyBytes  []byte
	AdapterPolicySHA256 string
	Cases               []H9ReplayCase
}

type H9ReplayRunRequest struct {
	Dataset  H9ReplayDataset
	RunsRoot string
	RunID    string
}

type H9RunManifest struct {
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

type H9ResultManifest struct {
	SchemaVersion              string `json:"schema_version"`
	BenchmarkID                string `json:"benchmark_id"`
	Mode                       string `json:"mode"`
	RunID                      string `json:"run_id"`
	ReplayCaseCount            int    `json:"replay_case_count"`
	ExpectedSuccessfulCalls    int    `json:"expected_successful_invocations"`
	BatchSummarySHA256         string `json:"batch_summary_sha256"`
	AdapterSummarySHA256       string `json:"adapter_summary_sha256"`
	EffectiveProviderDiversity int    `json:"effective_provider_diversity"`
	TotalAvailabilityFailovers int    `json:"total_availability_failovers"`
	HumanBrokerInvocations     int    `json:"human_broker_invocations"`
	SourceWorkflowRunID        int64  `json:"source_workflow_run_id"`
	SourceH8RunID              string `json:"source_h8_run_id"`
	SourceArtifactDigest       string `json:"source_h8_artifact_digest"`
}

func LoadH9Replay(root string) (H9ReplayDataset, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "." || strings.TrimSpace(root) == "" {
		return H9ReplayDataset{}, fmt.Errorf("H9 replay dataset root is required")
	}
	if err := requireRealDirectory(root); err != nil {
		return H9ReplayDataset{}, fmt.Errorf("validate H9 replay dataset root: %w", err)
	}
	manifestBytes, err := readDatasetFile(root, "manifest.json")
	if err != nil {
		return H9ReplayDataset{}, err
	}
	var manifest H9ReplayManifest
	if err := decodeStrict("manifest.json", manifestBytes, &manifest); err != nil {
		return H9ReplayDataset{}, err
	}
	if err := validateH9ReplayManifest(manifest); err != nil {
		return H9ReplayDataset{}, err
	}

	rubric, err := readDatasetFile(root, "rubric.json")
	if err != nil {
		return H9ReplayDataset{}, err
	}
	if err := verifyDigest("H9 rubric", rubric, manifest.RubricSHA256); err != nil {
		return H9ReplayDataset{}, err
	}
	if !json.Valid(rubric) {
		return H9ReplayDataset{}, fmt.Errorf("H9 rubric must be valid JSON")
	}

	policyBytes, err := readDatasetFile(root, "adapter-policy.json")
	if err != nil {
		return H9ReplayDataset{}, err
	}
	if err := verifyDigest("H9 adapter policy", policyBytes, manifest.AdapterPolicySHA256); err != nil {
		return H9ReplayDataset{}, err
	}
	var policy H9AdapterPolicy
	if err := decodeStrict("adapter-policy.json", policyBytes, &policy); err != nil {
		return H9ReplayDataset{}, err
	}
	if err := validateH9AdapterPolicy(policy); err != nil {
		return H9ReplayDataset{}, err
	}

	cases := make([]H9ReplayCase, 0, H9ReplayCaseCount)
	for index, cm := range manifest.Cases {
		if cm.ID != h9ReplayCaseIDs[index] {
			return H9ReplayDataset{}, fmt.Errorf("H9 case %d id %q, want %q", index+1, cm.ID, h9ReplayCaseIDs[index])
		}
		base := filepath.Join("cases", cm.ID)
		problem, err := readDatasetFile(root, filepath.Join(base, "problem.json"))
		if err != nil {
			return H9ReplayDataset{}, err
		}
		if err := verifyDigest(cm.ID+" problem", problem, cm.ProblemSHA256); err != nil {
			return H9ReplayDataset{}, err
		}
		if !json.Valid(problem) {
			return H9ReplayDataset{}, fmt.Errorf("H9 case %q problem must be valid JSON", cm.ID)
		}
		referenceSet, err := readDatasetFile(root, filepath.Join(base, "reference-set.json"))
		if err != nil {
			return H9ReplayDataset{}, err
		}
		if err := verifyDigest(cm.ID+" reference set", referenceSet, cm.ReferenceSetSHA256); err != nil {
			return H9ReplayDataset{}, err
		}
		if !json.Valid(referenceSet) {
			return H9ReplayDataset{}, fmt.Errorf("H9 case %q reference set must be valid JSON", cm.ID)
		}

		arms := make([]baseline.ArmResult, 0, len(h9ReplayArms))
		armBytes := make(map[baseline.Arm][]byte, len(h9ReplayArms))
		for armIndex, arm := range h9ReplayArms {
			am := cm.Arms[armIndex]
			if am.Arm != string(arm) {
				return H9ReplayDataset{}, fmt.Errorf("H9 case %q arm %d label %q, want %q", cm.ID, armIndex+1, am.Arm, arm)
			}
			raw, err := readDatasetFile(root, filepath.Join(base, "arm-"+string(arm)+".json"))
			if err != nil {
				return H9ReplayDataset{}, err
			}
			if err := verifyDigest(cm.ID+" arm "+string(arm), raw, am.SHA256); err != nil {
				return H9ReplayDataset{}, err
			}
			var result baseline.ArmResult
			if err := decodeStrict(cm.ID+" arm "+string(arm), raw, &result); err != nil {
				return H9ReplayDataset{}, err
			}
			if result.Arm != arm {
				return H9ReplayDataset{}, fmt.Errorf("H9 case %q replay arm file says %q, want %q", cm.ID, result.Arm, arm)
			}
			if result.InvocationCount <= 0 {
				return H9ReplayDataset{}, fmt.Errorf("H9 case %q arm %s has invalid source invocation count %d", cm.ID, arm, result.InvocationCount)
			}
			if _, err := evalharness.NormalizeCandidate(result); err != nil {
				return H9ReplayDataset{}, fmt.Errorf("H9 case %q arm %s candidate: %w", cm.ID, arm, err)
			}
			arms = append(arms, result)
			armBytes[arm] = append([]byte(nil), raw...)
		}
		cases = append(cases, H9ReplayCase{ID: cm.ID, Problem: append(json.RawMessage(nil), problem...), ProblemSHA256: strings.ToLower(cm.ProblemSHA256), ReferenceSet: append(json.RawMessage(nil), referenceSet...), ReferenceSetSHA256: strings.ToLower(cm.ReferenceSetSHA256), Arms: arms, ArmBytes: armBytes})
	}
	return H9ReplayDataset{Root: root, Manifest: manifest, ManifestBytes: append([]byte(nil), manifestBytes...), Rubric: append(json.RawMessage(nil), rubric...), RubricSHA256: strings.ToLower(manifest.RubricSHA256), AdapterPolicy: policy, AdapterPolicyBytes: append([]byte(nil), policyBytes...), AdapterPolicySHA256: strings.ToLower(manifest.AdapterPolicySHA256), Cases: cases}, nil
}

func validateH9ReplayManifest(m H9ReplayManifest) error {
	if m.SchemaVersion != H9ReplayDatasetSchemaVersion {
		return fmt.Errorf("H9 manifest schema_version %q, want %q", m.SchemaVersion, H9ReplayDatasetSchemaVersion)
	}
	if m.BenchmarkID != H9BenchmarkID || m.Mode != H9ReplayMode {
		return fmt.Errorf("H9 manifest benchmark/mode mismatch")
	}
	if m.SourceWorkflowRunID != H9SourceWorkflowRunID || m.SourceFrozenSHA != H9SourceFrozenSHA || m.SourceH8RunID != H9SourceH8RunID || m.SourceH8ArtifactDigest != H9SourceH8ArtifactDigest {
		return fmt.Errorf("H9 manifest source provenance mismatch")
	}
	if len(m.Cases) != H9ReplayCaseCount {
		return fmt.Errorf("H9 manifest cases count %d, want %d", len(m.Cases), H9ReplayCaseCount)
	}
	for i, c := range m.Cases {
		if !safeDatasetID(c.ID) || c.ID != h9ReplayCaseIDs[i] {
			return fmt.Errorf("H9 manifest unexpected case %q", c.ID)
		}
		if len(c.Arms) != len(h9ReplayArms) {
			return fmt.Errorf("H9 case %q arms count %d, want %d", c.ID, len(c.Arms), len(h9ReplayArms))
		}
		if len(c.ProblemSHA256) != 64 || len(c.ReferenceSetSHA256) != 64 {
			return fmt.Errorf("H9 case %q hashes must be SHA-256", c.ID)
		}
		for j, a := range c.Arms {
			if a.Arm != string(h9ReplayArms[j]) || len(a.SHA256) != 64 {
				return fmt.Errorf("H9 case %q invalid arm manifest at %d", c.ID, j+1)
			}
		}
	}
	return nil
}

func validateH9AdapterPolicy(policy H9AdapterPolicy) error {
	if policy.SchemaVersion != H9AdapterPolicySchemaVersion {
		return fmt.Errorf("H9 adapter policy schema_version %q, want %q", policy.SchemaVersion, H9AdapterPolicySchemaVersion)
	}
	if len(policy.Adapters) != 1 {
		return fmt.Errorf("H9 adapter policy must define exactly one adapter")
	}
	a := policy.Adapters[0]
	if a.ID != H9HumanAdapterID || a.ProviderFamily != "chatgpt" || a.Transport != "human-chatgpt-session" || a.AuthClass != "chatgpt-subscription" || a.Interaction != "human-broker" || strings.TrimSpace(a.Model) != "" {
		return fmt.Errorf("H9 adapter policy must use only the ChatGPT web human broker")
	}
	if len(policy.Slots) != 2 {
		return fmt.Errorf("H9 adapter policy slots count %d, want 2", len(policy.Slots))
	}
	for _, slot := range []string{"eval-judge-1", "eval-judge-2"} {
		chain, ok := policy.Slots[slot]
		if !ok || len(chain) != 1 || chain[0] != H9HumanAdapterID {
			return fmt.Errorf("H9 slot %q must be exactly [%q]", slot, H9HumanAdapterID)
		}
	}
	return nil
}

func CreateH9ReplayRun(ctx context.Context, runsRoot, runID string, dataset H9ReplayDataset, now time.Time) (string, H9RunManifest, error) {
	if err := ctx.Err(); err != nil {
		return "", H9RunManifest{}, err
	}
	if strings.TrimSpace(runsRoot) == "" {
		return "", H9RunManifest{}, fmt.Errorf("runs root is required")
	}
	if !safeDatasetID(runID) {
		return "", H9RunManifest{}, fmt.Errorf("invalid H9 run id %q", runID)
	}
	if len(dataset.Cases) != H9ReplayCaseCount || dataset.Manifest.BenchmarkID != H9BenchmarkID {
		return "", H9RunManifest{}, fmt.Errorf("dataset is not the frozen H9 replay dataset")
	}
	if err := os.MkdirAll(runsRoot, 0o750); err != nil {
		return "", H9RunManifest{}, fmt.Errorf("create H9 runs root: %w", err)
	}
	if err := requireRealDirectory(runsRoot); err != nil {
		return "", H9RunManifest{}, fmt.Errorf("validate H9 runs root: %w", err)
	}
	runRoot := filepath.Join(runsRoot, runID)
	if err := os.Mkdir(runRoot, 0o750); err != nil {
		return "", H9RunManifest{}, fmt.Errorf("create H9 run directory: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(runRoot)
		}
	}()
	manifest := H9RunManifest{SchemaVersion: H9RunSchemaVersion, BenchmarkID: H9BenchmarkID, RunID: runID, CreatedAt: now.UTC().Format(time.RFC3339Nano), ReplayManifestSHA256: sha256Hex(dataset.ManifestBytes), RubricSHA256: dataset.RubricSHA256, AdapterPolicySHA256: dataset.AdapterPolicySHA256, SourceWorkflowRunID: H9SourceWorkflowRunID, SourceFrozenSHA: H9SourceFrozenSHA, SourceH8RunID: H9SourceH8RunID, SourceArtifactDigest: H9SourceH8ArtifactDigest}
	runBytes, err := json.Marshal(manifest)
	if err != nil {
		return "", H9RunManifest{}, err
	}
	specs := []h1FileSpec{{rel: "h9-run.json", data: runBytes}, {rel: filepath.Join("inputs", "replay-manifest.json"), data: dataset.ManifestBytes}, {rel: filepath.Join("inputs", "rubric.json"), data: dataset.Rubric}, {rel: filepath.Join("inputs", "adapter-policy.json"), data: dataset.AdapterPolicyBytes}}
	for _, c := range dataset.Cases {
		base := filepath.Join("inputs", "cases", c.ID)
		specs = append(specs, h1FileSpec{rel: filepath.Join(base, "problem.json"), data: c.Problem}, h1FileSpec{rel: filepath.Join(base, "reference-set.json"), data: c.ReferenceSet})
		for _, arm := range h9ReplayArms {
			specs = append(specs, h1FileSpec{rel: filepath.Join(base, "arm-"+string(arm)+".json"), data: c.ArmBytes[arm]})
		}
	}
	if err := writeH1Specs(ctx, runRoot, specs); err != nil {
		return "", H9RunManifest{}, fmt.Errorf("freeze H9 replay inputs: %w", err)
	}
	committed = true
	return runRoot, manifest, nil
}

type H9ReplayRunner struct {
	Evaluator             EvalExecutor
	Now                   func() time.Time
	CollectAdapterSummary func(context.Context, string, string) (H5AdapterSummary, error)
}

func (r H9ReplayRunner) Run(ctx context.Context, req H9ReplayRunRequest) (RunResult, error) {
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	if r.Evaluator == nil {
		return RunResult{}, fmt.Errorf("H9 evaluator is required")
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	runRoot, _, err := CreateH9ReplayRun(ctx, req.RunsRoot, req.RunID, req.Dataset, now())
	if err != nil {
		return RunResult{}, fmt.Errorf("create H9 run: %w", err)
	}
	problems := make([]evalharness.ProblemResult, 0, H9ReplayCaseCount)
	for _, c := range req.Dataset.Cases {
		result, err := r.Evaluator.EvaluateProblem(ctx, evalharness.ProblemRequest{ProblemID: c.ID, RunID: req.RunID, RunRoot: runRoot, NormalizedProblem: c.Problem, Rubric: req.Dataset.Rubric, RubricSHA256: req.Dataset.RubricSHA256, ReferenceSet: c.ReferenceSet, ReferenceSetSHA256: c.ReferenceSetSHA256, Arms: c.Arms, RiskPolicy: H9RiskPolicy})
		if err != nil {
			return RunResult{}, fmt.Errorf("H9 replay case %q evaluation: %w", c.ID, err)
		}
		if result.ProblemID != c.ID || result.RiskPolicy != H9RiskPolicy {
			return RunResult{}, fmt.Errorf("H9 replay case %q returned invalid evaluation identity/policy", c.ID)
		}
		problems = append(problems, result)
	}
	if len(problems) != H9ReplayCaseCount {
		return RunResult{}, fmt.Errorf("H9 evaluated %d replay cases, want %d", len(problems), H9ReplayCaseCount)
	}
	summary, err := evalharness.SummarizeBatch(problems, H9RiskPolicy)
	if err != nil {
		return RunResult{}, fmt.Errorf("summarize H9 evaluation: %w", err)
	}
	if err := evalharness.WriteEvaluation(ctx, evalharness.WriteRequest{Root: runRoot, Policy: H9RiskPolicy, Problems: problems, Summary: summary}); err != nil {
		return RunResult{}, fmt.Errorf("persist H9 evaluation: %w", err)
	}
	collector := CollectH5AdapterSummary
	if r.CollectAdapterSummary != nil {
		collector = r.CollectAdapterSummary
	}
	adapterSummary, err := collector(ctx, runRoot, req.RunID)
	if err != nil {
		return RunResult{}, fmt.Errorf("collect H9 adapter summary: %w", err)
	}
	if err := validateH9AdapterSummary(adapterSummary); err != nil {
		return RunResult{}, err
	}
	adapterHash, err := WriteH5AdapterSummary(ctx, runRoot, adapterSummary)
	if err != nil {
		return RunResult{}, fmt.Errorf("persist H9 adapter summary: %w", err)
	}
	if _, err := WriteH9FinalResult(ctx, runRoot, req.RunID, summary, adapterSummary, adapterHash); err != nil {
		return RunResult{}, fmt.Errorf("persist H9 final result: %w", err)
	}
	return RunResult{RunID: req.RunID, RunDir: runRoot, Summary: summary, AdapterSummary: &adapterSummary}, nil
}

func validateH9AdapterSummary(s H5AdapterSummary) error {
	if s.SchemaVersion != H5AdapterSummarySchemaVersion {
		return fmt.Errorf("H9 adapter summary schema %q is invalid", s.SchemaVersion)
	}
	if s.SuccessfulInvocations != H9ExpectedSuccessfulInvocations {
		return fmt.Errorf("H9 successful invocations %d, want %d", s.SuccessfulInvocations, H9ExpectedSuccessfulInvocations)
	}
	if s.HumanBrokerInvocations != H9ExpectedSuccessfulInvocations {
		return fmt.Errorf("H9 human broker invocations %d, want %d", s.HumanBrokerInvocations, H9ExpectedSuccessfulInvocations)
	}
	if s.EffectiveAdapterDiversity != 1 || s.EffectiveProviderDiversity != 1 {
		return fmt.Errorf("H9 effective adapter/provider diversity must be 1/1")
	}
	if s.TotalAvailabilityFailovers != 0 || s.AvailabilityFailures != 0 {
		return fmt.Errorf("H9 must not contain availability failover/failure evidence")
	}
	return nil
}

func WriteH9FinalResult(ctx context.Context, runRoot, runID string, summary evalharness.BatchSummary, adapterSummary H5AdapterSummary, adapterSummarySHA256 string) (H9ResultManifest, error) {
	if err := ctx.Err(); err != nil {
		return H9ResultManifest{}, err
	}
	if err := requireRealDirectory(runRoot); err != nil {
		return H9ResultManifest{}, err
	}
	summaryBytes, err := json.Marshal(summary)
	if err != nil {
		return H9ResultManifest{}, err
	}
	summaryHash := sha256Hex(summaryBytes)
	batchPath := filepath.Join(runRoot, "eval", "batch-summary.json")
	if err := requireContainedRegularFile(runRoot, batchPath); err != nil {
		return H9ResultManifest{}, fmt.Errorf("validate H9 batch summary: %w", err)
	}
	stored, err := os.ReadFile(batchPath)
	if err != nil {
		return H9ResultManifest{}, err
	}
	if sha256Hex(stored) != summaryHash {
		return H9ResultManifest{}, fmt.Errorf("H9 batch summary hash mismatch")
	}
	adapterPath := filepath.Join(runRoot, "adapter-summary.json")
	if err := requireContainedRegularFile(runRoot, adapterPath); err != nil {
		return H9ResultManifest{}, fmt.Errorf("validate H9 adapter summary: %w", err)
	}
	adapterBytes, err := os.ReadFile(adapterPath)
	if err != nil {
		return H9ResultManifest{}, err
	}
	if sha256Hex(adapterBytes) != strings.ToLower(adapterSummarySHA256) {
		return H9ResultManifest{}, fmt.Errorf("H9 adapter summary hash mismatch")
	}
	manifest := H9ResultManifest{SchemaVersion: H9ResultSchemaVersion, BenchmarkID: H9BenchmarkID, Mode: H9ReplayMode, RunID: runID, ReplayCaseCount: summary.ProblemCount, ExpectedSuccessfulCalls: H9ExpectedSuccessfulInvocations, BatchSummarySHA256: summaryHash, AdapterSummarySHA256: strings.ToLower(adapterSummarySHA256), EffectiveProviderDiversity: adapterSummary.EffectiveProviderDiversity, TotalAvailabilityFailovers: adapterSummary.TotalAvailabilityFailovers, HumanBrokerInvocations: adapterSummary.HumanBrokerInvocations, SourceWorkflowRunID: H9SourceWorkflowRunID, SourceH8RunID: H9SourceH8RunID, SourceArtifactDigest: H9SourceH8ArtifactDigest}
	data, err := json.Marshal(manifest)
	if err != nil {
		return H9ResultManifest{}, err
	}
	if err := writeH1Specs(ctx, runRoot, []h1FileSpec{{rel: "h9-result.json", data: data}}); err != nil {
		return H9ResultManifest{}, fmt.Errorf("write H9 result: %w", err)
	}
	return manifest, nil
}
