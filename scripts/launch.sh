#!/usr/bin/env bash
# scripts/launch.sh — Seam AMS + xolu launcher
#
# Obtains xolu (a release binary, or built from source), builds Seam, and
# starts both. Both run in the foreground; Ctrl-C stops both cleanly.
#
# The first time it runs on a data directory it asks how the instance should
# start: empty, with an administrator you create, or with one of the
# demonstration datasets and what goes with it (work orders, issues,
# accounting). After that the data directory is left as it is.
#
# Normally invoked through make: make launch, make launch-fast,
# make launch-dev, make dev, make xolu, make start-xolu.
#
# Usage:
#   ./scripts/launch.sh [options]
#
# Options:
#   --xolu-port PORT    Port for xolu         (default: 9090)
#   --seam-port PORT    Port for Seam AMS     (default: 8080)
#   --data-dir DIR      Data directory        (default: ./data)
#   --lang CODE         UI language: en, es, pt or ja
#   --seed NAME         Load a demonstration dataset and its additions:
#                       01-constructec ... 07-beiramagalhaes, or "none" to start
#                       empty without being asked
#   --additions WHAT    With a dataset: "all" (default), "none", or a list
#                       such as "wo-templates,accounting"
#   --modules WHAT      The modules that are off by default (work_orders,
#                       issues, accounting) to switch on: "auto" (default:
#                       those the loaded data is for), "all", "none", or a
#                       list such as "issues,work_orders". The choice is kept
#                       with the data directory and used on later launches.
#   --only MODULE       That module as the whole system, without the Assets
#                       module: accounting. It loads the module's own data
#                       and wants a data directory of its own, which stays
#                       that way on later launches.
#   --dev               Dev mode: Seam's development fallbacks (never in production)
#   --skip-build        Do not rebuild binaries that exist
#   --no-admin          Do not ask to create an administrator
#   --yes               Ask nothing: take every default
#   --verbose           Show what xolu and Seam print on this terminal, instead
#                       of writing it to xolu.log and seam.log in the data directory
#   --xolu-only         Obtain and start xolu alone, without Seam
#   --no-start          Obtain and build, start nothing
#   --help              Show this help
#
# Environment overrides:
#   XOLU_PORT, SEAM_PORT, SEAM_DATA_DIR, SEAM_DEV_MODE, SEAM_FORCE_LANGUAGE,
#   SEAM_SEED, SEAM_SEED_ADDITIONS, SEAM_MODULES, SEAM_ONLY, XOLU_VERSION, XOLU_SOURCE
#   Seam's own variables pass through to it, and win over --modules:
#   SEAM_ENABLE_WORK_ORDERS, SEAM_ENABLE_ISSUES, SEAM_ENABLE_ACCOUNTING (1 or 0),
#   SEAM_ENABLE_DEVELOPER=1, SEAM_FEATURES=...
#   Output: NO_COLOR, SEAM_COLOR=always|never, SEAM_ASCII=1 (scripts/lib/ui.sh)
#
# XOLU_SOURCE:
#   Path to a local xolu source checkout. When it points to a directory
#   containing go.mod, xolu is built from that directory without Git operations.
#
# XOLU_VERSION:
#   latest              Latest GitHub release/source (default)
#   previous            Previous commit on the default branch
#   <tag|branch|SHA>    Specific version
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# shellcheck source=scripts/lib/ui.sh
source "$SCRIPT_DIR/lib/ui.sh"
# shellcheck source=scripts/lib/xolu-env.sh
source "$SCRIPT_DIR/lib/xolu-env.sh"
# shellcheck source=scripts/lib/seeds.sh
source "$SCRIPT_DIR/lib/seeds.sh"
# shellcheck source=scripts/lib/modules.sh
source "$SCRIPT_DIR/lib/modules.sh"

TAB=$'\t'

# ── Defaults ──────────────────────────────────────────────────────────────────
XOLU_PORT="${XOLU_PORT:-9090}"
SEAM_PORT="${SEAM_PORT:-8080}"
DATA_DIR="${SEAM_DATA_DIR:-$PROJECT_DIR/data}"
DEV_MODE="${SEAM_DEV_MODE:-false}"
LANG_CODE="${SEAM_FORCE_LANGUAGE:-}"
SEED_NAME="${SEAM_SEED:-}"
SEED_ADDITIONS="${SEAM_SEED_ADDITIONS:-all}"
MODULES="${SEAM_MODULES:-auto}"
ONLY="${SEAM_ONLY:-}"
XOLU_VERSION="${XOLU_VERSION:-latest}"
XOLU_SOURCE="${XOLU_SOURCE:-}"
SKIP_BUILD=false
NO_ADMIN=false
ASSUME_YES=false
VERBOSE=false
XOLU_ONLY=false
NO_START=false
XOLU_REPO="https://github.com/ha1tch/xolu.git"
XOLU_REPO_API="https://api.github.com/repos/ha1tch/xolu"
XOLU_DIR="$PROJECT_DIR/xolu"
XOLU_BIN="$XOLU_DIR/xolu"
LANGUAGES="en es pt ja"

# ── Version ───────────────────────────────────────────────────────────────────
SEAM_VERSION="$(cat "$PROJECT_DIR/VERSION" 2>/dev/null || echo "unknown")"

# ── Argument parsing ──────────────────────────────────────────────────────────
need_value() { [[ $# -ge 2 ]] || ui_die "$1 requires a value"; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --xolu-port)  need_value "$@"; XOLU_PORT="$2"; shift 2 ;;
        --seam-port)  need_value "$@"; SEAM_PORT="$2"; shift 2 ;;
        --data-dir)   need_value "$@"; DATA_DIR="$2"; shift 2 ;;
        --lang)       need_value "$@"; LANG_CODE="$2"; shift 2 ;;
        --seed)       need_value "$@"; SEED_NAME="$2"; shift 2 ;;
        --additions)  need_value "$@"; SEED_ADDITIONS="$2"; shift 2 ;;
        --modules)    need_value "$@"; MODULES="$2"; shift 2 ;;
        --only)       need_value "$@"; ONLY="$2"; shift 2 ;;
        --dev)        DEV_MODE=true;   shift ;;
        --skip-build) SKIP_BUILD=true; shift ;;
        --no-admin)   NO_ADMIN=true;   shift ;;
        --yes)        ASSUME_YES=true; shift ;;
        --verbose)    VERBOSE=true;    shift ;;
        --xolu-only)  XOLU_ONLY=true;  shift ;;
        --no-start)   NO_START=true;   shift ;;
        --help)
            sed -n '1,/^set -euo pipefail/{ /^set -euo pipefail/d; /^#!/d; s/^# \{0,1\}//; p; }' "$0"
            exit 0
            ;;
        *) ui_die "Unknown option: $1. Use --help for usage." ;;
    esac
done

case "$DEV_MODE" in 1|true|yes|on) DEV_MODE=true ;; *) DEV_MODE=false ;; esac
[[ "$ASSUME_YES" == "true" ]] && exec </dev/null      # nobody to ask: every question takes its default
if [[ "$MODULES" != "auto" ]]; then
    MODULES="$(modules_resolve "$MODULES" 2>&1)" || ui_die "--modules: ${MODULES}"
    [[ -n "$MODULES" ]] || MODULES="none"
fi
# One module as the whole system. The same checks run again further down for a
# data directory that was started this way.
only_check() {
    case " $(modules_standalone) " in
        *" $ONLY "*) ;;
        *) ui_die "--only: $ONLY cannot be the whole system. The modules that can: $(modules_standalone)" ;;
    esac
    [[ -z "$SEED_NAME" || "$SEED_NAME" == "none" ]] || ui_die "$(modules_titles "$ONLY") alone runs without the Assets module, and the dataset $SEED_NAME is data for it: use one or the other, each in its own data directory."
    [[ "$MODULES" == "auto" || "$MODULES" == "$ONLY" ]] || ui_die "--only $ONLY and --modules $MODULES do not go together."
    MODULES="$ONLY"; SEED_NAME=""
}
[[ -n "$ONLY" ]] && only_check

# A value Seam reads as "on" (internal/features: 1, true, yes, on).
is_on() { case "${1:-}" in 1|true|TRUE|yes|on) return 0 ;; *) return 1 ;; esac; }

# The modules beyond the defaults that this launch switches on, for the banner.
extra_modules() {
    local out=""
    is_on "${SEAM_ENABLE_DEVELOPER:-}"   && out="$out, Developer"
    is_on "${SEAM_ENABLE_DEMO:-}"        && out="$out, Demo"
    # shellcheck disable=SC2046
    local on; on="$(modules_titles $(modules_on))"
    [[ -n "$on" ]]                       && out="$out, $on"
    [[ -n "${SEAM_FEATURES:-}" ]]        && out="$out, SEAM_FEATURES=${SEAM_FEATURES}"
    echo "${out#, }"
}

# ── Banner ────────────────────────────────────────────────────────────────────
echo ""
banner=("Seam${TAB}http://localhost:${SEAM_PORT}" "xolu${TAB}http://localhost:${XOLU_PORT}" "Data${TAB}${DATA_DIR}")
if [[ -n "$XOLU_SOURCE" ]]; then
    banner+=("xolu source${TAB}${XOLU_SOURCE}")
else
    banner+=("xolu version${TAB}${XOLU_VERSION}")
fi
_extra="$(extra_modules)"
[[ -n "$_extra" ]] && banner+=("Also on${TAB}${_extra}")
[[ -n "$SEED_NAME" ]] && banner+=("Seed${TAB}${SEED_NAME}")
if [[ -n "$ONLY" ]]; then
    banner+=("System${TAB}$(modules_titles "$ONLY") alone, without the Assets module")
elif [[ "$MODULES" != "auto" ]]; then
    banner+=("Modules${TAB}${MODULES// /, }")
fi
[[ "$XOLU_ONLY" == "true" ]] && banner+=("Starting${TAB}xolu alone")
[[ "$NO_START" == "true" ]] && banner+=("Starting${TAB}nothing: build only")
ui_box --title "Seam AMS ${SEAM_VERSION} launcher" -- "${banner[@]}"
[[ "$DEV_MODE" == "true" ]] && ui_warn "Dev mode is on (Seam's development fallbacks). Never use it in production."

# ── Prerequisites ─────────────────────────────────────────────────────────────
ui_step "Prerequisites"

require() {
    command -v "$1" >/dev/null 2>&1 || ui_die "'$1' is required but not found."
}

require go
require git
require curl
# Python reads GitHub release metadata, loads the seeds and draws the boxes.
require python3

# The Go version go.mod asks for. An older Go (1.21 or later) downloads that
# toolchain by itself, so it is a note and not a refusal.
GO_VER="$(go version | awk '{print $3}' | sed 's/go//')"
GO_WANT="$(awk '$1 == "go" { print $2; exit }' "$PROJECT_DIR/go.mod" 2>/dev/null || echo "")"
GO_MINOR="$(echo "$GO_VER" | cut -d. -f2)"
GO_WANT_MINOR="$(echo "$GO_WANT" | cut -d. -f2)"
[[ "$GO_MINOR" =~ ^[0-9]+$ ]] || ui_die "Cannot read the Go version from '$(go version)'"
[[ "$GO_MINOR" -ge 21 ]] || ui_die "Go 1.21 or later is required (found $GO_VER); go.mod asks for $GO_WANT"
if [[ "$GO_WANT_MINOR" =~ ^[0-9]+$ && "$GO_MINOR" -lt "$GO_WANT_MINOR" ]]; then
    ui_ok "Go $GO_VER (go.mod asks for $GO_WANT: Go downloads that toolchain on the first build)"
else
    ui_ok "Go $GO_VER"
fi
ui_ok "Python $(python3 -c 'import sys; print("%d.%d" % sys.version_info[:2])'), git, curl"

case "$LANG_CODE" in
    ""|en|es|pt|ja) ;;
    *) ui_die "Unknown language '$LANG_CODE'. Seam has: $LANGUAGES" ;;
esac

# The seed, if one was named, must exist before anything is built.
SEED_LIST="$(seeds_datasets)" || ui_die "Cannot read the seeds in $PROJECT_DIR/seeds"
if [[ -n "$SEED_NAME" && "$SEED_NAME" != "none" ]]; then
    [[ -n "$(seeds_field "$SEED_NAME" 1)" ]] ||
        ui_die "Unknown seed '$SEED_NAME'. The datasets are: $(echo "$SEED_LIST" | cut -f1 | tr '\n' ' ')"
fi

# ── Resolve optional local xolu source override ────────────────────────────────
if [[ -n "$XOLU_SOURCE" ]]; then
    if [[ -d "$XOLU_SOURCE" && -f "$XOLU_SOURCE/go.mod" ]]; then
        XOLU_SOURCE="$(cd "$XOLU_SOURCE" && pwd)"
        ui_ok "Local xolu source: $XOLU_SOURCE"
    else
        ui_warn "XOLU_SOURCE is not a Go module directory: $XOLU_SOURCE"
        ui_warn "Falling back to xolu from GitHub"
        XOLU_SOURCE=""
    fi
fi

# ── Directories ───────────────────────────────────────────────────────────────
mkdir -p "$DATA_DIR/xolu" "$DATA_DIR/seam" "$PROJECT_DIR/bin" "$XOLU_DIR"

# ── Determine platform and release asset ──────────────────────────────────────
_uname_s="$(uname -s)"
_uname_m="$(uname -m)"

case "$_uname_s" in
    Linux)                _os="linux" ;;
    Darwin)               _os="darwin" ;;
    MINGW*|CYGWIN*|MSYS*) _os="windows" ;;
    *)                    _os="" ;;
esac

case "$_uname_m" in
    x86_64)        _arch="amd64" ;;
    arm64|aarch64) _arch="arm64" ;;
    *)             _arch="" ;;
esac

# macOS release assets are published for arm64 only.
[[ "$_os" == "darwin" && "$_arch" == "amd64" ]] && _arch=""

XOLU_ASSET_NAME="xolu-${_os}-${_arch}"

# ── Download latest xolu release ──────────────────────────────────────────────
try_download_xolu() {
    local token="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
    [[ -n "$token" ]] || return 1
    [[ -n "$_arch" ]] || {
        ui_warn "Unsupported platform: ${_uname_s}/${_uname_m}"
        return 1
    }

    ui_info "Checking latest xolu release (${XOLU_ASSET_NAME}) ..."

    local release_json tag_name asset_url
    release_json="$(
        curl -fsSL \
            -H "Authorization: Bearer ${token}" \
            -H "Accept: application/vnd.github+json" \
            "${XOLU_REPO_API}/releases/latest"
    )" || return 1

    tag_name="$(
        printf '%s' "$release_json" |
            python3 -c 'import json,sys; print(json.load(sys.stdin).get("tag_name",""))'
    )" || return 1

    asset_url="$(
        printf '%s' "$release_json" |
            python3 -c '
import json, sys
d = json.load(sys.stdin)
for a in d.get("assets", []):
    if a["name"] == sys.argv[1]:
        print(a["url"])
        break
' "$XOLU_ASSET_NAME"
    )" || return 1

    if [[ -z "$asset_url" ]]; then
        ui_warn "No release asset '${XOLU_ASSET_NAME}' found in ${tag_name}"
        return 1
    fi

    ui_info "Downloading xolu ${tag_name} ..."
    mkdir -p "$XOLU_DIR"

    local tmp="${XOLU_BIN}.tmp"
    if ! curl -fsSL \
        -H "Authorization: Bearer ${token}" \
        -H "Accept: application/octet-stream" \
        "$asset_url" -o "$tmp"; then
        rm -f "$tmp"
        return 1
    fi

    chmod +x "$tmp"
    mv "$tmp" "$XOLU_BIN"
    ui_ok "xolu ${tag_name} downloaded → ${XOLU_BIN}"
    return 0
}

# ── Clone, select version and build xolu from source ──────────────────────────
build_xolu_from_source() {
    if [[ ! -d "$XOLU_DIR/.git" ]]; then
        ui_info "Cloning xolu from ${XOLU_REPO} ..."
        git clone "$XOLU_REPO" "$XOLU_DIR" ||
            ui_die "Failed to clone xolu"
        ui_ok "xolu cloned → ${XOLU_DIR}"
    fi

    ui_info "Fetching xolu source ..."
    git -C "$XOLU_DIR" fetch origin --prune --tags ||
        ui_die "Failed to fetch xolu from origin"

    git -C "$XOLU_DIR" remote set-head origin -a >/dev/null 2>&1 ||
        true

    local version="$XOLU_VERSION"
    local ref

    case "$version" in
        latest)
            ref="origin/HEAD"
            git -C "$XOLU_DIR" rev-parse --verify \
                "${ref}^{commit}" >/dev/null 2>&1 ||
                ui_die "Cannot resolve the default branch (origin/HEAD)"
            ;;
        previous)
            ref="$(git -C "$XOLU_DIR" rev-parse \
                --verify 'origin/HEAD^' 2>/dev/null)" ||
                ui_die "Cannot resolve the previous commit on the default branch"
            ;;
        *)
            if git -C "$XOLU_DIR" show-ref --verify --quiet \
                "refs/remotes/origin/$version"; then
                ref="origin/$version"
            elif git -C "$XOLU_DIR" rev-parse --verify \
                "${version}^{commit}" >/dev/null 2>&1; then
                ref="$version"
            else
                # Fetch a specified branch, tag or commit if available remotely.
                git -C "$XOLU_DIR" fetch origin "$version" ||
                    ui_die "Cannot fetch requested xolu version: $version"
                ref="FETCH_HEAD"
            fi
            ;;
    esac

    git -C "$XOLU_DIR" checkout --detach "$ref" ||
        ui_die "Cannot check out xolu version: $version"

    ui_ok "Using xolu version: $(git -C "$XOLU_DIR" rev-parse --short HEAD)"

    ui_info "Building xolu ..."
    cd "$XOLU_DIR"

    if [[ -f Makefile ]] && grep -q '^build' Makefile; then
        make build 2>&1 | ui_indent
    else
        go build -o "$XOLU_BIN" ./cmd/xolu 2>&1 | ui_indent
    fi

    [[ -x "$XOLU_BIN" ]] || ui_die "xolu build did not produce ${XOLU_BIN}"
    ui_ok "xolu built → ${XOLU_BIN}"
    cd "$PROJECT_DIR"
}

# ── Obtain xolu binary ────────────────────────────────────────────────────────
ui_step "xolu"

if [[ "$SKIP_BUILD" == "true" && -x "$XOLU_BIN" ]]; then
    ui_ok "Skipping xolu fetch (binary exists at ${XOLU_BIN})"
elif [[ -n "$XOLU_SOURCE" ]]; then
    ui_info "Building xolu from local source: ${XOLU_SOURCE} ..."
    (
        cd "$XOLU_SOURCE"
        go build -o "$XOLU_BIN" ./cmd/xolu 2>&1 | ui_indent
    ) || ui_die "Failed to build xolu from local source: $XOLU_SOURCE"
    [[ -x "$XOLU_BIN" ]] || ui_die "Local xolu build did not produce ${XOLU_BIN}"
    ui_ok "xolu built from local source → ${XOLU_BIN}"
elif [[ "$XOLU_VERSION" == "latest" ]] && try_download_xolu; then
    :
else
    if [[ "$XOLU_VERSION" != "latest" ]]; then
        ui_info "Building requested xolu version: ${XOLU_VERSION}"
    elif [[ -z "${GH_TOKEN:-${GITHUB_TOKEN:-}}" ]]; then
        ui_info "No GitHub token set — building xolu from source"
    else
        ui_warn "Release download failed — falling back to source build"
    fi
    build_xolu_from_source
fi

# ── Build Seam and seam-admin ─────────────────────────────────────────────────
SEAM_BIN="$PROJECT_DIR/bin/seam"
ADMIN_BIN="$PROJECT_DIR/bin/seam-admin"

if [[ "$XOLU_ONLY" != "true" ]]; then
    ui_step "Seam"
    if [[ "$SKIP_BUILD" == "true" && -x "$SEAM_BIN" && -x "$ADMIN_BIN" ]]; then
        ui_ok "Skipping the Seam build (binaries exist)"
    else
        ui_info "Building Seam AMS ..."
        cd "$PROJECT_DIR"
        go build -o "$SEAM_BIN" ./cmd/seam 2>&1 | ui_indent
        go build -o "$ADMIN_BIN" ./cmd/seam-admin 2>&1 | ui_indent
        [[ -x "$SEAM_BIN" && -x "$ADMIN_BIN" ]] || ui_die "The Seam build did not produce bin/seam and bin/seam-admin"
        ui_ok "Seam built → bin/seam, bin/seam-admin"
    fi
fi

if [[ "$NO_START" == "true" ]]; then
    echo ""
    ui_ok "Built. Nothing was started (--no-start)."
    exit 0
fi

# ── Generate secrets on first run ─────────────────────────────────────────────
SECRETS_FILE="$DATA_DIR/.secrets"
SEAM_CONFIG="$DATA_DIR/seam/config.json"

if [[ "$XOLU_ONLY" != "true" ]]; then
    if [[ ! -f "$SECRETS_FILE" ]]; then
        require openssl
        SESSION_SECRET="$(openssl rand -hex 24)"
        JWT_SECRET="$(openssl rand -hex 24)"

        cat > "$SECRETS_FILE" <<SECRETS
# Auto-generated by launch.sh — do not commit this file
SESSION_SECRET=${SESSION_SECRET}
JWT_SECRET=${JWT_SECRET}
SECRETS

        chmod 600 "$SECRETS_FILE"
        ui_ok "Secrets written to ${SECRETS_FILE}"
    fi

    # shellcheck source=/dev/null
    source "$SECRETS_FILE"

    # ── Write Seam config on first run ────────────────────────────────────────
    # The file is written once and is then the user's to edit. What a launch
    # can change from the command line (ports, dev mode, modules) is passed to
    # Seam in its environment, which wins over the file, so --seam-port or
    # --dev on a later launch takes effect without rewriting it.
    if [[ ! -f "$SEAM_CONFIG" ]]; then
        cat > "$SEAM_CONFIG" <<CONFIG
{
  "server": {
    "host": "0.0.0.0",
    "port": ${SEAM_PORT},
    "read_timeout": "30s",
    "write_timeout": "30s",
    "shutdown_timeout": "10s",
    "max_body_size": 10485760
  },
  "xolu": {
    "url": "http://localhost:${XOLU_PORT}",
    "timeout": "30s",
    "max_connections": 100,
    "retry_attempts": 3,
    "retry_delay": "1s"
  },
  "auth": {
    "enabled": true,
    "jwt_secret": "${JWT_SECRET}",
    "token_expiry": "24h",
    "refresh_expiry": "168h",
    "session_secret": "${SESSION_SECRET}",
    "session_duration": "24h",
    "csrf_enabled": true,
    "tenant_from_claims_only": true
  },
  "features": {
    "enable_ui": true,
    "enable_api": true,
    "enable_webhooks": true,
    "enable_rules": true,
    "enable_fsm": true,
    "enable_qr": true,
    "qr_prefix": "SHF",
    "qr_base_url": "http://localhost:${SEAM_PORT}/s/",
    "rate_limit_per_min": 0,
    "dev_mode": false
  },
  "log": {
    "level": "info",
    "format": "json",
    "output": "stdout"
  }
}
CONFIG
        ui_ok "Seam config written to ${SEAM_CONFIG}"
    fi
fi

# ── Cleanup on exit ───────────────────────────────────────────────────────────
XOLU_PID=""
SEAM_PID=""

cleanup() {
    trap - INT TERM EXIT
    echo ""
    ui_info "Shutting down ..."

    if [[ -n "$SEAM_PID" ]]; then
        kill "$SEAM_PID" 2>/dev/null || true
        wait "$SEAM_PID" 2>/dev/null || true
        ui_ok "Seam stopped"
    fi

    if [[ -n "$XOLU_PID" ]]; then
        kill "$XOLU_PID" 2>/dev/null || true
        wait "$XOLU_PID" 2>/dev/null || true
        ui_ok "xolu stopped"
    fi
}

# Ctrl-C is how this is stopped: it is not a failure, so make reports none.
trap 'cleanup; exit 0' INT TERM
trap cleanup EXIT

# ── Clipboard helper ──────────────────────────────────────────────────────────
copy_to_clipboard() {
    local text="$1"
    [[ -t 1 ]] || return 1

    if command -v pbcopy >/dev/null 2>&1; then
        printf '%s' "$text" | pbcopy && return 0
    elif command -v wl-copy >/dev/null 2>&1; then
        printf '%s' "$text" | wl-copy && return 0
    elif command -v xclip >/dev/null 2>&1; then
        printf '%s' "$text" | xclip -selection clipboard && return 0
    elif command -v xsel >/dev/null 2>&1; then
        printf '%s' "$text" | xsel --clipboard --input && return 0
    fi

    return 1
}

# ── Start xolu ─────────────────────────────────────────────────────────────────
ui_step "Starting"

XOLU_URL_LOCAL="http://localhost:${XOLU_PORT}"

if curl -sf "$XOLU_URL_LOCAL/health" >/dev/null 2>&1; then
    ui_die "Something already answers on port ${XOLU_PORT}. Stop it, or choose another port with --xolu-port."
fi

ui_info "Starting xolu on :${XOLU_PORT} ..."

# What the two servers print goes to a file each, so that this terminal shows
# what the launcher says and nothing scrolls it away; --verbose puts it here.
XOLU_LOG="$DATA_DIR/xolu.log"
SEAM_LOG="$DATA_DIR/seam.log"

# show_log FILE — the end of a server's log, when it failed to start.
show_log() {
    [[ "$VERBOSE" == "true" || ! -s "$1" ]] && return 0
    ui_note "The last lines of $1:" >&2
    tail -n 15 "$1" | ui_indent >&2
}

# The settings are in scripts/lib/xolu-env.sh, shared with the demo builder.
(
    xolu_env_export "$XOLU_PORT" "$DATA_DIR/xolu" info
    [[ "$VERBOSE" == "true" ]] || exec >> "$XOLU_LOG" 2>&1
    exec "$XOLU_BIN"
) &
XOLU_PID=$!

xolu_wait "$XOLU_URL_LOCAL" 30 "$XOLU_PID" || {
    rc=$?
    show_log "$XOLU_LOG"
    [[ $rc -eq 2 ]] && ui_die "xolu exited unexpectedly"
    ui_die "xolu did not become ready within 30s"
}
ui_ok "xolu ready (${WAITED}s)"

if [[ "$XOLU_ONLY" == "true" ]]; then
    echo ""
    ui_box --title "xolu is running" --style ok -- \
        "xolu${TAB}${XOLU_URL_LOCAL}" \
        "Data${TAB}${DATA_DIR}/xolu" \
        "Log${TAB}$([[ "$VERBOSE" == "true" ]] && echo "this terminal" || echo "$XOLU_LOG")" \
        "---" \
        "Press Ctrl-C to stop it"
    echo ""
    wait "$XOLU_PID"
    exit 0
fi

# ── The data this instance starts with (first run only) ───────────────────────
# .seeded holds the name of the dataset loaded into this data directory;
# .admin_created says an administrator was made by hand. Either one means the
# directory is in use and nobody is asked again.
ADMIN_MARKER="$DATA_DIR/.admin_created"
SEED_MARKER="$DATA_DIR/.seeded"
ONLY_MARKER="$DATA_DIR/.only"
FIRST_RUN=false
[[ ! -f "$ADMIN_MARKER" && ! -f "$SEED_MARKER" && ! -f "$ONLY_MARKER" ]] && FIRST_RUN=true

# A data directory started with --only stays that way; one that holds a
# dataset, or was started as a whole instance, cannot become one.
if [[ -z "$ONLY" && -f "$ONLY_MARKER" ]]; then
    ONLY="$(cat "$ONLY_MARKER")"
    only_check
    ui_ok "This data directory runs $(modules_titles "$ONLY") alone"
elif [[ -n "$ONLY" && ! -f "$ONLY_MARKER" && "$FIRST_RUN" == "false" ]]; then
    ui_die "--only $ONLY needs a data directory of its own: ${DATA_DIR} already holds an instance with the Assets module. Use another --data-dir."
fi
if [[ -n "$ONLY" && ! -f "$ONLY_MARKER" ]]; then
    only_seeds=""
    [[ "$SEED_ADDITIONS" == "none" ]] || only_seeds="$(seeds_standalone "$ONLY")"
    if [[ -n "$only_seeds" ]]; then
        ui_info "Loading the data of $(modules_titles "$ONLY") (${only_seeds// /, }) ..."
        # shellcheck disable=SC2086
        seeds_load "$XOLU_URL_LOCAL" $only_seeds > "$DATA_DIR/.seed-load.log" 2>&1 || {
            tail -n 12 "$DATA_DIR/.seed-load.log" | ui_indent >&2
            ui_die "Loading the data failed. The whole output is in ${DATA_DIR}/.seed-load.log"
        }
        ui_ok "Data loaded: $(grep -E 'entity +' "$DATA_DIR/.seed-load.log" | head -1 | sed 's/^ *entity *//')"
    fi
    echo "$ONLY" > "$ONLY_MARKER"
fi

if [[ "$FIRST_RUN" == "true" && -z "$ONLY" && -z "$SEED_NAME" && "$NO_ADMIN" == "false" && -t 0 ]]; then
    ui_step "First run: what should this instance start with?"
    ui_note "A dataset comes with its work orders, issues and accounting, with those modules on, and with accounts to sign in with."
    menu=("Empty${TAB}you create an administrator now")
    while IFS="$TAB" read -r _name _company _lang _email _pw; do
        menu+=("${_name}${TAB}${_company} (${_lang})")
    done <<< "$SEED_LIST"
    ui_menu choice "Start with" 1 "${menu[@]}"
    if [[ "$choice" -gt 1 ]]; then
        SEED_NAME="$(echo "$SEED_LIST" | sed -n "$((choice - 1))p" | cut -f1)"
    fi
fi
[[ "$SEED_NAME" == "none" ]] && SEED_NAME=""

if [[ -n "$SEED_NAME" ]]; then
    if [[ -f "$SEED_MARKER" && "$(cat "$SEED_MARKER")" != "$SEED_NAME" ]]; then
        ui_warn "This data directory holds the dataset $(cat "$SEED_MARKER"); $SEED_NAME was not loaded."
        ui_warn "Two datasets do not go in one instance: use another --data-dir, or remove ${DATA_DIR}."
        SEED_NAME="$(cat "$SEED_MARKER")"
    elif [[ -f "$SEED_MARKER" ]]; then
        # Loaded on an earlier launch. It is not loaded again: what was changed or
        # deleted since stays that way. To add what a newer version of the seeds
        # brings: python3 seeds/seed.py load NAME --url ...
        ui_ok "Data: ${SEED_NAME} (loaded on an earlier launch)"
    else
        case "$SEED_ADDITIONS" in
            all)  additions="$(seeds_additions "$SEED_NAME")" ;;
            none) additions="" ;;
            *)    additions="${SEED_ADDITIONS//,/ }" ;;
        esac
        ui_info "Loading the dataset ${SEED_NAME}${additions:+ and its additions (${additions// /, })} ..."
        # Records that exist are left alone, so this is safe on every launch.
        set +e
        # shellcheck disable=SC2086
        seeds_load "$XOLU_URL_LOCAL" "$SEED_NAME" $additions > "$DATA_DIR/.seed-load.log" 2>&1
        seed_rc=$?
        set -e
        if [[ "$seed_rc" -eq 0 ]]; then
            echo "$SEED_NAME" > "$SEED_MARKER"
            ui_ok "Data loaded: $(grep -c . <<< "${SEED_NAME} ${additions}" >/dev/null; grep -E 'entity +' "$DATA_DIR/.seed-load.log" | head -1 | sed 's/^ *entity *//')"
        elif [[ "$seed_rc" -eq 3 ]]; then
            ui_warn "The instance already holds other data, so the seed was not loaded:"
            grep -i 'refused\|already' "$DATA_DIR/.seed-load.log" | head -3 | ui_indent >&2 || true
            SEED_NAME=""
        else
            tail -n 12 "$DATA_DIR/.seed-load.log" | ui_indent >&2
            ui_die "Loading the seed failed (exit $seed_rc). The whole output is in ${DATA_DIR}/.seed-load.log"
        fi
    fi
elif [[ -f "$SEED_MARKER" ]]; then
    SEED_NAME="$(cat "$SEED_MARKER")"
fi

# ── Create initial admin user (first run, no dataset) ─────────────────────────
if [[ "$FIRST_RUN" == "true" && -z "$SEED_NAME" && "$NO_ADMIN" == "false" ]]; then
    if [[ -t 0 ]]; then
        ui_step "First run: create an administrator"
        ui_note "Leave the email empty to skip; 'make admin-create' does it later."
        echo ""
        ui_ask ADMIN_EMAIL "Admin email" ""
        ADMIN_NAME=""
        [[ -n "$ADMIN_EMAIL" ]] && ui_ask ADMIN_NAME "Admin name" ""

        if [[ -z "$ADMIN_EMAIL" || -z "$ADMIN_NAME" ]]; then
            ui_warn "No administrator was created. Run 'make admin-create' while this is running."
        else
            ui_note "You will be asked for a password."
            echo ""
            if SEAM_XOLU_URL="$XOLU_URL_LOCAL" \
                "$ADMIN_BIN" user create \
                    --email="$ADMIN_EMAIL" \
                    --name="$ADMIN_NAME" \
                    --role=admin; then
                touch "$ADMIN_MARKER"
                ui_ok "Administrator created: ${ADMIN_EMAIL}"
            else
                ui_warn "The administrator was not created. Run 'make admin-create' while this is running."
            fi
        fi
    elif [[ -z "$ONLY" ]]; then
        ui_warn "Nobody can sign in yet: no dataset was loaded and there is no terminal to ask for an administrator."
        ui_warn "Run 'make admin-create' while this is running, or start again with --seed NAME."
    fi
fi

# ── Modules ───────────────────────────────────────────────────────────────────
# Work Orders, Issues and Accounting are off in Seam unless they are switched
# on (scripts/lib/modules.sh). A data directory keeps which of them it runs
# with: the ones its data was loaded for, or the ones named with --modules.
# An empty instance runs with none, as a new installation of Seam does.
MODULES_MARKER="$DATA_DIR/.modules"
if [[ "$MODULES" != "auto" ]]; then
    modules_now="$MODULES"; [[ "$modules_now" == "none" ]] && modules_now=""
    echo "$modules_now" > "$MODULES_MARKER"
elif [[ -f "$MODULES_MARKER" ]]; then
    modules_now="$(modules_resolve "$(cat "$MODULES_MARKER")" 2>/dev/null || true)"
elif [[ -n "$SEED_NAME" ]]; then
    # loaded now, or by a launch from before this file was kept
    # shellcheck disable=SC2086
    modules_now="$(seeds_modules "$SEED_NAME" ${additions-$(seeds_additions "$SEED_NAME")})"
    modules_now="$(modules_resolve "$modules_now" 2>/dev/null || true)"
    echo "$modules_now" > "$MODULES_MARKER"
else
    modules_now=""
fi
# shellcheck disable=SC2086
modules_export $modules_now
# shellcheck disable=SC2046
_on="$(modules_titles $(modules_on))"
ui_ok "Modules beyond the defaults: ${_on:-none}"

# ── Language selection ────────────────────────────────────────────────────────
# Asked once for a data directory and remembered; --lang and
# SEAM_FORCE_LANGUAGE always win. A dataset proposes its own language.
LANG_FILE="$DATA_DIR/.language"
if [[ -z "$LANG_CODE" && -f "$LANG_FILE" ]]; then
    LANG_CODE="$(cat "$LANG_FILE")"
elif [[ -z "$LANG_CODE" ]]; then
    lang_default=1
    if [[ -n "$SEED_NAME" ]]; then
        case "$(seeds_field "$SEED_NAME" 3)" in es) lang_default=2 ;; pt) lang_default=3 ;; ja) lang_default=4 ;; esac
    fi
    if [[ -t 0 ]]; then
        ui_step "Language of the interface"
        ui_menu lang_choice "Language" "$lang_default" "English${TAB}en" "Español${TAB}es" "Português${TAB}pt" "日本語${TAB}ja"
    else
        lang_choice="$lang_default"
    fi
    case "$lang_choice" in 1) LANG_CODE="en" ;; 2) LANG_CODE="es" ;; 3) LANG_CODE="pt" ;; 4) LANG_CODE="ja" ;; esac
fi
echo "$LANG_CODE" > "$LANG_FILE"

# ── Start Seam ────────────────────────────────────────────────────────────────
SEAM_URL="http://localhost:${SEAM_PORT}"

if curl -sf "$SEAM_URL/health" >/dev/null 2>&1; then
    ui_die "Something already answers on port ${SEAM_PORT}. Stop it, or choose another port with --seam-port."
fi

ui_info "Starting Seam AMS on :${SEAM_PORT} (language: ${LANG_CODE}) ..."

# Dev mode has no variable of its own in Seam: it goes in the SEAM_FEATURES list.
seam_features="${SEAM_FEATURES:-}"
[[ "$DEV_MODE" == "true" ]] && seam_features="${seam_features:+$seam_features,}dev_mode"

(
    export SEAM_FORCE_LANGUAGE="$LANG_CODE"
    export SEAM_PORT="$SEAM_PORT"
    export XOLU_URL="$XOLU_URL_LOCAL"
    export SEAM_ACCOUNTING_DOCS_DIR="${SEAM_ACCOUNTING_DOCS_DIR:-$DATA_DIR/accounting-docs}"
    # one module as the whole system: the Assets module's routes stay unmounted
    [[ -n "$ONLY" ]] && export SEAM_ENABLE_OPERATIONAL=0
    [[ -n "$seam_features" ]] && export SEAM_FEATURES="$seam_features"
    [[ "$VERBOSE" == "true" ]] || exec >> "$SEAM_LOG" 2>&1
    exec "$SEAM_BIN" -config "$SEAM_CONFIG"
) &
SEAM_PID=$!

xolu_wait "$SEAM_URL" 30 "$SEAM_PID" || {
    rc=$?
    show_log "$SEAM_LOG"
    [[ $rc -eq 2 ]] && ui_die "Seam exited unexpectedly"
    ui_die "Seam did not become ready within 30s"
}
ui_ok "Seam ready (${WAITED}s)"

# ── Ready ─────────────────────────────────────────────────────────────────────
ready=("Open${TAB}${SEAM_URL}" "API${TAB}${SEAM_URL}/api/v1/" "xolu${TAB}${XOLU_URL_LOCAL}" "Language${TAB}${LANG_CODE}")
_extra="$(extra_modules)"
[[ "$DEV_MODE" == "true" ]] && _extra="${_extra:+$_extra, }dev mode"
[[ -n "$_extra" ]] && ready+=("Also on${TAB}${_extra}")
if [[ -n "$SEED_NAME" ]]; then
    ready+=("" "Data${TAB}${SEED_NAME}: $(seeds_field "$SEED_NAME" 2)")
    ready+=("Sign in${TAB}$(seeds_field "$SEED_NAME" 4) / $(seeds_field "$SEED_NAME" 5)")
    is_on "${SEAM_ENABLE_DEVELOPER:-}" && ready+=("Developer${TAB}dev@ual.li / holaquetal, then open /developer")
    case " $(modules_on) " in *" accounting "*) ready+=("Accounting${TAB}accounting.staff@example.com / holaquetal, then open /accounting") ;; esac
elif [[ -n "$ONLY" ]]; then
    ready+=("" "System${TAB}$(modules_titles "$ONLY") alone, without the Assets module")
    _login="$(seeds_login "$(seeds_standalone "$ONLY" | cut -d' ' -f1)")"
    [[ -n "$_login" ]] && ready+=("Sign in${TAB}$(echo "$_login" | cut -f1) / $(echo "$_login" | cut -f2)")
    if is_on "${SEAM_ENABLE_DEVELOPER:-}"; then
        ready+=("Developer${TAB}dev@ual.li / holaquetal, then open /developer: view it as each role")
    else
        ready+=("Users${TAB}dev@ual.li / holaquetal manages the others")
    fi
elif is_on "${SEAM_ENABLE_DEVELOPER:-}"; then
    ready+=("" "Developer${TAB}this data has no developer account: 'make user-create' with the role developer")
fi
[[ "$VERBOSE" == "true" ]] || ready+=("" "Logs${TAB}${DATA_DIR}/seam.log, xolu.log")
ready+=("---")
copy_to_clipboard "$SEAM_URL" && ready+=("The address is in the clipboard.")
ready+=("Press Ctrl-C to stop both services.")

echo ""
ui_box --title "Seam AMS ${SEAM_VERSION} is running" --style ok -- "${ready[@]}"
echo ""

wait "$XOLU_PID" "$SEAM_PID"
