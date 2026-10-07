package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	delivery "github.com/geo-prophet/geo-prophet/api/internal/delivery/http"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
	"github.com/gin-gonic/gin"
)

type repositoryStub struct {
	filter         domain.TransactionFilter
	summaryFilters []domain.TransactionFilter
	feedQuery      domain.FeedQuery
	districtQuery  domain.DistrictComparisonQuery
	cellRows       []domain.CellRow
	stationFilter  *domain.TransactionFilter
	// yoyFilter is separate from stationFilter: the service queries both concurrently.
	yoyFilter *domain.TransactionFilter
	yoyWindow [3]time.Time
}

func (*repositoryStub) TransitLines(context.Context) ([]domain.TransitLine, error) {
	opened := "2020-01-31"
	return []domain.TransitLine{{ID: "NTMC-Y", System: "metro", Name: "環狀線", Color: "#fedb00", OpenedOn: &opened,
		Geometry: []byte(`{"type":"MultiLineString","coordinates":[[[121.5,25.0],[121.51,25.01]]]}`)}}, nil
}

func (*repositoryStub) TransitStations(_ context.Context, cityCode string) ([]domain.TransitStation, error) {
	return []domain.TransitStation{{ID: "TRTC-BL07", Name: "板橋", CityCode: "f"}}, nil
}

func (*repositoryStub) TransitStation(_ context.Context, id string) (domain.TransitStation, bool, error) {
	return domain.TransitStation{ID: id, Name: "板橋"}, id == "TRTC-BL07", nil
}

func (repository *repositoryStub) StationRings(_ context.Context, _ string, filter domain.TransactionFilter, radii []int) ([]domain.StationRing, error) {
	repository.stationFilter = &filter
	rings := make([]domain.StationRing, len(radii))
	for index, radius := range radii {
		rings[index] = domain.StationRing{Radius: radius, Count: radius / 10, Trends: []domain.TrendPoint{{Month: "2026-02"}, {Month: "2026-03"}}}
	}
	return rings, nil
}

func (repository *repositoryStub) StationYoYTotals(_ context.Context, filter domain.TransactionFilter, radius int, _, _, _ time.Time) (map[string]domain.DistrictYoYTotals, error) {
	repository.yoyFilter = &filter
	return map[string]domain.DistrictYoYTotals{
		"TRTC-BL07": {CurrentCount: 10, PreviousCount: 8, CurrentBuildingPrice: 1_100_000, PreviousBuildingPrice: 1_000_000, CurrentArea: 1, PreviousArea: 1},
		"NTMC-Y08":  {CurrentCount: 2, PreviousCount: 9, CurrentBuildingPrice: 1, PreviousBuildingPrice: 1, CurrentArea: 1, PreviousArea: 1},
	}, nil
}

func (repository *repositoryStub) StationRankings(_ context.Context, filter domain.TransactionFilter, radius int) ([]domain.StationRank, error) {
	repository.stationFilter = &filter
	return []domain.StationRank{
		{StationID: "TRTC-BL07", Count: 81, MedianPricePerPing: 734_000, Trend: []domain.TrendPoint{{Month: "2026-02"}, {Month: "2026-03"}}},
		{StationID: "NTMC-Y08", Count: 3, MedianPricePerPing: 677_000, Trend: []domain.TrendPoint{}},
	}, nil
}

func (repository *repositoryStub) DistanceCurve(_ context.Context, filter domain.TransactionFilter) ([]domain.DistanceBucket, error) {
	repository.stationFilter = &filter
	to := 100
	return []domain.DistanceBucket{{CityCode: "a", FromM: 0, ToM: &to, Count: 120, MedianPricePerPing: 900_000}, {CityCode: "a", FromM: 800, Count: 30, MedianPricePerPing: 700_000}}, nil
}

func (*repositoryStub) OpeningEvents(context.Context) ([]domain.OpeningEvent, error) {
	return []domain.OpeningEvent{
		{StationID: "NTMC-Y08", StationName: "十四張", LineID: "NTMC-Y", OpenedOn: "2020-01-31"},
		{StationID: "NTALRT-K01", StationName: "雙城", LineID: "NTALRT-K", OpenedOn: "2023-02-10"},
		{StationID: "NTMC-LB06", StationName: "三峽", LineID: "NTMC-LB", OpenedOn: "2026-06-30"},
	}, nil
}

// OpeningSums gives 十四張 a 20% rise against a 10% district rise, and 雙城 too few sales.
func (repository *repositoryStub) OpeningSums(_ context.Context, filter domain.TransactionFilter, _ int, events []domain.OpeningEvent, months int) (map[int]map[string]domain.MonthlySums, map[int]map[string]domain.MonthlySums, error) {
	repository.stationFilter = &filter
	station := map[int]map[string]domain.MonthlySums{0: {"2019-06": {Count: 12, BuildingPrice: 12_000_000, Area: 24}, "2020-06": {Count: 12, BuildingPrice: 14_400_000, Area: 24}},
		1: {"2022-06": {Count: 2, BuildingPrice: 1_000_000, Area: 2}, "2023-06": {Count: 2, BuildingPrice: 1_000_000, Area: 2}}}
	baseline := map[int]map[string]domain.MonthlySums{0: {"2019-06": {Count: 40, BuildingPrice: 40_000_000, Area: 80}, "2020-06": {Count: 40, BuildingPrice: 44_000_000, Area: 80}},
		1: {"2022-06": {Count: 40, BuildingPrice: 40_000_000, Area: 80}, "2023-06": {Count: 40, BuildingPrice: 40_000_000, Area: 80}}}
	return station, baseline, nil
}

func (repository *repositoryStub) StationRingYoY(_ context.Context, _ string, _ domain.TransactionFilter, _ []int, previousFrom, currentFrom, currentTo time.Time) (map[int]domain.DistrictYoYTotals, error) {
	repository.yoyWindow = [3]time.Time{previousFrom, currentFrom, currentTo}
	return map[int]domain.DistrictYoYTotals{
		300: {CurrentCount: 3, PreviousCount: 3, CurrentBuildingPrice: 1, PreviousBuildingPrice: 1, CurrentArea: 1, PreviousArea: 1},
		500: {CurrentCount: 10, PreviousCount: 8, CurrentBuildingPrice: 1_100_000, PreviousBuildingPrice: 1_000_000, CurrentArea: 1, PreviousArea: 1},
	}, nil
}

func (repository *repositoryStub) List(_ context.Context, filter domain.TransactionFilter) ([]domain.Transaction, error) {
	repository.filter = filter
	return []domain.Transaction{}, nil
}

func (repository *repositoryStub) ForEachPricePoint(_ context.Context, filter domain.TransactionFilter, _ func(domain.PricePoint) error) error {
	repository.filter = filter
	return nil
}

func (repository *repositoryStub) MonthlyTrends(_ context.Context, filter domain.TransactionFilter) ([]domain.TrendPoint, error) {
	repository.filter = filter
	return nil, nil
}

func (repository *repositoryStub) Summary(_ context.Context, filter domain.TransactionFilter) (domain.MarketSummary, error) {
	repository.summaryFilters = append(repository.summaryFilters, filter)
	return domain.MarketSummary{Count: 42, AvgPricePerPing: 700_000, MedianTotalPrice: 16_800_000}, nil
}

func (repository *repositoryStub) Analysis(_ context.Context, filter domain.TransactionFilter) (domain.MarketAnalysis, error) {
	repository.filter = filter
	return domain.MarketAnalysis{
		Monthly:      []domain.MonthlyAnalysis{{Month: "2026-09", Count: 42, MedianPricePerPing: 680_000}},
		Categories:   []domain.CategoryAnalysis{{Category: "公寓", Count: 12}},
		AgeGroups:    []domain.BreakdownAnalysis{{Key: "10_19", Count: 8}},
		AreaGroups:   []domain.BreakdownAnalysis{{Key: "15_29", Count: 10}},
		Distribution: []domain.PriceBucket{{MinPricePerPing: 600_000, MaxPricePerPing: 700_000, Count: 8}},
	}, nil
}

func (repository *repositoryStub) DistrictRankings(_ context.Context, query domain.DistrictComparisonQuery) ([]domain.DistrictRank, error) {
	repository.districtQuery = query
	return []domain.DistrictRank{{District: "大安區", Count: 42, MedianPricePerPing: 800_000}}, nil
}

func (*repositoryStub) DistrictTrends(_ context.Context, _ domain.DistrictComparisonQuery) ([]domain.DistrictTrend, error) {
	return []domain.DistrictTrend{{District: "大安區", Month: "2026-09", Count: 3}}, nil
}

func (*repositoryStub) DistrictYoYTotals(_ context.Context, _ domain.DistrictComparisonQuery, _, _, _ time.Time) (map[string]domain.DistrictYoYTotals, error) {
	return map[string]domain.DistrictYoYTotals{}, nil
}

func (*repositoryStub) FindAddresses(context.Context, domain.PlaceQuery) ([]domain.PlaceMatch, error) {
	return nil, nil
}

func (*repositoryStub) FindRoads(_ context.Context, query domain.PlaceQuery) ([]domain.PlaceMatch, error) {
	return []domain.PlaceMatch{{CityCode: "a", DistrictCode: "63000030", Road: query.Road, Lat: 25.04, Lng: 121.55}}, nil
}

func (*repositoryStub) FindStations(context.Context, string, int) ([]domain.Place, error) {
	return nil, nil
}

func (*repositoryStub) DistrictCenter(context.Context, string, string) (float64, float64, bool, error) {
	return 25.03, 121.54, true, nil
}

func (*repositoryStub) ImportedSales(context.Context) ([]string, []string, error) {
	return []string{"115S2"}, nil, nil // registrations complete through 2026-02
}

func (*repositoryStub) ImportedSalesByCity(context.Context, string) ([]string, []string, error) {
	return []string{"115S2"}, nil, nil
}

func (*repositoryStub) InvestmentAggregates(_ context.Context, _ domain.InvestmentQuery, _ domain.InvestmentWindow) ([]domain.InvestmentAggregate, error) {
	median := 800_000.0
	return []domain.InvestmentAggregate{{CurrentCount: 45, MedianPricePerPing: &median},
		{District: "大安區", CurrentCount: 45, PreviousCount: 36, MedianPricePerPing: &median, PreviousMedianPricePerPing: &median}}, nil
}

// feedRows serves 60 sales, newest first, so the first page has a next cursor.
func (repository *repositoryStub) Feed(_ context.Context, query domain.FeedQuery, _ []int64, limit int) ([]domain.Transaction, error) {
	repository.feedQuery = query
	start := 0
	if query.Cursor != nil {
		start, _ = strconv.Atoi(strings.TrimPrefix(query.Cursor.ID, "T"))
		start++
	}
	var rows []domain.Transaction
	for index := start; index < 60 && len(rows) < limit; index++ {
		rows = append(rows, domain.Transaction{
			ID: fmt.Sprintf("T%02d", index), Address: "台北市大安區仁愛路四段1號", TotalPrice: 20_000_000,
			TransactedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -index), IsNew: index < 2,
		})
	}
	return rows, nil
}

func (*repositoryStub) FeedCounts(context.Context, domain.FeedQuery, []int64) (int, int, error) {
	return 60, 2, nil
}

func (repository *repositoryStub) ExportFeed(ctx context.Context, query domain.FeedQuery, runIDs []int64, limit int, emit func(domain.Transaction) error) error {
	rows, _ := repository.Feed(ctx, query, runIDs, 3)
	for _, row := range rows {
		if err := emit(row); err != nil {
			return err
		}
	}
	return nil
}

func (*repositoryStub) LatestRelease(context.Context) (string, []int64, error) {
	return "1150901-1150910", []int64{7}, nil
}

func (*repositoryStub) DistrictBoundaries(context.Context, string) ([]domain.DistrictBoundary, error) {
	return []domain.DistrictBoundary{{District: "大安區", Cells: 2, Coordinates: [][][][]float64{{{{121.53, 25.03}, {121.54, 25.03}, {121.54, 25.04}, {121.53, 25.03}}}}}}, nil
}

func (repository *repositoryStub) CellRows(_ context.Context, filter domain.TransactionFilter) ([]domain.CellRow, error) {
	repository.filter = filter
	return repository.cellRows, nil
}

func (*repositoryStub) Ping(context.Context) error { return nil }

// services wires the stub into every use case; data status and admin need a database and
// stay nil, as without DATABASE_URL.
func services(repository *repositoryStub) delivery.Services {
	transactions := usecase.NewTransactionService(repository)
	return delivery.Services{
		Transactions: transactions, Places: usecase.NewPlaceService(repository), Feed: usecase.NewFeedService(repository),
		Investment: usecase.NewInvestmentService(repository),
		Transit:    usecase.NewTransitService(repository, transactions),
	}
}

func serve(t *testing.T, repository *repositoryStub, target string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := delivery.NewRouter(delivery.NewHandler(services(repository)), []string{"http://localhost:3000"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func TestListTransactionsRejectsMalformedBBox(t *testing.T) {
	recorder := serve(t, &repositoryStub{}, "/api/v1/transactions?bbox=bad")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "invalid_request") {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestListTransactionsParsesFilters(t *testing.T) {
	repository := &repositoryStub{}
	recorder := serve(t, repository, "/api/v1/transactions?types=公寓,其他&include_special=true&from=2026-01&to=2026-03&bbox=121.4,24.9,121.7,25.2")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	filter := repository.filter
	if !slices.Equal(filter.Categories, []string{"公寓", "其他"}) || !filter.IncludeSpecial || filter.Bounds == nil {
		t.Fatalf("unexpected filter: %#v", filter)
	}
	if filter.From.Format("2006-01-02") != "2026-01-01" || filter.To.Format("2006-01-02") != "2026-03-31" {
		t.Fatalf("unexpected date window: %v - %v", filter.From, filter.To)
	}
}

func TestTrendsDefaultsToTwelveMonthWindow(t *testing.T) {
	repository := &repositoryStub{}
	if recorder := serve(t, repository, "/api/v1/trends"); recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	months := (repository.filter.To.Year()-repository.filter.From.Year())*12 + int(repository.filter.To.Month()-repository.filter.From.Month()) + 1
	if months != 12 || repository.filter.IncludeSpecial {
		t.Fatalf("expected a 12 month window without special transactions, got %v - %v", repository.filter.From, repository.filter.To)
	}
}

func TestRejectsInvalidBooleanAndBuildingType(t *testing.T) {
	for _, target := range []string{"/api/v1/trends?include_special=maybe", "/api/v1/aggregations/h3?types=整層住家"} {
		if recorder := serve(t, &repositoryStub{}, target); recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected status 400, got %d", target, recorder.Code)
		}
	}
}

func TestSummaryUsesFilters(t *testing.T) {
	repository := &repositoryStub{}
	recorder := serve(t, repository, "/api/v1/summary?types=公寓&from=2026-09&to=2026-09")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"medianTotalPrice":16800000`) {
		t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
	}
	// The first query is the selected month; the others compute the year-over-year change.
	first := repository.summaryFilters[0]
	if !slices.Equal(first.Categories, []string{"公寓"}) || first.From.Format("2006-01") != "2026-09" {
		t.Fatalf("unexpected filter: %#v", first)
	}
	if !strings.Contains(recorder.Body.String(), `"yoy":{"basis":"3m"`) {
		t.Fatalf("expected a 3-month year-over-year basis: %s", recorder.Body.String())
	}
}

func TestAnalysisUsesSharedFilters(t *testing.T) {
	repository := &repositoryStub{}
	recorder := serve(t, repository, "/api/v1/analysis?types=公寓&from=2021-01&to=2026-09")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"medianPricePerPing":680000`) ||
		!strings.Contains(recorder.Body.String(), `"minPricePerPing":600000`) ||
		!strings.Contains(recorder.Body.String(), `"ageGroups":[{"key":"10_19"`) ||
		!strings.Contains(recorder.Body.String(), `"provisional":true`) {
		t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
	}
	if !slices.Equal(repository.filter.Categories, []string{"公寓"}) || repository.filter.From.Format("2006-01") != "2021-01" {
		t.Fatalf("unexpected filter: %#v", repository.filter)
	}
}

func TestAnalysisDefaultsToFiveYears(t *testing.T) {
	repository := &repositoryStub{}
	if recorder := serve(t, repository, "/api/v1/analysis"); recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	months := (repository.filter.To.Year()-repository.filter.From.Year())*12 + int(repository.filter.To.Month()-repository.filter.From.Month()) + 1
	if months != 60 {
		t.Fatalf("expected 60 months, got %d", months)
	}
}

func TestDistrictComparisonDefaultsAndValidatesSelection(t *testing.T) {
	repository := &repositoryStub{}
	recorder := serve(t, repository, "/api/v1/districts/compare?districts=大安區&types=公寓")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"district":"大安區"`) ||
		!strings.Contains(recorder.Body.String(), `"provisional":true`) {
		t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
	}
	months := (repository.districtQuery.Filter.To.Year()-repository.districtQuery.Filter.From.Year())*12 +
		int(repository.districtQuery.Filter.To.Month()-repository.districtQuery.Filter.From.Month()) + 1
	if repository.districtQuery.CityCode != "a" || months != 36 || !slices.Equal(repository.districtQuery.Districts, []string{"大安區"}) {
		t.Fatalf("unexpected query: %#v", repository.districtQuery)
	}
	for _, target := range []string{
		"/api/v1/districts/compare?city=z",
		"/api/v1/districts/compare?city=a&districts=不存在區",
		"/api/v1/districts/compare?city=a&districts=大安區,大安區",
	} {
		if recorder = serve(t, &repositoryStub{}, target); recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", target, recorder.Code)
		}
	}
}

func TestInvestmentAreasAndInvalidCohort(t *testing.T) {
	ok := serve(t, &repositoryStub{}, "/api/v1/investment/areas?city=a&type=住宅大樓&min_area=15&max_area=45&min_age=5&max_age=30")
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"currentCount":45`) ||
		!strings.Contains(ok.Body.String(), `"minimumSample":30`) {
		t.Fatalf("unexpected investment response: %d %s", ok.Code, ok.Body.String())
	}
	bad := serve(t, &repositoryStub{}, "/api/v1/investment/areas?min_age=40&max_age=5")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid age band: %d %s", bad.Code, bad.Body.String())
	}
}

func TestInvestmentWorkbenchNeedsDatabase(t *testing.T) {
	response := serve(t, &repositoryStub{}, "/api/v1/investment/workbench")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("mock workbench status = %d, want 503", response.Code)
	}
}

func TestSearchPlaces(t *testing.T) {
	recorder := serve(t, &repositoryStub{}, "/api/v1/places?q=敦化南路")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"label":"台北市大安區敦化南路"`) {
		t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder = serve(t, &repositoryStub{}, "/api/v1/places?q=a"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected a too-short query to be rejected, got %d", recorder.Code)
	}
}

func TestHealth(t *testing.T) {
	if recorder := serve(t, &repositoryStub{}, "/healthz"); recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

type statusRepositoryStub struct{}

func (statusRepositoryStub) DataStatusSnapshot(context.Context) (domain.DataStatusSnapshot, error) {
	return domain.DataStatusSnapshot{
		ImportedSeasons: map[string][]string{"a": {"115S1", "115S3"}},
		RecentRuns:      []domain.ImportRunStatus{{ID: 7, Status: "failed", Error: "get https://tdx.example/v2: 429 Too Many Requests"}},
	}, nil
}

func (statusRepositoryStub) DatabaseUsage(context.Context) (domain.DatabaseUsage, error) {
	return domain.DatabaseUsage{Bytes: 1_390_000_000, Tables: []domain.TableSize{{Name: "transactions", Bytes: 838 << 20}}}, nil
}

func (statusRepositoryStub) Openings(context.Context) (domain.Openings, error) {
	opened := "2020-01-31"
	return domain.Openings{Lines: []domain.LineOpening{{LineID: "NTMC-Y", Name: "環狀線", OpenedOn: &opened}}}, nil
}

func (statusRepositoryStub) SaveOpening(_ context.Context, edit domain.OpeningEdit, updatedBy string) error {
	if edit.TargetID != "NTMC-Y" {
		return domain.ErrUnknownTarget
	}
	return nil
}

func (statusRepositoryStub) EnqueueJob(_ context.Context, kind domain.JobKind, _ any, requestedBy string) (domain.AdminJob, error) {
	if kind == domain.JobTransit {
		return domain.AdminJob{}, domain.ErrJobActive // already queued
	}
	return domain.AdminJob{ID: 9, Kind: kind, Status: domain.JobQueued, RequestedBy: requestedBy, Result: []byte("{}")}, nil
}

func (statusRepositoryStub) Jobs(context.Context) ([]domain.AdminJob, error) {
	return []domain.AdminJob{{ID: 9, Kind: domain.JobSync, Status: domain.JobFailed, Result: []byte("{}"), Error: "502 Bad Gateway"}}, nil
}

// serveAdmin requests target as login (none when empty) with the admin console wired up.
func serveAdmin(t *testing.T, target, login string) *httptest.ResponseRecorder {
	t.Helper()
	return serveAdminRequest(t, httptest.NewRequest(http.MethodGet, target, nil), login)
}

func serveAdminRequest(t *testing.T, request *http.Request, login string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	wired := services(&repositoryStub{})
	wired.DataStatus = usecase.NewDataStatusService(statusRepositoryStub{})
	wired.Admin = usecase.NewAdminService(statusRepositoryStub{}, usecase.AdminSettings{Logins: []string{"admin@example.com"}, BackupKeep: 7})
	if login != "" {
		request.Header.Set("Tailscale-User-Login", login)
		request.Header.Set("Tailscale-User-Name", "=?utf-8?q?=E5=BB=96?=") // 廖, RFC 2047 encoded
	}
	recorder := httptest.NewRecorder()
	delivery.NewRouter(delivery.NewHandler(wired), []string{"http://localhost:3000"}).ServeHTTP(recorder, request)
	return recorder
}

func TestAdminConsoleNeedsAnAllowedTailscaleUser(t *testing.T) {
	for _, target := range []string{"/api/v1/admin/data-status", "/api/v1/admin/system"} {
		if recorder := serveAdmin(t, target, ""); recorder.Code != http.StatusForbidden {
			t.Errorf("%s without Tailscale: got %d", target, recorder.Code)
		}
		if recorder := serveAdmin(t, target, "someone@example.com"); recorder.Code != http.StatusForbidden {
			t.Errorf("%s as a non-admin: got %d", target, recorder.Code)
		}
		if recorder := serveAdmin(t, target, "admin@example.com"); recorder.Code != http.StatusOK {
			t.Errorf("%s as an admin: got %d: %s", target, recorder.Code, recorder.Body.String())
		}
	}
	recorder := serveAdmin(t, "/api/v1/admin/me", "someone@example.com")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"admin":false`) || !strings.Contains(recorder.Body.String(), `"name":"廖"`) {
		t.Fatalf("unexpected session %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminDataStatusShowsCoverageAndErrors(t *testing.T) {
	recorder := serveAdmin(t, "/api/v1/admin/data-status", "admin@example.com")
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"missing":["115S2"]`) ||
		!strings.Contains(body, `"error":"get https://tdx.example/v2: 429 Too Many Requests"`) || !strings.Contains(body, `"errorKind":"rate_limited"`) ||
		!strings.Contains(body, `"quality":{"metrics":[],"alerts":[]}`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, body)
	}
	if recorder := serve(t, &repositoryStub{}, "/api/v1/data/status"); recorder.Code != http.StatusNotFound {
		t.Fatalf("the public data status endpoint should be gone, got %d", recorder.Code)
	}
	if recorder := serve(t, &repositoryStub{}, "/api/v1/admin/me"); recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without a database, got %d", recorder.Code)
	}
}

func TestFeedPagesWithCursor(t *testing.T) {
	repository := &repositoryStub{}
	recorder := serve(t, repository, "/api/v1/feed?city=a&district=大安區&sort=date_desc")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var first struct {
		Data []domain.Transaction `json:"data"`
		Meta struct {
			Total         int             `json:"total"`
			NewCount      int             `json:"newCount"`
			NextCursor    string          `json:"nextCursor"`
			LatestRelease *domain.Release `json:"latestRelease"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(first.Data) != 50 || first.Meta.Total != 60 || first.Meta.NewCount != 2 || first.Meta.NextCursor == "" ||
		first.Meta.LatestRelease == nil || first.Meta.LatestRelease.Label != "1150901-1150910" || !first.Data[0].IsNew {
		t.Fatalf("unexpected first page: %d rows, meta %+v", len(first.Data), first.Meta)
	}
	if repository.feedQuery.CityCode != "a" || repository.feedQuery.District != "大安區" {
		t.Fatalf("unexpected feed query %#v", repository.feedQuery)
	}

	recorder = serve(t, repository, "/api/v1/feed?city=a&cursor="+first.Meta.NextCursor)
	var second struct {
		Data []domain.Transaction `json:"data"`
		Meta map[string]any       `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Totals come with the first page only; the last page has no cursor.
	if len(second.Data) != 10 || second.Data[0].ID != "T50" || second.Meta["total"] != nil || second.Meta["nextCursor"] != nil {
		t.Fatalf("unexpected second page: %d rows from %s, meta %v", len(second.Data), second.Data[0].ID, second.Meta)
	}
}

func TestFeedRejectsInvalidInput(t *testing.T) {
	for _, target := range []string{
		"/api/v1/feed?cursor=not-base64!",
		"/api/v1/feed?sort=random",
		"/api/v1/feed?district=大安區",
		"/api/v1/feed?city=a&district=板橋區",
		"/api/v1/feed?city=e",
	} {
		if recorder := serve(t, &repositoryStub{}, target); recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", target, recorder.Code)
		}
	}
}

func TestExportCSV(t *testing.T) {
	recorder := serve(t, &repositoryStub{}, "/api/v1/export.csv?types=公寓")
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.HasPrefix(body, "\ufeff編號,縣市代碼,行政區,地址,成交日") {
		t.Fatalf("unexpected export %d: %.120s", recorder.Code, body)
	}
	if !strings.Contains(recorder.Header().Get("Content-Disposition"), "geoprophet-transactions-") {
		t.Fatalf("missing attachment header: %v", recorder.Header())
	}
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 4 || !strings.HasSuffix(lines[1], ",Y") || strings.HasSuffix(lines[3], ",Y") {
		t.Fatalf("expected a header and 3 rows with the first flagged new, got %q", lines)
	}
}

func TestRegions(t *testing.T) {
	recorder := serve(t, &repositoryStub{}, "/api/v1/regions")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"code":"a","name":"台北市","districts":["松山區","信義區"`) {
		t.Fatalf("unexpected regions: %.200s", recorder.Body.String())
	}
}

func TestDistrictBoundariesServeGeoJSON(t *testing.T) {
	recorder := serve(t, &repositoryStub{}, "/api/v1/districts/boundaries?city=a")
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"type":"FeatureCollection"`) ||
		!strings.Contains(body, `"geometry":{"coordinates":[[[[121.53,25.03]`) || !strings.Contains(body, `"district":"大安區"`) {
		t.Fatalf("unexpected response %d: %.300s", recorder.Code, body)
	}
	if recorder.Header().Get("Cache-Control") == "" {
		t.Fatal("expected a cache header")
	}
	if recorder = serve(t, &repositoryStub{}, "/api/v1/districts/boundaries?city=e"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected an unknown city to be rejected, got %d", recorder.Code)
	}
}

func TestCellDetailValidatesHex(t *testing.T) {
	for _, target := range []string{"/api/v1/aggregations/h3/cell", "/api/v1/aggregations/h3/cell?hex=zzz", "/api/v1/aggregations/h3/cell?hex=8a4ba0a4b2dffff&types=整層住家"} {
		if recorder := serve(t, &repositoryStub{}, target); recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", target, recorder.Code)
		}
	}
	// A valid resolution-9 cell with no sales still answers.
	if recorder := serve(t, &repositoryStub{}, "/api/v1/aggregations/h3/cell?hex=894ba0a4b2fffff"); recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"count":0`) {
		t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestTransitLinesServeGeoJSON(t *testing.T) {
	recorder := serve(t, &repositoryStub{}, "/api/v1/transit/lines")
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"geometry":{"type":"MultiLineString"`) ||
		!strings.Contains(body, `"openedOn":"2020-01-31"`) || !strings.Contains(body, `"color":"#fedb00"`) {
		t.Fatalf("unexpected response %d: %.300s", recorder.Code, body)
	}
	if recorder = serve(t, &repositoryStub{}, "/api/v1/transit/stations?city=e"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected an unknown city to be rejected, got %d", recorder.Code)
	}
}

func TestStationTrendComparesRings(t *testing.T) {
	repository := &repositoryStub{}
	recorder := serve(t, repository, "/api/v1/transit/stations/TRTC-BL07/trend?bbox=121.4,24.9,121.7,25.2&from=2026-01&to=2026-03")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if repository.stationFilter.From.Format("2006-01") != "2026-01" || repository.stationFilter.Bounds != nil {
		t.Fatalf("expected the requested window and no bbox, got %+v", repository.stationFilter)
	}
	if got := repository.yoyWindow[1].Format("2006-01") + "/" + repository.yoyWindow[2].Format("2006-01"); got != "2025-03/2026-02" {
		t.Fatalf("unexpected YoY window %s", got)
	}
	var response struct{ Data domain.StationTrend }
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	rings := response.Data.Rings
	if response.Data.YoYThrough != "2026-02" || len(rings) != 3 || rings[0].Radius != 300 {
		t.Fatalf("unexpected trend %+v", response.Data)
	}
	// 300 m has too few sales for a YoY; 500 m rose 10%; 800 m has no sales a year ago.
	if rings[0].PriceYoY != nil || rings[1].PriceYoY == nil || fmt.Sprintf("%.2f", *rings[1].PriceYoY) != "0.10" || rings[2].PriceYoY != nil {
		t.Fatalf("unexpected YoY %v %v %v", rings[0].PriceYoY, rings[1].PriceYoY, rings[2].PriceYoY)
	}
	if rings[1].Trends[0].Provisional || !rings[1].Trends[1].Provisional {
		t.Fatalf("expected only months after 2026-02 to be provisional: %+v", rings[1].Trends)
	}

	if recorder = serve(t, repository, "/api/v1/transit/stations/NOPE/trend"); recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown station, got %d", recorder.Code)
	}
}

func TestStationYoYsColourEveryStation(t *testing.T) {
	repository := &repositoryStub{}
	recorder := serve(t, repository, "/api/v1/transit/stations/yoy?radius=500&types=公寓")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct{ Data domain.StationYoYs }
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	stations := response.Data.Stations
	if response.Data.YoYThrough != "2026-02" || len(stations) != 2 || stations[0].StationID != "NTMC-Y08" ||
		stations[0].PriceYoY != nil || stations[1].PriceYoY == nil || stations[1].Count != 10 {
		t.Fatalf("unexpected response %+v", response.Data)
	}
	if !slices.Equal(repository.yoyFilter.Categories, []string{"公寓"}) {
		t.Fatalf("expected the building type filter, got %+v", repository.yoyFilter)
	}
	if recorder = serve(t, repository, "/api/v1/transit/stations/yoy?radius=400"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected an unsupported radius to be rejected, got %d", recorder.Code)
	}
}

func TestStationRankingMergesYoY(t *testing.T) {
	repository := &repositoryStub{}
	recorder := serve(t, repository, "/api/v1/transit/stations/ranking?radius=500&from=2025-03&to=2026-02")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct{ Data domain.StationRanking }
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	stations := response.Data.Stations
	// Sorted by ID; YoY from StationYoYTotals, where NTMC-Y08 has too few recent sales.
	if response.Data.YoYThrough != "2026-02" || len(stations) != 2 || stations[0].StationID != "NTMC-Y08" ||
		stations[0].PriceYoY != nil || stations[1].PriceYoY == nil || !stations[1].Trend[1].Provisional {
		t.Fatalf("unexpected ranking %+v", response.Data)
	}
	if repository.stationFilter.From.Format("2006-01") != "2025-03" {
		t.Fatalf("expected the requested window, got %+v", repository.stationFilter)
	}
	if recorder = serve(t, repository, "/api/v1/transit/stations/ranking?radius=400"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected an unsupported radius to be rejected, got %d", recorder.Code)
	}
}

func TestOpeningEffectsCompareWithTheDistrict(t *testing.T) {
	repository := &repositoryStub{}
	recorder := serve(t, repository, "/api/v1/transit/openings?radius=500&from=2025-01")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct{ Data domain.OpeningEffects }
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	events := response.Data.Events
	if len(events) != 3 || repository.stationFilter.From != nil || repository.stationFilter.ExcludeNewBuildings {
		t.Fatalf("expected three events over all dates, got %d with filter %+v", len(events), repository.stationFilter)
	}
	shisizhang := events[0]
	if shisizhang.Status != "ok" || shisizhang.Relative == nil || fmt.Sprintf("%.2f", *shisizhang.Relative) != "0.10" ||
		fmt.Sprintf("%.2f", *shisizhang.Station.Change) != "0.20" || shisizhang.Station.BeforeCount != 12 {
		t.Fatalf("unexpected 十四張 effect %+v", shisizhang)
	}
	// The chart covers 36 months before the opening, up to the last complete month.
	if len(shisizhang.Series) != 72 || shisizhang.Series[0].Offset != -36 || shisizhang.Series[0].Month != "2017-01" {
		t.Fatalf("unexpected series of %d points starting %+v", len(shisizhang.Series), shisizhang.Series[0])
	}
	if events[1].Status != "insufficient" || events[1].Relative != nil {
		t.Fatalf("expected 雙城 to lack sales, got %+v", events[1])
	}
	if events[2].Status != "too_recent" {
		t.Fatalf("expected 三峽 to be too recent, got %s", events[2].Status)
	}
}

func TestOpeningEffectsCanExcludeNewBuildings(t *testing.T) {
	repository := &repositoryStub{}
	if recorder := serve(t, repository, "/api/v1/transit/openings?radius=500&exclude_new=true"); recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !repository.stationFilter.ExcludeNewBuildings {
		t.Fatal("expected exclude_new to reach the repository")
	}
	if recorder := serve(t, repository, "/api/v1/transit/openings?exclude_new=maybe"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected an invalid boolean to be rejected, got %d", recorder.Code)
	}
}

func TestDistanceCurveUsesTheWindow(t *testing.T) {
	repository := &repositoryStub{}
	recorder := serve(t, repository, "/api/v1/transit/distance-curve?from=2025-10&exclude_new=true&bbox=121.4,24.9,121.7,25.2")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"fromM":800,"toM":null`) {
		t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
	}
	if filter := repository.stationFilter; filter.From.Format("2006-01") != "2025-10" || !filter.ExcludeNewBuildings || filter.Bounds != nil {
		t.Fatalf("unexpected filter %+v", filter)
	}
}

func TestAdminJobsAreQueuedOnlyBySameOriginAdmins(t *testing.T) {
	post := func(body, contentType string, headers map[string]string) *http.Request {
		request := httptest.NewRequest(http.MethodPost, "http://geoprophet.test/api/v1/admin/jobs", strings.NewReader(body))
		request.Header.Set("Content-Type", contentType)
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		return request
	}
	sameOrigin := map[string]string{"Origin": "http://geoprophet.test:3000", "Sec-Fetch-Site": "same-origin"}

	recorder := serveAdminRequest(t, post(`{"kind":"sync"}`, "application/json", sameOrigin), "admin@example.com")
	if recorder.Code != http.StatusCreated || !strings.Contains(recorder.Body.String(), `"requestedBy":"admin@example.com"`) {
		t.Fatalf("expected the job to be queued by the admin, got %d: %s", recorder.Code, recorder.Body.String())
	}
	cases := []struct {
		name   string
		req    *http.Request
		login  string
		status int
	}{
		{"cross-site fetch", post(`{"kind":"sync"}`, "application/json", map[string]string{"Sec-Fetch-Site": "cross-site"}), "admin@example.com", http.StatusForbidden},
		{"other origin", post(`{"kind":"sync"}`, "application/json", map[string]string{"Origin": "http://evil.example"}), "admin@example.com", http.StatusForbidden},
		{"form post", post("kind=sync", "application/x-www-form-urlencoded", sameOrigin), "admin@example.com", http.StatusUnsupportedMediaType},
		{"not an admin", post(`{"kind":"sync"}`, "application/json", sameOrigin), "someone@example.com", http.StatusForbidden},
		{"unknown kind", post(`{"kind":"drop_tables"}`, "application/json", sameOrigin), "admin@example.com", http.StatusBadRequest},
		{"already queued", post(`{"kind":"transit"}`, "application/json", sameOrigin), "admin@example.com", http.StatusConflict},
	}
	for _, test := range cases {
		if recorder := serveAdminRequest(t, test.req, test.login); recorder.Code != test.status {
			t.Errorf("%s: got %d, want %d: %s", test.name, recorder.Code, test.status, recorder.Body.String())
		}
	}
	if recorder := serveAdmin(t, "/api/v1/admin/jobs", "admin@example.com"); recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"error":"502 Bad Gateway"`) {
		t.Fatalf("unexpected job list %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCORSStillGuardsOtherOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := delivery.NewRouter(delivery.NewHandler(services(&repositoryStub{})), []string{"http://localhost:3000"})
	for origin, want := range map[string]int{
		"http://geoprophet.test:3000": http.StatusOK,        // same host, e.g. through nginx
		"http://localhost:3000":       http.StatusOK,        // listed
		"http://evil.example":         http.StatusForbidden, // anything else
	} {
		request := httptest.NewRequest(http.MethodGet, "http://geoprophet.test/api/v1/regions", nil)
		request.Header.Set("Origin", origin)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != want {
			t.Errorf("Origin %s: got %d, want %d", origin, recorder.Code, want)
		}
	}
}

func TestAdminOpeningsAreEditedBySameOriginAdmins(t *testing.T) {
	put := func(body string) *http.Request {
		request := httptest.NewRequest(http.MethodPut, "http://geoprophet.test/api/v1/admin/openings", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		return request
	}
	for body, want := range map[string]int{
		`{"kind":"line","targetId":"NTMC-Y","openedOn":"2020-01-31","source":"https://example.test"}`: http.StatusOK,
		`{"kind":"line","targetId":"NTMC-Y","openedOn":null}`:                                         http.StatusOK, // clear
		`{"kind":"line","targetId":"NOPE-X","openedOn":"2020-01-31"}`:                                 http.StatusNotFound,
		`{"kind":"line","targetId":"NTMC-Y","openedOn":"2020/01/31"}`:                                 http.StatusBadRequest,
		`{"kind":"station","targetId":"NTMC-Y","openedOn":"2020-01-31"}`:                              http.StatusBadRequest,
	} {
		if recorder := serveAdminRequest(t, put(body), "admin@example.com"); recorder.Code != want {
			t.Errorf("%s: got %d, want %d: %s", body, recorder.Code, want, recorder.Body.String())
		}
	}
	if recorder := serveAdminRequest(t, put(`{"kind":"line","targetId":"NTMC-Y","openedOn":"2020-01-31"}`), "someone@example.com"); recorder.Code != http.StatusForbidden {
		t.Fatalf("non-admins must not edit openings, got %d", recorder.Code)
	}
	if recorder := serveAdmin(t, "/api/v1/admin/openings", "admin@example.com"); recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"openedOn":"2020-01-31"`) {
		t.Fatalf("unexpected openings %d: %s", recorder.Code, recorder.Body.String())
	}
}

type rentalRepositoryStub struct{ query domain.RentalQuery }

func (*rentalRepositoryStub) ImportedSalesByCity(context.Context, string) ([]string, []string, error) {
	return []string{"115S2"}, nil, nil
}

func (*rentalRepositoryStub) ImportedRentalsByCity(context.Context, string) ([]string, []string, error) {
	return []string{"115S2"}, nil, nil
}

func (stub *rentalRepositoryStub) RentalAggregates(_ context.Context, query domain.RentalQuery, _, _ time.Time) ([]domain.RentalAggregate, error) {
	stub.query = query
	return nil, nil
}

func (*rentalRepositoryStub) RentTrend(context.Context, domain.RentalQuery, string, time.Time, time.Time) ([]domain.RentPoint, error) {
	return nil, nil
}

func (*rentalRepositoryStub) StationDistrict(_ context.Context, stationID string) (string, string, bool, error) {
	return "a", "中山區", stationID == "TRTC-R11", nil
}

func (*rentalRepositoryStub) StationRentRings(context.Context, string, string, domain.RentalQuery, time.Time, time.Time) ([]domain.RentalAggregate, *float64, error) {
	return make([]domain.RentalAggregate, 3), nil, nil
}

func (*rentalRepositoryStub) StationRentTrends(context.Context, string, domain.RentalQuery, time.Time, time.Time) ([][]domain.RentPoint, error) {
	return make([][]domain.RentPoint, 3), nil
}

func TestRentalDistrictsParsesFilters(t *testing.T) {
	if recorder := serve(t, &repositoryStub{}, "/api/v1/rentals/districts"); recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("without a database: got %d", recorder.Code)
	}
	gin.SetMode(gin.TestMode)
	stub := &rentalRepositoryStub{}
	wired := services(&repositoryStub{})
	wired.Rentals = usecase.NewRentalService(stub)
	router := delivery.NewRouter(delivery.NewHandler(wired), []string{"http://localhost:3000"})
	for target, want := range map[string]int{
		"/api/v1/rentals/districts?city=f&type=華廈&min_area=20&max_area=40&include_social=true": http.StatusOK,
		"/api/v1/rentals/districts?include_social=maybe":                                       http.StatusBadRequest,
		"/api/v1/rentals/districts?type=店面":                                                    http.StatusBadRequest,
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != want {
			t.Errorf("%s: got %d, want %d", target, recorder.Code, want)
		}
	}
	if want := (domain.RentalQuery{CityCode: "f", Category: "華廈", MinArea: 20, MaxArea: 40, IncludeSocialHousing: true}); stub.query != want {
		t.Fatalf("unexpected query %+v", stub.query)
	}
}

type estimateRepositoryStub struct{ query domain.EstimateQuery }

func (*estimateRepositoryStub) ImportedSalesByCity(context.Context, string) ([]string, []string, error) {
	return []string{"115S2"}, nil, nil
}

func (stub *estimateRepositoryStub) EstimateCandidates(_ context.Context, query domain.EstimateQuery, _, _, _ float64, _, _ time.Time) ([]domain.ComparableSale, error) {
	stub.query = query
	return nil, nil
}

func (*estimateRepositoryStub) PriceIndex(context.Context, string, time.Time, time.Time) ([]domain.PriceIndexPoint, error) {
	return nil, nil
}

func (*estimateRepositoryStub) ParkingPrice(context.Context, string, string, string, time.Time, time.Time) (*float64, int, error) {
	return nil, 0, nil
}

func (*estimateRepositoryStub) ImportedRentalsByCity(context.Context, string) ([]string, []string, error) {
	return nil, nil, nil
}

func (*estimateRepositoryStub) RentCandidates(context.Context, domain.EstimateQuery, float64, float64, float64, time.Time, time.Time) ([]domain.LeaseCandidate, error) {
	return nil, nil
}

func (*estimateRepositoryStub) RentTrend(context.Context, domain.RentalQuery, string, time.Time, time.Time) ([]domain.RentPoint, error) {
	return nil, nil
}

func (*estimateRepositoryStub) RentalAggregates(context.Context, domain.RentalQuery, time.Time, time.Time) ([]domain.RentalAggregate, error) {
	return nil, nil
}

func (*estimateRepositoryStub) BuildingProfile(_ context.Context, building string, _, _ float64) (domain.BuildingProfile, error) {
	return domain.BuildingProfile{Address: building}, nil
}

func TestEstimateParsesTheHome(t *testing.T) {
	if recorder := serve(t, &repositoryStub{}, "/api/v1/estimate?lat=25&lng=121.5"); recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("without a database: got %d", recorder.Code)
	}
	gin.SetMode(gin.TestMode)
	stub := &estimateRepositoryStub{}
	wired := services(&repositoryStub{})
	wired.Estimates = usecase.NewEstimateService(stub)
	router := delivery.NewRouter(delivery.NewHandler(wired), []string{"http://localhost:3000"})
	for target, want := range map[string]int{
		"/api/v1/estimate?address=台北市大安區仁愛路四段27巷3號&lat=25.0389&lng=121.5453&type=華廈&area=32.5&age=53&floor=2&total_floors=6&parking=true": http.StatusOK,
		"/api/v1/estimate?lat=25.0389&type=華廈&area=32.5":                              http.StatusBadRequest,
		"/api/v1/estimate?lat=25.0389&lng=121.5453&type=華廈&area=32.5&age=x":           http.StatusBadRequest,
		"/api/v1/estimate/building?address=台北市大安區仁愛路四段27巷3號&lat=25.0389&lng=121.5453": http.StatusOK,
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != want {
			t.Errorf("%s: got %d, want %d", target, recorder.Code, want)
		}
	}
	if stub.query.Age == nil || *stub.query.Age != 53 || stub.query.Floor != 2 || stub.query.TotalFloors != 6 || !stub.query.Parking || stub.query.AreaPing != 32.5 {
		t.Fatalf("unexpected query %+v", stub.query)
	}
}

type presaleRepositoryStub struct{ district string }

func (*presaleRepositoryStub) ImportedSalesByCity(context.Context, string) ([]string, []string, error) {
	return nil, nil, nil
}

func (stub *presaleRepositoryStub) PresaleProjects(_ context.Context, query domain.PresaleProjectQuery, _ time.Time) ([]domain.PresaleProject, error) {
	stub.district = query.District
	return nil, nil
}

func (*presaleRepositoryStub) NearbyPresaleProjects(context.Context, domain.NearPoint, time.Time, time.Time) ([]domain.PresaleProject, error) {
	return nil, nil
}

func (*presaleRepositoryStub) BoundedPresaleProjects(context.Context, domain.Bounds, time.Time, time.Time) ([]domain.PresaleProject, error) {
	return nil, nil
}

func (*presaleRepositoryStub) PresaleProject(_ context.Context, key string, _ time.Time) (domain.PresaleProject, bool, error) {
	return domain.PresaleProject{Key: key}, key == "a|中山區|測試", nil
}

func (*presaleRepositoryStub) PresaleUnits(context.Context, string) ([]domain.Presale, error) {
	return nil, nil
}

func (*presaleRepositoryStub) ResalesNearby(context.Context, float64, float64, float64, int, time.Time, time.Time) (int, *float64, int, *float64, error) {
	return 0, nil, 0, nil, nil
}

func TestPresaleRoutes(t *testing.T) {
	if recorder := serve(t, &repositoryStub{}, "/api/v1/presales/projects"); recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("without a database: got %d", recorder.Code)
	}
	gin.SetMode(gin.TestMode)
	stub := &presaleRepositoryStub{}
	wired := services(&repositoryStub{})
	wired.Presales = usecase.NewPresaleService(stub)
	router := delivery.NewRouter(delivery.NewHandler(wired), []string{"http://localhost:3000"})
	for target, want := range map[string]int{
		"/api/v1/presales/projects?city=a&district=" + url.QueryEscape("中山區") + "&months=36": http.StatusOK,
		"/api/v1/presales/projects?months=x":                          http.StatusBadRequest,
		"/api/v1/presales/project?key=" + url.QueryEscape("a|中山區|測試"): http.StatusOK,
		"/api/v1/presales/project?key=nope":                           http.StatusNotFound,
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != want {
			t.Errorf("%s: got %d, want %d", target, recorder.Code, want)
		}
	}
	if stub.district != "中山區" {
		t.Fatalf("unexpected district %q", stub.district)
	}
}

type facilityRepositoryStub struct{ categories []string }

func (stub *facilityRepositoryStub) Facilities(_ context.Context, query domain.FacilityQuery, _ int) ([]domain.Facility, error) {
	stub.categories = query.Categories
	return nil, nil
}

func (*facilityRepositoryStub) DistrictFacilities(context.Context, string) ([]domain.DistrictFacilities, error) {
	return nil, nil
}

func (*facilityRepositoryStub) NearbyFacilities(context.Context, float64, float64, float64, float64) ([]domain.NearbyCategory, error) {
	return nil, nil
}

func TestFacilityRoutes(t *testing.T) {
	if recorder := serve(t, &repositoryStub{}, "/api/v1/facilities/nearby?lat=25.05&lng=121.52"); recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("without a database: got %d", recorder.Code)
	}
	gin.SetMode(gin.TestMode)
	stub := &facilityRepositoryStub{}
	wired := services(&repositoryStub{})
	wired.Facilities = usecase.NewFacilityService(stub)
	router := delivery.NewRouter(delivery.NewHandler(wired), []string{"http://localhost:3000"})
	for target, want := range map[string]int{
		"/api/v1/facilities?bbox=121.5,25.0,121.6,25.1&categories=park,clinic": http.StatusOK,
		"/api/v1/facilities?bbox=121.5,25.0":                                   http.StatusBadRequest,
		"/api/v1/facilities/nearby?lat=25.05&lng=121.52&radius=500":            http.StatusOK,
		"/api/v1/facilities/nearby?lat=25.05":                                  http.StatusBadRequest,
		"/api/v1/facilities/density?bbox=121.5,25.0,121.6,25.1&res=8":          http.StatusOK,
		"/api/v1/facilities/density?bbox=121.5,25.0,121.6,25.1&res=12":         http.StatusBadRequest,
		"/api/v1/facilities/districts?city=a":                                  http.StatusOK,
		"/api/v1/facilities/districts?city=e":                                  http.StatusBadRequest,
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != want {
			t.Errorf("%s: got %d, want %d", target, recorder.Code, want)
		}
	}
}
