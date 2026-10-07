package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

// routeTolerance simplifies routes to about 5 m, plenty for a city map.
const routeTolerance = 0.00005

func (repository *TransactionRepository) TransitLines(ctx context.Context) ([]domain.TransitLine, error) {
	rows, err := repository.pool.Query(ctx, `SELECT id, operator, system, name, name_en, color, to_char(opened_on, 'YYYY-MM-DD'),
        ST_AsGeoJSON(ST_Simplify(geometry::geometry, $1), 6)
        FROM transit_lines ORDER BY array_position(ARRAY['metro', 'lrt', 'tra', 'thsr'], system), id`, routeTolerance)
	if err != nil {
		return nil, fmt.Errorf("query transit lines: %w", err)
	}
	lines, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.TransitLine, error) {
		var line domain.TransitLine
		var geometry string
		err := row.Scan(&line.ID, &line.Operator, &line.System, &line.Name, &line.NameEn, &line.Color, &line.OpenedOn, &geometry)
		line.Geometry = []byte(geometry)
		return line, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan transit lines: %w", err)
	}
	if lines == nil {
		return []domain.TransitLine{}, nil
	}
	return lines, nil
}

const stationSelect = `SELECT s.id, s.name, s.name_en, s.city_code, s.district, ST_Y(s.location::geometry), ST_X(s.location::geometry),
        s.systems, to_char(s.opened_on, 'YYYY-MM-DD'),
        COALESCE(json_agg(json_build_object('lineId', l.id, 'name', l.name, 'color', l.color, 'platformId', sl.platform_id,
            'platformName', sl.platform_name, 'sequence', sl.sequence, 'openedOn', to_char(sl.opened_on, 'YYYY-MM-DD'))
            ORDER BY array_position(ARRAY['metro', 'lrt', 'tra', 'thsr'], l.system), l.id) FILTER (WHERE l.id IS NOT NULL), '[]')
        FROM transit_stations s
        LEFT JOIN transit_station_lines sl ON sl.station_id = s.id
        LEFT JOIN transit_lines l ON l.id = sl.line_id`

func scanStation(row pgx.CollectableRow) (domain.TransitStation, error) {
	var station domain.TransitStation
	err := row.Scan(&station.ID, &station.Name, &station.NameEn, &station.CityCode, &station.District, &station.Lat, &station.Lng,
		&station.Systems, &station.OpenedOn, &station.Lines)
	return station, err
}

func (repository *TransactionRepository) TransitStations(ctx context.Context, cityCode string) ([]domain.TransitStation, error) {
	rows, err := repository.pool.Query(ctx, stationSelect+` WHERE $1 = '' OR s.city_code = $1
        GROUP BY s.id ORDER BY s.city_code, s.district, s.name`, cityCode)
	if err != nil {
		return nil, fmt.Errorf("query transit stations: %w", err)
	}
	stations, err := pgx.CollectRows(rows, scanStation)
	if err != nil {
		return nil, fmt.Errorf("scan transit stations: %w", err)
	}
	if stations == nil {
		return []domain.TransitStation{}, nil
	}
	return stations, nil
}

func (repository *TransactionRepository) TransitStation(ctx context.Context, id string) (domain.TransitStation, bool, error) {
	rows, err := repository.pool.Query(ctx, stationSelect+` WHERE s.id = $1 GROUP BY s.id`, id)
	if err != nil {
		return domain.TransitStation{}, false, fmt.Errorf("query transit station: %w", err)
	}
	station, err := pgx.CollectExactlyOneRow(rows, scanStation)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TransitStation{}, false, nil
	}
	if err != nil {
		return domain.TransitStation{}, false, fmt.Errorf("scan transit station: %w", err)
	}
	return station, true, nil
}

// stationSales joins a station's measured sales to the rings containing them; the format
// verbs take the station and radii placeholders, then the shared filter.
const stationSales = `FROM transaction_station_distances d
        JOIN transactions ON transactions.id = d.transaction_id
        JOIN unnest(%s::INTEGER[]) AS ring(radius) ON d.distance_m <= ring.radius
        WHERE d.station_id = %s AND %s`

func (repository *TransactionRepository) StationRings(ctx context.Context, stationID string, filter domain.TransactionFilter, radii []int) ([]domain.StationRing, error) {
	where, args := whereClause(filter)
	args = append(args, radii, stationID)
	from := fmt.Sprintf(stationSales, fmt.Sprintf("$%d", len(args)-1), fmt.Sprintf("$%d", len(args)), where)

	rings := make([]domain.StationRing, len(radii))
	byRadius := make(map[int]*domain.StationRing, len(radii))
	for index, radius := range radii {
		rings[index] = domain.StationRing{Radius: radius, Trends: []domain.TrendPoint{}}
		byRadius[radius] = &rings[index]
	}
	rows, err := repository.pool.Query(ctx, `SELECT ring.radius, COUNT(*),
        SUM(building_price)::DOUBLE PRECISION / NULLIF(SUM(building_area_ping), 0),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping) `+from+` GROUP BY ring.radius`, args...)
	if err != nil {
		return nil, fmt.Errorf("query station rings: %w", err)
	}
	var radius int
	var ring domain.StationRing
	if _, err = pgx.ForEachRow(rows, []any{&radius, &ring.Count, &ring.AvgPricePerPing, &ring.MedianPricePerPing}, func() error {
		target := byRadius[radius]
		target.Count, target.AvgPricePerPing, target.MedianPricePerPing = ring.Count, ring.AvgPricePerPing, ring.MedianPricePerPing
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan station rings: %w", err)
	}

	// The trend spans all years regardless of the window.
	allYears := filter
	allYears.From, allYears.To = nil, nil
	where, args = whereClause(allYears)
	args = append(args, radii, stationID)
	from = fmt.Sprintf(stationSales, fmt.Sprintf("$%d", len(args)-1), fmt.Sprintf("$%d", len(args)), where)
	rows, err = repository.pool.Query(ctx, `SELECT ring.radius, to_char(transacted_on, 'YYYY-MM') AS month,
        SUM(building_price)::DOUBLE PRECISION / SUM(building_area_ping), COUNT(*) `+from+`
        GROUP BY ring.radius, month ORDER BY ring.radius, month`, args...)
	if err != nil {
		return nil, fmt.Errorf("query station trends: %w", err)
	}
	var point domain.TrendPoint
	if _, err = pgx.ForEachRow(rows, []any{&radius, &point.Month, &point.AvgPricePerPing, &point.Count}, func() error {
		byRadius[radius].Trends = append(byRadius[radius].Trends, point)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan station trends: %w", err)
	}
	return rings, nil
}

func (repository *TransactionRepository) StationRingYoY(ctx context.Context, stationID string, filter domain.TransactionFilter, radii []int, previousFrom, currentFrom, currentTo time.Time) (map[int]domain.DistrictYoYTotals, error) {
	filter.From, filter.To = &previousFrom, &currentTo
	where, args := whereClause(filter)
	args = append(args, radii, stationID, currentFrom)
	current := fmt.Sprintf("$%d", len(args))
	rows, err := repository.pool.Query(ctx, fmt.Sprintf(`SELECT ring.radius,
        COUNT(*) FILTER (WHERE transacted_on >= %[1]s), COUNT(*) FILTER (WHERE transacted_on < %[1]s),
        COALESCE(SUM(building_price) FILTER (WHERE transacted_on >= %[1]s), 0),
        COALESCE(SUM(building_price) FILTER (WHERE transacted_on < %[1]s), 0),
        COALESCE(SUM(building_area_ping) FILTER (WHERE transacted_on >= %[1]s), 0),
        COALESCE(SUM(building_area_ping) FILTER (WHERE transacted_on < %[1]s), 0) `, current)+
		fmt.Sprintf(stationSales, fmt.Sprintf("$%d", len(args)-2), fmt.Sprintf("$%d", len(args)-1), where)+` GROUP BY ring.radius`, args...)
	if err != nil {
		return nil, fmt.Errorf("query station yoy: %w", err)
	}
	totals := make(map[int]domain.DistrictYoYTotals, len(radii))
	var radius int
	var value domain.DistrictYoYTotals
	if _, err = pgx.ForEachRow(rows, []any{&radius, &value.CurrentCount, &value.PreviousCount, &value.CurrentBuildingPrice,
		&value.PreviousBuildingPrice, &value.CurrentArea, &value.PreviousArea}, func() error {
		totals[radius] = value
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan station yoy: %w", err)
	}
	return totals, nil
}

func (repository *TransactionRepository) StationYoYTotals(ctx context.Context, filter domain.TransactionFilter, radius int, previousFrom, currentFrom, currentTo time.Time) (map[string]domain.DistrictYoYTotals, error) {
	filter.From, filter.To = &previousFrom, &currentTo
	where, args := whereClause(filter)
	args = append(args, radius, currentFrom)
	current := fmt.Sprintf("$%d", len(args))
	rows, err := repository.pool.Query(ctx, fmt.Sprintf(`SELECT d.station_id,
        COUNT(*) FILTER (WHERE transacted_on >= %[1]s), COUNT(*) FILTER (WHERE transacted_on < %[1]s),
        COALESCE(SUM(building_price) FILTER (WHERE transacted_on >= %[1]s), 0),
        COALESCE(SUM(building_price) FILTER (WHERE transacted_on < %[1]s), 0),
        COALESCE(SUM(building_area_ping) FILTER (WHERE transacted_on >= %[1]s), 0),
        COALESCE(SUM(building_area_ping) FILTER (WHERE transacted_on < %[1]s), 0)
        FROM transaction_station_distances d JOIN transactions ON transactions.id = d.transaction_id
        WHERE d.distance_m <= $%[2]d AND %[3]s GROUP BY d.station_id`, current, len(args)-1, where), args...)
	if err != nil {
		return nil, fmt.Errorf("query station yoy totals: %w", err)
	}
	totals := make(map[string]domain.DistrictYoYTotals)
	var stationID string
	var value domain.DistrictYoYTotals
	if _, err = pgx.ForEachRow(rows, []any{&stationID, &value.CurrentCount, &value.PreviousCount, &value.CurrentBuildingPrice,
		&value.PreviousBuildingPrice, &value.CurrentArea, &value.PreviousArea}, func() error {
		totals[stationID] = value
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan station yoy totals: %w", err)
	}
	return totals, nil
}

// StationRankings returns every station's figures within radius for the filter's window,
// with its monthly trend over the same window.
func (repository *TransactionRepository) StationRankings(ctx context.Context, filter domain.TransactionFilter, radius int) ([]domain.StationRank, error) {
	where, args := whereClause(filter)
	args = append(args, radius)
	from := fmt.Sprintf(`FROM transaction_station_distances d JOIN transactions ON transactions.id = d.transaction_id
        WHERE d.distance_m <= $%d AND %s`, len(args), where)

	ranks := map[string]*domain.StationRank{}
	rows, err := repository.pool.Query(ctx, `SELECT d.station_id, COUNT(*),
        SUM(building_price)::DOUBLE PRECISION / NULLIF(SUM(building_area_ping), 0),
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping) `+from+` GROUP BY d.station_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("query station rankings: %w", err)
	}
	var rank domain.StationRank
	if _, err = pgx.ForEachRow(rows, []any{&rank.StationID, &rank.Count, &rank.AvgPricePerPing, &rank.MedianPricePerPing}, func() error {
		copied := rank
		copied.Trend = []domain.TrendPoint{}
		ranks[rank.StationID] = &copied
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan station rankings: %w", err)
	}

	rows, err = repository.pool.Query(ctx, `SELECT d.station_id, to_char(transacted_on, 'YYYY-MM') AS month,
        SUM(building_price)::DOUBLE PRECISION / SUM(building_area_ping), COUNT(*) `+from+`
        GROUP BY d.station_id, month ORDER BY d.station_id, month`, args...)
	if err != nil {
		return nil, fmt.Errorf("query station ranking trends: %w", err)
	}
	var stationID string
	var point domain.TrendPoint
	if _, err = pgx.ForEachRow(rows, []any{&stationID, &point.Month, &point.AvgPricePerPing, &point.Count}, func() error {
		if target := ranks[stationID]; target != nil {
			target.Trend = append(target.Trend, point)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan station ranking trends: %w", err)
	}

	result := make([]domain.StationRank, 0, len(ranks))
	for _, rank := range ranks {
		result = append(result, *rank)
	}
	return result, nil
}

// OpeningEvents lists each line's first platform opening at each station since 2015.
func (repository *TransactionRepository) OpeningEvents(ctx context.Context) ([]domain.OpeningEvent, error) {
	rows, err := repository.pool.Query(ctx, `SELECT s.id, s.name, s.city_code, s.district, l.id, l.name, l.color,
        to_char(MIN(sl.opened_on), 'YYYY-MM-DD')
        FROM transit_station_lines sl
        JOIN transit_stations s ON s.id = sl.station_id
        JOIN transit_lines l ON l.id = sl.line_id
        WHERE sl.opened_on IS NOT NULL
        GROUP BY s.id, l.id ORDER BY MIN(sl.opened_on), s.name`)
	if err != nil {
		return nil, fmt.Errorf("query opening events: %w", err)
	}
	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.OpeningEvent, error) {
		var event domain.OpeningEvent
		err := row.Scan(&event.StationID, &event.StationName, &event.CityCode, &event.District,
			&event.LineID, &event.LineName, &event.LineColor, &event.OpenedOn)
		return event, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan opening events: %w", err)
	}
	return events, nil
}

// OpeningSums returns monthly sums within `months` months of each event's opening month,
// by event index and YYYY-MM: sales within radius of the station, and the baseline of
// located sales in the station's district that are not within 800 m of it. The baseline is
// the district's monthly sums minus those of its sales within 800 m of the station, which
// scans each district once instead of once per event.
func (repository *TransactionRepository) OpeningSums(ctx context.Context, filter domain.TransactionFilter, radius int, events []domain.OpeningEvent, months int) (station, baseline map[int]map[string]domain.MonthlySums, err error) {
	filter.From, filter.To = nil, nil
	where, args := whereClause(filter)
	base := len(args)
	indexes := make([]int32, len(events))
	stationIDs := make([]string, len(events))
	cities := make([]string, len(events))
	districts := make([]string, len(events))
	opened := make([]string, len(events))
	var first, last time.Time
	for index, event := range events {
		indexes[index], stationIDs[index], cities[index], districts[index] = int32(index), event.StationID, event.CityCode, event.District
		opened[index] = event.OpenedOn[:7] + "-01"
		month, err := time.Parse(time.DateOnly, opened[index])
		if err != nil {
			return nil, nil, fmt.Errorf("opening date of %s: %w", event.StationID, err)
		}
		if first.IsZero() || month.Before(first) {
			first = month
		}
		if month.After(last) {
			last = month
		}
	}
	eventsCTE := fmt.Sprintf(`WITH events AS (
        SELECT idx, station_id, ev_city, ev_district,
            opened::DATE - make_interval(months => $%[6]d) AS window_from,
            opened::DATE + make_interval(months => $%[6]d) AS window_to
        FROM unnest($%[1]d::INTEGER[], $%[2]d::TEXT[], $%[3]d::TEXT[], $%[4]d::TEXT[], $%[5]d::TEXT[])
            AS e(idx, station_id, ev_city, ev_district, opened))`, base+1, base+2, base+3, base+4, base+5, base+6)
	eventArgs := append(append([]any{}, args...), indexes, stationIDs, cities, districts, opened, months)
	// Both pair queries join the same way; the nearby one keeps only sales in the station's
	// district through FILTER, since a join condition on district made PostgreSQL scan
	// whole districts and took seconds.
	pairs := func(inDistrict bool) string {
		match := "TRUE"
		if inDistrict {
			match = "transactions.city_code = e.ev_city AND transactions.district = e.ev_district"
		}
		return eventsCTE + fmt.Sprintf(` SELECT e.idx, to_char(transacted_on, 'YYYY-MM'),
                COUNT(*) FILTER (WHERE %[1]s),
                COALESCE(SUM(building_price) FILTER (WHERE %[1]s), 0)::BIGINT,
                COALESCE(SUM(building_area_ping) FILTER (WHERE %[1]s), 0)
            FROM events e
            JOIN transaction_station_distances d ON d.station_id = e.station_id AND d.distance_m <= $%[2]d
            JOIN transactions ON transactions.id = d.transaction_id
            WHERE transacted_on >= e.window_from AND transacted_on < e.window_to AND %[3]s
            GROUP BY 1, 2`, match, base+7, where)
	}

	collect := func(sql string, args []any) (map[int]map[string]domain.MonthlySums, error) {
		rows, err := repository.pool.Query(ctx, sql, args...)
		if err != nil {
			return nil, err
		}
		result := map[int]map[string]domain.MonthlySums{}
		var index int
		var month string
		var value domain.MonthlySums
		_, err = pgx.ForEachRow(rows, []any{&index, &month, &value.Count, &value.BuildingPrice, &value.Area}, func() error {
			if value.Count == 0 {
				return nil
			}
			if result[index] == nil {
				result[index] = map[string]domain.MonthlySums{}
			}
			result[index][month] = value
			return nil
		})
		return result, err
	}
	// Monthly sums of every located sale in the events' districts over all their windows.
	districtArgs := append(append([]any{}, args...), cities, districts,
		first.AddDate(0, -months, 0), last.AddDate(0, months, 0))
	districtSQL := fmt.Sprintf(`SELECT transactions.city_code, transactions.district,
            to_char(transacted_on, 'YYYY-MM'), COUNT(*), SUM(building_price)::BIGINT, SUM(building_area_ping)
        FROM transactions JOIN (SELECT DISTINCT * FROM unnest($%d::TEXT[], $%d::TEXT[])) AS areas(city, district)
            ON transactions.city_code = areas.city AND transactions.district = areas.district
        WHERE transacted_on >= $%d AND transacted_on < $%d AND geocode_precision IN ('address', 'nearby') AND %s
        GROUP BY 1, 2, 3`, base+1, base+2, base+3, base+4, where)

	var near map[int]map[string]domain.MonthlySums
	districtSums := map[string]domain.MonthlySums{}
	group, groupCtx := errgroup.WithContext(ctx)
	ctx = groupCtx
	group.Go(func() (err error) {
		if station, err = collect(pairs(false), append(eventArgs, radius)); err != nil {
			err = fmt.Errorf("query opening station sums: %w", err)
		}
		return err
	})
	group.Go(func() (err error) {
		if near, err = collect(pairs(true), append(eventArgs, 800)); err != nil {
			err = fmt.Errorf("query opening nearby sums: %w", err)
		}
		return err
	})
	group.Go(func() error {
		rows, err := repository.pool.Query(groupCtx, districtSQL, districtArgs...)
		if err != nil {
			return fmt.Errorf("query opening district sums: %w", err)
		}
		var city, district, month string
		var value domain.MonthlySums
		if _, err = pgx.ForEachRow(rows, []any{&city, &district, &month, &value.Count, &value.BuildingPrice, &value.Area}, func() error {
			districtSums[city+"|"+district+"|"+month] = value
			return nil
		}); err != nil {
			return fmt.Errorf("scan opening district sums: %w", err)
		}
		return nil
	})
	if err = group.Wait(); err != nil {
		return nil, nil, err
	}

	baseline = make(map[int]map[string]domain.MonthlySums, len(events))
	for index, event := range events {
		start, _ := time.Parse(time.DateOnly, opened[index])
		baseline[index] = map[string]domain.MonthlySums{}
		for offset := -months; offset < months; offset++ {
			label := start.AddDate(0, offset, 0).Format("2006-01")
			total, excluded := districtSums[event.CityCode+"|"+event.District+"|"+label], near[index][label]
			if remaining := (domain.MonthlySums{Count: total.Count - excluded.Count, BuildingPrice: total.BuildingPrice - excluded.BuildingPrice, Area: total.Area - excluded.Area}); remaining.Count > 0 {
				baseline[index][label] = remaining
			}
		}
	}
	return station, baseline, nil
}

// FindStations matches station names by prefix after writing 臺 as 台. Names equal to the
// query once 站 or 車站 is dropped come first, so 台北 finds 台北車站 before 台北橋; then
// shorter names.
func (repository *TransactionRepository) FindStations(ctx context.Context, name string, limit int) ([]domain.Place, error) {
	rows, err := repository.pool.Query(ctx, `SELECT id, name, city_code, district, ST_Y(location::geometry), ST_X(location::geometry)
        FROM transit_stations WHERE replace(name, '臺', '台') LIKE replace(replace($1, '%', ''), '_', '') || '%'
        ORDER BY regexp_replace(replace(name, '臺', '台'), '(車站|站)$', '') = $1 DESC, length(name), name LIMIT $2`, name, limit)
	if err != nil {
		return nil, fmt.Errorf("query stations by name: %w", err)
	}
	places, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Place, error) {
		var place domain.Place
		var stationName, cityCode, district string
		err := row.Scan(&place.StationID, &stationName, &cityCode, &district, &place.Lat, &place.Lng)
		place.Kind = domain.PlaceStation
		place.Label = stationName + "｜" + lvr.Cities[cityCode] + district
		return place, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan stations: %w", err)
	}
	return places, nil
}

// DistanceCurve groups the filter's located sales by city and by distance to their nearest
// station exit in 100 m buckets up to 800 m; sales farther away (no stored pair) form the
// last bucket.
func (repository *TransactionRepository) DistanceCurve(ctx context.Context, filter domain.TransactionFilter) ([]domain.DistanceBucket, error) {
	where, args := whereClause(filter)
	const beyond = 800 / domain.DistanceBucketM
	rows, err := repository.pool.Query(ctx, fmt.Sprintf(`SELECT t.city_code,
            -- LEAST ignores NULLs, so sales with no nearby station need their own branch.
            CASE WHEN nearest.distance IS NULL THEN %[2]d
                 ELSE LEAST(floor(nearest.distance / %[1]d)::INTEGER, %[2]d - 1) END AS bucket,
            COUNT(*), SUM(t.building_price)::DOUBLE PRECISION / NULLIF(SUM(t.building_area_ping), 0),
            percentile_cont(0.5) WITHIN GROUP (ORDER BY t.price_per_ping)
        FROM (SELECT id, city_code, building_price, building_area_ping, price_per_ping FROM transactions
              WHERE geocode_precision IN ('address', 'nearby') AND %[3]s) t
        LEFT JOIN LATERAL (SELECT MIN(distance_m) AS distance FROM transaction_station_distances
              WHERE transaction_id = t.id) nearest ON TRUE
        GROUP BY 1, 2 ORDER BY 1, 2`, domain.DistanceBucketM, beyond, where), args...)
	if err != nil {
		return nil, fmt.Errorf("query distance curve: %w", err)
	}
	buckets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.DistanceBucket, error) {
		var bucket domain.DistanceBucket
		var index int
		err := row.Scan(&bucket.CityCode, &index, &bucket.Count, &bucket.AvgPricePerPing, &bucket.MedianPricePerPing)
		bucket.FromM = index * domain.DistanceBucketM
		if index < beyond {
			to := bucket.FromM + domain.DistanceBucketM
			bucket.ToM = &to
		}
		return bucket, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan distance curve: %w", err)
	}
	if buckets == nil {
		return []domain.DistanceBucket{}, nil
	}
	return buckets, nil
}
