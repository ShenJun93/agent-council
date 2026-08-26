# Phase G Evaluation Harness Design

## Goal

Build the v0 evaluation harness that scores frozen baseline arms A–F for each normalized benchmark problem using the same two fixed fresh judges per problem, then reports both central tendency and distributional/tail risk before H1.

## Scope

Phase G evaluates already-produced Phase F arm results. It does not change protocol execution, Visibility Firewall behavior, runtime billing/auth policy, artifact persistence, or the A–F arm definitions.

The harness supports one normalized problem, one frozen rubric and reference set, six candidate arm results A–F, two fixed judge providers for that problem, independent fresh judge invocations, candidate identity masking, citation verification, deterministic aggregation, mean/variance/tail-risk metrics, and an explicit pre-frozen material-worse threshold.

Out of scope: automatic rubric/reference generation, model rotation, third-party judge providers, persistent databases, dashboards, generic statistics frameworks, or H1 dataset authoring.

## Inputs and Frozen Configuration

Each problem supplies normalized problem bytes, rubric bytes, reference-set bytes, and SHA-256 hashes for the rubric and reference set. The harness verifies those hashes before any judge invocation and fails closed on mismatch.

Evaluation policy is explicit:

```go
type RiskPolicy struct {
    Comparator         Comparator
    MaterialWorseDelta float64
}

const ComparatorBestSingle Comparator = "best_single"
```

For v0, `best_single` is the only supported comparator. It means the stronger of arms A and B for that problem. `MaterialWorseDelta` must be positive and supplied before the benchmark batch begins; the harness never infers or tunes it from results.

## Judge Independence and Identity Masking

For each problem, the same two judge runtimes score every arm: Judge 1 is Claude and Judge 2 is Codex. They do not alternate or rotate between arms.

Every arm/judge evaluation is a fresh isolated invocation with a unique workspace outside the full run root. Each judge receives the normalized problem, frozen rubric, frozen reference set, one masked candidate result, and only the evidence artifacts needed to validate that candidate. It does not receive the arm label, candidate provider provenance, the other judge verdict, scores for other arms, aggregate benchmark results, or hidden artifacts.

Masking is enforced through materialized evaluation context, not prompt instruction alone.

## Candidate Normalization

A–D produce `baseline.AnswerArtifact`; E/F produce `protocol.Result`. The evaluation layer deterministically converts both into one masked candidate envelope without provider metadata. The envelope carries decision/action, reasons, assumptions, risks, evidence, minority report, unresolved items, and next validations where available. The arm label remains only in harness bookkeeping and is never rendered into the judge workspace.

## Judge Output

Each judge returns strict JSON with an overall score normalized to 0–100, rubric-dimension scores, citation checks, critical errors, strengths, weaknesses, and confidence. Every rubric dimension required by the frozen rubric must be present. Missing dimensions, malformed JSON, out-of-range values, or materially relied-on citations that cannot be verified fail closed rather than being silently coerced.

The judge prompt explicitly requires citation verification before relying on a claim, using only artifacts visible in the judge workspace.

## Per-Arm Aggregation

Each arm receives two independent judge scores. The arm mean is the arithmetic mean of the two overall scores. The absolute difference between the two judge scores is retained as evaluator spread. Judges never reconcile with each other.

## Per-Problem Comparison

Definitions are frozen as:

```text
best_single = max(mean_score(A), mean_score(B))
council = mean_score(F)
council_delta = council - best_single
materially_worse = council_delta <= -MaterialWorseDelta
```

This intentionally compares Blind Council against the stronger single-agent baseline. Arm E remains a diagnostic full-information comparator and is not the tail-risk baseline.

## Batch Metrics

For every arm across N problems, report sample count, mean, population variance, min, max, median, p10, p90, and mean judge spread. Percentiles use a deterministic nearest-rank rule documented in code.

For Blind Council versus best single additionally report mean delta, delta variance, minimum delta, p10 delta, materially-worse count, materially-worse rate, and materially-worse problem IDs for audit.

Empty batches fail closed. Metrics are not silently omitted merely because N is small.

## Artifact Outputs

Phase G writes immutable structured artifacts under an evaluation root, for example:

```text
eval/
  problems/<problem-id>/
    arm-A.json
    arm-B.json
    arm-C.json
    arm-D.json
    arm-E.json
    arm-F.json
    problem-summary.json
  batch-summary.json
  eval-policy.json
```

Score provenance records judge slot/provider, input hashes, timestamps, and output hashes separately from candidate content shown to judges. Existing artifact-store invariants apply: immutable writes, hashes, containment checks, lineage, and fail-closed overwrite/path validation.

## Failure Handling

Fail closed on rubric/reference hash mismatch, invalid IDs, missing or duplicate arms, runtime/auth/billing/isolation failure, malformed judge JSON, missing rubric dimensions, scores outside 0–100, unsupported comparator, non-positive material-worse threshold, or workspace/path isolation failure. No judge provider substitution is permitted.

## Testing Strategy

Implementation follows TDD. Required coverage includes candidate normalization, exact fixed judge routing for all A–F arms, 12 fresh judge workdirs per problem, identity masking, judge independence, pre-runtime hash validation, malformed/out-of-range output rejection, citation-verification requirements, arithmetic aggregation, best-single and material-worse boundary behavior, deterministic mean/variance/median/p10/p90, materially-worse count/rate/IDs, invalid risk policy and empty-batch failure, and immutable/contained evaluation artifact writes.

Repository verification remains gofmt, `go test ./...`, `go vet ./...`, golangci-lint, and CLA on the exact merge head.

## Acceptance Criteria

Phase G is complete when all six Phase F arms can be scored by the same fixed Claude+Codex judge pair per problem; identity and independence constraints are enforced by materialized context; rubric/reference hashes are verified before scoring; per-arm/per-problem results are reproducible from frozen inputs; batch reports expose mean, variance, evaluator disagreement and tail risk; Blind Council tail failures versus best single are auditable; `MaterialWorseDelta` is a frozen explicit input; exact-head CI and CLA are green; and no H1 benchmark run begins before these metric definitions are merged and frozen.
