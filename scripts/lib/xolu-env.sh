#!/usr/bin/env bash
# scripts/lib/xolu-env.sh — the one place that says how Seam runs xolu
#
# Source it; do not run it. Every way of starting xolu from this repository
# (scripts/launch.sh, the seeding step of scripts/build-demo.sh, and the
# START.sh and START.cmd that build-demo.sh writes into a demo package) takes
# its settings from here, so they cannot drift apart. A module of Seam that
# needs something of xolu (the Issues lifecycle needs API v2, the QR images
# need blobs) gets it in all of them or in none.
#
#   xolu_env_pairs PORT BASE_DIR [LOG_LEVEL]     prints NAME=VALUE, one per line
#   xolu_env_export PORT BASE_DIR [LOG_LEVEL]    exports them into this shell
#   xolu_wait URL [SECONDS] [PID]                waits for /health; 1 if it does not come
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

xolu_env_pairs() {
    local port="$1" base="$2" level="${3:-info}"
    cat <<PAIRS
XOLU_PORT=$port
XOLU_BASE_DIR=$base
XOLU_STORAGE_TYPE=sqlite
XOLU_TENANT_MODE=path
XOLU_TENANT_AUTO_REGISTER=true
XOLU_GRAPH_MODE=flat
XOLU_FULLTEXT_ENABLED=true
XOLU_TIMESERIES_ENABLED=true
XOLU_TS_RETENTION_ENABLED=true
XOLU_TS_COMPRESSION=zstd
XOLU_STRICT_COMMIT=true
XOLU_CASCADING_DELETE=true
XOLU_API_V2_ENABLED=true
XOLU_BLOB_ENABLED=true
XOLU_METRICS_ENABLED=true
XOLU_LOG_LEVEL=$level
XOLU_NO_ASCII=true
PAIRS
}

xolu_env_export() {
    local pair
    while IFS= read -r pair; do
        export "${pair?}"
    done <<PAIRS
$(xolu_env_pairs "$@")
PAIRS
}

# xolu_wait URL [SECONDS] [PID] — also used for Seam: both answer GET /health.
# Sets WAITED to the seconds it took. Returns 1 on timeout, 2 if the process died.
xolu_wait() {
    local url="$1" seconds="${2:-30}" pid="${3:-}" i=0
    WAITED=0
    while [ "$i" -lt $((seconds * 2)) ]; do
        if curl -sf "$url/health" >/dev/null 2>&1; then
            WAITED=$(((i + 1) / 2))
            return 0
        fi
        if [ -n "$pid" ] && ! kill -0 "$pid" 2>/dev/null; then
            return 2
        fi
        sleep 0.5
        i=$((i + 1))
    done
    return 1
}
