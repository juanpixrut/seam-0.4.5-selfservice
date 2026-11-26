#!/usr/bin/env bash
# release.sh — seam release hygiene automation
#
# Single-pass release preparation:
#   1.  Validates version string format
#   2.  Dirty tree warning (git)
#   3.  Checks CHANGELOG.md has a ## [X.Y.Z] entry for this version
#   4.  Syncs VERSION file and derived artefacts via syncver.sh
#   5.  Go compile check
#   6.  Go test suite
#   7.  Consistency check (VERSION == CHANGELOG.md top entry)
#   8.  Cuts a source-only checkpoint zip (no binaries)
#
# Usage:
#   ./release.sh <version>             e.g. ./release.sh 0.2.0
#   ./release.sh <version> --no-zip    dry run, no checkpoint
#   ./release.sh <version> --no-build  skip go build check
#
# Before running, ensure:
#   - CHANGELOG.md has a '## [X.Y.Z] - YYYY-MM-DD' entry at the top.
#   - All intended changes are committed (dirty tree produces a warning,
#     not an error, but the checkpoint will not correspond to a clean commit).
#
# The checkpoint zip is SOURCE-ONLY. It never contains compiled binaries.
# Build the project from source using: make build
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

CUT_ZIP=true
DO_BUILD=true
VERSION=""

for arg in "$@"; do
    case "$arg" in
        --no-zip)   CUT_ZIP=false ;;
        --no-build) DO_BUILD=false ;;
        --help|-h)
            sed -n '3,22p' "$0" | sed 's/^# \?//'
            exit 0
            ;;
        --*) echo "Unknown option: $arg" >&2; exit 1 ;;
        *)
            if [ -z "$VERSION" ]; then VERSION="$arg"
            else echo "Unexpected argument: $arg" >&2; exit 1; fi
            ;;
    esac
done

[ -z "$VERSION" ] && { echo "Usage: $0 <version> [--no-zip] [--no-build]" >&2; exit 1; }

step() { echo ""; echo "-- $1"; }
ok()   { echo "   OK $1"; }
warn() { echo "   !! $1"; }
fail() { echo "   FAIL $1" >&2; exit 1; }

# ---------------------------------------------------------------------------
# 1. Validate version format
# ---------------------------------------------------------------------------
step "Version: $VERSION"
echo "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9][a-zA-Z0-9.-]*)?$' \
    || fail "Invalid version format. Expected X.Y.Z or X.Y.Z-suffix (no leading 'v')"
ok "Format valid"

# ---------------------------------------------------------------------------
# 2. Dirty tree warning
# ---------------------------------------------------------------------------
if git rev-parse --git-dir > /dev/null 2>&1; then
    if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
        warn "Working tree has uncommitted changes — checkpoint will not correspond to a clean commit"
    else
        GIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
        ok "Git tree clean at $GIT_SHA"
    fi
else
    warn "Not a git repository — skipping tree check"
fi

# ---------------------------------------------------------------------------
# 3. Check CHANGELOG.md has a versioned entry
# ---------------------------------------------------------------------------
step "Checking CHANGELOG.md"
if grep -q "^## \[${VERSION}\]" CHANGELOG.md; then
    ok "Entry found: ## [${VERSION}]"
else
    fail "No '## [${VERSION}]' entry in CHANGELOG.md. Add one before releasing."
fi

# ---------------------------------------------------------------------------
# 4. Sync VERSION file and derived artefacts
# ---------------------------------------------------------------------------
step "Syncing VERSION and derived artefacts"
./syncver.sh set "$VERSION"
ok "VERSION = $VERSION"

# ---------------------------------------------------------------------------
# 3a. Shell script linting (shellcheck)
# ---------------------------------------------------------------------------
step "Shell script lint"
if ! command -v shellcheck > /dev/null 2>&1; then
    warn "shellcheck not found — skipping (install: apt install shellcheck)"
else
    shellcheck --format=gcc release.sh syncver.sh scripts/launch.sh scripts/bundle.sh
    ok "All shell scripts pass shellcheck"
fi

# ---------------------------------------------------------------------------
# 4b. Tailwind CSS build (Node, build-time only — zero runtime dependency)
# ---------------------------------------------------------------------------
step "Tailwind CSS build"
if ! command -v node > /dev/null 2>&1; then
    fail "Node.js is required at build time (tailwind CSS generation)"
fi
# Always write canonical tailwind.config.js — idempotent, safe to overwrite.
# Written unconditionally so it is guaranteed to exist for both the Tailwind
# build step below and the zip step later in this script.
cat > tailwind.config.js << 'TWEOF'
/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    './internal/**/*.go',
    './web/static/js/*.js',
    './web/templates/**/*.html',
  ],
  theme: { extend: {} },
  plugins: [],
}
TWEOF
# Install from lockfile — reproducible, pinned tree (package-lock.json must be committed)
npm ci --prefer-offline 2>&1 | sed 's/^/  /'
npx tailwindcss \
    -c tailwind.config.js \
    -i scripts/tailwind.input.css \
    -o web/static/css/tailwind.css \
    --minify 2>&1 | sed 's/^/  /'
ok "Tailwind CSS → web/static/css/tailwind.css"

# ---------------------------------------------------------------------------
# 4c. Icon audit — verify all referenced icons are in the vendored font
# ---------------------------------------------------------------------------
step "Icon audit"
if ! command -v python3 > /dev/null 2>&1; then
    warn "python3 not found — skipping icon audit (run: make check-icons manually)"
elif ! python3 -c "import fonttools, brotli" > /dev/null 2>&1; then
    warn "fonttools/brotli not installed — skipping icon audit (run: pip install fonttools brotli)"
else
    python3 scripts/check-icons.py --check-only 2>&1 | sed 's/^/  /'
    [ "${PIPESTATUS[0]}" -eq 0 ] || fail "Icon audit failed — run: make check-icons"
    ok "All icon names confirmed in vendored font"
fi

# ---------------------------------------------------------------------------
# 5. Go compile check
# ---------------------------------------------------------------------------
step "Go compile check"
if ! $DO_BUILD; then
    warn "Skipping (--no-build)"
elif ! command -v go > /dev/null 2>&1; then
    warn "go not found on PATH — compile check skipped"
else
    GO_VERSION=$(go version | awk '{print $3}')
    ok "Using $GO_VERSION"
    if (go build ./... 2>&1); then
        ok "Builds clean"
        # Remove any binaries that go build may have deposited at the root.
        find . -maxdepth 1 -type f ! -name "*.go" ! -name "*.mod" ! -name "*.sum" \
            ! -name "*.sh" ! -name "*.md" ! -name "*.yaml" ! -name "*.yml" \
            ! -name "*.json" ! -name "*.txt" ! -name "VERSION" ! -name "LICENSE" \
            ! -name "Makefile" \
            -exec rm -f {} \; 2>/dev/null || true
        # JS syntax check — catches missing method declarations, unbalanced braces, etc.
        for jsfile in web/static/js/seam-fsm-editor.js web/static/js/fsm-editor.js; do
            if [ -f "$jsfile" ]; then
                if node --check "$jsfile" 2>&1; then
                    ok "JS syntax OK: $jsfile"
                else
                    fail "JS syntax error in $jsfile — fix before releasing"
                fi
            fi
        done
    else
        fail "Build failed — fix compilation errors before releasing"
    fi
fi

# ---------------------------------------------------------------------------
# 6. Run test suite
# ---------------------------------------------------------------------------
step "Running tests"
if ! $DO_BUILD; then
    warn "Skipping (--no-build)"
elif ! command -v go > /dev/null 2>&1; then
    warn "go not found on PATH — tests skipped"
else
    if go test ./... 2>&1; then
        ok "All tests passed"
    else
        fail "Test suite failed — fix failures before releasing"
    fi
fi
# The seed loader and the scripts have tests of their own, in Python: the
# launch scripts and the Makefile are part of what a release ships.
if ! command -v python3 > /dev/null 2>&1; then
    warn "python3 not found — seed and script tests skipped (run: make test-seeds test-scripts)"
else
    if python3 -m unittest discover -s seeds/tests 2>&1 && python3 -m unittest discover -s scripts/tests 2>&1; then
        ok "Seed and script tests passed"
    else
        fail "Seed or script tests failed — run: make test-seeds test-scripts"
    fi
fi

# ---------------------------------------------------------------------------
# 7. Consistency check
# ---------------------------------------------------------------------------
step "Consistency check"
./syncver.sh check
ok "All version strings consistent: $VERSION"

# ---------------------------------------------------------------------------
# 8. Cut source-only checkpoint zip
# ---------------------------------------------------------------------------
if $CUT_ZIP; then
    step "Cutting checkpoint"
    ZIPNAME="seam-v${VERSION}-checkpoint.zip"
    rm -f "$ZIPNAME"

    # Explicit source list — never zip '.' to avoid sweeping in build
    # artefacts, editor swap files, or local config. Add new top-level
    # items here when they are committed to the repository.
    ZIP_SOURCES=(
        VERSION
        go.mod
        go.sum
        README.md
        CHANGELOG.md
        ROADMAP.md
        DEPLOY.md
        LICENSE
        Makefile
        Dockerfile
        docker-compose.yml
        config.example.json
        package.json
        package-lock.json
        tailwind.config.js
        syncver.sh
        release.sh
        scripts/
        seeds/
        api/
        cmd/
        docs/
        internal/
        pkg/
        web/
        attic/
        .repoman.json
    )

    # Filter to items that actually exist (some may be absent in early releases).
    EXISTING_SOURCES=()
    for src in "${ZIP_SOURCES[@]}"; do
        [ -e "$src" ] && EXISTING_SOURCES+=("$src")
    done

    # Write MANIFEST.txt — records version, date, git commit, key asset checksums.
    # Written to the project root, zipped in, then deleted.
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unavailable")
    NODE_VER=$(node --version 2>/dev/null || echo "unavailable")
    GO_VER=$(go version | awk '{print $3}')
    {
        echo "Seam AMS Checkpoint Manifest"
        echo "=============================="
        echo "Version:      $VERSION"
        echo "Build date:   $(date -u +'%Y-%m-%d %H:%M:%S UTC')"
        echo "Git commit:   $GIT_COMMIT"
        echo "Go version:   $GO_VER"
        echo "Node version: $NODE_VER"
        echo ""
        echo "Key asset SHA-256:"
        sha256sum web/static/css/tailwind.css \
                  web/static/vendor/fonts/MaterialIcons-Regular.woff2 \
                  web/static/vendor/htmx@1.9.10.min.js 2>/dev/null
    } > MANIFEST.txt

    zip -X -r "$ZIPNAME" "${EXISTING_SOURCES[@]}" \
        -x "*.env" \
        -x ".env" \
        -x "*.db" -x "*.db-shm" -x "*.db-wal" -x "*.db-journal" \
        -x "*.tmp" \
        -x "*.so" -x "*.dylib" -x "*.dll" -x "*.exe" -x "*.a" -x "*.o" \
        -x "bin/*" \
        -x "coverage.out" -x "coverage.html" \
        -x "*__pycache__*" -x "*.pyc" \
        > /dev/null 2>&1

    # Inject MANIFEST.txt into the zip, then clean up the loose copy
    zip "$ZIPNAME" MANIFEST.txt > /dev/null 2>&1
    rm -f MANIFEST.txt

    # Inject tailwind.config.js — written fresh here so it is guaranteed
    # present in the zip even if npm ci removed it from the working tree.
    cat > tailwind.config.js << 'TWEOF'
/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    './internal/**/*.go',
    './web/static/js/*.js',
    './web/templates/**/*.html',
  ],
  theme: { extend: {} },
  plugins: [],
}
TWEOF
    zip "$ZIPNAME" tailwind.config.js > /dev/null 2>&1
    rm -f tailwind.config.js

    # --- Post-zip sanity checks ---

    # 1. No .env files.
    ENV_COUNT=$(unzip -l "$ZIPNAME" | grep -cE '(^|/)\.env$' || true)
    [ "$ENV_COUNT" -gt 0 ] && fail "Checkpoint contains .env file(s) — secrets would be exposed"

    # 2. No database files.
    DB_COUNT=$(unzip -l "$ZIPNAME" | grep -cE '\.db$|\.db-shm$|\.db-wal$' || true)
    [ "$DB_COUNT" -gt 0 ] && fail "Checkpoint contains $DB_COUNT .db file(s)"

    # 3. No compiled binaries (ELF, Mach-O, PE).
    if command -v python3 > /dev/null 2>&1; then
        BINARY_FILES=$(python3 - "$ZIPNAME" <<'PYEOF'
import sys, zipfile
MAGIC = {
    b'\x7fELF': 'ELF',
    b'\xca\xfe\xba\xbe': 'Mach-O',
    b'\xfe\xed\xfa\xce': 'Mach-O',
    b'\xfe\xed\xfa\xcf': 'Mach-O',
    b'MZ\x90\x00': 'PE/MZ',
}
hits = []
with zipfile.ZipFile(sys.argv[1]) as zf:
    for name in zf.namelist():
        if name.endswith('/'):
            continue
        with zf.open(name) as f:
            header = f.read(4)
        for magic, label in MAGIC.items():
            if header.startswith(magic):
                hits.append(f"  {label}: {name}")
for h in hits:
    print(h)
PYEOF
        )
    elif command -v od > /dev/null 2>&1; then
        BINARY_FILES=""
        while IFS= read -r entry; do
            magic=$(unzip -p "$ZIPNAME" "$entry" 2>/dev/null | head -c 4 | od -A n -t x1 -N 4 2>/dev/null | tr -d ' \n' || true)
            case "$magic" in
                7f454c46)  BINARY_FILES="$BINARY_FILES\n  ELF:    $entry" ;;
                cafebabe)  BINARY_FILES="$BINARY_FILES\n  Mach-O: $entry" ;;
                feedface)  BINARY_FILES="$BINARY_FILES\n  Mach-O: $entry" ;;
                feedfacf)  BINARY_FILES="$BINARY_FILES\n  Mach-O: $entry" ;;
                4d5a*)     BINARY_FILES="$BINARY_FILES\n  PE/MZ:  $entry" ;;
            esac
        done < <(unzip -l "$ZIPNAME" | awk 'NR>3 && /[^/]$/ {print $NF}' | head -300)
    else
        fail "Binary guard requires python3 or od — neither found on PATH"
    fi

    if [ -n "$BINARY_FILES" ]; then
        printf "   Detected binary files in checkpoint:%b\n" "$BINARY_FILES" >&2
        fail "Checkpoint contains binary files — aborting"
    fi
    ok "No binaries detected"

    # 4. Size guard: source-only checkpoints should be well under 3 MB.
    ZIP_BYTES=$(wc -c < "$ZIPNAME")
    if [ "$ZIP_BYTES" -gt 3145728 ]; then
        ZIP_MB=$(awk "BEGIN {printf \"%.1f\", $ZIP_BYTES/1048576}")
        warn "Checkpoint is ${ZIP_MB} MB — exceeds 3 MB ceiling for source-only archive; review contents"
    fi

    ZIP_SIZE=$(du -sh "$ZIPNAME" | cut -f1)
    ok "Created: ${ZIPNAME} (${ZIP_SIZE})"

    # 5. SHA256 checksum companion file.
    CHECKSUMFILE="${ZIPNAME%.zip}.sha256"
    if command -v sha256sum > /dev/null 2>&1; then
        sha256sum "$ZIPNAME" > "$CHECKSUMFILE"
        ok "Checksum: ${CHECKSUMFILE}"
    elif command -v shasum > /dev/null 2>&1; then
        shasum -a 256 "$ZIPNAME" > "$CHECKSUMFILE"
        ok "Checksum: ${CHECKSUMFILE}"
    else
        warn "sha256sum and shasum not found — checksum file not generated"
        CHECKSUMFILE=""
    fi

else
    warn "Skipping zip (--no-zip)"
fi

echo ""
echo "======================================"
echo "  Release v${VERSION} prepared"
if $CUT_ZIP; then
    echo "  Zip:      seam-v${VERSION}-checkpoint.zip"
    [ -n "${CHECKSUMFILE:-}" ] && echo "  Checksum: ${CHECKSUMFILE}"
fi
echo "======================================"
echo ""
echo "Next steps:"
echo "  git add VERSION CHANGELOG.md README.md internal/version/version.go"
echo "  git commit -m \"release: ${VERSION}\""
echo "  git tag v${VERSION}"
echo "  git push && git push --tags"
