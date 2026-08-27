# H2 Benchmark

H2 is the versioned successor to frozen H1 after real H1 execution exposed transport/audit failures.

H2 intentionally preserves H1 benchmark semantics: the same 20 curated cases, candidate-visible problem evidence, evaluator-only corroboration, rubric dimensions/weights, A-F arm topology, challenger schedule, comparator `best_single`, and `MaterialWorseDelta=10.0`.

The version boundary changes only model-output transport tolerance and execution evidence semantics implemented in code. H2 accepts exactly one raw JSON object or one untagged/`json` Markdown fence containing exactly one JSON object, preserves raw invocation evidence before parsing, and persists completed baseline arms incrementally.

`manifest.json` freezes the H2 identity and SHA-256 digests of `rubric.json` and `cases.json`. Problem/reference payloads remain semantically identical to H1; tests enforce this invariant.

No H1 artifact, result, or previous model response is reinterpreted as H2. Every real H2 execution starts with a fresh H2 run ID and no resume/provider substitution/metered fallback.
