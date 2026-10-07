package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
)

type dataStatusRepositoryStub struct {
	snapshot domain.DataStatusSnapshot
	err      error
}

func (stub dataStatusRepositoryStub) DataStatusSnapshot(context.Context) (domain.DataStatusSnapshot, error) {
	return stub.snapshot, stub.err
}

func TestDataStatusFindsOnlyGapsBetweenImportedSeasons(t *testing.T) {
	service := usecase.NewDataStatusService(dataStatusRepositoryStub{snapshot: domain.DataStatusSnapshot{
		ImportedSeasons: map[string][]string{
			"a": {"114S4", "115S2", "115S2"},
			"f": {"115S1"},
		},
	}})
	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Coverage) != 2 {
		t.Fatalf("coverage = %#v", status.Coverage)
	}
	if got := status.Coverage[0]; got.CityCode != "a" || len(got.Missing) != 1 || got.Missing[0] != "115S1" {
		t.Fatalf("a coverage = %#v", got)
	}
	if got := status.Coverage[1]; got.CityCode != "f" || len(got.Missing) != 0 {
		t.Fatalf("f coverage = %#v", got)
	}
	if status.RecentRuns == nil || status.LatestRuns == nil || status.Cities == nil || status.Quality.Metrics == nil || status.Quality.Alerts == nil {
		t.Fatal("empty collections should be JSON arrays")
	}
}

func TestDataStatusPropagatesRepositoryError(t *testing.T) {
	want := errors.New("database unavailable")
	service := usecase.NewDataStatusService(dataStatusRepositoryStub{err: want})
	_, err := service.Status(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestErrorKind(t *testing.T) {
	for message, want := range map[string]string{
		"NTMC: get https://tdx.transportdata.tw/api/basic/v2/Rail/Metro/StationExit/NTMC?%24format=JSON: 429 Too Many Requests": "rate_limited",
		"request TDX token: 401 Unauthorized; check TDX_CLIENT_ID and TDX_CLIENT_SECRET":                                        "unauthorized",
		"season 115S3: season archive is not published":                                                                         "unpublished",
		"no address points loaded for city; run the addresses import first":                                                     "missing_input",
		"get https://plvr.land.moi.gov.tw/Download: unexpected status 502 Bad Gateway":                                          "download",
		"parse address points: bad header":                                                                                      "parse",
		"upsert transactions: deadlock detected":                                                                                "database",
		"something else":                                                                                                        "other",
	} {
		if got := usecase.ErrorKind(message); got != want {
			t.Errorf("ErrorKind(%q) = %s, want %s", message, got, want)
		}
	}
}

func TestDataStatusDescribesRuns(t *testing.T) {
	started := time.Now().Add(-7 * time.Hour)
	service := usecase.NewDataStatusService(dataStatusRepositoryStub{snapshot: domain.DataStatusSnapshot{
		RecentRuns: []domain.ImportRunStatus{
			{ID: 1, Status: "failed", Error: "get https://tdx.example/v2: 429 Too Many Requests"},
			{ID: 2, Status: "running", StartedAt: started},
			{ID: 3, Status: "running", StartedAt: time.Now()},
		},
	}})
	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runs := status.RecentRuns
	if runs[0].ErrorKind != "rate_limited" || runs[1].Status != "interrupted" || runs[2].Status != "running" {
		t.Fatalf("runs = %+v", runs)
	}
}
