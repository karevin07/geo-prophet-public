package usecase_test

import (
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
)

func TestAssessQualityComparesDistinctReleasePeriodsAndRates(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	snapshot := domain.DataStatusSnapshot{QualityRuns: []domain.QualityRun{
		{ID: 3, Dataset: "sales_release", CityCode: "a", Season: "115-09-21", RowsRead: 600,
			RowsUpserted: 500, RowsGeocodeFailed: 50, Detail: map[string]int{"geocode_address": 300, "geocode_nearby": 50, "geocode_road": 80, "geocode_district": 20}, FinishedAt: now.Add(-24 * time.Hour)},
		{ID: 2, Dataset: "sales_release", CityCode: "a", Season: "115-09-11", RowsRead: 1000,
			RowsUpserted: 900, FinishedAt: now.Add(-10 * 24 * time.Hour)},
	}}
	result := usecase.AssessQuality(snapshot, now)
	if len(result.Metrics) != 1 || result.Metrics[0].PreciseRate == nil || *result.Metrics[0].PreciseRate != 0.7 ||
		result.Metrics[0].LocatedRate == nil || *result.Metrics[0].LocatedRate != 0.9 || result.Metrics[0].PreviousRowsRead == nil || *result.Metrics[0].PreviousRowsRead != 1000 {
		t.Fatalf("incorrect quality metrics: %#v", result.Metrics)
	}
	if len(result.Alerts) != 1 || result.Alerts[0].Code != "volume_drop" || result.Alerts[0].RunID != 3 {
		t.Fatalf("expected only volume alert: %#v", result.Alerts)
	}
}

func TestAssessQualityIgnoresUnpublishedSeasonAndMissingBaseline(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	snapshot := domain.DataStatusSnapshot{
		QualityRuns: []domain.QualityRun{{ID: 1, Dataset: "sales", CityCode: "h", Season: "115S3", RowsRead: 300,
			RowsUpserted: 200, FinishedAt: now}},
		LatestRuns: []domain.ImportRunStatus{{ID: 2, Source: "lvr", Dataset: "sales", CityCode: "h", Season: "115S4",
			Status: "failed", Error: "season archive is not published"}},
	}
	result := usecase.AssessQuality(snapshot, now)
	if len(result.Metrics) != 1 || result.Metrics[0].PreciseRate != nil || len(result.Alerts) != 0 {
		t.Fatalf("missing geocode counters or unpublished season should not alarm: %#v", result)
	}
}

func TestAssessQualityWarnsOnLowGeocodingAndStaleRelease(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	snapshot := domain.DataStatusSnapshot{QualityRuns: []domain.QualityRun{{
		ID: 9, Dataset: "sales_release", CityCode: "f", Season: "1150901-1150910",
		RowsRead: 250, RowsUpserted: 200, RowsGeocodeFailed: 60,
		Detail:     map[string]int{"geocode_address": 90, "geocode_nearby": 10, "geocode_road": 25, "geocode_district": 15},
		FinishedAt: now.Add(-22 * 24 * time.Hour),
	}}}
	result := usecase.AssessQuality(snapshot, now)
	codes := map[string]bool{}
	for _, alert := range result.Alerts {
		codes[alert.Code] = true
	}
	for _, code := range []string{"low_location_rate", "low_precise_rate", "stale_release"} {
		if !codes[code] {
			t.Fatalf("missing %s alert: %#v", code, result.Alerts)
		}
	}
}

func TestAssessQualityReimportDoesNotResetReleaseFreshness(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	snapshot := domain.DataStatusSnapshot{QualityRuns: []domain.QualityRun{{
		ID: 10, Dataset: "sales_release", CityCode: "a", Season: "1150901-1150910", FinishedAt: now,
	}}}
	result := usecase.AssessQuality(snapshot, now)
	if len(result.Alerts) != 1 || result.Alerts[0].Code != "stale_release" {
		t.Fatalf("reimport must not hide stale source: %#v", result.Alerts)
	}
}

func TestAssessQualityCoversLeases(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	snapshot := domain.DataStatusSnapshot{
		QualityRuns: []domain.QualityRun{{ID: 11, Dataset: "rentals_release", CityCode: "a", Season: "1150901-1150910", FinishedAt: now}},
		LatestRuns: []domain.ImportRunStatus{{ID: 12, Source: "lvr", Dataset: "rentals", CityCode: "f", Season: "115S2",
			Status: "failed", Error: "rental file is missing columns: 總額元"}},
	}
	result := usecase.AssessQuality(snapshot, now)
	codes := map[string]string{}
	for _, alert := range result.Alerts {
		codes[alert.Code] = alert.Dataset
	}
	if codes["stale_release"] != "rentals_release" || codes["import_failed"] != "rentals" {
		t.Fatalf("lease imports should be assessed like sales: %#v", result.Alerts)
	}
}
