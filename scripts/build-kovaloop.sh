#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-$ROOT_DIR/dist/kovaloop}"
TARGET_GOOS="${GOOS:-$(go env GOOS)}"
TARGET_GOARCH="${GOARCH:-$(go env GOARCH)}"

mkdir -p "$(dirname "$OUT")"
CGO_ENABLED="${CGO_ENABLED:-0}" GOOS="$TARGET_GOOS" GOARCH="$TARGET_GOARCH" \
  go build -buildvcs=false -trimpath -ldflags="-s -w" -o "$OUT" "$ROOT_DIR/cmd/kovaloop"
