#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
# https://www.apache.org/licenses/LICENSE-2.0
"""Seam AMS work orders: three work-order templates and what they need (asset type, assets, unit, consumables, actions).

The data is in seed-wo-templates.seed, a .seed file (docs/SEED_FORMAT.md); this command
loads it with the seed_system package in seeds/.

    python3 seeds/workorders/seed-wo-templates.py [--url http://localhost:9090] [--dry-run]

It is an addition: it loads over any seed, or into an empty instance. Every
record is found by a field of its own and left alone if it exists, so it can
be run again; --replace (or --update-existing) writes the records again.
Python 3.8 or later, standard library only.
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from seed_system.compat import run  # noqa: E402

if __name__ == "__main__":
    sys.exit(run("wo-templates", file="workorders/seed-wo-templates.seed", update_existing=True,
                 description="Seam AMS work orders: three work-order templates and what they need (asset type, assets, unit, consumables, actions)."))
