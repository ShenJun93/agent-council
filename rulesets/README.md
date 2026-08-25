# Ruleset Bootstrap

GitHub required checks have a chicken-and-egg constraint: the check must have run before it can be reliably selected/required.

## Stage 1

After the initial scaffold is pushed, import:

```text
01-main-base.json
```

This enables:

- require pull request
- zero required human approvals
- linear history
- block deletion
- block force-push / non-fast-forward updates

It intentionally does **not** require CI or CLA status checks yet.

## Stage 2

Open a real bootstrap pull request and allow these workflows to run at least once:

```text
CI / quality
CLA Assistant / cla
```

After GitHub has observed both checks, import/replace the ruleset with:

```text
02-main-with-checks.json
```

Review the imported ruleset in the GitHub UI before activating it.

For a solo personal repository, do not enable settings that prohibit the repository administrator from bypassing protections unless you intentionally want hard self-locking.


## Verify the actual required-check contexts

The committed stage-2 ruleset currently expects:

```text
CI / quality
CLA Assistant / cla
```

These are derived from the workflow name plus job name, but **do not import stage 2 based on this assumption alone**.

After opening the bootstrap PR:

1. Inspect the PR checks in the GitHub UI, or run `gh pr checks <PR_NUMBER>`.
2. Record the exact contexts GitHub shows.
3. If either context differs, edit `02-main-with-checks.json` to match the observed value exactly.
4. Only then import/activate the stage-2 ruleset.

## Keep `cla-signatures` outside main protection

Both rulesets intentionally target only:

```text
~DEFAULT_BRANCH
```

The CLA workflow writes signature commits to:

```text
cla-signatures
```

Do not broaden the branch condition to `*`, `**`, or all branches without explicitly excluding `cla-signatures`, or the CLA bot may lose permission to persist signatures.
