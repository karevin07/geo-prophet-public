package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// keptJobs bounds the audit history; older finished jobs are pruned on each request.
const keptJobs = 50

const jobColumns = `id, kind, status, requested_by, requested_at, params, started_at, finished_at, result, COALESCE(error, '')`

func scanJob(row pgx.Row) (domain.AdminJob, error) {
	var job domain.AdminJob
	var params, result []byte
	err := row.Scan(&job.ID, &job.Kind, &job.Status, &job.RequestedBy, &job.RequestedAt, &params, &job.StartedAt, &job.FinishedAt, &result, &job.Error)
	job.Params, job.Result = params, result
	return job, err
}

// EnqueueJob queues a job, failing with domain.ErrJobActive when one of its kind is pending.
func (repository *TransactionRepository) EnqueueJob(ctx context.Context, kind domain.JobKind, params any, requestedBy string) (domain.AdminJob, error) {
	encoded, err := json.Marshal(params)
	if err != nil {
		return domain.AdminJob{}, fmt.Errorf("encode job params: %w", err)
	}
	var job domain.AdminJob
	err = pgx.BeginFunc(ctx, repository.pool, func(tx pgx.Tx) error {
		var err error
		job, err = scanJob(tx.QueryRow(ctx, `INSERT INTO admin_jobs (kind, params, requested_by) VALUES ($1, $2, $3) RETURNING `+jobColumns, kind, encoded, requestedBy))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM admin_jobs WHERE status IN ('succeeded', 'failed')
            AND id NOT IN (SELECT id FROM admin_jobs ORDER BY id DESC LIMIT $1)`, keptJobs)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation on admin_jobs_one_active_idx
		return domain.AdminJob{}, domain.ErrJobActive
	}
	if err != nil {
		return domain.AdminJob{}, fmt.Errorf("enqueue job: %w", err)
	}
	return job, nil
}

// Jobs lists the most recent jobs, newest first.
func (repository *TransactionRepository) Jobs(ctx context.Context) ([]domain.AdminJob, error) {
	rows, err := repository.pool.Query(ctx, `SELECT `+jobColumns+` FROM admin_jobs ORDER BY id DESC LIMIT $1`, keptJobs)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()
	jobs := []domain.AdminJob{}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// ClaimJob marks the oldest queued job of the given kinds as running and returns it, or
// false when none is queued. SKIP LOCKED keeps two runners from taking the same job.
func (store *ImportStore) ClaimJob(ctx context.Context, kinds []domain.JobKind) (domain.AdminJob, bool, error) {
	names := make([]string, len(kinds))
	for index, kind := range kinds {
		names[index] = string(kind)
	}
	job, err := scanJob(store.pool.QueryRow(ctx, `UPDATE admin_jobs SET status = 'running', started_at = NOW()
        WHERE id = (SELECT id FROM admin_jobs WHERE status = 'queued' AND kind = ANY($1)
                    ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED)
        RETURNING `+jobColumns, names))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminJob{}, false, nil
	}
	if err != nil {
		return domain.AdminJob{}, false, fmt.Errorf("claim job: %w", err)
	}
	return job, true, nil
}

// FinishJob records a job's result, or its error when jobErr is not nil.
func (store *ImportStore) FinishJob(ctx context.Context, id int64, result any, jobErr error) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode job result: %w", err)
	}
	status, message := domain.JobSucceeded, (*string)(nil)
	if jobErr != nil {
		status = domain.JobFailed
		text := jobErr.Error()
		message = &text
	}
	if _, err = store.pool.Exec(ctx, `UPDATE admin_jobs SET status = $2, finished_at = NOW(), result = $3, error = $4 WHERE id = $1`,
		id, status, encoded, message); err != nil {
		return fmt.Errorf("finish job: %w", err)
	}
	return nil
}

// FailInterruptedJobs fails jobs of the given kinds left running by a runner that stopped,
// so they no longer block new requests of their kind.
func (store *ImportStore) FailInterruptedJobs(ctx context.Context, kinds []domain.JobKind) (int, error) {
	names := make([]string, len(kinds))
	for index, kind := range kinds {
		names[index] = string(kind)
	}
	tag, err := store.pool.Exec(ctx, `UPDATE admin_jobs SET status = 'failed', finished_at = NOW(),
        error = 'interrupted: the importer stopped before the job finished'
        WHERE status = 'running' AND kind = ANY($1)`, names)
	if err != nil {
		return 0, fmt.Errorf("fail interrupted jobs: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ImportSummary totals the import runs started since a job began.
func (store *ImportStore) ImportSummary(ctx context.Context, since time.Time) (domain.ImportSummary, error) {
	var summary domain.ImportSummary
	err := store.pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'failed'), COALESCE(SUM(rows_upserted), 0)
        FROM import_runs WHERE started_at >= $1`, since).Scan(&summary.Runs, &summary.FailedRuns, &summary.RowsUpserted)
	if err != nil {
		return summary, fmt.Errorf("summarize job runs: %w", err)
	}
	return summary, nil
}
