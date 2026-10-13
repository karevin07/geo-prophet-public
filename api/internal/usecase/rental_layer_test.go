package usecase

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

func TestRentLayerAdjustsRentsAndComputesYields(t *testing.T) {
	// Leases are registered through 115S2 (registered by 2026-06-10): complete through 2026-02.
	month := func(year int, month time.Month) time.Time { return time.Date(year, month, 10, 0, 0, 0, 0, time.UTC) }
	lease := func(lat float64, at time.Time, rent float64, rooms int, category string) domain.MappedLease {
		return domain.MappedLease{CityCode: "a", Lat: lat, Lng: 121.5435, LeasedAt: at, RentPerPing: rent,
			MonthlyRent: int64(rent * 25), AreaPing: 25, Rooms: rooms, Category: category}
	}
	daan := 25.0330
	repository := &rentalRepositoryStub{
		saleSeasons: []string{"115S2"}, rentalSeasons: []string{"115S2"},
		// The city's rent rose 10% from 2025-02 to 2026-02.
		points: map[string][]domain.RentPoint{"": {
			{Month: "2025-02", Rentals: 300, MedianRentPerPing: new(1000.0)},
			{Month: "2026-02", Rentals: 300, MedianRentPerPing: new(1100.0)},
			{Month: "2026-01", Rentals: 10, MedianRentPerPing: new(5000.0)}, // too few leases to index
		}},
		leases: []domain.MappedLease{
			lease(daan, month(2025, 2), 1000, 2, "住宅大樓"), // adjusted to 1,100
			lease(daan, month(2026, 2), 1200, 2, "住宅大樓"),
			lease(daan, month(2026, 1), 1300, 3, "華廈"), // its month has no index, kept
			lease(daan, month(2026, 2), 1000, 0, "住宅大樓"),
			lease(daan, month(2026, 2), 1150, 3, "公寓"),
			// A cell 2 km north with too few leases.
			lease(daan+0.02, month(2026, 2), 900, 1, "公寓"),
		},
		sales: []domain.MappedSale{
			{Lat: daan, Lng: 121.5435, PricePerPing: 700_000}, {Lat: daan, Lng: 121.5435, PricePerPing: 720_000},
			{Lat: daan, Lng: 121.5435, PricePerPing: 690_000}, {Lat: daan, Lng: 121.5435, PricePerPing: 710_000},
			{Lat: daan, Lng: 121.5435, PricePerPing: 690_000},
			{Lat: daan + 0.02, Lng: 121.5435, PricePerPing: 500_000},
		},
	}
	bounds := domain.Bounds{West: 121.5, South: 25.0, East: 121.6, North: 25.1}
	filter := domain.TransactionFilter{MaxPrice: math.MaxInt64, MaxArea: math.MaxFloat64, Bounds: &bounds, Categories: []string{"住宅大樓", "華廈", "公寓", "其他"}}
	layer, err := NewRentalService(repository).Layer(context.Background(), filter, 9)
	if err != nil {
		t.Fatal(err)
	}
	if layer.From != "2023-03" || layer.SalesFrom != "2025-03" || layer.Through != "2026-02" || !repository.to.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected window %s / %s – %s", layer.From, layer.SalesFrom, layer.Through)
	}
	if len(repository.filter.Categories) != 3 {
		t.Fatalf("其他 should be dropped: %v", repository.filter.Categories)
	}
	if len(layer.Cells) != 1 {
		t.Fatalf("only the cell with 5 leases should show: %+v", layer.Cells)
	}
	cell := layer.Cells[0]
	// Rents 1,100, 1,200, 1,300, 1,000, 1,150.
	if cell.Rentals != 5 || cell.MedianRentPerPing != 1150 || cell.MedianMonthlyRent != 1150*25 || cell.MedianAreaPing != 25 {
		t.Fatalf("unexpected rents: %+v", cell)
	}
	if cell.MainRooms == nil || *cell.MainRooms != 2 {
		t.Fatalf("2 and 3 rooms tie; the smaller wins: %v", cell.MainRooms)
	}
	if len(cell.Categories) != 3 || cell.Categories[0] != (domain.CategoryShare{Category: "住宅大樓", Share: 0.6}) {
		t.Fatalf("unexpected categories: %+v", cell.Categories)
	}
	if cell.Sales != 5 || cell.MedianPricePerPing == nil || *cell.MedianPricePerPing != 700_000 ||
		cell.GrossYield == nil || math.Abs(*cell.GrossYield-1150*12/700_000.0) > 1e-12 {
		t.Fatalf("unexpected yield: %+v", cell)
	}
}

func TestRentLayerValidates(t *testing.T) {
	service := NewRentalService(&rentalRepositoryStub{})
	bounds := domain.Bounds{West: 121.5, South: 25.0, East: 121.6, North: 25.1}
	for name, filter := range map[string]domain.TransactionFilter{
		"no bbox":   {MaxPrice: 1, MaxArea: 1},
		"bad area":  {MaxPrice: 1, MinArea: 2, MaxArea: 1, Bounds: &bounds},
		"bad types": {MaxPrice: 1, MaxArea: 1, Bounds: &bounds, Categories: []string{"店面"}},
	} {
		if _, err := service.Layer(context.Background(), filter, 9); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if _, err := service.Layer(context.Background(), domain.TransactionFilter{MaxPrice: 1, MaxArea: 1, Bounds: &bounds}, 6); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("resolution 6: got %v", err)
	}
	layer, err := service.Layer(context.Background(), domain.TransactionFilter{MaxPrice: 1, MaxArea: 1, Bounds: &bounds, Categories: []string{"其他"}}, 9)
	if err != nil || len(layer.Cells) != 0 || layer.Cells == nil {
		t.Fatalf("only 其他: %+v, %v", layer, err)
	}
}
