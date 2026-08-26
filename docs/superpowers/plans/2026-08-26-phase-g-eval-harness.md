# Phase G Evaluation Harness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Score frozen Phase F arms A–F with the same fixed fresh Claude+Codex judge pair per problem, then produce deterministic quality, variance, evaluator-spread, and Council tail-risk reports.

**Architecture:** Add a focused `internal/council/evalharness` package. Pure functions normalize Phase F candidates and compute statistics; a harness layer validates frozen hashes/policy, materializes masked judge contexts through the existing Visibility Firewall, invokes exactly two fixed judge runtimes for each arm, and aggregates results; a small immutable writer persists evaluation outputs under `eval/` without changing Phase E artifact storage or protocol execution.

**Tech Stack:** Go 1.26.x, standard library JSON/crypto/math/filesystem, existing `baseline`, `protocol`, `runtime.AgentRuntime`, and `visibility.Materialize`.

**Spec:** `docs/superpowers/specs/2026-08-26-phase-g-eval-harness-design.md`

## Global Constraints

- Evaluate the frozen Phase F arms A–F only; do not modify their execution semantics.
- Judge 1 is Claude and Judge 2 is Codex for every arm of a problem; never rotate or substitute providers.
- Every judge invocation is fresh and isolated outside the full run root.
- Judge-visible candidate context contains no arm label or provider provenance added by the harness.
- Verify rubric/reference SHA-256 before any judge invocation.
- `best_single` is the only v0 comparator and equals `max(mean(A), mean(B))`.
- `MaterialWorseDelta` is explicit, positive, and frozen before a benchmark batch.
- Report variance and tail outcomes in addition to means.
- No databases, dashboards, new runtime abstraction, generic statistics framework, or automatic rubric/reference generation.

---

### Task 1: Candidate normalization and frozen evaluation types

**Files:**
- Create: `internal/council/evalharness/types.go`
- Create: `internal/council/evalharness/candidate.go`
- Create: `internal/council/evalharness/candidate_test.go`

**Interfaces:**
- Consumes: `baseline.ArmResult`, `baseline.AnswerArtifact`, `protocol.Result`.
- Produces: `Comparator`, `ComparatorBestSingle`, `RiskPolicy`, `RubricDocument`, `RubricDimension`, `MaskedCandidate`, `JudgeArtifact`, `JudgeScore`, `ArmScore`, `ProblemResult`, `BatchSummary`, and `NormalizeCandidate(result baseline.ArmResult) (MaskedCandidate, error)`.

- [ ] **Step 1: Write failing candidate-normalization tests**

Tests must assert A–D normalize decision/action/reasons/assumptions/risks/citations without the arm label; E/F normalize the protocol decision plus judge reasons/evidence/minority/unresolved/next-validation without provider fields; malformed arm result shapes fail closed.

- [ ] **Step 2: Run RED**

Run: `go test ./internal/council/evalharness -run 'Candidate|RiskPolicy'`
Expected: FAIL because the package/types/functions do not exist.

- [ ] **Step 3: Implement minimal types and normalization**

Use these core shapes:

```go
type Comparator string
const ComparatorBestSingle Comparator = "best_single"

type RiskPolicy struct {
    Comparator Comparator `json:"comparator"`
    MaterialWorseDelta float64 `json:"material_worse_delta"`
}

type RubricDocument struct {
    Dimensions []RubricDimension `json:"dimensions"`
}

type RubricDimension struct {
    ID string `json:"id"`
}

type MaskedCandidate struct {
    Decision string `json:"decision"`
    Action string `json:"action,omitempty"`
    Reasons []string `json:"reasons"`
    Assumptions []string `json:"assumptions"`
    Risks []string `json:"risks"`
    Citations []protocol.EvidenceRef `json:"citations"`
    Evidence []string `json:"evidence"`
    Minority []string `json:"minority"`
    Unresolved []string `json:"unresolved"`
    NextValidations []string `json:"next_validations"`
}
```

Protocol candidates must merge judge-visible reasoning deterministically by judge order and must never embed provider provenance.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/council/evalharness -run 'Candidate|RiskPolicy'`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit message: `feat: add eval candidate normalization`.

### Task 2: Fixed dual-judge evaluation harness

**Files:**
- Create: `internal/council/evalharness/harness.go`
- Create: `internal/council/evalharness/harness_test.go`

**Interfaces:**
- Consumes: Task 1 types, `runtime.AgentRuntime`, `visibility.Materialize`.
- Produces:

```go
type Harness struct {
    Claude runtime.AgentRuntime
    Codex runtime.AgentRuntime
    TempRoot string
}

type ProblemRequest struct {
    ProblemID string
    RunID string
    RunRoot string
    NormalizedProblem json.RawMessage
    Rubric json.RawMessage
    RubricSHA256 string
    ReferenceSet json.RawMessage
    ReferenceSetSHA256 string
    Arms []baseline.ArmResult
    RiskPolicy RiskPolicy
}

func (h Harness) EvaluateProblem(ctx context.Context, req ProblemRequest) (ProblemResult, error)
```

- [ ] **Step 1: Write failing judge-routing/isolation tests**

Use fake Claude/Codex runtimes. Assert exactly 12 calls per problem; each provider receives A–F once; all 12 workdirs are unique and outside `RunRoot`; every prompt contains problem/rubric/reference/candidate but no `arm-A`…`arm-F`, `"arm":"A"`…`"arm":"F"`, or harness-added provider provenance; neither judge sees the other judge output or another arm score.

Add a preflight test that intentionally passes the wrong rubric/reference hash and asserts zero runtime calls.

- [ ] **Step 2: Run RED**

Run: `go test ./internal/council/evalharness -run 'EvaluateProblem|Hash|Judge'`
Expected: FAIL because `Harness.EvaluateProblem` is missing.

- [ ] **Step 3: Implement minimal judge flow**

Validate problem ID, run fields, both runtimes, temp root, six unique A–F arms, risk policy, normalized JSON, rubric schema, rubric/reference hashes. Materialize exactly four judge-visible artifacts: `problem`, `rubric`, `reference-set`, `candidate`. Use `MaskProviderIdentity: true`. Invoke participant `eval-judge-1` with Claude and `eval-judge-2` with Codex in phase `eval-judge` for every arm.

Judge instruction must require strict JSON:

```go
type JudgeArtifact struct {
    OverallScore float64 `json:"overall_score"`
    Dimensions map[string]float64 `json:"dimensions"`
    CitationChecks []protocol.CitationCheck `json:"citation_checks"`
    ReliedOnCitations []string `json:"relied_on_citations"`
    CriticalErrors []string `json:"critical_errors"`
    Strengths []string `json:"strengths"`
    Weaknesses []string `json:"weaknesses"`
    Confidence float64 `json:"confidence"`
}
```

Reject unknown/trailing JSON, overall/dimension scores outside 0–100, confidence outside 0–1, missing/extra rubric dimensions, duplicate/missing citation checks, or any relied-on citation whose check is not `verified`. Record output SHA-256 and timestamps/provider only in non-visible `JudgeScore` bookkeeping.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/council/evalharness -run 'EvaluateProblem|Hash|Judge'`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit message: `feat: add fixed dual-judge eval harness`.

### Task 3: Deterministic aggregation and tail-risk metrics

**Files:**
- Create: `internal/council/evalharness/metrics.go`
- Create: `internal/council/evalharness/metrics_test.go`

**Interfaces:**
- Consumes: `ProblemResult`, `ArmScore`, `RiskPolicy`.
- Produces `SummarizeBatch(problems []ProblemResult, policy RiskPolicy) (BatchSummary, error)` plus deterministic per-arm statistics.

- [ ] **Step 1: Write failing metrics tests**

Test arithmetic mean, population variance, min/max, median, deterministic nearest-rank p10/p90, mean judge spread, best-single selection from A/B, Council delta `F-bestSingle`, and the inclusive material-worse boundary `delta <= -MaterialWorseDelta`. Include at least three problem IDs and assert materially-worse count/rate/IDs and delta variance. Test empty batch and unsupported/zero threshold failure.

- [ ] **Step 2: Run RED**

Run: `go test ./internal/council/evalharness -run 'Summary|Metrics|Tail|Percentile'`
Expected: FAIL because metrics functions are missing.

- [ ] **Step 3: Implement deterministic statistics**

Use population variance `sum((x-mean)^2)/N`. Median is middle value for odd N and midpoint of the two middle values for even N. Nearest-rank percentile uses `ceil(p*N)` clamped to `[1,N]`. Preserve problem IDs in input order for normal results and sort materially-worse IDs lexically for stable reports.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/council/evalharness -run 'Summary|Metrics|Tail|Percentile'`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit message: `feat: add eval distribution and tail metrics`.

### Task 4: Immutable evaluation artifact writer

**Files:**
- Create: `internal/council/evalharness/store.go`
- Create: `internal/council/evalharness/store_test.go`

**Interfaces:**
- Consumes: `ProblemResult`, `BatchSummary`, `RiskPolicy`.
- Produces:

```go
type WriteRequest struct {
    Root string
    Policy RiskPolicy
    Problems []ProblemResult
    Summary BatchSummary
}

func WriteEvaluation(ctx context.Context, req WriteRequest) error
```

- [ ] **Step 1: Write failing storage tests**

Assert immutable files exist at `eval/problems/<problem-id>/arm-A.json` through `arm-F.json`, `problem-summary.json`, `eval/batch-summary.json`, `eval/eval-policy.json`, and `eval/provenance.jsonl`; second write fails; unsafe IDs and symlink/path escape fail; partial failed writes are cleaned up where safe.

- [ ] **Step 2: Run RED**

Run: `go test ./internal/council/evalharness -run 'WriteEvaluation|Store|Immutable|Symlink'`
Expected: FAIL because writer is missing.

- [ ] **Step 3: Implement minimal contained immutable writer**

Use `os.OpenFile(..., O_CREATE|O_EXCL, 0600)`, `filepath.EvalSymlinks` containment checks, JSON encoding, SHA-256, and JSONL provenance. Never overwrite an existing evaluation artifact. Provenance records file path, SHA-256, problem/arm where applicable, and judge-slot/provider/input/output hashes already present in `JudgeScore`; provider provenance is never copied into judge-visible candidate content.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/council/evalharness -run 'WriteEvaluation|Store|Immutable|Symlink'`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit message: `feat: persist immutable eval artifacts`.

### Task 5: Exact-head repository verification and merge gate

**Files:**
- Update: PR description only.

**Interfaces:**
- Consumes: Tasks 1–4.
- Produces: merge-ready Phase G branch.

- [ ] **Step 1: Run full CI verification**

CI must run gofmt check, `go test ./...`, `go vet ./...`, and golangci-lint on the exact head.

- [ ] **Step 2: Confirm exact-head CLA**

`cla` must pass on the same head SHA as `quality`.

- [ ] **Step 3: Review changed-file scope**

Only Phase G spec/plan and `internal/council/evalharness/**` may change unless a test proves a minimal existing-file adjustment is required. No runtime, visibility, protocol, or baseline weakening is allowed.

- [ ] **Step 4: Mark ready and squash merge**

Re-fetch exact PR head and checks immediately before merge. Squash merge with `expected_head_sha`, then verify PR merged and `main` moved to the returned merge SHA.
