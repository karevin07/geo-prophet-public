package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

const importRunStatusColumns = `id, source, dataset, season, city_code, status,
    rows_read, rows_skipped, rows_upserted, rows_geocode_failed, started_at, finished_at, COALESCE(error, '')`

func (repository *TransactionRepository) DataStatusSnapshot(ctx context.Context) (domain.DataStatusSnapshot, error) {
	var snapshot domain.DataStatusSnapshot
	var err error
	snapshot.RecentRuns, err = repository.importRunStatuses(ctx,
		`SELECT `+importRunStatusColumns+` FROM import_runs ORDER BY started_at DESC, id DESC LIMIT 30`)
	if err != nil {
		return snapshot, err
	}
	// The newest period succeeded, not the latest to finish: a backfill of old seasons
	// finishes after the current one was imported.
	snapshot.LastSuccessfulRuns, err = repository.importRunStatuses(ctx,
		`SELECT `+importRunStatusColumns+` FROM (SELECT DISTINCT ON (source, dataset, city_code) *
         FROM import_runs WHERE status = 'succeeded'
           AND (dataset NOT IN ('sales', 'rentals', 'presales') OR COALESCE((detail->>'missing_file')::integer, 0) = 0)
         ORDER BY source, dataset, city_code, season DESC, finished_at DESC, id DESC) latest
         ORDER BY source, dataset, city_code`)
	if err != nil {
		return snapshot, err
	}
	snapshot.LatestRuns, err = repository.importRunStatuses(ctx,
		`SELECT `+importRunStatusColumns+` FROM (SELECT DISTINCT ON (source, dataset, city_code) *
         FROM import_runs ORDER BY source, dataset, city_code, started_at DESC, id DESC) latest
         ORDER BY source, dataset, city_code`)
	if err != nil {
		return snapshot, err
	}
	snapshot.QualityRuns, err = repository.qualityRuns(ctx)
	if err != nil {
		return snapshot, err
	}

	rows, err := repository.pool.Query(ctx, `SELECT cities.code, COUNT(t.id),
        COUNT(t.id) FILTER (WHERE t.location IS NOT NULL),
        COUNT(t.id) FILTER (WHERE t.transacted_on > CURRENT_DATE),
        MAX(t.transacted_on) FILTER (WHERE t.transacted_on <= CURRENT_DATE)
        FROM (VALUES ('a'), ('f'), ('h')) AS cities(code)
        LEFT JOIN transactions t ON t.city_code = cities.code
        GROUP BY cities.code ORDER BY cities.code`)
	if err != nil {
		return snapshot, fmt.Errorf("query city data counts: %w", err)
	}
	for rows.Next() {
		var city domain.CityDataCount
		if err = rows.Scan(&city.CityCode, &city.Transactions, &city.Located, &city.FutureDated, &city.LatestSaleDate); err != nil {
			break
		}
		snapshot.Cities = append(snapshot.Cities, city)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return snapshot, fmt.Errorf("read city data counts: %w", err)
	}

	rows, err = repository.pool.Query(ctx, `SELECT cities.code, COUNT(r.id),
        COUNT(r.id) FILTER (WHERE r.location IS NOT NULL),
        MAX(r.leased_on) FILTER (WHERE r.leased_on <= CURRENT_DATE)
        FROM (VALUES ('a'), ('f'), ('h')) AS cities(code)
        LEFT JOIN rentals r ON r.city_code = cities.code
        GROUP BY cities.code ORDER BY cities.code`)
	if err != nil {
		return snapshot, fmt.Errorf("query city rental counts: %w", err)
	}
	for index := 0; rows.Next(); index++ {
		var code string
		var count, located int64
		var latest *time.Time
		if err = rows.Scan(&code, &count, &located, &latest); err != nil {
			break
		}
		if index < len(snapshot.Cities) && snapshot.Cities[index].CityCode == code {
			snapshot.Cities[index].Rentals, snapshot.Cities[index].RentalsLocated, snapshot.Cities[index].LatestLeaseDate = count, located, latest
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return snapshot, fmt.Errorf("read city rental counts: %w", err)
	}

	rows, err = repository.pool.Query(ctx, `SELECT city_code, season FROM import_runs
        WHERE source = 'lvr' AND dataset = 'sales' AND status = 'succeeded'
          AND COALESCE((detail->>'missing_file')::integer, 0) = 0
        GROUP BY city_code, season ORDER BY city_code, season`)
	if err != nil {
		return snapshot, fmt.Errorf("query imported seasons: %w", err)
	}
	snapshot.ImportedSeasons = make(map[string][]string)
	for rows.Next() {
		var city, season string
		if err = rows.Scan(&city, &season); err != nil {
			break
		}
		snapshot.ImportedSeasons[city] = append(snapshot.ImportedSeasons[city], season)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return snapshot, fmt.Errorf("read imported seasons: %w", err)
	}
	return snapshot, nil
}

// qualityRuns keeps two distinct published periods per city and 實價登錄 dataset.
// Reimports of the same period must not become their own comparison baseline.
func (repository *TransactionRepository) qualityRuns(ctx context.Context) ([]domain.QualityRun, error) {
	rows, err := repository.pool.Query(ctx, `WITH periods AS (
        SELECT DISTINCT ON (dataset, city_code, season) id, dataset, season, city_code,
            rows_read, rows_skipped, rows_upserted, rows_geocode_failed, detail, finished_at
        FROM import_runs
        WHERE source = 'lvr' AND dataset IN ('sales', 'sales_release', 'rentals', 'rentals_release', 'presales', 'presales_release') AND status = 'succeeded'
          AND finished_at IS NOT NULL AND COALESCE((detail->>'missing_file')::integer, 0) = 0
        ORDER BY dataset, city_code, season, finished_at DESC, id DESC
    ), ranked AS (
        SELECT *, row_number() OVER (PARTITION BY dataset, city_code ORDER BY season DESC) AS rank
        FROM periods
    )
    SELECT id, dataset, season, city_code, rows_read, rows_skipped, rows_upserted,
        rows_geocode_failed, detail, finished_at FROM ranked WHERE rank <= 2`)
	if err != nil {
		return nil, fmt.Errorf("query quality runs: %w", err)
	}
	defer rows.Close()
	runs := []domain.QualityRun{}
	for rows.Next() {
		var run domain.QualityRun
		if err := rows.Scan(&run.ID, &run.Dataset, &run.Season, &run.CityCode, &run.RowsRead,
			&run.RowsSkipped, &run.RowsUpserted, &run.RowsGeocodeFailed, &run.Detail, &run.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan quality run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (repository *TransactionRepository) importRunStatuses(ctx context.Context, query string) ([]domain.ImportRunStatus, error) {
	rows, err := repository.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query import runs: %w", err)
	}
	defer rows.Close()
	result := []domain.ImportRunStatus{}
	for rows.Next() {
		run, err := scanImportRunStatus(rows)
		if err != nil {
			return nil, fmt.Errorf("scan import run: %w", err)
		}
		result = append(result, run)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("read import runs: %w", err)
	}
	return result, nil
}

func scanImportRunStatus(row pgx.Row) (domain.ImportRunStatus, error) {
	var run domain.ImportRunStatus
	err := row.Scan(&run.ID, &run.Source, &run.Dataset, &run.Season, &run.CityCode,
		&run.Status, &run.RowsRead, &run.RowsSkipped, &run.RowsUpserted,
		&run.RowsGeocodeFailed, &run.StartedAt, &run.FinishedAt, &run.Error)
	return run, err
}

// DatabaseUsage reports the database size, its sales and its largest tables.
func (repository *TransactionRepository) DatabaseUsage(ctx context.Context) (domain.DatabaseUsage, error) {
	var usage domain.DatabaseUsage
	if err := repository.pool.QueryRow(ctx, `SELECT pg_database_size(current_database()), (SELECT COUNT(*) FROM transactions)`).
		Scan(&usage.Bytes, &usage.Transactions); err != nil {
		return usage, fmt.Errorf("query database size: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `SELECT relname, pg_total_relation_size(relid) FROM pg_statio_user_tables
        ORDER BY 2 DESC, 1`)
	if err != nil {
		return usage, fmt.Errorf("query table sizes: %w", err)
	}
	usage.Tables, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.TableSize, error) {
		var table domain.TableSize
		err := row.Scan(&table.Name, &table.Bytes)
		return table, err
	})
	if err != nil {
		return usage, fmt.Errorf("scan table sizes: %w", err)
	}
	return usage, nil
}
