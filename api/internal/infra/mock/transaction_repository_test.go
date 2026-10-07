package mock

import (
	"context"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

func TestAnalysisGroupsAccountForEverySale(t *testing.T) {
	now := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	repository := NewTransactionRepository(120, now)
	from, to := now.AddDate(-2, 0, 0), now
	analysis, err := repository.Analysis(context.Background(), domain.TransactionFilter{
		MinPrice: 0, MaxPrice: 100_000_000, MinArea: 0, MaxArea: 200, From: &from, To: &to,
	})
	if err != nil {
		t.Fatal(err)
	}
	count := func(groups []domain.BreakdownAnalysis) int {
		var total int
		for _, group := range groups {
			total += group.Count
		}
		return total
	}
	if count(analysis.AgeGroups) != 120 || count(analysis.AreaGroups) != 120 || len(analysis.Monthly) == 0 {
		t.Fatalf("incomplete analysis groups: %#v", analysis)
	}
}

func TestDistrictComparisonGroupsMatchSaleCount(t *testing.T) {
	now := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	repository := NewTransactionRepository(120, now)
	from, to := now.AddDate(-2, 0, 0), now
	query := domain.DistrictComparisonQuery{CityCode: "a", Districts: []string{"大安區"}, Filter: domain.TransactionFilter{
		MinPrice: 0, MaxPrice: 100_000_000, MinArea: 0, MaxArea: 200, From: &from, To: &to,
	}}
	ranking, err := repository.DistrictRankings(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, row := range ranking {
		count += row.Count
	}
	trends, err := repository.DistrictTrends(context.Background(), query)
	if err != nil || count != 120 || len(trends) == 0 {
		t.Fatalf("unexpected comparison: count=%d trends=%d err=%v", count, len(trends), err)
	}
}
