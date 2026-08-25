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

The project is not yet claiming that multi-agent councils are better. The first milestone is to measure whether they are.

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

The v0 public core is subscription-first, but the runtime boundary is intentionally designed so future releases can support both subscription and direct API execution.

The permanent rule is **explicit mode selection**:

- subscription execution stays fail-closed
- API execution is a separate runtime
- subscription quota exhaustion never silently falls back to a metered API

See [`docs/FUTURE_RUNTIME_EXPANSION.md`](docs/FUTURE_RUNTIME_EXPANSION.md).
