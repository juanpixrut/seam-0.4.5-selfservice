#!/usr/bin/env bash
# scripts/lib/modules.sh — the application modules that are off by default
#
# Source it; do not run it. Work Orders, Issues and Accounting are each behind
# a flag of Seam that is off unless it is set (internal/features/catalog.go).
# scripts/launch.sh, scripts/build-demo.sh and the Makefile switch them on
# through this one list, so that every module is handled the same way: a
# module is on where its data is loaded, or where it is asked for by name.
#
#   modules_table                  the modules, one per line:
#                                  id<TAB>title<TAB>variable<TAB>config key
#   modules_ids                    their ids, on one line
#   modules_field ID N             field N (2 to 4) of that line for one module
#   modules_resolve WHAT           "all", "none", or a list of ids separated by
#                                  commas or spaces -> the ids, on one line.
#                                  An id that is not a module is an error (1).
#   modules_titles ID...           their titles, separated by ", "
#   modules_export ID...           sets each module's variable to 1 for Seam,
#                                  unless the caller's environment already
#                                  says something for it: that is kept
#   modules_on                     the ids of the modules whose variable is on
#   modules_standalone             the ids of the modules that can be the whole
#                                  system: Seam without the Assets module
#                                  (enable_operational off) and with that one
#
# The id of a module is the key of its flag in Seam, and what the header of a
# seed says in x.seam.module (seeds_modules in scripts/lib/seeds.sh).
# scripts/tests/test_makefile.py holds this list to the catalogue.
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

modules_table() {
    printf '%s\t%s\t%s\t%s\n' \
        work_orders "Work Orders" SEAM_ENABLE_WORK_ORDERS enable_work_orders \
        issues      "Issues"      SEAM_ENABLE_ISSUES      enable_issues \
        accounting  "Accounting"  SEAM_ENABLE_ACCOUNTING  enable_accounting
}

modules_ids() {
    local _id _rest _out=""
    while IFS=$'\t' read -r _id _rest; do _out="$_out $_id"; done <<< "$(modules_table)"
    echo "${_out# }"
}

modules_field() {
    local _id _title _var _key
    while IFS=$'\t' read -r _id _title _var _key; do
        if [ "$_id" = "$1" ]; then
            case "$2" in 2) echo "$_title" ;; 3) echo "$_var" ;; 4) echo "$_key" ;; esac
            return 0
        fi
    done <<< "$(modules_table)"
    return 1
}

modules_resolve() {
    local _what="${1:-none}" _word _out=""
    case "$_what" in
        all)  modules_ids; return 0 ;;
        none) echo ""; return 0 ;;
    esac
    for _word in ${_what//,/ }; do
        if ! modules_field "$_word" 2 >/dev/null; then
            echo "$_word is not a module: the modules are $(modules_ids)" >&2
            return 1
        fi
        case " $_out " in *" $_word "*) ;; *) _out="$_out $_word" ;; esac
    done
    echo "${_out# }"
}

modules_titles() {
    local _id _out=""
    for _id in "$@"; do _out="$_out, $(modules_field "$_id" 2)"; done
    echo "${_out#, }"
}

modules_export() {
    local _id _var
    for _id in "$@"; do
        _var="$(modules_field "$_id" 3)" || continue
        # set by the caller, to anything: left as it is
        if [ -z "$(eval "echo \"\${$_var+set}\"")" ]; then
            export "$_var=1"
        fi
    done
}

# Accounting holds the books of the company that sells Seam and reads nothing
# of Assets. Work Orders and Issues are about assets: they are not here.
modules_standalone() {
    echo "accounting"
}

modules_on() {
    local _id _var _out=""
    for _id in $(modules_ids); do
        _var="$(modules_field "$_id" 3)"
        case "$(eval "echo \"\${$_var:-}\"")" in
            1|true|TRUE|yes|on) _out="$_out $_id" ;;
        esac
    done
    echo "${_out# }"
}
