#!/bin/sh
# Dumps the database to /backups every BACKUP_INTERVAL seconds (default daily) and keeps
# the newest BACKUP_KEEP dumps (default 7). In between it runs backups requested from the
# admin console (admin_jobs of kind backup). With --once it dumps once and exits.
#
# Dumps use pg_dump's custom format: compressed, and restorable table by table with
# pg_restore (see docs/BACKUP.md). A dump is written under a temporary name and renamed
# when complete, so a crash never leaves a truncated file that looks like a backup.
set -eu

keep="${BACKUP_KEEP:-7}"
interval="${BACKUP_INTERVAL:-86400}"
# How often to look for backups requested from the admin console.
poll="${BACKUP_POLL:-15}"

# backup dumps the database and sets written/written_bytes to the new file.
backup() {
  stamp=$(date -u +%Y%m%dT%H%M%SZ)
  partial="/backups/.geoprophet-$stamp.dump.partial"
  if ! pg_dump --format=custom --compress=6 --file="$partial"; then
    rm -f "$partial"
    echo "backup failed" >&2
    return 1
  fi
  written="geoprophet-$stamp.dump"
  mv "$partial" "/backups/$written"
  written_bytes=$(stat -c %s "/backups/$written")
  echo "backup written: $written ($(du -h "/backups/$written" | cut -f1))"
  # Newest first; delete everything after the first $keep.
  ls -1t /backups/geoprophet-*.dump 2>/dev/null | tail -n +"$((keep + 1))" | while read -r old; do
    rm -f "$old"
    echo "removed old backup: $(basename "$old")"
  done
}

# psql reads SQL from stdin so :'name' variables are quoted safely.
sql() {
  psql --no-psqlrc --quiet --tuples-only --no-align -v ON_ERROR_STOP=1 "$@"
}

# claim_job marks the oldest queued backup job running and prints its ID, or nothing.
claim_job() {
  sql <<'SQL'
UPDATE admin_jobs SET status = 'running', started_at = NOW()
WHERE id = (SELECT id FROM admin_jobs WHERE kind = 'backup' AND status = 'queued'
            ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED)
RETURNING id;
SQL
}

run_job() {
  job="$1"
  echo "running admin job $job: backup"
  if backup; then
    sql -v id="$job" -v file="$written" -v bytes="$written_bytes" <<'SQL'
UPDATE admin_jobs SET status = 'succeeded', finished_at = NOW(),
  result = jsonb_build_object('file', :'file', 'bytes', :'bytes'::BIGINT) WHERE id = :'id'::BIGINT;
SQL
  else
    sql -v id="$job" <<'SQL'
UPDATE admin_jobs SET status = 'failed', finished_at = NOW(), error = 'pg_dump failed; see docker compose logs backup'
WHERE id = :'id'::BIGINT;
SQL
  fi
}

if [ "${1:-}" = "--once" ]; then
  backup
  exit
fi

# A job left running by a previous container never finished.
sql <<'SQL'
UPDATE admin_jobs SET status = 'failed', finished_at = NOW(),
  error = 'interrupted: the backup service stopped before the job finished'
WHERE kind = 'backup' AND status = 'running';
SQL

# Restarting the stack must not dump again early: frequent restarts would otherwise push
# the daily history out of the kept set. The next scheduled dump is an interval after the
# newest one.
now=$(date +%s)
next=$now
newest=$(ls -1t /backups/geoprophet-*.dump 2>/dev/null | head -n 1 || true)
if [ -n "$newest" ]; then
  next=$(($(stat -c %Y "$newest") + interval))
  if [ "$next" -gt "$now" ]; then
    wait=$((next - now))
    echo "latest backup $(basename "$newest") is recent; next backup in $((wait / 3600))h$((wait % 3600 / 60))m"
  fi
fi

while true; do
  # Failures are logged and retried at the next interval or request; the loop goes on.
  job=$(claim_job || true)
  if [ -n "$job" ]; then
    run_job "$job" || true
  fi
  if [ "$(date +%s)" -ge "$next" ]; then
    backup || true
    next=$(($(date +%s) + interval))
  fi
  sleep "$poll"
done
