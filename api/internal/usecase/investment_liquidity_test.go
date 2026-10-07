package usecase

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type liquidityRepositoryStub struct {
	seasons, releases []string
	rows              []domain.InvestmentMonthlyCount
	query             domain.InvestmentQuery
	from, to          time.Time
	err               error
	queried           bool
}

func (stub *liquidityRepositoryStub) ImportedSalesByCity(context.Context, string) ([]string, []string, error) {
	return stub.seasons, stub.releases, stub.err
}

func (stub *liquidityRepositoryStub) InvestmentMonthlyCounts(_ context.Context, query domain.InvestmentQuery, from, to time.Time) ([]domain.InvestmentMonthlyCount, error) {
	stub.query, stub.from, stub.to, stub.queried = query, from, to, true
	return stub.rows, nil
}

func liquidityFixture() *liquidityRepositoryStub {
	return &liquidityRepositoryStub{
		seasons: []string{"113S1", "113S2", "113S3", "113S4", "114S1", "114S2", "114S3", "114S4", "115S1", "115S2"},
		rows: []domain.InvestmentMonthlyCount{
			{District: "甲區", Month: "2024-03", Count: 15}, {District: "甲區", Month: "2025-02", Count: 15},
			{District: "甲區", Month: "2025-03", Count: 10}, {District: "甲區", Month: "2025-06", Count: 10}, {District: "甲區", Month: "2026-02", Count: 10},
			{District: "乙區", Month: "2025-03", Count: 29}, {District: "乙區", Month: "2024-03", Count: 60},
		},
	}
}

var liquidityQuery = domain.InvestmentQuery{CityCode: "a", Category: "住宅大樓", MinArea: 15, MaxArea: 45, MinAge: 5, MaxAge: 30}

func TestLiquidityFills24MonthsAndUsesCurrentPeriodOnly(t *testing.T) {
	stub := liquidityFixture()
	report, err := NewInvestmentLiquidityService(stub).Compare(context.Background(), liquidityQuery)
	if err != nil {
		t.Fatal(err)
	}
	if report.Through != "2026-02" || report.CurrentFrom != "2025-03" || report.PreviousFrom != "2024-03" ||
		!report.CurrentCoverageComplete || !report.PreviousCoverageComplete || len(report.IncompleteMonths) != 0 || stub.query != liquidityQuery ||
		stub.from.Format("2006-01-02") != "2024-03-01" || stub.to.Format("2006-01-02") != "2026-03-01" {
		t.Fatalf("unexpected report or query: %+v, %+v", report, stub)
	}
	area := report.Areas[0]
	if area.District != "甲區" || area.CurrentCount != 30 || area.PreviousCount != 30 || area.ActiveMonths == nil || *area.ActiveMonths != 3 ||
		area.LongestZeroRun == nil || *area.LongestZeroRun != 7 || area.VolumeChange == nil || *area.VolumeChange != 0 || len(area.Monthly) != 24 ||
		area.Monthly[0].Month != "2024-03" || area.Monthly[23].Month != "2026-02" || area.Monthly[1].Count != 0 || !area.Monthly[1].CoverageComplete {
		t.Fatalf("incorrect series or current metrics: %+v", area)
	}
	small := report.Areas[1]
	if small.CurrentCount != 29 || small.ActiveMonths != nil || small.LongestZeroRun != nil || small.VolumeChange != nil || small.SufficientCurrentSample {
		t.Fatalf("small sample exposed metrics: %+v", small)
	}
}

func TestLiquidityMissingImportsHideOnlyAffectedMetrics(t *testing.T) {
	for _, testCase := range []struct {
		missing string
		current bool
	}{{"113S3", false}, {"114S3", true}} {
		t.Run(testCase.missing, func(t *testing.T) {
			stub := liquidityFixture()
			stub.seasons = slices.DeleteFunc(stub.seasons, func(label string) bool { return label == testCase.missing })
			report, err := NewInvestmentLiquidityService(stub).Compare(context.Background(), liquidityQuery)
			if err != nil {
				t.Fatal(err)
			}
			area := report.Areas[0]
			if area.VolumeChange != nil || len(report.IncompleteMonths) == 0 {
				t.Fatalf("import gap exposed comparison: %+v", report)
			}
			if testCase.current {
				if report.CurrentCoverageComplete || area.ActiveMonths != nil || area.LongestZeroRun != nil {
					t.Fatalf("current gap exposed derived metrics: %+v", report)
				}
			} else if report.PreviousCoverageComplete || !report.CurrentCoverageComplete || area.ActiveMonths == nil {
				t.Fatalf("previous gap should keep current metrics: %+v", report)
			}
		})
	}
}

func TestLiquidityZeroIntervalsIncludeTrailingMonthsAndGatingPreviousSample(t *testing.T) {
	stub := liquidityFixture()
	stub.rows = []domain.InvestmentMonthlyCount{{District: "甲區", Month: "2025-03", Count: 40}, {District: "甲區", Month: "2024-03", Count: 29}}
	report, err := NewInvestmentLiquidityService(stub).Compare(context.Background(), liquidityQuery)
	if err != nil {
		t.Fatal(err)
	}
	area := report.Areas[0]
	if area.LongestZeroRun == nil || *area.LongestZeroRun != 11 || area.ActiveMonths == nil || *area.ActiveMonths != 1 || area.VolumeChange != nil || area.SufficientComparisonSample {
		t.Fatalf("wrong trailing interval or prior gating: %+v", area)
	}
	stub.rows[1].Count = 32
	report, err = NewInvestmentLiquidityService(stub).Compare(context.Background(), liquidityQuery)
	if err != nil || report.Areas[0].VolumeChange == nil || math.Abs(*report.Areas[0].VolumeChange-0.25) > 1e-9 {
		t.Fatalf("wrong volume change: %+v, %v", report, err)
	}
}

func TestLiquidityCoverageMergesArchivesAndReleases(t *testing.T) {
	windows := liquidityRegistrationWindows([]string{"115S2", "bad"}, []string{"1150611-1150620", "1150621-1150630", "1150621-1150630", "bad"})
	from := time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !liquidityWindowCovered(windows, from, to) || liquidityWindowCovered(windows, from.AddDate(0, 0, -1), to) || liquidityWindowCovered(windows, from, to.AddDate(0, 0, 1)) {
		t.Fatal("registration coverage must merge adjacent/overlapping windows and respect both boundaries")
	}
}

func TestLiquidityCoverageValidationAndRepositoryErrors(t *testing.T) {
	stub := &liquidityRepositoryStub{}
	service := NewInvestmentLiquidityService(stub)
	report, err := service.Compare(context.Background(), liquidityQuery)
	if err != nil || report.Through != "" || report.Areas == nil || stub.queried {
		t.Fatalf("unknown coverage: %+v, %v", report, err)
	}
	invalid := liquidityQuery
	invalid.MaxAge = invalid.MinAge
	if _, err := service.Compare(context.Background(), invalid); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid cohort: %v", err)
	}
	stub.err = errors.New("coverage query failed")
	if _, err := service.Compare(context.Background(), liquidityQuery); !errors.Is(err, stub.err) {
		t.Fatalf("repository error not propagated: %v", err)
	}
}
