package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// validPresale restricts presales s to house-and-land sales that are neither cancelled
// nor special.
const validPresale = `s.transaction_type LIKE '房地%' AND NOT s.is_cancelled AND NOT s.is_special`

// projectColumns summarise presale_projects p over its presales s; $2 starts the recent
// window.
const projectColumns = `p.project_key, p.name, p.city_code, p.district, p.address,
    ST_Y(p.location::geometry), ST_X(p.location::geometry), COALESCE(p.geocode_precision, ''),
    COALESCE(mode() WITHIN GROUP (ORDER BY s.building_category), ''), MAX(s.total_floors),
    COUNT(*) FILTER (WHERE ` + validPresale + `), COUNT(*) FILTER (WHERE s.is_cancelled),
    COUNT(*) FILTER (WHERE ` + validPresale + ` AND s.transacted_on >= $2),
    MIN(s.transacted_on), MAX(s.transacted_on),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY s.price_per_ping) FILTER (WHERE ` + validPresale + `),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY s.total_price) FILTER (WHERE ` + validPresale + `),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY s.building_area_ping) FILTER (WHERE ` + validPresale + `)`

func scanProject(row pgx.Row, extra ...any) (domain.PresaleProject, error) {
	var project domain.PresaleProject
	var first, last time.Time
	err := row.Scan(append([]any{&project.Key, &project.Name, &project.CityCode, &project.District, &project.Address,
		&project.Lat, &project.Lng, &project.GeocodePrecision, &project.Category, &project.TotalFloors,
		&project.Sales, &project.Cancelled, &project.RecentSales, &first, &last,
		&project.MedianPricePerPing, &project.MedianTotalPrice, &project.MedianAreaPing}, extra...)...)
	project.FirstContract, project.LastContract = first.Format("2006-01-02"), last.Format("2006-01-02")
	return project, err
}

// PresaleProjects lists the projects of a city, or one district, with a contract since
// query.From; RecentSales count valid sales since recentFrom.
func (repository *TransactionRepository) PresaleProjects(ctx context.Context, query domain.PresaleProjectQuery, recentFrom time.Time) ([]domain.PresaleProject, error) {
	sql := `SELECT ` + projectColumns + `
        FROM presale_projects p JOIN presales s ON s.project_key = p.project_key
        WHERE p.city_code = $1 AND ($3 = '' OR p.district = $3)
        GROUP BY p.project_key
        HAVING MAX(s.transacted_on) >= $4
        ORDER BY MAX(s.transacted_on) DESC, p.project_key
        LIMIT 1000`
	projects := []domain.PresaleProject{}
	err := scanRows(ctx, repository, sql, []any{query.CityCode, recentFrom, query.District, query.From}, func(rows pgx.Rows) error {
		project, err := scanProject(rows)
		projects = append(projects, project)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("presale projects: %w", err)
	}
	return projects, nil
}

// NearbyPresaleProjects lists projects located better than their district within radius
// metres of a point with a contract since from, nearest first.
func (repository *TransactionRepository) NearbyPresaleProjects(ctx context.Context, near domain.NearPoint, from, recentFrom time.Time) ([]domain.PresaleProject, error) {
	sql := `WITH point AS (SELECT ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography AS location)
        SELECT ` + projectColumns + `, MIN(ST_Distance(p.location, point.location, FALSE)) AS distance
        FROM presale_projects p JOIN presales s ON s.project_key = p.project_key, point
        WHERE ST_DWithin(p.location, point.location, $5, FALSE) AND p.geocode_precision IN ('address', 'nearby', 'road')
        GROUP BY p.project_key
        HAVING MAX(s.transacted_on) >= $1
        ORDER BY distance
        LIMIT 100`
	projects := []domain.PresaleProject{}
	err := scanRows(ctx, repository, sql, []any{from, recentFrom, near.Lng, near.Lat, near.RadiusM}, func(rows pgx.Rows) error {
		var distance float64
		project, err := scanProject(rows, &distance)
		project.DistanceM = new(math.Round(distance))
		projects = append(projects, project)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("nearby presale projects: %w", err)
	}
	return projects, nil
}

// BoundedPresaleProjects lists projects located better than their district inside the
// bounds with a contract since from, latest first.
func (repository *TransactionRepository) BoundedPresaleProjects(ctx context.Context, bounds domain.Bounds, from, recentFrom time.Time) ([]domain.PresaleProject, error) {
	sql := `SELECT ` + projectColumns + `
        FROM presale_projects p JOIN presales s ON s.project_key = p.project_key
        WHERE p.location && ST_MakeEnvelope($3, $4, $5, $6, 4326)::geography AND p.geocode_precision IN ('address', 'nearby', 'road')
        GROUP BY p.project_key
        HAVING MAX(s.transacted_on) >= $1
        ORDER BY MAX(s.transacted_on) DESC, p.project_key
        LIMIT 1000`
	projects := []domain.PresaleProject{}
	err := scanRows(ctx, repository, sql, []any{from, recentFrom, bounds.West, bounds.South, bounds.East, bounds.North}, func(rows pgx.Rows) error {
		project, err := scanProject(rows)
		projects = append(projects, project)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("bounded presale projects: %w", err)
	}
	return projects, nil
}

// PresaleProject returns one project and whether it exists.
func (repository *TransactionRepository) PresaleProject(ctx context.Context, key string, recentFrom time.Time) (domain.PresaleProject, bool, error) {
	project, err := scanProject(repository.pool.QueryRow(ctx, `SELECT `+projectColumns+`
        FROM presale_projects p JOIN presales s ON s.project_key = p.project_key
        WHERE p.project_key = $1 GROUP BY p.project_key`, key, recentFrom))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PresaleProject{}, false, nil
	}
	if err != nil {
		return domain.PresaleProject{}, false, fmt.Errorf("presale project: %w", err)
	}
	return project, true, nil
}

// PresaleUnits returns a project's presales, latest contract first.
func (repository *TransactionRepository) PresaleUnits(ctx context.Context, key string) ([]domain.Presale, error) {
	sql := `SELECT id, unit_label, floor_label, total_floors, building_type, transaction_type, total_price, parking_price,
            building_area_ping, price_per_ping, rooms, halls, baths, transacted_on, cancellation, cancelled_on, special_reasons, season
        FROM presales WHERE project_key = $1 ORDER BY transacted_on DESC, id LIMIT 3000`
	units := []domain.Presale{}
	err := scanRows(ctx, repository, sql, []any{key}, func(rows pgx.Rows) error {
		var unit domain.Presale
		var totalFloors *int
		if err := rows.Scan(&unit.ID, &unit.UnitLabel, &unit.FloorLabel, &totalFloors, &unit.BuildingType, &unit.TransactionType,
			&unit.TotalPrice, &unit.ParkingPrice, &unit.BuildingAreaPing, &unit.PricePerPing, &unit.Rooms, &unit.Halls, &unit.Baths,
			&unit.TransactedAt, &unit.Cancellation, &unit.CancelledOn, &unit.SpecialReasons, &unit.Season); err != nil {
			return err
		}
		if totalFloors != nil {
			unit.TotalFloors = *totalFloors
		}
		unit.BuildingPrice = unit.TotalPrice - unit.ParkingPrice
		units = append(units, unit)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("presale units: %w", err)
	}
	return units, nil
}

// ResalesNearby returns the count and median price per ping of residential resales within
// radius metres of a point from from to to (exclusive), for all ages and for homes at most
// maxNewAge years old.
func (repository *TransactionRepository) ResalesNearby(ctx context.Context, lat, lng, radius float64, maxNewAge int, from, to time.Time) (int, *float64, int, *float64, error) {
	var count, newCount int
	var median, newMedian *float64
	err := repository.pool.QueryRow(ctx, `SELECT COUNT(*), percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping),
            COUNT(*) FILTER (WHERE building_age <= $4),
            percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping) FILTER (WHERE building_age <= $4)
        FROM transactions
        WHERE ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3, FALSE)
          AND geocode_precision IN ('address', 'nearby')
          AND building_category = ANY (`+residentialCategories+`) AND `+marketSale+`
          AND transacted_on >= $5 AND transacted_on < $6`, lng, lat, radius, maxNewAge, from, to).Scan(&count, &median, &newCount, &newMedian)
	if err != nil {
		return 0, nil, 0, nil, fmt.Errorf("resales nearby: %w", err)
	}
	return count, median, newCount, newMedian, nil
}
