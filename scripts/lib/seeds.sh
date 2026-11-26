#!/usr/bin/env bash
# scripts/lib/seeds.sh — the demonstration data, for the scripts that load it
#
# Source it after setting PROJECT_DIR; do not run it. scripts/launch.sh and
# scripts/build-demo.sh both load data through these, so a system started by
# one has what a system built by the other has.
#
#   seeds_datasets                 the whole datasets, one per line:
#                                  name<TAB>company<TAB>language<TAB>email<TAB>password
#   seeds_field NAME N             field N (2 to 5) of that line for one dataset
#   seeds_additions NAME           the additions that go with a dataset, in load
#                                  order: work orders, its Issues file, accounting
#   seeds_modules NAME...          the modules of Seam those seeds hold data for
#                                  (x.seam.module in each header), on one line:
#                                  what must be on for the data to show
#   seeds_standalone MODULE        the additions for that module that need no
#                                  dataset under them, on one line
#   seeds_login NAME               the first account a seed creates:
#                                  email<TAB>password<TAB>role
#   seeds_load URL NAME...         loads them (python3 seeds/seed.py load)
#
# The data itself and its format: seeds/ and docs/SEED_FORMAT.md.
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

SEEDS_DIR="${PROJECT_DIR:?PROJECT_DIR must be set before sourcing seeds.sh}/seeds"

seeds_datasets() {
    python3 - "$SEEDS_DIR" <<'PY'
import glob, os, sys
sys.path.insert(0, sys.argv[1])
from seed_system import read_seed
for path in sorted(glob.glob(os.path.join(sys.argv[1], "seed-*.seed"))):
    header = read_seed(path)[0]
    if not header.get("exclusive"):
        continue
    seam = header.get("x", {}).get("seam", {})
    logins = seam.get("logins", [])
    admins = [l for l in logins if l.get("role") == "admin"] or logins or [{}]
    tags = header.get("tags") or [""]
    print("\t".join([header["name"], seam.get("tenant_name", ""), tags[-1], admins[0].get("email", ""), admins[0].get("password", "")]))
PY
}

seeds_field() {
    seeds_datasets | awk -F '\t' -v name="$1" -v n="$2" '$1 == name { print $n }'
}

seeds_additions() {
    local name="$1" out=""
    [ -f "$SEEDS_DIR/workorders/seed-wo-templates.seed" ] && out="$out wo-templates"
    [ -f "$SEEDS_DIR/issues/seed-issues-$name.seed" ] && out="$out issues-$name"
    [ -f "$SEEDS_DIR/accounting/seed-accounting.seed" ] && out="$out accounting"
    echo "${out# }"
}

seeds_modules() {
    python3 - "$SEEDS_DIR" "$@" <<'PY'
import glob, os, sys
sys.path.insert(0, sys.argv[1])
from seed_system import read_seed
wanted, found = sys.argv[2:], {}
for path in glob.glob(os.path.join(sys.argv[1], "**", "seed-*.seed"), recursive=True):
    header = read_seed(path)[0]
    if header.get("name") in wanted:
        found[header["name"]] = ((header.get("x") or {}).get("seam") or {}).get("module", "")
out = []
for name in wanted:
    if found.get(name) and found[name] not in out:
        out.append(found[name])
print(" ".join(out))
PY
}

seeds_standalone() {
    python3 - "$SEEDS_DIR" "$1" <<'PY'
import glob, os, sys
sys.path.insert(0, sys.argv[1])
from seed_system import read_seed
out = []
for path in sorted(glob.glob(os.path.join(sys.argv[1], "**", "seed-*.seed"), recursive=True)):
    header = read_seed(path)[0]
    module = ((header.get("x") or {}).get("seam") or {}).get("module", "")
    if header.get("addition") and module == sys.argv[2] and not header.get("requires"):
        out.append(header["name"])
print(" ".join(out))
PY
}

seeds_login() {
    python3 - "$SEEDS_DIR" "$1" <<'PY'
import glob, os, sys
sys.path.insert(0, sys.argv[1])
from seed_system import read_seed
for path in glob.glob(os.path.join(sys.argv[1], "**", "seed-*.seed"), recursive=True):
    header = read_seed(path)[0]
    if header.get("name") == sys.argv[2]:
        logins = ((header.get("x") or {}).get("seam") or {}).get("logins") or []
        if logins:
            print("\t".join([logins[0].get("email", ""), logins[0].get("password", ""), logins[0].get("role", "")]))
PY
}

seeds_load() {
    local url="$1"; shift
    python3 "$SEEDS_DIR/seed.py" load "$@" --url "$url"
}
