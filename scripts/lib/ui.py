#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
# https://www.apache.org/licenses/LICENSE-2.0
"""Terminal output for the scripts of this repository: boxes, tables, columns, rules, status lines.

Everything here measures text by the columns it takes on the screen, not by
its bytes or its characters: a box-drawing character is three bytes and one
column, a Japanese character is one character and two columns, a colour code
is several characters and no column. The shell's printf pads by bytes (or by
characters), which is why hand-made boxes come out crooked.

Invariants, checked on every call and by scripts/tests/test_ui.py:

  1. Every line of a box, a table or a set of columns has the same display
     width, whatever the text.
  2. Text never breaks a border: a line too long for the box is wrapped, and a
     box is never wider than the terminal (or than --width).
  3. Colour is decoration only. With no colour (a pipe, NO_COLOR, --color
     never) the text is the same, character for character.
  4. Red is for errors and nothing else.
  5. With --ascii (or SEAM_ASCII=1, or a terminal that is not UTF-8) only
     ASCII is used for what this module draws.

As a command (Python 3.8 or later, standard library only):

    ui.py box [--title T] [--style plain|ok|warn|error] -- LINE...
    ui.py table [--header "A<TAB>B"] -- ROW...   (cells separated by a tab)
    ui.py columns --left FILE --right FILE        (two blocks of text side by side)
    ui.py rule [TITLE]
    ui.py status ok|warn|error|info|step TEXT
    ui.py width TEXT

A LINE of a box is text; "key<TAB>value" is a pair, and the values of all the
pairs line up; "" is an empty line; "---" is a line across the box. The "--"
before the lines keeps one that starts with a dash from being read as an option.
With no lines, they are read from standard input, one per line.
From bash, source scripts/lib/ui.sh, which wraps these.
"""
import argparse
import os
import re
import shutil
import sys
import unicodedata

ANSI = re.compile(r"\x1b\[[0-9;]*[A-Za-z]")
CODES = {"reset": "\x1b[0m", "bold": "\x1b[1m", "dim": "\x1b[2m", "red": "\x1b[31m", "green": "\x1b[32m", "yellow": "\x1b[33m", "cyan": "\x1b[36m"}
STYLES = {"plain": "bold", "ok": "green", "warn": "yellow", "error": "red", "info": "cyan"}
UNICODE = dict(tl="┌", tr="┐", bl="└", br="┘", h="─", v="│", ml="├", mr="┤", col="│", ok="✓", warn="!", error="✗", info="▸", ellipsis="…", bullet="•")
ASCII = dict(tl="+", tr="+", bl="+", br="+", h="-", v="|", ml="+", mr="+", col="|", ok="+", warn="!", error="x", info=">", ellipsis="...", bullet="*")
DEFAULT_WIDTH = 80
MIN_WIDTH = 24
RULE_WIDTH = 64


class Misaligned(AssertionError):
    """A block was produced whose lines are not all the same width: a defect of this module."""


def strip(text):
    """The text without colour codes."""
    return ANSI.sub("", text)


def width(text):
    """The number of terminal columns the text takes."""
    n = 0
    for ch in strip(text):
        if ch == "\t":
            n += 1
        elif unicodedata.combining(ch) or unicodedata.category(ch) in ("Mn", "Me", "Cf", "Cc"):
            continue
        elif unicodedata.east_asian_width(ch) in ("W", "F"):
            n += 2
        else:
            n += 1
    return n


def pad(text, n, align="left"):
    """The text followed (or preceded) by spaces up to n columns. Text wider than n is returned as it is."""
    gap = max(0, n - width(text))
    if align == "right":
        return " " * gap + text
    if align == "center":
        return " " * (gap // 2) + text + " " * (gap - gap // 2)
    return text + " " * gap


def truncate(text, n, ellipsis="…"):
    """The text cut to n columns, ending in an ellipsis when something was cut. Colour codes are dropped from cut text."""
    if width(text) <= n:
        return text
    out, used, room = "", 0, max(0, n - width(ellipsis))
    for ch in strip(text):
        w = width(ch)
        if used + w > room:
            break
        out += ch
        used += w
    return out + ellipsis if n >= width(ellipsis) else out


def wrap(text, n):
    """The text as lines of at most n columns: broken at spaces, and inside a word only when the word alone is too long."""
    if n < 1:
        return [text]
    lines = []
    for para in text.split("\n"):
        indent = re.match(r" *", para).group(0)
        if width(indent) > n - 2:                         # no room left for a wide character after it
            indent = ""
        line, used = "", 0
        for word in para[len(indent):].split(" "):
            w = width(word)
            lead = indent if not line else " "
            if line and used + 1 + w <= n:
                line += " " + word
                used += 1 + w
                continue
            if line:
                lines.append(line)
                line, used = "", 0
                lead = indent
            while width(lead) + width(word) > n:          # a word longer than a line: cut it by columns
                room, piece, got = n - width(lead), "", 0
                for ch in word:
                    cw = width(ch)
                    if got + cw > room:
                        break
                    piece += ch
                    got += cw
                if not piece:                             # a single character wider than the room there is
                    piece = word[0]
                lines.append(lead + piece)
                word = word[len(piece):]
                lead = indent
            line, used = lead + word, width(lead) + width(word)
        lines.append(line)
    return lines


def aligned(lines):
    """Raise Misaligned unless every line has the same display width. Returns the lines."""
    widths = {width(line) for line in lines}
    if len(widths) > 1:
        raise Misaligned("lines of %s columns in one block:\n%s" % (sorted(widths), "\n".join(strip(l) for l in lines)))
    return lines


class Screen:
    """Where the output goes: how wide it is, whether it takes colour, whether it takes Unicode."""

    def __init__(self, stream=None, color=None, ascii_only=None, columns=None, env=None):
        env = os.environ if env is None else env
        stream = stream or sys.stdout
        tty = hasattr(stream, "isatty") and stream.isatty()
        if color is None:
            mode = env.get("SEAM_COLOR", "auto")
            color = mode == "always" or (mode != "never" and tty and "NO_COLOR" not in env and env.get("TERM") != "dumb")
        if ascii_only is None:
            enc = (getattr(stream, "encoding", None) or "").lower().replace("-", "")
            ascii_only = env.get("SEAM_ASCII", "") not in ("", "0") or enc not in ("utf8", "utf8sig")
        if columns is None:
            try:
                columns = int(env.get("SEAM_COLUMNS", "") or 0) or (shutil.get_terminal_size((DEFAULT_WIDTH, 24)).columns if tty else DEFAULT_WIDTH)
            except ValueError:
                columns = DEFAULT_WIDTH
        self.color, self.ascii, self.columns = bool(color), bool(ascii_only), max(MIN_WIDTH, columns)
        self.g = ASCII if self.ascii else UNICODE

    def paint(self, text, *names):
        if not self.color or not text or not names:
            return text
        return "".join(CODES[n] for n in names) + text + CODES["reset"]


def _rows_of(lines):
    """Box lines as ("text", s), ("pair", key, value), ("blank",) or ("sep",)."""
    rows = []
    for line in lines:
        if line == "":
            rows.append(("blank",))
        elif line == "---":
            rows.append(("sep",))
        elif "\t" in line:
            key, value = line.split("\t", 1)
            rows.append(("pair", key, value.replace("\t", " ")))
        else:
            rows.append(("text", line))
    return rows


def box(lines, title=None, style="plain", screen=None, max_width=None, margin=2):
    """A box around the lines. Returns the lines of the box, all of one width."""
    s = screen or Screen()
    g, colour = s.g, STYLES.get(style, "bold")
    limit = max(MIN_WIDTH, min(max_width or s.columns, s.columns)) - margin
    inner_max = limit - 4                                    # a border and a space on each side
    rows = _rows_of(lines)
    key_w = max([width(r[1]) for r in rows if r[0] == "pair"] or [0])
    key_w = min(key_w, max(4, inner_max // 2))
    body = []
    for r in rows:
        if r[0] == "pair":
            key = truncate(r[1], key_w, g["ellipsis"])
            parts = wrap(r[2], max(1, inner_max - key_w - 2)) or [""]
            body.append(("line", s.paint(pad(key, key_w), "dim") + "  " + parts[0]))
            body.extend(("line", " " * (key_w + 2) + p) for p in parts[1:])
        elif r[0] == "text":
            body.extend(("line", p) for p in wrap(r[1], inner_max))
        else:
            body.append(r)
    head = wrap(title, inner_max) if title else []
    inner = max([width(b[1]) for b in body if b[0] == "line"] + [width(h) for h in head] + [MIN_WIDTH - 4 - margin])
    edge = lambda ch: s.paint(ch, colour, "bold") if colour != "bold" else s.paint(ch, "bold")
    side = edge(g["v"])
    out = [edge(g["tl"] + g["h"] * (inner + 2) + g["tr"])]
    for h in head:
        out.append(side + " " + s.paint(pad(h, inner), "bold") + " " + side)
    if head and body:
        out.append(edge(g["ml"] + g["h"] * (inner + 2) + g["mr"]))
    for b in body:
        if b[0] == "sep":
            out.append(edge(g["ml"] + g["h"] * (inner + 2) + g["mr"]))
        else:
            out.append(side + " " + pad(b[1] if b[0] == "line" else "", inner) + " " + side)
    out.append(edge(g["bl"] + g["h"] * (inner + 2) + g["br"]))
    return [" " * margin + line for line in aligned(out)]


def table(rows, header=None, screen=None, max_width=None, margin=2, align=None):
    """Rows of cells as columns that line up, with a rule under the header. Returns lines of one width.

    A table too wide for the screen has its widest column wrapped. `align` is a
    string of l and r, one for each column."""
    s = screen or Screen()
    rows = [[str(c) for c in r] for r in rows]
    count = max([len(r) for r in rows] + [len(header or [])] + [1])
    rows = [r + [""] * (count - len(r)) for r in rows]
    header = list(header) + [""] * (count - len(header)) if header else None
    widths = [max([width(r[i]) for r in rows] + [width(header[i]) if header else 0]) for i in range(count)]
    limit = max(MIN_WIDTH, min(max_width or s.columns, s.columns)) - margin
    over = sum(widths) + 2 * (count - 1) - limit
    if over > 0:
        i = widths.index(max(widths))
        widths[i] = max(8, widths[i] - over)
    align = (align or "") + "l" * count
    side = lambda i: "right" if align[i] == "r" else "left"
    out = []
    if header:
        out.append("  ".join(s.paint(pad(truncate(header[i], widths[i], s.g["ellipsis"]), widths[i], side(i)), "bold") for i in range(count)))
        out.append("  ".join(s.paint(s.g["h"] * widths[i], "dim") for i in range(count)))
    for r in rows:
        cells = [wrap(r[i], widths[i]) for i in range(count)]
        for k in range(max(len(c) for c in cells)):
            out.append("  ".join(pad(cells[i][k] if k < len(cells[i]) else "", widths[i], side(i)) for i in range(count)))
    return [" " * margin + line for line in aligned(out)]


def columns(left, right, screen=None, gap=2, margin=0):
    """Two blocks of lines side by side with a rule between them, or one under the other when the screen is too narrow."""
    s = screen or Screen()
    lw, rw = max([width(l) for l in left] + [0]), max([width(l) for l in right] + [0])
    if margin + lw + gap * 2 + 1 + rw > s.columns:
        return [" " * margin + l for l in left] + [""] + [" " * margin + l for l in right]
    n = max(len(left), len(right))
    left, right = left + [""] * (n - len(left)), right + [""] * (n - len(right))
    bar = s.paint(s.g["col"], "dim")
    return aligned([" " * margin + pad(left[i], lw) + " " * gap + bar + " " * gap + pad(right[i], rw) for i in range(n)])


def rule(title=None, screen=None, total=RULE_WIDTH, colour="cyan"):
    """A line across, with a title in it: "── Title ─────"."""
    s = screen or Screen()
    total = min(total, s.columns)
    h = s.g["h"]
    if not title:
        return s.paint(h * total, colour)
    title = truncate(title, max(1, total - 6), s.g["ellipsis"])
    return s.paint(h * 2 + " ", colour) + s.paint(title, colour, "bold") + s.paint(" " + h * max(2, total - 4 - width(title)), colour)


def status(kind, text, screen=None):
    """One line saying how something went: a mark and the text. Only an error is red."""
    s = screen or Screen()
    if kind == "step":
        return rule(text, s)
    colour = {"ok": "green", "warn": "yellow", "error": "red", "info": "bold"}[kind]
    return "  " + s.paint(s.g[kind], colour, "bold") + " " + text


def heading(text, screen=None):
    """A section title for a column of help: "── Title"."""
    s = screen or Screen()
    return s.paint(s.g["h"] * 2 + " " + text, "bold")


def main(argv=None):
    ap = argparse.ArgumentParser(prog="ui.py", description="Boxes, tables and status lines that stay aligned (see the top of this file).")
    ap.add_argument("--color", choices=("auto", "always", "never"), default=None)
    ap.add_argument("--ascii", action="store_true", default=None)
    ap.add_argument("--width", type=int, default=None, help="the widest a block may be")
    ap.add_argument("--stderr", action="store_true", help="write to standard error")
    sub = ap.add_subparsers(dest="cmd")
    p = sub.add_parser("box")
    p.add_argument("--title")
    p.add_argument("--style", choices=sorted(STYLES), default="plain")
    p.add_argument("lines", nargs="*")
    p = sub.add_parser("table")
    p.add_argument("--header")
    p.add_argument("--align", default="")
    p.add_argument("rows", nargs="*")
    p = sub.add_parser("columns")
    p.add_argument("--left", required=True)
    p.add_argument("--right", required=True)
    p = sub.add_parser("rule")
    p.add_argument("title", nargs="?")
    p = sub.add_parser("status")
    p.add_argument("kind", choices=("ok", "warn", "error", "info", "step"))
    p.add_argument("text")
    p = sub.add_parser("width")
    p.add_argument("text")
    args = ap.parse_args(argv)
    if not args.cmd:
        ap.print_help()
        return 2
    stream = sys.stderr if args.stderr else sys.stdout
    try:
        stream.reconfigure(errors="replace")
    except (AttributeError, ValueError):
        pass
    color = None if args.color in (None, "auto") else args.color == "always"
    s = Screen(stream=stream, color=color, ascii_only=args.ascii)
    if args.cmd == "width":
        print(width(args.text))
        return 0
    if args.cmd == "box":
        lines = args.lines if args.lines else [l.rstrip("\n") for l in sys.stdin]
        out = box(lines, title=args.title, style=args.style, screen=s, max_width=args.width)
    elif args.cmd == "table":
        rows = args.rows if args.rows else [l.rstrip("\n") for l in sys.stdin]
        out = table([r.split("\t") for r in rows], header=args.header.split("\t") if args.header else None, screen=s, max_width=args.width, align=args.align)
    elif args.cmd == "columns":
        read = lambda path: [l.rstrip("\n") for l in open(path, encoding="utf-8")]
        out = columns(read(args.left), read(args.right), screen=s)
    elif args.cmd == "rule":
        out = [rule(args.title, s)]
    else:
        out = [status(args.kind, args.text, s)]
    stream.write("\n".join(out) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
