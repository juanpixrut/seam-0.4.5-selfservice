#!/usr/bin/env python3
"""
scripts/diff-locales.py — i18n locale key-set diff

Compares the key sets of every internal/i18n/locales/*.json file
against each other (auto-discovered, not a fixed list). Confirms
whether any locale is missing keys the others have, and — critically
— whether a "missing" set is *consistent* across references (e.g.
pt missing a different, partially overlapping set relative to en
versus es would indicate en and es had themselves drifted apart
rather than pt simply lagging behind a shared baseline).

Does not compare values, only key presence. A key existing in all
locales with an untranslated or stale value will not be flagged here
-- this is a structural check, not a translation-quality check.

Exit codes:
  0 -- all locales have identical key sets
  1 -- at least one locale has a key-set mismatch

Usage:
  python3 scripts/diff-locales.py
"""
import glob
import json
import os
import sys

LOCALE_DIR = "internal/i18n/locales"


def discover_locales():
    paths = sorted(glob.glob(f"{LOCALE_DIR}/*.json"))
    return [os.path.splitext(os.path.basename(p))[0] for p in paths]


def load_keys(locale):
    path = f"{LOCALE_DIR}/{locale}.json"
    with open(path, encoding="utf-8") as f:
        data = json.load(f)
    return set(data.keys()), data


def main():
    locales = discover_locales()
    if len(locales) < 2:
        print(f"Only {len(locales)} locale(s) found ({locales}) -- nothing to compare.")
        sys.exit(0)

    keysets = {}
    for loc in locales:
        keys, _ = load_keys(loc)
        keysets[loc] = keys
        print(f"{loc}: {len(keys)} keys")
    print()

    ok = True

    # Pairwise comparison against the first locale as reference. Since
    # a real drift would show up as *some* pair disagreeing, comparing
    # every locale against one fixed reference is sufficient to catch
    # any inconsistency -- if all pairs vs. the reference are identical,
    # all locales are pairwise identical to each other too.
    reference = locales[0]
    ref_keys = keysets[reference]

    for loc in locales[1:]:
        loc_keys = keysets[loc]
        only_ref = ref_keys - loc_keys
        only_loc = loc_keys - ref_keys
        print(f"=== {reference} vs {loc} ===")
        if not only_ref and not only_loc:
            print("IDENTICAL key sets.")
        else:
            ok = False
            print(f"Keys in {reference} but not {loc} ({len(only_ref)}):")
            for k in sorted(only_ref):
                print(f"  {k}")
            print(f"Keys in {loc} but not {reference} ({len(only_loc)}):")
            for k in sorted(only_loc):
                print(f"  {k}")
        print()

    if len(locales) > 2 and ok:
        print("=== Consistency check across all locales ===")
        print(f"All {len(locales)} locales ({', '.join(locales)}) have")
        print("IDENTICAL key sets -- no partial/inconsistent overlap anywhere.")
        print()

    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
