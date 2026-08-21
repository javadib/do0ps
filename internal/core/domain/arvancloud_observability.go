package domain

// The types below model ArvanCloud's Log Forwarders and Metric Exporters
// (issue #76): two observability-integration capabilities that both
// configure ArvanCloud to PUSH data to an external system (S3-compatible
// storage, Datadog, Kafka, syslog, ...) rather than exposing data through
// this project's own reporting tools — contrast with arvancloud_reports.go
// (issue #75), which reads report data directly. Confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Log Forwarders" and "Metric
// Exporters" tags (the log-forwarders.* and metric-exporters.* operationIds)
// and the LogForwarder/LogForwarderGeneric/MetricExporter schemas.
//
// Spec-ambiguity resolutions (issue #76 explicitly calls out not to guess
// these):
//
//   - LogForwarder.type is a 5-value enum (access/waf/dns/error/event,
//     confirmed against both the LogForwarder schema and the LogForwarderTypes
//     query parameter) while MetricExporter.type is a narrower 4-value enum
//     (access/dns/error/event — confirmed against both the MetricExporter
//     schema and the MetricExporterTypes query parameter). This is NOT an
//     oversight: waf log STREAMS exist and can be forwarded, but there is no
//     waf-specific METRIC group in metric-exporters.metrics.index's response
//     shape either, so the two enums are deliberately separate types here
//     (ArvanCloudLogForwarderType vs. ArvanCloudMetricExporterType) rather
//     than one shared enum.
//   - LogForwarder.data_fields varies by LogForwarder.type: the spec's
//     LogForwarderDataFields is a oneOf across five schemas
//     (LogForwarderAccessLogType/WAFType/DNSType/ErrorType/EventType), each a
//     flat object of boolean toggles with its own field set. Modeled here as
//     ArvanCloudLogForwarderDataFields, one flattened struct documenting per
//     field which LogForwarder.Type value(s) it applies to — the same
//     flattened-union treatment ArvanCloudDNSRecordValue (arvancloud_dns.go)
//     uses for DNS's per-record-type value shape, chosen because (like DNS)
//     the field names essentially never collide in meaning across the five
//     types.
//   - LogForwarder.settings varies by LogForwarder.connection_type: the
//     spec's LogForwarderSetting is a oneOf across six schemas (S3/Datadog/
//     Kafka/Loggly/Syslog/ArvanLogsConnectionType). Modeled here as
//     ArvanCloudLogForwarderSettings, a tagged struct keyed by
//     ConnectionType: exactly one of its six pointer fields is populated,
//     selected the same way ArvanCloudHealthCheckRequestConfig
//     (arvancloud_healthcheck.go) picks its TCP/HTTP branch by the parent's
//     Type field.
//   - MetricExporter's list endpoint (metric-exporters.index) is
//     account-wide (no {domain} path segment) while create/get/update/
//     delete/status are all domain-scoped (/domains/{domain}/metric-
//     exporters...). Confirmed NOT a gap: MetricExporterSummary (the list
//     response's item shape) carries its own "domain" field, so a caller
//     listing across the whole account can still tell which domain each
//     exporter belongs to without a separate domain-scoped list endpoint (the
//     spec defines none). ArvanCloudMetricExporter.Domain carries that value,
//     populated only by the list endpoint (see its own doc comment below).
//
// settings sensitivity: LogForwarderSetting fields carry credentials for the
// EXTERNAL destination (S3 access/secret keys, a Datadog API key, a Kafka
// broker list, ...) — caller-supplied third-party credential material this
// server passes through without storing, not an ArvanCloud credential of its
// own. Treated as sensitive the same way domain.ArvanCloudDdosSettings.SecretKey
// and domain.ArvanCloudSSLCertificate-adjacent private_key material are
// (AGENTS.md 4.2's principle extended to runtime logging): never logged,
// never embedded in an error message. See
// internal/adapters/providers/arvancloud/observability.go's package comment
// and observability_test.go's redaction tests.

// ArvanCloudLogForwarderType is which log stream a LogForwarder forwards
// (LogForwarder.type). Confirmed against both the LogForwarder schema's enum
// and the LogForwarderTypes query parameter's enum: five values.
type ArvanCloudLogForwarderType string

const (
	ArvanCloudLogForwarderTypeAccess ArvanCloudLogForwarderType = "access"
	ArvanCloudLogForwarderTypeWAF    ArvanCloudLogForwarderType = "waf"
	ArvanCloudLogForwarderTypeDNS    ArvanCloudLogForwarderType = "dns"
	ArvanCloudLogForwarderTypeError  ArvanCloudLogForwarderType = "error"
	ArvanCloudLogForwarderTypeEvent  ArvanCloudLogForwarderType = "event"
)

var arvanCloudLogForwarderTypes = []string{
	string(ArvanCloudLogForwarderTypeAccess), string(ArvanCloudLogForwarderTypeWAF),
	string(ArvanCloudLogForwarderTypeDNS), string(ArvanCloudLogForwarderTypeError),
	string(ArvanCloudLogForwarderTypeEvent),
}

// ValidArvanCloudLogForwarderType reports whether s is one of
// LogForwarder.type's five values.
func ValidArvanCloudLogForwarderType(s string) bool { return contains(arvanCloudLogForwarderTypes, s) }

// ArvanCloudConnectionType is a LogForwarder's destination
// (LogForwarder.connection_type). Confirmed against the LogForwarder
// schema's enum: nine values, three of them S3-compatible variants that
// share one settings shape (see ArvanCloudLogForwarderSettings).
type ArvanCloudConnectionType string

const (
	ArvanCloudConnectionTypeArvanS3   ArvanCloudConnectionType = "arvan_s3"
	ArvanCloudConnectionTypeAlibabaS3 ArvanCloudConnectionType = "alibaba_s3"
	ArvanCloudConnectionTypeAmazonS3  ArvanCloudConnectionType = "amazon_s3"
	ArvanCloudConnectionTypeCustomS3  ArvanCloudConnectionType = "custom_s3"
	ArvanCloudConnectionTypeLoggly    ArvanCloudConnectionType = "loggly"
	ArvanCloudConnectionTypeDatadog   ArvanCloudConnectionType = "datadog"
	ArvanCloudConnectionTypeSyslog    ArvanCloudConnectionType = "syslog"
	ArvanCloudConnectionTypeKafka     ArvanCloudConnectionType = "kafka"
	ArvanCloudConnectionTypeArvanLogs ArvanCloudConnectionType = "arvan_logs"
)

var arvanCloudConnectionTypes = []string{
	string(ArvanCloudConnectionTypeArvanS3), string(ArvanCloudConnectionTypeAlibabaS3),
	string(ArvanCloudConnectionTypeAmazonS3), string(ArvanCloudConnectionTypeCustomS3),
	string(ArvanCloudConnectionTypeLoggly), string(ArvanCloudConnectionTypeDatadog),
	string(ArvanCloudConnectionTypeSyslog), string(ArvanCloudConnectionTypeKafka),
	string(ArvanCloudConnectionTypeArvanLogs),
}

// ValidArvanCloudConnectionType reports whether s is one of
// LogForwarder.connection_type's nine values.
func ValidArvanCloudConnectionType(s string) bool { return contains(arvanCloudConnectionTypes, s) }

// IsArvanCloudS3ConnectionType reports whether ct is one of the three
// S3-compatible connection types that share
// ArvanCloudLogForwarderS3Settings' shape (LogForwarderS3ConnectionType in
// the spec).
func IsArvanCloudS3ConnectionType(ct ArvanCloudConnectionType) bool {
	return ct == ArvanCloudConnectionTypeArvanS3 || ct == ArvanCloudConnectionTypeAlibabaS3 ||
		ct == ArvanCloudConnectionTypeAmazonS3 || ct == ArvanCloudConnectionTypeCustomS3
}

// ArvanCloudLogForwarderSyslogType is LogForwarderSyslogConnectionType's
// "logtype" enum: which transport a syslog destination uses.
type ArvanCloudLogForwarderSyslogType string

const (
	ArvanCloudLogForwarderSyslogUDP ArvanCloudLogForwarderSyslogType = "syslogudp"
	ArvanCloudLogForwarderSyslogTCP ArvanCloudLogForwarderSyslogType = "syslogtcp"
)

var arvanCloudLogForwarderSyslogTypes = []string{
	string(ArvanCloudLogForwarderSyslogUDP), string(ArvanCloudLogForwarderSyslogTCP),
}

// ValidArvanCloudLogForwarderSyslogType reports whether s is one of
// LogForwarderSyslogConnectionType.logtype's two values.
func ValidArvanCloudLogForwarderSyslogType(s string) bool {
	return contains(arvanCloudLogForwarderSyslogTypes, s)
}

// ArvanCloudLogForwarderS3Settings mirrors LogForwarderS3ConnectionType:
// settings shared by the ArvanS3/AlibabaS3/AmazonS3/CustomS3 connection
// types (isArvanCloudS3ConnectionType). AccessKey and SecretKey are the
// destination bucket's own credentials — sensitive, see this file's package
// comment.
type ArvanCloudLogForwarderS3Settings struct {
	SampleRate    int
	S3Endpoint    string
	AccessKey     string
	SecretKey     string
	BucketName    string
	ObjectSize    int
	FlushInterval int
}

// ArvanCloudLogForwarderDatadogSettings mirrors
// LogForwarderDatadogConnectionType. APIKey and AppKey are Datadog's own
// credentials — sensitive, see this file's package comment.
type ArvanCloudLogForwarderDatadogSettings struct {
	SampleRate    int
	URL           string
	APIKey        string
	AppKey        string
	FlushInterval int
	BufferSize    int
}

// ArvanCloudLogForwarderKafkaSettings mirrors
// LogForwarderKafkaConnectionType. KafkaBrokers entries are "host:port"
// strings per the spec's own example (e.g. "example.com:9092").
type ArvanCloudLogForwarderKafkaSettings struct {
	SampleRate                    int
	KafkaVersion                  string
	KafkaBrokers                  []string
	KafkaTopicToWrite             string
	KafkaProducerBatchSize        int
	KafkaProducerFlushFrequencyMS int
}

// ArvanCloudLogForwarderLogglySettings mirrors
// LogForwarderLogglyConnectionType. Token is Loggly's own credential —
// sensitive, see this file's package comment.
type ArvanCloudLogForwarderLogglySettings struct {
	SampleRate    int
	Token         string
	URL           string
	FlushInterval int
	BufferSize    int
}

// ArvanCloudLogForwarderSyslogSettings mirrors
// LogForwarderSyslogConnectionType. LogType must be one of
// ValidArvanCloudLogForwarderSyslogType's values. Cert, when set, is
// forwarded verbatim; treated as sensitive alongside the rest of this
// LogForwarder's settings even though the spec does not itself flag it
// (this file's package comment redacts the whole settings object, not only
// the fields ArvanCloud's own spec happens to call out).
type ArvanCloudLogForwarderSyslogSettings struct {
	SampleRate int
	LogType    ArvanCloudLogForwarderSyslogType
	Host       string
	Port       int
	TLS        bool
	Cert       string
	RetryTime  int
}

// ArvanCloudLogForwarderArvanLogsSettings mirrors
// LogForwarderArvanLogsConnectionType — ArvanCloud's own log storage, the
// only connection type with no external credential of any kind.
type ArvanCloudLogForwarderArvanLogsSettings struct {
	SampleRate int
}

// ArvanCloudLogForwarderSettings is LogForwarder.settings: a tagged union
// keyed by the parent LogForwarder's ConnectionType, mirroring the treatment
// ArvanCloudHealthCheckRequestConfig gives request_config (picked by the
// parent's Type) rather than one flat struct carrying every connection
// type's fields as optional strings (see this file's package comment).
// Exactly one field is populated, selected by ConnectionType; the others are
// nil. The whole struct is sensitive — see this file's package comment.
type ArvanCloudLogForwarderSettings struct {
	S3        *ArvanCloudLogForwarderS3Settings
	Datadog   *ArvanCloudLogForwarderDatadogSettings
	Kafka     *ArvanCloudLogForwarderKafkaSettings
	Loggly    *ArvanCloudLogForwarderLogglySettings
	Syslog    *ArvanCloudLogForwarderSyslogSettings
	ArvanLogs *ArvanCloudLogForwarderArvanLogsSettings
}

// ArvanCloudMaskRule mirrors MaskRule: a regex-based redaction pass applied
// to a log line before it is forwarded (LogForwarder.mask_rules), unrelated
// to this file's own package-comment redaction of the settings object
// itself — this one masks PII/sensitive data INSIDE the forwarded log
// content, at ArvanCloud's end, not this server's logging.
type ArvanCloudMaskRule struct {
	// Pattern is the regular expression matched against a log line.
	Pattern string
	// Replace is the replacement expression, e.g. "${1}*****${2}" to keep a
	// capture group's surrounding context while masking the group itself.
	Replace string
}

// ArvanCloudLogForwarderDataFields is LogForwarder.data_fields: which fields
// to include in a forwarded log line. The spec's LogForwarderDataFields is a
// oneOf across five per-type boolean-toggle schemas
// (LogForwarderAccessLogType/WAFType/DNSType/ErrorType/EventType); this is
// their flattened union, the same treatment ArvanCloudDNSRecordValue gives
// DNS's per-record-type value shape (see this file's package comment).
// Every field's doc comment names which LogForwarderType value(s) it
// applies to; a field is meaningless (and should be left false) for any
// other type. RecordType and WAFEvent are renamed from the spec's own
// "type"/"waf" JSON keys to avoid colliding with this package's own Type/WAF
// identifiers.
type ArvanCloudLogForwarderDataFields struct {
	// Method. access, event.
	Method bool
	// Scheme. access, event.
	Scheme bool
	// Domain. access, waf, event.
	Domain bool
	// URI. access, event.
	URI bool
	// QueryString. access, event.
	QueryString bool
	// Referer. access only.
	Referer bool
	// IP. access, dns, event.
	IP bool
	// UserAgent (spec key "ua"). access only.
	UserAgent bool
	// Country. access, dns, event.
	Country bool
	// ASN. access, dns.
	ASN bool
	// NodeID. access only.
	NodeID bool
	// ContentType. access only.
	ContentType bool
	// Status. access, event.
	Status bool
	// ServerIP. access, event.
	ServerIP bool
	// ServerPort. access, event.
	ServerPort bool
	// BytesSent. access only.
	BytesSent bool
	// BytesReceived. access only.
	BytesReceived bool
	// UpstreamTime. access only.
	UpstreamTime bool
	// Cache. access only.
	Cache bool
	// RequestID. access, error.
	RequestID bool
	// Timestamp. access, waf, dns, event.
	Timestamp bool
	// ISOTimestamp. access, waf, dns, event, error.
	ISOTimestamp bool
	// TLSFingerprint. access, event.
	TLSFingerprint bool
	// UpstreamAddr. access only.
	UpstreamAddr bool
	// Product. waf only.
	Product bool
	// RemoteAddress. waf only.
	RemoteAddress bool
	// Data. waf only.
	Data bool
	// UUID. waf, event.
	UUID bool
	// Record. dns only.
	Record bool
	// RecordType is the spec's DNS-log "type" field (the record's DNS type,
	// e.g. "A"/"CNAME") — renamed to avoid colliding with this package's own
	// Type identifiers. dns only.
	RecordType bool
	// ResponseCode. dns only.
	ResponseCode bool
	// ProcessTime. dns only.
	ProcessTime bool
	// TotalTime. event only.
	TotalTime bool
	// General. event only.
	General bool
	// Firewall. event only.
	Firewall bool
	// Proxy. event only.
	Proxy bool
	// DNSResolver. event only.
	DNSResolver bool
	// DDoS. event only.
	DDoS bool
	// RateLimit. event only.
	RateLimit bool
	// WAFEvent is the spec's event-log "waf" field (whether WAF acted on this
	// event) — renamed to avoid colliding with ArvanCloudLogForwarderTypeWAF.
	// event only.
	WAFEvent bool
	// ClientIP. error only.
	ClientIP bool
	// UpstreamProto. error only.
	UpstreamProto bool
	// UpstreamURI. error only.
	UpstreamURI bool
	// UpstreamPort. error only.
	UpstreamPort bool
	// UpstreamIP. error only.
	UpstreamIP bool
	// DomainName. error only.
	DomainName bool
	// HTTPVersion. error only.
	HTTPVersion bool
	// RequestMethod. error only.
	RequestMethod bool
	// RequestURI. error only.
	RequestURI bool
	// RealTimestamp. error only.
	RealTimestamp bool
	// ErrorMessage. error only.
	ErrorMessage bool
	// PopSite. error only.
	PopSite bool
}

// ArvanCloudLogForwarder is a domain-scoped log forwarder
// (/domains/{domain}/log-forwarders[/{id}], the LogForwarder request schema
// / LogForwarderGeneric response schema): configures ArvanCloud to push one
// log stream to an external destination. See this file's package comment
// for how Type/ConnectionType/DataFields/Settings resolve the spec's
// per-type and per-connection-type shape variance.
type ArvanCloudLogForwarder struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string
	// Name is a caller-supplied label.
	Name string
	// Description is a caller-supplied note.
	Description string
	// Type selects which log stream this forwards. Must be one of
	// ValidArvanCloudLogForwarderType's five values.
	Type ArvanCloudLogForwarderType
	// ConnectionType selects the destination and which of Settings' six
	// pointer fields is populated. Must be one of
	// ValidArvanCloudConnectionType's nine values.
	ConnectionType ArvanCloudConnectionType
	// DataFormatExpr is an Expr-language expression selecting which log
	// lines to forward, max 5000 chars per the spec (e.g. "Domain ==
	// 'arvancloud.ir' && Method == 'GET' && Scheme == 'HTTPS'").
	DataFormatExpr string
	// DataFields selects which fields a forwarded log line includes. See
	// ArvanCloudLogForwarderDataFields' own doc comment for which fields
	// apply to which Type.
	DataFields ArvanCloudLogForwarderDataFields
	// MaskRules redacts PII/sensitive data inside forwarded log lines,
	// applied at ArvanCloud's end before the data leaves — not related to
	// this file's own settings-redaction concern (see
	// ArvanCloudMaskRule's own doc comment).
	MaskRules []ArvanCloudMaskRule
	// Settings holds the destination's connection details, sensitive — see
	// this file's package comment. Exactly the branch ConnectionType selects
	// is meaningful.
	Settings ArvanCloudLogForwarderSettings
	// Status is whether the forwarder is active.
	Status bool
}

// ArvanCloudMetricExporterType is which metric group a MetricExporter
// exports (MetricExporter.type). Confirmed against both the MetricExporter
// schema's enum and the MetricExporterTypes query parameter's enum: four
// values — narrower than ArvanCloudLogForwarderType's five (no "waf" value
// here; see this file's package comment for why that is not an oversight).
type ArvanCloudMetricExporterType string

const (
	ArvanCloudMetricExporterTypeAccess ArvanCloudMetricExporterType = "access"
	ArvanCloudMetricExporterTypeDNS    ArvanCloudMetricExporterType = "dns"
	ArvanCloudMetricExporterTypeError  ArvanCloudMetricExporterType = "error"
	ArvanCloudMetricExporterTypeEvent  ArvanCloudMetricExporterType = "event"
)

var arvanCloudMetricExporterTypes = []string{
	string(ArvanCloudMetricExporterTypeAccess), string(ArvanCloudMetricExporterTypeDNS),
	string(ArvanCloudMetricExporterTypeError), string(ArvanCloudMetricExporterTypeEvent),
}

// ValidArvanCloudMetricExporterType reports whether s is one of
// MetricExporter.type's four values.
func ValidArvanCloudMetricExporterType(s string) bool {
	return contains(arvanCloudMetricExporterTypes, s)
}

// ArvanCloudMetricExporterInterval is MetricExporter.interval: how often the
// exporter reports metrics. Confirmed against the MetricExporter schema's
// enum: exactly three fixed values, all plain seconds-suffixed strings, not
// a numeric field.
type ArvanCloudMetricExporterInterval string

const (
	ArvanCloudMetricExporterInterval10s ArvanCloudMetricExporterInterval = "10s"
	ArvanCloudMetricExporterInterval30s ArvanCloudMetricExporterInterval = "30s"
	ArvanCloudMetricExporterInterval60s ArvanCloudMetricExporterInterval = "60s"
)

var arvanCloudMetricExporterIntervals = []string{
	string(ArvanCloudMetricExporterInterval10s), string(ArvanCloudMetricExporterInterval30s),
	string(ArvanCloudMetricExporterInterval60s),
}

// ValidArvanCloudMetricExporterInterval reports whether s is one of
// MetricExporter.interval's three values.
func ValidArvanCloudMetricExporterInterval(s string) bool {
	return contains(arvanCloudMetricExporterIntervals, s)
}

// ArvanCloudMetricExporter is a metric exporter (create/get/update/delete/
// status scoped to /domains/{domain}/metric-exporters[/{id}]; list is
// account-wide at /metric-exporters — see this file's package comment for
// that asymmetry and how Domain resolves it).
type ArvanCloudMetricExporter struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string
	// Name is a caller-supplied label.
	Name string
	// Type selects which metric group this exports. Must be one of
	// ValidArvanCloudMetricExporterType's four values.
	Type ArvanCloudMetricExporterType
	// URL is the provider-assigned endpoint metrics are exposed at.
	// Read-only, and populated only by responses that carry it
	// (MetricExporterResponse's data — get/create/update/status); the
	// account-wide list endpoint's MetricExporterSummary items also carry
	// it.
	URL string
	// Domain is the domain this exporter belongs to. Read-only, and
	// populated ONLY by the account-wide list endpoint
	// (metric-exporters.index's MetricExporterSummary, which is the only
	// response shape carrying a "domain" field) — always empty on the result
	// of create/get/update/status, which are already scoped to one domain by
	// the caller. See this file's package comment for why the list endpoint
	// needs this field at all.
	Domain string
	// Interval is how often metrics are reported. Must be one of
	// ValidArvanCloudMetricExporterInterval's three values.
	Interval ArvanCloudMetricExporterInterval
	// Status is whether the exporter is active.
	Status bool
}

// ArvanCloudLogForwarderListQuery is ListArvanCloudLogForwarders' filter set
// (log-forwarders.index's query parameters: name, types, plus the shared
// per_page/page).
type ArvanCloudLogForwarderListQuery struct {
	// Name filters by name (substring match per the spec's own description
	// convention for similar Name filters elsewhere). Empty means no filter.
	Name string
	// Types filters to forwarders of these LogForwarderType values. Empty
	// means no filter (every type).
	Types []string
	// Page is the 1-indexed page to return. Zero means the provider's own
	// default (page 1).
	Page int
	// PerPage is how many items per page. Zero means the provider's own
	// default.
	PerPage int
}

// ArvanCloudMetricExporterListQuery is ListArvanCloudMetricExporters' filter
// set (metric-exporters.index's query parameters: name, domain,
// metric_exporter_id, types, plus the shared per_page/page). This endpoint
// is account-wide — see this file's package comment.
type ArvanCloudMetricExporterListQuery struct {
	// Name filters by name. Empty means no filter.
	Name string
	// Domain filters to one domain's exporters (the DomainQuery parameter:
	// a domain name or its provider ID). Empty means every domain visible
	// to the credentials.
	Domain string
	// MetricExporterID filters to one exporter by ID (the
	// MetricExporterQuery parameter). Empty means no filter.
	MetricExporterID string
	// Types filters to exporters of these MetricExporterType values. Empty
	// means no filter (every type).
	Types []string
	// Page is the 1-indexed page to return. Zero means the provider's own
	// default (page 1).
	Page int
	// PerPage is how many items per page. Zero means the provider's own
	// default.
	PerPage int
}

// ArvanCloudMetricExporterMetricItem is one entry of a metric group's
// "items" (metric-exporters.metrics.index's MetricExporterMetrics.data[].items[]).
type ArvanCloudMetricExporterMetricItem struct {
	Name        string
	Description string
}

// ArvanCloudMetricExporterMetricGroup is one metric group
// (MetricExporterMetrics.data[]): the set of individual metrics available
// under one MetricExporterType.
type ArvanCloudMetricExporterMetricGroup struct {
	// Metric is the group's name — one of ValidArvanCloudMetricExporterType's
	// values per the spec's own enum on this field, confirmed identical to
	// MetricExporter.type's four values (access/dns/error/event).
	Metric string
	Items  []ArvanCloudMetricExporterMetricItem
}

// ArvanCloudMetricExporterMetrics is ListArvanCloudMetricExporterTypes'
// result (metric-exporters.metrics.index, the MetricExporterMetrics schema):
// the catalog of metric groups and their individual metrics available to
// choose from when creating a metric exporter.
type ArvanCloudMetricExporterMetrics struct {
	Groups  []ArvanCloudMetricExporterMetricGroup
	Message string
}
