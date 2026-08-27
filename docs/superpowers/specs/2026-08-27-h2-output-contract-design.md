# H2 Output Contract and Failure Evidence Design

## Status

H1 is frozen and inconclusive. Real H1 attempt 1 stopped on Codex subscription quota exhaustion. Real H1 attempt 2 stopped on Claude arm A with `malformed_output` because stdout began with a backtick while the frozen baseline parser required a raw JSON value. H1 code, data, policies, and artifacts remain unchanged after those calls.

H2 is a new benchmark version. It preserves H1 decision content and evaluation semantics while changing only the model-output transport contract and failure-evidence persistence required to make runs auditable and executable.

## Goals

1. Prevent Markdown code fences from turning otherwise valid structured model answers into transport failures.
2. Apply the same transport normalization to Claude and Codex and to every model-generated JSON artifact.
3. Persist exact model stdout/stderr before semantic parsing so malformed output is auditable.
4. Persist completed baseline arms incrementally so a later failure does not erase earlier evidence.
5. Make the H2 version boundary explicit in committed benchmark data, CLI routing, run/result schemas, and future execution workflow.
6. Preserve H1 unchanged and never resume or reuse H1 partial runs.

## Non-goals

- No prompt-content tuning, model substitution, threshold tuning, scoring changes, rubric changes, challenge-routing changes, retry-on-malformed-output, or provider-specific cleanup heuristics.
- No metered API fallback.
- No automatic resume from a partial H2 run.
- No attempt to reinterpret failed H1 outputs as successful H2 outputs.

## Frozen semantics inherited from H1

H2 keeps the same 20 cases (10 technical, 10 product), shared five-dimension equal-weight rubric, comparator `best_single`, `MaterialWorseDelta=10.0`, full challenge path, odd/even challenger routing, A-F arm topology, fixed Claude+Codex Phase G judges, isolation rules, subscription-first auth policy, no retries/provider substitution, and fail-closed behavior.

## Version boundary

Add benchmark ID `h2` and committed `benchmarks/h2` data. H2 cases, evidence facts, reference claims, options, constraints, and rubric dimensions/weights are semantically identical to H1. H2 files use H2 schema/version identifiers so their hashes are independently frozen. A committed-data test must compare H1 and H2 decoded semantic content after removing version-only fields.

H2 run and result manifests use H2 schema identifiers and benchmark ID. H1 loaders, schemas, CLI command, and artifacts remain readable and unchanged.

## Common model-output transport contract

Create one shared decoder used by baseline, protocol, and evaluation model-output parsing.

Accepted forms after trimming leading/trailing whitespace:

1. A single JSON object whose first non-whitespace byte is `{`.
2. A single Markdown fence with opening line exactly ` ``` ` or ` ```json ` (without surrounding spaces), body containing exactly one JSON object, and closing line exactly ` ``` `.

The decoder rejects:

- prose before or after a fence,
- fence language tags other than empty or `json`,
- nested or multiple fences,
- multiple JSON values,
- trailing non-whitespace output,
- non-object top-level JSON,
- malformed JSON,
- unknown struct fields through `json.Decoder.DisallowUnknownFields`.

The decoder performs serialization normalization only. It does not repair JSON, infer missing fields, remove prose, change values, or retry a provider. Existing semantic validators still enforce required fields, confidence ranges, citations, and artifact IDs.

Prompts remain unchanged. Claude remains on `--output-format text`; Codex invocation flags remain unchanged. This avoids provider-specific structured-output features that could constrain one provider differently from the other.

## Invocation evidence

Add an `invocationlog` runtime decorator implementing `runtime.AgentRuntime`. H2 wraps both provider runtimes once and passes the wrapped runtimes through baseline, protocol, and eval harness.

For every actual provider execution response (success or failure after the model process starts), the decorator writes a create-only JSON record before returning to the caller. Auth-preflight output is not recorded.

Record fields:

- schema version,
- run ID,
- provider,
- participant,
- role,
- phase,
- prompt SHA-256,
- stdout,
- stderr,
- exit code,
- attempt count,
- started_at,
- finished_at.

Records live under:

`invocations/<provider>/<participant>/<phase>/<sequence>.json`

Sequence is monotonic per wrapped provider runtime and concurrency-safe. Path components are validated as safe single components. Writes are create-only, component-wise symlink-safe, and must remain under the run root. Failure to persist invocation evidence is an isolation failure and fails the run closed. If both provider execution and evidence persistence fail, return both errors.

## Safe immutable writes

Extract the existing component-wise containment, symlink, and exclusive-write logic used by benchmark/artifact persistence into a small internal helper package so invocation evidence does not duplicate weaker path logic. Existing stores migrate to the helper without changing their on-disk contract.

## Incremental baseline persistence

H1 `baseline.Runner.RunAll` remains available for compatibility. Export a frozen arm-order accessor returning a copy of A-F.

H2 orchestration runs one arm at a time with `RunArm`. After each arm succeeds, it immediately writes that arm to `baseline/<problem-id>/arm-<A-F>.json` using a create-only `WriteBaselineArmResult` API. The H2 runner never overwrites an arm and never resumes a partial run.

If an arm fails:

- previously completed arm files remain,
- invocation evidence for the failing provider call remains if a model process started,
- no later arm runs,
- no evaluator runs,
- no batch summary or result manifest is produced.

When all six arms are present, evaluation proceeds with the unchanged H1/H2 evaluation policy and summary semantics.

## H2 CLI and dataset

Add `agentd council benchmark h2` with the same operational flags as H1 and no policy override flags. Default dataset is `benchmarks/h2`. The H2 loader validates exactly 20 cases, category split/order, challenger routing, risk policy, rubric dimensions/weights, evidence/reference mirroring, and frozen hashes.

H2 creates fresh run IDs prefixed `h2-`. H2 never reads or resumes H1/H2 partial run directories.

## Execution boundary

The implementation/data PR must merge before any real H2 model call. After merge, record the exact main SHA as the H2 frozen implementation SHA. A separate ops PR adds a self-hosted Linux execution workflow pinned to that frozen SHA. The workflow exposes no benchmark-policy inputs and uploads full or partial evidence on every terminal outcome.

## Testing

Required TDD coverage:

- shared decoder accepts raw JSON and `json`/untagged single fences;
- rejects prose, unsupported tags, nested/multiple fences, multiple JSON values, trailing text, malformed/non-object JSON, and unknown fields;
- baseline/protocol/eval parsing use the shared decoder;
- invocation evidence is written for successful and failed provider executions before caller parsing;
- evidence paths reject unsafe components/symlinks and never overwrite;
- concurrent invocation sequences do not collide;
- incremental baseline arm writes preserve earlier arms after a later failure;
- H1 behavior/contracts remain compatible;
- H2 committed dataset is semantically identical to H1 except version identity;
- H2 CLI routing/defaults/no-overrides;
- full `gofmt`, `go test ./...`, `go vet ./...`, and lint gate.

## Success criterion

H2 is ready for its first real run only when implementation and dataset are merged on main, exact-head CI and CLA are green, the H2 frozen SHA is recorded, and the separate execution workflow is pinned to that SHA. A failed H2 run remains a valid fail-closed execution record; it is never resumed or silently retried.