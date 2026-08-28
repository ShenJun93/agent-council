#!/usr/bin/env bash
set -euo pipefail

REPO="ShenJun93/agent-council"
BENCHMARK=""
ISSUE=""
ATTEMPT=""
WORKFLOW=""
POLL_SECONDS="${BENCHMARK_DISPATCH_POLL_SECONDS:-1}"

usage() {
  cat <<'EOF'
Usage: bash scripts/dispatch-frozen-benchmark.sh \
  --benchmark <id> --issue <n> --attempt <n> --workflow <file>
EOF
}

die() {
  echo "benchmark-dispatch: $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --benchmark) BENCHMARK="${2:-}"; shift 2 ;;
    --issue) ISSUE="${2:-}"; shift 2 ;;
    --attempt) ATTEMPT="${2:-}"; shift 2 ;;
    --workflow) WORKFLOW="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; die "unknown argument: $1" ;;
  esac
done
[[ "$BENCHMARK" =~ ^[a-z][a-z0-9-]{0,31}$ ]] || die "invalid benchmark id: $BENCHMARK"
[[ "$ISSUE" =~ ^[1-9][0-9]*$ ]] || die "invalid issue number: $ISSUE"
[[ "$ATTEMPT" =~ ^[1-9][0-9]*$ ]] || die "invalid attempt number: $ATTEMPT"
[[ "$WORKFLOW" == "${BENCHMARK}-frozen-execution.yml" ]] || die "workflow must be ${BENCHMARK}-frozen-execution.yml"
[[ "$POLL_SECONDS" =~ ^[0-9]+$ ]] || die "invalid poll interval: $POLL_SECONDS"
(( POLL_SECONDS <= 30 )) || die "poll interval exceeds 30 seconds"

command -v gh >/dev/null 2>&1 || die "required command not found: gh"
gh auth status >/dev/null 2>&1 || die "GitHub CLI is not authenticated"

MARKER="[${BENCHMARK}-fresh-dispatch-created attempt=${ATTEMPT}]"
comments="$(gh api --paginate "repos/$REPO/issues/$ISSUE/comments" --jq '.[].body')" || \
  die "unable to read issue #$ISSUE comments"
if grep -Fq "$MARKER" <<<"$comments"; then
  die "attempt marker already exists: $MARKER"
fi

list_runs() {
  gh run list \
    --repo "$REPO" \
    --workflow "$WORKFLOW" \
    --event workflow_dispatch \
    --limit 100 \
    --json databaseId,status \
    --jq '.[] | [.databaseId,.status] | @tsv'
}

before="$(list_runs)" || die "unable to list existing workflow runs"
prior_count=0
while IFS=$'\t' read -r run_id status; do
  [[ -n "$run_id" ]] || continue
  prior_count=$((prior_count + 1))
  if [[ "$status" != "completed" ]]; then
    die "workflow already has non-terminal run $run_id ($status)"
  fi
done <<<"$before"

expected_prior=$((ATTEMPT - 1))
if (( prior_count != expected_prior )); then
  die "attempt $ATTEMPT expects $expected_prior prior runs, found $prior_count"
fi

before_ids="$(printf '%s\n' "$before" | awk -F '\t' 'NF {print $1}' | sort -u)"

gh workflow run "$WORKFLOW" --repo "$REPO" --ref main || die "workflow dispatch failed"

new_ids=()
for ((poll = 0; poll < 20; poll++)); do
  after="$(list_runs)" || die "unable to discover dispatched workflow run"
  after_ids="$(printf '%s\n' "$after" | awk -F '\t' 'NF {print $1}' | sort -u)"
  mapfile -t new_ids < <(
    comm -13 \
      <(printf '%s\n' "$before_ids" | sed '/^$/d') \
      <(printf '%s\n' "$after_ids" | sed '/^$/d')
  )
  if (( ${#new_ids[@]} > 0 )); then
    break
  fi
  sleep "$POLL_SECONDS"
done
if (( ${#new_ids[@]} == 0 )); then
  die "no new workflow run was discoverable after dispatch"
fi
if (( ${#new_ids[@]} != 1 )); then
  die "ambiguous dispatch discovery: found ${#new_ids[@]} new run IDs"
fi

run_id="${new_ids[0]}"
BENCHMARK_UPPER="${BENCHMARK^^}"
body="$(cat <<EOF
Fresh ${BENCHMARK_UPPER} execution attempt #${ATTEMPT} dispatched exactly once.

- workflow run: \`${run_id}\`
- workflow: \`${WORKFLOW}\`
- ref: \`main\`
- attempt: \`${ATTEMPT}\`

${MARKER}
EOF
)"

gh issue comment "$ISSUE" --repo "$REPO" --body "$body" || \
  die "run $run_id was dispatched but issue marker write failed"

printf 'run_id=%s\n' "$run_id"
printf 'marker=%s\n' "$MARKER"
