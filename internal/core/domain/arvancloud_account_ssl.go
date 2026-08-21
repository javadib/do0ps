package domain

// The types below model ArvanCloud's ACCOUNT-scoped Certum certificate
// ordering workflow (issue #74/AC14): a paid, CA-issued (Certum-branded)
// certificate product, purchased against the account and then installed
// onto the edge servers serving one or more domains. Confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "SSL/TLS" tag — the
// account_certificate.* operationIds under /certificate-products and
// /certificates/... — and the CertificateProduct/CertificateOrderIssueRequest/
// AccountCertificateOrder schemas.
//
// This is deliberately NOT the domain-scoped SSL/TLS settings and
// managed/uploaded certificate workflow (issue #73) — see
// arvancloud_ssl.go's package comment for the same base-concern split. Do
// not unify these two order types: the spec gives them different wire
// shapes (see ArvanCloudAccountCertificateOrder's own doc comment for the
// specific fields that differ from ArvanCloudCertificateOrder).

// ArvanCloudCertificateProduct is a purchasable Certum certificate product
// (the CertificateProduct schema, account_certificate.product.index). Price
// is a real cost — see ArvanCloudCertificateOrderIssueRequest and the
// issue_arvancloud_account_certificate tool description, which must surface
// this before ordering (this project has no payment-confirmation UI of its
// own, per AGENTS.md 4.2/4.3's credential-and-cost-handling principles
// extended to paid operations).
type ArvanCloudCertificateProduct struct {
	// ID is the provider-assigned UUID, used as ProductID on
	// ArvanCloudCertificateOrderIssueRequest.
	ID   string
	Name string
	// IsMulti reports whether this product can cover multiple SANs (Subject
	// Alternative Names) in one certificate.
	IsMulti bool
	// HasWildcard reports whether this product can cover a wildcard domain
	// name (e.g. "*.example.com").
	HasWildcard bool
	// Limit is the maximum number of domain names (or SANs) this product
	// covers.
	Limit int
	// Price is the product's cost. Currency/unit is whatever the provider's
	// account billing uses; this project neither interprets nor converts it,
	// only surfaces it so the caller can see cost before ordering.
	Price float64
}

// ArvanCloudCertificateIssueDomain is one entry of
// ArvanCloudCertificateOrderIssueRequest.Domains: which domain (by its
// internal UUID, NOT its name — see below) the order covers, and which
// hostnames on it the certificate should list.
//
// Confirmed against the spec: CertificateOrderIssueRequest.domains[].domain
// is declared "format: uuid". Every other ArvanCloud CDN endpoint addresses
// a domain by its NAME (ports.ArvanCloudProvider, domain/arvancloud.go's own
// package comment) — this is the one exception, because this order is
// placed against the ACCOUNT, not against a /domains/{domain}/... path, so
// it needs an explicit reference to which domain resource(s) the resulting
// certificate should later be installed onto. Callers obtain a domain's ID
// from ListArvanCloudDomains/GetArvanCloudDomain (issue #62,
// ArvanCloudDomain.ID) — never from the domain name itself.
type ArvanCloudCertificateIssueDomain struct {
	// DomainID is the target domain's internal UUID (ArvanCloudDomain.ID),
	// not its hostname.
	DomainID string
	// DomainNames are the hostnames on DomainID the certificate should
	// cover (e.g. the apex plus a wildcard).
	DomainNames []string
}

// ArvanCloudCertificateOrderIssueRequest is the request body for
// IssueArvanCloudAccountCertificate (account_certificate.issue). Confirmed
// against the spec's CertificateOrderIssueRequest schema: required fields
// are Domains and ProductID; PrivateKeySize is a closed two-value enum,
// checked client-side by ValidArvanCloudCertificatePrivateKeySize before
// this ever reaches the provider.
//
// Unlike UploadArvanCloudCertificate (issue #73), this request carries no
// private key material at all: ArvanCloud/Certum generates and holds the
// key itself for this CA-issued flow (see the KeepPrivateKey query
// parameter on GetArvanCloudAccountCertificateOrder, which controls whether
// that provider-held key is retained or permanently destroyed after
// issuance) — there is nothing caller-supplied here to redact.
type ArvanCloudCertificateOrderIssueRequest struct {
	// Domains lists which domain(s) and hostnames the certificate should
	// cover. Required, at least one entry.
	Domains []ArvanCloudCertificateIssueDomain
	// ProductID selects which ArvanCloudCertificateProduct to purchase
	// (from ListArvanCloudCertificateProducts). Required.
	ProductID string
	// CommonName is the certificate's primary subject name. Optional per
	// the spec; when empty the provider presumably derives one from
	// Domains.
	CommonName string
	// PrivateKeySize is the key size ArvanCloud/Certum generates for this
	// certificate. Must be one of ValidArvanCloudCertificatePrivateKeySize's
	// two values (2048 or 4096) — the spec declares this a fixed enum, so
	// any other value is rejected client-side rather than sent to the
	// provider only to fail there.
	PrivateKeySize int
}

// ValidArvanCloudCertificatePrivateKeySize reports whether n is one of
// CertificateOrderIssueRequest.private_key_size's two enum values.
func ValidArvanCloudCertificatePrivateKeySize(n int) bool { return n == 2048 || n == 4096 }

// ArvanCloudAccountCertificateOrderStatus is an
// ArvanCloudAccountCertificateOrder.Status value.
//
// UNLIKE ArvanCloudCertificateOrderStatus (issue #73's domain-scoped
// CertificateOrder.status), the spec declares this field as a bare
// "type: string" with NO enum and no per-value description text — issue
// #74 itself flagged this as needing verification rather than an assumed
// shared type, and the spec confirms the two are NOT the same declared Go
// type: this one is deliberately left an open string, not a closed enum,
// and has no ValidArvanCloud...Status validator (there is nothing to
// validate against — the provider is the sole source of truth for this
// field's actual values).
//
// ArvanCloudAccountCertificateOrderTerminal below still needs to recognize
// when polling can stop. Reusing the same terminal-signal strings issue
// #73's CertificateOrder.status enum settled on ("valid" for success,
// "killed" for permanent failure) is an inference, not a spec-confirmed
// fact: both flows sit under the same "SSL/TLS" spec tag, are both
// Certum/ACME-style CA order state machines, and use identical wording
// ("valid"/"killed") wherever this account-level flow's own documentation
// happens to mention a concrete status value — but this has NOT been
// confirmed against a declared enum the way the domain-scoped flow's 9
// values were. If this project's owner has since observed a different
// account-level status vocabulary in practice, update this list (and its
// doc comment) rather than assuming it silently still holds.
//
// Because that inference could be wrong, IssueArvanCloudAccountCertificate's
// poll loop always still has PollTimeout as a hard backstop: an order whose
// status never matches a known terminal value is reported as a timeout
// failure rather than polled forever.
type ArvanCloudAccountCertificateOrderStatus string

const (
	// ArvanCloudAccountCertificateOrderStatusValid is the inferred SUCCESS
	// terminal value — see this type's own doc comment for why "inferred".
	ArvanCloudAccountCertificateOrderStatusValid ArvanCloudAccountCertificateOrderStatus = "valid"
	// ArvanCloudAccountCertificateOrderStatusKilled is the inferred
	// permanent-FAILURE terminal value — see this type's own doc comment.
	// ReissueArvanCloudAccountCertificate is the manual recovery path for an
	// order in this state (there is no order-level "retry" endpoint at the
	// account level, unlike issue #73's ssl.cert.order.retry).
	ArvanCloudAccountCertificateOrderStatusKilled ArvanCloudAccountCertificateOrderStatus = "killed"
)

// ArvanCloudAccountCertificateOrderTerminal reports whether status is one of
// the two inferred terminal values. See ArvanCloudAccountCertificateOrderStatus's
// own doc comment for why this is an inference backed by a poll timeout,
// not a spec-confirmed closed set.
func ArvanCloudAccountCertificateOrderTerminal(status ArvanCloudAccountCertificateOrderStatus) bool {
	return status == ArvanCloudAccountCertificateOrderStatusValid || status == ArvanCloudAccountCertificateOrderStatusKilled
}

// ArvanCloudAccountCertificateOrder is an account-level Certum certificate
// order (the AccountCertificateOrder schema): returned by
// ListArvanCloudCertificateProducts's sibling list/get/action endpoints and
// created by IssueArvanCloudAccountCertificate.
//
// Confirmed field-shape differences from issue #73's
// ArvanCloudCertificateOrder that rule out reusing that type here (per
// issue #74's own instruction to verify rather than assume):
//   - OrderID is a plain string on the wire here (spec: "type: string", no
//     format annotation) — NOT the same ambiguous "type: string, format:
//     int32" the domain-scoped schema declares (which this adapter decodes
//     as a Go int). No int/string coercion happens on this field.
//   - Errors has no declared shape at all (spec: "type: object,
//     nullable: true", no $ref) — unlike the domain-scoped order's
//     structured SSLProblem. Modeled as a generic map so whatever the
//     provider actually returns is preserved without inventing fields the
//     spec never promised.
//   - Product is a plain string (the product name/ID echoed back), not a
//     nested ArvanCloudCertificateProduct object.
type ArvanCloudAccountCertificateOrder struct {
	// ID is the provider-assigned UUID — the path parameter every other
	// per-order operation (Get/Revoke/Reissue/Install) addresses this order
	// by.
	ID string
	// OrderID is a separate provider-assigned string identifier reported
	// alongside ID. See this type's own doc comment for why it is NOT
	// decoded as an int the way issue #73's OrderID is.
	OrderID string
	// Status is where the order stands in its issuance state machine. See
	// ArvanCloudAccountCertificateOrderStatus's own doc comment: this is an
	// open string, not a closed enum.
	Status ArvanCloudAccountCertificateOrderStatus
	// DomainNames are the hostnames this order covers.
	DomainNames []string
	// Product is the purchased product's name/ID as echoed back by the
	// provider (a plain string, not a nested ArvanCloudCertificateProduct).
	Product string
	// Errors is populated when Status reports a failure. The spec declares
	// no fixed shape for this field, so it is preserved as-is rather than
	// forced into issue #73's structured ArvanCloudSSLProblem.
	Errors map[string]any
	// IsRevoked reports whether the order's certificate has been revoked
	// (RevokeArvanCloudAccountCertificate).
	IsRevoked  bool
	ExpiryDate string
	CreatedAt  string
	UpdatedAt  string
}

// ArvanCloudCertificateInstallWarning is one entry of
// ArvanCloudCertificateInstallResult.Warnings.
type ArvanCloudCertificateInstallWarning struct {
	Domain string
	Reason string
}

// ArvanCloudCertificateInstallResult is the response of
// InstallArvanCloudAccountCertificate (account_certificate.install).
// Confirmed against the spec: unlike every order-returning operation above,
// this endpoint's response has no "status" field at all — just a plain
// success/message/warnings shape — which is why InstallArvanCloudAccountCertificate
// is a FAST operation (AGENTS.md 4.3), not a long one: the install either
// completes within the call or reports per-domain warnings about why it
// partially didn't, with nothing left to poll afterward.
type ArvanCloudCertificateInstallResult struct {
	Success  bool
	Message  string
	Warnings []ArvanCloudCertificateInstallWarning
}
