package domain

// The types below model the Parspack SSL certificate ordering workflow
// (AGENTS.md 4.5's SSL surface, https://my.parspack.com/sslv2). Unlike VM
// lifecycle, this is a genuine multi-step order: create an order, submit a
// CSR plus contact details, complete a domain-ownership challenge, then
// download the issued certificate. Field shapes are confirmed against
// docs/api-specs/parspack-ssl.openapi.yaml (committed, authoritative per AGENTS.md
// 4.5), not derived from the JS-rendered docs site.

// SSLPrice is one billing-cycle price of an SSLProduct.
type SSLPrice struct {
	Cycle    string
	Price    float64
	SetupFee float64
	Currency string
}

// SSLProduct is an SSL certificate product available for order (DV, OV, EV,
// wildcard, multi-domain, ...). Callers pass Slug as ProductSlug on
// SSLOrderSpec.
type SSLProduct struct {
	Slug         string
	Title        string
	Description  string
	ProductGroup string // "dv", "ov", "ev", ...
	Wildcard     bool
	MultiDomain  bool
	Available    bool
	Prices       map[string]SSLPrice // keyed by billing cycle: annually, biennially, triennially
}

// SSLSAN is one additional Subject Alternative Name on a multi-domain order.
type SSLSAN struct {
	Name string
	IsIP bool
	WWW  bool
}

// SSLOrderSpec is the normalized request for a new SSL order.
type SSLOrderSpec struct {
	ProductSlug  string
	Domain       string
	BillingCycle string // annually, biennially, or triennially
	WWW          bool
	PromoCode    string
	SANs         []SSLSAN
}

// SSLOrder confirms a newly created SSL order. The order is not active yet:
// InvoiceStatus starts "unpaid", and ProcessSSLOrder cannot run until it is
// paid.
type SSLOrder struct {
	ID            string
	InvoiceID     string
	InvoiceStatus string // "unpaid", "paid", "cancelled", or "refunded"
}

// SSLContact is the certificate applicant's contact and organization
// information, submitted together with the CSR when processing an order.
// Country is an ISO 3166-1 alpha-2 code (AGENTS.md 4.5's confirmed process
// payload). JurisdictionCountry, BusinessCategory and RegistrationNumber are
// only meaningful for OV/EV products.
type SSLContact struct {
	FirstName  string
	LastName   string
	Country    string
	City       string
	Address    string
	Phone      string
	Email      string
	PostalCode string

	JurisdictionCountry string
	BusinessCategory    string
	RegistrationNumber  string
}

// SSLChallenge is one domain-ownership verification token for one domain
// under one method.
type SSLChallenge struct {
	ID       int
	OrderID  string
	Domain   string
	Token    string // the value to publish (DNS record content, file content, ...)
	Verified bool
	Type     string // matches the parent SSLChallengeMethod.Method
	FileName string // FILE method only
	FilePath string // FILE method only: where to publish the file
}

// SSLChallengeMethod groups the challenges available for one verification
// method (e.g. all DNS_TXT challenges across the order's domain and SANs).
type SSLChallengeMethod struct {
	Method     string
	Name       string
	Challenges []SSLChallenge
}

// SSLAllowedMethod is one verification method the caller may choose or
// switch to via ReloadSSLChallenge.
type SSLAllowedMethod struct {
	Method   string
	Name     string
	Prefixes []string // ADMIN method only: valid email prefixes (ADMIN, POSTMASTER, ...)
}

// SSLChallengeSet is the caller-facing view returned by processing an order,
// showing its challenges, or reloading them with a different method.
type SSLChallengeSet struct {
	// Methods is keyed by method name (DNS_TXT, FILE, DNS_CNAME, ...), one
	// entry per method the provider currently has challenges generated for.
	Methods map[string]SSLChallengeMethod
	// AllowedMethods is every method the caller could switch to, regardless
	// of whether it already has challenges generated.
	AllowedMethods []SSLAllowedMethod
	// Deadline is the provider's raw verification deadline string (its own
	// date format, e.g. "2025/11/17") — kept as-is rather than parsed, since
	// the format is not confirmed to be stable across locales.
	Deadline string
}

// SSLVerifyResult is the outcome of a verify-domain-ownership attempt.
// CertificateReady distinguishes "verified, certificate issued" from
// "verified, certificate still being issued" — poll GetSSLCertificate in the
// latter case. RequiresDocuments is set instead for OV/EV products, which
// need manual document review before a certificate is issued at all.
type SSLVerifyResult struct {
	Step              string
	CertificateReady  bool
	Certificate       string // PEM, only set when CertificateReady
	CABundle          string // PEM, only set when CertificateReady
	RequiresDocuments bool
	ProductGroup      string
	Message           string
}

// SSLCertificate is an issued (or not-yet-issued) certificate download.
type SSLCertificate struct {
	Ready       bool
	Certificate string // PEM
	CABundle    string // PEM
}
