package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// Load balancer methods on the shared fakeProvider (app_test.go) so the use
// cases run without any real transport, job store or worker pool.

func (p *fakeProvider) CreateLoadBalancer(_ context.Context, _ domain.ProviderCredentials, lb domain.LoadBalancer) (*domain.LoadBalancer, error) {
	lb.ID = "lb-1"
	lb.Status = "new"
	p.createdLB = &lb
	return &lb, nil
}

func (p *fakeProvider) GetLoadBalancer(_ context.Context, _ domain.ProviderCredentials, id string) (*domain.LoadBalancer, error) {
	for i := range p.loadBalancers {
		if p.loadBalancers[i].ID == id {
			return &p.loadBalancers[i], nil
		}
	}
	return nil, fmt.Errorf("load balancer %q: %w", id, domain.ErrNotFound)
}

func (p *fakeProvider) ListLoadBalancers(context.Context, domain.ProviderCredentials) ([]domain.LoadBalancer, error) {
	return p.loadBalancers, nil
}

func (p *fakeProvider) UpdateLoadBalancer(_ context.Context, _ domain.ProviderCredentials, id string, lb domain.LoadBalancer) (*domain.LoadBalancer, error) {
	lb.ID = id
	p.updatedLB = &lb
	return &lb, nil
}

func (p *fakeProvider) DeleteLoadBalancer(_ context.Context, _ domain.ProviderCredentials, id string) error {
	if p.deleteLBErr != nil {
		return p.deleteLBErr
	}
	p.deletedLBID = id
	return nil
}

func (p *fakeProvider) FindLoadBalancerByName(_ context.Context, _ domain.ProviderCredentials, name string) (*domain.LoadBalancer, error) {
	for i := range p.loadBalancers {
		if p.loadBalancers[i].Name == name {
			return &p.loadBalancers[i], nil
		}
	}
	return nil, fmt.Errorf("load balancer %q: %w", name, domain.ErrNotFound)
}

// lbProvisioningProvider flips a freshly created load balancer to "active" on
// the first poll, so a provisioning job converges without real time passing.
type lbProvisioningProvider struct {
	fakeProvider
}

func (p *lbProvisioningProvider) GetLoadBalancer(_ context.Context, _ domain.ProviderCredentials, id string) (*domain.LoadBalancer, error) {
	if p.createdLB != nil && p.createdLB.ID == id {
		lb := *p.createdLB
		lb.Status = "active"
		return &lb, nil
	}
	return nil, fmt.Errorf("load balancer %q: %w", id, domain.ErrNotFound)
}

func sampleLoadBalancer() domain.LoadBalancer {
	return domain.LoadBalancer{
		Name: "api-lb",
		ForwardingRules: []domain.ForwardingRule{
			{EntryProtocol: "http", EntryPort: 80, TargetProtocol: "http", TargetPort: 8080},
		},
		ServerIDs: []string{"vm-1"},
	}
}

func newProvisionLoadBalancer(t *testing.T, jobs *memJobs, queue *inlineQueue, provider ports.ParspackProvider) *app.ProvisionLoadBalancer {
	t.Helper()
	uc, err := app.NewProvisionLoadBalancer(jobs, queue, provider, fixedClock{
		t: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
	}, fixedIDs{id: "op-1"},
		app.WithLoadBalancerPollInterval(time.Millisecond),
		app.WithLoadBalancerPollTimeout(time.Minute),
	)
	if err != nil {
		t.Fatalf("NewProvisionLoadBalancer: %v", err)
	}
	return uc
}

func TestProvisionLoadBalancerReturnsPendingOperation(t *testing.T) {
	jobs := newMemJobs()
	queue := &inlineQueue{}

	uc := newProvisionLoadBalancer(t, jobs, queue, &fakeProvider{})

	out, err := uc.Execute(context.Background(), app.ProvisionLoadBalancerInput{
		Credentials:  domain.ProviderCredentials{APIKey: "k"},
		LoadBalancer: sampleLoadBalancer(),
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
	// Provider credentials must never reach storage.
	if payload := string(stored.Payload); strings.Contains(payload, "api_key") || strings.Contains(payload, "APIKey") {
		t.Errorf("payload appears to contain credentials: %s", payload)
	}
}

func TestProvisionLoadBalancerRequiresNameAndForwardingRules(t *testing.T) {
	uc := newProvisionLoadBalancer(t, newMemJobs(), &inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.ProvisionLoadBalancerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput (missing name)", err)
	}

	_, err = uc.Execute(context.Background(), app.ProvisionLoadBalancerInput{
		Credentials:  domain.ProviderCredentials{APIKey: "k"},
		LoadBalancer: domain.LoadBalancer{Name: "api-lb"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput (missing forwarding rules)", err)
	}
}

// TestProvisionLoadBalancerHandleWaitsUntilActive runs the full job handler:
// create, poll, converge on "active".
func TestProvisionLoadBalancerHandleWaitsUntilActive(t *testing.T) {
	jobs := newMemJobs()
	queue := &inlineQueue{}
	provider := &lbProvisioningProvider{}

	uc := newProvisionLoadBalancer(t, jobs, queue, provider)

	out, err := uc.Execute(context.Background(), app.ProvisionLoadBalancerInput{
		Credentials:  domain.ProviderCredentials{APIKey: "k"},
		LoadBalancer: sampleLoadBalancer(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	job, err := jobs.Get(context.Background(), out.OperationID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	result, err := uc.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var lb domain.LoadBalancer
	if err := json.Unmarshal(result, &lb); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if lb.ID != "lb-1" || lb.Status != "active" {
		t.Errorf("result = %+v, want id lb-1 and status active", lb)
	}
	if provider.createdLB == nil {
		t.Errorf("CreateLoadBalancer was never called")
	}
}

// TestProvisionLoadBalancerHandleAdoptsOnRetry proves a retry never
// duplicates a balancer the provider already has: an existing one is adopted
// instead of creating a second.
func TestProvisionLoadBalancerHandleAdoptsOnRetry(t *testing.T) {
	provider := &fakeProvider{
		loadBalancers: []domain.LoadBalancer{{ID: "lb-9", Name: "api-lb", Status: "active"}},
	}
	jobs := newMemJobs()
	uc := newProvisionLoadBalancer(t, jobs, &inlineQueue{}, provider)

	out, err := uc.Execute(context.Background(), app.ProvisionLoadBalancerInput{
		Credentials:  domain.ProviderCredentials{APIKey: "k"},
		LoadBalancer: sampleLoadBalancer(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	job, err := jobs.Get(context.Background(), out.OperationID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	job.Attempts = 2 // simulate a retry of an operation already attempted

	result, err := uc.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var lb domain.LoadBalancer
	if err := json.Unmarshal(result, &lb); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if lb.ID != "lb-9" {
		t.Errorf("result ID = %q, want lb-9 (adopted, not recreated)", lb.ID)
	}
	if provider.createdLB != nil {
		t.Errorf("CreateLoadBalancer was called on a retry, want adoption instead")
	}
}
