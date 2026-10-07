package mock

import (
	"context"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

// The mock repository has no rail network: stations need the TDX import.

func (*TransactionRepository) TransitLines(context.Context) ([]domain.TransitLine, error) {
	return []domain.TransitLine{}, nil
}

func (*TransactionRepository) TransitStations(context.Context, string) ([]domain.TransitStation, error) {
	return []domain.TransitStation{}, nil
}

func (*TransactionRepository) TransitStation(context.Context, string) (domain.TransitStation, bool, error) {
	return domain.TransitStation{}, false, nil
}

func (*TransactionRepository) StationRings(context.Context, string, domain.TransactionFilter, []int) ([]domain.StationRing, error) {
	return nil, nil
}

func (*TransactionRepository) StationRingYoY(context.Context, string, domain.TransactionFilter, []int, time.Time, time.Time, time.Time) (map[int]domain.DistrictYoYTotals, error) {
	return nil, nil
}

func (*TransactionRepository) StationYoYTotals(context.Context, domain.TransactionFilter, int, time.Time, time.Time, time.Time) (map[string]domain.DistrictYoYTotals, error) {
	return nil, nil
}

func (*TransactionRepository) StationRankings(context.Context, domain.TransactionFilter, int) ([]domain.StationRank, error) {
	return nil, nil
}

func (*TransactionRepository) OpeningEvents(context.Context) ([]domain.OpeningEvent, error) {
	return nil, nil
}

func (*TransactionRepository) OpeningSums(context.Context, domain.TransactionFilter, int, []domain.OpeningEvent, int) (map[int]map[string]domain.MonthlySums, map[int]map[string]domain.MonthlySums, error) {
	return nil, nil, nil
}

func (*TransactionRepository) FindStations(context.Context, string, int) ([]domain.Place, error) {
	return nil, nil
}

func (*TransactionRepository) DistanceCurve(context.Context, domain.TransactionFilter) ([]domain.DistanceBucket, error) {
	return []domain.DistanceBucket{}, nil
}
