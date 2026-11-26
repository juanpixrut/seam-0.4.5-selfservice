#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
seeds/md2pdf.py — the seeds catalogue as a web page and as a PDF.

Reads SEEDS-CATALOGO.md and writes, beside it:

    SEEDS-CATALOGO.html   always: one self-contained page (no network, no
                          scripts), laid out for the screen and for A4 paper,
                          so "Print" in any browser gives the same document
    SEEDS-CATALOGO.pdf    when a Chrome, Chromium or Edge is installed: the
                          page printed by that browser, headless

Python 3 standard library only: nothing to install. It runs the same on Linux
and on macOS. A browser is looked for in $CHROME, then on the PATH, then in
/Applications.

Usage:
    python3 seeds/md2pdf.py                       # the catalogue, HTML and PDF
    python3 seeds/md2pdf.py --html                # the page only
    python3 seeds/md2pdf.py FILE.md [OUT.pdf]     # another document

The Markdown it reads is the subset the catalogue uses: headings, paragraphs,
tables, bullet and numbered lists, fenced code, rules, and bold, italic, code
and links inside a line. Anything else comes out as the text it is.
"""

import argparse
import html
import os
import re
import shutil
import subprocess
import sys
import tempfile

BROWSERS = ["google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "msedge"]
MAC_BROWSERS = [
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Chromium.app/Contents/MacOS/Chromium",
    "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
]

CSS = """
  @page {
    size: A4;
    margin: 22mm 18mm 22mm 18mm;
    @bottom-center {
      content: "%(footer)s — " counter(page) " / " counter(pages);
      font-size: 8pt;
      color: #888;
      font-family: -apple-system, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
    }
  }
  html { background: #f3f3f3; }
  body {
    font-family: -apple-system, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
    font-size: 10.5pt; line-height: 1.55; color: #1a1a1a;
    max-width: 52rem; margin: 2rem auto; padding: 2.5rem 3rem; background: #fff;
    box-shadow: 0 1px 4px rgba(0,0,0,.15);
  }
  h1 { font-size: 20pt; color: #111; border-bottom: 2px solid #333; padding-bottom: 6pt; margin-top: 0; }
  h2 { font-size: 15pt; color: #222; border-bottom: 1px solid #ccc; padding-bottom: 4pt; margin-top: 28pt; }
  h3 { font-size: 12pt; color: #333; margin-top: 16pt; }
  h2, h3, h4 { page-break-after: avoid; break-after: avoid; }
  p { margin: 6pt 0; orphans: 3; widows: 3; }
  p.meta { line-height: 1.7; }
  a { color: #1a4f8b; text-decoration: none; }
  nav.toc { background: #fafafa; border: 1px solid #ddd; border-radius: 4pt; padding: 8pt 14pt; margin: 14pt 0; }
  nav.toc p { margin: 0 0 4pt 0; font-weight: 600; }
  nav.toc ol { list-style: none; margin: 0; padding: 0; columns: 2; }
  nav.toc li { margin: 2pt 0; }
  table { border-collapse: collapse; width: 100%%; margin: 10pt 0; font-size: 9.5pt; }
  tr { page-break-inside: avoid; break-inside: avoid; }
  th { background: #f0f0f0; border: 1px solid #ccc; padding: 5pt 8pt; text-align: left; font-weight: 600; }
  td { border: 1px solid #ddd; padding: 4pt 8pt; vertical-align: top; }
  tr:nth-child(even) td { background: #fafafa; }
  code { font-family: Menlo, Consolas, 'DejaVu Sans Mono', monospace; font-size: 9pt; background: #f4f4f4;
         padding: 1pt 4pt; border-radius: 2pt; }
  pre { background: #f4f4f4; border: 1px solid #ddd; border-radius: 4pt; padding: 10pt 12pt; font-size: 8.5pt;
        line-height: 1.4; white-space: pre-wrap; page-break-inside: avoid; break-inside: avoid; }
  pre code { background: none; padding: 0; font-size: inherit; }
  hr { border: none; border-top: 1px solid #ddd; margin: 20pt 0; }
  ul, ol { margin: 6pt 0; padding-left: 20pt; }
  li { margin: 3pt 0; }
  @media print {
    html { background: none; }
    body { max-width: none; margin: 0; padding: 0; box-shadow: none; }
    h2 { page-break-before: always; break-before: page; }
    h2.first { page-break-before: avoid; break-before: avoid; }
    hr { display: none; }
    a { color: inherit; }
  }
"""

FENCE = re.compile(r"^```\s*(\S*)\s*$")
HEADING = re.compile(r"^(#{1,4})\s+(.*?)\s*#*\s*$")
BULLET = re.compile(r"^\s{0,3}[-*]\s+(.*)$")
NUMBERED = re.compile(r"^\s{0,3}(\d+)[.)]\s+(.*)$")
RULE = re.compile(r"^\s{0,3}(-{3,}|\*{3,}|_{3,})\s*$")
TABLE_SEP = re.compile(r"^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$")
META = re.compile(r"^\*\*[^*]+\*\*")


def inline(text):
    """One line of Markdown as HTML: code, bold, italic and links."""
    out, pos = [], 0
    for m in re.finditer(r"`([^`]+)`", text):
        out.append(emphasis(text[pos:m.start()]))
        out.append("<code>" + html.escape(m.group(1), quote=False) + "</code>")
        pos = m.end()
    out.append(emphasis(text[pos:]))
    return "".join(out)


def emphasis(text):
    text = html.escape(text, quote=False)
    text = re.sub(r"\[([^\]]+)\]\((https?://[^)\s]+|#[^)\s]+|[\w./-]+)\)",
                  lambda m: '<a href="%s">%s</a>' % (html.escape(m.group(2)), m.group(1)), text)
    text = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", text)
    text = re.sub(r"(?<![\w*])\*([^*\s][^*]*?)\*(?![\w*])", r"<em>\1</em>", text)
    return text


def slug(text, taken):
    base = re.sub(r"[^\w]+", "-", re.sub(r"[`*]", "", text).lower(), flags=re.UNICODE).strip("-") or "s"
    s, n = base, 2
    while s in taken:
        s, n = "%s-%d" % (base, n), n + 1
    taken.add(s)
    return s


def cells(line):
    line = line.strip()
    if line.startswith("|"):
        line = line[1:]
    if line.endswith("|"):
        line = line[:-1]
    return [c.strip() for c in re.split(r"(?<!\\)\|", line)]


def convert(md):
    """The document as (title, body HTML, table of contents entries)."""
    lines = md.replace("\r\n", "\n").split("\n")
    out, toc, taken = [], [], set()
    title = ""
    i, n = 0, len(lines)

    def blank(k):
        return k >= n or not lines[k].strip()

    def block_start(k):
        ln = lines[k]
        return bool(FENCE.match(ln) or HEADING.match(ln) or RULE.match(ln) or BULLET.match(ln) or NUMBERED.match(ln)
                    or (ln.lstrip().startswith("|") and k + 1 < n and TABLE_SEP.match(lines[k + 1])))

    while i < n:
        ln = lines[i]
        if not ln.strip():
            i += 1
            continue
        m = FENCE.match(ln)
        if m:
            i += 1
            code = []
            while i < n and not FENCE.match(lines[i]):
                code.append(lines[i])
                i += 1
            i += 1
            out.append("<pre><code>%s</code></pre>" % html.escape("\n".join(code), quote=False))
            continue
        m = HEADING.match(ln)
        if m:
            level, text = len(m.group(1)), m.group(2)
            ident = slug(text, taken)
            cls = ""
            if level == 1 and not title:
                title = re.sub(r"[`*]", "", text)
            if level == 2:
                if not toc:
                    cls = ' class="first"'
                toc.append((ident, text))
            out.append('<h%d id="%s"%s>%s</h%d>' % (level, ident, cls, inline(text), level))
            i += 1
            continue
        if RULE.match(ln):
            out.append("<hr>")
            i += 1
            continue
        if ln.lstrip().startswith("|") and i + 1 < n and TABLE_SEP.match(lines[i + 1]):
            head = cells(ln)
            i += 2
            rows = []
            while i < n and lines[i].lstrip().startswith("|"):
                rows.append(cells(lines[i]))
                i += 1
            t = ["<table>", "<thead><tr>%s</tr></thead>" % "".join("<th>%s</th>" % inline(c) for c in head), "<tbody>"]
            for r in rows:
                r = (r + [""] * len(head))[:len(head)]
                t.append("<tr>%s</tr>" % "".join("<td>%s</td>" % inline(c) for c in r))
            t.append("</tbody></table>")
            out.append("\n".join(t))
            continue
        if BULLET.match(ln) or NUMBERED.match(ln):
            ordered = bool(NUMBERED.match(ln))
            pat = NUMBERED if ordered else BULLET
            items = []
            while i < n:
                m = pat.match(lines[i])
                if m:
                    items.append([m.group(2) if ordered else m.group(1)])
                    i += 1
                elif lines[i].strip() and lines[i][:1] in (" ", "\t") and items and not block_start(i):
                    items[-1].append(lines[i].strip())  # a wrapped line of the item
                    i += 1
                elif blank(i) and i + 1 < n and pat.match(lines[i + 1]):
                    i += 1  # a loose list
                else:
                    break
            tag = "ol" if ordered else "ul"
            out.append("<%s>\n%s\n</%s>" % (tag, "\n".join("<li>%s</li>" % inline(" ".join(it)) for it in items), tag))
            continue
        para = []
        while i < n and lines[i].strip() and not (para and block_start(i)):
            para.append(lines[i].strip())
            i += 1
        # lines that each open with a bold label are a block of facts: one per line
        starts = [k for k, p in enumerate(para) if META.match(p)]
        if starts and starts[0] == 0 and len(starts) > 1:
            groups = []
            for p in para:
                if META.match(p) or not groups:
                    groups.append(p)
                else:
                    groups[-1] += " " + p
            out.append('<p class="meta">%s</p>' % "<br>\n".join(inline(g) for g in groups))
        else:
            out.append("<p>%s</p>" % inline(" ".join(para)))
    return title, "\n".join(out), toc


def page(md, lang="es"):
    title, body, toc = convert(md)
    if toc:
        nav = '<nav class="toc"><p>%s</p><ol>\n%s\n</ol></nav>' % (
            "Contenido" if lang == "es" else "Contents",
            "\n".join('<li><a href="#%s">%s</a></li>' % (i, inline(t)) for i, t in toc))
        first = body.find('<h2 id="')
        if first >= 0:
            body = body[:first] + nav + "\n" + body[first:]
    footer = title.replace("\\", "").replace('"', "'")
    return """<!DOCTYPE html>
<html lang="%s">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>%s</style>
</head>
<body>
%s
</body>
</html>
""" % (lang, html.escape(title), CSS % {"footer": footer}, body)


def find_browser():
    named = os.environ.get("CHROME", "").strip()
    if named:
        return named if os.path.exists(named) else shutil.which(named)
    for name in BROWSERS:
        found = shutil.which(name)
        if found:
            return found
    for path in MAC_BROWSERS:
        if os.path.exists(path):
            return path
    return None


def print_pdf(browser, html_path, pdf_path):
    """Prints the page with a headless browser; returns "" or what went wrong."""
    with tempfile.TemporaryDirectory(prefix="md2pdf-") as profile:
        cmd = [browser, "--headless", "--disable-gpu", "--no-pdf-header-footer", "--user-data-dir=" + profile,
               "--print-to-pdf=" + os.path.abspath(pdf_path)]
        if hasattr(os, "geteuid") and os.geteuid() == 0:
            cmd.append("--no-sandbox")  # the browser refuses to run as root with its sandbox
        cmd.append("file://" + os.path.abspath(html_path))
        try:
            r = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=180)
        except (OSError, subprocess.TimeoutExpired) as e:
            return str(e)
    if r.returncode != 0 or not os.path.exists(pdf_path) or os.path.getsize(pdf_path) == 0:
        return (r.stderr.decode("utf-8", "replace").strip().splitlines() or ["exit %d" % r.returncode])[-1]
    return ""


def main(argv=None):
    here = os.path.dirname(os.path.abspath(__file__))
    ap = argparse.ArgumentParser(description="The seeds catalogue as a web page and as a PDF.")
    ap.add_argument("markdown", nargs="?", default=os.path.join(here, "SEEDS-CATALOGO.md"), help="the Markdown file to read")
    ap.add_argument("pdf", nargs="?", help="where to write the PDF (default: beside the Markdown file)")
    ap.add_argument("--html", action="store_true", help="write the page only, no PDF")
    ap.add_argument("--lang", default="es", help="language of the document (default: es)")
    args = ap.parse_args(argv)

    if not os.path.exists(args.markdown):
        print("Error: %s not found" % args.markdown, file=sys.stderr)
        return 1
    stem = os.path.splitext(args.markdown)[0]
    html_path = stem + ".html"
    pdf_path = args.pdf or stem + ".pdf"

    with open(args.markdown, encoding="utf-8") as f:
        doc = page(f.read(), args.lang)
    with open(html_path, "w", encoding="utf-8") as f:
        f.write(doc)
    print("Wrote %s (%d KB)" % (os.path.relpath(html_path), max(1, os.path.getsize(html_path) // 1024)))
    if args.html:
        return 0

    browser = find_browser()
    if not browser:
        print("No Chrome, Chromium or Edge found: the PDF was not written.")
        print("  Open the page and print it to PDF, or install a browser and run this again:")
        print("    Linux: apt install chromium      macOS: brew install --cask chromium")
        print("  A browser somewhere else: CHROME=/path/to/browser python3 seeds/md2pdf.py")
        return 0
    problem = print_pdf(browser, html_path, pdf_path)
    if problem:
        print("Error: %s could not print the PDF: %s" % (os.path.basename(browser), problem), file=sys.stderr)
        return 1
    print("Wrote %s (%d KB), printed with %s" % (os.path.relpath(pdf_path), max(1, os.path.getsize(pdf_path) // 1024), os.path.basename(browser)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
