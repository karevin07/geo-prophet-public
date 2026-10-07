package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

// JobStore is the admin job queue as the importer sees it.
type JobStore interface {
	ClaimJob(ctx context.Context, kinds []domain.JobKind) (domain.AdminJob, bool, error)
	FinishJob(ctx context.Context, id int64, result any, jobErr error) error
	FailInterruptedJobs(ctx context.Context, kinds []domain.JobKind) (int, error)
	ImportSummary(ctx context.Context, since time.Time) (domain.ImportSummary, error)
}

// ImporterJobs are the job kinds the importer runs; backups belong to the backup service.
var ImporterJobs = []domain.JobKind{domain.JobSync, domain.JobCurrentRelease, domain.JobTransit, domain.JobSeasons}

// Jobs runs admin console requests with the importer's settings.
type Jobs struct {
	Service *Service
	Store   JobStore
	Cities  []string
	Options SyncOptions
}

// RecoverInterrupted fails jobs a previous importer process left running.
func (jobs *Jobs) RecoverInterrupted(ctx context.Context) error {
	count, err := jobs.Store.FailInterruptedJobs(ctx, ImporterJobs)
	if count > 0 {
		jobs.Service.logger.Warn("failed jobs interrupted by a restart", "count", count)
	}
	return err
}

// RunQueued runs queued jobs one at a time until none is left.
func (jobs *Jobs) RunQueued(ctx context.Context) error {
	for ctx.Err() == nil {
		job, found, err := jobs.Store.ClaimJob(ctx, ImporterJobs)
		if err != nil || !found {
			return err
		}
		started := jobs.Service.now()
		jobs.Service.logger.Info("running admin job", "id", job.ID, "kind", job.Kind, "requested_by", job.RequestedBy)
		jobErr := jobs.run(ctx, job)
		summary, err := jobs.Store.ImportSummary(context.WithoutCancel(ctx), started)
		if err != nil {
			jobErr = errors.Join(jobErr, err)
		}
		// Record the outcome even if ctx was cancelled mid-job.
		finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		err = jobs.Store.FinishJob(finishCtx, job.ID, summary, jobErr)
		cancel()
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (jobs *Jobs) run(ctx context.Context, job domain.AdminJob) error {
	switch job.Kind {
	case domain.JobSync:
		// A requested sync refreshes stale stations too, like the scheduled one.
		return jobs.Service.Sync(ctx, jobs.Cities, jobs.Options)
	case domain.JobCurrentRelease:
		// Re-import even when this release was imported, e.g. after fixing a parser.
		return jobs.Service.ImportCurrentRelease(ctx, jobs.Cities, Datasets, "", false)
	case domain.JobTransit:
		return jobs.Service.ImportTransit(ctx)
	case domain.JobSeasons:
		// The API validated the params; parse them again rather than trust the row.
		var params struct {
			Seasons string   `json:"seasons"`
			Cities  []string `json:"cities"`
		}
		if err := json.Unmarshal(job.Params, &params); err != nil {
			return fmt.Errorf("read seasons job params: %w", err)
		}
		seasons, err := lvr.ParseSeasons(params.Seasons)
		if err != nil {
			return err
		}
		return jobs.Service.ImportSeasons(ctx, seasons, params.Cities, Datasets, "")
	}
	return fmt.Errorf("the importer does not run %q jobs", job.Kind)
}
