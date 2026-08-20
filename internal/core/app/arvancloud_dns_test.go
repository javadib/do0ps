package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// fakeArvanCloudDNSProvider embeds the port so a test only needs to override
// the methods it actually exercises, mirroring fakeArvanCloudProvider in
// arvancloud_domain_test.go.
type fakeArvanCloudDNSProvider struct {
	ports.ArvanCloudProvider

	records map[string]domain.ArvanCloudDNSRecord // by id

	createdRecord domain.ArvanCloudDNSRecord
	createErr     error

	updatedID     string
	updatedRecord domain.ArvanCloudDNSRecord

	deletedID string
	deleteErr error

	toggledID    string
	toggledCloud bool

	importedZoneFile []byte
	importErr        error

	exportContent string
	exportErr     error

	dnsSecStatus domain.ArvanCloudDNSSecStatus

	secondaryDNSConfig domain.ArvanCloudSecondaryDNSConfig
	removedSecondary   bool
	removeErr          error
}

func (p *fakeArvanCloudDNSProvider) ListArvanCloudDNSRecords(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudDNSRecord, error) {
	out := make([]domain.ArvanCloudDNSRecord, 0, len(p.records))
	for _, r := range p.records {
		out = append(out, r)
	}
	return out, nil
}

func (p *fakeArvanCloudDNSProvider) CreateArvanCloudDNSRecord(_ context.Context, _ domain.ProviderCredentials, _ string, rec domain.ArvanCloudDNSRecord) (*domain.ArvanCloudDNSRecord, error) {
	if p.createErr != nil {
		return nil, p.createErr
	}
	p.createdRecord = rec
	created := rec
	created.ID = "uuid-new"
	return &created, nil
}

func (p *fakeArvanCloudDNSProvider) GetArvanCloudDNSRecord(_ context.Context, _ domain.ProviderCredentials, _, id string) (*domain.ArvanCloudDNSRecord, error) {
	if rec, ok := p.records[id]; ok {
		return &rec, nil
	}
	return nil, fmt.Errorf("record %q: %w", id, domain.ErrNotFound)
}

func (p *fakeArvanCloudDNSProvider) UpdateArvanCloudDNSRecord(_ context.Context, _ domain.ProviderCredentials, _, id string, rec domain.ArvanCloudDNSRecord) (*domain.ArvanCloudDNSRecord, error) {
	p.updatedID = id
	p.updatedRecord = rec
	updated := rec
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudDNSProvider) DeleteArvanCloudDNSRecord(_ context.Context, _ domain.ProviderCredentials, _, id string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedID = id
	return nil
}

func (p *fakeArvanCloudDNSProvider) ToggleArvanCloudDNSRecordCloud(_ context.Context, _ domain.ProviderCredentials, _, id string, cloud bool) (*domain.ArvanCloudDNSRecord, error) {
	p.toggledID = id
	p.toggledCloud = cloud
	return &domain.ArvanCloudDNSRecord{ID: id, Cloud: cloud}, nil
}

func (p *fakeArvanCloudDNSProvider) ImportArvanCloudDNSRecords(_ context.Context, _ domain.ProviderCredentials, _ string, zoneFile []byte) error {
	if p.importErr != nil {
		return p.importErr
	}
	p.importedZoneFile = zoneFile
	return nil
}

func (p *fakeArvanCloudDNSProvider) ExportArvanCloudDNSRecords(context.Context, domain.ProviderCredentials, string) (string, error) {
	if p.exportErr != nil {
		return "", p.exportErr
	}
	return p.exportContent, nil
}

func (p *fakeArvanCloudDNSProvider) GetArvanCloudDNSSecStatus(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudDNSSecStatus, error) {
	status := p.dnsSecStatus
	return &status, nil
}

func (p *fakeArvanCloudDNSProvider) UpdateArvanCloudDNSSecStatus(_ context.Context, _ domain.ProviderCredentials, _ string, enable, _ bool) (*domain.ArvanCloudDNSSecStatus, error) {
	p.dnsSecStatus.Enabled = enable
	status := p.dnsSecStatus
	return &status, nil
}

func (p *fakeArvanCloudDNSProvider) GetArvanCloudSecondaryDNS(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudSecondaryDNSConfig, error) {
	cfg := p.secondaryDNSConfig
	return &cfg, nil
}

func (p *fakeArvanCloudDNSProvider) SetArvanCloudSecondaryDNS(_ context.Context, _ domain.ProviderCredentials, _ string, cfg domain.ArvanCloudSecondaryDNSConfig) (*domain.ArvanCloudSecondaryDNSConfig, error) {
	p.secondaryDNSConfig = cfg
	out := cfg
	return &out, nil
}

func (p *fakeArvanCloudDNSProvider) RemoveArvanCloudSecondaryDNS(context.Context, domain.ProviderCredentials, string) error {
	if p.removeErr != nil {
		return p.removeErr
	}
	p.removedSecondary = true
	return nil
}

// validARecord is a minimal, valid A record used by tests that only need
// something that passes validation.
func validARecord() domain.ArvanCloudDNSRecord {
	return domain.ArvanCloudDNSRecord{
		Name: "www", Type: domain.ArvanCloudDNSRecordTypeA, TTL: 3600,
		Values: []domain.ArvanCloudDNSRecordValue{{IP: "198.51.100.1"}},
	}
}

func TestCreateArvanCloudDNSRecordSuccess(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewCreateArvanCloudDNSRecord(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudDNSRecordInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", Record: validARecord(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "uuid-new" {
		t.Errorf("created.ID = %q, want %q", created.ID, "uuid-new")
	}
	if provider.createdRecord.Name != "www" {
		t.Errorf("provider received record %+v, want name \"www\"", provider.createdRecord)
	}
}

// TestCreateArvanCloudDNSRecordRejectsInvalidTTL is the AC #63 pin: the
// use-case-level TTL enum validator must reject an arbitrary value like 121
// before dispatching to the provider.
func TestCreateArvanCloudDNSRecordRejectsInvalidTTL(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewCreateArvanCloudDNSRecord(&inlineQueue{}, provider)

	rec := validARecord()
	rec.TTL = 121

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudDNSRecordInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", Record: rec,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdRecord.Name != "" {
		t.Error("provider was called despite the invalid TTL; validation must happen before dispatch")
	}
}

// TestCreateArvanCloudDNSRecordRejectsInvalidType is the AC #63 pin: a
// record type outside the 13-value set must be rejected before dispatch.
func TestCreateArvanCloudDNSRecordRejectsInvalidType(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewCreateArvanCloudDNSRecord(&inlineQueue{}, provider)

	rec := validARecord()
	rec.Type = domain.ArvanCloudDNSRecordTypeUnknown

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudDNSRecordInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", Record: rec,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// TestCreateArvanCloudDNSRecordRejectsCAAMissingTag is the AC #63 pin: a CAA
// record missing "tag" must be rejected client-side before dispatch, rather
// than relying on the provider's 422.
func TestCreateArvanCloudDNSRecordRejectsCAAMissingTag(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewCreateArvanCloudDNSRecord(&inlineQueue{}, provider)

	rec := domain.ArvanCloudDNSRecord{
		Name: "@", Type: domain.ArvanCloudDNSRecordTypeCAA, TTL: 3600,
		Values: []domain.ArvanCloudDNSRecordValue{{CAAValue: "letsencrypt.org"}}, // Tag deliberately omitted
	}

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudDNSRecordInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", Record: rec,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdRecord.Name != "" {
		t.Error("provider was called despite the missing CAA tag; validation must happen before dispatch")
	}
}

// TestCreateArvanCloudDNSRecordRejectsCAAInvalidTag proves the tag, once
// present, is also checked against its enum.
func TestCreateArvanCloudDNSRecordRejectsCAAInvalidTag(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewCreateArvanCloudDNSRecord(&inlineQueue{}, provider)

	rec := domain.ArvanCloudDNSRecord{
		Name: "@", Type: domain.ArvanCloudDNSRecordTypeCAA, TTL: 3600,
		Values: []domain.ArvanCloudDNSRecordValue{{CAAValue: "letsencrypt.org", Tag: "bogus"}},
	}

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudDNSRecordInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", Record: rec,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// TestCreateArvanCloudDNSRecordRejectsMissingRequiredValueFields exercises
// the rest of the per-type required-field checks the issue calls for, one
// case per type that has at least one required value field.
func TestCreateArvanCloudDNSRecordRejectsMissingRequiredValueFields(t *testing.T) {
	tests := []struct {
		name string
		rec  domain.ArvanCloudDNSRecord
	}{
		{"A missing ip", domain.ArvanCloudDNSRecord{Name: "www", Type: domain.ArvanCloudDNSRecordTypeA, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{}}}},
		{"CNAME missing host", domain.ArvanCloudDNSRecord{Name: "www", Type: domain.ArvanCloudDNSRecordTypeCNAME, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{}}}},
		{"ANAME missing location", domain.ArvanCloudDNSRecord{Name: "@", Type: domain.ArvanCloudDNSRecordTypeANAME, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{}}}},
		{"MX missing host", domain.ArvanCloudDNSRecord{Name: "@", Type: domain.ArvanCloudDNSRecordTypeMX, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Priority: 10}}}},
		{"SRV missing target", domain.ArvanCloudDNSRecord{Name: "_sip._tcp", Type: domain.ArvanCloudDNSRecordTypeSRV, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Port: 5060}}}},
		{"SRV missing port", domain.ArvanCloudDNSRecord{Name: "_sip._tcp", Type: domain.ArvanCloudDNSRecordTypeSRV, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Target: "sip.example.com"}}}},
		{"TXT missing text", domain.ArvanCloudDNSRecord{Name: "@", Type: domain.ArvanCloudDNSRecordTypeTXT, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{}}}},
		{"NS missing host", domain.ArvanCloudDNSRecord{Name: "sub", Type: domain.ArvanCloudDNSRecordTypeNS, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{}}}},
		{
			"TLSA missing fields", domain.ArvanCloudDNSRecord{
				Name: "_443._tcp", Type: domain.ArvanCloudDNSRecordTypeTLSA, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Usage: "3"}}, // selector/matching_type/certificate missing
			},
		},
		{"CAA missing value", domain.ArvanCloudDNSRecord{Name: "@", Type: domain.ArvanCloudDNSRecordTypeCAA, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Tag: "issue"}}}},
		{"wrong value count for a single-value type", domain.ArvanCloudDNSRecord{Name: "www", Type: domain.ArvanCloudDNSRecordTypeCNAME, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Host: "a"}, {Host: "b"}}}},
		{"A with no values at all", domain.ArvanCloudDNSRecord{Name: "www", Type: domain.ArvanCloudDNSRecordTypeA, TTL: 3600}},
	}

	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewCreateArvanCloudDNSRecord(&inlineQueue{}, provider)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), app.CreateArvanCloudDNSRecordInput{
				Credentials: validArvanCloudCreds(), DomainName: "example.com", Record: tc.rec,
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

// TestValidArvanCloudDNSRecordAcceptsEveryType is the positive counterpart:
// a well-formed record of each of the 13 types must pass validation and
// reach the provider.
func TestValidArvanCloudDNSRecordAcceptsEveryType(t *testing.T) {
	tests := []struct {
		name string
		rec  domain.ArvanCloudDNSRecord
	}{
		{"A", domain.ArvanCloudDNSRecord{Name: "www", Type: domain.ArvanCloudDNSRecordTypeA, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{IP: "198.51.100.1"}}}},
		{"AAAA", domain.ArvanCloudDNSRecord{Name: "www", Type: domain.ArvanCloudDNSRecordTypeAAAA, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{IP: "2001:db8::1"}}}},
		{"CNAME", domain.ArvanCloudDNSRecord{Name: "shop", Type: domain.ArvanCloudDNSRecordTypeCNAME, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Host: "cdn.example.net"}}}},
		{"ANAME", domain.ArvanCloudDNSRecord{Name: "@", Type: domain.ArvanCloudDNSRecordTypeANAME, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Location: "cdn.example.com"}}}},
		{"MX", domain.ArvanCloudDNSRecord{Name: "@", Type: domain.ArvanCloudDNSRecordTypeMX, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Host: "mail.example.com", Priority: 10}}}},
		{"SRV", domain.ArvanCloudDNSRecord{Name: "_sip._tcp", Type: domain.ArvanCloudDNSRecordTypeSRV, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Target: "sip.example.com", Port: 5060}}}},
		{"TXT", domain.ArvanCloudDNSRecord{Name: "@", Type: domain.ArvanCloudDNSRecordTypeTXT, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Text: "hello"}}}},
		{"SPF", domain.ArvanCloudDNSRecord{Name: "@", Type: domain.ArvanCloudDNSRecordTypeSPF, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Text: "v=spf1 -all"}}}},
		{"DKIM", domain.ArvanCloudDNSRecord{Name: "default._domainkey", Type: domain.ArvanCloudDNSRecordTypeDKIM, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Text: "v=DKIM1"}}}},
		{"NS", domain.ArvanCloudDNSRecord{Name: "sub", Type: domain.ArvanCloudDNSRecordTypeNS, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Host: "ns1.example.com"}}}},
		{"PTR", domain.ArvanCloudDNSRecord{Name: "42.100.51.198.in-addr.arpa", Type: domain.ArvanCloudDNSRecordTypePTR, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{}}}},
		{"TLSA", domain.ArvanCloudDNSRecord{Name: "_443._tcp", Type: domain.ArvanCloudDNSRecordTypeTLSA, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{Usage: "3", Selector: "1", MatchingType: "1", Certificate: "abc"}}}},
		{"CAA", domain.ArvanCloudDNSRecord{Name: "@", Type: domain.ArvanCloudDNSRecordTypeCAA, TTL: 3600, Values: []domain.ArvanCloudDNSRecordValue{{CAAValue: "letsencrypt.org", Tag: "issue"}}}},
	}
	if len(tests) != 13 {
		t.Fatalf("len(tests) = %d, want 13", len(tests))
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &fakeArvanCloudDNSProvider{}
			uc := app.NewCreateArvanCloudDNSRecord(&inlineQueue{}, provider)

			created, err := uc.Execute(context.Background(), app.CreateArvanCloudDNSRecordInput{
				Credentials: validArvanCloudCreds(), DomainName: "example.com", Record: tc.rec,
			})
			if err != nil {
				t.Fatalf("Execute() error = %v, want a valid %s record to be accepted", err, tc.name)
			}
			if created.ID == "" {
				t.Error("created.ID is empty, want the fake provider's assigned id")
			}
		})
	}
}

func TestListArvanCloudDNSRecords(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{records: map[string]domain.ArvanCloudDNSRecord{
		"uuid-1": {ID: "uuid-1", Name: "www", Type: domain.ArvanCloudDNSRecordTypeA},
	}}
	uc := app.NewListArvanCloudDNSRecords(&inlineQueue{}, provider)

	records, err := uc.Execute(context.Background(), app.ListArvanCloudDNSRecordsInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
}

func TestGetArvanCloudDNSRecordNotFound(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{records: map[string]domain.ArvanCloudDNSRecord{}}
	uc := app.NewGetArvanCloudDNSRecord(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetArvanCloudDNSRecordInput{Credentials: validArvanCloudCreds(), DomainName: "example.com", ID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateArvanCloudDNSRecordSuccess(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewUpdateArvanCloudDNSRecord(&inlineQueue{}, provider)

	rec := validARecord()
	rec.TTL = 7200
	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudDNSRecordInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", ID: "uuid-1", Record: rec,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.ID != "uuid-1" || provider.updatedID != "uuid-1" || provider.updatedRecord.TTL != 7200 {
		t.Errorf("updated = %+v, provider state = id=%q record=%+v, want id/ttl propagated", updated, provider.updatedID, provider.updatedRecord)
	}
}

func TestUpdateArvanCloudDNSRecordRejectsInvalidTTL(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewUpdateArvanCloudDNSRecord(&inlineQueue{}, provider)

	rec := validARecord()
	rec.TTL = 121
	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudDNSRecordInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", ID: "uuid-1", Record: rec,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteArvanCloudDNSRecordTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{deleteErr: fmt.Errorf("record: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudDNSRecord(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteArvanCloudDNSRecordInput{Credentials: validArvanCloudCreds(), DomainName: "example.com", ID: "gone"})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil: deleting an already-gone record must be treated as done", err)
	}
}

func TestDeleteArvanCloudDNSRecordMissingID(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewDeleteArvanCloudDNSRecord(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteArvanCloudDNSRecordInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestToggleArvanCloudDNSRecordCloudSuccess(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewToggleArvanCloudDNSRecordCloud(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.ToggleArvanCloudDNSRecordCloudInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", ID: "uuid-1", Cloud: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !updated.Cloud || provider.toggledID != "uuid-1" || !provider.toggledCloud {
		t.Errorf("updated = %+v, provider state = id=%q cloud=%v, want toggled to true", updated, provider.toggledID, provider.toggledCloud)
	}
}

func TestImportArvanCloudDNSRecordsSuccess(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewImportArvanCloudDNSRecords(&inlineQueue{}, provider)

	zoneFile := []byte("www IN A 198.51.100.1\n")
	err := uc.Execute(context.Background(), app.ImportArvanCloudDNSRecordsInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", ZoneFile: zoneFile,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if string(provider.importedZoneFile) != string(zoneFile) {
		t.Errorf("provider received zone file %q, want %q", provider.importedZoneFile, zoneFile)
	}
}

func TestImportArvanCloudDNSRecordsRejectsEmptyFile(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewImportArvanCloudDNSRecords(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ImportArvanCloudDNSRecordsInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestExportArvanCloudDNSRecordsSuccess(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{exportContent: "www IN A 198.51.100.1\n"}
	uc := app.NewExportArvanCloudDNSRecords(&inlineQueue{}, provider)

	content, err := uc.Execute(context.Background(), app.ExportArvanCloudDNSRecordsInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if content != provider.exportContent {
		t.Errorf("content = %q, want %q", content, provider.exportContent)
	}
}

func TestGetArvanCloudDNSSecStatusSuccess(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{dnsSecStatus: domain.ArvanCloudDNSSecStatus{Enabled: true, DS: "example. IN DS ..."}}
	uc := app.NewGetArvanCloudDNSSecStatus(&inlineQueue{}, provider)

	status, err := uc.Execute(context.Background(), app.GetArvanCloudDNSSecStatusInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !status.Enabled || status.DS == "" {
		t.Errorf("status = %+v, want the fake provider's status", status)
	}
}

func TestUpdateArvanCloudDNSSecStatusSuccess(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewUpdateArvanCloudDNSSecStatus(&inlineQueue{}, provider)

	status, err := uc.Execute(context.Background(), app.UpdateArvanCloudDNSSecStatusInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", Enable: true, Rotate: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !status.Enabled {
		t.Errorf("status.Enabled = false, want true")
	}
}

func TestGetArvanCloudSecondaryDNSSuccess(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{secondaryDNSConfig: domain.ArvanCloudSecondaryDNSConfig{Status: true, Nameserver: "ns1.example.com"}}
	uc := app.NewGetArvanCloudSecondaryDNS(&inlineQueue{}, provider)

	cfg, err := uc.Execute(context.Background(), app.GetArvanCloudSecondaryDNSInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !cfg.Status || cfg.Nameserver != "ns1.example.com" {
		t.Errorf("cfg = %+v, want the fake provider's config", cfg)
	}
}

func TestSetArvanCloudSecondaryDNSSuccess(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewSetArvanCloudSecondaryDNS(&inlineQueue{}, provider)

	cfg, err := uc.Execute(context.Background(), app.SetArvanCloudSecondaryDNSInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com",
		Config: domain.ArvanCloudSecondaryDNSConfig{Status: true, Nameserver: "ns1.example.com"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !cfg.Status || cfg.Nameserver != "ns1.example.com" || provider.secondaryDNSConfig.Nameserver != "ns1.example.com" {
		t.Errorf("cfg = %+v, provider state = %+v, want the config propagated", cfg, provider.secondaryDNSConfig)
	}
}

func TestRemoveArvanCloudSecondaryDNSTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{removeErr: fmt.Errorf("secondary dns: %w", domain.ErrNotFound)}
	uc := app.NewRemoveArvanCloudSecondaryDNS(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.RemoveArvanCloudSecondaryDNSInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil: removing an already-gone config must be treated as done", err)
	}
	if provider.removedSecondary {
		t.Error("removedSecondary = true, but the provider returned ErrNotFound and should not have been treated as a real removal")
	}
}

func TestRemoveArvanCloudSecondaryDNSSuccess(t *testing.T) {
	provider := &fakeArvanCloudDNSProvider{}
	uc := app.NewRemoveArvanCloudSecondaryDNS(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.RemoveArvanCloudSecondaryDNSInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !provider.removedSecondary {
		t.Error("removedSecondary = false, want true")
	}
}
