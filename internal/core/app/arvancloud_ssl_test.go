package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// This file exercises the FAST SSL/TLS use cases for ArvanCloud (issue #73):
// arvancloud_ssl.go. IssueArvanCloudManagedCertificate (the one long
// operation) is exercised in issue_arvancloud_managed_certificate_test.go.

func (p *fakeArvanCloudSSLProvider) GetArvanCloudSslSettings(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudSslSettings, error) {
	if p.settingsErr != nil {
		return nil, p.settingsErr
	}
	settings := p.settings
	return &settings, nil
}

func (p *fakeArvanCloudSSLProvider) UpdateArvanCloudSslSettings(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.ArvanCloudSslSettings) (*domain.ArvanCloudSslSettings, error) {
	if p.settingsErr != nil {
		return nil, p.settingsErr
	}
	p.updatedWith = settings
	return &settings, nil
}

func (p *fakeArvanCloudSSLProvider) ListArvanCloudCertificates(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudCertificate, error) {
	if p.certificatesErr != nil {
		return nil, p.certificatesErr
	}
	return p.certificates, nil
}

func (p *fakeArvanCloudSSLProvider) UploadArvanCloudCertificate(_ context.Context, _ domain.ProviderCredentials, _ string, certificatePEM, privateKeyPEM []byte) error {
	if p.uploadErr != nil {
		return p.uploadErr
	}
	p.uploadedCertificate = certificatePEM
	p.uploadedPrivateKey = privateKeyPEM
	return nil
}

func (p *fakeArvanCloudSSLProvider) GetArvanCloudCertificate(_ context.Context, _ domain.ProviderCredentials, _, certificateID string) (*domain.ArvanCloudCertificate, error) {
	for i := range p.certificates {
		if p.certificates[i].ID == certificateID {
			return &p.certificates[i], nil
		}
	}
	return nil, domain.ErrNotFound
}

func (p *fakeArvanCloudSSLProvider) DeleteArvanCloudCertificate(_ context.Context, _ domain.ProviderCredentials, _, certificateID string) error {
	if p.deleteCertErr != nil {
		return p.deleteCertErr
	}
	p.deletedCertificateID = certificateID
	return nil
}

func (p *fakeArvanCloudSSLProvider) RevokeArvanCloudCertificate(_ context.Context, _ domain.ProviderCredentials, _, certificateID string) error {
	if p.revokeErr != nil {
		return p.revokeErr
	}
	p.revokedCertificateID = certificateID
	return nil
}

func (p *fakeArvanCloudSSLProvider) RetryArvanCloudSslOrder(_ context.Context, _ domain.ProviderCredentials, domainName string) error {
	if p.retryErr != nil {
		return p.retryErr
	}
	p.retryCalledDomain = domainName
	return nil
}

func validArvanCloudSslSettings() domain.ArvanCloudSslSettings {
	return domain.ArvanCloudSslSettings{
		SSLStatus:          true,
		TLSVersion:         domain.ArvanCloudTlsVersion12,
		CertificateKeyType: domain.ArvanCloudCertificateKeyTypeRSA,
	}
}

// --- Per-domain SSL settings -------------------------------------------

func TestGetArvanCloudSslSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudSSLProvider{settings: domain.ArvanCloudSslSettings{
		SSLStatus: true, CertificateMode: domain.ArvanCloudCertificateModeManaged,
	}}
	uc := app.NewGetArvanCloudSslSettings(&inlineQueue{}, provider)

	got, err := uc.Execute(context.Background(), app.GetArvanCloudSslSettingsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !got.SSLStatus || got.CertificateMode != domain.ArvanCloudCertificateModeManaged {
		t.Errorf("got = %+v, want the fake's settings", got)
	}
}

func TestGetArvanCloudSslSettingsMissingDomain(t *testing.T) {
	uc := app.NewGetArvanCloudSslSettings(&inlineQueue{}, &fakeArvanCloudSSLProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudSslSettingsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want a validation error for the missing domain")
	}
}

func TestUpdateArvanCloudSslSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudSSLProvider{}
	uc := app.NewUpdateArvanCloudSslSettings(&inlineQueue{}, provider)

	settings := validArvanCloudSslSettings()
	settings.Certificate = "4e0de55d-96f5-471b-8ee5-f2667738320e"
	got, err := uc.Execute(context.Background(), app.UpdateArvanCloudSslSettingsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Settings: settings,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Certificate != settings.Certificate {
		t.Errorf("got.Certificate = %q, want %q", got.Certificate, settings.Certificate)
	}
	if provider.updatedWith.TLSVersion != domain.ArvanCloudTlsVersion12 {
		t.Errorf("provider received TLSVersion = %q, want TLSv1.2", provider.updatedWith.TLSVersion)
	}
}

func TestUpdateArvanCloudSslSettingsRejectsInvalidTLSVersion(t *testing.T) {
	uc := app.NewUpdateArvanCloudSslSettings(&inlineQueue{}, &fakeArvanCloudSSLProvider{})
	settings := validArvanCloudSslSettings()
	settings.TLSVersion = "TLSv2"
	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudSslSettingsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Settings: settings,
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want a validation error for the invalid tls_version")
	}
}

func TestUpdateArvanCloudSslSettingsRejectsInvalidHstsMaxAge(t *testing.T) {
	uc := app.NewUpdateArvanCloudSslSettings(&inlineQueue{}, &fakeArvanCloudSSLProvider{})
	settings := validArvanCloudSslSettings()
	settings.HSTSMaxAge = "7mo" // not one of the spec's eight named durations
	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudSslSettingsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Settings: settings,
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want a validation error for the invalid hsts_max_age")
	}
}

func TestUpdateArvanCloudSslSettingsRejectsInvalidCertificateKeyType(t *testing.T) {
	uc := app.NewUpdateArvanCloudSslSettings(&inlineQueue{}, &fakeArvanCloudSSLProvider{})
	settings := validArvanCloudSslSettings()
	settings.CertificateKeyType = "dsa"
	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudSslSettingsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", Settings: settings,
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want a validation error for the invalid certificate_key_type")
	}
}

// --- Certificates ----------------------------------------------------------

func TestListArvanCloudCertificatesSuccess(t *testing.T) {
	provider := &fakeArvanCloudSSLProvider{certificates: []domain.ArvanCloudCertificate{{ID: "cert-1"}, {ID: "cert-2"}}}
	uc := app.NewListArvanCloudCertificates(&inlineQueue{}, provider)

	certs, err := uc.Execute(context.Background(), app.ListArvanCloudCertificatesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(certs) != 2 {
		t.Errorf("len(certs) = %d, want 2", len(certs))
	}
}

// TestUploadArvanCloudCertificateSuccess proves the use case forwards
// certificate/private_key straight through to the provider unchanged.
func TestUploadArvanCloudCertificateSuccess(t *testing.T) {
	provider := &fakeArvanCloudSSLProvider{}
	uc := app.NewUploadArvanCloudCertificate(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.UploadArvanCloudCertificateInput{
		Credentials:    domain.ProviderCredentials{APIKey: "k"},
		Domain:         "example.com",
		CertificatePEM: []byte("cert-body"),
		PrivateKeyPEM:  []byte("key-body"),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if string(provider.uploadedCertificate) != "cert-body" || string(provider.uploadedPrivateKey) != "key-body" {
		t.Errorf("provider received certificate=%q private_key=%q, want cert-body/key-body",
			provider.uploadedCertificate, provider.uploadedPrivateKey)
	}
}

func TestUploadArvanCloudCertificateRejectsMissingPrivateKey(t *testing.T) {
	uc := app.NewUploadArvanCloudCertificate(&inlineQueue{}, &fakeArvanCloudSSLProvider{})
	err := uc.Execute(context.Background(), app.UploadArvanCloudCertificateInput{
		Credentials:    domain.ProviderCredentials{APIKey: "k"},
		Domain:         "example.com",
		CertificatePEM: []byte("cert-body"),
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want a validation error for the missing private_key")
	}
}

func TestUploadArvanCloudCertificateRejectsMissingCertificate(t *testing.T) {
	uc := app.NewUploadArvanCloudCertificate(&inlineQueue{}, &fakeArvanCloudSSLProvider{})
	err := uc.Execute(context.Background(), app.UploadArvanCloudCertificateInput{
		Credentials:   domain.ProviderCredentials{APIKey: "k"},
		Domain:        "example.com",
		PrivateKeyPEM: []byte("key-body"),
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want a validation error for the missing certificate")
	}
}

// TestUploadArvanCloudCertificateInputNeverLogsPrivateKey is the app-layer
// half of issue #73's redaction acceptance criterion: proves the private key
// never appears in any use-case-level error, mirroring the adapter-layer
// guard in ssl_test.go (TestUploadArvanCloudCertificateNeverLogsPrivateKey).
func TestUploadArvanCloudCertificateInputNeverLogsPrivateKey(t *testing.T) {
	const privateKey = "super-secret-key-material-should-never-leak"
	provider := &fakeArvanCloudSSLProvider{uploadErr: domain.ErrProviderUnavailable}
	uc := app.NewUploadArvanCloudCertificate(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.UploadArvanCloudCertificateInput{
		Credentials:    domain.ProviderCredentials{APIKey: "k"},
		Domain:         "example.com",
		CertificatePEM: []byte("cert-body"),
		PrivateKeyPEM:  []byte(privateKey),
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want the provider error to surface")
	}
	if strings.Contains(err.Error(), privateKey) {
		t.Errorf("error message contains the private key verbatim: %v", err)
	}
}

func TestGetArvanCloudCertificateNotFound(t *testing.T) {
	uc := app.NewGetArvanCloudCertificate(&inlineQueue{}, &fakeArvanCloudSSLProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", CertificateID: "missing",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want domain.ErrNotFound to surface")
	}
}

func TestDeleteArvanCloudCertificateTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudSSLProvider{deleteCertErr: domain.ErrNotFound}
	uc := app.NewDeleteArvanCloudCertificate(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteArvanCloudCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", CertificateID: "missing",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want a not-found delete to be tolerated", err)
	}
}

func TestRevokeArvanCloudCertificateSuccess(t *testing.T) {
	provider := &fakeArvanCloudSSLProvider{}
	uc := app.NewRevokeArvanCloudCertificate(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.RevokeArvanCloudCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com", CertificateID: "cert-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.revokedCertificateID != "cert-1" {
		t.Errorf("revokedCertificateID = %q, want cert-1", provider.revokedCertificateID)
	}
}

// --- Managed-certificate orders ---------------------------------------------

func TestListArvanCloudSslOrdersSuccess(t *testing.T) {
	provider := &fakeArvanCloudSSLProvider{orders: []domain.ArvanCloudCertificateOrder{
		{ID: "order-1", Status: domain.ArvanCloudCertificateOrderStatusKilled},
	}}
	uc := app.NewListArvanCloudSslOrders(&inlineQueue{}, provider)

	orders, err := uc.Execute(context.Background(), app.ListArvanCloudSslOrdersInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(orders) != 1 || orders[0].Status != domain.ArvanCloudCertificateOrderStatusKilled {
		t.Errorf("orders = %+v, want the one killed order", orders)
	}
}

func TestRetryArvanCloudSslOrderSuccess(t *testing.T) {
	provider := &fakeArvanCloudSSLProvider{}
	uc := app.NewRetryArvanCloudSslOrder(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.RetryArvanCloudSslOrderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, Domain: "example.com",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.retryCalledDomain != "example.com" {
		t.Errorf("retryCalledDomain = %q, want example.com", provider.retryCalledDomain)
	}
}

func TestRetryArvanCloudSslOrderMissingDomain(t *testing.T) {
	uc := app.NewRetryArvanCloudSslOrder(&inlineQueue{}, &fakeArvanCloudSSLProvider{})
	err := uc.Execute(context.Background(), app.RetryArvanCloudSslOrderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want a validation error for the missing domain")
	}
}
