package queue_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/adapters/queue"
	"github.com/javadib/do0ps/internal/core/domain"
)

// sweepJobs is a recordingJobs whose ListDue behaves like the SQLite store's:
// it returns pending jobs whose retry time has arrived.
type sweepJobs struct {
	*recordingJobs
}

func newSweepJobs() *sweepJobs { return &sweepJobs{recordingJobs: newRecordingJobs()} }

// Create and Update store a copy rather than the caller's pointer. The pool
// mutates a job outside any repository lock, so keeping that same pointer in
// the map would let ListDue read a struct a worker is writing. The SQLite
// store has no such sharing: it scans every row into a fresh struct.

func (r *sweepJobs) Create(_ context.Context, job *domain.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *job
	r.jobs[job.ID] = &cp
	return nil
}

func (r *sweepJobs) Update(_ context.Context, job *domain.Job) error {
	cp := *job
	r.mu.Lock()
	r.jobs[job.ID] = &cp
	r.mu.Unlock()

	published := cp
	r.updates <- &published
	return nil
}

func (r *sweepJobs) ListDue(_ context.Context, now time.Time, limit int) ([]*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var due []*domain.Job
	for _, job := range r.jobs {
		if job.Status != domain.JobStatusPending || job.NextRetryAt.After(now) {
			continue
		}
		// Hand out a copy, as the SQLite store does by scanning each row into
		// a fresh struct. Returning the stored pointer would let the sweep
		// read a job a worker is concurrently mutating.
		cp := *job
		due = append(due, &cp)
		if len(due) == limit {
			break
		}
	}
	return due, nil
}

func pendingJob(id string) *domain.Job {
	now := time.Now().Add(-time.Minute)
	return &domain.Job{
		ID:          id,
		Type:        testJobType,
		Payload:     json.RawMessage(`{}`),
		Status:      domain.JobStatusPending,
		NextRetryAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// A job the pool dropped — persisted as pending, but with no timer and no
// claim behind it — is the whole reason the sweep exists.
func TestSweepRecoversDroppedJob(t *testing.T) {
	jobs := newSweepJobs()

	var runs atomic.Int32
	pool := newTestPool(t, jobs, queue.WithSweepInterval(10*time.Millisecond))
	pool.Register(testJobType, func(context.Context, *domain.Job) (json.RawMessage, error) {
		runs.Add(1)
		return json.RawMessage(`{"ok":true}`), nil
	})

	job := pendingJob("dropped-1")
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("seeding job: %v", err)
	}

	pool.Start(context.Background())
	t.Cleanup(func() { _ = pool.Shutdown(context.Background()) })

	done := waitForStatus(t, jobs.updates, job.ID, domain.JobStatusDone, 2*time.Second)
	if done.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", done.Attempts)
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("handler ran %d times, want 1", got)
	}
}

// The safety property that matters most. A job left behind by a previous
// process carries no credentials — they were memory-only — so running it would
// fail on authentication and burn its retry budget down to a terminal failure,
// which is exactly what would stop get_operation_status from ever reconciling
// it against the provider (AGENTS.md 4.4).
func TestSweepSkipsInterruptedJob(t *testing.T) {
	jobs := newSweepJobs()

	var runs atomic.Int32
	pool := newTestPool(t, jobs, queue.WithSweepInterval(10*time.Millisecond))
	pool.Register(testJobType, func(context.Context, *domain.Job) (json.RawMessage, error) {
		runs.Add(1)
		return nil, nil
	})

	job := pendingJob("interrupted-1")
	if err := job.MarkInterrupted(time.Now()); err != nil {
		t.Fatalf("marking interrupted: %v", err)
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("seeding job: %v", err)
	}

	pool.Start(context.Background())
	t.Cleanup(func() { _ = pool.Shutdown(context.Background()) })

	// Several sweep ticks' worth of time, so a wrongly-swept job would show up.
	time.Sleep(150 * time.Millisecond)

	if got := runs.Load(); got != 0 {
		t.Fatalf("interrupted job was run %d times; it must be left for reconciliation", got)
	}
	stored, err := jobs.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("reading job: %v", err)
	}
	if !stored.WasInterrupted() {
		t.Fatalf("job no longer reads as interrupted: status=%s error=%q", stored.Status, stored.Error)
	}
}

// A job the pool already accepted must not also be picked up by the sweep.
func TestSweepSkipsClaimedJob(t *testing.T) {
	jobs := newSweepJobs()

	release := make(chan struct{})
	var runs atomic.Int32

	pool := newTestPool(t, jobs, queue.WithSweepInterval(10*time.Millisecond))
	pool.Register(testJobType, func(context.Context, *domain.Job) (json.RawMessage, error) {
		runs.Add(1)
		<-release // hold the job open across several sweep ticks
		return json.RawMessage(`{}`), nil
	})

	job := pendingJob("claimed-1")
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("seeding job: %v", err)
	}

	pool.Start(context.Background())
	t.Cleanup(func() {
		close(release)
		_ = pool.Shutdown(context.Background())
	})

	if err := pool.Submit(context.Background(), job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	if got := runs.Load(); got != 1 {
		t.Fatalf("handler ran %d times, want exactly 1", got)
	}
}

// Credentials must survive every retry and be released exactly once, when the
// job can no longer be re-attempted.
func TestSettledFiresOnceOnTerminalState(t *testing.T) {
	jobs := newRecordingJobs()

	var settled atomic.Int32
	var attempts atomic.Int32

	opts := append(fastRetryOptions(3), queue.WithSweepInterval(0))
	pool := newTestPool(t, jobs, opts...)
	pool.Register(testJobType, func(context.Context, *domain.Job) (json.RawMessage, error) {
		attempts.Add(1)
		return nil, errors.New("transient failure")
	})
	pool.RegisterSettled(testJobType, func(string) { settled.Add(1) })

	pool.Start(context.Background())
	t.Cleanup(func() { _ = pool.Shutdown(context.Background()) })

	job := pendingJob("settle-1")
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("seeding job: %v", err)
	}
	if err := pool.Submit(context.Background(), job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	waitForStatus(t, jobs.updates, job.ID, domain.JobStatusFailed, 2*time.Second)

	if got := attempts.Load(); got != 3 {
		t.Fatalf("handler ran %d times, want 3 (the full retry budget)", got)
	}
	// Give any stray late call a chance to land before asserting.
	time.Sleep(50 * time.Millisecond)
	if got := settled.Load(); got != 1 {
		t.Fatalf("settled called %d times, want exactly 1", got)
	}
}
