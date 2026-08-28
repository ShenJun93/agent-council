# H5 Provider-Agnostic Adapter Failover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build H5 so logical council roles can transparently fail over between frozen subscription-backed adapter chains when a provider is unavailable, without changing H1-H4 behavior.

**Architecture:** Add a small adapter-pool layer above existing provider runtimes, versioned invocation evidence for adapter attempts, and optional logical-slot bindings in baseline/protocol/evaluator. H5 commits the adapter registry and per-slot chains into its benchmark hash boundary and uses existing structured-output plus generic execution tooling.

**Tech Stack:** Go 1.26, Claude Code CLI, Codex CLI authenticated with ChatGPT, GitHub Actions, existing safestore/visibility/structuredoutput packages.

**Spec:** `docs/superpowers/specs/2026-08-28-h5-adapter-failover-design.md`

## Global Constraints
- H1-H4 frozen benchmark files/workflows/scripts remain unchanged.
- Metered API credentials remain forbidden.
- H5 same-adapter max attempts is exactly 1; legacy zero-value behavior remains 2.
- Failover only on availability classes; malformed/semantic/isolation/billing failures never fail over.
- Raw provider output is persisted before structured-output normalization.
- No real H5 model call before implementation/data merge and exact frozen SHA are recorded.
- Issue #31 is the lifecycle record.

---

### Task 1: Runtime attempt control and adapter pool

**Files:** create `internal/council/adapterpool/{types.go,runtime.go,runtime_test.go}`; modify `internal/council/runtime/runtime.go` and tests.

**Interfaces:** `AdapterID`, `SlotID`, `Adapter{ID,Provider,Runtime}`, `Policy{Slot,Chain}`, `New(registry, policy) AgentRuntime`; optional `AgentRequest.MaxAttempts`, `AgentRequest.SlotID`, adapter metadata in `AgentResponse`; `FailureAdapterUnavailable`, `FailureAdapterPoolExhausted`.

- [ ] Write RED tests proving quota/auth fail over primary?secondary once; malformed/process/isolation do not fail over; exhausted pool returns ordered terminal error; H5 request forces one same-adapter attempt; legacy request still allows two process attempts.
- [ ] Implement safe ID/policy validation, availability-class traversal across joined errors, deterministic chain execution, and response adapter metadata.
- [ ] Run `go test ./internal/council/runtime ./internal/council/adapterpool` and `go test -race ./internal/council/adapterpool`.
- [ ] Commit `feat: add provider adapter failover pool`.
### Task 2: Invocation evidence v2

**Files:** modify `internal/council/invocationlog/runtime.go`; add adapter-evidence tests.

**Interfaces:** keep legacy `Wrap`; add `WrapAdapter(inner, AdapterMetadata)` where metadata contains adapter ID/provider family. V2 evidence consumes request slot/failover fields and emits `council.invocation-evidence.v2`.

- [ ] RED tests: quota/auth failure with zero inner response still writes v2 evidence; successful Claude/Codex attempts preserve raw stdout; failover index/trigger and hashes are exact; legacy Wrap remains byte-shape compatible with v1 expectations.
- [ ] Implement wrapper-level timestamps only for zero-response adapter attempts, failure-class capture, adapter path namespace, and immutable create-only writes.
- [ ] Integration test `adapterpool -> structuredoutput -> invocationlog` proving first quota evidence and second successful raw evidence both exist while caller sees normalized payload.
- [ ] Run invocationlog/adapterpool race tests and commit `feat: preserve adapter failover evidence`.

### Task 3: Logical slot wiring

**Files:** modify `internal/council/protocol/{types.go,engine.go,fullinfo.go}`; `internal/council/baseline/{types.go,runner.go}`; `internal/council/evalharness/harness.go` and focused tests.

**Interfaces:** `protocol.SlotRuntimes`; optional baseline slot-A/slot-B plus council slots; optional eval judge slots. Legacy Claude/Codex fields remain fallback.

- [ ] RED protocol tests run researcher/reviewer/challenger/judges with swapped providers and prove prompts/visibility remain unchanged.
- [ ] RED baseline tests prove A/C use slot-A and B/D slot-B; add H5 aliases for A-F without changing wire values.
- [ ] RED evaluator tests prove adaptive judge slots accept actual provider after failover, record that provider, and preserve fixed-provider legacy rejection behavior.
- [ ] Implement slot resolution helpers and remove provider assumptions only on the adaptive path.
- [ ] Add execution provenance fields with `omitempty` so legacy JSON remains stable when no adapter metadata exists.
- [ ] Run baseline/protocol/eval suites and commit `refactor: bind council roles through logical slots`.

### Task 4: Frozen H5 dataset and adapter policy

**Files:** create `benchmarks/h5/{README.md,manifest.json,cases.json,rubric.json,adapter-policy.json}`; extend benchmark H5 loader/types/store tests.

**Interfaces:** H5 policy declares adapter registry, provider family, slot chains, availability classes, max attempts, and challenger primary orientation. Manifest includes SHA-256 for adapter-policy in addition to dataset/rubric/cases.

- [ ] RED tests load H5, reject policy hash drift/unknown adapter/empty chain/duplicate adapter/unsupported availability class, and prove H4?H5 problem/reference/rubric semantic equality.
- [ ] Generate H5 data from H4 with only schema/version/provider-binding metadata changes; preserve all decision evidence and reference hashes semantically.
- [ ] Freeze policy chains: A-side Claude-first/Codex-second, B-side Codex-first/Claude-second; challenger primary follows H4 odd/even schedule with deterministic secondary.
- [ ] Record exact H5 manifest/rubric/cases/policy hashes and commit `feat: freeze H5 adapter benchmark dataset`.
### Task 5: H5 runner, CLI, and realized binding summary

**Files:** create versioned H5 benchmark runner/store files and `cmd/agentd/h5_benchmark.go`; minimally extend command router/help.

**Interfaces:** construct concrete adapters `claude-max` and `codex-chatgpt`, each as `CLI -> WrapAdapter -> structuredoutput`; create slot pools from frozen policy; H5 runner emits adapter/failover summary in result.

- [ ] RED tests prove CLI has no provider-policy override, metered fallback remains rejected, and real constructors are provider-agnostic slots rather than fixed Claude reviewer/judge fields.
- [ ] RED full-run tests inject quota on one adapter and prove problem completes through fallback; inject malformed output and prove immediate terminal failure without fallback.
- [ ] Implement H5 run/store schemas, fresh `h5-*` IDs, incremental arm persistence, adaptive evaluator, policy/hash provenance, realized adapter trace and effective-diversity/failover counts.
- [ ] Run H1-H5 compatibility suites, full repository tests, vet, race for adapter/invocation packages, and exact golangci-lint v2.12.2.
- [ ] Commit `feat: add H5 adapter benchmark command`.

### Task 6: Implementation PR and freeze

**Files:** no new semantic code unless CI identifies a defect.

- [ ] Verify `gofmt`, `go test ./...`, `go vet ./...`, adapter/invocation race tests, `git diff --check`, golangci-lint v2.12.2.
- [ ] Push branch and open PR tracking #31 with exact local head and frozen hashes.
- [ ] Require exact-head CI + CLA success, self-review changed-file scope, then squash merge with expected-head guard.
- [ ] Verify `main` exact merge SHA and post it to #31 as `H5_FROZEN_SHA`.

### Task 7: Frozen execution via generic tooling

**Files:** generated `.github/workflows/h5-frozen-execution.yml`; no H5 semantic mutation.

- [ ] Use `render-frozen-benchmark-workflow.sh` with H5 frozen SHA; extend renderer if required to include adapter-policy hash, under TDD and without rewriting H1-H4 workflows.
- [ ] Use generic bootstrap preflight that accepts the run when at least one approved adapter can serve every required slot; it must not require Claude specifically when Codex can cover all chains.
- [ ] Open separate ops PR, require exact-head CI/CLA, merge, then start one ephemeral remote-first runner.
- [ ] Dispatch exactly once with generic idempotent helper, collect Issue #31 marker, and monitor to terminal.
- [ ] On success verify final H5 result, batch summary, adapter evidence v2, failover trace/hashes; close #31. On terminal failure preserve evidence and do not create another attempt automatically.