package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// fakeArvanCloudObservabilityProvider embeds the port so a test only needs
// to override the methods it actually exercises, the same pattern
// fakeArvanCloudHealthCheckProvider uses.
type fakeArvanCloudObservabilityProvider struct {
	ports.ArvanCloudProvider

	forwarders []domain.ArvanCloudLogForwarder
	fwPage     domain.ArvanCloudReportPageMeta

	createdForwarder       domain.ArvanCloudLogForwarder
	createdForwarderDomain string
	updatedForwarderID     string
	updatedForwarder       domain.ArvanCloudLogForwarder
	deleteForwarderErr     error
	statusForwarderID      string
	statusForwarderValue   bool

	exporters []domain.ArvanCloudMetricExporter
	expPage   domain.ArvanCloudReportPageMeta
	metrics   *domain.ArvanCloudMetricExporterMetrics

	createdExporter       domain.ArvanCloudMetricExporter
	createdExporterDomain string
	updatedExporterID     string
	updatedExporter       domain.ArvanCloudMetricExporter
	deleteExporterErr     error
	statusExporterID      string
	statusExporterValue   bool
}

func (p *fakeArvanCloudObservabilityProvider) ListArvanCloudLogForwarders(context.Context, domain.ProviderCredentials, string, domain.ArvanCloudLogForwarderListQuery) ([]domain.ArvanCloudLogForwarder, domain.ArvanCloudReportPageMeta, error) {
	return p.forwarders, p.fwPage, nil
}

func (p *fakeArvanCloudObservabilityProvider) CreateArvanCloudLogForwarder(_ context.Context, _ domain.ProviderCredentials, domainName string, lf domain.ArvanCloudLogForwarder) (*domain.ArvanCloudLogForwarder, error) {
	p.createdForwarder = lf
	p.createdForwarderDomain = domainName
	created := lf
	created.ID = "lf-1"
	return &created, nil
}

func (p *fakeArvanCloudObservabilityProvider) GetArvanCloudLogForwarder(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudLogForwarder, error) {
	for i := range p.forwarders {
		if p.forwarders[i].ID == id {
			return &p.forwarders[i], nil
		}
	}
	return nil, errors.New("log forwarder not found")
}

func (p *fakeArvanCloudObservabilityProvider) UpdateArvanCloudLogForwarder(_ context.Context, _ domain.ProviderCredentials, _ string, id string, lf domain.ArvanCloudLogForwarder) (*domain.ArvanCloudLogForwarder, error) {
	p.updatedForwarderID = id
	p.updatedForwarder = lf
	updated := lf
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudObservabilityProvider) DeleteArvanCloudLogForwarder(context.Context, domain.ProviderCredentials, string, string) error {
	return p.deleteForwarderErr
}

func (p *fakeArvanCloudObservabilityProvider) SetArvanCloudLogForwarderStatus(_ context.Context, _ domain.ProviderCredentials, _ string, id string, status bool) (*domain.ArvanCloudLogForwarder, error) {
	p.statusForwarderID = id
	p.statusForwarderValue = status
	return &domain.ArvanCloudLogForwarder{ID: id, Status: status}, nil
}

func (p *fakeArvanCloudObservabilityProvider) ListArvanCloudMetricExporters(context.Context, domain.ProviderCredentials, domain.ArvanCloudMetricExporterListQuery) ([]domain.ArvanCloudMetricExporter, domain.ArvanCloudReportPageMeta, error) {
	return p.exporters, p.expPage, nil
}

func (p *fakeArvanCloudObservabilityProvider) ListArvanCloudMetricExporterTypes(context.Context, domain.ProviderCredentials) (*domain.ArvanCloudMetricExporterMetrics, error) {
	return p.metrics, nil
}

func (p *fakeArvanCloudObservabilityProvider) CreateArvanCloudMetricExporter(_ context.Context, _ domain.ProviderCredentials, domainName string, me domain.ArvanCloudMetricExporter) (*domain.ArvanCloudMetricExporter, error) {
	p.createdExporter = me
	p.createdExporterDomain = domainName
	created := me
	created.ID = "me-1"
	return &created, nil
}

func (p *fakeArvanCloudObservabilityProvider) GetArvanCloudMetricExporter(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudMetricExporter, error) {
	for i := range p.exporters {
		if p.exporters[i].ID == id {
			return &p.exporters[i], nil
		}
	}
	return nil, errors.New("metric exporter not found")
}

func (p *fakeArvanCloudObservabilityProvider) UpdateArvanCloudMetricExporter(_ context.Context, _ domain.ProviderCredentials, _ string, id string, me domain.ArvanCloudMetricExporter) (*domain.ArvanCloudMetricExporter, error) {
	p.updatedExporterID = id
	p.updatedExporter = me
	updated := me
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudObservabilityProvider) DeleteArvanCloudMetricExporter(context.Context, domain.ProviderCredentials, string, string) error {
	return p.deleteExporterErr
}

func (p *fakeArvanCloudObservabilityProvider) SetArvanCloudMetricExporterStatus(_ context.Context, _ domain.ProviderCredentials, _ string, id string, status bool) (*domain.ArvanCloudMetricExporter, error) {
	p.statusExporterID = id
	p.statusExporterValue = status
	return &domain.ArvanCloudMetricExporter{ID: id, Status: status}, nil
}

// validArvanCloudLogForwarder returns a minimal, valid S3-connected
// forwarder for tests that only care about validation surrounding a field
// other than Settings/ConnectionType.
func validArvanCloudLogForwarder() domain.ArvanCloudLogForwarder {
	return domain.ArvanCloudLogForwarder{
		Name: "s3 forwarder", Description: "ships access logs to s3",
		Type: domain.ArvanCloudLogForwarderTypeAccess, ConnectionType: domain.ArvanCloudConnectionTypeArvanS3,
		Settings: domain.ArvanCloudLogForwarderSettings{S3: &domain.ArvanCloudLogForwarderS3Settings{
			S3Endpoint: "s3.example.com", AccessKey: "AKIA", SecretKey: "secret", BucketName: "logs",
		}},
		Status: true,
	}
}

func TestCreateArvanCloudLogForwarderReturnsProviderCopy(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{}
	uc := app.NewCreateArvanCloudLogForwarder(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudLogForwarderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Forwarder: validArvanCloudLogForwarder(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "lf-1" {
		t.Errorf("created.ID = %q, want lf-1", created.ID)
	}
	if provider.createdForwarderDomain != "example.com" {
		t.Errorf("createdForwarderDomain = %q, want example.com", provider.createdForwarderDomain)
	}
}

func TestCreateArvanCloudLogForwarderValidation(t *testing.T) {
	uc := app.NewCreateArvanCloudLogForwarder(&inlineQueue{}, &fakeArvanCloudObservabilityProvider{})

	base := validArvanCloudLogForwarder()
	cases := map[string]app.CreateArvanCloudLogForwarderInput{
		"missing domain": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Forwarder: base},
		"missing name": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Forwarder: func() domain.ArvanCloudLogForwarder {
			f := base
			f.Name = ""
			return f
		}()},
		"missing description": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Forwarder: func() domain.ArvanCloudLogForwarder {
			f := base
			f.Description = ""
			return f
		}()},
		"invalid type": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Forwarder: func() domain.ArvanCloudLogForwarder {
			f := base
			f.Type = "bogus"
			return f
		}()},
		"invalid connection_type": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Forwarder: func() domain.ArvanCloudLogForwarder {
			f := base
			f.ConnectionType = "bogus"
			return f
		}()},
		"missing settings": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Forwarder: func() domain.ArvanCloudLogForwarder {
			f := base
			f.Settings = domain.ArvanCloudLogForwarderSettings{}
			return f
		}()},
		"settings mismatch with connection_type": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Forwarder: func() domain.ArvanCloudLogForwarder {
			f := base
			f.ConnectionType = domain.ArvanCloudConnectionTypeDatadog // Settings still carries S3
			return f
		}()},
		"incomplete s3 settings": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Forwarder: func() domain.ArvanCloudLogForwarder {
			f := base
			f.Settings = domain.ArvanCloudLogForwarderSettings{S3: &domain.ArvanCloudLogForwarderS3Settings{S3Endpoint: "s3.example.com"}}
			return f
		}()},
		"two settings branches populated": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Forwarder: func() domain.ArvanCloudLogForwarder {
			f := base
			f.Settings.Datadog = &domain.ArvanCloudLogForwarderDatadogSettings{URL: "https://example.com", APIKey: "k"}
			return f
		}()},
	}

	for name, in := range cases {
		if _, err := uc.Execute(context.Background(), in); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: error = %v, want domain.ErrInvalidInput", name, err)
		}
	}
}

func TestCreateArvanCloudLogForwarderAcceptsEveryConnectionType(t *testing.T) {
	cases := map[string]domain.ArvanCloudLogForwarderSettings{
		"arvan_s3":   {S3: &domain.ArvanCloudLogForwarderS3Settings{S3Endpoint: "e", AccessKey: "a", SecretKey: "s", BucketName: "b"}},
		"datadog":    {Datadog: &domain.ArvanCloudLogForwarderDatadogSettings{URL: "https://example.com", APIKey: "k"}},
		"kafka":      {Kafka: &domain.ArvanCloudLogForwarderKafkaSettings{KafkaBrokers: []string{"broker:9092"}, KafkaTopicToWrite: "topic"}},
		"loggly":     {Loggly: &domain.ArvanCloudLogForwarderLogglySettings{Token: "t", URL: "https://example.com"}},
		"syslog":     {Syslog: &domain.ArvanCloudLogForwarderSyslogSettings{LogType: domain.ArvanCloudLogForwarderSyslogUDP, Host: "syslog.example.com", Port: 514}},
		"arvan_logs": {ArvanLogs: &domain.ArvanCloudLogForwarderArvanLogsSettings{}},
	}
	connectionTypes := map[string]domain.ArvanCloudConnectionType{
		"arvan_s3": domain.ArvanCloudConnectionTypeArvanS3, "datadog": domain.ArvanCloudConnectionTypeDatadog,
		"kafka": domain.ArvanCloudConnectionTypeKafka, "loggly": domain.ArvanCloudConnectionTypeLoggly,
		"syslog": domain.ArvanCloudConnectionTypeSyslog, "arvan_logs": domain.ArvanCloudConnectionTypeArvanLogs,
	}

	for name, settings := range cases {
		uc := app.NewCreateArvanCloudLogForwarder(&inlineQueue{}, &fakeArvanCloudObservabilityProvider{})
		forwarder := domain.ArvanCloudLogForwarder{
			Name: "f", Description: "d", Type: domain.ArvanCloudLogForwarderTypeAccess,
			ConnectionType: connectionTypes[name], Settings: settings, Status: true,
		}
		if _, err := uc.Execute(context.Background(), app.CreateArvanCloudLogForwarderInput{
			Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Forwarder: forwarder,
		}); err != nil {
			t.Errorf("%s: Execute error = %v, want nil", name, err)
		}
	}
}

func TestGetArvanCloudLogForwarderUnknownID(t *testing.T) {
	uc := app.NewGetArvanCloudLogForwarder(&inlineQueue{}, &fakeArvanCloudObservabilityProvider{})

	_, err := uc.Execute(context.Background(), app.GetArvanCloudLogForwarderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", ID: "missing",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want the provider's not-found error")
	}
}

func TestUpdateArvanCloudLogForwarderCallsProvider(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{}
	uc := app.NewUpdateArvanCloudLogForwarder(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudLogForwarderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", ID: "lf-1", Forwarder: validArvanCloudLogForwarder(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.ID != "lf-1" || provider.updatedForwarderID != "lf-1" {
		t.Errorf("updated.ID/provider.updatedForwarderID = %q/%q, want lf-1/lf-1", updated.ID, provider.updatedForwarderID)
	}
}

func TestDeleteArvanCloudLogForwarderTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{deleteForwarderErr: errors.Join(errors.New("gone"), domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudLogForwarder(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteArvanCloudLogForwarderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", ID: "lf-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted forwarder", err)
	}
}

func TestSetArvanCloudLogForwarderStatusCallsProvider(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{}
	uc := app.NewSetArvanCloudLogForwarderStatus(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.SetArvanCloudLogForwarderStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", ID: "lf-1", Status: false,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.statusForwarderID != "lf-1" || provider.statusForwarderValue {
		t.Errorf("provider status call = (%q, %v), want (lf-1, false)", provider.statusForwarderID, provider.statusForwarderValue)
	}
	if updated.Status {
		t.Errorf("updated.Status = true, want false")
	}
}

func TestListArvanCloudLogForwardersRejectsInvalidTypeFilter(t *testing.T) {
	uc := app.NewListArvanCloudLogForwarders(&inlineQueue{}, &fakeArvanCloudObservabilityProvider{})

	_, err := uc.Execute(context.Background(), app.ListArvanCloudLogForwardersInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com",
		Query: domain.ArvanCloudLogForwarderListQuery{Types: []string{"bogus"}},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestListArvanCloudLogForwardersReturnsProviderResult(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{
		forwarders: []domain.ArvanCloudLogForwarder{{ID: "lf-1"}, {ID: "lf-2"}},
		fwPage:     domain.ArvanCloudReportPageMeta{Total: 2},
	}
	uc := app.NewListArvanCloudLogForwarders(&inlineQueue{}, provider)

	result, err := uc.Execute(context.Background(), app.ListArvanCloudLogForwardersInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Forwarders) != 2 || result.Page.Total != 2 {
		t.Errorf("result = %+v, want 2 forwarders and page.total 2", result)
	}
}

// --- Metric Exporters -------------------------------------------------------

func validArvanCloudMetricExporter() domain.ArvanCloudMetricExporter {
	return domain.ArvanCloudMetricExporter{
		Name: "exporter one", Type: domain.ArvanCloudMetricExporterTypeAccess,
		Interval: domain.ArvanCloudMetricExporterInterval30s, Status: true,
	}
}

func TestCreateArvanCloudMetricExporterReturnsProviderCopy(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{}
	uc := app.NewCreateArvanCloudMetricExporter(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudMetricExporterInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Exporter: validArvanCloudMetricExporter(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "me-1" || provider.createdExporterDomain != "example.com" {
		t.Errorf("created/domain = %+v/%q, want id me-1 and domain example.com", created, provider.createdExporterDomain)
	}
}

func TestCreateArvanCloudMetricExporterValidation(t *testing.T) {
	uc := app.NewCreateArvanCloudMetricExporter(&inlineQueue{}, &fakeArvanCloudObservabilityProvider{})

	base := validArvanCloudMetricExporter()
	cases := map[string]app.CreateArvanCloudMetricExporterInput{
		"missing domain": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Exporter: base},
		"missing name": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Exporter: func() domain.ArvanCloudMetricExporter {
			e := base
			e.Name = ""
			return e
		}()},
		"invalid type": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Exporter: func() domain.ArvanCloudMetricExporter {
			e := base
			e.Type = "waf" // narrower enum than LogForwarderType, no "waf" value
			return e
		}()},
		"invalid interval": {Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Exporter: func() domain.ArvanCloudMetricExporter {
			e := base
			e.Interval = "5s"
			return e
		}()},
	}

	for name, in := range cases {
		if _, err := uc.Execute(context.Background(), in); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: error = %v, want domain.ErrInvalidInput", name, err)
		}
	}
}

func TestGetArvanCloudMetricExporterUnknownID(t *testing.T) {
	uc := app.NewGetArvanCloudMetricExporter(&inlineQueue{}, &fakeArvanCloudObservabilityProvider{})

	_, err := uc.Execute(context.Background(), app.GetArvanCloudMetricExporterInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", ID: "missing",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want the provider's not-found error")
	}
}

func TestUpdateArvanCloudMetricExporterCallsProvider(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{}
	uc := app.NewUpdateArvanCloudMetricExporter(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudMetricExporterInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", ID: "me-1", Exporter: validArvanCloudMetricExporter(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.ID != "me-1" || provider.updatedExporterID != "me-1" {
		t.Errorf("updated.ID/provider.updatedExporterID = %q/%q, want me-1/me-1", updated.ID, provider.updatedExporterID)
	}
}

func TestDeleteArvanCloudMetricExporterTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{deleteExporterErr: errors.Join(errors.New("gone"), domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudMetricExporter(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteArvanCloudMetricExporterInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", ID: "me-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted exporter", err)
	}
}

func TestSetArvanCloudMetricExporterStatusCallsProvider(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{}
	uc := app.NewSetArvanCloudMetricExporterStatus(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.SetArvanCloudMetricExporterStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", ID: "me-1", Status: false,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.statusExporterID != "me-1" || provider.statusExporterValue {
		t.Errorf("provider status call = (%q, %v), want (me-1, false)", provider.statusExporterID, provider.statusExporterValue)
	}
	if updated.Status {
		t.Errorf("updated.Status = true, want false")
	}
}

func TestListArvanCloudMetricExportersRejectsInvalidTypeFilter(t *testing.T) {
	uc := app.NewListArvanCloudMetricExporters(&inlineQueue{}, &fakeArvanCloudObservabilityProvider{})

	_, err := uc.Execute(context.Background(), app.ListArvanCloudMetricExportersInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Query:       domain.ArvanCloudMetricExporterListQuery{Types: []string{"waf"}},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestListArvanCloudMetricExportersReturnsProviderResult(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{
		exporters: []domain.ArvanCloudMetricExporter{{ID: "me-1", Domain: "a.com"}, {ID: "me-2", Domain: "b.com"}},
		expPage:   domain.ArvanCloudReportPageMeta{Total: 2},
	}
	uc := app.NewListArvanCloudMetricExporters(&inlineQueue{}, provider)

	result, err := uc.Execute(context.Background(), app.ListArvanCloudMetricExportersInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Exporters) != 2 || result.Page.Total != 2 {
		t.Errorf("result = %+v, want 2 exporters and page.total 2", result)
	}
}

func TestListArvanCloudMetricExporterTypesReturnsProviderResult(t *testing.T) {
	provider := &fakeArvanCloudObservabilityProvider{
		metrics: &domain.ArvanCloudMetricExporterMetrics{
			Groups: []domain.ArvanCloudMetricExporterMetricGroup{
				{Metric: "access", Items: []domain.ArvanCloudMetricExporterMetricItem{{Name: "requests_total"}}},
			},
			Message: "ok",
		},
	}
	uc := app.NewListArvanCloudMetricExporterTypes(&inlineQueue{}, provider)

	metrics, err := uc.Execute(context.Background(), app.ListArvanCloudMetricExporterTypesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(metrics.Groups) != 1 || metrics.Groups[0].Metric != "access" || metrics.Message != "ok" {
		t.Errorf("metrics = %+v, want the provider's parsed catalog", metrics)
	}
}
