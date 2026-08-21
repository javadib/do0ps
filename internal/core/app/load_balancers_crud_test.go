package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

func TestGetLoadBalancerReturnsMatchingBalancer(t *testing.T) {
	provider := &fakeProvider{loadBalancers: []domain.LoadBalancer{{ID: "lb-1", Name: "api-lb"}}}
	uc := app.NewGetLoadBalancer(&inlineQueue{}, provider)

	lb, err := uc.Execute(context.Background(), app.GetLoadBalancerInput{
		Credentials:    domain.ProviderCredentials{APIKey: "k"},
		LoadBalancerID: "lb-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if lb.Name != "api-lb" {
		t.Errorf("Name = %q, want api-lb", lb.Name)
	}
}

func TestGetLoadBalancerUnknownID(t *testing.T) {
	uc := app.NewGetLoadBalancer(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetLoadBalancerInput{
		Credentials:    domain.ProviderCredentials{APIKey: "k"},
		LoadBalancerID: "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetLoadBalancerRequiresID(t *testing.T) {
	uc := app.NewGetLoadBalancer(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetLoadBalancerInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestListLoadBalancersReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{loadBalancers: []domain.LoadBalancer{
		{ID: "lb-1", Name: "api-lb"}, {ID: "lb-2", Name: "db-lb"},
	}}
	uc := app.NewListLoadBalancers(&inlineQueue{}, provider)

	balancers, err := uc.Execute(context.Background(), app.ListLoadBalancersInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(balancers) != 2 {
		t.Fatalf("len(balancers) = %d, want 2", len(balancers))
	}
}

func TestUpdateLoadBalancerCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewUpdateLoadBalancer(&inlineQueue{}, provider)

	lb, err := uc.Execute(context.Background(), app.UpdateLoadBalancerInput{
		Credentials:    domain.ProviderCredentials{APIKey: "k"},
		LoadBalancerID: "lb-1",
		LoadBalancer:   sampleLoadBalancer(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.updatedLB == nil || provider.updatedLB.Name != "api-lb" {
		t.Errorf("updatedLB = %+v, want the submitted configuration", provider.updatedLB)
	}
	if lb.ID != "lb-1" {
		t.Errorf("result ID = %q, want lb-1", lb.ID)
	}
}

func TestDeleteLoadBalancerCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewDeleteLoadBalancer(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteLoadBalancerInput{
		Credentials:    domain.ProviderCredentials{APIKey: "k"},
		LoadBalancerID: "lb-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedLBID != "lb-1" {
		t.Errorf("deletedLBID = %q, want lb-1", provider.deletedLBID)
	}
}

// TestDeleteLoadBalancerTreatsAlreadyGoneAsSuccess proves delete_load_balancer
// can be called more than once safely.
func TestDeleteLoadBalancerTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeProvider{deleteLBErr: fmt.Errorf("load balancer lb-1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteLoadBalancer(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteLoadBalancerInput{
		Credentials:    domain.ProviderCredentials{APIKey: "k"},
		LoadBalancerID: "lb-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted load balancer", err)
	}
}

// TestGetOperationStatusReconcilesInterruptedLoadBalancer proves an
// interrupted provisioning job is resolved by asking the provider whether the
// balancer already exists, never by replaying the create call (AGENTS.md 4.4).
func TestGetOperationStatusReconcilesInterruptedLoadBalancer(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()

	payload, err := json.Marshal(map[string]any{
		"load_balancer": sampleLoadBalancer(),
	})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}
	job := &domain.Job{
		ID:        "op-1",
		Type:      domain.JobTypeProvisionLoadBalancer,
		Payload:   payload,
		Status:    domain.JobStatusPending,
		Error:     domain.InterruptedReason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The provider already has the balancer: reconcile must mark the
	// operation done instead of recreating it.
	provider := &fakeProvider{loadBalancers: []domain.LoadBalancer{{ID: "lb-7", Name: "api-lb", Status: "active"}}}
	uc := app.NewGetOperationStatus(jobs, provider, nil, fixedClock{t: now.Add(time.Minute)})

	op, err := uc.Execute(context.Background(), app.GetOperationStatusInput{
		OperationID: "op-1",
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if op.Status != domain.OperationStatusSucceeded {
		t.Fatalf("status = %s, want succeeded", op.Status)
	}

	var lb domain.LoadBalancer
	if err := json.Unmarshal(op.Result, &lb); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if lb.ID != "lb-7" {
		t.Errorf("result ID = %q, want lb-7", lb.ID)
	}
	if provider.createdLB != nil {
		t.Errorf("CreateLoadBalancer was called during reconciliation, want lookup only")
	}
}

// TestGetOperationStatusReconcilesMissingLoadBalancerAsFailed proves an
// interrupted job whose balancer was never created is marked failed so the
// request can be retried safely.
func TestGetOperationStatusReconcilesMissingLoadBalancerAsFailed(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()

	payload, err := json.Marshal(map[string]any{
		"load_balancer": sampleLoadBalancer(),
	})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}
	job := &domain.Job{
		ID:        "op-1",
		Type:      domain.JobTypeProvisionLoadBalancer,
		Payload:   payload,
		Status:    domain.JobStatusPending,
		Error:     domain.InterruptedReason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	uc := app.NewGetOperationStatus(jobs, &fakeProvider{}, nil, fixedClock{t: now.Add(time.Minute)})

	op, err := uc.Execute(context.Background(), app.GetOperationStatusInput{
		OperationID: "op-1",
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if op.Status != domain.OperationStatusFailed {
		t.Fatalf("status = %s, want failed", op.Status)
	}
}
