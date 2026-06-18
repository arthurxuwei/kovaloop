#!/usr/bin/env bash
set -euo pipefail

KOVALOOP_INSTALL_BASE_URL="${KOVALOOP_INSTALL_BASE_URL:-https://raw.githubusercontent.com/arthurxuwei/kovaloop/main}"
KOVALOOP_INSTALL_BIN_BASE_URL="${KOVALOOP_INSTALL_BIN_BASE_URL:-https://github.com/arthurxuwei/kovaloop/releases/latest/download}"
ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

discover_runtime_root() {
  local search_dir="$PWD"
  if [[ "$search_dir" == /private/var/* && -d "${search_dir#/private}" ]]; then
    search_dir="${search_dir#/private}"
  fi
  printf '%s\n' "$search_dir"
}

install_discovered_runtimes() {
  if [[ -n "${OPENCLAW_WORKSPACE_DIR:-}" ]]; then
    install_openclaw_workspace "$OPENCLAW_WORKSPACE_DIR"
    return
  fi

  if [[ -n "${HERMES_CONFIG_DIR:-}" ]]; then
    install_hermes_config "$HERMES_CONFIG_DIR"
    return
  fi

  local search_dir
  search_dir="$(discover_runtime_root)"

  local found=0
  for workspace in "$search_dir"/runtime-openclaw-*/workspace; do
    if [[ -d "$workspace" ]]; then
      found=1
      install_openclaw_workspace "$workspace"
    fi
  done
  for config in "$search_dir"/runtime-hermes-*/config; do
    if [[ -d "$config" ]]; then
      found=1
      install_hermes_config "$config"
    fi
  done

  if [[ "$found" -eq 0 ]]; then
    echo "No OpenClaw workspace or Hermes config found. Set OPENCLAW_WORKSPACE_DIR=/path/to/workspace or HERMES_CONFIG_DIR=/path/to/runtime-hermes-x/config." >&2
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
  shift 2
  local extra_files=("$@")
  local dest_dir="$skills_dest/$skill_name"

  rm -rf "$dest_dir"
  mkdir -p "$dest_dir"

  if [ -d "$ROOT_DIR/skills/$skill_name" ]; then
    # Local checkout: copy the whole skill tree (SKILL.md + references/, etc.).
    cp -R "$ROOT_DIR/skills/$skill_name/." "$dest_dir/"
  else
    # Remote install: there is no directory listing over HTTP, so fetch SKILL.md
    # plus each explicitly listed file (e.g. references/*.md).
    install_file "skills/$skill_name/SKILL.md" "$dest_dir/SKILL.md"
    local rel
    for rel in "${extra_files[@]}"; do
      mkdir -p "$dest_dir/$(dirname "$rel")"
      install_file "skills/$skill_name/$rel" "$dest_dir/$rel"
    done
  fi
}

shell_quote() {
  printf '%q' "$1"
}

install_runtime() {
  local label="$1"
  local root="$2"
  local skills_dest="$3"
  local bin_dest="$4"
  local kovaloop_home="$5"
  local quoted_kovaloop

  # We never set EIGENFLUX_* variables (those belong to the EigenFlux runtime
  # and are read-only for us). The installer points the CLI at the profile via
  # our own KOVALOOP_AGENT_PROFILE_PATH and writes .kovaloop under KOVALOOP_HOME.
  local agent_profile_path="$root/.eigenflux/servers/eigenflux/profile.json"
  quoted_kovaloop="$(shell_quote "$bin_dest/kovaloop")"

  mkdir -p "$skills_dest" "$bin_dest"
  find "$skills_dest" -maxdepth 1 -type d -name 'chief-*' -exec rm -rf {} +
  find "$skills_dest" -maxdepth 1 -type d -name 'kovaloop-*' -exec rm -rf {} +
  rm -f "$bin_dest/chief"

  install_skill_to "$skills_dest" kovaloop-ledger \
    references/balance-state.md \
    references/troubleshooting.md \
    references/payment-routing.md \
    references/direct-transfer.md \
    references/onboarding.md

  install_kovaloop_binary "$bin_dest/kovaloop"

  cat <<EOF
Kovaloop installed successfully.

${label}: $root
CLI:                $bin_dest/kovaloop
Skills:             $skills_dest
EOF

  # Mint the KovaLoop identity if absent (idempotent: reused when credentials exist).
  env KOVALOOP_AGENT_PROFILE_PATH="$agent_profile_path" KOVALOOP_HOME="$kovaloop_home" "$bin_dest/kovaloop" profile create || true

  if env KOVALOOP_AGENT_PROFILE_PATH="$agent_profile_path" KOVALOOP_HOME="$kovaloop_home" "$bin_dest/kovaloop" claim link; then
    return 0
  fi

  cat <<EOF
Claim link unavailable for $root.
Retry:
KOVALOOP_AGENT_PROFILE_PATH=$(shell_quote "$agent_profile_path") KOVALOOP_HOME=$(shell_quote "$kovaloop_home") $quoted_kovaloop claim link
EOF
}

install_openclaw_workspace() {
  local workspace="$1"
  # Binary goes to $HOME/.local/bin (on PATH, alongside the eigenflux CLI);
  # the skill stays per-workspace; .kovaloop lives at the config volume root.
  install_runtime "OpenClaw workspace" "$workspace" "$workspace/skills" "$HOME/.local/bin" "$(dirname "$workspace")"
}

install_hermes_config() {
  local config="$1"
  install_runtime "Hermes config" "$config" "$config/skills" "$HOME/.local/bin" "$config"
}

install_discovered_runtimes
