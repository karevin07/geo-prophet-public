package lvr

import (
	"slices"
	"testing"
	"time"
)

func TestParseROCDate(t *testing.T) {
	cases := map[string]time.Time{
		"1150601": time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		"0690110": time.Date(1980, 1, 10, 0, 0, 0, 0, time.UTC),
		"11412":   time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
	}
	for input, want := range cases {
		got, err := parseROCDate(input)
		if err != nil || !got.Equal(want) {
			t.Fatalf("parseROCDate(%q) = %v, %v; want %v", input, got, err, want)
		}
	}
	for _, input := range []string{"", "010", "1150230", "1151301", "0000101", "11a0101"} {
		if _, err := parseROCDate(input); err == nil {
			t.Fatalf("parseROCDate(%q) expected an error", input)
		}
	}
}

func TestParseFloorCount(t *testing.T) {
	cases := map[string]int{"23": 23, "五層": 5, "十層": 10, "十二層": 12, "二十三層": 23, "一百零一層": 101, "４２層": 42}
	for input, want := range cases {
		if got, ok := parseFloorCount(input); !ok || got != want {
			t.Fatalf("parseFloorCount(%q) = %d, %v; want %d", input, got, ok, want)
		}
	}
	for _, input := range []string{"", "全", "層", "地下一層"} {
		if _, ok := parseFloorCount(input); ok {
			t.Fatalf("parseFloorCount(%q) expected failure", input)
		}
	}
}

func TestSpecialReasons(t *testing.T) {
	cases := map[string][]string{
		"其他增建;陽台外推;":   nil,
		"僅建物交易,無土地持分;": {SpecialBuildingOnly},
		"二親等間買賣親友、員工、共有人或其他特殊關係間之交易": {SpecialRelatedParty},
		"協議價購;包含公共設施保留地用地;":          {SpecialGovernmentSale, SpecialPublicFacilityLand},
		"區段徵收專案配售。土地及建物分次登記案件。":      {SpecialGovernmentSale},
		"土地及建物分次登記案件。*向政府機構申購之案件":    {SpecialGovernmentSale},
		"新北市政府安置住宅移轉":                {SpecialGovernmentSale},
		"本案屬於國防部眷村改建配售":              {SpecialGovernmentSale},
		"土地及建物分次登記案件。前案土地登記收件":       nil,
		"陽台外推國民住宅":                   nil,
	}
	for remarks, want := range cases {
		if got := specialReasons(remarks); !slices.Equal(got, want) {
			t.Fatalf("specialReasons(%q) = %v; want %v", remarks, got, want)
		}
	}
}

func TestFloorNumber(t *testing.T) {
	for label, want := range map[string]int{"二層,陽台": 2, "十二層": 12, "地上三層": 3, "4層": 4, "全": 0, "地下一層": 0, "": 0} {
		got, ok := FloorNumber(label)
		if got != want || ok != (want > 0) {
			t.Errorf("%q: got %d %v, want %d", label, got, ok, want)
		}
	}
}
