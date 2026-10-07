package lvr_test

import (
	"slices"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

func TestMarkPriceOutliers(t *testing.T) {
	transactions := make([]domain.Transaction, 0, 24)
	for index := range 20 {
		transactions = append(transactions, domain.Transaction{CityCode: "a", District: "大安區", PricePerPing: float64(900_000 + index*10_000)})
	}
	transactions = append(transactions,
		domain.Transaction{ID: "high", CityCode: "a", District: "大安區", PricePerPing: 5_000_000},
		domain.Transaction{ID: "low", CityCode: "a", District: "大安區", PricePerPing: 50_000},
		// A special transaction far from the market must not widen the fence.
		domain.Transaction{ID: "related", CityCode: "a", District: "大安區", PricePerPing: 90_000_000, SpecialReasons: []string{lvr.SpecialRelatedParty}},
		// Too few samples in 萬華區 to judge.
		domain.Transaction{ID: "sparse", CityCode: "a", District: "萬華區", PricePerPing: 9_000_000},
	)

	lvr.MarkPriceOutliers(transactions)
	lvr.MarkPriceOutliers(transactions) // idempotent

	for _, transaction := range transactions {
		want := []string(nil)
		switch transaction.ID {
		case "high", "low":
			want = []string{lvr.SpecialPriceOutlier}
		case "related":
			want = []string{lvr.SpecialRelatedParty, lvr.SpecialPriceOutlier}
		}
		if !slices.Equal(transaction.SpecialReasons, want) {
			t.Fatalf("%q: expected %v, got %v", transaction.ID, want, transaction.SpecialReasons)
		}
	}
}

func TestMarkInvalidDates(t *testing.T) {
	_, registeredTo := lvr.Season{Year: 110, Quarter: 2}.RegistrationWindow()
	transactions := []domain.Transaction{
		{ID: "typo", TransactedAt: time.Date(2101, 3, 3, 0, 0, 0, 0, time.UTC), SpecialReasons: []string{lvr.SpecialSplitRegistration}},
		{ID: "last-day", TransactedAt: registeredTo},
		{ID: "normal", TransactedAt: time.Date(2021, 3, 3, 0, 0, 0, 0, time.UTC)},
	}
	lvr.MarkInvalidDates(transactions, registeredTo)
	lvr.MarkInvalidDates(transactions, registeredTo) // idempotent
	if !slices.Equal(transactions[0].SpecialReasons, []string{lvr.SpecialSplitRegistration, lvr.SpecialInvalidDate}) {
		t.Fatalf("typo reasons = %v", transactions[0].SpecialReasons)
	}
	if transactions[1].IsSpecial() || transactions[2].IsSpecial() {
		t.Fatalf("valid dates were flagged: %+v", transactions[1:])
	}
}
