#!/usr/bin/env bash
# scripts/lib/ui.sh — terminal output for the shell scripts of this repository
#
# Source it; do not run it:
#
#   source "$SCRIPT_DIR/lib/ui.sh"
#
# What a script says goes through these, so that every script looks the same
# and nothing is drawn by counting spaces:
#
#   ui_step "Title"            a section:        ── Title ─────────────
#   ui_info "text"             something begins:   ▸ text
#   ui_ok   "text"             it went well:       ✓ text
#   ui_warn "text"             look at this:       ! text        (standard error)
#   ui_err  "text"             it failed:          ✗ text        (standard error)
#   ui_die  "text"             ui_err, then exit 1
#   ui_note "text"             an indented line under one of the above
#   ui_box   [--title T] [--style plain|ok|warn|error] LINE...
#   ui_table [--header "A<TAB>B"] [--align llr] ROW...
#   ui_rule  [TITLE]
#   ui_indent                  a filter: indents what another command prints
#   ui_ask VAR "Question" DEFAULT     reads an answer; the default when there
#                                     is no terminal to ask at
#   ui_menu VAR "Question" DEFAULT_NUMBER ITEM...   a numbered choice
#
# A LINE of a box is text; "key<TAB>value" is a pair whose values line up;
# "" is an empty line; "---" is a line across. Boxes and tables are drawn by
# scripts/lib/ui.py, which measures text by screen columns, so accents, box
# characters and Japanese do not push a border out of place. Without python3
# (or with SEAM_UI_NO_PYTHON=1) the same text is printed as plain lines.
#
# Colour: only on a terminal, never when NO_COLOR is set, and red only for
# errors. SEAM_COLOR=always|never overrides. SEAM_ASCII=1, or a locale that
# is not UTF-8, gives ASCII marks.
#
# Works with bash 3.2 (macOS) and later.
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

UI_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UI_PY="$UI_LIB_DIR/ui.py"

_ui_setup() {
    local color=false
    case "${SEAM_COLOR:-auto}" in
        always) color=true ;;
        never)  color=false ;;
        *) if [ -t 1 ] && [ -z "${NO_COLOR+x}" ] && [ "${TERM:-}" != "dumb" ]; then color=true; fi ;;
    esac
    if [ "$color" = true ]; then
        UI_RED=$'\033[31m'; UI_GREEN=$'\033[32m'; UI_YELLOW=$'\033[33m'; UI_CYAN=$'\033[36m'
        UI_BOLD=$'\033[1m'; UI_DIM=$'\033[2m'; UI_RESET=$'\033[0m'
        export SEAM_COLOR=always          # what ui.py draws matches, also through a pipe
    else
        UI_RED=""; UI_GREEN=""; UI_YELLOW=""; UI_CYAN=""; UI_BOLD=""; UI_DIM=""; UI_RESET=""
        export SEAM_COLOR=never
    fi
    local charset="${LC_ALL:-${LC_CTYPE:-${LANG:-}}}"
    if [ -n "${SEAM_ASCII:-}" ] && [ "${SEAM_ASCII}" != "0" ]; then
        UI_ASCII=true
    else
        case "$charset" in
            *[Uu][Tt][Ff]-8*|*[Uu][Tt][Ff]8*) UI_ASCII=false ;;
            *) UI_ASCII=true ;;
        esac
    fi
    if [ "$UI_ASCII" = true ]; then
        UI_MARK_OK="+"; UI_MARK_WARN="!"; UI_MARK_ERR="x"; UI_MARK_INFO=">"; UI_HLINE="-"
        export SEAM_ASCII=1
    else
        UI_MARK_OK="✓"; UI_MARK_WARN="!"; UI_MARK_ERR="✗"; UI_MARK_INFO="▸"; UI_HLINE="─"
    fi
    # SEAM_UI_NO_PYTHON=1 draws without ui.py, as on a machine with no python3.
    UI_HAVE_PY=false
    if [ -z "${SEAM_UI_NO_PYTHON:-}" ] && command -v python3 >/dev/null 2>&1 && [ -f "$UI_PY" ]; then UI_HAVE_PY=true; fi
}
_ui_setup

ui_info() { printf '  %s%s%s %s\n' "$UI_BOLD" "$UI_MARK_INFO" "$UI_RESET" "$*"; }
ui_ok()   { printf '  %s%s%s%s %s\n' "$UI_GREEN" "$UI_BOLD" "$UI_MARK_OK" "$UI_RESET" "$*"; }
ui_warn() { printf '  %s%s%s%s %s\n' "$UI_YELLOW" "$UI_BOLD" "$UI_MARK_WARN" "$UI_RESET" "$*" >&2; }
ui_err()  { printf '  %s%s%s%s %s\n' "$UI_RED" "$UI_BOLD" "$UI_MARK_ERR" "$UI_RESET" "$*" >&2; }
ui_die()  { ui_err "$*"; exit 1; }
ui_note() { printf '    %s\n' "$*"; }
ui_indent() { while IFS= read -r _ui_line || [ -n "$_ui_line" ]; do printf '    %s\n' "$_ui_line"; done; }

ui_rule() {
    if [ "$UI_HAVE_PY" = true ]; then
        python3 "$UI_PY" rule ${1:+"$1"}
    else
        printf '%s%s %s %s%s\n' "$UI_CYAN" "$UI_HLINE$UI_HLINE" "${1:-}" "$UI_HLINE$UI_HLINE$UI_HLINE$UI_HLINE" "$UI_RESET"
    fi
}

ui_step() { echo ""; ui_rule "$*"; }

# ui_box [--title T] [--style S] [--width N] LINE...
ui_box() {
    local opts=() title=""
    while [ $# -gt 0 ]; do
        case "$1" in
            --title) title="$2"; opts+=("--title" "$2"); shift 2 ;;
            --style) opts+=("--style" "$2"); shift 2 ;;
            --width) UI_BOX_WIDTH="$2"; shift 2 ;;
            --) shift; break ;;
            *) break ;;
        esac
    done
    if [ "$UI_HAVE_PY" = true ]; then
        python3 "$UI_PY" ${UI_BOX_WIDTH:+--width "$UI_BOX_WIDTH"} box ${opts[@]+"${opts[@]}"} -- "$@"
    else
        [ -n "$title" ] && printf '  %s%s%s\n' "$UI_BOLD" "$title" "$UI_RESET"
        local line
        for line in "$@"; do
            [ "$line" = "---" ] && line=""
            printf '  %s\n' "${line//$'\t'/  }"
        done
    fi
    unset UI_BOX_WIDTH
}

# ui_table [--header "A<TAB>B"] [--align llr] ROW...
ui_table() {
    local opts=()
    while [ $# -gt 0 ]; do
        case "$1" in
            --header|--align) opts+=("$1" "$2"); shift 2 ;;
            --) shift; break ;;
            *) break ;;
        esac
    done
    if [ "$UI_HAVE_PY" = true ]; then
        python3 "$UI_PY" table ${opts[@]+"${opts[@]}"} -- "$@"
    else
        local row
        for row in "$@"; do printf '  %s\n' "${row//$'\t'/  }"; done
    fi
}

# ui_ask VAR "Question" DEFAULT — the default is taken when there is nobody to ask.
ui_ask() {
    local _ui_ask_var="$1" _ui_ask_q="$2" _ui_ask_def="${3:-}" _ui_ask_ans=""
    if [ -t 0 ]; then
        if [ -n "$_ui_ask_def" ]; then
            read -r -p "  $_ui_ask_q [$_ui_ask_def]: " _ui_ask_ans || _ui_ask_ans=""
        else
            read -r -p "  $_ui_ask_q: " _ui_ask_ans || _ui_ask_ans=""
        fi
    fi
    printf -v "$_ui_ask_var" '%s' "${_ui_ask_ans:-$_ui_ask_def}"
}

# ui_menu VAR "Question" DEFAULT_NUMBER ITEM... — sets VAR to the number chosen (1-based).
# An ITEM is "label" or "label<TAB>description". Without a terminal the default is taken.
ui_menu() {
    local _ui_menu_var="$1" _ui_menu_q="$2" _ui_menu_def="$3"; shift 3
    local _ui_menu_n=$# _ui_menu_i=1 _ui_menu_rows=() _ui_menu_item _ui_menu_mark _ui_menu_ans=""
    for _ui_menu_item in "$@"; do
        _ui_menu_mark=""
        [ "$_ui_menu_i" = "$_ui_menu_def" ] && _ui_menu_mark="(default)"
        _ui_menu_rows+=("$_ui_menu_i)"$'\t'"$_ui_menu_item"$'\t'"$_ui_menu_mark")
        _ui_menu_i=$((_ui_menu_i + 1))
    done
    echo ""
    ui_table -- "${_ui_menu_rows[@]}"
    echo ""
    while :; do
        ui_ask _ui_menu_ans "$_ui_menu_q, 1 to $_ui_menu_n" "$_ui_menu_def"
        case "$_ui_menu_ans" in
            ''|*[!0-9]*) ;;
            *) if [ "$_ui_menu_ans" -ge 1 ] && [ "$_ui_menu_ans" -le "$_ui_menu_n" ]; then break; fi ;;
        esac
        ui_warn "'$_ui_menu_ans' is not one of 1 to $_ui_menu_n"
        [ -t 0 ] || { _ui_menu_ans="$_ui_menu_def"; break; }
    done
    printf -v "$_ui_menu_var" '%s' "$_ui_menu_ans"
}
