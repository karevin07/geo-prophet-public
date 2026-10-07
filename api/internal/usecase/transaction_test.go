package usecase_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
)

type repositoryStub struct {
	points        []domain.PricePoint
	filter        domain.TransactionFilter
	districtQuery domain.DistrictComparisonQuery
	yoyTotals     *domain.DistrictYoYTotals
	cellRows      []domain.CellRow
}

func (repository *repositoryStub) List(_ context.Context, filter domain.TransactionFilter) ([]domain.Transaction, error) {
	repository.filter = filter
	return nil, nil
}

func (repository *repositoryStub) ForEachPricePoint(_ context.Context, filter domain.TransactionFilter, emit func(domain.PricePoint) error) error {
	repository.filter = filter
	for _, point := range repository.points {
		if err := emit(point); err != nil {
			return err
		}
	}
	return nil
}

func (repository *repositoryStub) MonthlyTrends(_ context.Context, filter domain.TransactionFilter) ([]domain.TrendPoint, error) {
	repository.filter = filter
	return nil, nil
}

func (repository *repositoryStub) Summary(_ context.Context, filter domain.TransactionFilter) (domain.MarketSummary, error) {
	repository.filter = filter
	return domain.MarketSummary{Count: 3}, nil
}

func (repository *repositoryStub) Analysis(_ context.Context, filter domain.TransactionFilter) (domain.MarketAnalysis, error) {
	repository.filter = filter
	return domain.MarketAnalysis{Monthly: []domain.MonthlyAnalysis{{Month: "2026-09", Count: 3}}}, nil
}

func (repository *repositoryStub) DistrictRankings(_ context.Context, query domain.DistrictComparisonQuery) ([]domain.DistrictRank, error) {
	repository.districtQuery = query
	return []domain.DistrictRank{{District: "大安區", Count: 10}}, nil
}

func (*repositoryStub) DistrictTrends(_ context.Context, _ domain.DistrictComparisonQuery) ([]domain.DistrictTrend, error) {
	return []domain.DistrictTrend{{District: "大安區", Month: "2026-09", Count: 3}}, nil
}

func (repository *repositoryStub) DistrictYoYTotals(_ context.Context, _ domain.DistrictComparisonQuery, _, _, _ time.Time) (map[string]domain.DistrictYoYTotals, error) {
	if repository.yoyTotals != nil {
		return map[string]domain.DistrictYoYTotals{"大安區": *repository.yoyTotals}, nil
	}
	return map[string]domain.DistrictYoYTotals{"大安區": {
		CurrentCount: 10, PreviousCount: 10, CurrentBuildingPrice: 105_000_000, PreviousBuildingPrice: 100_000_000,
		CurrentArea: 100, PreviousArea: 100,
	}}, nil
}

func TestCompareDistrictsHidesLowSampleYoY(t *testing.T) {
	repository := &repositoryStub{yoyTotals: &domain.DistrictYoYTotals{
		CurrentCount: 4, PreviousCount: 20, CurrentBuildingPrice: 100, PreviousBuildingPrice: 100,
		CurrentArea: 10, PreviousArea: 10,
	}}
	result, err := usecase.NewTransactionService(repository).CompareDistricts(context.Background(), domain.DistrictComparisonQuery{
		Filter: validFilter(), CityCode: "a", Districts: []string{"大安區"},
	})
	if err != nil || result.Ranking[0].PriceYoY != nil {
		t.Fatalf("expected no YoY with fewer than five sales, got %#v, %v", result, err)
	}
}

func (*repositoryStub) ImportedSales(context.Context) ([]string, []string, error) {
	return []string{"115S2"}, nil, nil // registrations complete through 2026-02
}

func (*repositoryStub) DistrictBoundaries(context.Context, string) ([]domain.DistrictBoundary, error) {
	return []domain.DistrictBoundary{{District: "大安區", Cells: 2, Coordinates: [][][][]float64{{{{121.53, 25.03}, {121.54, 25.03}, {121.54, 25.04}, {121.53, 25.03}}}}}}, nil
}

func (repository *repositoryStub) CellRows(_ context.Context, filter domain.TransactionFilter) ([]domain.CellRow, error) {
	repository.filter = filter
	return repository.cellRows, nil
}

func (*repositoryStub) Ping(context.Context) error { return nil }

func TestAggregateH3ExcludesParkingFromUnitPrice(t *testing.T) {
	repository := &repositoryStub{points: []domain.PricePoint{
		{Lat: 25.033, Lng: 121.5654, TotalPrice: 12_000_000, BuildingPrice: 10_000_000, BuildingAreaPing: 20},
		{Lat: 25.03301, Lng: 121.56541, TotalPrice: 20_000_000, BuildingPrice: 20_000_000, BuildingAreaPing: 30},
	}}
	service := usecase.NewTransactionService(repository)

	result, err := service.AggregateH3(context.Background(), validFilter(), 9)
	if err != nil {
		t.Fatalf("AggregateH3 returned an error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected one aggregate, got %d", len(result))
	}
	if result[0].Count != 2 || result[0].Price != 16_000_000 || result[0].AvgArea != 25 || result[0].AvgPricePerPing != 600_000 {
		t.Fatalf("unexpected aggregate: %#v", result[0])
	}
	if !repository.filter.PreciseLocation {
		t.Fatal("expected district-level locations to be excluded from the map")
	}
}

func TestAggregateH3DoesNotTruncateAtTwoHundredThousand(t *testing.T) {
	const count = 200_001
	points := make([]domain.PricePoint, count)
	for index := range points {
		points[index] = domain.PricePoint{Lat: 25.033, Lng: 121.5654, TotalPrice: 12_000_000, BuildingPrice: 10_000_000, BuildingAreaPing: 20}
	}
	repository := &repositoryStub{points: points}
	result, err := usecase.NewTransactionService(repository).AggregateH3(context.Background(), validFilter(), 9)
	if err != nil || len(result) != 1 || result[0].Count != count {
		t.Fatalf("expected all %d sales in one H3 cell, got %#v, %v", count, result, err)
	}
}

func TestAggregateH3RejectsInvalidResolution(t *testing.T) {
	service := usecase.NewTransactionService(&repositoryStub{})
	if _, err := service.AggregateH3(context.Background(), validFilter(), 12); !errors.Is(err, usecase.ErrInvalidInput) {
		t.Fatalf("expected an invalid resolution error, got %v", err)
	}
}

func TestListClampsLimit(t *testing.T) {
	repository := &repositoryStub{}
	filter := validFilter()
	filter.Limit = 100_000
	if _, err := usecase.NewTransactionService(repository).List(context.Background(), filter); err != nil {
		t.Fatalf("List returned an error: %v", err)
	}
	if repository.filter.Limit != 500 || !repository.filter.PreciseLocation {
		t.Fatalf("unexpected repository filter: %#v", repository.filter)
	}
}

func TestTrendsKeepsDistrictLevelLocations(t *testing.T) {
	repository := &repositoryStub{}
	if _, err := usecase.NewTransactionService(repository).Trends(context.Background(), validFilter()); err != nil {
		t.Fatalf("Trends returned an error: %v", err)
	}
	if repository.filter.PreciseLocation {
		t.Fatal("trends should count every located transaction")
	}
}

func TestAnalysisKeepsDistrictLevelLocations(t *testing.T) {
	repository := &repositoryStub{}
	result, err := usecase.NewTransactionService(repository).Analysis(context.Background(), validFilter())
	if err != nil || len(result.Monthly) != 1 {
		t.Fatalf("unexpected analysis result %#v, %v", result, err)
	}
	if repository.filter.PreciseLocation {
		t.Fatal("analysis should count every located transaction")
	}
}

func TestCompareDistrictsValidatesSelectionAndMarksCoverage(t *testing.T) {
	repository := &repositoryStub{}
	service := usecase.NewTransactionService(repository)
	query := domain.DistrictComparisonQuery{Filter: validFilter(), CityCode: "a", Districts: []string{"大安區"}}
	result, err := service.CompareDistricts(context.Background(), query)
	if err != nil || len(result.Ranking) != 1 || !result.Trends[0].Provisional || result.YoYThrough != "2026-02" ||
		result.Ranking[0].PriceYoY == nil || math.Abs(*result.Ranking[0].PriceYoY-0.05) > 1e-9 {
		t.Fatalf("unexpected comparison: %#v, %v", result, err)
	}
	if repository.districtQuery.Filter.PreciseLocation {
		t.Fatal("district comparison must include district-level geocodes")
	}
	for _, districts := range [][]string{{"大安區", "大安區"}, {"不存在區"}, {"信義區", "大安區", "松山區", "中山區", "中正區"}} {
		query.Districts = districts
		if _, err := service.CompareDistricts(context.Background(), query); !errors.Is(err, usecase.ErrInvalidInput) {
			t.Fatalf("expected invalid district selection %v, got %v", districts, err)
		}
	}
}

func TestValidateFilterRejectsUnknownBuildingType(t *testing.T) {
	filter := validFilter()
	filter.Categories = []string{"整層住家"}
	if _, err := usecase.NewTransactionService(&repositoryStub{}).List(context.Background(), filter); !errors.Is(err, usecase.ErrInvalidInput) {
		t.Fatalf("expected an invalid input error, got %v", err)
	}
}

func validFilter() domain.TransactionFilter {
	return domain.TransactionFilter{MinPrice: 0, MaxPrice: 100_000_000, MinArea: 0, MaxArea: 200}
}
