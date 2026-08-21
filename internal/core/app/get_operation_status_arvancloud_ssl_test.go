package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// TestGetOperationStatusReconcilesArvanCloudSslOrder proves an interrupted
// managed-certificate issuance job (one recovery.Run found "running" at
// process startup and flagged as interrupted) is resolved by querying
// ssl.cert.order.index (ListArvanCloudSslOrders) for the domain's actual
// current order — never by calling IssueArvanCloudManagedCertificate a
// second time (AGENTS.md 4.4, issue #73's explicit acceptance criterion).
// Mirrors TestGetOperationStatusReconcilesInterruptedSnapshot (snapshots_test.go).
func TestGetOperationStatusReconcilesArvanCloudSslOrder(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()

	payload, err := json.Marshal(map[string]any{"domain": "example.com"})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}
	job := &domain.Job{
		ID:        "op-1",
		Type:      domain.JobTypeIssueArvanCloudManagedCertificate,
		Payload:   payload,
		Status:    domain.JobStatusPending,
		Error:     domain.InterruptedReason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The provider already reports the order as valid (issued moments before
	// the process crashed), so reconcile must adopt it rather than issue a
	// second one.
	provider := &fakeArvanCloudSSLProvider{
		orders: []domain.ArvanCloudCertificateOrder{
			{ID: "order-9", Status: domain.ArvanCloudCertificateOrderStatusValid, DomainNames: []string{"example.com"}, CreatedAt: now.Format(time.RFC3339)},
		},
	}
	uc := app.NewGetOperationStatus(jobs, &fakeProvider{}, provider, fixedClock{t: now.Add(time.Minute)})

	op, err := uc.Execute(context.Background(), app.GetOperationStatusInput{
		OperationID: "op-1",
		Credentials: domain.ProviderCredentials{APIKey: "arvan-key"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if op.Status != domain.OperationStatusSucceeded {
		t.Fatalf("status = %s, want succeeded", op.Status)
	}
	if provider.issueCalls != 0 {
		t.Errorf("IssueArvanCloudManagedCertificate called %d times during reconciliation, want 0 (lookup only)", provider.issueCalls)
	}

	var order domain.ArvanCloudCertificateOrder
	if err := json.Unmarshal(op.Result, &order); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if order.ID != "order-9" {
		t.Errorf("result order ID = %q, want order-9", order.ID)
	}
}

// TestGetOperationStatusReconcilesArvanCloudSslOrderStillInFlight proves that
// when the domain's latest order exists but has not reached a terminal
// state, reconciliation reports the job as running rather than a final
// outcome that has not happened yet — and, critically, still never calls
// IssueArvanCloudManagedCertificate again.
func TestGetOperationStatusReconcilesArvanCloudSslOrderStillInFlight(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()

	payload, err := json.Marshal(map[string]any{"domain": "example.com"})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}
	job := &domain.Job{
		ID:        "op-1",
		Type:      domain.JobTypeIssueArvanCloudManagedCertificate,
		Payload:   payload,
		Status:    domain.JobStatusPending,
		Error:     domain.InterruptedReason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	provider := &fakeArvanCloudSSLProvider{
		orders: []domain.ArvanCloudCertificateOrder{
			{ID: "order-9", Status: domain.ArvanCloudCertificateOrderStatusPending, DomainNames: []string{"example.com"}, CreatedAt: now.Format(time.RFC3339)},
		},
	}
	uc := app.NewGetOperationStatus(jobs, &fakeProvider{}, provider, fixedClock{t: now.Add(time.Minute)})

	op, err := uc.Execute(context.Background(), app.GetOperationStatusInput{
		OperationID: "op-1",
		Credentials: domain.ProviderCredentials{APIKey: "arvan-key"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if op.Status != domain.OperationStatusRunning {
		t.Fatalf("status = %s, want running", op.Status)
	}
	if provider.issueCalls != 0 {
		t.Errorf("IssueArvanCloudManagedCertificate called %d times during reconciliation, want 0 (lookup only)", provider.issueCalls)
	}

	// The persisted job itself must reflect Running too, not just this one
	// response, so a later call without credentials also reports "running"
	// instead of a stale "interrupted".
	stored, err := jobs.Get(context.Background(), "op-1")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	if stored.Status != domain.JobStatusRunning {
		t.Errorf("stored job status = %s, want running", stored.Status)
	}
	if stored.Error != "" {
		t.Errorf("stored job error = %q, want cleared once genuinely running", stored.Error)
	}
}

// TestGetOperationStatusReconcilesMissingArvanCloudSslOrderAsFailed proves an
// interrupted job whose order was never actually placed (the process crashed
// before the ssl.cert.issue call reached ArvanCloud) is marked failed so the
// request can be retried safely, and confirms no order lookup mistakenly
// reports success.
func TestGetOperationStatusReconcilesMissingArvanCloudSslOrderAsFailed(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()

	payload, err := json.Marshal(map[string]any{"domain": "example.com"})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}
	job := &domain.Job{
		ID:        "op-1",
		Type:      domain.JobTypeIssueArvanCloudManagedCertificate,
		Payload:   payload,
		Status:    domain.JobStatusPending,
		Error:     domain.InterruptedReason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	provider := &fakeArvanCloudSSLProvider{} // no orders at all for the domain
	uc := app.NewGetOperationStatus(jobs, &fakeProvider{}, provider, fixedClock{t: now.Add(time.Minute)})

	op, err := uc.Execute(context.Background(), app.GetOperationStatusInput{
		OperationID: "op-1",
		Credentials: domain.ProviderCredentials{APIKey: "arvan-key"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if op.Status != domain.OperationStatusFailed {
		t.Fatalf("status = %s, want failed", op.Status)
	}
	if provider.issueCalls != 0 {
		t.Errorf("IssueArvanCloudManagedCertificate called %d times during reconciliation, want 0 (lookup only)", provider.issueCalls)
	}
}
