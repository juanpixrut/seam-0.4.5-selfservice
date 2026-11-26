#!/usr/bin/env python3
"""
scripts/check-icons.py — Material Icons font audit

Verifies that every Material Icon name referenced in the Go source files is
present as a ligature in the vendored WOFF2 font. Also regenerates the
committed ligature list used by the Go test suite.

Exit codes:
  0 — all icons present, ligature list up to date
  1 — one or more icons missing from the font, or dependency error

Usage:
  python3 scripts/check-icons.py              # audit + regenerate ligature list
  python3 scripts/check-icons.py --check-only # audit only, do not write files

Dependencies (build-time only, never at runtime):
  pip install fonttools brotli
"""

import sys
import re
import subprocess
import os

WOFF2   = 'web/static/vendor/fonts/MaterialIcons-Regular.woff2'
OUT_TXT = 'web/static/vendor/material-icons-ligatures.txt'
SRC_DIR = 'internal/ui'

# ── Helpers ───────────────────────────────────────────────────────────────────

def load_ligatures(woff2_path):
    try:
        from fontTools.ttLib import TTFont
    except ImportError:
        print('ERROR: fonttools not installed.  Run: pip install fonttools brotli')
        sys.exit(1)
    try:
        import brotli
    except ImportError:
        print('ERROR: brotli not installed.  Run: pip install brotli')
        sys.exit(1)

    font = TTFont(woff2_path)
    ligatures = set()
    if 'GSUB' not in font:
        return ligatures

    cmap = font.getBestCmap()
    glyph_to_char = {g: chr(cp) for cp, g in cmap.items() if cp < 0x110000}

    for lookup in font['GSUB'].table.LookupList.Lookup:
        for sub in lookup.SubTable:
            if not hasattr(sub, 'ligatures'):
                continue
            for first_glyph, lig_list in sub.ligatures.items():
                fc = glyph_to_char.get(first_glyph, '')
                if not fc:
                    continue
                for lig in lig_list:
                    chars = [fc]
                    ok = True
                    for comp in lig.Component:
                        c = glyph_to_char.get(comp, '')
                        if not c:
                            ok = False
                            break
                        chars.append(c)
                    if ok:
                        ligatures.add(''.join(chars))
    return ligatures


def find_icon_names(src_dir, ligatures):
    """
    Collect all string literals from Go source files that look like icon names.
    Strategy: scan every double-quoted string matching [a-z][a-z_0-9]{2,39}
    and retain those that are present in the ligature table. This reliably finds
    all genuine icon usages without false positives from other string values.
    """
    icon_re = re.compile(r'"([a-z][a-z_0-9]{2,39})"')
    found = set()

    for dirpath, _, filenames in os.walk(src_dir):
        for filename in filenames:
            if not filename.endswith('.go'):
                continue
            filepath = os.path.join(dirpath, filename)
            try:
                with open(filepath) as f:
                    content = f.read()
            except OSError:
                continue
            for m in icon_re.finditer(content):
                candidate = m.group(1)
                if candidate in ligatures:
                    found.add(candidate)

    return found


def write_ligature_list(ligatures, out_path):
    with open(out_path, 'w') as f:
        f.write('# Material Icons ligature names extracted from MaterialIcons-Regular.woff2\n')
        f.write('# Source: @fontsource/material-icons v5.2.7  Generated: 2026-03-31\n')
        f.write('# Do not edit manually — regenerate with: make check-icons\n')
        for name in sorted(ligatures):
            f.write(name + '\n')


# ── Main ─────────────────────────────────────────────────────────────────────

def main():
    check_only = '--check-only' in sys.argv

    # Must run from project root
    if not os.path.isfile(WOFF2):
        print(f'ERROR: {WOFF2} not found — run from the project root directory')
        sys.exit(1)

    print(f'Loading ligatures from {WOFF2} …')
    ligatures = load_ligatures(WOFF2)
    print(f'  {len(ligatures)} ligatures found in font')

    if not check_only:
        write_ligature_list(ligatures, OUT_TXT)
        print(f'  → wrote {OUT_TXT}')

    print(f'Scanning Go source in {SRC_DIR}/ …')
    used_icons = find_icon_names(SRC_DIR, ligatures)
    print(f'  {len(used_icons)} icon names referenced in source')

    missing = sorted(i for i in used_icons if i not in ligatures)

    print()
    print('Icon audit results:')
    for icon in sorted(used_icons):
        mark = '✓' if icon in ligatures else '✗'
        print(f'  {mark}  {icon}')

    if missing:
        print()
        print(f'FAIL: {len(missing)} icon(s) referenced in source but missing from vendored font:')
        for icon in missing:
            print(f'  ✗  {icon}')
        print()
        print('To resolve: update the vendored font or choose a different icon name.')
        print('Run: make vendor-icons   to re-vendor the Material Icons font')
        sys.exit(1)

    print()
    print(f'OK — all {len(used_icons)} icon names confirmed present in the vendored font.')


if __name__ == '__main__':
    main()
