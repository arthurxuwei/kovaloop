#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
EXPECTED_ASSETS=(
  kovaloop_darwin_amd64
  kovaloop_darwin_arm64
  kovaloop_linux_amd64
  kovaloop_linux_arm64
)

mkdir -p "$DIST_DIR"

for asset in "${EXPECTED_ASSETS[@]}"; do
  rm -f "$DIST_DIR/$asset"
done

for asset in "${EXPECTED_ASSETS[@]}"; do
  target="${asset#kovaloop_}"
  os="${target%_*}"
  arch="${target#*_}"
  out="$DIST_DIR/$asset"
  echo "building $out"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" "$ROOT_DIR/scripts/build-kovaloop.sh" "$out"
done

for asset in "${EXPECTED_ASSETS[@]}"; do
  if [[ ! -x "$DIST_DIR/$asset" ]]; then
    echo "missing release asset: $DIST_DIR/$asset" >&2
    exit 1
  fi
done

unexpected=()
for path in "$DIST_DIR"/kovaloop*; do
  [[ -e "$path" ]] || continue
  [[ -f "$path" ]] || continue
  name="$(basename -- "$path")"
  case " ${EXPECTED_ASSETS[*]} " in
    *" $name "*) ;;
    *) unexpected+=("$path") ;;
  esac
done

if (( ${#unexpected[@]} > 0 )); then
  printf 'unexpected kovaloop release asset: %s\n' "${unexpected[@]}" >&2
  exit 1
fi
