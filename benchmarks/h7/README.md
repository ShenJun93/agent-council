# H7 benchmark dataset

H7 preserves H6 problem/reference payloads, rubric semantics, case order, adapter policy bytes, slot topology, and availability-only failover behavior. Only H7 identity/schema markers and the claim-aware evaluator citation wire contract differ: evaluator citation identity is the full `(artifact_id, locator, claim)` tuple instead of H6's `(artifact_id, locator)` pair, so two candidate citation occurrences that share a source location but carry different claims are both preserved instead of colliding.

Frozen SHA-256:
- `manifest.json`: `e6c0d380048d696c8a4c0c40680dd809661860ce99004b490c7c0c820cd3081f`
- `rubric.json`: `ce43c11d7b654ea01f42c5a2699091f1f87828946b002fbc65dc6e32df4fa7bc`
- `cases.json`: `a3e908b2520fae3fd703998b0df59c2f5b228b6b05b8f109237b46806561c451`
- `adapter-policy.json`: `e12e67dac8af5f7cba704a36f2b3030a898ae869bac4ce4573421b2e2a93d890` (byte-identical to H6)
