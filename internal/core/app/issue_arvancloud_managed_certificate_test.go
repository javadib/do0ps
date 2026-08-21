package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// fakeArvanCloudSSLProvider embeds the port so a test only needs to override
// the methods it actually exercises (the same pattern as fakeArvanCloudProvider
// in arvancloud_domain_test.go, for ports.ArvanCloudProvider).
type fakeArvanCloudSSLProvider struct {
	ports.ArvanCloudProvider

	issueCalls int
	issueErr   error
	issued     domain.ArvanCloudCertificateOrder

	orders    []domain.ArvanCloudCertificateOrder
	ordersErr error

	settings    domain.ArvanCloudSslSettings
	settingsErr error
	updatedWith domain.ArvanCloudSslSettings

	certificates    []domain.ArvanCloudCertificate
	certificatesErr error

	uploadedCertificate []byte
	uploadedPrivateKey  []byte
	uploadErr           error

	deletedCertificateID string
	deleteCertErr        error

	revokedCertificateID string
	revokeErr            error

	retryCalledDomain string
	retryErr          error
}

func (p *fakeArvanCloudSSLProvider) IssueArvanCloudManagedCertificate(_ context.Context, _ domain.ProviderCredentials, domainName string) (*domain.ArvanCloudCertificateOrder, error) {
	p.issueCalls++
	if p.issueErr != nil {
		return nil, p.issueErr
	}
	order := p.issued
	if order.DomainNames == nil {
		order.DomainNames = []string{domainName}
	}
	return &order, nil
}

func (p *fakeArvanCloudSSLProvider) ListArvanCloudSslOrders(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudCertificateOrder, error) {
	if p.ordersErr != nil {
		return nil, p.ordersErr
	}
	return p.orders, nil
}

// TestIssueArvanCloudManagedCertificateReturnsPendingOperation proves the
// tool returns a pending job/operation immediately rather than blocking on
// the provider — issue #73's acceptance criterion, mirroring
// TestProvisionServerReturnsPendingOperation (app_test.go).
func TestIssueArvanCloudManagedCertificateReturnsPendingOperation(t *testing.T) {
	jobs := newMemJobs()
	queue := &inlineQueue{}
	clock := fixedClock{t: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)}
	provider := &fakeArvanCloudSSLProvider{}

	uc, err := app.NewIssueArvanCloudManagedCertificate(jobs, queue, provider, clock, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("NewIssueArvanCloudManagedCertificate: %v", err)
	}

	out, err := uc.Execute(context.Background(), app.IssueArvanCloudManagedCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "arvan-key"},
		Domain:      "example.com",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if out.OperationID != "op-1" {
		t.Errorf("operation ID = %q, want %q", out.OperationID, "op-1")
	}
	if out.Status != domain.OperationStatusPending {
		t.Errorf("status = %s, want pending", out.Status)
	}
	// The provider is never called synchronously: Execute only persists and
	// submits the job — Submit (inlineQueue.Submit) just records it, it does
	// not run Handle. This is the blocking-vs-not distinction the acceptance
	// criterion is about.
	if provider.issueCalls != 0 {
		t.Errorf("IssueArvanCloudManagedCertificate called %d times during Execute, want 0 (it must run on a worker, not inline)", provider.issueCalls)
	}
	if len(queue.submitted) != 1 {
		t.Fatalf("submitted %d jobs, want 1", len(queue.submitted))
	}

	stored, err := jobs.Get(context.Background(), "op-1")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	if stored.Status != domain.JobStatusPending {
		t.Errorf("stored status = %s, want pending", stored.Status)
	}
	if stored.Type != domain.JobTypeIssueArvanCloudManagedCertificate {
		t.Errorf("stored job type = %s, want %s", stored.Type, domain.JobTypeIssueArvanCloudManagedCertificate)
	}
	// Provider credentials must never reach storage (AGENTS.md 4.2).
	if payload := string(stored.Payload); strings.Contains(payload, "arvan-key") || strings.Contains(payload, "APIKey") {
		t.Errorf("payload appears to contain credentials: %s", payload)
	}
}

// TestIssueArvanCloudManagedCertificateRejectsMissingDomain proves the use
// case validates its input before ever touching the queue or the provider.
func TestIssueArvanCloudManagedCertificateRejectsMissingDomain(t *testing.T) {
	uc, err := app.NewIssueArvanCloudManagedCertificate(newMemJobs(), &inlineQueue{}, &fakeArvanCloudSSLProvider{}, fixedClock{}, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("NewIssueArvanCloudManagedCertificate: %v", err)
	}

	_, err = uc.Execute(context.Background(), app.IssueArvanCloudManagedCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want a validation error for the missing domain")
	}
}

// TestIssueArvanCloudManagedCertificateHandleAdoptsInFlightOrderOnRetry
// proves that within the SAME process, a retried Handle call (job.Attempts >
// 1) never calls IssueArvanCloudManagedCertificate a second time when the
// provider already reports an order for the domain — the in-process half of
// AGENTS.md 4.4's "never call ssl.cert.issue a second time" rule. The
// cross-process half (a job found "running" at startup) is proven by
// TestGetOperationStatusReconcilesArvanCloudSslOrder below.
func TestIssueArvanCloudManagedCertificateHandleAdoptsInFlightOrderOnRetry(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()
	queue := &inlineQueue{}

	provider := &fakeArvanCloudSSLProvider{
		orders: []domain.ArvanCloudCertificateOrder{
			{ID: "order-1", Status: domain.ArvanCloudCertificateOrderStatusValid, DomainNames: []string{"example.com"}, CreatedAt: now.Format(time.RFC3339)},
		},
	}

	uc, err := app.NewIssueArvanCloudManagedCertificate(jobs, queue, provider, fixedClock{t: now}, fixedIDs{id: "op-1"},
		app.WithArvanCloudSslOrderPollInterval(time.Millisecond),
		app.WithArvanCloudSslOrderPollTimeout(time.Second),
	)
	if err != nil {
		t.Fatalf("NewIssueArvanCloudManagedCertificate: %v", err)
	}

	if _, err := uc.Execute(context.Background(), app.IssueArvanCloudManagedCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Domain:      "example.com",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	job, err := jobs.Get(context.Background(), "op-1")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	// Simulate a retry: the queue's MarkRunning already bumped Attempts past
	// 1 for a second execution of this job.
	job.Attempts = 2

	result, err := uc.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if provider.issueCalls != 0 {
		t.Errorf("IssueArvanCloudManagedCertificate called %d times on retry, want 0 (must adopt the existing order)", provider.issueCalls)
	}

	var order domain.ArvanCloudCertificateOrder
	if err := json.Unmarshal(result, &order); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if order.ID != "order-1" || order.Status != domain.ArvanCloudCertificateOrderStatusValid {
		t.Errorf("order = %+v, want the adopted valid order-1", order)
	}
}

// TestFindLatestArvanCloudSslOrderNotFound proves an empty order list is
// reported as domain.ErrNotFound, the signal createOrAdopt uses to decide it
// is safe to place a brand new order.
func TestFindLatestArvanCloudSslOrderNotFound(t *testing.T) {
	jobs := newMemJobs()
	queue := &inlineQueue{}
	provider := &fakeArvanCloudSSLProvider{
		issued: domain.ArvanCloudCertificateOrder{ID: "order-1", Status: domain.ArvanCloudCertificateOrderStatusValid},
	}

	uc, err := app.NewIssueArvanCloudManagedCertificate(jobs, queue, provider, fixedClock{t: time.Now()}, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("NewIssueArvanCloudManagedCertificate: %v", err)
	}

	if _, err := uc.Execute(context.Background(), app.IssueArvanCloudManagedCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Domain:      "example.com",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	job, err := jobs.Get(context.Background(), "op-1")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	job.Attempts = 2 // force the retry/adopt path; provider has no orders yet

	result, err := uc.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if provider.issueCalls != 1 {
		t.Errorf("IssueArvanCloudManagedCertificate called %d times, want 1 (no existing order to adopt)", provider.issueCalls)
	}
	var order domain.ArvanCloudCertificateOrder
	if err := json.Unmarshal(result, &order); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if order.ID != "order-1" {
		t.Errorf("order.ID = %q, want order-1", order.ID)
	}
}
