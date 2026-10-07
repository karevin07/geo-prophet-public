-- One notification per observed import run and quality rule. A failed webhook stays
-- pending for retry; a successful one is never sent again on a page refresh.
CREATE TABLE IF NOT EXISTS quality_alert_deliveries (
    run_id BIGINT NOT NULL REFERENCES import_runs (id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    severity TEXT NOT NULL,
    city_code TEXT NOT NULL,
    dataset TEXT NOT NULL,
    season TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (run_id, code)
);
CREATE INDEX IF NOT EXISTS quality_alert_deliveries_pending_idx
    ON quality_alert_deliveries (next_attempt_at) WHERE sent_at IS NULL AND resolved_at IS NULL;
