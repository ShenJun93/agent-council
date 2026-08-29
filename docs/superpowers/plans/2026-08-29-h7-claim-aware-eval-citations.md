# H7 Claim-Aware Evaluator Citations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Version the evaluator citation contract so H7 identifies candidate citation occurrences by the full `(artifact_id, locator, claim)` tuple and can evaluate the H6 arm-F failure fixture without collapsing distinct claims.

**Architecture:** Add an opt-in `CitationContractStructuredV2` plus `SchemaProfileH7`, leaving legacy/H6 paths untouched. H7 validates exact full citation tuples, then losslessly serializes validated tuples as compact canonical JSON strings only when crossing the existing `JudgeArtifact` compatibility boundary. Version H6 benchmark data/runtime identity to H7 without changing benchmark semantics, adapter policy, or failover behavior.

**Tech Stack:** Go 1.26, existing evalharness/structuredoutput/benchmark packages, Claude/Codex/Antigravity subscription adapters, human ChatGPT broker, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-08-29-h7-claim-aware-eval-citations-design.md`

## Global Constraints
- Never mutate or rerun frozen H6.
- H1-H6 frozen behavior and paths remain unchanged.
- H7 citation identity is exactly `(artifact_id, locator, claim)`; no source-only dedupe, fuzzy matching, trimming repair, or claim normalization.
- Duplicate identical full tuples, unknown tuples, and unverified relied-on tuples remain terminal `malformed_output`.
- H7 preserves H6 problem/reference/rubric semantics, adapter-policy bytes, logical-slot topology, availability-only failover, and final `human-chatgpt-session` fallback.
- No provider-policy override, metered API fallback, retry loop, or real H7 model call before implementation/data freeze.

---

### Task 1: Add H7 claim-aware evaluator contract

**Files:**
- Create: `internal/council/evalharness/h7_citations.go`
- Create: `internal/council/evalharness/h7_citations_test.go`
- Modify: `internal/council/evalharness/h6_citations.go`
- Modify: `internal/council/evalharness/harness.go`

**Interfaces:**
- Add `CitationContractStructuredV2`.
- Add `CitationOccurrenceKey{ArtifactID string, Locator string, Claim string}` plus `H7CitationCheck` and `H7JudgeArtifact`.
- Keep `CitationContractStructuredV1`, `CitationKey`, H6 prompt, validation, and serialization behavior unchanged.
- H7 validated references cross the existing result-model boundary as compact canonical JSON strings generated with `json.Marshal(CitationOccurrenceKey)`.

- [ ] **Step 1: Write the H6 failure regression before production code.** Create a candidate with two citations sharing `artifact_id=problem` and `locator=constraints[1]` but different claims. Prove V1 rejects two checks as a duplicate source and V2 expects both full tuples to validate and survive serialization.

```go
candidate := MaskedCandidate{Citations: []protocol.EvidenceRef{
    {ArtifactID: "problem", Locator: "constraints[1]", Claim: "claim one"},
    {ArtifactID: "problem", Locator: "constraints[1]", Claim: "claim two"},
}}
```

The H7 judge response uses exact full references:

```json
{"reference":{"artifact_id":"problem","locator":"constraints[1]","claim":"claim one"},"status":"verified","note":"matched"}
```

- [ ] **Step 2: Add RED invalid-tuple tests.** Cover an unknown claim with valid artifact/locator, duplicate identical full tuples in `citation_checks`, duplicate identical full tuples in `relied_on_citations`, and a relied-on full tuple without a matching `verified` check.
- [ ] **Step 3: Verify RED.** Run `go test ./internal/council/evalharness -run 'H6|H7|Citation'`. Expected: compile/test failure because V2/H7 handling does not exist.
- [ ] **Step 4: Implement minimal V2.** Add a dedicated H7 prompt requiring `{artifact_id, locator, claim}` copied exactly from `candidate.citations`. Build candidate/check/reliance maps on the full tuple. Reject empty/mutated/unknown tuples without claim normalization.
- [ ] **Step 5: Canonicalize only after validation.** Use `json.Marshal(CitationOccurrenceKey)` to produce the compatibility string; route V2 through H7 prompt/decode while leaving legacy and V1 branches unchanged.
- [ ] **Step 6: Verify GREEN and commit.** Run `gofmt`, `go test ./internal/council/evalharness`, and `git diff --check`; commit `fix: add H7 claim-aware evaluator citations`.

---

### Task 2: Add the H7 structured-output schema profile

**Files:**
- Create: `internal/council/structuredoutput/h7_schema_test.go`
- Modify: `internal/council/structuredoutput/profiles.go`

**Interfaces:**
- Add `SchemaProfileH7`.
- H7 eval-judge citation reference objects require exactly `artifact_id`, `locator`, and `claim`.
- `SchemaProfileH6` stays byte-identical; all H7 non-eval schemas are byte-identical to H6/default equivalents.

- [ ] **Step 1: Write RED schema tests.** Assert H7 `citation_checks[].reference` and `relied_on_citations[]` item shapes contain required properties `artifact_id`, `claim`, `locator` with `additionalProperties:false`. Assert H6 eval bytes remain unchanged and H7 research/protocol schemas equal H6 bytes.
- [ ] **Step 2: Verify RED.** Run `go test ./internal/council/structuredoutput -run 'H6|H7|Profile'`. Expected: compile failure because `SchemaProfileH7` is undefined.
- [ ] **Step 3: Implement H7 profile.** Add a closed three-field citation occurrence schema and H7 eval schema. Extend `SchemaForProfile` so only eval-judge differs for H7; all other role/phase pairs use the existing frozen schema set. Preserve existing caller schema-injection rejection.
- [ ] **Step 4: Verify GREEN and commit.** Run `gofmt`, `go test ./internal/council/structuredoutput ./internal/council/invocationlog`, targeted race, and `git diff --check`; commit `feat: add H7 structured output profile`.

---

### Task 3: Version H6 benchmark mechanics to H7

**Files:**
- Create: `benchmarks/h7/{README.md,adapter-policy.json,cases.json,manifest.json,rubric.json}`
- Create: `internal/council/benchmark/h7_types.go`
- Create: `internal/council/benchmark/h7_runner.go`
- Create: `internal/council/benchmark/h7_full_runner.go`
- Create: `internal/council/benchmark/h7_run_store.go`
- Create: `internal/council/benchmark/h7_dataset_test.go`
- Create: `internal/council/benchmark/h7_runner_test.go`
- Create: `internal/council/benchmark/h7_full_runner_test.go`
- Create: `internal/council/benchmark/h7_run_store_test.go`
- Create: `internal/council/benchmark/h7_claim_aware_eval_integration_test.go`
- Create: `cmd/agentd/h7_benchmark.go`
- Create: `cmd/agentd/h7_benchmark_test.go`
- Create: `cmd/agentd/h7_profile_test.go`
- Modify: `internal/council/benchmark/dataset.go`
- Modify: `cmd/agentd/main.go`

**Interfaces:**
- Add H7 run/result identity and `LoadH7`.
- H7 runner selects `evalharness.CitationContractStructuredV2` and `structuredoutput.SchemaProfileH7`.
- `benchmarks/h7/adapter-policy.json` is byte-identical to H6.

- [ ] **Step 1: Write H7 identity/profile RED tests first.** Version H6 dataset/profile tests to H7 before creating production H7 files. Assert semantic equality with H6, byte-equal adapter policy, fresh `h7-*` run IDs, no policy-override CLI surface, and V2/H7 profile selection.
- [ ] **Step 2: Add the real failure-shape integration RED test.** Use a normalized council candidate with two same-source/different-claim citations and judge output containing both full tuple references. Expect H7 evaluation to complete, preserve two distinct citation-check strings, and not trigger failover for a valid V2 response.
- [ ] **Step 3: Verify RED.** Run `go test ./internal/council/benchmark ./cmd/agentd -run 'H7|ClaimAware'`. Expected: failures for missing H7 dataset/types/runner/CLI.
- [ ] **Step 4: Generate H7 data from H6 and recompute hashes.** Copy H6 problem/reference/rubric semantics and case order. Change only H7 identity/schema markers required by loader/runtime. Copy `adapter-policy.json` byte-for-byte. Recompute frozen SHA-256 values using the same representation used by H6 and verify with `cmp`.
- [ ] **Step 5: Implement H7 runner/store/CLI by the H6 pattern.** Do not change adapter chains, availability classes, human broker, persistence, or execution order. H7 eval runtime construction is the only semantic runtime change: V2 citation contract plus H7 schema profile.
- [ ] **Step 6: Verify GREEN and commit.** Run `gofmt`, benchmark/cmd tests, targeted race, `git diff --check`, and `cmp benchmarks/h6/adapter-policy.json benchmarks/h7/adapter-policy.json`; commit `feat: add H7 claim-aware evaluator benchmark`.

---

### Task 4: Full verification, implementation PR, and freeze

- [ ] **Step 1: Run repository-wide gates.** Run `test -z "$(gofmt -l .)"`, `git diff --check`, `go test ./...`, `go vet ./...`, targeted race for evalharness/structuredoutput/adapterpool/invocationlog/humanbroker/benchmark, `go test -race ./...`, and `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...`.
- [ ] **Step 2: Audit frozen scope.** Prove no tracked H1-H6 benchmark/workflow bytes or legacy runtime contracts changed unexpectedly; specifically compare H6 data files and `.github/workflows/h6-frozen-execution.yml` against `origin/main`.
- [ ] **Step 3: Push exact branch head.** Ensure docs are committed, then `git push -u origin feat/h7-eval-citation-identity`.
- [ ] **Step 4: Open PR tracking #42.** Record H6 failure evidence, exact H7 manifest/rubric/cases/adapter-policy hashes, local verification results, and state that no real H7 model call occurred.
- [ ] **Step 5: Require exact-head quality + CLA, then merge/freeze.** Merge only when both checks succeed on the same head SHA. Read `origin/main`, record `H7_FROZEN_SHA`, and post all frozen hashes to #42.

---

### Task 5: Add the separately pinned H7 execution layer

**Files:**
- Create: `.github/workflows/h7-frozen-execution.yml`
- Modify generic renderer/bootstrap/dispatch tests only if H7 exposes a proven generic-tooling gap.

- [ ] **Step 1: Generate a manual-only workflow pinned to `H7_FROZEN_SHA`.** Use the existing generic renderer. Freeze and verify H7 manifest/rubric/cases/adapter-policy hashes before execution. Preserve always-upload evidence, subscription-only adapters, and no resume/retry inputs.
- [ ] **Step 2: Verify renderer/bootstrap behavior under TDD.** Require H7 renderer `--check`, focused runnerbootstrap tests, full tests/vet/lint, and H6 workflow byte stability.
- [ ] **Step 3: Open and merge a separate ops PR.** Require exact-head quality + CLA. Do not combine implementation/data and execution pinning in one PR.
- [ ] **Step 4: Bootstrap one ephemeral H7 runner and dispatch exactly once.** Use idempotent tooling, reject metered credentials, accept serviceability through the frozen automated chain or final human broker, and record run/job/attempt marker in #42.
- [ ] **Step 5: Process the terminal result once.** On success independently verify final result, eval batch summary, adapter/failover evidence, hashes, and artifact digest before closing #42. On failure preserve the initiating evidence and do not automatically create H7 attempt #2.

## Self-review checklist
- Every design requirement maps to Tasks 1-5.
- H6 source-only typed contract remains a frozen V1 path.
- H7 full tuple identity is preserved through validation and result serialization.
- H7 non-eval schemas and adapter policy remain H6-equivalent.
- No placeholders, automatic retries, hidden output repair, or H6 redispatch are introduced.