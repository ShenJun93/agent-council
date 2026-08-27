# H1 Benchmark Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the frozen 20-case H1 dataset, validate it before any model call, run existing Phase F A–F arms and Phase G evaluation over the batch, and persist auditable immutable H1 artifacts.

**Architecture:** Add a focused `internal/council/benchmark` package with three responsibilities split across files: strict dataset loading/validation, immutable H1-owned artifact writes, and batch orchestration that composes `baseline.Runner` plus `evalharness.Harness`. Extend `agentd council` with `benchmark h1`; do not modify Phase F, Phase G, runtime, protocol, or Visibility Firewall semantics.

**Tech Stack:** Go 1.26.x, standard library JSON/crypto/filesystem packages, existing `baseline`, `evalharness`, `protocol`, `runtime`, and `config` packages.

**Spec:** `docs/superpowers/specs/2026-08-27-h1-benchmark-design.md`

## Global Constraints

- Exactly 20 H1 cases: 10 technical followed by 10 product in the frozen manifest order.
- Every decision-critical fact visible to candidates is inside `problem`; `reference_set` may only corroborate evidence IDs declared by that problem.
- Shared rubric dimensions are exactly `correctness_soundness`, `evidence_use`, `risk_handling`, `actionability`, `calibration`, each weight 1.
- Comparator is exactly `best_single`; `MaterialWorseDelta` is exactly `10.0`.
- `ChallengePolicy.AllowAbbreviated` is exactly `false`; `HighConfidenceThreshold` is `1.0`.
- Odd global case indices use Claude challenger; even indices use Codex; E and F use the same challenger for a case.
- Subscription-only execution remains fail-closed; no metered fallback or provider substitution.
- No modifications to existing Phase F arm behavior or Phase G judge/metric behavior.
- H1-owned artifacts are create-only, contained under the run root, and never overwritten.
- The first real H1 model call happens only after this change is merged to `main` with exact-head CI and CLA green.

---

### Task 1: Strict H1 dataset loader

**Files:**
- Create: `internal/council/benchmark/types.go`
- Create: `internal/council/benchmark/dataset.go`
- Create: `internal/council/benchmark/dataset_test.go`

**Interfaces:**
- Produces: `LoadH1(root string) (Dataset, error)`.
- Produces frozen constants/policies used by later tasks.

Define the public data surface in `types.go`:

```go
package benchmark

import (
    "encoding/json"

    "github.com/ShenJun93/agent-council/internal/council/evalharness"
    "github.com/ShenJun93/agent-council/internal/council/protocol"
    councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

const (
    H1BenchmarkID = "h1"
    H1DatasetSchemaVersion = "council.h1-dataset.v0"
    H1CaseCount = 20
    H1TechnicalCount = 10
    H1ProductCount = 10
)

var H1RiskPolicy = evalharness.RiskPolicy{
    Comparator: evalharness.ComparatorBestSingle,
    MaterialWorseDelta: 10.0,
}

var H1ChallengePolicy = protocol.ChallengePolicy{
    AllowAbbreviated: false,
    HighConfidenceThreshold: 1.0,
}

type Manifest struct {
    SchemaVersion string `json:"schema_version"`
    BenchmarkID string `json:"benchmark_id"`
    CaseCount int `json:"case_count"`
    CategoryCounts map[string]int `json:"category_counts"`
    CaseIDs []string `json:"case_ids"`
    RubricSHA256 string `json:"rubric_sha256"`
    CasesSHA256 string `json:"cases_sha256"`
    Comparator evalharness.Comparator `json:"comparator"`
    MaterialWorseDelta float64 `json:"material_worse_delta"`
}

type Case struct {
    ID string
    Category string
    ChallengerProvider councilruntime.Provider
    Problem json.RawMessage
    ProblemSHA256 string
    ReferenceSet json.RawMessage
    ReferenceSetSHA256 string
}

type Dataset struct {
    Root string
    Manifest Manifest
    ManifestBytes []byte
    Rubric json.RawMessage
    RubricSHA256 string
    CasesBytes []byte
    Cases []Case
}
```

`dataset.go` must strictly decode `manifest.json` and `cases.json` with `DisallowUnknownFields`, compact each case `problem` and `reference_set`, verify SHA-256, validate the exact case roster and category split, validate the frozen challenger schedule, and validate reference evidence IDs against problem evidence IDs.

Use fixed internal schemas for candidate-visible evidence:

```go
type problemDocument struct {
    Title string `json:"title"`
    Decision string `json:"decision"`
    Context []string `json:"context"`
    Constraints []string `json:"constraints"`
    Options []string `json:"options"`
    Evidence []problemEvidence `json:"evidence"`
}

type problemEvidence struct {
    ID string `json:"id"`
    Fact string `json:"fact"`
}

type referenceDocument struct {
    Evidence []referenceEvidence `json:"evidence"`
}

type referenceEvidence struct {
    ID string `json:"id"`
    Claim string `json:"claim"`
    EvaluationNote string `json:"evaluation_note"`
}
```

- [ ] **Step 1: Write failing loader tests**

Test one valid generated 20-case fixture plus mutations for wrong manifest hash, duplicate ID, wrong order, wrong 10/10 split, wrong challenger, unknown field, and a reference evidence ID absent from the problem.

```go
func TestLoadH1RejectsReferenceEvidenceNotVisibleToCandidate(t *testing.T) {
    root := writeValidFixture(t)
    mutateCases(t, root, func(cases *casesDocument) {
        cases.Cases[0].ReferenceSet = json.RawMessage(`{"evidence":[{"id":"hidden","claim":"secret","evaluation_note":"must reject"}]}`)
        cases.Cases[0].ReferenceSetSHA256 = digestCompact(t, cases.Cases[0].ReferenceSet)
    })
    rewriteCasesAndManifestHashes(t, root)

    _, err := LoadH1(root)
    if err == nil || !strings.Contains(err.Error(), "reference evidence") {
        t.Fatalf("expected hidden reference evidence rejection, got %v", err)
    }
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./internal/council/benchmark -run TestLoadH1 -count=1`

Expected: FAIL because package/types/loader do not exist yet.

- [ ] **Step 3: Implement strict loader**

Implement compact hashing with `sha256.Sum256`, safe IDs using only `[A-Za-z0-9._-]`, exact ordered IDs from the spec, exact category counts, exact challenger-by-index checks, and exact `H1RiskPolicy` checks. Reject non-regular dataset files and symlink escapes from `root`.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/council/benchmark -run TestLoadH1 -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/council/benchmark/types.go internal/council/benchmark/dataset.go internal/council/benchmark/dataset_test.go
git commit -m "feat: add frozen H1 dataset loader"
```

---

### Task 2: Author and freeze the 20-case H1 dataset

**Files:**
- Create: `benchmarks/h1/README.md`
- Create: `benchmarks/h1/rubric.json`
- Create: `benchmarks/h1/cases.json`
- Create: `benchmarks/h1/manifest.json`
- Modify: `internal/council/benchmark/dataset_test.go`

**Interfaces:**
- Consumes: `LoadH1` from Task 1.
- Produces: repository-frozen H1 inputs that all later real runs consume.

Use this exact shared rubric structure:

```json
{
  "schema_version": "council.h1-rubric.v0",
  "overall_score_rule": "overall_score is the arithmetic mean of the five equally weighted dimension scores",
  "dimensions": [
    {"id":"correctness_soundness","weight":1,"description":"Sound, internally consistent recommendation that respects supplied facts and hard constraints."},
    {"id":"evidence_use","weight":1,"description":"Uses supplied evidence correctly, distinguishes evidence from assumption, and invents no material facts."},
    {"id":"risk_handling","weight":1,"description":"Identifies material failure modes, downside risk, reversibility, and proportionate mitigations."},
    {"id":"actionability","weight":1,"description":"Makes a concrete executable decision with prioritized next actions or validation where useful."},
    {"id":"calibration","weight":1,"description":"Handles uncertainty, assumptions, confidence, and change conditions appropriately."}
  ]
}
```

Each of the 20 case problems must contain 6–10 evidence facts and at least 2 meaningful hard constraints. Reference sets mirror every problem evidence ID with a verified claim and evaluator note; they must not contain extra evidence IDs.

The case envelope challenger schedule is fixed by global index: Claude for 1,3,...,19 and Codex for 2,4,...,20.

- [ ] **Step 1: Add committed-dataset integration test before data files**

```go
func TestCommittedH1DatasetLoads(t *testing.T) {
    root := filepath.Join("..", "..", "..", "benchmarks", "h1")
    dataset, err := LoadH1(root)
    if err != nil {
        t.Fatal(err)
    }
    if len(dataset.Cases) != 20 {
        t.Fatalf("got %d cases", len(dataset.Cases))
    }
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./internal/council/benchmark -run TestCommittedH1DatasetLoads -count=1`

Expected: FAIL because `benchmarks/h1` does not exist.

- [ ] **Step 3: Author `rubric.json` and all 20 evidence-bounded cases**

Use only synthetic facts. Do not use web-derived current facts, named real customers, private data, or a hidden answer key. Problems ask for a decision and expose all relevant evidence; judges evaluate quality rather than matching a single oracle option.

- [ ] **Step 4: Compute and insert exact hashes**

For every case, compact `problem` and `reference_set` with the same algorithm as `LoadH1`, compute SHA-256, and insert `problem_sha256` / `reference_set_sha256`. Then compute exact file SHA-256 for `rubric.json` and `cases.json` and write `manifest.json` with the exact ordered 20 IDs and frozen policy.

- [ ] **Step 5: Run GREEN and deterministic integrity test**

Run: `go test ./internal/council/benchmark -run 'TestCommittedH1DatasetLoads|TestLoadH1' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add benchmarks/h1 internal/council/benchmark/dataset_test.go
git commit -m "testdata: freeze H1 benchmark dataset"
```

---

### Task 3: Immutable H1 run artifact store

**Files:**
- Create: `internal/council/benchmark/store.go`
- Create: `internal/council/benchmark/store_test.go`

**Interfaces:**
- Consumes: `Dataset` and `baseline.ArmResult`.
- Produces:

```go
type RunManifest struct {
    SchemaVersion string `json:"schema_version"`
    BenchmarkID string `json:"benchmark_id"`
    RunID string `json:"run_id"`
    CreatedAt string `json:"created_at"`
    DatasetManifestSHA256 string `json:"dataset_manifest_sha256"`
    RubricSHA256 string `json:"rubric_sha256"`
    CasesSHA256 string `json:"cases_sha256"`
}

type ResultManifest struct {
    SchemaVersion string `json:"schema_version"`
    BenchmarkID string `json:"benchmark_id"`
    RunID string `json:"run_id"`
    ProblemCount int `json:"problem_count"`
    BatchSummarySHA256 string `json:"batch_summary_sha256"`
}

func CreateRun(ctx context.Context, runsRoot, runID string, dataset Dataset, now time.Time) (string, RunManifest, error)
func WriteBaselineResults(ctx context.Context, runRoot, problemID string, results []baseline.ArmResult) error
func WriteFinalResult(ctx context.Context, runRoot, runID string, summary evalharness.BatchSummary) (ResultManifest, error)
```

`CreateRun` creates `<runsRoot>/<runID>` exclusively, writes `h1-run.json`, freezes manifest/rubric and every compact problem/reference file, and removes the new run directory only if freezing inputs fails before commit. `WriteBaselineResults` requires exactly A–F once each and writes six exclusive JSON files. `WriteFinalResult` verifies `eval/batch-summary.json` exists and hashes to the supplied summary bytes before writing `h1-result.json` exclusively.

- [ ] **Step 1: Write failing store tests**

Cover successful layout, duplicate run rejection, baseline overwrite rejection, unsafe problem ID rejection, symlink escape rejection, wrong A–F set, and final-result rejection before `eval/batch-summary.json` exists.

```go
func TestWriteBaselineResultsRefusesOverwrite(t *testing.T) {
    runRoot := t.TempDir()
    results := sixArmResults()
    if err := WriteBaselineResults(context.Background(), runRoot, "tech-01-db-cutover", results); err != nil {
        t.Fatal(err)
    }
    if err := WriteBaselineResults(context.Background(), runRoot, "tech-01-db-cutover", results); err == nil {
        t.Fatal("expected immutable overwrite rejection")
    }
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./internal/council/benchmark -run 'TestCreateRun|TestWriteBaseline|TestWriteFinal' -count=1`

Expected: FAIL because store functions do not exist.

- [ ] **Step 3: Implement exclusive contained writes**

Use `os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)`, `Sync`, checked `Close`, `filepath.EvalSymlinks`, `filepath.Rel`, and create-only parent directories. JSON marshal once and hash exactly those bytes.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/council/benchmark -run 'TestCreateRun|TestWriteBaseline|TestWriteFinal' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/council/benchmark/store.go internal/council/benchmark/store_test.go
git commit -m "feat: persist immutable H1 artifacts"
```

---

### Task 4: H1 batch orchestrator

**Files:**
- Create: `internal/council/benchmark/runner.go`
- Create: `internal/council/benchmark/runner_test.go`

**Interfaces:**
- Consumes: frozen `Dataset`, Phase F baseline execution, Phase G evaluator, and Task 3 store.
- Produces:

```go
type BaselineExecutor interface {
    RunAll(context.Context, baseline.RunRequest) ([]baseline.ArmResult, error)
}

type EvalExecutor interface {
    EvaluateProblem(context.Context, evalharness.ProblemRequest) (evalharness.ProblemResult, error)
}

type Runner struct {
    NewBaseline func(councilruntime.Provider) BaselineExecutor
    Evaluator EvalExecutor
    Now func() time.Time
}

type RunRequest struct {
    Dataset Dataset
    RunsRoot string
    RunID string
}

type RunResult struct {
    RunID string `json:"run_id"`
    RunDir string `json:"run_dir"`
    Summary evalharness.BatchSummary `json:"batch_summary"`
}

func (r Runner) Run(ctx context.Context, req RunRequest) (RunResult, error)
```

For each dataset case in order, call `r.NewBaseline(case.ChallengerProvider)` and then `RunAll` with the same `RunID`, batch `RunRoot`, and exact compact problem bytes. Persist six raw arm results. Then evaluate using exact rubric/reference hashes and `H1RiskPolicy`. Accumulate `ProblemResult`; only after all 20 succeed call `evalharness.SummarizeBatch`, `evalharness.WriteEvaluation`, and `WriteFinalResult`.

- [ ] **Step 1: Write failing orchestration tests**

Use a fake baseline factory that records challenger provider and returns six valid arms, and a fake evaluator that records requests and returns deterministic valid arm scores.

```go
func TestRunnerUsesFrozenChallengerSchedule(t *testing.T) {
    dataset := validInMemoryDataset(t)
    var got []councilruntime.Provider
    runner := Runner{
        NewBaseline: func(provider councilruntime.Provider) BaselineExecutor {
            got = append(got, provider)
            return fakeBaseline{results: sixArmResults()}
        },
        Evaluator: fakeEvaluator{},
        Now: func() time.Time { return time.Unix(0, 0).UTC() },
    }
    _, err := runner.Run(context.Background(), RunRequest{Dataset: dataset, RunsRoot: t.TempDir(), RunID: "h1-test"})
    if err != nil { t.Fatal(err) }
    for i, provider := range got {
        want := councilruntime.ProviderClaude
        if (i+1)%2 == 0 { want = councilruntime.ProviderCodex }
        if provider != want { t.Fatalf("case %d got %s want %s", i+1, provider, want) }
    }
}
```

Add a failure test where case 3 baseline errors and assert cases 4–20 are never invoked and no `eval/batch-summary.json` or `h1-result.json` exists.

- [ ] **Step 2: Run RED**

Run: `go test ./internal/council/benchmark -run TestRunner -count=1`

Expected: FAIL because `Runner` does not exist.

- [ ] **Step 3: Implement minimal orchestrator**

Validate required dependencies and `req.Dataset` frozen invariants before `CreateRun`. Preserve case order. Wrap errors with case ID and phase. Do not retry or continue after failure.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/council/benchmark -run TestRunner -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/council/benchmark/runner.go internal/council/benchmark/runner_test.go
git commit -m "feat: orchestrate H1 benchmark batch"
```

---

### Task 5: CLI wiring without policy override knobs

**Files:**
- Modify: `cmd/agentd/main.go`
- Modify: `cmd/agentd/main_test.go`

**Interfaces:**
- Produces command: `agentd council benchmark h1 [flags]`.

Add a `benchmark` case under the existing `council` command. `runCouncilBenchmarkH1` parses only:

```text
--dataset
--runs-dir
--config
--temp-root
--claude-bin
--codex-bin
```

Load `config.Load` and `benchmark.LoadH1` before creating the run. Construct one Claude and one Codex runtime and reuse them for both Phase F and Phase G. Production runner wiring is:

```go
claude := councilruntime.NewClaudeCLI(*claudeBin)
codex := councilruntime.NewCodexCLI(*codexBin)
evaluator := evalharness.Harness{Claude: claude, Codex: codex, TempRoot: *tempRoot}
r := benchmark.Runner{
    NewBaseline: func(provider councilruntime.Provider) benchmark.BaselineExecutor {
        return baseline.Runner{
            Claude: claude,
            Codex: codex,
            TempRoot: *tempRoot,
            ChallengerProvider: provider,
            ChallengePolicy: benchmark.H1ChallengePolicy,
        }
    },
    Evaluator: evaluator,
}
```

Generate a safe unique run ID with UTC timestamp plus 6 crypto-random bytes, prefixed `h1-`. Encode `benchmark.RunResult` as JSON to stdout.

- [ ] **Step 1: Write failing CLI tests**

Refactor command construction minimally so tests can inject a benchmark runner function rather than execute real model CLIs. Test command routing, default dataset/runs/temp flags, config rejection for metered fallback, unknown/frozen-policy flag rejection, and JSON success output.

```go
func TestRunRoutesH1Benchmark(t *testing.T) {
    // inject runBenchmarkH1Fn, call run([]string{"council","benchmark","h1","--dataset",dataset}, ...),
    // and assert the injected function receives the dataset path exactly once.
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./cmd/agentd -run Benchmark -count=1`

Expected: FAIL because benchmark routing does not exist.

- [ ] **Step 3: Implement CLI wiring**

Keep the existing `council run` and `council doctor isolation` behavior unchanged. Do not expose flags for comparator, delta, challenge mode, challenger provider, judge provider, arm selection, or case selection.

- [ ] **Step 4: Run GREEN**

Run: `go test ./cmd/agentd -run Benchmark -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/agentd/main.go cmd/agentd/main_test.go
git commit -m "feat: add H1 benchmark CLI"
```

---

### Task 6: Exact-head repository gate and PR

**Files:**
- Update: PR description only after implementation.

- [ ] **Step 1: Run formatting check**

Run: `test -z "$(gofmt -l cmd internal)"`

Expected: exit 0.

- [ ] **Step 2: Run full tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Run vet**

Run: `go vet ./...`

Expected: PASS.

- [ ] **Step 4: Run lint**

Run the repository-pinned golangci-lint configuration/version used by CI.

Expected: PASS.

- [ ] **Step 5: Audit scope**

Changed files must be limited to H1 spec/plan, `benchmarks/h1/**`, `internal/council/benchmark/**`, and `cmd/agentd/main*.go`. Confirm no changes to `baseline/**`, `evalharness/**`, `protocol/**`, `runtime/**`, or `visibility/**`.

- [ ] **Step 6: Open draft PR and obtain exact-head CI + CLA**

The PR body must restate the frozen 20-case split, shared rubric, `MaterialWorseDelta=10`, full challenge policy, balanced challenger schedule, evidence visibility invariant, and the rule that no real H1 call occurs before merge.

- [ ] **Step 7: Merge only on exact head**

After both `quality` and `cla` are green on the same final head SHA, mark ready, re-fetch the head/checks, squash merge using `expected_head_sha`, and verify `main` moved to the returned merge SHA.

- [ ] **Step 8: Do not run H1 yet inside this implementation PR**

A real model-backed H1 execution is a separate post-merge operation so the benchmark and policy are frozen on `main` before any result exists.
