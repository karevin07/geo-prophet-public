package facility

import (
	"math"
	"strings"
	"testing"
)

func TestReadSchoolsKeepsTheLatestYearInCoveredCities(t *testing.T) {
	csv := "\ufeff學年度,代碼,學校名稱,公/私立,縣市名稱,地址,電話,網址\n" +
		"114,014501,市立板橋國中,公立,[01]新北市,[220]新北市板橋區中正路437號,(02)1,x\n" +
		"115,014501,市立板橋國中,公立,[01]新北市,[220]新北市板橋區中正路４３７號,(02)1,x\n" +
		"115,034501,縣立龜山國中,公立,[03]桃園縣,[333]桃園縣龜山區自強南路1號,(03)1,x\n" +
		"115,174501,市立鼓山國中,公立,[17]高雄市,[804]高雄市鼓山區1號,(07)1,x\n"
	schools, err := ReadSchools(strings.NewReader(csv), "junior_high")
	if err != nil {
		t.Fatal(err)
	}
	if len(schools) != 2 {
		t.Fatalf("expected the two 115 schools in covered cities, got %+v", schools)
	}
	first := schools[0]
	if first.ID != "moe:junior_high:014501" || first.CityCode != "f" || first.District != "板橋區" || first.Address != "新北市板橋區中正路437號" {
		t.Fatalf("unexpected school: %+v", first)
	}
	if second := schools[1]; second.CityCode != "h" || second.Address != "桃園市龜山區自強南路1號" {
		t.Fatalf("old county names should become 桃園市: %+v", second)
	}
}

func TestReadNHIRequiresItsColumns(t *testing.T) {
	if _, err := ReadNHI(strings.NewReader("代碼,名稱\n1,x\n"), "clinic"); err == nil {
		t.Fatal("a file without 醫事機構代碼 should fail")
	}
	clinics, err := ReadNHI(strings.NewReader("醫事機構代碼,醫事機構名稱,地址\n0101,臺大醫院,臺北市中正區中山南路７號\n"), "hospital")
	if err != nil || len(clinics) != 1 || clinics[0].Name != "台大醫院" || clinics[0].Address != "台北市中正區中山南路7號" {
		t.Fatalf("unexpected hospitals: %+v, %v", clinics, err)
	}
}

func TestRingAreaAndCentre(t *testing.T) {
	// A square of 0.001° near 25°N: about 111 m × 101 m.
	ring := [][2]float64{{121.5, 25.0}, {121.501, 25.0}, {121.501, 25.001}, {121.5, 25.001}, {121.5, 25.0}}
	if area := ringArea(ring); math.Abs(area-11_232) > 50 {
		t.Fatalf("unexpected area %v", area)
	}
	if lat, lng := centre(ring); math.Abs(lat-25.0005) > 1e-9 || math.Abs(lng-121.5005) > 1e-9 {
		t.Fatalf("unexpected centre %v %v", lat, lng)
	}
}
