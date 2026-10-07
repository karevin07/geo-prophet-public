package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
)

type usageStub struct{}

func (usageStub) DatabaseUsage(context.Context) (domain.DatabaseUsage, error) {
	return domain.DatabaseUsage{Bytes: 1_390_000_000, Transactions: 1_433_022}, nil
}

// lastParams records what EnqueueJob stored.
var lastParams any

func (usageStub) EnqueueJob(_ context.Context, kind domain.JobKind, params any, requestedBy string) (domain.AdminJob, error) {
	if kind == domain.JobTransit {
		return domain.AdminJob{}, domain.ErrJobActive
	}
	lastParams = params
	return domain.AdminJob{ID: 1, Kind: kind, Status: domain.JobQueued, RequestedBy: requestedBy}, nil
}

func (usageStub) Openings(context.Context) (domain.Openings, error) { return domain.Openings{}, nil }

func (usageStub) SaveOpening(_ context.Context, edit domain.OpeningEdit, _ string) error {
	if edit.TargetID == "NOPE" {
		return domain.ErrUnknownTarget
	}
	return nil
}

func (usageStub) Jobs(context.Context) ([]domain.AdminJob, error) { return []domain.AdminJob{}, nil }

type backupStub struct{}

func (backupStub) Backups() ([]domain.BackupFile, error) {
	return []domain.BackupFile{{Name: "geoprophet-20261001T003435Z.dump", Bytes: 127 << 20}}, nil
}

func (backupStub) Disk() (domain.DiskUsage, error) {
	return domain.DiskUsage{TotalBytes: 194 << 30, UsedBytes: 169 << 30, FreeBytes: 25 << 30}, nil
}

func TestAdminSessionAllowsListedTailscaleLogins(t *testing.T) {
	service := usecase.NewAdminService(usageStub{}, usecase.AdminSettings{Logins: []string{" Admin@Example.com ", ""}})
	cases := map[string]bool{"admin@example.com": true, "ADMIN@example.com": true, "other@example.com": false, "": false}
	for login, want := range cases {
		if got := service.Session(domain.Identity{Login: login}).Admin; got != want {
			t.Errorf("Session(%q).Admin = %t, want %t", login, got, want)
		}
	}
	if usecase.NewAdminService(usageStub{}, usecase.AdminSettings{}).Session(domain.Identity{Login: "admin@example.com"}).Admin {
		t.Error("an empty allow list must admit nobody")
	}
}

func TestAdminSystemReportsBackupsWhenMounted(t *testing.T) {
	status, err := usecase.NewAdminService(usageStub{}, usecase.AdminSettings{Backups: backupStub{}, BackupKeep: 7, BackupIntervalHours: 24}).System(context.Background())
	if err != nil || status.Disk == nil || len(status.Backups) != 1 || status.BackupKeep != 7 || status.Database.Transactions != 1_433_022 {
		t.Fatalf("status = %+v, %v", status, err)
	}
	status, err = usecase.NewAdminService(usageStub{}, usecase.AdminSettings{}).System(context.Background())
	if err != nil || status.Disk != nil || status.Backups == nil {
		t.Fatalf("expected no disk and an empty backup list without a mount, got %+v, %v", status, err)
	}
}

func TestAdminEnqueueValidatesKindsAndReportsConflicts(t *testing.T) {
	service := usecase.NewAdminService(usageStub{}, usecase.AdminSettings{})
	if job, err := service.Enqueue(context.Background(), domain.JobBackup, nil, "admin@example.com"); err != nil || job.RequestedBy != "admin@example.com" {
		t.Fatalf("Enqueue backup = %+v, %v", job, err)
	}
	if _, err := service.Enqueue(context.Background(), "drop_tables", nil, "admin@example.com"); !errors.Is(err, usecase.ErrInvalidInput) {
		t.Fatalf("expected an unknown kind to be invalid, got %v", err)
	}
	if _, err := service.Enqueue(context.Background(), domain.JobTransit, nil, "admin@example.com"); !errors.Is(err, usecase.ErrConflict) {
		t.Fatalf("expected a pending kind to conflict, got %v", err)
	}
}

func TestAdminEnqueueValidatesSeasons(t *testing.T) {
	service := usecase.NewAdminService(usageStub{}, usecase.AdminSettings{})
	valid := `{"seasons":"105S1 - 115S2","cities":["a","h"]}`
	if _, err := service.Enqueue(context.Background(), domain.JobSeasons, []byte(valid), "admin@example.com"); err != nil {
		t.Fatalf("valid seasons rejected: %v", err)
	}
	if stored, ok := lastParams.(usecase.SeasonsParams); !ok || stored.Seasons != "105S1-115S2" {
		t.Fatalf("stored params = %#v", lastParams)
	}
	for _, params := range []string{`{"seasons":"115S5","cities":["a"]}`, `{"seasons":"90S1-115S2","cities":["a"]}`, `{"seasons":"115S2","cities":[]}`, `{"seasons":"115S2","cities":["e"]}`, `not json`} {
		if _, err := service.Enqueue(context.Background(), domain.JobSeasons, []byte(params), "admin@example.com"); !errors.Is(err, usecase.ErrInvalidInput) {
			t.Errorf("%s: expected invalid input, got %v", params, err)
		}
	}
}

func TestAdminSaveOpeningValidatesAndNotifies(t *testing.T) {
	changed := 0
	service := usecase.NewAdminService(usageStub{}, usecase.AdminSettings{OpeningsChanged: func() { changed++ }})
	day := "2026-06-30"
	if err := service.SaveOpening(context.Background(), domain.OpeningEdit{Kind: "line", TargetID: "NTMC-LB", OpenedOn: &day}, "admin@example.com"); err != nil || changed != 1 {
		t.Fatalf("save = %v, changed %d", err, changed)
	}
	bad := "1850-01-01"
	if err := service.SaveOpening(context.Background(), domain.OpeningEdit{Kind: "line", TargetID: "NTMC-LB", OpenedOn: &bad}, ""); !errors.Is(err, usecase.ErrInvalidInput) {
		t.Fatalf("expected an implausible date to be rejected, got %v", err)
	}
	if err := service.SaveOpening(context.Background(), domain.OpeningEdit{Kind: "line", TargetID: "NOPE"}, ""); !errors.Is(err, usecase.ErrNotFound) || changed != 1 {
		t.Fatalf("expected not found without notifying, got %v (changed %d)", err, changed)
	}
}
