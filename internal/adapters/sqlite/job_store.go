package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// JobStore persists domain jobs in SQLite.
type JobStore struct {
	db *sql.DB
}

var _ ports.JobRepository = (*JobStore)(nil)

// NewJobStore wraps an open database handle.
func NewJobStore(db *sql.DB) *JobStore {
	return &JobStore{db: db}
}

const jobColumns = `id, tenant_id, type, payload, status, attempts, next_retry_at, result, error, created_at, updated_at`

// Create inserts a new job.
func (s *JobStore) Create(ctx context.Context, job *domain.Job) error {
	const query = `INSERT INTO jobs (` + jobColumns + `) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		job.ID,
		job.TenantID,
		string(job.Type),
		[]byte(job.Payload),
		job.Status.String(),
		job.Attempts,
		toMillis(job.NextRetryAt),
		[]byte(job.Result),
		job.Error,
		toMillis(job.CreatedAt),
		toMillis(job.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("inserting job %s: %w", job.ID, err)
	}
	return nil
}

// Get loads a job by ID, returning domain.ErrNotFound when it is unknown.
func (s *JobStore) Get(ctx context.Context, id string) (*domain.Job, error) {
	const query = `SELECT ` + jobColumns + ` FROM jobs WHERE id = ?`

	job, err := scanJob(s.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("job %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("loading job %s: %w", id, err)
	}
	return job, nil
}

// Update writes the mutable fields of an existing job.
func (s *JobStore) Update(ctx context.Context, job *domain.Job) error {
	const query = `
UPDATE jobs
   SET status = ?, attempts = ?, next_retry_at = ?, result = ?, error = ?, updated_at = ?
 WHERE id = ?`

	res, err := s.db.ExecContext(ctx, query,
		job.Status.String(),
		job.Attempts,
		toMillis(job.NextRetryAt),
		[]byte(job.Result),
		job.Error,
		toMillis(job.UpdatedAt),
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("updating job %s: %w", job.ID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking update of job %s: %w", job.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("job %s: %w", job.ID, domain.ErrNotFound)
	}
	return nil
}

// ListUnfinished returns every job that has not reached a terminal state.
func (s *JobStore) ListUnfinished(ctx context.Context) ([]*domain.Job, error) {
	const query = `SELECT ` + jobColumns + ` FROM jobs WHERE status IN ('pending', 'running') ORDER BY created_at`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing unfinished jobs: %w", err)
	}
	defer rows.Close()

	return scanJobs(rows)
}

// ListDue returns at most limit pending jobs whose retry time has arrived.
func (s *JobStore) ListDue(ctx context.Context, now time.Time, limit int) ([]*domain.Job, error) {
	const query = `
SELECT ` + jobColumns + `
  FROM jobs
 WHERE status = 'pending' AND next_retry_at <= ?
 ORDER BY next_retry_at
 LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, toMillis(now), limit)
	if err != nil {
		return nil, fmt.Errorf("listing due jobs: %w", err)
	}
	defer rows.Close()

	return scanJobs(rows)
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (*domain.Job, error) {
	var (
		job         domain.Job
		jobType     string
		status      string
		payload     []byte
		result      []byte
		nextRetryAt int64
		createdAt   int64
		updatedAt   int64
	)

	if err := row.Scan(
		&job.ID,
		&job.TenantID,
		&jobType,
		&payload,
		&status,
		&job.Attempts,
		&nextRetryAt,
		&result,
		&job.Error,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}

	parsed, err := domain.ParseJobStatus(status)
	if err != nil {
		return nil, fmt.Errorf("job %s has status %q: %w", job.ID, status, err)
	}

	job.Type = domain.JobType(jobType)
	job.Status = parsed
	job.Payload = json.RawMessage(payload)
	job.Result = json.RawMessage(result)
	job.NextRetryAt = fromMillis(nextRetryAt)
	job.CreatedAt = fromMillis(createdAt)
	job.UpdatedAt = fromMillis(updatedAt)

	return &job, nil
}

func scanJobs(rows *sql.Rows) ([]*domain.Job, error) {
	var jobs []*domain.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning job row: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating job rows: %w", err)
	}
	return jobs, nil
}

// Times are stored as milliseconds since the Unix epoch: portable across
// SQLite versions and trivially comparable in SQL.
func toMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UTC().UnixMilli()
}

func fromMillis(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
