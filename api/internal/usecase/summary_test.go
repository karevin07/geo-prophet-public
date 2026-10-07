package usecase_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
)

// summaryStub answers Summary by window, keyed "YYYY-MM..YYYY-MM".
type summaryStub struct {
	repositoryStub
	windows map[string]domain.MarketSummary
	asked   []string
}

func (stub *summaryStub) Summary(_ context.Context, filter domain.TransactionFilter) (domain.MarketSummary, error) {
	key := filter.From.Format("2006-01") + ".." + filter.To.Format("2006-01")
	stub.asked = append(stub.asked, key)
	return stub.windows[key], nil
}

func window(from, to string) domain.TransactionFilter {
	start, _ := time.Parse("2006-01", from)
	month, _ := time.Parse("2006-01", to)
	end := month.AddDate(0, 1, 0).Add(-time.Nanosecond)
	filter := validFilter()
	filter.From, filter.To = &start, &end
	return filter
}

func near(value *float64, want float64) bool { return value != nil && math.Abs(*value-want) < 1e-9 }

// The stub reports season 115S2 imported, so sales are complete through 2026-02.

func TestSummaryComparesTwelveMonthsWithPreviousYear(t *testing.T) {
	stub := &summaryStub{windows: map[string]domain.MarketSummary{
		"2025-03..2026-02": {Count: 900, AvgPricePerPing: 660_000, MedianTotalPrice: 17_000_000},
		"2024-03..2025-02": {Count: 1000, AvgPricePerPing: 600_000, MedianTotalPrice: 17_000_000},
	}}
	summary, err := usecase.NewTransactionService(stub).Summary(context.Background(), window("2025-03", "2026-02"))
	if err != nil {
		t.Fatalf("Summary returned an error: %v", err)
	}
	if summary.Count != 900 || summary.YoY == nil || summary.YoY.Basis != "12m" || summary.YoY.Through != "2026-02" {
		t.Fatalf("unexpected summary %#v yoy %#v", summary, summary.YoY)
	}
	if !near(summary.YoY.Count, -0.1) || !near(summary.YoY.AvgPricePerPing, 0.1) || !near(summary.YoY.MedianTotalPrice, 0) {
		t.Fatalf("unexpected changes %#v", summary.YoY)
	}
	if len(stub.asked) != 2 {
		t.Fatalf("expected only the window and its previous year, asked %v", stub.asked)
	}
}

func TestSummaryStopsComparisonAtLastCompleteMonth(t *testing.T) {
	stub := &summaryStub{windows: map[string]domain.MarketSummary{
		"2025-10..2026-09": {Count: 500, AvgPricePerPing: 700_000},
		"2025-03..2026-02": {Count: 1100, AvgPricePerPing: 690_000},
		"2024-03..2025-02": {Count: 1000, AvgPricePerPing: 600_000},
	}}
	summary, err := usecase.NewTransactionService(stub).Summary(context.Background(), window("2025-10", "2026-09"))
	if err != nil {
		t.Fatalf("Summary returned an error: %v", err)
	}
	// The requested window's own figures are shown; the change uses complete months only.
	if summary.Count != 500 || summary.YoY.Through != "2026-02" || !near(summary.YoY.Count, 0.1) || !near(summary.YoY.AvgPricePerPing, 0.15) {
		t.Fatalf("unexpected summary %#v yoy %#v (asked %v)", summary, summary.YoY, stub.asked)
	}
}

func TestSummaryWidensSingleMonthToThreeMonths(t *testing.T) {
	stub := &summaryStub{windows: map[string]domain.MarketSummary{
		"2026-01..2026-01": {Count: 40, AvgPricePerPing: 700_000},
		"2025-11..2026-01": {Count: 120, AvgPricePerPing: 690_000},
		"2024-11..2025-01": {Count: 100, AvgPricePerPing: 600_000},
	}}
	summary, err := usecase.NewTransactionService(stub).Summary(context.Background(), window("2026-01", "2026-01"))
	if err != nil {
		t.Fatalf("Summary returned an error: %v", err)
	}
	if summary.Count != 40 || summary.YoY.Basis != "3m" || summary.YoY.Through != "2026-01" || !near(summary.YoY.AvgPricePerPing, 0.15) || !near(summary.YoY.Count, 0.2) {
		t.Fatalf("unexpected summary %#v yoy %#v (asked %v)", summary, summary.YoY, stub.asked)
	}
}

func TestSummaryOmitsChangesWithTooFewSales(t *testing.T) {
	stub := &summaryStub{windows: map[string]domain.MarketSummary{
		"2025-03..2026-02": {Count: 30, AvgPricePerPing: 700_000},
		"2024-03..2025-02": {Count: 4, AvgPricePerPing: 500_000},
	}}
	summary, err := usecase.NewTransactionService(stub).Summary(context.Background(), window("2025-03", "2026-02"))
	if err != nil {
		t.Fatalf("Summary returned an error: %v", err)
	}
	if summary.YoY == nil || summary.YoY.AvgPricePerPing != nil || summary.YoY.Count != nil {
		t.Fatalf("expected no changes with 4 previous sales, got %#v", summary.YoY)
	}
}

func TestTrendsMarkIncompleteMonthsProvisional(t *testing.T) {
	stub := &trendStub{points: []domain.TrendPoint{{Month: "2026-01"}, {Month: "2026-02"}, {Month: "2026-03"}}}
	trends, err := usecase.NewTransactionService(stub).Trends(context.Background(), validFilter())
	if err != nil {
		t.Fatalf("Trends returned an error: %v", err)
	}
	if trends[0].Provisional || trends[1].Provisional || !trends[2].Provisional {
		t.Fatalf("expected only 2026-03 to be provisional: %#v", trends)
	}
}

type trendStub struct {
	repositoryStub
	points []domain.TrendPoint
}

func (stub *trendStub) MonthlyTrends(context.Context, domain.TransactionFilter) ([]domain.TrendPoint, error) {
	return stub.points, nil
}
