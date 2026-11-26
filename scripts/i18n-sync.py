#!/usr/bin/env python3
"""
scripts/i18n-sync.py -- add and check translations across every locale file.

Adding a key means touching every internal/i18n/locales/*.json. This tool does
it in one step and keeps the files consistent (same keys, same order, same
formatting as the existing files: 2-space indent, UTF-8, trailing newline).

Commands
  check                     Every locale has the same keys (exit 1 if not) and
                            no value still carries the TODO marker (reported,
                            exit code unaffected).
  new KEY [KEY...]          Print a JSON skeleton for the keys, one entry per
                            locale, to fill in and feed to `add`.
  add FILE.json             Add the keys in FILE to every locale. FILE maps a
                            key to its text per locale:
                              {"demo.title": {"en": "Demo", "es": "Demo",
                                              "pt": "Demo", "ja": "..."}}
                            A key that already exists is left alone (use
                            --overwrite to replace it). A locale with no text
                            for a key stops the command unless --allow-missing,
                            which fills it with the English text behind the
                            TODO marker.
  scaffold                  Add to every locale the keys it lacks, copied from
                            the reference locale behind the TODO marker.
  todo                      List every value that still carries the marker.

Options
  --dir DIR        locale directory (default internal/i18n/locales)
  --ref LOCALE     reference locale for scaffold / --allow-missing (default en)
  --marker TEXT    TODO marker (default "[TODO] ")
  --dry-run        show what would change, write nothing

Typical flow:  scripts/i18n-sync.py new my.key.one my.key.two > new.json
               (fill in new.json)   scripts/i18n-sync.py add new.json
               scripts/i18n-sync.py check
"""
import argparse
import glob
import json
import os
import sys


def load(dirpath):
    out = {}
    for path in sorted(glob.glob(os.path.join(dirpath, "*.json"))):
        loc = os.path.splitext(os.path.basename(path))[0]
        with open(path, encoding="utf-8") as f:
            out[loc] = (path, json.load(f))
    return out


def save(path, data):
    with open(path, "w", encoding="utf-8") as f:
        f.write(json.dumps(data, indent=2, ensure_ascii=False) + "\n")


def cmd_check(locs, a):
    keysets = {l: set(d) for l, (_, d) in locs.items()}
    union = set().union(*keysets.values())
    ok = True
    for l in locs:
        missing = sorted(union - keysets[l])
        print(f"{l}: {len(keysets[l])} keys" + (f", missing {len(missing)}" if missing else ""))
        for k in missing:
            print(f"   missing: {k}")
            ok = False
    todo = [(l, k) for l, (_, d) in locs.items() for k, v in d.items()
            if isinstance(v, str) and v.startswith(a.marker)]
    if todo:
        print(f"\n{len(todo)} value(s) still carry the TODO marker (run `todo` to list them)")
    print("\nOK: all locales have the same keys" if ok else "\nFAIL: key sets differ")
    return 0 if ok else 1


def cmd_new(locs, a):
    skeleton = {k: {l: "" for l in locs} for k in a.args}
    print(json.dumps(skeleton, indent=2, ensure_ascii=False))
    return 0


def cmd_add(locs, a):
    if len(a.args) != 1:
        sys.exit("add: expected one FILE.json")
    with open(a.args[0], encoding="utf-8") as f:
        new = json.load(f)
    if a.ref not in locs:
        sys.exit(f"reference locale {a.ref!r} not found in {sorted(locs)}")
    changes = {l: {} for l in locs}
    for key, texts in new.items():
        if not isinstance(texts, dict):
            sys.exit(f"{key}: expected an object with one text per locale")
        for l, (_, data) in locs.items():
            if key in data and not a.overwrite:
                print(f"skip {l}: {key} (exists)")
                continue
            text = texts.get(l, "")
            if not text:
                if not a.allow_missing:
                    sys.exit(f"{key}: no text for locale {l!r} (use --allow-missing to fill it from {a.ref})")
                text = a.marker + (texts.get(a.ref) or "")
            changes[l][key] = text
    return apply(locs, changes, a)


def cmd_scaffold(locs, a):
    if a.ref not in locs:
        sys.exit(f"reference locale {a.ref!r} not found in {sorted(locs)}")
    ref = locs[a.ref][1]
    changes = {l: {k: a.marker + str(v) for k, v in ref.items() if k not in data}
               for l, (_, data) in locs.items() if l != a.ref}
    changes[a.ref] = {}
    return apply(locs, changes, a)


def cmd_todo(locs, a):
    n = 0
    for l, (_, d) in locs.items():
        for k, v in d.items():
            if isinstance(v, str) and v.startswith(a.marker):
                print(f"{l}\t{k}\t{v[len(a.marker):]}")
                n += 1
    print(f"\n{n} pending", file=sys.stderr)
    return 0


def apply(locs, changes, a):
    total = 0
    for l, add in changes.items():
        if not add:
            continue
        path, data = locs[l]
        for k, v in add.items():
            data[k] = v  # an existing key keeps its position; a new one goes last
        total += len(add)
        print(f"{l}: {'would write' if a.dry_run else 'wrote'} {len(add)} key(s)")
        if not a.dry_run:
            save(path, data)
    if total == 0:
        print("nothing to do")
    return 0


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("command", choices=["check", "new", "add", "scaffold", "todo"])
    p.add_argument("args", nargs="*")
    p.add_argument("--dir", default="internal/i18n/locales")
    p.add_argument("--ref", default="en")
    p.add_argument("--marker", default="[TODO] ")
    p.add_argument("--overwrite", action="store_true")
    p.add_argument("--allow-missing", action="store_true")
    p.add_argument("--dry-run", action="store_true")
    a = p.parse_args()
    locs = load(a.dir)
    if len(locs) < 2:
        sys.exit(f"found {len(locs)} locale file(s) in {a.dir}; nothing to sync")
    sys.exit({"check": cmd_check, "new": cmd_new, "add": cmd_add,
              "scaffold": cmd_scaffold, "todo": cmd_todo}[a.command](locs, a))


if __name__ == "__main__":
    main()
