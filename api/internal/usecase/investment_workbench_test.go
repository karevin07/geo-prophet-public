package usecase

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type workbenchStub struct {
	investmentRepositoryStub
	rentSeasons      []string
	rents            []domain.InvestmentRentAggregate
	rentFrom, rentTo time.Time
}

func (stub *workbenchStub) ImportedRentalsByCity(context.Context, string) ([]string, []string, error) {
	return stub.rentSeasons, nil, nil
}

func (stub *workbenchStub) InvestmentRentAggregates(_ context.Context, _ domain.InvestmentQuery, from, to time.Time) ([]domain.InvestmentRentAggregate, error) {
	stub.rentFrom, stub.rentTo = from, to
	return stub.rents, nil
}

func TestInvestmentWorkbenchMatchesWindowAndGatesYield(t *testing.T) {
	stub := &workbenchStub{
		investmentRepositoryStub: investmentRepositoryStub{seasons: []string{"115S2"}, rows: []domain.InvestmentAggregate{
			{District: "甲區", CurrentCount: 35, PreviousCount: 32, MedianPricePerPing: investmentPrice(600_000), PreviousMedianPricePerPing: investmentPrice(550_000)},
			{District: "乙區", CurrentCount: 29, PreviousCount: 35, MedianPricePerPing: investmentPrice(500_000)},
		}},
		rentSeasons: []string{"115S1"},
		rents:       []domain.InvestmentRentAggregate{{District: "甲區", Count: 20, MedianRentPerPing: investmentPrice(1000)}, {District: "乙區", Count: 50, MedianRentPerPing: investmentPrice(800)}},
	}
	query := domain.InvestmentQuery{CityCode: "a", Category: "住宅大樓", MinArea: 15, MaxArea: 45, MinAge: 5, MaxAge: 30}
	report, err := NewInvestmentWorkbenchService(stub).Compare(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if report.Through == "" || !stub.rentFrom.Equal(stub.window.CurrentFrom) || !stub.rentTo.Equal(stub.window.EndExclusive) {
		t.Fatalf("window mismatch: %+v, %+v", report, stub.window)
	}
	if report.Areas[0].GrossYield == nil || math.Abs(*report.Areas[0].GrossYield-0.02) > 1e-9 {
		t.Fatalf("unexpected yield: %+v", report.Areas[0])
	}
	if report.Areas[1].GrossYield != nil || report.Areas[1].MedianPricePerPing != nil {
		t.Fatalf("low sale sample exposed yield: %+v", report.Areas[1])
	}
}
