package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ImportedRentalsByCity returns the lease seasons and 10-day releases imported for a city.
func (repository *TransactionRepository) ImportedRentalsByCity(ctx context.Context, city string) ([]string, []string, error) {
	return repository.importedPeriods(ctx, city, "rentals")
}

// residentialCategories mirrors domain.BuildingCategories without 其他, which holds shops
// and offices.
const residentialCategories = `ARRAY['住宅大樓', '華廈', '公寓', '透天厝']`

// comparableHomes restricts a table aliased %[1]s to a domain.RentalQuery's homes: $1 is
// the city, $4 the building category (” for all residential ones), $5 and $6 the area
// band (0 for no upper bound).
const comparableHomes = `%[1]s.city_code = $1
    AND ($4 = '' AND %[1]s.building_category = ANY (` + residentialCategories + `) OR %[1]s.building_category = $4)
    AND %[1]s.building_area_ping >= $5 AND ($6::float8 = 0 OR %[1]s.building_area_ping < $6::float8)`

// marketLeases restricts rentals r to a domain.RentalQuery's leases: new whole-home leases
// that are not special and, unless $7 is true, not social housing, whose rents are capped.
// Renewals lag the market.
var marketLeases = fmt.Sprintf(comparableHomes, "r") + `
    AND r.whole_unit AND NOT r.renewal AND NOT r.is_special AND ($7 OR NOT r.social_housing)`

// marketSales restricts transactions t to sales of a domain.RentalQuery's homes.
var marketSales = fmt.Sprintf(comparableHomes, "t") + `
    AND t.transaction_type LIKE '房地%' AND NOT t.is_special`

// ringValues lists domain.StationRings as a VALUES list of radii.
func ringValues() string {
	rows := make([]string, len(domain.StationRings))
	for index, radius := range domain.StationRings {
		rows[index] = "(" + strconv.Itoa(radius) + ")"
	}
	return "(VALUES " + strings.Join(rows, ", ") + ") AS ring(radius)"
}

// RentalAggregates computes, for each district and the whole city (ROLLUP), the medians
// of market leases and of sales of the same kind of home between from and to (exclusive).
func (repository *TransactionRepository) RentalAggregates(ctx context.Context, query domain.RentalQuery, from, to time.Time) ([]domain.RentalAggregate, error) {
	rentSQL := `SELECT GROUPING(r.district), COALESCE(r.district, ''), COUNT(*),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY r.rent_per_ping),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY r.monthly_rent)
        FROM rentals r
        WHERE r.leased_on >= $2 AND r.leased_on < $3 AND ` + marketLeases + `
        GROUP BY ROLLUP(r.district)`
	saleSQL := `SELECT GROUPING(t.district), COALESCE(t.district, ''), COUNT(*),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY t.price_per_ping)
        FROM transactions t
        WHERE t.transacted_on >= $2 AND t.transacted_on < $3 AND ` + marketSales + `
        GROUP BY ROLLUP(t.district)`
	args := []any{query.CityCode, from, to, query.Category, query.MinArea, query.MaxArea}

	byDistrict := map[string]*domain.RentalAggregate{}
	row := func(district string) *domain.RentalAggregate {
		if byDistrict[district] == nil {
			byDistrict[district] = &domain.RentalAggregate{District: district}
		}
		return byDistrict[district]
	}
	err := scanRows(ctx, repository, rentSQL, append(args, query.IncludeSocialHousing), func(rows pgx.Rows) error {
		var rollup int
		var district string
		var aggregate domain.RentalAggregate
		if err := rows.Scan(&rollup, &district, &aggregate.Rentals, &aggregate.MedianRentPerPing, &aggregate.MedianMonthlyRent); err != nil {
			return err
		}
		if rollup == 1 {
			district = ""
		}
		target := row(district)
		target.Rentals, target.MedianRentPerPing, target.MedianMonthlyRent = aggregate.Rentals, aggregate.MedianRentPerPing, aggregate.MedianMonthlyRent
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("rental aggregates: %w", err)
	}
	err = scanRows(ctx, repository, saleSQL, args, func(rows pgx.Rows) error {
		var rollup, count int
		var district string
		var median *float64
		if err := rows.Scan(&rollup, &district, &count, &median); err != nil {
			return err
		}
		if rollup == 1 {
			district = ""
		}
		target := row(district)
		target.Sales, target.MedianPricePerPing = count, median
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("sale aggregates: %w", err)
	}

	result := make([]domain.RentalAggregate, 0, len(byDistrict))
	for _, aggregate := range byDistrict {
		result = append(result, *aggregate)
	}
	return result, nil
}

// pooledMonths spreads each lease of rentals r over the month it was signed in and the two
// after, so grouping by shifted.month pools three months of leases. Joining every month
// to a date range instead is several times slower.
const pooledMonths = `CROSS JOIN LATERAL (VALUES (date_trunc('month', r.leased_on)::date),
    ((date_trunc('month', r.leased_on) + INTERVAL '1 month')::date),
    ((date_trunc('month', r.leased_on) + INTERVAL '2 months')::date)) AS shifted(month)`

// RentTrend returns, for each month from first to last (both the first of a month), the
// median rent per ping of market leases signed in that month and the two before, in the
// city or, when district is not empty, in one district.
func (repository *TransactionRepository) RentTrend(ctx context.Context, query domain.RentalQuery, district string, first, last time.Time) ([]domain.RentPoint, error) {
	sql := `WITH months AS (SELECT generate_series($2::date, $3::date, INTERVAL '1 month')::date AS month),
        leases AS (
            SELECT shifted.month, r.rent_per_ping FROM rentals r ` + pooledMonths + `
            WHERE r.leased_on >= $2::date - INTERVAL '2 months' AND r.leased_on < $3::date + INTERVAL '1 month'
              AND ($8 = '' OR r.district = $8) AND ` + marketLeases + `)
        SELECT to_char(m.month, 'YYYY-MM'), COUNT(l.month), percentile_cont(0.5) WITHIN GROUP (ORDER BY l.rent_per_ping)
        FROM months m LEFT JOIN leases l ON l.month = m.month
        GROUP BY m.month ORDER BY m.month`
	args := []any{query.CityCode, first, last, query.Category, query.MinArea, query.MaxArea, query.IncludeSocialHousing, district}
	points := []domain.RentPoint{}
	err := scanRows(ctx, repository, sql, args, func(rows pgx.Rows) error {
		var point domain.RentPoint
		if err := rows.Scan(&point.Month, &point.Rentals, &point.MedianRentPerPing); err != nil {
			return err
		}
		points = append(points, point)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("rent trend: %w", err)
	}
	return points, nil
}

// StationDistrict returns the city and district of a station and whether it exists.
func (repository *TransactionRepository) StationDistrict(ctx context.Context, stationID string) (string, string, bool, error) {
	var city, district string
	err := repository.pool.QueryRow(ctx, `SELECT city_code, district FROM transit_stations WHERE id = $1`, stationID).Scan(&city, &district)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("query station district: %w", err)
	}
	return city, district, true, nil
}

// StationRentRings computes, within each of domain.StationRings of a station, the medians
// of market leases and of sales of the same kind of home between from and to
// (exclusive), and the median rent per ping of the station's whole district.
func (repository *TransactionRepository) StationRentRings(ctx context.Context, stationID, district string, query domain.RentalQuery, from, to time.Time) ([]domain.RentalAggregate, *float64, error) {
	rentSQL := `SELECT ring.radius, COUNT(r.id),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY r.rent_per_ping),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY r.monthly_rent)
        FROM ` + ringValues() + `
        LEFT JOIN (rental_station_distances d
            JOIN rentals r ON r.id = d.rental_id AND r.leased_on >= $2 AND r.leased_on < $3 AND ` + marketLeases + `)
            ON d.station_id = $8 AND d.distance_m <= ring.radius
        GROUP BY ring.radius ORDER BY ring.radius`
	saleSQL := `SELECT ring.radius, COUNT(t.id), percentile_cont(0.5) WITHIN GROUP (ORDER BY t.price_per_ping)
        FROM ` + ringValues() + `
        LEFT JOIN (transaction_station_distances d
            JOIN transactions t ON t.id = d.transaction_id AND t.transacted_on >= $2 AND t.transacted_on < $3 AND ` + marketSales + `)
            ON d.station_id = $7 AND d.distance_m <= ring.radius
        GROUP BY ring.radius ORDER BY ring.radius`
	districtSQL := `SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY r.rent_per_ping) FROM rentals r
        WHERE r.district = $8 AND r.leased_on >= $2 AND r.leased_on < $3 AND ` + marketLeases
	args := []any{query.CityCode, from, to, query.Category, query.MinArea, query.MaxArea}
	rentArgs := append(append([]any{}, args...), query.IncludeSocialHousing, stationID)

	rings := make([]domain.RentalAggregate, len(domain.StationRings))
	err := scanRows(ctx, repository, rentSQL, rentArgs, func(rows pgx.Rows) error {
		var radius int
		var aggregate domain.RentalAggregate
		if err := rows.Scan(&radius, &aggregate.Rentals, &aggregate.MedianRentPerPing, &aggregate.MedianMonthlyRent); err != nil {
			return err
		}
		for index, ring := range domain.StationRings {
			if ring == radius {
				rings[index] = aggregate
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("station rents: %w", err)
	}
	err = scanRows(ctx, repository, saleSQL, append(append([]any{}, args...), stationID), func(rows pgx.Rows) error {
		var radius, count int
		var median *float64
		if err := rows.Scan(&radius, &count, &median); err != nil {
			return err
		}
		for index, ring := range domain.StationRings {
			if ring == radius {
				rings[index].Sales, rings[index].MedianPricePerPing = count, median
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("station sales: %w", err)
	}
	var districtMedian *float64
	districtArgs := append(append([]any{}, args...), query.IncludeSocialHousing, district)
	if err = repository.pool.QueryRow(ctx, districtSQL, districtArgs...).Scan(&districtMedian); err != nil {
		return nil, nil, fmt.Errorf("district rent: %w", err)
	}
	return rings, districtMedian, nil
}

// StationRentTrends returns each ring's rent trend like RentTrend, in the order of
// domain.StationRings.
func (repository *TransactionRepository) StationRentTrends(ctx context.Context, stationID string, query domain.RentalQuery, first, last time.Time) ([][]domain.RentPoint, error) {
	sql := `WITH months AS (SELECT generate_series($2::date, $3::date, INTERVAL '1 month')::date AS month),
        near AS (
            SELECT d.distance_m, shifted.month, r.rent_per_ping
            FROM rental_station_distances d JOIN rentals r ON r.id = d.rental_id ` + pooledMonths + `
            WHERE d.station_id = $8 AND r.leased_on >= $2::date - INTERVAL '2 months'
              AND r.leased_on < $3::date + INTERVAL '1 month' AND ` + marketLeases + `)
        SELECT ring.radius, to_char(m.month, 'YYYY-MM'), COUNT(n.month),
            percentile_cont(0.5) WITHIN GROUP (ORDER BY n.rent_per_ping)
        FROM ` + ringValues() + ` CROSS JOIN months m
        LEFT JOIN near n ON n.distance_m <= ring.radius AND n.month = m.month
        GROUP BY ring.radius, m.month ORDER BY ring.radius, m.month`
	args := []any{query.CityCode, first, last, query.Category, query.MinArea, query.MaxArea, query.IncludeSocialHousing, stationID}
	trends := make([][]domain.RentPoint, len(domain.StationRings))
	err := scanRows(ctx, repository, sql, args, func(rows pgx.Rows) error {
		var radius int
		var point domain.RentPoint
		if err := rows.Scan(&radius, &point.Month, &point.Rentals, &point.MedianRentPerPing); err != nil {
			return err
		}
		for index, ring := range domain.StationRings {
			if ring == radius {
				trends[index] = append(trends[index], point)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("station rent trends: %w", err)
	}
	return trends, nil
}

// scanRows runs a query and calls scan for each row.
func scanRows(ctx context.Context, repository *TransactionRepository, sql string, args []any, scan func(pgx.Rows) error) error {
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err = scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// MappedLeases streams the market leases located to an address inside the bounds, signed
// between from and to (exclusive), of the filter's categories (residential ones when
// empty), area and age. Social housing is left out.
func (repository *TransactionRepository) MappedLeases(ctx context.Context, filter domain.TransactionFilter, from, to time.Time, emit func(domain.MappedLease) error) error {
	where, args := mappedWhere("r", "leased_on", filter, from, to)
	sql := `SELECT r.city_code, ST_Y(r.location::geometry), ST_X(r.location::geometry), r.leased_on, r.rent_per_ping,
            r.monthly_rent, r.building_area_ping, r.rooms, r.building_category
        FROM rentals r
        WHERE ` + where + ` AND r.whole_unit AND NOT r.renewal AND NOT r.is_special AND NOT r.social_housing`
	return scanRows(ctx, repository, sql, args, func(rows pgx.Rows) error {
		var lease domain.MappedLease
		if err := rows.Scan(&lease.CityCode, &lease.Lat, &lease.Lng, &lease.LeasedAt, &lease.RentPerPing,
			&lease.MonthlyRent, &lease.AreaPing, &lease.Rooms, &lease.Category); err != nil {
			return err
		}
		return emit(lease)
	})
}

// MappedSales streams the market sales of the same homes as MappedLeases.
func (repository *TransactionRepository) MappedSales(ctx context.Context, filter domain.TransactionFilter, from, to time.Time, emit func(domain.MappedSale) error) error {
	where, args := mappedWhere("t", "transacted_on", filter, from, to)
	sql := `SELECT ST_Y(t.location::geometry), ST_X(t.location::geometry), t.price_per_ping
        FROM transactions t
        WHERE ` + where + ` AND t.transaction_type LIKE '房地%' AND NOT t.is_special`
	return scanRows(ctx, repository, sql, args, func(rows pgx.Rows) error {
		var sale domain.MappedSale
		if err := rows.Scan(&sale.Lat, &sale.Lng, &sale.PricePerPing); err != nil {
			return err
		}
		return emit(sale)
	})
}

// mappedWhere restricts a table aliased alias to homes located to an address inside the
// filter's bounds and dated (by column) between from and to, exclusive.
func mappedWhere(alias, column string, filter domain.TransactionFilter, from, to time.Time) (string, []any) {
	categories := filter.Categories
	if len(categories) == 0 {
		categories = domain.BuildingCategories[:len(domain.BuildingCategories)-1]
	}
	bounds := filter.Bounds
	args := []any{bounds.West, bounds.South, bounds.East, bounds.North, from, to, categories, filter.MinArea, filter.MaxArea}
	where := withinBounds(alias+".location", "ST_MakeEnvelope($1, $2, $3, $4, 4326)") + fmt.Sprintf(`
        AND %[1]s.geocode_precision IN ('address', 'nearby')
        AND %[1]s.%[2]s >= $5 AND %[1]s.%[2]s < $6
        AND %[1]s.building_category = ANY ($7) AND %[1]s.building_area_ping BETWEEN $8 AND $9`, alias, column)
	if filter.ExcludeNewBuildings {
		args = append(args, domain.NewBuildingYears)
		where += fmt.Sprintf(" AND (%[1]s.building_age IS NULL OR %[1]s.building_age >= $10)", alias)
	}
	return where, args
}
