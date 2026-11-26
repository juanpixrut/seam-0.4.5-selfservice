#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
# https://www.apache.org/licenses/LICENSE-2.0
"""Seam AMS seed 01: Constructec S.A. (construction machinery and facilities, Uruguay).

The data is in seed-01-constructec.seed, a .seed file (docs/SEED_FORMAT.md); this
command loads it with the seed_system package beside it.

    python3 seeds/seed-01-constructec.py [--url http://localhost:9090] [--dry-run] [--schemas]

It loads into an empty instance. Run again, it leaves the records that exist
alone and loads the rest; --replace writes them again; --check compares and
writes nothing. Python 3.8 or later, standard library only.
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from seed_system.compat import run  # noqa: E402

if __name__ == "__main__":
    sys.exit(run("01-constructec", schemas=True, description="Seam AMS seed 01: Constructec S.A. (construction machinery and facilities, Uruguay)."))
