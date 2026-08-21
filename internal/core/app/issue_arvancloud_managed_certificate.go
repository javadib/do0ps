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

// IssueArvanCloudManagedCertificateInput is the normalized form of an
// issue_arvancloud_managed_certificate tool call.
type IssueArvanCloudManagedCertificateInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

// IssueArvanCloudManagedCertificateOutput is returned immediately, before the
// certificate is issued.
type IssueArvanCloudManagedCertificateOutput struct {
	OperationID string
	Status      domain.OperationStatus
}

// issueArvanCloudManagedCertificatePayload is what gets persisted for an
// issuance job. Credentials are deliberately absent: they are never written
// to storage (AGENTS.md 4.2) and live only in memory for the lifetime of the
// operation. Also read by GetOperationStatus.reconcile
// (get_operation_status.go), which decodes this same shape for an
// interrupted job of this type.
type issueArvanCloudManagedCertificatePayload struct {
	Domain string `json:"domain"`
}

// IssueArvanCloudManagedCertificate is a long operation (AGENTS.md 4.3): it
// starts a managed-certificate order and returns an operation ID right away;
// a worker polls domain.ArvanCloudCertificateOrderStatus until it reaches a
// terminal state (issue #73). It cannot reuse the Parspack-only longOp
// helper (internal/core/app/longop.go), which is typed to
// ports.ParspackProvider — this use case is its own self-contained copy of
// the same machinery, scoped to ports.ArvanCloudProvider instead.
type IssueArvanCloudManagedCertificate struct {
	jobs     ports.JobRepository
	queue    ports.Queue
	provider ports.ArvanCloudProvider
	clock    ports.Clock
	ids      ports.IDGenerator

	pollInterval time.Duration
	pollTimeout  time.Duration

	// inflightCreds holds caller credentials for operations currently being
	// executed by this process, exactly like ProvisionServer's own field.
	mu            sync.Mutex
	inflightCreds map[string]domain.ProviderCredentials
}

// IssueArvanCloudManagedCertificateOption configures an
// IssueArvanCloudManagedCertificate. Options validate their input so bad
// configuration fails at construction, not at request time.
type IssueArvanCloudManagedCertificateOption func(*IssueArvanCloudManagedCertificate) error

// WithArvanCloudSslOrderPollInterval sets how often the provider is polled
// for order progress.
func WithArvanCloudSslOrderPollInterval(d time.Duration) IssueArvanCloudManagedCertificateOption {
	return func(uc *IssueArvanCloudManagedCertificate) error {
		if d <= 0 {
			return fmt.Errorf("poll interval must be positive, got %s", d)
		}
		uc.pollInterval = d
		return nil
	}
}

// WithArvanCloudSslOrderPollTimeout caps how long a single issuance job may
// run before it is recorded as failed.
func WithArvanCloudSslOrderPollTimeout(d time.Duration) IssueArvanCloudManagedCertificateOption {
	return func(uc *IssueArvanCloudManagedCertificate) error {
		if d <= 0 {
			return fmt.Errorf("poll timeout must be positive, got %s", d)
		}
		uc.pollTimeout = d
		return nil
	}
}

// NewIssueArvanCloudManagedCertificate builds the use case from its ports.
func NewIssueArvanCloudManagedCertificate(
	jobs ports.JobRepository,
	queue ports.Queue,
	provider ports.ArvanCloudProvider,
	clock ports.Clock,
	ids ports.IDGenerator,
	opts ...IssueArvanCloudManagedCertificateOption,
) (*IssueArvanCloudManagedCertificate, error) {
	uc := &IssueArvanCloudManagedCertificate{
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
			return nil, fmt.Errorf("configuring issue arvancloud managed certificate use case: %w", err)
		}
	}
	return uc, nil
}

// Execute validates the request, persists a pending job and hands it to the
// queue. It never blocks on the provider.
func (uc *IssueArvanCloudManagedCertificate) Execute(ctx context.Context, in IssueArvanCloudManagedCertificateInput) (IssueArvanCloudManagedCertificateOutput, error) {
	if err := in.Credentials.Validate(); err != nil {
		return IssueArvanCloudManagedCertificateOutput{}, err
	}
	if in.Domain == "" {
		return IssueArvanCloudManagedCertificateOutput{}, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}

	payload, err := json.Marshal(issueArvanCloudManagedCertificatePayload{Domain: in.Domain})
	if err != nil {
		return IssueArvanCloudManagedCertificateOutput{}, fmt.Errorf("encoding issuance payload: %w", err)
	}

	now := uc.clock.Now()
	job := &domain.Job{
		ID:          uc.ids.NewID(),
		Type:        domain.JobTypeIssueArvanCloudManagedCertificate,
		Payload:     payload,
		Status:      domain.JobStatusPending,
		NextRetryAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := uc.jobs.Create(ctx, job); err != nil {
		return IssueArvanCloudManagedCertificateOutput{}, fmt.Errorf("persisting issuance job: %w", err)
	}

	uc.rememberCredentials(job.ID, in.Credentials)
	if err := uc.queue.Submit(ctx, job); err != nil {
		uc.forgetCredentials(job.ID)
		return IssueArvanCloudManagedCertificateOutput{}, fmt.Errorf("submitting issuance job: %w", err)
	}

	return IssueArvanCloudManagedCertificateOutput{OperationID: job.ID, Status: domain.OperationStatusPending}, nil
}

// Handle executes an issuance job on a worker. It satisfies ports.JobHandler
// and is registered with the queue adapter at wiring time.
func (uc *IssueArvanCloudManagedCertificate) Handle(ctx context.Context, job *domain.Job) (json.RawMessage, error) {
	creds, ok := uc.credentials(job.ID)
	if !ok {
		return nil, fmt.Errorf("credentials for operation %s are no longer held in memory: %w", job.ID, domain.ErrInvalidCredentials)
	}
	// Credentials are deliberately NOT released here: a failed attempt is
	// retried by the queue, and that retry needs them just as much. They are
	// released from Settled, once the job can no longer be re-attempted.

	var payload issueArvanCloudManagedCertificatePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decoding issuance payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, uc.pollTimeout)
	defer cancel()

	order, err := uc.createOrAdopt(ctx, job, creds, payload.Domain)
	if err != nil {
		return nil, err
	}

	order, err = uc.waitUntilTerminal(ctx, creds, payload.Domain, order)
	if err != nil {
		return nil, err
	}

	result, err := json.Marshal(order)
	if err != nil {
		return nil, fmt.Errorf("encoding issuance result: %w", err)
	}
	return result, nil
}

// createOrAdopt starts a new managed-certificate order, or adopts one a
// previous attempt already started. A retry must never call
// IssueArvanCloudManagedCertificate a second time while an order is already
// in flight for the domain (AGENTS.md 4.4) — see findLatestArvanCloudSslOrder.
func (uc *IssueArvanCloudManagedCertificate) createOrAdopt(
	ctx context.Context,
	job *domain.Job,
	creds domain.ProviderCredentials,
	domainName string,
) (*domain.ArvanCloudCertificateOrder, error) {
	if job.Attempts > 1 || job.WasInterrupted() {
		existing, err := findLatestArvanCloudSslOrder(ctx, uc.provider, creds, domainName)
		if err == nil {
			return existing, nil
		}
		if !isNotFound(err) {
			return nil, fmt.Errorf("reconciling ssl order for domain %q before retry: %w", domainName, err)
		}
	}

	order, err := uc.provider.IssueArvanCloudManagedCertificate(ctx, creds, domainName)
	if err != nil {
		return nil, fmt.Errorf("issuing arvancloud managed certificate for domain %q: %w", domainName, err)
	}
	return order, nil
}

// waitUntilTerminal polls the order until it reaches a terminal state
// (domain.ArvanCloudCertificateOrderTerminal), the deadline passes, or the
// context is canceled. ArvanCloud automatically replaces an
// invalid/terminated/canceled order with a new one (per the spec's own
// description on ArvanCloudCertificateOrderStatus), so each poll re-resolves
// the domain's LATEST order rather than re-fetching the same order ID.
func (uc *IssueArvanCloudManagedCertificate) waitUntilTerminal(
	ctx context.Context,
	creds domain.ProviderCredentials,
	domainName string,
	order *domain.ArvanCloudCertificateOrder,
) (*domain.ArvanCloudCertificateOrder, error) {
	ticker := time.NewTicker(uc.pollInterval)
	defer ticker.Stop()

	for {
		switch order.Status {
		case domain.ArvanCloudCertificateOrderStatusValid:
			return order, nil
		case domain.ArvanCloudCertificateOrderStatusKilled:
			return nil, fmt.Errorf(
				"arvancloud managed certificate order %s for domain %q failed permanently (status killed); "+
					"retry_arvancloud_ssl_order can attempt a manual retry", order.ID, domainName)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for arvancloud managed certificate order %s for domain %q: %w", order.ID, domainName, ctx.Err())
		case <-ticker.C:
		}

		next, err := findLatestArvanCloudSslOrder(ctx, uc.provider, creds, domainName)
		if err != nil {
			return nil, fmt.Errorf("polling arvancloud ssl orders for domain %q: %w", domainName, err)
		}
		order = next
	}
}

func (uc *IssueArvanCloudManagedCertificate) rememberCredentials(jobID string, creds domain.ProviderCredentials) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.inflightCreds[jobID] = creds
}

func (uc *IssueArvanCloudManagedCertificate) credentials(jobID string) (domain.ProviderCredentials, bool) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	creds, ok := uc.inflightCreds[jobID]
	return creds, ok
}

// Settled releases the operation's in-memory credentials. It satisfies
// ports.JobSettled and is registered with the queue adapter at wiring time,
// which calls it once the job reaches a terminal state.
func (uc *IssueArvanCloudManagedCertificate) Settled(jobID string) { uc.forgetCredentials(jobID) }

func (uc *IssueArvanCloudManagedCertificate) forgetCredentials(jobID string) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	delete(uc.inflightCreds, jobID)
}

// findLatestArvanCloudSslOrder returns the most recently created order
// ListArvanCloudSslOrders reports for domainName, or domain.ErrNotFound when
// the domain has no order at all. Shared by createOrAdopt, waitUntilTerminal
// and GetOperationStatus.reconcile (get_operation_status.go) — every place
// that must resolve "the order this issuance is about" without an order ID
// of its own to look up, since ArvanCloud can silently replace an order
// (invalid/terminated/canceled statuses) with a new one bearing a new ID.
func findLatestArvanCloudSslOrder(
	ctx context.Context, provider ports.ArvanCloudProvider, creds domain.ProviderCredentials, domainName string,
) (*domain.ArvanCloudCertificateOrder, error) {
	orders, err := provider.ListArvanCloudSslOrders(ctx, creds, domainName)
	if err != nil {
		return nil, fmt.Errorf("listing arvancloud ssl orders for domain %q: %w", domainName, err)
	}
	if len(orders) == 0 {
		return nil, fmt.Errorf("no arvancloud ssl order for domain %q: %w", domainName, domain.ErrNotFound)
	}

	latest := &orders[0]
	latestCreated, _ := time.Parse(time.RFC3339, latest.CreatedAt)
	for i := 1; i < len(orders); i++ {
		created, err := time.Parse(time.RFC3339, orders[i].CreatedAt)
		if err != nil {
			continue
		}
		if created.After(latestCreated) {
			latest = &orders[i]
			latestCreated = created
		}
	}
	return latest, nil
}
