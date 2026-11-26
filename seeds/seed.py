#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
# https://www.apache.org/licenses/LICENSE-2.0
"""seed.py -- load, check and dump .seed files for xolu.

    python3 seeds/seed.py list
    python3 seeds/seed.py load 01-constructec --url http://localhost:9090
    python3 seeds/seed.py check 01-constructec --url http://localhost:9090
    python3 seeds/seed.py dump --url http://localhost:9090 -o my.seed

Python 3.8 or later, standard library only. The format is in docs/SEED_FORMAT.md;
the code is the seed_system package beside this file.
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from seed_system.cli import main  # noqa: E402

if __name__ == "__main__":
    sys.exit(main())
