package importer_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/importer"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
	"github.com/geo-prophet/geo-prophet/api/internal/transit"
)

var sales = []importer.Dataset{importer.DatasetSales}

var syncOptions = importer.SyncOptions{AddressMaxAge: 30 * 24 * time.Hour, TransitMaxAge: 7 * 24 * time.Hour}

type storeStub struct {
	runs         []domain.ImportRun
	finished     map[int64]domain.ImportStats
	runErrors    map[int64]error
	finishedAt   map[int64]time.Time
	points       map[string][]addresspoint.Point
	transactions []domain.Transaction
	rentals      []domain.Rental
	presales     []domain.Presale
	facilities   map[string][]domain.Facility
	upsertRunID  int64
	pointLoads   int
	boundaries   map[string][]domain.DistrictBoundary
	networks     []transit.Network
}

func newStoreStub() *storeStub {
	return &storeStub{
		finished: map[int64]domain.ImportStats{}, runErrors: map[int64]error{}, finishedAt: map[int64]time.Time{},
		points: map[string][]addresspoint.Point{}, boundaries: map[string][]domain.DistrictBoundary{},
	}
}

func (store *storeStub) ReplaceDistrictBoundaries(_ context.Context, cityCode string, boundaries []domain.DistrictBoundary) error {
	store.boundaries[cityCode] = boundaries
	return nil
}

func (store *storeStub) HasDistrictBoundaries(_ context.Context, cityCode string) (bool, error) {
	return len(store.boundaries[cityCode]) > 0, nil
}

func (store *storeStub) StartRun(_ context.Context, run domain.ImportRun) (int64, error) {
	store.runs = append(store.runs, run)
	return int64(len(store.runs)), nil
}

func (store *storeStub) FinishRun(_ context.Context, runID int64, stats domain.ImportStats, runErr error) error {
	store.finished[runID] = stats
	store.runErrors[runID] = runErr
	store.finishedAt[runID] = time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	return nil
}

// succeeded seeds a successful run, as if imported by an earlier invocation.
func (store *storeStub) succeeded(run domain.ImportRun, at time.Time) {
	store.runs = append(store.runs, run)
	runID := int64(len(store.runs))
	store.finished[runID] = domain.ImportStats{}
	store.runErrors[runID] = nil
	store.finishedAt[runID] = at
}

func (store *storeStub) LastSucceeded(_ context.Context, query domain.ImportRun) (time.Time, bool, error) {
	var last time.Time
	found := false
	for index, run := range store.runs {
		runID := int64(index + 1)
		_, finished := store.finished[runID]
		if !finished || store.runErrors[runID] != nil || run.Source != query.Source || run.Dataset != query.Dataset ||
			run.CityCode != query.CityCode || (query.Season != "" && run.Season != query.Season) {
			continue
		}
		if !found || store.finishedAt[runID].After(last) {
			last, found = store.finishedAt[runID], true
		}
	}
	return last, found, nil
}

func (store *storeStub) ReplaceAddressPoints(_ context.Context, cityCode string, points []addresspoint.Point) error {
	store.points[cityCode] = points
	return nil
}

func (store *storeStub) LoadAddressPoints(_ context.Context, cityCode string) ([]addresspoint.Point, error) {
	store.pointLoads++
	return store.points[cityCode], nil
}

func (store *storeStub) ReplaceTransit(_ context.Context, network transit.Network) (int, error) {
	store.networks = append(store.networks, network)
	return 42, nil
}

func (store *storeStub) UpsertTransactions(_ context.Context, runID int64, transactions []domain.Transaction) error {
	store.upsertRunID = runID
	store.transactions = transactions
	return nil
}

func (store *storeStub) ReplaceFacilities(_ context.Context, _ int64, source string, facilities []domain.Facility) error {
	if store.facilities == nil {
		store.facilities = map[string][]domain.Facility{}
	}
	store.facilities[source] = facilities
	return nil
}

func (store *storeStub) UpsertPresales(_ context.Context, runID int64, presales []domain.Presale) error {
	store.presales = presales
	return nil
}

func (store *storeStub) UpsertRentals(_ context.Context, runID int64, rentals []domain.Rental) error {
	store.rentals = rentals
	return nil
}

func (store *storeStub) runsFor(dataset string) []string {
	var seasons []string
	for index, run := range store.runs {
		if run.Dataset == dataset && store.runErrors[int64(index+1)] == nil {
			seasons = append(seasons, run.Season+"/"+run.CityCode)
		}
	}
	return seasons
}

type fetcherStub struct {
	files   map[string][]byte
	opened  []string
	dataset string
}

func (fetcher *fetcherStub) LatestCSVURL(_ context.Context, datasetID string) (string, error) {
	fetcher.dataset = datasetID
	return "https://example.test/points-" + datasetID + ".csv", nil
}

func (*fetcherStub) LVRSeasonURL(season string) string {
	return "https://example.test/" + season + ".zip"
}

func (*fetcherStub) LVRCurrentURL() string { return "https://example.test/current.zip" }

func (*fetcherStub) OSMExtractURL() string { return "https://example.test/taiwan.osm.pbf" }

func (fetcher *fetcherStub) Open(_ context.Context, target string) (io.ReadCloser, error) {
	fetcher.opened = append(fetcher.opened, target)
	content, ok := fetcher.files[target]
	if !ok {
		// Mirrors 實價登錄: unknown seasons get an HTML page with status 200.
		return io.NopCloser(strings.NewReader("<html>not published</html>")), nil
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

const taipeiPointsCSV = "省市縣市代碼,鄉鎮市區代碼,村里,鄰,街路段,地區,巷,弄,號,橫座標,縱座標\n" +
	"63000,63000020,西村里,008,市府路,,,,１號,306891.63,2770052.15\n" +
	"63000,63000020,西村里,008,市府路,,,,３號,306895.63,2770056.15\n" +
	"63000,63000020,西村里,008,市府路,,,,５號,306899.63,2770060.15\n" +
	"63000,63000020,西村里,008,市府路,,,,１號二樓,306891.63,2770052.15\n" +
	"63000,63000020,西村里,008,市府路,,,,臨２號,306891.63,2770052.15\n"

// archive builds a 實價登錄 zip holding the Taipei sales and rental fixtures and optional
// extra files.
func archive(t *testing.T, extra map[string]string) []byte {
	t.Helper()
	buffer := bytes.Buffer{}
	writer := zip.NewWriter(&buffer)
	for name, fixture := range map[string]string{"a_lvr_land_a.csv": "a_lvr_land_a_sample.csv", "a_lvr_land_c.csv": "a_lvr_land_c_sample.csv", "a_lvr_land_b.csv": "a_lvr_land_b_sample.csv"} {
		content, err := os.ReadFile("../lvr/testdata/" + fixture)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		entry, _ := writer.Create(name)
		_, _ = entry.Write(content)
	}
	for name, content := range extra {
		entry, _ := writer.Create(name)
		_, _ = entry.Write([]byte(content))
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	return buffer.Bytes()
}

const buildTime = `<lvr_land><lvr_time>資料內容：登記日期 115年9月1日至 115年9月10日之買賣案件</lvr_time></lvr_land>`

func taipeiPoints() []addresspoint.Point {
	return []addresspoint.Point{
		{DistrictCode: "63000040", Road: "示範一路", Lane: "133巷", Number: 12, Lat: 25.06, Lng: 121.53},
		{DistrictCode: "63000040", Road: "示範二路", Number: 101, Lat: 25.05, Lng: 121.54},
	}
}

func newService(store *storeStub, fetcher *fetcherStub) *importer.Service {
	return newServiceWithTransit(store, fetcher, nil)
}

func newServiceWithTransit(store *storeStub, fetcher *fetcherStub, source importer.TransitSource) *importer.Service {
	service := importer.NewService(store, fetcher, source, slog.New(slog.NewTextHandler(io.Discard, nil)))
	importer.SetClock(service, func() time.Time { return time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC) })
	return service
}

func seasons(t *testing.T, value string) []lvr.Season {
	t.Helper()
	parsed, err := lvr.ParseSeasons(value)
	if err != nil {
		t.Fatalf("parse seasons: %v", err)
	}
	return parsed
}

func TestImportAddressPointsDownloadsLatestCSV(t *testing.T) {
	store := newStoreStub()
	fetcher := &fetcherStub{files: map[string][]byte{"https://example.test/points-155472.csv": []byte(taipeiPointsCSV)}}

	stats, err := newService(store, fetcher).ImportAddressPoints(context.Background(), "a", "")
	if err != nil {
		t.Fatalf("ImportAddressPoints returned an error: %v", err)
	}
	if fetcher.dataset != "155472" || len(store.points["a"]) != 3 {
		t.Fatalf("expected Taipei dataset with one point, got dataset %q and %#v", fetcher.dataset, store.points["a"])
	}
	if stats.RowsRead != 5 || stats.RowsSkipped != 1 || stats.RowsUpserted != 3 || stats.Detail["skipped_temporary_number"] != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	// Three door numbers on the fixture's floors are enough for one 信義區 cell.
	if got := store.boundaries["a"]; len(got) != 1 || got[0].District != "信義區" || stats.Detail["district_boundaries"] != 1 {
		t.Fatalf("expected a 信義區 boundary from the imported door numbers, got %#v", got)
	}
}

func TestImportSalesGeocodesAndUpserts(t *testing.T) {
	store := newStoreStub()
	store.points["a"] = taipeiPoints()
	fetcher := &fetcherStub{files: map[string][]byte{"https://example.test/115S2.zip": archive(t, nil)}}

	if err := newService(store, fetcher).ImportSeasons(context.Background(), seasons(t, "115S2"), []string{"a"}, sales, ""); err != nil {
		t.Fatalf("ImportSeasons returned an error: %v", err)
	}
	if len(store.transactions) != 7 || store.upsertRunID != 1 {
		t.Fatalf("expected 7 transactions upserted for run 1, got %d for run %d", len(store.transactions), store.upsertRunID)
	}
	precisions := map[string]string{}
	for _, transaction := range store.transactions {
		precisions[transaction.ID] = transaction.GeocodePrecision
	}
	want := map[string]string{
		"SYN-S-001": "address",  // 示範一路133巷12號
		"SYN-S-002": "nearby",   // 示範二路105號, 4 numbers from 101
		"SYN-S-004": "district", // 示範三路 has no points
		"SYN-S-003": "",         // 大同區 has no points
	}
	for id, precision := range want {
		if precisions[id] != precision {
			t.Fatalf("%s: expected precision %q, got %q", id, precision, precisions[id])
		}
	}
	stats := store.finished[1]
	if stats.RowsRead != 9 || stats.RowsSkipped != 2 || stats.RowsUpserted != 7 || stats.RowsGeocodeFailed != 4 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestImportPresalesGeocodesAndUpserts(t *testing.T) {
	store := newStoreStub()
	store.points["a"] = []addresspoint.Point{{DistrictCode: "63000050", Road: "示範五路", Number: 161, SubNumber: 1, Lat: 25.02, Lng: 121.52}}
	fetcher := &fetcherStub{files: map[string][]byte{"https://example.test/113S3.zip": archive(t, nil)}}

	presales := []importer.Dataset{importer.DatasetPresales}
	if err := newService(store, fetcher).ImportSeasons(context.Background(), seasons(t, "113S3"), []string{"a"}, presales, ""); err != nil {
		t.Fatalf("ImportSeasons returned an error: %v", err)
	}
	if len(store.presales) != 4 || len(store.transactions) != 0 {
		t.Fatalf("expected only 4 presales, got %d presales and %d sales", len(store.presales), len(store.transactions))
	}
	for _, presale := range store.presales {
		if presale.ID == "SYN-P-002" && presale.GeocodePrecision != "address" {
			t.Fatalf("示範五路161-1號 should be located once 共13筆 is dropped, got %q", presale.GeocodePrecision)
		}
	}
	if got := store.runsFor("presales"); !slices.Equal(got, []string{"113S3/a"}) {
		t.Fatalf("unexpected runs: %v", got)
	}
}

func TestImportRentalsGeocodesAndUpserts(t *testing.T) {
	store := newStoreStub()
	store.points["a"] = []addresspoint.Point{{DistrictCode: "63000070", Road: "示範六路", Lane: "66巷", Alley: "21弄", Number: 23, Lat: 25.03, Lng: 121.49}}
	fetcher := &fetcherStub{files: map[string][]byte{"https://example.test/115S2.zip": archive(t, nil)}}

	rentals := []importer.Dataset{importer.DatasetRentals}
	if err := newService(store, fetcher).ImportSeasons(context.Background(), seasons(t, "115S2"), []string{"a"}, rentals, ""); err != nil {
		t.Fatalf("ImportSeasons returned an error: %v", err)
	}
	if len(store.transactions) != 0 || len(store.rentals) != 4 {
		t.Fatalf("expected only 4 leases, got %d sales and %d leases", len(store.transactions), len(store.rentals))
	}
	if got := store.runsFor("rentals"); !slices.Equal(got, []string{"115S2/a"}) {
		t.Fatalf("unexpected runs: %v", got)
	}
	for _, rental := range store.rentals {
		if rental.ID == "SYN-R-001" && rental.GeocodePrecision != "address" {
			t.Fatalf("示範六路66巷21弄23號 should be located, got %q", rental.GeocodePrecision)
		}
	}
	if stats := store.finished[1]; stats.RowsRead != 6 || stats.RowsSkipped != 2 || stats.RowsUpserted != 4 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestImportSeasonsRejectsUnknownDatasets(t *testing.T) {
	fetcher := &fetcherStub{}
	err := newService(newStoreStub(), fetcher).ImportSeasons(context.Background(), seasons(t, "115S2"), []string{"a"}, []importer.Dataset{"presale"}, "")
	if err == nil || len(fetcher.opened) != 0 {
		t.Fatalf("expected an error before downloading, got %v after %v", err, fetcher.opened)
	}
}

func TestImportSalesBackfillSkipsUnpublishedSeasonsAndReusesGeocoder(t *testing.T) {
	store := newStoreStub()
	store.points["a"] = taipeiPoints()
	fetcher := &fetcherStub{files: map[string][]byte{
		"https://example.test/115S1.zip": archive(t, nil),
		"https://example.test/115S2.zip": archive(t, nil),
	}}

	if err := newService(store, fetcher).ImportSeasons(context.Background(), seasons(t, "114S4-115S3"), []string{"a"}, sales, ""); err != nil {
		t.Fatalf("ImportSeasons returned an error: %v", err)
	}
	if got := store.runsFor("sales"); !slices.Equal(got, []string{"115S1/a", "115S2/a"}) {
		t.Fatalf("unexpected imported seasons: %v", got)
	}
	if store.pointLoads != 1 {
		t.Fatalf("expected address points to load once, got %d", store.pointLoads)
	}
}

func TestImportSalesSkipsCitiesMissingFromArchive(t *testing.T) {
	store := newStoreStub()
	fetcher := &fetcherStub{files: map[string][]byte{"https://example.test/101S4.zip": archive(t, nil)}}

	if err := newService(store, fetcher).ImportSeasons(context.Background(), seasons(t, "101S4"), []string{"f"}, sales, ""); err != nil {
		t.Fatalf("expected a missing city file to be skipped, got %v", err)
	}
	if store.finished[1].Detail["missing_file"] != 1 || store.runErrors[1] != nil {
		t.Fatalf("expected a recorded skip, got %#v", store.finished[1])
	}
}

func TestImportSalesRequiresAddressPoints(t *testing.T) {
	store := newStoreStub()
	fetcher := &fetcherStub{files: map[string][]byte{"https://example.test/115S2.zip": archive(t, nil)}}

	err := newService(store, fetcher).ImportSeasons(context.Background(), seasons(t, "115S2"), []string{"a"}, sales, "")
	if !errors.Is(err, importer.ErrNoAddressPoints) || !errors.Is(store.runErrors[1], importer.ErrNoAddressPoints) {
		t.Fatalf("expected a recorded ErrNoAddressPoints, got %v", err)
	}
}

func TestImportSalesRejectsUnpublishedSingleSeason(t *testing.T) {
	err := newService(newStoreStub(), &fetcherStub{}).ImportSeasons(context.Background(), seasons(t, "120S1"), []string{"a"}, sales, "")
	if !errors.Is(err, importer.ErrSeasonUnavailable) {
		t.Fatalf("expected ErrSeasonUnavailable, got %v", err)
	}
}

func TestImportSalesRejectsUnsupportedCityBeforeDownloading(t *testing.T) {
	fetcher := &fetcherStub{}
	if err := newService(newStoreStub(), fetcher).ImportSeasons(context.Background(), seasons(t, "115S2"), []string{"a", "e"}, sales, ""); err == nil {
		t.Fatal("expected an unsupported city error")
	}
	if len(fetcher.opened) != 0 {
		t.Fatalf("expected no download, got %v", fetcher.opened)
	}
}

func TestImportCurrentReleaseUsesReleaseSeasonAndSkipsRepeats(t *testing.T) {
	store := newStoreStub()
	store.points["a"] = taipeiPoints()
	fetcher := &fetcherStub{files: map[string][]byte{"https://example.test/current.zip": archive(t, map[string]string{"build_time.xml": buildTime})}}
	service := newService(store, fetcher)

	for range 2 {
		if err := service.ImportCurrentRelease(context.Background(), []string{"a"}, importer.Datasets, "", true); err != nil {
			t.Fatalf("ImportCurrentRelease returned an error: %v", err)
		}
	}
	if got := store.runsFor("sales_release"); !slices.Equal(got, []string{"1150901-1150910/a"}) {
		t.Fatalf("expected one release import, got %v", got)
	}
	if store.transactions[0].Season != "115S3" {
		t.Fatalf("expected release rows in season 115S3, got %s", store.transactions[0].Season)
	}
}

func TestSyncRefreshesStaleDataAndImportsNewestSeason(t *testing.T) {
	store := newStoreStub()
	store.points["a"] = taipeiPoints()
	// Addresses are 40 days old; 115S2 is the newest published season (115S3/S4 are not).
	store.succeeded(domain.ImportRun{Source: "addresspoint", Dataset: "door_numbers", CityCode: "a"}, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	fetcher := &fetcherStub{files: map[string][]byte{
		"https://example.test/points-155472.csv": []byte(taipeiPointsCSV),
		"https://example.test/115S2.zip":         archive(t, nil),
		"https://example.test/current.zip":       archive(t, map[string]string{"build_time.xml": buildTime}),
	}}
	service := newService(store, fetcher)

	if err := service.Sync(context.Background(), []string{"a"}, syncOptions); err != nil {
		t.Fatalf("Sync returned an error: %v", err)
	}
	if got := store.runsFor("door_numbers"); len(got) != 2 {
		t.Fatalf("expected stale addresses to be refreshed, got %v", got)
	}
	if got := store.runsFor("sales"); !slices.Equal(got, []string{"115S2/a"}) {
		t.Fatalf("expected the newest published season, got %v", got)
	}
	if got := store.runsFor("sales_release"); !slices.Equal(got, []string{"1150901-1150910/a"}) {
		t.Fatalf("expected the current release, got %v", got)
	}
	if got := store.runsFor("rentals"); !slices.Equal(got, []string{"115S2/a"}) {
		t.Fatalf("expected leases of the newest published season, got %v", got)
	}
	if got := store.runsFor("rentals_release"); !slices.Equal(got, []string{"1150901-1150910/a"}) {
		t.Fatalf("expected leases of the current release, got %v", got)
	}
	if got := store.runsFor("presales"); !slices.Equal(got, []string{"115S2/a"}) {
		t.Fatalf("expected presales of the newest published season, got %v", got)
	}

	// A second sync finds everything up to date and downloads only the probes it needs.
	fetcher.opened = nil
	if err := service.Sync(context.Background(), []string{"a"}, syncOptions); err != nil {
		t.Fatalf("second Sync returned an error: %v", err)
	}
	want := []string{"https://example.test/115S4.zip", "https://example.test/115S3.zip", "https://example.test/current.zip"}
	if !slices.Equal(fetcher.opened, want) {
		t.Fatalf("unexpected downloads on an up-to-date sync: %v", fetcher.opened)
	}
}

func TestImportBoundariesNeedsStoredDoorNumbers(t *testing.T) {
	store := newStoreStub()
	service := newService(store, &fetcherStub{})
	if err := service.ImportBoundaries(context.Background(), "a"); !errors.Is(err, importer.ErrNoAddressPoints) {
		t.Fatalf("expected ErrNoAddressPoints, got %v", err)
	}
	store.points["a"] = []addresspoint.Point{
		{DistrictCode: "63000040", Lat: 25.06, Lng: 121.53, Number: 1},
		{DistrictCode: "63000040", Lat: 25.0601, Lng: 121.5301, Number: 2},
		{DistrictCode: "63000040", Lat: 25.0602, Lng: 121.5302, Number: 3},
	}
	if err := service.ImportBoundaries(context.Background(), "a"); err != nil {
		t.Fatalf("ImportBoundaries returned an error: %v", err)
	}
	if got := store.boundaries["a"]; len(got) != 1 || got[0].District != "中山區" {
		t.Fatalf("expected a 中山區 boundary, got %#v", got)
	}
}

func TestImportTransitRecordsTheNetwork(t *testing.T) {
	store := newStoreStub()
	network := transit.Network{
		Stations: []transit.Station{{ID: "TRTC-BL07", Name: "板橋", Platforms: []transit.Platform{
			{ID: "TRTC-BL07", Exits: []transit.Exit{{ID: "1"}, {ID: "2"}}},
			{ID: "TRA-1020", Exits: []transit.Exit{{ID: "N"}}},
		}}},
		Lines: []transit.Line{{ID: "TRTC-BL"}, {ID: "TRA-WL"}},
	}
	service := newServiceWithTransit(store, &fetcherStub{}, func(context.Context) (transit.Network, error) { return network, nil })

	if err := service.ImportTransit(context.Background()); err != nil {
		t.Fatalf("ImportTransit: %v", err)
	}
	if len(store.networks) != 1 {
		t.Fatalf("expected the network to be stored once, got %d", len(store.networks))
	}
	stats := store.finished[1]
	if store.runs[0].Source != "tdx" || stats.RowsUpserted != 1 || stats.Detail["platforms"] != 2 ||
		stats.Detail["exits"] != 3 || stats.Detail["lines"] != 2 || stats.Detail["sale_station_pairs"] != 42 {
		t.Fatalf("unexpected run %+v with stats %+v", store.runs[0], stats)
	}
}

func TestImportTransitRejectsAnEmptyNetwork(t *testing.T) {
	store := newStoreStub()
	service := newServiceWithTransit(store, &fetcherStub{}, func(context.Context) (transit.Network, error) { return transit.Network{}, nil })
	if err := service.ImportTransit(context.Background()); err == nil || len(store.networks) != 0 {
		t.Fatalf("expected an empty network to fail without replacing stations, got %v", err)
	}
	if err := newService(store, &fetcherStub{}).ImportTransit(context.Background()); !errors.Is(err, importer.ErrTransitNotConfigured) {
		t.Fatalf("expected ErrTransitNotConfigured, got %v", err)
	}
}

func TestSyncRefreshesStaleTransitOnly(t *testing.T) {
	loads := 0
	source := func(context.Context) (transit.Network, error) {
		loads++
		return transit.Network{Stations: []transit.Station{{ID: "TRA-1075"}}}, nil
	}
	store := newStoreStub()
	store.points["a"] = taipeiPoints()
	now := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	store.succeeded(domain.ImportRun{Source: "addresspoint", Dataset: "door_numbers", CityCode: "a"}, now)
	store.succeeded(domain.ImportRun{Source: "addresspoint", Dataset: "district_boundaries", CityCode: "a"}, now)
	store.boundaries["a"] = []domain.DistrictBoundary{{District: "中山區"}}
	for _, season := range []string{"115S4", "115S3", "115S2"} {
		store.succeeded(domain.ImportRun{Source: "lvr", Dataset: "sales", Season: season, CityCode: "a"}, now)
	}
	store.succeeded(domain.ImportRun{Source: "lvr", Dataset: "sales_release", Season: "1150901-1150910", CityCode: "a"}, now)
	fetcher := &fetcherStub{files: map[string][]byte{"https://example.test/current.zip": archive(t, map[string]string{"build_time.xml": buildTime})}}
	service := newServiceWithTransit(store, fetcher, source)

	// Stations loaded 8 days ago are refreshed, then left alone.
	store.succeeded(domain.ImportRun{Source: "tdx", Dataset: "transit"}, now.AddDate(0, 0, -8))
	for range 2 {
		if err := service.Sync(context.Background(), []string{"a"}, syncOptions); err != nil {
			t.Fatalf("Sync: %v", err)
		}
	}
	if loads != 1 {
		t.Fatalf("expected one transit refresh, got %d", loads)
	}
}

func TestImportFacilitiesLocatesGovernmentLists(t *testing.T) {
	store := newStoreStub()
	store.points["a"] = taipeiPoints()
	header := "醫事機構代碼,醫事機構名稱,醫事機構種類,電話,地址\n"
	empty := []byte(header)
	fetcher := &fetcherStub{files: map[string][]byte{
		"https://example.test/points-39280.csv": empty,
		"https://example.test/points-39281.csv": empty,
		"https://example.test/points-39282.csv": empty,
		"https://example.test/points-39283.csv": []byte(header +
			"0101010001,新生診所,西醫診所,(02)1,臺北市中山區示範一路１３３巷１２號\n" +
			"0101010002,遠方診所,西醫診所,(02)2,臺北市中山區示範三路１號\n" +
			"0501010003,高雄診所,西醫診所,(07)3,高雄市苓雅區凱旋二路１號\n" +
			"0201010004,板橋診所,西醫診所,(02)4,新北市板橋區示範五路１號\n"),
	}}
	if err := newService(store, fetcher).ImportFacilities(context.Background(), []string{importer.FacilitiesNHI}, ""); err != nil {
		t.Fatalf("ImportFacilities returned an error: %v", err)
	}
	// 示範三路 has no door numbers, so it is only located at its district and dropped; 新北
	// has none imported, so its clinic is skipped rather than failing the import.
	clinics := store.facilities["nhi"]
	if len(clinics) != 1 || clinics[0].ID != "nhi:0101010001" || clinics[0].Category != "clinic" || clinics[0].District != "中山區" || clinics[0].GeocodePrecision != "address" {
		t.Fatalf("unexpected clinics: %+v", clinics)
	}
	if got := store.runsFor("nhi_facilities"); len(got) != 1 {
		t.Fatalf("expected one run, got %v", got)
	}
	if stats := store.finished[1]; stats.RowsRead != 3 || stats.RowsUpserted != 1 || stats.RowsGeocodeFailed != 1 ||
		stats.RowsSkipped != 1 || stats.Detail["skipped_no_address_points"] != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if err := newService(store, fetcher).ImportFacilities(context.Background(), []string{"google"}, ""); err == nil {
		t.Fatal("an unknown source should fail")
	}
}
