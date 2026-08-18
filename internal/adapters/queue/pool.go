// Package queue is the secondary adapter implementing ports.Queue with Go
// channels and a bounded in-process worker pool. No external broker is
// involved, and nothing outside this package touches a channel.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ErrQueueFull is returned when the pool cannot accept more work. Every buffer
// here is bounded on purpose: shedding load is better than growing until the
// process dies.
var ErrQueueFull = errors.New("queue is full")

// ErrClosed is returned once the pool has been shut down.
var ErrClosed = errors.New("queue is closed")

// defaultMaxAttempts bounds retries when a job type has no explicit policy.
const defaultMaxAttempts = 5

type unit func(ctx context.Context)

// Pool executes fast tasks and persisted jobs on a fixed set of workers.
type Pool struct {
	jobs   ports.JobRepository
	clock  ports.Clock
	logger *slog.Logger

	workers  int
	capacity int

	work chan unit
	wg   sync.WaitGroup

	mu       sync.RWMutex
	handlers map[domain.JobType]ports.JobHandler
	started  bool
	closed   bool

	// runCtx bounds background jobs; it is canceled on shutdown so in-flight
	// provider polling stops instead of outliving the process teardown.
	runCtx    context.Context
	runCancel context.CancelFunc

	maxAttempts int
	newBackOff  func() backoff.BackOff

	// retry tracks the in-progress backoff sequence and pending retry timer
	// for each job currently waiting to be re-attempted, keyed by job ID. Both
	// are cleared once the job reaches a terminal state or its timer fires.
	retryMu sync.Mutex
	retry   map[string]*retryState
}

type retryState struct {
	backOff backoff.BackOff
	timer   *time.Timer
}

var _ ports.Queue = (*Pool)(nil)

// Option configures a Pool. Options validate eagerly so misconfiguration fails
// at startup rather than under load.
type Option func(*Pool) error

// WithWorkers sets the number of worker goroutines.
func WithWorkers(n int) Option {
	return func(p *Pool) error {
		if n <= 0 {
			return fmt.Errorf("worker count must be positive, got %d", n)
		}
		p.workers = n
		return nil
	}
}

// WithCapacity sets the bounded depth of the work channel.
func WithCapacity(n int) Option {
	return func(p *Pool) error {
		if n <= 0 {
			return fmt.Errorf("queue capacity must be positive, got %d", n)
		}
		p.capacity = n
		return nil
	}
}

// WithLogger sets the logger used for background job outcomes.
func WithLogger(l *slog.Logger) Option {
	return func(p *Pool) error {
		if l == nil {
			return errors.New("logger must not be nil")
		}
		p.logger = l
		return nil
	}
}

// WithMaxAttempts sets how many times a submitted job's handler may be
// attempted (the initial try plus retries) before it is recorded as failed.
func WithMaxAttempts(n int) Option {
	return func(p *Pool) error {
		if n <= 0 {
			return fmt.Errorf("max attempts must be positive, got %d", n)
		}
		p.maxAttempts = n
		return nil
	}
}

// WithBackOff overrides the backoff sequence used between retries of a failed
// job. f is called once per job, the first time it fails, to build a fresh
// backoff.BackOff whose NextBackOff is then called once per subsequent
// failure of that same job.
func WithBackOff(f func() backoff.BackOff) Option {
	return func(p *Pool) error {
		if f == nil {
			return errors.New("backoff factory must not be nil")
		}
		p.newBackOff = f
		return nil
	}
}

// defaultBackOff returns sane exponential-backoff-with-jitter defaults.
// MaxElapsedTime is left at zero (unbounded): the pool's own maxAttempts
// governs when retries stop, not elapsed wall-clock time.
func defaultBackOff() backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 500 * time.Millisecond
	b.Multiplier = 2
	b.RandomizationFactor = 0.3
	b.MaxInterval = 30 * time.Second
	b.MaxElapsedTime = 0
	return b
}

// New builds a pool. Call Start before submitting work.
func New(jobs ports.JobRepository, clock ports.Clock, opts ...Option) (*Pool, error) {
	p := &Pool{
		jobs:        jobs,
		clock:       clock,
		logger:      slog.Default(),
		workers:     8,
		capacity:    256,
		handlers:    make(map[domain.JobType]ports.JobHandler),
		maxAttempts: defaultMaxAttempts,
		newBackOff:  defaultBackOff,
		retry:       make(map[string]*retryState),
	}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, fmt.Errorf("configuring queue: %w", err)
		}
	}
	p.work = make(chan unit, p.capacity)
	return p, nil
}

// Register binds a job type to the use case that executes it. Call before
// Start; registering an unknown type at run time is a wiring bug.
func (p *Pool) Register(t domain.JobType, h ports.JobHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[t] = h
}

// Start launches the workers. The given context bounds all background job
// execution.
func (p *Pool) Start(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return
	}
	p.started = true
	p.runCtx, p.runCancel = context.WithCancel(ctx)

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for u := range p.work {
		u(p.runCtx)
	}
}

// Shutdown stops accepting work, waits for in-flight units to finish, and
// gives up when ctx expires. Jobs waiting out a retry backoff are left
// pending in the job store rather than force-fired: their NextRetryAt is
// already persisted, so the next process (or a future poller reading
// ports.JobRepository.ListDue) picks them back up.
func (p *Pool) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	close(p.work)
	cancel := p.runCancel
	p.mu.Unlock()

	p.stopPendingRetries()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if cancel != nil {
			cancel()
		}
		return nil
	case <-ctx.Done():
		if cancel != nil {
			cancel() // stop in-flight provider calls, then report the timeout
		}
		return fmt.Errorf("waiting for workers: %w", ctx.Err())
	}
}

// Dispatch runs a fast task on a worker and blocks until it completes, the
// caller's context expires, or the pool shuts down.
func (p *Pool) Dispatch(ctx context.Context, task ports.Task) (json.RawMessage, error) {
	type outcome struct {
		result json.RawMessage
		err    error
	}
	// Buffered so a worker never blocks writing a result the caller abandoned.
	resultCh := make(chan outcome, 1)

	u := func(runCtx context.Context) {
		// The caller is waiting, so its deadline governs; runCtx only signals
		// process shutdown.
		taskCtx, cancel := mergeCancel(ctx, runCtx)
		defer cancel()

		result, err := task(taskCtx)
		resultCh <- outcome{result: result, err: err}
	}

	if err := p.enqueue(ctx, u); err != nil {
		return nil, err
	}

	select {
	case out := <-resultCh:
		return out.result, out.err
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for task result: %w", ctx.Err())
	}
}

// Submit schedules a persisted job for background execution and returns as
// soon as the pool accepts it.
func (p *Pool) Submit(ctx context.Context, job *domain.Job) error {
	p.mu.RLock()
	handler, ok := p.handlers[job.Type]
	p.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no handler registered for job type %q", job.Type)
	}

	return p.enqueue(ctx, func(runCtx context.Context) {
		p.runJob(runCtx, job, handler)
	})
}

// runJob executes a persisted job and records its outcome. Background work
// must not lose state on the way out, so every branch persists. A failed
// attempt is retried with backoff until maxAttempts is reached, at which
// point the job is recorded as terminally failed.
func (p *Pool) runJob(ctx context.Context, job *domain.Job, handler ports.JobHandler) {
	now := p.clock.Now()
	if err := job.MarkRunning(now); err != nil {
		p.logger.Error("job not runnable", "job_id", job.ID, "status", job.Status.String(), "error", err)
		return
	}
	if err := p.jobs.Update(ctx, job); err != nil {
		p.logger.Error("persisting running job", "job_id", job.ID, "error", err)
		return
	}

	result, runErr := handler(ctx, job)
	now = p.clock.Now()

	if runErr == nil {
		p.clearRetry(job.ID)
		if err := job.MarkDone(result, now); err != nil {
			p.logger.Error("marking job done", "job_id", job.ID, "error", err)
			return
		}
		if err := p.jobs.Update(ctx, job); err != nil {
			p.logger.Error("persisting job outcome", "job_id", job.ID, "status", job.Status.String(), "error", err)
		}
		return
	}

	if job.Attempts < p.maxAttempts {
		p.retryJob(job, handler, runErr, now)
		return
	}

	p.clearRetry(job.ID)
	if err := job.MarkFailed(runErr.Error(), now); err != nil {
		p.logger.Error("marking job failed", "job_id", job.ID, "error", err)
		return
	}
	p.logger.Error("job failed, retries exhausted", "job_id", job.ID, "type", string(job.Type), "attempts", job.Attempts, "error", runErr)
	if err := p.jobs.Update(ctx, job); err != nil {
		p.logger.Error("persisting job outcome", "job_id", job.ID, "status", job.Status.String(), "error", err)
	}
}

// retryJob reschedules job for another attempt after a backoff delay. The job
// is persisted as pending immediately so its retry survives a process crash
// during the wait, and a timer re-enqueues it once the delay elapses.
func (p *Pool) retryJob(job *domain.Job, handler ports.JobHandler, runErr error, now time.Time) {
	delay := p.nextDelay(job.ID)
	nextAttemptAt := now.Add(delay)
	if err := job.Reschedule(nextAttemptAt, runErr.Error(), now); err != nil {
		p.logger.Error("rescheduling job for retry", "job_id", job.ID, "error", err)
		return
	}
	if err := p.jobs.Update(context.Background(), job); err != nil {
		p.logger.Error("persisting rescheduled job", "job_id", job.ID, "error", err)
		return
	}
	p.logger.Warn("job failed, retrying", "job_id", job.ID, "type", string(job.Type), "attempts", job.Attempts, "delay", delay, "error", runErr)

	p.scheduleRetry(job, handler, delay)
}

// nextDelay returns the next backoff duration for job, creating a fresh
// backoff sequence on the job's first failure and advancing an existing one
// on subsequent failures.
func (p *Pool) nextDelay(jobID string) time.Duration {
	p.retryMu.Lock()
	defer p.retryMu.Unlock()
	st, ok := p.retry[jobID]
	if !ok {
		st = &retryState{backOff: p.newBackOff()}
		p.retry[jobID] = st
	}
	return st.backOff.NextBackOff()
}

// scheduleRetry arranges for job to be re-enqueued after delay, tracking the
// timer so Shutdown can stop it instead of leaving it to fire after teardown.
func (p *Pool) scheduleRetry(job *domain.Job, handler ports.JobHandler, delay time.Duration) {
	timer := time.AfterFunc(delay, func() {
		if err := p.enqueue(context.Background(), func(runCtx context.Context) {
			p.runJob(runCtx, job, handler)
		}); err != nil {
			p.logger.Error("re-enqueueing retry", "job_id", job.ID, "error", err)
		}
	})

	p.retryMu.Lock()
	st, ok := p.retry[job.ID]
	if !ok {
		st = &retryState{backOff: p.newBackOff()}
		p.retry[job.ID] = st
	}
	st.timer = timer
	p.retryMu.Unlock()
}

// clearRetry drops any backoff sequence and pending timer tracked for jobID.
// Called once the job reaches a terminal state so retry state never leaks.
func (p *Pool) clearRetry(jobID string) {
	p.retryMu.Lock()
	defer p.retryMu.Unlock()
	if st, ok := p.retry[jobID]; ok {
		if st.timer != nil {
			st.timer.Stop()
		}
		delete(p.retry, jobID)
	}
}

// stopPendingRetries cancels every retry timer still waiting to fire. Called
// from Shutdown; the jobs themselves stay pending in the job store.
func (p *Pool) stopPendingRetries() {
	p.retryMu.Lock()
	defer p.retryMu.Unlock()
	for id, st := range p.retry {
		if st.timer != nil {
			st.timer.Stop()
		}
		delete(p.retry, id)
	}
}

// enqueue offers a unit to the workers without ever blocking indefinitely.
//
// The closed check and the channel send happen under the same read lock so
// Shutdown (which takes the write lock before closing the channel) can never
// interleave with a send here — otherwise a send could race a close of
// p.work and panic.
func (p *Pool) enqueue(ctx context.Context, u unit) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	if !p.started {
		return errors.New("queue not started")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("enqueueing work: %w", err)
	}

	// Never block here: a saturated pool is reported to the caller as load,
	// not absorbed by an unbounded wait.
	select {
	case p.work <- u:
		return nil
	default:
		return ErrQueueFull
	}
}

// mergeCancel returns a context canceled when either parent is canceled,
// carrying the values and deadline of primary.
func mergeCancel(primary, secondary context.Context) (context.Context, context.CancelFunc) {
	if secondary == nil {
		return context.WithCancel(primary)
	}
	ctx, cancel := context.WithCancel(primary)
	stop := context.AfterFunc(secondary, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}
