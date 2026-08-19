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

// ProvisionLoadBalancerInput is the normalized form of a create_load_balancer
// tool call.
type ProvisionLoadBalancerInput struct {
	Credentials  domain.ProviderCredentials
	LoadBalancer domain.LoadBalancer
}

// ProvisionLoadBalancerOutput is returned immediately, before the load
// balancer exists.
type ProvisionLoadBalancerOutput struct {
	OperationID string
	Status      domain.OperationStatus
}

// provisionLoadBalancerPayload is what gets persisted for a provisioning job.
// Credentials are deliberately absent: they are never written to storage
// (AGENTS.md 4.2) and live only in memory for the lifetime of the operation.
type provisionLoadBalancerPayload struct {
	LoadBalancer domain.LoadBalancer `json:"load_balancer"`
}

// ProvisionLoadBalancer is a long operation: creating a load balancer returns
// it in "new" status and provisioning completes on a worker, which polls until
// the balancer turns "active" (confirmed against terraform-provider-abrha,
// which waits for the same transition). It returns an operation ID right away
// and finishes the work on a worker goroutine.
type ProvisionLoadBalancer struct {
	jobs     ports.JobRepository
	queue    ports.Queue
	provider ports.ParspackProvider
	clock    ports.Clock
	ids      ports.IDGenerator

	pollInterval time.Duration
	pollTimeout  time.Duration

	// inflightCreds holds caller credentials for operations currently being
	// executed by this process, exactly like ProvisionServer.
	mu            sync.Mutex
	inflightCreds map[string]domain.ProviderCredentials
}

// ProvisionLoadBalancerOption configures a ProvisionLoadBalancer.
type ProvisionLoadBalancerOption func(*ProvisionLoadBalancer) error

// WithLoadBalancerPollInterval sets how often the provider is polled for
// progress.
func WithLoadBalancerPollInterval(d time.Duration) ProvisionLoadBalancerOption {
	return func(uc *ProvisionLoadBalancer) error {
		if d <= 0 {
			return fmt.Errorf("poll interval must be positive, got %s", d)
		}
		uc.pollInterval = d
		return nil
	}
}

// WithLoadBalancerPollTimeout caps how long a single provisioning job may run
// before it is recorded as failed.
func WithLoadBalancerPollTimeout(d time.Duration) ProvisionLoadBalancerOption {
	return func(uc *ProvisionLoadBalancer) error {
		if d <= 0 {
			return fmt.Errorf("poll timeout must be positive, got %s", d)
		}
		uc.pollTimeout = d
		return nil
	}
}

// NewProvisionLoadBalancer builds the use case from its ports.
func NewProvisionLoadBalancer(
	jobs ports.JobRepository,
	queue ports.Queue,
	provider ports.ParspackProvider,
	clock ports.Clock,
	ids ports.IDGenerator,
	opts ...ProvisionLoadBalancerOption,
) (*ProvisionLoadBalancer, error) {
	uc := &ProvisionLoadBalancer{
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
			return nil, fmt.Errorf("configuring provision load balancer use case: %w", err)
		}
	}
	return uc, nil
}

// Execute validates the request, persists a pending job and hands it to the
// queue. It never blocks on the provider.
func (uc *ProvisionLoadBalancer) Execute(ctx context.Context, in ProvisionLoadBalancerInput) (ProvisionLoadBalancerOutput, error) {
	if err := in.Credentials.Validate(); err != nil {
		return ProvisionLoadBalancerOutput{}, err
	}
	if err := validateLoadBalancer(in.LoadBalancer); err != nil {
		return ProvisionLoadBalancerOutput{}, err
	}

	payload, err := json.Marshal(provisionLoadBalancerPayload{LoadBalancer: in.LoadBalancer})
	if err != nil {
		return ProvisionLoadBalancerOutput{}, fmt.Errorf("encoding provisioning payload: %w", err)
	}

	now := uc.clock.Now()
	job := &domain.Job{
		ID:          uc.ids.NewID(),
		Type:        domain.JobTypeProvisionLoadBalancer,
		Payload:     payload,
		Status:      domain.JobStatusPending,
		NextRetryAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := uc.jobs.Create(ctx, job); err != nil {
		return ProvisionLoadBalancerOutput{}, fmt.Errorf("persisting provisioning job: %w", err)
	}

	uc.rememberCredentials(job.ID, in.Credentials)
	if err := uc.queue.Submit(ctx, job); err != nil {
		uc.forgetCredentials(job.ID)
		return ProvisionLoadBalancerOutput{}, fmt.Errorf("submitting provisioning job: %w", err)
	}

	return ProvisionLoadBalancerOutput{OperationID: job.ID, Status: domain.OperationStatusPending}, nil
}

// Handle executes a provisioning job on a worker. It satisfies
// ports.JobHandler and is registered with the queue adapter at wiring time.
func (uc *ProvisionLoadBalancer) Handle(ctx context.Context, job *domain.Job) (json.RawMessage, error) {
	creds, ok := uc.credentials(job.ID)
	if !ok {
		return nil, fmt.Errorf("credentials for operation %s are no longer held in memory: %w", job.ID, domain.ErrInvalidCredentials)
	}
	// Credentials are deliberately NOT released here: a failed attempt is
	// retried by the queue, and that retry needs them just as much. They are
	// released from Settled, once the job can no longer be re-attempted.

	var payload provisionLoadBalancerPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decoding provisioning payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, uc.pollTimeout)
	defer cancel()

	lb, err := uc.createOrAdopt(ctx, job, creds, payload.LoadBalancer)
	if err != nil {
		return nil, err
	}

	lb, err = uc.waitUntilActive(ctx, creds, lb)
	if err != nil {
		return nil, err
	}

	result, err := json.Marshal(lb)
	if err != nil {
		return nil, fmt.Errorf("encoding provisioning result: %w", err)
	}
	return result, nil
}

// createOrAdopt creates the load balancer, or adopts one a previous attempt
// already created. A retry must never duplicate a resource on the provider
// side.
func (uc *ProvisionLoadBalancer) createOrAdopt(
	ctx context.Context,
	job *domain.Job,
	creds domain.ProviderCredentials,
	lb domain.LoadBalancer,
) (*domain.LoadBalancer, error) {
	if job.Attempts > 1 || job.WasInterrupted() {
		existing, err := uc.provider.FindLoadBalancerByName(ctx, creds, lb.Name)
		if err == nil {
			return existing, nil
		}
		if !isNotFound(err) {
			return nil, fmt.Errorf("reconciling load balancer %q before retry: %w", lb.Name, err)
		}
	}

	created, err := uc.provider.CreateLoadBalancer(ctx, creds, lb)
	if err != nil {
		return nil, fmt.Errorf("creating load balancer %q: %w", lb.Name, err)
	}
	return created, nil
}

// waitUntilActive polls the provider until the load balancer reaches the
// "active" status (the provisioning state is "new"), the deadline passes, or
// the context is canceled. These are the status values terraform-provider-abrha
// waits on for load balancers; anything else keeps polling.
func (uc *ProvisionLoadBalancer) waitUntilActive(
	ctx context.Context,
	creds domain.ProviderCredentials,
	lb *domain.LoadBalancer,
) (*domain.LoadBalancer, error) {
	ticker := time.NewTicker(uc.pollInterval)
	defer ticker.Stop()

	for {
		switch lb.Status {
		case "active":
			return lb, nil
		case "errored":
			return nil, fmt.Errorf("provider reported load balancer %s in error state", lb.ID)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for load balancer %s: %w", lb.ID, ctx.Err())
		case <-ticker.C:
		}

		next, err := uc.provider.GetLoadBalancer(ctx, creds, lb.ID)
		if err != nil {
			return nil, fmt.Errorf("polling load balancer %s: %w", lb.ID, err)
		}
		lb = next
	}
}

func (uc *ProvisionLoadBalancer) rememberCredentials(jobID string, creds domain.ProviderCredentials) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.inflightCreds[jobID] = creds
}

func (uc *ProvisionLoadBalancer) credentials(jobID string) (domain.ProviderCredentials, bool) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	creds, ok := uc.inflightCreds[jobID]
	return creds, ok
}

// Settled releases the operation's in-memory credentials. It satisfies
// ports.JobSettled and is registered with the queue adapter at wiring time,
// which calls it once the job reaches a terminal state.
func (uc *ProvisionLoadBalancer) Settled(jobID string) { uc.forgetCredentials(jobID) }

func (uc *ProvisionLoadBalancer) forgetCredentials(jobID string) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	delete(uc.inflightCreds, jobID)
}

func validateLoadBalancer(lb domain.LoadBalancer) error {
	if lb.Name == "" {
		return fmt.Errorf("load balancer name is required: %w", domain.ErrInvalidInput)
	}
	if len(lb.ForwardingRules) == 0 {
		return fmt.Errorf("at least one forwarding rule is required: %w", domain.ErrInvalidInput)
	}
	return nil
}
