# H2 Output Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a versioned H2 benchmark that preserves H1 decision/evaluation semantics while tolerating a single JSON Markdown fence, recording exact provider responses, and preserving completed baseline arms on failure.

**Architecture:** Add a shared `modeloutput` decoder, a shared safe immutable-file helper, and an `invocationlog` AgentRuntime decorator. H2 uses new dataset/run identity and a runner that executes/persists A-F incrementally; H1 code paths remain compatible. Provider prompts and CLI execution flags do not change.

**Tech Stack:** Go 1.26, existing council runtime/protocol/evalharness/benchmark packages, JSON files, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-08-27-h2-output-contract-design.md`

## Global Constraints

- H1 remains frozen and must not be mutated to reinterpret or retry previous model calls.
- H2 keeps H1 case content, rubric semantics, arm topology, challenge routing, comparator `best_single`, `MaterialWorseDelta=10.0`, and subscription-only provider policy.
- Common decoder accepts only raw single-object JSON or one untagged/`json` Markdown fence containing one object; no repair/prose stripping.
- Every actual provider response must be persisted before caller parsing.
- No resume, provider substitution, metered fallback, or automatic model retry is introduced.
- No real H2 model call until implementation/data are merged and a separate execution workflow is pinned to the frozen H2 SHA.

---

### Task 1: Shared strict model-output decoder

**Files:**
- Create: `internal/council/modeloutput/decode.go`
- Create: `internal/council/modeloutput/decode_test.go`
- Modify: `internal/council/baseline/runner.go`
- Modify: `internal/council/protocol/engine.go`
- Modify: `internal/council/protocol/fullinfo.go` if it has an independent decoder
- Modify: `internal/council/evalharness/harness.go`
- Modify: `internal/council/evalharness/candidate.go` if it has an independent decoder

**Interfaces:**
- Produces: `func DecodeStrict(raw string, out any) error`
- Produces: `func Normalize(raw string) (string, error)` for focused unit tests; callers use `DecodeStrict`.

- [ ] **Step 1: Write failing decoder tests**

Cover table cases for raw object, untagged fence, `json` fence, prose prefix/suffix, unsupported tag, nested/multiple fence, multiple JSON values, trailing text, array/null/string top-level, malformed JSON, and unknown struct field.

```go
type sample struct { Value string `json:"value"` }

func TestDecodeStrictAcceptedTransportForms(t *testing.T) {
    for _, raw := range []string{`{"value":"x"}`, "```json\n{\"value\":\"x\"}\n```", "```\n{\"value\":\"x\"}\n```"} {
        var got sample
        if err := DecodeStrict(raw, &got); err != nil { t.Fatal(err) }
        if got.Value != "x" { t.Fatalf("got %q", got.Value) }
    }
}
```

- [ ] **Step 2: Run `go test ./internal/council/modeloutput` and verify RED because package/function is absent.**

- [ ] **Step 3: Implement exact transport grammar**

`Normalize` trims outer whitespace; raw mode requires first byte `{`; fenced mode requires opening line exactly ` ``` ` or ` ```json `, closing line exactly ` ``` `, no other fence token in body, and body first byte `{`. `DecodeStrict` then uses `json.Decoder.DisallowUnknownFields()` and verifies EOF after one value.

- [ ] **Step 4: Run decoder tests GREEN.**

- [ ] **Step 5: Replace duplicated strict JSON decoders at baseline/protocol/eval call sites with `modeloutput.DecodeStrict`, preserving their existing semantic validation and failure-class wrapping. Add focused regression tests showing fenced provider stdout now reaches semantic parsing.**

- [ ] **Step 6: Run `go test ./internal/council/baseline ./internal/council/protocol ./internal/council/evalharness ./internal/council/modeloutput`.**

- [ ] **Step 7: Commit `feat: normalize fenced model JSON output`.**

### Task 2: Shared create-only symlink-safe file helper

**Files:**
- Create: `internal/council/safestore/store.go`
- Create: `internal/council/safestore/store_test.go`
- Modify: `internal/council/benchmark/store.go`
- Modify: `internal/council/artifactstore/store.go`
- Modify: `internal/council/evalharness/store.go` only where equivalent private helpers can be replaced without changing paths/schema.

**Interfaces:**
- Produces: `func WriteExclusive(root, rel string, data []byte) (string, error)`
- Produces: `func EnsureDirectory(root, rel string) (string, error)` if required by existing stores.

- [ ] **Step 1: Write RED tests for traversal, absolute path, symlinked parent, final-path symlink, existing file, and successful nested create-only write.**
- [ ] **Step 2: Implement component-wise `Lstat`/containment checks and `os.OpenFile(..., O_CREATE|O_EXCL|O_WRONLY, 0o600)`; create directories only after validating each component.**
- [ ] **Step 3: Run safestore tests GREEN.**
- [ ] **Step 4: Migrate existing stores only where behavior is byte/path compatible; run artifactstore/benchmark/evalharness tests to prove no H1 contract change.**
- [ ] **Step 5: Commit `refactor: share immutable artifact writes`.**

### Task 3: Provider invocation evidence decorator

**Files:**
- Create: `internal/council/invocationlog/runtime.go`
- Create: `internal/council/invocationlog/runtime_test.go`

**Interfaces:**

```go
type Runtime struct {
    Inner runtime.AgentRuntime
    Provider runtime.Provider
}

func Wrap(inner runtime.AgentRuntime, provider runtime.Provider) runtime.AgentRuntime
```

Evidence schema `council.invocation-evidence.v1` includes run/provider/participant/role/phase/prompt SHA-256/stdout/stderr/exit code/attempts/start/finish.

- [ ] **Step 1: Write RED tests with fake runtimes for success, quota/process failure with populated response, pre-model auth failure with zero response, evidence-write failure, concurrent calls, and unsafe participant/phase components.**
- [ ] **Step 2: Implement a concurrency-safe monotonic sequence per wrapper using `atomic.Uint64`; write `invocations/<provider>/<participant>/<phase>/<sequence>.json` with `safestore.WriteExclusive`.**
- [ ] **Step 3: Persist after `Inner.Run` returns and before returning to the caller whenever `StartedAt` is non-zero or `Attempts > 0`. On persistence failure return/join `runtime.FailureIsolation`.**
- [ ] **Step 4: Run invocationlog tests GREEN and commit `feat: persist raw provider invocation evidence`.**

### Task 4: Incremental baseline arm persistence for H2

**Files:**
- Modify: `internal/council/baseline/runner.go`
- Modify: `internal/council/baseline/runner_test.go`
- Modify: `internal/council/benchmark/store.go`
- Modify: `internal/council/benchmark/store_test.go`
- Create/modify: `internal/council/benchmark/h2_runner.go`
- Create: `internal/council/benchmark/h2_runner_test.go`

**Interfaces:**
- Produces: `func FrozenArms() []Arm` returning a defensive copy of A-F.
- Produces: `func WriteBaselineArmResult(ctx context.Context, runRoot, problemID string, result baseline.ArmResult) error`.
- Produces H2 runner behavior: loop `baseline.FrozenArms()`, call `RunArm`, persist each success immediately, then evaluate only after all six succeed.

- [ ] **Step 1: RED test: arm C failure leaves arm-A and arm-B files, no C-F files, no eval summary/result.**
- [ ] **Step 2: RED store tests for duplicate arm write, unsafe problem ID, symlinked baseline parent, and exact arm filename.**
- [ ] **Step 3: Implement `FrozenArms` and `WriteBaselineArmResult`; keep existing H1 `RunAll`/`WriteBaselineResults` behavior intact.**
- [ ] **Step 4: Implement H2 runner incremental loop and unchanged evaluator request/risk-policy validation.**
- [ ] **Step 5: Run baseline/benchmark tests GREEN and commit `feat: preserve partial H2 arm results`.**

### Task 5: Versioned H2 dataset and manifests

**Files:**
- Modify: `internal/council/benchmark/types.go`
- Modify: `internal/council/benchmark/dataset.go`
- Add H2 loader tests under `internal/council/benchmark/`
- Create: `benchmarks/h2/README.md`
- Create: `benchmarks/h2/manifest.json`
- Create: `benchmarks/h2/rubric.json`
- Create: `benchmarks/h2/cases.json`

**Interfaces:**
- `H2BenchmarkID = "h2"`
- H2 schema constants use `council.h2-*.v0`.
- `LoadH2(root string) (Dataset, error)` shares validation machinery with `LoadH1` but validates H2 identity.

- [ ] **Step 1: Refactor loader tests first so existing H1 tests remain green and new H2 fixture tests are RED.**
- [ ] **Step 2: Generalize internal loader with explicit expected benchmark/schema parameters; public `LoadH1` behavior remains unchanged.**
- [ ] **Step 3: Generate H2 dataset by copying H1 semantic content and changing only version/schema identity. Recompute SHA-256 manifest values.**
- [ ] **Step 4: Add semantic-equality test that decodes H1/H2 and compares title/decision/context/constraints/options/evidence/reference claims/rubric dimensions+weights after version-only normalization.**
- [ ] **Step 5: Run committed dataset tests GREEN and commit `feat: freeze H2 benchmark dataset`.**

### Task 6: H2 run/store identity and CLI routing

**Files:**
- Modify: `internal/council/benchmark/store.go`
- Add H2 store tests
- Modify: `cmd/agentd/main.go`
- Modify: `cmd/agentd/main_test.go`

**Interfaces:**
- H2 run IDs: `h2-<UTC timestamp>-<12 hex chars>`.
- `agentd council benchmark h2` flags exactly mirror H1 operational flags; default dataset `benchmarks/h2`; no policy override flags.
- H2 wraps Claude/Codex runtimes with `invocationlog.Wrap` before constructing baseline/evaluator dependencies.

- [ ] **Step 1: RED CLI tests for H2 routing/default dataset/new run prefix and rejection of policy overrides/metered fallback.**
- [ ] **Step 2: Add version-aware run/result schema construction without altering H1 output.**
- [ ] **Step 3: Wire H2 runner + wrapped runtimes; run `go test ./cmd/agentd ./internal/council/benchmark`.**
- [ ] **Step 4: Commit `feat: add H2 benchmark command`.**

### Task 7: Final verification, PR, and H2 freeze

**Files:**
- Update this plan checkboxes/PR body only as evidence requires; no opportunistic product changes.

- [ ] **Step 1: Run `gofmt -w` on changed Go files and verify `gofmt -l .` is empty.**
- [ ] **Step 2: Run `go test ./...`.**
- [ ] **Step 3: Run `go vet ./...`.**
- [ ] **Step 4: Run repository lint command/workflow equivalent.**
- [ ] **Step 5: Audit changed filenames: only H2 design/plan/data plus decoder/safestore/invocationlog/baseline/benchmark/protocol/eval/CLI files required above.**
- [ ] **Step 6: Open PR linked to #18, wait for exact-head CI + CLA, repair until both green, then squash merge with expected-head SHA.**
- [ ] **Step 7: Fetch `main` and record the exact H2 frozen implementation SHA in #18. No real H2 model call before this point.**

### Task 8: Separate pinned H2 execution workflow

**Files:**
- Create after Task 7 merge: `.github/workflows/h2-frozen-execution.yml`
- Add focused workflow contract test under `internal/council/runnerbootstrap/`.

- [ ] **Step 1: Create a fresh ops branch from the H2 frozen implementation SHA. Write a RED test requiring manual dispatch with no inputs, self-hosted/linux/h2-benchmark labels, and checkout pinned to the exact H2 frozen SHA.**
- [ ] **Step 2: Implement workflow: setup Go, verify frozen checkout, verify subscription CLIs, run repo tests, execute `agentd council benchmark h2` exactly once, always collect hashes/full-or-partial evidence, upload artifact.**
- [ ] **Step 3: Exact-head CI + CLA green, squash merge.**
- [ ] **Step 4: Bootstrap a fresh ephemeral WSL2 runner labeled `h2-benchmark`, dispatch one new H2 workflow run, and record run ID in #18. No auto-retry.**
- [ ] **Step 5: On terminal state, collect evidence, close #18 only if a final `h2-result.json` and batch summary exist; otherwise document exact failure and stop for root-cause analysis.**
