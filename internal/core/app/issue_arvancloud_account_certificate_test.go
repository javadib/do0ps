package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

func sampleAccountCertificateIssueRequest() domain.ArvanCloudCertificateOrderIssueRequest {
	return domain.ArvanCloudCertificateOrderIssueRequest{
		Domains: []domain.ArvanCloudCertificateIssueDomain{
			{DomainID: "domain-uuid-1", DomainNames: []string{"example.com"}},
		},
		ProductID:      "prod-1",
		PrivateKeySize: 2048,
	}
}

// TestIssueArvanCloudAccountCertificateReturnsPendingOperation proves the
// tool returns a pending job/operation immediately rather than blocking on
// the provider — issue #74's acceptance criterion, mirroring
// TestIssueArvanCloudManagedCertificateReturnsPendingOperation.
func TestIssueArvanCloudAccountCertificateReturnsPendingOperation(t *testing.T) {
	jobs := newMemJobs()
	queue := &inlineQueue{}
	clock := fixedClock{t: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)}
	provider := &fakeArvanCloudAccountSSLProvider{}

	uc, err := app.NewIssueArvanCloudAccountCertificate(jobs, queue, provider, clock, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("NewIssueArvanCloudAccountCertificate: %v", err)
	}

	out, err := uc.Execute(context.Background(), app.IssueArvanCloudAccountCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "arvan-key"},
		Request:     sampleAccountCertificateIssueRequest(),
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
	if provider.issueCalls != 0 {
		t.Errorf("IssueArvanCloudAccountCertificate called %d times during Execute, want 0 (it must run on a worker, not inline)", provider.issueCalls)
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
	if stored.Type != domain.JobTypeIssueArvanCloudAccountCertificate {
		t.Errorf("stored job type = %s, want %s", stored.Type, domain.JobTypeIssueArvanCloudAccountCertificate)
	}
	if payload := string(stored.Payload); strings.Contains(payload, "arvan-key") || strings.Contains(payload, "APIKey") {
		t.Errorf("payload appears to contain credentials: %s", payload)
	}
}

// TestIssueArvanCloudAccountCertificateRejectsInvalidRequest proves the use
// case validates its input before ever touching the queue or the provider —
// missing domains, missing product_id, and an out-of-enum private_key_size.
func TestIssueArvanCloudAccountCertificateRejectsInvalidRequest(t *testing.T) {
	uc, err := app.NewIssueArvanCloudAccountCertificate(newMemJobs(), &inlineQueue{}, &fakeArvanCloudAccountSSLProvider{}, fixedClock{}, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("NewIssueArvanCloudAccountCertificate: %v", err)
	}

	tests := []struct {
		name string
		req  domain.ArvanCloudCertificateOrderIssueRequest
	}{
		{"missing domains", domain.ArvanCloudCertificateOrderIssueRequest{ProductID: "prod-1"}},
		{"missing product_id", domain.ArvanCloudCertificateOrderIssueRequest{Domains: []domain.ArvanCloudCertificateIssueDomain{{DomainID: "d1", DomainNames: []string{"example.com"}}}}},
		{"invalid private_key_size", func() domain.ArvanCloudCertificateOrderIssueRequest {
			req := sampleAccountCertificateIssueRequest()
			req.PrivateKeySize = 1024
			return req
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), app.IssueArvanCloudAccountCertificateInput{
				Credentials: domain.ProviderCredentials{APIKey: "k"},
				Request:     tt.req,
			})
			if err == nil {
				t.Fatal("Execute() error = nil, want a validation error")
			}
		})
	}
}

// TestIssueArvanCloudAccountCertificateHandleAdoptsInFlightOrderOnRetry
// proves that within the SAME process, a retried Handle call (job.Attempts >
// 1) never calls IssueArvanCloudAccountCertificate a second time when the
// provider already reports a matching order — the in-process half of
// AGENTS.md 4.4's "never issue twice" rule. Mirrors
// TestIssueArvanCloudManagedCertificateHandleAdoptsInFlightOrderOnRetry.
func TestIssueArvanCloudAccountCertificateHandleAdoptsInFlightOrderOnRetry(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()
	queue := &inlineQueue{}

	provider := &fakeArvanCloudAccountSSLProvider{
		orders: []domain.ArvanCloudAccountCertificateOrder{
			{ID: "order-1", Status: domain.ArvanCloudAccountCertificateOrderStatusValid, DomainNames: []string{"example.com"}, CreatedAt: now.Format(time.RFC3339)},
		},
	}

	uc, err := app.NewIssueArvanCloudAccountCertificate(jobs, queue, provider, fixedClock{t: now}, fixedIDs{id: "op-1"},
		app.WithArvanCloudAccountCertificatePollInterval(time.Millisecond),
		app.WithArvanCloudAccountCertificatePollTimeout(time.Second),
	)
	if err != nil {
		t.Fatalf("NewIssueArvanCloudAccountCertificate: %v", err)
	}

	if _, err := uc.Execute(context.Background(), app.IssueArvanCloudAccountCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Request:     sampleAccountCertificateIssueRequest(),
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	job, err := jobs.Get(context.Background(), "op-1")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	job.Attempts = 2 // simulate a retry

	result, err := uc.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if provider.issueCalls != 0 {
		t.Errorf("IssueArvanCloudAccountCertificate called %d times on retry, want 0 (must adopt the existing order)", provider.issueCalls)
	}

	var order domain.ArvanCloudAccountCertificateOrder
	if err := json.Unmarshal(result, &order); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if order.ID != "order-1" || order.Status != domain.ArvanCloudAccountCertificateOrderStatusValid {
		t.Errorf("order = %+v, want the adopted valid order-1", order)
	}
}

// TestIssueArvanCloudAccountCertificateHandleIssuesWhenNoMatchingOrder
// proves that when no existing order matches the requested domain names, a
// retry places a brand new order rather than adopting an unrelated one.
func TestIssueArvanCloudAccountCertificateHandleIssuesWhenNoMatchingOrder(t *testing.T) {
	jobs := newMemJobs()
	queue := &inlineQueue{}
	provider := &fakeArvanCloudAccountSSLProvider{
		issued: domain.ArvanCloudAccountCertificateOrder{ID: "order-1", Status: domain.ArvanCloudAccountCertificateOrderStatusValid},
	}

	uc, err := app.NewIssueArvanCloudAccountCertificate(jobs, queue, provider, fixedClock{t: time.Now()}, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("NewIssueArvanCloudAccountCertificate: %v", err)
	}

	if _, err := uc.Execute(context.Background(), app.IssueArvanCloudAccountCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Request:     sampleAccountCertificateIssueRequest(),
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	job, err := jobs.Get(context.Background(), "op-1")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	job.Attempts = 2 // force the retry/adopt path; provider has no matching orders yet

	result, err := uc.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if provider.issueCalls != 1 {
		t.Errorf("IssueArvanCloudAccountCertificate called %d times, want 1 (no existing order to adopt)", provider.issueCalls)
	}
	var order domain.ArvanCloudAccountCertificateOrder
	if err := json.Unmarshal(result, &order); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if order.ID != "order-1" {
		t.Errorf("order.ID = %q, want order-1", order.ID)
	}
}
