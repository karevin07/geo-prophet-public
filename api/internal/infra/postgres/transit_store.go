package postgres

import (
	"context"
	"fmt"
	"math"

	"github.com/geo-prophet/geo-prophet/api/internal/transit"
	"github.com/jackc/pgx/v5"
)

// insertStationDistances measures sales located at a door number (road and district
// centroids are too coarse) to each station's nearest exit, up to 800 m (the widest ring
// in domain.StationRings). The format verb takes an extra condition on transactions t.
const insertStationDistances = `INSERT INTO transaction_station_distances (station_id, transaction_id, distance_m)
    SELECT exit.station_id, t.id, MIN(ST_Distance(t.location, exit.location, FALSE))
    FROM transactions t JOIN transit_exits exit ON ST_DWithin(t.location, exit.location, 800, FALSE)
    WHERE t.geocode_precision IN ('address', 'nearby') AND %s
    GROUP BY exit.station_id, t.id`

// insertRentalStationDistances measures leases like insertStationDistances measures sales;
// the format verb takes an extra condition on rentals r.
const insertRentalStationDistances = `INSERT INTO rental_station_distances (station_id, rental_id, distance_m)
    SELECT exit.station_id, r.id, MIN(ST_Distance(r.location, exit.location, FALSE))
    FROM rentals r JOIN transit_exits exit ON ST_DWithin(r.location, exit.location, 800, FALSE)
    WHERE r.geocode_precision IN ('address', 'nearby') AND %s
    GROUP BY exit.station_id, r.id`

// routeMargin keeps a few kilometres of each route beyond the outermost stations.
const routeMargin = 0.03

// ReplaceTransit replaces all stations and lines, then re-measures every sale's and
// lease's distance to them, in one transaction so the API never sees a partial network.
// It returns the number of sale–station pairs within 800 m.
func (store *ImportStore) ReplaceTransit(ctx context.Context, network transit.Network) (int, error) {
	var pairs int
	err := pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error {
		// Stations cascade to their lines, exits and distances.
		if _, err := tx.Exec(ctx, `DELETE FROM transit_stations`); err != nil {
			return fmt.Errorf("delete stations: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM transit_lines`); err != nil {
			return fmt.Errorf("delete lines: %w", err)
		}
		west, south, east, north := extent(network.Stations)
		for _, line := range network.Lines {
			// Rail routes run the length of Taiwan; keep the part near the covered cities.
			if _, err := tx.Exec(ctx, `INSERT INTO transit_lines (id, operator, system, name, name_en, color, geometry)
                VALUES ($1, $2, $3, $4, $5, $6, ST_Multi(ST_CollectionExtract(ST_Intersection(
                    ST_GeomFromText($7, 4326), ST_MakeEnvelope($8, $9, $10, $11, 4326)), 2))::GEOGRAPHY)`,
				line.ID, line.Operator, line.System, line.Name, line.NameEn, line.Color, line.Geometry,
				west, south, east, north); err != nil {
				return fmt.Errorf("insert line %s: %w", line.ID, err)
			}
		}
		for _, station := range network.Stations {
			if err := insertStation(ctx, tx, station); err != nil {
				return err
			}
		}
		// Opening dates come from transit_openings, edited in the admin console.
		if _, err := tx.Exec(ctx, `SELECT refresh_transit_openings()`); err != nil {
			return fmt.Errorf("apply opening dates: %w", err)
		}
		tag, err := tx.Exec(ctx, fmt.Sprintf(insertStationDistances, "TRUE"))
		if err != nil {
			return fmt.Errorf("measure station distances: %w", err)
		}
		pairs = int(tag.RowsAffected())
		if _, err = tx.Exec(ctx, fmt.Sprintf(insertRentalStationDistances, "TRUE")); err != nil {
			return fmt.Errorf("measure rental station distances: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if _, err = store.pool.Exec(ctx, `ANALYZE transaction_station_distances, rental_station_distances`); err != nil {
		return pairs, fmt.Errorf("analyze station distances: %w", err)
	}
	return pairs, nil
}

func insertStation(ctx context.Context, tx pgx.Tx, station transit.Station) error {
	systems := make([]string, 0, 4)
	for _, system := range station.Systems() {
		systems = append(systems, string(system))
	}
	if _, err := tx.Exec(ctx, `INSERT INTO transit_stations (id, name, name_en, city_code, district, location, systems)
        VALUES ($1, $2, $3, $4, $5, ST_SetSRID(ST_MakePoint($6, $7), 4326)::GEOGRAPHY, $8)`,
		station.ID, station.Name, station.NameEn, station.CityCode, station.District, station.Lng, station.Lat, systems); err != nil {
		return fmt.Errorf("insert station %s: %w", station.ID, err)
	}
	for _, platform := range station.Platforms {
		for _, stop := range platform.Lines {
			if _, err := tx.Exec(ctx, `INSERT INTO transit_station_lines (station_id, platform_id, line_id, sequence, platform_name)
                VALUES ($1, $2, $3, $4, $5)`, station.ID, platform.ID, stop.LineID, stop.Sequence, platform.Name); err != nil {
				return fmt.Errorf("insert platform %s: %w", platform.ID, err)
			}
		}
	}
	for _, exit := range station.Exits() {
		if _, err := tx.Exec(ctx, `INSERT INTO transit_exits (station_id, exit_id, name, location)
            VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::GEOGRAPHY) ON CONFLICT DO NOTHING`,
			station.ID, exit.ID, exit.Name, exit.Lng, exit.Lat); err != nil {
			return fmt.Errorf("insert exit %s: %w", exit.ID, err)
		}
	}
	return nil
}

// extent returns the stations' bounding box widened by routeMargin.
func extent(stations []transit.Station) (west, south, east, north float64) {
	west, south, east, north = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, station := range stations {
		for _, platform := range station.Platforms {
			west, east = min(west, platform.Lng), max(east, platform.Lng)
			south, north = min(south, platform.Lat), max(north, platform.Lat)
		}
	}
	return west - routeMargin, south - routeMargin, east + routeMargin, north + routeMargin
}
