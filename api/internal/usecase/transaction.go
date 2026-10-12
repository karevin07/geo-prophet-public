package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/uber/h3-go/v4"
	"golang.org/x/sync/errgroup"
)

var ErrInvalidInput = errors.New("invalid input")

const (
	defaultListLimit = 500
	maxListLimit     = 2000
)

type TransactionRepository interface {
	List(context.Context, domain.TransactionFilter) ([]domain.Transaction, error)
	ForEachPricePoint(context.Context, domain.TransactionFilter, func(domain.PricePoint) error) error
	MonthlyTrends(context.Context, domain.TransactionFilter) ([]domain.TrendPoint, error)
	Summary(context.Context, domain.TransactionFilter) (domain.MarketSummary, error)
	Analysis(context.Context, domain.TransactionFilter) (domain.MarketAnalysis, error)
	DistrictRankings(context.Context, domain.DistrictComparisonQuery) ([]domain.DistrictRank, error)
	DistrictTrends(context.Context, domain.DistrictComparisonQuery) ([]domain.DistrictTrend, error)
	DistrictYoYTotals(context.Context, domain.DistrictComparisonQuery, time.Time, time.Time, time.Time) (map[string]domain.DistrictYoYTotals, error)
	DistrictBoundaries(ctx context.Context, cityCode string) ([]domain.DistrictBoundary, error)
	// CellRows returns the sales matching filter (its Bounds a cell's bounding box).
	CellRows(context.Context, domain.TransactionFilter) ([]domain.CellRow, error)
	// ImportedSales lists the season archives and 10-day release labels imported so far.
	ImportedSales(context.Context) (seasons []string, releases []string, err error)
	Ping(context.Context) error
}

const coverageTTL = 5 * time.Minute

type TransactionService struct {
	repository TransactionRepository
	now        func() time.Time

	coverageMu      sync.Mutex
	coverage        time.Time // first day of the last complete month
	coverageKnown   bool
	coverageFetched time.Time
}

func NewTransactionService(repository TransactionRepository) *TransactionService {
	return &TransactionService{repository: repository, now: time.Now}
}

// completeMonth returns the first day of the last month whose sales are fully registered.
// It changes only after imports, so it is cached briefly.
func (service *TransactionService) completeMonth(ctx context.Context) (time.Time, bool, error) {
	service.coverageMu.Lock()
	defer service.coverageMu.Unlock()
	if service.now().Sub(service.coverageFetched) < coverageTTL {
		return service.coverage, service.coverageKnown, nil
	}
	seasons, releases, err := service.repository.ImportedSales(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	registered, known := registeredThrough(seasons, releases)
	service.coverage, service.coverageKnown, service.coverageFetched = time.Time{}, known, service.now()
	if known {
		service.coverage = completeThrough(registered)
	}
	return service.coverage, known, nil
}

// CompleteMonth returns the first day of the last month whose sales are fully registered,
// and false when no season has been imported.
func (service *TransactionService) CompleteMonth(ctx context.Context) (time.Time, bool, error) {
	return service.completeMonth(ctx)
}

func (service *TransactionService) List(ctx context.Context, filter domain.TransactionFilter) ([]domain.Transaction, error) {
	if err := validateFilter(filter); err != nil {
		return nil, err
	}
	if filter.Limit <= 0 || filter.Limit > maxListLimit {
		filter.Limit = defaultListLimit
	}
	filter.PreciseLocation = true
	return service.repository.List(ctx, filter)
}

func (service *TransactionService) AggregateH3(ctx context.Context, filter domain.TransactionFilter, resolution int) ([]domain.H3Aggregate, error) {
	if resolution < 7 || resolution > 10 {
		return nil, fmt.Errorf("%w: resolution must be between 7 and 10", ErrInvalidInput)
	}
	if err := validateFilter(filter); err != nil {
		return nil, err
	}
	filter.PreciseLocation = true

	type accumulator struct {
		count         int
		totalPrice    int64
		buildingPrice int64
		totalArea     float64
	}
	groups := make(map[h3.Cell]accumulator)
	err := service.repository.ForEachPricePoint(ctx, filter, func(point domain.PricePoint) error {
		cell, cellErr := h3.LatLngToCell(h3.NewLatLng(point.Lat, point.Lng), resolution)
		if cellErr != nil {
			return fmt.Errorf("create h3 cell: %w", cellErr)
		}
		group := groups[cell]
		group.count++
		group.totalPrice += point.TotalPrice
		group.buildingPrice += point.BuildingPrice
		group.totalArea += point.BuildingAreaPing
		groups[cell] = group
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := make([]domain.H3Aggregate, 0, len(groups))
	for cell, group := range groups {
		center, centerErr := cell.LatLng()
		if centerErr != nil {
			return nil, fmt.Errorf("find h3 center: %w", centerErr)
		}
		result = append(result, domain.H3Aggregate{
			Hex:             cell.String(),
			Lat:             center.Lat,
			Lng:             center.Lng,
			Count:           group.count,
			Price:           float64(group.totalPrice) / float64(group.count),
			AvgArea:         group.totalArea / float64(group.count),
			AvgPricePerPing: float64(group.buildingPrice) / group.totalArea,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Hex < result[j].Hex })
	return result, nil
}

// Trends returns the area-weighted average price per ping (parking excluded) by month,
// marking months whose registrations are not complete yet.
func (service *TransactionService) Trends(ctx context.Context, filter domain.TransactionFilter) ([]domain.TrendPoint, error) {
	if err := validateFilter(filter); err != nil {
		return nil, err
	}
	trends, err := service.repository.MonthlyTrends(ctx, filter)
	if err != nil {
		return nil, err
	}
	complete, known, err := service.completeMonth(ctx)
	if err != nil {
		return nil, err
	}
	if known {
		last := complete.Format("2006-01")
		for index := range trends {
			trends[index].Provisional = trends[index].Month > last
		}
	}
	return trends, nil
}

// minYoYSales is the fewest sales either period needs before a change is reported.
const minYoYSales = 5

// Summary counts matching transactions with the area-weighted price per ping and the
// median total price. Like trends, it includes district-level locations. The year-over-
// year change compares a window of the same length (at least three months, since single
// months have too few sales) with the same months a year earlier. The window ends at the
// requested month or, if that is still being registered, at the last complete month.
func (service *TransactionService) Summary(ctx context.Context, filter domain.TransactionFilter) (domain.SummaryWithYoY, error) {
	if err := validateFilter(filter); err != nil {
		return domain.SummaryWithYoY{}, err
	}
	current, err := service.repository.Summary(ctx, filter)
	if err != nil || filter.From == nil || filter.To == nil {
		return domain.SummaryWithYoY{MarketSummary: current}, err
	}

	months := max(monthSpan(*filter.From, *filter.To), 3)
	end := time.Date(filter.To.Year(), filter.To.Month(), 1, 0, 0, 0, 0, time.UTC)
	complete, known, err := service.completeMonth(ctx)
	if err != nil {
		return domain.SummaryWithYoY{}, err
	}
	if known && end.After(complete) {
		end = complete // compare only fully registered months
	}
	yoy := &domain.YearOverYear{Basis: fmt.Sprintf("%dm", months), Through: end.Format("2006-01")}

	// The compared window has the requested length (at least 3 months) and ends at end.
	basis, basisCurrent := filter, current
	basisFrom, basisTo := end.AddDate(0, -(months-1), 0), end.AddDate(0, 1, 0).Add(-time.Nanosecond)
	if !basisFrom.Equal(*filter.From) || !basisTo.Equal(*filter.To) {
		basis.From, basis.To = &basisFrom, &basisTo
		if basisCurrent, err = service.repository.Summary(ctx, basis); err != nil {
			return domain.SummaryWithYoY{}, err
		}
	}
	previous := basis
	previousFrom, previousTo := basis.From.AddDate(-1, 0, 0), basis.To.AddDate(-1, 0, 0)
	previous.From, previous.To = &previousFrom, &previousTo
	basisPrevious, err := service.repository.Summary(ctx, previous)
	if err != nil {
		return domain.SummaryWithYoY{}, err
	}

	if basisCurrent.Count >= minYoYSales && basisPrevious.Count >= minYoYSales {
		yoy.Count = change(float64(basisCurrent.Count), float64(basisPrevious.Count))
		yoy.AvgPricePerPing = change(basisCurrent.AvgPricePerPing, basisPrevious.AvgPricePerPing)
		yoy.MedianTotalPrice = change(basisCurrent.MedianTotalPrice, basisPrevious.MedianTotalPrice)
	}
	return domain.SummaryWithYoY{MarketSummary: current, YoY: yoy}, nil
}

// Analysis returns database-aggregated multi-year series, category comparisons and a
// price-per-ping histogram. District-level locations remain useful for market statistics.
func (service *TransactionService) Analysis(ctx context.Context, filter domain.TransactionFilter) (domain.MarketAnalysis, error) {
	if err := validateFilter(filter); err != nil {
		return domain.MarketAnalysis{}, err
	}
	analysis, err := service.repository.Analysis(ctx, filter)
	if err != nil {
		return domain.MarketAnalysis{}, err
	}
	complete, known, err := service.completeMonth(ctx)
	if err != nil {
		return domain.MarketAnalysis{}, err
	}
	if known {
		last := complete.Format("2006-01")
		for index := range analysis.Monthly {
			analysis.Monthly[index].Provisional = analysis.Monthly[index].Month > last
		}
	}
	return analysis, nil
}

const maxComparedDistricts = 4

// CompareDistricts ranks all districts in one city, then returns monthly series for up to
// four selected districts. YoY compares the latest fully registered 12 months with the
// preceding 12 months, independent of the ranking's chosen date window.
func (service *TransactionService) CompareDistricts(ctx context.Context, query domain.DistrictComparisonQuery) (domain.DistrictComparison, error) {
	if err := validateFilter(query.Filter); err != nil {
		return domain.DistrictComparison{}, err
	}
	source, ok := addresspoint.Sources[query.CityCode]
	if !ok {
		return domain.DistrictComparison{}, fmt.Errorf("%w: unknown city %q", ErrInvalidInput, query.CityCode)
	}
	if len(query.Districts) > maxComparedDistricts {
		return domain.DistrictComparison{}, fmt.Errorf("%w: compare at most %d districts", ErrInvalidInput, maxComparedDistricts)
	}
	seen := make(map[string]bool, len(query.Districts))
	for _, district := range query.Districts {
		if _, ok := source.Districts[district]; !ok || seen[district] {
			return domain.DistrictComparison{}, fmt.Errorf("%w: unknown or duplicate district %q", ErrInvalidInput, district)
		}
		seen[district] = true
	}
	query.Filter.PreciseLocation = false
	result := domain.DistrictComparison{CityCode: query.CityCode, Ranking: []domain.DistrictRank{}, Trends: []domain.DistrictTrend{}}
	complete, known, err := service.completeMonth(ctx)
	if err != nil {
		return domain.DistrictComparison{}, err
	}

	// The ranking, trends and YoY totals are independent scans of up to hundreds of
	// thousands of rows, so run them concurrently.
	var yoy map[string]domain.DistrictYoYTotals
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		result.Ranking, err = service.repository.DistrictRankings(groupCtx, query)
		return err
	})
	if len(query.Districts) > 0 {
		group.Go(func() (err error) {
			result.Trends, err = service.repository.DistrictTrends(groupCtx, query)
			return err
		})
	}
	if known {
		currentFrom := complete.AddDate(0, -11, 0)
		currentTo := complete.AddDate(0, 1, 0).Add(-time.Nanosecond)
		group.Go(func() (err error) {
			yoy, err = service.repository.DistrictYoYTotals(groupCtx, query, currentFrom.AddDate(-1, 0, 0), currentFrom, currentTo)
			return err
		})
	}
	if err = group.Wait(); err != nil {
		return domain.DistrictComparison{}, err
	}
	if !known {
		return result, nil
	}
	result.YoYThrough = complete.Format("2006-01")
	for index := range result.Trends {
		result.Trends[index].Provisional = result.Trends[index].Month > result.YoYThrough
	}
	for index := range result.Ranking {
		totals := yoy[result.Ranking[index].District]
		if totals.CurrentCount >= minYoYSales && totals.PreviousCount >= minYoYSales && totals.CurrentArea > 0 && totals.PreviousArea > 0 {
			result.Ranking[index].PriceYoY = change(float64(totals.CurrentBuildingPrice)/totals.CurrentArea, float64(totals.PreviousBuildingPrice)/totals.PreviousArea)
		}
	}
	return result, nil
}

// DistrictBoundaries returns a city's district areas derived from door-number points.
func (service *TransactionService) DistrictBoundaries(ctx context.Context, cityCode string) ([]domain.DistrictBoundary, error) {
	if _, ok := addresspoint.Sources[cityCode]; !ok {
		return nil, fmt.Errorf("%w: unknown city %q", ErrInvalidInput, cityCode)
	}
	return service.repository.DistrictBoundaries(ctx, cityCode)
}

func change(current, previous float64) *float64 {
	if previous == 0 {
		return nil
	}
	value := current/previous - 1
	return &value
}

// monthSpan counts calendar months from from to to, inclusive.
func monthSpan(from, to time.Time) int {
	return (to.Year()-from.Year())*12 + int(to.Month()-from.Month()) + 1
}

func (service *TransactionService) Ready(ctx context.Context) error {
	return service.repository.Ping(ctx)
}

func validateFilter(filter domain.TransactionFilter) error {
	if filter.MinPrice < 0 || filter.MaxPrice < filter.MinPrice {
		return fmt.Errorf("%w: invalid price range", ErrInvalidInput)
	}
	if filter.MinArea < 0 || filter.MaxArea < filter.MinArea {
		return fmt.Errorf("%w: invalid area range", ErrInvalidInput)
	}
	if filter.Bounds != nil && (filter.Bounds.West >= filter.Bounds.East || filter.Bounds.South >= filter.Bounds.North) {
		return fmt.Errorf("%w: invalid bounding box", ErrInvalidInput)
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return fmt.Errorf("%w: invalid date range", ErrInvalidInput)
	}
	for _, category := range filter.Categories {
		if !slices.Contains(domain.BuildingCategories, category) {
			return fmt.Errorf("%w: unknown building type %q", ErrInvalidInput, category)
		}
	}
	return nil
}

func MonthRange(now time.Time, months int) (time.Time, time.Time) {
	end := time.Date(now.UTC().Year(), now.UTC().Month()+1, 1, 0, 0, 0, 0, time.UTC)
	return end.AddDate(0, -months, 0), end.Add(-time.Nanosecond)
}
