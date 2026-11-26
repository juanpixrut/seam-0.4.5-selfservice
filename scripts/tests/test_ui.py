# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
# https://www.apache.org/licenses/LICENSE-2.0
"""The invariants of scripts/lib/ui.py and scripts/lib/ui.sh (they are listed at the top of ui.py).

    python3 -m unittest discover -s scripts/tests

No server, no network. The property tests draw boxes, tables and columns from
text made to be awkward (accents, box characters, Japanese, colour codes, words
longer than the screen) at many widths, and hold every result to the same rule.
"""
import io
import os
import random
import subprocess
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
LIB = os.path.join(os.path.dirname(HERE), "lib")
sys.path.insert(0, LIB)
import ui  # noqa: E402

WORDS = ["Seam", "xolu", "http://localhost:8080/api/v1/", "Constructec S.A.", "朝日建機株式会社", "日本語", "Español", "Português", "Suárez",
         "Suárez", "─│┌┐", "✓", "!", "\x1b[32mgreen\x1b[0m", "\x1b[1;31mred\x1b[0m", "/a/very/long/path/that/has/no/space/in/it/at/all/and/goes/on/and/on",
         "ｆｕｌｌ", "a", "", "  indented", "demo/seam-demo-linux-seed03-001/", "--flag", "---x", "100%", "$HOME", "`tick`", "\"quoted\"", "tab\tbed"]


def screen(columns=80, color=False, ascii_only=False):
    return ui.Screen(stream=io.StringIO(), color=color, ascii_only=ascii_only, columns=columns)


def text(rng, n):
    return " ".join(rng.choice(WORDS) for _ in range(rng.randint(0, n)))


class Width(unittest.TestCase):
    def test_columns_not_bytes_or_characters(self):
        for s, want in (("", 0), ("abc", 3), ("─│┌", 3), ("日本語", 6), ("Suárez", 6), ("Suárez", 6), ("\x1b[1;32mok\x1b[0m", 2), ("ｆｕｌｌ", 8), ("✓ done", 6)):
            self.assertEqual(ui.width(s), want, repr(s))

    def test_pad_truncate_wrap(self):
        self.assertEqual(ui.pad("日本", 6), "日本  ")
        self.assertEqual(ui.pad("ab", 5, "right"), "   ab")
        self.assertEqual(ui.width(ui.pad("\x1b[31mx\x1b[0m", 4)), 4)
        self.assertEqual(ui.truncate("abcdef", 4), "abc…")
        self.assertEqual(ui.truncate("日本語です", 5), "日本…")
        self.assertEqual(ui.truncate("abc", 3), "abc")
        rng = random.Random(7)
        for _ in range(400):
            s, n = text(rng, 8), rng.randint(1, 40)
            for line in ui.wrap(s, n):
                # only a single character wider than the room can stick out
                self.assertTrue(ui.width(line) <= n or ui.width(line) <= 2, (s, n, line))
            self.assertLessEqual(ui.width(ui.truncate(s.replace("\t", " "), n)), max(n, 1), (s, n))

    def test_wrap_keeps_every_character(self):
        s = "one two three /a/long/unbreakable/token/that/exceeds four"
        self.assertEqual("".join(ui.wrap(s, 12)).replace(" ", ""), s.replace(" ", ""))


class Blocks(unittest.TestCase):
    def check(self, lines, s, what):
        widths = {ui.width(l) for l in lines}
        self.assertEqual(len(widths), 1, "%s: lines of %s columns\n%s" % (what, sorted(widths), "\n".join(lines)))
        self.assertLessEqual(widths.pop(), s.columns, what)

    def test_boxes_stay_aligned_whatever_the_text(self):
        rng = random.Random(1)
        for i in range(600):
            s = screen(columns=rng.choice((24, 30, 40, 60, 80, 100, 132, 200)), color=rng.random() < 0.5, ascii_only=rng.random() < 0.3)
            lines = []
            for _ in range(rng.randint(0, 7)):
                kind = rng.random()
                lines.append("" if kind < 0.1 else "---" if kind < 0.2 else text(rng, 3).replace("\t", " ") + "\t" + text(rng, 6) if kind < 0.6 else text(rng, 10).replace("\t", " "))
            title = text(rng, 4).replace("\t", " ") if rng.random() < 0.7 else None
            out = ui.box(lines, title=title, style=rng.choice(sorted(ui.STYLES)), screen=s, max_width=rng.choice((None, 50, 72)))
            self.check(out, s, "box %d" % i)

    def test_tables_stay_aligned_whatever_the_text(self):
        rng = random.Random(2)
        for i in range(400):
            s = screen(columns=rng.choice((40, 80, 120)), color=rng.random() < 0.5, ascii_only=rng.random() < 0.3)
            n = rng.randint(1, 5)
            rows = [[text(rng, 3).replace("\t", " ") for _ in range(rng.randint(1, n))] for _ in range(rng.randint(1, 6))]
            header = [rng.choice(WORDS).replace("\t", " ") for _ in range(n)] if rng.random() < 0.6 else None
            out = ui.table(rows, header=header, screen=s, align="".join(rng.choice("lr") for _ in range(n)))
            widths = {ui.width(l) for l in out}
            self.assertEqual(len(widths), 1, "table %d\n%s" % (i, "\n".join(out)))

    def test_columns_side_by_side_or_stacked(self):
        left, right = ["one", "日本語の行", "three"], ["a", "b"]
        wide = ui.columns(left, right, screen=screen(80))
        self.assertEqual(len(wide), 3)
        self.assertEqual(len({ui.width(l) for l in wide}), 1)
        self.assertIn("│", wide[0])
        narrow = ui.columns(left, ["x" * 30], screen=screen(24))
        self.assertEqual(narrow, left + [""] + ["x" * 30])          # too narrow: one under the other, nothing cut

    def test_a_box_is_no_wider_than_asked_and_wraps_instead_of_breaking(self):
        long = "/a/very/long/path/" + "x" * 300
        for columns in (24, 40, 80):
            out = ui.box(["Path\t" + long, long], title=long, screen=screen(columns))
            self.assertTrue(all(ui.width(l) <= columns for l in out), columns)
            self.assertEqual(len({ui.width(l) for l in out}), 1)
            self.assertEqual("".join(ui.strip(l) for l in out).count("x"), 900)       # all of it is there, three times

    def test_colour_is_decoration_only(self):
        lines = ["Open\thttp://localhost:8080", "", "---", "Press Ctrl-C"]
        plain = ui.box(lines, title="Seam", style="ok", screen=screen(color=False))
        coloured = ui.box(lines, title="Seam", style="ok", screen=screen(color=True))
        self.assertEqual([ui.strip(l) for l in coloured], plain)
        self.assertTrue(any("\x1b[" in l for l in coloured))
        self.assertFalse(any("\x1b[" in l for l in plain))

    def test_red_is_for_errors_only(self):
        s = screen(color=True)
        red = ui.CODES["red"]
        for kind in ("ok", "warn", "info", "step"):
            self.assertNotIn(red, ui.status(kind, "text", s), kind)
        for style in ("plain", "ok", "warn", "info"):
            self.assertNotIn(red, "".join(ui.box(["a\tb"], title="t", style=style, screen=s)), style)
        self.assertIn(red, ui.status("error", "text", s))
        self.assertIn(red, "".join(ui.box(["a"], style="error", screen=s)))
        self.assertNotIn(red, ui.rule("Title", s) + "".join(ui.table([["a", "b"]], header=["x", "y"], screen=s)))

    def test_ascii_mode_draws_with_ascii_only(self):
        s = screen(ascii_only=True)
        drawn = "\n".join(ui.box(["key\tvalue", "---", "text"], title="Title", screen=s) + ui.table([["a", "b"]], header=["x", "y"], screen=s)
                          + [ui.rule("Title", s)] + [ui.status(k, "text", s) for k in ("ok", "warn", "error", "info", "step")] + ui.columns(["l"], ["r"], screen=s))
        self.assertTrue(all(ord(ch) < 128 for ch in drawn), [ch for ch in drawn if ord(ch) > 127])
        self.assertEqual(ui.truncate("abcdef", 5, s.g["ellipsis"]), "ab...")

    def test_misalignment_is_refused(self):
        with self.assertRaises(ui.Misaligned):
            ui.aligned(["abc", "日本語"])
        self.assertEqual(ui.aligned(["abcdef", "日本語"]), ["abcdef", "日本語"])

    def test_where_colour_and_unicode_come_from(self):
        class Tty(io.StringIO):
            encoding = "utf-8"

            def isatty(self):
                return True
        self.assertTrue(ui.Screen(stream=Tty(), env={}).color)
        self.assertFalse(ui.Screen(stream=Tty(), env={"NO_COLOR": ""}).color)
        self.assertFalse(ui.Screen(stream=Tty(), env={"TERM": "dumb"}).color)
        self.assertFalse(ui.Screen(stream=io.StringIO(), env={}).color)                  # a pipe
        self.assertTrue(ui.Screen(stream=io.StringIO(), env={"SEAM_COLOR": "always"}).color)
        self.assertFalse(ui.Screen(stream=Tty(), env={"SEAM_COLOR": "never"}).color)
        self.assertFalse(ui.Screen(stream=Tty(), env={}).ascii)
        self.assertTrue(ui.Screen(stream=Tty(), env={"SEAM_ASCII": "1"}).ascii)
        self.assertEqual(ui.Screen(stream=Tty(), env={"SEAM_COLUMNS": "50"}).columns, 50)
        self.assertEqual(ui.Screen(stream=io.StringIO(), env={}).columns, ui.DEFAULT_WIDTH)


class CommandLine(unittest.TestCase):
    def run_ui(self, *args, env=None, stdin=None):
        e = dict(os.environ, SEAM_COLOR="never", PYTHONIOENCODING="utf-8")
        e.pop("SEAM_ASCII", None)
        e.update(env or {})
        p = subprocess.run([sys.executable, os.path.join(LIB, "ui.py"), *args], capture_output=True, text=True, encoding="utf-8", env=e, input=stdin)
        self.assertEqual(p.returncode, 0, p.stderr)
        return p.stdout.rstrip("\n").split("\n")

    def test_box_table_rule_status(self):
        out = self.run_ui("box", "--title", "Seam AMS", "--style", "ok", "--", "Open\thttp://localhost:8080", "---", "言語\t日本語", "--not-an-option")
        self.assertEqual(len({ui.width(l) for l in out}), 1)
        self.assertIn("--not-an-option", "\n".join(out))
        self.assertEqual(out[0].strip()[0], "┌")
        out = self.run_ui("--ascii", "table", "--header", "Seed\tCompany", "--", "06\t朝日建機", "01\tConstructec")
        self.assertEqual(len({ui.width(l) for l in out}), 1)
        self.assertEqual(self.run_ui("width", "日本語"), ["6"])
        self.assertTrue(self.run_ui("rule", "Title")[0].startswith("── Title "))
        self.assertEqual(self.run_ui("status", "ok", "done"), ["  ✓ done"])
        out = self.run_ui("box", "--", stdin="one\ntwo\tthree\n")                # no lines given: read from standard input
        self.assertEqual([l.strip("│ ") for l in out[1:3]], ["one", "two  three"])
        self.assertEqual(len({ui.width(l) for l in out}), 1)

    def test_width_option_limits_a_box(self):
        out = self.run_ui("--width", "40", "box", "--", "word " * 40)
        self.assertTrue(all(ui.width(l) <= 40 for l in out))


class Shell(unittest.TestCase):
    """scripts/lib/ui.sh, through bash."""

    def bash(self, script, env=None, stdin=""):
        e = dict(os.environ, LC_ALL="C.UTF-8", SEAM_COLOR="never")
        e.pop("SEAM_ASCII", None)
        e.pop("NO_COLOR", None)
        e.update(env or {})
        p = subprocess.run(["bash", "-c", "set -euo pipefail; source '%s/ui.sh'; %s" % (LIB, script)], capture_output=True, text=True, encoding="utf-8", env=e, input=stdin)
        return p.returncode, p.stdout, p.stderr

    def test_status_lines_and_where_they_go(self):
        rc, out, err = self.bash('ui_step "Build"; ui_info "going"; ui_ok "done"; ui_warn "careful"; ui_err "bad"; echo "raw output" | ui_indent; ui_note "a note"')
        self.assertEqual(rc, 0, err)
        self.assertEqual(out.split("\n")[1][:9], "── Build ")
        self.assertIn("  ▸ going\n  ✓ done\n    raw output\n    a note\n", out)
        self.assertEqual(err, "  ! careful\n  ✗ bad\n")                 # warnings and errors on standard error

    def test_die_exits_1(self):
        rc, out, err = self.bash('ui_die "stop"; echo not reached')
        self.assertEqual((rc, out, err), (1, "", "  ✗ stop\n"))

    def test_box_and_table_are_aligned(self):
        rc, out, err = self.bash('ui_box --title "Ready" --style ok "Open\thttp://localhost:8080" "言語\t日本語" "---" "--dash first"; ui_table --header "A\tB" "1\t朝日建機" "22\tx"')
        self.assertEqual(rc, 0, err)
        lines = out.rstrip("\n").split("\n")
        self.assertEqual(len({ui.width(l) for l in lines[:8]}), 1, out)        # the box: 8 lines
        self.assertEqual(len({ui.width(l) for l in lines[8:]}), 1, out)        # the table

    def test_ascii_when_the_locale_is_not_utf8_and_python_draws_the_same(self):
        rc, out, err = self.bash('ui_ok "done"; ui_box "text"', env={"LC_ALL": "C"})
        self.assertEqual(rc, 0, err)
        self.assertTrue(all(ord(c) < 128 for c in out), out)
        self.assertIn("  + done\n  +------", out)

    def test_colour_only_when_asked_and_never_red_for_success(self):
        rc, out, err = self.bash('ui_ok "done"; ui_info "x"; ui_warn "w"; ui_err "e"', env={"SEAM_COLOR": "always"})
        self.assertIn("\x1b[32m", out)
        self.assertNotIn("\x1b[31m", out)
        self.assertEqual(err.count("\x1b[31m"), 1)                       # the error, and only the error
        rc, out, err = self.bash('ui_ok "done"', env={"SEAM_COLOR": "auto"})
        self.assertNotIn("\x1b[", out)                                   # a pipe is not a terminal

    def test_without_python_the_text_is_still_there(self):
        rc, out, err = self.bash('ui_box --title "Ready" "Open\thttp://x" "---" "last"; ui_table "a\tb"; ui_rule "T"', env={"SEAM_UI_NO_PYTHON": "1"})
        self.assertEqual(rc, 0, err)
        for piece in ("Ready", "Open  http://x", "last", "a  b", "T"):
            self.assertIn(piece, out)

    def test_questions_take_the_default_when_nobody_is_there(self):
        rc, out, err = self.bash('ui_ask NAME "Your name" "nobody"; ui_menu PICK "Pick" 2 "one" "two\tthe second" "three"; echo "$NAME/$PICK"')
        self.assertEqual(rc, 0, err)
        self.assertEqual(out.rstrip("\n").split("\n")[-1], "nobody/2")
        self.assertIn("(default)", out)


if __name__ == "__main__":
    unittest.main()
