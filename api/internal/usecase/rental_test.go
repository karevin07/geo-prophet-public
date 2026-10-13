package usecase

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type rentalRepositoryStub struct {
	saleSeasons, rentalSeasons []string
	rows                       []domain.RentalAggregate
	from, to                   time.Time
	first, last                time.Time
	points                     map[string][]domain.RentPoint // by district, "" for the city
	stationFound               bool
	districtMedian             *float64
	ringTrends                 [][]domain.RentPoint
	leases                     []domain.MappedLease
	sales                      []domain.MappedSale
	filter                     domain.TransactionFilter
}

func (stub *rentalRepositoryStub) MappedLeases(_ context.Context, filter domain.TransactionFilter, from, to time.Time, emit func(domain.MappedLease) error) error {
	stub.filter, stub.from, stub.to = filter, from, to
	for _, lease := range stub.leases {
		if err := emit(lease); err != nil {
			return err
		}
	}
	return nil
}

func (stub *rentalRepositoryStub) MappedSales(_ context.Context, _ domain.TransactionFilter, _, _ time.Time, emit func(domain.MappedSale) error) error {
	for _, sale := range stub.sales {
		if err := emit(sale); err != nil {
			return err
		}
	}
	return nil
}

func (stub *rentalRepositoryStub) ImportedSalesByCity(context.Context, string) ([]string, []string, error) {
	return stub.saleSeasons, nil, nil
}

func (stub *rentalRepositoryStub) ImportedRentalsByCity(context.Context, string) ([]string, []string, error) {
	return stub.rentalSeasons, nil, nil
}

func (stub *rentalRepositoryStub) RentalAggregates(_ context.Context, _ domain.RentalQuery, from, to time.Time) ([]domain.RentalAggregate, error) {
	stub.from, stub.to = from, to
	return stub.rows, nil
}

func (stub *rentalRepositoryStub) RentTrend(_ context.Context, _ domain.RentalQuery, district string, first, last time.Time) ([]domain.RentPoint, error) {
	stub.first, stub.last = first, last
	return append([]domain.RentPoint{}, stub.points[district]...), nil
}

func (stub *rentalRepositoryStub) StationDistrict(context.Context, string) (string, string, bool, error) {
	return "a", "中山區", stub.stationFound, nil
}

func (stub *rentalRepositoryStub) StationRentRings(_ context.Context, _, _ string, _ domain.RentalQuery, from, to time.Time) ([]domain.RentalAggregate, *float64, error) {
	stub.from, stub.to = from, to
	return stub.rows, stub.districtMedian, nil
}

func (stub *rentalRepositoryStub) StationRentTrends(_ context.Context, _ string, _ domain.RentalQuery, first, last time.Time) ([][]domain.RentPoint, error) {
	stub.first, stub.last = first, last
	return stub.ringTrends, nil
}

func TestRentalCompareDistrictsComputesGrossYield(t *testing.T) {
	repository := &rentalRepositoryStub{
		// Sales are registered through 115S3 but leases only through 115S2 (registered by
		// 2026-06-10), so the window ends with the last month complete for leases: 2026-02.
		saleSeasons: []string{"115S2", "115S3"}, rentalSeasons: []string{"115S2"},
		rows: []domain.RentalAggregate{
			{Rentals: 500, MedianRentPerPing: new(1000.0), Sales: 900, MedianPricePerPing: new(600_000.0)},
			{District: "大安區", Rentals: 40, MedianRentPerPing: new(1500.0), MedianMonthlyRent: new(45_000.0), Sales: 30, MedianPricePerPing: new(1_000_000.0)},
			{District: "北投區", Rentals: 12, MedianRentPerPing: new(900.0), Sales: 80, MedianPricePerPing: new(550_000.0)},
		},
	}
	report, err := NewRentalService(repository).CompareDistricts(context.Background(), domain.RentalQuery{CityCode: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if report.From != "2025-03" || report.Through != "2026-02" || !repository.to.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected window %s–%s (to %v)", report.From, report.Through, repository.to)
	}
	if report.City.GrossYield == nil || math.Abs(*report.City.GrossYield-0.02) > 1e-9 {
		t.Fatalf("unexpected city yield: %+v", report.City)
	}
	if len(report.Districts) != 2 || report.Districts[0].District != "大安區" {
		t.Fatalf("districts should be sorted by leases: %+v", report.Districts)
	}
	daan, beitou := report.Districts[0], report.Districts[1]
	if daan.GrossYield == nil || math.Abs(*daan.GrossYield-0.018) > 1e-9 {
		t.Fatalf("unexpected 大安區 yield: %+v", daan)
	}
	// 12 leases are too few: the rent and yield are hidden, the sale price is not.
	if beitou.MedianRentPerPing != nil || beitou.GrossYield != nil || beitou.MedianPricePerPing == nil {
		t.Fatalf("北投區 should hide its rent: %+v", beitou)
	}
}

func TestRentalCompareDistrictsWithoutLeasesIsEmpty(t *testing.T) {
	repository := &rentalRepositoryStub{saleSeasons: []string{"115S2"}}
	report, err := NewRentalService(repository).CompareDistricts(context.Background(), domain.RentalQuery{CityCode: "a"})
	if err != nil || report.Through != "" || len(report.Districts) != 0 || !repository.to.IsZero() {
		t.Fatalf("expected an empty report without querying, got %+v, %v", report, err)
	}
}

func TestRentalCompareDistrictsValidatesInput(t *testing.T) {
	for _, query := range []domain.RentalQuery{
		{CityCode: "e"},
		{CityCode: "a", Category: "其他"},
		{CityCode: "a", MinArea: 40, MaxArea: 20},
		{CityCode: "a", MaxArea: 500},
	} {
		if _, err := NewRentalService(&rentalRepositoryStub{}).CompareDistricts(context.Background(), query); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: expected invalid input, got %v", query, err)
		}
	}
}

func TestRentalTrendMarksProvisionalMonthsAndSmallSamples(t *testing.T) {
	repository := &rentalRepositoryStub{
		saleSeasons: []string{"115S2"}, rentalSeasons: []string{"115S2"},
		points: map[string][]domain.RentPoint{
			"":    {{Month: "2026-02", Rentals: 300, MedianRentPerPing: new(1500.0)}, {Month: "2026-06", Rentals: 120, MedianRentPerPing: new(1550.0)}},
			"中山區": {{Month: "2026-02", Rentals: 6, MedianRentPerPing: new(2100.0)}},
		},
	}
	trend, err := NewRentalService(repository).Trend(context.Background(), domain.RentalQuery{CityCode: "a", IncludeSocialHousing: true}, "中山區")
	if err != nil {
		t.Fatal(err)
	}
	// Leases are registered through 2026-06-10: trends run to June, complete through February.
	if trend.Through != "2026-02" || !repository.last.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) ||
		!repository.first.Equal(time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected window: through %s, %v–%v", trend.Through, repository.first, repository.last)
	}
	if trend.City[0].Provisional || !trend.City[1].Provisional || trend.City[1].MedianRentPerPing == nil {
		t.Fatalf("unexpected city points: %+v", trend.City)
	}
	if len(trend.Districts) != 1 || trend.Districts[0].MedianRentPerPing != nil {
		t.Fatalf("6 leases should hide the district median: %+v", trend.Districts)
	}
}

func TestRentalStationRentComparesWithTheDistrict(t *testing.T) {
	repository := &rentalRepositoryStub{
		saleSeasons: []string{"115S2"}, rentalSeasons: []string{"115S2"}, stationFound: true, districtMedian: new(2000.0),
		rows: []domain.RentalAggregate{
			{Rentals: 15, MedianRentPerPing: new(2300.0), Sales: 40, MedianPricePerPing: new(900_000.0)},
			{Rentals: 60, MedianRentPerPing: new(2200.0), MedianMonthlyRent: new(48_000.0), Sales: 120, MedianPricePerPing: new(880_000.0)},
			{Rentals: 140, MedianRentPerPing: new(2100.0), Sales: 300, MedianPricePerPing: new(860_000.0)},
		},
		ringTrends: [][]domain.RentPoint{nil, {{Month: "2026-06", Rentals: 30, MedianRentPerPing: new(2250.0)}}, nil},
	}
	rent, err := NewRentalService(repository).StationRent(context.Background(), "TRTC-R11", false)
	if err != nil {
		t.Fatal(err)
	}
	if rent.District != "中山區" || len(rent.Rings) != 3 || rent.Rings[1].Radius != 500 {
		t.Fatalf("unexpected rings: %+v", rent)
	}
	if rent.Rings[0].MedianRentPerPing != nil || rent.Rings[0].GrossYield != nil || rent.Rings[0].RelativeToDistrict != nil {
		t.Fatalf("15 leases should hide the 300 m rent: %+v", rent.Rings[0])
	}
	ring := rent.Rings[1]
	if ring.RelativeToDistrict == nil || math.Abs(*ring.RelativeToDistrict-0.1) > 1e-9 || ring.GrossYield == nil || math.Abs(*ring.GrossYield-0.03) > 1e-9 {
		t.Fatalf("unexpected 500 m figures: %+v", ring)
	}
	if len(ring.Trend) != 1 || !ring.Trend[0].Provisional || rent.Rings[0].Trend == nil {
		t.Fatalf("unexpected trends: %+v / %+v", ring.Trend, rent.Rings[0].Trend)
	}
}

func TestRentalStationRentUnknownStation(t *testing.T) {
	if _, err := NewRentalService(&rentalRepositoryStub{}).StationRent(context.Background(), "nope", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestRentalTrendWithoutSocialHousingStartsWhenItIsKnown(t *testing.T) {
	repository := &rentalRepositoryStub{saleSeasons: []string{"115S2"}, rentalSeasons: []string{"115S2"}}
	if _, err := NewRentalService(repository).Trend(context.Background(), domain.RentalQuery{CityCode: "a"}, ""); err != nil {
		t.Fatal(err)
	}
	if !repository.first.Equal(time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("trend should start in 2023-12, got %v", repository.first)
	}
}
