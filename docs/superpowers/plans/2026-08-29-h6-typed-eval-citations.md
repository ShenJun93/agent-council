# H6 Typed Evaluator Citations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Version the evaluator citation wire contract so H6 uses typed citation keys and can complete the H5 failure fixture without free-form string normalization.

**Architecture:** Add an opt-in H6 citation contract in evalharness and a matching internal structured-output schema profile. Keep legacy zero-value H1-H5 behavior unchanged. Version H5 benchmark identity to H6 while preserving dataset semantics and adapter topology.

**Tech Stack:** Go 1.26, existing evalharness/structuredoutput/benchmark packages, Claude/Codex/Antigravity subscription adapters, human ChatGPT broker, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-08-29-h6-typed-eval-citations-design.md`

## Global Constraints
- Never mutate or rerun frozen H5.
- H6 only; zero-value legacy behavior remains H1-H5 compatible.
- No output repair or delimiter normalization.
- No provider-policy override or metered API fallback.
- No real H6 model call before implementation/data freeze SHA is recorded.

---
### Task 1: H6 typed evaluator citation contract

**Files:** modify `internal/council/evalharness/{types.go,harness.go,harness_test.go}`; create `internal/council/evalharness/h6_citations_test.go`.

**Interfaces:** add `CitationContract` with zero-value legacy and `CitationContractStructuredV1`; add internal typed wire structs `CitationKey{ArtifactID,Locator}`, `H6CitationCheck`, `H6JudgeArtifact`.

- [ ] Write RED test reproducing H5 failure: candidate has `{artifact_id:"problem", locator:"constraints[0]"}` and H6 judge returns typed key; legacy string variant remains rejected on H6 path.

```go
h := Harness{CitationContract: CitationContractStructuredV1}
// typed relied_on_citations must validate against candidate.Citations.
```

- [ ] Run `go test ./internal/council/evalharness -run 'H6|Legacy'`; expect H6 symbols missing.
- [ ] Implement H6 prompt, strict typed decode/validation, duplicate checks, and deterministic canonical serialization only after validation.
- [ ] Re-run focused tests plus all evalharness tests; commit `fix: type H6 evaluator citation references`.
### Task 2: Versioned structured-output schema profile

**Files:** modify `internal/council/structuredoutput/{runtime.go,schemas.go,runtime_test.go}`; add `h6_schema_test.go`.

**Interfaces:** add `SchemaProfile` with zero-value H4/H5 behavior and `SchemaProfileH6`; add `WrapProfile(inner, profile)` while keeping `Wrap(inner)` unchanged.

- [ ] RED tests prove H6 eval-judge schema uses closed `{artifact_id,locator}` reference objects while H5/default schema digest and shape remain unchanged.

```go
legacy := Wrap(inner)
h6 := WrapProfile(inner, SchemaProfileH6)
```

- [ ] Run focused structuredoutput tests and confirm RED on missing profile API.
- [ ] Implement profile-aware schema selection; reuse legacy schemas byte-for-byte for every non-eval role/phase.
- [ ] Add regression proving callers still cannot inject arbitrary `AgentRequest.OutputSchema`.
- [ ] Run `go test ./internal/council/structuredoutput ./internal/council/invocationlog` and race tests; commit `feat: add H6 structured output profile`.
### Task 3: H6 dataset, runner, CLI, and store

**Files:** create `benchmarks/h6/*`; create H6 benchmark runner/store files patterned on H5; create `cmd/agentd/h6_benchmark.go`; minimally extend command routing/help.

**Interfaces:** `LoadH6`, H6 run/result schema constants, `H6Runner`; H6 runtime constructors use `CitationContractStructuredV1` and `SchemaProfileH6` while retaining H5 adapter policy semantics.

- [ ] RED tests prove H6 semantic equality with H5, adapter-policy byte equality, H6 fresh run IDs, no policy override, and H6 constructor selects both typed evaluator contract and H6 schema profile.
- [ ] Generate H6 files from H5 with only H6 identity/schema markers; recompute manifest/rubric/cases hashes while keeping `adapter-policy.json` byte-identical.
- [ ] Implement H6 runner/store/CLI by versioning H5 mechanics without changing failover ordering or human-broker behavior.
- [ ] Add full-run fixture where the prior H5 citation shape succeeds as typed H6 output; malformed/unknown typed key remains terminal without adapter failover.
- [ ] Run H1-H6 compatibility suites, `go test ./...`, `go vet ./...`, race tests, `git diff --check`, and golangci-lint v2.12.2.
- [ ] Commit `feat: add H6 typed evaluator benchmark`.

### Task 4: Implementation PR and freeze

- [ ] Push branch and open PR tracking #38 with exact H6 hashes and local verification evidence.
- [ ] Require exact-head CI quality + CLA success; audit changed-file scope and H1-H5 frozen paths.
- [ ] Squash merge using expected-head guard, verify `main`, and post `H6_FROZEN_SHA` to #38.
### Task 5: Frozen H6 execution

**Files:** generate `.github/workflows/h6-frozen-execution.yml`; extend generic renderer/bootstrap only if required under TDD.

- [ ] Generate workflow pinned to `H6_FROZEN_SHA`, H6 dataset hashes, and adapter-policy hash; manual dispatch only, always-upload evidence, no retry/resume.
- [ ] Open separate ops PR; require exact-head CI/CLA and merge with expected-head guard.
- [ ] Remote-first preflight: reject metered keys; accept serviceability through any approved automated adapter or the frozen human broker path; start one ephemeral `h6-benchmark` runner.
- [ ] Dispatch exactly once using the idempotent helper and write the run/job/runner marker to #38.
- [ ] Monitor terminal. If human broker is reached, surface the immutable packet for a brand-new ChatGPT New Chat and accept one raw response submission only.
- [ ] On success independently verify `h6-result.json`, `eval/batch-summary.json`, adapter/failover evidence, key SHA-256 values and artifact digest, then close #38. On failure preserve evidence and do not create attempt #2 automatically.

## Self-review checklist
- Every design requirement maps to Tasks 1-5.
- No H1-H5 mutation is required.
- H6 schema profile and evaluator contract names are consistent across tasks.
- No placeholders or unspecified retry behavior remain.
