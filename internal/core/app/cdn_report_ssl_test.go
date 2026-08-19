package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// cdnReportSSLFakeProvider is a package-local fake implementing only the
// methods the use cases in cdn_report_ssl.go call. The provider fields on
// those use cases are small local interfaces (not ports.ParspackProvider,
// which does not declare these methods yet — see cdn_report_ssl.go's
// package comment), so this fake only needs to satisfy those narrow
// interfaces, not the whole port.
type cdnReportSSLFakeProvider struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	accessLogPage          *domain.CDNAccessLogPage
	accessLogErr           error
	securityLogPage        *domain.CDNSecurityLogPage
	errorLogPage           *domain.CDNErrorLogPage
	wafLogPage             *domain.CDNWAFLogPage
	topVisitors            []domain.CDNTopVisitor
	trafficUsage           *domain.CDNTrafficUsage
	minTLSVersion          domain.CDNMinTLSVersion
	updatedMinTLS          domain.CDNMinTLSVersion
	updateMinTLSErr        error
	certificates           []domain.CDNCertificate
	hstsSettings           *domain.CDNHSTSSettings
	updatedHSTS            domain.CDNHSTSSettings
	updateHSTSErr          error
}

func (p *cdnReportSSLFakeProvider) GetCDNAccessLog(_ context.Context, _ domain.ProviderCredentials, _ string, _ domain.CDNLogQuery) (*domain.CDNAccessLogPage, error) {
	if p.accessLogErr != nil {
		return nil, p.accessLogErr
	}
	return p.accessLogPage, nil
}

func (p *cdnReportSSLFakeProvider) GetCDNSecurityLog(context.Context, domain.ProviderCredentials, string, domain.CDNLogQuery) (*domain.CDNSecurityLogPage, error) {
	return p.securityLogPage, nil
}

func (p *cdnReportSSLFakeProvider) GetCDNErrorLog(context.Context, domain.ProviderCredentials, string, domain.CDNLogQuery) (*domain.CDNErrorLogPage, error) {
	return p.errorLogPage, nil
}

func (p *cdnReportSSLFakeProvider) GetCDNWAFLog(context.Context, domain.ProviderCredentials, string, domain.CDNLogQuery) (*domain.CDNWAFLogPage, error) {
	return p.wafLogPage, nil
}

func (p *cdnReportSSLFakeProvider) GetCDNTopVisitors(_ context.Context, _ domain.ProviderCredentials, _, _, _ string) ([]domain.CDNTopVisitor, error) {
	return p.topVisitors, nil
}

func (p *cdnReportSSLFakeProvider) GetCDNMonthlyTrafficUsage(context.Context, domain.ProviderCredentials, string) (*domain.CDNTrafficUsage, error) {
	return p.trafficUsage, nil
}

func (p *cdnReportSSLFakeProvider) GetCDNMinTLSVersion(context.Context, domain.ProviderCredentials, string) (domain.CDNMinTLSVersion, error) {
	return p.minTLSVersion, nil
}

func (p *cdnReportSSLFakeProvider) UpdateCDNMinTLSVersion(_ context.Context, _ domain.ProviderCredentials, _ string, version domain.CDNMinTLSVersion) error {
	if p.updateMinTLSErr != nil {
		return p.updateMinTLSErr
	}
	p.updatedMinTLS = version
	return nil
}

func (p *cdnReportSSLFakeProvider) ListCDNCertificates(context.Context, domain.ProviderCredentials, string, int, int, string) ([]domain.CDNCertificate, error) {
	return p.certificates, nil
}

func (p *cdnReportSSLFakeProvider) GetCDNHSTS(context.Context, domain.ProviderCredentials, string) (*domain.CDNHSTSSettings, error) {
	return p.hstsSettings, nil
}

func (p *cdnReportSSLFakeProvider) UpdateCDNHSTS(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.CDNHSTSSettings) error {
	if p.updateHSTSErr != nil {
		return p.updateHSTSErr
	}
	p.updatedHSTS = settings
	return nil
}

func TestGetCDNAccessLogReturnsProviderResult(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{
		accessLogPage: &domain.CDNAccessLogPage{
			Records: []domain.CDNAccessLogEntry{{ID: "rec-1", StatusCode: 200}},
			Meta:    domain.CDNLogPageMeta{Total: 1},
		},
	}
	uc := app.NewGetCDNAccessLog(&inlineQueue{}, provider)

	page, err := uc.Execute(context.Background(), app.GetCDNAccessLogInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		Query:       domain.CDNLogQuery{Step: 25},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(page.Records) != 1 || page.Records[0].ID != "rec-1" {
		t.Errorf("records = %+v, want a single rec-1 entry", page.Records)
	}
}

func TestGetCDNAccessLogRequiresZoneUUID(t *testing.T) {
	uc := app.NewGetCDNAccessLog(&inlineQueue{}, &cdnReportSSLFakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNAccessLogInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNAccessLogRejectsInvalidStep(t *testing.T) {
	uc := app.NewGetCDNAccessLog(&inlineQueue{}, &cdnReportSSLFakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNAccessLogInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		Query:       domain.CDNLogQuery{Step: 7},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNSecurityLogReturnsProviderResult(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{
		securityLogPage: &domain.CDNSecurityLogPage{Records: []domain.CDNSecurityLogEntry{{SecurityType: "modsec_waf"}}},
	}
	uc := app.NewGetCDNSecurityLog(&inlineQueue{}, provider)

	page, err := uc.Execute(context.Background(), app.GetCDNSecurityLogInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(page.Records) != 1 || page.Records[0].SecurityType != "modsec_waf" {
		t.Errorf("records = %+v, want a single modsec_waf entry", page.Records)
	}
}

func TestGetCDNErrorLogReturnsProviderResult(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{
		errorLogPage: &domain.CDNErrorLogPage{Records: []domain.CDNErrorLogEntry{{ErrorType: "upstream"}}},
	}
	uc := app.NewGetCDNErrorLog(&inlineQueue{}, provider)

	page, err := uc.Execute(context.Background(), app.GetCDNErrorLogInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(page.Records) != 1 || page.Records[0].ErrorType != "upstream" {
		t.Errorf("records = %+v, want a single upstream entry", page.Records)
	}
}

func TestGetCDNWAFLogReturnsProviderResult(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{
		wafLogPage: &domain.CDNWAFLogPage{Records: []domain.CDNWAFLogEntry{{SecurityType: "modsec_waf", AdditionalLogs: []domain.CDNWAFLogDetail{{Message: "m"}}}}},
	}
	uc := app.NewGetCDNWAFLog(&inlineQueue{}, provider)

	page, err := uc.Execute(context.Background(), app.GetCDNWAFLogInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(page.Records) != 1 || len(page.Records[0].AdditionalLogs) != 1 {
		t.Errorf("records = %+v, want a single entry with one additional log", page.Records)
	}
}

func TestGetCDNTopVisitorsReturnsProviderResult(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{topVisitors: []domain.CDNTopVisitor{{IP: "1.2.3.4", Count: 10}}}
	uc := app.NewGetCDNTopVisitors(&inlineQueue{}, provider)

	visitors, err := uc.Execute(context.Background(), app.GetCDNTopVisitorsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		Start: "2024-01-01", End: "2024-01-31",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(visitors) != 1 || visitors[0].IP != "1.2.3.4" {
		t.Errorf("visitors = %+v, want a single 1.2.3.4 entry", visitors)
	}
}

func TestGetCDNTopVisitorsRequiresStartAndEnd(t *testing.T) {
	uc := app.NewGetCDNTopVisitors(&inlineQueue{}, &cdnReportSSLFakeProvider{})

	for name, in := range map[string]app.GetCDNTopVisitorsInput{
		"missing start": {Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", End: "2024-01-31"},
		"missing end":   {Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Start: "2024-01-01"},
	} {
		if _, err := uc.Execute(context.Background(), in); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: error = %v, want domain.ErrInvalidInput", name, err)
		}
	}
}

func TestGetCDNMonthlyTrafficUsageReturnsProviderResult(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{trafficUsage: &domain.CDNTrafficUsage{ReceivedBytes: 100, TrafficLimit: "200"}}
	uc := app.NewGetCDNMonthlyTrafficUsage(&inlineQueue{}, provider)

	usage, err := uc.Execute(context.Background(), app.GetCDNMonthlyTrafficUsageInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if usage.ReceivedBytes != 100 || usage.TrafficLimit != "200" {
		t.Errorf("usage = %+v, want receive 100 and limit 200", usage)
	}
}

func TestGetCDNMinTLSVersionReturnsProviderResult(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{minTLSVersion: domain.CDNMinTLSVersion12}
	uc := app.NewGetCDNMinTLSVersion(&inlineQueue{}, provider)

	version, err := uc.Execute(context.Background(), app.GetCDNMinTLSVersionInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if version != domain.CDNMinTLSVersion12 {
		t.Errorf("version = %q, want 1.2", version)
	}
}

func TestUpdateCDNMinTLSVersionAppliesValue(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{}
	uc := app.NewUpdateCDNMinTLSVersion(&inlineQueue{}, provider)

	version, err := uc.Execute(context.Background(), app.UpdateCDNMinTLSVersionInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		MinTLSVersion: domain.CDNMinTLSVersion13,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if version != domain.CDNMinTLSVersion13 || provider.updatedMinTLS != domain.CDNMinTLSVersion13 {
		t.Errorf("version = %q, provider.updatedMinTLS = %q, want both 1.3", version, provider.updatedMinTLS)
	}
}

func TestUpdateCDNMinTLSVersionRejectsInvalidVersion(t *testing.T) {
	uc := app.NewUpdateCDNMinTLSVersion(&inlineQueue{}, &cdnReportSSLFakeProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNMinTLSVersionInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		MinTLSVersion: "2.0",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestListCDNCertificatesReturnsProviderResult(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{certificates: []domain.CDNCertificate{{Domain: "example.com", Status: "ok"}}}
	uc := app.NewListCDNCertificates(&inlineQueue{}, provider)

	certs, err := uc.Execute(context.Background(), app.ListCDNCertificatesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(certs) != 1 || certs[0].Domain != "example.com" {
		t.Errorf("certs = %+v, want a single example.com certificate", certs)
	}
}

func TestListCDNCertificatesRequiresZoneUUID(t *testing.T) {
	uc := app.NewListCDNCertificates(&inlineQueue{}, &cdnReportSSLFakeProvider{})

	_, err := uc.Execute(context.Background(), app.ListCDNCertificatesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNHSTSReturnsProviderResult(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{hstsSettings: &domain.CDNHSTSSettings{Enabled: true, MaxAgeSeconds: 3600}}
	uc := app.NewGetCDNHSTS(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetCDNHSTSInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !settings.Enabled || settings.MaxAgeSeconds != 3600 {
		t.Errorf("settings = %+v, want enabled with max_age 3600", settings)
	}
}

func TestUpdateCDNHSTSAppliesSettings(t *testing.T) {
	provider := &cdnReportSSLFakeProvider{}
	uc := app.NewUpdateCDNHSTS(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.UpdateCDNHSTSInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		Settings: domain.CDNHSTSSettings{Enabled: true, MaxAgeSeconds: 7200},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !settings.Enabled || settings.MaxAgeSeconds != 7200 {
		t.Errorf("settings = %+v, want enabled with max_age 7200", settings)
	}
	if !provider.updatedHSTS.Enabled || provider.updatedHSTS.MaxAgeSeconds != 7200 {
		t.Errorf("provider.updatedHSTS = %+v, want enabled with max_age 7200", provider.updatedHSTS)
	}
}

func TestUpdateCDNHSTSRejectsOutOfRangeMaxAge(t *testing.T) {
	uc := app.NewUpdateCDNHSTS(&inlineQueue{}, &cdnReportSSLFakeProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNHSTSInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		Settings: domain.CDNHSTSSettings{Enabled: true, MaxAgeSeconds: 99999999},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
