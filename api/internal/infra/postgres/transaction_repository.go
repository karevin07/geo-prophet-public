package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository struct {
	pool *pgxpool.Pool
}

func NewTransactionRepository(ctx context.Context, databaseURL string) (*TransactionRepository, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	// pgx caches prepared statements, and after a few runs PostgreSQL may switch to a
	// generic plan that cannot see the parameters. For these analytical queries a date
	// range can match 1% or 100% of the table, so a generic plan estimated one row for an
	// all-years feed and took 1.4s instead of 0.4ms. Always plan with the actual values.
	config.ConnConfig.RuntimeParams["plan_cache_mode"] = "force_custom_plan"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &TransactionRepository{pool: pool}, nil
}

func (repository *TransactionRepository) Close() { repository.pool.Close() }

func (repository *TransactionRepository) Ping(ctx context.Context) error {
	return repository.pool.Ping(ctx)
}

// transactionSelectColumns lists the columns scanned by scanTransaction, ending with whether the
// row came from the latest release (always false outside the feed).
const transactionSelectColumns = `id, city_code, district, address, transaction_type, building_type, main_use,
        total_price, parking_price, building_price, building_area_ping, price_per_ping, rooms, halls, baths,
        floor_label, COALESCE(total_floors, 0), completed_on, transacted_on, remarks, special_reasons, season,
        ST_Y(location::geometry), ST_X(location::geometry), geocode_precision, COALESCE(community_key, '')`

func scanTransaction(row pgx.CollectableRow) (domain.Transaction, error) {
	var transaction domain.Transaction
	err := row.Scan(&transaction.ID, &transaction.CityCode, &transaction.District, &transaction.Address,
		&transaction.TransactionType, &transaction.BuildingType, &transaction.MainUse, &transaction.TotalPrice,
		&transaction.ParkingPrice, &transaction.BuildingPrice, &transaction.BuildingAreaPing, &transaction.PricePerPing,
		&transaction.Rooms, &transaction.Halls, &transaction.Baths, &transaction.FloorLabel, &transaction.TotalFloors,
		&transaction.CompletedAt, &transaction.TransactedAt, &transaction.Remarks, &transaction.SpecialReasons,
		&transaction.Season, &transaction.Lat, &transaction.Lng, &transaction.GeocodePrecision, &transaction.CommunityID,
		&transaction.IsNew)
	return transaction, err
}

func (repository *TransactionRepository) List(ctx context.Context, filter domain.TransactionFilter) ([]domain.Transaction, error) {
	where, args := whereClause(filter)
	args = append(args, filter.Limit)
	query := fmt.Sprintf(`SELECT %s, FALSE FROM transactions WHERE %s ORDER BY transacted_on DESC, id LIMIT $%d`,
		transactionSelectColumns, where, len(args))

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	transactions, err := pgx.CollectRows(rows, scanTransaction)
	if err != nil {
		return nil, fmt.Errorf("scan transactions: %w", err)
	}
	return transactions, nil
}

// feedSortColumns maps each sort to its column, the SQL type of cursor values and direction.
var feedSortColumns = map[domain.FeedSort]struct {
	column, cast string
	descending   bool
}{
	domain.SortDateDesc:         {"transacted_on", "DATE", true},
	domain.SortDateAsc:          {"transacted_on", "DATE", false},
	domain.SortPricePerPingDesc: {"price_per_ping", "DOUBLE PRECISION", true},
	domain.SortPricePerPingAsc:  {"price_per_ping", "DOUBLE PRECISION", false},
	domain.SortTotalPriceDesc:   {"total_price", "BIGINT", true},
}

// feedWhere adds the feed's region and release conditions to the shared filter.
func feedWhere(query domain.FeedQuery, latestRunIDs []int64) (string, []any) {
	where, args := whereClause(query.Filter)
	next := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if query.CityCode != "" {
		where += " AND city_code = " + next(query.CityCode)
	}
	if query.District != "" {
		where += " AND district = " + next(query.District)
	}
	if query.LatestReleaseOnly {
		where += " AND import_run_id = ANY(" + next(latestRunIDs) + ")"
	}
	return where, args
}

// feedSelect builds the ordered feed query; the cursor, if any, resumes after that row.
func feedSelect(query domain.FeedQuery, latestRunIDs []int64, limit int) (string, []any) {
	where, args := feedWhere(query, latestRunIDs)
	sort := feedSortColumns[query.Sort]
	direction, comparison := "ASC", ">"
	if sort.descending {
		direction, comparison = "DESC", "<"
	}
	if query.Cursor != nil {
		args = append(args, query.Cursor.Value, query.Cursor.ID)
		where += fmt.Sprintf(" AND (%s, id) %s ($%d::%s, $%d)", sort.column, comparison, len(args)-1, sort.cast, len(args))
	}
	args = append(args, latestRunIDs, limit)
	return fmt.Sprintf(`SELECT %s, COALESCE(import_run_id = ANY($%d), FALSE) FROM transactions WHERE %s
        ORDER BY %s %s, id %s LIMIT $%d`, transactionSelectColumns, len(args)-1, where, sort.column, direction, direction, len(args)), args
}

func (repository *TransactionRepository) Feed(ctx context.Context, query domain.FeedQuery, latestRunIDs []int64, limit int) ([]domain.Transaction, error) {
	sql, args := feedSelect(query, latestRunIDs, limit)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query feed: %w", err)
	}
	transactions, err := pgx.CollectRows(rows, scanTransaction)
	if err != nil {
		return nil, fmt.Errorf("scan feed: %w", err)
	}
	return transactions, nil
}

func (repository *TransactionRepository) FeedCounts(ctx context.Context, query domain.FeedQuery, latestRunIDs []int64) (int, int, error) {
	query.Cursor = nil
	where, args := feedWhere(query, latestRunIDs)
	args = append(args, latestRunIDs)
	var total, fresh int
	err := repository.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*), COUNT(*) FILTER (WHERE import_run_id = ANY($%d))
        FROM transactions WHERE %s`, len(args), where), args...).Scan(&total, &fresh)
	if err != nil {
		return 0, 0, fmt.Errorf("count feed: %w", err)
	}
	return total, fresh, nil
}

// ExportFeed streams up to limit rows in feed order to emit without holding them in memory.
func (repository *TransactionRepository) ExportFeed(ctx context.Context, query domain.FeedQuery, latestRunIDs []int64, limit int, emit func(domain.Transaction) error) error {
	query.Cursor = nil
	sql, args := feedSelect(query, latestRunIDs, limit)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("query export: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		transaction, err := scanTransaction(rows)
		if err != nil {
			return fmt.Errorf("scan export: %w", err)
		}
		if err = emit(transaction); err != nil {
			return err
		}
	}
	return rows.Err()
}

// LatestRelease returns the newest imported 10-day release and the import runs (one per
// city) that wrote it; rows written by those runs are "new in this release".
func (repository *TransactionRepository) LatestRelease(ctx context.Context) (string, []int64, error) {
	var label string
	err := repository.pool.QueryRow(ctx, `SELECT season FROM import_runs
        WHERE source = 'lvr' AND dataset = 'sales_release' AND status = 'succeeded'
        ORDER BY season DESC LIMIT 1`).Scan(&label)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("query latest release: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `SELECT id FROM import_runs
        WHERE source = 'lvr' AND dataset = 'sales_release' AND status = 'succeeded' AND season = $1`, label)
	if err != nil {
		return "", nil, fmt.Errorf("query latest release runs: %w", err)
	}
	runIDs, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return "", nil, fmt.Errorf("scan latest release runs: %w", err)
	}
	return label, runIDs, nil
}

// ForEachPricePoint streams every matching sale into the H3 accumulator. A LIMIT here
// would silently produce incomplete map prices when a viewport has many sales.
func (repository *TransactionRepository) ForEachPricePoint(ctx context.Context, filter domain.TransactionFilter, emit func(domain.PricePoint) error) error {
	where, args := whereClause(filter)
	query := fmt.Sprintf(`SELECT ST_Y(location::geometry), ST_X(location::geometry), total_price, building_price, building_area_ping
        FROM transactions WHERE %s`, where)

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query price points: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var point domain.PricePoint
		if err := rows.Scan(&point.Lat, &point.Lng, &point.TotalPrice, &point.BuildingPrice, &point.BuildingAreaPing); err != nil {
			return fmt.Errorf("scan price point: %w", err)
		}
		if err := emit(point); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (repository *TransactionRepository) MonthlyTrends(ctx context.Context, filter domain.TransactionFilter) ([]domain.TrendPoint, error) {
	where, args := whereClause(filter)
	query := fmt.Sprintf(`SELECT to_char(transacted_on, 'YYYY-MM') AS month,
        SUM(building_price)::DOUBLE PRECISION / SUM(building_area_ping), COUNT(*)
        FROM transactions WHERE %s GROUP BY month ORDER BY month`, where)

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query trends: %w", err)
	}
	trends, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.TrendPoint, error) {
		var point domain.TrendPoint
		err := row.Scan(&point.Month, &point.AvgPricePerPing, &point.Count)
		return point, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan trends: %w", err)
	}
	return trends, nil
}

func (repository *TransactionRepository) Summary(ctx context.Context, filter domain.TransactionFilter) (domain.MarketSummary, error) {
	where, args := whereClause(filter)
	query := fmt.Sprintf(`SELECT COUNT(*),
        COALESCE(SUM(building_price)::DOUBLE PRECISION / NULLIF(SUM(building_area_ping), 0), 0),
        COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY total_price), 0)
        FROM transactions WHERE %s`, where)
	var summary domain.MarketSummary
	if err := repository.pool.QueryRow(ctx, query, args...).Scan(&summary.Count, &summary.AvgPricePerPing, &summary.MedianTotalPrice); err != nil {
		return domain.MarketSummary{}, fmt.Errorf("query summary: %w", err)
	}
	return summary, nil
}

const analysisBucketWidth int64 = 100_000

func (repository *TransactionRepository) Analysis(ctx context.Context, filter domain.TransactionFilter) (domain.MarketAnalysis, error) {
	where, args := whereClause(filter)
	analysis := domain.MarketAnalysis{
		Monthly: []domain.MonthlyAnalysis{}, Categories: []domain.CategoryAnalysis{},
		AgeGroups: []domain.BreakdownAnalysis{}, AreaGroups: []domain.BreakdownAnalysis{}, Distribution: []domain.PriceBucket{},
	}

	monthlySQL := fmt.Sprintf(`SELECT to_char(date_trunc('month', transacted_on), 'YYYY-MM'), COUNT(*),
        SUM(building_price)::DOUBLE PRECISION / NULLIF(SUM(building_area_ping), 0),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY total_price), AVG(building_area_ping)
        FROM transactions WHERE %s GROUP BY date_trunc('month', transacted_on) ORDER BY date_trunc('month', transacted_on)`, where)
	rows, err := repository.pool.Query(ctx, monthlySQL, args...)
	if err != nil {
		return domain.MarketAnalysis{}, fmt.Errorf("query analysis months: %w", err)
	}
	analysis.Monthly, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.MonthlyAnalysis, error) {
		var point domain.MonthlyAnalysis
		err := row.Scan(&point.Month, &point.Count, &point.AvgPricePerPing, &point.MedianPricePerPing, &point.MedianTotalPrice, &point.AvgArea)
		return point, err
	})
	if err != nil {
		return domain.MarketAnalysis{}, fmt.Errorf("scan analysis months: %w", err)
	}

	categorySQL := fmt.Sprintf(`SELECT building_category, COUNT(*),
        SUM(building_price)::DOUBLE PRECISION / NULLIF(SUM(building_area_ping), 0),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY total_price)
        FROM transactions WHERE %s GROUP BY building_category
        ORDER BY array_position(ARRAY['住宅大樓','華廈','公寓','透天厝','其他'], building_category)`, where)
	rows, err = repository.pool.Query(ctx, categorySQL, args...)
	if err != nil {
		return domain.MarketAnalysis{}, fmt.Errorf("query analysis categories: %w", err)
	}
	analysis.Categories, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.CategoryAnalysis, error) {
		var group domain.CategoryAnalysis
		err := row.Scan(&group.Category, &group.Count, &group.AvgPricePerPing, &group.MedianPricePerPing, &group.MedianTotalPrice)
		return group, err
	})
	if err != nil {
		return domain.MarketAnalysis{}, fmt.Errorf("scan analysis categories: %w", err)
	}

	const ageBand = `CASE WHEN building_age IS NULL THEN 'unknown'
        WHEN building_age < 5 THEN 'under5' WHEN building_age < 10 THEN '5_9'
        WHEN building_age < 20 THEN '10_19' WHEN building_age < 30 THEN '20_29'
        ELSE '30plus' END`
	const areaBand = `CASE WHEN building_area_ping < 15 THEN 'under15'
        WHEN building_area_ping < 30 THEN '15_29' WHEN building_area_ping < 45 THEN '30_44'
        ELSE '45plus' END`
	if analysis.AgeGroups, err = repository.analysisBreakdown(ctx, where, args, ageBand); err != nil {
		return domain.MarketAnalysis{}, fmt.Errorf("query age breakdown: %w", err)
	}
	if analysis.AreaGroups, err = repository.analysisBreakdown(ctx, where, args, areaBand); err != nil {
		return domain.MarketAnalysis{}, fmt.Errorf("query area breakdown: %w", err)
	}
	sortBreakdown(analysis.AgeGroups, []string{"under5", "5_9", "10_19", "20_29", "30plus", "unknown"})
	sortBreakdown(analysis.AreaGroups, []string{"under15", "15_29", "30_44", "45plus"})

	distributionSQL := fmt.Sprintf(`SELECT (floor(price_per_ping / %d) * %d)::BIGINT AS lower_bound, COUNT(*)
        FROM transactions WHERE %s GROUP BY lower_bound ORDER BY lower_bound`, analysisBucketWidth, analysisBucketWidth, where)
	rows, err = repository.pool.Query(ctx, distributionSQL, args...)
	if err != nil {
		return domain.MarketAnalysis{}, fmt.Errorf("query price distribution: %w", err)
	}
	analysis.Distribution, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.PriceBucket, error) {
		var bucket domain.PriceBucket
		err := row.Scan(&bucket.MinPricePerPing, &bucket.Count)
		bucket.MaxPricePerPing = bucket.MinPricePerPing + analysisBucketWidth
		return bucket, err
	})
	if err != nil {
		return domain.MarketAnalysis{}, fmt.Errorf("scan price distribution: %w", err)
	}
	return analysis, nil
}

func sortBreakdown(groups []domain.BreakdownAnalysis, order []string) {
	slices.SortFunc(groups, func(a, b domain.BreakdownAnalysis) int {
		return slices.Index(order, a.Key) - slices.Index(order, b.Key)
	})
}

func (repository *TransactionRepository) analysisBreakdown(ctx context.Context, where string, args []any, band string) ([]domain.BreakdownAnalysis, error) {
	query := fmt.Sprintf(`SELECT %s AS band, COUNT(*),
        SUM(building_price)::DOUBLE PRECISION / NULLIF(SUM(building_area_ping), 0),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY total_price)
        FROM transactions WHERE %s GROUP BY band ORDER BY band`, band, where)
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.BreakdownAnalysis, error) {
		var group domain.BreakdownAnalysis
		err := row.Scan(&group.Key, &group.Count, &group.AvgPricePerPing, &group.MedianPricePerPing, &group.MedianTotalPrice)
		return group, err
	})
}

func (repository *TransactionRepository) DistrictRankings(ctx context.Context, query domain.DistrictComparisonQuery) ([]domain.DistrictRank, error) {
	where, args := whereClause(query.Filter)
	args = append(args, query.CityCode)
	sql := fmt.Sprintf(`SELECT district, COUNT(*),
        SUM(building_price)::DOUBLE PRECISION / NULLIF(SUM(building_area_ping), 0),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY total_price)
        FROM transactions WHERE %s AND city_code = $%d
        GROUP BY district ORDER BY 4 DESC, district`, where, len(args))
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query district rankings: %w", err)
	}
	ranks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.DistrictRank, error) {
		var rank domain.DistrictRank
		err := row.Scan(&rank.District, &rank.Count, &rank.AvgPricePerPing, &rank.MedianPricePerPing, &rank.MedianTotalPrice)
		return rank, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan district rankings: %w", err)
	}
	if ranks == nil {
		return []domain.DistrictRank{}, nil
	}
	return ranks, nil
}

func (repository *TransactionRepository) DistrictTrends(ctx context.Context, query domain.DistrictComparisonQuery) ([]domain.DistrictTrend, error) {
	if len(query.Districts) == 0 {
		return []domain.DistrictTrend{}, nil
	}
	where, args := whereClause(query.Filter)
	args = append(args, query.CityCode, query.Districts)
	sql := fmt.Sprintf(`SELECT district, to_char(date_trunc('month', transacted_on), 'YYYY-MM'), COUNT(*),
        SUM(building_price)::DOUBLE PRECISION / NULLIF(SUM(building_area_ping), 0)
        FROM transactions WHERE %s AND city_code = $%d AND district = ANY($%d)
        GROUP BY district, date_trunc('month', transacted_on)
        ORDER BY date_trunc('month', transacted_on), district`, where, len(args)-1, len(args))
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query district trends: %w", err)
	}
	trends, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.DistrictTrend, error) {
		var point domain.DistrictTrend
		err := row.Scan(&point.District, &point.Month, &point.Count, &point.AvgPricePerPing)
		return point, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan district trends: %w", err)
	}
	if trends == nil {
		return []domain.DistrictTrend{}, nil
	}
	return trends, nil
}

func (repository *TransactionRepository) DistrictYoYTotals(ctx context.Context, query domain.DistrictComparisonQuery, previousFrom, currentFrom, currentTo time.Time) (map[string]domain.DistrictYoYTotals, error) {
	filter := query.Filter
	filter.From, filter.To = &previousFrom, &currentTo
	where, args := whereClause(filter)
	args = append(args, query.CityCode, currentFrom)
	sql := fmt.Sprintf(`SELECT district,
        COUNT(*) FILTER (WHERE transacted_on >= $%d), COUNT(*) FILTER (WHERE transacted_on < $%d),
        COALESCE(SUM(building_price) FILTER (WHERE transacted_on >= $%d), 0),
        COALESCE(SUM(building_price) FILTER (WHERE transacted_on < $%d), 0),
        COALESCE(SUM(building_area_ping) FILTER (WHERE transacted_on >= $%d), 0),
        COALESCE(SUM(building_area_ping) FILTER (WHERE transacted_on < $%d), 0)
        FROM transactions WHERE %s AND city_code = $%d GROUP BY district`,
		len(args), len(args), len(args), len(args), len(args), len(args), where, len(args)-1)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query district yoy: %w", err)
	}
	defer rows.Close()
	totals := make(map[string]domain.DistrictYoYTotals)
	for rows.Next() {
		var district string
		var value domain.DistrictYoYTotals
		if err = rows.Scan(&district, &value.CurrentCount, &value.PreviousCount, &value.CurrentBuildingPrice,
			&value.PreviousBuildingPrice, &value.CurrentArea, &value.PreviousArea); err != nil {
			return nil, fmt.Errorf("scan district yoy: %w", err)
		}
		totals[district] = value
	}
	return totals, rows.Err()
}

func (repository *TransactionRepository) DistrictBoundaries(ctx context.Context, cityCode string) ([]domain.DistrictBoundary, error) {
	rows, err := repository.pool.Query(ctx, `SELECT district, cells, coordinates FROM district_boundaries
        WHERE city_code = $1 ORDER BY district`, cityCode)
	if err != nil {
		return nil, fmt.Errorf("query district boundaries: %w", err)
	}
	boundaries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.DistrictBoundary, error) {
		var boundary domain.DistrictBoundary
		err := row.Scan(&boundary.District, &boundary.Cells, &boundary.Coordinates)
		return boundary, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan district boundaries: %w", err)
	}
	if boundaries == nil {
		return []domain.DistrictBoundary{}, nil
	}
	return boundaries, nil
}

func (repository *TransactionRepository) CellRows(ctx context.Context, filter domain.TransactionFilter) ([]domain.CellRow, error) {
	where, args := whereClause(filter)
	args = append(args, filter.Limit)
	query := fmt.Sprintf(`SELECT city_code, district, address, ST_Y(location::geometry), ST_X(location::geometry),
        transacted_on, total_price, building_price, building_area_ping, building_age, COALESCE(community_key, '')
        FROM transactions WHERE %s LIMIT $%d`, where, len(args))
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query cell rows: %w", err)
	}
	cellRows, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.CellRow, error) {
		var cell domain.CellRow
		err := row.Scan(&cell.CityCode, &cell.District, &cell.Address, &cell.Lat, &cell.Lng, &cell.TransactedAt,
			&cell.TotalPrice, &cell.BuildingPrice, &cell.AreaPing, &cell.BuildingAge, &cell.CommunityID)
		return cell, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan cell rows: %w", err)
	}
	return cellRows, nil
}

func (repository *TransactionRepository) ImportedSales(ctx context.Context) ([]string, []string, error) {
	rows, err := repository.pool.Query(ctx, `SELECT DISTINCT dataset, season FROM import_runs
        WHERE source = 'lvr' AND status = 'succeeded' AND dataset IN ('sales', 'sales_release')`)
	if err != nil {
		return nil, nil, fmt.Errorf("query imported sales: %w", err)
	}
	defer rows.Close()
	var seasons, releases []string
	for rows.Next() {
		var dataset, season string
		if err = rows.Scan(&dataset, &season); err != nil {
			return nil, nil, fmt.Errorf("scan imported sales: %w", err)
		}
		if dataset == "sales" {
			seasons = append(seasons, season)
		} else {
			releases = append(releases, season)
		}
	}
	return seasons, releases, rows.Err()
}

func (repository *TransactionRepository) FindAddresses(ctx context.Context, query domain.PlaceQuery) ([]domain.PlaceMatch, error) {
	sql := `SELECT city_code, district_code, road, lane, alley, number, sub_number, lat, lng FROM address_points
        WHERE road = $1 AND lane = $2 AND alley = $3 AND number = $4 AND sub_number = $5
          AND ($6 = '' OR city_code = $6) AND (cardinality($7::TEXT[]) = 0 OR district_code = ANY($7))
        ORDER BY city_code, district_code LIMIT $8`
	return repository.placeMatches(ctx, sql, query.Road, query.Lane, query.Alley, query.Number, query.SubNumber,
		query.CityCode, districtCodes(query), query.Limit)
}

func (repository *TransactionRepository) FindRoads(ctx context.Context, query domain.PlaceQuery) ([]domain.PlaceMatch, error) {
	sql := `SELECT city_code, district_code, road, '', '', 0, 0, AVG(lat), AVG(lng) FROM address_points
        WHERE road LIKE '%' || $1 || '%' ESCAPE '\'
          AND ($2 = '' OR city_code = $2) AND (cardinality($3::TEXT[]) = 0 OR district_code = ANY($3))
        GROUP BY city_code, district_code, road
        ORDER BY road = $4 DESC, COUNT(*) DESC LIMIT $5`
	escaped := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(query.Road)
	return repository.placeMatches(ctx, sql, escaped, query.CityCode, districtCodes(query), query.Road, query.Limit)
}

func (repository *TransactionRepository) DistrictCenter(ctx context.Context, cityCode, districtCode string) (float64, float64, bool, error) {
	var lat, lng *float64
	err := repository.pool.QueryRow(ctx, `SELECT AVG(lat), AVG(lng) FROM address_points
        WHERE city_code = $1 AND district_code = $2`, cityCode, districtCode).Scan(&lat, &lng)
	if err != nil {
		return 0, 0, false, fmt.Errorf("query district center: %w", err)
	}
	if lat == nil || lng == nil {
		return 0, 0, false, nil
	}
	return *lat, *lng, true, nil
}

func (repository *TransactionRepository) placeMatches(ctx context.Context, sql string, args ...any) ([]domain.PlaceMatch, error) {
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query places: %w", err)
	}
	matches, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.PlaceMatch, error) {
		var match domain.PlaceMatch
		err := row.Scan(&match.CityCode, &match.DistrictCode, &match.Road, &match.Lane, &match.Alley,
			&match.Number, &match.SubNumber, &match.Lat, &match.Lng)
		return match, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan places: %w", err)
	}
	return matches, nil
}

func districtCodes(query domain.PlaceQuery) []string {
	if query.DistrictCodes == nil {
		return []string{}
	}
	return query.DistrictCodes
}

// whereClause builds the shared filter; it always requires a location because every
// endpoint is scoped to the map viewport.
func whereClause(filter domain.TransactionFilter) (string, []any) {
	conditions := []string{"location IS NOT NULL", "total_price BETWEEN $1 AND $2", "building_area_ping BETWEEN $3 AND $4"}
	args := []any{filter.MinPrice, filter.MaxPrice, filter.MinArea, filter.MaxArea}
	next := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	if !filter.IncludeSpecial {
		conditions = append(conditions, "NOT is_special")
	}
	if filter.PreciseLocation {
		conditions = append(conditions, "geocode_precision <> 'district'")
	}
	if filter.ExcludeNewBuildings {
		conditions = append(conditions, "(building_age IS NULL OR building_age >= "+next(domain.NewBuildingYears)+")")
	}
	if len(filter.Categories) > 0 {
		// building_category (migration 000005) already maps unknown types to 其他.
		conditions = append(conditions, "building_category = ANY("+next(filter.Categories)+")")
	}
	if filter.Bounds != nil {
		envelope := fmt.Sprintf("ST_MakeEnvelope(%s, %s, %s, %s, 4326)",
			next(filter.Bounds.West), next(filter.Bounds.South), next(filter.Bounds.East), next(filter.Bounds.North))
		conditions = append(conditions, withinBounds("location", envelope))
	}
	if filter.From != nil {
		conditions = append(conditions, "transacted_on >= "+next(*filter.From))
	}
	// A sale dated after today is a typo in the source (flagged invalid_date), e.g.
	// 2026-10-24 registered in 115S1, so the window ends today at the latest; the default
	// one runs to the end of this month.
	to := today()
	if filter.To != nil && filter.To.Before(to) {
		to = *filter.To
	}
	conditions = append(conditions, "transacted_on <= "+next(to))
	return strings.Join(conditions, " AND "), args
}

// taipei is Taiwan's time zone, where sales are dated.
var taipei = time.FixedZone("Asia/Taipei", 8*60*60)

// today is the current date in Taiwan, at midnight UTC like the dates sales carry.
func today() time.Time {
	year, month, day := time.Now().In(taipei).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// withinBounds keeps the geography column inside the longitude/latitude envelope. && on
// geography uses the GiST index but compares geocentric boxes, which reach past a box's
// edges (by 0.008° on a 0.06° viewport); && on geometry then trims to the box itself.
func withinBounds(column, envelope string) string {
	return fmt.Sprintf("%[1]s && %[2]s::GEOGRAPHY AND %[1]s::GEOMETRY && %[2]s", column, envelope)
}
