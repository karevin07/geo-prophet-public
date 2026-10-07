package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

func (repository *TransactionRepository) ImportedSalesByCity(ctx context.Context, city string) ([]string, []string, error) {
	return repository.importedPeriods(ctx, city, "sales")
}

// InvestmentRentAggregates matches the workbench sale cohort and period. Only
// non-special new whole-home leases outside social housing are included.
func (repository *TransactionRepository) InvestmentRentAggregates(ctx context.Context, query domain.InvestmentQuery, from, to time.Time) ([]domain.InvestmentRentAggregate, error) {
	const sql = `SELECT GROUPING(district), COALESCE(district, ''), COUNT(*),
		percentile_cont(0.5) WITHIN GROUP (ORDER BY rent_per_ping)
		FROM rentals
		WHERE city_code = $1 AND building_category = $2
		AND building_area_ping >= $3 AND building_area_ping < $4
		AND building_age >= $5 AND building_age < $6
		AND leased_on >= $7 AND leased_on < $8
		AND main_use = '住家用'
		AND whole_unit AND NOT renewal AND NOT is_special AND NOT social_housing
		GROUP BY ROLLUP(district)
		ORDER BY GROUPING(district) DESC, district`
	rows, err := repository.pool.Query(ctx, sql, query.CityCode, query.Category, query.MinArea, query.MaxArea, query.MinAge, query.MaxAge, from, to)
	if err != nil {
		return nil, fmt.Errorf("query investment rents: %w", err)
	}
	defer rows.Close()
	result := []domain.InvestmentRentAggregate{}
	for rows.Next() {
		var rollup int
		var area domain.InvestmentRentAggregate
		if err := rows.Scan(&rollup, &area.District, &area.Count, &area.MedianRentPerPing); err != nil {
			return nil, fmt.Errorf("scan investment rent: %w", err)
		}
		if rollup == 1 {
			area.District = ""
		}
		result = append(result, area)
	}
	return result, rows.Err()
}

// InvestmentMonthlyCounts returns sparse observed sale counts. The use case
// fills missing months with zero while warning that import gaps may exist.
func (repository *TransactionRepository) InvestmentMonthlyCounts(ctx context.Context, query domain.InvestmentQuery, from, to time.Time) ([]domain.InvestmentMonthlyCount, error) {
	const sql = `SELECT district, to_char(date_trunc('month', transacted_on), 'YYYY-MM'), COUNT(*)
		FROM transactions
		WHERE city_code = $1 AND building_category = $2
		AND building_area_ping >= $3 AND building_area_ping < $4
		AND building_age >= $5 AND building_age < $6
		AND transacted_on >= $7 AND transacted_on < $8
		AND main_use = '住家用' AND transaction_type LIKE '房地%'
		AND NOT is_special
		GROUP BY district, date_trunc('month', transacted_on)
		ORDER BY district, date_trunc('month', transacted_on)`
	rows, err := repository.pool.Query(ctx, sql, query.CityCode, query.Category, query.MinArea, query.MaxArea, query.MinAge, query.MaxAge, from, to)
	if err != nil {
		return nil, fmt.Errorf("query investment monthly counts: %w", err)
	}
	defer rows.Close()
	result := []domain.InvestmentMonthlyCount{}
	for rows.Next() {
		var row domain.InvestmentMonthlyCount
		if err := rows.Scan(&row.District, &row.Month, &row.Count); err != nil {
			return nil, fmt.Errorf("scan investment monthly count: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// importedPeriods returns the seasons and 10-day releases of a 實價登錄 dataset imported
// for a city.
func (repository *TransactionRepository) importedPeriods(ctx context.Context, city, dataset string) ([]string, []string, error) {
	rows, err := repository.pool.Query(ctx, `SELECT DISTINCT dataset, season FROM import_runs
        WHERE source = 'lvr' AND city_code = $1 AND status = 'succeeded'
          AND dataset IN ($2, $2 || '_release')
          AND COALESCE((detail->>'missing_file')::integer, 0) = 0`, city, dataset)
	if err != nil {
		return nil, nil, fmt.Errorf("query city import coverage: %w", err)
	}
	defer rows.Close()
	var seasons, releases []string
	for rows.Next() {
		var imported, season string
		if err := rows.Scan(&imported, &season); err != nil {
			return nil, nil, fmt.Errorf("scan city import coverage: %w", err)
		}
		if imported == dataset {
			seasons = append(seasons, season)
		} else {
			releases = append(releases, season)
		}
	}
	return seasons, releases, rows.Err()
}

// InvestmentAggregates scans the two 12-month windows once. ROLLUP adds the
// city-wide peer cohort without a second query; no geocoding requirement is
// imposed because the district is known even when a street address is not.
func (repository *TransactionRepository) InvestmentAggregates(ctx context.Context, query domain.InvestmentQuery, window domain.InvestmentWindow) ([]domain.InvestmentAggregate, error) {
	const sql = `SELECT GROUPING(district), COALESCE(district, ''),
        COUNT(*) FILTER (WHERE transacted_on >= $8),
        COUNT(*) FILTER (WHERE transacted_on < $8),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping)
          FILTER (WHERE transacted_on >= $8),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping)
          FILTER (WHERE transacted_on < $8)
        FROM transactions
        WHERE city_code = $1 AND building_category = $2
          AND building_area_ping >= $3 AND building_area_ping < $4
          AND building_age >= $5 AND building_age < $6
          AND transacted_on >= $7 AND transacted_on < $9
          AND main_use = '住家用' AND transaction_type LIKE '房地%'
          AND NOT is_special
        GROUP BY ROLLUP(district)
        ORDER BY GROUPING(district) DESC, district`
	rows, err := repository.pool.Query(ctx, sql, query.CityCode, query.Category,
		query.MinArea, query.MaxArea, query.MinAge, query.MaxAge,
		window.PreviousFrom, window.CurrentFrom, window.EndExclusive)
	if err != nil {
		return nil, fmt.Errorf("query investment areas: %w", err)
	}
	defer rows.Close()
	result := []domain.InvestmentAggregate{}
	for rows.Next() {
		var rollup int
		var area domain.InvestmentAggregate
		if err := rows.Scan(&rollup, &area.District, &area.CurrentCount, &area.PreviousCount,
			&area.MedianPricePerPing, &area.PreviousMedianPricePerPing); err != nil {
			return nil, fmt.Errorf("scan investment area: %w", err)
		}
		if rollup == 1 {
			area.District = ""
		}
		result = append(result, area)
	}
	return result, rows.Err()
}
