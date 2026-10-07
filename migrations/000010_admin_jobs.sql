-- Work requested from the admin console (e.g. "sync now"). The importer and the backup
-- service poll for queued jobs of their kinds, claim one with FOR UPDATE SKIP LOCKED and
-- record the outcome, so every manual run is audited with who asked for it.
CREATE TABLE IF NOT EXISTS admin_jobs (
    id BIGSERIAL PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('sync', 'current_release', 'transit', 'backup')),
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    requested_by TEXT NOT NULL, -- Tailscale login
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    result JSONB NOT NULL DEFAULT '{}', -- e.g. {"runs": 3, "rowsUpserted": 3088} or {"file": "...", "bytes": 1}
    error TEXT
);

-- At most one queued or running job per kind, so repeated clicks cannot pile up work.
CREATE UNIQUE INDEX IF NOT EXISTS admin_jobs_one_active_idx ON admin_jobs (kind) WHERE status IN ('queued', 'running');
CREATE INDEX IF NOT EXISTS admin_jobs_requested_idx ON admin_jobs (requested_at DESC);
