package usecase

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

const (
	// minimumRentalSample is the fewest leases, and sales, behind a median that is shown.
	minimumRentalSample = 20
	// minimumTrendSample is the fewest leases behind a point of a rent trend, which
	// pools three months.
	minimumTrendSample = 10
	// rentTrendMonths is how far back rent trends go.
	rentTrendMonths = 60
)

// socialHousingKnownFrom is the first month whose three pooled months of leases all carry
// 租賃住宅服務 (from season 112S4). Before it, social housing is only recognised from
// remarks: about 43% of New Taipei leases are flagged against 78% after, so trends that
// exclude social housing would jump about 20% at the change. They start here instead.
var socialHousingKnownFrom = time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC)

type RentalRepository interface {
	ImportedSalesByCity(context.Context, string) (seasons []string, releases []string, err error)
	ImportedRentalsByCity(context.Context, string) (seasons []string, releases []string, err error)
	RentalAggregates(context.Context, domain.RentalQuery, time.Time, time.Time) ([]domain.RentalAggregate, error)
	RentTrend(ctx context.Context, query domain.RentalQuery, district string, first, last time.Time) ([]domain.RentPoint, error)
	StationDistrict(ctx context.Context, stationID string) (city, district string, found bool, err error)
	StationRentRings(ctx context.Context, stationID, district string, query domain.RentalQuery, from, to time.Time) ([]domain.RentalAggregate, *float64, error)
	StationRentTrends(ctx context.Context, stationID string, query domain.RentalQuery, first, last time.Time) ([][]domain.RentPoint, error)
	MappedLeases(ctx context.Context, filter domain.TransactionFilter, from, to time.Time, emit func(domain.MappedLease) error) error
	MappedSales(ctx context.Context, filter domain.TransactionFilter, from, to time.Time, emit func(domain.MappedSale) error) error
}

type RentalService struct {
	repository RentalRepository
	now        func() time.Time

	indexesMu sync.Mutex
	indexes   map[string]cachedRentIndex
}

// cachedRentIndex keeps a city's rent index for the rent layer: it takes about 0.3 s per
// city, each map move asks for it, and it changes only after imports.
type cachedRentIndex struct {
	at      time.Time
	byMonth map[string]float64
}

// rentIndexTTL bounds how long an import takes to show in the rent layer's adjustment.
const rentIndexTTL = 10 * time.Minute

func NewRentalService(repository RentalRepository) *RentalService {
	return &RentalService{repository: repository, now: time.Now, indexes: map[string]cachedRentIndex{}}
}

// CompareDistricts reports each district's rent and gross yield over the last 12 months
// whose sales and leases are both completely registered. Yields compare medians of
// different homes, so they describe a district, not any one property.
func (service *RentalService) CompareDistricts(ctx context.Context, query domain.RentalQuery) (domain.RentalReport, error) {
	if err := validateRentalQuery(query); err != nil {
		return domain.RentalReport{}, err
	}
	report := domain.RentalReport{
		CityCode: query.CityCode, MinimumSample: minimumRentalSample, IncludeSocialHousing: query.IncludeSocialHousing,
		City: domain.RentalDistrict{}, Districts: []domain.RentalDistrict{},
	}
	window, known, err := service.window(ctx, query.CityCode)
	if err != nil || !known {
		return report, err
	}
	report.From, report.Through = window.from.Format("2006-01"), window.through.Format("2006-01")

	rows, err := service.repository.RentalAggregates(ctx, query, window.from, window.through.AddDate(0, 1, 0))
	if err != nil {
		return domain.RentalReport{}, err
	}
	for _, row := range rows {
		district := rentalDistrict(row)
		if row.District == "" {
			report.City = district
		} else {
			report.Districts = append(report.Districts, district)
		}
	}
	sort.Slice(report.Districts, func(i, j int) bool {
		if report.Districts[i].Rentals != report.Districts[j].Rentals {
			return report.Districts[i].Rentals > report.Districts[j].Rentals
		}
		return report.Districts[i].District < report.Districts[j].District
	})
	return report, nil
}

// Trend reports the city's monthly rent per ping over five years and, when district is
// not empty, the district's. Each point pools three months of leases.
func (service *RentalService) Trend(ctx context.Context, query domain.RentalQuery, district string) (domain.RentTrend, error) {
	if err := validateRentalQuery(query); err != nil {
		return domain.RentTrend{}, err
	}
	trend := domain.RentTrend{CityCode: query.CityCode, District: district, MinimumSample: minimumTrendSample, City: []domain.RentPoint{}, Districts: []domain.RentPoint{}}
	window, known, err := service.window(ctx, query.CityCode)
	if err != nil || !known {
		return trend, err
	}
	trend.Through = window.through.Format("2006-01")
	window = window.forSocialHousing(query.IncludeSocialHousing)
	if trend.City, err = service.repository.RentTrend(ctx, query, "", window.trendFirst, window.trendLast); err != nil {
		return domain.RentTrend{}, err
	}
	if district != "" {
		if trend.Districts, err = service.repository.RentTrend(ctx, query, district, window.trendFirst, window.trendLast); err != nil {
			return domain.RentTrend{}, err
		}
	}
	window.finish(trend.City)
	window.finish(trend.Districts)
	return trend, nil
}

// StationRent reports rents and gross yields within each ring of a station, for all
// residential homes, compared with the station's district.
func (service *RentalService) StationRent(ctx context.Context, stationID string, includeSocialHousing bool) (domain.StationRent, error) {
	city, district, found, err := service.repository.StationDistrict(ctx, stationID)
	if err != nil {
		return domain.StationRent{}, err
	}
	if !found {
		return domain.StationRent{}, fmt.Errorf("%w: station %q", ErrNotFound, stationID)
	}
	rent := domain.StationRent{StationID: stationID, CityCode: city, District: district, MinimumSample: minimumRentalSample, Rings: []domain.StationRentRing{}}
	window, known, err := service.window(ctx, city)
	if err != nil || !known {
		return rent, err
	}
	rent.From, rent.Through = window.from.Format("2006-01"), window.through.Format("2006-01")
	query := domain.RentalQuery{CityCode: city, IncludeSocialHousing: includeSocialHousing}
	rows, districtMedian, err := service.repository.StationRentRings(ctx, stationID, district, query, window.from, window.through.AddDate(0, 1, 0))
	if err != nil {
		return domain.StationRent{}, err
	}
	trendWindow := window.forSocialHousing(includeSocialHousing)
	trends, err := service.repository.StationRentTrends(ctx, stationID, query, trendWindow.trendFirst, trendWindow.trendLast)
	if err != nil {
		return domain.StationRent{}, err
	}
	rent.DistrictMedianRentPerPing = districtMedian
	for index, row := range rows {
		figures := rentalDistrict(row)
		ring := domain.StationRentRing{
			Radius: domain.StationRings[index], Rentals: figures.Rentals, MedianRentPerPing: figures.MedianRentPerPing,
			MedianMonthlyRent: figures.MedianMonthlyRent, Sales: figures.Sales, MedianPricePerPing: figures.MedianPricePerPing,
			GrossYield: figures.GrossYield, Trend: []domain.RentPoint{},
		}
		if ring.MedianRentPerPing != nil && districtMedian != nil {
			ring.RelativeToDistrict = change(*ring.MedianRentPerPing, *districtMedian)
		}
		if index < len(trends) && trends[index] != nil {
			ring.Trend = trends[index]
			window.finish(ring.Trend)
		}
		rent.Rings = append(rent.Rings, ring)
	}
	return rent, nil
}

// rentalWindow holds the months rent figures cover. from–through are the last 12 months
// complete for both sales and leases; trends run to the last month with registered leases,
// those after through being provisional.
type rentalWindow struct {
	from, through, trendFirst, trendLast time.Time
}

// forSocialHousing starts trends that exclude social housing at socialHousingKnownFrom.
func (window rentalWindow) forSocialHousing(included bool) rentalWindow {
	if !included && window.trendFirst.Before(socialHousingKnownFrom) {
		window.trendFirst = socialHousingKnownFrom
	}
	return window
}

// finish marks provisional points and hides medians from too few leases.
func (window rentalWindow) finish(points []domain.RentPoint) {
	through := window.through.Format("2006-01")
	for index := range points {
		points[index].Provisional = points[index].Month > through
		if points[index].Rentals < minimumTrendSample {
			points[index].MedianRentPerPing = nil
		}
	}
}

func (service *RentalService) window(ctx context.Context, city string) (rentalWindow, bool, error) {
	saleSeasons, saleReleases, err := service.repository.ImportedSalesByCity(ctx, city)
	if err != nil {
		return rentalWindow{}, false, err
	}
	rentalSeasons, rentalReleases, err := service.repository.ImportedRentalsByCity(ctx, city)
	if err != nil {
		return rentalWindow{}, false, err
	}
	salesThrough, salesKnown := registeredThrough(saleSeasons, saleReleases)
	rentalsThrough, rentalsKnown := registeredThrough(rentalSeasons, rentalReleases)
	if !salesKnown || !rentalsKnown {
		return rentalWindow{}, false, nil
	}
	registered := salesThrough
	if rentalsThrough.Before(registered) {
		registered = rentalsThrough
	}
	window := rentalWindow{through: completeThrough(registered)}
	window.from = window.through.AddDate(0, -11, 0)
	window.trendLast = time.Date(rentalsThrough.Year(), rentalsThrough.Month(), 1, 0, 0, 0, 0, time.UTC)
	window.trendFirst = window.trendLast.AddDate(0, -(rentTrendMonths - 1), 0)
	return window, true, nil
}

func validateRentalQuery(query domain.RentalQuery) error {
	if _, ok := addresspoint.Sources[query.CityCode]; !ok {
		return fmt.Errorf("%w: unknown city %q", ErrInvalidInput, query.CityCode)
	}
	if query.Category != "" && !slices.Contains(domain.BuildingCategories[:len(domain.BuildingCategories)-1], query.Category) {
		return fmt.Errorf("%w: choose one residential building type", ErrInvalidInput)
	}
	if query.MinArea < 0 || query.MaxArea < 0 || (query.MaxArea > 0 && query.MinArea >= query.MaxArea) || query.MaxArea > 300 {
		return fmt.Errorf("%w: invalid area range", ErrInvalidInput)
	}
	return nil
}

// rentalDistrict hides medians drawn from fewer than minimumRentalSample homes.
func rentalDistrict(row domain.RentalAggregate) domain.RentalDistrict {
	district := domain.RentalDistrict{District: row.District, Rentals: row.Rentals, Sales: row.Sales}
	if row.Rentals >= minimumRentalSample {
		district.MedianRentPerPing, district.MedianMonthlyRent = row.MedianRentPerPing, row.MedianMonthlyRent
	}
	if row.Sales >= minimumRentalSample {
		district.MedianPricePerPing = row.MedianPricePerPing
	}
	if district.MedianRentPerPing != nil && district.MedianPricePerPing != nil && *district.MedianPricePerPing > 0 {
		yield := *district.MedianRentPerPing * 12 / *district.MedianPricePerPing
		district.GrossYield = &yield
	}
	return district
}
