# H1 Benchmark Design

## Goal

Run the first validation benchmark for Agent Council v0 using the frozen Phase F A–F arms and Phase G evaluation semantics, measuring whether Blind Council improves decision quality without hiding distributional or tail risk.

H1 is validation-first. It does not claim Council superiority in advance and it does not tune the benchmark, rubric, evaluator, or material-worse threshold after seeing H1 outcomes.

## Frozen Benchmark Shape

H1 contains exactly 20 evidence-bounded curated decision problems:

- 10 technical/engineering decisions
- 10 product decisions

All cases are synthetic, self-contained, and newly authored for H1. They do not require model memory, web access, private customer data, or live external state. Every factual premise a candidate needs is present in the normalized problem because the frozen Phase F A–F runners receive only `NormalizedProblem`. The reference set is evaluator-side corroboration: it restates and structures the supplied evidence for Phase G judges and must not introduce a decision-critical fact that candidates were not allowed to see.

The exact case roster is:

### Technical

1. `tech-01-db-cutover` — choose a database migration cutover strategy under downtime and rollback constraints.
2. `tech-02-api-rate-limits` — choose an API rate-limiting architecture under fairness, burst, and operational constraints.
3. `tech-03-cache-stampede` — choose an incident mitigation and follow-up plan for cache stampede failures.
4. `tech-04-token-rotation` — choose an authentication-token rotation strategy with legacy-client constraints.
5. `tech-05-queue-ordering` — choose a queue design balancing ordering guarantees, throughput, and retry semantics.
6. `tech-06-backup-retention` — choose backup/retention policy under recovery objectives and cost limits.
7. `tech-07-deploy-rollback` — choose rollout and rollback policy for a high-risk service release.
8. `tech-08-observability-sampling` — choose telemetry sampling policy under cost and incident-detection constraints.
9. `tech-09-search-build-buy` — choose build versus managed search under capability, lock-in, and staffing constraints.
10. `tech-10-data-reconciliation` — choose a consistency/reconciliation strategy for conflicting replicated records.

### Product

1. `product-01-pricing-tiers` — simplify pricing tiers without violating revenue and customer-migration constraints.
2. `product-02-onboarding-friction` — reduce onboarding drop-off while preserving fraud and compliance controls.
3. `product-03-notification-launch` — choose launch scope and defaults for a new notification channel.
4. `product-04-enterprise-sso` — prioritize or defer enterprise SSO against competing roadmap work.
5. `product-05-marketplace-moderation` — choose moderation rollout policy balancing abuse risk and seller friction.
6. `product-06-feature-deprecation` — choose a deprecation/migration plan for a low-usage legacy feature.
7. `product-07-regional-expansion` — choose whether and how to enter a new region under support and compliance constraints.
8. `product-08-experiment-guardrails` — choose an experiment design when primary metrics conflict with safety guardrails.
9. `product-09-support-automation` — choose deployment scope for support automation under quality and escalation constraints.
10. `product-10-roadmap-retention` — choose between acquisition and retention investments with conflicting evidence.

No case may be added, removed, replaced, or edited after H1 begins. A modified dataset is H2 or a later benchmark, not H1.

## Dataset Layout and Integrity

The repository contains:

```text
benchmarks/h1/
  manifest.json
  rubric.json
  cases.json
  README.md
```

`cases.json` contains the 20 case envelopes. Each envelope has:

```json
{
  "id": "tech-01-db-cutover",
  "category": "technical",
  "challenger_provider": "claude",
  "problem": {},
  "problem_sha256": "...",
  "reference_set": {},
  "reference_set_sha256": "..."
}
```

Each `problem` is a normalized decision packet with the decision to make, context, hard constraints, named options where useful, and an evidence list whose entries have stable IDs. Each `reference_set` independently mirrors those evidence IDs with the verified claim and evaluator note needed to judge whether a candidate used the supplied evidence correctly. Reference sets may clarify how to interpret supplied facts but may not add hidden facts required to reach a competent decision.

Hashes are computed over the exact compact JSON bytes materialized by the H1 loader for `problem` and `reference_set`. `manifest.json` freezes the benchmark schema version, benchmark id, case count, category counts, exact ordered case IDs, SHA-256 of `rubric.json`, SHA-256 of `cases.json`, comparator, and material-worse delta.

The loader fails closed on malformed JSON, unknown fields, duplicate or unsafe IDs, wrong case count, wrong 10/10 category split, unknown category, case order mismatch, hash mismatch, unsupported comparator, non-positive material-worse threshold, or any H1 policy value that differs from the frozen constants below.

## Shared Rubric

All 20 problems use one rubric with five equally weighted dimensions:

1. `correctness_soundness` — reasoning and recommendation are internally sound and consistent with the supplied evidence and constraints.
2. `evidence_use` — material claims and trade-offs use the supplied evidence correctly and do not invent unsupported facts.
3. `risk_handling` — important failure modes, downside risk, reversibility, and mitigations are identified and handled proportionately.
4. `actionability` — the decision is concrete, prioritized, executable, and includes useful next actions or validation where needed.
5. `calibration` — uncertainty, assumptions, confidence, and conditions that would change the decision are handled appropriately.

Each dimension has weight 1. The rubric instructs evaluation judges that `overall_score` is the equal-weight arithmetic mean of the five dimension scores. H1 does not add a second scoring formula outside the frozen Phase G judge contract.

## Frozen Evaluation Policy

H1 uses the already-merged Phase G definitions:

```text
comparator = best_single
best_single = max(mean_score(A), mean_score(B))
council = mean_score(F)
council_delta = council - best_single
materially_worse = council_delta <= -10
```

`MaterialWorseDelta` is exactly `10.0` points on the 0–100 judge scale. It is frozen before any H1 baseline or evaluation call and must not be changed based on H1 results.

The same fixed fresh Claude + Codex evaluation judge pair scores every A–F arm for a problem, using Phase G identity masking, hash checks, citation verification, strict JSON validation, independent workspaces, and no judge reconciliation.

## Frozen Phase F Execution Policy

H1 does not modify any A–F arm implementation.

For both E and F, `ChallengePolicy` is frozen to:

```go
protocol.ChallengePolicy{
    AllowAbbreviated:        false,
    HighConfidenceThreshold: 1.0,
}
```

The threshold is inert because abbreviated challenge is disabled. This guarantees the full Phase F path for every H1 E/F execution.

The manifest order is globally indexed from 1 through 20: technical cases occupy indices 1–10 in the order listed above, and product cases occupy indices 11–20 in the order listed above. Odd global indices use Claude as challenger and even global indices use Codex. The same challenger provider is used for E and F of the same problem. This produces exactly 10 Claude-challenger and 10 Codex-challenger problems overall, with 5/5 challenger balance inside each category, without changing any arm semantics.

## H1 Orchestration

Add a small `internal/council/benchmark` package. It owns only H1 dataset validation, batch orchestration, and H1 artifact persistence. It composes existing components rather than duplicating them:

1. load and validate the frozen H1 dataset before any runtime call;
2. create one immutable H1 run root;
3. freeze exact H1 manifest, rubric, compact problem bytes, and compact reference-set bytes into that run root;
4. for each problem in manifest order, call `baseline.Runner.RunAll` with the frozen challenger policy/provider;
5. persist raw A–F `baseline.ArmResult` values immutably before evaluation;
6. call `evalharness.Harness.EvaluateProblem` with the shared rubric, per-case reference set, exact hashes, and `RiskPolicy{ComparatorBestSingle, 10.0}`;
7. after all 20 problems succeed, call `evalharness.SummarizeBatch` and `evalharness.WriteEvaluation` once for the complete batch;
8. write a final immutable H1 result manifest that records the benchmark input hashes and final batch-summary hash.

A runtime/evaluation failure stops the batch. Completed immutable raw artifacts remain for audit, but there is no automatic resume and no final H1 batch summary. A rerun uses a new run ID. This avoids complex resume semantics in v0 while preserving evidence about spent calls and partial progress.

## H1 Artifact Layout

```text
<runs-root>/<run-id>/
  h1-run.json
  inputs/
    benchmark-manifest.json
    rubric.json
    cases/<problem-id>/problem.json
    cases/<problem-id>/reference-set.json
  baseline/
    <problem-id>/arm-A.json
    <problem-id>/arm-B.json
    <problem-id>/arm-C.json
    <problem-id>/arm-D.json
    <problem-id>/arm-E.json
    <problem-id>/arm-F.json
  eval/
    eval-policy.json
    batch-summary.json
    problems/<problem-id>/...
    provenance.jsonl
  h1-result.json
```

All H1-owned writes are exclusive create-only writes. Existing paths are never overwritten. Paths must remain within the run root after symlink resolution. Raw baseline artifacts are not placed into evaluation judge workspaces; Phase G continues to materialize only its masked candidate envelope and allowed evaluation inputs.

## CLI

Extend the existing command surface with:

```text
agentd council benchmark h1 [flags]
```

Flags:

- `--dataset` defaults to `benchmarks/h1`
- `--runs-dir` defaults through existing v0 config to `.council/runs`
- `--config` optionally supplies the existing subscription-only v0 config
- `--temp-root` defaults to `os.TempDir()`
- `--claude-bin` defaults to `claude`
- `--codex-bin` defaults to `codex`

The command validates existing billing policy and the entire H1 dataset before constructing the first baseline call. It prints JSON with `run_id`, `run_dir`, and final `batch_summary` on success. It exits non-zero on any failure.

No flag may override H1 case count, category split, rubric, comparator, material-worse delta, challenge mode, challenger schedule, judge roster, or A–F definitions.

## Failure Handling

Fail closed before any model call on dataset-policy/hash/schema failure, unsafe paths or IDs, unsupported billing mode, missing runtime, or run-root creation failure.

During execution fail closed on any existing Phase F runtime/isolation/malformed-output failure, Phase G hash/judge/citation failure, immutable artifact collision, containment failure, or context cancellation.

There is no provider substitution, metered fallback, best-effort scoring, missing-arm coercion, automatic retry, benchmark mutation, or threshold tuning.

## Testing Strategy

Implementation follows TDD.

Required coverage:

- H1 loader accepts exactly the committed 20-case dataset and rejects mutated hashes, duplicate IDs, wrong order, wrong category split, wrong policy, and malformed envelopes before runtime calls.
- Every candidate-visible decision-critical fact is present in `problem`; tests ensure the committed reference sets use only evidence IDs declared by the corresponding problem.
- Challenger schedule is exactly 10 Claude / 10 Codex and stable by global manifest index, with 5/5 balance in each category.
- Batch orchestration calls A–F once per case through the existing runner and passes the exact same case bytes/hashes to Phase G.
- `AllowAbbreviated` is false for every E/F problem.
- Raw A–F outputs and frozen inputs are written immutably with containment checks.
- A failure stops later problems and never writes a final H1 summary.
- Successful completion writes exactly 20 `ProblemResult` values, a Phase G batch summary, and a final H1 result manifest.
- CLI rejects attempts to change frozen H1 policy and preserves subscription-only configuration.
- Existing repository gate remains gofmt, `go test ./...`, `go vet ./...`, golangci-lint, and CLA on the exact PR head.

## Acceptance Criteria

H1 is ready to run only when the exact 20-case curated dataset, one shared five-dimension equal-weight rubric, `MaterialWorseDelta=10`, full challenge policy, balanced frozen challenger schedule, dataset hashes, batch runner, immutable H1 artifacts, and CLI are merged on `main` with exact-head CI and CLA green.

The first real H1 model call must occur only after that merge. Any change to benchmark contents or scoring policy after a real H1 call begins creates a new benchmark version rather than silently redefining H1.
