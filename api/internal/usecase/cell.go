package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/geocode"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
	"github.com/uber/h3-go/v4"
)

var ageBuckets = []struct {
	key      string
	from, to int // years, to exclusive; to 0 means no upper bound
}{{"0-5", 0, 5}, {"5-15", 5, 15}, {"15-30", 15, 30}, {"30+", 30, 0}}

// ageMixKeys orders the card's age buckets, with unknown (e.g. presale) last.
var ageMixKeys = []string{"0-5", "5-15", "15-30", "30+", "unknown"}

// CellDetail describes the sales in one H3 cell. Rows are fetched by the cell's bounding
// box and kept only when they fall in the cell, using the same h3 function as AggregateH3
// so the card always matches the hexagon on the map.
func (service *TransactionService) CellDetail(ctx context.Context, filter domain.TransactionFilter, hex string) (domain.CellDetail, error) {
	if err := validateFilter(filter); err != nil {
		return domain.CellDetail{}, err
	}
	cell := h3.CellFromString(hex)
	if !cell.IsValid() || cell.Resolution() < 7 || cell.Resolution() > 10 {
		return domain.CellDetail{}, fmt.Errorf("%w: invalid hex %q", ErrInvalidInput, hex)
	}
	if filter.From == nil || filter.To == nil {
		return domain.CellDetail{}, fmt.Errorf("%w: a date window is required", ErrInvalidInput)
	}
	boundary, err := cell.Boundary()
	if err != nil {
		return domain.CellDetail{}, fmt.Errorf("hex boundary: %w", err)
	}
	bounds := domain.Bounds{West: 180, South: 90, East: -180, North: -90}
	for _, vertex := range boundary {
		bounds.West, bounds.East = min(bounds.West, vertex.Lng), max(bounds.East, vertex.Lng)
		bounds.South, bounds.North = min(bounds.South, vertex.Lat), max(bounds.North, vertex.Lat)
	}

	// The YoY windows end at the last complete month, like the summary's.
	complete, known, err := service.completeMonth(ctx)
	if err != nil {
		return domain.CellDetail{}, err
	}
	end := time.Date(filter.To.Year(), filter.To.Month(), 1, 0, 0, 0, 0, time.UTC)
	if known && end.After(complete) {
		end = complete
	}
	currentFrom, currentTo := end.AddDate(0, -11, 0), end.AddDate(0, 1, 0).Add(-time.Nanosecond)
	previousFrom := currentFrom.AddDate(-1, 0, 0)

	wide := filter
	wide.Bounds, wide.PreciseLocation, wide.Limit = &bounds, true, 200_000
	from, to := *filter.From, *filter.To
	if previousFrom.Before(from) {
		wide.From = &previousFrom
	}
	if currentTo.After(to) {
		wide.To = &currentTo
	}
	rows, err := service.repository.CellRows(ctx, wide)
	if err != nil {
		return domain.CellDetail{}, err
	}

	detail := domain.CellDetail{Hex: cell.String(), YoYThrough: end.Format("2006-01"), AgeMix: []domain.AgeBucket{}}
	var totalPrice, buildingPrice int64
	var area float64
	districts, roads, ages := map[string]int{}, map[string]int{}, map[string]int{}
	var current, previous struct {
		count int
		price int64
		area  float64
	}
	for _, row := range rows {
		inCell, cellErr := h3.LatLngToCell(h3.NewLatLng(row.Lat, row.Lng), cell.Resolution())
		if cellErr != nil || inCell != cell {
			continue
		}
		switch {
		case !row.TransactedAt.Before(currentFrom) && !row.TransactedAt.After(currentTo):
			current.count, current.price, current.area = current.count+1, current.price+row.BuildingPrice, current.area+row.AreaPing
		case !row.TransactedAt.Before(previousFrom) && row.TransactedAt.Before(currentFrom):
			previous.count, previous.price, previous.area = previous.count+1, previous.price+row.BuildingPrice, previous.area+row.AreaPing
		}
		if row.TransactedAt.Before(from) || row.TransactedAt.After(to) {
			continue // outside the map's window: only used for YoY
		}
		detail.Count++
		totalPrice += row.TotalPrice
		buildingPrice += row.BuildingPrice
		area += row.AreaPing
		districts[row.CityCode+"|"+row.District]++
		if address, ok := geocode.ParseAddress(row.Address, lvr.Cities[row.CityCode], row.District); ok && address.Road != "" {
			roads[address.Road]++
		}
		ages[ageKey(row.BuildingAge)]++
	}

	if detail.Count > 0 {
		detail.AvgPricePerPing = float64(buildingPrice) / area
		detail.AvgTotalPrice = float64(totalPrice) / float64(detail.Count)
		detail.AvgArea = area / float64(detail.Count)
		if key := mostCommon(districts); key != "" {
			detail.CityCode, detail.District = key[:1], key[2:]
		}
		detail.Road = mostCommon(roads)
		for _, key := range ageMixKeys {
			detail.AgeMix = append(detail.AgeMix, domain.AgeBucket{Key: key, Count: ages[key]})
		}
	}
	if current.count >= minYoYSales && previous.count >= minYoYSales && current.area > 0 && previous.area > 0 {
		detail.PriceYoY = change(float64(current.price)/current.area, float64(previous.price)/previous.area)
	}
	return detail, nil
}

func ageKey(age *int) string {
	if age == nil {
		return "unknown"
	}
	for _, bucket := range ageBuckets {
		if *age >= bucket.from && (bucket.to == 0 || *age < bucket.to) {
			return bucket.key
		}
	}
	return "unknown"
}

// mostCommon returns the key with the highest count, breaking ties alphabetically.
func mostCommon(counts map[string]int) string {
	best, bestCount := "", 0
	for key, count := range counts {
		if count > bestCount || (count == bestCount && key < best) {
			best, bestCount = key, count
		}
	}
	return best
}
