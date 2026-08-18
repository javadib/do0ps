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

	servers   []domain.Server
	deletedID string
	deleteErr error

	cdnZones        []domain.CDNZone
	createdZone     *domain.CDNZone
	createZoneErr   error
	deletedZoneUUID string
	deleteZoneErr   error
	cdnZonePlans    []domain.CDNZonePlanPricing
	nsRecords       *domain.NameserverRecords

	dnsRecords           []domain.DNSRecord
	createdRecord        *domain.DNSRecord
	updatedRecord        *domain.DNSRecord
	deletedRecordHost    string
	deletedRecordType    domain.DNSRecordType
	deletedRecordContent string
	deleteRecordErr      error
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

func (p *fakeProvider) CreateCDNZone(_ context.Context, _ domain.ProviderCredentials, spec domain.CDNZoneSpec) (*domain.CDNZone, error) {
	if p.createZoneErr != nil {
		return nil, p.createZoneErr
	}
	zone := domain.CDNZone{UUID: "zone-uuid-1", Domain: spec.Domain, Status: "active", Plan: spec.Plan, BillingCycle: spec.BillingCycle}
	p.createdZone = &zone
	return &zone, nil
}

func (p *fakeProvider) ListCDNZones(context.Context, domain.ProviderCredentials) ([]domain.CDNZone, error) {
	return p.cdnZones, nil
}

func (p *fakeProvider) GetCDNZone(_ context.Context, _ domain.ProviderCredentials, zoneUUID string) (*domain.CDNZone, error) {
	for i := range p.cdnZones {
		if p.cdnZones[i].UUID == zoneUUID {
			return &p.cdnZones[i], nil
		}
	}
	return nil, fmt.Errorf("zone %q: %w", zoneUUID, domain.ErrNotFound)
}

func (p *fakeProvider) DeleteCDNZone(_ context.Context, _ domain.ProviderCredentials, zoneUUID string) error {
	if p.deleteZoneErr != nil {
		return p.deleteZoneErr
	}
	p.deletedZoneUUID = zoneUUID
	return nil
}

func (p *fakeProvider) ListCDNZonePlans(context.Context, domain.ProviderCredentials) ([]domain.CDNZonePlanPricing, error) {
	return p.cdnZonePlans, nil
}

func (p *fakeProvider) GetNameserverRecords(context.Context, domain.ProviderCredentials, string) (*domain.NameserverRecords, error) {
	return p.nsRecords, nil
}

func (p *fakeProvider) ListDNSRecords(context.Context, domain.ProviderCredentials, string) ([]domain.DNSRecord, error) {
	return p.dnsRecords, nil
}

func (p *fakeProvider) CreateDNSRecord(_ context.Context, _ domain.ProviderCredentials, zoneUUID string, rec domain.DNSRecord) (*domain.DNSRecord, error) {
	rec.ZoneUUID = zoneUUID
	p.createdRecord = &rec
	return &rec, nil
}

func (p *fakeProvider) UpdateDNSRecord(_ context.Context, _ domain.ProviderCredentials, zoneUUID string, rec domain.DNSRecord) (*domain.DNSRecord, error) {
	rec.ZoneUUID = zoneUUID
	p.updatedRecord = &rec
	return &rec, nil
}

func (p *fakeProvider) DeleteDNSRecord(_ context.Context, _ domain.ProviderCredentials, _, host string, recordType domain.DNSRecordType, content string) error {
	if p.deleteRecordErr != nil {
		return p.deleteRecordErr
	}
	p.deletedRecordHost = host
	p.deletedRecordType = recordType
	p.deletedRecordContent = content
	return nil
}

func TestCreateCDNZoneSuccess(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewCreateCDNZone(&inlineQueue{}, provider)

	zone, err := uc.Execute(context.Background(), app.CreateCDNZoneInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Spec:        domain.CDNZoneSpec{Domain: "example.com", Plan: "free", BillingCycle: "free"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if zone.UUID != "zone-uuid-1" {
		t.Errorf("UUID = %q, want zone-uuid-1", zone.UUID)
	}
	if provider.createdZone.Domain != "example.com" {
		t.Errorf("created zone domain = %q, want example.com", provider.createdZone.Domain)
	}
}

func TestCreateCDNZoneRejectsInvalidPlan(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewCreateCDNZone(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNZoneInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Spec:        domain.CDNZoneSpec{Domain: "example.com", Plan: "deluxe", BillingCycle: "free"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdZone != nil {
		t.Error("provider was called with an invalid plan")
	}
}

func TestListCDNZonesReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{cdnZones: []domain.CDNZone{{UUID: "z1", Domain: "example.com"}, {UUID: "z2", Domain: "example.org"}}}
	uc := app.NewListCDNZones(&inlineQueue{}, provider)

	zones, err := uc.Execute(context.Background(), app.ListCDNZonesInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(zones) != 2 {
		t.Fatalf("len(zones) = %d, want 2", len(zones))
	}
}

func TestGetCDNZoneReturnsMatchingZone(t *testing.T) {
	provider := &fakeProvider{cdnZones: []domain.CDNZone{{UUID: "z1", Domain: "example.com"}}}
	uc := app.NewGetCDNZone(&inlineQueue{}, provider)

	zone, err := uc.Execute(context.Background(), app.GetCDNZoneInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if zone.Domain != "example.com" {
		t.Errorf("Domain = %q, want example.com", zone.Domain)
	}
}

func TestGetCDNZoneUnknownUUID(t *testing.T) {
	uc := app.NewGetCDNZone(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNZoneInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestDeleteCDNZoneCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewDeleteCDNZone(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNZoneInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedZoneUUID != "z1" {
		t.Errorf("deletedZoneUUID = %q, want z1", provider.deletedZoneUUID)
	}
}

// TestDeleteCDNZoneTreatsAlreadyGoneAsSuccess proves delete_cdn_zone can be
// called more than once safely.
func TestDeleteCDNZoneTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeProvider{deleteZoneErr: fmt.Errorf("zone z1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteCDNZone(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNZoneInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1"})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted zone", err)
	}
}

func TestListCDNZonePlansReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{cdnZonePlans: []domain.CDNZonePlanPricing{{Plan: "free"}, {Plan: "standard"}}}
	uc := app.NewListCDNZonePlans(&inlineQueue{}, provider)

	plans, err := uc.Execute(context.Background(), app.ListCDNZonePlansInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(plans) != 2 {
		t.Fatalf("len(plans) = %d, want 2", len(plans))
	}
}

func TestGetNameserverRecordsReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{nsRecords: &domain.NameserverRecords{NS1: "ns1.example.net"}}
	uc := app.NewGetNameserverRecords(&inlineQueue{}, provider)

	ns, err := uc.Execute(context.Background(), app.GetNameserverRecordsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if ns.NS1 != "ns1.example.net" {
		t.Errorf("NS1 = %q, want ns1.example.net", ns.NS1)
	}
}

func TestListDNSRecordsReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{dnsRecords: []domain.DNSRecord{{Host: "@", Type: domain.DNSRecordTypeA}}}
	uc := app.NewListDNSRecords(&inlineQueue{}, provider)

	records, err := uc.Execute(context.Background(), app.ListDNSRecordsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
}

func TestCreateDNSRecordSuccess(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewCreateDNSRecord(&inlineQueue{}, provider)

	rec, err := uc.Execute(context.Background(), app.CreateDNSRecordInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "z1",
		Record: domain.DNSRecord{
			Host: "api", Type: domain.DNSRecordTypeA, TTL: 3600, Proxy: domain.DNSRecordProxyDirect,
			Values: []domain.DNSRecordValue{{Content: "203.0.113.10"}},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rec.ZoneUUID != "z1" {
		t.Errorf("ZoneUUID = %q, want z1", rec.ZoneUUID)
	}
	if provider.createdRecord.Host != "api" {
		t.Errorf("created record host = %q, want api", provider.createdRecord.Host)
	}
}

// TestCreateDNSRecordRejectsInvalidTTL proves a bad TTL (and, by the same
// validateDNSRecord path, an unsupported type) is rejected before the
// provider is ever called, per issue #19's acceptance criteria.
func TestCreateDNSRecordRejectsInvalidTTL(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewCreateDNSRecord(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateDNSRecordInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "z1",
		Record: domain.DNSRecord{
			Host: "api", Type: domain.DNSRecordTypeA, TTL: 42, Proxy: domain.DNSRecordProxyDirect,
			Values: []domain.DNSRecordValue{{Content: "203.0.113.10"}},
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdRecord != nil {
		t.Error("provider was called with an invalid TTL")
	}
}

func TestUpdateDNSRecordSuccess(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewUpdateDNSRecord(&inlineQueue{}, provider)

	rec, err := uc.Execute(context.Background(), app.UpdateDNSRecordInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "z1",
		Record: domain.DNSRecord{
			Host: "api", Type: domain.DNSRecordTypeA, TTL: 3600, Proxy: domain.DNSRecordProxyDirect,
			Values: []domain.DNSRecordValue{{Content: "203.0.113.20"}},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rec.Values[0].Content != "203.0.113.20" {
		t.Errorf("content = %q, want 203.0.113.20", rec.Values[0].Content)
	}
}

func TestDeleteDNSRecordCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewDeleteDNSRecord(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteDNSRecordInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "z1",
		Host:        "api",
		Type:        domain.DNSRecordTypeA,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedRecordHost != "api" {
		t.Errorf("deletedRecordHost = %q, want api", provider.deletedRecordHost)
	}
}

// TestDeleteDNSRecordTreatsAlreadyGoneAsSuccess proves delete_dns_record can
// be called more than once safely.
func TestDeleteDNSRecordTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeProvider{deleteRecordErr: fmt.Errorf("record api: %w", domain.ErrNotFound)}
	uc := app.NewDeleteDNSRecord(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteDNSRecordInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "z1",
		Host:        "api",
		Type:        domain.DNSRecordTypeA,
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted record", err)
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
