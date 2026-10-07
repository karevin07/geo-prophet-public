package postgres

import (
	"context"
	"fmt"
	"math"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Facilities returns facilities of the categories inside the bounds, at most limit.
func (repository *TransactionRepository) Facilities(ctx context.Context, query domain.FacilityQuery, limit int) ([]domain.Facility, error) {
	sql := `SELECT id, source, category, name, brand, city_code, district, address,
            ST_Y(location::geometry), ST_X(location::geometry), COALESCE(area_m2, 0)
        FROM facilities
        WHERE location && ST_MakeEnvelope($1, $2, $3, $4, 4326)::geography AND category = ANY ($5)
        ORDER BY id LIMIT $6`
	bounds := query.Bounds
	facilities := []domain.Facility{}
	err := scanRows(ctx, repository, sql, []any{bounds.West, bounds.South, bounds.East, bounds.North, query.Categories, limit}, func(rows pgx.Rows) error {
		var facility domain.Facility
		if err := rows.Scan(&facility.ID, &facility.Source, &facility.Category, &facility.Name, &facility.Brand, &facility.CityCode,
			&facility.District, &facility.Address, &facility.Lat, &facility.Lng, &facility.AreaM2); err != nil {
			return err
		}
		facilities = append(facilities, facility)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("facilities: %w", err)
	}
	return facilities, nil
}

// NearbyFacilities counts each category within radius metres of a point and finds its
// nearest facility within search metres. Distances to parks and other areas are measured
// to their outline, so a point inside one is 0 m away.
func (repository *TransactionRepository) NearbyFacilities(ctx context.Context, lat, lng, radius, search float64) ([]domain.NearbyCategory, error) {
	sql := `WITH point AS (SELECT ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography AS location),
        near AS (
            SELECT f.category, f.name, ST_Distance(COALESCE(f.area, f.location), point.location, FALSE) AS distance
            FROM facilities f, point
            WHERE ST_DWithin(f.location, point.location, $4, FALSE)
               OR (f.area IS NOT NULL AND ST_DWithin(f.area, point.location, $4, FALSE)))
        SELECT category, COUNT(*) FILTER (WHERE distance <= $3),
            (array_agg(name ORDER BY distance))[1], MIN(distance)
        FROM near WHERE distance <= $4 GROUP BY category`
	categories := []domain.NearbyCategory{}
	err := scanRows(ctx, repository, sql, []any{lng, lat, radius, search}, func(rows pgx.Rows) error {
		var category domain.NearbyCategory
		var nearest float64
		if err := rows.Scan(&category.Category, &category.Count, &category.NearestName, &nearest); err != nil {
			return err
		}
		category.NearestM = &nearest
		categories = append(categories, category)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("nearby facilities: %w", err)
	}
	return categories, nil
}

// DistrictFacilities counts each district's facilities by category and measures the
// district boundaries' area.
func (repository *TransactionRepository) DistrictFacilities(ctx context.Context, city string) ([]domain.DistrictFacilities, error) {
	byDistrict := map[string]*domain.DistrictFacilities{}
	row := func(district string) *domain.DistrictFacilities {
		if byDistrict[district] == nil {
			byDistrict[district] = &domain.DistrictFacilities{District: district, Counts: map[string]int{}}
		}
		return byDistrict[district]
	}
	err := scanRows(ctx, repository, `SELECT district,
            ST_Area(ST_GeomFromGeoJSON(json_build_object('type', 'MultiPolygon', 'coordinates', coordinates)::text)::geography) / 1e6
        FROM district_boundaries WHERE city_code = $1`, []any{city}, func(rows pgx.Rows) error {
		var district string
		var area float64
		if err := rows.Scan(&district, &area); err != nil {
			return err
		}
		row(district).AreaKm2 = math.Round(area*100) / 100
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("district areas: %w", err)
	}
	err = scanRows(ctx, repository, `SELECT district, category, COUNT(*) FROM facilities WHERE city_code = $1 GROUP BY 1, 2`,
		[]any{city}, func(rows pgx.Rows) error {
			var district, category string
			var count int
			if err := rows.Scan(&district, &category, &count); err != nil {
				return err
			}
			row(district).Counts[category] = count
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("district facilities: %w", err)
	}
	districts := make([]domain.DistrictFacilities, 0, len(byDistrict))
	for _, district := range byDistrict {
		districts = append(districts, *district)
	}
	return districts, nil
}
