package domain

// The types below model ArvanCloud's domain-scoped SSL/TLS surface (issue
// #73): a domain's SSL settings plus the certificates (customer-uploaded or
// ArvanCloud-managed) attached to it. Confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "SSL/TLS" tag — the ssl.index,
// ssl.update, ssl.cert.* and ssl.cert.order.* operationIds under
// /domains/{domain}/ssl... — and the Ssl/SslUpdate/Certificate/
// CertificateStore/CertificateOrder/SSLProblem schemas.
//
// This is deliberately NOT the account-scoped Certum certificate ORDERING
// workflow (a distinct purchase/issuance product with its own state machine,
// tracked separately in issue #74/AC14) — the same base-concern split this
// project already made for Parspack (issue #18's SSL ordering workflow vs.
// the CDN API's own zone-level SSL settings). Nothing here should be unified
// with a future account-level Certum type.

// ArvanCloudCertificateMode is a domain's Ssl.certificate_mode: which
// certificate actually serves HTTPS traffic for the domain. Confirmed
// against the spec: a plain two-value string enum, and readOnly on the Ssl
// GET response — it is reported, never sent directly on update. It changes
// as a side effect of SslUpdate.certificate (see
// ArvanCloudSslSettings.Certificate's doc comment): sending "managed"
// switches the domain to ArvanCloud's own automatically issued/renewed
// certificate (ssl.cert.issue drives this), sending an uploaded certificate's
// UUID switches it to that customer-owned certificate (ssl.cert.store
// uploads it first).
type ArvanCloudCertificateMode string

const (
	// ArvanCloudCertificateModeManaged means ArvanCloud automatically issues
	// and renews the domain's certificate — the free/automatic path, driven
	// by IssueArvanCloudManagedCertificate (ssl.cert.issue).
	ArvanCloudCertificateModeManaged ArvanCloudCertificateMode = "managed"
	// ArvanCloudCertificateModeCustom means a certificate the caller uploaded
	// themselves (UploadArvanCloudCertificate, ssl.cert.store) serves the
	// domain.
	ArvanCloudCertificateModeCustom ArvanCloudCertificateMode = "custom"
)

var arvanCloudCertificateModes = []string{
	string(ArvanCloudCertificateModeManaged),
	string(ArvanCloudCertificateModeCustom),
}

// ValidArvanCloudCertificateMode reports whether s is one of
// Ssl.certificate_mode's two values.
func ValidArvanCloudCertificateMode(s string) bool { return contains(arvanCloudCertificateModes, s) }

// ArvanCloudTlsVersion is a domain's Ssl.tls_version: the minimum TLS
// protocol version accepted for the domain. Confirmed against the spec: a
// five-value string enum where the empty string is itself a valid value
// ("Empty (”) means default" — presumably no minimum enforced beyond
// whatever ArvanCloud's edge defaults to).
type ArvanCloudTlsVersion string

const (
	// ArvanCloudTlsVersionDefault leaves the minimum TLS version at
	// ArvanCloud's own default (the spec's empty-string value).
	ArvanCloudTlsVersionDefault ArvanCloudTlsVersion = ""
	ArvanCloudTlsVersion10      ArvanCloudTlsVersion = "TLSv1"
	ArvanCloudTlsVersion11      ArvanCloudTlsVersion = "TLSv1.1"
	ArvanCloudTlsVersion12      ArvanCloudTlsVersion = "TLSv1.2"
	ArvanCloudTlsVersion13      ArvanCloudTlsVersion = "TLSv1.3"
)

var arvanCloudTlsVersions = []string{
	string(ArvanCloudTlsVersionDefault),
	string(ArvanCloudTlsVersion10),
	string(ArvanCloudTlsVersion11),
	string(ArvanCloudTlsVersion12),
	string(ArvanCloudTlsVersion13),
}

// ValidArvanCloudTlsVersion reports whether s is one of Ssl.tls_version's
// five values, the empty string (the default) included.
func ValidArvanCloudTlsVersion(s string) bool { return contains(arvanCloudTlsVersions, s) }

// ArvanCloudCertificateKeyType is a domain's Ssl.certificate_key_type: the
// key algorithm ArvanCloud uses when it issues a managed certificate, and the
// algorithm reported for an already-attached certificate (Certificate.key_type).
// Confirmed against the spec: a plain two-value string enum.
type ArvanCloudCertificateKeyType string

const (
	ArvanCloudCertificateKeyTypeRSA ArvanCloudCertificateKeyType = "rsa"
	ArvanCloudCertificateKeyTypeEC  ArvanCloudCertificateKeyType = "ec"
)

var arvanCloudCertificateKeyTypes = []string{
	string(ArvanCloudCertificateKeyTypeRSA),
	string(ArvanCloudCertificateKeyTypeEC),
}

// ValidArvanCloudCertificateKeyType reports whether s is one of
// Ssl.certificate_key_type's two values.
func ValidArvanCloudCertificateKeyType(s string) bool {
	return contains(arvanCloudCertificateKeyTypes, s)
}

// arvanCloudHstsMaxAges is the exact set Ssl.hsts_max_age's spec enum
// declares — eight named durations, NOT every whole month from 1 to 24 (the
// spec skips 7-11mo and 13-23mo entirely).
var arvanCloudHstsMaxAges = []string{"1mo", "2mo", "3mo", "4mo", "5mo", "6mo", "12mo", "24mo"}

// ValidArvanCloudHstsMaxAge reports whether s is one of Ssl.hsts_max_age's
// eight values.
func ValidArvanCloudHstsMaxAge(s string) bool { return contains(arvanCloudHstsMaxAges, s) }

// ArvanCloudCertificateType is a Certificate.type: who the certificate
// attached to a domain belongs to / was issued by. Confirmed against the
// spec: a plain three-value string enum, readOnly (reported, never sent).
type ArvanCloudCertificateType string

const (
	// ArvanCloudCertificateTypeArvan is a certificate ArvanCloud itself
	// issued for the domain (the ArvanCloudCertificateModeManaged path).
	ArvanCloudCertificateTypeArvan ArvanCloudCertificateType = "arvan"
	// ArvanCloudCertificateTypeUser is a certificate the caller uploaded
	// themselves (UploadArvanCloudCertificate).
	ArvanCloudCertificateTypeUser ArvanCloudCertificateType = "user"
	// ArvanCloudCertificateTypeOrigin is a certificate used for the
	// connection between ArvanCloud's edge and the domain's origin server,
	// rather than the edge-to-visitor connection the other two types cover.
	ArvanCloudCertificateTypeOrigin ArvanCloudCertificateType = "origin"
)

// ArvanCloudCertificate is a certificate attached to a domain (the
// Certificate schema, returned by ListArvanCloudCertificates/
// GetArvanCloudCertificate and embedded in ArvanCloudSslSettings.Certificates).
// Every field the spec declares here is readOnly — this type is reported
// state, never a request body on its own; certificate/private_key material
// is submitted separately via UploadArvanCloudCertificate.
type ArvanCloudCertificate struct {
	// ID is the provider-assigned UUID, used as CertificateID on
	// GetArvanCloudCertificate/DeleteArvanCloudCertificate/
	// RevokeArvanCloudCertificate and as the value of
	// ArvanCloudSslSettings.Certificate to activate this certificate.
	ID string
	// Type reports whether ArvanCloud issued this certificate, the caller
	// uploaded it, or it is an origin-connection certificate.
	Type ArvanCloudCertificateType
	// Active reports whether this is the certificate currently serving the
	// domain's HTTPS traffic.
	Active bool
	// KeyType is the certificate's key algorithm.
	KeyType ArvanCloudCertificateKeyType
	// DomainNames are every hostname this certificate covers (e.g. a
	// wildcard plus its apex).
	DomainNames []string
	Issuer      string
	// IsRevoked reports whether the certificate has been revoked
	// (RevokeArvanCloudCertificate).
	IsRevoked  bool
	ExpiryDate string
	CreatedAt  string
	UpdatedAt  string
}

// ArvanCloudCertificateOrderStatus is a CertificateOrder.status: the
// nine-value state machine a managed-certificate issuance order moves
// through. Confirmed against the spec's own per-value description text
// (reproduced below per value). This is the confirmation that
// IssueArvanCloudManagedCertificate is a long operation (AGENTS.md 4.3):
// ssl.cert.issue starts an order in a non-terminal status, and DNS/HTTP
// validation ArvanCloud performs automatically drives it toward "valid" or a
// failure state over time — the same shape as Parspack's SSL ordering
// workflow (issue #18), just with ArvanCloud doing the challenge validation
// itself instead of exposing challenge tokens to the caller.
type ArvanCloudCertificateOrderStatus string

const (
	// ArvanCloudCertificateOrderStatusUnprocessed: order is in the process
	// queue.
	ArvanCloudCertificateOrderStatusUnprocessed ArvanCloudCertificateOrderStatus = "unprocessed"
	// ArvanCloudCertificateOrderStatusPending: authorization challenges are
	// set, ArvanCloud is validating them.
	ArvanCloudCertificateOrderStatusPending ArvanCloudCertificateOrderStatus = "pending"
	// ArvanCloudCertificateOrderStatusProcessing: challenges validated,
	// issuing the certificate.
	ArvanCloudCertificateOrderStatusProcessing ArvanCloudCertificateOrderStatus = "processing"
	// ArvanCloudCertificateOrderStatusReady: challenges validated, ready to
	// issue the certificate.
	ArvanCloudCertificateOrderStatusReady ArvanCloudCertificateOrderStatus = "ready"
	// ArvanCloudCertificateOrderStatusValid is the terminal SUCCESS state:
	// the certificate is issued.
	ArvanCloudCertificateOrderStatusValid ArvanCloudCertificateOrderStatus = "valid"
	// ArvanCloudCertificateOrderStatusInvalid: an error occurred, this order
	// cannot proceed; ArvanCloud automatically creates a new order.
	ArvanCloudCertificateOrderStatusInvalid ArvanCloudCertificateOrderStatus = "invalid"
	// ArvanCloudCertificateOrderStatusCanceled: canceled in favor of a new
	// order with updated subject names.
	ArvanCloudCertificateOrderStatusCanceled ArvanCloudCertificateOrderStatus = "canceled"
	// ArvanCloudCertificateOrderStatusTerminated: an unknown error occurred,
	// this order cannot proceed; ArvanCloud automatically creates a new
	// order.
	ArvanCloudCertificateOrderStatusTerminated ArvanCloudCertificateOrderStatus = "terminated"
	// ArvanCloudCertificateOrderStatusKilled is the terminal FAILURE state:
	// the order failed despite many retries and needs manual intervention —
	// RetryArvanCloudSslOrder (ssl.cert.order.retry) is the manual retry.
	ArvanCloudCertificateOrderStatusKilled ArvanCloudCertificateOrderStatus = "killed"
)

// ArvanCloudCertificateOrderTerminal reports whether status will not change
// on its own anymore: "valid" (success) or "killed" (failure needing
// RetryArvanCloudSslOrder). "invalid"/"terminated"/"canceled" are NOT
// terminal in that sense — the spec's own description says ArvanCloud
// automatically creates a replacement order for those, so a poller should
// keep watching ListArvanCloudSslOrders for the domain's newest order rather
// than stopping.
func ArvanCloudCertificateOrderTerminal(status ArvanCloudCertificateOrderStatus) bool {
	return status == ArvanCloudCertificateOrderStatusValid || status == ArvanCloudCertificateOrderStatusKilled
}

// ArvanCloudSSLProblem is an order's errors field (the SSLProblem schema): an
// ACME-style problem document ArvanCloud reports when an order cannot
// proceed.
type ArvanCloudSSLProblem struct {
	Type   string
	Detail string
	Status string
}

// ArvanCloudCertificateOrder is a managed-certificate issuance order (the
// CertificateOrder schema, returned by ListArvanCloudSslOrders and created by
// IssueArvanCloudManagedCertificate). Every field is readOnly on the wire —
// an order is reported state, never a request body.
type ArvanCloudCertificateOrder struct {
	// ID is the provider-assigned UUID.
	ID string
	// OrderID is a separate provider-assigned integer identifier reported
	// alongside ID (int32 on the wire; see this type's own adapter wire
	// struct for why it decodes as a plain int).
	OrderID int
	// Status is where the order stands in its issuance state machine. See
	// ArvanCloudCertificateOrderStatus's doc comment.
	Status ArvanCloudCertificateOrderStatus
	// DomainNames are the hostnames this order covers.
	DomainNames []string
	// Errors is populated when Status reports a failure; the zero value
	// (every field empty) otherwise.
	Errors     ArvanCloudSSLProblem
	ExpiryDate string
	CreatedAt  string
	UpdatedAt  string
}

// ArvanCloudSslSettings is a domain's SSL/TLS configuration (the Ssl schema
// for GetArvanCloudSslSettings, the SslUpdate schema for
// UpdateArvanCloudSslSettings — the two share every field below except
// Certificate, which exists only on the write side).
//
// CertificateMode, Certificates and Orders are readOnly on the wire: they are
// reported by GetArvanCloudSslSettings but never sent by
// UpdateArvanCloudSslSettings (see ssl.go's request-body builder). Which
// certificate actually serves the domain is instead changed indirectly, by
// setting Certificate on an update call.
type ArvanCloudSslSettings struct {
	// FingerprintStatus reports whether the domain is using ArvanCloud's
	// "fingerprint" feature.
	FingerprintStatus bool
	// SSLStatus is whether the domain's SSL module is enabled at all.
	SSLStatus bool
	// CertificateMode reports which certificate serves the domain: ArvanCloud's
	// own managed one, or a customer-uploaded one. Read-only — see this
	// type's own doc comment for how Certificate actually changes it.
	CertificateMode ArvanCloudCertificateMode
	// TLSVersion is the minimum TLS protocol version accepted for the
	// domain. Must be one of ValidArvanCloudTlsVersion's values.
	TLSVersion ArvanCloudTlsVersion
	// HSTSStatus enables HTTP Strict Transport Security for the domain.
	HSTSStatus bool
	// HSTSMaxAge is the HSTS max-age directive's duration. Must be one of
	// ValidArvanCloudHstsMaxAge's values; only meaningful when HSTSStatus is
	// true.
	HSTSMaxAge string
	// HSTSSubdomain adds the "includeSubDomains" directive to the HSTS
	// header.
	HSTSSubdomain bool
	// HSTSPreload adds the "preload" directive to the HSTS header.
	HSTSPreload bool
	// QUICStatus enables the QUIC transport protocol for the domain.
	QUICStatus bool
	// VerifySNI, when true, requires the TLS SNI hostname to match the
	// requested Host header.
	VerifySNI bool
	// HTTPSRedirect, when true, redirects HTTP requests to HTTPS.
	HTTPSRedirect bool
	// ReplaceHTTP, when true, rewrites "http://" to "https://" in the
	// domain's own HTML and JavaScript responses.
	ReplaceHTTP bool
	// CertificateKeyType is the key algorithm ArvanCloud uses when it issues
	// a managed certificate for the domain. Must be one of
	// ValidArvanCloudCertificateKeyType's values.
	CertificateKeyType ArvanCloudCertificateKeyType
	// Certificates lists every certificate attached to the domain. Read-only
	// — the same data ListArvanCloudCertificates returns.
	Certificates []ArvanCloudCertificate
	// Orders lists the domain's certificate orders since its last invalid or
	// canceled order. Read-only — a narrower view of what
	// ListArvanCloudSslOrders returns in full.
	Orders []ArvanCloudCertificateOrder

	// Certificate is WRITE-ONLY (SslUpdate.certificate): it selects which
	// certificate serves the domain, and is never populated by
	// GetArvanCloudSslSettings (use CertificateMode/Certificates to read the
	// active certificate back). Send the literal string "managed" to switch
	// to ArvanCloud's automatically issued/renewed certificate, or an
	// already-uploaded certificate's UUID (ArvanCloudCertificate.ID) to
	// switch to that customer-owned one. Leave empty to change nothing about
	// which certificate is active.
	Certificate string
}
