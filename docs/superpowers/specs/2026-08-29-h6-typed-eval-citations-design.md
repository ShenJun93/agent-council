# H6 Typed Evaluator Citations Design

**Date:** 2026-08-29
**Issue:** #38
**Status:** Approved by standing automation-first authorization

## Problem
H5 real run `33194301989` failed after all six arms of problem 1 were persisted. The evaluator returned a semantically correct citation string `problem constraints[0]`, while the candidate represented the same citation as `{artifact_id:"problem", locator:"constraints[0]"}` and the validator required exact canonical string `problem:constraints[0]`.

The evaluator prompt only declared `relied_on_citations` as `string[]`; it never specified the validator's delimiter. This is a deterministic wire-contract mismatch, not an adapter availability, quota, or model-quality failure.

## Decision
H6 replaces free-form evaluator citation references with typed citation keys on the H6 path only:

```json
{"artifact_id":"problem","locator":"constraints[0]"}
```

Both `citation_checks[].reference` and `relied_on_citations[]` use the same typed key. Validation compares typed keys directly against the candidate citation key set.
## Compatibility Boundary
- H1-H5 frozen commits, datasets, workflows, scripts, and evidence remain unchanged.
- Latest-main legacy evaluator behavior remains the zero-value/default path.
- H6 explicitly selects `CitationContractStructuredV1` and structured-output schema profile `h6`.
- H6 may convert validated typed keys to canonical `artifact_id:locator` strings only when populating the existing result model; this is deterministic serialization, not model-output repair.
- Malformed typed objects, unknown candidate citations, unverified relied-on citations, duplicates, and extra schema fields remain terminal `malformed_output` failures.

## Structured Output
`structuredoutput.Wrap` keeps the H4/H5 schema set. Add a versioned wrapper/profile for H6. All non-evaluator role/phase schemas are byte-equivalent to H5. Only eval-judge changes:
- `citation_checks[].reference`: closed object with required `artifact_id` and `locator` strings.
- `relied_on_citations[]`: array of the same closed citation-key object.

No CLI flag may override this schema profile. Invocation evidence continues to record the actual schema SHA-256.

## Evaluator Prompt and Validation
The H6 prompt states that citation reference fields must copy `{artifact_id, locator}` exactly from the masked candidate's `citations` array. The judge may inspect problem/rubric/reference-set to verify claims, but `relied_on_citations` identifies candidate citations, not arbitrary visible-artifact locators.

The validator builds a set of typed candidate keys, validates every relied-on key is present, and requires a matching `citation_checks` entry with status `verified`.
## H6 Benchmark Identity
H6 preserves H5 problem/reference payloads, rubric semantics, case order, comparator, material-worse threshold, logical-slot topology, availability-only failover classes, same-adapter max-attempts=1, Antigravity adapter, and final `human-chatgpt-session` broker.

Create `benchmarks/h6` with H6 manifest/cases/rubric identity markers. `adapter-policy.json` is byte-identical to H5 unless loader mechanics require a version marker; no slot/adapter ordering changes are allowed. Manifest hashes all committed inputs.

## Runtime and Operations
H6 runner uses the existing adapter pools and human broker. Quota/auth/adapter-unavailable may fail over; malformed output, typed-citation violations, semantic/isolation/process/billing failures remain terminal.

Use generic remote-first execution tooling. Implementation/data merges first and yields `H6_FROZEN_SHA`; a separate ops PR generates/pins `h6-frozen-execution.yml` to that exact SHA. Preflight only needs an approved chain to be serviceable; it must not require Claude specifically.

Exactly one real H6 workflow dispatch is allowed after the ops PR merges. Partial evidence is always uploaded. No automatic second attempt.

## Success Criteria
- Real H5 failure fixture passes under H6 typed contract without string normalization.
- Legacy evaluator string-contract tests remain unchanged and pass.
- H6 schema digest differs only for eval-judge relative to H5.
- H6 dataset/policy semantic-equivalence gates pass.
- Full tests, vet, race, diff-check, and golangci-lint v2.12.2 pass locally and in exact-head CI/CLA.
- Real H6 run succeeds with consistent `h6-result.json` and `eval/batch-summary.json`, or terminal evidence identifies a different initiating failure without redispatch.
