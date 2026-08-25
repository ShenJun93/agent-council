# GitHub Setup Checklist

## Repository creation

- [ ] Create `agent-council` under the personal GitHub account.
- [ ] Keep it private during bootstrap if you want to review legal/setup files before publication.
- [ ] Copy this scaffold.
- [ ] Run `bash scripts/install-agpl-license.sh`.
- [ ] Verify `LICENSE` contains the full GNU Affero GPL v3 text.
- [ ] Push initial scaffold to `main`.

## Repository settings

### General

- [ ] Default branch: `main`.
- [ ] Allow squash merge.
- [ ] Allow rebase merge.
- [ ] Disable merge commits so linear-history protection is satisfiable.

### Actions

- [ ] Settings → Actions → General.
- [ ] Set default `GITHUB_TOKEN` workflow permissions to **read repository contents and packages**.
- [ ] Keep workflow write permissions explicit inside workflows that need them.
- [ ] Configure an Actions budget and enable **Stop usage when budget limit is reached**.

### Security

For the public repository:

- [ ] Enable secret scanning where available.
- [ ] Enable push protection.
- [ ] Enable Dependabot alerts/security updates as desired.
- [ ] Enable private vulnerability reporting if available.

## Rulesets

### First stage

- [ ] Import `rulesets/01-main-base.json`.

### Bootstrap checks

- [ ] Create a real pull request.
- [ ] Confirm the CI check runs and record its exact displayed context.
- [ ] Confirm the CLA check runs and record its exact displayed context.
- [ ] Expected contexts are `CI / quality` and `CLA Assistant / cla`; if GitHub displays different names, edit `rulesets/02-main-with-checks.json` before importing it.
- [ ] Sign/recheck CLA if needed.

### Second stage

- [ ] Import/replace with `rulesets/02-main-with-checks.json`.
- [ ] Confirm both status checks are required using the exact contexts observed on the bootstrap PR.
- [ ] Confirm the ruleset still targets only `~DEFAULT_BRANCH` and does not cover `cla-signatures`.
- [ ] Do not enable "do not allow bypassing" while solo unless intentionally desired.

## CLA signatures

The CLA workflow stores signatures in:

```text
branch: cla-signatures
path: signatures/cla.json
```

GitHub usernames and signature timestamps in that file are public.

## Publication

Before changing visibility to public:

- [ ] Full AGPL license is installed.
- [ ] CLA text reviewed.
- [ ] `OPEN_CORE_BOUNDARY.md` reviewed.
- [ ] No credentials in history.
- [ ] Push protection enabled.
- [ ] README accurately says pre-v0 / experimental.

## Pro repository

- [ ] Create `agent-council-pro` as private.
- [ ] Copy the Pro scaffold.
- [ ] Do not add public-core code by copy/paste without boundary review.
- [ ] Do not add GitHub Actions until the private repo has actual code that needs CI.
