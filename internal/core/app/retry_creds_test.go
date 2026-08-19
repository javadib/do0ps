package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// flakyProvider fails every CreateServer, standing in for a provider having a
// bad minute. FindServerByName reports nothing exists, so reconciliation on a
// retry falls through to another create attempt.
type flakyProvider struct {
	ports.ParspackProvider
	createCalls int
}

func (p *flakyProvider) CreateServer(context.Context, domain.ProviderCredentials, domain.ServerSpec) (*domain.Server, error) {
	p.createCalls++
	return nil, errors.New("provider temporarily unavailable")
}

func (p *flakyProvider) FindServerByName(_ context.Context, _ domain.ProviderCredentials, name string) (*domain.Server, error) {
	return nil, fmt.Errorf("server %q: %w", name, domain.ErrNotFound)
}

// A retried long job must still be able to authenticate. Credentials live only
// in memory for the lifetime of the operation, so if the first attempt drops
// them, every later attempt fails with ErrInvalidCredentials and the job burns
// its retry budget without the provider ever being called again.
func TestProvisionServerRetryKeepsCredentials(t *testing.T) {
	jobs := newMemJobs()
	queue := &inlineQueue{}
	provider := &flakyProvider{}
	clock := fixedClock{t: time.Unix(1700000000, 0).UTC()}

	uc, err := app.NewProvisionServer(jobs, queue, provider, clock, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("building use case: %v", err)
	}

	ctx := context.Background()
	if _, err := uc.Execute(ctx, app.ProvisionServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "key"},
		Spec:        domain.ServerSpec{Name: "web-01", PlanID: "p1"},
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	job := queue.submitted[0]

	// Attempt 1: the provider fails, so the pool reschedules the job.
	if err := job.MarkRunning(clock.Now()); err != nil {
		t.Fatalf("marking running: %v", err)
	}
	if _, err := uc.Handle(ctx, job); err == nil {
		t.Fatal("attempt 1: want a provider error, got nil")
	}

	// Attempt 2: exactly what the pool does after its backoff elapses.
	if err := job.Reschedule(clock.Now(), "retrying", clock.Now()); err != nil {
		t.Fatalf("rescheduling: %v", err)
	}
	if err := job.MarkRunning(clock.Now()); err != nil {
		t.Fatalf("marking running: %v", err)
	}
	if _, err := uc.Handle(ctx, job); errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("attempt 2 lost the caller's credentials: %v", err)
	}

	if provider.createCalls != 2 {
		t.Fatalf("provider saw %d create attempts, want 2", provider.createCalls)
	}
}
