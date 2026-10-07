package usecase

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

// DataStatusRepository reads importer history without exposing the database to HTTP.
type DataStatusRepository interface {
	DataStatusSnapshot(ctx context.Context) (domain.DataStatusSnapshot, error)
}

type DataStatusService struct {
	repository DataStatusRepository
	now        func() time.Time
}

func NewDataStatusService(repository DataStatusRepository) *DataStatusService {
	return &DataStatusService{repository: repository, now: time.Now}
}

func (service *DataStatusService) Status(ctx context.Context) (domain.DataStatus, error) {
	snapshot, err := service.repository.DataStatusSnapshot(ctx)
	if err != nil {
		return domain.DataStatus{}, err
	}
	status := domain.DataStatus{
		GeneratedAt:        service.now().UTC(),
		RecentRuns:         snapshot.RecentRuns,
		LatestRuns:         snapshot.LatestRuns,
		LastSuccessfulRuns: snapshot.LastSuccessfulRuns,
		Cities:             snapshot.Cities,
		Coverage:           make([]domain.SeasonCoverage, 0, len(snapshot.ImportedSeasons)),
		Quality:            AssessQuality(snapshot, service.now()),
	}
	for _, runs := range [][]domain.ImportRunStatus{status.RecentRuns, status.LatestRuns, status.LastSuccessfulRuns} {
		for index := range runs {
			service.describe(&runs[index])
		}
	}
	if status.RecentRuns == nil {
		status.RecentRuns = []domain.ImportRunStatus{}
	}
	if status.LatestRuns == nil {
		status.LatestRuns = []domain.ImportRunStatus{}
	}
	if status.LastSuccessfulRuns == nil {
		status.LastSuccessfulRuns = []domain.ImportRunStatus{}
	}
	if status.Cities == nil {
		status.Cities = []domain.CityDataCount{}
	}
	for city, labels := range snapshot.ImportedSeasons {
		var seasons []lvr.Season
		for _, label := range labels {
			season, err := lvr.ParseSeason(label)
			if err == nil {
				seasons = append(seasons, season)
			}
		}
		if len(seasons) == 0 {
			continue
		}
		sort.Slice(seasons, func(i, j int) bool { return seasons[i].Before(seasons[j]) })
		known := make(map[string]bool, len(seasons))
		for _, season := range seasons {
			known[season.String()] = true
		}
		coverage := domain.SeasonCoverage{
			CityCode: city, FirstImported: seasons[0].String(),
			LatestImported: seasons[len(seasons)-1].String(), Missing: []string{},
		}
		for season := seasons[0]; season.Before(seasons[len(seasons)-1]); season = season.Next() {
			if !known[season.String()] {
				coverage.Missing = append(coverage.Missing, season.String())
			}
		}
		status.Coverage = append(status.Coverage, coverage)
	}
	sort.Slice(status.Coverage, func(i, j int) bool { return status.Coverage[i].CityCode < status.Coverage[j].CityCode })
	return status, nil
}

// staleRun is how long a run may stay "running" before it is taken as interrupted: the
// longest step, a multi-season backfill, takes well under this.
const staleRun = 6 * time.Hour

// describe marks runs whose process died as interrupted and classifies failures.
func (service *DataStatusService) describe(run *domain.ImportRunStatus) {
	if run.Status == "running" && run.FinishedAt == nil && service.now().Sub(run.StartedAt) > staleRun {
		run.Status = "interrupted"
	}
	if run.Status == "failed" {
		run.ErrorKind = ErrorKind(run.Error)
	}
}

// ErrorKind sorts an importer error message into a short category that is safe to show:
// rate_limited, unauthorized, unpublished, missing_input, download, parse, database or other.
func ErrorKind(message string) string {
	text := strings.ToLower(message)
	switch {
	case strings.Contains(text, "429") || strings.Contains(text, "too many requests"):
		return "rate_limited"
	case strings.Contains(text, "401") || strings.Contains(text, "403") || strings.Contains(text, "tdx_client_id") || strings.Contains(text, "token"):
		return "unauthorized"
	case strings.Contains(text, "not published"):
		return "unpublished"
	case strings.Contains(text, "no address points") || strings.Contains(text, "not configured"):
		return "missing_input"
	case strings.Contains(text, "get http") || strings.Contains(text, "unexpected status") || strings.Contains(text, "timeout") ||
		strings.Contains(text, "connection") || strings.Contains(text, "no such host") || strings.Contains(text, "read archive"):
		return "download"
	case strings.Contains(text, "parse") || strings.Contains(text, "decode") || strings.Contains(text, "open archive"):
		return "parse"
	case strings.Contains(text, "upsert") || strings.Contains(text, "copy ") || strings.Contains(text, "insert") || strings.Contains(text, "postgres"):
		return "database"
	}
	return "other"
}
