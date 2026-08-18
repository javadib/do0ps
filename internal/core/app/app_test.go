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

	servers   []domain.Server
	deletedID string
	deleteErr error

	vpcs         []domain.VPC
	createdVPC   *domain.VPC
	deletedVPCID string
	vpcDeleteErr error
}

func (p *fakeProvider) ListServers(context.Context, domain.ProviderCredentials) ([]domain.Server, error) {
	return p.servers, nil
}

func (p *fakeProvider) GetServer(_ context.Context, _ domain.ProviderCredentials, id string) (*domain.Server, error) {
	for i := range p.servers {
		if p.servers[i].ID == id {
			return &p.servers[i], nil
		}
	}
	return nil, fmt.Errorf("server %q: %w", id, domain.ErrNotFound)
}

func (p *fakeProvider) DeleteServer(_ context.Context, _ domain.ProviderCredentials, id string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedID = id
	return nil
}

func (p *fakeProvider) ListDNSZones(context.Context, domain.ProviderCredentials) ([]domain.DNSZone, error) {
	return p.zones, nil
}

func (p *fakeProvider) CreateDNSRecord(_ context.Context, _ domain.ProviderCredentials, rec domain.DNSRecord) (*domain.DNSRecord, error) {
	rec.ID = "rec-1"
	p.created = &rec
	return &rec, nil
}

func (p *fakeProvider) CreateVPC(_ context.Context, _ domain.ProviderCredentials, vpc domain.VPC) (*domain.VPC, error) {
	vpc.ID = "vpc-1"
	p.createdVPC = &vpc
	return &vpc, nil
}

func (p *fakeProvider) GetVPC(_ context.Context, _ domain.ProviderCredentials, id string) (*domain.VPC, error) {
	for i := range p.vpcs {
		if p.vpcs[i].ID == id {
			return &p.vpcs[i], nil
		}
	}
	return nil, fmt.Errorf("VPC %q: %w", id, domain.ErrNotFound)
}

func (p *fakeProvider) ListVPCs(context.Context, domain.ProviderCredentials) ([]domain.VPC, error) {
	return p.vpcs, nil
}

func (p *fakeProvider) DeleteVPC(_ context.Context, _ domain.ProviderCredentials, id string) error {
	if p.vpcDeleteErr != nil {
		return p.vpcDeleteErr
	}
	p.deletedVPCID = id
	return nil
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

func TestListServersReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{servers: []domain.Server{{ID: "vm-1", Name: "web-01"}, {ID: "vm-2", Name: "web-02"}}}
	uc := app.NewListServers(&inlineQueue{}, provider)

	servers, err := uc.Execute(context.Background(), app.ListServersInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("len(servers) = %d, want 2", len(servers))
	}
}

func TestListServersRequiresCredentials(t *testing.T) {
	uc := app.NewListServers(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.ListServersInput{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetServerReturnsMatchingServer(t *testing.T) {
	provider := &fakeProvider{servers: []domain.Server{{ID: "vm-1", Name: "web-01"}}}
	uc := app.NewGetServer(&inlineQueue{}, provider)

	srv, err := uc.Execute(context.Background(), app.GetServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "vm-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if srv.Name != "web-01" {
		t.Errorf("Name = %q, want web-01", srv.Name)
	}
}

func TestGetServerUnknownID(t *testing.T) {
	uc := app.NewGetServer(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetServerRequiresServerID(t *testing.T) {
	uc := app.NewGetServer(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetServerInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteServerCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewDeleteServer(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "vm-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedID != "vm-1" {
		t.Errorf("deletedID = %q, want vm-1", provider.deletedID)
	}
}

// TestDeleteServerTreatsAlreadyGoneAsSuccess proves delete_server can be
// called more than once safely: a not-found response from the provider is
// not surfaced as an error.
func TestDeleteServerTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeProvider{deleteErr: fmt.Errorf("server vm-1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteServer(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "vm-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted server", err)
	}
}

func TestCreateVPCReturnsProviderCopy(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewCreateVPC(&inlineQueue{}, provider)

	vpc, err := uc.Execute(context.Background(), app.CreateVPCInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		VPC:         domain.VPC{Name: "web-net", Region: "tehran", Description: "web tier"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if vpc.ID != "vpc-1" || vpc.Name != "web-net" {
		t.Errorf("vpc = %+v, want id vpc-1 and name web-net", vpc)
	}
	if provider.createdVPC.Region != "tehran" {
		t.Errorf("createdVPC.Region = %q, want tehran", provider.createdVPC.Region)
	}
}

func TestCreateVPCRequiresNameAndRegion(t *testing.T) {
	uc := app.NewCreateVPC(&inlineQueue{}, &fakeProvider{})

	for name, in := range map[string]app.CreateVPCInput{
		"missing name":   {Credentials: domain.ProviderCredentials{APIKey: "k"}, VPC: domain.VPC{Region: "tehran"}},
		"missing region": {Credentials: domain.ProviderCredentials{APIKey: "k"}, VPC: domain.VPC{Name: "web-net"}},
		"missing creds":  {VPC: domain.VPC{Name: "web-net", Region: "tehran"}},
	} {
		if _, err := uc.Execute(context.Background(), in); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: error = %v, want domain.ErrInvalidInput", name, err)
		}
	}
}

func TestListVPCsReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{vpcs: []domain.VPC{{ID: "vpc-1", Name: "web-net"}, {ID: "vpc-2", Name: "db-net"}}}
	uc := app.NewListVPCs(&inlineQueue{}, provider)

	vpcs, err := uc.Execute(context.Background(), app.ListVPCsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(vpcs) != 2 {
		t.Fatalf("len(vpcs) = %d, want 2", len(vpcs))
	}
}

func TestListVPCsRequiresCredentials(t *testing.T) {
	uc := app.NewListVPCs(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.ListVPCsInput{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetVPCReturnsMatchingVPC(t *testing.T) {
	provider := &fakeProvider{vpcs: []domain.VPC{{ID: "vpc-1", Name: "web-net"}}}
	uc := app.NewGetVPC(&inlineQueue{}, provider)

	vpc, err := uc.Execute(context.Background(), app.GetVPCInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		VPCID:       "vpc-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if vpc.Name != "web-net" {
		t.Errorf("Name = %q, want web-net", vpc.Name)
	}
}

func TestGetVPCUnknownID(t *testing.T) {
	uc := app.NewGetVPC(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetVPCInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		VPCID:       "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetVPCRequiresVPCID(t *testing.T) {
	uc := app.NewGetVPC(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetVPCInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteVPCCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewDeleteVPC(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteVPCInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		VPCID:       "vpc-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedVPCID != "vpc-1" {
		t.Errorf("deletedVPCID = %q, want vpc-1", provider.deletedVPCID)
	}
}

// TestDeleteVPCTreatsAlreadyGoneAsSuccess proves delete_vpc can be called more
// than once safely: a not-found response from the provider is not surfaced as
// an error.
func TestDeleteVPCTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeProvider{vpcDeleteErr: fmt.Errorf("VPC vpc-1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteVPC(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteVPCInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		VPCID:       "vpc-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted VPC", err)
	}
}
