package arvancloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Log Forwarders and Metric Exporters (issue #76), wired to the real CDN
// API: both push data to an external system (S3-compatible storage,
// Datadog, Kafka, syslog, ...) rather than exposing it through Reports'
// (reports.go, issue #75) own GET endpoints. Base paths are confirmed
// against docs/api-specs/arvancloud-cdn-4.0.yml's "Log Forwarders" and
// "Metric Exporters" tags. Log Forwarders is entirely domain-scoped,
// relative to domainPath (defined in domain.go); Metric Exporters mixes an
// account-wide list/metrics-catalog pair with domain-scoped create/get/
// update/delete/status — see domain/arvancloud_observability.go's package
// comment for that asymmetry.
//
// Response-shape quirk worth flagging: LogForwarderGeneric (the response
// schema for get/create/update/status) and LogForwarderSummary (the list
// item shape) both omit data_fields and mask_rules entirely — the provider
// does not echo either back. toLogForwarderDomain therefore always decodes
// those two fields as their Go zero value; a caller wanting to know a
// forwarder's current data_fields/mask_rules has to keep its own record of
// what it last sent, not read it back from this API.
//
// settings sensitivity: every LogForwarderSetting variant carries the
// external destination's own credentials (S3 access/secret keys, a Datadog
// API key, ...) — see domain/arvancloud_observability.go's package comment.
// This adapter never logs a request body — the shared client's debug log
// (client.go's roundTrip) only ever logs the method, URL and redacted
// headers — and no method below embeds a settings field in any fmt.Errorf
// message. See observability_test.go's redaction tests, the same style as
// ddos_test.go's TestUpdateArvanCloudDdosSettingsNeverLogsSecretKey.

const logForwardersPathSuffix = "/log-forwarders"

func logForwardersPath(domainName string) string {
	return domainPath(domainName) + logForwardersPathSuffix
}
func logForwarderPath(domainName, id string) string {
	return logForwardersPath(domainName) + "/" + id
}
func logForwarderStatusPath(domainName, id string) string {
	return logForwarderPath(domainName, id) + "/status"
}

// metricExportersPath is account-wide, unlike every other path in this
// section — no /domains/{domain} prefix (metric-exporters.index).
const metricExportersPath = "metric-exporters"

// metricExporterMetricsPath is also account-wide
// (metric-exporters.metrics.index).
const metricExporterMetricsPath = "metric-exporters/metrics"

func domainMetricExportersPath(domainName string) string {
	return domainPath(domainName) + "/metric-exporters"
}
func domainMetricExporterPath(domainName, id string) string {
	return domainMetricExportersPath(domainName) + "/" + id
}
func domainMetricExporterStatusPath(domainName, id string) string {
	return domainMetricExporterPath(domainName, id) + "/status"
}

// --- Log Forwarders: data_fields -----------------------------------------

// logForwarderDataFieldsWire mirrors the union of
// LogForwarderAccessLogType/WAFType/DNSType/ErrorType/EventType: one struct
// covering every field across all five, decoded/encoded generically and
// narrowed to the meaningful subset by the parent LogForwarder's Type — the
// same "one wire struct, meaning selected by the parent's own type field"
// choice healthCheckRequestConfigWire makes for HttpConfig/TcpConfig.
type logForwarderDataFieldsWire struct {
	Method         bool `json:"method,omitempty"`
	Scheme         bool `json:"scheme,omitempty"`
	Domain         bool `json:"domain,omitempty"`
	URI            bool `json:"uri,omitempty"`
	QueryString    bool `json:"query_string,omitempty"`
	Referer        bool `json:"referer,omitempty"`
	IP             bool `json:"ip,omitempty"`
	UA             bool `json:"ua,omitempty"`
	Country        bool `json:"country,omitempty"`
	ASN            bool `json:"asn,omitempty"`
	NodeID         bool `json:"node_id,omitempty"`
	ContentType    bool `json:"content_type,omitempty"`
	Status         bool `json:"status,omitempty"`
	ServerIP       bool `json:"server_ip,omitempty"`
	ServerPort     bool `json:"server_port,omitempty"`
	BytesSent      bool `json:"bytes_sent,omitempty"`
	BytesReceived  bool `json:"bytes_received,omitempty"`
	UpstreamTime   bool `json:"upstream_time,omitempty"`
	Cache          bool `json:"cache,omitempty"`
	RequestID      bool `json:"request_id,omitempty"`
	Timestamp      bool `json:"timestamp,omitempty"`
	ISOTimestamp   bool `json:"iso_timestamp,omitempty"`
	TLSFingerprint bool `json:"tls_fingerprint,omitempty"`
	UpstreamAddr   bool `json:"upstream_addr,omitempty"`
	Product        bool `json:"product,omitempty"`
	RemoteAddress  bool `json:"remote_address,omitempty"`
	Data           bool `json:"data,omitempty"`
	UUID           bool `json:"uuid,omitempty"`
	Record         bool `json:"record,omitempty"`
	Type           bool `json:"type,omitempty"`
	ResponseCode   bool `json:"response_code,omitempty"`
	ProcessTime    bool `json:"process_time,omitempty"`
	TotalTime      bool `json:"total_time,omitempty"`
	General        bool `json:"general,omitempty"`
	Firewall       bool `json:"firewall,omitempty"`
	Proxy          bool `json:"proxy,omitempty"`
	DNSResolver    bool `json:"dns_resolver,omitempty"`
	DDoS           bool `json:"ddos,omitempty"`
	RateLimit      bool `json:"ratelimit,omitempty"`
	WAF            bool `json:"waf,omitempty"`
	ClientIP       bool `json:"client_ip,omitempty"`
	UpstreamProto  bool `json:"upstream_proto,omitempty"`
	UpstreamURI    bool `json:"upstream_uri,omitempty"`
	UpstreamPort   bool `json:"upstream_port,omitempty"`
	UpstreamIP     bool `json:"upstream_ip,omitempty"`
	DomainName     bool `json:"domain_name,omitempty"`
	HTTPVersion    bool `json:"http_version,omitempty"`
	RequestMethod  bool `json:"request_method,omitempty"`
	RequestURI     bool `json:"request_uri,omitempty"`
	RealTimestamp  bool `json:"real_timestamp,omitempty"`
	ErrorMessage   bool `json:"error_message,omitempty"`
	PopSite        bool `json:"pop_site,omitempty"`
}

func toLogForwarderDataFieldsDomain(w *logForwarderDataFieldsWire) domain.ArvanCloudLogForwarderDataFields {
	if w == nil {
		return domain.ArvanCloudLogForwarderDataFields{}
	}
	return domain.ArvanCloudLogForwarderDataFields{
		Method: w.Method, Scheme: w.Scheme, Domain: w.Domain, URI: w.URI, QueryString: w.QueryString,
		Referer: w.Referer, IP: w.IP, UserAgent: w.UA, Country: w.Country, ASN: w.ASN, NodeID: w.NodeID,
		ContentType: w.ContentType, Status: w.Status, ServerIP: w.ServerIP, ServerPort: w.ServerPort,
		BytesSent: w.BytesSent, BytesReceived: w.BytesReceived, UpstreamTime: w.UpstreamTime, Cache: w.Cache,
		RequestID: w.RequestID, Timestamp: w.Timestamp, ISOTimestamp: w.ISOTimestamp,
		TLSFingerprint: w.TLSFingerprint, UpstreamAddr: w.UpstreamAddr, Product: w.Product,
		RemoteAddress: w.RemoteAddress, Data: w.Data, UUID: w.UUID, Record: w.Record, RecordType: w.Type,
		ResponseCode: w.ResponseCode, ProcessTime: w.ProcessTime, TotalTime: w.TotalTime, General: w.General,
		Firewall: w.Firewall, Proxy: w.Proxy, DNSResolver: w.DNSResolver, DDoS: w.DDoS, RateLimit: w.RateLimit,
		WAFEvent: w.WAF, ClientIP: w.ClientIP, UpstreamProto: w.UpstreamProto, UpstreamURI: w.UpstreamURI,
		UpstreamPort: w.UpstreamPort, UpstreamIP: w.UpstreamIP, DomainName: w.DomainName,
		HTTPVersion: w.HTTPVersion, RequestMethod: w.RequestMethod, RequestURI: w.RequestURI,
		RealTimestamp: w.RealTimestamp, ErrorMessage: w.ErrorMessage, PopSite: w.PopSite,
	}
}

// logForwarderDataFieldsRequestBody builds data_fields' request value,
// including only the fields the caller set to true — matching the spec's
// own example ({"method": true}), which shows only positively-selected
// fields, not a full explicit true/false map.
func logForwarderDataFieldsRequestBody(d domain.ArvanCloudLogForwarderDataFields) map[string]any {
	body := map[string]any{}
	add := func(key string, v bool) {
		if v {
			body[key] = true
		}
	}
	add("method", d.Method)
	add("scheme", d.Scheme)
	add("domain", d.Domain)
	add("uri", d.URI)
	add("query_string", d.QueryString)
	add("referer", d.Referer)
	add("ip", d.IP)
	add("ua", d.UserAgent)
	add("country", d.Country)
	add("asn", d.ASN)
	add("node_id", d.NodeID)
	add("content_type", d.ContentType)
	add("status", d.Status)
	add("server_ip", d.ServerIP)
	add("server_port", d.ServerPort)
	add("bytes_sent", d.BytesSent)
	add("bytes_received", d.BytesReceived)
	add("upstream_time", d.UpstreamTime)
	add("cache", d.Cache)
	add("request_id", d.RequestID)
	add("timestamp", d.Timestamp)
	add("iso_timestamp", d.ISOTimestamp)
	add("tls_fingerprint", d.TLSFingerprint)
	add("upstream_addr", d.UpstreamAddr)
	add("product", d.Product)
	add("remote_address", d.RemoteAddress)
	add("data", d.Data)
	add("uuid", d.UUID)
	add("record", d.Record)
	add("type", d.RecordType)
	add("response_code", d.ResponseCode)
	add("process_time", d.ProcessTime)
	add("total_time", d.TotalTime)
	add("general", d.General)
	add("firewall", d.Firewall)
	add("proxy", d.Proxy)
	add("dns_resolver", d.DNSResolver)
	add("ddos", d.DDoS)
	add("ratelimit", d.RateLimit)
	add("waf", d.WAFEvent)
	add("client_ip", d.ClientIP)
	add("upstream_proto", d.UpstreamProto)
	add("upstream_uri", d.UpstreamURI)
	add("upstream_port", d.UpstreamPort)
	add("upstream_ip", d.UpstreamIP)
	add("domain_name", d.DomainName)
	add("http_version", d.HTTPVersion)
	add("request_method", d.RequestMethod)
	add("request_uri", d.RequestURI)
	add("real_timestamp", d.RealTimestamp)
	add("error_message", d.ErrorMessage)
	add("pop_site", d.PopSite)
	return body
}

// --- Log Forwarders: settings ---------------------------------------------

// logForwarderSettingsWire mirrors the union of
// LogForwarderS3ConnectionType/DatadogConnectionType/KafkaConnectionType/
// LogglyConnectionType/SyslogConnectionType/ArvanLogsConnectionType — one
// wire struct, meaning selected by the parent LogForwarder's ConnectionType,
// the same choice logForwarderDataFieldsWire makes for data_fields.
type logForwarderSettingsWire struct {
	SampleRate                    int      `json:"sample_rate,omitempty"`
	S3Endpoint                    string   `json:"s3_endpoint,omitempty"`
	AccessKey                     string   `json:"access_key,omitempty"`
	SecretKey                     string   `json:"secret_key,omitempty"`
	BucketName                    string   `json:"bucket_name,omitempty"`
	ObjectSize                    int      `json:"object_size,omitempty"`
	FlushInterval                 int      `json:"flush_interval,omitempty"`
	URL                           string   `json:"url,omitempty"`
	APIKey                        string   `json:"api_key,omitempty"`
	AppKey                        string   `json:"app_key,omitempty"`
	BufferSize                    int      `json:"buffer_size,omitempty"`
	KafkaVersion                  string   `json:"kafka_version,omitempty"`
	KafkaBrokers                  []string `json:"kafka_brokers,omitempty"`
	KafkaTopicToWrite             string   `json:"kafka_topic_to_write,omitempty"`
	KafkaProducerBatchSize        int      `json:"kafka_producer_batch_size,omitempty"`
	KafkaProducerFlushFrequencyMs int      `json:"kafka_producer_flush_frequency_ms,omitempty"`
	Token                         string   `json:"token,omitempty"`
	LogType                       string   `json:"logtype,omitempty"`
	Host                          string   `json:"host,omitempty"`
	Port                          int      `json:"port,omitempty"`
	TLS                           bool     `json:"tls,omitempty"`
	Cert                          string   `json:"cert,omitempty"`
	RetryTime                     int      `json:"retry_time,omitempty"`
}

// toLogForwarderSettingsDomain builds the tagged domain.ArvanCloudLogForwarderSettings
// union, selecting which of its six branches to populate by connectionType —
// the parent LogForwarder.ConnectionType, since the wire settings object
// itself carries no shape marker of its own.
func toLogForwarderSettingsDomain(connectionType domain.ArvanCloudConnectionType, w *logForwarderSettingsWire) domain.ArvanCloudLogForwarderSettings {
	if w == nil {
		return domain.ArvanCloudLogForwarderSettings{}
	}
	switch {
	case domain.IsArvanCloudS3ConnectionType(connectionType):
		return domain.ArvanCloudLogForwarderSettings{S3: &domain.ArvanCloudLogForwarderS3Settings{
			SampleRate: w.SampleRate, S3Endpoint: w.S3Endpoint, AccessKey: w.AccessKey, SecretKey: w.SecretKey,
			BucketName: w.BucketName, ObjectSize: w.ObjectSize, FlushInterval: w.FlushInterval,
		}}
	case connectionType == domain.ArvanCloudConnectionTypeDatadog:
		return domain.ArvanCloudLogForwarderSettings{Datadog: &domain.ArvanCloudLogForwarderDatadogSettings{
			SampleRate: w.SampleRate, URL: w.URL, APIKey: w.APIKey, AppKey: w.AppKey,
			FlushInterval: w.FlushInterval, BufferSize: w.BufferSize,
		}}
	case connectionType == domain.ArvanCloudConnectionTypeKafka:
		return domain.ArvanCloudLogForwarderSettings{Kafka: &domain.ArvanCloudLogForwarderKafkaSettings{
			SampleRate: w.SampleRate, KafkaVersion: w.KafkaVersion, KafkaBrokers: w.KafkaBrokers,
			KafkaTopicToWrite: w.KafkaTopicToWrite, KafkaProducerBatchSize: w.KafkaProducerBatchSize,
			KafkaProducerFlushFrequencyMS: w.KafkaProducerFlushFrequencyMs,
		}}
	case connectionType == domain.ArvanCloudConnectionTypeLoggly:
		return domain.ArvanCloudLogForwarderSettings{Loggly: &domain.ArvanCloudLogForwarderLogglySettings{
			SampleRate: w.SampleRate, Token: w.Token, URL: w.URL, FlushInterval: w.FlushInterval, BufferSize: w.BufferSize,
		}}
	case connectionType == domain.ArvanCloudConnectionTypeSyslog:
		return domain.ArvanCloudLogForwarderSettings{Syslog: &domain.ArvanCloudLogForwarderSyslogSettings{
			SampleRate: w.SampleRate, LogType: domain.ArvanCloudLogForwarderSyslogType(w.LogType), Host: w.Host,
			Port: w.Port, TLS: w.TLS, Cert: w.Cert, RetryTime: w.RetryTime,
		}}
	case connectionType == domain.ArvanCloudConnectionTypeArvanLogs:
		return domain.ArvanCloudLogForwarderSettings{ArvanLogs: &domain.ArvanCloudLogForwarderArvanLogsSettings{
			SampleRate: w.SampleRate,
		}}
	default:
		return domain.ArvanCloudLogForwarderSettings{}
	}
}

// logForwarderSettingsRequestBody builds settings' request value from
// whichever branch of s is populated.
func logForwarderSettingsRequestBody(s domain.ArvanCloudLogForwarderSettings) map[string]any {
	switch {
	case s.S3 != nil:
		v := s.S3
		body := map[string]any{
			"s3_endpoint": v.S3Endpoint, "access_key": v.AccessKey, "secret_key": v.SecretKey,
			"bucket_name": v.BucketName,
		}
		if v.SampleRate > 0 {
			body["sample_rate"] = v.SampleRate
		}
		if v.ObjectSize > 0 {
			body["object_size"] = v.ObjectSize
		}
		if v.FlushInterval > 0 {
			body["flush_interval"] = v.FlushInterval
		}
		return body
	case s.Datadog != nil:
		v := s.Datadog
		body := map[string]any{"url": v.URL, "api_key": v.APIKey}
		if v.AppKey != "" {
			body["app_key"] = v.AppKey
		}
		if v.SampleRate > 0 {
			body["sample_rate"] = v.SampleRate
		}
		if v.FlushInterval > 0 {
			body["flush_interval"] = v.FlushInterval
		}
		if v.BufferSize > 0 {
			body["buffer_size"] = v.BufferSize
		}
		return body
	case s.Kafka != nil:
		v := s.Kafka
		body := map[string]any{"kafka_brokers": v.KafkaBrokers, "kafka_topic_to_write": v.KafkaTopicToWrite}
		if v.KafkaVersion != "" {
			body["kafka_version"] = v.KafkaVersion
		}
		if v.SampleRate > 0 {
			body["sample_rate"] = v.SampleRate
		}
		if v.KafkaProducerBatchSize > 0 {
			body["kafka_producer_batch_size"] = v.KafkaProducerBatchSize
		}
		if v.KafkaProducerFlushFrequencyMS > 0 {
			body["kafka_producer_flush_frequency_ms"] = v.KafkaProducerFlushFrequencyMS
		}
		return body
	case s.Loggly != nil:
		v := s.Loggly
		body := map[string]any{"token": v.Token, "url": v.URL}
		if v.SampleRate > 0 {
			body["sample_rate"] = v.SampleRate
		}
		if v.FlushInterval > 0 {
			body["flush_interval"] = v.FlushInterval
		}
		if v.BufferSize > 0 {
			body["buffer_size"] = v.BufferSize
		}
		return body
	case s.Syslog != nil:
		v := s.Syslog
		body := map[string]any{
			"logtype": string(v.LogType), "host": v.Host, "port": v.Port, "tls": v.TLS,
		}
		if v.SampleRate > 0 {
			body["sample_rate"] = v.SampleRate
		}
		if v.Cert != "" {
			body["cert"] = v.Cert
		}
		if v.RetryTime > 0 {
			body["retry_time"] = v.RetryTime
		}
		return body
	case s.ArvanLogs != nil:
		body := map[string]any{}
		if s.ArvanLogs.SampleRate > 0 {
			body["sample_rate"] = s.ArvanLogs.SampleRate
		}
		return body
	default:
		return map[string]any{}
	}
}

// --- Log Forwarders: entity ------------------------------------------------

// logForwarderMaskRuleWire mirrors MaskRule.
type logForwarderMaskRuleWire struct {
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
}

// logForwarderWire mirrors LogForwarder (request) / LogForwarderGeneric
// (response) — see this file's package comment for the fields
// LogForwarderGeneric never actually echoes back (DataFields, MaskRules).
type logForwarderWire struct {
	ID             string                      `json:"id,omitempty"`
	Name           string                      `json:"name,omitempty"`
	Description    string                      `json:"description,omitempty"`
	Type           string                      `json:"type,omitempty"`
	ConnectionType string                      `json:"connection_type,omitempty"`
	DataFormatExpr string                      `json:"data_format_expr,omitempty"`
	DataFields     *logForwarderDataFieldsWire `json:"data_fields,omitempty"`
	MaskRules      []logForwarderMaskRuleWire  `json:"mask_rules,omitempty"`
	Settings       *logForwarderSettingsWire   `json:"settings,omitempty"`
	Status         *bool                       `json:"status,omitempty"`
}

func toLogForwarderDomain(w logForwarderWire) domain.ArvanCloudLogForwarder {
	connectionType := domain.ArvanCloudConnectionType(w.ConnectionType)
	lf := domain.ArvanCloudLogForwarder{
		ID: w.ID, Name: w.Name, Description: w.Description,
		Type: domain.ArvanCloudLogForwarderType(w.Type), ConnectionType: connectionType,
		DataFormatExpr: w.DataFormatExpr,
		DataFields:     toLogForwarderDataFieldsDomain(w.DataFields),
		Settings:       toLogForwarderSettingsDomain(connectionType, w.Settings),
	}
	if w.Status != nil {
		lf.Status = *w.Status
	}
	if len(w.MaskRules) > 0 {
		lf.MaskRules = make([]domain.ArvanCloudMaskRule, len(w.MaskRules))
		for i, r := range w.MaskRules {
			lf.MaskRules[i] = domain.ArvanCloudMaskRule{Pattern: r.Pattern, Replace: r.Replace}
		}
	}
	return lf
}

// logForwarderRequestBody builds the JSON body for a log forwarder
// create/update, sent as a plain map so status's explicit false reaches the
// provider (the same reasoning healthCheckRequestBody documents).
func logForwarderRequestBody(lf domain.ArvanCloudLogForwarder) map[string]any {
	body := map[string]any{
		"name": lf.Name, "description": lf.Description, "type": string(lf.Type),
		"connection_type": string(lf.ConnectionType), "settings": logForwarderSettingsRequestBody(lf.Settings),
		"status": lf.Status,
	}
	if lf.DataFormatExpr != "" {
		body["data_format_expr"] = lf.DataFormatExpr
	}
	if fields := logForwarderDataFieldsRequestBody(lf.DataFields); len(fields) > 0 {
		body["data_fields"] = fields
	}
	if len(lf.MaskRules) > 0 {
		rules := make([]map[string]any, len(lf.MaskRules))
		for i, r := range lf.MaskRules {
			rules[i] = map[string]any{"pattern": r.Pattern, "replace": r.Replace}
		}
		body["mask_rules"] = rules
	}
	return body
}

// logForwarderListEnvelope mirrors the PaginatedResponse shape
// log-forwarders.index returns — data plus meta at the TOP LEVEL of the
// response body, the same situation highRequestIPsEnvelope documents for
// reports.go.
type logForwarderListEnvelope struct {
	Data []logForwarderWire        `json:"data"`
	Meta paginatedResponseMetaWire `json:"meta"`
}

// logForwarderListQueryValues builds log-forwarders.index's query
// parameters. Types is sent as repeated same-key "types" values (OpenAPI's
// default form/explode=true query array style) since the spec declares no
// explicit style/explode for this parameter.
func logForwarderListQueryValues(q domain.ArvanCloudLogForwarderListQuery) url.Values {
	values := url.Values{}
	if q.Name != "" {
		values.Set("name", q.Name)
	}
	for _, t := range q.Types {
		values.Add("types", t)
	}
	if q.PerPage > 0 {
		values.Set("per_page", strconv.Itoa(q.PerPage))
	}
	if q.Page > 0 {
		values.Set("page", strconv.Itoa(q.Page))
	}
	return values
}

// ListArvanCloudLogForwarders returns a page of domainName's log forwarders.
func (p *Provider) ListArvanCloudLogForwarders(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudLogForwarderListQuery) ([]domain.ArvanCloudLogForwarder, domain.ArvanCloudReportPageMeta, error) {
	path := logForwardersPath(domainName) + "?" + logForwarderListQueryValues(query).Encode()
	raw, err := p.client.doRawGET(ctx, creds, path, "application/json")
	if err != nil {
		return nil, domain.ArvanCloudReportPageMeta{}, fmt.Errorf("listing arvancloud log forwarders of domain %q: %w", domainName, err)
	}
	var envelope logForwarderListEnvelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, domain.ArvanCloudReportPageMeta{}, fmt.Errorf("decoding arvancloud log forwarder list of domain %q: %w", domainName, err)
		}
	}
	out := make([]domain.ArvanCloudLogForwarder, len(envelope.Data))
	for i, w := range envelope.Data {
		out[i] = toLogForwarderDomain(w)
	}
	return out, toReportPageMetaDomain(envelope.Meta), nil
}

// CreateArvanCloudLogForwarder creates a new log forwarder.
func (p *Provider) CreateArvanCloudLogForwarder(ctx context.Context, creds domain.ProviderCredentials, domainName string, forwarder domain.ArvanCloudLogForwarder) (*domain.ArvanCloudLogForwarder, error) {
	body := logForwarderRequestBody(forwarder)
	var wire logForwarderWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, logForwardersPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud log forwarder on domain %q: %w", domainName, err)
	}
	created := toLogForwarderDomain(wire)
	return &created, nil
}

// GetArvanCloudLogForwarder returns a single log forwarder by id.
func (p *Provider) GetArvanCloudLogForwarder(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudLogForwarder, error) {
	var wire logForwarderWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, logForwarderPath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud log forwarder %q on domain %q: %w", id, domainName, err)
	}
	found := toLogForwarderDomain(wire)
	return &found, nil
}

// UpdateArvanCloudLogForwarder updates a log forwarder and returns it as
// stored afterward.
func (p *Provider) UpdateArvanCloudLogForwarder(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, forwarder domain.ArvanCloudLogForwarder) (*domain.ArvanCloudLogForwarder, error) {
	body := logForwarderRequestBody(forwarder)
	var wire logForwarderWire
	if err := p.client.doJSON(ctx, creds, http.MethodPut, logForwarderPath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud log forwarder %q on domain %q: %w", id, domainName, err)
	}
	updated := toLogForwarderDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudLogForwarder removes a log forwarder by id.
func (p *Provider) DeleteArvanCloudLogForwarder(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, logForwarderPath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud log forwarder %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// SetArvanCloudLogForwarderStatus enables or disables a log forwarder and
// returns it as stored afterward.
func (p *Provider) SetArvanCloudLogForwarderStatus(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, status bool) (*domain.ArvanCloudLogForwarder, error) {
	body := map[string]any{"status": status}
	var wire logForwarderWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, logForwarderStatusPath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("setting arvancloud log forwarder %q status on domain %q: %w", id, domainName, err)
	}
	updated := toLogForwarderDomain(wire)
	return &updated, nil
}

// --- Metric Exporters -------------------------------------------------------

// metricExporterWire mirrors both MetricExporter (request) and
// MetricExporterResponse.data / MetricExporterSummary (response shapes) —
// Domain is only ever populated by the list endpoint (MetricExporterSummary
// carries it; the others do not), matching
// domain.ArvanCloudMetricExporter.Domain's own doc comment.
type metricExporterWire struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	URL      string `json:"url,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Interval string `json:"interval,omitempty"`
	Status   *bool  `json:"status,omitempty"`
}

func toMetricExporterDomain(w metricExporterWire) domain.ArvanCloudMetricExporter {
	me := domain.ArvanCloudMetricExporter{
		ID: w.ID, Name: w.Name, Type: domain.ArvanCloudMetricExporterType(w.Type), URL: w.URL,
		Domain: w.Domain, Interval: domain.ArvanCloudMetricExporterInterval(w.Interval),
	}
	if w.Status != nil {
		me.Status = *w.Status
	}
	return me
}

// metricExporterRequestBody builds the JSON body for a metric exporter
// create/update, as a plain map so status's explicit false reaches the
// provider.
func metricExporterRequestBody(me domain.ArvanCloudMetricExporter) map[string]any {
	return map[string]any{
		"name": me.Name, "type": string(me.Type), "interval": string(me.Interval), "status": me.Status,
	}
}

// metricExporterListEnvelope mirrors the PaginatedResponse shape
// metric-exporters.index returns — the same top-level data+meta shape
// logForwarderListEnvelope documents.
type metricExporterListEnvelope struct {
	Data []metricExporterWire      `json:"data"`
	Meta paginatedResponseMetaWire `json:"meta"`
}

// metricExporterListQueryValues builds metric-exporters.index's query
// parameters. Types uses the same repeated-key convention
// logForwarderListQueryValues documents.
func metricExporterListQueryValues(q domain.ArvanCloudMetricExporterListQuery) url.Values {
	values := url.Values{}
	if q.Name != "" {
		values.Set("name", q.Name)
	}
	if q.Domain != "" {
		values.Set("domain", q.Domain)
	}
	if q.MetricExporterID != "" {
		values.Set("metric_exporter_id", q.MetricExporterID)
	}
	for _, t := range q.Types {
		values.Add("types", t)
	}
	if q.PerPage > 0 {
		values.Set("per_page", strconv.Itoa(q.PerPage))
	}
	if q.Page > 0 {
		values.Set("page", strconv.Itoa(q.Page))
	}
	return values
}

// ListArvanCloudMetricExporters returns a page of metric exporters across
// the whole account (metric-exporters.index) — NOT scoped to a single
// domain; see domain/arvancloud_observability.go's package comment.
func (p *Provider) ListArvanCloudMetricExporters(ctx context.Context, creds domain.ProviderCredentials, query domain.ArvanCloudMetricExporterListQuery) ([]domain.ArvanCloudMetricExporter, domain.ArvanCloudReportPageMeta, error) {
	path := metricExportersPath + "?" + metricExporterListQueryValues(query).Encode()
	raw, err := p.client.doRawGET(ctx, creds, path, "application/json")
	if err != nil {
		return nil, domain.ArvanCloudReportPageMeta{}, fmt.Errorf("listing arvancloud metric exporters: %w", err)
	}
	var envelope metricExporterListEnvelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, domain.ArvanCloudReportPageMeta{}, fmt.Errorf("decoding arvancloud metric exporter list: %w", err)
		}
	}
	out := make([]domain.ArvanCloudMetricExporter, len(envelope.Data))
	for i, w := range envelope.Data {
		out[i] = toMetricExporterDomain(w)
	}
	return out, toReportPageMetaDomain(envelope.Meta), nil
}

// metricExporterMetricItemWire mirrors one metric group's items[] entry.
type metricExporterMetricItemWire struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// metricExporterMetricGroupWire mirrors one MetricExporterMetrics.data[]
// entry.
type metricExporterMetricGroupWire struct {
	Metric string                         `json:"metric"`
	Items  []metricExporterMetricItemWire `json:"items"`
}

// metricExporterMetricsWire mirrors MetricExporterMetrics.
type metricExporterMetricsWire struct {
	Data    []metricExporterMetricGroupWire `json:"data"`
	Message string                          `json:"message"`
}

// ListArvanCloudMetricExporterTypes returns the catalog of metric groups and
// individual metrics available when creating a metric exporter — also
// account-wide, no domain parameter. Uses doRawGET rather than doJSON: the
// MetricExporterMetrics schema's own top-level shape is {"data": [...],
// "message": ...} — its "data" key is the metric group array itself, not an
// envelope wrapper, so doJSON's usual one-level "data" unwrap would try to
// decode that array into this wire struct and fail. The same situation
// healthCheckDetailsEnvelope/highRequestIPsEnvelope document for their own
// top-level, non-nested response shapes.
func (p *Provider) ListArvanCloudMetricExporterTypes(ctx context.Context, creds domain.ProviderCredentials) (*domain.ArvanCloudMetricExporterMetrics, error) {
	raw, err := p.client.doRawGET(ctx, creds, metricExporterMetricsPath, "application/json")
	if err != nil {
		return nil, fmt.Errorf("listing arvancloud metric exporter types: %w", err)
	}
	var wire metricExporterMetricsWire
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, fmt.Errorf("decoding arvancloud metric exporter types: %w", err)
		}
	}
	groups := make([]domain.ArvanCloudMetricExporterMetricGroup, len(wire.Data))
	for i, g := range wire.Data {
		items := make([]domain.ArvanCloudMetricExporterMetricItem, len(g.Items))
		for j, it := range g.Items {
			items[j] = domain.ArvanCloudMetricExporterMetricItem{Name: it.Name, Description: it.Description}
		}
		groups[i] = domain.ArvanCloudMetricExporterMetricGroup{Metric: g.Metric, Items: items}
	}
	return &domain.ArvanCloudMetricExporterMetrics{Groups: groups, Message: wire.Message}, nil
}

// CreateArvanCloudMetricExporter creates a new metric exporter, scoped to
// domainName.
func (p *Provider) CreateArvanCloudMetricExporter(ctx context.Context, creds domain.ProviderCredentials, domainName string, exporter domain.ArvanCloudMetricExporter) (*domain.ArvanCloudMetricExporter, error) {
	body := metricExporterRequestBody(exporter)
	var wire metricExporterWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainMetricExportersPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud metric exporter on domain %q: %w", domainName, err)
	}
	created := toMetricExporterDomain(wire)
	return &created, nil
}

// GetArvanCloudMetricExporter returns a single metric exporter by id.
func (p *Provider) GetArvanCloudMetricExporter(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudMetricExporter, error) {
	var wire metricExporterWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, domainMetricExporterPath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud metric exporter %q on domain %q: %w", id, domainName, err)
	}
	found := toMetricExporterDomain(wire)
	return &found, nil
}

// UpdateArvanCloudMetricExporter updates a metric exporter and returns it as
// stored afterward.
func (p *Provider) UpdateArvanCloudMetricExporter(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, exporter domain.ArvanCloudMetricExporter) (*domain.ArvanCloudMetricExporter, error) {
	body := metricExporterRequestBody(exporter)
	var wire metricExporterWire
	if err := p.client.doJSON(ctx, creds, http.MethodPut, domainMetricExporterPath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud metric exporter %q on domain %q: %w", id, domainName, err)
	}
	updated := toMetricExporterDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudMetricExporter removes a metric exporter by id.
func (p *Provider) DeleteArvanCloudMetricExporter(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, domainMetricExporterPath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud metric exporter %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// SetArvanCloudMetricExporterStatus enables or disables a metric exporter
// and returns it as stored afterward.
func (p *Provider) SetArvanCloudMetricExporterStatus(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, status bool) (*domain.ArvanCloudMetricExporter, error) {
	body := map[string]any{"status": status}
	var wire metricExporterWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, domainMetricExporterStatusPath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("setting arvancloud metric exporter %q status on domain %q: %w", id, domainName, err)
	}
	updated := toMetricExporterDomain(wire)
	return &updated, nil
}
