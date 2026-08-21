package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// accountCertificatePayload builds a job payload matching exactly what
// Execute persists: issueArvanCloudAccountCertificatePayload wraps the
// request under a "request" key (its own json tag), but
// domain.ArvanCloudCertificateOrderIssueRequest and
// domain.ArvanCloudCertificateIssueDomain carry no json tags of their own —
// like every other domain type this project round-trips through
// encoding/json purely internally (job persistence, queue dispatch), field
// names serialize as encoding/json's default: the exact Go field name.
func accountCertificatePayload(t *testing.T) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"request": map[string]any{
			"Domains": []map[string]any{
				{"DomainID": "domain-uuid-1", "DomainNames": []string{"example.com"}},
			},
			"ProductID":      "prod-1",
			"PrivateKeySize": 2048,
		},
	})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}
	return payload
}

// TestGetOperationStatusReconcilesArvanCloudAccountCertificateOrder proves an
// interrupted account-level Certum certificate issuance job is resolved by
// matching the account's order history against the request's domain names,
// never by calling IssueArvanCloudAccountCertificate a second time
// (AGENTS.md 4.4, issue #74's explicit acceptance criterion). Mirrors
// TestGetOperationStatusReconcilesArvanCloudSslOrder.
func TestGetOperationStatusReconcilesArvanCloudAccountCertificateOrder(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()

	job := &domain.Job{
		ID:        "op-1",
		Type:      domain.JobTypeIssueArvanCloudAccountCertificate,
		Payload:   accountCertificatePayload(t),
		Status:    domain.JobStatusPending,
		Error:     domain.InterruptedReason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	provider := &fakeArvanCloudAccountSSLProvider{
		orders: []domain.ArvanCloudAccountCertificateOrder{
			{ID: "order-9", Status: domain.ArvanCloudAccountCertificateOrderStatusValid, DomainNames: []string{"example.com"}, CreatedAt: now.Format(time.RFC3339)},
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
		t.Errorf("IssueArvanCloudAccountCertificate called %d times during reconciliation, want 0 (lookup only)", provider.issueCalls)
	}

	var order domain.ArvanCloudAccountCertificateOrder
	if err := json.Unmarshal(op.Result, &order); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if order.ID != "order-9" {
		t.Errorf("result order ID = %q, want order-9", order.ID)
	}
}

// TestGetOperationStatusReconcilesArvanCloudAccountCertificateOrderStillInFlight
// proves that when a matching order exists but has not reached a terminal
// state, reconciliation reports the job as running rather than a final
// outcome that has not happened yet.
func TestGetOperationStatusReconcilesArvanCloudAccountCertificateOrderStillInFlight(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()

	job := &domain.Job{
		ID:        "op-1",
		Type:      domain.JobTypeIssueArvanCloudAccountCertificate,
		Payload:   accountCertificatePayload(t),
		Status:    domain.JobStatusPending,
		Error:     domain.InterruptedReason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	provider := &fakeArvanCloudAccountSSLProvider{
		orders: []domain.ArvanCloudAccountCertificateOrder{
			{ID: "order-9", Status: "pending", DomainNames: []string{"example.com"}, CreatedAt: now.Format(time.RFC3339)},
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
		t.Errorf("IssueArvanCloudAccountCertificate called %d times during reconciliation, want 0 (lookup only)", provider.issueCalls)
	}

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

// TestGetOperationStatusReconcilesMissingArvanCloudAccountCertificateOrderAsFailed
// proves an interrupted job whose order was never actually placed (the
// process crashed before the issue call reached ArvanCloud) is marked
// failed so the request can be retried safely, and confirms no order lookup
// mistakenly reports success.
func TestGetOperationStatusReconcilesMissingArvanCloudAccountCertificateOrderAsFailed(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()

	job := &domain.Job{
		ID:        "op-1",
		Type:      domain.JobTypeIssueArvanCloudAccountCertificate,
		Payload:   accountCertificatePayload(t),
		Status:    domain.JobStatusPending,
		Error:     domain.InterruptedReason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	provider := &fakeArvanCloudAccountSSLProvider{} // no orders at all
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
		t.Errorf("IssueArvanCloudAccountCertificate called %d times during reconciliation, want 0 (lookup only)", provider.issueCalls)
	}
}
