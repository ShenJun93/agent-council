# Contributing

## Before opening a pull request

1. Keep changes focused and small.
2. Add or update tests for behavior changes.
3. Run the local Go quality checks once the Go module exists.
4. Do not add secrets, API keys, tokens, or private benchmark data.

## Contributor License Agreement

External contributions require the Agent Council CLA.

The CLA workflow will ask you to comment:

```text
I have read the CLA Document and I hereby sign the CLA
```

The agreement is in [`docs/CLA.md`](docs/CLA.md).

Automated dependency/action PRs from the configured bot actors are exempt.

## Public/proprietary boundary

Do not introduce dependencies from the public core to proprietary Agent Council code.

Any third-party code added to a path intended for proprietary reuse must have licensing compatible with DAITHIENSTUDIO's relicensing rights. See [`OPEN_CORE_BOUNDARY.md`](OPEN_CORE_BOUNDARY.md).

## Pull requests

- Prefer squash or rebase merges.
- Keep generated files out of commits unless the repository explicitly tracks them.
- Explain observable behavior changes.
- Call out licensing implications when adding dependencies.
