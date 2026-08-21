package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud Log Forwarders and Metric Exporters tools (issue #76): both
// configure ArvanCloud to PUSH data to an external system (S3-compatible
// storage, Datadog, Kafka, syslog, ...) rather than exposing data through
// this project's own reporting tools (contrast with get_arvancloud_*_report
// and friends, arvancloud_reports_tools.go). All fast operations
// (AGENTS.md 4.3): every tool below returns its result within the call, with
// no operation_id to poll afterward.
//
// settings sensitivity: a log forwarder's "settings" carries the external
// destination's own credentials (S3 access/secret keys, a Datadog API key,
// ...). Every tool below that returns a log forwarder includes settings in
// its response — the same "the caller already holds what they just gave us"
// reasoning arvanCloudDdosSettingsToMap documents: the sensitivity this
// server guards against is a log line or error message leaking settings to
// somewhere OTHER than the caller who supplied it, not the tool response
// itself. See observability.go's package comment and
// observability_test.go's redaction tests.
const arvanCloudLogForwarderVsMetricExporterNote = "Log Forwarders push raw log lines to an external destination " +
	"(S3-compatible storage, Datadog, Kafka, syslog, ArvanCloud's own log storage); Metric Exporters expose " +
	"aggregate metrics for scraping instead. Both are distinct from this server's own get_arvancloud_*_report " +
	"tools, which read report data directly rather than configuring a push destination."

// --- Log Forwarders: data_fields -------------------------------------------

// arvanCloudLogForwarderDataFieldsArgs decodes "data_fields": one flat
// object covering every LogForwarderType's field set (see
// domain.ArvanCloudLogForwarderDataFields' own doc comment for which fields
// apply to which "type"); only fields meaningful for the forwarder's own
// Type should be set to true.
type arvanCloudLogForwarderDataFieldsArgs struct {
	Method         bool `json:"method"`
	Scheme         bool `json:"scheme"`
	Domain         bool `json:"domain"`
	URI            bool `json:"uri"`
	QueryString    bool `json:"query_string"`
	Referer        bool `json:"referer"`
	IP             bool `json:"ip"`
	UA             bool `json:"ua"`
	Country        bool `json:"country"`
	ASN            bool `json:"asn"`
	NodeID         bool `json:"node_id"`
	ContentType    bool `json:"content_type"`
	Status         bool `json:"status"`
	ServerIP       bool `json:"server_ip"`
	ServerPort     bool `json:"server_port"`
	BytesSent      bool `json:"bytes_sent"`
	BytesReceived  bool `json:"bytes_received"`
	UpstreamTime   bool `json:"upstream_time"`
	Cache          bool `json:"cache"`
	RequestID      bool `json:"request_id"`
	Timestamp      bool `json:"timestamp"`
	ISOTimestamp   bool `json:"iso_timestamp"`
	TLSFingerprint bool `json:"tls_fingerprint"`
	UpstreamAddr   bool `json:"upstream_addr"`
	Product        bool `json:"product"`
	RemoteAddress  bool `json:"remote_address"`
	Data           bool `json:"data"`
	UUID           bool `json:"uuid"`
	Record         bool `json:"record"`
	RecordType     bool `json:"record_type"`
	ResponseCode   bool `json:"response_code"`
	ProcessTime    bool `json:"process_time"`
	TotalTime      bool `json:"total_time"`
	General        bool `json:"general"`
	Firewall       bool `json:"firewall"`
	Proxy          bool `json:"proxy"`
	DNSResolver    bool `json:"dns_resolver"`
	DDoS           bool `json:"ddos"`
	RateLimit      bool `json:"ratelimit"`
	WAFEvent       bool `json:"waf_event"`
	ClientIP       bool `json:"client_ip"`
	UpstreamProto  bool `json:"upstream_proto"`
	UpstreamURI    bool `json:"upstream_uri"`
	UpstreamPort   bool `json:"upstream_port"`
	UpstreamIP     bool `json:"upstream_ip"`
	DomainName     bool `json:"domain_name"`
	HTTPVersion    bool `json:"http_version"`
	RequestMethod  bool `json:"request_method"`
	RequestURI     bool `json:"request_uri"`
	RealTimestamp  bool `json:"real_timestamp"`
	ErrorMessage   bool `json:"error_message"`
	PopSite        bool `json:"pop_site"`
}

func (a arvanCloudLogForwarderDataFieldsArgs) toDomain() domain.ArvanCloudLogForwarderDataFields {
	return domain.ArvanCloudLogForwarderDataFields{
		Method: a.Method, Scheme: a.Scheme, Domain: a.Domain, URI: a.URI, QueryString: a.QueryString,
		Referer: a.Referer, IP: a.IP, UserAgent: a.UA, Country: a.Country, ASN: a.ASN, NodeID: a.NodeID,
		ContentType: a.ContentType, Status: a.Status, ServerIP: a.ServerIP, ServerPort: a.ServerPort,
		BytesSent: a.BytesSent, BytesReceived: a.BytesReceived, UpstreamTime: a.UpstreamTime, Cache: a.Cache,
		RequestID: a.RequestID, Timestamp: a.Timestamp, ISOTimestamp: a.ISOTimestamp,
		TLSFingerprint: a.TLSFingerprint, UpstreamAddr: a.UpstreamAddr, Product: a.Product,
		RemoteAddress: a.RemoteAddress, Data: a.Data, UUID: a.UUID, Record: a.Record, RecordType: a.RecordType,
		ResponseCode: a.ResponseCode, ProcessTime: a.ProcessTime, TotalTime: a.TotalTime, General: a.General,
		Firewall: a.Firewall, Proxy: a.Proxy, DNSResolver: a.DNSResolver, DDoS: a.DDoS, RateLimit: a.RateLimit,
		WAFEvent: a.WAFEvent, ClientIP: a.ClientIP, UpstreamProto: a.UpstreamProto, UpstreamURI: a.UpstreamURI,
		UpstreamPort: a.UpstreamPort, UpstreamIP: a.UpstreamIP, DomainName: a.DomainName,
		HTTPVersion: a.HTTPVersion, RequestMethod: a.RequestMethod, RequestURI: a.RequestURI,
		RealTimestamp: a.RealTimestamp, ErrorMessage: a.ErrorMessage, PopSite: a.PopSite,
	}
}

func arvanCloudLogForwarderDataFieldsFromDomain(d domain.ArvanCloudLogForwarderDataFields) map[string]any {
	return map[string]any{
		"method": d.Method, "scheme": d.Scheme, "domain": d.Domain, "uri": d.URI, "query_string": d.QueryString,
		"referer": d.Referer, "ip": d.IP, "ua": d.UserAgent, "country": d.Country, "asn": d.ASN,
		"node_id": d.NodeID, "content_type": d.ContentType, "status": d.Status, "server_ip": d.ServerIP,
		"server_port": d.ServerPort, "bytes_sent": d.BytesSent, "bytes_received": d.BytesReceived,
		"upstream_time": d.UpstreamTime, "cache": d.Cache, "request_id": d.RequestID, "timestamp": d.Timestamp,
		"iso_timestamp": d.ISOTimestamp, "tls_fingerprint": d.TLSFingerprint, "upstream_addr": d.UpstreamAddr,
		"product": d.Product, "remote_address": d.RemoteAddress, "data": d.Data, "uuid": d.UUID,
		"record": d.Record, "record_type": d.RecordType, "response_code": d.ResponseCode,
		"process_time": d.ProcessTime, "total_time": d.TotalTime, "general": d.General, "firewall": d.Firewall,
		"proxy": d.Proxy, "dns_resolver": d.DNSResolver, "ddos": d.DDoS, "ratelimit": d.RateLimit,
		"waf_event": d.WAFEvent, "client_ip": d.ClientIP, "upstream_proto": d.UpstreamProto,
		"upstream_uri": d.UpstreamURI, "upstream_port": d.UpstreamPort, "upstream_ip": d.UpstreamIP,
		"domain_name": d.DomainName, "http_version": d.HTTPVersion, "request_method": d.RequestMethod,
		"request_uri": d.RequestURI, "real_timestamp": d.RealTimestamp, "error_message": d.ErrorMessage,
		"pop_site": d.PopSite,
	}
}

func arvanCloudLogForwarderDataFieldsProperty() map[string]any {
	boolProp := func(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
	return map[string]any{
		"type": "object",
		"description": "Which fields to include in a forwarded log line. Only fields meaningful for \"type\" apply " +
			"(others are ignored) — access: method/scheme/domain/uri/query_string/referer/ip/ua/country/asn/node_id/" +
			"content_type/status/server_ip/server_port/bytes_sent/bytes_received/upstream_time/cache/request_id/" +
			"timestamp/iso_timestamp/tls_fingerprint/upstream_addr; waf: timestamp/iso_timestamp/domain/product/" +
			"remote_address/data/uuid; dns: timestamp/iso_timestamp/record/record_type/ip/country/asn/response_code/" +
			"process_time; event: domain/uuid/timestamp/iso_timestamp/method/scheme/ip/country/status/server_ip/" +
			"server_port/uri/query_string/tls_fingerprint/total_time/general/firewall/proxy/dns_resolver/ddos/" +
			"ratelimit/waf_event; error: client_ip/upstream_proto/upstream_uri/upstream_port/upstream_ip/domain_name/" +
			"http_version/request_method/request_uri/real_timestamp/iso_timestamp/error_message/request_id/pop_site. " +
			"Leave empty to include ArvanCloud's own default field set.",
		"properties": map[string]any{
			"method":          boolProp("access, event."),
			"scheme":          boolProp("access, event."),
			"domain":          boolProp("access, waf, event."),
			"uri":             boolProp("access, event."),
			"query_string":    boolProp("access, event."),
			"referer":         boolProp("access only."),
			"ip":              boolProp("access, dns, event."),
			"ua":              boolProp("access only. The client's User-Agent header."),
			"country":         boolProp("access, dns, event."),
			"asn":             boolProp("access, dns."),
			"node_id":         boolProp("access only."),
			"content_type":    boolProp("access only."),
			"status":          boolProp("access, event. The HTTP response status code."),
			"server_ip":       boolProp("access, event."),
			"server_port":     boolProp("access, event."),
			"bytes_sent":      boolProp("access only."),
			"bytes_received":  boolProp("access only."),
			"upstream_time":   boolProp("access only."),
			"cache":           boolProp("access only."),
			"request_id":      boolProp("access, error."),
			"timestamp":       boolProp("access, waf, dns, event."),
			"iso_timestamp":   boolProp("access, waf, dns, event, error."),
			"tls_fingerprint": boolProp("access, event."),
			"upstream_addr":   boolProp("access only."),
			"product":         boolProp("waf only."),
			"remote_address":  boolProp("waf only."),
			"data":            boolProp("waf only. Details of the matched WAF rule."),
			"uuid":            boolProp("waf, event."),
			"record":          boolProp("dns only."),
			"record_type":     boolProp("dns only. The DNS record type (e.g. \"A\"), NOT this LogForwarder's own \"type\"."),
			"response_code":   boolProp("dns only."),
			"process_time":    boolProp("dns only."),
			"total_time":      boolProp("event only."),
			"general":         boolProp("event only."),
			"firewall":        boolProp("event only."),
			"proxy":           boolProp("event only."),
			"dns_resolver":    boolProp("event only."),
			"ddos":            boolProp("event only."),
			"ratelimit":       boolProp("event only."),
			"waf_event":       boolProp("event only. Whether WAF acted on this event."),
			"client_ip":       boolProp("error only."),
			"upstream_proto":  boolProp("error only."),
			"upstream_uri":    boolProp("error only."),
			"upstream_port":   boolProp("error only."),
			"upstream_ip":     boolProp("error only."),
			"domain_name":     boolProp("error only."),
			"http_version":    boolProp("error only."),
			"request_method":  boolProp("error only."),
			"request_uri":     boolProp("error only."),
			"real_timestamp":  boolProp("error only."),
			"error_message":   boolProp("error only."),
			"pop_site":        boolProp("error only."),
		},
	}
}

// --- Log Forwarders: settings -----------------------------------------------

// arvanCloudLogForwarderSettingsArgs decodes "settings": one flat object
// covering every ConnectionType's fields (see
// domain.ArvanCloudLogForwarderSettings' own doc comment for which fields
// apply to which "connection_type"); only the fields matching the
// forwarder's own ConnectionType should be set.
type arvanCloudLogForwarderSettingsArgs struct {
	SampleRate                    int      `json:"sample_rate"`
	S3Endpoint                    string   `json:"s3_endpoint"`
	AccessKey                     string   `json:"access_key"`
	SecretKey                     string   `json:"secret_key"`
	BucketName                    string   `json:"bucket_name"`
	ObjectSize                    int      `json:"object_size"`
	FlushInterval                 int      `json:"flush_interval"`
	URL                           string   `json:"url"`
	APIKey                        string   `json:"api_key"`
	AppKey                        string   `json:"app_key"`
	BufferSize                    int      `json:"buffer_size"`
	KafkaVersion                  string   `json:"kafka_version"`
	KafkaBrokers                  []string `json:"kafka_brokers"`
	KafkaTopicToWrite             string   `json:"kafka_topic_to_write"`
	KafkaProducerBatchSize        int      `json:"kafka_producer_batch_size"`
	KafkaProducerFlushFrequencyMs int      `json:"kafka_producer_flush_frequency_ms"`
	Token                         string   `json:"token"`
	LogType                       string   `json:"logtype"`
	Host                          string   `json:"host"`
	Port                          int      `json:"port"`
	TLS                           bool     `json:"tls"`
	Cert                          string   `json:"cert"`
	RetryTime                     int      `json:"retry_time"`
}

// toDomain builds the tagged domain.ArvanCloudLogForwarderSettings union,
// selecting which branch to populate by connectionType — the same selection
// arvanCloudHealthCheckArgs.toDomain makes for RequestConfig by "type".
func (a arvanCloudLogForwarderSettingsArgs) toDomain(connectionType domain.ArvanCloudConnectionType) domain.ArvanCloudLogForwarderSettings {
	switch {
	case domain.IsArvanCloudS3ConnectionType(connectionType):
		return domain.ArvanCloudLogForwarderSettings{S3: &domain.ArvanCloudLogForwarderS3Settings{
			SampleRate: a.SampleRate, S3Endpoint: a.S3Endpoint, AccessKey: a.AccessKey, SecretKey: a.SecretKey,
			BucketName: a.BucketName, ObjectSize: a.ObjectSize, FlushInterval: a.FlushInterval,
		}}
	case connectionType == domain.ArvanCloudConnectionTypeDatadog:
		return domain.ArvanCloudLogForwarderSettings{Datadog: &domain.ArvanCloudLogForwarderDatadogSettings{
			SampleRate: a.SampleRate, URL: a.URL, APIKey: a.APIKey, AppKey: a.AppKey,
			FlushInterval: a.FlushInterval, BufferSize: a.BufferSize,
		}}
	case connectionType == domain.ArvanCloudConnectionTypeKafka:
		return domain.ArvanCloudLogForwarderSettings{Kafka: &domain.ArvanCloudLogForwarderKafkaSettings{
			SampleRate: a.SampleRate, KafkaVersion: a.KafkaVersion, KafkaBrokers: a.KafkaBrokers,
			KafkaTopicToWrite: a.KafkaTopicToWrite, KafkaProducerBatchSize: a.KafkaProducerBatchSize,
			KafkaProducerFlushFrequencyMS: a.KafkaProducerFlushFrequencyMs,
		}}
	case connectionType == domain.ArvanCloudConnectionTypeLoggly:
		return domain.ArvanCloudLogForwarderSettings{Loggly: &domain.ArvanCloudLogForwarderLogglySettings{
			SampleRate: a.SampleRate, Token: a.Token, URL: a.URL, FlushInterval: a.FlushInterval, BufferSize: a.BufferSize,
		}}
	case connectionType == domain.ArvanCloudConnectionTypeSyslog:
		return domain.ArvanCloudLogForwarderSettings{Syslog: &domain.ArvanCloudLogForwarderSyslogSettings{
			SampleRate: a.SampleRate, LogType: domain.ArvanCloudLogForwarderSyslogType(a.LogType), Host: a.Host,
			Port: a.Port, TLS: a.TLS, Cert: a.Cert, RetryTime: a.RetryTime,
		}}
	case connectionType == domain.ArvanCloudConnectionTypeArvanLogs:
		return domain.ArvanCloudLogForwarderSettings{ArvanLogs: &domain.ArvanCloudLogForwarderArvanLogsSettings{
			SampleRate: a.SampleRate,
		}}
	default:
		return domain.ArvanCloudLogForwarderSettings{}
	}
}

// arvanCloudLogForwarderSettingsToMap flattens whichever branch of s is
// populated back into the single "settings" object shape the create/update
// tools accept — included in every log-forwarder-returning tool's response;
// see this file's own package comment for why that is safe.
func arvanCloudLogForwarderSettingsToMap(s domain.ArvanCloudLogForwarderSettings) map[string]any {
	switch {
	case s.S3 != nil:
		v := s.S3
		return map[string]any{
			"sample_rate": v.SampleRate, "s3_endpoint": v.S3Endpoint, "access_key": v.AccessKey,
			"secret_key": v.SecretKey, "bucket_name": v.BucketName, "object_size": v.ObjectSize,
			"flush_interval": v.FlushInterval,
		}
	case s.Datadog != nil:
		v := s.Datadog
		return map[string]any{
			"sample_rate": v.SampleRate, "url": v.URL, "api_key": v.APIKey, "app_key": v.AppKey,
			"flush_interval": v.FlushInterval, "buffer_size": v.BufferSize,
		}
	case s.Kafka != nil:
		v := s.Kafka
		return map[string]any{
			"sample_rate": v.SampleRate, "kafka_version": v.KafkaVersion, "kafka_brokers": v.KafkaBrokers,
			"kafka_topic_to_write": v.KafkaTopicToWrite, "kafka_producer_batch_size": v.KafkaProducerBatchSize,
			"kafka_producer_flush_frequency_ms": v.KafkaProducerFlushFrequencyMS,
		}
	case s.Loggly != nil:
		v := s.Loggly
		return map[string]any{
			"sample_rate": v.SampleRate, "token": v.Token, "url": v.URL, "flush_interval": v.FlushInterval,
			"buffer_size": v.BufferSize,
		}
	case s.Syslog != nil:
		v := s.Syslog
		return map[string]any{
			"sample_rate": v.SampleRate, "logtype": string(v.LogType), "host": v.Host, "port": v.Port,
			"tls": v.TLS, "cert": v.Cert, "retry_time": v.RetryTime,
		}
	case s.ArvanLogs != nil:
		return map[string]any{"sample_rate": s.ArvanLogs.SampleRate}
	default:
		return map[string]any{}
	}
}

func arvanCloudLogForwarderSettingsProperty() map[string]any {
	return map[string]any{
		"type": "object",
		"description": "REQUIRED. The destination's connection details — shape depends on \"connection_type\"; set " +
			"only the matching fields. Contains the destination's own credentials (S3 keys, a Datadog API key, ...); " +
			"never store these anywhere but pass them here on every call, same as \"api_key\"/\"secret_key\" above.",
		"properties": map[string]any{
			"sample_rate":    map[string]any{"type": "integer", "description": "All connection types. Percentage (0-100) of matching log lines actually forwarded, e.g. 100 to forward every line."},
			"s3_endpoint":    map[string]any{"type": "string", "description": "arvan_s3/alibaba_s3/amazon_s3/custom_s3. The S3-compatible endpoint hostname, e.g. \"s3.ir-thr-at1.arvanstorage.ir\"."},
			"access_key":     map[string]any{"type": "string", "description": "arvan_s3/alibaba_s3/amazon_s3/custom_s3. The bucket's access key."},
			"secret_key":     map[string]any{"type": "string", "description": "arvan_s3/alibaba_s3/amazon_s3/custom_s3. The bucket's secret key. Sensitive — never logged by this server."},
			"bucket_name":    map[string]any{"type": "string", "description": "arvan_s3/alibaba_s3/amazon_s3/custom_s3. The destination bucket's name."},
			"object_size":    map[string]any{"type": "integer", "description": "arvan_s3/alibaba_s3/amazon_s3/custom_s3. Max size, in BYTES, of one uploaded object before a new one starts."},
			"flush_interval": map[string]any{"type": "integer", "description": "arvan_s3/alibaba_s3/amazon_s3/custom_s3, datadog, loggly. How often buffered log lines are flushed to the destination. Unit not specified by ArvanCloud's own API documentation."},
			"url":            map[string]any{"type": "string", "description": "datadog, loggly. REQUIRED for these two. The destination's endpoint URL."},
			"api_key":        map[string]any{"type": "string", "description": "datadog. REQUIRED. Datadog's own API key. Sensitive — never logged by this server."},
			"app_key":        map[string]any{"type": "string", "description": "datadog. Datadog's own Application key, if used. Sensitive — never logged by this server."},
			"buffer_size":    map[string]any{"type": "integer", "description": "datadog, loggly. Max buffered log lines before an intermediate flush."},
			"kafka_version":  map[string]any{"type": "string", "description": "kafka. The Kafka protocol version to speak, e.g. \"2.8.0\"."},
			"kafka_brokers": map[string]any{
				"type": "array", "items": map[string]any{"type": "string"},
				"description": "kafka. REQUIRED. Broker addresses as \"host:port\", e.g. [\"broker1.example.com:9092\"].",
			},
			"kafka_topic_to_write":              map[string]any{"type": "string", "description": "kafka. REQUIRED. The Kafka topic to write log lines to."},
			"kafka_producer_batch_size":         map[string]any{"type": "integer", "description": "kafka. Producer batch size before a send."},
			"kafka_producer_flush_frequency_ms": map[string]any{"type": "integer", "description": "kafka. Producer flush interval, in MILLISECONDS."},
			"token":                             map[string]any{"type": "string", "description": "loggly. REQUIRED. Loggly's own customer token. Sensitive — never logged by this server."},
			"logtype":                           map[string]any{"type": "string", "enum": []string{"syslogudp", "syslogtcp"}, "description": "syslog. REQUIRED. Transport protocol."},
			"host":                              map[string]any{"type": "string", "description": "syslog. REQUIRED. The syslog server's hostname or IP."},
			"port":                              map[string]any{"type": "integer", "description": "syslog. REQUIRED. The syslog server's port, e.g. 514."},
			"tls":                               map[string]any{"type": "boolean", "description": "syslog. Whether to use TLS."},
			"cert":                              map[string]any{"type": "string", "description": "syslog. A PEM-encoded certificate, when required by the syslog server."},
			"retry_time":                        map[string]any{"type": "integer", "description": "syslog. Retry interval after a failed delivery."},
		},
	}
}

// --- Log Forwarders: mask_rules ---------------------------------------------

type arvanCloudMaskRuleArgs struct {
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
}

func (a arvanCloudMaskRuleArgs) toDomain() domain.ArvanCloudMaskRule {
	return domain.ArvanCloudMaskRule{Pattern: a.Pattern, Replace: a.Replace}
}

func arvanCloudMaskRuleToMap(r domain.ArvanCloudMaskRule) map[string]any {
	return map[string]any{"pattern": r.Pattern, "replace": r.Replace}
}

func arvanCloudMaskRulesProperty() map[string]any {
	return map[string]any{
		"type": "array",
		"description": "Regex-based redaction rules applied to forwarded log lines' content before they leave " +
			"ArvanCloud, e.g. to mask a cookie value. Unrelated to \"settings\"' own sensitivity — this masks data " +
			"INSIDE the forwarded logs, not the destination credentials.",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "REQUIRED. A regular expression matched against a log line, e.g. \"(cookie=.*?_).*?(&)\"."},
				"replace": map[string]any{"type": "string", "description": "REQUIRED. The replacement, e.g. \"${1}*****${2}\" to keep the pattern's capture groups but mask what is between them."},
			},
		},
	}
}

// --- Log Forwarders: entity -------------------------------------------------

// arvanCloudLogForwarderToMap renders a domain.ArvanCloudLogForwarder the
// way every log-forwarder-returning tool reports it back to the caller.
// data_fields and mask_rules come back empty on get/create/update/status —
// ArvanCloud's own API does not echo either back (observability.go's
// package comment) — callers should keep their own record of what they last
// sent for those two fields.
func arvanCloudLogForwarderToMap(lf domain.ArvanCloudLogForwarder) map[string]any {
	maskRules := make([]map[string]any, len(lf.MaskRules))
	for i, r := range lf.MaskRules {
		maskRules[i] = arvanCloudMaskRuleToMap(r)
	}
	return map[string]any{
		"id":               lf.ID,
		"name":             lf.Name,
		"description":      lf.Description,
		"type":             string(lf.Type),
		"connection_type":  string(lf.ConnectionType),
		"data_format_expr": lf.DataFormatExpr,
		"data_fields":      arvanCloudLogForwarderDataFieldsFromDomain(lf.DataFields),
		"mask_rules":       maskRules,
		"settings":         arvanCloudLogForwarderSettingsToMap(lf.Settings),
		"status":           lf.Status,
	}
}

// arvanCloudLogForwarderIDProperty describes the "id" parameter every
// single-forwarder tool below needs.
func arvanCloudLogForwarderIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The log forwarder's provider-assigned ID (a UUID), as returned by create_arvancloud_log_forwarder or list_arvancloud_log_forwarders.",
	}
}

// arvanCloudLogForwarderProperties adds the field set shared by
// create_arvancloud_log_forwarder and update_arvancloud_log_forwarder to
// props.
func arvanCloudLogForwarderProperties(props map[string]any) {
	props["name"] = map[string]any{"type": "string", "description": "REQUIRED. A caller-supplied label for the forwarder."}
	props["description"] = map[string]any{"type": "string", "description": "REQUIRED. A caller-supplied note about what this forwarder does."}
	props["type"] = map[string]any{
		"type":        "string",
		"enum":        []string{"access", "waf", "dns", "error", "event"},
		"description": "REQUIRED. Which log stream to forward. " + arvanCloudLogForwarderVsMetricExporterNote,
	}
	props["connection_type"] = map[string]any{
		"type": "string",
		"enum": []string{"arvan_s3", "alibaba_s3", "amazon_s3", "custom_s3", "loggly", "datadog", "syslog", "kafka", "arvan_logs"},
		"description": "REQUIRED. The destination. Selects which fields of \"settings\" are meaningful — see " +
			"\"settings\"' own description.",
	}
	props["data_format_expr"] = map[string]any{
		"type": "string",
		"description": "An Expr-language filter selecting which log lines to forward, e.g. \"Domain == " +
			"'example.com' && Method == 'GET' && Scheme == 'HTTPS'\". Leave empty to forward every line of \"type\".",
	}
	props["data_fields"] = arvanCloudLogForwarderDataFieldsProperty()
	props["mask_rules"] = arvanCloudMaskRulesProperty()
	props["settings"] = arvanCloudLogForwarderSettingsProperty()
	props["status"] = map[string]any{"type": "boolean", "description": "Whether the forwarder is active. Defaults to true when omitted."}
}

// arvanCloudLogForwarderArgs is embedded/decoded by
// create/update_arvancloud_log_forwarder.
type arvanCloudLogForwarderArgs struct {
	arvanCloudDomainNameArgs
	Name           string                               `json:"name"`
	Description    string                               `json:"description"`
	Type           string                               `json:"type"`
	ConnectionType string                               `json:"connection_type"`
	DataFormatExpr string                               `json:"data_format_expr"`
	DataFields     arvanCloudLogForwarderDataFieldsArgs `json:"data_fields"`
	MaskRules      []arvanCloudMaskRuleArgs             `json:"mask_rules"`
	Settings       arvanCloudLogForwarderSettingsArgs   `json:"settings"`
	Status         *bool                                `json:"status"`
}

func (a arvanCloudLogForwarderArgs) toDomain() domain.ArvanCloudLogForwarder {
	status := true
	if a.Status != nil {
		status = *a.Status
	}
	connectionType := domain.ArvanCloudConnectionType(a.ConnectionType)
	maskRules := make([]domain.ArvanCloudMaskRule, len(a.MaskRules))
	for i, r := range a.MaskRules {
		maskRules[i] = r.toDomain()
	}
	return domain.ArvanCloudLogForwarder{
		Name: a.Name, Description: a.Description, Type: domain.ArvanCloudLogForwarderType(a.Type),
		ConnectionType: connectionType, DataFormatExpr: a.DataFormatExpr,
		DataFields: a.DataFields.toDomain(), MaskRules: maskRules,
		Settings: a.Settings.toDomain(connectionType), Status: status,
	}
}

// --- Log Forwarders: tools --------------------------------------------------

func listArvanCloudLogForwardersTool(uc *app.ListArvanCloudLogForwarders) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["name"] = map[string]any{"type": "string", "description": "Filter forwarders by name."}
	props["types"] = map[string]any{
		"type": "array", "items": map[string]any{"type": "string", "enum": []string{"access", "waf", "dns", "error", "event"}},
		"description": "Filter to forwarders of these log stream types. Leave empty for every type.",
	}
	props["per_page"] = map[string]any{"type": "integer", "description": "How many forwarders per page. Omit for ArvanCloud's own default."}
	props["page"] = map[string]any{"type": "integer", "description": "Which page to return, 1-indexed. Omit for page 1."}

	return Tool{
		Name:        "list_arvancloud_log_forwarders",
		Description: "List a domain's log forwarders. " + arvanCloudLogForwarderVsMetricExporterNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Name    string   `json:"name"`
				Types   []string `json:"types"`
				PerPage int      `json:"per_page"`
				Page    int      `json:"page"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			result, err := uc.Execute(ctx, app.ListArvanCloudLogForwardersInput{
				Credentials: args.domain(), Domain: args.Domain,
				Query: domain.ArvanCloudLogForwarderListQuery{
					Name: args.Name, Types: args.Types, PerPage: args.PerPage, Page: args.Page,
				},
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(result.Forwarders))
			for i, f := range result.Forwarders {
				out[i] = arvanCloudLogForwarderToMap(f)
			}
			return map[string]any{
				"log_forwarders": out,
				"page": map[string]any{
					"current_page": result.Page.CurrentPage, "from": result.Page.From, "last_page": result.Page.LastPage,
					"per_page": result.Page.PerPage, "to": result.Page.To, "total": result.Page.Total,
				},
			}, nil
		},
	}
}

func createArvanCloudLogForwarderTool(uc *app.CreateArvanCloudLogForwarder) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudLogForwarderProperties(props)

	return Tool{
		Name: "create_arvancloud_log_forwarder",
		Description: "Create a new log forwarder: push a log stream to an external destination. " +
			arvanCloudLogForwarderVsMetricExporterNote + " This is a fast operation: the created forwarder, " +
			"including its provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "name", "description", "type", "connection_type", "settings"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudLogForwarderArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudLogForwarderInput{
				Credentials: args.domain(), Domain: args.Domain, Forwarder: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLogForwarderToMap(*created), nil
		},
	}
}

func getArvanCloudLogForwarderTool(uc *app.GetArvanCloudLogForwarder) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudLogForwarderIDProperty()

	return Tool{
		Name:        "get_arvancloud_log_forwarder",
		Description: "Get the current state of one log forwarder by ID. " + arvanCloudLogForwarderVsMetricExporterNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudLogForwarderInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudLogForwarderToMap(*found), nil
		},
	}
}

func updateArvanCloudLogForwarderTool(uc *app.UpdateArvanCloudLogForwarder) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudLogForwarderIDProperty()
	arvanCloudLogForwarderProperties(props)

	return Tool{
		Name: "update_arvancloud_log_forwarder",
		Description: "Update a log forwarder. This replaces the forwarder's fields with the given values — pass " +
			"every field you want to keep, not only the ones changing. " + arvanCloudLogForwarderVsMetricExporterNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "name", "description", "type", "connection_type", "settings"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudLogForwarderArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudLogForwarderInput{
				Credentials: args.domain(), Domain: args.Domain, ID: args.ID, Forwarder: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLogForwarderToMap(*updated), nil
		},
	}
}

func deleteArvanCloudLogForwarderTool(uc *app.DeleteArvanCloudLogForwarder) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudLogForwarderIDProperty()

	return Tool{
		Name: "delete_arvancloud_log_forwarder",
		Description: "Permanently delete a log forwarder by ID. " + arvanCloudLogForwarderVsMetricExporterNote +
			" This is a fast operation and cannot be undone. Deleting a forwarder that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudLogForwarderInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

func setArvanCloudLogForwarderStatusTool(uc *app.SetArvanCloudLogForwarderStatus) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudLogForwarderIDProperty()
	props["status"] = map[string]any{"type": "boolean", "description": "REQUIRED. true to enable the forwarder, false to disable it."}

	return Tool{
		Name:        "set_arvancloud_log_forwarder_status",
		Description: "Enable or disable a log forwarder without changing any other field. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "status"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID     string `json:"id"`
				Status bool   `json:"status"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.SetArvanCloudLogForwarderStatusInput{
				Credentials: args.domain(), Domain: args.Domain, ID: args.ID, Status: args.Status,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLogForwarderToMap(*updated), nil
		},
	}
}

// --- Metric Exporters: entity -----------------------------------------------

func arvanCloudMetricExporterToMap(me domain.ArvanCloudMetricExporter) map[string]any {
	return map[string]any{
		"id":       me.ID,
		"name":     me.Name,
		"type":     string(me.Type),
		"url":      me.URL,
		"domain":   me.Domain,
		"interval": string(me.Interval),
		"status":   me.Status,
	}
}

func arvanCloudMetricExporterIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The metric exporter's provider-assigned ID (a UUID), as returned by create_arvancloud_metric_exporter or list_arvancloud_metric_exporters.",
	}
}

func arvanCloudMetricExporterProperties(props map[string]any) {
	props["name"] = map[string]any{"type": "string", "description": "REQUIRED. A caller-supplied label for the exporter."}
	props["type"] = map[string]any{
		"type": "string",
		"enum": []string{"access", "dns", "error", "event"},
		"description": "REQUIRED. Which metric group to export. Narrower than a log forwarder's \"type\" — no " +
			"\"waf\" value exists here. " + arvanCloudLogForwarderVsMetricExporterNote +
			" Use list_arvancloud_metric_exporter_types to see the individual metrics available under each group.",
	}
	props["interval"] = map[string]any{
		"type":        "string",
		"enum":        []string{"10s", "30s", "60s"},
		"description": "REQUIRED. How often metrics are reported.",
	}
	props["status"] = map[string]any{"type": "boolean", "description": "Whether the exporter is active. Defaults to true when omitted."}
}

type arvanCloudMetricExporterArgs struct {
	arvanCloudDomainNameArgs
	Name     string `json:"name"`
	Type     string `json:"type"`
	Interval string `json:"interval"`
	Status   *bool  `json:"status"`
}

func (a arvanCloudMetricExporterArgs) toDomain() domain.ArvanCloudMetricExporter {
	status := true
	if a.Status != nil {
		status = *a.Status
	}
	return domain.ArvanCloudMetricExporter{
		Name: a.Name, Type: domain.ArvanCloudMetricExporterType(a.Type),
		Interval: domain.ArvanCloudMetricExporterInterval(a.Interval), Status: status,
	}
}

// --- Metric Exporters: tools -------------------------------------------------

func listArvanCloudMetricExportersTool(uc *app.ListArvanCloudMetricExporters) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{"type": "string", "description": "Filter exporters by name."}
	props["domain"] = map[string]any{
		"type":        "string",
		"description": "Filter to one domain's exporters, e.g. \"example.com\". Leave empty to list across every domain visible to the credentials.",
	}
	props["metric_exporter_id"] = map[string]any{"type": "string", "description": "Filter to one exporter by its provider-assigned ID."}
	props["types"] = map[string]any{
		"type": "array", "items": map[string]any{"type": "string", "enum": []string{"access", "dns", "error", "event"}},
		"description": "Filter to exporters of these metric group types. Leave empty for every type.",
	}
	props["per_page"] = map[string]any{"type": "integer", "description": "How many exporters per page. Omit for ArvanCloud's own default."}
	props["page"] = map[string]any{"type": "integer", "description": "Which page to return, 1-indexed. Omit for page 1."}

	return Tool{
		Name: "list_arvancloud_metric_exporters",
		Description: "List metric exporters across the WHOLE ACCOUNT (not one domain) — each entry's \"domain\" " +
			"field says which domain it belongs to. Use the \"domain\" filter to narrow to one domain. " +
			arvanCloudLogForwarderVsMetricExporterNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				Name             string   `json:"name"`
				Domain           string   `json:"domain"`
				MetricExporterID string   `json:"metric_exporter_id"`
				Types            []string `json:"types"`
				PerPage          int      `json:"per_page"`
				Page             int      `json:"page"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			result, err := uc.Execute(ctx, app.ListArvanCloudMetricExportersInput{
				Credentials: args.domain(),
				Query: domain.ArvanCloudMetricExporterListQuery{
					Name: args.Name, Domain: args.Domain, MetricExporterID: args.MetricExporterID,
					Types: args.Types, PerPage: args.PerPage, Page: args.Page,
				},
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(result.Exporters))
			for i, e := range result.Exporters {
				out[i] = arvanCloudMetricExporterToMap(e)
			}
			return map[string]any{
				"metric_exporters": out,
				"page": map[string]any{
					"current_page": result.Page.CurrentPage, "from": result.Page.From, "last_page": result.Page.LastPage,
					"per_page": result.Page.PerPage, "to": result.Page.To, "total": result.Page.Total,
				},
			}, nil
		},
	}
}

func listArvanCloudMetricExporterTypesTool(uc *app.ListArvanCloudMetricExporterTypes) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_arvancloud_metric_exporter_types",
		Description: "List the catalog of metric groups (access/dns/error/event) and the individual metrics " +
			"available under each, to choose from when creating a metric exporter. Account-wide, no domain " +
			"parameter. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args credentialArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			metrics, err := uc.Execute(ctx, app.ListArvanCloudMetricExporterTypesInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			groups := make([]map[string]any, len(metrics.Groups))
			for i, g := range metrics.Groups {
				items := make([]map[string]any, len(g.Items))
				for j, it := range g.Items {
					items[j] = map[string]any{"name": it.Name, "description": it.Description}
				}
				groups[i] = map[string]any{"metric": g.Metric, "items": items}
			}
			return map[string]any{"metric_groups": groups, "message": metrics.Message}, nil
		},
	}
}

func createArvanCloudMetricExporterTool(uc *app.CreateArvanCloudMetricExporter) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudMetricExporterProperties(props)

	return Tool{
		Name: "create_arvancloud_metric_exporter",
		Description: "Create a new metric exporter, scoped to one domain (even though list_arvancloud_metric_exporters " +
			"is account-wide). " + arvanCloudLogForwarderVsMetricExporterNote + " This is a fast operation: the " +
			"created exporter, including its provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "name", "type", "interval"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudMetricExporterArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudMetricExporterInput{
				Credentials: args.domain(), Domain: args.Domain, Exporter: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudMetricExporterToMap(*created), nil
		},
	}
}

func getArvanCloudMetricExporterTool(uc *app.GetArvanCloudMetricExporter) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudMetricExporterIDProperty()

	return Tool{
		Name:        "get_arvancloud_metric_exporter",
		Description: "Get the current state of one metric exporter by ID. " + arvanCloudLogForwarderVsMetricExporterNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudMetricExporterInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudMetricExporterToMap(*found), nil
		},
	}
}

func updateArvanCloudMetricExporterTool(uc *app.UpdateArvanCloudMetricExporter) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudMetricExporterIDProperty()
	arvanCloudMetricExporterProperties(props)

	return Tool{
		Name: "update_arvancloud_metric_exporter",
		Description: "Update a metric exporter. This replaces the exporter's fields with the given values — pass " +
			"every field you want to keep, not only the ones changing. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "name", "type", "interval"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudMetricExporterArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudMetricExporterInput{
				Credentials: args.domain(), Domain: args.Domain, ID: args.ID, Exporter: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudMetricExporterToMap(*updated), nil
		},
	}
}

func deleteArvanCloudMetricExporterTool(uc *app.DeleteArvanCloudMetricExporter) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudMetricExporterIDProperty()

	return Tool{
		Name: "delete_arvancloud_metric_exporter",
		Description: "Permanently delete a metric exporter by ID. This is a fast operation and cannot be undone. " +
			"Deleting an exporter that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudMetricExporterInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

func setArvanCloudMetricExporterStatusTool(uc *app.SetArvanCloudMetricExporterStatus) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudMetricExporterIDProperty()
	props["status"] = map[string]any{"type": "boolean", "description": "REQUIRED. true to enable the exporter, false to disable it."}

	return Tool{
		Name:        "set_arvancloud_metric_exporter_status",
		Description: "Enable or disable a metric exporter without changing any other field. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "status"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID     string `json:"id"`
				Status bool   `json:"status"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.SetArvanCloudMetricExporterStatusInput{
				Credentials: args.domain(), Domain: args.Domain, ID: args.ID, Status: args.Status,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudMetricExporterToMap(*updated), nil
		},
	}
}
