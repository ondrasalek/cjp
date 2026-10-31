#!/bin/sh
set -eu

DB_PATH="${CJP_DB_PATH:-/data/cjp.db}"
RAW_DIR="${CJP_RAW_DIR:-/data/raw}"
WEB_DIR="${CJP_WEB_DIR:-}"
INCLUDE_GPL="${CJP_INCLUDE_GPL:-true}"
USE_HUNSPELL="${CJP_USE_HUNSPELL:-true}"
ADDR="${CJP_ADDR:-:8080}"
RATE_LIMIT_RPS="${CJP_RATE_LIMIT_RPS:-5}"
RATE_LIMIT_BURST="${CJP_RATE_LIMIT_BURST:-30}"

if [ ! -f "$DB_PATH" ]; then
    echo "[entrypoint] Database $DB_PATH not found. Bootstrapping from open datasets..."
    cjp-etl fetch --raw-dir "$RAW_DIR" --include-gpl="$INCLUDE_GPL"
    cjp-etl build --raw-dir "$RAW_DIR" --db "$DB_PATH" --include-gpl="$INCLUDE_GPL"
fi

exec cjp-api \
    --addr "$ADDR" \
    --db "$DB_PATH" \
    --raw-dir "$RAW_DIR" \
    --web-dir "$WEB_DIR" \
    --hunspell="$USE_HUNSPELL" \
    --rate-limit="$RATE_LIMIT_RPS" \
    --rate-burst="$RATE_LIMIT_BURST"
