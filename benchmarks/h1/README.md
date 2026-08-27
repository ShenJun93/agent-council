# H1 benchmark

H1 is the first frozen validation benchmark for Agent Council v0.

- 20 synthetic, evidence-bounded decision problems: 10 technical and 10 product.
- Every candidate-required fact is present in each problem packet.
- Reference sets only corroborate evidence already visible in the problem.
- One shared five-dimension equal-weight rubric.
- `best_single` comparator with `MaterialWorseDelta = 10.0`.
- Full Phase F challenge path; odd global cases use Claude challenger and even cases use Codex.
- No benchmark content or scoring-policy changes after the first real H1 model call begins.

`manifest.json` freezes exact input hashes and case order. `LoadH1` verifies the complete dataset before any model call.
