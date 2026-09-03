#!/usr/bin/env bash
set -euo pipefail

REPO="ShenJun93/agent-council"
BENCHMARK=""
PREFLIGHT_ONLY=false

usage() {
  cat <<'EOF'
Usage: bash scripts/bootstrap-benchmark-runner.sh --benchmark <id> [--preflight-only]

Registers one ephemeral Linux GitHub Actions runner for a frozen benchmark workflow.
Run this as a non-root user with GitHub runner-registration access. Benchmarks that
use subscription CLIs must run as the user that owns those sessions; H9 uses only
the human ChatGPT web broker.
EOF
}

die() {
  echo "benchmark-runner-bootstrap: $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --benchmark) BENCHMARK="${2:-}"; shift 2 ;;
    --preflight-only) PREFLIGHT_ONLY=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; die "unknown argument: $1" ;;
  esac
done
[[ "$BENCHMARK" =~ ^[a-z][a-z0-9-]{0,31}$ ]] || die "invalid benchmark id: $BENCHMARK"
RUNNER_LABEL="${BENCHMARK}-benchmark"
BENCHMARK_UPPER="${BENCHMARK^^}"

metered_keys=(OPENAI_API_KEY CODEX_API_KEY ANTHROPIC_API_KEY)
if [[ "$BENCHMARK" == "h5" || "$BENCHMARK" == "h6" || "$BENCHMARK" == "h7" || "$BENCHMARK" == "h8" ]]; then
  metered_keys+=(ANTHROPIC_AUTH_TOKEN CLAUDE_CODE_OAUTH_TOKEN GEMINI_API_KEY GOOGLE_API_KEY)
elif [[ "$BENCHMARK" == "h9" ]]; then
  metered_keys+=(ANTHROPIC_AUTH_TOKEN CLAUDE_CODE_OAUTH_TOKEN GEMINI_API_KEY GOOGLE_API_KEY AZURE_OPENAI_API_KEY)
fi
for key in "${metered_keys[@]}"; do
  if [[ -n "${!key:-}" ]]; then
    die "metered API credentials are forbidden for ${BENCHMARK_UPPER}; unset $key"
  fi
done

command -v gh >/dev/null 2>&1 || die "required command not found: gh"
gh auth status >/dev/null 2>&1 || die "GitHub CLI is not authenticated; run: gh auth login"
registration_token="$(gh api -X POST "repos/$REPO/actions/runners/registration-token" --jq '.token' 2>/dev/null)" || \
  die "GitHub auth lacks runner registration permission; authenticate an admin token with repository access"
[[ -n "$registration_token" ]] || die "GitHub auth lacks runner registration permission; registration token was empty"

claude_version="unavailable"
codex_version="unavailable"
if [[ "$BENCHMARK" == "h9" ]]; then
  claude_version="not-used"
  codex_version="not-used"
  printf '%s human broker: current ChatGPT web session only\n' "$BENCHMARK_UPPER"
elif [[ "$BENCHMARK" == "h5" || "$BENCHMARK" == "h6" || "$BENCHMARK" == "h7" || "$BENCHMARK" == "h8" ]]; then
  if command -v claude >/dev/null 2>&1; then
    candidate_version="$(claude --version 2>&1 || true)"
    candidate_status="$(claude auth status 2>&1 || true)"
    if grep -Eq '"loggedIn"[[:space:]]*:[[:space:]]*true' <<<"$candidate_status" &&
       grep -Eq '"authMethod"[[:space:]]*:[[:space:]]*"claude\.ai"' <<<"$candidate_status" &&
       grep -Eq '"apiProvider"[[:space:]]*:[[:space:]]*"firstParty"' <<<"$candidate_status" &&
       grep -Eq '"subscriptionType"[[:space:]]*:[[:space:]]*"[^\"]+"' <<<"$candidate_status"; then
      claude_version="$candidate_version"
    fi
  fi
  if command -v codex >/dev/null 2>&1; then
    candidate_version="$(codex --version 2>&1 || true)"
    candidate_status="$(codex login status 2>&1 || true)"
    if grep -Fq 'Logged in using ChatGPT' <<<"$candidate_status"; then
      codex_version="$candidate_version"
    fi
  fi
  printf '%s human broker: available as final frozen fallback\n' "$BENCHMARK_UPPER"
else
  for cmd in claude codex; do
    command -v "$cmd" >/dev/null 2>&1 || die "required command not found: $cmd"
  done
  claude_version="$(claude --version 2>&1)" || die "claude --version failed"
  claude_status="$(claude auth status 2>&1)" || die "Claude Code is not authenticated"
  grep -Eq '"loggedIn"[[:space:]]*:[[:space:]]*true' <<<"$claude_status" || die "Claude Code auth status is not logged in"
  grep -Eq '"authMethod"[[:space:]]*:[[:space:]]*"claude\.ai"' <<<"$claude_status" || die "Claude Code must use claude.ai subscription authentication"
  grep -Eq '"apiProvider"[[:space:]]*:[[:space:]]*"firstParty"' <<<"$claude_status" || die "Claude Code must use the first-party provider"
  grep -Eq '"subscriptionType"[[:space:]]*:[[:space:]]*"[^\"]+"' <<<"$claude_status" || die "Claude Code did not report an active subscription type"
  codex_version="$(codex --version 2>&1)" || die "codex --version failed"
  codex_status="$(codex login status 2>&1)" || die "Codex is not authenticated"
  grep -Fq 'Logged in using ChatGPT' <<<"$codex_status" || die "Codex must use ChatGPT subscription authentication"
fi

os_name="$(uname -s)"
arch_name="$(uname -m)"
[[ "$os_name" == "Linux" ]] || die "the queued ${BENCHMARK_UPPER} workflow requires a Linux self-hosted runner; got $os_name"

case "$arch_name" in
  x86_64|amd64) runner_arch="x64" ;;
  aarch64|arm64) runner_arch="arm64" ;;
  *) die "unsupported Linux architecture: $arch_name" ;;
esac

printf 'GitHub auth: OK\n'
printf 'GitHub runner registration: OK\n'
printf 'Claude: %s\n' "$claude_version"
printf 'Codex: %s\n' "$codex_version"
printf 'Host: %s/%s\n' "$os_name" "$runner_arch"
printf '%s runner preflight OK\n' "$BENCHMARK_UPPER"

if [[ "$PREFLIGHT_ONLY" == "true" ]]; then
  unset registration_token
  exit 0
fi

if [[ "$(id -u)" == "0" ]]; then
  die "do not run as root; run as the non-root benchmark runner user"
fi

for cmd in curl tar awk sed mktemp; do
  command -v "$cmd" >/dev/null 2>&1 || die "required command not found: $cmd"
done

if command -v sha256sum >/dev/null 2>&1; then
  hash_file() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  hash_file() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  die "sha256sum or shasum is required"
fi

runner_dir="$(mktemp -d "${TMPDIR:-/tmp}/agent-council-${BENCHMARK}-runner.XXXXXX")"
cleanup() {
  rm -rf "$runner_dir"
}
trap cleanup EXIT INT TERM

release_tag="$(gh api repos/actions/runner/releases/latest --jq '.tag_name')"
[[ "$release_tag" == v* ]] || die "unexpected actions/runner release tag: $release_tag"
runner_version="${release_tag#v}"
asset="actions-runner-linux-${runner_arch}-${runner_version}.tar.gz"

asset_meta="$(gh api repos/actions/runner/releases/latest --jq ".assets[] | select(.name == \"$asset\") | [.browser_download_url, .digest] | @tsv")"
[[ -n "$asset_meta" ]] || die "runner release asset not found: $asset"
IFS=$'\t' read -r download_url digest <<<"$asset_meta"
[[ -n "$download_url" ]] || die "runner release asset has no download URL"
[[ "$digest" == sha256:* ]] || die "runner release asset has no SHA-256 digest"
expected_hash="${digest#sha256:}"

archive="$runner_dir/$asset"
echo "Downloading official GitHub Actions runner $release_tag..."
curl --fail --location --silent --show-error "$download_url" --output "$archive"
actual_hash="$(hash_file "$archive")"
[[ "$actual_hash" == "$expected_hash" ]] || die "runner archive SHA-256 mismatch"

tar -xzf "$archive" -C "$runner_dir"
rm -f "$archive"

host_name="$(uname -n | sed 's/[^A-Za-z0-9._-]/-/g')"
runner_name="${BENCHMARK}-${host_name}-$$"

cd "$runner_dir"
./config.sh \
  --url "https://github.com/$REPO" \
  --token "$registration_token" \
  --name "$runner_name" \
  --labels "$RUNNER_LABEL" \
  --work _work \
  --unattended \
  --ephemeral \
  --replace

unset registration_token

echo "Ephemeral ${BENCHMARK_UPPER} runner registered as $runner_name."
echo "Waiting for one ${BENCHMARK_UPPER} Frozen Execution job; this process exits after that one job."
./run.sh
