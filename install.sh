#!/usr/bin/env bash
set -euo pipefail

KOVALOOP_INSTALL_BASE_URL="${KOVALOOP_INSTALL_BASE_URL:-https://raw.githubusercontent.com/arthurxuwei/kovaloop/main}"
KOVALOOP_INSTALL_BIN_BASE_URL="${KOVALOOP_INSTALL_BIN_BASE_URL:-https://github.com/arthurxuwei/kovaloop/releases/latest/download}"
ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

discover_workspaces() {
  if [[ -n "${OPENCLAW_WORKSPACE_DIR:-}" ]]; then
    printf '%s\n' "$OPENCLAW_WORKSPACE_DIR"
    return
  fi

  local search_dir="$PWD"
  if [[ "$search_dir" == /private/var/* && -d "${search_dir#/private}" ]]; then
    search_dir="${search_dir#/private}"
  fi

  local found=0
  for workspace in "$search_dir"/runtime-openclaw-*/workspace; do
    if [[ -d "$workspace" ]]; then
      found=1
      printf '%s\n' "$workspace"
    fi
  done

  if [[ "$found" -eq 0 ]]; then
    echo "No OpenClaw workspace found. Set OPENCLAW_WORKSPACE_DIR=/path/to/workspace." >&2
    return 2
  fi
}

download_file() {
  local path="$1"
  local dest="$2"
  curl -fsSL "$KOVALOOP_INSTALL_BASE_URL/$path" -o "$dest"
}

download_kovaloop_binary() {
  local asset="$1"
  local dest="$2"
  curl -fsSL "$KOVALOOP_INSTALL_BIN_BASE_URL/$asset" -o "$dest"
}

install_file() {
  local path="$1"
  local dest="$2"

  if [ -f "$ROOT_DIR/$path" ]; then
    cp "$ROOT_DIR/$path" "$dest"
  else
    download_file "$path" "$dest"
  fi
}

kovaloop_asset_name() {
  local os
  local arch

  case "$(uname -s)" in
    Darwin)
      os="darwin"
      ;;
    Linux)
      os="linux"
      ;;
    *)
      echo "Unsupported platform: $(uname -s)/$(uname -m)" >&2
      return 2
      ;;
  esac

  case "$(uname -m)" in
    x86_64 | amd64)
      arch="amd64"
      ;;
    arm64 | aarch64)
      arch="arm64"
      ;;
    *)
      echo "Unsupported platform: $(uname -s)/$(uname -m)" >&2
      return 2
      ;;
  esac

  printf 'kovaloop_%s_%s\n' "$os" "$arch"
}

install_kovaloop_binary() {
  local dest="$1"
  local asset

  asset="$(kovaloop_asset_name)" || return $?

  if [[ -n "${KOVALOOP_INSTALL_BIN_DIR:-}" && -f "$KOVALOOP_INSTALL_BIN_DIR/$asset" ]]; then
    cp "$KOVALOOP_INSTALL_BIN_DIR/$asset" "$dest"
  elif [[ -f "$ROOT_DIR/dist/$asset" ]]; then
    cp "$ROOT_DIR/dist/$asset" "$dest"
  else
    download_kovaloop_binary "$asset" "$dest"
  fi
  chmod +x "$dest"
}

install_skill_to() {
  local skills_dest="$1"
  local skill_name="$2"
  local dest_dir="$skills_dest/$skill_name"

  rm -rf "$dest_dir"
  mkdir -p "$dest_dir"

  if [ -d "$ROOT_DIR/skills/$skill_name" ]; then
    cp -R "$ROOT_DIR/skills/$skill_name/." "$dest_dir/"
  else
    install_file "skills/$skill_name/SKILL.md" "$dest_dir/SKILL.md"
  fi
}

shell_quote() {
  printf '%q' "$1"
}

install_workspace() {
  local workspace="$1"
  local skills_dest="$workspace/skills"
  local bin_dest="$workspace/.local/bin"
  local quoted_workspace
  local quoted_kovaloop

  quoted_workspace="$(shell_quote "$workspace")"
  quoted_kovaloop="$(shell_quote "$bin_dest/kovaloop")"

  mkdir -p "$skills_dest" "$bin_dest"
  find "$skills_dest" -maxdepth 1 -type d -name 'chief-*' -exec rm -rf {} +
  find "$skills_dest" -maxdepth 1 -type d -name 'kovaloop-*' -exec rm -rf {} +
  rm -f "$bin_dest/chief"

  install_skill_to "$skills_dest" kovaloop-ledger

  install_kovaloop_binary "$bin_dest/kovaloop"

  cat <<EOF
Kovaloop installed successfully.

OpenClaw workspace: $workspace
CLI:                $bin_dest/kovaloop
Skills:             $skills_dest
EOF

  if OPENCLAW_WORKSPACE_DIR="$workspace" "$bin_dest/kovaloop" claim link; then
    return 0
  fi

  cat <<EOF
Claim link unavailable for $workspace.
Retry:
OPENCLAW_WORKSPACE_DIR=$quoted_workspace $quoted_kovaloop claim link
EOF
}

workspaces="$(discover_workspaces)" || exit $?
while IFS= read -r workspace; do
  install_workspace "$workspace"
done <<< "$workspaces"
