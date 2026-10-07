package usecase

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

// The rent estimate's parameters were chosen by backtesting 2,500 whole-home leases of
// 2026-01 to 2026-06 against the leases recorded before each: the weighted median of
// comparable leases had a median absolute error of 9.8% (towers 8.4%, mid-rises 11.2%,
// walk-ups 15.7%), against 17.8% for the median within 3 km. Facilities and stations
// were also tried as weights for prices and did not lower the error.
const (
	rentRadiusM      = 1500.0
	rentDistanceM    = 500.0
	minimumLeases    = 5
	maxLeasesShown   = 20
	minimumRentIndex = 30
	// Same-building leases need to share less: two suffice for a high confidence.
	highConfidenceLeases = 2
	mediumConfidenceRent = 15
)

var rentConfidenceBands = map[string]confidenceBand{
	"high":   {low: 0.897, high: 1.106, medianError: 0.050}, // 16% of estimates
	"medium": {low: 0.805, high: 1.228, medianError: 0.095}, // 39%
	"low":    {low: 0.738, high: 1.369, medianError: 0.141}, // 45%
}

// rentEstimate values a home's monthly rent from whole-home leases nearby, weighting them
// like sales (with distance falling by e every 500 m, since leases are sparser) and
// adjusting them with the city's rent index. Houses (透天厝) have too few leases.
func (service *EstimateService) rentEstimate(ctx context.Context, query domain.EstimateQuery, estimate domain.Estimate) (*domain.RentEstimate, error) {
	rent := &domain.RentEstimate{Supported: query.Category != "透天厝", MinimumSample: minimumLeases, Leases: []domain.ComparableLease{}}
	if !rent.Supported {
		return rent, nil
	}
	seasons, releases, err := service.repository.ImportedRentalsByCity(ctx, estimate.CityCode)
	if err != nil {
		return nil, err
	}
	registered, known := registeredThrough(seasons, releases)
	if !known {
		return rent, nil
	}
	asOf := completeThrough(registered)
	rent.AsOf = asOf.Format("2006-01")
	now := service.now().UTC()
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	leases, err := service.repository.RentCandidates(ctx, query, rentRadiusM, query.AreaPing*areaRatioLimit, query.AreaPing/areaRatioLimit,
		thisMonth.AddDate(0, -estimateMonths-3, 0), thisMonth.AddDate(0, 1, 0))
	if err != nil {
		return nil, err
	}
	index, err := service.repository.RentTrend(ctx, domain.RentalQuery{CityCode: estimate.CityCode}, "", asOf.AddDate(0, -estimateMonths-3, 0), thisMonth)
	if err != nil {
		return nil, err
	}
	indexByMonth := map[string]float64{}
	for _, point := range index {
		if point.MedianRentPerPing != nil && point.Rentals >= minimumRentIndex {
			indexByMonth[point.Month] = *point.MedianRentPerPing
		}
	}

	key := building(query.Address)
	var comparables []domain.ComparableLease
	var values, weights []float64
	for _, lease := range leases {
		month := time.Date(lease.LeasedAt.Year(), lease.LeasedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
		monthsAgo := float64(monthsBetween(month, asOf))
		if monthsAgo > estimateMonths {
			continue
		}
		ageGap := 0.0
		if query.Age != nil && lease.Age != nil {
			ageGap = math.Abs(float64(*query.Age - *lease.Age))
			if ageGap > maxAgeGap {
				continue
			}
		}
		adjusted := lease.RentPerPing
		if then, ok := indexByMonth[month.Format("2006-01")]; ok {
			if target, ok := indexByMonth[asOf.Format("2006-01")]; ok {
				adjusted *= target / then
			}
		}
		same := key != "" && building(lease.Address) == key
		weight := math.Exp(-lease.DistanceM/rentDistanceM) * math.Exp(-math.Abs(math.Log(lease.AreaPing/query.AreaPing))/areaScale) *
			math.Exp(-math.Max(monthsAgo, 0)/monthScale) * math.Exp(-ageGap/ageScale)
		if same {
			weight *= sameBuildingGain
			rent.SameBuildingLeases++
		}
		values = append(values, adjusted)
		weights = append(weights, weight)
		comparables = append(comparables, domain.ComparableLease{
			ID: lease.ID, Address: lease.Address, Lat: lease.Lat, Lng: lease.Lng, LeasedAt: lease.LeasedAt.Format("2006-01-02"),
			AreaPing: lease.AreaPing, Age: lease.Age, MonthlyRent: lease.MonthlyRent, RentPerPing: lease.RentPerPing,
			AdjustedRentPerPing: math.Round(adjusted), DistanceM: math.Round(lease.DistanceM), SameBuilding: same, Weight: weight,
		})
	}
	rent.LeaseCount = len(comparables)
	if len(comparables) < minimumLeases {
		if comparables != nil {
			rent.Leases = comparables
		}
		return rent, nil
	}

	total := 0.0
	for _, weight := range weights {
		total += weight
	}
	for index := range comparables {
		comparables[index].Weight /= total
	}
	sort.SliceStable(comparables, func(i, j int) bool { return comparables[i].Weight > comparables[j].Weight })
	rent.Leases = comparables[:min(len(comparables), maxLeasesShown)]

	median := weightedQuantile(values, weights, 0.5)
	spread := (weightedQuantile(values, weights, 0.75) - weightedQuantile(values, weights, 0.25)) / median
	rent.Sufficient = true
	rent.Spread = math.Round(spread*1000) / 1000
	switch {
	case rent.SameBuildingLeases >= highConfidenceLeases && spread < 0.15:
		rent.Confidence = "high"
	case len(comparables) >= mediumConfidenceRent && spread < 0.25:
		rent.Confidence = "medium"
	default:
		rent.Confidence = "low"
	}
	band := rentConfidenceBands[rent.Confidence]
	rent.BacktestMedianError, rent.RangeLow, rent.RangeHigh = band.medianError, band.low, band.high
	perPing := math.Round(median)
	monthly := math.Round(perPing*query.AreaPing/100) * 100
	rent.RentPerPing = &perPing
	rent.MonthlyRent = &monthly
	rent.LowMonthlyRent = new(math.Round(monthly*band.low/100) * 100)
	rent.HighMonthlyRent = new(math.Round(monthly*band.high/100) * 100)
	if estimate.BuildingPrice != nil && *estimate.BuildingPrice > 0 {
		rent.GrossYield = new(monthly * 12 / *estimate.BuildingPrice)
	}

	// The district's yield over the last 12 months complete for sales and leases, as on
	// the rents page.
	salesAsOf, _ := time.Parse("2006-01", estimate.AsOf)
	through := asOf
	if salesAsOf.Before(through) {
		through = salesAsOf
	}
	rows, err := service.repository.RentalAggregates(ctx, domain.RentalQuery{CityCode: estimate.CityCode, Category: query.Category},
		through.AddDate(0, -11, 0), through.AddDate(0, 1, 0))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.District == estimate.District {
			rent.DistrictGrossYield = rentalDistrict(row).GrossYield
		}
	}
	return rent, nil
}
