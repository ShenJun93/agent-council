#!/usr/bin/env bash
set -euo pipefail

url="https://www.gnu.org/licenses/agpl-3.0.txt"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl --fail --location --silent --show-error "$url" --output "$tmp"
elif command -v wget >/dev/null 2>&1; then
  wget --quiet "$url" --output-document="$tmp"
else
  echo "error: curl or wget is required" >&2
  exit 1
fi

if ! grep -q "GNU AFFERO GENERAL PUBLIC LICENSE" "$tmp"; then
  echo "error: downloaded file does not look like AGPL-3.0" >&2
  exit 1
fi

cp "$tmp" LICENSE
echo "Installed canonical AGPL-3.0 license text into LICENSE"
