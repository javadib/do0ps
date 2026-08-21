package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// fakeArvanCloudHealthCheckProvider embeds the port so a test only needs to
// override the methods it actually exercises, the same pattern
// fakeArvanCloudRateLimitProvider uses.
type fakeArvanCloudHealthCheckProvider struct {
	ports.ArvanCloudProvider

	checks []domain.ArvanCloudHealthCheck

	createdCheck       domain.ArvanCloudHealthCheck
	createdCheckDomain string

	updatedCheckID string
	updatedCheck   domain.ArvanCloudHealthCheck

	deleteCheckErr error

	domainZones []domain.ArvanCloudHealthCheckZoneName
	globalZones []domain.ArvanCloudHealthCheckZoneName

	summary []domain.ArvanCloudHealthCheckReportSummary
	details []domain.ArvanCloudHealthCheckReportDetail
	page    domain.ArvanCloudHealthCheckReportPageMeta
}

func (p *fakeArvanCloudHealthCheckProvider) ListArvanCloudHealthChecks(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudHealthCheck, error) {
	return p.checks, nil
}

func (p *fakeArvanCloudHealthCheckProvider) CreateArvanCloudHealthCheck(_ context.Context, _ domain.ProviderCredentials, domainName string, hc domain.ArvanCloudHealthCheck) (*domain.ArvanCloudHealthCheck, error) {
	p.createdCheck = hc
	p.createdCheckDomain = domainName
	created := hc
	created.ID = "hc-1"
	return &created, nil
}

func (p *fakeArvanCloudHealthCheckProvider) GetArvanCloudHealthCheck(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudHealthCheck, error) {
	for i := range p.checks {
		if p.checks[i].ID == id {
			return &p.checks[i], nil
		}
	}
	return nil, errors.New("health check not found")
}

func (p *fakeArvanCloudHealthCheckProvider) UpdateArvanCloudHealthCheck(_ context.Context, _ domain.ProviderCredentials, _ string, id string, hc domain.ArvanCloudHealthCheck) (*domain.ArvanCloudHealthCheck, error) {
	p.updatedCheckID = id
	p.updatedCheck = hc
	updated := hc
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudHealthCheckProvider) DeleteArvanCloudHealthCheck(_ context.Context, _ domain.ProviderCredentials, _ string, _ string) error {
	return p.deleteCheckErr
}

func (p *fakeArvanCloudHealthCheckProvider) ListArvanCloudDomainHealthCheckZones(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudHealthCheckZoneName, error) {
	return p.domainZones, nil
}

func (p *fakeArvanCloudHealthCheckProvider) ListArvanCloudHealthCheckZones(context.Context, domain.ProviderCredentials) ([]domain.ArvanCloudHealthCheckZoneName, error) {
	return p.globalZones, nil
}

func (p *fakeArvanCloudHealthCheckProvider) GetArvanCloudHealthCheckSummary(context.Context, domain.ProviderCredentials, string, domain.ArvanCloudHealthCheckReportQuery) ([]domain.ArvanCloudHealthCheckReportSummary, error) {
	return p.summary, nil
}

func (p *fakeArvanCloudHealthCheckProvider) GetArvanCloudHealthCheckDetails(context.Context, domain.ProviderCredentials, string, domain.ArvanCloudHealthCheckReportQuery) ([]domain.ArvanCloudHealthCheckReportDetail, domain.ArvanCloudHealthCheckReportPageMeta, error) {
	return p.details, p.page, nil
}

// validArvanCloudHealthCheck returns a minimal, valid TCP check for tests
// that only care about validation surrounding a field other than
// RequestConfig.
func validArvanCloudHealthCheck() domain.ArvanCloudHealthCheck {
	return domain.ArvanCloudHealthCheck{
		Name: "db-check", Origin: "pool-1", OriginType: domain.ArvanCloudHealthCheckOriginTypePool,
		IntervalMS: 30000, Threshold: 3, Type: domain.ArvanCloudHealthCheckTCP,
		RequestConfig: domain.ArvanCloudHealthCheckRequestConfig{
			TCP: &domain.ArvanCloudHealthCheckTCPConfig{Port: 5432, TimeoutMS: 3000},
		},
	}
}

func TestCreateArvanCloudHealthCheckSuccess(t *testing.T) {
	provider := &fakeArvanCloudHealthCheckProvider{}
	uc := app.NewCreateArvanCloudHealthCheck(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudHealthCheckInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Check: validArvanCloudHealthCheck(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "hc-1" || provider.createdCheckDomain != "example.com" {
		t.Errorf("created = %+v, provider.createdCheckDomain = %q, want id=hc-1 domain=example.com", created, provider.createdCheckDomain)
	}
}

// TestCreateArvanCloudHealthCheckRejectsBadInterval proves interval_ms is
// validated against the millisecond enum, rejecting a plausible-looking
// value that is actually seconds (30) or wildly too large (30000000) — the
// issue's own acceptance-criteria example.
func TestCreateArvanCloudHealthCheckRejectsBadInterval(t *testing.T) {
	for _, ms := range []int{30, 30000000, 45000} {
		check := validArvanCloudHealthCheck()
		check.IntervalMS = ms
		uc := app.NewCreateArvanCloudHealthCheck(&inlineQueue{}, &fakeArvanCloudHealthCheckProvider{})
		_, err := uc.Execute(context.Background(), app.CreateArvanCloudHealthCheckInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com", Check: check,
		})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("Execute() with interval_ms=%d error = %v, want domain.ErrInvalidInput", ms, err)
		}
	}
}

// TestCreateArvanCloudHealthCheckAcceptsEveryValidInterval proves the three
// declared enum values are all accepted.
func TestCreateArvanCloudHealthCheckAcceptsEveryValidInterval(t *testing.T) {
	for _, ms := range []int{30000, 60000, 120000} {
		check := validArvanCloudHealthCheck()
		check.IntervalMS = ms
		uc := app.NewCreateArvanCloudHealthCheck(&inlineQueue{}, &fakeArvanCloudHealthCheckProvider{})
		if _, err := uc.Execute(context.Background(), app.CreateArvanCloudHealthCheckInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com", Check: check,
		}); err != nil {
			t.Errorf("Execute() with interval_ms=%d error = %v, want success", ms, err)
		}
	}
}

// TestCreateArvanCloudHealthCheckRejectsMismatchedRequestConfig proves a TCP
// check with an HTTP request_config (or vice versa) is rejected rather than
// silently sent as-is.
func TestCreateArvanCloudHealthCheckRejectsMismatchedRequestConfig(t *testing.T) {
	check := validArvanCloudHealthCheck()
	check.RequestConfig = domain.ArvanCloudHealthCheckRequestConfig{
		HTTP: &domain.ArvanCloudHealthCheckHTTPConfig{
			Method: domain.ArvanCloudHealthCheckHTTPMethodGet, Port: 80, Path: "/", TimeoutMS: 1000,
		},
	}
	uc := app.NewCreateArvanCloudHealthCheck(&inlineQueue{}, &fakeArvanCloudHealthCheckProvider{})
	_, err := uc.Execute(context.Background(), app.CreateArvanCloudHealthCheckInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Check: check,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput for a TCP check carrying an HTTP request_config", err)
	}
}

// TestCreateArvanCloudHealthCheckRejectsMissingRequestConfig proves a check
// whose Type selects a branch that is left nil is rejected.
func TestCreateArvanCloudHealthCheckRejectsMissingRequestConfig(t *testing.T) {
	check := validArvanCloudHealthCheck()
	check.RequestConfig = domain.ArvanCloudHealthCheckRequestConfig{}
	uc := app.NewCreateArvanCloudHealthCheck(&inlineQueue{}, &fakeArvanCloudHealthCheckProvider{})
	_, err := uc.Execute(context.Background(), app.CreateArvanCloudHealthCheckInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Check: check,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput for a missing request_config", err)
	}
}

// TestCreateArvanCloudHealthCheckRejectsBadHTTPMethod proves
// request_config.http.method is validated against its enum.
func TestCreateArvanCloudHealthCheckRejectsBadHTTPMethod(t *testing.T) {
	check := validArvanCloudHealthCheck()
	check.Type = domain.ArvanCloudHealthCheckHTTP
	check.RequestConfig = domain.ArvanCloudHealthCheckRequestConfig{
		HTTP: &domain.ArvanCloudHealthCheckHTTPConfig{
			Method: "TRACE", Port: 80, Path: "/", TimeoutMS: 1000,
		},
	}
	uc := app.NewCreateArvanCloudHealthCheck(&inlineQueue{}, &fakeArvanCloudHealthCheckProvider{})
	_, err := uc.Execute(context.Background(), app.CreateArvanCloudHealthCheckInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Check: check,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput for method=TRACE", err)
	}
}

func TestGetArvanCloudHealthCheckMissingID(t *testing.T) {
	uc := app.NewGetArvanCloudHealthCheck(&inlineQueue{}, &fakeArvanCloudHealthCheckProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudHealthCheckInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// TestDeleteArvanCloudHealthCheckToleratesNotFound proves a provider-side
// ErrNotFound is swallowed rather than surfaced, matching
// DeleteArvanCloudRateLimitRule's own tolerant-delete contract.
func TestDeleteArvanCloudHealthCheckToleratesNotFound(t *testing.T) {
	provider := &fakeArvanCloudHealthCheckProvider{deleteCheckErr: domain.ErrNotFound}
	uc := app.NewDeleteArvanCloudHealthCheck(&inlineQueue{}, provider)
	err := uc.Execute(context.Background(), app.DeleteArvanCloudHealthCheckInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "missing",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil (tolerant delete)", err)
	}
}

func TestListArvanCloudDomainHealthCheckZonesSuccess(t *testing.T) {
	provider := &fakeArvanCloudHealthCheckProvider{domainZones: []domain.ArvanCloudHealthCheckZoneName{{ID: "fra", Name: "Frankfurt"}}}
	uc := app.NewListArvanCloudDomainHealthCheckZones(&inlineQueue{}, provider)
	zones, err := uc.Execute(context.Background(), app.ListArvanCloudDomainHealthCheckZonesInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(zones) != 1 || zones[0].ID != "fra" {
		t.Errorf("zones = %+v, want one zone", zones)
	}
}

func TestListArvanCloudHealthCheckZonesSuccess(t *testing.T) {
	provider := &fakeArvanCloudHealthCheckProvider{globalZones: []domain.ArvanCloudHealthCheckZoneName{{ID: "fra", Name: "Frankfurt"}}}
	uc := app.NewListArvanCloudHealthCheckZones(&inlineQueue{}, provider)
	zones, err := uc.Execute(context.Background(), app.ListArvanCloudHealthCheckZonesInput{Credentials: validArvanCloudCreds()})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(zones) != 1 {
		t.Errorf("zones = %+v, want one zone", zones)
	}
}

func TestGetArvanCloudHealthCheckSummaryMissingUpstream(t *testing.T) {
	uc := app.NewGetArvanCloudHealthCheckSummary(&inlineQueue{}, &fakeArvanCloudHealthCheckProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudHealthCheckSummaryInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Query: domain.ArvanCloudHealthCheckReportQuery{Name: "db-check"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput for a missing upstream", err)
	}
}

func TestGetArvanCloudHealthCheckSummarySuccess(t *testing.T) {
	provider := &fakeArvanCloudHealthCheckProvider{summary: []domain.ArvanCloudHealthCheckReportSummary{{Zone: "fra", Status: true}}}
	uc := app.NewGetArvanCloudHealthCheckSummary(&inlineQueue{}, provider)
	report, err := uc.Execute(context.Background(), app.GetArvanCloudHealthCheckSummaryInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Query: domain.ArvanCloudHealthCheckReportQuery{Name: "db-check", Upstream: "1.1.1.1"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(report) != 1 || report[0].Zone != "fra" {
		t.Errorf("report = %+v, want one zone summary", report)
	}
}

// TestGetArvanCloudHealthCheckDetailsInvalidType proves the details-only
// "type" filter is validated against its enum.
func TestGetArvanCloudHealthCheckDetailsInvalidType(t *testing.T) {
	uc := app.NewGetArvanCloudHealthCheckDetails(&inlineQueue{}, &fakeArvanCloudHealthCheckProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudHealthCheckDetailsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Query: domain.ArvanCloudHealthCheckReportQuery{Name: "db-check", Upstream: "1.1.1.1", Type: "bogus"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput for type=bogus", err)
	}
}

func TestGetArvanCloudHealthCheckDetailsSuccess(t *testing.T) {
	provider := &fakeArvanCloudHealthCheckProvider{
		details: []domain.ArvanCloudHealthCheckReportDetail{{Zone: "fra", Status: false, Message: "timeout"}},
		page:    domain.ArvanCloudHealthCheckReportPageMeta{Total: 1, LastPage: 1},
	}
	uc := app.NewGetArvanCloudHealthCheckDetails(&inlineQueue{}, provider)
	result, err := uc.Execute(context.Background(), app.GetArvanCloudHealthCheckDetailsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Query: domain.ArvanCloudHealthCheckReportQuery{Name: "db-check", Upstream: "1.1.1.1"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Details) != 1 || result.Details[0].Message != "timeout" || result.Page.Total != 1 {
		t.Errorf("result = %+v, want one detail and page.total=1", result)
	}
}
