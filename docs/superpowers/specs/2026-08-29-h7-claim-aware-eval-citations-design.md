# H7 Claim-Aware Evaluator Citation Identity Design

**Date:** 2026-08-29
**Issue:** #42
**Status:** Approved by standing automation-first authorization

## Problem
H6 real run `33236596983` failed on problem 1 (`tech-01-db-cutover`), arm F, eval judge-1 with terminal `malformed_output`: `duplicate citation check "problem:constraints[1]"`.

The H6 evidence shows the masked candidate legitimately contains two citation occurrences with the same source location but different claims:
- `{artifact_id:"problem", locator:"constraints[1]", claim:"No committed write may be silently lost, the constraint dual-write most directly endangers."}`
- `{artifact_id:"problem", locator:"constraints[1]", claim:"No committed write may be silently lost."}`

Protocol candidate normalization deduplicates citations by `(artifact_id, locator, claim)`. H6 evaluator identity uses only `(artifact_id, locator)`, so two valid candidate occurrences collapse to one validator key. The H6 prompt says to verify every candidate citation and does not define a unique-source projection. The judge returned both occurrences and the validator rejected the second.

This is a deterministic evaluator wire-contract mismatch, not an adapter availability, quota, or model-quality failure. H6 is frozen and must not be mutated or rerun.

## Decision
H7 makes evaluator citation identity equal to the candidate citation occurrence identity:

```json
{"artifact_id":"problem","locator":"constraints[1]","claim":"No committed write may be silently lost."}
```

Both `citation_checks[].reference` and `relied_on_citations[]` use this closed typed object. All three fields must copy exactly from one entry in `candidate.citations`.

## Compatibility Boundary
- H1-H6 frozen commits, datasets, workflows, evidence, and runtime semantics remain unchanged.
- H6 keeps `CitationContractStructuredV1` and `SchemaProfileH6` with two-field `{artifact_id, locator}` evaluator references.
- H7 adds `CitationContractStructuredV2` and `SchemaProfileH7` with three-field `{artifact_id, locator, claim}` references.
- H7 does not deduplicate candidate citations by source location and does not discard or merge claims.
- Unknown tuples, whitespace-mutated fields, duplicate identical full tuples, and unverified relied-on tuples remain terminal `malformed_output`.
- No output repair, fuzzy matching, claim normalization, provider override, metered fallback, retry, or H6 redispatch is allowed.

## Validation Semantics
The H7 validator builds the candidate set from the full comparable tuple `(artifact_id, locator, claim)`. A citation check is valid only if the exact full tuple exists in the masked candidate. A relied-on tuple must also exist in the candidate and have a matching `verified` citation check for the same full tuple.

Two references sharing `artifact_id` and `locator` are distinct when `claim` differs. Two identical full tuples are duplicates and are rejected.

## Existing Result Model Serialization
The existing `JudgeArtifact` stores citation references as strings. H7 preserves full tuple identity across that compatibility boundary by serializing each already-validated citation tuple as compact canonical JSON using the fixed struct field order:

```text
{"artifact_id":"problem","locator":"constraints[1]","claim":"No committed write may be silently lost."}
```

This is deterministic, lossless serialization after validation. It is not repair or normalization of model output. H6 continues serializing its validated V1 keys as `artifact_id:locator`; no H6 bytes or behavior change.

## Structured Output
Add `SchemaProfileH7`. It is byte-identical to H6 for every role/phase except eval-judge. H7 eval-judge uses a closed citation-reference object with required string fields `artifact_id`, `locator`, and `claim` for both citation checks and relied-on citations. Caller schema injection remains forbidden.

## H7 Benchmark Identity
Create `benchmarks/h7` as a new version boundary. Preserve H6 problem payloads, reference sets, rubric semantics, case order, comparator, `MaterialWorseDelta`, logical-slot topology, availability-only failover classes, same-adapter max-attempts=1, Antigravity adapter, and final `human-chatgpt-session` broker. `adapter-policy.json` is byte-identical to H6.

Only H7 identity/schema markers and hashes required by the new evaluator contract change. No provider-policy override or metered API fallback is introduced.

## Runtime and Operations
H7 runner selects `CitationContractStructuredV2` and `SchemaProfileH7`; all adapter/failover behavior remains H6-equivalent. Implementation/data merge first and yields `H7_FROZEN_SHA`. A separate ops PR generates and pins `h7-frozen-execution.yml` to that exact SHA and committed input hashes.

Exactly one real H7 dispatch is permitted after the ops PR merges. Partial evidence is always uploaded. Any non-availability failure is terminal and must not trigger an automatic second attempt.

## Success Criteria
- A regression fixture matching the H6 arm-F duplicate-source/different-claim citations fails under H6 V1 and passes under H7 V2.
- Both claim-distinct citation checks survive H7 canonicalization without collision.
- H7 rejects an unknown claim attached to an otherwise valid artifact/locator.
- H7 rejects duplicate identical full tuples in checks and relied-on citations.
- H6 schema/contract behavior remains byte- and test-stable.
- H7 non-eval structured-output schemas are byte-identical to H6.
- H7 dataset semantic-equivalence and adapter-policy byte-equality gates pass.
- `go test ./...`, `go vet ./...`, targeted/full race, `git diff --check`, and golangci-lint v2.12.2 pass before exact-head CI/CLA.
- Real H7 execution either completes with consistent final artifacts or yields a new initiating failure with no automatic attempt #2.