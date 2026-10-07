package lvr_test

import (
	"slices"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

func TestParseSeasons(t *testing.T) {
	cases := map[string][]string{
		"115S2":              {"115S2"},
		"114S3-115S1":        {"114S3", "114S4", "115S1"},
		"105S1, 110S4-111S1": {"105S1", "110S4", "111S1"},
	}
	for input, want := range cases {
		seasons, err := lvr.ParseSeasons(input)
		if err != nil {
			t.Fatalf("ParseSeasons(%q) returned an error: %v", input, err)
		}
		got := make([]string, len(seasons))
		for index, season := range seasons {
			got[index] = season.String()
		}
		if !slices.Equal(got, want) {
			t.Fatalf("ParseSeasons(%q) = %v; want %v", input, got, want)
		}
	}
	for _, input := range []string{"", "115Q2", "115S5", "115S2-114S1"} {
		if _, err := lvr.ParseSeasons(input); err == nil {
			t.Fatalf("ParseSeasons(%q) expected an error", input)
		}
	}
}

func TestSeasonOfRegistration(t *testing.T) {
	// 115S2 is documented as registrations from 115/3/11 to 115/6/10.
	cases := map[string]string{
		"2026-03-10": "115S1", "2026-03-11": "115S2", "2026-06-10": "115S2", "2026-06-11": "115S3",
		"2026-09-10": "115S3", "2026-09-11": "115S4", "2026-12-10": "115S4", "2026-12-11": "116S1",
		"2026-01-05": "115S1", "2026-02-28": "115S1",
	}
	for input, want := range cases {
		date, _ := time.Parse("2006-01-02", input)
		if got := lvr.SeasonOfRegistration(date).String(); got != want {
			t.Fatalf("SeasonOfRegistration(%s) = %s; want %s", input, got, want)
		}
	}
}

func TestParseRelease(t *testing.T) {
	buildTime := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<lvr_land>
  <lvr_time>資料內容：登記日期 115年9月1日至 115年9月10日之買賣案件，及訂約日期 115年8月1日至 115年8月10日之租賃案件</lvr_time>
</lvr_land>`)
	release, err := lvr.ParseRelease(buildTime)
	if err != nil {
		t.Fatalf("ParseRelease returned an error: %v", err)
	}
	if release.Label() != "1150901-1150910" || release.Season().String() != "115S3" {
		t.Fatalf("unexpected release %s in %s", release.Label(), release.Season())
	}
	if _, err = lvr.ParseRelease([]byte("<lvr_land></lvr_land>")); err == nil {
		t.Fatal("expected an error without a registration window")
	}
}

func TestRegistrationWindow(t *testing.T) {
	cases := map[string][2]string{
		"115S1": {"2025-12-11", "2026-03-10"},
		"115S2": {"2026-03-11", "2026-06-10"},
		"115S4": {"2026-09-11", "2026-12-10"},
	}
	for input, want := range cases {
		season, _ := lvr.ParseSeason(input)
		start, end := season.RegistrationWindow()
		if start.Format("2006-01-02") != want[0] || end.Format("2006-01-02") != want[1] {
			t.Fatalf("%s: got %s..%s, want %v", input, start.Format("2006-01-02"), end.Format("2006-01-02"), want)
		}
		// Every registration date in the window maps back to the season.
		if lvr.SeasonOfRegistration(start) != season || lvr.SeasonOfRegistration(end) != season {
			t.Fatalf("%s: window endpoints map to another season", input)
		}
	}
}

func TestParseReleaseLabel(t *testing.T) {
	release, err := lvr.ParseReleaseLabel("1150901-1150910")
	if err != nil || release.Label() != "1150901-1150910" {
		t.Fatalf("unexpected release %#v, %v", release, err)
	}
	if _, err = lvr.ParseReleaseLabel("115S2"); err == nil {
		t.Fatal("expected an error for a season label")
	}
}
