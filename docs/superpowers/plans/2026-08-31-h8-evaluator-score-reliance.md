# H8 Evaluator Score-Reliance Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an H8-only evaluator wire contract that makes verification and judge score reliance co-located and unambiguous, while preserving all H1-H7 frozen behavior and H7 claim-aware citation identity.

**Architecture:** H8 adds `CitationContractStructuredV3` and `SchemaProfileH8`. The H8 wire object removes model-authored `relied_on_citations`; each citation check carries exact full-tuple reference, finite `verified|unverified` status, `relied_on` boolean, and note. After fail-closed validation, the existing internal `JudgeArtifact.ReliedOnCitations` is derived deterministically from `verified && relied_on` checks. H8 benchmark/data/CLI are versioned copies of H7 so no frozen H1-H7 path is mutated semantically.

**Tech Stack:** Go 1.26, strict JSON decoding, JSON Schema profiles, existing benchmark/evalharness/adapterpool packages, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-08-31-h8-evaluator-score-reliance-design.md`

## Global Constraints

- Never mutate or rerun frozen H1-H7.
- Preserve H7 full citation identity `(artifact_id, locator, claim)` and canonical tuple serialization.
- H8 verification vocabulary is exactly `verified|unverified`; partial support is `unverified` with explanatory note.
- `relied_on` means the judge itself uses the citation as support for numeric scoring.
- `unverified + relied_on:true` is terminal malformed output; no repair/coercion.
- Preserve H7 adapter policy, availability-only failover, same-adapter `max_attempts=1`, no metered credentials, and human final fallback.
- H8 implementation/data freeze precedes a separate pinned ops workflow; no H7 attempt #3 and no H6 redispatch.

---

### Task 1: Prove the H8 evaluator contract with RED tests

**Files:**
- Create: `internal/council/evalharness/h8_citations_test.go`
- Read-only reference: `internal/council/evalharness/h7_citations_test.go`

**Interfaces:**
- Consumes: existing `CitationOccurrenceKey`, `MaskedCandidate`, `h7DuplicateSourceCandidate`, `Harness.evaluateCandidate`.
- Produces expected future interfaces: `CitationContractStructuredV3`, `H8CitationCheck`, `H8JudgeArtifact`, `validateH8CitationReferences`, `decodeH8Judge`.

- [ ] **Step 1: Add a real-failure-shaped contradiction test**

Create a test whose wire result contains the exact full tuple, `status:"unverified"`, `relied_on:true`, and asserts `validateH8CitationReferences` rejects with `relied-on citation ... is not verified`. Use the H7 helper candidate so the failure cannot be attributed to unknown tuple identity.

- [ ] **Step 2: Add acceptance tests for valid reliance states**

Add one test for `verified + relied_on:true` that expects the canonical tuple in internal `JudgeArtifact.ReliedOnCitations`, and one for `unverified + relied_on:false` that expects no derived reliance.

- [ ] **Step 3: Add a claim-distinct regression test**

Use the two citations sharing `artifact_id:"problem"` and `locator:"constraints[1]"` with different claims; assert both H8 checks survive canonicalization independently.

- [ ] **Step 4: Run focused tests and record RED**

Run:

```bash
go test ./internal/council/evalharness -run 'TestH8' -count=1
```

Expected: FAIL because H8 contract types/functions are not implemented. The failure must be attributable to missing H8 behavior, not malformed test setup.

- [ ] **Step 5: Commit RED tests**

```bash
git add internal/council/evalharness/h8_citations_test.go
git commit -m "test: reproduce H8 evaluator reliance contradiction"
```

---

### Task 2: Implement the H8 evaluator wire contract minimally

**Files:**
- Create: `internal/council/evalharness/h8_citations.go`
- Modify: `internal/council/evalharness/h6_citations.go`
- Modify: `internal/council/evalharness/harness.go`
- Test: `internal/council/evalharness/h8_citations_test.go`

**Interfaces:**
- Produces `CitationContractStructuredV3`.
- Produces `H8CitationCheck { Reference CitationOccurrenceKey; Status string; ReliedOn bool; Note string }`.
- Produces `H8JudgeArtifact` with all existing judge fields except model-authored `ReliedOnCitations`.
- Produces `decodeH8Judge(raw string, dimensions []string, candidate MaskedCandidate) (JudgeArtifact, error)`.

- [ ] **Step 1: Add V3 without changing V1/V2 values**

Append `CitationContractStructuredV3` after V2 in `h6_citations.go`, include it in `validateCitationContract`, and route V3 in `decodeJudgeForContract` to `decodeH8Judge`. Do not edit H6/H7 decoder semantics.

- [ ] **Step 2: Implement exact H8 validation**

In `h8_citations.go`, build candidate membership by full `CitationOccurrenceKey`; reject unknown or duplicate full tuples; normalize status only by trim/lower for comparison; accept only exact semantic values `verified` or `unverified`; reject `ReliedOn && status != "verified"`.

Use an error of the form:

```go
return fmt.Errorf("relied-on citation %q is not verified", canonicalOccurrenceString(check.Reference))
```

Do not repair the wire object.

- [ ] **Step 3: Derive internal reliance deterministically**

Canonicalize every H8 check into `protocol.CitationCheck`. Append canonical tuple JSON to `JudgeArtifact.ReliedOnCitations` only when the validated check has `ReliedOn == true`.

- [ ] **Step 4: Add the explicit H8 judge instruction**

The prompt must say, literally in substance:

```text
status must be exactly "verified" or "unverified".
"relied_on" means you, the evaluation judge, use this exact citation as evidentiary support for your numeric score.
A partially supported, inferred, over-broad, contradicted, or unsupported full claim is "unverified" and MUST have "relied_on": false.
Only "verified" citations may have "relied_on": true.
```

Render the same problem/rubric/reference-set/candidate artifacts and preserve exact tuple-copy requirements.

- [ ] **Step 5: Route V3 prompt rendering**

In `Harness.evaluateCandidate`, select `renderH8JudgePrompt` only for `CitationContractStructuredV3`. Keep Legacy/V1/V2 routing unchanged.

- [ ] **Step 6: Run focused tests GREEN**

```bash
gofmt -w internal/council/evalharness/h8_citations.go internal/council/evalharness/h8_citations_test.go internal/council/evalharness/h6_citations.go internal/council/evalharness/harness.go
go test ./internal/council/evalharness -run 'TestH8|TestH7|TestH6' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit evaluator contract**

```bash
git add internal/council/evalharness
git commit -m "fix: make H8 evaluator score reliance explicit"
```

---

### Task 3: Add an H8-only structured-output schema profile

**Files:**
- Create: `internal/council/structuredoutput/h8_schema_test.go`
- Modify: `internal/council/structuredoutput/profiles.go`

**Interfaces:**
- Produces `SchemaProfileH8`.
- H8 eval-judge schema consumes the H7 `citationOccurrenceKeySchema` and emits citation checks with `status` enum `["verified","unverified"]`, required boolean `relied_on`, and note.
- H8 eval-judge schema does not contain a top-level `relied_on_citations` property.

- [ ] **Step 1: Write schema RED tests**

Assert the H8 eval-judge schema:

```text
requires citation_checks[].relied_on
status has exactly two enum values: verified, unverified
does not define top-level relied_on_citations
```

Also assert a non-eval schema returned under `SchemaProfileH8` is byte-identical to H7 for the same role/phase.

- [ ] **Step 2: Run schema tests RED**

```bash
go test ./internal/council/structuredoutput -run 'TestH8' -count=1
```

Expected: FAIL because `SchemaProfileH8` is absent.

- [ ] **Step 3: Implement `SchemaProfileH8`**

Append the profile enum after H7, add closed H8 citation-check/eval-judge schemas, and route only `role=="judge" && phase==evalharness.PhaseEvalJudge` to the H8 schema. Delegate every other role/phase to `SchemaForProfile(..., SchemaProfileH7)` so H7-compatible bytes remain unchanged.

- [ ] **Step 4: Run schema tests GREEN**

```bash
gofmt -w internal/council/structuredoutput/profiles.go internal/council/structuredoutput/h8_schema_test.go
go test ./internal/council/structuredoutput -run 'TestH8|TestH7|TestH6' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit schema profile**

```bash
git add internal/council/structuredoutput
git commit -m "feat: add H8 structured output profile"
```

---

### Task 4: Create the frozen H8 dataset/version boundary

**Files:**
- Create: `internal/council/benchmark/h8_types.go`
- Create: `internal/council/benchmark/h8_dataset_test.go`
- Modify: `internal/council/benchmark/dataset.go`
- Create directory: `benchmarks/h8/`
- Create: `benchmarks/h8/manifest.json`
- Create: `benchmarks/h8/rubric.json`
- Create: `benchmarks/h8/cases.json`
- Create: `benchmarks/h8/adapter-policy.json`
- Create: `benchmarks/h8/README.md`

**Interfaces:**
- Produces `H8BenchmarkID="h8"`, H8 dataset/cases/rubric/run/result schema version constants, `H8RiskPolicy`, `H8ChallengePolicy`, and `LoadH8`.

- [ ] **Step 1: Write dataset RED tests**

Require `LoadH8("benchmarks/h8")` to load 20 cases; require H8 risk/challenge policy to equal H7; require adapter policy bytes to be identical to H7; require normalized problem/reference payloads and case order to be semantically identical to H7; require H8 schema/benchmark identity fields to differ only at the version boundary.

- [ ] **Step 2: Run dataset tests RED**

```bash
go test ./internal/council/benchmark -run 'TestH8.*Dataset|TestH8.*Equivalent' -count=1
```

Expected: FAIL because H8 dataset/version does not exist.

- [ ] **Step 3: Add H8 constants and loader**

Add `h8DatasetVersion` and `LoadH8` in `dataset.go`. Include H8 in the same adapter-policy/challenger-unbound branch currently used by H5-H7. Keep existing H1-H7 conditions intact except adding the new H8 ID to the version-aware condition.

- [ ] **Step 4: Materialize H8 dataset from H7**

Copy H7 rubric/cases/adapter policy, change only H8 schema markers where required, recompute SHA-256 for changed `rubric.json` and `cases.json`, and write those exact hashes plus the unchanged adapter-policy hash into `manifest.json`. Do not alter problem/reference content, case order, rubric dimension semantics, comparator, threshold, or adapter chains.

- [ ] **Step 5: Run dataset tests GREEN**

```bash
gofmt -w internal/council/benchmark/h8_types.go internal/council/benchmark/h8_dataset_test.go internal/council/benchmark/dataset.go
go test ./internal/council/benchmark -run 'TestH8.*Dataset|TestH8.*Equivalent' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit H8 dataset boundary**

```bash
git add internal/council/benchmark benchmarks/h8
git commit -m "feat: freeze H8 benchmark dataset identity"
```

---

### Task 5: Wire H8 benchmark execution without changing H7

**Files:**
- Create H8-versioned benchmark runner/store files under `internal/council/benchmark/` following the H7 runner boundaries, with H8 schema/result constants.
- Create: `cmd/agentd/h8_benchmark.go`
- Create: `cmd/agentd/h8_benchmark_test.go`
- Modify: `cmd/agentd/main.go`

**Interfaces:**
- Produces `agentd council benchmark h8`.
- H8 adapter wrappers use `structuredoutput.SchemaProfileH8`.
- H8 evaluator uses `evalharness.CitationContractStructuredV3`.
- H8 baseline/protocol execution keeps H7 challenge/citation-authority behavior and adapter topology.

- [ ] **Step 1: Write CLI/runner RED tests**

Assert `council benchmark h8` selects `benchmarks/h8`, emits `h8-<timestamp>-<suffix>` run IDs, loads H8, and the evaluator is configured with `SchemaProfileH8` + `CitationContractStructuredV3`. Add a regression asserting the H7 command still selects V2/H7 profile.

- [ ] **Step 2: Run RED tests**

```bash
go test ./cmd/agentd ./internal/council/benchmark -run 'Test.*H8' -count=1
```

Expected: FAIL because H8 command/runner are absent.

- [ ] **Step 3: Add H8 runner/store types as versioned H7-equivalent code**

Keep the execution algorithm byte/logic-equivalent to H7 except H8 type/schema names. Do not generalize H1-H7 into a shared abstraction in this change; version-copying is intentional to avoid frozen behavioral drift.

- [ ] **Step 4: Add `cmd/agentd/h8_benchmark.go`**

Use the H7 command structure with H8 labels/default dataset/RunID, call `benchmark.LoadH8`, wrap adapters with `SchemaProfileH8`, and construct the evaluator with `CitationContractStructuredV3`.

- [ ] **Step 5: Route H8 first in `main.go`**

Introduce `runWithH8BenchmarkExecutors`; dispatch `council benchmark h8` there, then delegate unchanged H7-and-older commands to the existing chain.

- [ ] **Step 6: Run focused GREEN tests**

```bash
gofmt -w cmd/agentd/h8_benchmark.go cmd/agentd/h8_benchmark_test.go cmd/agentd/main.go internal/council/benchmark/h8_*.go
go test ./cmd/agentd ./internal/council/benchmark ./internal/council/evalharness ./internal/council/structuredoutput -run 'Test.*H8|Test.*H7' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit execution wiring**

```bash
git add cmd/agentd internal/council/benchmark
git commit -m "feat: wire H8 frozen benchmark execution"
```

---

### Task 6: Full verification, freeze evidence, and implementation PR

**Files:**
- Update only H8 README/design evidence if verification discovers a documentation mismatch.
- No ops workflow in this PR.

**Interfaces:**
- Produces an implementation/data PR for issue #45 and an exact merge SHA suitable to become `H8_FROZEN_SHA` after merge.

- [ ] **Step 1: Run formatting/static correctness**

```bash
gofmt -w cmd/agentd internal/council/evalharness internal/council/structuredoutput internal/council/benchmark
git diff --check
go vet ./...
```

Expected: all commands exit 0.

- [ ] **Step 2: Run full test suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Run race coverage**

```bash
go test -race ./internal/council/evalharness ./internal/council/structuredoutput ./internal/council/benchmark ./cmd/agentd
go test -race ./...
```

Expected: PASS.

- [ ] **Step 4: Run pinned lint**

```bash
golangci-lint run
```

Use repository-pinned/required golangci-lint v2.12.2. Expected: PASS.

- [ ] **Step 5: Verify frozen compatibility explicitly**

Confirm H1-H7 tests pass unchanged; compare H8 vs H7 adapter-policy bytes; compare every H8/H7 case problem/reference payload; verify H7 structured-output profile bytes remain unchanged; verify the H6 duplicate-source/different-claim failure fixture and H7 success fixture retain prior results.

- [ ] **Step 6: Record H8 input hashes**

```bash
sha256sum benchmarks/h8/manifest.json benchmarks/h8/rubric.json benchmarks/h8/cases.json benchmarks/h8/adapter-policy.json
```

Record exact values in PR evidence; do not call them frozen until the implementation/data PR is merged.

- [ ] **Step 7: Open implementation/data PR**

PR title:

```text
H8: make evaluator score reliance generation-safe
```

PR body must link `Fixes #45` only if project policy allows the implementation issue to close on merge; otherwise `Refs #45` and keep #45 open until real H8 terminal execution. Because #45 represents the real-execution validation lane, prefer `Refs #45` and keep it open through the later ops/run audit.

- [ ] **Step 8: Require exact-head CI/CLA before merge**

Do not merge on stale checks. Review exact PR head, resolve findings, rerun required verification after any code change, and merge only when exact-head CI + CLA are green.

- [ ] **Step 9: Freeze after merge**

Record the merge commit as `H8_FROZEN_SHA`, recompute the four H8 input hashes directly from that SHA, and add a freeze checkpoint comment to #45. Only then begin the separate H8 ops-workflow task; never dispatch H8 from this implementation PR.