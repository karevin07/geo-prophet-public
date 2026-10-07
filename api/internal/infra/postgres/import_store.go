package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ImportStore implements importer.Store.
type ImportStore struct {
	pool *pgxpool.Pool
}

func NewImportStore(ctx context.Context, databaseURL string) (*ImportStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &ImportStore{pool: pool}, nil
}

func (store *ImportStore) Close() { store.pool.Close() }

func (store *ImportStore) StartRun(ctx context.Context, run domain.ImportRun) (int64, error) {
	var runID int64
	err := store.pool.QueryRow(ctx, `INSERT INTO import_runs (source, dataset, season, city_code, status)
        VALUES ($1, $2, $3, $4, 'running') RETURNING id`, run.Source, run.Dataset, run.Season, run.CityCode).Scan(&runID)
	if err != nil {
		return 0, fmt.Errorf("start import run: %w", err)
	}
	return runID, nil
}

func (store *ImportStore) FinishRun(ctx context.Context, runID int64, stats domain.ImportStats, runErr error) error {
	status, message := "succeeded", (*string)(nil)
	if runErr != nil {
		status = "failed"
		text := runErr.Error()
		message = &text
	}
	detail, err := json.Marshal(stats.Detail)
	if err != nil {
		return fmt.Errorf("encode import detail: %w", err)
	}
	_, err = store.pool.Exec(ctx, `UPDATE import_runs SET status = $2, rows_read = $3, rows_skipped = $4,
        rows_upserted = $5, rows_geocode_failed = $6, detail = $7, error = $8, finished_at = NOW() WHERE id = $1`,
		runID, status, stats.RowsRead, stats.RowsSkipped, stats.RowsUpserted, stats.RowsGeocodeFailed, detail, message)
	if err != nil {
		return fmt.Errorf("finish import run: %w", err)
	}
	return nil
}

func (store *ImportStore) LastSucceeded(ctx context.Context, run domain.ImportRun) (time.Time, bool, error) {
	var finished time.Time
	err := store.pool.QueryRow(ctx, `SELECT finished_at FROM import_runs
        WHERE source = $1 AND dataset = $2 AND city_code = $3 AND ($4 = '' OR season = $4) AND status = 'succeeded'
        ORDER BY finished_at DESC LIMIT 1`, run.Source, run.Dataset, run.CityCode, run.Season).Scan(&finished)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("query last import run: %w", err)
	}
	return finished, true, nil
}

func (store *ImportStore) ReplaceAddressPoints(ctx context.Context, cityCode string, points []addresspoint.Point) error {
	return pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM address_points WHERE city_code = $1`, cityCode); err != nil {
			return fmt.Errorf("delete address points: %w", err)
		}
		rows := pgx.CopyFromSlice(len(points), func(index int) ([]any, error) {
			point := points[index]
			return []any{cityCode, point.DistrictCode, point.Road, point.Lane, point.Alley, point.Number, point.SubNumber, point.Lat, point.Lng}, nil
		})
		_, err := tx.CopyFrom(ctx, pgx.Identifier{"address_points"},
			[]string{"city_code", "district_code", "road", "lane", "alley", "number", "sub_number", "lat", "lng"}, rows)
		if err != nil {
			return fmt.Errorf("copy address points: %w", err)
		}
		return nil
	})
}

func (store *ImportStore) ReplaceDistrictBoundaries(ctx context.Context, cityCode string, boundaries []domain.DistrictBoundary) error {
	return pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM district_boundaries WHERE city_code = $1`, cityCode); err != nil {
			return fmt.Errorf("delete district boundaries: %w", err)
		}
		for _, boundary := range boundaries {
			coordinates, err := json.Marshal(boundary.Coordinates)
			if err != nil {
				return fmt.Errorf("encode boundary of %s: %w", boundary.District, err)
			}
			if _, err = tx.Exec(ctx, `INSERT INTO district_boundaries (city_code, district, cells, coordinates)
                VALUES ($1, $2, $3, $4)`, cityCode, boundary.District, boundary.Cells, coordinates); err != nil {
				return fmt.Errorf("insert boundary of %s: %w", boundary.District, err)
			}
		}
		return nil
	})
}

func (store *ImportStore) HasDistrictBoundaries(ctx context.Context, cityCode string) (bool, error) {
	var exists bool
	if err := store.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM district_boundaries WHERE city_code = $1)`, cityCode).Scan(&exists); err != nil {
		return false, fmt.Errorf("query district boundaries: %w", err)
	}
	return exists, nil
}

func (store *ImportStore) LoadAddressPoints(ctx context.Context, cityCode string) ([]addresspoint.Point, error) {
	rows, err := store.pool.Query(ctx, `SELECT district_code, road, lane, alley, number, sub_number, lat, lng
        FROM address_points WHERE city_code = $1`, cityCode)
	if err != nil {
		return nil, fmt.Errorf("query address points: %w", err)
	}
	defer rows.Close()
	var points []addresspoint.Point
	for rows.Next() {
		var point addresspoint.Point
		if err = rows.Scan(&point.DistrictCode, &point.Road, &point.Lane, &point.Alley, &point.Number,
			&point.SubNumber, &point.Lat, &point.Lng); err != nil {
			return nil, fmt.Errorf("scan address point: %w", err)
		}
		points = append(points, point)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate address points: %w", err)
	}
	return points, nil
}

var transactionColumns = []string{
	"id", "city_code", "district", "address", "transaction_type", "building_type", "main_use", "total_price",
	"parking_price", "building_price", "building_area_ping", "price_per_ping", "rooms", "halls", "baths",
	"floor_label", "total_floors", "completed_on", "transacted_on", "remarks", "special_reasons",
	"geocode_precision", "season",
}

// UpsertTransactions copies rows into a staging table, then inserts or overwrites them by
// 編號 so that official corrections replace earlier versions.
func (store *ImportStore) UpsertTransactions(ctx context.Context, runID int64, transactions []domain.Transaction) error {
	return pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `CREATE TEMP TABLE transactions_staging
            (LIKE transactions INCLUDING DEFAULTS, lat DOUBLE PRECISION, lng DOUBLE PRECISION) ON COMMIT DROP`); err != nil {
			return fmt.Errorf("create staging table: %w", err)
		}
		columns := append(append([]string{}, transactionColumns...), "lat", "lng")
		rows := pgx.CopyFromSlice(len(transactions), func(index int) ([]any, error) {
			return transactionRow(transactions[index]), nil
		})
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"transactions_staging"}, columns, rows); err != nil {
			return fmt.Errorf("copy transactions: %w", err)
		}

		updates := make([]string, 0, len(transactionColumns))
		for _, column := range transactionColumns[1:] {
			updates = append(updates, column+" = EXCLUDED."+column)
		}
		columnList := strings.Join(transactionColumns, ", ")
		query := fmt.Sprintf(`INSERT INTO transactions (%s, location, import_run_id)
            SELECT DISTINCT ON (id) %s,
                CASE WHEN geocode_precision IS NULL THEN NULL
                     ELSE ST_SetSRID(ST_MakePoint(lng, lat), 4326)::GEOGRAPHY END,
                $1
            FROM transactions_staging ORDER BY id
            ON CONFLICT (id) DO UPDATE SET %s, location = EXCLUDED.location,
                import_run_id = EXCLUDED.import_run_id, updated_at = NOW()`, columnList, columnList, strings.Join(updates, ", "))
		if _, err := tx.Exec(ctx, query, runID); err != nil {
			return fmt.Errorf("upsert transactions: %w", err)
		}
		// Re-measure these sales to stations, since corrections may move them.
		if _, err := tx.Exec(ctx, `DELETE FROM transaction_station_distances
            WHERE transaction_id IN (SELECT id FROM transactions_staging)`); err != nil {
			return fmt.Errorf("delete station distances: %w", err)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(insertStationDistances, "t.id IN (SELECT id FROM transactions_staging)")); err != nil {
			return fmt.Errorf("measure station distances: %w", err)
		}
		return nil
	})
}

func transactionRow(transaction domain.Transaction) []any {
	var totalFloors, precision, lat, lng any
	if transaction.TotalFloors > 0 {
		totalFloors = transaction.TotalFloors
	}
	if transaction.GeocodePrecision != "" {
		precision, lat, lng = transaction.GeocodePrecision, transaction.Lat, transaction.Lng
	}
	specialReasons := transaction.SpecialReasons
	if specialReasons == nil {
		specialReasons = []string{}
	}
	return []any{
		transaction.ID, transaction.CityCode, transaction.District, transaction.Address, transaction.TransactionType,
		transaction.BuildingType, transaction.MainUse, transaction.TotalPrice, transaction.ParkingPrice,
		transaction.BuildingPrice, transaction.BuildingAreaPing, transaction.PricePerPing, transaction.Rooms,
		transaction.Halls, transaction.Baths, transaction.FloorLabel, totalFloors, transaction.CompletedAt,
		transaction.TransactedAt, transaction.Remarks, specialReasons, precision, transaction.Season, lat, lng,
	}
}

var rentalColumns = []string{
	"id", "city_code", "district", "address", "transaction_type", "rental_type", "whole_unit", "whole_unit_inferred",
	"social_housing", "rental_service", "building_type", "main_use", "monthly_rent", "parking_rent", "building_rent",
	"building_area_ping", "rent_per_ping", "rooms", "halls", "baths", "floor_label", "total_floors", "completed_on",
	"leased_on", "lease_start", "lease_end", "furnished", "has_manager", "has_elevator", "equipment", "renewal",
	"remarks", "special_reasons", "geocode_precision", "season",
}

// UpsertRentals stores leases like UpsertTransactions stores sales.
func (store *ImportStore) UpsertRentals(ctx context.Context, runID int64, rentals []domain.Rental) error {
	return pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `CREATE TEMP TABLE rentals_staging
            (LIKE rentals INCLUDING DEFAULTS, lat DOUBLE PRECISION, lng DOUBLE PRECISION) ON COMMIT DROP`); err != nil {
			return fmt.Errorf("create staging table: %w", err)
		}
		columns := append(append([]string{}, rentalColumns...), "lat", "lng")
		rows := pgx.CopyFromSlice(len(rentals), func(index int) ([]any, error) {
			return rentalRow(rentals[index]), nil
		})
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"rentals_staging"}, columns, rows); err != nil {
			return fmt.Errorf("copy rentals: %w", err)
		}

		updates := make([]string, 0, len(rentalColumns))
		for _, column := range rentalColumns[1:] {
			updates = append(updates, column+" = EXCLUDED."+column)
		}
		columnList := strings.Join(rentalColumns, ", ")
		query := fmt.Sprintf(`INSERT INTO rentals (%s, location, import_run_id)
            SELECT DISTINCT ON (id) %s,
                CASE WHEN geocode_precision IS NULL THEN NULL
                     ELSE ST_SetSRID(ST_MakePoint(lng, lat), 4326)::GEOGRAPHY END,
                $1
            FROM rentals_staging ORDER BY id
            ON CONFLICT (id) DO UPDATE SET %s, location = EXCLUDED.location,
                import_run_id = EXCLUDED.import_run_id, updated_at = NOW()`, columnList, columnList, strings.Join(updates, ", "))
		if _, err := tx.Exec(ctx, query, runID); err != nil {
			return fmt.Errorf("upsert rentals: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM rental_station_distances
            WHERE rental_id IN (SELECT id FROM rentals_staging)`); err != nil {
			return fmt.Errorf("delete rental station distances: %w", err)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(insertRentalStationDistances, "r.id IN (SELECT id FROM rentals_staging)")); err != nil {
			return fmt.Errorf("measure rental station distances: %w", err)
		}
		return nil
	})
}

func rentalRow(rental domain.Rental) []any {
	var rentalType, totalFloors, precision, lat, lng any
	if rental.RentalType != "" {
		rentalType = rental.RentalType
	}
	if rental.TotalFloors > 0 {
		totalFloors = rental.TotalFloors
	}
	if rental.GeocodePrecision != "" {
		precision, lat, lng = rental.GeocodePrecision, rental.Lat, rental.Lng
	}
	specialReasons := rental.SpecialReasons
	if specialReasons == nil {
		specialReasons = []string{}
	}
	return []any{
		rental.ID, rental.CityCode, rental.District, rental.Address, rental.TransactionType, rentalType, rental.WholeUnit,
		rental.WholeUnitInferred, rental.SocialHousing, rental.RentalService, rental.BuildingType, rental.MainUse,
		rental.MonthlyRent, rental.ParkingRent, rental.BuildingRent, rental.BuildingAreaPing, rental.RentPerPing,
		rental.Rooms, rental.Halls, rental.Baths, rental.FloorLabel, totalFloors, rental.CompletedAt, rental.LeasedAt,
		rental.LeaseStart, rental.LeaseEnd, rental.Furnished, rental.HasManager, rental.HasElevator, rental.Equipment,
		rental.Renewal, rental.Remarks, specialReasons, precision, rental.Season, lat, lng,
	}
}

var presaleColumns = append(append([]string{}, transactionColumns[:len(transactionColumns)-2]...),
	"project_name", "project_key", "unit_label", "cancellation", "cancelled_on", "geocode_precision", "season")

// UpsertPresales stores presales like UpsertTransactions stores sales, then rebuilds the
// projects they belong to, including the old project of a renamed presale.
func (store *ImportStore) UpsertPresales(ctx context.Context, runID int64, presales []domain.Presale) error {
	return pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `CREATE TEMP TABLE presales_staging
            (LIKE presales INCLUDING DEFAULTS, lat DOUBLE PRECISION, lng DOUBLE PRECISION) ON COMMIT DROP`); err != nil {
			return fmt.Errorf("create staging table: %w", err)
		}
		columns := append(append([]string{}, presaleColumns...), "lat", "lng")
		rows := pgx.CopyFromSlice(len(presales), func(index int) ([]any, error) {
			return presaleRow(presales[index]), nil
		})
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"presales_staging"}, columns, rows); err != nil {
			return fmt.Errorf("copy presales: %w", err)
		}
		if _, err := tx.Exec(ctx, `CREATE TEMP TABLE presales_touched ON COMMIT DROP AS
            SELECT project_key FROM presales_staging WHERE project_key IS NOT NULL
            UNION SELECT p.project_key FROM presales p JOIN presales_staging s USING (id) WHERE p.project_key IS NOT NULL`); err != nil {
			return fmt.Errorf("collect projects: %w", err)
		}

		updates := make([]string, 0, len(presaleColumns))
		for _, column := range presaleColumns[1:] {
			updates = append(updates, column+" = EXCLUDED."+column)
		}
		columnList := strings.Join(presaleColumns, ", ")
		query := fmt.Sprintf(`INSERT INTO presales (%s, location, import_run_id)
            SELECT DISTINCT ON (id) %s,
                CASE WHEN geocode_precision IS NULL THEN NULL
                     ELSE ST_SetSRID(ST_MakePoint(lng, lat), 4326)::GEOGRAPHY END,
                $1
            FROM presales_staging ORDER BY id
            ON CONFLICT (id) DO UPDATE SET %s, location = EXCLUDED.location,
                import_run_id = EXCLUDED.import_run_id, updated_at = NOW()`, columnList, columnList, strings.Join(updates, ", "))
		if _, err := tx.Exec(ctx, query, runID); err != nil {
			return fmt.Errorf("upsert presales: %w", err)
		}
		if _, err := tx.Exec(ctx, `SELECT refresh_presale_projects(ARRAY(SELECT project_key FROM presales_touched))`); err != nil {
			return fmt.Errorf("refresh presale projects: %w", err)
		}
		return nil
	})
}

func presaleRow(presale domain.Presale) []any {
	row := transactionRow(presale.Transaction)
	// transactionRow ends with geocode_precision, season, lat and lng.
	base := row[:len(row)-4]
	var projectKey any
	if presale.ProjectKey != "" {
		projectKey = presale.ProjectKey
	}
	return append(append([]any{}, base...), presale.ProjectName, projectKey, presale.UnitLabel, presale.Cancellation,
		presale.CancelledOn, row[len(row)-4], row[len(row)-3], row[len(row)-2], row[len(row)-1])
}
