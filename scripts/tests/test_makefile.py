# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
# https://www.apache.org/licenses/LICENSE-2.0
"""The Makefile, scripts/launch.sh and scripts/build-demo.sh agree with each other.

    python3 -m unittest discover -s scripts/tests

`make launch`, `make dev`, `make start-xolu` and `make demo` are four ways to a
running Seam. Each used to carry its own copy of how xolu is started and of
which data is loaded, and the copies drifted: one started xolu without the API
the Issues module needs, another under a name xolu no longer reads. These tests
hold them to one copy. They read files; nothing is started.
"""
import importlib.util
import io
import os
import re
import subprocess
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPTS = os.path.dirname(HERE)
ROOT = os.path.dirname(SCRIPTS)
sys.path.insert(0, os.path.join(SCRIPTS, "lib"))
import ui  # noqa: E402

spec = importlib.util.spec_from_file_location("make_help", os.path.join(SCRIPTS, "make-help.py"))
make_help = importlib.util.module_from_spec(spec)
spec.loader.exec_module(make_help)

ALIASES = {"olu", "start-olu"}          # old names kept as aliases; not listed in the help


def read(*parts):
    with open(os.path.join(ROOT, *parts), encoding="utf-8") as fh:
        return fh.read()


def code(text):
    """A shell script without its comment lines."""
    return "\n".join(l for l in text.split("\n") if not l.lstrip().startswith("#"))


def recipe(makefile, target):
    m = re.search(r"^%s:[^\n]*\n((?:\t[^\n]*\n|\n)*)" % re.escape(target), makefile, re.M)
    return m.group(1) if m else None


class Help(unittest.TestCase):
    def setUp(self):
        self.makefile = read("Makefile")
        self.sections = make_help.read(os.path.join(ROOT, "Makefile"))
        self.described = {name for s in self.sections for name, _ in s["targets"]}

    def phony(self):
        m = re.search(r"^\.PHONY:((?:[^\n]*\\\n)*[^\n]*)", self.makefile, re.M)
        return set(m.group(1).replace("\\", " ").split())

    def test_every_public_target_is_described_and_declared(self):
        phony = self.phony()
        self.assertEqual(phony - ALIASES - self.described, set(), "in .PHONY with no '## description'")
        self.assertEqual(self.described - phony, set(), "described but not in .PHONY")
        for name in phony:
            self.assertIsNotNone(recipe(self.makefile, name), "%s is in .PHONY and has no rule" % name)

    def test_the_help_lists_every_target_at_any_width(self):
        for columns in (40, 60, 80, 100, 110, 132, 200):
            screen = ui.Screen(stream=io.StringIO(), color=False, ascii_only=False, columns=columns)
            lines = make_help.render(self.sections, "Seam AMS", screen)
            text = "\n".join(lines)
            for name in self.described:
                self.assertIn("make " + name, text, (columns, name))
            self.assertTrue(all(ui.width(l) <= columns for l in lines), (columns, max(lines, key=ui.width)))
            body = [l for l in lines if "│" in l]
            self.assertLessEqual(len({ui.width(l) for l in body}), 1, columns)         # two columns: every line the same width
            self.assertEqual(bool(body), columns >= 110, columns)

    def test_make_help_runs(self):
        p = subprocess.run(["make", "-s", "-C", ROOT, "help"], capture_output=True, text=True, encoding="utf-8")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn("make launch", p.stdout)
        self.assertNotIn("\x1b[", p.stdout)                                           # a pipe: no colour


class OneWayToStartXolu(unittest.TestCase):
    def setUp(self):
        self.env = read("scripts", "lib", "xolu-env.sh")
        self.launch = read("scripts", "launch.sh")
        self.demo = read("scripts", "build-demo.sh")
        self.makefile = read("Makefile")

    def pairs(self):
        p = subprocess.run(["bash", "-c", "source '%s/lib/xolu-env.sh'; xolu_env_pairs 9090 /data info" % SCRIPTS], capture_output=True, text=True)
        self.assertEqual(p.returncode, 0, p.stderr)
        return dict(l.split("=", 1) for l in p.stdout.strip().split("\n"))

    def test_the_settings_seam_needs_are_in_the_one_list(self):
        pairs = self.pairs()
        for name, why in (("XOLU_API_V2_ENABLED", "the Issues lifecycle is a state machine, /api/v2/fsm"), ("XOLU_BLOB_ENABLED", "QR images and attachments"),
                          ("XOLU_FULLTEXT_ENABLED", "search"), ("XOLU_TIMESERIES_ENABLED", "readings"), ("XOLU_CASCADING_DELETE", "deleting an asset"),
                          ("XOLU_TENANT_AUTO_REGISTER", "the first write to a tenant")):
            self.assertEqual(pairs.get(name), "true", "%s (%s)" % (name, why))
        self.assertEqual((pairs["XOLU_PORT"], pairs["XOLU_BASE_DIR"], pairs["XOLU_LOG_LEVEL"]), ("9090", "/data", "info"))
        self.assertTrue(all(k.startswith("XOLU_") for k in pairs), "xolu reads XOLU_ names only")

    def test_nothing_else_sets_xolu_up(self):
        setting = re.compile(r"\b(?:export |set )?X?OLU_(?!PORT\b|PATH\b|LOG\b|BIN\b|DIR\b|PID\b|REPO|SOURCE\b|VERSION\b|URL|ONLY\b|TRIES\b|LATEST|PLATFORM|PREV|REF|RELEASE|ASSET|TMP)[A-Z0-9_]+=")
        for name, text in (("scripts/launch.sh", self.launch), ("scripts/build-demo.sh", self.demo), ("Makefile", self.makefile)):
            found = [l.strip() for l in code(text).split("\n") if setting.search(l)]
            self.assertEqual(found, [], "%s sets xolu up on its own; scripts/lib/xolu-env.sh is the place" % name)
        for name, text in (("scripts/launch.sh", self.launch), ("scripts/build-demo.sh", self.demo)):
            self.assertIn('source "$SCRIPT_DIR/lib/xolu-env.sh"', text, name)
            self.assertIn("xolu_env_export", code(text), name)
        # the two launchers a demo package carries are written from the same list
        self.assertEqual(code(self.demo).count("xolu_env_pairs"), 2)

    def test_docker_compose_starts_xolu_the_same_way_and_names_what_seam_reads(self):
        compose = read("docker-compose.yml")
        seam_block, xolu_block = compose.split("\n  xolu:\n")
        given = dict(re.findall(r"^\s+- (XOLU_[A-Z0-9_]+)=(\S+)", xolu_block, re.M))
        for name, value in self.pairs().items():
            if name in ("XOLU_PORT", "XOLU_BASE_DIR", "XOLU_NO_ASCII"):      # the container's own port and volume; a banner
                continue
            self.assertEqual(given.get(name), value, "docker-compose.yml: " + name)
        self.assertEqual(set(given) - set(self.pairs()), set(), "docker-compose.yml sets what scripts/lib/xolu-env.sh does not")
        config = read("internal", "config", "config.go")
        for name in re.findall(r"^\s+- ([A-Z][A-Z0-9_]+)=", seam_block, re.M):
            self.assertIn('os.Getenv("%s")' % name, config, "docker-compose.yml gives Seam %s, which it does not read" % name)

    def test_every_xolu_target_goes_through_the_launcher(self):
        for target in ("launch", "launch-fast", "launch-dev", "dev", "xolu", "start-xolu"):
            self.assertIn("./scripts/launch.sh", recipe(self.makefile, target), target)
        self.assertEqual(recipe(self.makefile, "olu").strip(), "")
        self.assertRegex(self.makefile, r"(?m)^olu: xolu$")
        self.assertRegex(self.makefile, r"(?m)^start-olu: start-xolu$")

    def test_names_that_nothing_reads_are_gone(self):
        self.assertNotIn("SEAM_OLU_URL", self.makefile)
        self.assertNotRegex(self.makefile, r"\bOLU_(PORT|DIR|DATA|BASE_DIR|STORAGE_TYPE)\b")
        admin = read("cmd", "seam-admin", "main.go")
        self.assertIn('os.Getenv("SEAM_XOLU_URL")', admin)
        users = [t for t in ("admin-create", "user-create", "user-list", "user-disable", "user-enable", "user-set-role", "user-set-password", "user-delete")]
        for target in users:
            self.assertIn("SEAM_XOLU_URL=$(XOLU_URL)", recipe(self.makefile, target), target)


class TheSameSystem(unittest.TestCase):
    """What `make dev` starts and what a demo package holds: the same modules over the same data."""

    def setUp(self):
        self.launch = read("scripts", "launch.sh")
        self.demo = read("scripts", "build-demo.sh")
        self.makefile = read("Makefile")
        self.seeds = read("scripts", "lib", "seeds.sh")

    def test_both_load_data_the_same_way(self):
        for name, text in (("scripts/launch.sh", self.launch), ("scripts/build-demo.sh", self.demo)):
            self.assertIn('source "$SCRIPT_DIR/lib/seeds.sh"', text, name)
            self.assertIn("seeds_load ", code(text), name)
            self.assertIn("seeds_additions ", code(text), name)
            self.assertNotRegex(code(text), r"seeds/seed-\S*\.py", name + " runs a seed script of its own")

    def test_the_additions_exist_and_their_modules_are_on(self):
        p = subprocess.run(["bash", "-c", "PROJECT_DIR='%s'; source '%s/lib/seeds.sh'; seeds_additions 01-constructec; seeds_datasets" % (ROOT, SCRIPTS)],
                           capture_output=True, text=True, encoding="utf-8")
        self.assertEqual(p.returncode, 0, p.stderr)
        lines = p.stdout.strip().split("\n")
        self.assertEqual(lines[0], "wo-templates issues-01-constructec accounting")
        datasets = [l.split("\t") for l in lines[1:]]
        self.assertEqual(len(datasets), 7)
        for name, company, lang, email, password in datasets:
            self.assertTrue(company and email and password, name)
            self.assertIn(lang, ("en", "es", "pt", "ja"), name)
            self.assertTrue(os.path.isfile(os.path.join(ROOT, "seeds", "issues", "seed-issues-%s.seed" % name)), name)
        # work orders, issues and accounting are off by default: where their data is loaded, they are switched on
        dev = recipe(self.makefile, "dev")
        for var in ("SEAM_ENABLE_WORK_ORDERS=1", "SEAM_ENABLE_ISSUES=1", "SEAM_ENABLE_ACCOUNTING=1", "SEAM_ENABLE_DEVELOPER=1"):
            self.assertIn(var, dev)
        self.assertIn("--seed", dev)
        # the package's configuration is written from the one list of modules, not by hand
        self.assertIn("${MODULE_CONFIG}", self.demo)
        self.assertNotRegex(self.demo, r'"enable_(work_orders|issues|accounting)": (true|false)')
        catalog = read("internal", "features", "catalog.go")
        for var in re.findall(r"\b(SEAM_[A-Z_]+)=", dev):
            self.assertIn('"%s"' % var, catalog + read("internal", "config", "config.go"), "%s is not a variable Seam reads" % var)

    def test_the_same_languages_everywhere(self):
        locales = sorted(f[:-5] for f in os.listdir(os.path.join(ROOT, "internal", "i18n", "locales")) if f.endswith(".json"))
        self.assertEqual(locales, ["en", "es", "ja", "pt"])
        self.assertIn('LANGUAGES="en es pt ja"', self.launch)
        self.assertIn('""|en|es|pt|ja)', self.launch)
        for code_ in locales:
            self.assertIn("%s" % code_, re.search(r"ui_menu lang_choice[^\n]*", self.launch).group(0))
            self.assertRegex(self.demo, r'set SEAM_FORCE_LANGUAGE=%s\b' % code_)             # START.cmd
            self.assertRegex(self.demo, r'SEAM_LANG="%s"' % code_)                           # START.sh

    def test_a_later_launch_can_change_what_the_first_one_wrote(self):
        # config.json is written once; ports and dev mode reach Seam through its environment
        start = self.launch[self.launch.index("# ── Start Seam"):]
        for line in ('export SEAM_PORT="$SEAM_PORT"', 'export XOLU_URL="$XOLU_URL_LOCAL"', 'export SEAM_FEATURES="$seam_features"'):
            self.assertIn(line, start)
        config = read("internal", "config", "config.go")
        for var in ("SEAM_PORT", "XOLU_URL", "SEAM_FORCE_LANGUAGE", "SEAM_ACCOUNTING_DOCS_DIR"):
            self.assertIn('"%s"' % var, config, var)
        self.assertIn('"dev_mode"', read("internal", "features", "catalog.go"))

    def test_options_in_the_help_are_the_options_parsed(self):
        for name, text in (("scripts/launch.sh", self.launch), ("scripts/build-demo.sh", self.demo)):
            header = text[:text.index("set -euo pipefail")]
            documented = set(re.findall(r"^#   (--[a-z-]+)", header, re.M))
            parsed = set(re.findall(r"^\s+(--[a-z-]+)\)", text, re.M))
            self.assertEqual(documented, parsed, name)


class OneWayToSwitchModulesOn(unittest.TestCase):
    """Work Orders, Issues and Accounting are off by default and switched on in one manner everywhere."""

    def sh(self, script, **env):
        p = subprocess.run(["bash", "-c", "PROJECT_DIR='%s'; source '%s/lib/seeds.sh'; source '%s/lib/modules.sh'; %s" % (ROOT, SCRIPTS, SCRIPTS, script)],
                           capture_output=True, text=True, encoding="utf-8", env=dict(os.environ, **env))
        return p.returncode, p.stdout.strip(), p.stderr.strip()

    def table(self):
        rc, out, err = self.sh("modules_table")
        self.assertEqual(rc, 0, err)
        return [line.split("\t") for line in out.split("\n")]

    def test_the_list_is_the_catalogue(self):
        catalog = read("internal", "features", "catalog.go")
        keys = dict(re.findall(r'^\s+(\w+)\s+= "(\w+)"$', catalog, re.M))
        table = self.table()
        self.assertEqual([row[0] for row in table], ["work_orders", "issues", "accounting"])
        for mid, title, var, key in table:
            const = [c for c, v in keys.items() if v == mid]
            self.assertEqual(len(const), 1, mid)
            flag = re.search(r'\{Key: %s, EnvVar: "(\w+)", ConfigKey: "(\w+)",([^\n]*)' % const[0], catalog)
            self.assertIsNotNone(flag, mid)
            self.assertEqual((flag.group(1), flag.group(2)), (var, key), mid)
            self.assertNotIn("Default: true", flag.group(3), mid + " is on by default: it does not belong in this list")
            self.assertIn('json:"%s"' % key, read("internal", "config", "config.go"), mid)
            self.assertIn('"%s": false' % key, read("config.example.json"), mid)

    def test_every_addition_says_which_module_it_is_for(self):
        ids = [row[0] for row in self.table()]
        rc, out, err = self.sh("seeds_modules wo-templates; seeds_modules wo-actions; seeds_modules issues-03-extinfire; seeds_modules accounting; "
                               "seeds_modules 01-constructec; seeds_modules 01-constructec $(seeds_additions 01-constructec)")
        self.assertEqual(rc, 0, err)
        self.assertEqual(out.split("\n"), ["work_orders", "work_orders", "issues", "accounting", "", "work_orders issues accounting"])
        self.assertEqual(out.split("\n")[-1].split(), ids)
        # and says so to whoever loads it by hand
        for path, var in (("workorders/seed-wo-templates.seed", "SEAM_ENABLE_WORK_ORDERS=1"), ("workorders/seed-wo-actions.seed", "SEAM_ENABLE_WORK_ORDERS=1"),
                          ("issues/seed-issues-01-constructec.seed", "SEAM_ENABLE_ISSUES=1"), ("accounting/seed-accounting.seed", "SEAM_ENABLE_ACCOUNTING=1")):
            self.assertIn(var, read("seeds", *path.split("/"))[:4000], path)

    def test_choosing_modules(self):
        self.assertEqual(self.sh("modules_resolve all")[1], "work_orders issues accounting")
        self.assertEqual(self.sh("modules_resolve none")[:2], (0, ""))
        self.assertEqual(self.sh("modules_resolve issues,work_orders,issues")[1], "issues work_orders")
        rc, out, err = self.sh("modules_resolve issues,tickets")
        self.assertEqual((rc, out), (1, ""))
        self.assertIn("tickets is not a module", err)
        self.assertEqual(self.sh("modules_titles work_orders issues")[1], "Work Orders, Issues")

    def test_what_the_caller_set_is_kept(self):
        env = {k: v for k, v in os.environ.items() if not k.startswith("SEAM_ENABLE_")}
        script = "PROJECT_DIR='%s'; source '%s/lib/modules.sh'; modules_export work_orders issues; echo \"$SEAM_ENABLE_WORK_ORDERS/$SEAM_ENABLE_ISSUES/${SEAM_ENABLE_ACCOUNTING-unset}\"; modules_on" % (ROOT, SCRIPTS)
        p = subprocess.run(["bash", "-c", script], capture_output=True, text=True, env=dict(env, SEAM_ENABLE_ISSUES="0"))
        self.assertEqual(p.stdout.split("\n")[:2], ["1/0/unset", "work_orders"], p.stderr)

    def test_the_launcher_and_the_demo_decide_the_same_way(self):
        launch, demo, makefile = read("scripts", "launch.sh"), read("scripts", "build-demo.sh"), read("Makefile")
        for name, text in (("scripts/launch.sh", launch), ("scripts/build-demo.sh", demo)):
            self.assertIn('source "$SCRIPT_DIR/lib/modules.sh"', text, name)
            self.assertIn("seeds_modules ", code(text), name)
            self.assertIn("modules_resolve ", code(text), name)
            # no module is named by its variable in code: the list is the only place
            self.assertNotRegex(code(text), r"SEAM_ENABLE_(WORK_ORDERS|ISSUES|ACCOUNTING)\b", name)
        # the launcher exports before Seam starts, and keeps the choice with the data
        self.assertLess(launch.index("modules_export $modules_now"), launch.index("# ── Start Seam"))
        self.assertIn('MODULES_MARKER="$DATA_DIR/.modules"', launch)
        for target in ("launch", "launch-fast", "launch-dev"):
            self.assertIn("$(LAUNCH_ARGS)", recipe(makefile, target), target)
        self.assertIn("--modules $(MODULES)", makefile[makefile.index("LAUNCH_ARGS ="):].split("\n")[0])
        self.assertIn("--modules $(MODULES)", recipe(makefile, "demo"))
        dev = recipe(makefile, "dev")
        for _, _, var, _ in self.table():
            self.assertIn(var + "=1", dev, "make dev leaves %s off" % var)


class OneModuleAsTheWholeSystem(unittest.TestCase):
    """make launch-accounting and make demo-accounting: Seam without the Assets module, with Accounting."""

    def sh(self, script):
        p = subprocess.run(["bash", "-c", "PROJECT_DIR='%s'; source '%s/lib/seeds.sh'; source '%s/lib/modules.sh'; %s" % (ROOT, SCRIPTS, SCRIPTS, script)],
                           capture_output=True, text=True, encoding="utf-8")
        self.assertEqual(p.returncode, 0, p.stderr)
        return p.stdout.rstrip("\n")

    def test_which_modules_and_their_data(self):
        self.assertEqual(self.sh("modules_standalone"), "accounting")
        self.assertIn("accounting", self.sh("modules_ids").split())
        # its data loads over nothing, and brings the account to sign in with
        self.assertEqual(self.sh("seeds_standalone accounting"), "accounting")
        self.assertEqual(self.sh("seeds_standalone issues"), "")
        email, password, role = self.sh("seeds_login accounting").split("\t")
        self.assertTrue(email and password)
        self.assertEqual(role, "admin_staff")

    def test_the_targets(self):
        makefile = read("Makefile")
        launch = recipe(makefile, "launch-accounting")
        self.assertIn("./scripts/launch.sh --only accounting", launch)
        self.assertIn('--data-dir "$(ACCOUNTING_DATA_DIR)"', launch)
        self.assertRegex(makefile, r"(?m)^ACCOUNTING_DATA_DIR \?= \$\(CURDIR\)/data/")
        self.assertIn("./scripts/build-demo.sh --only accounting", recipe(makefile, "demo-accounting"))

    def test_a_developer_can_be_each_role(self):
        # the seed has the developer account of the other seeds, last, so the first login stays the back office's
        seed = read("seeds", "accounting", "seed-accounting.seed")
        header = seed[seed.index('{"seed"'):seed.index('"x": {"seam"') + 1200]
        self.assertRegex(header, r'"logins": \[\{"email": "accounting\.staff@example\.com"')
        self.assertIn('{"email": "dev@ual.li", "password": "holaquetal", "role": "developer"}', header)
        for role in ("admin_staff", "accountant", "account_executive"):
            self.assertIn('"role": "%s"' % role, header)
        # and that account is an AE with a customer, for the portal
        self.assertIn('"name": "Dev (as an AE)"', seed)
        self.assertIn('"name": "Sandbox Demo Co."', seed)
        makefile = read("Makefile")
        dev = recipe(makefile, "dev-accounting")
        self.assertIn("SEAM_ENABLE_DEVELOPER=1", dev)
        self.assertIn('./scripts/launch.sh --only accounting --data-dir "$(ACCOUNTING_DATA_DIR)"', dev)
        catalog = read("internal", "features", "catalog.go") + read("internal", "config", "config.go")
        for var in re.findall(r"\b(SEAM_[A-Z_]+)=", dev):
            self.assertIn('"%s"' % var, catalog, "%s is not a variable Seam reads" % var)

    def test_assets_is_switched_off_by_the_flag_seam_has(self):
        catalog = read("internal", "features", "catalog.go")
        self.assertIn('EnvVar: "SEAM_ENABLE_OPERATIONAL", ConfigKey: "enable_operational"', catalog)
        launch, demo = read("scripts", "launch.sh"), read("scripts", "build-demo.sh")
        start = launch[launch.index("# ── Start Seam"):]
        self.assertIn('[[ -n "$ONLY" ]] && export SEAM_ENABLE_OPERATIONAL=0', start)
        self.assertIn('"enable_operational": ${OPERATIONAL},', demo)
        self.assertIn("OPERATIONAL=false", demo)
        for name, text in (("scripts/launch.sh", launch), ("scripts/build-demo.sh", demo)):
            self.assertIn("modules_standalone", code(text), name)
            self.assertIn("seeds_standalone ", code(text), name)
        # a data directory started this way stays this way, and is not shared with a whole instance
        self.assertIn('ONLY_MARKER="$DATA_DIR/.only"', launch)
        self.assertIn("needs a data directory of its own", launch)

    def test_the_launcher_refuses_what_cannot_be(self):
        for args, want in ((["--only", "issues"], "cannot be the whole system"), (["--only", "accounting", "--seed", "01-constructec"], "use one or the other"),
                           (["--only", "accounting", "--modules", "issues"], "do not go together")):
            p = subprocess.run(["bash", os.path.join(SCRIPTS, "launch.sh"), "--no-start"] + args, capture_output=True, text=True, encoding="utf-8",
                               env=dict(os.environ, SEAM_COLOR="never"))
            self.assertNotEqual(p.returncode, 0, args)
            self.assertIn(want, p.stdout + p.stderr, args)


class NothingDrawnByHand(unittest.TestCase):
    def test_scripts_draw_through_the_library(self):
        drawn = re.compile(r"[┌┐└┘├┤│╔╗╚╝║]")
        for name in ("scripts/launch.sh", "scripts/build-demo.sh", "scripts/bundle.sh", "Makefile"):
            text = read(*name.split("/"))
            bad = [l for l in code(text).split("\n") if drawn.search(l)]
            self.assertEqual(bad, [], "%s draws a box by hand; use ui_box (scripts/lib/ui.sh)" % name)
        for name in ("scripts/launch.sh", "scripts/build-demo.sh"):
            text = code(read(*name.split("/")))
            self.assertNotRegex(text, r"\\033\[|\\e\[", name + " writes colour codes itself")
            self.assertNotRegex(text, r"printf '?\"?%-\d+s", name + " pads with printf, which counts bytes")

    def test_scripts_are_valid_shell(self):
        for name in ("launch.sh", "build-demo.sh", "bundle.sh", "lib/ui.sh", "lib/xolu-env.sh", "lib/seeds.sh"):
            p = subprocess.run(["bash", "-n", os.path.join(SCRIPTS, name)], capture_output=True, text=True)
            self.assertEqual(p.returncode, 0, name + ": " + p.stderr)

    def test_scripts_work_with_the_bash_of_macos(self):
        # bash 3.2: no associative arrays, no ${var,,}, no mapfile, no &>>, no negative indices
        for name in ("launch.sh", "build-demo.sh", "lib/ui.sh", "lib/xolu-env.sh", "lib/seeds.sh"):
            text = code(read("scripts", *name.split("/")))
            for pattern, what in ((r"declare -A|local -A", "associative arrays"), (r"\$\{[A-Za-z_]+(,,|\^\^)", "case conversion"), (r"\bmapfile\b|\breadarray\b", "mapfile"),
                                  (r"&>>", "&>>"), (r"\$\{[A-Za-z_]+\[-1\]\}", "negative index"), (r"\bcoproc\b", "coproc")):
                self.assertNotRegex(text, pattern, "%s uses %s, which bash 3.2 does not have" % (name, what))


if __name__ == "__main__":
    unittest.main()
