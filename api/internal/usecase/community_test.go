package usecase

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type communityRepositoryStub struct {
	record        domain.CommunityRecord
	found         bool
	sales         []domain.Transaction
	district      domain.CommunityStats
	districtFrom  time.Time
	districtTo    time.Time
	yearMedians   map[int]float64
	leases        int
	rentPerPing   *float64
	monthlyRent   *float64
	leaseFrom     time.Time
	nearby        []domain.CommunityRecord
	nearbyStats   []domain.NearbyCommunity
	byDoor        map[string][]domain.CommunityRecord
	byName        []domain.CommunityRecord
	byID          []domain.CommunityRecord
	rankingWindow [2]time.Time
}

func (*communityRepositoryStub) ImportedSalesByCity(context.Context, string) ([]string, []string, error) {
	return []string{"115S2"}, nil, nil // complete through 2026-02
}

func (stub *communityRepositoryStub) Community(context.Context, string) (domain.CommunityRecord, bool, error) {
	return stub.record, stub.found, nil
}

func (stub *communityRepositoryStub) CommunitiesByID(context.Context, []string) ([]domain.CommunityRecord, error) {
	return stub.byID, nil
}

func (stub *communityRepositoryStub) CommunitiesByDoor(_ context.Context, address string) ([]domain.CommunityRecord, error) {
	return stub.byDoor[address], nil
}

func (stub *communityRepositoryStub) CommunitiesByName(context.Context, string, int) ([]domain.CommunityRecord, error) {
	return stub.byName, nil
}

func (stub *communityRepositoryStub) CommunitySales(context.Context, string) ([]domain.Transaction, error) {
	return stub.sales, nil
}

func (stub *communityRepositoryStub) DistrictYearMedians(context.Context, string, string, string, time.Time) (map[int]float64, error) {
	return stub.yearMedians, nil
}

func (stub *communityRepositoryStub) DistrictMedian(_ context.Context, _, _, _ string, from, to time.Time) (domain.CommunityStats, error) {
	stub.districtFrom, stub.districtTo = from, to
	return stub.district, nil
}

func (stub *communityRepositoryStub) CommunityLeases(_ context.Context, _ string, from time.Time) (int, *float64, *float64, error) {
	stub.leaseFrom = from
	return stub.leases, stub.rentPerPing, stub.monthlyRent, nil
}

func (stub *communityRepositoryStub) NearbyCommunities(context.Context, string, float64, int, int, time.Time, time.Time, int) ([]domain.CommunityRecord, []domain.NearbyCommunity, error) {
	return stub.nearby, stub.nearbyStats, nil
}

func (stub *communityRepositoryStub) RankedCommunities(_ context.Context, _, _ string, from, to time.Time, _ int) ([]domain.CommunityRecord, []domain.RankedCommunity, error) {
	stub.rankingWindow = [2]time.Time{from, to}
	return []domain.CommunityRecord{stub.record}, []domain.RankedCommunity{{RecentSales: 4, MedianPricePerPing: new(712345.6)}}, nil
}

func newCommunityService(stub *communityRepositoryStub) *CommunityService {
	service := NewCommunityService(stub)
	service.now = func() time.Time { return time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC) }
	return service
}

func communityRecord(street string, doors ...string) domain.CommunityRecord {
	record := domain.CommunityRecord{Street: "台北市中山區" + street}
	record.ID, record.CityCode, record.District, record.Category = "c1", "a", "中山區", "住宅大樓"
	for _, door := range doors {
		record.Doors = append(record.Doors, record.Street+door)
	}
	return record
}

func communitySale(day string, price float64, floor string) domain.Transaction {
	date, _ := time.Parse("2006-01-02", day)
	return domain.Transaction{TransactionType: "房地(土地+建物)", TransactedAt: date, PricePerPing: price, FloorLabel: floor,
		BuildingAreaPing: 30, Rooms: 2}
}

func TestDescribeCommunity(t *testing.T) {
	for _, test := range []struct {
		record domain.CommunityRecord
		want   string
	}{
		{communityRecord("敬業三路162巷", "100號"), "敬業三路162巷100號"},
		{communityRecord("敬業三路162巷", "100號", "102號", "248號", "99之1號"), "敬業三路162巷 99–248號"},
		{communityRecord("興南街", "101號", "101之1號"), "興南街101號"},
	} {
		if got := describeCommunity(test.record).Address; got != test.want {
			t.Errorf("%v: got %q, want %q", test.record.Doors, got, test.want)
		}
	}
}

func TestCommunityDetail(t *testing.T) {
	stub := &communityRepositoryStub{
		record: communityRecord("敬業三路162巷", "100號", "102號"), found: true,
		district:    domain.CommunityStats{Sales: 400, MedianPricePerPing: new(800_000.0)},
		yearMedians: map[int]float64{2025: 780_000.4, 2026: 810_000},
		leases:      4, rentPerPing: new(1_000.4), monthlyRent: new(30_000.0),
	}
	stub.record.Lat, stub.record.Lng = new(25.07), new(121.55)
	stub.nearby = []domain.CommunityRecord{communityRecord("樂群二路", "160號")}
	stub.nearbyStats = []domain.NearbyCommunity{{DistanceM: 290, RecentSales: 5, MedianPricePerPing: new(660_000.4)}}
	// Five sales in the 12 months to February 2026 and five the year before, one special
	// sale, one after the last complete month, and a parking space alone.
	for index, day := range []string{"2025-03-02", "2025-06-01", "2025-09-01", "2025-12-01", "2025-12-31"} {
		stub.sales = append(stub.sales, communitySale(day, 600_000+float64(index)*10_000, "五層"))
		stub.sales = append(stub.sales, communitySale(day[:3]+"4"+day[4:], 500_000, "三層"))
	}
	special := communitySale("2025-10-01", 100_000, "二層")
	special.SpecialReasons = []string{"親友交易"}
	parking := communitySale("2025-10-01", 100_000, "二層")
	parking.TransactionType = "車位"
	stub.sales = append(stub.sales, special, parking, communitySale("2026-04-01", 900_000, "十層"))

	detail, err := newCommunityService(stub).Community(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.From != "2025-03" || detail.Through != "2026-02" || detail.RecentSales != 5 || *detail.MedianPricePerPing != 620_000 {
		t.Fatalf("unexpected recent figures: %+v", detail)
	}
	if !stub.districtFrom.Equal(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)) || !stub.districtTo.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("district window %v–%v", stub.districtFrom, stub.districtTo)
	}
	if math.Abs(*detail.PriceYoY-0.24) > 1e-9 || *detail.DistrictPricePerPing != 800_000 || math.Abs(*detail.Premium+0.225) > 1e-9 {
		t.Fatalf("unexpected comparisons: yoy %v premium %v", *detail.PriceYoY, *detail.Premium)
	}
	// 2026 runs past the last complete month; the district has no 2024 median.
	if len(detail.Years) != 3 || detail.Years[0].Year != 2024 || detail.Years[0].DistrictMedianPricePerPing != nil ||
		detail.Years[1].Provisional || *detail.Years[1].DistrictMedianPricePerPing != 780_000 || !detail.Years[2].Provisional {
		t.Fatalf("unexpected years: %+v", detail.Years)
	}
	if detail.Rent == nil || detail.Rent.From != "2023-03" || *detail.Rent.MedianRentPerPing != 1_000 || detail.Rent.GrossYield == nil {
		t.Fatalf("unexpected rent: %+v", detail.Rent)
	}
	if len(detail.Nearby) != 1 || detail.Nearby[0].Address != "樂群二路160號" || *detail.Nearby[0].MedianPricePerPing != 660_000 ||
		math.Abs(*detail.Nearby[0].Difference-(660_000.0/620_000-1)) > 1e-9 {
		t.Fatalf("unexpected nearby: %+v", detail.Nearby)
	}
	if len(detail.Sales) != len(stub.sales) {
		t.Fatalf("expected every sale, got %d", len(detail.Sales))
	}

	stub.found = false
	if _, err := newCommunityService(stub).Community(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCommunityWithFewSales(t *testing.T) {
	stub := &communityRepositoryStub{record: communityRecord("敬業三路162巷", "100號"), found: true, leases: 2, rentPerPing: new(900.0)}
	stub.sales = []domain.Transaction{communitySale("2025-12-01", 600_000, "五層"), communitySale("2026-01-01", 700_000, "六層")}
	detail, err := newCommunityService(stub).Community(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.RecentSales != 2 || detail.MedianPricePerPing != nil || detail.PriceYoY != nil || detail.Premium != nil || detail.FloorPremium != nil {
		t.Fatalf("expected no medians: %+v", detail)
	}
	if detail.Rent.Leases != 2 || detail.Rent.MedianRentPerPing != nil || detail.Nearby == nil || len(detail.Nearby) != 0 {
		t.Fatalf("unexpected rent or nearby: %+v %+v", detail.Rent, detail.Nearby)
	}
}

func TestCommunityFloorPremium(t *testing.T) {
	var sales []domain.Transaction
	labels := map[int]string{1: "一層", 2: "二層", 4: "四層", 8: "八層"}
	for _, year := range []string{"2023", "2025"} {
		scale := map[string]float64{"2023": 1, "2025": 1.2}[year]
		for floor, label := range labels {
			price := 500_000 * scale * math.Pow(1.01, float64(floor))
			if floor == 1 {
				price *= 1.3 // shops on the ground floor are left out of the slope
			}
			sales = append(sales, communitySale(year+"-05-01", price, label), communitySale(year+"-06-01", price, label))
		}
	}
	premium := floorPremium(sales, time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	if premium == nil || math.Abs(*premium-0.01) > 1e-9 {
		t.Fatalf("premium = %v, want 1%%", premium)
	}
	if premium := floorPremium(sales[:6], time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)); premium != nil {
		t.Fatalf("expected no premium from 6 sales, got %v", *premium)
	}
}

func TestCommunityRanking(t *testing.T) {
	stub := &communityRepositoryStub{record: communityRecord("敬業三路162巷", "100號")}
	ranking, err := newCommunityService(stub).Ranking(context.Background(), "a", "中山區")
	if err != nil {
		t.Fatal(err)
	}
	if ranking.Through != "2026-02" || len(ranking.Communities) != 1 || *ranking.Communities[0].MedianPricePerPing != 712_346 ||
		ranking.Communities[0].Address != "敬業三路162巷100號" || !stub.rankingWindow[1].Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected ranking: %+v", ranking)
	}
	for _, args := range [][2]string{{"x", "中山區"}, {"a", ""}} {
		if _, err := newCommunityService(stub).Ranking(context.Background(), args[0], args[1]); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%v: expected invalid input, got %v", args, err)
		}
	}
}

func TestCommunitySearch(t *testing.T) {
	named := communityRecord("裕民六路", "111號", "115號")
	named.ID, named.Name, named.Lat, named.Lng = "named", "吾上琚", new(25.1), new(121.5)
	atDoor := communityRecord("敬業三路162巷", "100號")
	atDoor.ID, atDoor.Lat, atDoor.Lng = "door", new(25.07), new(121.55)
	unlocated := communityRecord("農安街", "6號")
	unlocated.ID, unlocated.Name = "unlocated", "吾上琚二"
	stub := &communityRepositoryStub{
		byName: []domain.CommunityRecord{named, unlocated},
		byDoor: map[string][]domain.CommunityRecord{"台北市中山區敬業三路162巷100號": {atDoor}},
	}
	places, err := newCommunityService(stub).Search(context.Background(), "吾上", []domain.Place{
		{Kind: domain.PlaceAddress, Label: "台北市中山區敬業三路162巷100號"},
		{Kind: domain.PlaceRoad, Label: "台北市中山區敬業三路"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(places) != 2 || places[0].Label != "吾上琚（中山區裕民六路 111–115號）" || places[0].CommunityID != "named" ||
		places[1].Label != "台北市中山區敬業三路162巷100號" || places[1].Kind != domain.PlaceCommunity {
		t.Fatalf("unexpected places: %+v", places)
	}
}

func TestCommunityDescribe(t *testing.T) {
	known := communityRecord("敬業三路162巷", "100號")
	stub := &communityRepositoryStub{byID: []domain.CommunityRecord{known}}
	refs, err := newCommunityService(stub).Describe(context.Background(), []domain.CommunityRef{{ID: "gone", Sales: 9}, {ID: "c1", Sales: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].ID != "c1" || refs[0].Sales != 3 || refs[0].Address != "敬業三路162巷100號" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}
