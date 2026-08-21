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

// Active Health Check (issue #70), wired to the real CDN API: a
// domain-scoped monitor that periodically probes an origin (in practice, a
// Load Balancing pool, #69) over TCP or HTTP(S) and reports whether it is
// reachable. Base paths are confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Active Health Check" tag,
// relative to domainPath (defined in domain.go) — i.e.
// https://napi.arvancloud.ir/cdn/4.0/domains/{domain}/health-checks/... .
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real ArvanCloud responses correctly. Nothing above
// the adapter boundary ever sees them — every method here translates to/from
// internal/core/domain types.

const (
	healthChecksPathSuffix = "/health-checks"
	// globalHealthCheckZonesPath is account-independent (no domain in the
	// path), unlike every other endpoint in this file.
	globalHealthCheckZonesPath = "health-checks/zones"
)

func healthChecksPath(domainName string) string {
	return domainPath(domainName) + healthChecksPathSuffix
}
func healthCheckPath(domainName, id string) string {
	return healthChecksPath(domainName) + "/" + id
}
func healthCheckDomainZonesPath(domainName string) string {
	return healthChecksPath(domainName) + "/zones"
}
func healthCheckSummaryPath(domainName string) string {
	return healthChecksPath(domainName) + "/reports/summary"
}
func healthCheckDetailsPath(domainName string) string {
	return healthChecksPath(domainName) + "/reports/details"
}

// --- request_config (the spec's HealthCheckRequestConfig oneOf) -----------

// healthCheckExpectedHeaderWire mirrors one ExpectedHeaders entry.
type healthCheckExpectedHeaderWire struct {
	Key   string   `json:"key"`
	Value []string `json:"value"`
}

// healthCheckExpectedResponseWire mirrors ExpectedResponse.
type healthCheckExpectedResponseWire struct {
	Codes           []int                           `json:"codes,omitempty"`
	Headers         map[string][]string             `json:"headers,omitempty"`
	ExpectedHeaders []healthCheckExpectedHeaderWire `json:"expected_headers,omitempty"`
	Body            string                          `json:"body,omitempty"`
}

func toExpectedResponseDomain(w *healthCheckExpectedResponseWire) domain.ArvanCloudHealthCheckExpectedResponse {
	if w == nil {
		return domain.ArvanCloudHealthCheckExpectedResponse{}
	}
	out := domain.ArvanCloudHealthCheckExpectedResponse{
		Codes:   w.Codes,
		Headers: w.Headers,
		Body:    w.Body,
	}
	if len(w.ExpectedHeaders) > 0 {
		out.ExpectedHeaders = make([]domain.ArvanCloudHealthCheckExpectedHeader, len(w.ExpectedHeaders))
		for i, h := range w.ExpectedHeaders {
			out.ExpectedHeaders[i] = domain.ArvanCloudHealthCheckExpectedHeader{Key: h.Key, Value: h.Value}
		}
	}
	return out
}

func expectedResponseRequestBody(r domain.ArvanCloudHealthCheckExpectedResponse) map[string]any {
	body := map[string]any{}
	if len(r.Codes) > 0 {
		body["codes"] = r.Codes
	}
	if len(r.Headers) > 0 {
		body["headers"] = r.Headers
	}
	if len(r.ExpectedHeaders) > 0 {
		headers := make([]map[string]any, len(r.ExpectedHeaders))
		for i, h := range r.ExpectedHeaders {
			headers[i] = map[string]any{"key": h.Key, "value": h.Value}
		}
		body["expected_headers"] = headers
	}
	if r.Body != "" {
		body["body"] = r.Body
	}
	return body
}

// healthCheckSentHeaderWire mirrors one HttpConfig.sent_headers entry.
type healthCheckSentHeaderWire struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// healthCheckRequestConfigWire mirrors both HttpConfig and TcpConfig in one
// struct — the fields that only apply to one of them are left unset by the
// other — since the response shape (HealthCheckView.request_config) is
// declared as a plain "object" in the spec, not a typed oneOf: this adapter
// decodes whichever fields the provider actually echoes back and lets the
// parent check's Type say which branch they belong to.
type healthCheckRequestConfigWire struct {
	// Shared by both HttpConfig and TcpConfig.
	Port    int `json:"port,omitempty"`
	Timeout int `json:"timeout,omitempty"`
	// HttpConfig only.
	Method           string                           `json:"method,omitempty"`
	Path             string                           `json:"path,omitempty"`
	AllowInsecure    *bool                            `json:"allow_insecure,omitempty"`
	ExpectedResponse *healthCheckExpectedResponseWire `json:"expected_response,omitempty"`
	Headers          map[string]string                `json:"headers,omitempty"`
	SentHeaders      []healthCheckSentHeaderWire      `json:"sent_headers,omitempty"`
	FollowRedirects  bool                             `json:"follow_redirects,omitempty"`
}

// toRequestConfigDomain builds the domain oneOf wrapper from the wire form,
// selecting the HTTP or TCP branch by checkType (the parent
// ArvanCloudHealthCheck.Type) rather than by which wire fields happen to be
// present, since the response's request_config carries no shape marker of
// its own.
func toRequestConfigDomain(checkType string, w *healthCheckRequestConfigWire) domain.ArvanCloudHealthCheckRequestConfig {
	if w == nil {
		return domain.ArvanCloudHealthCheckRequestConfig{}
	}
	if checkType == string(domain.ArvanCloudHealthCheckTCP) {
		return domain.ArvanCloudHealthCheckRequestConfig{
			TCP: &domain.ArvanCloudHealthCheckTCPConfig{Port: w.Port, TimeoutMS: w.Timeout},
		}
	}

	allowInsecure := false
	if w.AllowInsecure != nil {
		allowInsecure = *w.AllowInsecure
	}
	sentHeaders := make([]domain.ArvanCloudHealthCheckSentHeader, len(w.SentHeaders))
	for i, h := range w.SentHeaders {
		sentHeaders[i] = domain.ArvanCloudHealthCheckSentHeader{Key: h.Key, Value: h.Value}
	}
	return domain.ArvanCloudHealthCheckRequestConfig{
		HTTP: &domain.ArvanCloudHealthCheckHTTPConfig{
			Method:           domain.ArvanCloudHealthCheckHTTPMethod(w.Method),
			Port:             w.Port,
			Path:             w.Path,
			AllowInsecure:    allowInsecure,
			ExpectedResponse: toExpectedResponseDomain(w.ExpectedResponse),
			Headers:          w.Headers,
			SentHeaders:      sentHeaders,
			FollowRedirects:  w.FollowRedirects,
			TimeoutMS:        w.Timeout,
		},
	}
}

// requestConfigRequestBody builds request_config's request value from
// whichever branch checkType selects. A nil branch (the caller left
// RequestConfig unset, or set the wrong branch for Type) sends an empty
// object rather than panicking — validateArvanCloudHealthCheckInput (app
// layer) is what rejects that before this is ever called.
func requestConfigRequestBody(checkType domain.ArvanCloudHealthCheckType, cfg domain.ArvanCloudHealthCheckRequestConfig) map[string]any {
	if checkType == domain.ArvanCloudHealthCheckTCP {
		if cfg.TCP == nil {
			return map[string]any{}
		}
		return map[string]any{
			"port":    cfg.TCP.Port,
			"timeout": cfg.TCP.TimeoutMS,
		}
	}

	if cfg.HTTP == nil {
		return map[string]any{}
	}
	h := cfg.HTTP
	body := map[string]any{
		"method":            string(h.Method),
		"port":              h.Port,
		"path":              h.Path,
		"allow_insecure":    h.AllowInsecure,
		"expected_response": expectedResponseRequestBody(h.ExpectedResponse),
		"timeout":           h.TimeoutMS,
	}
	if len(h.Headers) > 0 {
		body["headers"] = h.Headers
	} else {
		body["headers"] = map[string]string{}
	}
	sentHeaders := make([]map[string]any, len(h.SentHeaders))
	for i, sh := range h.SentHeaders {
		sentHeaders[i] = map[string]any{"key": sh.Key, "value": sh.Value}
	}
	body["sent_headers"] = sentHeaders
	return body
}

// --- health checks ----------------------------------------------------

// healthCheckZoneWire mirrors HealthCheckZone (embedded on a check's own
// Zones field), distinct from healthCheckZoneNameWire below.
type healthCheckZoneWire struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	MonitoringLevel string `json:"monitoring_level,omitempty"`
}

// healthCheckWire mirrors BaseHealthCheck plus request_config, covering both
// HealthCheckView (list/get response) and HealthCheck (create/update
// request/response) — the two schemas differ only in how strictly
// request_config is typed, which does not matter for decoding.
type healthCheckWire struct {
	ID                  string                        `json:"id,omitempty"`
	Name                string                        `json:"name,omitempty"`
	Description         string                        `json:"description,omitempty"`
	Origin              string                        `json:"origin,omitempty"`
	OriginType          string                        `json:"origin_type,omitempty"`
	Upstreams           []string                      `json:"upstreams,omitempty"`
	Interval            int                           `json:"interval,omitempty"`
	Threshold           int                           `json:"threshold,omitempty"`
	Type                string                        `json:"type,omitempty"`
	Status              *bool                         `json:"status,omitempty"`
	Retries             int                           `json:"retries,omitempty"`
	Zones               []healthCheckZoneWire         `json:"zones,omitempty"`
	MonitoringUpdatedAt string                        `json:"monitoring_updated_at,omitempty"`
	RequestConfig       *healthCheckRequestConfigWire `json:"request_config,omitempty"`
}

func toHealthCheckDomain(w healthCheckWire) domain.ArvanCloudHealthCheck {
	hc := domain.ArvanCloudHealthCheck{
		ID:                  w.ID,
		Name:                w.Name,
		Description:         w.Description,
		Origin:              w.Origin,
		OriginType:          domain.ArvanCloudHealthCheckOriginType(w.OriginType),
		Upstreams:           w.Upstreams,
		IntervalMS:          w.Interval,
		Threshold:           w.Threshold,
		Type:                domain.ArvanCloudHealthCheckType(w.Type),
		Retries:             w.Retries,
		MonitoringUpdatedAt: w.MonitoringUpdatedAt,
		RequestConfig:       toRequestConfigDomain(w.Type, w.RequestConfig),
	}
	if w.Status != nil {
		hc.Status = *w.Status
	}
	if len(w.Zones) > 0 {
		hc.Zones = make([]domain.ArvanCloudHealthCheckZone, len(w.Zones))
		for i, z := range w.Zones {
			hc.Zones[i] = domain.ArvanCloudHealthCheckZone{ID: z.ID, Name: z.Name, MonitoringLevel: z.MonitoringLevel}
		}
	}
	return hc
}

// healthCheckRequestBody builds the JSON body for a health-check
// create/update as a plain map, the same "explicit false must reach the
// provider" reasoning ddosSettingsRequestBody and rateLimitSettingsRequestBody
// document: status defaults to true per the spec, so an explicit false
// disabling a check must not be dropped by encoding/json's omitempty.
func healthCheckRequestBody(hc domain.ArvanCloudHealthCheck) map[string]any {
	body := map[string]any{
		"name":           hc.Name,
		"origin":         hc.Origin,
		"origin_type":    string(hc.OriginType),
		"interval":       hc.IntervalMS,
		"threshold":      hc.Threshold,
		"type":           string(hc.Type),
		"status":         hc.Status,
		"request_config": requestConfigRequestBody(hc.Type, hc.RequestConfig),
	}
	if hc.Description != "" {
		body["description"] = hc.Description
	}
	if len(hc.Upstreams) > 0 {
		body["upstreams"] = hc.Upstreams
	}
	// Retries' spec minimum is 0, a meaningful value ("no retries") that is
	// indistinguishable from Go's own int zero — the same limitation
	// domain.ArvanCloudLoadBalancerSettings.MaxFails documents, handled the
	// same way: sent only when positive, otherwise left for the provider's
	// own default.
	if hc.Retries > 0 {
		body["retries"] = hc.Retries
	}
	if len(hc.Zones) > 0 {
		zones := make([]map[string]any, len(hc.Zones))
		for i, z := range hc.Zones {
			zone := map[string]any{"name": z.Name}
			if z.ID != "" {
				zone["id"] = z.ID
			}
			if z.MonitoringLevel != "" {
				zone["monitoring_level"] = z.MonitoringLevel
			}
			zones[i] = zone
		}
		body["zones"] = zones
	}
	return body
}

// ListArvanCloudHealthChecks returns every health check configured for
// domainName.
func (p *Provider) ListArvanCloudHealthChecks(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudHealthCheck, error) {
	var items []healthCheckWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, healthChecksPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud health checks of domain %q: %w", domainName, err)
	}
	checks := make([]domain.ArvanCloudHealthCheck, len(items))
	for i := range items {
		checks[i] = toHealthCheckDomain(items[i])
	}
	return checks, nil
}

// CreateArvanCloudHealthCheck creates a new health check.
func (p *Provider) CreateArvanCloudHealthCheck(ctx context.Context, creds domain.ProviderCredentials, domainName string, hc domain.ArvanCloudHealthCheck) (*domain.ArvanCloudHealthCheck, error) {
	body := healthCheckRequestBody(hc)
	var wire healthCheckWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, healthChecksPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud health check on domain %q: %w", domainName, err)
	}
	created := toHealthCheckDomain(wire)
	return &created, nil
}

// GetArvanCloudHealthCheck returns a single health check by id.
func (p *Provider) GetArvanCloudHealthCheck(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudHealthCheck, error) {
	var wire healthCheckWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, healthCheckPath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud health check %q on domain %q: %w", id, domainName, err)
	}
	found := toHealthCheckDomain(wire)
	return &found, nil
}

// UpdateArvanCloudHealthCheck updates a health check and returns it as
// stored afterward.
func (p *Provider) UpdateArvanCloudHealthCheck(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, hc domain.ArvanCloudHealthCheck) (*domain.ArvanCloudHealthCheck, error) {
	body := healthCheckRequestBody(hc)
	var wire healthCheckWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, healthCheckPath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud health check %q on domain %q: %w", id, domainName, err)
	}
	updated := toHealthCheckDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudHealthCheck removes a health check by id.
func (p *Provider) DeleteArvanCloudHealthCheck(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, healthCheckPath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud health check %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// --- zones --------------------------------------------------------------

// healthCheckZoneNameWire mirrors HealthCheckZoneName, returned by both
// zone-listing endpoints — distinct from healthCheckZoneWire above, which
// additionally carries MonitoringLevel and is only ever embedded on a
// check's own Zones field.
type healthCheckZoneNameWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toHealthCheckZoneNameDomain(items []healthCheckZoneNameWire) []domain.ArvanCloudHealthCheckZoneName {
	zones := make([]domain.ArvanCloudHealthCheckZoneName, len(items))
	for i, z := range items {
		zones[i] = domain.ArvanCloudHealthCheckZoneName{ID: z.ID, Name: z.Name}
	}
	return zones
}

// ListArvanCloudDomainHealthCheckZones lists the check-execution zones
// available to domainName.
func (p *Provider) ListArvanCloudDomainHealthCheckZones(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudHealthCheckZoneName, error) {
	var items []healthCheckZoneNameWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, healthCheckDomainZonesPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud health check zones of domain %q: %w", domainName, err)
	}
	return toHealthCheckZoneNameDomain(items), nil
}

// ListArvanCloudHealthCheckZones lists the account-independent, global set
// of check-execution zones. Deprecated in the spec, but still implemented —
// see this port method's own doc comment.
func (p *Provider) ListArvanCloudHealthCheckZones(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudHealthCheckZoneName, error) {
	var items []healthCheckZoneNameWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, globalHealthCheckZonesPath, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud global health check zones: %w", err)
	}
	return toHealthCheckZoneNameDomain(items), nil
}

// --- reports --------------------------------------------------------------

// reportQueryValues builds the shared query-string parameters both report
// endpoints accept from an domain.ArvanCloudHealthCheckReportQuery. Empty
// fields are omitted entirely rather than sent as empty strings, so the
// provider applies its own default instead of rejecting an empty enum value.
func reportQueryValues(q domain.ArvanCloudHealthCheckReportQuery, includeTypeAndPaging bool) url.Values {
	values := url.Values{}
	if q.Name != "" {
		values.Set("name", q.Name)
	}
	if q.Upstream != "" {
		values.Set("upstream", q.Upstream)
	}
	if q.Period != "" {
		values.Set("period", q.Period)
	}
	if q.Since != "" {
		values.Set("since", q.Since)
	}
	if q.Until != "" {
		values.Set("until", q.Until)
	}
	if q.Direction != "" {
		values.Set("direction", q.Direction)
	}
	if includeTypeAndPaging {
		if q.Type != "" {
			values.Set("type", q.Type)
		}
		if q.PerPage > 0 {
			values.Set("per_page", strconv.Itoa(q.PerPage))
		}
		if q.Page > 0 {
			values.Set("page", strconv.Itoa(q.Page))
		}
	}
	return values
}

// healthCheckReportSummaryDetailWire mirrors HealthCheckReportSummaryDetail.
type healthCheckReportSummaryDetailWire struct {
	Date   string `json:"date"`
	Status bool   `json:"status"`
}

// healthCheckReportSummaryWire mirrors HealthCheckReportSummary.
type healthCheckReportSummaryWire struct {
	Zone    string                               `json:"zone"`
	Status  bool                                 `json:"status"`
	Total   int                                  `json:"total"`
	Failed  int                                  `json:"failed"`
	Details []healthCheckReportSummaryDetailWire `json:"details,omitempty"`
}

// GetArvanCloudHealthCheckSummary returns a health check's per-zone summary
// report. Not paginated — the provider returns the full breakdown in one
// call.
func (p *Provider) GetArvanCloudHealthCheckSummary(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudHealthCheckReportQuery) ([]domain.ArvanCloudHealthCheckReportSummary, error) {
	path := healthCheckSummaryPath(domainName) + "?" + reportQueryValues(query, false).Encode()
	var items []healthCheckReportSummaryWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &items); err != nil {
		return nil, fmt.Errorf("getting arvancloud health check summary report on domain %q: %w", domainName, err)
	}
	out := make([]domain.ArvanCloudHealthCheckReportSummary, len(items))
	for i, s := range items {
		details := make([]domain.ArvanCloudHealthCheckReportSummaryDetail, len(s.Details))
		for j, d := range s.Details {
			details[j] = domain.ArvanCloudHealthCheckReportSummaryDetail{Date: d.Date, Status: d.Status}
		}
		out[i] = domain.ArvanCloudHealthCheckReportSummary{
			Zone: s.Zone, Status: s.Status, Total: s.Total, Failed: s.Failed, Details: details,
		}
	}
	return out, nil
}

// healthCheckReportDetailWire mirrors HealthCheckReportDetail.
type healthCheckReportDetailWire struct {
	Date     string `json:"date"`
	Zone     string `json:"zone"`
	Upstream string `json:"upstream"`
	Status   bool   `json:"status"`
	Message  string `json:"message"`
}

// paginatedResponseMetaWire mirrors PaginatedResponseMeta.
type paginatedResponseMetaWire struct {
	CurrentPage int `json:"current_page"`
	From        int `json:"from"`
	LastPage    int `json:"last_page"`
	PerPage     int `json:"per_page"`
	To          int `json:"to"`
	Total       int `json:"total"`
}

// healthCheckDetailsEnvelope mirrors the PaginatedResponse shape
// active-health-check.reports.details returns — data plus links/meta at the
// TOP LEVEL of the response body, not nested one level deeper under a
// "data" key the way every other endpoint in this adapter is. That is
// exactly what Client.doJSON's envelope type assumes (it unwraps one
// "data" key and discards everything else, including "meta"), so this
// method goes through doRawGET instead and decodes the full body itself.
type healthCheckDetailsEnvelope struct {
	Data []healthCheckReportDetailWire `json:"data"`
	Meta paginatedResponseMetaWire     `json:"meta"`
}

// GetArvanCloudHealthCheckDetails returns a page of a health check's
// individual probe results, paginated per query.PerPage/query.Page.
func (p *Provider) GetArvanCloudHealthCheckDetails(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudHealthCheckReportQuery) ([]domain.ArvanCloudHealthCheckReportDetail, domain.ArvanCloudHealthCheckReportPageMeta, error) {
	path := healthCheckDetailsPath(domainName) + "?" + reportQueryValues(query, true).Encode()
	raw, err := p.client.doRawGET(ctx, creds, path, "application/json")
	if err != nil {
		return nil, domain.ArvanCloudHealthCheckReportPageMeta{}, fmt.Errorf("getting arvancloud health check details report on domain %q: %w", domainName, err)
	}
	var envelope healthCheckDetailsEnvelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, domain.ArvanCloudHealthCheckReportPageMeta{}, fmt.Errorf("decoding arvancloud health check details report on domain %q: %w", domainName, err)
		}
	}
	out := make([]domain.ArvanCloudHealthCheckReportDetail, len(envelope.Data))
	for i, d := range envelope.Data {
		out[i] = domain.ArvanCloudHealthCheckReportDetail{
			Date: d.Date, Zone: d.Zone, Upstream: d.Upstream, Status: d.Status, Message: d.Message,
		}
	}
	meta := domain.ArvanCloudHealthCheckReportPageMeta{
		CurrentPage: envelope.Meta.CurrentPage, From: envelope.Meta.From, LastPage: envelope.Meta.LastPage,
		PerPage: envelope.Meta.PerPage, To: envelope.Meta.To, Total: envelope.Meta.Total,
	}
	return out, meta, nil
}
