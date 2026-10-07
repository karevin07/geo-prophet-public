package usecase

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type presaleRepositoryStub struct {
	project              domain.PresaleProject
	found                bool
	units                []domain.Presale
	query                domain.PresaleProjectQuery
	recentFrom           time.Time
	resales, newResales  int
	resaleMedian, newMed *float64
	resaleFrom, resaleTo time.Time
	bounds               *domain.Bounds
}

func (*presaleRepositoryStub) ImportedSalesByCity(context.Context, string) ([]string, []string, error) {
	return []string{"115S2"}, nil, nil // complete through 2026-02
}

func (stub *presaleRepositoryStub) PresaleProjects(_ context.Context, query domain.PresaleProjectQuery, recentFrom time.Time) ([]domain.PresaleProject, error) {
	stub.query, stub.recentFrom = query, recentFrom
	return []domain.PresaleProject{stub.project}, nil
}

func (stub *presaleRepositoryStub) NearbyPresaleProjects(_ context.Context, near domain.NearPoint, _, _ time.Time) ([]domain.PresaleProject, error) {
	stub.query.Near = &near
	return []domain.PresaleProject{stub.project}, nil
}

func (stub *presaleRepositoryStub) BoundedPresaleProjects(_ context.Context, bounds domain.Bounds, _, _ time.Time) ([]domain.PresaleProject, error) {
	stub.bounds = &bounds
	return []domain.PresaleProject{stub.project}, nil
}

func (stub *presaleRepositoryStub) PresaleProject(context.Context, string, time.Time) (domain.PresaleProject, bool, error) {
	return stub.project, stub.found, nil
}

func (stub *presaleRepositoryStub) PresaleUnits(context.Context, string) ([]domain.Presale, error) {
	return stub.units, nil
}

func (stub *presaleRepositoryStub) ResalesNearby(_ context.Context, _, _, _ float64, _ int, from, to time.Time) (int, *float64, int, *float64, error) {
	stub.resaleFrom, stub.resaleTo = from, to
	return stub.resales, stub.resaleMedian, stub.newResales, stub.newMed, nil
}

func newPresaleService(stub *presaleRepositoryStub) *PresaleService {
	service := NewPresaleService(stub)
	service.now = func() time.Time { return time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC) }
	return service
}

func presaleUnit(floor string, price float64, signed string) domain.Presale {
	date, _ := time.Parse("2006-01-02", signed)
	return domain.Presale{Transaction: domain.Transaction{TransactionType: "房地(土地+建物)+車位", FloorLabel: floor, PricePerPing: price, TransactedAt: date}}
}

func TestPresaleProjectsWindow(t *testing.T) {
	stub := &presaleRepositoryStub{}
	projects, err := newPresaleService(stub).Projects(context.Background(), "f", "板橋區", 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if projects.From != "2024-07" || stub.query.District != "板橋區" || !stub.recentFrom.Equal(time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected window: %+v from %v", stub.query, stub.recentFrom)
	}
	for _, months := range []int{-1, 241} {
		if _, err := newPresaleService(stub).Projects(context.Background(), "f", "", months, nil, nil); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("months %d: expected invalid input, got %v", months, err)
		}
	}
	if _, err := newPresaleService(stub).Projects(context.Background(), "e", "", 0, nil, nil); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("unknown city should be invalid: %v", err)
	}
	if _, err := newPresaleService(stub).Projects(context.Background(), "", "", 0, &domain.NearPoint{Lat: 25.05, Lng: 121.52, RadiusM: 800}, nil); err != nil || stub.query.Near == nil {
		t.Errorf("a point should list nearby projects: %v", err)
	}
	if _, err := newPresaleService(stub).Projects(context.Background(), "", "", 0, nil, &domain.Bounds{West: 121.5, South: 25.0, East: 121.6, North: 25.1}); err != nil || stub.bounds == nil {
		t.Errorf("bounds should list the projects on a map: %v", err)
	}
	if _, err := newPresaleService(stub).Projects(context.Background(), "", "", 0, nil, &domain.Bounds{West: 120, South: 24, East: 122, North: 26}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("a 2° box should be invalid: %v", err)
	}
	if _, err := newPresaleService(stub).Projects(context.Background(), "", "", 0, &domain.NearPoint{Lat: 25.05, Lng: 121.52, RadiusM: 5000}, nil); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("a 5 km radius should be invalid: %v", err)
	}
}

func TestPresaleProjectSummarisesValidSales(t *testing.T) {
	cancelled := presaleUnit("五層", 9_000_000, "2026-03-01")
	cancelled.Cancellation = "1150401全部解約"
	special := presaleUnit("五層", 9_000_000, "2026-03-02")
	special.SpecialReasons = []string{"related_party"}
	parking := presaleUnit("地下二層", 9_000_000, "2026-03-03")
	parking.TransactionType = "車位"
	stub := &presaleRepositoryStub{
		found:   true,
		project: domain.PresaleProject{Key: "a|中山區|測試", CityCode: "a", Lat: new(25.05), Lng: new(121.52), GeocodePrecision: "road"},
		units: []domain.Presale{
			presaleUnit("五層", 1_000_000, "2026-03-10"), presaleUnit("五層", 1_100_000, "2026-03-20"),
			presaleUnit("十層", 1_200_000, "2026-04-05"), presaleUnit("十層", 1_300_000, "2024-01-05"),
			cancelled, special, parking,
		},
		resales: 50, resaleMedian: new(800_000.0), newResales: 3, newMed: new(1_000_000.0),
	}
	detail, err := newPresaleService(stub).Project(context.Background(), "a|中山區|測試")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Months) != 3 || detail.Months[1].Month != "2026-03" || detail.Months[1].Sales != 2 || *detail.Months[1].MedianPricePerPing != 1_050_000 {
		t.Fatalf("unexpected months: %+v", detail.Months)
	}
	if *detail.MainRooms != 0 || *detail.LowPricePerPing != 1_030_000 || *detail.HighPricePerPing != 1_270_000 {
		t.Fatalf("unexpected range: %v %v %v", *detail.MainRooms, *detail.LowPricePerPing, *detail.HighPricePerPing)
	}
	if len(detail.Floors) != 2 || detail.Floors[0].Floor != 5 || detail.Floors[1].MedianPricePerPing != 1_250_000 {
		t.Fatalf("unexpected floors: %+v", detail.Floors)
	}
	resale := detail.Resale
	// The project's last 12 months of contracts run 2025-05 to 2026-04: 1.0, 1.1 and 1.2M.
	if resale == nil || resale.ProjectFrom != "2025-05" || resale.ProjectSales != 3 || *resale.ProjectPricePerPing != 1_100_000 {
		t.Fatalf("unexpected project window: %+v", resale)
	}
	if math.Abs(*resale.Premium-0.375) > 1e-9 || resale.NewPricePerPing != nil || resale.NewPremium != nil || resale.Through != "2026-02" {
		t.Fatalf("unexpected comparison: %+v", resale)
	}
}

func TestPresaleProjectWithoutLocationSkipsResales(t *testing.T) {
	stub := &presaleRepositoryStub{found: true, project: domain.PresaleProject{CityCode: "a", GeocodePrecision: "district", Lat: new(25.0), Lng: new(121.5)}}
	detail, err := newPresaleService(stub).Project(context.Background(), "a|中山區|測試")
	if err != nil || detail.Resale != nil || !stub.resaleTo.IsZero() {
		t.Fatalf("a district-level project should not be compared: %+v, %v", detail.Resale, err)
	}
	if _, err := newPresaleService(&presaleRepositoryStub{}).Project(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
