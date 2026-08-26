# Phase F Baselines Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the frozen A–F baseline arms so H1 can compare single-agent, self-review, full-information multi-agent, and blind Council outputs without changing provider/runtime isolation.

**Architecture:** Add a small `baseline` package that executes A–D directly through isolated workspaces and delegates E/F to protocol runners. Keep the existing blind `protocol.Engine` unchanged; implement E as a separate `protocol.FullInfoEngine` comparator that reuses the same internal invocation/firewall helpers while changing only which already-produced peer artifacts are visible. Both E and F preserve the same 9-call provider/phase budget.

**Tech Stack:** Go 1.26.x, existing `runtime.AgentRuntime`, `visibility.Materialize`, `protocol.Engine`, standard library JSON.

**Spec:** Issue #7 plus frozen Phase F decision: baseline E uses the same phase/call budget as Blind Council but removes inter-agent blindness so all appropriate prior artifacts are visible.

## Global Constraints

- Subscription-only Claude Code + Codex CLI; no metered fallback.
- All agent invocations use isolated workspaces outside the full run root.
- Blind Council behavior remains unchanged.
- Baseline E differs from F by visibility policy, not call budget or provider roster.
- Arms are fixed: A Claude single, B Codex single, C Claude self-review, D Codex self-review, E full-info Claude+Codex, F blind Council.
- No generic orchestration framework, database, dashboard, mailbox, or new runtime abstraction.

---

### Task 1: Full-information protocol comparator

**Files:**
- Create: `internal/council/protocol/engine_visibility_test.go`
- Create: `internal/council/protocol/fullinfo.go`

**Interfaces:**
- Produces: `protocol.FullInfoEngine` wrapping the existing `protocol.Engine` configuration.
- Full-info retains independent research, gives each reviewer both research reports, gives each rebuttal both research reports and both reviews, and retains the same challenge/judge inputs and 9-call budget.

- [x] **Step 1: Write the failing test**

Assert both full-info review prompts contain both research reports, both rebuttal prompts contain all prior peer artifacts, total runtime calls remain 9, and every invocation gets a fresh workspace outside the run root.

- [x] **Step 2: Verify RED**

Observed CI failure before production implementation because the full-info protocol API did not exist.

- [x] **Step 3: Write minimal implementation**

Implemented `FullInfoEngine` without modifying the existing blind engine or runtime/isolation boundary.

- [x] **Step 4: Verify GREEN**

Observed exact-head CI success for gofmt, tests, vet, and lint after `FullInfoEngine` implementation.

### Task 2: A–F baseline runner

**Files:**
- Create: `internal/council/baseline/runner_test.go`
- Create: `internal/council/baseline/runner.go`
- Create: `internal/council/baseline/types.go`

**Interfaces:**
- Produces: `Arm` constants `A` through `F`, `AnswerArtifact`, `ArmResult`, `RunRequest`, `Runner.RunArm`, and `Runner.RunAll`.
- A/B: one isolated final-answer invocation on Claude/Codex.
- C/D: one isolated draft invocation plus one fresh isolated self-review invocation on the same provider.
- E: `protocol.FullInfoEngine`.
- F: existing blind `protocol.Engine`.

- [x] **Step 1: Write the failing test**

Test exact call budgets/provider routing (A=1 Claude, B=1 Codex, C=2 Claude, D=2 Codex, E=9 total, F=9 total), fresh workdirs for every call, self-review prompt sees its own draft, E review prompts see both research reports, and F review prompts remain blind.

- [x] **Step 2: Verify RED**

Observed gofmt-clean CI failure before production implementation on missing baseline types/functions.

- [x] **Step 3: Write minimal implementation**

Use `visibility.Materialize` for A–D with strict JSON decoding and malformed-output classification. Delegate E/F to the full-info/blind protocol engines.

- [x] **Step 4: Verify GREEN**

Repository CI must be green on the final exact head before merge.

### Task 3: PR gate

**Files:**
- Update: PR description only.

- [ ] **Step 1: Verify exact-head repository quality**

CI must pass gofmt, `go test ./...`, `go vet ./...`, and golangci-lint on the final head.

- [ ] **Step 2: Confirm CLA**

`cla` must pass on the same final head SHA.

- [ ] **Step 3: Review diff for scope**

Confirm only this plan, full-info comparator/tests, and baseline files changed; no runtime/isolation weakening.

- [ ] **Step 4: Mark PR ready and squash merge**

Use the final exact head SHA, then re-fetch PR and `main` after merge.
