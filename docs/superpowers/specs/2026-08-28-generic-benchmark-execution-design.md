# Generic Frozen Benchmark Execution Design

**Issue:** #28
**Date:** 2026-08-28

## Context

H2-H4 proved the frozen benchmark execution pattern, but the runner bootstrap and workflow YAML are copied per version. Remote Desktop is now the execution control plane, while GitHub Actions remains the immutable freeze/audit envelope.

## Goals

- Remove copy/paste when adding H5+ execution plumbing.
- Preserve static, reviewable frozen workflow YAML per benchmark version.
- Keep subscription-only Claude/Codex execution and fail-closed credential checks.
- Make dispatch idempotent and resistant to duplicate attempts.
- Keep H1-H4 historical artifacts unchanged.

## Non-goals

- Do not modify H4 implementation, dataset, workflow, or attempt #2 behavior.
- Do not add retries, resume, provider substitution, API-key fallback, or policy overrides.
- Do not replace static frozen workflows with a moving reusable workflow.
- Do not automate successor benchmark creation after a failed run.
## Components

### 1. Generic runner bootstrap

`scripts/bootstrap-benchmark-runner.sh --benchmark <id> [--preflight-only]`

The script validates `<id>` as a safe lowercase benchmark component, derives runner label `<id>-benchmark` and runner name `<id>-<host>-<pid>`, then reuses the proven H4 subscription-auth, runner-registration, official-release digest, Linux/architecture, and ephemeral-runner checks.

Existing `bootstrap-h1-runner.sh` through `bootstrap-h4-runner.sh` remain untouched.

### 2. Frozen workflow renderer

`scripts/render-frozen-benchmark-workflow.sh --benchmark <id> --frozen-sha <40hex> --output <path>`

The renderer reads `benchmarks/<id>/{manifest.json,rubric.json,cases.json}`, computes SHA-256 locally, and emits a complete static manual-only workflow. The generated workflow embeds the exact frozen commit, hashes, runner label, benchmark command, audit paths, result filenames, and 90-day artifact retention.

The renderer refuses a dirty/missing dataset, unsafe benchmark IDs, invalid SHA, existing output unless `--check` is used, or missing benchmark CLI wiring.
### 3. Idempotent dispatch helper

`scripts/dispatch-frozen-benchmark.sh --benchmark <id> --issue <n> --attempt <n> --workflow <file>`

Before mutation it checks GitHub auth, verifies no issue marker `[<id>-fresh-dispatch-created attempt=<n>]`, and rejects any non-terminal workflow run for the same workflow. It dispatches exactly once from `main`, discovers the new workflow run, verifies the run differs from prior terminal runs, and writes the immutable issue marker with run metadata.

Runner startup stays a separate foreground operation because Desktop Commander must keep the ephemeral runner process alive. This separation avoids unreliable detached WSL processes observed during H2/H3.

## Freeze and rollout rules

- Generated workflow is committed only after the benchmark implementation/data merge SHA exists.
- The workflow itself checks out that frozen SHA, never the moving trigger SHA.
- Tooling branch may be implemented and CI-tested now but must not merge before H4 attempt #2 reaches terminal state.
- H4 attempt #2 continues using the already-merged H4 workflow/bootstrap and prepared launchers.

## Acceptance

TDD must prove safe ID validation, subscription/metered-key behavior, deterministic rendering, exact SHA/hash embedding, manual-only/no-input workflow, exactly-one benchmark command, duplicate dispatch rejection, and no modifications to H1-H4 frozen files. Full gofmt/test/vet/lint and shell syntax gates must pass before PR.