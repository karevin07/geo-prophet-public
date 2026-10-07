package usecase

import (
	"context"
	"math"
	"testing"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type investmentRepositoryStub struct {
	seasons []string
	rows    []domain.InvestmentAggregate
	query   domain.InvestmentQuery
	window  domain.InvestmentWindow
}

func (stub *investmentRepositoryStub) ImportedSalesByCity(_ context.Context, _ string) ([]string, []string, error) {
	return stub.seasons, nil, nil
}

func (stub *investmentRepositoryStub) InvestmentAggregates(_ context.Context, query domain.InvestmentQuery, window domain.InvestmentWindow) ([]domain.InvestmentAggregate, error) {
	stub.query, stub.window = query, window
	return stub.rows, nil
}

func investmentPrice(value float64) *float64 { return &value }

func TestInvestmentCompareAreasGatesSmallSamples(t *testing.T) {
	repository := &investmentRepositoryStub{seasons: []string{"115S2"}, rows: []domain.InvestmentAggregate{
		{CurrentCount: 100, MedianPricePerPing: investmentPrice(800_000)},
		{District: "大安區", CurrentCount: 40, PreviousCount: 50, MedianPricePerPing: investmentPrice(900_000), PreviousMedianPricePerPing: investmentPrice(850_000)},
		{District: "中山區", CurrentCount: 12, PreviousCount: 40, MedianPricePerPing: investmentPrice(700_000), PreviousMedianPricePerPing: investmentPrice(680_000)},
	}}
	report, err := NewInvestmentService(repository).CompareAreas(context.Background(), domain.InvestmentQuery{
		CityCode: "a", Category: "住宅大樓", MinArea: 15, MaxArea: 45, MinAge: 5, MaxAge: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Through == "" || report.CityCurrentCount != 100 || len(report.Areas) != 2 ||
		repository.window.CurrentFrom.Sub(repository.window.PreviousFrom).Hours() < 24*360 {
		t.Fatalf("unexpected report/window: %+v, %+v", report, repository.window)
	}
	first, second := report.Areas[0], report.Areas[1]
	if first.District != "大安區" || !first.SufficientComparisonSample || first.VolumeChange == nil || math.Abs(*first.VolumeChange+0.2) > 1e-9 ||
		first.RelativeToCityMedian == nil || *first.RelativeToCityMedian != 0.125 {
		t.Fatalf("unexpected sufficient area: %+v", first)
	}
	if second.SufficientCurrentSample || second.MedianPricePerPing != nil || second.PriceChange != nil || second.RelativeToCityMedian != nil {
		t.Fatalf("small sample must hide price comparison: %+v", second)
	}
}

func TestInvestmentRequiresCoverageAndComparableCohort(t *testing.T) {
	service := NewInvestmentService(&investmentRepositoryStub{})
	valid := domain.InvestmentQuery{CityCode: "a", Category: "住宅大樓", MinArea: 15, MaxArea: 45, MinAge: 5, MaxAge: 30}
	report, err := service.CompareAreas(context.Background(), valid)
	if err != nil || report.Through != "" || len(report.Areas) != 0 {
		t.Fatalf("unknown coverage must be empty, got %+v, %v", report, err)
	}
	for _, query := range []domain.InvestmentQuery{
		{CityCode: "z", Category: valid.Category, MinArea: 15, MaxArea: 45, MinAge: 5, MaxAge: 30},
		{CityCode: "a", Category: "其他", MinArea: 15, MaxArea: 45, MinAge: 5, MaxAge: 30},
		{CityCode: "a", Category: valid.Category, MinArea: 45, MaxArea: 15, MinAge: 5, MaxAge: 30},
		{CityCode: "a", Category: valid.Category, MinArea: 15, MaxArea: 45, MinAge: 30, MaxAge: 5},
	} {
		if _, err := service.CompareAreas(context.Background(), query); err == nil {
			t.Fatalf("expected invalid input for %+v", query)
		}
	}
}
