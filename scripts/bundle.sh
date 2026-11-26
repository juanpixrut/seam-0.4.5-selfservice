#!/usr/bin/env bash
# scripts/bundle.sh — Create a distributable launch zip
#
# Produces seam-ams-VERSION-bundle.zip containing the full seam source
# tree plus a top-level launch.sh that delegates to scripts/launch.sh.
# xolu is NOT included — the launch script clones it at runtime.
#
# Usage:
#   ./scripts/bundle.sh [--output-dir DIR]
#   make bundle
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

# shellcheck source=scripts/lib/ui.sh
source "$SCRIPT_DIR/lib/ui.sh"

OUTPUT_DIR="$PROJECT_DIR"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --output-dir) [[ $# -ge 2 ]] || ui_die "--output-dir requires a value"; OUTPUT_DIR="$2"; shift 2 ;;
        *) ui_die "Unknown option: $1" ;;
    esac
done

command -v zip >/dev/null 2>&1 || ui_die "'zip' is required but not found."
command -v python3 >/dev/null 2>&1 || ui_die "'python3' is required but not found."

VERSION="$(cat VERSION)"
BUNDLE_NAME="seam-ams-${VERSION}-bundle.zip"
BUNDLE_PATH="$OUTPUT_DIR/$BUNDLE_NAME"

echo ""
ui_info "Creating ${BUNDLE_NAME} ..."
rm -f "$BUNDLE_PATH"

# Build zip from the project tree, excluding build artefacts and local state:
# binaries, data, the xolu checkout, demo packages and their build cache,
# Python's caches, and the edit journal of the working copy.
zip -r "$BUNDLE_PATH" . \
    --exclude ".git/*" \
    --exclude ".git" \
    --exclude "bin/*" \
    --exclude "data/*" \
    --exclude "xolu/*" \
    --exclude "demo/*" \
    --exclude "*/__pycache__/*" \
    --exclude "*.pyc" \
    --exclude ".ed-journal.json" \
    --exclude ".DS_Store" \
    --exclude "*/.DS_Store" \
    --exclude "*.zip" \
    --exclude "*.sha256" \
    --exclude "coverage.out" \
    -q

# Inject a top-level launch.sh wrapper that delegates to scripts/launch.sh.
# Written as a heredoc into a temp file to avoid any escaping issues.
WRAPPER=$(mktemp)
cat > "$WRAPPER" << 'WRAPPER_SCRIPT'
#!/usr/bin/env bash
# launch.sh — Seam AMS launcher
# Delegates to scripts/launch.sh in the unpacked seam source tree.
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
exec "$DIR/scripts/launch.sh" "$@"
WRAPPER_SCRIPT
chmod +x "$WRAPPER"

# Inject wrapper into the zip as launch.sh at the root
python3 - "$BUNDLE_PATH" "$WRAPPER" << 'PYEOF'
import sys, zipfile, os

bundle_path = sys.argv[1]
wrapper_path = sys.argv[2]
tmp_path = bundle_path + ".tmp"

wrapper_bytes = open(wrapper_path, "rb").read()

with zipfile.ZipFile(bundle_path, "r") as zin, \
     zipfile.ZipFile(tmp_path, "w", zipfile.ZIP_DEFLATED) as zout:
    for item in zin.infolist():
        zout.writestr(item, zin.read(item.filename))
    info = zipfile.ZipInfo("launch.sh")
    info.external_attr = 0o100755 << 16          # a regular file that can be run
    info.compress_type = zipfile.ZIP_DEFLATED
    zout.writestr(info, wrapper_bytes)

os.replace(tmp_path, bundle_path)
PYEOF

rm -f "$WRAPPER"

SIZE=$(du -sh "$BUNDLE_PATH" | cut -f1)
echo ""
ui_box --title "Bundle ready" --style ok -- \
    "File"$'\t'"${BUNDLE_PATH}" \
    "Size"$'\t'"${SIZE}" \
    "---" \
    "The recipient unzips it and runs ./launch.sh (Go, git, curl and python3 are needed there)."
