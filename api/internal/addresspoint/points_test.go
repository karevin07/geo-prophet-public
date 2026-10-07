package addresspoint_test

import (
	"math"
	"strings"
	"testing"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
)

func TestTWD97ToWGS84(t *testing.T) {
	// 市府路1號 (Taipei City Hall) in the Taipei door-number data.
	lat, lng := addresspoint.TWD97ToWGS84(306891.63, 2770052.15)
	if math.Abs(lat-25.0375) > 0.0005 || math.Abs(lng-121.5637) > 0.0005 {
		t.Fatalf("expected Taipei City Hall near (25.0375, 121.5637), got (%.5f, %.5f)", lat, lng)
	}
}

func TestParseMergesFloorsAndSkipsUnusableRows(t *testing.T) {
	input := "\ufeffcountycode,areacode,village,neighbor,street、road、section,area,lane,alley,number,x_3826,y_3826\n" +
		"65000,65000010,九如里,012,三民路二段,,正隆巷,４６弄,１號,298143.0434,2767433.7872\n" +
		"65000,65000010,九如里,012,三民路二段,,正隆巷,４６弄,１號二樓,298143.0434,2767433.7872\n" +
		"65000,65000010,九如里,012,三民路二段,,正隆巷,４６弄,１之２號地下室,298150.0,2767440.0\n" +
		"65000,65000010,九如里,012,三民路二段,,,,臨１號,298150.0,2767440.0\n" +
		"65000,65000010,九如里,012,三民路二段,,,,５號,,\n" +
		"65000,65000010,九如里,012,三民路二段,,,,無號,298150.0,2767440.0\n" +
		"65000,65000210,埔頭里,005,,海尾,,,１３之６７號十樓,294000.0,2790000.0\n" +
		"65000,65000010\n"
	result, err := addresspoint.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned an error: %v", err)
	}
	if result.Rows != 8 || len(result.Points) != 3 {
		t.Fatalf("expected 8 rows and 3 points, got %d rows and %#v", result.Rows, result.Points)
	}
	first := result.Points[0]
	if first.DistrictCode != "65000010" || first.Road != "三民路二段" || first.Lane != "正隆巷" || first.Alley != "46弄" ||
		first.Number != 1 || first.SubNumber != 0 {
		t.Fatalf("unexpected first point: %#v", first)
	}
	if second := result.Points[1]; second.Number != 1 || second.SubNumber != 2 {
		t.Fatalf("unexpected second point: %#v", second)
	}
	if rural := result.Points[2]; rural.Road != "海尾" || rural.Number != 13 || rural.SubNumber != 67 {
		t.Fatalf("expected the place name to stand in for the road, got %#v", rural)
	}
	want := map[addresspoint.SkipReason]int{
		addresspoint.SkipTemporaryNumber: 1, addresspoint.SkipInvalidCoordinate: 1,
		addresspoint.SkipInvalidNumber: 1, addresspoint.SkipMalformed: 1,
	}
	for reason, count := range want {
		if result.Skipped[reason] != count {
			t.Fatalf("expected %d %s, got %#v", count, reason, result.Skipped)
		}
	}
}

func TestParseRejectsUnexpectedHeader(t *testing.T) {
	if _, err := addresspoint.Parse(strings.NewReader("a,b,c\n")); err == nil {
		t.Fatal("expected an error for a file with too few columns")
	}
}

func TestSourcesCoverEveryDistrictCodeOnce(t *testing.T) {
	for cityCode, source := range addresspoint.Sources {
		if source.CityCode != cityCode || source.DatasetID == "" || len(source.Districts) == 0 {
			t.Fatalf("incomplete source %q: %#v", cityCode, source)
		}
		seen := map[string]string{}
		for district, code := range source.Districts {
			if previous, ok := seen[code]; ok {
				t.Fatalf("city %s: %s and %s share code %s", cityCode, previous, district, code)
			}
			seen[code] = district
		}
	}
}
