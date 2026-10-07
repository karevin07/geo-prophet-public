package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/uber/h3-go/v4"
)

type facilityRepositoryStub struct {
	facilities []domain.Facility
	nearby     []domain.NearbyCategory
	query      domain.FacilityQuery
	limit      int
}

func (stub *facilityRepositoryStub) Facilities(_ context.Context, query domain.FacilityQuery, limit int) ([]domain.Facility, error) {
	stub.query, stub.limit = query, limit
	if len(stub.facilities) > limit {
		return stub.facilities[:limit], nil
	}
	return stub.facilities, nil
}

func (stub *facilityRepositoryStub) DistrictFacilities(context.Context, string) ([]domain.DistrictFacilities, error) {
	return []domain.DistrictFacilities{
		{District: "信義區", AreaKm2: 8.5, Counts: map[string]int{"convenience": 150, "park": 30}},
		{District: "大安區", AreaKm2: 10.25, Counts: map[string]int{"convenience": 250}},
	}, nil
}

func (stub *facilityRepositoryStub) NearbyFacilities(context.Context, float64, float64, float64, float64) ([]domain.NearbyCategory, error) {
	return stub.nearby, nil
}

var taipeiBox = domain.Bounds{West: 121.50, South: 25.02, East: 121.56, North: 25.06}

func TestFacilitiesValidatesAndTruncates(t *testing.T) {
	stub := &facilityRepositoryStub{facilities: make([]domain.Facility, maxFacilitiesShown+10)}
	list, err := NewFacilityService(stub).Facilities(context.Background(), taipeiBox, []string{"park", "convenience", "park"})
	if err != nil {
		t.Fatal(err)
	}
	if !list.Truncated || len(list.Facilities) != maxFacilitiesShown || len(stub.query.Categories) != 2 {
		t.Fatalf("unexpected list: %d truncated %v categories %v", len(list.Facilities), list.Truncated, stub.query.Categories)
	}
	for _, call := range []struct {
		bounds     domain.Bounds
		categories []string
	}{
		{taipeiBox, []string{"casino"}},
		{domain.Bounds{West: 121, South: 24, East: 123, North: 26}, nil},
		{domain.Bounds{West: 121.6, South: 25, East: 121.5, North: 25.1}, nil},
	} {
		if _, err := NewFacilityService(stub).Facilities(context.Background(), call.bounds, call.categories); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: expected invalid input, got %v", call, err)
		}
	}
}

func TestNearbyListsEveryCategory(t *testing.T) {
	stub := &facilityRepositoryStub{nearby: []domain.NearbyCategory{{Category: "park", Count: 2, NearestName: "大安森林公園", NearestM: new(120.4)}}}
	nearby, err := NewFacilityService(stub).Nearby(context.Background(), 25.03, 121.53, 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(nearby.Categories) != len(domain.FacilityCategories) || nearby.Categories[0].Category != "convenience" || nearby.Categories[0].NearestM != nil {
		t.Fatalf("every category should be listed in order: %+v", nearby.Categories)
	}
	for _, category := range nearby.Categories {
		if category.Category == "park" && (category.Count != 2 || *category.NearestM != 120 || category.Group != domain.GroupLeisure) {
			t.Fatalf("unexpected park: %+v", category)
		}
	}
	if _, err := NewFacilityService(stub).Nearby(context.Background(), 22.6, 120.3, 500); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Kaohsiung is not covered: %v", err)
	}
	if _, err := NewFacilityService(stub).Nearby(context.Background(), 25.03, 121.53, 5000); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a 5 km radius is too large: %v", err)
	}
}

func TestDensityPoolsNeighbouringCells(t *testing.T) {
	stub := &facilityRepositoryStub{facilities: []domain.Facility{
		{Category: "convenience", Lat: 25.0400, Lng: 121.5300},
		{Category: "convenience", Lat: 25.0401, Lng: 121.5301},
		{Category: "park", Lat: 25.0420, Lng: 121.5300},
		{Category: "substation", Lat: 25.0400, Lng: 121.5310},
	}}
	cells, err := NewFacilityService(stub).Density(context.Background(), taipeiBox, 9, nil)
	if err != nil {
		t.Fatal(err)
	}
	home, _ := h3.LatLngToCell(h3.NewLatLng(25.0400, 121.5300), 9)
	var found *domain.FacilityCell
	for index := range cells {
		if cells[index].Hex == home.String() {
			found = &cells[index]
		}
	}
	// Two convenience stores count once; the park 220 m away and the substation are within two rings.
	if found == nil || found.Score != 2 || found.Nuisances != 1 || found.Groups[domain.GroupDaily] != 1 || found.Groups[domain.GroupLeisure] != 1 {
		t.Fatalf("unexpected cell: %+v", found)
	}
	coarse, err := NewFacilityService(stub).Density(context.Background(), taipeiBox, 8, nil)
	// The substation falls in the next resolution 8 cell, which no longer pools with it.
	if err != nil || len(coarse) != 2 || coarse[0].Score+coarse[1].Score != 2 || coarse[0].Nuisances+coarse[1].Nuisances != 1 {
		t.Fatalf("at resolution 8 a cell counts only itself: %+v, %v", coarse, err)
	}
	for _, cell := range coarse {
		if cell.Score > 0 && cell.Nuisances > 0 {
			t.Fatalf("the cells should stay apart: %+v", coarse)
		}
	}
	if _, err := NewFacilityService(stub).Density(context.Background(), taipeiBox, 9, []string{"park"}); err != nil || len(stub.query.Categories) != 1 {
		t.Fatalf("density should read only the given categories: %v %v", stub.query.Categories, err)
	}
	if _, err := NewFacilityService(stub).Density(context.Background(), taipeiBox, 7, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("resolution 7 should be invalid: %v", err)
	}
}

func TestDistrictFacilitiesSumsTheCity(t *testing.T) {
	result, err := NewFacilityService(&facilityRepositoryStub{}).Districts(context.Background(), "a")
	if err != nil {
		t.Fatal(err)
	}
	if result.Districts[0].District != "信義區" || result.City.AreaKm2 != 18.75 || result.City.Counts["convenience"] != 400 || result.City.Counts["park"] != 30 {
		t.Fatalf("unexpected totals: %+v", result)
	}
	if _, err := NewFacilityService(&facilityRepositoryStub{}).Districts(context.Background(), "e"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Kaohsiung is not covered: %v", err)
	}
}
