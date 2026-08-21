package arvancloud

import (
	"context"
	"fmt"
	"net/http"

	"github.com/javadib/do0ps/internal/core/domain"
)

// SSL/TLS — domain-scoped settings plus uploaded/managed certificates (issue
// #73), wired to the real CDN API: a domain's SSL settings, the certificates
// attached to it, and the account-side workflow that drives an
// ArvanCloud-managed certificate to issuance. Base paths are confirmed
// against docs/api-specs/arvancloud-cdn-4.0.yml's "SSL/TLS" tag, relative to
// domainPath (defined in domain.go) — i.e.
// https://napi.arvancloud.ir/cdn/4.0/domains/{domain}/ssl/... .
//
// Deliberately NOT the account-scoped Certum certificate ordering workflow
// (issue #74/AC14) — see domain/arvancloud_ssl.go's package comment.
//
// private_key handling: CertificateStore.private_key (UploadArvanCloudCertificate)
// is caller-supplied sensitive material, treated as sensitive the same way
// domain.ProviderCredentials is. This adapter never logs a request body —
// the shared client's debug log (client.go's roundTrip) only ever logs the
// method, URL and redacted headers — and client.go's sensitiveResponseFields
// additionally redacts a "private_key" key wherever it might appear in a
// logged response or error body. See ssl_test.go's
// TestUploadArvanCloudCertificateNeverLogsPrivateKey for the guard.
const (
	sslPathSuffix              = "/ssl"
	sslCertificatesPathSuffix  = sslPathSuffix + "/certificates"
	sslOrdersPathSuffix        = sslPathSuffix + "/orders"
	sslIssuePathSuffix         = sslPathSuffix + "/issue"
	sslOrderRetryPathSuffix    = sslPathSuffix + "/orders/action/retry"
	sslCertificateRevokeSuffix = "/revoke"
)

func sslPath(domainName string) string { return domainPath(domainName) + sslPathSuffix }
func sslCertificatesPath(domainName string) string {
	return domainPath(domainName) + sslCertificatesPathSuffix
}
func sslCertificatePath(domainName, certificateID string) string {
	return sslCertificatesPath(domainName) + "/" + certificateID
}
func sslCertificateRevokePath(domainName, certificateID string) string {
	return sslCertificatePath(domainName, certificateID) + sslCertificateRevokeSuffix
}
func sslOrdersPath(domainName string) string { return domainPath(domainName) + sslOrdersPathSuffix }
func sslIssuePath(domainName string) string  { return domainPath(domainName) + sslIssuePathSuffix }
func sslOrderRetryPath(domainName string) string {
	return domainPath(domainName) + sslOrderRetryPathSuffix
}

// --- Wire types ------------------------------------------------------------

// certificateWire mirrors the Certificate schema. Decode-only: every field is
// readOnly on the wire.
type certificateWire struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Active      bool     `json:"active"`
	KeyType     string   `json:"key_type"`
	DomainNames []string `json:"domain_names"`
	Issuer      string   `json:"issuer"`
	IsRevoked   bool     `json:"is_revoked"`
	ExpiryDate  string   `json:"expiry_date"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

func toCertificateDomain(w certificateWire) domain.ArvanCloudCertificate {
	return domain.ArvanCloudCertificate{
		ID:          w.ID,
		Type:        domain.ArvanCloudCertificateType(w.Type),
		Active:      w.Active,
		KeyType:     domain.ArvanCloudCertificateKeyType(w.KeyType),
		DomainNames: w.DomainNames,
		Issuer:      w.Issuer,
		IsRevoked:   w.IsRevoked,
		ExpiryDate:  w.ExpiryDate,
		CreatedAt:   w.CreatedAt,
		UpdatedAt:   w.UpdatedAt,
	}
}

// sslProblemWire mirrors the SSLProblem schema. Decode-only.
type sslProblemWire struct {
	Type   string `json:"type"`
	Detail string `json:"detail"`
	Status string `json:"status"`
}

func toSSLProblemDomain(w sslProblemWire) domain.ArvanCloudSSLProblem {
	return domain.ArvanCloudSSLProblem{Type: w.Type, Detail: w.Detail, Status: w.Status}
}

// certificateOrderWire mirrors the CertificateOrder schema. Decode-only:
// every field is readOnly on the wire.
type certificateOrderWire struct {
	ID          string         `json:"id"`
	OrderID     int            `json:"order_id"`
	Status      string         `json:"status"`
	DomainNames []string       `json:"domain_names"`
	Errors      sslProblemWire `json:"errors"`
	ExpiryDate  string         `json:"expiry_date"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

func toCertificateOrderDomain(w certificateOrderWire) domain.ArvanCloudCertificateOrder {
	return domain.ArvanCloudCertificateOrder{
		ID:          w.ID,
		OrderID:     w.OrderID,
		Status:      domain.ArvanCloudCertificateOrderStatus(w.Status),
		DomainNames: w.DomainNames,
		Errors:      toSSLProblemDomain(w.Errors),
		ExpiryDate:  w.ExpiryDate,
		CreatedAt:   w.CreatedAt,
		UpdatedAt:   w.UpdatedAt,
	}
}

// sslWire mirrors the Ssl schema (GET response) and, for the fields it
// shares with SslUpdate, the shape a PATCH request builds. Decode-only for
// the same reason as ddosSettingsWire: a PATCH request is built separately
// (sslUpdateRequestBody) as a plain map, so an explicit `false` on a boolean
// toggle reaches the provider rather than being dropped by encoding/json's
// omitempty.
type sslWire struct {
	FingerprintStatus  bool                   `json:"fingerprint_status"`
	SSLStatus          bool                   `json:"ssl_status"`
	CertificateMode    string                 `json:"certificate_mode"`
	TLSVersion         string                 `json:"tls_version"`
	HSTSStatus         bool                   `json:"hsts_status"`
	QUICStatus         bool                   `json:"quic_status"`
	VerifySNI          bool                   `json:"verify_sni"`
	HSTSMaxAge         string                 `json:"hsts_max_age"`
	HSTSSubdomain      bool                   `json:"hsts_subdomain"`
	HSTSPreload        bool                   `json:"hsts_preload"`
	HTTPSRedirect      bool                   `json:"https_redirect"`
	ReplaceHTTP        bool                   `json:"replace_http"`
	CertificateKeyType string                 `json:"certificate_key_type"`
	Certificates       []certificateWire      `json:"certificates"`
	Orders             []certificateOrderWire `json:"orders"`
}

func toSslSettingsDomain(w sslWire) domain.ArvanCloudSslSettings {
	certs := make([]domain.ArvanCloudCertificate, len(w.Certificates))
	for i := range w.Certificates {
		certs[i] = toCertificateDomain(w.Certificates[i])
	}
	orders := make([]domain.ArvanCloudCertificateOrder, len(w.Orders))
	for i := range w.Orders {
		orders[i] = toCertificateOrderDomain(w.Orders[i])
	}
	return domain.ArvanCloudSslSettings{
		FingerprintStatus:  w.FingerprintStatus,
		SSLStatus:          w.SSLStatus,
		CertificateMode:    domain.ArvanCloudCertificateMode(w.CertificateMode),
		TLSVersion:         domain.ArvanCloudTlsVersion(w.TLSVersion),
		HSTSStatus:         w.HSTSStatus,
		HSTSMaxAge:         w.HSTSMaxAge,
		HSTSSubdomain:      w.HSTSSubdomain,
		HSTSPreload:        w.HSTSPreload,
		QUICStatus:         w.QUICStatus,
		VerifySNI:          w.VerifySNI,
		HTTPSRedirect:      w.HTTPSRedirect,
		ReplaceHTTP:        w.ReplaceHTTP,
		CertificateKeyType: domain.ArvanCloudCertificateKeyType(w.CertificateKeyType),
		Certificates:       certs,
		Orders:             orders,
	}
}

// sslUpdateRequestBody builds the JSON body for an SSL settings PATCH
// (ssl.update). certificate_mode, certificates and orders are readOnly on
// the wire (see domain.ArvanCloudSslSettings's own doc comment), so they are
// never part of a request body; certificate is sent only when the caller set
// it (Settings.Certificate's own doc comment: empty means "change nothing
// about which certificate is active").
func sslUpdateRequestBody(settings domain.ArvanCloudSslSettings) map[string]any {
	body := map[string]any{
		"fingerprint_status":   settings.FingerprintStatus,
		"ssl_status":           settings.SSLStatus,
		"tls_version":          string(settings.TLSVersion),
		"hsts_status":          settings.HSTSStatus,
		"hsts_subdomain":       settings.HSTSSubdomain,
		"hsts_preload":         settings.HSTSPreload,
		"quic_status":          settings.QUICStatus,
		"verify_sni":           settings.VerifySNI,
		"https_redirect":       settings.HTTPSRedirect,
		"replace_http":         settings.ReplaceHTTP,
		"certificate_key_type": string(settings.CertificateKeyType),
	}
	if settings.HSTSMaxAge != "" {
		body["hsts_max_age"] = settings.HSTSMaxAge
	}
	if settings.Certificate != "" {
		body["certificate"] = settings.Certificate
	}
	return body
}

// --- Per-domain SSL settings -------------------------------------------

// GetArvanCloudSslSettings returns domainName's SSL/TLS configuration.
func (p *Provider) GetArvanCloudSslSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudSslSettings, error) {
	var wire sslWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, sslPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud ssl settings for domain %q: %w", domainName, err)
	}
	settings := toSslSettingsDomain(wire)
	return &settings, nil
}

// UpdateArvanCloudSslSettings changes domainName's SSL/TLS configuration and
// returns it as stored afterward.
func (p *Provider) UpdateArvanCloudSslSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudSslSettings) (*domain.ArvanCloudSslSettings, error) {
	body := sslUpdateRequestBody(settings)
	var wire sslWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, sslPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud ssl settings for domain %q: %w", domainName, err)
	}
	updated := toSslSettingsDomain(wire)
	return &updated, nil
}

// --- Certificates ---------------------------------------------------------

// ListArvanCloudCertificates returns every certificate attached to
// domainName, unfiltered — matching ListArvanCloudDdosRules' own convention.
func (p *Provider) ListArvanCloudCertificates(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudCertificate, error) {
	var items []certificateWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, sslCertificatesPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud certificates of domain %q: %w", domainName, err)
	}
	certs := make([]domain.ArvanCloudCertificate, len(items))
	for i := range items {
		certs[i] = toCertificateDomain(items[i])
	}
	return certs, nil
}

// UploadArvanCloudCertificate stores a customer-owned certificate. See this
// file's package comment for how privateKeyPEM is kept out of logs.
func (p *Provider) UploadArvanCloudCertificate(ctx context.Context, creds domain.ProviderCredentials, domainName string, certificatePEM, privateKeyPEM []byte) error {
	if err := p.client.doCertificateUpload(ctx, creds, http.MethodPost, sslCertificatesPath(domainName), certificatePEM, privateKeyPEM, nil); err != nil {
		return fmt.Errorf("uploading arvancloud certificate for domain %q: %w", domainName, err)
	}
	return nil
}

// GetArvanCloudCertificate returns a single certificate by id.
func (p *Provider) GetArvanCloudCertificate(ctx context.Context, creds domain.ProviderCredentials, domainName, certificateID string) (*domain.ArvanCloudCertificate, error) {
	var wire certificateWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, sslCertificatePath(domainName, certificateID), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud certificate %q on domain %q: %w", certificateID, domainName, err)
	}
	found := toCertificateDomain(wire)
	return &found, nil
}

// DeleteArvanCloudCertificate removes an unused certificate by id.
func (p *Provider) DeleteArvanCloudCertificate(ctx context.Context, creds domain.ProviderCredentials, domainName, certificateID string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, sslCertificatePath(domainName, certificateID), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud certificate %q on domain %q: %w", certificateID, domainName, err)
	}
	return nil
}

// RevokeArvanCloudCertificate revokes a certificate for security reasons.
func (p *Provider) RevokeArvanCloudCertificate(ctx context.Context, creds domain.ProviderCredentials, domainName, certificateID string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodPost, sslCertificateRevokePath(domainName, certificateID), nil, nil); err != nil {
		return fmt.Errorf("revoking arvancloud certificate %q on domain %q: %w", certificateID, domainName, err)
	}
	return nil
}

// --- Managed-certificate orders --------------------------------------------

// ListArvanCloudSslOrders returns domainName's managed-certificate order
// history.
func (p *Provider) ListArvanCloudSslOrders(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudCertificateOrder, error) {
	var items []certificateOrderWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, sslOrdersPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud ssl orders of domain %q: %w", domainName, err)
	}
	orders := make([]domain.ArvanCloudCertificateOrder, len(items))
	for i := range items {
		orders[i] = toCertificateOrderDomain(items[i])
	}
	return orders, nil
}

// IssueArvanCloudManagedCertificate requests issuance of a managed
// certificate for domainName and returns the order it started. See
// ports.ArvanCloudProvider's own doc comment: this is a long operation, and
// the caller (app.IssueArvanCloudManagedCertificate) is responsible for not
// calling this a second time while an order is already in flight.
func (p *Provider) IssueArvanCloudManagedCertificate(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudCertificateOrder, error) {
	var wire certificateOrderWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, sslIssuePath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("issuing arvancloud managed certificate for domain %q: %w", domainName, err)
	}
	order := toCertificateOrderDomain(wire)
	return &order, nil
}

// RetryArvanCloudSslOrder retries a previously "killed" order.
func (p *Provider) RetryArvanCloudSslOrder(ctx context.Context, creds domain.ProviderCredentials, domainName string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodPost, sslOrderRetryPath(domainName), nil, nil); err != nil {
		return fmt.Errorf("retrying arvancloud ssl order for domain %q: %w", domainName, err)
	}
	return nil
}
