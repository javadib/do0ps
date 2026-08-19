package queue

import (
	"context"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
)

// sweep periodically re-reads the job store for pending work the pool is no
// longer tracking, and re-enqueues it.
//
// The in-process retry timer covers the normal path, so this exists for the
// one case it cannot: a job the pool persisted as pending and then dropped
// because re-enqueueing hit a full queue (see scheduleRetry). Its timer is
// gone, and nothing else would ever look at it again.
//
// It deliberately does NOT resurrect jobs left behind by a previous process.
// Those carry no credentials — they are memory-only and die with the process —
// so re-running them would fail on authentication, burn the whole retry budget
// and end in a terminal failure. That would destroy the reconciliation path in
// GetOperationStatus, which is what tells the caller whether the provider
// actually created the resource (AGENTS.md 4.4). Startup marks them
// interrupted, and interrupted jobs are skipped here.
func (p *Pool) sweep(ctx context.Context) {
	ticker := time.NewTicker(p.sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.sweepOnce(ctx)
		}
	}
}

// sweepOnce re-enqueues every due job the pool has dropped. Errors are logged
// rather than returned: a sweep that fails is retried on the next tick.
func (p *Pool) sweepOnce(ctx context.Context) {
	due, err := p.jobs.ListDue(ctx, p.clock.Now(), p.sweepLimit)
	if err != nil {
		p.logger.Error("sweeping for due jobs", "error", err)
		return
	}

	for _, job := range due {
		if !p.shouldSweep(job) {
			continue
		}

		p.mu.RLock()
		handler, ok := p.handlers[job.Type]
		p.mu.RUnlock()
		if !ok {
			// A job type this build does not know how to run. Leave it alone
			// rather than failing it: a later build may well handle it.
			continue
		}

		if !p.tryClaim(job.ID) {
			// Something took it between the query and here.
			continue
		}

		// Read what we want to log before handing the job over: from the
		// enqueue on, a worker owns the struct and mutates it.
		id, jobType, attempts := job.ID, string(job.Type), job.Attempts

		if err := p.enqueue(ctx, func(runCtx context.Context) {
			p.runJob(runCtx, job, handler)
		}); err != nil {
			// Still saturated. Give the claim back and try again next tick.
			p.release(id)
			p.logger.Warn("re-enqueueing swept job", "job_id", id, "error", err)
			return
		}
		p.logger.Info("recovered dropped job", "job_id", id, "type", jobType, "attempts", attempts)
	}
}

// shouldSweep reports whether a due job is worth attempting at all. Whether
// the pool is already carrying it is decided by tryClaim, not here, so the
// check and the claim cannot drift apart.
func (p *Pool) shouldSweep(job *domain.Job) bool {
	switch {
	case job.WasInterrupted():
		// Awaiting reconciliation with credentials only the caller can supply.
		return false
	case job.Attempts >= p.maxAttempts:
		// Out of retry budget; running it again would only fail it.
		return false
	default:
		return true
	}
}
