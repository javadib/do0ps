package parspack

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/javadib/do0ps/internal/core/domain"
)

// CDN report/analytics (read-only) and CDN-zone-level SSL settings, wired to
// the real CDN API (issue #24). Base path is confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml's Report and SSL tags, relative
// to Client.cdnBaseURL, i.e.
// https://my.parspack.com/cdnapi/external/api/v1/zones/{zone_uuid}/... .
//
// This file is deliberately separate from cdn.go (zone/order/DNS, issue #19)
// and does not touch the SSL certificate ORDERING adapter (issue #18, the
// sslv2 surface) — see internal/core/domain/cdn_report_ssl.go's package
// comment for why the two "SSL" surfaces coexist without colliding.

const (
	accessLogPathSuffix    = "/report/access-log"
	securityLogPathSuffix  = "/report/security-log"
	errorLogPathSuffix     = "/report/error-log"
	wafLogPathSuffix       = "/report/waf-log"
	topVisitorsPathSuffix  = "/analytics/top-visitors"
	trafficUsagePathSuffix = "/analytics/monthly-traffic-usage"
	minTLSVersionSuffix    = "/ssl/min-tls-version"
	certificatesSuffix     = "/ssl/certificates"
	hstsSuffix             = "/hsts"
)

// logPageMetaWire mirrors the "meta" object nested in every log endpoint's
// "data" field.
type logPageMetaWire struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
}

func toDomainLogPageMeta(w logPageMetaWire) domain.CDNLogPageMeta {
	return domain.CDNLogPageMeta{
		CurrentPage: w.CurrentPage, LastPage: w.LastPage, PerPage: w.PerPage, Total: w.Total,
	}
}

// logQueryValues turns a domain.CDNLogQuery into the query string the four
// report endpoints accept. includeWCDNState is only set for the access log,
// the one endpoint the spec documents a wcdn_state filter for.
func logQueryValues(q domain.CDNLogQuery, includeWCDNState bool) url.Values {
	values := url.Values{}
	if q.Page != 0 {
		values.Set("page", strconv.Itoa(q.Page))
	}
	if q.Step != 0 {
		values.Set("step", strconv.Itoa(q.Step))
	}
	if q.From != "" {
		values.Set("from", q.From)
	}
	if q.To != "" {
		values.Set("to", q.To)
	}
	if q.URI != "" {
		values.Set("uri", q.URI)
	}
	if q.StatusCode != 0 {
		values.Set("status_code", strconv.Itoa(q.StatusCode))
	}
	if q.UserAgent != "" {
		values.Set("user_agent", q.UserAgent)
	}
	if q.Method != "" {
		values.Set("method", q.Method)
	}
	if q.RayID != "" {
		values.Set("ray_id", q.RayID)
	}
	if q.TargetDomain != "" {
		values.Set("target_domain", q.TargetDomain)
	}
	if includeWCDNState && q.WCDNState != "" {
		values.Set("wcdn_state", q.WCDNState)
	}
	return values
}

// withQuery appends a non-empty query string to path.
func withQuery(path string, values url.Values) string {
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}

// accessLogEntryWire mirrors one record of GET .../report/access-log.
type accessLogEntryWire struct {
	ID                  string `json:"_id"`
	Date                string `json:"date"`
	Timestamp           string `json:"timestamp"`
	Host                string `json:"host"`
	UserHost            string `json:"user_host"`
	TargetDomain        string `json:"target_domain"`
	URI                 string `json:"uri"`
	Method              string `json:"method"`
	Scheme              string `json:"scheme"`
	StatusCode          int    `json:"status_code"`
	Level               int    `json:"level"`
	RayID               string `json:"ray_id"`
	EdgeID              string `json:"edge_id"`
	Source              string `json:"source"`
	WCDNState           string `json:"wcdn_state"`
	LogType             string `json:"log_type"`
	ByteIn              string `json:"byte_in"`
	ByteOut             string `json:"byte_out"`
	RemoteIP            string `json:"remote_ip"`
	ZID                 string `json:"zid"`
	ConnectionDuration  string `json:"connection_duration"`
	DeliveryDuration    string `json:"delivery_duration"`
	HostingWaitDuration string `json:"hosting_wait_duration"`
	TotalDuration       string `json:"total_duration"`
	UserAgent           string `json:"user_agent"`
	Facility            string `json:"facility"`
}

func toDomainAccessLogEntry(w accessLogEntryWire) domain.CDNAccessLogEntry {
	return domain.CDNAccessLogEntry{
		ID: w.ID, Date: w.Date, Timestamp: w.Timestamp, Host: w.Host, UserHost: w.UserHost,
		TargetDomain: w.TargetDomain, URI: w.URI, Method: w.Method, Scheme: w.Scheme,
		StatusCode: w.StatusCode, Level: w.Level, RayID: w.RayID, EdgeID: w.EdgeID,
		Source: w.Source, WCDNState: w.WCDNState, LogType: w.LogType, ByteIn: w.ByteIn,
		ByteOut: w.ByteOut, RemoteIP: w.RemoteIP, ZID: w.ZID,
		ConnectionDuration: w.ConnectionDuration, DeliveryDuration: w.DeliveryDuration,
		HostingWaitDuration: w.HostingWaitDuration, TotalDuration: w.TotalDuration,
		UserAgent: w.UserAgent, Facility: w.Facility,
	}
}

type accessLogResponseWire struct {
	Records []accessLogEntryWire `json:"records"`
	Meta    logPageMetaWire      `json:"meta"`
}

// GetCDNAccessLog returns one page of a zone's access log.
func (c *Client) GetCDNAccessLog(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, query domain.CDNLogQuery) (*domain.CDNAccessLogPage, error) {
	path := withQuery(zonesBasePath+"/"+zoneUUID+accessLogPathSuffix, logQueryValues(query, true))
	var wire accessLogResponseWire
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get access log for zone %s: %w", zoneUUID, err)
	}

	records := make([]domain.CDNAccessLogEntry, len(wire.Records))
	for i, r := range wire.Records {
		records[i] = toDomainAccessLogEntry(r)
	}
	return &domain.CDNAccessLogPage{Records: records, Meta: toDomainLogPageMeta(wire.Meta)}, nil
}

// securityLogEntryWire mirrors one record of GET .../report/security-log.
type securityLogEntryWire struct {
	ID              string `json:"_id"`
	Date            string `json:"date"`
	Timestamp       string `json:"timestamp"`
	Host            string `json:"host"`
	UserHost        string `json:"user_host"`
	TargetDomain    string `json:"target_domain"`
	URI             string `json:"uri"`
	Method          string `json:"method"`
	Scheme          string `json:"scheme"`
	StatusCode      int    `json:"status_code"`
	Level           int    `json:"level"`
	RayID           string `json:"ray_id"`
	EdgeID          string `json:"edge_id"`
	Source          string `json:"source"`
	LogType         string `json:"log_type"`
	SecurityType    string `json:"security_type"`
	SecurityMessage string `json:"security_message"`
	RequestTime     string `json:"request_time"`
	RemoteIP        string `json:"remote_ip"`
	ZID             string `json:"zid"`
	AdditionalLogs  string `json:"additional_logs"`
	UserAgent       string `json:"user_agent"`
	Facility        string `json:"facility"`
}

func toDomainSecurityLogEntry(w securityLogEntryWire) domain.CDNSecurityLogEntry {
	return domain.CDNSecurityLogEntry{
		ID: w.ID, Date: w.Date, Timestamp: w.Timestamp, Host: w.Host, UserHost: w.UserHost,
		TargetDomain: w.TargetDomain, URI: w.URI, Method: w.Method, Scheme: w.Scheme,
		StatusCode: w.StatusCode, Level: w.Level, RayID: w.RayID, EdgeID: w.EdgeID,
		Source: w.Source, LogType: w.LogType, SecurityType: w.SecurityType,
		SecurityMessage: w.SecurityMessage, RequestTime: w.RequestTime, RemoteIP: w.RemoteIP,
		ZID: w.ZID, AdditionalLogs: w.AdditionalLogs, UserAgent: w.UserAgent, Facility: w.Facility,
	}
}

type securityLogResponseWire struct {
	Records []securityLogEntryWire `json:"records"`
	Meta    logPageMetaWire        `json:"meta"`
}

// GetCDNSecurityLog returns one page of a zone's security log.
func (c *Client) GetCDNSecurityLog(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, query domain.CDNLogQuery) (*domain.CDNSecurityLogPage, error) {
	path := withQuery(zonesBasePath+"/"+zoneUUID+securityLogPathSuffix, logQueryValues(query, false))
	var wire securityLogResponseWire
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get security log for zone %s: %w", zoneUUID, err)
	}

	records := make([]domain.CDNSecurityLogEntry, len(wire.Records))
	for i, r := range wire.Records {
		records[i] = toDomainSecurityLogEntry(r)
	}
	return &domain.CDNSecurityLogPage{Records: records, Meta: toDomainLogPageMeta(wire.Meta)}, nil
}

// errorLogEntryWire mirrors one record of GET .../report/error-log.
type errorLogEntryWire struct {
	ID          string `json:"_id"`
	Date        string `json:"date"`
	Timestamp   string `json:"timestamp"`
	Host        string `json:"host"`
	UserHost    string `json:"user_host"`
	URI         string `json:"uri"`
	Method      string `json:"method"`
	Scheme      string `json:"scheme"`
	StatusCode  int    `json:"status_code"`
	Level       int    `json:"level"`
	RayID       string `json:"ray_id"`
	EdgeID      string `json:"edge_id"`
	Source      string `json:"source"`
	LogType     string `json:"log_type"`
	ErrorType   string `json:"error_type"`
	RequestTime string `json:"request_time"`
	RemoteIP    string `json:"remote_ip"`
	ZID         string `json:"zid"`
	UserAgent   string `json:"user_agent"`
	Facility    string `json:"facility"`
}

func toDomainErrorLogEntry(w errorLogEntryWire) domain.CDNErrorLogEntry {
	return domain.CDNErrorLogEntry{
		ID: w.ID, Date: w.Date, Timestamp: w.Timestamp, Host: w.Host, UserHost: w.UserHost,
		URI: w.URI, Method: w.Method, Scheme: w.Scheme, StatusCode: w.StatusCode, Level: w.Level,
		RayID: w.RayID, EdgeID: w.EdgeID, Source: w.Source, LogType: w.LogType,
		ErrorType: w.ErrorType, RequestTime: w.RequestTime, RemoteIP: w.RemoteIP, ZID: w.ZID,
		UserAgent: w.UserAgent, Facility: w.Facility,
	}
}

type errorLogResponseWire struct {
	Records []errorLogEntryWire `json:"records"`
	Meta    logPageMetaWire     `json:"meta"`
}

// GetCDNErrorLog returns one page of a zone's error log.
func (c *Client) GetCDNErrorLog(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, query domain.CDNLogQuery) (*domain.CDNErrorLogPage, error) {
	path := withQuery(zonesBasePath+"/"+zoneUUID+errorLogPathSuffix, logQueryValues(query, false))
	var wire errorLogResponseWire
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get error log for zone %s: %w", zoneUUID, err)
	}

	records := make([]domain.CDNErrorLogEntry, len(wire.Records))
	for i, r := range wire.Records {
		records[i] = toDomainErrorLogEntry(r)
	}
	return &domain.CDNErrorLogPage{Records: records, Meta: toDomainLogPageMeta(wire.Meta)}, nil
}

// wafLogDetailWire mirrors one entry of a WAF log record's additional_logs
// array.
type wafLogDetailWire struct {
	Message string `json:"message"`
	Details struct {
		Match string `json:"match"`
		Data  string `json:"data"`
	} `json:"details"`
}

// wafLogEntryWire mirrors one record of GET .../report/waf-log.
type wafLogEntryWire struct {
	ID              string             `json:"_id"`
	Date            string             `json:"date"`
	Timestamp       string             `json:"timestamp"`
	Host            string             `json:"host"`
	UserHost        string             `json:"user_host"`
	TargetDomain    string             `json:"target_domain"`
	URI             string             `json:"uri"`
	Method          string             `json:"method"`
	Scheme          string             `json:"scheme"`
	StatusCode      int                `json:"status_code"`
	Level           int                `json:"level"`
	RayID           string             `json:"ray_id"`
	EdgeID          string             `json:"edge_id"`
	Source          string             `json:"source"`
	LogType         string             `json:"log_type"`
	SecurityType    string             `json:"security_type"`
	SecurityMessage string             `json:"security_message"`
	RequestTime     string             `json:"request_time"`
	RemoteIP        string             `json:"remote_ip"`
	ZID             string             `json:"zid"`
	AdditionalLogs  []wafLogDetailWire `json:"additional_logs"`
	UserAgent       string             `json:"user_agent"`
	Facility        string             `json:"facility"`
}

func toDomainWAFLogEntry(w wafLogEntryWire) domain.CDNWAFLogEntry {
	details := make([]domain.CDNWAFLogDetail, len(w.AdditionalLogs))
	for i, d := range w.AdditionalLogs {
		details[i] = domain.CDNWAFLogDetail{Message: d.Message, Match: d.Details.Match, Data: d.Details.Data}
	}
	return domain.CDNWAFLogEntry{
		ID: w.ID, Date: w.Date, Timestamp: w.Timestamp, Host: w.Host, UserHost: w.UserHost,
		TargetDomain: w.TargetDomain, URI: w.URI, Method: w.Method, Scheme: w.Scheme,
		StatusCode: w.StatusCode, Level: w.Level, RayID: w.RayID, EdgeID: w.EdgeID,
		Source: w.Source, LogType: w.LogType, SecurityType: w.SecurityType,
		SecurityMessage: w.SecurityMessage, RequestTime: w.RequestTime, RemoteIP: w.RemoteIP,
		ZID: w.ZID, AdditionalLogs: details, UserAgent: w.UserAgent, Facility: w.Facility,
	}
}

type wafLogResponseWire struct {
	Records []wafLogEntryWire `json:"records"`
	Meta    logPageMetaWire   `json:"meta"`
}

// GetCDNWAFLog returns one page of a zone's WAF log.
func (c *Client) GetCDNWAFLog(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, query domain.CDNLogQuery) (*domain.CDNWAFLogPage, error) {
	path := withQuery(zonesBasePath+"/"+zoneUUID+wafLogPathSuffix, logQueryValues(query, false))
	var wire wafLogResponseWire
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get WAF log for zone %s: %w", zoneUUID, err)
	}

	records := make([]domain.CDNWAFLogEntry, len(wire.Records))
	for i, r := range wire.Records {
		records[i] = toDomainWAFLogEntry(r)
	}
	return &domain.CDNWAFLogPage{Records: records, Meta: toDomainLogPageMeta(wire.Meta)}, nil
}

// topVisitorWire mirrors one entry of GET .../analytics/top-visitors, whose
// "data" field is a bare array (not wrapped in an object like the log
// endpoints).
type topVisitorWire struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

// GetCDNTopVisitors returns the top visitor IPs for a zone within a date
// range. start and end are required by the provider ("YYYY-MM-DD").
func (c *Client) GetCDNTopVisitors(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, start, end string) ([]domain.CDNTopVisitor, error) {
	values := url.Values{"start": {start}, "end": {end}}
	path := withQuery(zonesBasePath+"/"+zoneUUID+topVisitorsPathSuffix, values)

	var items []topVisitorWire
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &items); err != nil {
		return nil, fmt.Errorf("get top visitors for zone %s: %w", zoneUUID, err)
	}

	visitors := make([]domain.CDNTopVisitor, len(items))
	for i, it := range items {
		visitors[i] = domain.CDNTopVisitor{IP: it.IP, Count: it.Count}
	}
	return visitors, nil
}

// trafficUsageWire mirrors GET .../analytics/monthly-traffic-usage.
type trafficUsageWire struct {
	Receive      int64  `json:"receive"`
	TrafficLimit string `json:"traffic_limit"`
}

// GetCDNMonthlyTrafficUsage returns a zone's current-month traffic usage and
// plan limit.
func (c *Client) GetCDNMonthlyTrafficUsage(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNTrafficUsage, error) {
	var wire trafficUsageWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+trafficUsagePathSuffix, nil, &wire); err != nil {
		return nil, fmt.Errorf("get monthly traffic usage for zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNTrafficUsage{ReceivedBytes: wire.Receive, TrafficLimit: wire.TrafficLimit}, nil
}

// minTLSVersionWire mirrors GET .../ssl/min-tls-version's "data" object.
type minTLSVersionWire struct {
	MinTLSVersion string `json:"min_tls_version"`
}

// GetCDNMinTLSVersion returns the minimum TLS version a zone currently
// accepts.
func (c *Client) GetCDNMinTLSVersion(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (domain.CDNMinTLSVersion, error) {
	var wire minTLSVersionWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+minTLSVersionSuffix, nil, &wire); err != nil {
		return "", fmt.Errorf("get min TLS version for zone %s: %w", zoneUUID, err)
	}
	return domain.CDNMinTLSVersion(wire.MinTLSVersion), nil
}

// minTLSVersionUpdateRequest is the body of PUT .../ssl/min-tls-version.
type minTLSVersionUpdateRequest struct {
	MinTLSVersion string `json:"min_tls_version"`
}

// UpdateCDNMinTLSVersion sets the minimum TLS version a zone accepts.
func (c *Client) UpdateCDNMinTLSVersion(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, version domain.CDNMinTLSVersion) error {
	reqBody := minTLSVersionUpdateRequest{MinTLSVersion: string(version)}
	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+minTLSVersionSuffix, reqBody, nil); err != nil {
		return fmt.Errorf("update min TLS version for zone %s: %w", zoneUUID, err)
	}
	return nil
}

// certificateCredentialsWire mirrors the "credentials" object nested under
// each issuance method of GET .../ssl/certificates.
type certificateCredentialsWire struct {
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
	CABundle    string `json:"ca_bundle"`
}

// certificateDetailWire mirrors one issuance method (letsencrypt or custom)
// of a certificate entry.
type certificateDetailWire struct {
	ExpirationTime string                     `json:"expiration_time"`
	Active         bool                       `json:"active"`
	Credentials    certificateCredentialsWire `json:"credentials"`
}

// certificateEntryWire mirrors one entry of GET .../ssl/certificates.
type certificateEntryWire struct {
	Domain  string `json:"domain"`
	Status  string `json:"status"`
	SSLType string `json:"ssl_type"`
	SSL     struct {
		LetsEncrypt *certificateDetailWire `json:"letsencrypt"`
		Custom      *certificateDetailWire `json:"custom"`
	} `json:"ssl"`
}

func toCertificateDetail(w *certificateDetailWire) *domain.CDNCertificateDetail {
	if w == nil {
		return nil
	}
	return &domain.CDNCertificateDetail{
		ExpirationTime: w.ExpirationTime, Active: w.Active,
		Certificate: w.Credentials.Certificate, PrivateKey: w.Credentials.PrivateKey,
		CABundle: w.Credentials.CABundle,
	}
}

func toDomainCertificate(w certificateEntryWire) domain.CDNCertificate {
	return domain.CDNCertificate{
		Domain: w.Domain, Status: w.Status, SSLType: w.SSLType,
		LetsEncrypt: toCertificateDetail(w.SSL.LetsEncrypt),
		Custom:      toCertificateDetail(w.SSL.Custom),
	}
}

// ListCDNCertificates lists the certificates currently attached to a zone.
// This is read-only: ordering a new certificate is a separate workflow on
// the SSL surface (ports.ParspackProvider.CreateSSLOrder and friends, issue
// #18), not exposed here.
func (c *Client) ListCDNCertificates(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, perPage, page int, domainFilter string) ([]domain.CDNCertificate, error) {
	values := url.Values{}
	if perPage != 0 {
		values.Set("per_page", strconv.Itoa(perPage))
	}
	if page != 0 {
		values.Set("page", strconv.Itoa(page))
	}
	if domainFilter != "" {
		values.Set("domain_filter", domainFilter)
	}
	path := withQuery(zonesBasePath+"/"+zoneUUID+certificatesSuffix, values)

	var items []certificateEntryWire
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &items); err != nil {
		return nil, fmt.Errorf("list SSL certificates for zone %s: %w", zoneUUID, err)
	}

	certs := make([]domain.CDNCertificate, len(items))
	for i, it := range items {
		certs[i] = toDomainCertificate(it)
	}
	return certs, nil
}

// hstsWire mirrors both GET .../hsts's "data" object and PUT .../hsts's
// request body — the same shape either way.
type hstsWire struct {
	Enabled bool `json:"enabled"`
	MaxAge  int  `json:"max_age"`
}

// GetCDNHSTS returns a zone's current HSTS configuration.
func (c *Client) GetCDNHSTS(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNHSTSSettings, error) {
	var wire hstsWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+hstsSuffix, nil, &wire); err != nil {
		return nil, fmt.Errorf("get HSTS settings for zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNHSTSSettings{Enabled: wire.Enabled, MaxAgeSeconds: wire.MaxAge}, nil
}

// UpdateCDNHSTS sets a zone's HSTS configuration.
func (c *Client) UpdateCDNHSTS(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, settings domain.CDNHSTSSettings) error {
	reqBody := hstsWire{Enabled: settings.Enabled, MaxAge: settings.MaxAgeSeconds}
	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+hstsSuffix, reqBody, nil); err != nil {
		return fmt.Errorf("update HSTS settings for zone %s: %w", zoneUUID, err)
	}
	return nil
}
