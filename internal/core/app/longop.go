package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// longOp is the shared scaffolding for long operations that start an async
// provider action and poll it to completion: CreateSnapshot and RestoreVM.
// It owns the memory-only credentials map (AGENTS.md 4.2) and the polling
// knobs those use cases need. ProvisionServer predates this helper and keeps
// its own copy of the same machinery.
type longOp struct {
	jobs     ports.JobRepository
	queue    ports.Queue
	provider ports.ParspackProvider
	clock    ports.Clock
	ids      ports.IDGenerator

	pollInterval time.Duration
	pollTimeout  time.Duration

	// inflightCreds holds caller credentials for operations currently being
	// executed by this process. Memory-only, cleared when a job reaches a
	// terminal state; a restart loses it, which is why interrupted jobs are
	// reconciled later with credentials the caller re-supplies.
	mu            sync.Mutex
	inflightCreds map[string]domain.ProviderCredentials
}

// longOpOption configures a longOp. Options validate eagerly so bad
// configuration fails at construction.
type longOpOption func(*longOp) error

// WithActionPollInterval sets how often the provider is polled for progress.
func WithActionPollInterval(d time.Duration) longOpOption {
	return func(l *longOp) error {
		if d <= 0 {
			return fmt.Errorf("poll interval must be positive, got %s", d)
		}
		l.pollInterval = d
		return nil
	}
}

// WithActionPollTimeout caps how long a single operation may run before it is
// recorded as failed.
func WithActionPollTimeout(d time.Duration) longOpOption {
	return func(l *longOp) error {
		if d <= 0 {
			return fmt.Errorf("poll timeout must be positive, got %s", d)
		}
		l.pollTimeout = d
		return nil
	}
}

func newLongOp(
	jobs ports.JobRepository,
	queue ports.Queue,
	provider ports.ParspackProvider,
	clock ports.Clock,
	ids ports.IDGenerator,
	opts ...longOpOption,
) (*longOp, error) {
	l := &longOp{
		jobs:          jobs,
		queue:         queue,
		provider:      provider,
		clock:         clock,
		ids:           ids,
		pollInterval:  10 * time.Second,
		pollTimeout:   20 * time.Minute,
		inflightCreds: make(map[string]domain.ProviderCredentials),
	}
	for _, opt := range opts {
		if err := opt(l); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// start persists a pending job and hands it to the queue, returning the
// operation ID callers poll with. It never blocks on the provider.
func (l *longOp) start(ctx context.Context, jobType domain.JobType, payload []byte, creds domain.ProviderCredentials) (string, error) {
	now := l.clock.Now()
	job := &domain.Job{
		ID:          l.ids.NewID(),
		Type:        jobType,
		Payload:     payload,
		Status:      domain.JobStatusPending,
		NextRetryAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := l.jobs.Create(ctx, job); err != nil {
		return "", fmt.Errorf("persisting %s job: %w", jobType, err)
	}

	l.rememberCredentials(job.ID, creds)
	if err := l.queue.Submit(ctx, job); err != nil {
		l.forgetCredentials(job.ID)
		return "", fmt.Errorf("submitting %s job: %w", jobType, err)
	}
	return job.ID, nil
}

// waitAction polls the action until it completes or the context expires.
func (l *longOp) waitAction(ctx context.Context, creds domain.ProviderCredentials, action *domain.VMAction) (*domain.VMAction, error) {
	ticker := time.NewTicker(l.pollInterval)
	defer ticker.Stop()

	for {
		switch action.Status {
		case domain.VMActionStatusCompleted:
			return action, nil
		case domain.VMActionStatusInProgress:
		default:
			return nil, fmt.Errorf("provider reported %s action %s on server %s in state %q",
				action.Type, action.ID, action.ServerID, action.Status)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for %s action %s: %w", action.Type, action.ID, ctx.Err())
		case <-ticker.C:
		}

		next, err := l.provider.GetVMAction(ctx, creds, action.ServerID, action.ID)
		if err != nil {
			return nil, fmt.Errorf("polling %s action %s: %w", action.Type, action.ID, err)
		}
		action = next
	}
}

// waitSnapshotVisible polls the snapshot list until the snapshot for serverID
// and name appears. Snapshot creation finishes behind an async action; the
// snapshot object is only listed once that action completes.
func (l *longOp) waitSnapshotVisible(ctx context.Context, creds domain.ProviderCredentials, serverID, name string) (*domain.VMSnapshot, error) {
	ticker := time.NewTicker(l.pollInterval)
	defer ticker.Stop()

	for {
		snap, err := l.findSnapshot(ctx, creds, serverID, name)
		if err == nil {
			return snap, nil
		}
		if !isNotFound(err) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for snapshot %q of server %s: %w", name, serverID, ctx.Err())
		case <-ticker.C:
		}
	}
}

// findSnapshot returns the snapshot matching serverID and name, or
// domain.ErrNotFound when there is none. Matching by name and server is how a
// retry recognizes a snapshot a previous attempt already created.
func (l *longOp) findSnapshot(ctx context.Context, creds domain.ProviderCredentials, serverID, name string) (*domain.VMSnapshot, error) {
	snapshots, err := l.provider.ListVMSnapshots(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("listing VM snapshots: %w", err)
	}
	return findSnapshotByServerAndName(snapshots, serverID, name)
}

func (l *longOp) rememberCredentials(jobID string, creds domain.ProviderCredentials) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inflightCreds[jobID] = creds
}

func (l *longOp) credentials(jobID string) (domain.ProviderCredentials, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	creds, ok := l.inflightCreds[jobID]
	return creds, ok
}

func (l *longOp) forgetCredentials(jobID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.inflightCreds, jobID)
}
