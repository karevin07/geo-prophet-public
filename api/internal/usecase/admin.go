package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

// AdminRepository reads system health and keeps the admin job queue.
type AdminRepository interface {
	DatabaseUsage(ctx context.Context) (domain.DatabaseUsage, error)
	// EnqueueJob fails with domain.ErrJobActive when a job of the kind is pending.
	EnqueueJob(ctx context.Context, kind domain.JobKind, params any, requestedBy string) (domain.AdminJob, error)
	Jobs(ctx context.Context) ([]domain.AdminJob, error)
	Openings(ctx context.Context) (domain.Openings, error)
	// SaveOpening fails with domain.ErrUnknownTarget when the line or platform is not stored.
	SaveOpening(ctx context.Context, edit domain.OpeningEdit, updatedBy string) error
}

// ErrConflict means the request clashes with current state, e.g. a job already queued.
var ErrConflict = errors.New("conflict")

// BackupStore reads the backup service's dumps and the space left beside them.
type BackupStore interface {
	Backups() ([]domain.BackupFile, error)
	Disk() (domain.DiskUsage, error)
}

// AdminSettings configure the admin console.
type AdminSettings struct {
	// Logins are the Tailscale logins allowed in, e.g. someone@example.com.
	Logins []string
	// Backups is nil when the backup directory is not mounted into the API.
	Backups             BackupStore
	BackupKeep          int
	BackupIntervalHours int
	// OpeningsChanged runs after an opening date is edited, e.g. to drop cached results.
	OpeningsChanged func()
}

// AdminService decides who may use the admin console and reports system health.
type AdminService struct {
	repository AdminRepository
	settings   AdminSettings
	logins     map[string]bool
	now        func() time.Time
}

func NewAdminService(repository AdminRepository, settings AdminSettings) *AdminService {
	logins := make(map[string]bool, len(settings.Logins))
	for _, login := range settings.Logins {
		if login = strings.ToLower(strings.TrimSpace(login)); login != "" {
			logins[login] = true
		}
	}
	return &AdminService{repository: repository, settings: settings, logins: logins, now: time.Now}
}

// Session reports whether identity may use the console. A request that did not come
// through `tailscale serve` has no login and is never an admin.
func (service *AdminService) Session(identity domain.Identity) domain.AdminSession {
	return domain.AdminSession{Identity: identity, Admin: identity.Login != "" && service.logins[strings.ToLower(identity.Login)]}
}

// System reports database size, disk space and backups.
func (service *AdminService) System(ctx context.Context) (domain.SystemStatus, error) {
	usage, err := service.repository.DatabaseUsage(ctx)
	if err != nil {
		return domain.SystemStatus{}, err
	}
	status := domain.SystemStatus{
		GeneratedAt: service.now().UTC(), Database: usage, Backups: []domain.BackupFile{},
		BackupKeep: service.settings.BackupKeep, BackupIntervalHours: service.settings.BackupIntervalHours,
	}
	if service.settings.Backups == nil {
		return status, nil
	}
	if status.Backups, err = service.settings.Backups.Backups(); err != nil {
		return domain.SystemStatus{}, err
	}
	disk, err := service.settings.Backups.Disk()
	if err != nil {
		return domain.SystemStatus{}, err
	}
	status.Disk = &disk
	return status, nil
}

// Jobs lists recent admin jobs, newest first.
func (service *AdminService) Jobs(ctx context.Context) ([]domain.AdminJob, error) {
	return service.repository.Jobs(ctx)
}

// SeasonsParams are a seasons job's input: which 實價登錄 seasons to re-import, for which
// cities.
type SeasonsParams struct {
	Seasons string   `json:"seasons"` // 115S2, 114S1,114S2 or 105S1-115S2
	Cities  []string `json:"cities"`
}

// maxJobSeasons bounds one re-import; the full history since 101S1 is about 60 seasons.
const maxJobSeasons = 60

// Enqueue queues a job requested by an admin; the importer or backup service runs it.
// Only seasons jobs take params.
func (service *AdminService) Enqueue(ctx context.Context, kind domain.JobKind, params json.RawMessage, requestedBy string) (domain.AdminJob, error) {
	if !slices.Contains(domain.JobKinds, kind) {
		return domain.AdminJob{}, fmt.Errorf("%w: unknown job kind %q", ErrInvalidInput, kind)
	}
	var stored any = struct{}{}
	if kind == domain.JobSeasons {
		var seasons SeasonsParams
		if err := json.Unmarshal(params, &seasons); err != nil {
			return domain.AdminJob{}, fmt.Errorf("%w: params must be {\"seasons\", \"cities\"}", ErrInvalidInput)
		}
		parsed, err := lvr.ParseSeasons(strings.ReplaceAll(seasons.Seasons, " ", ""))
		if err != nil || len(parsed) == 0 || len(parsed) > maxJobSeasons {
			return domain.AdminJob{}, fmt.Errorf("%w: seasons must be 1 to %d seasons such as 115S2 or 105S1-115S2", ErrInvalidInput, maxJobSeasons)
		}
		if len(seasons.Cities) == 0 {
			return domain.AdminJob{}, fmt.Errorf("%w: choose at least one city", ErrInvalidInput)
		}
		for _, city := range seasons.Cities {
			if _, ok := addresspoint.Sources[city]; !ok {
				return domain.AdminJob{}, fmt.Errorf("%w: unknown city %q", ErrInvalidInput, city)
			}
		}
		seasons.Seasons = strings.ReplaceAll(seasons.Seasons, " ", "")
		stored = seasons
	}
	job, err := service.repository.EnqueueJob(ctx, kind, stored, requestedBy)
	if errors.Is(err, domain.ErrJobActive) {
		return domain.AdminJob{}, fmt.Errorf("%w: %w", ErrConflict, err)
	}
	return job, err
}

// Openings lists line and platform opening dates for editing.
func (service *AdminService) Openings(ctx context.Context) (domain.Openings, error) {
	return service.repository.Openings(ctx)
}

// SaveOpening sets or removes an opening date, recording who changed it.
func (service *AdminService) SaveOpening(ctx context.Context, edit domain.OpeningEdit, updatedBy string) error {
	if edit.Kind != "line" && edit.Kind != "platform" {
		return fmt.Errorf("%w: kind must be line or platform", ErrInvalidInput)
	}
	if edit.TargetID == "" {
		return fmt.Errorf("%w: targetId is required", ErrInvalidInput)
	}
	if edit.OpenedOn != nil {
		day, err := time.Parse(time.DateOnly, *edit.OpenedOn)
		if err != nil || day.Year() < 1990 || day.Year() > 2100 {
			return fmt.Errorf("%w: openedOn must be a date such as 2020-01-31", ErrInvalidInput)
		}
	}
	if edit.Source = strings.TrimSpace(edit.Source); len(edit.Source) > 500 {
		return fmt.Errorf("%w: source is too long", ErrInvalidInput)
	}
	err := service.repository.SaveOpening(ctx, edit, updatedBy)
	if errors.Is(err, domain.ErrUnknownTarget) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	if err == nil && service.settings.OpeningsChanged != nil {
		service.settings.OpeningsChanged()
	}
	return err
}
