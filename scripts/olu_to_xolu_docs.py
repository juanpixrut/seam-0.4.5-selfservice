#!/usr/bin/env python3
"""
olu_to_xolu_docs.py -- systematic olu->xolu nomenclature check/fix for
Seam's living spec documents.

Why this exists: the Go codebase's olu->xolu rename (2026-07-28) covered
code, not docs. The spec documents this script targets are blueprints
for not-yet-built modules (Documents, Media, Settings/i18n, Admin) --
if they still say `olu:"field_name"` or "olu.Commit", whoever
implements those modules later will faithfully copy that straight into
new Go code, silently reintroducing exactly the naming this rename was
meant to retire.

Two modes:
  --check   (default) Report every "olu"/"OLU" match found, classified
             by rule, without writing anything. Exit code 1 if any
             match is found, 0 if clean. Meant to be re-run any time
             (e.g. before a release, or after adding new spec content)
             to catch regressions -- not a one-time script.
  --apply   Apply the classified rename rules and write the files.

Rules are intentionally narrow and named, not one blanket regex --
each pattern below was chosen because it matches a *specific*,
verified shape found in these documents; anything not matching one of
these shapes is left alone and reported separately under "unclassified"
so it gets a human's attention rather than a guess.

File scope is deliberately hand-picked, not "every .md in docs/":
living specs (blueprints for future/current implementation) are
in scope; historical records (CHANGELOG.md, RESOLVED.md, anything
marked Complete/archived, stale roadmaps referencing old version
numbers) are explicitly excluded, since rewriting "olu" there would
misrepresent what was actually true at the time. See the exclusion
list below for the reasoning behind each specific exclusion.
"""
import argparse
import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]

# Living specs: blueprints for future/current implementation. In scope.
TARGET_FILES = [
    "docs/FORM_ENGINE_SPEC.md",
    "docs/ADMIN_MODULE_SPEC.md",
    "docs/I18N_SPEC.md",
    "docs/SEAM_MEDIA_SPEC.md",
    "docs/DOCUMENTS_MODULE_SPEC.md",
    "docs/FSM_DESIGN.md",
    "seeds/SEEDS-CATALOGO.md",
]

# Explicitly excluded, and why -- checked against each file's actual
# content before this list was written, not assumed:
#   CHANGELOG.md, docs/RESOLVED.md    - append-only historical record
#   docs/OLU_CLIENT_REMEDIATION.md    - "Status: Complete", describes
#                                        internal/olu/client.go, which
#                                        genuinely existed under that
#                                        name at the time; also already
#                                        flagged elsewhere for archiving
#   docs/RBAC-COMPLETION-PLAN.md      - also flagged for archiving
#   ROADMAP.md                        - "Current version: 0.1.0-patched243",
#                                        wildly stale against the real
#                                        current version; its "Completed"
#                                        section describes past work
#   docs/TRACKING.md                  - mix of live and historical-fact
#                                        content in the same document;
#                                        handled manually all session,
#                                        deliberately not scripted
#   docs/KNOWN_ISSUES.md              - same mixed-content reasoning

# Each rule: (name, compiled pattern, replacement, description).
# Order matters -- more specific patterns are listed first so a later,
# broader rule can't consume text a narrower rule was meant to handle.
RULES = [
    (
        "struct_tag",
        re.compile(r'\bolu:"'),
        'xolu:"',
        'Go struct tag key (olu:"field_name" -> xolu:"field_name") -- '
        "these are example code in specs for not-yet-built modules; "
        "left as olu, they'd be copied verbatim into real Go structs.",
    ),
    (
        "method_call_style",
        re.compile(r'\bolu\.([A-Z][A-Za-z]*)\b'),
        r'xolu.\1',
        "Method-call-style prose (olu.Commit, olu.Create) describing "
        "calls on the client -- matches a capitalized identifier after "
        "the dot, same shape as an actual Go method call.",
    ),
    (
        "config_path_style",
        re.compile(r'\bolu\.(url|required|api_key)\b'),
        r'xolu.\1',
        "Config-path-style prose (olu.url, olu.required) describing a "
        "dotted config key, matching config.example.json's now-renamed "
        '"xolu" top-level key.',
    ),
    (
        "ini_section_header",
        re.compile(r'^\[olu\]$', re.MULTILINE),
        '[xolu]',
        "INI-style config section header in an example block, matching "
        'config.example.json\'s now-renamed "xolu" key.',
    ),
    (
        "bold_label",
        re.compile(r'\*\*olu\*\*'),
        '**xolu**',
        "Bold inline label in prose/tables.",
    ),
    (
        "generic_prose",
        # Bare "olu" as its own word, in prose. Deliberately excludes
        # anything already handled by a rule above (those are replaced
        # first, so by the time this rule runs, remaining bare "olu"
        # occurrences are genuinely generic prose, not one of the
        # specific shapes above).
        re.compile(r'\bolu\b'),
        'xolu',
        "Generic prose reference to the backend service name (English "
        'or Spanish: "stored in olu", "almacenado en olu", etc).',
    ),
]


# Explicit, named exclusions: specific strings that must NOT be renamed
# even though they contain "olu", because they refer to something that
# was deliberately NOT renamed elsewhere (a file that still exists
# under its old name). Discovered by running this script itself in
# --check mode and reviewing the "unclassified" output -- not
# anticipated in advance, so this list grows only from real findings,
# never speculative additions.
EXCLUDED_LITERALS = [
    "OLU_CLIENT_REMEDIATION",  # docs/OLU_CLIENT_REMEDIATION.md is
    # deliberately still named this (historical record, "Status:
    # Complete", already flagged for archiving) -- renaming a reference
    # to it would point at a file that doesn't exist under the new name.
]


def load_target_files():
    paths = []
    for rel in TARGET_FILES:
        p = REPO_ROOT / rel
        if not p.exists():
            print(f"WARNING: target file not found, skipping: {rel}", file=sys.stderr)
            continue
        paths.append(p)
    return paths


def classify_and_transform(text):
    """
    Applies each rule in order. Returns (new_text, per_rule_counts,
    match_details) where match_details is a list of (rule_name, line_no,
    matched_text, context) for reporting.
    """
    per_rule_counts = {name: 0 for name, *_ in RULES}
    match_details = []

    for rule_name, pattern, repl, _desc in RULES:
        def _record(m, rule_name=rule_name):
            # Check a window around the match for any excluded literal
            # (e.g. "OLU_CLIENT_REMEDIATION") before renaming -- a match
            # sitting inside an excluded literal is left untouched.
            window_start = max(0, m.start() - 30)
            window_end = min(len(text), m.end() + 30)
            window = text[window_start:window_end]
            if any(lit in window for lit in EXCLUDED_LITERALS):
                return m.group(0)
            per_rule_counts[rule_name] += 1
            line_no = text.count("\n", 0, m.start()) + 1
            line_start = text.rfind("\n", 0, m.start()) + 1
            line_end = text.find("\n", m.end())
            if line_end == -1:
                line_end = len(text)
            context = text[line_start:line_end].strip()
            match_details.append((rule_name, line_no, m.group(0), context))
            return m.expand(repl)

        text = pattern.sub(_record, text)

    return text, per_rule_counts, match_details


def find_unclassified(text):
    """
    After all rules would have applied, anything still matching bare
    olu/OLU is unclassified -- not silently renamed, surfaced instead.
    Run this against the ORIGINAL text with all rule patterns' matches
    removed conceptually by running the same substitution pipeline and
    then re-scanning for anything containing "olu" case-insensitively
    that isn't already "xolu"/"XOLU".
    """
    transformed, _, _ = classify_and_transform(text)
    leftover = []
    for m in re.finditer(r'\bolu[a-z]*\b|\bOLU[A-Z_]*\b', transformed):
        window_start = max(0, m.start() - 30)
        window_end = min(len(transformed), m.end() + 30)
        window = transformed[window_start:window_end]
        if any(lit in window for lit in EXCLUDED_LITERALS):
            continue
        line_no = transformed.count("\n", 0, m.start()) + 1
        line_start = transformed.rfind("\n", 0, m.start()) + 1
        line_end = transformed.find("\n", m.end())
        if line_end == -1:
            line_end = len(transformed)
        context = transformed[line_start:line_end].strip()
        leftover.append((line_no, m.group(0), context))
    return leftover


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--apply", action="store_true", help="Write changes. Default is a dry-run report only.")
    args = parser.parse_args()

    files = load_target_files()
    total_changes = 0
    any_unclassified = False

    for path in files:
        rel = path.relative_to(REPO_ROOT)
        original = path.read_text(encoding="utf-8")
        new_text, counts, details = classify_and_transform(original)
        file_changes = sum(counts.values())

        if file_changes == 0 and not any(c for c in counts.values()):
            leftover = find_unclassified(original)
            if not leftover:
                continue

        print(f"\n=== {rel} ===")
        for rule_name, count in counts.items():
            if count:
                print(f"  {rule_name}: {count}")
        if args.apply and file_changes:
            path.write_text(new_text, encoding="utf-8")
            print(f"  -> written ({file_changes} change(s))")
        total_changes += file_changes

        leftover = find_unclassified(original)
        if leftover:
            any_unclassified = True
            print(f"  UNCLASSIFIED (not renamed, needs a human decision): {len(leftover)}")
            for line_no, text_match, context in leftover[:10]:
                print(f"    line {line_no}: {text_match!r} in: {context[:100]}")
            if len(leftover) > 10:
                print(f"    ... and {len(leftover) - 10} more")

    print(f"\n{'Applied' if args.apply else 'Would apply'}: {total_changes} total change(s) across {len(files)} target file(s)")
    if any_unclassified:
        print("Unclassified matches found -- these need a rule added or a manual decision, not a blind rename.")
        return 1
    if total_changes and not args.apply:
        print("Dry run only. Re-run with --apply to write changes.")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
