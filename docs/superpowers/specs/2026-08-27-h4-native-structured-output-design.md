# H4 Native Structured Output Design

**Date:** 2026-08-27
**Status:** approved under the project's standing automation-first approval
**Tracks:** #25

## Context

H3 real run `33067558993` failed on problem 1, arm E, rebuttal after Codex exited 0 with syntactically invalid JSON: the `reasons` array was missing its closing `]`. The strict decoder correctly rejected the response; `parallel2` then cancelled the still-running Claude peer. H3 is frozen and inconclusive and must not be rerun.

Both subscription CLIs already expose provider-native structured output: Claude Code supports `--json-schema <schema>` and Codex supports `--output-schema <FILE>`. H4 uses those native contracts instead of adding JSON repair or retries.

## Goals

- Make every H4 model invocation request schema-conforming JSON at the provider transport boundary.
- Preserve the existing strict decoder and semantic validators as postconditions.
- Preserve H1-H3 runtime behavior when no output schema is supplied.
- Preserve all H3 benchmark semantics except the transport contract and H4 identity.
- Keep failures fail-closed with complete invocation evidence.

## Version boundary

H4 introduces benchmark ID `h4` and H4 dataset/run/result schema markers. `benchmarks/h4` is semantically identical to H3 for rubric, problems, reference sets, case order, category split, challenger schedule, comparator, `MaterialWorseDelta`, and citation authority. Only H4 identity/schema fields and consequent top-level hashes differ.

H1-H3 commands on later `main` must retain their previous prompt-only transport behavior. No real H4 model call occurs before H4 implementation/data and a separately pinned H4 execution workflow are merged and frozen.

## Runtime contract

Extend `runtime.AgentRequest` with optional `OutputSchema json.RawMessage`. Empty means legacy behavior. Non-empty schema must be one valid JSON object before any provider process starts.

Claude runtime passes the compact schema inline with `--json-schema` and switches only schema-enabled calls from legacy `--output-format text` to `--output-format json`. Claude therefore emits a raw JSON result envelope; invocation logging records that envelope unchanged, and the H4 structured-output wrapper extracts only its `structured_output` object before existing strict decoding. Codex runtime writes the compact schema to a temporary `0600` file outside the model workspace and run artifact tree, passes that path with `--output-schema`, and removes the file after the invocation. Schema materialization or envelope extraction failure is fail-closed; there is no prompt-only fallback.

`invocationlog.Evidence` records `output_schema_sha256,omitempty` when a schema is supplied, giving H4 an auditable contract digest while leaving legacy H1-H3 evidence shape unchanged when empty.

## Schema injection boundary

H4 constructs the normal provider runtimes, wraps them with invocation logging, then wraps those runtimes with one H4 schema injector. The injector selects a frozen schema from request `role` + `phase`. It rejects pre-populated `OutputSchema` values so callers cannot override the frozen H4 contract. Unknown H4 role/phase combinations fail closed rather than running without a schema. The wrapper sits outside invocation logging, so logging receives the schema-bearing request and raw provider response before Claude envelope extraction.

The schema injector covers every H4 model call:

- baseline `baseline-draft` and `baseline-final` → `baseline.AnswerArtifact`
- protocol `research` → `protocol.ResearchArtifact`
- protocol `review` → `protocol.ReviewArtifact`
- protocol `challenge` → `protocol.ChallengeArtifact`
- protocol `rebuttal` → `protocol.RebuttalArtifact`
- protocol `judge` → `protocol.JudgeArtifact`
- evaluator `eval-judge` → `evalharness.JudgeArtifact`

This keeps baseline, protocol, and evaluator free of H4 conditionals. H1-H3 do not install the injector.

## Frozen schemas

Schemas require an object, enumerate every top-level property, require every non-optional output field, and set `additionalProperties: false`. Nested evidence/citation objects are equally closed. Arrays have typed items. Numeric fields are typed as numbers and boolean fields as booleans.

Provider compatibility takes precedence over encoding non-structural domain rules into JSON Schema. Score/confidence ranges, citation authority, and other semantic invariants remain enforced by existing Go validators after strict decoding. Because strict provider schemas require closed objects, eval `dimensions` is a closed object containing exactly the five frozen rubric IDs as required numeric properties; the evaluator still re-validates those exact IDs and score ranges as a postcondition.

Schema definitions are versioned production data in a focused `structuredoutput` package. Tests reflect over the Go artifact structs and assert property/required-key parity so a struct change cannot silently drift from its H4 schema.

## Failure and concurrency semantics

Provider-native schema enforcement does not change call budget. There is no malformed-output retry, JSON repair, resume, or provider substitution. A provider process failure remains a runtime failure. A provider returning output that still fails strict decoding or semantic validation remains malformed output.

Existing `parallel2` cancellation semantics remain unchanged: the first peer failure cancels the sibling. Invocation evidence may therefore record a cancelled sibling after the initiating failure. H4 does not reinterpret that cancellation as an independent model defect.

## H4 orchestration and evaluation

H4 reuses the H3 sequential 20-problem runner, incremental A-F persistence, evaluator order, risk policy, challenge policy, subscription-only auth, and strict problem-only final citation authority. The H4 CLI exposes the same operational flags as H3 and no policy overrides.

The H4 constructor installs the structured-output injector around both provider runtimes before the same wrapped runtimes are passed to baseline and evaluator. This guarantees the transport contract applies to A-D, E/F internal phases, and both evaluation judges.

## Execution freeze

After implementation/data merge, record the exact `H4_FROZEN_SHA`. A separate manual-only H4 workflow must hard-pin that SHA, require one ephemeral Linux runner labeled `h4-benchmark`, verify subscription auth and frozen dataset hashes, run repository tests, execute H4 exactly once, and always upload full/partial evidence for 90 days. No automatic redispatch is permitted.

## Acceptance tests

1. Legacy runtime requests with no schema produce the exact existing Claude/Codex argument shape.
2. Claude H4 requests receive compact inline `--json-schema`; Codex H4 requests receive a temporary schema file via `--output-schema`, with exact content and cleanup verified.
3. Invalid or non-object schemas fail before provider execution and never fall back to prompt-only mode.
4. H4 schema injection covers baseline, all five protocol phases, and eval judges; unknown H4 role/phase fails closed.
5. Frozen schema top-level and nested object keys match the corresponding Go artifact structs.
6. H1-H3 tests remain green and H4 alone enables structured schemas plus H3 strict citation authority.
7. H4 dataset is semantically equal to H3 except identity/schema markers and top-level hashes.
8. Full gofmt/test/vet/lint/race and exact-head CI/CLA pass before each merge.
9. No real H4 model call occurs until both implementation/data and pinned execution workflow are merged and frozen.
10. Exactly one H4 real run is dispatched; terminal evidence determines success or a new version boundary, never an automatic rerun.

## Non-goals

- No JSON repair or extraction from prose.
- No retries for malformed output.
- No changes to model/provider selection, call counts, visibility, deliberation topology, scoring, comparator, material-worse threshold, or citation authority.
- No generic JSON-Schema generator or public schema API.
- No database/dashboard/resume system.
