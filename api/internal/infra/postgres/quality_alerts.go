package postgres

import (
	"context"
	"fmt"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// QualitySnapshot reads only import history, so the notifier need not count the
// entire transactions table every time it checks for an alert.
func (repository *TransactionRepository) QualitySnapshot(ctx context.Context) (domain.DataStatusSnapshot, error) {
	var snapshot domain.DataStatusSnapshot
	var err error
	snapshot.QualityRuns, err = repository.qualityRuns(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.LatestRuns, err = repository.importRunStatuses(ctx,
		`SELECT `+importRunStatusColumns+` FROM (SELECT DISTINCT ON (source, dataset, city_code) *
         FROM import_runs WHERE source = 'lvr' AND dataset IN ('sales', 'sales_release', 'rentals', 'rentals_release', 'presales', 'presales_release')
         ORDER BY source, dataset, city_code, started_at DESC, id DESC) latest`)
	return snapshot, err
}

func (repository *TransactionRepository) QueueQualityAlerts(ctx context.Context, alerts []domain.QualityAlert) error {
	for _, alert := range alerts {
		_, err := repository.pool.Exec(ctx, `INSERT INTO quality_alert_deliveries
            (run_id, code, severity, city_code, dataset, season) VALUES ($1, $2, $3, $4, $5, $6)
            ON CONFLICT (run_id, code) DO NOTHING`, alert.RunID, alert.Code, alert.Severity, alert.CityCode, alert.Dataset, alert.Season)
		if err != nil {
			return fmt.Errorf("queue quality alert: %w", err)
		}
	}
	return nil
}

// ResolveInactiveQualityAlerts prevents a failed, later-recovered condition from
// occupying the pending queue forever. Delivered alerts remain in the audit log.
func (repository *TransactionRepository) ResolveInactiveQualityAlerts(ctx context.Context, active []domain.QualityAlert) error {
	runIDs := make([]int64, 0, len(active))
	codes := make([]string, 0, len(active))
	for _, alert := range active {
		runIDs, codes = append(runIDs, alert.RunID), append(codes, alert.Code)
	}
	_, err := repository.pool.Exec(ctx, `UPDATE quality_alert_deliveries AS delivery SET resolved_at = NOW()
        WHERE sent_at IS NULL AND resolved_at IS NULL AND NOT EXISTS (
            SELECT 1 FROM unnest($1::bigint[], $2::text[]) AS active_alert(run_id, code)
            WHERE active_alert.run_id = delivery.run_id AND active_alert.code = delivery.code
        )`, runIDs, codes)
	return err
}

func (repository *TransactionRepository) PendingQualityAlerts(ctx context.Context) ([]domain.QualityAlert, error) {
	rows, err := repository.pool.Query(ctx, `SELECT run_id, code, severity, city_code, dataset, season
        FROM quality_alert_deliveries WHERE sent_at IS NULL AND resolved_at IS NULL AND next_attempt_at <= NOW()
        ORDER BY next_attempt_at, run_id LIMIT 20`)
	if err != nil {
		return nil, fmt.Errorf("query pending quality alerts: %w", err)
	}
	alerts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.QualityAlert, error) {
		var alert domain.QualityAlert
		err := row.Scan(&alert.RunID, &alert.Code, &alert.Severity, &alert.CityCode, &alert.Dataset, &alert.Season)
		return alert, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan pending quality alerts: %w", err)
	}
	return alerts, nil
}

func (repository *TransactionRepository) FinishQualityAlert(ctx context.Context, alert domain.QualityAlert, deliveryErr error) error {
	if deliveryErr == nil {
		_, err := repository.pool.Exec(ctx, `UPDATE quality_alert_deliveries
            SET sent_at = NOW(), attempts = attempts + 1, last_error = ''
			WHERE run_id = $1 AND code = $2 AND sent_at IS NULL AND resolved_at IS NULL`, alert.RunID, alert.Code)
		return err
	}
	// Exponential backoff capped at an hour. Never persist the response body, which
	// could contain private data returned by a third-party webhook.
	_, err := repository.pool.Exec(ctx, `UPDATE quality_alert_deliveries
        SET attempts = attempts + 1,
            next_attempt_at = NOW() + make_interval(secs => LEAST(3600.0, 60.0 * power(2.0, LEAST(attempts, 6)))::double precision),
			last_error = $3 WHERE run_id = $1 AND code = $2 AND sent_at IS NULL AND resolved_at IS NULL`, alert.RunID, alert.Code, deliveryErr.Error())
	return err
}
