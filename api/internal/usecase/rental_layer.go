package usecase

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/uber/h3-go/v4"
)

const (
	// rentLayerMonths of leases fill about as many cells as a year of sales: leases are
	// fewer, and rents change slowly enough to be adjusted with the city's rent index.
	rentLayerMonths = 36
	// rentLayerSaleMonths of sales give each cell's price for its gross yield.
	rentLayerSaleMonths = 12
	// minimumCellSample is the fewest leases a cell needs to be shown, and the fewest
	// sales behind its gross yield.
	minimumCellSample = 5
)

// Layer colours the map's H3 cells inside the filter's bounds by rent. It uses the
// filter's building types (residential ones only), area and new-building option but not
// its prices or dates: leases come from the last rentLayerMonths complete months, each
// adjusted to the last one by its city's rent index, and sales from the last
// rentLayerSaleMonths.
func (service *RentalService) Layer(ctx context.Context, filter domain.TransactionFilter, resolution int) (domain.RentLayer, error) {
	if resolution < 7 || resolution > 10 {
		return domain.RentLayer{}, fmt.Errorf("%w: resolution must be between 7 and 10", ErrInvalidInput)
	}
	if filter.Bounds == nil {
		return domain.RentLayer{}, fmt.Errorf("%w: bbox is required", ErrInvalidInput)
	}
	if err := validateFilter(filter); err != nil {
		return domain.RentLayer{}, err
	}
	layer := domain.RentLayer{MinimumSample: minimumCellSample, Cells: []domain.RentCell{}}
	if len(filter.Categories) > 0 {
		filter.Categories = slices.DeleteFunc(slices.Clone(filter.Categories), func(category string) bool {
			return category == domain.BuildingCategoryOther
		})
		if len(filter.Categories) == 0 {
			return layer, nil
		}
	}

	// Every city's leases and sales must be complete through the last month.
	var through time.Time
	for city := range addresspoint.Sources {
		window, known, err := service.window(ctx, city)
		if err != nil {
			return domain.RentLayer{}, err
		}
		if known && (through.IsZero() || window.through.Before(through)) {
			through = window.through
		}
	}
	if through.IsZero() {
		return layer, nil
	}
	from, salesFrom, end := through.AddDate(0, -(rentLayerMonths-1), 0), through.AddDate(0, -(rentLayerSaleMonths-1), 0), through.AddDate(0, 1, 0)
	layer.From, layer.SalesFrom, layer.Through = from.Format("2006-01"), salesFrom.Format("2006-01"), through.Format("2006-01")

	var leases []domain.MappedLease
	err := service.repository.MappedLeases(ctx, filter, from, end, func(lease domain.MappedLease) error {
		leases = append(leases, lease)
		return nil
	})
	if err != nil {
		return domain.RentLayer{}, err
	}
	adjust, err := service.rentIndexes(ctx, leases, from, through)
	if err != nil {
		return domain.RentLayer{}, err
	}

	type cell struct {
		rents, monthly, areas, prices []float64
		rooms                         map[int]int
		byCategory                    map[string]int
	}
	cells := map[h3.Cell]*cell{}
	for _, lease := range leases {
		key, err := h3.LatLngToCell(h3.NewLatLng(lease.Lat, lease.Lng), resolution)
		if err != nil {
			return domain.RentLayer{}, fmt.Errorf("create h3 cell: %w", err)
		}
		group := cells[key]
		if group == nil {
			group = &cell{rooms: map[int]int{}, byCategory: map[string]int{}}
			cells[key] = group
		}
		factor := adjust(lease)
		group.rents = append(group.rents, lease.RentPerPing*factor)
		group.monthly = append(group.monthly, float64(lease.MonthlyRent)*factor)
		group.areas = append(group.areas, lease.AreaPing)
		if lease.Rooms > 0 {
			group.rooms[lease.Rooms]++
		}
		group.byCategory[lease.Category]++
	}
	for key, group := range cells {
		if len(group.rents) < minimumCellSample {
			delete(cells, key)
		}
	}
	if len(cells) == 0 {
		return layer, nil
	}

	err = service.repository.MappedSales(ctx, filter, salesFrom, end, func(sale domain.MappedSale) error {
		key, err := h3.LatLngToCell(h3.NewLatLng(sale.Lat, sale.Lng), resolution)
		if err != nil {
			return fmt.Errorf("create h3 cell: %w", err)
		}
		if group := cells[key]; group != nil {
			group.prices = append(group.prices, sale.PricePerPing)
		}
		return nil
	})
	if err != nil {
		return domain.RentLayer{}, err
	}

	for key, group := range cells {
		center, err := key.LatLng()
		if err != nil {
			return domain.RentLayer{}, fmt.Errorf("find h3 center: %w", err)
		}
		result := domain.RentCell{
			Hex: key.String(), Lat: center.Lat, Lng: center.Lng, Rentals: len(group.rents),
			MedianRentPerPing: median(group.rents), MedianMonthlyRent: median(group.monthly), MedianAreaPing: median(group.areas),
			MainRooms: mostCommonRooms(group.rooms), Categories: shares(group.byCategory, len(group.rents)), Sales: len(group.prices),
		}
		if len(group.prices) >= minimumCellSample {
			price := median(group.prices)
			result.MedianPricePerPing = &price
			yield := result.MedianRentPerPing * 12 / price
			result.GrossYield = &yield
		}
		layer.Cells = append(layer.Cells, result)
	}
	slices.SortFunc(layer.Cells, func(a, b domain.RentCell) int { return cmp.Compare(a.Hex, b.Hex) })
	return layer, nil
}

// rentIndexes returns the factor that brings a lease's rent to through, from the rent
// index of its city: the median rent per ping of the three months to through over that
// of the three months to its own. Months with too few leases leave a rent as it is.
func (service *RentalService) rentIndexes(ctx context.Context, leases []domain.MappedLease, from, through time.Time) (func(domain.MappedLease) float64, error) {
	indexes := map[string]map[string]float64{}
	for _, lease := range leases {
		if _, done := indexes[lease.CityCode]; done {
			continue
		}
		byMonth, err := service.rentIndex(ctx, lease.CityCode, from, through)
		if err != nil {
			return nil, err
		}
		indexes[lease.CityCode] = byMonth
	}
	last := through.Format("2006-01")
	return func(lease domain.MappedLease) float64 {
		index := indexes[lease.CityCode]
		then, known := index[lease.LeasedAt.Format("2006-01")]
		target, targetKnown := index[last]
		if !known || !targetKnown {
			return 1
		}
		return target / then
	}, nil
}

// rentIndex returns a city's median rent per ping by month from from to through, leaving
// out months with too few leases, cached for rentIndexTTL.
func (service *RentalService) rentIndex(ctx context.Context, city string, from, through time.Time) (map[string]float64, error) {
	key := city + "|" + from.Format("2006-01") + "|" + through.Format("2006-01")
	service.indexesMu.Lock()
	cached, ok := service.indexes[key]
	service.indexesMu.Unlock()
	if ok && service.now().Sub(cached.at) < rentIndexTTL {
		return cached.byMonth, nil
	}
	points, err := service.repository.RentTrend(ctx, domain.RentalQuery{CityCode: city}, "", from, through)
	if err != nil {
		return nil, err
	}
	byMonth := map[string]float64{}
	for _, point := range points {
		if point.MedianRentPerPing != nil && point.Rentals >= minimumRentIndex {
			byMonth[point.Month] = *point.MedianRentPerPing
		}
	}
	service.indexesMu.Lock()
	defer service.indexesMu.Unlock()
	// Keys change only when a new month completes; drop the expired ones then.
	for candidate, entry := range service.indexes {
		if service.now().Sub(entry.at) >= rentIndexTTL {
			delete(service.indexes, candidate)
		}
	}
	service.indexes[key] = cachedRentIndex{at: service.now(), byMonth: byMonth}
	return byMonth, nil
}

// mostCommonRooms returns the most frequent number of rooms, the smaller one on a tie, or nil when empty.
func mostCommonRooms(counts map[int]int) *int {
	var best *int
	for key, count := range counts {
		if best == nil || count > counts[*best] || (count == counts[*best] && key < *best) {
			best = &key
		}
	}
	return best
}

// shares lists each category's share of total, largest first and in the order of
// domain.BuildingCategories on a tie.
func shares(counts map[string]int, total int) []domain.CategoryShare {
	result := make([]domain.CategoryShare, 0, len(counts))
	for _, category := range domain.BuildingCategories {
		if counts[category] > 0 {
			result = append(result, domain.CategoryShare{Category: category, Share: float64(counts[category]) / float64(total)})
		}
	}
	slices.SortStableFunc(result, func(a, b domain.CategoryShare) int { return cmp.Compare(b.Share, a.Share) })
	return result
}
