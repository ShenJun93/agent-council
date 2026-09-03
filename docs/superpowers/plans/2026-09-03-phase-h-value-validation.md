# Phase H Technical Value-Validation Pilot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and run the frozen ten-case technical Phase H evaluator-only replay pilot with deterministic PASS/FAIL/INCONCLUSIVE handling.

**Architecture:** Reuse the H9 replay pattern and H8/V3 evaluator contract, but load every complete H8 technical A-F candidate set. Add Phase H-specific dataset validation, value diagnostics, CLI wiring, and a one-shot workflow; do not change H1-H9 or regenerate candidates.

**Tech Stack:** Go, existing benchmark/evalharness/humanbroker packages, JSON frozen dataset, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-09-03-phase-h-value-validation-design.md`

## Global Constraints

- Exactly 10 frozen technical cases × 6 arms × 2 judges = 120 evaluator invocations.
- Source H8 workflow `33349114073`, frozen SHA `8d13f0a82758f5ea6286409d55123b97929dbce4`, run `h8-20260831T015809Z-e8ab51986046`, artifact ID `9745340503`, digest `sha256:c37cbab9cb4d1a8c5b4f6cc2535268c681456d2b6380587fb178396ae9dedfd6`.
- Comparator `best_single`; material-worse delta exactly `10.0`.
- H8/V3 citation semantics and current-session H9 broker contract remain unchanged.
- No retry, failover, provider substitution, candidate regeneration, or metered fallback.
- PASS iff mean council delta > 0 and materially-worse count == 0; incomplete execution is INCONCLUSIVE.

### Task 1: Freeze H8-derived technical replay dataset

- [ ] Write failing `TestLoadPhaseHReplayAcceptsCommittedFrozenReplay` and mutation-policy tests.
- [ ] Run focused test and confirm RED because Phase H loader does not exist.
- [ ] Copy exact problem/reference/rubric bytes and A-F bytes for the ten complete technical H8 cases into `benchmarks/phase-h`.
- [ ] Generate strict `manifest.json` and ChatGPT-web-only `adapter-policy.json` with exact hashes/provenance.
- [ ] Commit dataset + RED tests.

### Task 2: Implement strict Phase H loader

- [ ] Add `internal/council/benchmark/phase_h_replay.go` with constants/types/loader and adapter-policy validation, initially only enough to make loader tests green.
- [ ] Validate exact ten IDs/order, six arms, hashes, provenance, policy, 120 topology, and session flags.
- [ ] Run focused tests green and full benchmark tests green.
- [ ] Commit loader.

### Task 3: Add value diagnostics/classification

- [ ] Write RED tests for positive PASS, zero FAIL, materially-worse FAIL, win/tie/loss and judge disagreement.
- [ ] Add pure Phase H value summary and classification functions.
- [ ] Run focused tests green and commit.

### Task 4: Implement evaluator replay runner/result persistence

- [ ] Write RED tests using fake evaluator/adapter summary for ten cases and fail-closed behavior.
- [ ] Implement `PhaseHReplayRunner` by composing existing `EvalExecutor`, `evalharness.SummarizeBatch`, `WriteEvaluation`, adapter summary collection, and immutable stores.
- [ ] Require successful/human-broker counts exactly 120, diversity 1/1, failures/failovers 0.
- [ ] Write `phase-h-result.json` only after complete success and include value summary + source hashes.
- [ ] Run replay/eval tests green and commit.

### Task 5: Add `agentd council benchmark phase-h`

- [ ] Write RED CLI tests following H9 conventions.
- [ ] Add `cmd/agentd/phase_h_benchmark.go`, H8 schema profile, current-session human broker, no alternate adapter path.
- [ ] Register command in existing dispatch surface; run CLI tests green; commit.

### Task 6: Add governed one-shot workflow

- [ ] Add workflow-contract RED tests mirroring H9 invariants.
- [ ] Create `.github/workflows/phase-h-frozen-execution.yml` using labels `self-hosted`, `linux`, `phase-h-benchmark`.
- [ ] Bind frozen SHA, issue #56 marker/current-session attestation; verify hashes/provenance; reject metered keys; test before calls; upload audit always.
- [ ] Run workflow contract tests green and commit.

### Task 7: Full verification and PR

- [ ] `gofmt`, `go test ./...`, `go vet ./...`, and current lint/CI-equivalent checks PASS.
- [ ] Verify `benchmarks/h9` and H9 workflow unchanged and zero Phase H real runs occurred.
- [ ] Push/open PR linked to #56, inspect exact-head CI/review, fix actionable issues, merge when green.
- [ ] Record frozen merged SHA/tree/manifest/rubric/policy hashes on #56.

### Task 8: Execute exactly one governed real pilot

- [ ] Confirm zero prior Phase H runs; post marker `issue-56-phase-h-real-run-1` with frozen SHA.
- [ ] Provision ephemeral `phase-h-benchmark` runner and dispatch once with current-session attestation.
- [ ] Broker all 120 evaluator calls through this current ChatGPT conversation with no retry/failover.
- [ ] Record terminal workflow/internal-run/result/artifact identities and one-shot cardinality.

### Task 9: Close Phase H truthfully

- [ ] Update README/future-runtime status strictly from immutable terminal outcome.
- [ ] PASS wording is limited to the frozen technical pilot; product value remains untested and concrete-pain API/BYOK gate remains separate.
- [ ] FAIL/INCONCLUSIVE keeps measurable-value gate unsatisfied.
- [ ] Run full tests/vet, merge status-only follow-up if required, post evidence to #56 and close completed.

## Self-Review

The revised plan removes the impossible 20-case replay assumption, preserves source integrity, and maps the complete ten-case H8 corpus through dataset freeze, loader, diagnostics, replay, CLI, workflow, PR, one-shot execution, and terminal status update. No implementation task requires regenerated candidates or a product artifact that does not exist.
