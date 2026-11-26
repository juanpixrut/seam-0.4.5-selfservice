# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
# https://www.apache.org/licenses/LICENSE-2.0
"""The command each seed has always had: python3 seeds/seed-NN-name.py --url URL [--dry-run].

The seed itself is the .seed file of the same name; this turns the old options
into a load of it, so that every script and habit that calls the seed by its
old name keeps working.
"""

import argparse
import os

from .cli import main


def run(name, schemas=False, description=None, argv=None, file=None, update_existing=False):
    """Load the seed `name`: seeds/seed-NAME.seed, or `file`, a path relative to the seeds directory."""
    here = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    path = os.path.join(here, file or "seed-%s.seed" % name)
    ap = argparse.ArgumentParser(description=description or "Load the seed %s (%s)." % (name, os.path.basename(path)),
                                 epilog="Everything else (check, dump, other options): python3 seeds/seed.py --help")
    ap.add_argument("--url", default="http://localhost:9090", help="xolu's address (default: http://localhost:9090)")
    ap.add_argument("--dry-run", action="store_true", help="print the requests that would be made; contact nothing")
    if schemas:
        ap.add_argument("--schemas", action="store_true", help="also register the seed's entity schemas, so that xolu validates every record")
    ap.add_argument("--replace", action="store_true", help="write again the records that already exist (default: leave them alone)")
    if update_existing:
        ap.add_argument("--update-existing", dest="replace", action="store_true", help="the same as --replace")
    ap.add_argument("--allow-existing", action="store_true", help="load into an instance that holds other data")
    ap.add_argument("--as-of", metavar="WHEN", help="reference day or time for the seed's relative dates (default: now)")
    ap.add_argument("--check", action="store_true", help="compare the instance with the seed; write nothing")
    args = ap.parse_args(argv)

    out = ["check" if args.check else "load"]
    if schemas and args.schemas and not args.check:
        out.append(os.path.join(here, "seed-%s.schemas.seed" % name))    # schemas run first whatever the order
    out += [path, "--url", args.url]
    if args.as_of:
        out += ["--as-of", args.as_of]
    if not args.check:
        for flag in ("dry_run", "replace", "allow_existing"):
            if getattr(args, flag):
                out.append("--" + flag.replace("_", "-"))
    return main(out)
