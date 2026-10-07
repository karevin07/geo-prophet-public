package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ReplaceFacilities replaces every facility of a source in one transaction.
func (store *ImportStore) ReplaceFacilities(ctx context.Context, runID int64, source string, facilities []domain.Facility) error {
	return pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM facilities WHERE source = $1`, source); err != nil {
			return fmt.Errorf("delete facilities: %w", err)
		}
		if _, err := tx.Exec(ctx, `CREATE TEMP TABLE facilities_staging (
                id TEXT, source TEXT, category TEXT, name TEXT, brand TEXT, city_code TEXT, district TEXT,
                address TEXT, lat DOUBLE PRECISION, lng DOUBLE PRECISION, area TEXT, area_m2 DOUBLE PRECISION,
                geocode_precision TEXT) ON COMMIT DROP`); err != nil {
			return fmt.Errorf("create staging table: %w", err)
		}
		rows := pgx.CopyFromSlice(len(facilities), func(index int) ([]any, error) {
			facility := facilities[index]
			var area, areaM2, precision any
			if len(facility.Area) >= 4 {
				area, areaM2 = polygonWKT(facility.Area), facility.AreaM2
			}
			if facility.GeocodePrecision != "" {
				precision = facility.GeocodePrecision
			}
			return []any{facility.ID, facility.Source, facility.Category, facility.Name, facility.Brand, facility.CityCode,
				facility.District, facility.Address, facility.Lat, facility.Lng, area, areaM2, precision}, nil
		})
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"facilities_staging"}, []string{"id", "source", "category", "name", "brand",
			"city_code", "district", "address", "lat", "lng", "area", "area_m2", "geocode_precision"}, rows); err != nil {
			return fmt.Errorf("copy facilities: %w", err)
		}
		// ST_MakeValid repairs self-intersecting outlines drawn by mappers; anything that
		// does not stay a polygon keeps only its point.
		if _, err := tx.Exec(ctx, `INSERT INTO facilities (id, source, category, name, brand, city_code, district, address,
                location, area, area_m2, geocode_precision, import_run_id)
            SELECT DISTINCT ON (id) id, source, category, name, brand, city_code, district, address,
                ST_SetSRID(ST_MakePoint(lng, lat), 4326)::GEOGRAPHY,
                CASE WHEN area IS NULL THEN NULL ELSE (
                    SELECT CASE WHEN GeometryType(valid) = 'POLYGON' THEN valid::GEOGRAPHY END
                    FROM (SELECT ST_MakeValid(ST_GeomFromText(area, 4326)) AS valid) repaired) END,
                area_m2, geocode_precision, $1
            FROM facilities_staging ORDER BY id`, runID); err != nil {
			return fmt.Errorf("insert facilities: %w", err)
		}
		return nil
	})
}

// polygonWKT writes a closed [lng, lat] ring as a WKT polygon.
func polygonWKT(ring [][2]float64) string {
	var builder strings.Builder
	builder.WriteString("POLYGON((")
	for index, point := range ring {
		if index > 0 {
			builder.WriteString(",")
		}
		builder.WriteString(strconv.FormatFloat(point[0], 'f', 7, 64))
		builder.WriteString(" ")
		builder.WriteString(strconv.FormatFloat(point[1], 'f', 7, 64))
	}
	builder.WriteString("))")
	return builder.String()
}
