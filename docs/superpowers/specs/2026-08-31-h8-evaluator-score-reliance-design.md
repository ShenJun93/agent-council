# H8 Evaluator Score-Reliance Contract Design

**Date:** 2026-08-31  
**Issue:** #45  
**Status:** Approved by standing automation-first authorization

## Initiating Failure

H7 real execution run `33301107633` validated the claim-aware `(artifact_id, locator, claim)` citation identity fix, then failed later on problem 3 `tech-03-cache-stampede`, arm C, eval judge-1.

The frozen chain correctly failed over once from Claude quota exhaustion to Codex. Codex returned a syntactically valid evaluator object. For one candidate citation it judged the numeric observation supported but the causal diagnosis only partially supported, emitted `status = "partially_verified"`, and also copied that citation into `relied_on_citations`. The H7 validator correctly rejected the object because score-relied citations must be verified.

This is a new generation-contract failure, not the H6/H7 citation-identity regression.

## Root Cause

The fail-closed validator matches the frozen Phase G/H7 design and must not be weakened. H7's model-facing output contract leaves the relation between verification and score reliance under-specified:

- `relied_on_citations` is not explicitly defined as evidence the **judge itself uses to support its numeric score**;
- the required invariant `relied_on => verified` is enforced only after generation;
- `citation_checks[].status` is an unconstrained string, allowing plausible but non-contract vocabulary such as `partially_verified`;
- verification and reliance are represented in separate arrays, creating a cross-array consistency requirement that native structured output does not express.

The result is a judge response that is semantically understandable but contract-contradictory.

## Decision

H8 introduces a new evaluator wire contract only for H8. H1-H7 remain frozen.

Each citation check carries both verification and score-reliance state in one object:

```json
{
  "reference": {
    "artifact_id": "problem",
    "locator": "evidence.e1",
    "claim": "exact candidate claim"
  },
  "status": "verified",
  "relied_on": true,
  "note": "directly supported by visible evidence"
}
```

H8 removes model-authored `relied_on_citations` from the wire format. The existing internal `JudgeArtifact.ReliedOnCitations` remains unchanged and is derived deterministically after validation from checks where `relied_on == true`.

## Verification Vocabulary

H8 freezes `citation_checks[].status` to exactly:

- `verified`: the full cited claim matches the visible artifact;
- `unverified`: the full cited claim is not directly established by the visible artifact, including partially supported, inferred, over-broad, contradicted, or unsupported claims.

There is no `partially_verified` wire state. Partial support may be described in `note`, strengths, or weaknesses, but the full claim is `unverified` for score-support purposes.

## Score-Reliance Invariant

`relied_on` means: **the judge uses this exact citation occurrence as evidentiary support for its numeric score or dimension scores**.

Rules:

1. `relied_on == true` is permitted only when `status == "verified"`.
2. `status == "unverified"` requires `relied_on == false`.
3. Unverified citations may still be discussed as candidate weaknesses or critical errors, but they cannot support the score.
4. Every reference remains the exact full candidate tuple `(artifact_id, locator, claim)`; no repair, fuzzy match, claim normalization, or source-only deduplication.
5. Duplicate identical full tuples remain malformed.

The validator remains fail-closed as defense in depth. Contradictory output such as `status:"unverified", relied_on:true` is terminal `malformed_output`; H8 never repairs it.

## Prompt Contract

The H8 judge prompt must state the verification vocabulary and score-reliance invariant literally, including that `relied_on` refers to the judge's own scoring reliance, not the candidate's use of the citation.

It must explicitly instruct that a partially supported claim is `unverified`, must have `relied_on:false`, and may lower the evidence-use/correctness score rather than causing the judge to pretend the citation is verified.

## Structured Output

Add `SchemaProfileH8` for eval-judge only. Non-eval schemas are byte-identical to H7.

The H8 citation-check schema is closed and requires:

- `reference`: H7 three-field citation occurrence key;
- `status`: enum `verified|unverified`;
- `relied_on`: boolean;
- `note`: string.

The H8 eval-judge wire schema omits `relied_on_citations`. Existing result serialization is populated deterministically from validated `relied_on:true` checks, preserving the current internal result model and audit artifacts.

## Compatibility Boundary

- Never mutate or rerun frozen H1-H7.
- Preserve H7 claim-aware citation identity and canonical tuple serialization.
- Preserve H7 problem payloads, references, rubric semantics, case order, comparator, `MaterialWorseDelta`, logical-slot topology, adapter chains, availability-only failover classes, same-adapter `max_attempts=1`, no-metered-key policy, and final human broker.
- H8 receives its own dataset/version identity and structured-output profile.
- No provider override, automatic output repair, silent status coercion, same-adapter retry, H7 attempt #3, or H6 redispatch.

## Data Flow

1. Candidate normalization produces the existing claim-aware citation tuples.
2. H8 renders the same isolated judge workspace plus the explicit H8 prompt.
3. Native structured output constrains status vocabulary and the co-located `relied_on` boolean.
4. H8 decoder validates exact tuple membership, duplicate checks, allowed status, and `relied_on => verified`.
5. Decoder canonicalizes checks into the existing `protocol.CitationCheck` representation and derives `JudgeArtifact.ReliedOnCitations` from verified relied-on checks.
6. Existing scoring, aggregation, storage, metrics, and provenance code remains unchanged.

## TDD and Regression Coverage

Before production changes, add a failing H8 regression test that models the H7 problem-3 contradiction and asserts the H8 contract shape/semantics.

Required tests:

- H7 remains unchanged and rejects a non-verified tuple present in `relied_on_citations`.
- H8 schema exposes only `verified|unverified`, requires `relied_on`, and omits model-authored `relied_on_citations`.
- H8 accepts `verified + relied_on:true` and derives the exact canonical tuple into internal `ReliedOnCitations`.
- H8 accepts `unverified + relied_on:false` and does not derive score reliance.
- H8 rejects `unverified + relied_on:true` without repair.
- H8 prompt explicitly defines judge score reliance and partial-support behavior.
- H8 still preserves two citations sharing artifact+locator but differing in claim as separate occurrences.
- H1-H7 schema/profile behavior remains byte/test stable.

## Freeze and Execution

Implementation/data must pass gofmt, focused tests, `go test ./...`, `go vet ./...`, targeted/full race, `git diff --check`, golangci-lint v2.12.2, exact-head CI and CLA before merge/freeze.

After the implementation/data PR is merged, record `H8_FROZEN_SHA` and committed input hashes. A separate ops PR must pin a new H8 frozen-execution workflow to that exact SHA. Real execution is guarded and exactly once per approved fresh attempt; terminal evidence is always uploaded.