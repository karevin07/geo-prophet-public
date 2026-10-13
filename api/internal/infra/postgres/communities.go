package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// communityColumns are scanned by scanCommunity from communities c.
const communityColumns = `c.community_key, c.name, c.city_code, c.district, c.street, c.door_numbers,
    c.category, c.completed_on, c.total_floors, c.sales, c.last_sale,
    ST_Y(c.location::geometry), ST_X(c.location::geometry)`

// recentMarket summarises the market sales of community c from $from to $to (exclusive).
func recentMarket(from, to string) string {
	return `JOIN LATERAL (
        SELECT COUNT(*) AS sales, percentile_cont(0.5) WITHIN GROUP (ORDER BY t.price_per_ping) AS median
        FROM transactions t
        WHERE t.community_key = c.community_key AND t.transaction_type LIKE '房地%' AND NOT t.is_special
          AND t.transacted_on >= ` + from + ` AND t.transacted_on < ` + to + `) recent ON TRUE`
}

func scanCommunity(row pgx.Row, extra ...any) (domain.CommunityRecord, error) {
	var record domain.CommunityRecord
	var completed, last time.Time
	err := row.Scan(append([]any{&record.ID, &record.Name, &record.CityCode, &record.District, &record.Street, &record.Doors,
		&record.Category, &completed, &record.TotalFloors, &record.Sales, &last, &record.Lat, &record.Lng}, extra...)...)
	record.CompletedOn, record.LastSale = completed.Format("2006-01-02"), last.Format("2006-01-02")
	record.DoorNumbers = len(record.Doors)
	return record, err
}

func (repository *TransactionRepository) communities(ctx context.Context, sql string, args []any, extra func() []any, after func(*domain.CommunityRecord)) ([]domain.CommunityRecord, error) {
	records := []domain.CommunityRecord{}
	err := scanRows(ctx, repository, sql, args, func(rows pgx.Rows) error {
		var targets []any
		if extra != nil {
			targets = extra()
		}
		record, err := scanCommunity(rows, targets...)
		if err != nil {
			return err
		}
		if after != nil {
			after(&record)
		}
		records = append(records, record)
		return nil
	})
	return records, err
}

// Community returns one community and whether it exists.
func (repository *TransactionRepository) Community(ctx context.Context, id string) (domain.CommunityRecord, bool, error) {
	record, err := scanCommunity(repository.pool.QueryRow(ctx, `SELECT `+communityColumns+`
        FROM communities c WHERE c.community_key = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CommunityRecord{}, false, nil
	}
	if err != nil {
		return domain.CommunityRecord{}, false, fmt.Errorf("community: %w", err)
	}
	return record, true, nil
}

// CommunitiesByID returns the communities among ids that exist, in no particular order.
func (repository *TransactionRepository) CommunitiesByID(ctx context.Context, ids []string) ([]domain.CommunityRecord, error) {
	records, err := repository.communities(ctx, `SELECT `+communityColumns+`
        FROM communities c WHERE c.community_key = ANY ($1)`, []any{ids}, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("communities by id: %w", err)
	}
	return records, nil
}

// CommunitiesByDoor returns the communities with a door number, latest completed first:
// a rebuilt building keeps its door number.
func (repository *TransactionRepository) CommunitiesByDoor(ctx context.Context, address string) ([]domain.CommunityRecord, error) {
	records, err := repository.communities(ctx, `SELECT `+communityColumns+`
        FROM communities c
        WHERE c.door_numbers @> ARRAY[door_number($1)]
        ORDER BY c.completed_on DESC, c.community_key LIMIT 5`, []any{address}, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("communities by door: %w", err)
	}
	return records, nil
}

// CommunitiesByName returns communities named after a presale project whose name contains
// text, the ones with the most sales first.
func (repository *TransactionRepository) CommunitiesByName(ctx context.Context, text string, limit int) ([]domain.CommunityRecord, error) {
	escaped := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(text)
	records, err := repository.communities(ctx, `SELECT `+communityColumns+`
        FROM communities c
        WHERE c.name LIKE '%' || $1 || '%' ESCAPE '\'
        ORDER BY c.name = $2 DESC, c.sales DESC, c.community_key LIMIT $3`, []any{escaped, text, limit}, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("communities by name: %w", err)
	}
	return records, nil
}

// CommunitySales returns a community's sales, latest first, leaving out those dated after
// today like whereClause does.
func (repository *TransactionRepository) CommunitySales(ctx context.Context, id string) ([]domain.Transaction, error) {
	rows, err := repository.pool.Query(ctx, `SELECT `+transactionSelectColumns+`, FALSE FROM transactions
        WHERE community_key = $1 AND transacted_on <= $2 ORDER BY transacted_on DESC, id LIMIT 3000`, id, today())
	if err != nil {
		return nil, fmt.Errorf("community sales: %w", err)
	}
	sales, err := pgx.CollectRows(rows, scanTransaction)
	if err != nil {
		return nil, fmt.Errorf("scan community sales: %w", err)
	}
	return sales, nil
}

// DistrictYearMedians returns the median price per ping of a district's market sales of
// one category in each year from from to today.
func (repository *TransactionRepository) DistrictYearMedians(ctx context.Context, cityCode, district, category string, from time.Time) (map[int]float64, error) {
	medians := map[int]float64{}
	err := scanRows(ctx, repository, `SELECT EXTRACT(YEAR FROM transacted_on)::INTEGER,
            percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping)
        FROM transactions
        WHERE city_code = $1 AND district = $2 AND building_category = $3 AND `+marketSale+`
          AND transacted_on >= $4 AND transacted_on <= CURRENT_DATE
        GROUP BY 1`, []any{cityCode, district, category, from}, func(rows pgx.Rows) error {
		var year int
		var median float64
		if err := rows.Scan(&year, &median); err != nil {
			return err
		}
		medians[year] = median
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("district year medians: %w", err)
	}
	return medians, nil
}

// DistrictMedian returns the count and median price per ping of a district's market sales
// of one category from from to to (exclusive).
func (repository *TransactionRepository) DistrictMedian(ctx context.Context, cityCode, district, category string, from, to time.Time) (domain.CommunityStats, error) {
	var stats domain.CommunityStats
	err := repository.pool.QueryRow(ctx, `SELECT COUNT(*), percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping)
        FROM transactions
        WHERE city_code = $1 AND district = $2 AND building_category = $3 AND `+marketSale+`
          AND transacted_on >= $4 AND transacted_on < $5`, cityCode, district, category, from, to).Scan(&stats.Sales, &stats.MedianPricePerPing)
	if err != nil {
		return stats, fmt.Errorf("district median: %w", err)
	}
	return stats, nil
}

// CommunityLeases returns the count and median rent per ping and monthly rent of a
// community's whole-home market leases (new, not special, not social housing) since from.
func (repository *TransactionRepository) CommunityLeases(ctx context.Context, id string, from time.Time) (int, *float64, *float64, error) {
	var count int
	var perPing, monthly *float64
	err := repository.pool.QueryRow(ctx, `SELECT COUNT(*), percentile_cont(0.5) WITHIN GROUP (ORDER BY rent_per_ping),
            percentile_cont(0.5) WITHIN GROUP (ORDER BY monthly_rent)
        FROM rentals
        WHERE community_key = $1 AND whole_unit AND NOT renewal AND NOT is_special AND NOT social_housing
          AND leased_on >= $2 AND leased_on <= CURRENT_DATE`, id, from).Scan(&count, &perPing, &monthly)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("community leases: %w", err)
	}
	return count, perPing, monthly, nil
}

// NearbyCommunities lists communities of the same category completed within maxYears of
// community id and within radius metres of it, with at least minSales market sales from
// from to to (exclusive), nearest first.
func (repository *TransactionRepository) NearbyCommunities(ctx context.Context, id string, radius float64, maxYears, minSales int, from, to time.Time, limit int) ([]domain.CommunityRecord, []domain.NearbyCommunity, error) {
	sql := `WITH target AS (SELECT location, category, completed_on FROM communities WHERE community_key = $1)
        SELECT ` + communityColumns + `, ST_Distance(c.location, target.location, FALSE) AS distance, recent.sales, recent.median
        FROM communities c CROSS JOIN target ` + recentMarket("$5", "$6") + `
        WHERE c.community_key <> $1 AND ST_DWithin(c.location, target.location, $2, FALSE)
          AND c.category = target.category AND abs(c.completed_on - target.completed_on) <= $3 * 365.25
          AND recent.sales >= $4
        ORDER BY distance, c.community_key LIMIT $7`
	nearby := []domain.NearbyCommunity{}
	var distance float64
	var sales int
	var median *float64
	records, err := repository.communities(ctx, sql, []any{id, radius, maxYears, minSales, from, to, limit},
		func() []any { return []any{&distance, &sales, &median} },
		func(record *domain.CommunityRecord) {
			nearby = append(nearby, domain.NearbyCommunity{DistanceM: math.Round(distance), RecentSales: sales, MedianPricePerPing: median})
		})
	if err != nil {
		return nil, nil, fmt.Errorf("nearby communities: %w", err)
	}
	return records, nearby, nil
}

// RankedCommunities lists a district's communities by their market sales from from to to
// (exclusive), the most first.
func (repository *TransactionRepository) RankedCommunities(ctx context.Context, cityCode, district string, from, to time.Time, limit int) ([]domain.CommunityRecord, []domain.RankedCommunity, error) {
	sql := `SELECT ` + communityColumns + `, ranked.sales, ranked.median
        FROM (SELECT community_key, COUNT(*) AS sales, percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_ping) AS median
              FROM transactions
              WHERE city_code = $1 AND district = $2 AND community_key IS NOT NULL AND ` + marketSale + `
                AND transacted_on >= $3 AND transacted_on < $4
              GROUP BY community_key ORDER BY COUNT(*) DESC, community_key LIMIT $5) ranked
        JOIN communities c ON c.community_key = ranked.community_key
        ORDER BY ranked.sales DESC, c.community_key`
	var ranked []domain.RankedCommunity
	var sales int
	var median *float64
	records, err := repository.communities(ctx, sql, []any{cityCode, district, from, to, limit},
		func() []any { return []any{&sales, &median} },
		func(*domain.CommunityRecord) {
			ranked = append(ranked, domain.RankedCommunity{RecentSales: sales, MedianPricePerPing: median})
		})
	if err != nil {
		return nil, nil, fmt.Errorf("ranked communities: %w", err)
	}
	return records, ranked, nil
}
