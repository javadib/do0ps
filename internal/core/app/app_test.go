package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// The fakes below are the point of the ports layer: the use cases are tested
// with no database, no HTTP server and no worker pool in sight.

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fixedIDs struct{ id string }

func (g fixedIDs) NewID() string { return g.id }

// inlineQueue runs fast tasks synchronously and records submitted jobs.
type inlineQueue struct {
	submitted []*domain.Job
}

func (q *inlineQueue) Dispatch(ctx context.Context, task ports.Task) (json.RawMessage, error) {
	return task(ctx)
}

func (q *inlineQueue) Submit(_ context.Context, job *domain.Job) error {
	q.submitted = append(q.submitted, job)
	return nil
}

// memJobs is an in-memory ports.JobRepository.
type memJobs struct {
	jobs map[string]*domain.Job
}

func newMemJobs() *memJobs { return &memJobs{jobs: make(map[string]*domain.Job)} }

func (r *memJobs) Create(_ context.Context, job *domain.Job) error {
	r.jobs[job.ID] = job
	return nil
}

func (r *memJobs) Get(_ context.Context, id string) (*domain.Job, error) {
	job, ok := r.jobs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return job, nil
}

func (r *memJobs) Update(_ context.Context, job *domain.Job) error {
	r.jobs[job.ID] = job
	return nil
}

func (r *memJobs) ListUnfinished(context.Context) ([]*domain.Job, error) {
	var unfinished []*domain.Job
	for _, job := range r.jobs {
		if !job.IsTerminal() {
			unfinished = append(unfinished, job)
		}
	}
	return unfinished, nil
}

func (r *memJobs) ListDue(context.Context, time.Time, int) ([]*domain.Job, error) { return nil, nil }

// fakeProvider embeds the port so only the methods a test needs are written.
type fakeProvider struct {
	ports.ParspackProvider

	zones   []domain.DNSZone
	created *domain.DNSRecord
}

func (p *fakeProvider) ListDNSZones(context.Context, domain.ProviderCredentials) ([]domain.DNSZone, error) {
	return p.zones, nil
}

func (p *fakeProvider) CreateDNSRecord(_ context.Context, _ domain.ProviderCredentials, rec domain.DNSRecord) (*domain.DNSRecord, error) {
	rec.ID = "rec-1"
	p.created = &rec
	return &rec, nil
}

func TestSetupDNSResolvesZoneAndCreatesRecord(t *testing.T) {
	provider := &fakeProvider{zones: []domain.DNSZone{{ID: "zone-1", Name: "example.com"}}}
	uc := app.NewSetupDNS(&inlineQueue{}, provider)

	rec, err := uc.Execute(context.Background(), app.SetupDNSInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneName:    "example.com",
		Record: domain.DNSRecord{
			Name:  "api",
			Type:  domain.DNSRecordTypeA,
			Value: "203.0.113.10",
			TTL:   3600,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rec.ID != "rec-1" {
		t.Errorf("record ID = %q, want %q", rec.ID, "rec-1")
	}
	if provider.created.ZoneID != "zone-1" {
		t.Errorf("zone ID = %q, want %q — the zone name was not resolved", provider.created.ZoneID, "zone-1")
	}
}

func TestSetupDNSUnknownZone(t *testing.T) {
	uc := app.NewSetupDNS(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.SetupDNSInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneName:    "missing.example",
		Record:      domain.DNSRecord{Name: "api", Type: domain.DNSRecordTypeA, Value: "203.0.113.10"},
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestProvisionServerReturnsPendingOperation(t *testing.T) {
	jobs := newMemJobs()
	queue := &inlineQueue{}
	clock := fixedClock{t: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)}

	uc, err := app.NewProvisionServer(jobs, queue, &fakeProvider{}, clock, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("NewProvisionServer: %v", err)
	}

	out, err := uc.Execute(context.Background(), app.ProvisionServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Spec:        domain.ServerSpec{Name: "web-01", CPUCores: 2, RAMMB: 2048},
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

func TestProvisionServerRejectsUnderspecifiedRequest(t *testing.T) {
	uc, err := app.NewProvisionServer(newMemJobs(), &inlineQueue{}, &fakeProvider{}, fixedClock{}, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("NewProvisionServer: %v", err)
	}

	_, err = uc.Execute(context.Background(), app.ProvisionServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Spec:        domain.ServerSpec{Name: "web-01"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
