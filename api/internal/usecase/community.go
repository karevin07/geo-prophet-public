package usecase

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
	"github.com/geo-prophet/geo-prophet/api/internal/twtext"
)

const (
	// A community's median needs 3 market sales, its YoY 5 in each year.
	minCommunityMedianSales = 3
	minCommunityYoYSales    = 5
	// The floor premium, rooms and areas look back five years, leases three.
	communityProfileYears = 5
	communityRentMonths   = 36
	minCommunityLeases    = 3
	// The floor premium needs 10 sales above the ground floor, on 3 floors.
	minFloorPremiumSales  = 10
	minFloorPremiumFloors = 3
	// Nearby communities are within 500 m, completed within 5 years, with 3 recent sales.
	nearbyCommunityRadiusM = 500
	nearbyCommunityYears   = 5
	maxNearbyCommunities   = 6
	maxRankedCommunities   = 20
	// cellCommunities is how many communities a hexagon card lists, searchCommunities how
	// many named ones a search finds.
	cellCommunities   = 3
	searchCommunities = 3
)

type CommunityRepository interface {
	ImportedSalesByCity(context.Context, string) (seasons []string, releases []string, err error)
	Community(ctx context.Context, id string) (domain.CommunityRecord, bool, error)
	CommunitiesByID(ctx context.Context, ids []string) ([]domain.CommunityRecord, error)
	CommunitiesByDoor(ctx context.Context, address string) ([]domain.CommunityRecord, error)
	CommunitiesByName(ctx context.Context, text string, limit int) ([]domain.CommunityRecord, error)
	CommunitySales(ctx context.Context, id string) ([]domain.Transaction, error)
	DistrictYearMedians(ctx context.Context, cityCode, district, category string, from time.Time) (map[int]float64, error)
	DistrictMedian(ctx context.Context, cityCode, district, category string, from, to time.Time) (domain.CommunityStats, error)
	CommunityLeases(ctx context.Context, id string, from time.Time) (int, *float64, *float64, error)
	NearbyCommunities(ctx context.Context, id string, radius float64, maxYears, minSales int, from, to time.Time, limit int) ([]domain.CommunityRecord, []domain.NearbyCommunity, error)
	RankedCommunities(ctx context.Context, cityCode, district string, from, to time.Time, limit int) ([]domain.CommunityRecord, []domain.RankedCommunity, error)
}

type CommunityService struct {
	repository CommunityRepository
	now        func() time.Time
}

func NewCommunityService(repository CommunityRepository) *CommunityService {
	return &CommunityService{repository: repository, now: time.Now}
}

// Community describes one community: recent prices against its district, prices by year
// and floor, rents, similar communities nearby and every sale.
func (service *CommunityService) Community(ctx context.Context, id string) (domain.CommunityDetail, error) {
	record, found, err := service.repository.Community(ctx, id)
	if err != nil {
		return domain.CommunityDetail{}, err
	}
	if !found {
		return domain.CommunityDetail{}, fmt.Errorf("%w: community %q", ErrNotFound, id)
	}
	sales, err := service.repository.CommunitySales(ctx, id)
	if err != nil {
		return domain.CommunityDetail{}, err
	}
	through, err := service.through(ctx, record.CityCode)
	if err != nil {
		return domain.CommunityDetail{}, err
	}
	from, to := through.AddDate(0, -11, 0), through.AddDate(0, 1, 0)
	detail := domain.CommunityDetail{
		Community: describeCommunity(record), From: from.Format("2006-01"), Through: through.Format("2006-01"),
		Years: []domain.CommunityYear{}, Nearby: []domain.NearbyCommunity{}, Sales: sales,
	}

	current := marketPrices(sales, from, to)
	previous := marketPrices(sales, from.AddDate(-1, 0, 0), from)
	detail.RecentSales = len(current)
	if len(current) >= minCommunityMedianSales {
		detail.MedianPricePerPing = new(math.Round(median(current)))
	}
	if len(current) >= minCommunityYoYSales && len(previous) >= minCommunityYoYSales {
		detail.PriceYoY = change(median(current), median(previous))
	}
	district, err := service.repository.DistrictMedian(ctx, record.CityCode, record.District, record.Category, from, to)
	if err != nil {
		return domain.CommunityDetail{}, err
	}
	if district.MedianPricePerPing != nil {
		detail.DistrictPricePerPing = new(math.Round(*district.MedianPricePerPing))
		if detail.MedianPricePerPing != nil {
			detail.Premium = change(*detail.MedianPricePerPing, *detail.DistrictPricePerPing)
		}
	}

	profileFrom := to.AddDate(-communityProfileYears, 0, 0)
	detail.MedianAreaPing, detail.MainRooms, detail.MainRoomsShare = communityLayout(sales, profileFrom, to)
	detail.FloorPremium = floorPremium(sales, profileFrom, to)
	if detail.Years, err = service.years(ctx, record, sales, through); err != nil {
		return domain.CommunityDetail{}, err
	}
	if detail.Rent, err = service.rent(ctx, record.ID, sales, to); err != nil {
		return domain.CommunityDetail{}, err
	}
	if record.Lat != nil {
		records, nearby, err := service.repository.NearbyCommunities(ctx, record.ID, nearbyCommunityRadiusM, nearbyCommunityYears,
			minCommunityMedianSales, from, to, maxNearbyCommunities)
		if err != nil {
			return domain.CommunityDetail{}, err
		}
		for index := range nearby {
			nearby[index].Community = describeCommunity(records[index])
			if nearby[index].MedianPricePerPing != nil {
				nearby[index].MedianPricePerPing = new(math.Round(*nearby[index].MedianPricePerPing))
				if detail.MedianPricePerPing != nil {
					nearby[index].Difference = change(*nearby[index].MedianPricePerPing, *detail.MedianPricePerPing)
				}
			}
		}
		detail.Nearby = nearby
	}
	return detail, nil
}

// years compares the community's median in each year with its district's.
func (service *CommunityService) years(ctx context.Context, record domain.CommunityRecord, sales []domain.Transaction, through time.Time) ([]domain.CommunityYear, error) {
	prices := map[int][]float64{}
	today := service.now()
	for _, sale := range sales {
		if marketTransaction(sale) && !sale.TransactedAt.After(today) {
			prices[sale.TransactedAt.Year()] = append(prices[sale.TransactedAt.Year()], sale.PricePerPing)
		}
	}
	years := []domain.CommunityYear{}
	if len(prices) == 0 {
		return years, nil
	}
	first := slices.Min(mapKeys(prices))
	districtMedians, err := service.repository.DistrictYearMedians(ctx, record.CityCode, record.District, record.Category,
		time.Date(first, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, err
	}
	for _, year := range slices.Sorted(slices.Values(mapKeys(prices))) {
		point := domain.CommunityYear{Year: year, Sales: len(prices[year]), MedianPricePerPing: math.Round(median(prices[year])),
			Provisional: year > through.Year() || (year == through.Year() && through.Month() < time.December)}
		if value, ok := districtMedians[year]; ok {
			point.DistrictMedianPricePerPing = new(math.Round(value))
		}
		years = append(years, point)
	}
	return years, nil
}

// rent summarises leases over the last three years and the yield against sales over the
// same months.
func (service *CommunityService) rent(ctx context.Context, id string, sales []domain.Transaction, to time.Time) (*domain.CommunityRent, error) {
	from := to.AddDate(0, -communityRentMonths, 0)
	leases, perPing, monthly, err := service.repository.CommunityLeases(ctx, id, from)
	if err != nil {
		return nil, err
	}
	rent := &domain.CommunityRent{From: from.Format("2006-01"), Leases: leases}
	if leases < minCommunityLeases || perPing == nil {
		return rent, nil
	}
	rent.MedianRentPerPing = new(math.Round(*perPing))
	rent.MedianMonthlyRent = new(math.Round(*monthly))
	if prices := marketPrices(sales, from, to); len(prices) >= minCommunityMedianSales {
		rent.GrossYield = new(*perPing * 12 / median(prices))
	}
	return rent, nil
}

// Ranking lists a district's communities with the most market sales over the last 12
// complete months.
func (service *CommunityService) Ranking(ctx context.Context, city, district string) (domain.CommunityRanking, error) {
	if _, ok := addresspoint.Sources[city]; !ok {
		return domain.CommunityRanking{}, fmt.Errorf("%w: unknown city %q", ErrInvalidInput, city)
	}
	if district == "" {
		return domain.CommunityRanking{}, fmt.Errorf("%w: district is required", ErrInvalidInput)
	}
	through, err := service.through(ctx, city)
	if err != nil {
		return domain.CommunityRanking{}, err
	}
	from, to := through.AddDate(0, -11, 0), through.AddDate(0, 1, 0)
	records, ranked, err := service.repository.RankedCommunities(ctx, city, district, from, to, maxRankedCommunities)
	if err != nil {
		return domain.CommunityRanking{}, err
	}
	for index := range ranked {
		ranked[index].Community = describeCommunity(records[index])
		if ranked[index].MedianPricePerPing != nil {
			ranked[index].MedianPricePerPing = new(math.Round(*ranked[index].MedianPricePerPing))
		}
	}
	return domain.CommunityRanking{CityCode: city, District: district, From: from.Format("2006-01"), Through: through.Format("2006-01"),
		Communities: append([]domain.RankedCommunity{}, ranked...)}, nil
}

// ByDoor returns the community at an address's door number, the latest completed when a
// building was rebuilt, or nil.
func (service *CommunityService) ByDoor(ctx context.Context, address string) (*domain.CommunityRef, error) {
	records, err := service.repository.CommunitiesByDoor(ctx, address)
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return new(communityRef(records[0], 0)), nil
}

// Describe fills in the names and addresses of community links, dropping unknown ones.
func (service *CommunityService) Describe(ctx context.Context, refs []domain.CommunityRef) ([]domain.CommunityRef, error) {
	if len(refs) == 0 {
		return refs, nil
	}
	ids := make([]string, len(refs))
	for index, ref := range refs {
		ids[index] = ref.ID
	}
	records, err := service.repository.CommunitiesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	described := []domain.CommunityRef{}
	for _, ref := range refs {
		if index := slices.IndexFunc(records, func(record domain.CommunityRecord) bool { return record.ID == ref.ID }); index >= 0 {
			described = append(described, communityRef(records[index], ref.Sales))
		}
	}
	return described, nil
}

// Search finds communities for the search box: those named after a presale project
// matching text, then those at the door numbers among places. Communities without a
// location are left out, since the map cannot fly to them.
func (service *CommunityService) Search(ctx context.Context, text string, places []domain.Place) ([]domain.Place, error) {
	text = strings.ReplaceAll(twtext.Normalize(text), " ", "")
	var records []domain.CommunityRecord
	if utf8.RuneCountInString(text) >= 2 {
		named, err := service.repository.CommunitiesByName(ctx, text, searchCommunities)
		if err != nil {
			return nil, err
		}
		records = append(records, named...)
	}
	for _, place := range places {
		if place.Kind != domain.PlaceAddress {
			continue
		}
		atDoor, err := service.repository.CommunitiesByDoor(ctx, place.Label)
		if err != nil {
			return nil, err
		}
		if len(atDoor) > 0 {
			records = append(records, atDoor[0])
		}
	}
	found := []domain.Place{}
	seen := map[string]bool{}
	for _, record := range records {
		if seen[record.ID] || record.Lat == nil || record.Lng == nil {
			continue
		}
		seen[record.ID] = true
		community := describeCommunity(record)
		label := lvr.Cities[community.CityCode] + community.District + community.Address
		if community.Name != "" {
			label = community.Name + "（" + community.District + community.Address + "）"
		}
		found = append(found, domain.Place{Kind: domain.PlaceCommunity, Label: label, Lat: *record.Lat, Lng: *record.Lng, CommunityID: community.ID})
	}
	return found, nil
}

func communityRef(record domain.CommunityRecord, sales int) domain.CommunityRef {
	community := describeCommunity(record)
	return domain.CommunityRef{ID: community.ID, Name: community.Name, Address: community.Address, Sales: sales}
}

// through returns the city's last fully registered month, or last month before any import.
func (service *CommunityService) through(ctx context.Context, city string) (time.Time, error) {
	seasons, releases, err := service.repository.ImportedSalesByCity(ctx, city)
	if err != nil {
		return time.Time{}, err
	}
	if registered, known := registeredThrough(seasons, releases); known {
		return completeThrough(registered), nil
	}
	now := service.now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0), nil
}

// describeCommunity writes a community's door numbers without its city and district:
// 敬業三路162巷100號 for one, 敬業三路162巷 100–248號 for several.
func describeCommunity(record domain.CommunityRecord) domain.Community {
	community := record.Community
	street := strings.TrimPrefix(strings.TrimPrefix(record.Street, lvr.Cities[record.CityCode]), record.District)
	numbers := make([]string, 0, len(record.Doors))
	low, high := math.MaxInt, 0
	for _, door := range record.Doors {
		number := strings.TrimPrefix(door, record.Street)
		numbers = append(numbers, number)
		main, _, _ := strings.Cut(strings.TrimSuffix(number, "號"), "之")
		if value, err := strconv.Atoi(main); err == nil {
			low, high = min(low, value), max(high, value)
		}
	}
	switch {
	case len(numbers) == 0:
		community.Address = street
	case len(numbers) == 1 || low >= high:
		community.Address = street + numbers[0]
	default:
		community.Address = fmt.Sprintf("%s %d–%d號", street, low, high)
	}
	return community
}

// marketTransaction reports whether a sale counts in community prices: a house-and-land
// sale that is not special.
func marketTransaction(sale domain.Transaction) bool {
	return strings.HasPrefix(sale.TransactionType, "房地") && !sale.IsSpecial()
}

// marketPrices returns the price per ping of market sales from from to to (exclusive).
func marketPrices(sales []domain.Transaction, from, to time.Time) []float64 {
	var prices []float64
	for _, sale := range sales {
		if marketTransaction(sale) && !sale.TransactedAt.Before(from) && sale.TransactedAt.Before(to) {
			prices = append(prices, sale.PricePerPing)
		}
	}
	return prices
}

// communityLayout returns the median area and the most common number of rooms of market
// sales from from to to, with that number's share.
func communityLayout(sales []domain.Transaction, from, to time.Time) (*float64, *int, *float64) {
	var areas []float64
	rooms := map[int]int{}
	for _, sale := range sales {
		if marketTransaction(sale) && !sale.TransactedAt.Before(from) && sale.TransactedAt.Before(to) {
			areas = append(areas, sale.BuildingAreaPing)
			rooms[sale.Rooms]++
		}
	}
	if len(areas) == 0 {
		return nil, nil, nil
	}
	main := -1
	for count, homes := range rooms {
		if main < 0 || homes > rooms[main] || (homes == rooms[main] && count < main) {
			main = count
		}
	}
	return new(math.Round(median(areas)*10) / 10), &main, new(float64(rooms[main]) / float64(len(areas)))
}

// floorPremium returns the change in price per ping per floor above the ground floor of
// market sales from from to to: the slope of log price on floor after dividing each price
// by its year's median; nil with too few sales or floors.
func floorPremium(sales []domain.Transaction, from, to time.Time) *float64 {
	type point struct {
		floor int
		year  int
		price float64
	}
	var points []point
	yearPrices := map[int][]float64{}
	for _, sale := range sales {
		if !marketTransaction(sale) || sale.TransactedAt.Before(from) || !sale.TransactedAt.Before(to) {
			continue
		}
		if floor, ok := lvr.FloorNumber(sale.FloorLabel); ok {
			points = append(points, point{floor, sale.TransactedAt.Year(), sale.PricePerPing})
			yearPrices[sale.TransactedAt.Year()] = append(yearPrices[sale.TransactedAt.Year()], sale.PricePerPing)
		}
	}
	yearMedians := map[int]float64{}
	for year, prices := range yearPrices {
		yearMedians[year] = median(prices)
	}
	var xs, ys []float64
	upper := map[int]bool{}
	for _, item := range points {
		if item.floor >= 2 {
			xs = append(xs, float64(item.floor))
			ys = append(ys, math.Log(item.price/yearMedians[item.year]))
			upper[item.floor] = true
		}
	}
	if len(xs) < minFloorPremiumSales || len(upper) < minFloorPremiumFloors {
		return nil
	}
	var meanX, meanY float64
	for index := range xs {
		meanX += xs[index] / float64(len(xs))
		meanY += ys[index] / float64(len(ys))
	}
	var covariance, variance float64
	for index := range xs {
		covariance += (xs[index] - meanX) * (ys[index] - meanY)
		variance += (xs[index] - meanX) * (xs[index] - meanX)
	}
	return new(math.Exp(covariance/variance) - 1)
}

func mapKeys[V any](values map[int]V) []int {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
