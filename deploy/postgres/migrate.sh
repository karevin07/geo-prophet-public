#!/bin/sh
# Applies migrations/*.sql in order, once each, recording them in schema_migrations.
# Every migration is idempotent, so a database created before this runner existed is
# brought up to date safely on the first run.
set -eu

psql() { command psql -v ON_ERROR_STOP=1 -X -q "$@"; }

psql -c "CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())"

for file in /migrations/*.sql; do
  version=$(basename "$file" .sql)
  if [ -n "$(psql -tAc "SELECT 1 FROM schema_migrations WHERE version = '$version'")" ]; then
    continue
  fi
  echo "applying $version"
  psql -1 -f "$file" -c "INSERT INTO schema_migrations (version) VALUES ('$version')"
done
echo "migrations up to date"
