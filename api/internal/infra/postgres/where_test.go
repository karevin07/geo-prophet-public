package postgres

import (
	"strings"
	"testing"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

func TestWhereClauseExcludesOnlyKnownNewBuildings(t *testing.T) {
	where, args := whereClause(domain.TransactionFilter{MaxPrice: 1, MaxArea: 1, ExcludeNewBuildings: true})
	if !strings.Contains(where, "(building_age IS NULL OR building_age >= $5)") || len(args) != 5 || args[4] != domain.NewBuildingYears {
		t.Fatalf("where = %s, args = %v", where, args)
	}
	if where, _ = whereClause(domain.TransactionFilter{MaxPrice: 1, MaxArea: 1}); strings.Contains(where, "building_age") {
		t.Fatalf("building age filtered without the option: %s", where)
	}
}
