package usecase

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type estimateRepositoryStub struct {
	sales         []domain.ComparableSale
	index         []domain.PriceIndexPoint
	parking       map[string]*float64 // by district, "" for the city
	parkingCounts map[string]int
	profile       domain.BuildingProfile
	building      string
	leases        []domain.LeaseCandidate
	rentIndex     []domain.RentPoint
	districts     []domain.RentalAggregate
}

func (*estimateRepositoryStub) ImportedRentalsByCity(context.Context, string) ([]string, []string, error) {
	return []string{"115S2"}, nil, nil
}

func (stub *estimateRepositoryStub) RentCandidates(context.Context, domain.EstimateQuery, float64, float64, float64, time.Time, time.Time) ([]domain.LeaseCandidate, error) {
	return stub.leases, nil
}

func (stub *estimateRepositoryStub) RentTrend(context.Context, domain.RentalQuery, string, time.Time, time.Time) ([]domain.RentPoint, error) {
	return stub.rentIndex, nil
}

func (stub *estimateRepositoryStub) RentalAggregates(context.Context, domain.RentalQuery, time.Time, time.Time) ([]domain.RentalAggregate, error) {
	return stub.districts, nil
}

func (*estimateRepositoryStub) ImportedSalesByCity(context.Context, string) ([]string, []string, error) {
	return []string{"115S2"}, nil, nil // complete through 2026-02
}

func (stub *estimateRepositoryStub) EstimateCandidates(context.Context, domain.EstimateQuery, float64, float64, float64, time.Time, time.Time) ([]domain.ComparableSale, error) {
	return stub.sales, nil
}

func (stub *estimateRepositoryStub) PriceIndex(context.Context, string, time.Time, time.Time) ([]domain.PriceIndexPoint, error) {
	return stub.index, nil
}

func (stub *estimateRepositoryStub) ParkingPrice(_ context.Context, _, district, _ string, _, _ time.Time) (*float64, int, error) {
	return stub.parking[district], stub.parkingCounts[district], nil
}

func (stub *estimateRepositoryStub) BuildingProfile(_ context.Context, building string, _, _ float64) (domain.BuildingProfile, error) {
	stub.building = building
	return stub.profile, nil
}

func month(value string) time.Time {
	parsed, _ := time.Parse("2006-01-02", value)
	return parsed
}

func comparable(id, address, floor string, price, distance float64, soldOn string) domain.ComparableSale {
	return domain.ComparableSale{ID: id, Address: address, AreaPing: 30, Age: new(20), FloorLabel: floor, PricePerPing: price, DistanceM: distance, TransactedAt: month(soldOn)}
}

func newEstimateService(stub *estimateRepositoryStub) *EstimateService {
	service := NewEstimateService(stub)
	service.now = func() time.Time { return time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC) }
	return service
}

var estimateQuery = domain.EstimateQuery{
	Address: "台北市大安區仁愛路四段27巷3號", Lat: 25.0389, Lng: 121.5453, Category: "住宅大樓",
	AreaPing: 30, Age: new(20), Floor: 5, TotalFloors: 12, Parking: true,
}

func TestEstimateWeighsTheSameBuildingAndAdjustsForTimeAndFloor(t *testing.T) {
	stub := &estimateRepositoryStub{
		sales: []domain.ComparableSale{
			comparable("same1", "台北市大安區仁愛路四段27巷3號三樓", "三層", 1_000_000, 0, "2026-02-10"),
			comparable("same2", "台北市大安區仁愛路四段27巷3號五樓", "五層", 1_010_000, 0, "2026-02-12"),
			comparable("same3", "台北市大安區仁愛路四段27巷3號七樓", "七層", 1_020_000, 0, "2026-02-14"),
			comparable("near1", "台北市大安區仁愛路四段27巷5號二樓", "二層", 1_200_000, 200, "2025-02-01"),
			comparable("near2", "台北市大安區仁愛路四段31號四樓", "四層", 900_000, 600, "2026-01-05"),
			comparable("shop", "台北市大安區仁愛路四段27巷43號", "一層", 2_300_000, 50, "2026-02-01"),
		},
		// Prices rose 10% from 2025-02 to 2026-02.
		index: []domain.PriceIndexPoint{
			{Month: month("2025-02-01"), Sales: 300, Median: new(1_000_000.0)},
			{Month: month("2026-01-01"), Sales: 300, Median: new(1_100_000.0)},
			{Month: month("2026-02-01"), Sales: 300, Median: new(1_100_000.0)},
		},
		parking:       map[string]*float64{"大安區": new(2_000_000.0), "": new(2_500_000.0)},
		parkingCounts: map[string]int{"大安區": 4, "": 50},
	}
	estimate, err := newEstimateService(stub).Estimate(context.Background(), estimateQuery)
	if err != nil {
		t.Fatal(err)
	}
	if !estimate.Sufficient || estimate.CityCode != "a" || estimate.District != "大安區" || estimate.AsOf != "2026-02" {
		t.Fatalf("unexpected estimate: %+v", estimate)
	}
	if len(estimate.Comparables) != 5 || estimate.ComparableSales != 5 || estimate.SameBuildingSales != 3 {
		t.Fatalf("the ground-floor shop should be left out: %+v", estimate.Comparables)
	}
	byID := map[string]domain.Comparable{}
	for _, item := range estimate.Comparables {
		byID[item.ID] = item
	}
	// 1.2M a year before, +10% for the index and +1.5% for three floors.
	if got := byID["near1"].AdjustedPricePerPing; math.Abs(got-1_200_000*1.1*1.015) > 1 {
		t.Fatalf("unexpected adjusted price %v", got)
	}
	if estimate.Comparables[0].SameBuilding != true || !byID["same2"].SameBuilding || byID["near2"].SameBuilding {
		t.Fatalf("same-building sales should lead: %+v", estimate.Comparables)
	}
	if *estimate.PricePerPing < 1_000_000 || *estimate.PricePerPing > 1_030_000 || estimate.Confidence != "high" {
		t.Fatalf("expected a high-confidence estimate near the building's sales: %v %s", *estimate.PricePerPing, estimate.Confidence)
	}
	// Four district sales are too few, so the city's parking price is used.
	if *estimate.ParkingPrice != 2_500_000 || *estimate.TotalPrice != *estimate.BuildingPrice+2_500_000 {
		t.Fatalf("unexpected parking: %v %v %v", *estimate.ParkingPrice, *estimate.BuildingPrice, *estimate.TotalPrice)
	}
	if *estimate.LowPrice != math.Round(*estimate.BuildingPrice*0.886+2_500_000) {
		t.Fatalf("the range should leave parking alone: %v", *estimate.LowPrice)
	}
}

func TestEstimateNeedsFiveComparables(t *testing.T) {
	stub := &estimateRepositoryStub{sales: []domain.ComparableSale{comparable("one", "台北市大安區仁愛路四段31號四樓", "四層", 900_000, 300, "2026-01-05")}}
	estimate, err := newEstimateService(stub).Estimate(context.Background(), estimateQuery)
	if err != nil || estimate.Sufficient || estimate.PricePerPing != nil || len(estimate.Comparables) != 1 {
		t.Fatalf("expected an insufficient estimate, got %+v, %v", estimate, err)
	}
}

func TestEstimateValidatesInput(t *testing.T) {
	for _, change := range []func(*domain.EstimateQuery){
		func(query *domain.EstimateQuery) { query.Category = "其他" },
		func(query *domain.EstimateQuery) { query.Lat = 22.6 },
		func(query *domain.EstimateQuery) { query.AreaPing = 1 },
		func(query *domain.EstimateQuery) { query.Age = new(150) },
		func(query *domain.EstimateQuery) { query.Floor = 20 },
	} {
		query := estimateQuery
		change(&query)
		if _, err := newEstimateService(&estimateRepositoryStub{}).Estimate(context.Background(), query); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: expected invalid input, got %v", query, err)
		}
	}
}

func TestEstimateBuildingUsesTheDoorNumber(t *testing.T) {
	stub := &estimateRepositoryStub{}
	if _, err := newEstimateService(stub).Building(context.Background(), "台北市大安區仁愛路四段27巷3號二樓", 25.0389, 121.5453); err != nil || stub.building != "台北市大安區仁愛路四段27巷3號" {
		t.Fatalf("unexpected building %q, %v", stub.building, err)
	}
	if _, err := newEstimateService(stub).Building(context.Background(), "台北市大安區仁愛路四段", 25.0389, 121.5453); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a road is not a building: %v", err)
	}
}

func TestWeightedQuantile(t *testing.T) {
	values, weights := []float64{10, 20, 30}, []float64{1, 1, 1}
	if got := weightedQuantile(values, weights, 0.5); got != 20 {
		t.Fatalf("median %v", got)
	}
	if got := weightedQuantile([]float64{10, 20}, []float64{1, 3}, 0.5); got <= 15 || got >= 20 {
		t.Fatalf("weighted median %v should lean to 20", got)
	}
}

func lease(id, address string, rentPerPing, distance float64, signed string) domain.LeaseCandidate {
	return domain.LeaseCandidate{ID: id, Address: address, AreaPing: 30, Age: new(20), MonthlyRent: rentPerPing * 30, RentPerPing: rentPerPing, DistanceM: distance, LeasedAt: month(signed)}
}

func TestEstimateAddsRentAndGrossYield(t *testing.T) {
	stub := &estimateRepositoryStub{
		sales: []domain.ComparableSale{
			comparable("s1", "台北市大安區仁愛路四段27巷3號三樓", "三層", 1_000_000, 0, "2026-02-10"),
			comparable("s2", "台北市大安區仁愛路四段27巷3號五樓", "五層", 1_000_000, 0, "2026-02-12"),
			comparable("s3", "台北市大安區仁愛路四段27巷3號七樓", "七層", 1_000_000, 0, "2026-02-14"),
			comparable("s4", "台北市大安區仁愛路四段31號四樓", "四層", 1_000_000, 300, "2026-01-05"),
			comparable("s5", "台北市大安區仁愛路四段33號四樓", "四層", 1_000_000, 400, "2026-01-05"),
		},
		leases: []domain.LeaseCandidate{
			lease("l1", "台北市大安區仁愛路四段27巷3號二樓", 1500, 0, "2026-02-01"),
			lease("l2", "台北市大安區仁愛路四段27巷3號九樓", 1520, 0, "2026-01-20"),
			lease("l3", "台北市大安區仁愛路四段31號三樓", 1480, 300, "2026-01-10"),
			lease("l4", "台北市大安區仁愛路四段35號三樓", 1510, 600, "2025-12-10"),
			lease("l5", "台北市大安區仁愛路四段37號三樓", 1490, 900, "2025-11-10"),
		},
		districts: []domain.RentalAggregate{{District: "大安區", Rentals: 40, MedianRentPerPing: new(1400.0), Sales: 60, MedianPricePerPing: new(1_000_000.0)}},
	}
	query := estimateQuery
	query.Parking = false
	estimate, err := newEstimateService(stub).Estimate(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	rent := estimate.Rent
	if rent == nil || !rent.Sufficient || rent.Confidence != "high" || rent.SameBuildingLeases != 2 || rent.LeaseCount != 5 {
		t.Fatalf("unexpected rent: %+v", rent)
	}
	// About 1,500 per ping on 30 ping, rounded to NT$100.
	if *rent.MonthlyRent < 44_500 || *rent.MonthlyRent > 45_600 || math.Mod(*rent.MonthlyRent, 100) != 0 {
		t.Fatalf("unexpected monthly rent %v", *rent.MonthlyRent)
	}
	if want := *rent.MonthlyRent * 12 / *estimate.BuildingPrice; rent.GrossYield == nil || math.Abs(*rent.GrossYield-want) > 1e-12 {
		t.Fatalf("unexpected yield %v", rent.GrossYield)
	}
	if rent.DistrictGrossYield == nil || math.Abs(*rent.DistrictGrossYield-0.0168) > 1e-9 {
		t.Fatalf("unexpected district yield %v", rent.DistrictGrossYield)
	}
}

func TestEstimateSkipsRentForHouses(t *testing.T) {
	stub := &estimateRepositoryStub{sales: []domain.ComparableSale{
		comparable("s1", "台北市大安區仁愛路四段1號", "全", 1_000_000, 10, "2026-02-10"),
		comparable("s2", "台北市大安區仁愛路四段3號", "全", 1_000_000, 20, "2026-02-10"),
		comparable("s3", "台北市大安區仁愛路四段5號", "全", 1_000_000, 30, "2026-02-10"),
		comparable("s4", "台北市大安區仁愛路四段7號", "全", 1_000_000, 40, "2026-02-10"),
		comparable("s5", "台北市大安區仁愛路四段9號", "全", 1_000_000, 50, "2026-02-10"),
	}}
	query := estimateQuery
	query.Category, query.Floor, query.TotalFloors, query.Parking = "透天厝", 0, 0, false
	estimate, err := newEstimateService(stub).Estimate(context.Background(), query)
	if err != nil || estimate.Rent == nil || estimate.Rent.Supported || estimate.Rent.Sufficient {
		t.Fatalf("houses get no rent estimate: %+v, %v", estimate.Rent, err)
	}
}
