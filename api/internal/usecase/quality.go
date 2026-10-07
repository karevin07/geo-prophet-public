package usecase

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

const (
	minimumQualitySample = 100
	minimumLocatedRate   = 0.90
	minimumPreciseRate   = 0.70
	minimumVolumeRatio   = 0.65
	staleReleaseAge      = 21 * 24 * time.Hour
)

// archiveDatasets are the import_runs datasets read from 實價登錄 archives.
var archiveDatasets = []string{"sales", "sales_release", "rentals", "rentals_release", "presales", "presales_release"}

// AssessQuality evaluates successful sales and lease imports and the latest attempts.
// Release volume is compared only with the same city's previous release, never with a
// season.
func AssessQuality(snapshot domain.DataStatusSnapshot, now time.Time) domain.QualityStatus {
	quality := domain.QualityStatus{Metrics: []domain.QualityMetric{}, Alerts: []domain.QualityAlert{}}
	groups := make(map[string][]domain.QualityRun)
	for _, run := range snapshot.QualityRuns {
		key := run.Dataset + "|" + run.CityCode
		groups[key] = append(groups[key], run)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		runs := groups[key]
		sort.Slice(runs, func(i, j int) bool {
			if runs[i].Season != runs[j].Season {
				return runs[i].Season > runs[j].Season
			}
			return runs[i].FinishedAt.After(runs[j].FinishedAt)
		})
		run := runs[0]
		metric := domain.QualityMetric{
			CityCode: run.CityCode, Dataset: run.Dataset, Season: run.Season,
			RowsRead: run.RowsRead, RowsSkipped: run.RowsSkipped, RowsUpserted: run.RowsUpserted,
			RowsGeocodeFailed: run.RowsGeocodeFailed, FinishedAt: run.FinishedAt,
		}
		metric.PreciseCount = run.Detail["geocode_address"] + run.Detail["geocode_nearby"]
		metric.LocatedCount = metric.PreciseCount + run.Detail["geocode_road"] + run.Detail["geocode_district"]
		// Only report a rate when the precision counters account for every
		// written sale. Older runs may lack detail and must not look like 0%.
		if run.RowsUpserted > 0 && metric.LocatedCount+run.RowsGeocodeFailed == run.RowsUpserted {
			precise, located := float64(metric.PreciseCount)/float64(run.RowsUpserted), float64(metric.LocatedCount)/float64(run.RowsUpserted)
			metric.PreciseRate, metric.LocatedRate = &precise, &located
		}
		if len(runs) > 1 && runs[1].RowsRead > 0 {
			previous := runs[1].RowsRead
			change := float64(run.RowsRead-previous) / float64(previous)
			metric.PreviousRowsRead, metric.VolumeChange = &previous, &change
		}
		quality.Metrics = append(quality.Metrics, metric)
		add := func(code, severity string) {
			quality.Alerts = append(quality.Alerts, domain.QualityAlert{Code: code, Severity: severity, CityCode: run.CityCode, Dataset: run.Dataset, Season: run.Season, RunID: run.ID})
		}
		if run.RowsUpserted >= minimumQualitySample {
			if metric.LocatedRate != nil && *metric.LocatedRate < minimumLocatedRate {
				add("low_location_rate", "warning")
			}
			if metric.PreciseRate != nil && *metric.PreciseRate < minimumPreciseRate {
				add("low_precise_rate", "warning")
			}
		}
		if strings.HasSuffix(run.Dataset, "_release") {
			if metric.PreviousRowsRead != nil && *metric.PreviousRowsRead >= minimumQualitySample && run.RowsRead >= minimumQualitySample &&
				float64(run.RowsRead)/float64(*metric.PreviousRowsRead) < minimumVolumeRatio {
				add("volume_drop", "warning")
			}
			// Re-importing an old release must not reset freshness. Compare with
			// its registration window, not the date this importer ran.
			if end, ok := releasePeriodEnd(run.Season); ok && now.Sub(end) > staleReleaseAge {
				add("stale_release", "warning")
			}
		}
	}
	for _, run := range snapshot.LatestRuns {
		if run.Source != "lvr" || !slices.Contains(archiveDatasets, run.Dataset) || run.Status != "failed" {
			continue
		}
		// Do not alarm on a future season the ministry has not published yet.
		if ErrorKind(run.Error) == "unpublished" {
			continue
		}
		quality.Alerts = append(quality.Alerts, domain.QualityAlert{
			Code: "import_failed", Severity: "error", CityCode: run.CityCode,
			Dataset: run.Dataset, Season: run.Season, RunID: run.ID,
		})
	}
	return quality
}

func releasePeriodEnd(label string) (time.Time, bool) {
	parts := strings.Split(label, "-")
	if len(parts) != 2 || len(parts[1]) != 7 {
		return time.Time{}, false
	}
	year, yearErr := strconv.Atoi(parts[1][:3])
	month, monthErr := strconv.Atoi(parts[1][3:5])
	day, dayErr := strconv.Atoi(parts[1][5:])
	if yearErr != nil || monthErr != nil || dayErr != nil {
		return time.Time{}, false
	}
	date := time.Date(year+1911, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return date, date.Year() == year+1911 && int(date.Month()) == month && date.Day() == day
}
