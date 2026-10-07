package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"golang.org/x/sync/errgroup"
)

var ErrNotFound = errors.New("not found")

type TransitRepository interface {
	TransitLines(ctx context.Context) ([]domain.TransitLine, error)
	// TransitStations lists stations, of one city when cityCode is not empty.
	TransitStations(ctx context.Context, cityCode string) ([]domain.TransitStation, error)
	TransitStation(ctx context.Context, id string) (domain.TransitStation, bool, error)
	// StationRings returns each ring's figures and monthly trend, without YoY.
	StationRings(ctx context.Context, stationID string, filter domain.TransactionFilter, radii []int) ([]domain.StationRing, error)
	// StationRingYoY sums each ring's sales in the previous and current 12-month windows.
	StationRingYoY(ctx context.Context, stationID string, filter domain.TransactionFilter, radii []int, previousFrom, currentFrom, currentTo time.Time) (map[int]domain.DistrictYoYTotals, error)
	// StationYoYTotals sums every station's sales within radius in both 12-month windows.
	StationYoYTotals(ctx context.Context, filter domain.TransactionFilter, radius int, previousFrom, currentFrom, currentTo time.Time) (map[string]domain.DistrictYoYTotals, error)
	// StationRankings returns every station's figures and monthly trend within radius.
	StationRankings(ctx context.Context, filter domain.TransactionFilter, radius int) ([]domain.StationRank, error)
	OpeningEvents(ctx context.Context) ([]domain.OpeningEvent, error)
	// DistanceCurve groups located sales by city and distance to the nearest station.
	DistanceCurve(ctx context.Context, filter domain.TransactionFilter) ([]domain.DistanceBucket, error)
	// OpeningSums returns monthly sums around each event's opening, keyed by event index
	// and YYYY-MM, for the station's radius and for its district baseline.
	OpeningSums(ctx context.Context, filter domain.TransactionFilter, radius int, events []domain.OpeningEvent, months int) (station, baseline map[int]map[string]domain.MonthlySums, err error)
}

// TransitService serves rail stations and prices around them. It shares the
// transaction service's registration coverage for YoY and provisional months.
type TransitService struct {
	repository   TransitRepository
	transactions *TransactionService
	now          func() time.Time

	openingsMu sync.Mutex
	openings   map[string]cachedOpenings
}

// cachedOpenings keeps opening comparisons, which scan six years around every opening and
// change only after imports.
type cachedOpenings struct {
	at     time.Time
	effect domain.OpeningEffects
}

const (
	openingsTTL       = 10 * time.Minute
	maxCachedOpenings = 32
)

func NewTransitService(repository TransitRepository, transactions *TransactionService) *TransitService {
	return &TransitService{repository: repository, transactions: transactions, now: time.Now, openings: map[string]cachedOpenings{}}
}

func (service *TransitService) Lines(ctx context.Context) ([]domain.TransitLine, error) {
	return service.repository.TransitLines(ctx)
}

func (service *TransitService) Stations(ctx context.Context, cityCode string) ([]domain.TransitStation, error) {
	if _, ok := addresspoint.Sources[cityCode]; cityCode != "" && !ok {
		return nil, fmt.Errorf("%w: unknown city %q", ErrInvalidInput, cityCode)
	}
	return service.repository.TransitStations(ctx, cityCode)
}

// StationTrend compares sales within 300, 500 and 800 m of a station: figures for the
// filter's window, a monthly trend over all years, and the change over the last 12
// complete months. Only sales located at a door number are measured, so bbox is ignored.
func (service *TransitService) StationTrend(ctx context.Context, stationID string, filter domain.TransactionFilter) (domain.StationTrend, error) {
	if err := validateFilter(filter); err != nil {
		return domain.StationTrend{}, err
	}
	filter.Bounds = nil
	station, found, err := service.repository.TransitStation(ctx, stationID)
	if err != nil {
		return domain.StationTrend{}, err
	}
	if !found {
		return domain.StationTrend{}, fmt.Errorf("%w: station %q", ErrNotFound, stationID)
	}
	complete, known, err := service.transactions.completeMonth(ctx)
	if err != nil {
		return domain.StationTrend{}, err
	}

	result := domain.StationTrend{Station: station}
	var yoy map[int]domain.DistrictYoYTotals
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		result.Rings, err = service.repository.StationRings(groupCtx, stationID, filter, domain.StationRings)
		return err
	})
	if known {
		currentFrom := complete.AddDate(0, -11, 0)
		currentTo := complete.AddDate(0, 1, 0).Add(-time.Nanosecond)
		group.Go(func() (err error) {
			yoy, err = service.repository.StationRingYoY(groupCtx, stationID, filter, domain.StationRings, currentFrom.AddDate(-1, 0, 0), currentFrom, currentTo)
			return err
		})
	}
	if err = group.Wait(); err != nil {
		return domain.StationTrend{}, err
	}
	if !known {
		return result, nil
	}
	result.YoYThrough = complete.Format("2006-01")
	for index := range result.Rings {
		ring := &result.Rings[index]
		for month := range ring.Trends {
			ring.Trends[month].Provisional = ring.Trends[month].Month > result.YoYThrough
		}
		ring.PriceYoY = totalsChange(yoy[ring.Radius])
	}
	return result, nil
}

// StationYoYs returns every station's change within radius over the last 12 complete
// months, using the filter's building types, price, area and special-sale settings.
func (service *TransitService) StationYoYs(ctx context.Context, filter domain.TransactionFilter, radius int) (domain.StationYoYs, error) {
	if !slices.Contains(domain.StationRings, radius) {
		return domain.StationYoYs{}, fmt.Errorf("%w: radius must be one of %v", ErrInvalidInput, domain.StationRings)
	}
	if err := validateFilter(filter); err != nil {
		return domain.StationYoYs{}, err
	}
	filter.Bounds = nil
	result := domain.StationYoYs{Radius: radius, Stations: []domain.StationYoY{}}
	complete, known, err := service.transactions.completeMonth(ctx)
	if err != nil || !known {
		return result, err
	}
	currentFrom := complete.AddDate(0, -11, 0)
	totals, err := service.repository.StationYoYTotals(ctx, filter, radius, currentFrom.AddDate(-1, 0, 0), currentFrom, complete.AddDate(0, 1, 0).Add(-time.Nanosecond))
	if err != nil {
		return domain.StationYoYs{}, err
	}
	result.YoYThrough = complete.Format("2006-01")
	for stationID, total := range totals {
		result.Stations = append(result.Stations, domain.StationYoY{StationID: stationID, Count: total.CurrentCount, PriceYoY: totalsChange(total)})
	}
	slices.SortFunc(result.Stations, func(a, b domain.StationYoY) int { return strings.Compare(a.StationID, b.StationID) })
	return result, nil
}

// totalsChange compares area-weighted prices, or returns nil when either window has too
// few sales.
func totalsChange(totals domain.DistrictYoYTotals) *float64 {
	if totals.CurrentCount < minYoYSales || totals.PreviousCount < minYoYSales || totals.CurrentArea <= 0 || totals.PreviousArea <= 0 {
		return nil
	}
	return change(float64(totals.CurrentBuildingPrice)/totals.CurrentArea, float64(totals.PreviousBuildingPrice)/totals.PreviousArea)
}

// StationRanking returns every station's figures within radius for the filter's window,
// with YoY over the last 12 complete months; the page filters and sorts them.
func (service *TransitService) StationRanking(ctx context.Context, filter domain.TransactionFilter, radius int) (domain.StationRanking, error) {
	if !slices.Contains(domain.StationRings, radius) {
		return domain.StationRanking{}, fmt.Errorf("%w: radius must be one of %v", ErrInvalidInput, domain.StationRings)
	}
	if err := validateFilter(filter); err != nil {
		return domain.StationRanking{}, err
	}
	filter.Bounds = nil
	complete, known, err := service.transactions.completeMonth(ctx)
	if err != nil {
		return domain.StationRanking{}, err
	}
	result := domain.StationRanking{Radius: radius, Stations: []domain.StationRank{}}
	var totals map[string]domain.DistrictYoYTotals
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		result.Stations, err = service.repository.StationRankings(groupCtx, filter, radius)
		return err
	})
	if known {
		currentFrom := complete.AddDate(0, -11, 0)
		group.Go(func() (err error) {
			totals, err = service.repository.StationYoYTotals(groupCtx, filter, radius, currentFrom.AddDate(-1, 0, 0), currentFrom, complete.AddDate(0, 1, 0).Add(-time.Nanosecond))
			return err
		})
	}
	if err = group.Wait(); err != nil {
		return domain.StationRanking{}, err
	}
	if known {
		result.YoYThrough = complete.Format("2006-01")
	}
	for index := range result.Stations {
		rank := &result.Stations[index]
		if known {
			rank.PriceYoY = totalsChange(totals[rank.StationID])
			for month := range rank.Trend {
				rank.Trend[month].Provisional = rank.Trend[month].Month > result.YoYThrough
			}
		}
	}
	slices.SortFunc(result.Stations, func(a, b domain.StationRank) int { return strings.Compare(a.StationID, b.StationID) })
	return result, nil
}

const (
	// openingWindow is the months compared on each side of an opening.
	openingWindow = 12
	// openingSeries is the months charted on each side of an opening.
	openingSeries = 36
	// minOpeningSales is the fewest sales a window needs for a comparison.
	minOpeningSales = 10
)

// OpeningEffects compares each line opening since 2015 with its district: the change in
// area-weighted price per ping within radius between the 12 months before the opening
// month and the 12 months from it, minus the same change for district sales farther than
// 800 m from the station. Date filters are ignored; other filters apply.
func (service *TransitService) OpeningEffects(ctx context.Context, filter domain.TransactionFilter, radius int) (domain.OpeningEffects, error) {
	if !slices.Contains(domain.StationRings, radius) {
		return domain.OpeningEffects{}, fmt.Errorf("%w: radius must be one of %v", ErrInvalidInput, domain.StationRings)
	}
	if err := validateFilter(filter); err != nil {
		return domain.OpeningEffects{}, err
	}
	filter.Bounds, filter.From, filter.To = nil, nil, nil
	key := fmt.Sprintf("%d|%d|%d|%g|%g|%v|%t|%t", radius, filter.MinPrice, filter.MaxPrice, filter.MinArea, filter.MaxArea, filter.Categories, filter.IncludeSpecial, filter.ExcludeNewBuildings)
	service.openingsMu.Lock()
	cached, ok := service.openings[key]
	service.openingsMu.Unlock()
	if ok && service.now().Sub(cached.at) < openingsTTL {
		return cached.effect, nil
	}
	result, err := service.openingEffects(ctx, filter, radius)
	if err != nil {
		return domain.OpeningEffects{}, err
	}
	service.openingsMu.Lock()
	defer service.openingsMu.Unlock()
	if len(service.openings) >= maxCachedOpenings {
		for candidate, entry := range service.openings {
			if service.now().Sub(entry.at) >= openingsTTL || len(service.openings) >= maxCachedOpenings {
				delete(service.openings, candidate)
			}
		}
	}
	service.openings[key] = cachedOpenings{at: service.now(), effect: result}
	return result, nil
}

func (service *TransitService) openingEffects(ctx context.Context, filter domain.TransactionFilter, radius int) (domain.OpeningEffects, error) {
	result := domain.OpeningEffects{Radius: radius, Events: []domain.OpeningEffect{}}
	events, err := service.repository.OpeningEvents(ctx)
	if err != nil || len(events) == 0 {
		return result, err
	}
	complete, known, err := service.transactions.completeMonth(ctx)
	if err != nil {
		return domain.OpeningEffects{}, err
	}
	stationSums, baselineSums, err := service.repository.OpeningSums(ctx, filter, radius, events, openingSeries)
	if err != nil {
		return domain.OpeningEffects{}, err
	}
	for index, event := range events {
		opened, err := time.Parse(time.DateOnly, event.OpenedOn)
		if err != nil {
			return domain.OpeningEffects{}, fmt.Errorf("opening date of %s: %w", event.StationID, err)
		}
		openedMonth := time.Date(opened.Year(), opened.Month(), 1, 0, 0, 0, 0, time.UTC)
		effect := domain.OpeningEffect{
			OpeningEvent: event,
			Station:      openingWindows(stationSums[index], openedMonth),
			Baseline:     openingWindows(baselineSums[index], openedMonth),
			Series:       openingSeriesPoints(stationSums[index], baselineSums[index], openedMonth, complete, known),
			Status:       "ok",
		}
		// The year after opening must be fully registered to compare.
		lastAfter := openedMonth.AddDate(0, openingWindow-1, 0)
		switch {
		case !known || lastAfter.After(complete):
			effect.Status = "too_recent"
		case effect.Station.BeforeCount < minOpeningSales || effect.Station.AfterCount < minOpeningSales ||
			effect.Baseline.BeforeCount < minOpeningSales || effect.Baseline.AfterCount < minOpeningSales:
			effect.Status = "insufficient"
		case effect.Station.Change != nil && effect.Baseline.Change != nil:
			relative := *effect.Station.Change - *effect.Baseline.Change
			effect.Relative = &relative
		}
		result.Events = append(result.Events, effect)
	}
	return result, nil
}

// openingWindows sums the 12 months before the opening month and the 12 months from it.
func openingWindows(sums map[string]domain.MonthlySums, opened time.Time) domain.OpeningWindows {
	var before, after domain.MonthlySums
	for offset := -openingWindow; offset < openingWindow; offset++ {
		month := sums[opened.AddDate(0, offset, 0).Format("2006-01")]
		target := &after
		if offset < 0 {
			target = &before
		}
		target.Count += month.Count
		target.BuildingPrice += month.BuildingPrice
		target.Area += month.Area
	}
	windows := domain.OpeningWindows{BeforeCount: before.Count, AfterCount: after.Count, BeforePrice: price(before), AfterPrice: price(after)}
	if windows.BeforePrice != nil && windows.AfterPrice != nil {
		windows.Change = change(*windows.AfterPrice, *windows.BeforePrice)
	}
	return windows
}

// openingSeriesPoints lists every month within 36 months of the opening that is fully
// registered, so the chart never ends on a still-filling month.
func openingSeriesPoints(station, baseline map[string]domain.MonthlySums, opened, complete time.Time, known bool) []domain.OpeningPoint {
	points := []domain.OpeningPoint{}
	for offset := -openingSeries; offset < openingSeries; offset++ {
		month := opened.AddDate(0, offset, 0)
		if known && month.After(complete) {
			break
		}
		label := month.Format("2006-01")
		points = append(points, domain.OpeningPoint{
			Month: label, Offset: offset,
			StationPrice: price(station[label]), StationCount: station[label].Count,
			BaselinePrice: price(baseline[label]), BaselineCount: baseline[label].Count,
		})
	}
	return points
}

func price(sums domain.MonthlySums) *float64 {
	if sums.Area <= 0 {
		return nil
	}
	value := float64(sums.BuildingPrice) / sums.Area
	return &value
}

// DistanceCurve shows how price per ping changes with distance to the nearest station, by
// city, for the filter's window. Only sales located at a door number are measured, so bbox
// is ignored.
func (service *TransitService) DistanceCurve(ctx context.Context, filter domain.TransactionFilter) ([]domain.DistanceBucket, error) {
	if err := validateFilter(filter); err != nil {
		return nil, err
	}
	filter.Bounds = nil
	return service.repository.DistanceCurve(ctx, filter)
}

// InvalidateOpenings drops cached opening comparisons after opening dates change.
func (service *TransitService) InvalidateOpenings() {
	service.openingsMu.Lock()
	defer service.openingsMu.Unlock()
	clear(service.openings)
}
