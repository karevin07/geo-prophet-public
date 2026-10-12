package snapshot

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type fakeSales struct {
	sales  []domain.Transaction
	filter domain.TransactionFilter
}

func (fake *fakeSales) List(_ context.Context, filter domain.TransactionFilter) ([]domain.Transaction, error) {
	fake.filter = filter
	return fake.sales, nil
}

func (fake *fakeSales) Feed(_ context.Context, query domain.FeedQuery, runIDs []int64, _ int) ([]domain.Transaction, error) {
	if !query.LatestReleaseOnly || len(runIDs) == 0 {
		return nil, nil
	}
	return fake.sales[:1], nil
}

func (*fakeSales) LatestRelease(context.Context) (string, []int64, error) {
	return "1151001-1151010", []int64{7}, nil
}

func TestWrite(t *testing.T) {
	day := func(value string) time.Time {
		parsed, _ := time.Parse(time.DateOnly, value)
		return parsed
	}
	completed := day("2001-05-01")
	sales := &fakeSales{sales: []domain.Transaction{
		{ID: "A1", CityCode: "a", District: "大安區", Address: "台北市大安區敦化南路一段100號", BuildingType: "住宅大樓(11層含以上有電梯)",
			TotalPrice: 30_000_000, BuildingPrice: 30_000_000, BuildingAreaPing: 30.123, PricePerPing: 995_916.75,
			TotalFloors: 12, CompletedAt: &completed, TransactedAt: day("2026-09-03"), Season: "115S3",
			Lat: 25.0412345678, Lng: 121.5498765432, GeocodePrecision: "address", SpecialReasons: []string{"親友交易"}},
		{ID: "F1", CityCode: "f", District: "板橋區", TransactedAt: day("2026-08-01")},
		{ID: "A2", CityCode: "a", District: "信義區", Address: "台北市信義區", TransactedAt: day("2025-01-02")},
	}}
	var requested []string
	api := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requested = append(requested, request.URL.String())
		writer.Write([]byte(`{"data":{}}`))
	})
	dir := t.TempDir()
	meta, err := Write(context.Background(), dir, sales, Options{
		CityCode: "a", Months: 36, Now: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC), API: api,
		CompleteMonth: func(context.Context) (time.Time, bool, error) { return day("2026-06-01"), true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := sales.filter.From.Format(time.DateOnly); got != "2023-11-01" {
		t.Errorf("from = %s, want 36 months ending in October 2026", got)
	}
	if got := sales.filter.To.Format(time.DateOnly); got != "2026-10-12" || !sales.filter.IncludeSpecial {
		t.Errorf("filter = %+v, want every sale through today", sales.filter)
	}
	if meta.Sales != 2 || meta.Through != "2026-09-03" || meta.CompleteMonth != "2026-06" || meta.LatestRelease.Label != "1151001-1151010" {
		t.Errorf("meta = %+v", meta)
	}
	if meta.Region.Name != "台北市" || len(meta.Region.Districts) != 12 || meta.Responses != len(requested) || len(requested) != 3+2*12 {
		t.Errorf("region %+v, %d responses for %d requests", meta.Region, meta.Responses, len(requested))
	}

	var columns map[string][]any
	readJSON(t, filepath.Join(dir, "sales.json"), &columns)
	row := map[string]any{}
	for column, values := range columns {
		if len(values) != 2 {
			t.Fatalf("%s has %d values, want 2", column, len(values))
		}
		row[column] = values[0]
	}
	want := map[string]any{
		"id": "A1", "road": "敦化南路一段", "buildingAreaPing": 30.12, "pricePerPing": 995_916.75, "totalFloors": 12.0,
		"completedAt": "2001-05-01", "transactedAt": "2026-09-03", "lat": 25.041235, "lng": 121.549877, "isNew": 1.0,
	}
	for column, value := range want {
		if row[column] != value {
			t.Errorf("%s = %v, want %v", column, row[column], value)
		}
	}
	if reasons := row["specialReasons"].([]any); len(reasons) != 1 || columns["isNew"][1] != 0.0 || columns["id"][1] != "A2" {
		t.Errorf("first sale = %v, columns = %v", row, columns)
	}

	key, _ := Key("/api/v1/rentals/districts?city=a")
	if _, err = os.Stat(filepath.Join(dir, "api", key+".json")); err != nil {
		t.Error(err)
	}
	var written Meta
	readJSON(t, filepath.Join(dir, "meta.json"), &written)
	if written.Version != Version || written.Sales != 2 {
		t.Errorf("meta.json = %+v", written)
	}
}

func TestWriteFailsOnAnAPIError(t *testing.T) {
	sales := &fakeSales{sales: []domain.Transaction{{ID: "A1", CityCode: "a", TransactedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}}}
	api := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "no rentals", http.StatusServiceUnavailable)
	})
	dir := t.TempDir()
	_, err := Write(context.Background(), dir, sales, Options{
		CityCode: "a", Months: 12, Now: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), API: api,
		CompleteMonth: func(context.Context) (time.Time, bool, error) { return time.Time{}, false, nil },
	})
	if err == nil {
		t.Fatal("want an error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "meta.json")); !os.IsNotExist(statErr) {
		t.Error("an incomplete snapshot has meta.json")
	}
}

// The web app derives the same keys; see src/lib/api/snapshot-key.test.ts.
func TestKey(t *testing.T) {
	a, _ := Key("/api/v1/presales/projects?city=a&months=12&district=%E5%A4%A7%E5%AE%89%E5%8D%80")
	b, _ := Key("/api/v1/presales/projects?district=大安區&city=a&months=12")
	if a != b || a != "6c2d5a7d42e57a6b2044" {
		t.Errorf("keys %s and %s", a, b)
	}
}

func readJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(body, value); err != nil {
		t.Fatal(err)
	}
}
