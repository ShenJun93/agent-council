# Phase H Technical Value-Validation Pilot Design

## Status

Approved design for GitHub issue #56, revised after source-artifact audit.

The original 20-case replay is not executable without regenerating candidates. The terminal H8 artifact contains complete A-F artifacts for exactly the ten technical cases and only A-E for `product-01-pricing-tiers`; earlier terminal runs stopped earlier and do not provide a complete product source set.

Phase H therefore uses every complete H8 case and makes a narrower technical-domain claim rather than mixing fresh candidate generation with frozen outputs.

## Goal

Evaluate the ten complete frozen H8 technical cases using already-produced A-F candidates and the H8/V3 evaluator semantics validated by H9, then produce one immutable PASS, FAIL, or INCONCLUSIVE result.

Phase H does not regenerate candidates, rerun H1-H9, or claim product-decision generality.

## Frozen Source Provenance

- workflow run: `33349114073`
- frozen H8 SHA: `8d13f0a82758f5ea6286409d55123b97929dbce4`
- internal run: `h8-20260831T015809Z-e8ab51986046`
- artifact ID: `9745340503`
- artifact digest: `sha256:c37cbab9cb4d1a8c5b4f6cc2535268c681456d2b6380587fb178396ae9dedfd6`

## Frozen Pilot Shape

Exactly these ten cases, in original H1/H8 order:

1. `tech-01-db-cutover`
2. `tech-02-api-rate-limits`
3. `tech-03-cache-stampede`
4. `tech-04-token-rotation`
5. `tech-05-queue-ordering`
6. `tech-06-backup-retention`
7. `tech-07-deploy-rollback`
8. `tech-08-observability-sampling`
9. `tech-09-search-build-buy`
10. `tech-10-data-reconciliation`

Each case has six frozen arms A-F and two independent evaluator slots: exactly `10 × 6 × 2 = 120` evaluator invocations.

No product case is included because no complete frozen product A-F source artifact exists. No candidate arm may be regenerated, skipped, substituted, or repaired.

## Frozen Evaluation Semantics

Phase H reuses H8/V3 citation identity `(artifact_id, locator, claim)`, allows only `verified|unverified`, and forbids unverified evidence from supporting scores. Relied-on unverified citations are terminal malformed output.

The five equal-weight rubric dimensions remain `correctness_soundness`, `evidence_use`, `risk_handling`, `actionability`, and `calibration`; overall score remains their arithmetic mean.

## Comparator and Outcome

Per case:

```text
best_single = max(mean_score(A), mean_score(B))
council = mean_score(F)
council_delta = council - best_single
materially_worse = council_delta <= -10.0
```

`MaterialWorseDelta` is exactly `10.0` and not configurable.

Phase H reports mean delta, win/tie/loss counts, materially-worse count/rate/IDs, per-case deltas, existing arm distributions, and mean/max judge disagreement using `JudgeSpread`.

A complete run is PASS only when `overall_mean_council_delta > 0` and `materially_worse_count == 0`. A complete run is FAIL when mean delta is non-positive or any materially-worse case exists. Any incomplete/invalid execution is INCONCLUSIVE and writes no PASS/FAIL result.

A PASS demonstrates measurable value only for this frozen technical pilot. Product value and broader statistical generality remain untested. The separate concrete-pain gate for API/BYOK remains mandatory.

## Human-Broker Policy

All 120 evaluator invocations use the H9 current ChatGPT-web human broker: `require_current_session=true`, `require_fresh_session=false`; accepted responses report `current_session=true`, `fresh_session=false`.

Complete-run adapter evidence must show successful invocations 120, human-broker invocations 120, adapter/provider diversity 1/1, availability failures 0, and failovers 0. There is no automatic retry, Claude/Codex evaluator substitution, or metered API fallback.

## Dataset and Orchestration

`benchmarks/phase-h/` contains `README.md`, `manifest.json`, `rubric.json`, `adapter-policy.json`, and `cases/<id>/{problem.json,reference-set.json,arm-A.json,...,arm-F.json}`. All bytes derive from the H8 artifact and are hash-frozen in the manifest.

A separate `PhaseHReplayRunner` validates the dataset before calls, creates an immutable run root, freezes inputs, evaluates ten cases in order through H8/V3, persists accepted evidence before continuing, fails closed on first invalid invocation, and only after all 120 calls computes summaries and writes `phase-h-result.json`.

`council.phase-h-result.v0` records benchmark/mode/outcome, case count 10, invocation counts 120, diversity/failover evidence, mean delta, win/tie/loss, materially-worse fields, judge disagreement, batch/adapter hashes, and H8 provenance.

## Governed Workflow

`.github/workflows/phase-h-frozen-execution.yml` is `workflow_dispatch` only, bound to an exact frozen implementation SHA, issue #56 authorization marker, and `current_session_attestation=true`. It verifies dataset/provenance hashes, rejects metered credentials, runs tests before calls, enforces one-shot execution, and uploads audit evidence on every terminal path.

## Failure Handling and Tests

Fail closed on provenance/hash/schema/topology/policy/session/auth/path/authorization mismatch and on broker failure, malformed evaluator output, invalid citation identity, relied-on unverified evidence, duplicate/missing response, persistence failure, or cancellation. No retry/failover/substitution/regeneration/threshold tuning is allowed.

TDD must cover exact 10-case/6-arm topology, mutation rejection, 120-call adapter evidence, fail-at-N behavior, value diagnostics, zero-delta FAIL, materially-worse FAIL, positive/no-tail PASS, and unchanged H1-H9 behavior.

## Acceptance Criteria

Phase H is ready for one governed real run only after issue #56 is current source of truth, the H8-derived dataset is committed and hash-verified, implementation/workflow tests and exact-head review are green, the merged SHA is frozen on #56, and one explicit real-run marker is posted. The single dispatched run terminates as PASS, FAIL, or INCONCLUSIVE without post-hoc mutation.
