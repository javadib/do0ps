package queue_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/javadib/do0ps/internal/adapters/queue"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// testJobType is an arbitrary job type used only by these tests; domain.Job
// places no constraint on JobType beyond it being a lookup key for handlers.
const testJobType domain.JobType = "queue_test_job"

// wallClock is a real ports.Clock. The retry tests use small, real timer
// delays, so there is no need for a controllable fake clock here.
type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

// recordingJobs is an in-memory ports.JobRepository that also publishes a
// copy of every updated job on a channel, so tests can observe a job's
// status transitions without polling or sleeping.
type recordingJobs struct {
	mu      sync.Mutex
	jobs    map[string]*domain.Job
	updates chan *domain.Job
}

func newRecordingJobs() *recordingJobs {
	return &recordingJobs{
		jobs: make(map[string]*domain.Job),
		// Generous buffer: a job may go through several retry round trips
		// before this test drains the channel.
		updates: make(chan *domain.Job, 256),
	}
}

func (r *recordingJobs) Create(_ context.Context, job *domain.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[job.ID] = job
	return nil
}

func (r *recordingJobs) Get(_ context.Context, id string) (*domain.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return job, nil
}

func (r *recordingJobs) Update(_ context.Context, job *domain.Job) error {
	r.mu.Lock()
	r.jobs[job.ID] = job
	r.mu.Unlock()

	// Copy the struct before publishing: the pool keeps mutating the same
	// *domain.Job across retries, and the test goroutine reads this copy
	// concurrently with that.
	cp := *job
	r.updates <- &cp
	return nil
}

func (r *recordingJobs) ListUnfinished(context.Context) ([]*domain.Job, error) { return nil, nil }

func (r *recordingJobs) ListDue(context.Context, time.Time, int) ([]*domain.Job, error) {
	return nil, nil
}

// waitForStatus drains jobs.updates until it sees id reach want, or fails the
// test after timeout. It returns the matching snapshot.
func waitForStatus(t *testing.T, updates chan *domain.Job, id string, want domain.JobStatus, timeout time.Duration) *domain.Job {
	t.Helper()
	deadline := time.After(timeout)
	var seen []domain.JobStatus
	for {
		select {
		case job := <-updates:
			if job.ID != id {
				continue
			}
			seen = append(seen, job.Status)
			if job.Status == want {
				return job
			}
		case <-deadline:
			t.Fatalf("job %s did not reach status %s within %s; statuses seen: %v", id, want, timeout, seen)
			return nil
		}
	}
}

// fastRetryOptions makes retry delays short and deterministic enough for
// tests to run in well under a second.
func fastRetryOptions(maxAttempts int) []queue.Option {
	return []queue.Option{
		queue.WithMaxAttempts(maxAttempts),
		queue.WithBackOff(func() backoff.BackOff {
			return backoff.NewConstantBackOff(5 * time.Millisecond)
		}),
	}
}

func newTestPool(t *testing.T, jobs ports.JobRepository, opts ...queue.Option) *queue.Pool {
	t.Helper()
	pool, err := queue.New(jobs, wallClock{}, append([]queue.Option{queue.WithWorkers(2)}, opts...)...)
	if err != nil {
		t.Fatalf("queue.New: %v", err)
	}
	pool.Start(context.Background())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := pool.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
	})
	return pool
}

func newPendingJob(id string) *domain.Job {
	now := time.Now()
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

func TestDispatchFastPathRoundTrip(t *testing.T) {
	pool := newTestPool(t, newRecordingJobs())

	want := json.RawMessage(`{"ok":true}`)
	got, err := pool.Dispatch(context.Background(), func(context.Context) (json.RawMessage, error) {
		return want, nil
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("result = %s, want %s", got, want)
	}
}

func TestDispatchFastPathPropagatesError(t *testing.T) {
	pool := newTestPool(t, newRecordingJobs())

	wantErr := errors.New("provider unavailable")
	_, err := pool.Dispatch(context.Background(), func(context.Context) (json.RawMessage, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("Dispatch error = %v, want %v", err, wantErr)
	}
}

func TestSubmitLongPathProgressesThroughStatuses(t *testing.T) {
	jobs := newRecordingJobs()
	pool := newTestPool(t, jobs)
	pool.Register(testJobType, func(context.Context, *domain.Job) (json.RawMessage, error) {
		return json.RawMessage(`"server-1"`), nil
	})

	job := newPendingJob("job-happy-path")
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := pool.Submit(context.Background(), job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	var sawRunning bool
	deadline := time.After(2 * time.Second)
	for {
		select {
		case u := <-jobs.updates:
			if u.ID != job.ID {
				continue
			}
			switch u.Status {
			case domain.JobStatusRunning:
				sawRunning = true
			case domain.JobStatusDone:
				if !sawRunning {
					t.Error("job reached done without ever being observed running")
				}
				if u.Attempts != 1 {
					t.Errorf("Attempts = %d, want 1", u.Attempts)
				}
				if string(u.Result) != `"server-1"` {
					t.Errorf("Result = %s, want %q", u.Result, `"server-1"`)
				}
				return
			}
		case <-deadline:
			t.Fatal("job never reached done status")
		}
	}
}

func TestSubmitRetriesUntilSuccess(t *testing.T) {
	jobs := newRecordingJobs()
	pool := newTestPool(t, jobs, fastRetryOptions(5)...)

	var calls int32
	pool.Register(testJobType, func(context.Context, *domain.Job) (json.RawMessage, error) {
		if atomic.AddInt32(&calls, 1) < 3 {
			return nil, errors.New("transient failure")
		}
		return json.RawMessage(`"ok"`), nil
	})

	job := newPendingJob("job-retries-then-succeeds")
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := pool.Submit(context.Background(), job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	done := waitForStatus(t, jobs.updates, job.ID, domain.JobStatusDone, 2*time.Second)
	if done.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3 (two failures then a success)", done.Attempts)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("handler invoked %d times, want 3", got)
	}
}

func TestSubmitExhaustsRetriesAndFails(t *testing.T) {
	jobs := newRecordingJobs()
	pool := newTestPool(t, jobs, fastRetryOptions(3)...)

	var calls int32
	wantErr := "boom"
	pool.Register(testJobType, func(context.Context, *domain.Job) (json.RawMessage, error) {
		atomic.AddInt32(&calls, 1)
		return nil, errors.New(wantErr)
	})

	job := newPendingJob("job-exhausts-retries")
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := pool.Submit(context.Background(), job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	failed := waitForStatus(t, jobs.updates, job.ID, domain.JobStatusFailed, 2*time.Second)
	if failed.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3 (maxAttempts)", failed.Attempts)
	}
	if failed.Error != wantErr {
		t.Errorf("Error = %q, want %q", failed.Error, wantErr)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("handler invoked %d times, want 3", got)
	}
}

func TestSubmitUnknownJobTypeIsRejected(t *testing.T) {
	pool := newTestPool(t, newRecordingJobs())

	err := pool.Submit(context.Background(), newPendingJob("job-no-handler"))
	if err == nil {
		t.Fatal("Submit with no registered handler: want error, got nil")
	}
}

func TestShutdownStopsAcceptingWork(t *testing.T) {
	pool, err := queue.New(newRecordingJobs(), wallClock{})
	if err != nil {
		t.Fatalf("queue.New: %v", err)
	}
	pool.Start(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	_, err = pool.Dispatch(context.Background(), func(context.Context) (json.RawMessage, error) {
		return nil, nil
	})
	if !errors.Is(err, queue.ErrClosed) {
		t.Errorf("Dispatch after shutdown: err = %v, want %v", err, queue.ErrClosed)
	}
}
