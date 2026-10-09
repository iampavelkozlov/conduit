#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UPSTREAM_REPOSITORY="${REALWORLD_REPOSITORY:-https://github.com/realworld-apps/realworld.git}"
UPSTREAM_REF="${REALWORLD_REF:-main}"
TMP_PARENT="${TMPDIR:-/tmp}"
TMP_DIR="$(mktemp -d "${TMP_PARENT%/}/conduit-contract.XXXXXX")"

cleanup() {
  if [[ -n "${TMP_DIR:-}" && -d "$TMP_DIR" && "$(basename "$TMP_DIR")" == conduit-contract.* ]]; then
    rm -rf -- "$TMP_DIR"
  fi
}
trap cleanup EXIT

git clone --quiet --depth 1 --filter=blob:none --sparse \
  --branch "$UPSTREAM_REF" "$UPSTREAM_REPOSITORY" "$TMP_DIR/realworld"
git -C "$TMP_DIR/realworld" sparse-checkout set specs/api

UPSTREAM_API="$TMP_DIR/realworld/specs/api/openapi.yml"
UPSTREAM_HURL="$TMP_DIR/realworld/specs/api/hurl"

if ! cmp -s "$ROOT_DIR/api/open-api.yml" "$UPSTREAM_API"; then
  echo "api/open-api.yml differs from ${UPSTREAM_REPOSITORY}@${UPSTREAM_REF}" >&2
  diff -u "$UPSTREAM_API" "$ROOT_DIR/api/open-api.yml" || true
  exit 1
fi

if ! diff -qr "$UPSTREAM_HURL" "$ROOT_DIR/apitests"; then
  echo "apitests/ differs from ${UPSTREAM_REPOSITORY}@${UPSTREAM_REF}" >&2
  exit 1
fi

echo "OpenAPI and Hurl files match ${UPSTREAM_REPOSITORY}@${UPSTREAM_REF}"
