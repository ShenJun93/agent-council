# H3 Citation Authority Design

**Date:** 2026-08-27
**Status:** approved for implementation under the project's standing automation-first approval
**Tracks:** #21

## Context

H2 real run `33059381978` completed all six A-F baseline arms for the first problem, then failed before evaluation judges because arm C cited private artifact `draft`. H2 preserved enough evidence to make the root cause deterministic: self-review final sees `problem` and `draft`, its prompt permits citations to any visible artifact, while the evaluator intentionally accepts candidate citations only to `problem` or evaluator-only `reference-set`.

H2 remains frozen and inconclusive. H3 is a new benchmark version; H1/H2 frozen execution commits are never modified or rerun.

## Goals

- Separate what a model may **see for reasoning** from what its final answer may **cite as evidence**.
- Preserve self-review's access to its own draft without allowing the private draft to become evaluator evidence.
- Enforce final citation authority in code before arm persistence/evaluation.
- Keep final self-review output self-contained and avoid explicit draft/review-process references that can leak arm identity.
- Preserve every H2 transport, isolation, persistence, auth, arm, challenge, evaluation, and risk-policy invariant not explicitly changed here.
## Version boundary

H3 introduces benchmark ID `h3` and H3 dataset/run/result schema markers. `benchmarks/h3` must be semantically equivalent to H2 for rubric, problem, reference-set, case order, category split, challenger schedule, comparator, and `MaterialWorseDelta`; only H3 identity/schema fields and consequent top-level hashes change.

H1/H2 commands on later `main` must retain their previous citation behavior. H3 alone enables the stricter authority policy.

## Baseline citation authority

Add a version-selectable `baseline.CitationAuthority` on `baseline.Runner`:

- legacy/default: final citations may reference any artifact visible in that baseline invocation; this preserves H1/H2 behavior.
- H3 strict: final answer citations may reference `problem` only.

`invokeAnswer` receives separate `visibleIDs` and `citableIDs`. Visibility materialization uses only `visibleIDs`. Prompt rendering prints the visible artifacts and an explicit final citation allowlist. Parsed output is validated against `citableIDs`; any disallowed artifact ID or empty locator returns `FailureMalformedOutput` before the arm result can be persisted.

For H3 self-review final specifically: visible IDs are `problem,draft`; citable IDs are `problem`. The prompt also requires a self-contained final answer that does not mention the draft, review process, arm, or provider. We do not use keyword-based runtime rejection for prose because domain text can legitimately contain words such as “draft”; only citation authority is hard-enforced structurally.
## Evaluation and evidence

Eval-harness visibility stays unchanged: judges receive normalized problem, frozen rubric, frozen reference set, and one masked candidate. Self-review drafts are never added to judge visibility because that would give C/D asymmetric extra evidence and reveal process topology.

H3 reuses H2 strict raw/fenced JSON decoding, raw invocation logging, symlink-safe create-only storage, and incremental arm persistence. If a final self-review response cites `draft`, its raw invocation evidence remains persisted while that arm fails before baseline artifact persistence. No resume is allowed; any later execution uses a fresh H3 run ID.

## H3 orchestration

Add H3 dataset loader/constants, H3 run/result manifests, sequential H3 A-F runner, and `agentd council benchmark h3`. The H3 CLI exposes the same operational flags as H2 and no policy overrides. H3 runtime construction wraps Claude/Codex with invocation logging and sets strict citation authority on the baseline runner. Risk/challenge/eval policies remain equal to H2.

After implementation/data merge, freeze the exact implementation SHA. A separate manual-only H3 workflow must hard-pin that SHA, require a subscription-authenticated ephemeral Linux runner labeled `h3-benchmark`, run repository tests, execute H3 exactly once, and always upload full/partial evidence. No retry, resume, provider substitution, or metered fallback.

## Acceptance tests

1. Legacy self-review still sees and may cite `draft`, proving H1/H2 compatibility.
2. H3 self-review prompt contains the draft content but explicitly lists only `problem` as citable and requires self-contained final wording.
3. H3 self-review output citing `draft` fails as malformed after the model invocation; raw invocation evidence remains available when wrapped.
4. H3 self-review output citing `problem` succeeds.
5. H3 dataset is semantically equal to H2 except identity/schema markers and top-level hashes.
6. Full gofmt/test/vet/lint and exact-head CI/CLA pass before merge.
7. No real H3 model call occurs before implementation/data/workflow are merged and frozen.