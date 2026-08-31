#!/usr/bin/env bash
set -euo pipefail

BENCHMARK=""
FROZEN_SHA=""
OUTPUT=""
CHECK_ONLY=false

usage() {
  cat <<'EOF'
Usage: bash scripts/render-frozen-benchmark-workflow.sh \
  --benchmark <id> --frozen-sha <40hex> --output <path> [--check]
EOF
}

die() {
  echo "benchmark-workflow-renderer: $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --benchmark) BENCHMARK="${2:-}"; shift 2 ;;
    --frozen-sha) FROZEN_SHA="${2:-}"; shift 2 ;;
    --output) OUTPUT="${2:-}"; shift 2 ;;
    --check) CHECK_ONLY=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; die "unknown argument: $1" ;;
  esac
done
[[ "$BENCHMARK" =~ ^[a-z][a-z0-9-]{0,31}$ ]] || die "invalid benchmark id: $BENCHMARK"
[[ "$FROZEN_SHA" =~ ^[0-9a-f]{40}$ ]] || die "invalid frozen SHA: $FROZEN_SHA"
[[ -n "$OUTPUT" ]] || die "--output is required"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd -P)"
DATASET="$ROOT/benchmarks/$BENCHMARK"
TEMPLATE="$ROOT/.github/workflows/h4-frozen-execution.yml"

[[ -d "$DATASET" && ! -L "$DATASET" ]] || die "benchmark dataset not found: benchmarks/$BENCHMARK"
[[ -f "$ROOT/cmd/agentd/${BENCHMARK}_benchmark.go" ]] || die "benchmark CLI wiring not found: cmd/agentd/${BENCHMARK}_benchmark.go"
[[ -f "$TEMPLATE" && ! -L "$TEMPLATE" ]] || die "H4 frozen workflow template is unavailable"

for name in manifest.json rubric.json cases.json; do
  path="$DATASET/$name"
  [[ -f "$path" && ! -L "$path" ]] || die "missing real dataset file: benchmarks/$BENCHMARK/$name"
done

manifest_sha="$(sha256sum "$DATASET/manifest.json" | awk '{print $1}')"
rubric_sha="$(sha256sum "$DATASET/rubric.json" | awk '{print $1}')"
cases_sha="$(sha256sum "$DATASET/cases.json" | awk '{print $1}')"
policy_sha=""
if [[ -f "$DATASET/adapter-policy.json" && ! -L "$DATASET/adapter-policy.json" ]]; then
  policy_sha="$(sha256sum "$DATASET/adapter-policy.json" | awk '{print $1}')"
fi
BENCHMARK_UPPER="${BENCHMARK^^}"

H4_FROZEN_SHA="375be888a49e261667362063e8ec03a2c42e152f"
H4_MANIFEST_SHA="1286bbaa9bc630308f2cf81ac0811f11dc084c1d3092810b54ae3301eab0cad0"
H4_RUBRIC_SHA="6439c683279e3e7997bcfa19e42b8a42d1d3544141d6b39"
H4_CASES_SHA="1ec5d7aa3d36efbeffc53d4455143bb2a542326a954d77a77c006f8cbe77cfa8"
tmp="$(mktemp "${TMPDIR:-/tmp}/benchmark-workflow.XXXXXX")"
cleanup() { rm -f "$tmp"; }
trap cleanup EXIT INT TERM

sed \
  -e "s/$H4_FROZEN_SHA/$FROZEN_SHA/g" \
  -e "s/$H4_MANIFEST_SHA/$manifest_sha/g" \
  -e "s/$H4_RUBRIC_SHA/$rubric_sha/g" \
  -e "s/$H4_CASES_SHA/$cases_sha/g" \
  -e "s/H4/$BENCHMARK_UPPER/g" \
  -e "s/h4/$BENCHMARK/g" \
  "$TEMPLATE" > "$tmp"

if [[ -n "$policy_sha" ]]; then
  sed -i "/cases.json | awk/a\\          test \"\$(sha256sum benchmarks/$BENCHMARK/adapter-policy.json | awk '{print \$1}')\" = \"$policy_sha\"" "$tmp"
  sed -i "/sha256sum benchmarks\/$BENCHMARK\/manifest.json/ s#benchmarks/$BENCHMARK/cases.json#benchmarks/$BENCHMARK/cases.json benchmarks/$BENCHMARK/adapter-policy.json#" "$tmp"
fi

if [[ "$BENCHMARK" == "h5" || "$BENCHMARK" == "h6" || "$BENCHMARK" == "h7" || "$BENCHMARK" == "h8" ]]; then
  preflight="$(mktemp "${TMPDIR:-/tmp}/${BENCHMARK}-preflight.XXXXXX")"
  rewritten="$(mktemp "${TMPDIR:-/tmp}/${BENCHMARK}-workflow.XXXXXX")"
  cat > "$preflight" <<'EOF'
      - name: Verify subscription adapter availability
        shell: bash
        run: |
          set -euo pipefail
          for key in OPENAI_API_KEY CODEX_API_KEY ANTHROPIC_API_KEY ANTHROPIC_AUTH_TOKEN CLAUDE_CODE_OAUTH_TOKEN GEMINI_API_KEY GOOGLE_API_KEY; do
            if [[ -n "${!key:-}" ]]; then
              echo "metered API credentials are forbidden for __BENCHMARK_UPPER__; unset $key" >&2
              exit 1
            fi
          done
          automated_available=0
          if command -v claude >/dev/null 2>&1; then
            claude_version="$(claude --version 2>&1 || true)"
            claude_status="$(claude auth status 2>&1 || true)"
            if grep -Eq '"loggedIn"[[:space:]]*:[[:space:]]*true' <<<"$claude_status" &&
               grep -Eq '"authMethod"[[:space:]]*:[[:space:]]*"claude\.ai"' <<<"$claude_status" &&
               grep -Eq '"apiProvider"[[:space:]]*:[[:space:]]*"firstParty"' <<<"$claude_status"; then
              printf 'Claude: %s\n' "$claude_version" | tee -a .__BENCHMARK__-audit/preflight.txt
              automated_available=1
            else
              echo "Claude: unavailable; frozen chain may fail over" | tee -a .__BENCHMARK__-audit/preflight.txt
            fi
          else
            echo "Claude: unavailable; frozen chain may fail over" | tee -a .__BENCHMARK__-audit/preflight.txt
          fi
          if command -v codex >/dev/null 2>&1; then
            codex_version="$(codex --version 2>&1 || true)"
            codex_status="$(codex login status 2>&1 || true)"
            if grep -Fq 'Logged in using ChatGPT' <<<"$codex_status"; then
              printf 'Codex: %s\n' "$codex_version" | tee -a .__BENCHMARK__-audit/preflight.txt
              automated_available=1
            else
              echo "Codex: unavailable; frozen chain may fail over" | tee -a .__BENCHMARK__-audit/preflight.txt
            fi
          else
            echo "Codex: unavailable; frozen chain may fail over" | tee -a .__BENCHMARK__-audit/preflight.txt
          fi
          if command -v agy >/dev/null 2>&1; then
            printf 'Antigravity: %s\n' "$(agy --version 2>&1 || true)" | tee -a .__BENCHMARK__-audit/preflight.txt
          else
            echo "Antigravity: unavailable; frozen chain may fail over" | tee -a .__BENCHMARK__-audit/preflight.txt
          fi
          echo "human-chatgpt-session: frozen final availability fallback" | tee -a .__BENCHMARK__-audit/preflight.txt
          printf 'automated_adapter_available=%s\n' "$automated_available" | tee -a .__BENCHMARK__-audit/preflight.txt
          go version | tee -a .__BENCHMARK__-audit/preflight.txt
EOF
  sed -i -e "s/__BENCHMARK_UPPER__/$BENCHMARK_UPPER/g" -e "s/__BENCHMARK__/$BENCHMARK/g" "$preflight"
  awk -v replacement="$preflight" '
    BEGIN {
      while ((getline line < replacement) > 0) {
        block = block line ORS
      }
      close(replacement)
    }
    /^      - name: Verify subscription CLIs$/ {
      printf "%s", block
      skip = 1
      next
    }
    skip && /^      - name: Verify repository tests on frozen commit$/ {
      skip = 0
    }
    skip { next }
    { print }
  ' "$tmp" > "$rewritten"
  mv "$rewritten" "$tmp"
  rm -f "$preflight"
fi

if [[ "$CHECK_ONLY" == "true" ]]; then
  [[ -f "$OUTPUT" && ! -L "$OUTPUT" ]] || die "workflow to check is not a real file: $OUTPUT"
  cmp -s "$tmp" "$OUTPUT" || die "workflow drift detected: $OUTPUT"
  echo "workflow check OK: $OUTPUT"
  exit 0
fi

[[ ! -e "$OUTPUT" && ! -L "$OUTPUT" ]] || die "output already exists: $OUTPUT"
parent="$(dirname -- "$OUTPUT")"
[[ -d "$parent" ]] || die "output parent does not exist: $parent"
umask 077
set -o noclobber
cat "$tmp" > "$OUTPUT"
set +o noclobber
chmod 0644 "$OUTPUT"
printf 'rendered %s frozen workflow: %s\n' "$BENCHMARK_UPPER" "$OUTPUT"
