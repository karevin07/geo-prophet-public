package lvr_test

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

// testdata/a_lvr_land_c_sample.csv contains authored synthetic parser scenarios, not downloaded transaction records.
func parseRentalSample(t *testing.T) lvr.RentalsResult {
	t.Helper()
	file, err := os.Open("testdata/a_lvr_land_c_sample.csv")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()
	result, err := lvr.ParseRentals(file, "a", "115S2")
	if err != nil {
		t.Fatalf("ParseRentals returned an error: %v", err)
	}
	return result
}

func findRental(t *testing.T, result lvr.RentalsResult, id string) domain.Rental {
	t.Helper()
	for _, rental := range result.Rentals {
		if rental.ID == id {
			return rental
		}
	}
	t.Fatalf("rental %s not found", id)
	return domain.Rental{}
}

func TestParseRentalsSkipsParkingAndLand(t *testing.T) {
	result := parseRentalSample(t)
	if result.Rows != 6 || len(result.Rentals) != 4 {
		t.Fatalf("got %d rows and %d rentals, want 6 and 4", result.Rows, len(result.Rentals))
	}
	if result.Skipped[lvr.SkipParkingOnly] != 1 || result.Skipped[lvr.SkipNoBuilding] != 1 {
		t.Fatalf("unexpected skips: %v", result.Skipped)
	}
}

func TestParseRentalsNormalizesWholeHome(t *testing.T) {
	rental := findRental(t, parseRentalSample(t), "SYN-R-001")
	if rental.Address != "台北市萬華區示範六路66巷21弄23號5樓" || rental.District != "萬華區" {
		t.Fatalf("unexpected location: %q %q", rental.District, rental.Address)
	}
	if !rental.WholeUnit || rental.WholeUnitInferred || rental.SocialHousing || rental.Renewal || rental.IsSpecial() {
		t.Fatalf("unexpected flags: %+v", rental)
	}
	// 15,000 / (72.09 m² / 3.30579) = 687.8 per ping.
	if rental.MonthlyRent != 15000 || rental.BuildingRent != 15000 || rental.BuildingAreaPing != 21.81 || rental.RentPerPing != 687.8 {
		t.Fatalf("unexpected rent: %d %d %v %v", rental.MonthlyRent, rental.BuildingRent, rental.BuildingAreaPing, rental.RentPerPing)
	}
	wantStart, wantEnd := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, 3, 4, 0, 0, 0, 0, time.UTC)
	if !rental.LeasedAt.Equal(time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC)) || !rental.LeaseStart.Equal(wantStart) || !rental.LeaseEnd.Equal(wantEnd) {
		t.Fatalf("unexpected dates: %v %v %v", rental.LeasedAt, rental.LeaseStart, rental.LeaseEnd)
	}
	if !rental.Furnished || rental.HasManager == nil || *rental.HasManager || rental.HasElevator == nil || !*rental.HasElevator {
		t.Fatalf("unexpected amenities: %+v", rental)
	}
	if rental.FloorLabel != "五層" || rental.TotalFloors != 5 || rental.Rooms != 1 || rental.CompletedAt == nil {
		t.Fatalf("unexpected building: %+v", rental)
	}
}

func TestParseRentalsFlagsSocialHousingAndRenewals(t *testing.T) {
	rental := findRental(t, parseRentalSample(t), "SYN-R-002")
	if !rental.SocialHousing || rental.RentalService != "社會住宅代管" || !rental.Renewal || !rental.WholeUnit {
		t.Fatalf("unexpected flags: %+v", rental)
	}
}

func TestParseRentalsFlagsRoomsAsPartialArea(t *testing.T) {
	rental := findRental(t, parseRentalSample(t), "SYN-R-003")
	if rental.WholeUnit || rental.RentalType != "分租套房" || !slices.Contains(rental.SpecialReasons, lvr.SpecialPartialArea) {
		t.Fatalf("unexpected room lease: %+v", rental)
	}
}

func TestParseRentalsSeparatesParkingRent(t *testing.T) {
	rental := findRental(t, parseRentalSample(t), "SYN-R-004")
	// (138.35 − 36.5) m² = 30.81 ping; (62,878 − 4,500) / 30.81 = 1,894.8.
	if rental.ParkingRent != 4500 || rental.BuildingRent != 58378 || rental.BuildingAreaPing != 30.81 || rental.RentPerPing != 1894.8 {
		t.Fatalf("unexpected rent: %+v", rental)
	}
}

// Before 112S4 the file has 29 columns and no rental type, so whole homes are inferred.
func TestParseRentalsInfersWholeHomesInOlderSeasons(t *testing.T) {
	header := "鄉鎮市區,交易標的,土地位置建物門牌,土地面積平方公尺,都市土地使用分區,非都市土地使用分區,非都市土地使用編定,租賃年月日,租賃筆棟數,租賃層次,總樓層數,建物型態,主要用途,主要建材,建築完成年月,建物總面積平方公尺,建物現況格局-房,建物現況格局-廳,建物現況格局-衛,建物現況格局-隔間,有無管理組織,有無附傢俱,總額元,單價元平方公尺,車位類別,車位面積平方公尺,車位總額元,備註,編號\n"
	rows := []string{
		"內湖區,房地(土地+建物)+車位,台北市內湖區內湖路二段253巷58號8樓之3,4065.0,住,,,1010806,土地1建物1車位1,八層,11,住宅大樓(11層含以上有電梯),住家用,鋼筋混凝土造,0980923,88.7,3,2,2,有,有,有,36000,406,坡道平面,0.0,0,,OLD1",
		"大安區,建物,台北市大安區信義路三段1號3樓,0.0,,,,1020105,土地0建物1車位0,三層,5,公寓(5樓含以下無電梯),住家用,鋼筋混凝土造,0700101,30.0,1,0,1,有,無,有,8000,266,,0.0,0,,OLD2",
		"大安區,建物,台北市大安區信義路三段1號4樓,0.0,,,,1020105,土地0建物1車位0,四層,5,公寓(5樓含以下無電梯),住家用,鋼筋混凝土造,0700101,99.0,1,0,1,有,無,有,8000,80,,0.0,0,部分租賃,OLD3",
		"大安區,建物,台北市大安區信義路三段1號5樓,0.0,,,,1020105,土地0建物1車位0,五層,5,公寓(5樓含以下無電梯),住家用,鋼筋混凝土造,0700101,99.0,3,1,1,有,無,有,30000,303,,0.0,0,台北市社會住宅包租代管,OLD4",
	}
	result, err := lvr.ParseRentals(strings.NewReader(header+strings.Join(rows, "\n")), "a", "101S4")
	if err != nil {
		t.Fatalf("ParseRentals returned an error: %v", err)
	}
	want := map[string]bool{"OLD1": true, "OLD2": false, "OLD3": false, "OLD4": true}
	for id, whole := range want {
		rental := findRental(t, result, id)
		if rental.WholeUnit != whole || !rental.WholeUnitInferred || rental.RentalType != "" || rental.HasElevator != nil {
			t.Errorf("%s: whole %v inferred %v, want whole %v", id, rental.WholeUnit, rental.WholeUnitInferred, whole)
		}
	}
	if !findRental(t, result, "OLD4").SocialHousing {
		t.Error("OLD4 should be social housing from its remarks")
	}
}

func TestMarkRentOutliersChecksWholeHomesOnly(t *testing.T) {
	var rentals []domain.Rental
	for index := range 30 {
		rentals = append(rentals, domain.Rental{ID: string(rune('A' + index)), CityCode: "a", District: "大安區", RentPerPing: 1000 + float64(index), WholeUnit: true})
	}
	rentals = append(rentals,
		domain.Rental{ID: "high", CityCode: "a", District: "大安區", RentPerPing: 9000, WholeUnit: true},
		domain.Rental{ID: "room", CityCode: "a", District: "大安區", RentPerPing: 9000})
	lvr.MarkRentOutliers(rentals)
	for _, rental := range rentals {
		flagged := slices.Contains(rental.SpecialReasons, lvr.SpecialPriceOutlier)
		if flagged != (rental.ID == "high") {
			t.Errorf("%s flagged %v", rental.ID, flagged)
		}
	}
}
