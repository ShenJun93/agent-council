# H4 Native Structured Output Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add H4 as a new frozen benchmark version whose every model invocation uses provider-native JSON Schema structured output while preserving H3 deliberation/evaluation semantics.

**Architecture:** Add an optional schema field to the runtime request, teach Claude/Codex transports to enforce it natively, and add a focused H4-only schema-injecting runtime wrapper selected by role/phase. H4 reuses H3 runner/eval/citation semantics and gets independent dataset/run/result identity; its real workflow is added only after implementation/data are merged and frozen.

**Tech Stack:** Go 1.26, Claude Code CLI, Codex CLI, JSON Schema, GitHub Actions, WSL2/Linux self-hosted runner.

**Spec:** `docs/superpowers/specs/2026-08-27-h4-native-structured-output-design.md`

## Global Constraints

- H3 run `33067558993` is frozen/inconclusive and must not be rerun.
- No JSON repair, malformed-output retry, resume, provider substitution, policy override, or metered fallback.
- H1-H3 requests with no output schema retain their existing CLI argument shape and behavior.
- H4 preserves H3 cases, rubric, references, order, citation authority, visibility, challenge policy, evaluator, comparator, and `MaterialWorseDelta=10.0`.
- No real H4 model call before implementation/data and a separately pinned workflow are merged and frozen.

---
### Task 1: Add optional structured-output transport to runtime

**Files:** modify `internal/council/runtime/runtime.go`, `runtime_test.go`; add `structured_output_test.go`.

**Interfaces:**
- Produces `AgentRequest.OutputSchema json.RawMessage`.
- Claude non-empty schema adds `--json-schema <compact-json>`.
- Codex non-empty schema adds `--output-schema <temporary-file>` whose bytes equal compact schema and which is removed after execution.

- [ ] **Step 1:** Add RED runtime tests proving schema-free Claude/Codex args are unchanged, Claude receives inline schema, Codex receives a readable schema file during process execution, invalid/non-object schemas never invoke the agent process, and Codex schema temp files are removed after success and failure.
- [ ] **Step 2:** Run `go test ./internal/council/runtime -run 'Structured|Schema|Legacy'` and verify RED on missing `OutputSchema`/schema args.
- [ ] **Step 3:** Add `OutputSchema json.RawMessage`; validate and compact it with `json.Valid` plus object decoding before execution. Build provider args without changing the schema-free path. Materialize Codex schema with `os.CreateTemp("", "agent-council-output-schema-*.json")`, `Chmod(0600)`, close before execution, and defer removal.
- [ ] **Step 4:** Run `go test ./internal/council/runtime` plus `go test -race ./internal/council/runtime`; verify GREEN and `git diff --check` clean.
- [ ] **Step 5:** Commit `feat: add native structured output transport`.

### Task 2: Add frozen H4 schema injection and evidence digest

**Files:** create `internal/council/structuredoutput/{runtime.go,schemas.go,runtime_test.go,schemas_test.go}`; modify `internal/council/invocationlog/{runtime.go,runtime_test.go}`.

**Interfaces:**
- Produces `structuredoutput.Wrap(inner councilruntime.AgentRuntime) councilruntime.AgentRuntime`.
- Produces `structuredoutput.SchemaFor(role, phase string) (json.RawMessage, error)`.
- Adds optional `Evidence.OutputSchemaSHA256 string ` + "`json:\"output_schema_sha256,omitempty\"`" + `.

- [ ] **Step 1:** Add RED tests for all H4 mappings: baseline draft/final→Answer, research→Research, review→Review, challenge→Challenge, rebuttal→Rebuttal, protocol judge→protocol Judge, eval-judge→eval Judge. Test unknown role/phase fails before inner runtime; pre-populated schema is rejected to prevent accidental override.
- [ ] **Step 2:** Add RED schema-parity tests that reflect JSON tags from `baseline.AnswerArtifact`, protocol artifact structs/nested citation structs, and `evalharness.JudgeArtifact`; assert schema `properties`, `required`, and `additionalProperties:false` match all required output fields.
- [ ] **Step 3:** Implement frozen schemas with only provider-portable object/array/string/number/boolean rules. Keep `evalharness.JudgeArtifact.dimensions` as object with numeric `additionalProperties`; existing evaluator validates exact rubric dimension IDs and ranges.
- [ ] **Step 4:** Update invocation evidence to hash the exact compact output schema when non-empty. Test legacy evidence omits the field and H4 evidence records the expected SHA-256. Run `go test ./internal/council/structuredoutput ./internal/council/invocationlog` and race tests.
- [ ] **Step 5:** Commit `feat: freeze H4 structured output schemas`.

### Task 3: Add H4 dataset, runner, CLI, and schema-enabled constructor

**Files:** create `benchmarks/h4/{README.md,manifest.json,rubric.json,cases.json}`; create H4 benchmark types/store/runner tests and production files following H3 versioned patterns; create `cmd/agentd/h4_benchmark.go` and tests; modify `internal/council/benchmark/dataset.go` and `cmd/agentd/main.go` minimally.

**Interfaces:**
- Produces `H4BenchmarkID`, H4 schema constants, `H4RiskPolicy`, `H4ChallengePolicy`, `LoadH4`, `CreateH4Run`, `WriteH4FinalResult`, `H4Runner`.
- Produces `h4ExecutionRequest`, `h4Executor`, `runCouncilBenchmarkH4`, `executeH4Benchmark`, `newH4RunID`.
- H4 provider construction is `structuredoutput.Wrap(invocationlog.Wrap(New*CLI(...), provider))` for both Claude and Codex.

- [ ] **Step 1:** Add RED tests that H4 dataset accepts only H4 identity, is semantically equal to H3 after identity removal, and uses `h4-run.json`/`h4-result.json`.
- [ ] **Step 2:** Add RED runner/CLI tests for 20-case manifest-order execution, incremental A-F persistence, `h4-*` IDs, no policy override flags, metered-fallback rejection, and schema-enabled runtimes passed to both baseline and evaluator.
- [ ] **Step 3:** Copy H3 rubric/case semantic payloads, replace only top-level H3 schema markers with H4 markers, recompute rubric/cases/manifest SHA-256, and record exact hashes in `benchmarks/h4/README.md`.
- [ ] **Step 4:** Implement H4 store/runner by versioning H3 patterns without changing baseline/eval semantics. Implement H4 CLI route and constructor with `CitationAuthorityProblemOnlyFinal` and the H4 schema wrappers; preserve H1-H3 router helpers/tests.
- [ ] **Step 5:** Run `go test ./...`, `go vet ./...`, `gofmt -l .`, `git diff --check`, `go test -race ./internal/council/runtime ./internal/council/structuredoutput ./internal/council/invocationlog ./internal/council/baseline`, and `golangci-lint v2.12.2 run ./...`; commit `feat: add H4 structured-output benchmark`.

### Task 4: Integrate and freeze H4 implementation/data

**Files:** PR contains design, plan, Tasks 1-3; no execution workflow yet.

**Interfaces:** produces exact `H4_FROZEN_SHA` on `main`.

- [ ] **Step 1:** Normalize file modes (`*.go`, `*.md`, JSON/YAML 0644; executable scripts only 0755), verify clean status/diff-check, and rerun full quality gates on exact branch head.
- [ ] **Step 2:** Push `feat/h4-structured-output`, open a PR against `main` tracking #24 without auto-closing it, and record exact head SHA plus frozen H4 dataset hashes.
- [ ] **Step 3:** Require exact-head CI quality (gofmt/test/vet/golangci-lint) and CLA success. Any failure is debugged on the same branch with a new exact-head gate cycle.
- [ ] **Step 4:** Re-fetch PR state and squash merge only when mergeable and all exact-head gates are terminal success, using `expected_head_sha`.
- [ ] **Step 5:** Fetch `main`, record resulting signed merge SHA as `H4_FROZEN_SHA` in #24, and verify no H4 workflow/model run exists yet.

### Task 5: Add pinned H4 workflow/bootstrap and execute exactly once

**Files:** create `.github/workflows/h4-frozen-execution.yml`, `scripts/bootstrap-h4-runner.sh`, `internal/council/runnerbootstrap/h4_workflow_test.go`, `h4_bootstrap_test.go` on an ops branch created from exact `H4_FROZEN_SHA`.

**Interfaces:** manual-only workflow; ephemeral runner label `h4-benchmark`.

- [ ] **Step 1:** Add RED workflow/bootstrap tests requiring `workflow_dispatch` only/no inputs, labels `self-hosted/linux/h4-benchmark`, exact frozen checkout, exact dataset hashes, exactly one H4 command, subscription auth, fail-closed metered env, and always-upload evidence for 90 days.
- [ ] **Step 2:** Implement H4 bootstrap by versioning the proven H3 script: GitHub registration permission, Claude first-party subscription auth, Codex ChatGPT auth, official runner asset SHA-256, Linux-only ephemeral registration, label `h4-benchmark`; run `--preflight-only` without model calls.
- [ ] **Step 3:** Implement H4 workflow pinned to `H4_FROZEN_SHA`; verify frozen dataset hashes/auth/repo tests, execute `go run ./cmd/agentd council benchmark h4 --dataset benchmarks/h4` exactly once, hash key/all artifacts, and upload full/partial evidence even on failure.
- [ ] **Step 4:** Run full local gates plus bootstrap preflight, push ops branch, open PR, require exact-head CI/CLA, and squash merge with `expected_head_sha`.
- [ ] **Step 5:** Verify merged workflow on `main`; pre-audit that zero H4 workflow runs and zero `h4-benchmark` runners exist; start exactly one ephemeral runner and wait for `Listening for Jobs`.
- [ ] **Step 6:** Dispatch H4 workflow exactly once. Record workflow run ID, job ID, runner ID/name, trigger SHA, frozen SHA, and internal run ID in #24. Never redispatch automatically.
- [ ] **Step 7:** On terminal state, download logs/artifact ZIP, verify ZIP/key-artifact SHA-256, frozen SHA/dataset hashes, invocation schema digests, A-F counts, problem count, `eval/batch-summary.json`, and `h4-result.json`. Close #24 only on a consistent successful final result; otherwise keep it open, preserve evidence, classify the exact initiating failure, and create a new version boundary for any semantic/transport change.

## Self-review

- Runtime schema transport, provider-native args, temp-file cleanup, and legacy compatibility map to Task 1.
- Frozen schemas, mapping coverage, struct parity, and audit digests map to Task 2.
- H4 identity/data/runner/CLI and all-model-call schema activation map to Task 3.
- Exact-head integration/freeze is isolated from real execution in Task 4.
- One-shot subscription-authenticated real execution with no rerun is isolated in Task 5.
- No placeholder policy choices remain; H3 is never mutated or rerun.
