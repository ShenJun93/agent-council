# H8 benchmark dataset

H8 preserves H7 problem/reference payloads, rubric semantics, case order, adapter policy bytes, slot topology, claim-aware citation identity, and availability-only failover behavior. H8 changes only version/schema identity plus the evaluator score-reliance wire contract: citation verification is `verified|unverified`, score reliance is co-located as `relied_on`, and model-authored `relied_on_citations` is removed from the H8 wire schema.

Frozen SHA-256:
- `manifest.json`: `785a87b4b0cf13ec6edaad67202ca842157274951a6d74c0f993b111875d554b`
- `rubric.json`: `644d0ee576c4564b124c2303a9208c142e261b9ba6ef96d78ad706616c17b952`
- `cases.json`: `e36610729f0e5178a55eb28db1263114d8dbd477186153848123e8c2ed3249fc`
- `adapter-policy.json`: `e12e67dac8af5f7cba704a36f2b3030a898ae869bac4ce4573421b2e2a93d890` (byte-identical to H7)
