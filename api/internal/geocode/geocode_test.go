package geocode_test

import (
	"testing"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/geocode"
)

func TestParseAddress(t *testing.T) {
	cases := []struct {
		district, address string
		want              geocode.Address
	}{
		{"中山區", "台北市中山區新生北路二段133巷12號六樓之11", geocode.Address{Road: "新生北路二段", Lane: "133巷", Number: 12}},
		{"松山區", "台北市松山區南京東路五段251巷50弄9之1號", geocode.Address{Road: "南京東路五段", Lane: "251巷", Alley: "50弄", Number: 9, SubNumber: 1}},
		{"北投區", "台北市北投區溫泉路銀光巷13號五樓", geocode.Address{Road: "溫泉路", Lane: "銀光巷", Number: 13}},
		{"中正區", "台北市中正區凱達格蘭大道1號", geocode.Address{Road: "凱達格蘭大道", Number: 1}},
		{"中山區", "台北市中山區中山里5鄰南京東路一段1號", geocode.Address{Road: "南京東路一段", Number: 1}},
		{"中山區", "台北市中山區新生北路二段68巷", geocode.Address{Road: "新生北路二段"}},
		{"三芝區", "新北市三芝區海尾13之67號十樓", geocode.Address{Road: "海尾", Number: 13, SubNumber: 67}},
		{"新店區", "新北市新店區中興路三段221-4號十樓", geocode.Address{Road: "中興路三段", Number: 221, SubNumber: 4}},
		{"中正區", "台北市中正區和平西路2段70巷8弄16號", geocode.Address{Road: "和平西路二段", Lane: "70巷", Alley: "8弄", Number: 16}},
		{"中山區", "台北市中山區中山北路12段1號", geocode.Address{Road: "中山北路十二段", Number: 1}},
	}
	for _, testCase := range cases {
		got, ok := geocode.ParseAddress(testCase.address, testCase.address[:len("台北市")], testCase.district)
		if !ok || got != testCase.want {
			t.Fatalf("ParseAddress(%q) = %#v, %v; want %#v", testCase.address, got, ok, testCase.want)
		}
	}
	if _, ok := geocode.ParseAddress("台北市松山區敦化段二小段490地號", "台北市", "松山區"); ok {
		t.Fatal("expected a land section address not to parse")
	}
}

func testMatcher() *geocode.Matcher {
	point := func(road, lane string, number, sub int, lat, lng float64) addresspoint.Point {
		return addresspoint.Point{DistrictCode: "63000120", Road: road, Lane: lane, Number: number, SubNumber: sub, Lat: lat, Lng: lng}
	}
	return geocode.NewMatcher("台北市", map[string]string{"北投區": "63000120", "大安區": "63000030"}, []addresspoint.Point{
		point("公舘路", "", 126, 0, 25.1, 121.5),
		point("公舘路", "", 130, 0, 25.2, 121.6),
		point("溫泉路", "銀光巷", 13, 0, 25.3, 121.7),
		point("溫泉路", "銀光巷", 13, 2, 25.4, 121.8),
		point("中和街", "", 100, 0, 25.0, 121.0),
	})
}

func TestMatcherLocate(t *testing.T) {
	matcher := testMatcher()
	cases := []struct {
		name, district, address string
		want                    geocode.Location
	}{
		{"exact", "北投區", "台北市北投區溫泉路銀光巷13之2號三樓", geocode.Location{Lat: 25.4, Lng: 121.8, Precision: geocode.PrecisionAddress}},
		{"private use character", "北投區", "台北市北投區公\ue16d路126號六樓", geocode.Location{Lat: 25.1, Lng: 121.5, Precision: geocode.PrecisionAddress}},
		{"variant character", "北投區", "台北市北投區公館路130號", geocode.Location{Lat: 25.2, Lng: 121.6, Precision: geocode.PrecisionAddress}},
		{"missing sub number", "北投區", "台北市北投區溫泉路銀光巷13之9號", geocode.Location{Lat: 25.3, Lng: 121.7, Precision: geocode.PrecisionNearby}},
		{"nearby number", "北投區", "台北市北投區公舘路128號", geocode.Location{Lat: 25.1, Lng: 121.5, Precision: geocode.PrecisionNearby}},
		{"road centroid", "北投區", "台北市北投區公舘路500號", geocode.Location{Lat: 25.15, Lng: 121.55, Precision: geocode.PrecisionRoad}},
		{"road only", "北投區", "台北市北投區公舘路", geocode.Location{Lat: 25.15, Lng: 121.55, Precision: geocode.PrecisionRoad}},
		{"unknown road", "北投區", "台北市北投區不存在路1號", geocode.Location{Lat: 25.2, Lng: 121.52, Precision: geocode.PrecisionDistrict}},
		{"unparsed", "北投區", "台北市北投區桃源段一小段1地號", geocode.Location{Lat: 25.2, Lng: 121.52, Precision: geocode.PrecisionDistrict}},
	}
	for _, testCase := range cases {
		got, ok := matcher.Locate(testCase.district, testCase.address)
		if !ok || got.Precision != testCase.want.Precision || !near(got.Lat, testCase.want.Lat) || !near(got.Lng, testCase.want.Lng) {
			t.Fatalf("%s: Locate(%q) = %#v, %v; want %#v", testCase.name, testCase.address, got, ok, testCase.want)
		}
	}
}

func TestMatcherRejectsDistrictsWithoutPoints(t *testing.T) {
	matcher := testMatcher()
	for _, district := range []string{"大安區", "板橋區"} {
		if location, ok := matcher.Locate(district, "台北市"+district+"中和街100號"); ok {
			t.Fatalf("%s: expected no location, got %#v", district, location)
		}
	}
}

func TestMatcherRequiresUniqueWildcardRoad(t *testing.T) {
	matcher := geocode.NewMatcher("台北市", map[string]string{"北投區": "63000120"}, []addresspoint.Point{
		{DistrictCode: "63000120", Road: "公舘路", Number: 1, Lat: 25.1, Lng: 121.5},
		{DistrictCode: "63000120", Road: "公園路", Number: 1, Lat: 25.2, Lng: 121.6},
	})
	if location, _ := matcher.Locate("北投區", "台北市北投區公\ue16d路1號"); location.Precision != geocode.PrecisionDistrict {
		t.Fatalf("expected an ambiguous road to fall back to the district, got %#v", location)
	}
}

func near(left, right float64) bool { return left-right < 1e-9 && right-left < 1e-9 }

func TestMatcherInfersMissingSectionFromLane(t *testing.T) {
	matcher := geocode.NewMatcher("台北市", map[string]string{"大安區": "63000030"}, []addresspoint.Point{
		{DistrictCode: "63000030", Road: "安和路一段", Lane: "27巷", Number: 1, Lat: 25.1, Lng: 121.5},
		{DistrictCode: "63000030", Road: "安和路二段", Lane: "117巷", Number: 8, Lat: 25.2, Lng: 121.6},
		{DistrictCode: "63000030", Road: "安和街", Number: 1, Lat: 25.3, Lng: 121.7},
	})
	location, _ := matcher.Locate("大安區", "台北市大安區安和路117巷8號五樓")
	if location.Precision != geocode.PrecisionAddress || location.Lat != 25.2 {
		t.Fatalf("expected 安和路二段117巷8號, got %#v", location)
	}
	// Without a lane the section is ambiguous.
	if location, _ = matcher.Locate("大安區", "台北市大安區安和路8號"); location.Precision != geocode.PrecisionDistrict {
		t.Fatalf("expected a district fallback for an ambiguous section, got %#v", location)
	}
}
