#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
# https://www.apache.org/licenses/LICENSE-2.0
"""`make help`, written from the Makefile itself.

    python3 scripts/make-help.py Makefile "Seam AMS 0.4.3"

In the Makefile:  "##@ Title" starts a section;  "target: deps ## text"
describes a target;  "##> text" is a note under the section it is in. A target
with no "##" is internal and is not listed. Because the help is read from the
targets, a target cannot be renamed or added without its help following, and
scripts/tests/test_makefile.py fails if a .PHONY target has no description.

The layout is drawn with scripts/lib/ui.py: two columns when the terminal is
wide enough, one when it is not, every line measured by screen columns.
"""
import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "lib"))
import ui  # noqa: E402

TARGET = re.compile(r"^([A-Za-z0-9][A-Za-z0-9_.-]*):(?!=)[^#\n]*?##\s*(.+?)\s*$")
SECTION = re.compile(r"^##@\s*(.+?)\s*$")
NOTE = re.compile(r"^##>\s?(.*?)\s*$")
OTHER = "Other"


def read(path):
    """The sections of a Makefile: [{"title", "notes": [..], "targets": [(name, text)]}], in file order."""
    sections, current = [], None
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.rstrip("\n")
            m = SECTION.match(line)
            if m:
                current = next((s for s in sections if s["title"] == m.group(1)), None)
                if current is None:
                    current = {"title": m.group(1), "notes": [], "targets": []}
                    sections.append(current)
                continue
            m = NOTE.match(line)
            if m and current is not None:
                current["notes"].append(m.group(1))
                continue
            m = TARGET.match(line)
            if m:
                if current is None:
                    current = {"title": OTHER, "notes": [], "targets": []}
                    sections.append(current)
                current["targets"].append((m.group(1), m.group(2)))
    return [s for s in sections if s["targets"]]


def block(section, total, screen):
    """One section as lines no wider than `total` columns."""
    name_w = max(ui.width("make " + name) for name, _ in section["targets"])
    text_w = total - name_w - 4
    lines = [ui.heading(section["title"], screen), ""]
    for name, text in section["targets"]:
        if text_w < 20:                    # a narrow screen: the text goes under the name
            lines.append("  " + screen.paint("make " + name, "bold"))
            lines.extend("      " + p for p in ui.wrap(text, max(8, total - 6)))
            continue
        parts = ui.wrap(text, text_w)
        lines.append("  " + screen.paint(ui.pad("make " + name, name_w), "bold") + "  " + parts[0])
        lines.extend("  " + " " * name_w + "  " + p for p in parts[1:])
    # consecutive notes are one paragraph; an empty note ("##>") starts another
    paragraphs, current = [], []
    for note in section["notes"] + [""]:
        if note:
            current.append(note)
        elif current:
            paragraphs.append(" ".join(current))
            current = []
    if paragraphs:
        lines.append("")
        for para in paragraphs:
            lines.extend(screen.paint(p, "dim") for p in ui.wrap("  " + para, total))
    return lines


def render(sections, title, screen):
    two = screen.columns >= 110
    total = min(62, (screen.columns - 7) // 2) if two else min(78, screen.columns - 2)
    blocks = [block(s, total, screen) for s in sections]
    out = ["", "  " + screen.paint(title, "bold"), ""]
    if not two:
        for b in blocks:
            out.extend(b + [""])
        return out
    # fill the left column until it holds half of the lines
    height, left, right = sum(len(b) + 1 for b in blocks), [], []
    for b in blocks:
        side = left if len(left) + len(b) // 2 < height / 2 else right
        if side is right and not right and not left:
            side = left
        side.extend(b + [""])
    while left and left[-1] == "":
        left.pop()
    while right and right[-1] == "":
        right.pop()
    out.extend(ui.columns(left, right, screen=screen, margin=0))
    out.append("")
    return out


def main(argv):
    path = argv[1] if len(argv) > 1 else "Makefile"
    title = argv[2] if len(argv) > 2 else "make"
    screen = ui.Screen()
    sys.stdout.write("\n".join(render(read(path), title, screen)) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
