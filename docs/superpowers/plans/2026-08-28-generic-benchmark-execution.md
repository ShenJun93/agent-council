# Generic Frozen Benchmark Execution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace per-version execution copy/paste for H5+ with generic, fail-closed runner, workflow-rendering, and dispatch tooling while preserving static frozen workflows.

**Architecture:** Three shell tools share a strict benchmark-id contract but remain operationally separate: bootstrap keeps the foreground ephemeral runner alive, renderer creates a static audit workflow after a benchmark SHA is frozen, and dispatch performs one idempotent GitHub mutation. Existing H1-H4 scripts/workflows remain immutable.

**Tech Stack:** Bash, GitHub CLI, GitHub Actions YAML, Go contract tests.

**Spec:** `docs/superpowers/specs/2026-08-28-generic-benchmark-execution-design.md`

## Global Constraints

- Do not modify H1-H4 frozen workflows or bootstrap scripts.
- Do not merge this branch before H4 attempt #2 reaches terminal state.
- No retry/resume/provider substitution/metered fallback/policy overrides.
- Static generated workflow remains the reviewed execution artifact.
- TDD, shell syntax, gofmt/test/vet/lint, and diff-check are mandatory.

---
### Task 1: Generic subscription runner bootstrap

**Files:**
- Create: `scripts/bootstrap-benchmark-runner.sh`
- Test: `internal/council/runnerbootstrap/generic_bootstrap_test.go`

**Interfaces:**
- Consumes: `--benchmark <safe-id>`, optional `--preflight-only`.
- Produces: ephemeral runner label `<id>-benchmark` and success marker `<ID> runner preflight OK`.

- [ ] Write failing tests for safe/unsafe benchmark IDs, subscription-auth success, metered-key rejection, Linux guard, and fixed ephemeral label/name behavior using fake `gh`, `claude`, and `codex` binaries.
- [ ] Run `go test ./internal/council/runnerbootstrap -run GenericBootstrap` and verify RED because the script is missing.
- [ ] Implement the minimal generic bootstrap by extracting only the proven H4 logic into the new script; leave `bootstrap-h1-runner.sh` through `bootstrap-h4-runner.sh` untouched.
- [ ] Run the focused tests and `bash -n scripts/bootstrap-benchmark-runner.sh`; require GREEN.
- [ ] Commit as `feat: add generic benchmark runner bootstrap`.

### Task 2: Deterministic frozen workflow renderer

**Files:**
- Create: `scripts/render-frozen-benchmark-workflow.sh`
- Test: `internal/council/runnerbootstrap/generic_workflow_renderer_test.go`

**Interfaces:**
- Consumes: `--benchmark <safe-id> --frozen-sha <40hex> --output <path>` plus committed `benchmarks/<id>` files.
- Produces: static manual-only frozen workflow YAML.
- [ ] Write RED tests that render H4 into a temp path and assert exact frozen SHA, current H4 dataset hashes, manual-only/no-input trigger, `<id>-benchmark` label, exactly one `go run ... benchmark <id>` command, audit/result paths, `if: always()`, and 90-day retention.
- [ ] Add rejection tests for unsafe IDs, invalid SHA, missing dataset files, and existing output path.
- [ ] Implement deterministic rendering with local SHA-256 computation and fixed Actions versions matching the current frozen workflow pattern.
- [ ] Run focused tests twice and compare rendered bytes to prove determinism; run `bash -n`.
- [ ] Commit as `feat: render frozen benchmark workflows`.

### Task 3: Idempotent workflow dispatch helper

**Files:**
- Create: `scripts/dispatch-frozen-benchmark.sh`
- Test: `internal/council/runnerbootstrap/generic_dispatch_test.go`

**Interfaces:**
- Consumes: `--benchmark <id> --issue <positive-int> --attempt <positive-int> --workflow <file>`.
- Produces: exactly one `workflow_dispatch`, discovered run metadata, and marker `[<id>-fresh-dispatch-created attempt=<n>]`.

- [ ] Build fake-`gh` RED fixtures covering no-marker success, existing-marker rejection, active-run rejection, dispatch failure, ambiguous new-run discovery, and successful issue comment creation.
- [ ] Implement validation and read-before-write guards; query issue comments and workflow runs before dispatch.
- [ ] Dispatch exactly once from `main`, discover one new `workflow_dispatch` run ID, reject ambiguity, then add the immutable issue marker.
- [ ] Run focused tests with `-count=20` to catch accidental multiple dispatch calls; run `bash -n`.
- [ ] Commit as `feat: add idempotent benchmark dispatch`.

### Task 4: Verification and merge hold

**Files:**
- Modify only if tests require documentation: `docs/superpowers/specs/2026-08-28-generic-benchmark-execution-design.md`
- Modify: `docs/superpowers/plans/2026-08-28-generic-benchmark-execution.md` only for verified corrections.
- [ ] Run `gofmt -w` on new Go tests and `git diff --check`.
- [ ] Run `go test ./...`, `go vet ./...`, focused `-race` runnerbootstrap tests, and repository-pinned golangci-lint v2.12.2.
- [ ] Assert with `git diff --name-only origin/main...HEAD` that no `.github/workflows/h[1-4]-*`, `scripts/bootstrap-h[1-4]-runner.sh`, benchmark H1-H4 data, or runtime benchmark semantics changed.
- [ ] Push branch and open a PR linked to #28, but mark the PR merge-blocked by H4 attempt #2 terminal state.
- [ ] After H4 attempt #2 is terminal, rebase/update only if `main` changed outside this scope, rerun exact-head gates, require CI + CLA success, then merge with expected-head SHA.
- [ ] Close #28 only after merge and one future benchmark successfully uses the generic path or after a deterministic dry-run proves H4-equivalent generated execution semantics.