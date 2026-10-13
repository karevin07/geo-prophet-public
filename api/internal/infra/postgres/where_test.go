package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

func TestWhereClauseExcludesOnlyKnownNewBuildings(t *testing.T) {
	where, args := whereClause(domain.TransactionFilter{MaxPrice: 1, MaxArea: 1, ExcludeNewBuildings: true})
	if !strings.Contains(where, "(building_age IS NULL OR building_age >= $5)") || len(args) != 6 || args[4] != domain.NewBuildingYears {
		t.Fatalf("where = %s, args = %v", where, args)
	}
	if where, _ = whereClause(domain.TransactionFilter{MaxPrice: 1, MaxArea: 1}); strings.Contains(where, "building_age") {
		t.Fatalf("building age filtered without the option: %s", where)
	}
}

func TestWhereClauseTrimsBoundsToTheBox(t *testing.T) {
	bounds := domain.Bounds{West: 121.5, South: 25.02, East: 121.56, North: 25.06}
	where, args := whereClause(domain.TransactionFilter{MaxPrice: 1, MaxArea: 1, Bounds: &bounds})
	envelope := "ST_MakeEnvelope($5, $6, $7, $8, 4326)"
	if !strings.Contains(where, "location && "+envelope+"::GEOGRAPHY AND location::GEOMETRY && "+envelope) || len(args) != 9 {
		t.Fatalf("where = %s, args = %v", where, args)
	}
}

func TestWhereClauseEndsTodayAtTheLatest(t *testing.T) {
	where, args := whereClause(domain.TransactionFilter{MaxPrice: 1, MaxArea: 1, IncludeSpecial: true})
	if !strings.HasSuffix(where, "transacted_on <= $5") || args[4] != today() {
		t.Fatalf("where = %s, args = %v", where, args)
	}
	earlier := time.Date(2020, 1, 31, 0, 0, 0, 0, time.UTC)
	later := today().AddDate(0, 1, 0)
	for to, want := range map[time.Time]time.Time{earlier: earlier, later: today()} {
		if _, args = whereClause(domain.TransactionFilter{MaxPrice: 1, MaxArea: 1, To: &to}); args[len(args)-1] != want {
			t.Fatalf("to %v: bound %v, want %v", to, args[len(args)-1], want)
		}
	}
}
