# H5 benchmark dataset

H5 preserves the H4 problem/reference payloads, rubric semantics, case order, comparator, material-worse policy, citation authority, and structured-output contract. H5 removes provider identity from case metadata and freezes logical-slot adapter chains in `adapter-policy.json`.

Default chains use subscription-backed `claude-max` and `codex-chatgpt`, followed by `human-chatgpt-session` as the final availability-only fallback. `codex-chatgpt` means Codex CLI authenticated through ChatGPT; `human-chatgpt-session` is a distinct manual New Chat broker with fresh-session attestation. Every realized adapter attempt is evidence-logged.

Frozen SHA-256:
- `manifest.json`: `34e8a185218e676e1cd573a05cf5f3741f2c32fb90f6422907bc0c5e4aa2b664`
- `rubric.json`: `59b82a515549a59e4bf84dda6f906db359a31b7bdb9306ad3016a1222713cb11`
- `cases.json`: `95f08d7847dcd41a25ebfc0d7d904ed3b09011feeb3d038e7dd43fafe83847f9`
- `adapter-policy.json`: `9cbbc147ead9497643c2adfe0f39cf2c548652b35c436f997514583acccee773`
