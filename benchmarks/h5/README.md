# H5 benchmark dataset

H5 preserves the H4 problem/reference payloads, rubric semantics, case order, comparator, material-worse policy, citation authority, and structured-output contract. H5 removes provider identity from case metadata and freezes logical-slot adapter chains in `adapter-policy.json`.

Default chains use subscription-backed `claude-max`, `codex-chatgpt`, and `antigravity-subscription` (Google Antigravity CLI pinned to `gemini-3.1-pro-high`), followed by `human-chatgpt-session` as the final availability-only fallback. `codex-chatgpt` means Codex CLI authenticated through ChatGPT; `human-chatgpt-session` is a distinct manual New Chat broker with fresh-session attestation. Every realized adapter attempt is evidence-logged.

Frozen SHA-256:
- `manifest.json`: `d7ba2ffdd427733871a767b8733bcd98af2a457e23dbb261353bc61b61ac951d`
- `rubric.json`: `59b82a515549a59e4bf84dda6f906db359a31b7bdb9306ad3016a1222713cb11`
- `cases.json`: `95f08d7847dcd41a25ebfc0d7d904ed3b09011feeb3d038e7dd43fafe83847f9`
- `adapter-policy.json`: `e12e67dac8af5f7cba704a36f2b3030a898ae869bac4ce4573421b2e2a93d890`
