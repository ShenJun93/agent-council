# Agent Council

Agent Council is an experimental open-core project for evaluating whether blind, heterogeneous AI-agent deliberation can produce better technical and product decisions than strong single-agent baselines.

The v0 implementation is intentionally small:

- Claude Code CLI
- Codex CLI
- subscription-first execution
- blind independent research
- controlled cross-review
- adversarial challenge and rebuttal
- dual independent judges
- Visibility Firewall enforced at the filesystem boundary
- reproducible evaluation against single-agent baselines

## Status

**Pre-v0 / validation-first.**

The governed Phase H technical value-validation pilot completed on September 3, 2026. It evaluated the frozen 10-case technical corpus with 120 evaluator calls and produced a pre-registered **FAIL** outcome: Council mean delta versus the strongest frozen single-agent baseline was `-1.38`, with zero materially-worse cases.

Accordingly, the project does **not** claim that Council has demonstrated measurable value over strong single-agent baselines. Product-domain value remains untested because the frozen source artifacts did not contain a complete product A-F candidate set. See issue [`#56`](https://github.com/ShenJun93/agent-council/issues/56) and workflow run [`33747506790`](https://github.com/ShenJun93/agent-council/actions/runs/33747506790).

## Open-core model

This repository contains the public core and is licensed under **AGPL-3.0-only**.

Commercial/proprietary extensions may be developed separately. The public core never depends on proprietary code. See [`OPEN_CORE_BOUNDARY.md`](OPEN_CORE_BOUNDARY.md).

## Contributing

External contributions require acceptance of the Contributor License Agreement before merge. See [`CONTRIBUTING.md`](CONTRIBUTING.md) and [`docs/CLA.md`](docs/CLA.md).

## Security

Please follow [`SECURITY.md`](SECURITY.md). Do not publish credentials or vulnerability details in public issues.

## License

AGPL-3.0-only. Before the initial public release, install the complete canonical license text with:

```bash
bash scripts/install-agpl-license.sh
```

## Future runtime direction

The public core remains subscription-first, and the runtime boundary is intentionally designed so a future release could support both subscription and direct API execution.

The permanent rule is **explicit mode selection**:

- subscription execution stays fail-closed
- API execution is a separate runtime
- subscription quota exhaustion never silently falls back to a metered API

Direct API/BYOK work is **not currently authorized** because the Phase H measurable-value gate was not satisfied. See [`docs/FUTURE_RUNTIME_EXPANSION.md`](docs/FUTURE_RUNTIME_EXPANSION.md).

## Development status

The core implementation, governance, benchmark infrastructure, and frozen technical Phase H pilot are complete. Any further value-validation work or runtime expansion requires separate governance; the completed Phase H run must not be reinterpreted post hoc.
