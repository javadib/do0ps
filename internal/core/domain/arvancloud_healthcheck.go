package domain

// The types below model ArvanCloud's Active Health Check capability (issue
// #70): a domain-scoped monitor that periodically probes an origin (in
// practice, a Load Balancing pool — see #69/AC9) over TCP or HTTP(S) and
// reports whether it is reachable, confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Active Health Check" tag (the
// active-health-check.* and health-checks.* operationIds) and the
// HealthCheck/BaseHealthCheck/HttpConfig/TcpConfig/HealthCheckZone schemas.
//
// Conceptually related to #69's ArvanCloudLoadBalancerPool.HealthCheck field
// (an opaque reference to the check monitoring that pool), but this is its
// own resource family — the spec exposes it under its own path
// (/domains/{domain}/health-checks/...), not nested under load-balancers, so
// it is not a compile-time dependency of #69's types.

// ArvanCloudHealthCheckType is which protocol a health check probes its
// origin over (HealthCheck.type / BaseHealthCheck.type).
type ArvanCloudHealthCheckType string

const (
	ArvanCloudHealthCheckTCP   ArvanCloudHealthCheckType = "TCP"
	ArvanCloudHealthCheckHTTP  ArvanCloudHealthCheckType = "HTTP"
	ArvanCloudHealthCheckHTTPS ArvanCloudHealthCheckType = "HTTPS"
)

var arvanCloudHealthCheckTypes = []string{
	string(ArvanCloudHealthCheckTCP),
	string(ArvanCloudHealthCheckHTTP),
	string(ArvanCloudHealthCheckHTTPS),
}

// ValidArvanCloudHealthCheckType reports whether s is one of
// HealthCheck.type's three values.
func ValidArvanCloudHealthCheckType(s string) bool { return contains(arvanCloudHealthCheckTypes, s) }

// ArvanCloudHealthCheckOriginType is what kind of resource
// HealthCheck.origin addresses. The spec's enum currently declares exactly
// one value, "pool" — kept as its own type and exposed as a field (issue
// #70's own instruction) rather than hardcoded in the adapter, since the
// spec does not say whether this is a true single-value constant or an enum
// reserved for future expansion (e.g. a plain upstream IP/host, per
// BaseHealthCheck.origin's own description).
type ArvanCloudHealthCheckOriginType string

// ArvanCloudHealthCheckOriginTypePool is the only value the spec currently
// declares for HealthCheck.origin_type: origin addresses a Load Balancing
// pool (#69/AC9) by its ID.
const ArvanCloudHealthCheckOriginTypePool ArvanCloudHealthCheckOriginType = "pool"

var arvanCloudHealthCheckOriginTypes = []string{string(ArvanCloudHealthCheckOriginTypePool)}

// ValidArvanCloudHealthCheckOriginType reports whether s is one of
// HealthCheck.origin_type's declared values.
func ValidArvanCloudHealthCheckOriginType(s string) bool {
	return contains(arvanCloudHealthCheckOriginTypes, s)
}

// arvanCloudHealthCheckIntervalsMS is HealthCheck.interval's fixed enum, in
// MILLISECONDS per the spec's own description — not seconds, an easy
// off-by-1000 mistake the issue calls out explicitly.
var arvanCloudHealthCheckIntervalsMS = []int{30000, 60000, 120000}

// ValidArvanCloudHealthCheckIntervalMS reports whether ms is one of
// HealthCheck.interval's three values (30000/60000/120000 milliseconds,
// i.e. 30/60/120 seconds).
func ValidArvanCloudHealthCheckIntervalMS(ms int) bool {
	for _, v := range arvanCloudHealthCheckIntervalsMS {
		if v == ms {
			return true
		}
	}
	return false
}

// ArvanCloudHealthCheckHTTPMethod is the HTTP method an HTTP/HTTPS health
// check probes with (HttpConfig.method).
type ArvanCloudHealthCheckHTTPMethod string

const (
	ArvanCloudHealthCheckHTTPMethodHead ArvanCloudHealthCheckHTTPMethod = "HEAD"
	ArvanCloudHealthCheckHTTPMethodGet  ArvanCloudHealthCheckHTTPMethod = "GET"
	ArvanCloudHealthCheckHTTPMethodPost ArvanCloudHealthCheckHTTPMethod = "POST"
	ArvanCloudHealthCheckHTTPMethodPut  ArvanCloudHealthCheckHTTPMethod = "PUT"
)

var arvanCloudHealthCheckHTTPMethods = []string{
	string(ArvanCloudHealthCheckHTTPMethodHead),
	string(ArvanCloudHealthCheckHTTPMethodGet),
	string(ArvanCloudHealthCheckHTTPMethodPost),
	string(ArvanCloudHealthCheckHTTPMethodPut),
}

// ValidArvanCloudHealthCheckHTTPMethod reports whether s is one of
// HttpConfig.method's four values.
func ValidArvanCloudHealthCheckHTTPMethod(s string) bool {
	return contains(arvanCloudHealthCheckHTTPMethods, s)
}

// ArvanCloudHealthCheckExpectedHeader is one entry of
// ExpectedResponse.expected_headers: a response header this health check
// requires, and the values it accepts for it.
type ArvanCloudHealthCheckExpectedHeader struct {
	Key   string
	Value []string
}

// ArvanCloudHealthCheckExpectedResponse is what an HTTP/HTTPS health check
// considers a passing probe (HttpConfig.expected_response, the
// ExpectedResponse schema).
type ArvanCloudHealthCheckExpectedResponse struct {
	// Codes is the set of HTTP status codes considered healthy, e.g. [200,
	// 204]. Empty means the provider's own default applies.
	Codes []int
	// Headers is the response-header shape the spec's Headers schema
	// declares: a header name mapped to every value it may take. Distinct
	// from ExpectedHeaders below — the spec declares both fields on
	// ExpectedResponse (headers via a $ref to Headers, expected_headers via
	// a $ref to ExpectedHeaders), and this type keeps them as two separate
	// fields to match rather than guessing which one the provider actually
	// reads.
	Headers map[string][]string
	// ExpectedHeaders is the spec's ExpectedHeaders shape: an explicit list
	// of header-name/accepted-values pairs, rather than a map, so a header
	// name can in principle be checked more than once.
	ExpectedHeaders []ArvanCloudHealthCheckExpectedHeader
	// Body is a substring the response body must contain. Empty means no
	// body check.
	Body string
}

// ArvanCloudHealthCheckSentHeader is one request header an HTTP/HTTPS health
// check sends with its probe (HttpConfig.sent_headers entries).
type ArvanCloudHealthCheckSentHeader struct {
	Key   string
	Value string
}

// ArvanCloudHealthCheckHTTPConfig is the probe configuration for an HTTP or
// HTTPS health check (HealthCheckRequestConfig's HttpConfig branch),
// meaningful only when the parent ArvanCloudHealthCheck.Type is
// ArvanCloudHealthCheckHTTP or ArvanCloudHealthCheckHTTPS.
type ArvanCloudHealthCheckHTTPConfig struct {
	// Method must be one of ValidArvanCloudHealthCheckHTTPMethod's values.
	Method ArvanCloudHealthCheckHTTPMethod
	// Port is the TCP port to probe, e.g. 443. Spec range: 1-65535.
	Port int
	// Path is the request path to probe, e.g. "/healthz".
	Path string
	// AllowInsecure skips TLS certificate verification, for HTTPS checks
	// against an origin with a self-signed or otherwise untrusted
	// certificate.
	AllowInsecure bool
	// ExpectedResponse is what counts as a passing probe.
	ExpectedResponse ArvanCloudHealthCheckExpectedResponse
	// Headers is the plain header-name-to-single-value map sent WITH the
	// probe request (HttpConfig.headers) — distinct from SentHeaders below,
	// which is the spec's own separate sent_headers field; both are kept
	// since the spec declares both on HttpConfig rather than one superseding
	// the other.
	Headers map[string]string
	// SentHeaders is the spec's sent_headers list: request headers sent with
	// the probe, as explicit key/value pairs.
	SentHeaders []ArvanCloudHealthCheckSentHeader
	// FollowRedirects is read-only and deprecated per the spec
	// (HttpConfig.follow_redirects) — kept only so a caller reading back a
	// check created before the deprecation still sees the field; never sent
	// on create/update.
	FollowRedirects bool
	// TimeoutMS is the probe timeout, in MILLISECONDS (HttpConfig.timeout).
	// Spec range: 1-30000.
	TimeoutMS int
}

// ArvanCloudHealthCheckTCPConfig is the probe configuration for a TCP health
// check (HealthCheckRequestConfig's TcpConfig branch), meaningful only when
// the parent ArvanCloudHealthCheck.Type is ArvanCloudHealthCheckTCP.
type ArvanCloudHealthCheckTCPConfig struct {
	// Port is the TCP port to probe, e.g. 5432. Spec range: 1-65535.
	Port int
	// TimeoutMS is the probe timeout, in MILLISECONDS (TcpConfig.timeout).
	// Spec range: 1-10000 — a narrower range than the HTTP config's, per the
	// spec.
	TimeoutMS int
}

// ArvanCloudHealthCheckRequestConfig is the spec's HealthCheckRequestConfig
// oneOf (HttpConfig | TcpConfig), modeled as two optional branches rather
// than a Go interface: exactly one of HTTP/TCP is set, selected by the
// parent ArvanCloudHealthCheck.Type (TCP -> TCP; HTTP/HTTPS -> HTTP). Both
// nil is invalid input; both set is never produced by this adapter and is
// rejected the same way.
type ArvanCloudHealthCheckRequestConfig struct {
	HTTP *ArvanCloudHealthCheckHTTPConfig
	TCP  *ArvanCloudHealthCheckTCPConfig
}

// arvanCloudHealthCheckZoneMonitoringLevels is HealthCheckZone.monitoring_level's
// fixed enum.
var arvanCloudHealthCheckZoneMonitoringLevels = []string{"critical", "non-critical", "quiet-critical", "off"}

// ValidArvanCloudHealthCheckZoneMonitoringLevel reports whether s is one of
// HealthCheckZone.monitoring_level's four values, or empty (meaning "let the
// provider apply its own default for this zone").
func ValidArvanCloudHealthCheckZoneMonitoringLevel(s string) bool {
	if s == "" {
		return true
	}
	return contains(arvanCloudHealthCheckZoneMonitoringLevels, s)
}

// ArvanCloudHealthCheckZone is one check-execution zone a health check runs
// from, as embedded on ArvanCloudHealthCheck.Zones (the HealthCheckZone
// schema — NOT the same shape as ArvanCloudHealthCheckZoneName below, which
// is what the zone-LISTING endpoints return).
type ArvanCloudHealthCheckZone struct {
	// ID is the zone's identifier. Per the spec: if not provided, Name is
	// used as the ID.
	ID string
	// Name is the zone's human-readable name.
	Name string
	// MonitoringLevel is one of "critical"/"non-critical"/"quiet-critical"/"off".
	MonitoringLevel string
}

// ArvanCloudHealthCheckZoneName is a check-execution zone as returned by the
// zone-LISTING endpoints (ListArvanCloudDomainHealthCheckZones,
// ListArvanCloudHealthCheckZones — the HealthCheckZoneName schema): just an
// id/name pair, with no MonitoringLevel. A caller assembling
// ArvanCloudHealthCheck.Zones picks a MonitoringLevel itself; these
// endpoints only say which zones exist to choose from.
type ArvanCloudHealthCheckZoneName struct {
	ID   string
	Name string
}

// ArvanCloudHealthCheck is a domain-scoped active health check
// (/domains/{domain}/health-checks[/{id}], the HealthCheck/HealthCheckView/
// BaseHealthCheck schemas).
type ArvanCloudHealthCheck struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string
	// Name is a caller-supplied label for the check.
	Name string
	// Description is a caller-supplied note about the check.
	Description string
	// Origin is what this check monitors: per the spec, an IP/host when
	// OriginType is "upstream" (not currently a declared enum value — see
	// ArvanCloudHealthCheckOriginType's doc comment), otherwise a valid
	// record ID (in practice, a Load Balancing pool ID, #69/AC10).
	Origin string
	// OriginType must be one of ValidArvanCloudHealthCheckOriginType's
	// values.
	OriginType ArvanCloudHealthCheckOriginType
	// Upstreams is the set of upstream addresses this check probes within
	// Origin, e.g. ["1.1.1.1"].
	Upstreams []string
	// IntervalMS is how often the check probes, in MILLISECONDS. Must be one
	// of ValidArvanCloudHealthCheckIntervalMS's three values.
	IntervalMS int
	// Threshold is how many consecutive failed probes before an upstream is
	// marked unhealthy. Spec minimum: 1.
	Threshold int
	// Type selects the probe protocol and, correspondingly, which of
	// RequestConfig's branches applies. Must be one of
	// ValidArvanCloudHealthCheckType's values.
	Type ArvanCloudHealthCheckType
	// Status is the admin enable/disable switch for the whole check.
	// Defaults to true when left unset (the spec's own default).
	Status bool
	// Retries is how many immediate retries a timed-out probe gets before
	// counting as failed. Spec range: 0-10.
	Retries int
	// Zones is which check-execution zones this check runs from, each with
	// its own monitoring level. See ArvanCloudHealthCheckZone's doc comment
	// for how this differs from the zone-LISTING endpoints' return shape.
	Zones []ArvanCloudHealthCheckZone
	// MonitoringUpdatedAt is a read-only, nullable provider timestamp for
	// when this check's live monitoring status last changed.
	MonitoringUpdatedAt string
	// RequestConfig is the protocol-specific probe configuration selected by
	// Type. See ArvanCloudHealthCheckRequestConfig's doc comment.
	RequestConfig ArvanCloudHealthCheckRequestConfig
}

// --- Reports -----------------------------------------------------------

// arvanCloudHealthCheckReportPeriods is ReportPeriod's fixed enum: a period
// ending now, to report over.
var arvanCloudHealthCheckReportPeriods = []string{"5m", "1h", "3h", "6h", "12h", "24h", "7d", "30d"}

// ValidArvanCloudHealthCheckReportPeriod reports whether s is one of
// ReportPeriod's eight values, or empty (meaning "no period filter").
// "5m" is enterprise-domain-only per the spec's own description; this
// adapter does not attempt to enforce that account-level restriction
// client-side, since the caller's plan is not known here — an ineligible
// domain gets the provider's own rejection instead.
func ValidArvanCloudHealthCheckReportPeriod(s string) bool {
	if s == "" {
		return true
	}
	return contains(arvanCloudHealthCheckReportPeriods, s)
}

// arvanCloudHealthCheckReportTypes is HealthCheckReportType's fixed enum,
// used only by GetArvanCloudHealthCheckDetails (active-health-check.reports.details).
var arvanCloudHealthCheckReportTypes = []string{"all", "success", "error"}

// ValidArvanCloudHealthCheckReportType reports whether s is one of
// HealthCheckReportType's three values, or empty (meaning the provider's own
// "all" default applies).
func ValidArvanCloudHealthCheckReportType(s string) bool {
	if s == "" {
		return true
	}
	return contains(arvanCloudHealthCheckReportTypes, s)
}

// arvanCloudReportDirections is Direction's fixed enum, shared by both
// health-check report endpoints.
var arvanCloudReportDirections = []string{"asc", "desc"}

// ValidArvanCloudReportDirection reports whether s is "asc"/"desc", or empty
// (meaning the provider's own default order applies).
func ValidArvanCloudReportDirection(s string) bool {
	if s == "" {
		return true
	}
	return contains(arvanCloudReportDirections, s)
}

// ArvanCloudHealthCheckReportQuery is the filter shared by
// GetArvanCloudHealthCheckSummary and GetArvanCloudHealthCheckDetails
// (active-health-check.reports.summary / .details), the same
// shared-query-struct convention CDNLogQuery uses for Parspack's report
// endpoints. Name and Upstream are required by both endpoints; Type,
// PerPage and Page are meaningful only for GetArvanCloudHealthCheckDetails
// (the summary endpoint is not paginated).
type ArvanCloudHealthCheckReportQuery struct {
	// Name is the health check's name to report on. Required by both
	// endpoints.
	Name string
	// Upstream is which upstream (within the check's Origin) to report on.
	// Required by both endpoints.
	Upstream string
	// Period selects a period ending now. Must satisfy
	// ValidArvanCloudHealthCheckReportPeriod. Empty lets the provider apply
	// its own default.
	Period string
	// Since/Until bound the report window explicitly, ISO 8601 UTC (e.g.
	// "1988-07-11T00:00:00Z"). Either may be combined with Period per the
	// spec; this adapter passes through whatever the caller sets without
	// forcing Period and Since/Until to be mutually exclusive.
	Since string
	Until string
	// Direction must satisfy ValidArvanCloudReportDirection. Empty lets the
	// provider apply its own default order.
	Direction string
	// Type filters GetArvanCloudHealthCheckDetails results to "all"
	// (default)/"success"/"error". Must satisfy
	// ValidArvanCloudHealthCheckReportType. Ignored by
	// GetArvanCloudHealthCheckSummary — the spec does not declare this
	// parameter on that endpoint.
	Type string
	// PerPage/Page paginate GetArvanCloudHealthCheckDetails. Zero leaves
	// both to the provider's own defaults. Ignored by
	// GetArvanCloudHealthCheckSummary.
	PerPage int
	Page    int
}

// ArvanCloudHealthCheckReportPageMeta is the pagination info attached to
// GetArvanCloudHealthCheckDetails' result (the PaginatedResponseMeta
// schema), the same shape CDNLogPageMeta gives Parspack's own paginated
// report endpoints.
type ArvanCloudHealthCheckReportPageMeta struct {
	CurrentPage int
	From        int
	LastPage    int
	PerPage     int
	To          int
	Total       int
}

// ArvanCloudHealthCheckReportSummaryDetail is one data point of a
// per-zone summary report (HealthCheckReportSummaryDetail).
type ArvanCloudHealthCheckReportSummaryDetail struct {
	Date   string
	Status bool
}

// ArvanCloudHealthCheckReportSummary is one zone's aggregate report from
// GetArvanCloudHealthCheckSummary (the HealthCheckReportSummary schema).
type ArvanCloudHealthCheckReportSummary struct {
	Zone    string
	Status  bool
	Total   int
	Failed  int
	Details []ArvanCloudHealthCheckReportSummaryDetail
}

// ArvanCloudHealthCheckReportDetail is one probe result from
// GetArvanCloudHealthCheckDetails (the HealthCheckReportDetail schema).
type ArvanCloudHealthCheckReportDetail struct {
	Date     string
	Zone     string
	Upstream string
	Status   bool
	Message  string
}
