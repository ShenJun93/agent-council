#!/usr/bin/env bash
set -euo pipefail

REPO="ShenJun93/agent-council"
RUNNER_LABEL="h1-benchmark"
PREFLIGHT_ONLY=false

usage() {
  cat <<'EOF'
Usage: bash scripts/bootstrap-h1-runner.sh [--preflight-only]

Registers one ephemeral Linux GitHub Actions runner for the frozen H1 workflow.
Run this as the same non-root user whose Claude Code and Codex CLIs are already
signed in with subscription-backed accounts.
EOF
}

die() {
  echo "h1-runner-bootstrap: $*" >&2
  exit 1
}

case "${1:-}" in
  "") ;;
  --preflight-only) PREFLIGHT_ONLY=true ;;
  -h|--help) usage; exit 0 ;;
  *) usage >&2; die "unknown argument: $1" ;;
esac

for key in OPENAI_API_KEY CODEX_API_KEY ANTHROPIC_API_KEY; do
  if [[ -n "${!key:-}" ]]; then
    die "metered API credentials are forbidden for H1; unset $key"
  fi
done

for cmd in gh claude codex; do
  command -v "$cmd" >/dev/null 2>&1 || die "required command not found: $cmd"
done

gh auth status >/dev/null 2>&1 || die "GitHub CLI is not authenticated; run: gh auth login"

claude_version="$(claude --version 2>&1)" || die "claude --version failed"
claude_status="$(claude auth status 2>&1)" || die "Claude Code is not authenticated"

grep -Eq '"loggedIn"[[:space:]]*:[[:space:]]*true' <<<"$claude_status" || die "Claude Code auth status is not logged in"
grep -Eq '"authMethod"[[:space:]]*:[[:space:]]*"claude\.ai"' <<<"$claude_status" || die "Claude Code must use claude.ai subscription authentication"
grep -Eq '"apiProvider"[[:space:]]*:[[:space:]]*"firstParty"' <<<"$claude_status" || die "Claude Code must use the first-party provider"
grep -Eq '"subscriptionType"[[:space:]]*:[[:space:]]*"[^\"]+"' <<<"$claude_status" || die "Claude Code did not report an active subscription type"

codex_version="$(codex --version 2>&1)" || die "codex --version failed"
codex_status="$(codex login status 2>&1)" || die "Codex is not authenticated"
grep -Fq 'Logged in using ChatGPT' <<<"$codex_status" || die "Codex must use ChatGPT subscription authentication"

os_name="$(uname -s)"
arch_name="$(uname -m)"
[[ "$os_name" == "Linux" ]] || die "the queued H1 workflow requires a Linux self-hosted runner; got $os_name"

case "$arch_name" in
  x86_64|amd64) runner_arch="x64" ;;
  aarch64|arm64) runner_arch="arm64" ;;
  *) die "unsupported Linux architecture: $arch_name" ;;
esac

printf 'GitHub auth: OK\n'
printf 'Claude: %s\n' "$claude_version"
printf 'Codex: %s\n' "$codex_version"
printf 'Host: %s/%s\n' "$os_name" "$runner_arch"
printf 'H1 runner preflight OK\n'

if [[ "$PREFLIGHT_ONLY" == "true" ]]; then
  exit 0
fi

if [[ "$(id -u)" == "0" ]]; then
  die "do not run as root; run as the user that owns the Claude/Codex subscription credentials"
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

runner_dir="$(mktemp -d "${TMPDIR:-/tmp}/agent-council-h1-runner.XXXXXX")"
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

registration_token="$(gh api -X POST "repos/$REPO/actions/runners/registration-token" --jq '.token')"
[[ -n "$registration_token" ]] || die "failed to obtain GitHub runner registration token"

host_name="$(uname -n | sed 's/[^A-Za-z0-9._-]/-/g')"
runner_name="h1-${host_name}-$$"

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

echo "Ephemeral H1 runner registered as $runner_name."
echo "Waiting for the already-queued H1 Frozen Execution job; this process exits after that one job."
./run.sh
