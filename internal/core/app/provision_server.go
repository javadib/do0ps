// Package app holds the application use cases: one type per business
// operation. Use cases orchestrate domain types through ports and know nothing
// about MCP, Fiber, SQL, or any provider's HTTP API.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ProvisionServerInput is the normalized form of a create_server tool call.
type ProvisionServerInput struct {
	Credentials domain.ProviderCredentials
	Spec        domain.ServerSpec
}

// ProvisionServerOutput is returned immediately, before the server exists.
type ProvisionServerOutput struct {
	OperationID string
	Status      domain.OperationStatus
}

// provisionPayload is what gets persisted for a provisioning job.
// Credentials are deliberately absent: they are never written to storage
// (AGENTS.md 4.2) and live only in memory for the lifetime of the operation.
type provisionPayload struct {
	Spec domain.ServerSpec `json:"spec"`
}

// ProvisionServer is a long operation: it returns an operation ID right away
// and finishes the work on a worker goroutine.
type ProvisionServer struct {
	jobs     ports.JobRepository
	queue    ports.Queue
	provider ports.ParspackProvider
	clock    ports.Clock
	ids      ports.IDGenerator

	pollInterval time.Duration
	pollTimeout  time.Duration

	// inflightCreds holds caller credentials for operations currently being
	// executed by this process. It is memory-only and cleared as soon as the
	// job reaches a terminal state; a restart therefore loses it, which is why
	// interrupted jobs are reconciled later with credentials the caller
	// re-supplies (see GetOperationStatus).
	mu            sync.Mutex
	inflightCreds map[string]domain.ProviderCredentials
}

// ProvisionServerOption configures a ProvisionServer. Options validate their
// input so bad configuration fails at construction, not at request time.
type ProvisionServerOption func(*ProvisionServer) error

// WithPollInterval sets how often the provider is polled for progress.
func WithPollInterval(d time.Duration) ProvisionServerOption {
	return func(uc *ProvisionServer) error {
		if d <= 0 {
			return fmt.Errorf("poll interval must be positive, got %s", d)
		}
		uc.pollInterval = d
		return nil
	}
}

// WithPollTimeout caps how long a single provisioning job may run before it is
// recorded as failed.
func WithPollTimeout(d time.Duration) ProvisionServerOption {
	return func(uc *ProvisionServer) error {
		if d <= 0 {
			return fmt.Errorf("poll timeout must be positive, got %s", d)
		}
		uc.pollTimeout = d
		return nil
	}
}

// NewProvisionServer builds the use case from its ports.
func NewProvisionServer(
	jobs ports.JobRepository,
	queue ports.Queue,
	provider ports.ParspackProvider,
	clock ports.Clock,
	ids ports.IDGenerator,
	opts ...ProvisionServerOption,
) (*ProvisionServer, error) {
	uc := &ProvisionServer{
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
		if err := opt(uc); err != nil {
			return nil, fmt.Errorf("configuring provision server use case: %w", err)
		}
	}
	return uc, nil
}

// Execute validates the request, persists a pending job and hands it to the
// queue. It never blocks on the provider.
func (uc *ProvisionServer) Execute(ctx context.Context, in ProvisionServerInput) (ProvisionServerOutput, error) {
	if err := in.Credentials.Validate(); err != nil {
		return ProvisionServerOutput{}, err
	}
	if err := validateServerSpec(in.Spec); err != nil {
		return ProvisionServerOutput{}, err
	}

	payload, err := json.Marshal(provisionPayload{Spec: in.Spec})
	if err != nil {
		return ProvisionServerOutput{}, fmt.Errorf("encoding provisioning payload: %w", err)
	}

	now := uc.clock.Now()
	job := &domain.Job{
		ID:          uc.ids.NewID(),
		Type:        domain.JobTypeProvisionServer,
		Payload:     payload,
		Status:      domain.JobStatusPending,
		NextRetryAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := uc.jobs.Create(ctx, job); err != nil {
		return ProvisionServerOutput{}, fmt.Errorf("persisting provisioning job: %w", err)
	}

	uc.rememberCredentials(job.ID, in.Credentials)
	if err := uc.queue.Submit(ctx, job); err != nil {
		uc.forgetCredentials(job.ID)
		return ProvisionServerOutput{}, fmt.Errorf("submitting provisioning job: %w", err)
	}

	return ProvisionServerOutput{OperationID: job.ID, Status: domain.OperationStatusPending}, nil
}

// Handle executes a provisioning job on a worker. It satisfies
// ports.JobHandler and is registered with the queue adapter at wiring time.
func (uc *ProvisionServer) Handle(ctx context.Context, job *domain.Job) (json.RawMessage, error) {
	creds, ok := uc.credentials(job.ID)
	if !ok {
		return nil, fmt.Errorf("credentials for operation %s are no longer held in memory: %w", job.ID, domain.ErrInvalidCredentials)
	}
	// Credentials are deliberately NOT released here: a failed attempt is
	// retried by the queue, and that retry needs them just as much. They are
	// released from Settled, once the job can no longer be re-attempted.

	var payload provisionPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decoding provisioning payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, uc.pollTimeout)
	defer cancel()

	srv, err := uc.createOrAdopt(ctx, job, creds, payload.Spec)
	if err != nil {
		return nil, err
	}

	srv, err = uc.waitUntilReady(ctx, creds, srv)
	if err != nil {
		return nil, err
	}

	result, err := json.Marshal(srv)
	if err != nil {
		return nil, fmt.Errorf("encoding provisioning result: %w", err)
	}
	return result, nil
}

// createOrAdopt creates the server, or adopts one a previous attempt already
// created. A retry must never duplicate a resource on the provider side.
func (uc *ProvisionServer) createOrAdopt(
	ctx context.Context,
	job *domain.Job,
	creds domain.ProviderCredentials,
	spec domain.ServerSpec,
) (*domain.Server, error) {
	if job.Attempts > 1 || job.WasInterrupted() {
		existing, err := uc.provider.FindServerByName(ctx, creds, spec.Name)
		if err == nil {
			return existing, nil
		}
		if !isNotFound(err) {
			return nil, fmt.Errorf("reconciling server %q before retry: %w", spec.Name, err)
		}
	}

	srv, err := uc.provider.CreateServer(ctx, creds, spec)
	if err != nil {
		return nil, fmt.Errorf("creating server %q: %w", spec.Name, err)
	}
	return srv, nil
}

// waitUntilReady polls the provider until the server leaves the provisioning
// state, the deadline passes, or the context is canceled.
func (uc *ProvisionServer) waitUntilReady(
	ctx context.Context,
	creds domain.ProviderCredentials,
	srv *domain.Server,
) (*domain.Server, error) {
	ticker := time.NewTicker(uc.pollInterval)
	defer ticker.Stop()

	for {
		switch srv.Status {
		case domain.ServerStatusRunning, domain.ServerStatusStopped:
			return srv, nil
		case domain.ServerStatusError:
			return nil, fmt.Errorf("provider reported server %s in error state", srv.ID)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for server %s: %w", srv.ID, ctx.Err())
		case <-ticker.C:
		}

		next, err := uc.provider.GetServer(ctx, creds, srv.ID)
		if err != nil {
			return nil, fmt.Errorf("polling server %s: %w", srv.ID, err)
		}
		srv = next
	}
}

func (uc *ProvisionServer) rememberCredentials(jobID string, creds domain.ProviderCredentials) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.inflightCreds[jobID] = creds
}

func (uc *ProvisionServer) credentials(jobID string) (domain.ProviderCredentials, bool) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	creds, ok := uc.inflightCreds[jobID]
	return creds, ok
}

// Settled releases the operation's in-memory credentials. It satisfies
// ports.JobSettled and is registered with the queue adapter at wiring time,
// which calls it once the job reaches a terminal state.
func (uc *ProvisionServer) Settled(jobID string) { uc.forgetCredentials(jobID) }

func (uc *ProvisionServer) forgetCredentials(jobID string) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	delete(uc.inflightCreds, jobID)
}

func validateServerSpec(spec domain.ServerSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("server name is required: %w", domain.ErrInvalidInput)
	}
	if spec.PlanID == "" && (spec.RAMMB <= 0 || spec.CPUCores <= 0) {
		return fmt.Errorf("either plan_id or cpu_cores plus ram_mb is required: %w", domain.ErrInvalidInput)
	}
	return nil
}
