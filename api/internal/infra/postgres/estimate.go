package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// marketSale restricts transactions to house-and-land sales that are not special.
const marketSale = `transaction_type LIKE '房地%' AND NOT is_special`

// EstimateCandidates returns sales of the category within radius metres of the point,
// located at a door number, dated from from (inclusive) to to (exclusive), with an area
// between minArea and maxArea ping.
func (repository *TransactionRepository) EstimateCandidates(ctx context.Context, query domain.EstimateQuery, radius, minArea, maxArea float64, from, to time.Time) ([]domain.ComparableSale, error) {
	sql := `WITH point AS (SELECT ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography AS location)
        SELECT t.id, t.address, ST_Y(t.location::geometry), ST_X(t.location::geometry), t.building_area_ping,
            t.building_age, t.floor_label, t.price_per_ping, t.transacted_on, ST_Distance(t.location, point.location, FALSE)
        FROM transactions t, point
        WHERE ST_DWithin(t.location, point.location, $3, FALSE)
          AND t.geocode_precision IN ('address', 'nearby') AND t.building_category = $4 AND ` + marketSale + `
          AND t.building_area_ping BETWEEN $5 AND $6 AND t.transacted_on >= $7 AND t.transacted_on < $8`
	args := []any{query.Lng, query.Lat, radius, query.Category, minArea, maxArea, from, to}
	sales := []domain.ComparableSale{}
	err := scanRows(ctx, repository, sql, args, func(rows pgx.Rows) error {
		var sale domain.ComparableSale
		if err := rows.Scan(&sale.ID, &sale.Address, &sale.Lat, &sale.Lng, &sale.AreaPing, &sale.Age, &sale.FloorLabel,
			&sale.PricePerPing, &sale.TransactedAt, &sale.DistanceM); err != nil {
			return err
		}
		sales = append(sales, sale)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("estimate candidates: %w", err)
	}
	return sales, nil
}

// PriceIndex returns a city's monthly index from first to last: the median price per ping
// of residential sales in each month and the two before it.
func (repository *TransactionRepository) PriceIndex(ctx context.Context, city string, first, last time.Time) ([]domain.PriceIndexPoint, error) {
	sql := `WITH months AS (SELECT generate_series($2::date, $3::date, INTERVAL '1 month')::date AS month),
        sales AS (
            SELECT shifted.month, t.price_per_ping FROM transactions t
            CROSS JOIN LATERAL (VALUES (date_trunc('month', t.transacted_on)::date),
                ((date_trunc('month', t.transacted_on) + INTERVAL '1 month')::date),
                ((date_trunc('month', t.transacted_on) + INTERVAL '2 months')::date)) AS shifted(month)
            WHERE t.city_code = $1 AND t.transacted_on >= $2::date - INTERVAL '2 months'
              AND t.transacted_on < $3::date + INTERVAL '1 month'
              AND t.building_category = ANY (` + residentialCategories + `) AND ` + marketSale + `)
        SELECT m.month, COUNT(s.month), percentile_cont(0.5) WITHIN GROUP (ORDER BY s.price_per_ping)
        FROM months m LEFT JOIN sales s ON s.month = m.month
        GROUP BY m.month ORDER BY m.month`
	points := []domain.PriceIndexPoint{}
	err := scanRows(ctx, repository, sql, []any{city, first, last}, func(rows pgx.Rows) error {
		var point domain.PriceIndexPoint
		if err := rows.Scan(&point.Month, &point.Sales, &point.Median); err != nil {
			return err
		}
		points = append(points, point)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("price index: %w", err)
	}
	return points, nil
}

// ParkingPrice returns the median price of parking spaces sold separately priced with
// homes of the category from from to to, in a district or, when district is empty, the
// whole city, and how many sales it comes from.
func (repository *TransactionRepository) ParkingPrice(ctx context.Context, city, district, category string, from, to time.Time) (*float64, int, error) {
	var median *float64
	var count int
	err := repository.pool.QueryRow(ctx, `SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY parking_price), COUNT(*)
        FROM transactions
        WHERE city_code = $1 AND ($2 = '' OR district = $2) AND building_category = $3
          AND transaction_type LIKE '%車位%' AND parking_price > 0 AND `+marketSale+`
          AND transacted_on >= $4 AND transacted_on < $5`, city, district, category, from, to).Scan(&median, &count)
	if err != nil {
		return nil, 0, fmt.Errorf("parking price: %w", err)
	}
	return median, count, nil
}

// BuildingProfile summarises the sales within 150 m of the point whose address starts
// with the building's door number, such as 台北市大安區仁愛路四段27巷3號.
func (repository *TransactionRepository) BuildingProfile(ctx context.Context, building string, lat, lng float64) (domain.BuildingProfile, error) {
	profile := domain.BuildingProfile{Address: building}
	pattern := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(building) + "%"
	var category *string
	var lastSale *time.Time
	err := repository.pool.QueryRow(ctx, `SELECT COUNT(*), mode() WITHIN GROUP (ORDER BY building_category),
            mode() WITHIN GROUP (ORDER BY EXTRACT(YEAR FROM completed_on)::INTEGER), MAX(total_floors), MAX(transacted_on)
        FROM transactions
        WHERE ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 150, FALSE)
          AND address LIKE $3 AND transacted_on <= CURRENT_DATE`, lng, lat, pattern).
		Scan(&profile.Sales, &category, &profile.CompletedYear, &profile.TotalFloors, &lastSale)
	if err != nil {
		return profile, fmt.Errorf("building profile: %w", err)
	}
	if category != nil {
		profile.Category = *category
	}
	if lastSale != nil {
		profile.LastSale = lastSale.Format("2006-01-02")
	}
	return profile, nil
}

// RentCandidates returns whole-home market leases (new, not special, not social housing)
// of the category within radius metres of the point, located at a door number, signed
// from from to to (exclusive), with an area between minArea and maxArea ping.
func (repository *TransactionRepository) RentCandidates(ctx context.Context, query domain.EstimateQuery, radius, minArea, maxArea float64, from, to time.Time) ([]domain.LeaseCandidate, error) {
	sql := `WITH point AS (SELECT ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography AS location)
        SELECT r.id, r.address, ST_Y(r.location::geometry), ST_X(r.location::geometry), r.building_area_ping,
            r.building_age, r.monthly_rent, r.rent_per_ping, r.leased_on, ST_Distance(r.location, point.location, FALSE)
        FROM rentals r, point
        WHERE ST_DWithin(r.location, point.location, $3, FALSE)
          AND r.geocode_precision IN ('address', 'nearby') AND r.building_category = $4
          AND r.whole_unit AND NOT r.renewal AND NOT r.is_special AND NOT r.social_housing
          AND r.building_area_ping BETWEEN $5 AND $6 AND r.leased_on >= $7 AND r.leased_on < $8`
	args := []any{query.Lng, query.Lat, radius, query.Category, minArea, maxArea, from, to}
	leases := []domain.LeaseCandidate{}
	err := scanRows(ctx, repository, sql, args, func(rows pgx.Rows) error {
		var lease domain.LeaseCandidate
		if err := rows.Scan(&lease.ID, &lease.Address, &lease.Lat, &lease.Lng, &lease.AreaPing, &lease.Age,
			&lease.MonthlyRent, &lease.RentPerPing, &lease.LeasedAt, &lease.DistanceM); err != nil {
			return err
		}
		leases = append(leases, lease)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("rent candidates: %w", err)
	}
	return leases, nil
}
