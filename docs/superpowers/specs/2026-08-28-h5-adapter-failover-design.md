# H5 Provider-Agnostic Adapter Failover Design

**Status:** approved architecture for Issue #31.

## Goal
H5 removes provider names from council role policy. Claude, Codex, and future ChatGPT runtimes are adapter instances that can be bound to logical slots through a frozen ordered failover policy. Provider quota must not block a run while another approved subscription-backed adapter remains available.

## Historical boundary
H1-H4 remain immutable. H4 is frozen/inconclusive and will not be rerun. H5 preserves the H4 problem corpus, rubric, citation authority, structured-output contract, visibility firewall, six A-F wire arm IDs, evaluation statistics, and full-challenge behavior except where provider-binding semantics are explicitly versioned here.

## Identity model
Three identities are distinct:
- `ProviderFamily`: transport/model family such as `claude`, `codex`, or future `chatgpt`.
- `AdapterID`: concrete authenticated runtime instance, e.g. `claude-max`, `codex-chatgpt`, future `chatgpt-direct`.
- `SlotID`: logical council responsibility, independent of provider.

Initial H5 adapter registry contains `claude-max -> claude` and `codex-chatgpt -> codex`. A future direct ChatGPT adapter may be registered without changing protocol semantics.

## Logical slots
H5 uses these slot identities: `baseline-a`, `baseline-b`, `researcher-1`, `researcher-2`, `reviewer-1`, `reviewer-2`, `challenger`, `judge-1`, `judge-2`, `eval-judge-1`, and `eval-judge-2`.

A/C use `baseline-a`; B/D use `baseline-b`. Protocol E/F use the seven council slots. Evaluation uses the two eval judge slots.
## Frozen binding policy
Each slot owns an ordered adapter chain. H5 default chains preserve H4's original primary-family orientation while permitting failover:
- A-side slots (`baseline-a`, researcher/reviewer/judge `-1`, eval-judge-1): `[claude-max, codex-chatgpt]`.
- B-side slots (`baseline-b`, researcher/reviewer/judge `-2`, eval-judge-2): `[codex-chatgpt, claude-max]`.
- Challenger primary orientation follows the existing case challenger schedule; the other adapter is second.

The complete slot-to-chain mapping is committed in the H5 benchmark manifest and contributes to its frozen hash boundary. No CLI flag may override the policy during a frozen H5 run.

## Failover semantics
An adapter pool invokes at most once per adapter in chain order. H5 sets `AgentRequest.MaxAttempts=1`; zero remains legacy behavior and means two attempts for H1-H4 CLI runtimes.

Failover is permitted only when every failure class present in the returned error is an availability class. Initial availability classes are `quota_exhausted`, `auth_failure`, and explicit `adapter_unavailable`. Rate-limit/capacity text remains classified as `quota_exhausted` by the existing runtime classifier.

Do not fail over on `timeout`, `process_failure`, `malformed_output`, `billing_policy_violation`, `isolation_failure`, citation/semantic validation failure, or any model-quality outcome. Those are terminal outcomes. If all adapters are unavailable, return `adapter_pool_exhausted` containing the ordered availability failures.

Cancellation of a parallel sibling remains a consequence, not the initiating failure; evidence analysis must preserve this distinction.

## Evidence v2
Legacy `invocationlog.Wrap` remains v1. H5 uses `WrapAdapter` and `council.invocation-evidence.v2`.

V2 records: run ID, slot ID, adapter ID, provider family, participant, role, phase, failover index, failover trigger, prompt SHA-256, output-schema SHA-256, raw stdout/stderr, exit code, attempts, start/finish timestamps, and terminal failure class when present.

V2 persists an adapter attempt even when auth/quota/unavailable failure occurs before model execution. When the inner runtime produces no timestamps, the evidence wrapper records wrapper-level attempt start/finish timestamps without pretending a model process succeeded. Evidence path is `invocations/<adapter-id>/<participant>/<phase>/<sequence>.json`.
## Runtime composition
For each concrete adapter, H5 composes wrappers in this order from inner to outer:
`provider CLI -> invocationlog.WrapAdapter -> structuredoutput.Wrap`.
The slot-level `adapterpool.Runtime` wraps the resulting adapter runtimes. Raw CLI output is therefore persisted before Claude envelope extraction or other structured-output normalization.

`AgentResponse` gains optional adapter metadata (`AdapterID`, `SlotID`, `FailoverIndex`) populated by the pool. Existing provider family remains the raw provider that actually produced the response.

## Protocol and baseline compatibility
`protocol.Engine` gains optional `SlotRuntimes`. When supplied, every research/review/challenge/rebuttal/judge call resolves from logical slots and no provider assertion is made. When absent, the existing Claude/Codex fields and challenger-provider validation remain unchanged.

`baseline.Runner` gains optional H5 slot runtimes: A/C use slot A, B/D use slot B, and E/F use adaptive protocol slots. Existing Claude/Codex validation remains the fallback legacy path. A-F remain the wire arm identifiers; H5 docs and provenance call A/C slot-A and B/D slot-B rather than Claude/Codex arms.

## Evaluator compatibility
`evalharness.Harness` gains optional adaptive judge runtimes. Legacy mode still requires fixed Claude + Codex and enforces the expected provider. Adaptive mode invokes `eval-judge-1` and `eval-judge-2` slot runtimes, records `response.Provider`, adapter metadata, and does not require a specific provider family.

A provider failover never changes candidate visibility, rubric/reference-set visibility, masking, or score math.

## H5 artifacts and interpretation
Every H5 arm artifact and eval problem result includes execution provenance summarizing actual adapter selections and failover counts. The final H5 result also reports effective provider diversity and total availability failovers so a Council score obtained after both logical sides converged onto one adapter is not misrepresented as heterogeneous execution.

Failover changes operational continuity, not the claim being tested: H5 results must state the frozen preferred binding policy and the realized adapter binding trace.
## H5 dataset and freeze
`benchmarks/h5` is semantically identical to H4 for problems, reference sets, rubric, risk policy, challenge mode, citation authority, and material-worse threshold. H5 replaces provider-specific challenger policy with adapter-chain binding policy while preserving the prior odd/even primary orientation.

The manifest records adapter registry identities, provider families, every slot chain, availability classes, and same-adapter max attempts = 1. Dataset/rubric/cases/policy hashes are verified before execution.

No real H5 model call occurs before implementation/data merge and exact frozen SHA are recorded. After freeze, use generic workflow renderer/bootstrap/dispatch tooling merged at `390eb97f240a7cb219fb97689096f733fab1c788`.

## Operational policy
Provider quota is not a scheduling dependency. Preflight may report an adapter unavailable but must permit the run when every required slot still has at least one available adapter. During execution, quota/auth availability can also trigger the next adapter without human intervention.

Metered API credentials stay forbidden. No hidden provider substitution is permitted: every substitution is the deterministic next item in the frozen chain and is recorded before execution continues.

## Non-goals
H5 does not add a direct ChatGPT transport if no authenticated CLI/runtime is present; it only ensures such an adapter can be registered later without protocol changes. H5 does not perform load balancing, quality-based routing, retries after malformed output, model-response repair, dynamic scoring-based provider selection, or automatic chain mutation.

## Acceptance
Tests must prove: legacy H1-H4 behavior remains green; reviewer/judge slots can run with either provider; quota on primary falls through exactly once to secondary; malformed output never falls through; same-adapter process retry is disabled in H5; v2 evidence preserves both unavailable and successful attempts; evaluator records actual provider; adapter pool exhaustion is deterministic; H5 policy hashes are frozen; and a no-model preflight succeeds when Claude is unavailable but Codex-backed slot chains remain viable.