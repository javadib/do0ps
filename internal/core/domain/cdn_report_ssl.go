package domain

// The types below model the read-only CDN report/analytics endpoints and the
// CDN-zone-level SSL settings confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml's Report and SSL tags (issue #24).
//
// This is deliberately distinct from ssl.go: ssl.go models the SSL
// certificate ORDERING workflow on Parspack's separate
// https://my.parspack.com/sslv2 surface (issue #18). Everything here is
// CDN-zone-scoped settings and read-only reporting on the CDN surface
// (https://my.parspack.com/cdnapi), confirmed against the
// /external/api/v1/zones/{zone_uuid}/report/*, /analytics/*, /ssl/* and
// /hsts endpoints. All CDN* names here are prefixed to avoid any collision
// with ssl.go's types.

// CDNLogQuery is the shared filter accepted by the four CDN log endpoints
// (access, security, error, WAF). Every field is optional; a zero value
// means "no filter" for that field. WCDNState is only meaningful for the
// access log.
type CDNLogQuery struct {
	Page         int    // page number, minimum 1
	Step         int    // logs per page: one of 10, 25, 50, 100
	From         string // start date, "YYYY-MM-DD"
	To           string // end date, "YYYY-MM-DD"
	URI          string
	StatusCode   int
	UserAgent    string
	Method       string // case-sensitive HTTP method, e.g. "GET"
	RayID        string
	TargetDomain string
	WCDNState    string // access log only, e.g. "miss", "hit"
}

// cdnLogSteps is the enum of page sizes the report endpoints accept.
var cdnLogSteps = []int{10, 25, 50, 100}

// ValidCDNLogStep reports whether step is one of the page sizes the CDN
// report endpoints accept, or zero (meaning "use the provider default").
func ValidCDNLogStep(step int) bool {
	if step == 0 {
		return true
	}
	for _, v := range cdnLogSteps {
		if v == step {
			return true
		}
	}
	return false
}

// CDNLogPageMeta is the pagination info attached to every CDN log listing.
type CDNLogPageMeta struct {
	CurrentPage int
	LastPage    int
	PerPage     int
	Total       int
}

// CDNAccessLogEntry is one record of the access log
// (GET .../report/access-log). Most numeric-looking fields are strings on
// the wire; they are kept as strings here rather than parsed, since the
// spec does not confirm a stable numeric format for all of them (e.g.
// byte_in/byte_out can exceed common integer ranges).
type CDNAccessLogEntry struct {
	ID                  string
	Date                string
	Timestamp           string
	Host                string
	UserHost            string
	TargetDomain        string
	URI                 string
	Method              string
	Scheme              string
	StatusCode          int
	Level               int
	RayID               string
	EdgeID              string
	Source              string
	WCDNState           string
	LogType             string
	ByteIn              string
	ByteOut             string
	RemoteIP            string
	ZID                 string
	ConnectionDuration  string
	DeliveryDuration    string
	HostingWaitDuration string
	TotalDuration       string
	UserAgent           string
	Facility            string
}

// CDNAccessLogPage is one page of GetCDNAccessLog results.
type CDNAccessLogPage struct {
	Records []CDNAccessLogEntry
	Meta    CDNLogPageMeta
}

// CDNSecurityLogEntry is one record of the security log
// (GET .../report/security-log). AdditionalLogs is the provider's raw
// base64-encoded blob (its inner shape is not confirmed to be stable), kept
// as-is for callers that want to decode it themselves.
type CDNSecurityLogEntry struct {
	ID              string
	Date            string
	Timestamp       string
	Host            string
	UserHost        string
	TargetDomain    string
	URI             string
	Method          string
	Scheme          string
	StatusCode      int
	Level           int
	RayID           string
	EdgeID          string
	Source          string
	LogType         string
	SecurityType    string
	SecurityMessage string
	RequestTime     string
	RemoteIP        string
	ZID             string
	AdditionalLogs  string
	UserAgent       string
	Facility        string
}

// CDNSecurityLogPage is one page of GetCDNSecurityLog results.
type CDNSecurityLogPage struct {
	Records []CDNSecurityLogEntry
	Meta    CDNLogPageMeta
}

// CDNErrorLogEntry is one record of the error log
// (GET .../report/error-log).
type CDNErrorLogEntry struct {
	ID          string
	Date        string
	Timestamp   string
	Host        string
	UserHost    string
	URI         string
	Method      string
	Scheme      string
	StatusCode  int
	Level       int
	RayID       string
	EdgeID      string
	Source      string
	LogType     string
	ErrorType   string
	RequestTime string
	RemoteIP    string
	ZID         string
	UserAgent   string
	Facility    string
}

// CDNErrorLogPage is one page of GetCDNErrorLog results.
type CDNErrorLogPage struct {
	Records []CDNErrorLogEntry
	Meta    CDNLogPageMeta
}

// CDNWAFLogDetail is one entry of a CDNWAFLogEntry's AdditionalLogs — unlike
// the security log, the WAF log's additional_logs is a structured array
// (the WAF rule that matched), not a base64 blob.
type CDNWAFLogDetail struct {
	Message string
	Match   string
	Data    string
}

// CDNWAFLogEntry is one record of the WAF log (GET .../report/waf-log).
type CDNWAFLogEntry struct {
	ID              string
	Date            string
	Timestamp       string
	Host            string
	UserHost        string
	TargetDomain    string
	URI             string
	Method          string
	Scheme          string
	StatusCode      int
	Level           int
	RayID           string
	EdgeID          string
	Source          string
	LogType         string
	SecurityType    string
	SecurityMessage string
	RequestTime     string
	RemoteIP        string
	ZID             string
	AdditionalLogs  []CDNWAFLogDetail
	UserAgent       string
	Facility        string
}

// CDNWAFLogPage is one page of GetCDNWAFLog results.
type CDNWAFLogPage struct {
	Records []CDNWAFLogEntry
	Meta    CDNLogPageMeta
}

// CDNTopVisitor is one entry of GetCDNTopVisitors
// (GET .../analytics/top-visitors): a visitor IP and its request count over
// the queried date range.
type CDNTopVisitor struct {
	IP    string
	Count int
}

// CDNTrafficUsage is the result of GetCDNMonthlyTrafficUsage
// (GET .../analytics/monthly-traffic-usage). TrafficLimit is kept as a
// string since the provider returns it as one (it can exceed common integer
// display ranges, e.g. "53687091200" bytes).
type CDNTrafficUsage struct {
	ReceivedBytes int64
	TrafficLimit  string
}

// CDNMinTLSVersion is the minimum TLS version accepted by a CDN zone,
// confirmed against GET/PUT .../ssl/min-tls-version.
type CDNMinTLSVersion string

const (
	CDNMinTLSVersionUnknown CDNMinTLSVersion = ""
	CDNMinTLSVersion10      CDNMinTLSVersion = "1.0"
	CDNMinTLSVersion11      CDNMinTLSVersion = "1.1"
	CDNMinTLSVersion12      CDNMinTLSVersion = "1.2"
	CDNMinTLSVersion13      CDNMinTLSVersion = "1.3"
)

// cdnMinTLSVersions is the enum confirmed against the update endpoint's
// request body.
var cdnMinTLSVersions = []CDNMinTLSVersion{
	CDNMinTLSVersion10, CDNMinTLSVersion11, CDNMinTLSVersion12, CDNMinTLSVersion13,
}

// ValidCDNMinTLSVersion reports whether v is one of the TLS versions the CDN
// SSL settings API accepts.
func ValidCDNMinTLSVersion(v CDNMinTLSVersion) bool {
	for _, want := range cdnMinTLSVersions {
		if v == want {
			return true
		}
	}
	return false
}

// CDNCertificateDetail is the issuance detail nested under a
// CDNCertificate's LetsEncrypt or Custom field. Certificate, PrivateKey and
// CABundle are returned by the provider already base64-encoded — kept as-is
// rather than decoded, since callers may want to forward them unchanged.
type CDNCertificateDetail struct {
	ExpirationTime string
	Active         bool
	Certificate    string
	PrivateKey     string
	CABundle       string
}

// CDNCertificate is one certificate attached to a CDN zone
// (GET .../ssl/certificates). This is read-only: certificate ORDERING is a
// separate workflow on the SSL surface (ssl.go, issue #18) not modeled here.
type CDNCertificate struct {
	Domain      string
	Status      string
	SSLType     string // e.g. "letsencrypt", "custom"
	LetsEncrypt *CDNCertificateDetail
	Custom      *CDNCertificateDetail
}

// CDNHSTSSettings is a CDN zone's HTTP Strict Transport Security
// configuration, confirmed against GET/PUT .../hsts.
type CDNHSTSSettings struct {
	Enabled       bool
	MaxAgeSeconds int // 0 to 31536000 (one year)
}
