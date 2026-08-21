package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// IssueArvanCloudAccountCertificateInput is the normalized form of an
// issue_arvancloud_account_certificate tool call.
type IssueArvanCloudAccountCertificateInput struct {
	Credentials domain.ProviderCredentials
	Request     domain.ArvanCloudCertificateOrderIssueRequest
}

// IssueArvanCloudAccountCertificateOutput is returned immediately, before the
// certificate is issued.
type IssueArvanCloudAccountCertificateOutput struct {
	OperationID string
	Status      domain.OperationStatus
}

// validateArvanCloudCertificateOrderIssueRequest checks the fields an
// account-level certificate issue call must satisfy, per issue #74's
// explicit scope: PrivateKeySize (when set) against
// domain.ValidArvanCloudCertificatePrivateKeySize, and the required Domains/
// ProductID fields the spec itself marks required — rejected client-side
// rather than sent to the provider only to fail there.
func validateArvanCloudCertificateOrderIssueRequest(req domain.ArvanCloudCertificateOrderIssueRequest) error {
	if len(req.Domains) == 0 {
		return fmt.Errorf("domains is required and must have at least one entry: %w", domain.ErrInvalidInput)
	}
	for _, d := range req.Domains {
		if d.DomainID == "" {
			return fmt.Errorf("each domains entry requires a domain_id: %w", domain.ErrInvalidInput)
		}
		if len(d.DomainNames) == 0 {
			return fmt.Errorf("each domains entry requires at least one domain name: %w", domain.ErrInvalidInput)
		}
	}
	if req.ProductID == "" {
		return fmt.Errorf("product_id is required: %w", domain.ErrInvalidInput)
	}
	if req.PrivateKeySize != 0 && !domain.ValidArvanCloudCertificatePrivateKeySize(req.PrivateKeySize) {
		return fmt.Errorf("private_key_size %d is not one of 2048 or 4096: %w", req.PrivateKeySize, domain.ErrInvalidInput)
	}
	return nil
}

// issueArvanCloudAccountCertificatePayload is what gets persisted for an
// issuance job. Credentials are deliberately absent: they are never written
// to storage (AGENTS.md 4.2) and live only in memory for the lifetime of the
// operation. Also read by GetOperationStatus.reconcile
// (get_operation_status.go), which decodes this same shape for an
// interrupted job of this type.
type issueArvanCloudAccountCertificatePayload struct {
	Request domain.ArvanCloudCertificateOrderIssueRequest `json:"request"`
}

// IssueArvanCloudAccountCertificate is a long operation (AGENTS.md 4.3): it
// starts an account-level Certum certificate order and returns an operation
// ID right away; a worker polls domain.ArvanCloudAccountCertificateOrderStatus
// until it reaches a terminal state (issue #74). It cannot reuse the
// Parspack-only longOp helper (internal/core/app/longop.go), which is typed
// to ports.ParspackProvider — this use case is its own self-contained copy
// of the same machinery, scoped to ports.ArvanCloudProvider instead, and
// distinct from IssueArvanCloudManagedCertificate (issue #73) because the
// two orders have different wire shapes and reconciliation strategies (see
// findLatestArvanCloudAccountCertificateOrder's own doc comment).
type IssueArvanCloudAccountCertificate struct {
	jobs     ports.JobRepository
	queue    ports.Queue
	provider ports.ArvanCloudProvider
	clock    ports.Clock
	ids      ports.IDGenerator

	pollInterval time.Duration
	pollTimeout  time.Duration

	// inflightCreds holds caller credentials for operations currently being
	// executed by this process, exactly like IssueArvanCloudManagedCertificate's
	// own field.
	mu            sync.Mutex
	inflightCreds map[string]domain.ProviderCredentials
}

// IssueArvanCloudAccountCertificateOption configures an
// IssueArvanCloudAccountCertificate. Options validate their input so bad
// configuration fails at construction, not at request time.
type IssueArvanCloudAccountCertificateOption func(*IssueArvanCloudAccountCertificate) error

// WithArvanCloudAccountCertificatePollInterval sets how often the provider
// is polled for order progress.
func WithArvanCloudAccountCertificatePollInterval(d time.Duration) IssueArvanCloudAccountCertificateOption {
	return func(uc *IssueArvanCloudAccountCertificate) error {
		if d <= 0 {
			return fmt.Errorf("poll interval must be positive, got %s", d)
		}
		uc.pollInterval = d
		return nil
	}
}

// WithArvanCloudAccountCertificatePollTimeout caps how long a single
// issuance job may run before it is recorded as failed.
func WithArvanCloudAccountCertificatePollTimeout(d time.Duration) IssueArvanCloudAccountCertificateOption {
	return func(uc *IssueArvanCloudAccountCertificate) error {
		if d <= 0 {
			return fmt.Errorf("poll timeout must be positive, got %s", d)
		}
		uc.pollTimeout = d
		return nil
	}
}

// NewIssueArvanCloudAccountCertificate builds the use case from its ports.
func NewIssueArvanCloudAccountCertificate(
	jobs ports.JobRepository,
	queue ports.Queue,
	provider ports.ArvanCloudProvider,
	clock ports.Clock,
	ids ports.IDGenerator,
	opts ...IssueArvanCloudAccountCertificateOption,
) (*IssueArvanCloudAccountCertificate, error) {
	uc := &IssueArvanCloudAccountCertificate{
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
			return nil, fmt.Errorf("configuring issue arvancloud account certificate use case: %w", err)
		}
	}
	return uc, nil
}

// Execute validates the request, persists a pending job and hands it to the
// queue. It never blocks on the provider.
func (uc *IssueArvanCloudAccountCertificate) Execute(ctx context.Context, in IssueArvanCloudAccountCertificateInput) (IssueArvanCloudAccountCertificateOutput, error) {
	if err := in.Credentials.Validate(); err != nil {
		return IssueArvanCloudAccountCertificateOutput{}, err
	}
	if err := validateArvanCloudCertificateOrderIssueRequest(in.Request); err != nil {
		return IssueArvanCloudAccountCertificateOutput{}, err
	}

	payload, err := json.Marshal(issueArvanCloudAccountCertificatePayload{Request: in.Request})
	if err != nil {
		return IssueArvanCloudAccountCertificateOutput{}, fmt.Errorf("encoding issuance payload: %w", err)
	}

	now := uc.clock.Now()
	job := &domain.Job{
		ID:          uc.ids.NewID(),
		Type:        domain.JobTypeIssueArvanCloudAccountCertificate,
		Payload:     payload,
		Status:      domain.JobStatusPending,
		NextRetryAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := uc.jobs.Create(ctx, job); err != nil {
		return IssueArvanCloudAccountCertificateOutput{}, fmt.Errorf("persisting issuance job: %w", err)
	}

	uc.rememberCredentials(job.ID, in.Credentials)
	if err := uc.queue.Submit(ctx, job); err != nil {
		uc.forgetCredentials(job.ID)
		return IssueArvanCloudAccountCertificateOutput{}, fmt.Errorf("submitting issuance job: %w", err)
	}

	return IssueArvanCloudAccountCertificateOutput{OperationID: job.ID, Status: domain.OperationStatusPending}, nil
}

// Handle executes an issuance job on a worker. It satisfies ports.JobHandler
// and is registered with the queue adapter at wiring time.
func (uc *IssueArvanCloudAccountCertificate) Handle(ctx context.Context, job *domain.Job) (json.RawMessage, error) {
	creds, ok := uc.credentials(job.ID)
	if !ok {
		return nil, fmt.Errorf("credentials for operation %s are no longer held in memory: %w", job.ID, domain.ErrInvalidCredentials)
	}
	// Credentials are deliberately NOT released here: a failed attempt is
	// retried by the queue, and that retry needs them just as much. They are
	// released from Settled, once the job can no longer be re-attempted.

	var payload issueArvanCloudAccountCertificatePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decoding issuance payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, uc.pollTimeout)
	defer cancel()

	order, err := uc.createOrAdopt(ctx, job, creds, payload.Request)
	if err != nil {
		return nil, err
	}

	order, err = uc.waitUntilTerminal(ctx, creds, order)
	if err != nil {
		return nil, err
	}

	result, err := json.Marshal(order)
	if err != nil {
		return nil, fmt.Errorf("encoding issuance result: %w", err)
	}
	return result, nil
}

// createOrAdopt starts a new account certificate order, or adopts one a
// previous attempt already started. A retry must never call
// IssueArvanCloudAccountCertificate a second time while an order is already
// in flight for this request (AGENTS.md 4.4) — see
// findLatestArvanCloudAccountCertificateOrder.
func (uc *IssueArvanCloudAccountCertificate) createOrAdopt(
	ctx context.Context,
	job *domain.Job,
	creds domain.ProviderCredentials,
	req domain.ArvanCloudCertificateOrderIssueRequest,
) (*domain.ArvanCloudAccountCertificateOrder, error) {
	if job.Attempts > 1 || job.WasInterrupted() {
		existing, err := findLatestArvanCloudAccountCertificateOrder(ctx, uc.provider, creds, requestDomainNames(req))
		if err == nil {
			return existing, nil
		}
		if !isNotFound(err) {
			return nil, fmt.Errorf("reconciling account certificate order before retry: %w", err)
		}
	}

	order, err := uc.provider.IssueArvanCloudAccountCertificate(ctx, creds, req)
	if err != nil {
		return nil, fmt.Errorf("issuing arvancloud account certificate: %w", err)
	}
	return order, nil
}

// waitUntilTerminal polls the order by ID until it reaches a terminal state
// (domain.ArvanCloudAccountCertificateOrderTerminal), the deadline passes,
// or the context is canceled. Unlike IssueArvanCloudManagedCertificate's
// waitUntilTerminal, this re-fetches the SAME order by ID
// (GetArvanCloudAccountCertificateOrder) rather than re-resolving "the
// domain's latest order": the account-level API addresses a single order
// directly by ID, and nothing in the spec suggests ArvanCloud silently
// replaces an account-level order the way it does domain-scoped ones.
func (uc *IssueArvanCloudAccountCertificate) waitUntilTerminal(
	ctx context.Context,
	creds domain.ProviderCredentials,
	order *domain.ArvanCloudAccountCertificateOrder,
) (*domain.ArvanCloudAccountCertificateOrder, error) {
	ticker := time.NewTicker(uc.pollInterval)
	defer ticker.Stop()

	for {
		switch order.Status {
		case domain.ArvanCloudAccountCertificateOrderStatusValid:
			return order, nil
		case domain.ArvanCloudAccountCertificateOrderStatusKilled:
			return nil, fmt.Errorf(
				"arvancloud account certificate order %s failed permanently (status killed); "+
					"reissue_arvancloud_account_certificate can attempt a manual reissue", order.ID)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for arvancloud account certificate order %s: %w", order.ID, ctx.Err())
		case <-ticker.C:
		}

		next, err := uc.provider.GetArvanCloudAccountCertificateOrder(ctx, creds, order.ID, nil)
		if err != nil {
			return nil, fmt.Errorf("polling arvancloud account certificate order %s: %w", order.ID, err)
		}
		order = next
	}
}

func (uc *IssueArvanCloudAccountCertificate) rememberCredentials(jobID string, creds domain.ProviderCredentials) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.inflightCreds[jobID] = creds
}

func (uc *IssueArvanCloudAccountCertificate) credentials(jobID string) (domain.ProviderCredentials, bool) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	creds, ok := uc.inflightCreds[jobID]
	return creds, ok
}

// Settled releases the operation's in-memory credentials. It satisfies
// ports.JobSettled and is registered with the queue adapter at wiring time,
// which calls it once the job reaches a terminal state.
func (uc *IssueArvanCloudAccountCertificate) Settled(jobID string) { uc.forgetCredentials(jobID) }

func (uc *IssueArvanCloudAccountCertificate) forgetCredentials(jobID string) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	delete(uc.inflightCreds, jobID)
}

// requestDomainNames flattens every domain name across a
// CertificateOrderIssueRequest's Domains entries into one set (as a sorted
// slice), for matching against an existing order's DomainNames — the
// account-level list endpoint has no request-ID to look up by until an
// order actually exists (see findLatestArvanCloudAccountCertificateOrder).
func requestDomainNames(req domain.ArvanCloudCertificateOrderIssueRequest) []string {
	seen := make(map[string]bool)
	for _, d := range req.Domains {
		for _, name := range d.DomainNames {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// domainNameSetEqual reports whether a and b contain exactly the same
// hostnames, order ignored. Both inputs are expected pre-sorted (as
// requestDomainNames and this function's own callers on order.DomainNames
// produce).
func domainNameSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// findLatestArvanCloudAccountCertificateOrder returns the most recently
// created order among ListArvanCloudAccountCertificateOrders whose
// DomainNames match domainNames exactly (set equality, order ignored), or
// domain.ErrNotFound when nothing matches.
//
// UNLIKE findLatestArvanCloudSslOrder (issue #73), the account-level list
// endpoint has no per-domain filter (ports.ArvanCloudProvider's own doc
// comment) — so reconciliation cannot simply ask "what is domain X's latest
// order" the way the domain-scoped flow does. Matching on the exact
// requested domain-name set is the best available signal this API's real
// affordances offer for "does an order already exist for this request",
// without inventing a domain-scoped filter the API does not have. Shared by
// createOrAdopt and GetOperationStatus.reconcile
// (get_operation_status.go).
func findLatestArvanCloudAccountCertificateOrder(
	ctx context.Context, provider ports.ArvanCloudProvider, creds domain.ProviderCredentials, domainNames []string,
) (*domain.ArvanCloudAccountCertificateOrder, error) {
	orders, err := provider.ListArvanCloudAccountCertificateOrders(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("listing arvancloud account certificate orders: %w", err)
	}

	sortedWant := append([]string(nil), domainNames...)
	sort.Strings(sortedWant)

	var latest *domain.ArvanCloudAccountCertificateOrder
	var latestCreated time.Time
	for i := range orders {
		got := append([]string(nil), orders[i].DomainNames...)
		sort.Strings(got)
		if !domainNameSetEqual(sortedWant, got) {
			continue
		}
		created, _ := time.Parse(time.RFC3339, orders[i].CreatedAt)
		if latest == nil || created.After(latestCreated) {
			latest = &orders[i]
			latestCreated = created
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("no arvancloud account certificate order matching the requested domain names: %w", domain.ErrNotFound)
	}
	return latest, nil
}
