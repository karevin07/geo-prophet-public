package mock

import (
	"context"
	"fmt"
	"maps"
	"math"
	"math/rand"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/districtmap"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

// TransactionRepository serves deterministic Taipei transactions for development without a database.
type TransactionRepository struct {
	transactions []domain.Transaction
	now          time.Time
}

// NewTransactionRepository generates count transactions dated within the 12 months before now.
func NewTransactionRepository(count int, now time.Time) *TransactionRepository {
	random := rand.New(rand.NewSource(591))
	districts := []string{"信義區", "大安區", "松山區", "中山區", "中正區", "南港區", "內湖區", "士林區"}
	roads := []string{"忠孝東路四段", "信義路三段", "南京東路五段", "民生東路三段", "和平東路二段", "內湖路一段"}
	buildingTypes := []string{"住宅大樓(11層含以上有電梯)", "華廈(10層含以下有電梯)", "公寓(5樓含以下無電梯)", "透天厝", "套房(1房1廳1衛)"}
	start := now.UTC().AddDate(-1, 0, 0)

	transactions := make([]domain.Transaction, 0, count)
	for index := range count {
		lat := between(random, 24.98, 25.10)
		lng := between(random, 121.45, 121.60)
		distance := math.Hypot(lat-25.03, lng-121.56)
		pricePerPing := math.Max(350_000, 1_300_000-distance*8_000_000) + between(random, -100_000, 200_000)
		area := math.Round(between(random, 8, 60)*100) / 100
		buildingPrice := int64(math.Round(pricePerPing*area/10_000) * 10_000)
		var parkingPrice int64
		if random.Float64() < 0.35 {
			parkingPrice = int64(between(random, 150, 350)) * 10_000
		}
		totalFloors := 4 + random.Intn(20)
		district := districts[random.Intn(len(districts))]
		transactedAt := start.AddDate(0, 0, random.Intn(365)).Truncate(24 * time.Hour)
		var completedAt *time.Time
		if index%9 != 0 {
			completed := transactedAt.AddDate(-(index % 42), 0, 0)
			completedAt = &completed
		}
		transactions = append(transactions, domain.Transaction{
			ID:               fmt.Sprintf("MOCK%06d", index),
			CityCode:         "a",
			District:         district,
			Address:          fmt.Sprintf("台北市%s%s%d號", district, roads[random.Intn(len(roads))], 1+random.Intn(300)),
			TransactionType:  "房地(土地+建物)",
			BuildingType:     buildingTypes[random.Intn(len(buildingTypes))],
			MainUse:          "住家用",
			TotalPrice:       buildingPrice + parkingPrice,
			ParkingPrice:     parkingPrice,
			BuildingPrice:    buildingPrice,
			BuildingAreaPing: area,
			PricePerPing:     math.Round(float64(buildingPrice) / area),
			Rooms:            rooms(area),
			Halls:            1,
			Baths:            max(1, rooms(area)-1),
			FloorLabel:       fmt.Sprintf("%d層", 1+random.Intn(totalFloors)),
			TotalFloors:      totalFloors,
			CompletedAt:      completedAt,
			TransactedAt:     transactedAt,
			Season:           "mock",
			Lat:              lat,
			Lng:              lng,
			GeocodePrecision: "address",
		})
	}
	return &TransactionRepository{transactions: transactions, now: now}
}

func (repository *TransactionRepository) List(ctx context.Context, filter domain.TransactionFilter) ([]domain.Transaction, error) {
	result := make([]domain.Transaction, 0, min(filter.Limit, len(repository.transactions)))
	for _, transaction := range repository.transactions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if matches(transaction, filter) {
			result = append(result, transaction)
			if len(result) == filter.Limit {
				break
			}
		}
	}
	return result, nil
}

func (repository *TransactionRepository) ForEachPricePoint(ctx context.Context, filter domain.TransactionFilter, emit func(domain.PricePoint) error) error {
	for _, transaction := range repository.transactions {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !matches(transaction, filter) {
			continue
		}
		if err := emit(domain.PricePoint{
			Lat: transaction.Lat, Lng: transaction.Lng, TotalPrice: transaction.TotalPrice,
			BuildingPrice: transaction.BuildingPrice, BuildingAreaPing: transaction.BuildingAreaPing,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (repository *TransactionRepository) MonthlyTrends(ctx context.Context, filter domain.TransactionFilter) ([]domain.TrendPoint, error) {
	filter.Limit = len(repository.transactions)
	transactions, err := repository.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	type accumulator struct {
		count         int
		buildingPrice int64
		area          float64
	}
	groups := map[string]accumulator{}
	for _, transaction := range transactions {
		month := transaction.TransactedAt.Format("2006-01")
		group := groups[month]
		group.count++
		group.buildingPrice += transaction.BuildingPrice
		group.area += transaction.BuildingAreaPing
		groups[month] = group
	}
	trends := make([]domain.TrendPoint, 0, len(groups))
	for month, group := range groups {
		trends = append(trends, domain.TrendPoint{Month: month, AvgPricePerPing: float64(group.buildingPrice) / group.area, Count: group.count})
	}
	sort.Slice(trends, func(i, j int) bool { return trends[i].Month < trends[j].Month })
	return trends, nil
}

func (repository *TransactionRepository) Summary(ctx context.Context, filter domain.TransactionFilter) (domain.MarketSummary, error) {
	filter.Limit = len(repository.transactions)
	transactions, err := repository.List(ctx, filter)
	if err != nil || len(transactions) == 0 {
		return domain.MarketSummary{}, err
	}
	var buildingPrice int64
	var area float64
	prices := make([]int64, 0, len(transactions))
	for _, transaction := range transactions {
		buildingPrice += transaction.BuildingPrice
		area += transaction.BuildingAreaPing
		prices = append(prices, transaction.TotalPrice)
	}
	slices.Sort(prices)
	median := float64(prices[len(prices)/2])
	if len(prices)%2 == 0 {
		median = float64(prices[len(prices)/2-1]+prices[len(prices)/2]) / 2
	}
	return domain.MarketSummary{Count: len(transactions), AvgPricePerPing: float64(buildingPrice) / area, MedianTotalPrice: median}, nil
}

func (repository *TransactionRepository) Analysis(ctx context.Context, filter domain.TransactionFilter) (domain.MarketAnalysis, error) {
	filter.Limit = len(repository.transactions)
	transactions, err := repository.List(ctx, filter)
	if err != nil {
		return domain.MarketAnalysis{}, err
	}
	type analysisGroup struct {
		count         int
		buildingPrice int64
		area          float64
		unitPrices    []float64
		totalPrices   []float64
	}
	add := func(group analysisGroup, transaction domain.Transaction) analysisGroup {
		group.count++
		group.buildingPrice += transaction.BuildingPrice
		group.area += transaction.BuildingAreaPing
		group.unitPrices = append(group.unitPrices, transaction.PricePerPing)
		group.totalPrices = append(group.totalPrices, float64(transaction.TotalPrice))
		return group
	}

	months := map[string]analysisGroup{}
	categories := map[string]analysisGroup{}
	ages := map[string]analysisGroup{}
	areas := map[string]analysisGroup{}
	buckets := map[int64]int{}
	for _, transaction := range transactions {
		month := transaction.TransactedAt.Format("2006-01")
		months[month] = add(months[month], transaction)
		category := domain.BuildingCategory(transaction.BuildingType)
		categories[category] = add(categories[category], transaction)
		ageKey := "unknown"
		if transaction.CompletedAt != nil {
			age := transaction.TransactedAt.Year() - transaction.CompletedAt.Year()
			switch {
			case age < 5:
				ageKey = "under5"
			case age < 10:
				ageKey = "5_9"
			case age < 20:
				ageKey = "10_19"
			case age < 30:
				ageKey = "20_29"
			default:
				ageKey = "30plus"
			}
		}
		ages[ageKey] = add(ages[ageKey], transaction)
		areaKey := "45plus"
		switch {
		case transaction.BuildingAreaPing < 15:
			areaKey = "under15"
		case transaction.BuildingAreaPing < 30:
			areaKey = "15_29"
		case transaction.BuildingAreaPing < 45:
			areaKey = "30_44"
		}
		areas[areaKey] = add(areas[areaKey], transaction)
		lower := int64(math.Floor(transaction.PricePerPing/100_000)) * 100_000
		buckets[lower]++
	}

	result := domain.MarketAnalysis{
		Monthly: []domain.MonthlyAnalysis{}, Categories: []domain.CategoryAnalysis{},
		AgeGroups: []domain.BreakdownAnalysis{}, AreaGroups: []domain.BreakdownAnalysis{}, Distribution: []domain.PriceBucket{},
	}
	monthKeys := slices.Sorted(maps.Keys(months))
	for _, month := range monthKeys {
		group := months[month]
		result.Monthly = append(result.Monthly, domain.MonthlyAnalysis{
			Month: month, Count: group.count, AvgPricePerPing: float64(group.buildingPrice) / group.area,
			MedianPricePerPing: medianFloat(group.unitPrices), MedianTotalPrice: medianFloat(group.totalPrices), AvgArea: group.area / float64(group.count),
		})
	}
	for _, category := range domain.BuildingCategories {
		group, ok := categories[category]
		if !ok {
			continue
		}
		result.Categories = append(result.Categories, domain.CategoryAnalysis{
			Category: category, Count: group.count, AvgPricePerPing: float64(group.buildingPrice) / group.area,
			MedianPricePerPing: medianFloat(group.unitPrices), MedianTotalPrice: medianFloat(group.totalPrices),
		})
	}
	appendGroups := func(groups map[string]analysisGroup, keys []string) []domain.BreakdownAnalysis {
		result := []domain.BreakdownAnalysis{}
		for _, key := range keys {
			group, ok := groups[key]
			if !ok {
				continue
			}
			result = append(result, domain.BreakdownAnalysis{Key: key, Count: group.count,
				AvgPricePerPing:    float64(group.buildingPrice) / group.area,
				MedianPricePerPing: medianFloat(group.unitPrices), MedianTotalPrice: medianFloat(group.totalPrices)})
		}
		return result
	}
	result.AgeGroups = appendGroups(ages, []string{"under5", "5_9", "10_19", "20_29", "30plus", "unknown"})
	result.AreaGroups = appendGroups(areas, []string{"under15", "15_29", "30_44", "45plus"})
	for _, lower := range slices.Sorted(maps.Keys(buckets)) {
		result.Distribution = append(result.Distribution, domain.PriceBucket{MinPricePerPing: lower, MaxPricePerPing: lower + 100_000, Count: buckets[lower]})
	}
	return result, nil
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := slices.Clone(values)
	slices.Sort(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}

func (repository *TransactionRepository) districtSales(ctx context.Context, query domain.DistrictComparisonQuery) ([]domain.Transaction, error) {
	query.Filter.Limit = len(repository.transactions)
	rows, err := repository.List(ctx, query.Filter)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Transaction, 0, len(rows))
	for _, transaction := range rows {
		if transaction.CityCode == query.CityCode {
			result = append(result, transaction)
		}
	}
	return result, nil
}

func (repository *TransactionRepository) DistrictRankings(ctx context.Context, query domain.DistrictComparisonQuery) ([]domain.DistrictRank, error) {
	rows, err := repository.districtSales(ctx, query)
	if err != nil {
		return nil, err
	}
	type group struct {
		count         int
		buildingPrice int64
		area          float64
		unitPrices    []float64
		totalPrices   []float64
	}
	groups := make(map[string]group)
	for _, transaction := range rows {
		value := groups[transaction.District]
		value.count++
		value.buildingPrice += transaction.BuildingPrice
		value.area += transaction.BuildingAreaPing
		value.unitPrices = append(value.unitPrices, transaction.PricePerPing)
		value.totalPrices = append(value.totalPrices, float64(transaction.TotalPrice))
		groups[transaction.District] = value
	}
	ranking := make([]domain.DistrictRank, 0, len(groups))
	for district, value := range groups {
		ranking = append(ranking, domain.DistrictRank{District: district, Count: value.count,
			AvgPricePerPing:    float64(value.buildingPrice) / value.area,
			MedianPricePerPing: medianFloat(value.unitPrices), MedianTotalPrice: medianFloat(value.totalPrices)})
	}
	slices.SortFunc(ranking, func(a, b domain.DistrictRank) int {
		if a.MedianPricePerPing > b.MedianPricePerPing {
			return -1
		}
		if a.MedianPricePerPing < b.MedianPricePerPing {
			return 1
		}
		return strings.Compare(a.District, b.District)
	})
	return ranking, nil
}

func (repository *TransactionRepository) DistrictTrends(ctx context.Context, query domain.DistrictComparisonQuery) ([]domain.DistrictTrend, error) {
	rows, err := repository.districtSales(ctx, query)
	if err != nil {
		return nil, err
	}
	type group struct {
		count         int
		buildingPrice int64
		area          float64
	}
	groups := make(map[string]group)
	for _, transaction := range rows {
		if !slices.Contains(query.Districts, transaction.District) {
			continue
		}
		key := transaction.District + "|" + transaction.TransactedAt.Format("2006-01")
		value := groups[key]
		value.count++
		value.buildingPrice += transaction.BuildingPrice
		value.area += transaction.BuildingAreaPing
		groups[key] = value
	}
	trends := make([]domain.DistrictTrend, 0, len(groups))
	for key, value := range groups {
		district, month, _ := strings.Cut(key, "|")
		trends = append(trends, domain.DistrictTrend{District: district, Month: month, Count: value.count,
			AvgPricePerPing: float64(value.buildingPrice) / value.area})
	}
	slices.SortFunc(trends, func(a, b domain.DistrictTrend) int {
		if order := strings.Compare(a.Month, b.Month); order != 0 {
			return order
		}
		return strings.Compare(a.District, b.District)
	})
	return trends, nil
}

func (repository *TransactionRepository) DistrictYoYTotals(ctx context.Context, query domain.DistrictComparisonQuery, previousFrom, currentFrom, currentTo time.Time) (map[string]domain.DistrictYoYTotals, error) {
	query.Filter.From, query.Filter.To = &previousFrom, &currentTo
	rows, err := repository.districtSales(ctx, query)
	if err != nil {
		return nil, err
	}
	totals := make(map[string]domain.DistrictYoYTotals)
	for _, transaction := range rows {
		value := totals[transaction.District]
		if transaction.TransactedAt.Before(currentFrom) {
			value.PreviousCount++
			value.PreviousBuildingPrice += transaction.BuildingPrice
			value.PreviousArea += transaction.BuildingAreaPing
		} else {
			value.CurrentCount++
			value.CurrentBuildingPrice += transaction.BuildingPrice
			value.CurrentArea += transaction.BuildingAreaPing
		}
		totals[transaction.District] = value
	}
	return totals, nil
}

// feedRows filters, sorts and pages like the Postgres feed. The mock has no releases, so
// nothing is new and a latest-release-only query is empty.
func (repository *TransactionRepository) feedRows(query domain.FeedQuery, limit int) []domain.Transaction {
	if query.LatestReleaseOnly {
		return nil
	}
	var rows []domain.Transaction
	for _, transaction := range repository.transactions {
		if matches(transaction, query.Filter) && (query.CityCode == "" || transaction.CityCode == query.CityCode) &&
			(query.District == "" || transaction.District == query.District) {
			rows = append(rows, transaction)
		}
	}
	key := func(transaction domain.Transaction) float64 {
		switch query.Sort {
		case domain.SortPricePerPingDesc, domain.SortPricePerPingAsc:
			return transaction.PricePerPing
		case domain.SortTotalPriceDesc:
			return float64(transaction.TotalPrice)
		}
		return float64(transaction.TransactedAt.Unix())
	}
	descending := query.Sort != domain.SortDateAsc && query.Sort != domain.SortPricePerPingAsc
	before := func(a, b domain.Transaction) bool {
		if key(a) != key(b) {
			return (key(a) > key(b)) == descending
		}
		return (a.ID > b.ID) == descending
	}
	sort.Slice(rows, func(i, j int) bool { return before(rows[i], rows[j]) })
	if query.Cursor != nil {
		for index, transaction := range rows {
			if transaction.ID == query.Cursor.ID {
				rows = rows[index+1:]
				break
			}
		}
	}
	return rows[:min(limit, len(rows))]
}

func (repository *TransactionRepository) Feed(_ context.Context, query domain.FeedQuery, _ []int64, limit int) ([]domain.Transaction, error) {
	return repository.feedRows(query, limit), nil
}

func (repository *TransactionRepository) FeedCounts(_ context.Context, query domain.FeedQuery, _ []int64) (int, int, error) {
	query.Cursor = nil
	return len(repository.feedRows(query, len(repository.transactions))), 0, nil
}

func (repository *TransactionRepository) ExportFeed(_ context.Context, query domain.FeedQuery, _ []int64, limit int, emit func(domain.Transaction) error) error {
	query.Cursor = nil
	for _, transaction := range repository.feedRows(query, limit) {
		if err := emit(transaction); err != nil {
			return err
		}
	}
	return nil
}

func (*TransactionRepository) LatestRelease(context.Context) (string, []int64, error) {
	return "", nil, nil
}

// DistrictBoundaries derives areas from the mock sales, the way real ones come from door numbers.
func (repository *TransactionRepository) DistrictBoundaries(_ context.Context, cityCode string) ([]domain.DistrictBoundary, error) {
	districts := addresspoint.Sources[cityCode].Districts
	points := make([]addresspoint.Point, 0, len(repository.transactions))
	for _, transaction := range repository.transactions {
		if transaction.CityCode == cityCode {
			points = append(points, addresspoint.Point{DistrictCode: districts[transaction.District], Lat: transaction.Lat, Lng: transaction.Lng})
		}
	}
	return districtmap.Build(points, districts)
}

func (repository *TransactionRepository) CellRows(ctx context.Context, filter domain.TransactionFilter) ([]domain.CellRow, error) {
	transactions, err := repository.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	rows := make([]domain.CellRow, 0, len(transactions))
	for _, transaction := range transactions {
		var age *int
		if transaction.CompletedAt != nil {
			years := int(transaction.TransactedAt.Sub(*transaction.CompletedAt).Hours() / 24 / 365.25)
			age = &years
		}
		rows = append(rows, domain.CellRow{
			CityCode: transaction.CityCode, District: transaction.District, Address: transaction.Address,
			Lat: transaction.Lat, Lng: transaction.Lng, TransactedAt: transaction.TransactedAt,
			TotalPrice: transaction.TotalPrice, BuildingPrice: transaction.BuildingPrice,
			AreaPing: transaction.BuildingAreaPing, BuildingAge: age,
		})
	}
	return rows, nil
}

// ImportedSales pretends every season up to the previous one was imported, so the latest
// months show as provisional like they would with real data.
func (repository *TransactionRepository) ImportedSales(context.Context) ([]string, []string, error) {
	return []string{lvr.SeasonOfRegistration(repository.now).Previous().String()}, nil, nil
}

func (repository *TransactionRepository) ImportedSalesByCity(ctx context.Context, city string) ([]string, []string, error) {
	if city != "a" {
		return nil, nil, nil
	}
	return repository.ImportedSales(ctx)
}

func (repository *TransactionRepository) InvestmentAggregates(ctx context.Context, query domain.InvestmentQuery, window domain.InvestmentWindow) ([]domain.InvestmentAggregate, error) {
	type prices struct{ current, previous []float64 }
	groups := map[string]*prices{"": {}}
	for _, sale := range repository.transactions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if sale.CityCode != query.CityCode || sale.MainUse != "住家用" || !strings.HasPrefix(sale.TransactionType, "房地") ||
			sale.IsSpecial() || domain.BuildingCategory(sale.BuildingType) != query.Category ||
			sale.BuildingAreaPing < query.MinArea || sale.BuildingAreaPing >= query.MaxArea ||
			sale.CompletedAt == nil || sale.CompletedAt.After(sale.TransactedAt) ||
			sale.TransactedAt.Before(window.PreviousFrom) || !sale.TransactedAt.Before(window.EndExclusive) {
			continue
		}
		age := int(sale.TransactedAt.Sub(*sale.CompletedAt).Hours() / 24 / 365.25)
		if age < query.MinAge || age >= query.MaxAge {
			continue
		}
		if groups[sale.District] == nil {
			groups[sale.District] = &prices{}
		}
		for _, key := range []string{"", sale.District} {
			if !sale.TransactedAt.Before(window.CurrentFrom) {
				groups[key].current = append(groups[key].current, sale.PricePerPing)
			} else {
				groups[key].previous = append(groups[key].previous, sale.PricePerPing)
			}
		}
	}
	result := make([]domain.InvestmentAggregate, 0, len(groups))
	for district, group := range groups {
		row := domain.InvestmentAggregate{District: district, CurrentCount: len(group.current), PreviousCount: len(group.previous)}
		if row.CurrentCount > 0 {
			median := medianFloat(group.current)
			row.MedianPricePerPing = &median
		}
		if row.PreviousCount > 0 {
			median := medianFloat(group.previous)
			row.PreviousMedianPricePerPing = &median
		}
		result = append(result, row)
	}
	return result, nil
}

// The mock has no door-number data, so place search only resolves districts.
func (*TransactionRepository) FindAddresses(context.Context, domain.PlaceQuery) ([]domain.PlaceMatch, error) {
	return nil, nil
}

func (*TransactionRepository) FindRoads(context.Context, domain.PlaceQuery) ([]domain.PlaceMatch, error) {
	return nil, nil
}

func (repository *TransactionRepository) DistrictCenter(_ context.Context, cityCode, districtCode string) (float64, float64, bool, error) {
	var latSum, lngSum float64
	count := 0
	for _, transaction := range repository.transactions {
		if transaction.CityCode == cityCode && addresspoint.Sources[cityCode].Districts[transaction.District] == districtCode {
			latSum += transaction.Lat
			lngSum += transaction.Lng
			count++
		}
	}
	if count == 0 {
		return 0, 0, false, nil
	}
	return latSum / float64(count), lngSum / float64(count), true, nil
}

func (*TransactionRepository) Ping(context.Context) error { return nil }

func matches(transaction domain.Transaction, filter domain.TransactionFilter) bool {
	if transaction.TotalPrice < filter.MinPrice || transaction.TotalPrice > filter.MaxPrice ||
		transaction.BuildingAreaPing < filter.MinArea || transaction.BuildingAreaPing > filter.MaxArea {
		return false
	}
	if !filter.IncludeSpecial && transaction.IsSpecial() {
		return false
	}
	if len(filter.Categories) > 0 && !slices.Contains(filter.Categories, domain.BuildingCategory(transaction.BuildingType)) {
		return false
	}
	if filter.Bounds != nil && (transaction.Lng < filter.Bounds.West || transaction.Lng > filter.Bounds.East ||
		transaction.Lat < filter.Bounds.South || transaction.Lat > filter.Bounds.North) {
		return false
	}
	if filter.ExcludeNewBuildings && transaction.CompletedAt != nil &&
		transaction.TransactedAt.Before(transaction.CompletedAt.AddDate(domain.NewBuildingYears, 0, 0)) {
		return false
	}
	if filter.From != nil && transaction.TransactedAt.Before(*filter.From) {
		return false
	}
	return filter.To == nil || !transaction.TransactedAt.After(*filter.To)
}

func between(random *rand.Rand, minimum, maximum float64) float64 {
	return random.Float64()*(maximum-minimum) + minimum
}

func rooms(area float64) int {
	switch {
	case area < 15:
		return 1
	case area < 25:
		return 2
	case area < 40:
		return 3
	default:
		return 4
	}
}
