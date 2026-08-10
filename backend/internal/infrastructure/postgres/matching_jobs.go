package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	applicationmatching "swap-chain/internal/application/matching"
)

// MatchingJobs stores recoverable leases for the automatic matching worker.
type MatchingJobs struct {
	database *sql.DB
}

func NewMatchingJobs(database *sql.DB) (*MatchingJobs, error) {
	if database == nil {
		return nil, fmt.Errorf("matching jobs repository: database is required")
	}
	return &MatchingJobs{database: database}, nil
}

func (r *MatchingJobs) Claim(
	ctx context.Context,
	staleBefore time.Time,
	batchSize int,
) ([]applicationmatching.Job, error) {
	rows, err := r.database.QueryContext(ctx, `
		WITH claimable AS (
			SELECT job.item_id
			FROM matching_jobs AS job
			WHERE (job.status = 'PENDING' AND job.available_at <= NOW())
			   OR (job.status = 'PROCESSING' AND job.locked_at < $1)
			ORDER BY COALESCE(job.locked_at, job.available_at), job.item_id
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		UPDATE matching_jobs AS job
		SET status = 'PROCESSING',
		    attempts = job.attempts + 1,
		    locked_at = NOW(),
		    last_error = NULL,
		    updated_at = NOW()
		FROM claimable
		WHERE job.item_id = claimable.item_id
		RETURNING job.item_id, job.attempts`, staleBefore, batchSize)
	if err != nil {
		return nil, fmt.Errorf("claim matching jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	jobs := make([]applicationmatching.Job, 0, batchSize)
	for rows.Next() {
		var job applicationmatching.Job
		if err := rows.Scan(&job.ItemID, &job.Attempts); err != nil {
			return nil, fmt.Errorf("scan matching job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate matching jobs: %w", err)
	}
	return jobs, nil
}

func (r *MatchingJobs) Complete(ctx context.Context, itemID int64) (bool, error) {
	result, err := r.database.ExecContext(ctx, `
		UPDATE matching_jobs
		SET status = 'DONE',
		    locked_at = NULL,
		    last_error = NULL,
		    updated_at = NOW()
		WHERE item_id = $1
		  AND status = 'PROCESSING'`, itemID)
	if err != nil {
		return false, fmt.Errorf("complete matching job for item %d: %w", itemID, err)
	}
	return exactlyOneRowAffected(result, "complete matching job", itemID)
}

func (r *MatchingJobs) Retry(
	ctx context.Context,
	itemID int64,
	availableAt time.Time,
	lastError string,
) (bool, error) {
	result, err := r.database.ExecContext(ctx, `
		UPDATE matching_jobs
		SET status = 'PENDING',
		    available_at = $2,
		    locked_at = NULL,
		    last_error = $3,
		    updated_at = NOW()
		WHERE item_id = $1
		  AND status = 'PROCESSING'`, itemID, availableAt, lastError)
	if err != nil {
		return false, fmt.Errorf("retry matching job for item %d: %w", itemID, err)
	}
	return exactlyOneRowAffected(result, "retry matching job", itemID)
}

func exactlyOneRowAffected(result sql.Result, operation string, itemID int64) (bool, error) {
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("%s for item %d: rows affected: %w", operation, itemID, err)
	}
	if rows > 1 {
		return false, fmt.Errorf("%s for item %d: unexpected affected rows %d", operation, itemID, rows)
	}
	return rows == 1, nil
}

var _ applicationmatching.JobRepository = (*MatchingJobs)(nil)
