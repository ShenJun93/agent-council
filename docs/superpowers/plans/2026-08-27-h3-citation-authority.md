# H3 Citation Authority Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add H3 as a new benchmark version that preserves H2 semantics while separating self-review visibility from final citation authority and enforcing the final citation allowlist in code.

**Architecture:** Extend `baseline.Runner` with a version-selectable citation-authority mode whose zero value preserves H1/H2 behavior. H3 reuses H2 decoding, invocation evidence, sequential arm persistence, evaluator, auth, and risk policies, but constructs baseline runners in strict problem-only final-citation mode. H3 data/store/CLI receive independent version identity; execution workflow is added only after implementation/data are merged and the exact H3 implementation SHA is known.

**Tech Stack:** Go 1.26, GitHub Actions, Claude Code CLI subscription auth, Codex CLI ChatGPT auth.

**Spec:** `docs/superpowers/specs/2026-08-27-h3-citation-authority-design.md`

## Global Constraints

- H1/H2 frozen commits and execution runs are never mutated or rerun.
- Legacy/default baseline citation semantics remain unchanged for H1/H2 commands.
- H3 final A-D citations may reference `problem` only; self-review draft remains visible but non-citable.
- Eval-harness visibility remains unchanged.
- H3 retains H2 strict decoder, invocation evidence, incremental persistence, subscription-only auth, no resume/retry/substitution/policy override.
- No real H3 model call before implementation/data and workflow PRs are merged and frozen.

---

### Task 1: Separate visible artifacts from citation authority

**Files:** modify `internal/council/baseline/runner.go`; test `internal/council/baseline/runner_test.go`; add `internal/council/baseline/citation_authority_test.go`.

**Interfaces:** produce `type CitationAuthority uint8`, constants `CitationAuthorityVisibleArtifacts` and `CitationAuthorityProblemOnlyFinal`, and field `Runner.CitationAuthority CitationAuthority`.
- [ ] **Step 1:** Add RED tests proving legacy self-review can cite `draft`, H3 strict self-review sees draft content but prompt lists only `problem` as citable, strict output citing `draft` returns `FailureMalformedOutput`, and strict output citing `problem` succeeds.
- [ ] **Step 2:** Run `go test ./internal/council/baseline` and verify RED on missing citation-authority symbols/behavior.
- [ ] **Step 3:** Refactor `invokeAnswer` and `renderPrompt` to receive separate `visibleIDs` and `citableIDs`; visibility grants/rendered artifacts use `visibleIDs`, prompt prints a deterministic `CITABLE_ARTIFACT_IDS` list, and parsed citations are checked against `citableIDs` with non-empty locators.
- [ ] **Step 4:** For `CitationAuthorityProblemOnlyFinal`, make final single/self-review citable IDs exactly `problem`; keep self-review final visible IDs `problem,draft`. Add self-review instruction requiring self-contained output with no draft/review/arm/provider meta-reference.
- [ ] **Step 5:** Run `go test ./internal/council/baseline ./internal/council/evalharness ./internal/council/invocationlog` and verify GREEN; commit `feat: enforce H3 citation authority`.

### Task 2: Freeze H3 dataset and run identity

**Files:** create `internal/council/benchmark/h3_types.go`, `h3_dataset_test.go`, `h3_run_store.go`, `h3_run_store_test.go`; modify `internal/council/benchmark/dataset.go`; create `benchmarks/h3/{README.md,manifest.json,rubric.json,cases.json}`.

**Interfaces:** produce `H3BenchmarkID`, H3 schema constants, `H3RiskPolicy`, `H3ChallengePolicy`, `LoadH3(root string)`, `CreateH3Run(...)`, and `WriteH3FinalResult(...)`.

- [ ] **Step 1:** Add RED tests that H3 loads only H3 identity, committed H3 matches H2 semantic payload/case hashes after removing top-level schema identity, and H3 run/final files are named `h3-run.json`/`h3-result.json`.
- [ ] **Step 2:** Run `go test ./internal/council/benchmark` and verify RED on missing H3 symbols/files.
- [ ] **Step 3:** Extend the existing versioned dataset loader with H3 configuration; copy H2 rubric/cases bytes while replacing only top-level H2 schema markers with H3 markers, recompute rubric/cases hashes, and write H3 manifest with unchanged comparator/delta/case schedule.
- [ ] **Step 4:** Implement H3 run/result store by following H2 create-only containment logic with H3 schema/file names.
- [ ] **Step 5:** Run benchmark tests plus `git diff --check`; record exact committed H3 manifest/rubric/cases SHA-256 values in `benchmarks/h3/README.md`; commit `feat: freeze H3 benchmark dataset`.
### Task 3: Add H3 runner and CLI

**Files:** create `internal/council/benchmark/h3_runner.go`, `h3_runner_test.go`, `cmd/agentd/h3_benchmark.go`, `cmd/agentd/h3_benchmark_test.go`; modify `cmd/agentd/main.go` minimally.

**Interfaces:** produce `benchmark.H3Runner`, `h3ExecutionRequest`, `h3Executor`, `runCouncilBenchmarkH3`, `executeH3Benchmark`, and `newH3RunID`.

- [ ] **Step 1:** Add RED tests for 20-case manifest-order execution, A-F sequential persistence, H3 final artifacts, `h3-*` run IDs, no policy override flags, and metered-fallback rejection.
- [ ] **Step 2:** Run `go test ./internal/council/benchmark ./cmd/agentd` and verify RED.
- [ ] **Step 3:** Implement H3 runner by reusing H2 sequential problem flow and H3 store/policies. H3 CLI wraps both provider runtimes with `invocationlog.Wrap` and constructs `baseline.Runner{CitationAuthority: baseline.CitationAuthorityProblemOnlyFinal, ...}`; H1/H2 constructors keep `CitationAuthorityVisibleArtifacts` semantics.
- [ ] **Step 4:** Run `go test ./...`, `go vet ./...`, `gofmt -l .`, `git diff --check`, and `go test -race ./internal/council/invocationlog ./internal/council/safestore ./internal/council/baseline`.
- [ ] **Step 5:** Fix any failures without making model calls; commit `feat: add H3 benchmark command`.

### Task 4: Integrate and freeze H3 implementation/data

**Files:** no new production files unless CI exposes a defect; PR includes Tasks 1-3 plus spec/plan.

- [ ] **Step 1:** Normalize accidental executable modes on Markdown/Go/test files; verify `git status --short` and `git diff --check` clean.
- [ ] **Step 2:** Push `feat/h3-citation-contract`, open a PR against `main` tracking #21 without auto-closing the issue, and record exact head SHA.
- [ ] **Step 3:** Require exact-head CI quality (gofmt/test/vet/golangci-lint) and CLA success; if a gate fails, debug/fix on the same branch and re-check exact head.
- [ ] **Step 4:** Re-fetch PR state and merge with `expected_head_sha` using squash only when `CLEAN/MERGEABLE` and all gates are green.
- [ ] **Step 5:** Fetch `main` after merge and record the resulting exact SHA as `H3_FROZEN_SHA` in Issue #21. Do not make any H3 model call yet.
### Task 5: Add frozen H3 execution workflow and run once

**Files:** create `.github/workflows/h3-frozen-execution.yml`, `scripts/bootstrap-h3-runner.sh`, and `internal/council/runnerbootstrap/h3_workflow_test.go` on a new ops branch created from `H3_FROZEN_SHA`.

- [ ] **Step 1:** Capture `H3_FROZEN_SHA=$(git rev-parse origin/main)` and create an ops branch/worktree from exactly that SHA. Add RED tests requiring manual `workflow_dispatch` only, no inputs, labels `self-hosted/linux/h3-benchmark`, checkout `ref: $H3_FROZEN_SHA`, exact H3 CLI command once, and always-upload evidence.
- [ ] **Step 2:** Run `go test ./internal/council/runnerbootstrap` and verify RED because H3 workflow/bootstrap do not exist.
- [ ] **Step 3:** Implement H3 bootstrap by adapting the proven H2 script: fail closed on metered API env, verify GitHub runner registration permission, Claude first-party subscription auth, Codex ChatGPT auth, official runner asset SHA-256, ephemeral label `h3-benchmark`.
- [ ] **Step 4:** Implement manual-only H3 workflow pinned to `H3_FROZEN_SHA`; verify dataset hashes, auth, repo tests, run `go run ./cmd/agentd council benchmark h3 --dataset benchmarks/h3` exactly once, hash `h3-run.json`, `eval/batch-summary.json`, `h3-result.json` when present, and upload full/partial evidence for 90 days.
- [ ] **Step 5:** Run bootstrap `--preflight-only` on WSL without model calls plus full gofmt/test/vet/diff-check; open ops PR, require exact-head CI/CLA, merge with expected-head guard.
- [ ] **Step 6:** Start exactly one ephemeral H3 WSL runner from the merged bootstrap, wait for `Listening for Jobs`, dispatch H3 workflow exactly once, record workflow/job/internal run IDs in Issue #21, and never redispatch automatically.
- [ ] **Step 7:** On terminal state, collect logs/artifact ZIP/digests/invocation counts/arm counts/final summary. Close #21 only if workflow succeeds and `h3-result.json` plus `eval/batch-summary.json` are present and consistent; otherwise keep #21 open with exact failure evidence and stop.

## Self-review

- Every spec requirement maps to Tasks 1-5.
- H1/H2 compatibility is explicitly tested and version-gated.
- Eval visibility is unchanged; the fix is at baseline source authority.
- Real-model execution is after both merge/freeze boundaries only.
- No placeholders or unresolved policy choices remain.