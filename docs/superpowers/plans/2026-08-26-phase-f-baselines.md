# Phase F Baselines Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the frozen A–F baseline arms so H1 can compare single-agent, self-review, full-information multi-agent, and blind Council outputs without changing provider/runtime isolation.

**Architecture:** Add a small `baseline` package that executes A–D directly through isolated workspaces and delegates E/F to the existing protocol engine. Extend the protocol engine with an explicit visibility mode whose zero value remains blind; the full-info mode changes only which already-produced artifacts are materialized to later phases, preserving the same 9-call phase/call budget. Baseline outputs are structured wrappers suitable for the Phase G evaluation harness.

**Tech Stack:** Go 1.26.x, existing `runtime.AgentRuntime`, `visibility.Materialize`, `protocol.Engine`, standard library JSON.

**Spec:** Issue #7 plus frozen Phase F decision: baseline E uses the same phase/call budget as Blind Council but removes inter-agent blindness so all appropriate prior phase artifacts are visible.

## Global Constraints

- Subscription-only Claude Code + Codex CLI; no metered fallback.
- All agent invocations use isolated workspaces outside the full run root.
- Blind Council behavior must remain unchanged by default.
- Baseline E differs from F by visibility policy, not call budget or provider roster.
- Arms are fixed: A Claude single, B Codex single, C Claude self-review, D Codex self-review, E full-info Claude+Codex, F blind Council.
- No generic orchestration framework, database, dashboard, mailbox, or new runtime abstraction.

---

### Task 1: Freeze protocol visibility behavior with RED tests

**Files:**
- Create: `internal/council/protocol/engine_visibility_test.go`
- Modify: `internal/council/protocol/types.go`
- Modify: `internal/council/protocol/engine.go`

**Interfaces:**
- Produces: `type VisibilityMode string`, `VisibilityBlind`, `VisibilityFullInfo`, and `Engine.VisibilityMode`.
- Full-info mode retains independent research but gives each review/rebuttal invocation all appropriate already-produced peer artifacts; judges keep their existing complete content view.

- [ ] **Step 1: Write the failing test**

Create a protocol test that runs the existing fake Claude/Codex runtimes with `VisibilityFullInfo` and asserts both review prompts contain both research reports, both rebuttal prompts contain both research reports and both reviews, the total call count remains 9, and every invocation gets a fresh workspace outside the run root. Existing blind tests must continue asserting exactly one target report is visible to each reviewer.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/council/protocol`
Expected: FAIL because `VisibilityFullInfo` / `Engine.VisibilityMode` do not exist.

- [ ] **Step 3: Write minimal implementation**

Add the visibility enum and route the allowed artifact IDs through one helper. Keep the zero value blind. Do not alter research visibility; do not expose provenance/provider identity.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/council/protocol`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit message: `feat: add full-info protocol visibility mode`.

### Task 2: Implement A–F baseline runner with RED tests

**Files:**
- Create: `internal/council/baseline/runner_test.go`
- Create: `internal/council/baseline/runner.go`
- Create: `internal/council/baseline/types.go`

**Interfaces:**
- Produces: `Arm` constants `A` through `F`, `AnswerArtifact`, `ArmResult`, `RunRequest`, `Runner.RunArm`, and `Runner.RunAll`.
- A/B: one isolated final-answer invocation on Claude/Codex.
- C/D: one isolated draft invocation plus one fresh isolated self-review invocation on the same provider.
- E: `protocol.Engine` with `VisibilityFullInfo`.
- F: `protocol.Engine` with default blind visibility.

- [ ] **Step 1: Write the failing test**

Test exact call budgets/provider routing (A=1 Claude, B=1 Codex, C=2 Claude, D=2 Codex, E=9 total, F=9 total), fresh workdirs for every call, self-review prompt sees its own draft, E review prompts see both research reports, and F review prompts remain blind.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/council/baseline`
Expected: FAIL because baseline package production types/functions are missing.

- [ ] **Step 3: Write minimal implementation**

Use `visibility.Materialize` for A–D and strict JSON decoding with malformed-output classification. Delegate E/F to `protocol.Engine`; do not duplicate the nine-phase protocol.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/council/baseline ./internal/council/protocol`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit message: `feat: implement Phase F baseline arms`.

### Task 3: Verify repository quality and PR gate

**Files:**
- Update: PR description only.

**Interfaces:**
- Consumes: Phase F implementation.
- Produces: merge-ready Phase F branch with exact-head green `quality` and `cla` checks.

- [ ] **Step 1: Run repository verification**

Run in CI: gofmt check, `go test ./...`, `go vet ./...`, golangci-lint.
Expected: all pass.

- [ ] **Step 2: Confirm CLA**

Expected: `cla` passes on the same exact head SHA.

- [ ] **Step 3: Review diff for scope**

Confirm only plan, protocol visibility support, and baseline files changed; no runtime/isolation weakening.

- [ ] **Step 4: Mark PR ready and squash merge**

Use expected exact head SHA. Re-fetch PR and `main` after merge.
