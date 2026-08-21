package arvancloud

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// --- Per-domain SSL settings ------------------------------------------------

// TestGetArvanCloudSslSettings pins the request shape and response parsing of
// GET /domains/{domain}/ssl.
func TestGetArvanCloudSslSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{
			"fingerprint_status":true,"ssl_status":true,"certificate_mode":"managed","tls_version":"TLSv1.2",
			"hsts_status":true,"hsts_max_age":"12mo","hsts_subdomain":true,"hsts_preload":false,
			"quic_status":true,"verify_sni":true,"https_redirect":true,"replace_http":false,
			"certificate_key_type":"ec",
			"certificates":[{"id":"cert-1","type":"arvan","active":true,"key_type":"ec","domain_names":["example.com"],"issuer":"ArvanCloud","is_revoked":false,"expiry_date":"2027-01-01T00:00:00Z"}],
			"orders":[{"id":"order-1","order_id":42,"status":"valid","domain_names":["example.com"],"expiry_date":"2027-01-01T00:00:00Z"}]
		}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings, err := provider.GetArvanCloudSslSettings(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudSslSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/ssl" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/ssl", records)
	}
	if settings.CertificateMode != domain.ArvanCloudCertificateModeManaged || settings.TLSVersion != domain.ArvanCloudTlsVersion12 {
		t.Errorf("settings = %+v, want the parsed certificate_mode/tls_version", settings)
	}
	if settings.HSTSMaxAge != "12mo" || !settings.HSTSStatus {
		t.Errorf("settings HSTS = %+v, want hsts_max_age 12mo and hsts_status true", settings)
	}
	if len(settings.Certificates) != 1 || settings.Certificates[0].ID != "cert-1" {
		t.Errorf("certificates = %+v, want one parsed certificate", settings.Certificates)
	}
	if len(settings.Orders) != 1 || settings.Orders[0].OrderID != 42 || settings.Orders[0].Status != domain.ArvanCloudCertificateOrderStatusValid {
		t.Errorf("orders = %+v, want one parsed order", settings.Orders)
	}
}

// TestUpdateArvanCloudSslSettings pins the request body of PATCH
// /domains/{domain}/ssl: certificate_mode/certificates/orders are never sent
// (readOnly), hsts_max_age is omitted when empty, and certificate is included
// only when the caller set it.
func TestUpdateArvanCloudSslSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"fingerprint_status":true,"ssl_status":true,"certificate_mode":"custom","tls_version":"TLSv1.3","certificate_key_type":"rsa"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings := domain.ArvanCloudSslSettings{
		FingerprintStatus:  true,
		SSLStatus:          true,
		TLSVersion:         domain.ArvanCloudTlsVersion13,
		HTTPSRedirect:      true,
		CertificateKeyType: domain.ArvanCloudCertificateKeyTypeRSA,
		Certificate:        "4e0de55d-96f5-471b-8ee5-f2667738320e",
	}
	updated, err := provider.UpdateArvanCloudSslSettings(context.Background(), creds(), "example.com", settings)
	if err != nil {
		t.Fatalf("UpdateArvanCloudSslSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/ssl" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/ssl", records)
	}
	body := string(records[0].body)
	if !strings.Contains(body, `"certificate":"4e0de55d-96f5-471b-8ee5-f2667738320e"`) {
		t.Errorf("request body = %s, want the certificate field sent", body)
	}
	if strings.Contains(body, "hsts_max_age") {
		t.Errorf("request body = %s, want hsts_max_age omitted when empty", body)
	}
	if strings.Contains(body, "certificate_mode") || strings.Contains(body, `"certificates"`) || strings.Contains(body, `"orders"`) {
		t.Errorf("request body = %s, want readOnly fields never sent", body)
	}
	if updated.CertificateMode != domain.ArvanCloudCertificateModeCustom {
		t.Errorf("updated.CertificateMode = %q, want custom", updated.CertificateMode)
	}
}

// --- Certificates ----------------------------------------------------------

// TestListArvanCloudCertificates pins the request shape and response parsing
// of GET /domains/{domain}/ssl/certificates.
func TestListArvanCloudCertificates(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"cert-1","type":"arvan","active":true,"key_type":"ec","domain_names":["example.com"],"issuer":"ArvanCloud","is_revoked":false},
			{"id":"cert-2","type":"user","active":false,"key_type":"rsa","domain_names":["www.example.com"],"issuer":"Let's Encrypt","is_revoked":false}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	certs, err := provider.ListArvanCloudCertificates(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudCertificates() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/ssl/certificates" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/ssl/certificates", records)
	}
	if len(certs) != 2 || certs[0].Type != domain.ArvanCloudCertificateTypeArvan || certs[1].Type != domain.ArvanCloudCertificateTypeUser {
		t.Errorf("certs = %+v, want the two parsed entries", certs)
	}
}

// TestUploadArvanCloudCertificate pins the multipart/form-data request of
// POST /domains/{domain}/ssl/certificates: two file parts named "certificate"
// and "private_key", exactly the CertificateStore schema.
func TestUploadArvanCloudCertificate(t *testing.T) {
	const certPEM = "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----"
	const keyPEM = "-----BEGIN PRIVATE KEY-----\nMIIE...\n-----END PRIVATE KEY-----"

	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"certificate uploaded"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.UploadArvanCloudCertificate(context.Background(), creds(), "example.com", []byte(certPEM), []byte(keyPEM)); err != nil {
		t.Fatalf("UploadArvanCloudCertificate() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/ssl/certificates" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/ssl/certificates", records)
	}
	body := string(records[0].body)
	if !strings.Contains(body, `name="certificate"`) || !strings.Contains(body, certPEM) {
		t.Errorf("request body missing the certificate file part:\n%s", body)
	}
	if !strings.Contains(body, `name="private_key"`) || !strings.Contains(body, keyPEM) {
		t.Errorf("request body missing the private_key file part:\n%s", body)
	}
}

// TestUploadArvanCloudCertificateNeverLogsPrivateKey guards the debug log:
// private_key must never appear verbatim, mirroring
// TestUpdateArvanCloudDdosSettingsNeverLogsSecretKey (ddos_test.go).
// UploadArvanCloudCertificate's own request is never logged in the first
// place (client.go's roundTrip only logs method/URL/redacted headers), so
// this test also proves that holds for a multipart body specifically, not
// only the JSON bodies the other capability adapters send.
func TestUploadArvanCloudCertificateNeverLogsPrivateKey(t *testing.T) {
	const privateKey = "-----BEGIN PRIVATE KEY-----\nsuper-secret-key-material-should-never-leak\n-----END PRIVATE KEY-----"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":"certificate uploaded"}`))
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := newTestClient(t, srv, WithLogger(logger))
	provider, err := NewProvider(client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if err := provider.UploadArvanCloudCertificate(context.Background(), creds(), "example.com", []byte("cert-body"), []byte(privateKey)); err != nil {
		t.Fatalf("UploadArvanCloudCertificate() error = %v", err)
	}

	if strings.Contains(logBuf.String(), privateKey) {
		t.Errorf("debug log contains the private key verbatim:\n%s", logBuf.String())
	}
}

// TestUploadArvanCloudCertificateErrorNeverContainsPrivateKey proves a failed
// upload's error message does not contain private_key either, even when the
// provider's error response happens to echo the submitted value back —
// mirroring TestUpdateArvanCloudDdosSettingsErrorNeverContainsSecretKey.
func TestUploadArvanCloudCertificateErrorNeverContainsPrivateKey(t *testing.T) {
	// No embedded newlines here (unlike the multipart-body test above): the
	// error server below writes this straight into a JSON string literal, and
	// a raw newline there would make the response invalid JSON, which would
	// make redactedResponseBody fall back to the unparsed body BEFORE
	// redaction even runs — defeating the very thing this test checks.
	const privateKey = "super-secret-key-material-should-never-leak"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		// An error response shape this adapter does not recognize, so
		// mapErrorResponse falls back to the raw body excerpt — the same path
		// TestUploadArvanCloudCertificateNeverLogsPrivateKey does not exercise.
		_, _ = w.Write([]byte(`{"submitted":{"private_key":"` + privateKey + `"}}`))
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := newTestClient(t, srv, WithLogger(logger))
	provider, err := NewProvider(client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	err = provider.UploadArvanCloudCertificate(context.Background(), creds(), "example.com", []byte("cert-body"), []byte(privateKey))
	if err == nil {
		t.Fatal("UploadArvanCloudCertificate() error = nil, want the 422 to surface")
	}

	if strings.Contains(err.Error(), privateKey) {
		t.Errorf("error message contains the private key verbatim: %v", err)
	}
	if strings.Contains(logBuf.String(), privateKey) {
		t.Errorf("debug log contains the private key verbatim:\n%s", logBuf.String())
	}
}

// TestGetArvanCloudCertificate pins the request shape and response parsing of
// GET /domains/{domain}/ssl/certificates/{certificateId}.
func TestGetArvanCloudCertificate(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"cert-1","type":"user","active":true,"key_type":"rsa","domain_names":["example.com"],"issuer":"Let's Encrypt","is_revoked":false}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	cert, err := provider.GetArvanCloudCertificate(context.Background(), creds(), "example.com", "cert-1")
	if err != nil {
		t.Fatalf("GetArvanCloudCertificate() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/ssl/certificates/cert-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/ssl/certificates/cert-1", records)
	}
	if cert.ID != "cert-1" || cert.KeyType != domain.ArvanCloudCertificateKeyTypeRSA {
		t.Errorf("cert = %+v, want the parsed certificate", cert)
	}
}

// TestDeleteArvanCloudCertificate pins the request shape of DELETE
// /domains/{domain}/ssl/certificates/{certificateId}.
func TestDeleteArvanCloudCertificate(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"certificate deleted"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudCertificate(context.Background(), creds(), "example.com", "cert-1"); err != nil {
		t.Fatalf("DeleteArvanCloudCertificate() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/ssl/certificates/cert-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/ssl/certificates/cert-1", records)
	}
}

// TestDeleteArvanCloudCertificateNotFound proves a 404 is reported as
// domain.ErrNotFound so callers can treat delete as idempotent.
func TestDeleteArvanCloudCertificateNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudCertificate(context.Background(), creds(), "example.com", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteArvanCloudCertificate() error = %v, want domain.ErrNotFound", err)
	}
}

// TestRevokeArvanCloudCertificate pins the request shape of POST
// /domains/{domain}/ssl/certificates/{certificateId}/revoke.
func TestRevokeArvanCloudCertificate(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"certificate revoked"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.RevokeArvanCloudCertificate(context.Background(), creds(), "example.com", "cert-1"); err != nil {
		t.Fatalf("RevokeArvanCloudCertificate() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/ssl/certificates/cert-1/revoke" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/ssl/certificates/cert-1/revoke", records)
	}
}

// --- Managed-certificate orders ---------------------------------------------

// TestListArvanCloudSslOrders pins the request shape and response parsing of
// GET /domains/{domain}/ssl/orders.
func TestListArvanCloudSslOrders(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"order-1","order_id":1,"status":"killed","domain_names":["example.com"],"errors":{"type":"urn:ietf:params:acme:error:malformed","detail":"bad request","status":"400"}},
			{"id":"order-2","order_id":2,"status":"valid","domain_names":["example.com"]}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	orders, err := provider.ListArvanCloudSslOrders(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudSslOrders() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/ssl/orders" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/ssl/orders", records)
	}
	if len(orders) != 2 || orders[0].Status != domain.ArvanCloudCertificateOrderStatusKilled || orders[0].Errors.Detail != "bad request" {
		t.Errorf("orders = %+v, want the two parsed entries with errors decoded", orders)
	}
	if orders[1].Status != domain.ArvanCloudCertificateOrderStatusValid {
		t.Errorf("orders[1].Status = %q, want valid", orders[1].Status)
	}
}

// TestIssueArvanCloudManagedCertificate pins the request shape and response
// parsing of POST /domains/{domain}/ssl/issue.
func TestIssueArvanCloudManagedCertificate(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"order-1","order_id":7,"status":"unprocessed","domain_names":["example.com"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	order, err := provider.IssueArvanCloudManagedCertificate(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("IssueArvanCloudManagedCertificate() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/ssl/issue" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/ssl/issue", records)
	}
	if order.Status != domain.ArvanCloudCertificateOrderStatusUnprocessed || order.OrderID != 7 {
		t.Errorf("order = %+v, want the parsed order in a non-terminal status", order)
	}
}

// TestRetryArvanCloudSslOrder pins the request shape of POST
// /domains/{domain}/ssl/orders/action/retry.
func TestRetryArvanCloudSslOrder(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"retry order placed"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.RetryArvanCloudSslOrder(context.Background(), creds(), "example.com"); err != nil {
		t.Fatalf("RetryArvanCloudSslOrder() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/ssl/orders/action/retry" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/ssl/orders/action/retry", records)
	}
}
