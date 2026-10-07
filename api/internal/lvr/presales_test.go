package lvr_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

// testdata/a_lvr_land_b_sample.csv contains authored synthetic parser scenarios, not downloaded transaction records.
func parsePresaleSample(t *testing.T) lvr.PresalesResult {
	t.Helper()
	file, err := os.Open("testdata/a_lvr_land_b_sample.csv")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()
	result, err := lvr.ParsePresales(file, "a", "113S3")
	if err != nil {
		t.Fatalf("ParsePresales returned an error: %v", err)
	}
	return result
}

func findPresale(t *testing.T, result lvr.PresalesResult, id string) domain.Presale {
	t.Helper()
	for _, presale := range result.Presales {
		if presale.ID == id {
			return presale
		}
	}
	t.Fatalf("presale %s not found", id)
	return domain.Presale{}
}

func TestParsePresalesReadsProjects(t *testing.T) {
	result := parsePresaleSample(t)
	if result.Rows != 5 || len(result.Presales) != 4 || result.Skipped[lvr.SkipParkingOnly] != 1 {
		t.Fatalf("got %d rows, %d presales, skips %v", result.Rows, len(result.Presales), result.Skipped)
	}
	presale := findPresale(t, result, "SYN-P-001")
	if presale.ProjectName != "示範建案甲" || presale.ProjectKey != "a|大同區|示範建案甲" || presale.UnitLabel != "B號" {
		t.Fatalf("unexpected project: %+v", presale)
	}
	// (23,000,000 − 1,900,000) / ((104.65 − 24.46) / 3.30579) ping.
	if presale.Address != "台北市大同區示範七路17號" || presale.ParkingPrice != 1_900_000 || presale.PricePerPing != 869836 {
		t.Fatalf("unexpected sale: %q %d %v", presale.Address, presale.ParkingPrice, presale.PricePerPing)
	}
	if presale.CancelledOn != nil || presale.CompletedAt != nil || !presale.TransactedAt.Equal(time.Date(2024, 5, 13, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected dates: %+v", presale)
	}
}

func TestParsePresalesDropsLotCountsAndReadsCancellations(t *testing.T) {
	result := parsePresaleSample(t)
	if address := findPresale(t, result, "SYN-P-002").Address; address != "台北市中正區示範五路161-1號" {
		t.Fatalf("共13筆 should be dropped: %q", address)
	}
	cancelled := findPresale(t, result, "SYN-P-003")
	if cancelled.Cancellation != "1140409全部解約" || cancelled.CancelledOn == nil || !cancelled.CancelledOn.Equal(time.Date(2025, 4, 9, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected cancellation: %q %v", cancelled.Cancellation, cancelled.CancelledOn)
	}
}

// Before about 108S1 the file has the 28 sales columns and no project.
func TestParsePresalesAcceptsOlderSeasons(t *testing.T) {
	header := "鄉鎮市區,交易標的,土地位置建物門牌,土地移轉總面積平方公尺,都市土地使用分區,非都市土地使用分區,非都市土地使用編定,交易年月日,交易筆棟數,移轉層次,總樓層數,建物型態,主要用途,主要建材,建築完成年月,建物移轉總面積平方公尺,建物現況格局-房,建物現況格局-廳,建物現況格局-衛,建物現況格局-隔間,有無管理組織,總價元,單價元平方公尺,車位類別,車位移轉總面積平方公尺,車位總價元,備註,編號\n"
	row := "板橋區,房地(土地+建物),新北市板橋區文化路一段,10.0,住,,,1031105,土地1建物1車位0,五層,15,住宅大樓(11層含以上有電梯),住家用,鋼筋混凝土造,,99.17,3,2,2,有,有,15000000,151255,,0.0,0,,OLD1"
	result, err := lvr.ParsePresales(strings.NewReader(header+row), "f", "104S1")
	if err != nil || len(result.Presales) != 1 {
		t.Fatalf("got %v, %v", result, err)
	}
	if presale := result.Presales[0]; presale.ProjectName != "" || presale.ProjectKey != "" || presale.Cancellation != "" {
		t.Fatalf("older seasons have no project: %+v", presale)
	}
}
