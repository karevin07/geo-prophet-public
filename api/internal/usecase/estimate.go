package usecase

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

// The estimate's parameters were chosen by backtesting 3,000 sales of 2026-01 to 2026-06
// against the sales recorded before each: the weighted median of comparable sales had a
// median absolute error of 7.4% (towers 5.8%, mid-rises 8.0%, walk-ups 10.7%, houses
// 11.7%), against 16.0% for the district median.
const (
	estimateRadiusM     = 1000.0
	estimateMonths      = 36
	minimumComparables  = 5
	maxComparablesShown = 100
	// Comparable sales are 0.6 to 1/0.6 times the home's area and at most 10 years
	// apart in age when both ages are known.
	areaRatioLimit = 0.6
	maxAgeGap      = 10
	// Weights fall by e for every 300 m, 30% of area (in log ratio), 18 months and 8
	// years apart; a sale in the same building counts ten times.
	distanceScaleM   = 300.0
	areaScale        = 0.3
	monthScale       = 18.0
	ageScale         = 8.0
	sameBuildingGain = 10.0
	// Towers and mid-rises gain 0.5% per floor. Ground floors, often shops, are only
	// compared with ground floors outside houses: this cut the backtest error of
	// ground-floor homes from 23.6% to 12.8% and of walk-ups from 12.8% to 10.7%.
	floorAdjustment = 0.005
	// The price index needs this many sales a month (pooled over three).
	minimumIndexSample = 30
	parkingMonths      = 24
	minimumParkingSale = 10
)

// confidenceBand holds, for estimates of one confidence in the backtest, the ratios of
// the actual price to the estimate at the 10th and 90th percentiles and the median
// absolute error.
type confidenceBand struct{ low, high, medianError float64 }

var confidenceBands = map[string]confidenceBand{
	"high":   {low: 0.886, high: 1.079, medianError: 0.039}, // 16% of estimates
	"medium": {low: 0.830, high: 1.140, medianError: 0.073}, // 62%
	"low":    {low: 0.768, high: 1.316, medianError: 0.118}, // 22%
}

var (
	buildingPattern     = regexp.MustCompile(`^(.*?\d+號)`)
	cityDistrictPattern = regexp.MustCompile(`^(台北市|新北市|桃園市)(\S+?[區市鎮鄉])`)
	cityCodes           = map[string]string{"台北市": "a", "新北市": "f", "桃園市": "h"}
)

type EstimateRepository interface {
	ImportedSalesByCity(context.Context, string) (seasons []string, releases []string, err error)
	EstimateCandidates(ctx context.Context, query domain.EstimateQuery, radius, minArea, maxArea float64, from, to time.Time) ([]domain.ComparableSale, error)
	PriceIndex(ctx context.Context, city string, first, last time.Time) ([]domain.PriceIndexPoint, error)
	ParkingPrice(ctx context.Context, city, district, category string, from, to time.Time) (*float64, int, error)
	BuildingProfile(ctx context.Context, building string, lat, lng float64) (domain.BuildingProfile, error)
	ImportedRentalsByCity(context.Context, string) (seasons []string, releases []string, err error)
	RentCandidates(ctx context.Context, query domain.EstimateQuery, radius, minArea, maxArea float64, from, to time.Time) ([]domain.LeaseCandidate, error)
	RentTrend(ctx context.Context, query domain.RentalQuery, district string, first, last time.Time) ([]domain.RentPoint, error)
	RentalAggregates(ctx context.Context, query domain.RentalQuery, from, to time.Time) ([]domain.RentalAggregate, error)
}

type EstimateService struct {
	repository EstimateRepository
	now        func() time.Time
}

func NewEstimateService(repository EstimateRepository) *EstimateService {
	return &EstimateService{repository: repository, now: time.Now}
}

// building returns the door number part of an address, e.g. 台北市大安區仁愛路四段27巷3號
// for 台北市大安區仁愛路四段27巷3號二樓, or "" when it has none.
func building(address string) string {
	match := buildingPattern.FindStringSubmatch(address)
	if match == nil {
		return ""
	}
	return match[1]
}

// Building summarises the recorded sales of the building at an address.
func (service *EstimateService) Building(ctx context.Context, address string, lat, lng float64) (domain.BuildingProfile, error) {
	key := building(address)
	if key == "" {
		return domain.BuildingProfile{}, fmt.Errorf("%w: address needs a door number", ErrInvalidInput)
	}
	if !inCoveredArea(lat, lng) {
		return domain.BuildingProfile{}, fmt.Errorf("%w: location outside the covered cities", ErrInvalidInput)
	}
	return service.repository.BuildingProfile(ctx, key, lat, lng)
}

// Estimate values a home from comparable sales nearby: each sale's price per ping is
// adjusted to the latest complete month with the city's price index and, for towers and
// mid-rises, to the home's floor, then weighted by distance, area, age, recency and
// whether it is in the same building. The estimate is the weighted median.
func (service *EstimateService) Estimate(ctx context.Context, query domain.EstimateQuery) (domain.Estimate, error) {
	if err := validateEstimate(query); err != nil {
		return domain.Estimate{}, err
	}
	estimate := domain.Estimate{MinimumSample: minimumComparables, Comparables: []domain.Comparable{}}
	now := service.now().UTC()
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	candidates, err := service.repository.EstimateCandidates(ctx, query, estimateRadiusM,
		query.AreaPing*areaRatioLimit, query.AreaPing/areaRatioLimit, thisMonth.AddDate(0, -estimateMonths-3, 0), thisMonth.AddDate(0, 1, 0))
	if err != nil {
		return domain.Estimate{}, err
	}

	estimate.CityCode, estimate.District = locate(query.Address, candidates)
	if estimate.CityCode == "" {
		return estimate, nil
	}
	seasons, releases, err := service.repository.ImportedSalesByCity(ctx, estimate.CityCode)
	if err != nil {
		return domain.Estimate{}, err
	}
	registered, known := registeredThrough(seasons, releases)
	if !known {
		return estimate, nil
	}
	asOf := completeThrough(registered)
	estimate.AsOf = asOf.Format("2006-01")
	index, err := service.repository.PriceIndex(ctx, estimate.CityCode, asOf.AddDate(0, -estimateMonths-3, 0), thisMonth)
	if err != nil {
		return domain.Estimate{}, err
	}
	indexByMonth := map[time.Time]float64{}
	for _, point := range index {
		if point.Median != nil && point.Sales >= minimumIndexSample {
			indexByMonth[point.Month] = *point.Median
		}
	}

	key := building(query.Address)
	var comparables []domain.Comparable
	var values, weights []float64
	for _, sale := range candidates {
		month := time.Date(sale.TransactedAt.Year(), sale.TransactedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
		monthsAgo := float64(monthsBetween(month, asOf))
		if monthsAgo > estimateMonths {
			continue
		}
		ageGap := 0.0
		if query.Age != nil && sale.Age != nil {
			ageGap = math.Abs(float64(*query.Age - *sale.Age))
			if ageGap > maxAgeGap {
				continue
			}
		}
		floor, floorKnown := lvr.FloorNumber(sale.FloorLabel)
		if floorKnown && query.Floor > 0 && query.Category != "透天厝" && (floor == 1) != (query.Floor == 1) {
			continue
		}
		adjusted := sale.PricePerPing
		if then, ok := indexByMonth[month]; ok {
			if target, ok := indexByMonth[asOf]; ok {
				adjusted *= target / then
			}
		}
		if floorKnown && query.Floor > 0 && (query.Category == "住宅大樓" || query.Category == "華廈") {
			adjusted *= 1 + floorAdjustment*float64(query.Floor-floor)
		}
		same := key != "" && building(sale.Address) == key
		weight := math.Exp(-sale.DistanceM/distanceScaleM) * math.Exp(-math.Abs(math.Log(sale.AreaPing/query.AreaPing))/areaScale) *
			math.Exp(-math.Max(monthsAgo, 0)/monthScale) * math.Exp(-ageGap/ageScale)
		if same {
			weight *= sameBuildingGain
			estimate.SameBuildingSales++
		}
		estimate.MaxDistanceM = math.Max(estimate.MaxDistanceM, math.Round(sale.DistanceM))
		values = append(values, adjusted)
		weights = append(weights, weight)
		comparables = append(comparables, domain.Comparable{
			ID: sale.ID, Address: sale.Address, Lat: sale.Lat, Lng: sale.Lng, TransactedAt: sale.TransactedAt.Format("2006-01-02"),
			AreaPing: sale.AreaPing, Age: sale.Age, FloorLabel: sale.FloorLabel, PricePerPing: sale.PricePerPing,
			AdjustedPricePerPing: math.Round(adjusted), DistanceM: math.Round(sale.DistanceM), SameBuilding: same, Weight: weight,
		})
	}
	estimate.ComparableSales = len(comparables)
	if len(comparables) < minimumComparables {
		estimate.Comparables = comparables
		if estimate.Comparables == nil {
			estimate.Comparables = []domain.Comparable{}
		}
		return estimate, nil
	}

	total := 0.0
	for _, weight := range weights {
		total += weight
	}
	for index := range comparables {
		comparables[index].Weight /= total
	}
	sort.SliceStable(comparables, func(i, j int) bool { return comparables[i].Weight > comparables[j].Weight })
	estimate.Comparables = comparables[:min(len(comparables), maxComparablesShown)]

	median := weightedQuantile(values, weights, 0.5)
	spread := (weightedQuantile(values, weights, 0.75) - weightedQuantile(values, weights, 0.25)) / median
	estimate.Sufficient = true
	estimate.Spread = math.Round(spread*1000) / 1000
	switch {
	case estimate.SameBuildingSales >= 3 && spread < 0.15:
		estimate.Confidence = "high"
	case len(comparables) >= 20 && spread < 0.25:
		estimate.Confidence = "medium"
	default:
		estimate.Confidence = "low"
	}
	band := confidenceBands[estimate.Confidence]
	estimate.BacktestMedianError, estimate.RangeLow, estimate.RangeHigh = band.medianError, band.low, band.high

	perPing := math.Round(median)
	buildingPrice := perPing * query.AreaPing
	totalPrice := buildingPrice
	if query.Parking {
		from := asOf.AddDate(0, -parkingMonths+1, 0)
		parking, count, err := service.repository.ParkingPrice(ctx, estimate.CityCode, estimate.District, query.Category, from, asOf.AddDate(0, 1, 0))
		if err == nil && (parking == nil || count < minimumParkingSale) {
			parking, count, err = service.repository.ParkingPrice(ctx, estimate.CityCode, "", query.Category, from, asOf.AddDate(0, 1, 0))
		}
		if err != nil {
			return domain.Estimate{}, err
		}
		if parking != nil && count >= minimumParkingSale {
			estimate.ParkingPrice = new(math.Round(*parking))
			totalPrice += *estimate.ParkingPrice
		}
	}
	parkingPrice := totalPrice - buildingPrice
	estimate.PricePerPing = &perPing
	estimate.LowPricePerPing = new(math.Round(perPing * band.low))
	estimate.HighPricePerPing = new(math.Round(perPing * band.high))
	estimate.BuildingPrice = new(math.Round(buildingPrice))
	estimate.TotalPrice = new(math.Round(totalPrice))
	estimate.LowPrice = new(math.Round(buildingPrice*band.low + parkingPrice))
	estimate.HighPrice = new(math.Round(buildingPrice*band.high + parkingPrice))
	if estimate.Rent, err = service.rentEstimate(ctx, query, estimate); err != nil {
		return domain.Estimate{}, err
	}
	return estimate, nil
}

func validateEstimate(query domain.EstimateQuery) error {
	if !slices.Contains(domain.BuildingCategories[:len(domain.BuildingCategories)-1], query.Category) {
		return fmt.Errorf("%w: choose one residential building type", ErrInvalidInput)
	}
	if !inCoveredArea(query.Lat, query.Lng) {
		return fmt.Errorf("%w: location outside the covered cities", ErrInvalidInput)
	}
	if query.AreaPing < 3 || query.AreaPing > 300 {
		return fmt.Errorf("%w: area must be between 3 and 300 ping", ErrInvalidInput)
	}
	if query.Age != nil && (*query.Age < 0 || *query.Age > 100) {
		return fmt.Errorf("%w: age must be between 0 and 100 years", ErrInvalidInput)
	}
	if query.Floor < 0 || query.TotalFloors < 0 || query.Floor > 120 || query.TotalFloors > 120 ||
		(query.Floor > 0 && query.TotalFloors > 0 && query.Floor > query.TotalFloors) {
		return fmt.Errorf("%w: invalid floor", ErrInvalidInput)
	}
	return nil
}

// inCoveredArea roughly bounds 台北, 新北 and 桃園.
func inCoveredArea(lat, lng float64) bool {
	return lat >= 24.5 && lat <= 25.4 && lng >= 120.9 && lng <= 122.1
}

// locate returns the city code and district of the address, or else of the nearest sale.
func locate(address string, sales []domain.ComparableSale) (string, string) {
	if match := cityDistrictPattern.FindStringSubmatch(address); match != nil {
		return cityCodes[match[1]], match[2]
	}
	nearest := -1
	for index, sale := range sales {
		if nearest < 0 || sale.DistanceM < sales[nearest].DistanceM {
			nearest = index
		}
	}
	if nearest >= 0 {
		if match := cityDistrictPattern.FindStringSubmatch(sales[nearest].Address); match != nil {
			return cityCodes[match[1]], match[2]
		}
	}
	return "", ""
}

// monthsBetween counts whole months from from to to; negative when to is earlier.
func monthsBetween(from, to time.Time) int {
	return (to.Year()-from.Year())*12 + int(to.Month()-from.Month())
}

// weightedQuantile interpolates the quantile of values weighted by weights, placing each
// value at the middle of its weight.
func weightedQuantile(values, weights []float64, quantile float64) float64 {
	order := make([]int, len(values))
	for index := range order {
		order[index] = index
	}
	sort.Slice(order, func(i, j int) bool { return values[order[i]] < values[order[j]] })
	total := 0.0
	for _, weight := range weights {
		total += weight
	}
	cumulative := 0.0
	positions := make([]float64, len(order))
	for rank, index := range order {
		positions[rank] = (cumulative + weights[index]/2) / total
		cumulative += weights[index]
	}
	if quantile <= positions[0] {
		return values[order[0]]
	}
	for rank := 1; rank < len(order); rank++ {
		if quantile <= positions[rank] {
			fraction := (quantile - positions[rank-1]) / (positions[rank] - positions[rank-1])
			return values[order[rank-1]] + fraction*(values[order[rank]]-values[order[rank-1]])
		}
	}
	return values[order[len(order)-1]]
}
