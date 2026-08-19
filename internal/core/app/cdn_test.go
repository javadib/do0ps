package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// The CDN zone and DNS record fakes and tests live in their own file so the
// shared fakeProvider in app_test.go stays small; only the struct fields they
// need are declared there.

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

// TestCreateDNSRecordRejectsInvalidTTL proves a bad TTL is rejected before
// the provider is ever called, per issue #19's acceptance criteria.
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

// TestCreateDNSRecordRejectsInvalidType proves an unsupported/unrecognized
// record type is rejected by the same validateDNSRecord path before the
// provider is ever called, per issue #19's acceptance criteria. A record
// type string that domain.ParseDNSRecordType does not recognize (e.g. an
// MCP caller sending "BOGUS") resolves to domain.DNSRecordTypeUnknown at the
// MCP boundary, which is exactly what this test exercises here at the use
// case layer.
func TestCreateDNSRecordRejectsInvalidType(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewCreateDNSRecord(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateDNSRecordInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "z1",
		Record: domain.DNSRecord{
			Host: "api", Type: domain.DNSRecordTypeUnknown, TTL: 3600, Proxy: domain.DNSRecordProxyDirect,
			Values: []domain.DNSRecordValue{{Content: "203.0.113.10"}},
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdRecord != nil {
		t.Error("provider was called with an unsupported record type")
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
