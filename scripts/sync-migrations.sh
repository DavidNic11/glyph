#!/usr/bin/env bash
# Mirror api/migrations/ into helm/glyph/migrations/.
#
# The Helm chart's migrations ConfigMap is built with `.Files.Glob
# "migrations/*.sql"`, which can only see files inside the chart directory —
# Helm refuses to package anything above it. Upstream solves this in cd.yml by
# copying at deploy time; this fork is deployed by Argo CD reading the chart
# path out of git, where nothing runs beforehand, so the copy has to be
# committed.
#
# Run this after adding a migration. `--check` verifies without writing and is
# what CI runs.
set -euo pipefail

cd "$(dirname "$0")/.."

SRC=api/migrations
DST=helm/glyph/migrations

if [[ "${1:-}" == "--check" ]]; then
  if diff -rq "$SRC" "$DST" >/dev/null 2>&1; then
    echo "migrations in sync"
    exit 0
  fi
  echo "ERROR: $DST is out of sync with $SRC" >&2
  diff -rq "$SRC" "$DST" >&2 || true
  echo >&2
  echo "Run scripts/sync-migrations.sh and commit the result." >&2
  exit 1
fi

rm -rf "$DST"
cp -r "$SRC" "$DST"
echo "synced $(find "$DST" -name '*.sql' | wc -l | tr -d ' ') migration files into $DST"
