#!/usr/bin/env bash
# scripts/build-demo.sh — Build self-contained demo packages
#
# Cross-compiles xolu and Seam for the three platforms, loads a dataset and
# its additions (work orders, issues, accounting) into a temporary xolu, and
# packages the binaries, the database and a launcher into a demo package.
# The target machine needs zero dependencies — no Python, no Go, no network.
#
# Output: demo/seam-demo-{platform}-seed{NN}-{seq}/ and demo/zip/{name}.zip
#
# Usage:
#   ./scripts/build-demo.sh [options]
#   make demo [SEED=03-extinfire] [PLATFORM=linux]
#
# Options:
#   --seed NAME         The dataset: 01-constructec ... 07-beiramagalhaes
#                       (asked when not given)
#   --platform NAME     windows, linux or mac (asked when not given)
#   --additions WHAT    "all" (default), "none", or a list such as
#                       "wo-templates,accounting"
#   --modules WHAT      The modules that are off by default (work_orders,
#                       issues, accounting) the package runs with: "auto"
#                       (default: those its data is for), "all", "none", or
#                       a list such as "issues,work_orders"
#   --only MODULE       A package of that module as the whole system, without
#                       the Assets module and without a dataset: accounting
#   --help              Show this help
#
# Environment:
#   XOLU_SOURCE         A local xolu source checkout to build from, instead of
#                       cloning or updating ./xolu from GitHub
#   Output: NO_COLOR, SEAM_COLOR=always|never, SEAM_ASCII=1 (scripts/lib/ui.sh)
#
# What the package runs is what scripts/launch.sh runs: xolu is started with
# the settings of scripts/lib/xolu-env.sh and the data is loaded through
# scripts/lib/seeds.sh, in both.
#
# Prerequisites (build machine only — nothing needed on target):
#   - Go (the version go.mod asks for; an older one downloads it)
#   - Python 3.8 or later (standard library only)
#   - Git (first run only, to clone xolu), curl, zip
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

# shellcheck source=scripts/lib/ui.sh
source "$SCRIPT_DIR/lib/ui.sh"
# shellcheck source=scripts/lib/xolu-env.sh
source "$SCRIPT_DIR/lib/xolu-env.sh"
# shellcheck source=scripts/lib/seeds.sh
source "$SCRIPT_DIR/lib/seeds.sh"
# shellcheck source=scripts/lib/modules.sh
source "$SCRIPT_DIR/lib/modules.sh"

TAB=$'\t'

# ── Configuration ─────────────────────────────────────────────────────────────

DEMO_DIR="$PROJECT_DIR/demo"
BUILD_CACHE="$DEMO_DIR/build"
XOLU_DIR="$PROJECT_DIR/xolu"
XOLU_REPO="https://github.com/ha1tch/xolu.git"
XOLU_SOURCE="${XOLU_SOURCE:-}"

SEAM_VERSION="$(cat VERSION 2>/dev/null || echo dev)"

# Non-standard port to avoid clashing with running instances
SEED_PORT=19090

# Ports baked into demo packages
DEMO_XOLU_PORT=9090
DEMO_SEAM_PORT=8080

SEED_LABEL=""
PLATFORM_ARG=""
SEED_ADDITIONS="all"
MODULES="auto"
ONLY=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --seed)      [[ $# -ge 2 ]] || ui_die "--seed requires a value";      SEED_LABEL="$2"; shift 2 ;;
        --platform)  [[ $# -ge 2 ]] || ui_die "--platform requires a value";  PLATFORM_ARG="$2"; shift 2 ;;
        --additions) [[ $# -ge 2 ]] || ui_die "--additions requires a value"; SEED_ADDITIONS="$2"; shift 2 ;;
        --modules)   [[ $# -ge 2 ]] || ui_die "--modules requires a value";   MODULES="$2"; shift 2 ;;
        --only)      [[ $# -ge 2 ]] || ui_die "--only requires a value";      ONLY="$2"; shift 2 ;;
        --help)
            sed -n '1,/^set -euo pipefail/{ /^set -euo pipefail/d; /^#!/d; s/^# \{0,1\}//; p; }' "$0"
            exit 0
            ;;
        *) ui_die "Unknown option: $1. Use --help for usage." ;;
    esac
done

# ── Banner ────────────────────────────────────────────────────────────────────

echo ""
SEED_BANNER="${SEED_LABEL:-${ONLY:+none: $ONLY alone}}"
ui_box --title "Seam AMS ${SEAM_VERSION} demo builder" -- \
    "Output${TAB}demo/" \
    "Seed${TAB}${SEED_BANNER:-asked below}" \
    "Platform${TAB}${PLATFORM_ARG:-asked below}"

# ── Prerequisites ─────────────────────────────────────────────────────────────

ui_step "Prerequisites"

require() { command -v "$1" >/dev/null 2>&1 || ui_die "'$1' is required but not found."; }
require go
require git
require python3
require curl
require zip

ui_ok "Go $(go version | awk '{print $3}' | sed 's/go//')"

python3 -c "import sys; sys.exit(0 if sys.version_info >= (3, 8) else 1)" 2>/dev/null \
    || ui_die "Python 3.8 or later is needed for the seeds"
ui_ok "Python $(python3 -c 'import sys; print("%d.%d" % sys.version_info[:2])')"

SEED_LIST="$(seeds_datasets)" || ui_die "Cannot read the seeds in $PROJECT_DIR/seeds"
[[ -n "$SEED_LIST" ]] || ui_die "No datasets found in seeds/"
if [[ -n "$ONLY" ]]; then
    case " $(modules_standalone) " in
        *" $ONLY "*) ;;
        *) ui_die "--only: $ONLY cannot be the whole system. The modules that can: $(modules_standalone)" ;;
    esac
    [[ -z "$SEED_LABEL" ]] || ui_die "$(modules_titles "$ONLY") alone runs without the Assets module, and the dataset $SEED_LABEL is data for it: use one or the other."
    [[ "$MODULES" == "auto" || "$MODULES" == "$ONLY" ]] || ui_die "--only $ONLY and --modules $MODULES do not go together."
elif [[ -n "$SEED_LABEL" ]]; then
    [[ -n "$(seeds_field "$SEED_LABEL" 1)" ]] ||
        ui_die "Unknown seed '$SEED_LABEL'. The datasets are: $(echo "$SEED_LIST" | cut -f1 | tr '\n' ' ')"
fi
case "$PLATFORM_ARG" in
    ""|windows|win|linux|mac|macos|darwin) ;;
    *) ui_die "Unknown platform '$PLATFORM_ARG'. Use windows, linux or mac." ;;
esac

# ── Obtain xolu source ───────────────────────────────────────────────────────

ui_step "xolu source"

if [[ -n "$XOLU_SOURCE" ]]; then
    [[ -d "$XOLU_SOURCE" && -f "$XOLU_SOURCE/go.mod" ]] || ui_die "XOLU_SOURCE is not a Go module directory: $XOLU_SOURCE"
    XOLU_DIR="$(cd "$XOLU_SOURCE" && pwd)"
    ui_ok "Local xolu source: $XOLU_DIR"
elif [[ -d "$XOLU_DIR/.git" ]]; then
    ui_info "Updating xolu ..."
    git -C "$XOLU_DIR" fetch --depth 1 origin 2>&1 | ui_indent
    git -C "$XOLU_DIR" reset --hard origin/HEAD 2>/dev/null | ui_indent
    ui_ok "xolu @ $(git -C "$XOLU_DIR" rev-parse --short HEAD)"
elif [[ -d "$XOLU_DIR" ]] && [[ -f "$XOLU_DIR/go.mod" ]]; then
    ui_ok "xolu found at $XOLU_DIR (local copy)"
else
    ui_info "Cloning xolu from $XOLU_REPO ..."
    git clone --depth 1 "$XOLU_REPO" "$XOLU_DIR" 2>&1 | ui_indent
    ui_ok "xolu cloned"
fi

# ── Cross-compile all platforms ───────────────────────────────────────────────

ui_step "Compiling for three platforms (into demo/build/)"

mkdir -p "$BUILD_CACHE"

# Every run compiles: Go's own build cache makes an unchanged binary quick,
# and a package can never carry a Seam or a xolu older than the source.
cross_build() {
    local goos="$1" goarch="$2" label="$3" ext=""
    [[ "$goos" == "windows" ]] && ext=".exe"

    local dest="$BUILD_CACHE/$label"
    mkdir -p "$dest"

    (cd "$XOLU_DIR" && \
        CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
        go build -trimpath -ldflags "-s -w" \
        -o "$dest/xolu${ext}" ./cmd/xolu) 2>&1 | ui_indent
    [[ -f "$dest/xolu${ext}" ]] || ui_die "$label: the xolu build produced nothing"

    (cd "$PROJECT_DIR" && \
        CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
        go build -trimpath -ldflags "-s -w" \
        -o "$dest/seam${ext}" ./cmd/seam) 2>&1 | ui_indent
    [[ -f "$dest/seam${ext}" ]] || ui_die "$label: the Seam build produced nothing"
    ui_ok "$label: xolu, seam"
}

cross_build linux  amd64 linux-amd64
cross_build darwin arm64 darwin-arm64
cross_build windows amd64 windows-amd64

# A NATIVE xolu loads the data — this one is built with the platform's
# defaults (CGO where there is a C compiler) so that SQLite storage writes to
# disk as it does for a developer. The cross-compiled binaries above use
# CGO_ENABLED=0, which cross-compilation needs.
# This native binary is never packaged — it's only used during the build.
NATIVE_XOLU="$BUILD_CACHE/native/xolu"
mkdir -p "$BUILD_CACHE/native"
(cd "$XOLU_DIR" && \
    go build -trimpath -ldflags "-s -w" \
    -o "$NATIVE_XOLU" ./cmd/xolu) 2>&1 | ui_indent
[[ -x "$NATIVE_XOLU" ]] || ui_die "The native xolu build produced nothing"
ui_ok "native xolu, for loading the data ($(uname -s)/$(uname -m))"

# ── Select seed ──────────────────────────────────────────────────────────────

if [[ -n "$ONLY" ]]; then
    : # no dataset: the module's own data, below
elif [[ -z "$SEED_LABEL" ]]; then
    ui_step "Dataset"
    menu=()
    while IFS="$TAB" read -r _name _company _lang _email _pw; do
        menu+=("${_name}${TAB}${_company} (${_lang})")
    done <<< "$SEED_LIST"
    ui_menu seed_choice "Dataset" 1 "${menu[@]}"
    SEED_LABEL="$(echo "$SEED_LIST" | sed -n "${seed_choice}p" | cut -f1)"
fi
SEED_NUM="$(echo "$SEED_LABEL" | grep -oE '^[0-9]+' || true)"
SEED_COMPANY="$(seeds_field "$SEED_LABEL" 2)"
SEED_LANG="$(seeds_field "$SEED_LABEL" 3)"

# The login shown at the end comes from the seed itself: its header lists the
# accounts it creates (x.seam.logins), so a demo built from any seed shows
# that seed's own admin account and password.
LOGIN_EMAIL="$(seeds_field "$SEED_LABEL" 4)"
LOGIN_PASSWORD="$(seeds_field "$SEED_LABEL" 5)"
OPERATIONAL=true
if [[ -n "$ONLY" ]]; then
    # One module as the whole system: its own data in place of a dataset, its
    # own first account, and the Assets module's routes left unmounted.
    SEED_LABEL="$(seeds_standalone "$ONLY")"
    [[ -n "$SEED_LABEL" ]] || ui_die "No seed in seeds/ holds data for $ONLY alone"
    SEED_NUM=""
    SEED_COMPANY="$(modules_titles "$ONLY") alone, without the Assets module"
    SEED_LANG="en"
    _login="$(seeds_login "${SEED_LABEL%% *}")"
    LOGIN_EMAIL="$(echo "$_login" | cut -f1)"
    LOGIN_PASSWORD="$(echo "$_login" | cut -f2)"
    SEED_ADDITIONS="none"
    MODULES="$ONLY"
    OPERATIONAL=false
fi
[[ -n "$LOGIN_EMAIL" && -n "$LOGIN_PASSWORD" ]] || ui_die "The seed $SEED_LABEL lists no login in its header (x.seam.logins)"

case "$SEED_ADDITIONS" in
    all)  ADDITIONS="$(seeds_additions "${SEED_LABEL%% *}")" ;;
    none) ADDITIONS="" ;;
    *)    ADDITIONS="${SEED_ADDITIONS//,/ }" ;;
esac

# The modules the package runs with (scripts/lib/modules.sh): those its data
# is for, unless --modules names them. launch.sh decides the same way.
if [[ "$MODULES" == "auto" ]]; then
    # shellcheck disable=SC2086
    MODULES="$(seeds_modules "$SEED_LABEL" $ADDITIONS)"
fi
DEMO_MODULES="$(modules_resolve "$MODULES" 2>&1)" || ui_die "--modules: ${DEMO_MODULES}"
MODULE_CONFIG=""
for _id in $(modules_ids); do
    case " $DEMO_MODULES " in *" $_id "*) _on=true ;; *) _on=false ;; esac
    MODULE_CONFIG="${MODULE_CONFIG}    \"$(modules_field "$_id" 4)\": ${_on},"$'\n'
done
MODULE_CONFIG="${MODULE_CONFIG%$'\n'}"
# shellcheck disable=SC2086
MODULE_TITLES="$(modules_titles $DEMO_MODULES)"
MODULES_LINE="the defaults${MODULE_TITLES:+, and ${MODULE_TITLES}}"
[[ -n "$ONLY" ]] && MODULES_LINE="${MODULE_TITLES}, and none of the Assets module"

# ── Select platform ──────────────────────────────────────────────────────────

if [[ -z "$PLATFORM_ARG" ]]; then
    ui_step "Target platform"
    ui_menu plat_choice "Platform" 1 "windows${TAB}Windows 64-bit" "linux${TAB}Linux 64-bit" "mac${TAB}macOS Apple Silicon"
    case "$plat_choice" in 1) PLATFORM_ARG="windows" ;; 2) PLATFORM_ARG="linux" ;; 3) PLATFORM_ARG="mac" ;; esac
fi

case "$PLATFORM_ARG" in
    windows|win)      PLATFORM="windows-amd64"; PLAT_SHORT="win";   EXT=".exe" ;;
    linux)            PLATFORM="linux-amd64";   PLAT_SHORT="linux"; EXT="" ;;
    mac|macos|darwin) PLATFORM="darwin-arm64";  PLAT_SHORT="mac";   EXT="" ;;
esac

# ── Pre-seed database ─────────────────────────────────────────────────────────

ui_step "Loading the data: $SEED_LABEL"

SEED_TMPDIR=$(mktemp -d)

# Cleanup: always kill xolu and remove temp dir
cleanup() {
    if [[ -n "${XOLU_PID:-}" ]]; then
        kill "$XOLU_PID" 2>/dev/null || true
        wait "$XOLU_PID" 2>/dev/null || true
    fi
    rm -rf "$SEED_TMPDIR"
}
trap cleanup EXIT

mkdir -p "$SEED_TMPDIR/xolu"

if curl -sf "http://localhost:${SEED_PORT}/health" >/dev/null 2>&1; then
    ui_die "Something already answers on port ${SEED_PORT}, which the build uses to load the data. Stop it and run again."
fi

# The settings xolu runs with are the ones the package will run it with.
(
    xolu_env_export "$SEED_PORT" "$SEED_TMPDIR/xolu" error
    exec "$NATIVE_XOLU"
) > "$SEED_TMPDIR/xolu.log" 2>&1 &
XOLU_PID=$!

xolu_wait "http://localhost:${SEED_PORT}" 30 "$XOLU_PID" || {
    tail -n 8 "$SEED_TMPDIR/xolu.log" | ui_indent >&2
    ui_die "xolu did not start for loading the data"
}
ui_ok "xolu ready on :$SEED_PORT"

ui_info "Loading ${SEED_LABEL}${ADDITIONS:+ and ${ADDITIONS// /, }} ..."
set +e
# shellcheck disable=SC2086
seeds_load "http://localhost:${SEED_PORT}" $SEED_LABEL $ADDITIONS > "$SEED_TMPDIR/seed.log" 2>&1
SEED_EXIT=$?
set -e
if [[ "$SEED_EXIT" -ne 0 ]]; then
    tail -n 12 "$SEED_TMPDIR/seed.log" | ui_indent >&2
    ui_die "Loading the data failed (exit $SEED_EXIT)"
fi
ui_ok "Data loaded: $(grep -E 'entity +' "$SEED_TMPDIR/seed.log" | head -1 | sed 's/^ *entity *//')"

kill "$XOLU_PID" 2>/dev/null || true
wait "$XOLU_PID" 2>/dev/null || true
unset XOLU_PID
# Brief pause to let SQLite WAL checkpoint complete on disk
sleep 1
rm -f "$SEED_TMPDIR/xolu.log" "$SEED_TMPDIR/seed.log"

# Verify database files exist (xolu may use .db, .sqlite, or other extensions)
DB_COUNT=$(find "$SEED_TMPDIR/xolu" -type f 2>/dev/null | wc -l | tr -d ' ')
[[ "$DB_COUNT" -gt 0 ]] || ui_die "Loading the data produced no database files in $SEED_TMPDIR/xolu"
DB_SIZE=$(du -sh "$SEED_TMPDIR/xolu" | cut -f1)
ui_ok "Database captured: ${DB_COUNT} file(s), ${DB_SIZE}"

# ── Determine sequence number ────────────────────────────────────────────────

PREFIX="seam-demo-${PLAT_SHORT}-seed${SEED_NUM}"
[[ -n "$ONLY" ]] && PREFIX="seam-demo-${PLAT_SHORT}-${ONLY}-only"
SEQ=1
while [[ -d "$DEMO_DIR/${PREFIX}-$(printf '%03d' $SEQ)" ]]; do
    SEQ=$((SEQ + 1))
done
DEMO_NAME="${PREFIX}-$(printf '%03d' $SEQ)"
DEMO_PATH="$DEMO_DIR/$DEMO_NAME"

# ── Package demo ──────────────────────────────────────────────────────────────

ui_step "Packaging: $DEMO_NAME"

mkdir -p "$DEMO_PATH/data"

# Copy binaries
cp "$BUILD_CACHE/$PLATFORM/xolu${EXT}" "$DEMO_PATH/xolu${EXT}"
cp "$BUILD_CACHE/$PLATFORM/seam${EXT}" "$DEMO_PATH/seam${EXT}"
ui_ok "Binaries"

# Copy pre-seeded database (includes .db, .db-wal, .db-shm, or whatever xolu creates)
cp -a "$SEED_TMPDIR/xolu" "$DEMO_PATH/data/xolu"
DEMO_DB_COUNT=$(find "$DEMO_PATH/data/xolu" -type f 2>/dev/null | wc -l | tr -d ' ')
[[ "$DEMO_DB_COUNT" -gt 0 ]] || ui_die "Database copy failed — no files in demo package"
ui_ok "Database (${DEMO_DB_COUNT} file(s))"

# Generate config.json. The modules that are off by default are on where the
# package has data for them (MODULE_CONFIG above), so that all of it shows.
cat > "$DEMO_PATH/config.json" << CFGJSON
{
  "server": {
    "host": "0.0.0.0",
    "port": ${DEMO_SEAM_PORT},
    "read_timeout": "30s",
    "write_timeout": "30s",
    "shutdown_timeout": "10s",
    "max_body_size": 10485760
  },
  "xolu": {
    "url": "http://localhost:${DEMO_XOLU_PORT}",
    "timeout": "30s",
    "max_connections": 100,
    "retry_attempts": 3,
    "retry_delay": "1s"
  },
  "auth": {
    "enabled": true,
    "jwt_secret": "seam-demo-jwt-secret-not-for-production",
    "token_expiry": "24h",
    "refresh_expiry": "168h",
    "session_secret": "seam-demo-session-secret-not-for-production",
    "session_duration": "24h",
    "csrf_enabled": true,
    "tenant_from_claims_only": true
  },
  "features": {
    "enable_ui": true,
    "enable_api": true,
    "enable_webhooks": false,
    "enable_rules": true,
    "enable_fsm": true,
    "enable_qr": true,
    "enable_operational": ${OPERATIONAL},
    "enable_admin": true,
${MODULE_CONFIG}
    "qr_prefix": "SHF",
    "qr_base_url": "http://localhost:${DEMO_SEAM_PORT}/s/",
    "rate_limit_per_min": 0,
    "dev_mode": false
  },
  "log": {
    "level": "info",
    "format": "json",
    "output": "stdout"
  }
}
CFGJSON
ui_ok "config.json"

# seam uses go:embed — no external web/ directory is needed in the package.

# ── Generate launcher ─────────────────────────────────────────────────────────
# The launchers are written from templates with @@NAME@@ marks. fill_template
# puts the values in and refuses a template with a mark it was not given, or a
# value for a mark that is not there, so a launcher never ships half filled.

fill_template() {
    python3 - "$@" <<'PYEOF'
import re
import sys

path, pairs = sys.argv[1], sys.argv[2:]
with open(path, encoding="utf-8") as fh:
    text = fh.read()
for pair in pairs:
    name, _, value = pair.partition("=")
    mark = "@@%s@@" % name
    if mark not in text:
        sys.exit("fill_template: %s has no %s" % (path, mark))
    text = text.replace(mark, value)
left = sorted(set(re.findall(r"@@[A-Z_]+@@", text)))
if left:
    sys.exit("fill_template: %s still has %s" % (path, ", ".join(left)))
with open(path, "w", encoding="utf-8", newline="") as fh:
    fh.write(text)
PYEOF
}

case "$SEED_LANG" in es) LANG_DEFAULT=2 ;; pt) LANG_DEFAULT=3 ;; ja) LANG_DEFAULT=4 ;; *) LANG_DEFAULT=1 ;; esac
DEMO_URL="http://localhost:${DEMO_SEAM_PORT}"

if [[ "$PLAT_SHORT" == "win" ]]; then

    # cmd.exe reads these characters as syntax: a value that has one is escaped.
    cmd_escape() { printf '%s' "$1" | sed -e 's/\^/^^/g' -e 's/&/^\&/g' -e 's/|/^|/g' -e 's/</^</g' -e 's/>/^>/g' -e 's/%/%%/g'; }
    # The file is ASCII (see below): a company name that is not goes by the seed's name.
    CMD_COMPANY="$SEED_COMPANY"
    if printf '%s' "$SEED_COMPANY" | LC_ALL=C grep -q '[^ -~]'; then CMD_COMPANY="$SEED_LABEL"; fi
    ENV_CMD="$(xolu_env_pairs "$DEMO_XOLU_PORT" '%~dp0data\xolu' info | sed 's/^/set /')"

    cat > "$DEMO_PATH/START.cmd" << 'STARTCMD'
@echo off
title Seam AMS Demo
cd /d "%~dp0"

echo.
echo  =============================================
echo   Seam AMS @@VERSION@@ demo
echo   @@COMPANY@@
echo  =============================================
echo.

:: -- how xolu runs: written from scripts/lib/xolu-env.sh ----------------------
@@ENV@@

:: -- start database server ---------------------------------------------------
echo  Starting the database server (xolu)...
start "Seam DB" xolu.exe

set /a XOLU_TRIES=0
:wait_xolu
set /a XOLU_TRIES+=1
if %XOLU_TRIES% gtr 30 (
    echo.
    echo  ERROR: The database server did not start after 30 seconds.
    echo  Check the "Seam DB" window for error messages.
    echo.
    pause
    goto stop_all
)
timeout /t 1 /nobreak > nul 2>&1
curl -sf http://localhost:@@XOLU_PORT@@/health > nul 2>&1
if errorlevel 1 goto wait_xolu

echo  Database ready.
echo.

:: -- select UI language ------------------------------------------------------
echo  Select the language of the interface:
echo    1) English   (en)
echo    2) Espanol   (es)
echo    3) Portugues (pt)
echo    4) Japanese  (ja)
echo.
set "lang_choice="
set /p lang_choice="  Language [1-4, default=@@LANG_DEFAULT@@]: "
if "%lang_choice%"=="" set lang_choice=@@LANG_DEFAULT@@
if "%lang_choice%"=="1" set SEAM_FORCE_LANGUAGE=en
if "%lang_choice%"=="2" set SEAM_FORCE_LANGUAGE=es
if "%lang_choice%"=="3" set SEAM_FORCE_LANGUAGE=pt
if "%lang_choice%"=="4" set SEAM_FORCE_LANGUAGE=ja
if not defined SEAM_FORCE_LANGUAGE (
    echo  '%lang_choice%' is not one of 1 to 4 -- using English ^(en^)
    set SEAM_FORCE_LANGUAGE=en
)

:: -- start application server ------------------------------------------------
echo  Starting Seam AMS...
start "Seam AMS" seam.exe -config config.json

set /a SEAM_TRIES=0
:wait_seam
set /a SEAM_TRIES+=1
if %SEAM_TRIES% gtr 30 (
    echo.
    echo  ERROR: Seam AMS did not start after 30 seconds.
    echo  Check the "Seam AMS" window for error messages.
    echo.
    pause
    goto stop_all
)
timeout /t 1 /nobreak > nul 2>&1
curl -sf @@URL@@/health > nul 2>&1
if errorlevel 1 goto wait_seam

:: -- ready -------------------------------------------------------------------
echo.
echo  =============================================
echo   Seam AMS is running
echo.
echo   Open      @@URL@@
echo   Sign in   @@EMAIL@@
echo   Password  @@PASSWORD@@
echo  =============================================
echo.

echo @@URL@@| clip
echo  The address is in the clipboard.
echo.
echo  Press any key to stop the demo...
pause > nul

:stop_all
echo  Stopping...
taskkill /f /im seam.exe > nul 2>&1
taskkill /f /im xolu.exe > nul 2>&1
echo  Done.
STARTCMD
    fill_template "$DEMO_PATH/START.cmd" \
        "ENV=$ENV_CMD" "VERSION=$SEAM_VERSION" "COMPANY=$(cmd_escape "$CMD_COMPANY")" \
        "XOLU_PORT=$DEMO_XOLU_PORT" "URL=$DEMO_URL" "LANG_DEFAULT=$LANG_DEFAULT" \
        "EMAIL=$(cmd_escape "$LOGIN_EMAIL")" "PASSWORD=$(cmd_escape "$LOGIN_PASSWORD")"
    # cmd.exe wants CRLF line ends, and reads the file in the console's code page: keep it ASCII
    python3 - "$DEMO_PATH/START.cmd" <<'PYEOF'
import sys
path = sys.argv[1]
text = open(path, encoding="utf-8").read().replace("\r\n", "\n")
text = text.encode("ascii", "replace").decode("ascii")
open(path, "w", encoding="ascii", newline="").write(text.replace("\n", "\r\n"))
PYEOF
    ui_ok "START.cmd"

    # Also create a STOP.cmd for safety
    printf '@echo off\r\necho Stopping Seam AMS Demo...\r\ntaskkill /f /im seam.exe > nul 2>&1\r\ntaskkill /f /im xolu.exe > nul 2>&1\r\necho Done.\r\n' > "$DEMO_PATH/STOP.cmd"
    ui_ok "STOP.cmd"

else
    # Linux / macOS. The two boxes are drawn here, at build time, by
    # scripts/lib/ui.py: the target machine has no Python to draw them with.
    ENV_SH="$(xolu_env_pairs "$DEMO_XOLU_PORT" '$DIR/data/xolu' error | sed -e 's/^\([A-Z0-9_]*\)=\(.*\)$/export \1="\2"/')"
    BANNER_BOX="$(SEAM_COLOR=never python3 "$SCRIPT_DIR/lib/ui.py" --width 72 box --title "Seam AMS ${SEAM_VERSION} demo" -- "${SEED_COMPANY}")"
    READY_BOX="$(SEAM_COLOR=never python3 "$SCRIPT_DIR/lib/ui.py" --width 72 box --title "Seam AMS is running" -- \
        "Open${TAB}${DEMO_URL}" "Sign in${TAB}${LOGIN_EMAIL}" "Password${TAB}${LOGIN_PASSWORD}" "---" "Press Ctrl-C to stop the demo.")"

    cat > "$DEMO_PATH/START.sh" << 'STARTSH'
#!/bin/bash
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"

echo ""
cat <<'BOX'
@@BANNER_BOX@@
BOX
echo ""
echo "  Starting the database server (xolu) ..."

# How xolu runs: written from scripts/lib/xolu-env.sh
@@ENV@@

XOLU_PID=""
SEAM_PID=""
cleanup() {
    trap - INT TERM EXIT
    echo ""
    echo "  Stopping ..."
    [ -n "$SEAM_PID" ] && { kill "$SEAM_PID" 2>/dev/null || true; wait "$SEAM_PID" 2>/dev/null || true; }
    [ -n "$XOLU_PID" ] && { kill "$XOLU_PID" 2>/dev/null || true; wait "$XOLU_PID" 2>/dev/null || true; }
    echo "  Done."
}
trap cleanup INT TERM EXIT

./xolu > "$DIR/data/xolu.log" 2>&1 &
XOLU_PID=$!

for i in $(seq 1 60); do
    curl -sf http://localhost:@@XOLU_PORT@@/health >/dev/null 2>&1 && break
    kill -0 "$XOLU_PID" 2>/dev/null || { echo "  ERROR: the database server stopped: see data/xolu.log"; exit 1; }
    sleep 0.5
    [ "$i" -eq 60 ] && { echo "  ERROR: the database server did not start: see data/xolu.log"; exit 1; }
done
echo "  Database ready."

SEAM_LANG=""
if [ -t 0 ]; then
    echo ""
    echo "  Select the language of the interface:"
    echo "    1) English    (en)"
    echo "    2) Español    (es)"
    echo "    3) Português  (pt)"
    echo "    4) 日本語     (ja)"
    echo ""
    read -r -p "  Language [1-4, default=@@LANG_DEFAULT@@]: " lang_choice || lang_choice=""
else
    lang_choice=""
fi
case "${lang_choice:-@@LANG_DEFAULT@@}" in
    1) SEAM_LANG="en" ;;
    2) SEAM_LANG="es" ;;
    3) SEAM_LANG="pt" ;;
    4) SEAM_LANG="ja" ;;
    *) echo "  '$lang_choice' is not one of 1 to 4 -- using English (en)"; SEAM_LANG="en" ;;
esac

echo "  Starting Seam AMS ..."
SEAM_FORCE_LANGUAGE="$SEAM_LANG" ./seam -config config.json > "$DIR/data/seam.log" 2>&1 &
SEAM_PID=$!

for i in $(seq 1 60); do
    curl -sf @@URL@@/health >/dev/null 2>&1 && break
    kill -0 "$SEAM_PID" 2>/dev/null || { echo "  ERROR: Seam stopped: see data/seam.log"; exit 1; }
    sleep 0.5
    [ "$i" -eq 60 ] && { echo "  ERROR: Seam did not start: see data/seam.log"; exit 1; }
done

echo ""
cat <<'BOX'
@@READY_BOX@@
BOX
echo ""

# Copy the address to the clipboard (best effort)
if [ -t 1 ]; then
    printf '%s' "@@URL@@" | pbcopy 2>/dev/null \
        || printf '%s' "@@URL@@" | xclip -selection clipboard 2>/dev/null \
        || printf '%s' "@@URL@@" | xsel --clipboard --input 2>/dev/null \
        || true
fi

wait "$XOLU_PID" "$SEAM_PID"
STARTSH
    fill_template "$DEMO_PATH/START.sh" \
        "ENV=$ENV_SH" "BANNER_BOX=$BANNER_BOX" "READY_BOX=$READY_BOX" \
        "XOLU_PORT=$DEMO_XOLU_PORT" "URL=$DEMO_URL" "LANG_DEFAULT=$LANG_DEFAULT"
    chmod +x "$DEMO_PATH/START.sh"
    bash -n "$DEMO_PATH/START.sh" || ui_die "The START.sh that was written is not valid shell"
    ui_ok "START.sh"
fi

# ── Generate README ───────────────────────────────────────────────────────────

{
    echo "Seam AMS ${SEAM_VERSION} demo"
    echo "========================================"
    echo ""
    echo "Data:     ${SEED_LABEL} (${SEED_COMPANY})${ADDITIONS:+, with ${ADDITIONS// /, }}"
    echo "Modules:  ${MODULES_LINE}"
    echo "Platform: ${PLATFORM}"
    echo "Built:    $(date -u +"%Y-%m-%d %H:%M UTC")"
    echo ""
    echo "HOW TO RUN"
    echo "----------"
    if [[ "$PLAT_SHORT" == "win" ]]; then
        echo "Double-click START.cmd (or run it from a command prompt)."
        echo ""
        echo "The application starts with its data already loaded."
        echo "When it is ready, the address is copied to your clipboard."
        echo ""
        echo "To stop: press any key in the demo window, or double-click STOP.cmd."
    else
        echo "Run:  chmod +x START.sh && ./START.sh"
        echo ""
        echo "The application starts with its data already loaded."
        echo "When it is ready, the address is copied to your clipboard."
        echo "What the two servers print goes to data/xolu.log and data/seam.log."
        echo ""
        echo "To stop: press Ctrl+C in the terminal."
    fi
    echo ""
    echo "SIGN IN"
    echo "-------"
    echo "Address:  ${DEMO_URL}"
    echo "Email:    ${LOGIN_EMAIL}"
    echo "Password: ${LOGIN_PASSWORD}"
    if [[ " $ADDITIONS $SEED_LABEL " == *" accounting "* ]]; then
        echo ""
        echo "Accounting (${DEMO_URL}/accounting) has people of its own:"
        echo "  accounting.staff@example.com / holaquetal   back office"
        echo "  accountant@example.com       / holaquetal   the books"
        echo "  marta.rivas@example.com      / holaquetal   an account executive"
    fi
    echo ""
    echo "PORTS"
    echo "-----"
    echo "Seam AMS:   ${DEMO_URL}"
    echo "xolu (API): http://localhost:${DEMO_XOLU_PORT}"
    echo ""
    echo "CONTENTS"
    echo "--------"
    # shellcheck disable=SC2012
    ls -1 "$DEMO_PATH" | sed 's/^/  /'
} > "$DEMO_PATH/README.txt"

ui_ok "README.txt"

# ── Create distributable zip ─────────────────────────────────────────────────

ZIP_DIR="$DEMO_DIR/zip"
mkdir -p "$ZIP_DIR"
ZIP_PATH="$ZIP_DIR/${DEMO_NAME}.zip"
rm -f "$ZIP_PATH"

(cd "$DEMO_DIR" && zip -r -q "$ZIP_PATH" "$DEMO_NAME/")
ZIP_SIZE=$(du -sh "$ZIP_PATH" | cut -f1)
ui_ok "${DEMO_NAME}.zip (${ZIP_SIZE})"

# ── Summary ───────────────────────────────────────────────────────────────────

DEMO_SIZE=$(du -sh "$DEMO_PATH" | cut -f1)

if [[ "$PLAT_SHORT" == "win" ]]; then
    HOW="Send the zip to the target machine, unzip it, and double-click START.cmd"
else
    HOW="Send the zip to the target machine, unzip it, and run ./START.sh"
fi

echo ""
ui_box --title "Demo package ready" --style ok -- \
    "Directory${TAB}demo/${DEMO_NAME}/" \
    "Zip${TAB}demo/zip/${DEMO_NAME}.zip" \
    "Data${TAB}${SEED_LABEL}: ${SEED_COMPANY}${ADDITIONS:+, with ${ADDITIONS// /, }}" \
    "Modules${TAB}${MODULES_LINE}" \
    "Platform${TAB}${PLATFORM}" \
    "Size${TAB}${DEMO_SIZE} directory, ${ZIP_SIZE} zip" \
    "Sign in${TAB}${LOGIN_EMAIL} / ${LOGIN_PASSWORD}" \
    "---" \
    "$HOW"
echo ""
