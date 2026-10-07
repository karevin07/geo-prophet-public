-- Jobs that need input, such as re-importing chosen seasons, keep it in params.
ALTER TABLE admin_jobs ADD COLUMN IF NOT EXISTS params JSONB NOT NULL DEFAULT '{}';

ALTER TABLE admin_jobs DROP CONSTRAINT IF EXISTS admin_jobs_kind_check;
ALTER TABLE admin_jobs ADD CONSTRAINT admin_jobs_kind_check
    CHECK (kind IN ('sync', 'current_release', 'transit', 'backup', 'seasons'));
