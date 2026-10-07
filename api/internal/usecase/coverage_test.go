package usecase

import (
	"testing"
	"time"
)

func TestRegisteredThroughFollowsContiguousReleases(t *testing.T) {
	cases := []struct {
		name     string
		seasons  []string
		releases []string
		want     string
	}{
		{"season only", []string{"115S1", "115S2"}, nil, "2026-06-10"},
		{"gap after season", []string{"115S2"}, []string{"1150901-1150910"}, "2026-06-10"},
		{"contiguous releases", []string{"115S2"}, []string{"1150611-1150620", "1150621-1150630", "1150701-1150710"}, "2026-07-10"},
		{"later season closes the gap", []string{"115S2", "115S3"}, []string{"1150901-1150910"}, "2026-09-10"},
	}
	for _, testCase := range cases {
		got, ok := registeredThrough(testCase.seasons, testCase.releases)
		if !ok || got.Format("2006-01-02") != testCase.want {
			t.Fatalf("%s: got %s (%v), want %s", testCase.name, got.Format("2006-01-02"), ok, testCase.want)
		}
	}
	if _, ok := registeredThrough(nil, []string{"1150901-1150910"}); ok {
		t.Fatal("expected no coverage without a season archive")
	}
}

func TestCompleteThrough(t *testing.T) {
	cases := map[string]string{
		"2026-06-10": "2026-02", // cutoff 2026-03-12, March is not over
		"2026-07-01": "2026-03", // cutoff 2026-04-02
		"2026-06-29": "2026-03", // cutoff 2026-03-31, the last day of March
	}
	for input, want := range cases {
		date, _ := time.Parse("2006-01-02", input)
		if got := completeThrough(date).Format("2006-01"); got != want {
			t.Fatalf("completeThrough(%s) = %s; want %s", input, got, want)
		}
	}
}
