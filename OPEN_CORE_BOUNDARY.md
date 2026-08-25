# Open-Core Boundary

## Purpose

This document defines the one-way dependency and licensing boundary between:

- `agent-council` — public core, AGPL-3.0-only
- `agent-council-pro` — private proprietary/commercial extensions

## Rules

### 1. Public core never depends on Pro

`agent-council` must build, test, and operate without access to `agent-council-pro`.

The public repository must not import packages, copy files, fetch artifacts, or require services that exist only in Pro.

### 2. Dependency direction is one way

Allowed:

```text
agent-council-pro  --->  agent-council
```

Forbidden:

```text
agent-council  --->  agent-council-pro
```

### 3. Process boundary is the safest default

Where practical, Pro should interact with public core through documented process, CLI, RPC, or protocol boundaries.

This reduces accidental coupling and makes licensing boundaries easier to audit.

### 4. Direct Go linking/import requires separate rights

Pro may directly import/link public-core Go packages only when DAITHIENSTUDIO has sufficient separate rights to distribute that combined proprietary work.

Do **not** assume that the public AGPL license by itself grants a safe proprietary-linking path.

### 5. CLA is required for external public-core contributions

External contributions are merged only after the contributor accepts [`docs/CLA.md`](docs/CLA.md).

The CLA is intended to preserve DAITHIENSTUDIO's ability to sublicense/relicense contributed code for separately licensed editions.

### 6. Third-party licensing must be audited

Do not place third-party AGPL-only, GPL-only, or otherwise commercially incompatible code into components intended for proprietary reuse unless separate rights are obtained.

Record material third-party license obligations when dependencies are introduced.

### 7. Copying between repos is an explicit licensing event

Moving code from Pro to public core, or from public core to Pro, must be intentional and reviewed for:

- copyright ownership
- CLA coverage
- third-party dependencies
- applicable license obligations

### 8. Public core remains independently useful

Do not deliberately cripple public core merely to force Pro adoption.

Commercial differentiation should come from separately valuable capabilities, hosting, enterprise operations, integrations, proprietary datasets/evals, or other extensions.

## Legal note

This document is an engineering/governance boundary, not legal advice. Obtain legal review before significant commercialization.

## Future runtime/API boundary

The public core may define transport-neutral runtime contracts that can be implemented by subscription or API runtimes.

For v0, the public implementation remains subscription-only.

Possible future commercial extensions may include:

- direct provider API runtimes
- BYOK
- tenant-scoped credential management
- usage/cost controls
- enterprise provider policies
- hosted execution

Those features must not create a dependency from public core to Pro.

If Pro directly imports/links public-core Go packages, the separate-rights rule above still applies.

No implementation may silently switch a subscription-backed runtime to metered API execution.
