package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
	"github.com/uber/h3-go/v4"
)

func sale(lat, lng float64, month string, buildingPrice int64, area float64, age *int, address string) domain.CellRow {
	date, _ := time.Parse("2006-01", month)
	return domain.CellRow{
		CityCode: "a", District: "大安區", Address: address, Lat: lat, Lng: lng, TransactedAt: date,
		TotalPrice: buildingPrice, BuildingPrice: buildingPrice, AreaPing: area, BuildingAge: age,
	}
}

func years(value int) *int { return &value }

func TestCellDetail(t *testing.T) {
	const lat, lng = 25.0418, 121.5485
	cell, _ := h3.LatLngToCell(h3.NewLatLng(lat, lng), 9)
	road := "台北市大安區敦化南路一段187巷12號"
	var rows []domain.CellRow
	// Six sales in the map window (2025-10..2026-09) at 100萬/坪 with varied ages.
	for index, age := range []*int{years(2), years(20), years(20), years(40), nil, years(8)} {
		rows = append(rows, sale(lat+float64(index)*1e-5, lng, "2025-11", 30_000_000, 30, age, road))
	}
	// Five in the previous comparison year at 80萬/坪, outside the map window.
	for index := range 5 {
		rows = append(rows, sale(lat+float64(index)*1e-5, lng, "2024-06", 24_000_000, 30, years(10), road))
	}
	// A sale in the bounding box but outside the hexagon is ignored.
	rows = append(rows, sale(lat+0.01, lng+0.01, "2025-11", 99_000_000, 10, nil, "台北市大安區仁愛路四段1號"))

	stub := &repositoryStub{cellRows: rows}
	filter := window("2025-10", "2026-09")
	detail, err := usecase.NewTransactionService(stub).CellDetail(context.Background(), filter, cell.String())
	if err != nil {
		t.Fatalf("CellDetail returned an error: %v", err)
	}
	if detail.Count != 6 || detail.AvgPricePerPing != 1_000_000 || detail.AvgArea != 30 || detail.AvgTotalPrice != 30_000_000 {
		t.Fatalf("unexpected figures %#v", detail)
	}
	if detail.District != "大安區" || detail.CityCode != "a" || detail.Road != "敦化南路一段" {
		t.Fatalf("unexpected place %q %q %q", detail.CityCode, detail.District, detail.Road)
	}
	want := map[string]int{"0-5": 1, "5-15": 1, "15-30": 2, "30+": 1, "unknown": 1}
	for _, bucket := range detail.AgeMix {
		if want[bucket.Key] != bucket.Count {
			t.Fatalf("unexpected age mix %#v", detail.AgeMix)
		}
	}
	// Sales are complete through 2026-02: 2025-03..2026-02 vs 2024-03..2025-02 → 100/80.
	if detail.YoYThrough != "2026-02" || !near(detail.PriceYoY, 0.25) {
		t.Fatalf("unexpected YoY %v through %s", detail.PriceYoY, detail.YoYThrough)
	}
	// The repository was asked for the cell's bounding box and the widened window.
	if stub.filter.Bounds == nil || stub.filter.From.Format("2006-01") != "2024-03" || !stub.filter.PreciseLocation {
		t.Fatalf("unexpected repository filter %#v", stub.filter)
	}
}

func TestCellDetailRejectsInvalidCells(t *testing.T) {
	service := usecase.NewTransactionService(&repositoryStub{})
	coarse, _ := h3.LatLngToCell(h3.NewLatLng(25.04, 121.55), 5)
	for _, hex := range []string{"", "not-a-cell", coarse.String()} {
		if _, err := service.CellDetail(context.Background(), window("2025-10", "2026-09"), hex); !errors.Is(err, usecase.ErrInvalidInput) {
			t.Fatalf("%q: expected ErrInvalidInput, got %v", hex, err)
		}
	}
}
