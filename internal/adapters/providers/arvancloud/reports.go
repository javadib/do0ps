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

// Reports (per-domain) and Aggregated Reports (account-wide) (issue #75),
// wired to the real CDN API: pure GET traffic/security/DNS analytics. Base
// paths are confirmed against docs/api-specs/arvancloud-cdn-4.0.yml's
// "Reports" and "Aggregated Reports" tags — the per-domain ones relative to
// domainPath (defined in domain.go), the account-wide ones relative to
// Client.baseURL directly, with no /domains/{domain} prefix at all.
//
// The wire types below mirror the spec's response shapes exactly so this
// adapter decodes real ArvanCloud responses correctly. Nothing above the
// adapter boundary ever sees them — every method here translates to/from
// internal/core/domain types (see domain/arvancloud_reports.go's package
// comment for the shared response building blocks most of these wire types
// funnel into).

const reportsPathSuffix = "/reports"

func reportsPath(domainName string) string { return domainPath(domainName) + reportsPathSuffix }

func trafficReportPath(domainName string) string { return reportsPath(domainName) + "/traffics" }
func trafficSavedReportPath(domainName string) string {
	return reportsPath(domainName) + "/traffics/saved"
}
func trafficMapPath(domainName string) string     { return reportsPath(domainName) + "/traffics/map" }
func visitorsReportPath(domainName string) string { return reportsPath(domainName) + "/visitors" }
func highRequestIPsPath(domainName string) string {
	return reportsPath(domainName) + "/high-request-ips"
}
func responseTimeReportPath(domainName string) string {
	return reportsPath(domainName) + "/response-time"
}
func statusCodeReportPath(domainName string) string { return reportsPath(domainName) + "/status" }
func statusCodeSummaryPath(domainName string) string {
	return reportsPath(domainName) + "/status/summary"
}
func errorLogsPath(domainName string) string { return reportsPath(domainName) + "/error-logs" }
func errorLogsChartPath(domainName string) string {
	return reportsPath(domainName) + "/error-logs/chart"
}
func errorLogDetailsPath(domainName string) string {
	return reportsPath(domainName) + "/error-log-details"
}
func dnsRequestsReportPath(domainName string) string {
	return reportsPath(domainName) + "/dns-requests"
}
func dnsGeoReportPath(domainName string) string { return reportsPath(domainName) + "/dns-geo" }
func attackReportPath(domainName string) string { return reportsPath(domainName) + "/attacks" }
func attacksListPath(domainName string) string  { return reportsPath(domainName) + "/attacks/list" }
func attackersPath(domainName string) string    { return reportsPath(domainName) + "/attacks/attackers" }
func attackMapPath(domainName string) string    { return reportsPath(domainName) + "/attacks/map" }
func attackedURIsPath(domainName string) string { return reportsPath(domainName) + "/attacks/uri" }

func transportLayerProxyTrafficPath(domainName, transportLayerProxyID string) string {
	return reportsPath(domainName) + "/transport-layer-proxies/" + transportLayerProxyID + "/traffics"
}

// domainsReportDownloadPath is NOT scoped to a single domain, unlike every
// path above — see ports.ArvanCloudProvider.DownloadArvanCloudDomainsReport's
// own doc comment.
const domainsReportDownloadPath = "domains/reports/download"

const (
	aggregatedReportDetailsPath = "reports/aggregated/details"
	aggregatedReportChartsPath  = "reports/aggregated/charts"
	aggregatedReportFiltersPath = "reports/aggregated/filters"
)

// --- Shared query builders ---------------------------------------------

// reportQueryOptions selects which of domain.ArvanCloudReportQuery's fields
// a given per-domain report endpoint actually accepts, per that type's own
// field comments — period itself is always included when set.
type reportQueryOptions struct {
	sinceUntil bool
	subdomain  bool
	paging     bool
	errorParam bool
}

func arvanCloudReportQueryValues(q domain.ArvanCloudReportQuery, opts reportQueryOptions) url.Values {
	values := url.Values{}
	if q.Period != "" {
		values.Set("period", q.Period)
	}
	if opts.sinceUntil {
		if q.Since != "" {
			values.Set("since", q.Since)
		}
		if q.Until != "" {
			values.Set("until", q.Until)
		}
	}
	if opts.subdomain && q.Subdomain != "" {
		values.Set("filter[subdomain]", q.Subdomain)
	}
	if opts.paging {
		if q.PerPage > 0 {
			values.Set("per_page", strconv.Itoa(q.PerPage))
		}
		if q.Page > 0 {
			values.Set("page", strconv.Itoa(q.Page))
		}
	}
	if opts.errorParam && q.Error != "" {
		values.Set("error", q.Error)
	}
	return values
}

// aggregatedQueryOptions selects which of domain.ArvanCloudAggregatedReportQuery's
// fields a given Aggregated Reports endpoint actually accepts — domains is
// always included when set.
type aggregatedQueryOptions struct {
	reportType bool
	filters    bool // category_type/pops/asns/period
	paging     bool
}

func aggregatedQueryValues(q domain.ArvanCloudAggregatedReportQuery, opts aggregatedQueryOptions) url.Values {
	values := url.Values{}
	if q.Domains != "" {
		values.Set("domains", q.Domains)
	}
	if opts.reportType && q.ReportType != "" {
		values.Set("report_type", q.ReportType)
	}
	if opts.filters {
		if q.CategoryType != "" {
			values.Set("category_type", q.CategoryType)
		}
		if q.Pops != "" {
			values.Set("pops", q.Pops)
		}
		if q.Asns != "" {
			values.Set("asns", q.Asns)
		}
		if q.Period != "" {
			values.Set("period", q.Period)
		}
	}
	if opts.paging {
		if q.PerPage > 0 {
			values.Set("per_page", strconv.Itoa(q.PerPage))
		}
		if q.Page > 0 {
			values.Set("page", strconv.Itoa(q.Page))
		}
	}
	return values
}

// --- Shared wire building blocks -----------------------------------------

// reportChartSeriesWire mirrors one chart series entry: {"name": ...,
// "data": [...]}.
type reportChartSeriesWire struct {
	Name string    `json:"name"`
	Data []float64 `json:"data"`
}

// reportChartWire mirrors the recurring {title/name, categories[], series[]}
// time-series chart shape — see domain.ArvanCloudReportChart's own doc
// comment. Title carries the wire field name on most endpoints; Name is
// status.index's own field name for the same concept (its chart is declared
// with "name" instead of "title") — toReportChartDomain falls back to Name
// when Title is empty.
type reportChartWire struct {
	Title      string                  `json:"title"`
	Name       string                  `json:"name"`
	Categories []string                `json:"categories"`
	Series     []reportChartSeriesWire `json:"series"`
}

func toReportChartDomain(w reportChartWire) domain.ArvanCloudReportChart {
	title := w.Title
	if title == "" {
		title = w.Name
	}
	series := make([]domain.ArvanCloudReportChartSeries, len(w.Series))
	for i, s := range w.Series {
		series[i] = domain.ArvanCloudReportChartSeries{Name: s.Name, Data: s.Data}
	}
	return domain.ArvanCloudReportChart{Title: title, Categories: w.Categories, Series: series}
}

// pieSliceWire mirrors the recurring {"name": ..., "y": ...} pie-chart shape.
type pieSliceWire struct {
	Name string `json:"name"`
	Y    int64  `json:"y"`
}

func toPieSlicesDomain(items []pieSliceWire) []domain.ArvanCloudPieSlice {
	out := make([]domain.ArvanCloudPieSlice, len(items))
	for i, s := range items {
		out[i] = domain.ArvanCloudPieSlice{Name: s.Name, Value: s.Y}
	}
	return out
}

// geoMapEntryWire mirrors one entry of a country-code-keyed geo-map object
// (e.g. {"IRN": {"fillKey": 4, "name": "...", "value": 12456789}}).
type geoMapEntryWire struct {
	FillKey int64  `json:"fillKey"`
	Name    string `json:"name"`
	Value   int64  `json:"value"`
}

func toGeoMapDomain(m map[string]geoMapEntryWire) map[string]domain.ArvanCloudGeoMapEntry {
	out := make(map[string]domain.ArvanCloudGeoMapEntry, len(m))
	for code, e := range m {
		out[code] = domain.ArvanCloudGeoMapEntry{CountryCode: code, FillKey: e.FillKey, Name: e.Name, Value: e.Value}
	}
	return out
}

func toReportPageMetaDomain(w paginatedResponseMetaWire) domain.ArvanCloudReportPageMeta {
	return domain.ArvanCloudReportPageMeta{
		CurrentPage: w.CurrentPage, From: w.From, LastPage: w.LastPage, PerPage: w.PerPage, To: w.To, Total: w.Total,
	}
}

// --- Traffic reports ---------------------------------------------------

type trafficStatWire struct {
	Saved  int64  `json:"saved"`
	Bypass int64  `json:"bypass"`
	Top    string `json:"top"`
	Total  int64  `json:"total"`
}

func toTrafficStatDomain(w trafficStatWire) domain.ArvanCloudTrafficStat {
	return domain.ArvanCloudTrafficStat{Saved: w.Saved, Bypass: w.Bypass, Top: w.Top, Total: w.Total}
}

// trafficsWire mirrors the Traffics schema (reports.traffics.total's "data").
type trafficsWire struct {
	Statistics struct {
		Traffics trafficStatWire `json:"traffics"`
		Requests trafficStatWire `json:"requests"`
	} `json:"statistics"`
	Charts struct {
		Requests reportChartWire `json:"requests"`
		Traffics reportChartWire `json:"traffics"`
	} `json:"charts"`
}

// GetArvanCloudTrafficReport returns the traffic report for a domain
// (reports.traffics.total).
func (p *Provider) GetArvanCloudTrafficReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudTrafficReport, error) {
	path := trafficReportPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true, subdomain: true}).Encode()
	var wire trafficsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud traffic report for domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudTrafficReport{
		TrafficStatistics: toTrafficStatDomain(wire.Statistics.Traffics),
		RequestStatistics: toTrafficStatDomain(wire.Statistics.Requests),
		TrafficChart:      toReportChartDomain(wire.Charts.Traffics),
		RequestChart:      toReportChartDomain(wire.Charts.Requests),
	}, nil
}

type savedStatWire struct {
	Saved int64 `json:"saved"`
	Total int64 `json:"total"`
}

func toSavedStatDomain(w savedStatWire) domain.ArvanCloudSavedStat {
	return domain.ArvanCloudSavedStat{Saved: w.Saved, Total: w.Total}
}

// savedTrafficsWire mirrors SavedTrafficsData's "data" (reports.traffics.saved).
type savedTrafficsWire struct {
	Statistics struct {
		Traffic savedStatWire `json:"traffic"`
		Request savedStatWire `json:"request"`
	} `json:"statistics"`
	Charts struct {
		Request []pieSliceWire `json:"request"`
		Traffic []pieSliceWire `json:"traffic"`
	} `json:"charts"`
}

// GetArvanCloudTrafficSavedReport returns the bandwidth-saved pie chart for a
// domain (reports.traffics.saved).
func (p *Provider) GetArvanCloudTrafficSavedReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudTrafficSavedReport, error) {
	path := trafficSavedReportPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true, subdomain: true}).Encode()
	var wire savedTrafficsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud traffic saved report for domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudTrafficSavedReport{
		TrafficStatistics: toSavedStatDomain(wire.Statistics.Traffic),
		RequestStatistics: toSavedStatDomain(wire.Statistics.Request),
		TrafficChart:      toPieSlicesDomain(wire.Charts.Traffic),
		RequestChart:      toPieSlicesDomain(wire.Charts.Request),
	}, nil
}

// countryListWire mirrors one entry of TrafficsMap.lists (the CountryList
// schema).
type countryListWire struct {
	Country  string `json:"country"`
	Code     string `json:"code"`
	Requests int64  `json:"requests"`
	Traffics int64  `json:"traffics"`
}

// trafficsMapWire mirrors TrafficsMap's "data" (reports.traffics.map).
// Statistics is deprecated in the spec and not decoded here.
type trafficsMapWire struct {
	Charts struct {
		Requests map[string]geoMapEntryWire `json:"requests"`
		Traffics map[string]geoMapEntryWire `json:"traffics"`
	} `json:"charts"`
	Lists []countryListWire `json:"lists"`
}

// GetArvanCloudTrafficMap returns traffic as a geo-map for a domain
// (reports.traffics.map).
func (p *Provider) GetArvanCloudTrafficMap(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudTrafficMapReport, error) {
	path := trafficMapPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true, subdomain: true}).Encode()
	var wire trafficsMapWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud traffic map for domain %q: %w", domainName, err)
	}
	lists := make([]domain.ArvanCloudTrafficMapCountry, len(wire.Lists))
	for i, l := range wire.Lists {
		lists[i] = domain.ArvanCloudTrafficMapCountry{Country: l.Country, Code: l.Code, Requests: l.Requests, Traffics: l.Traffics}
	}
	return &domain.ArvanCloudTrafficMapReport{
		RequestsChart: toGeoMapDomain(wire.Charts.Requests),
		TrafficsChart: toGeoMapDomain(wire.Charts.Traffics),
		Lists:         lists,
	}, nil
}

// --- Visitors ------------------------------------------------------------

// visitorsWire mirrors Visitors' "data" (reports.visitors.index).
type visitorsWire struct {
	Statistics struct {
		Visitors struct {
			TopVisitors   string `json:"top_visitors"`
			TotalVisitors int64  `json:"total_visitors"`
		} `json:"visitors"`
	} `json:"statistics"`
	Charts struct {
		Visitors reportChartWire `json:"visitors"`
	} `json:"charts"`
}

// GetArvanCloudVisitorsReport returns the visitor report for a domain
// (reports.visitors.index).
func (p *Provider) GetArvanCloudVisitorsReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudVisitorsReport, error) {
	path := visitorsReportPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true, subdomain: true}).Encode()
	var wire visitorsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud visitors report for domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudVisitorsReport{
		TopVisitors:   wire.Statistics.Visitors.TopVisitors,
		TotalVisitors: wire.Statistics.Visitors.TotalVisitors,
		Chart:         toReportChartDomain(wire.Charts.Visitors),
	}, nil
}

// highRequestedIPWire mirrors the HighRequestedIp schema.
type highRequestedIPWire struct {
	IP           string `json:"ip"`
	RequestCount int64  `json:"request_count"`
}

// highRequestIPsEnvelope mirrors the PaginatedResponse shape
// reports.visitors.high-request-ips returns — data plus meta at the TOP
// LEVEL of the response body, not nested one level deeper under a "data"
// key the way Client.doJSON's envelope assumes — the same situation
// GetArvanCloudHealthCheckDetails handles via doRawGET
// (healthCheckDetailsEnvelope).
type highRequestIPsEnvelope struct {
	Data []highRequestedIPWire     `json:"data"`
	Meta paginatedResponseMetaWire `json:"meta"`
}

// ListArvanCloudHighRequestIPs returns a page of the domain's top requesting
// IPs (reports.visitors.high-request-ips).
func (p *Provider) ListArvanCloudHighRequestIPs(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) ([]domain.ArvanCloudHighRequestIP, domain.ArvanCloudReportPageMeta, error) {
	path := highRequestIPsPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true, paging: true}).Encode()
	raw, err := p.client.doRawGET(ctx, creds, path, "application/json")
	if err != nil {
		return nil, domain.ArvanCloudReportPageMeta{}, fmt.Errorf("listing arvancloud high-request ips for domain %q: %w", domainName, err)
	}
	var envelope highRequestIPsEnvelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, domain.ArvanCloudReportPageMeta{}, fmt.Errorf("decoding arvancloud high-request ips for domain %q: %w", domainName, err)
		}
	}
	out := make([]domain.ArvanCloudHighRequestIP, len(envelope.Data))
	for i, d := range envelope.Data {
		out[i] = domain.ArvanCloudHighRequestIP{IP: d.IP, RequestCount: d.RequestCount}
	}
	return out, toReportPageMetaDomain(envelope.Meta), nil
}

// --- Response time -----------------------------------------------------

// responseTimeWire mirrors ResponseTime's "data" (reports.response-time.index).
// The spec declares only "charts.ir" — no statistics block.
type responseTimeWire struct {
	Charts struct {
		IR reportChartWire `json:"ir"`
	} `json:"charts"`
}

// GetArvanCloudResponseTimeReport returns the response-time report for a
// domain (reports.response-time.index).
func (p *Provider) GetArvanCloudResponseTimeReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudResponseTimeReport, error) {
	path := responseTimeReportPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true, subdomain: true}).Encode()
	var wire responseTimeWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud response time report for domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudResponseTimeReport{Chart: toReportChartDomain(wire.Charts.IR)}, nil
}

// --- Status code reports -------------------------------------------------

// statusCodeReportWire mirrors StatusCodeReport's "data" (reports.status.index).
type statusCodeReportWire struct {
	Statistics struct {
		StatusCodes struct {
			TwoXX   int64 `json:"2xx_sum"`
			ThreeXX int64 `json:"3xx_sum"`
			FourXX  int64 `json:"4xx_sum"`
			FiveXX  int64 `json:"5xx_sum"`
		} `json:"status_codes"`
	} `json:"statistics"`
	Charts struct {
		StatusCode reportChartWire `json:"status_code"`
	} `json:"charts"`
}

// GetArvanCloudStatusCodeReport returns the status-code pie chart for a
// domain, reported as a time series (reports.status.index).
func (p *Provider) GetArvanCloudStatusCodeReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudStatusCodeReport, error) {
	path := statusCodeReportPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true}).Encode()
	var wire statusCodeReportWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud status code report for domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudStatusCodeReport{
		Statistics: domain.ArvanCloudStatusCodeStat{
			TwoXX: wire.Statistics.StatusCodes.TwoXX, ThreeXX: wire.Statistics.StatusCodes.ThreeXX,
			FourXX: wire.Statistics.StatusCodes.FourXX, FiveXX: wire.Statistics.StatusCodes.FiveXX,
		},
		Chart: toReportChartDomain(wire.Charts.StatusCode),
	}, nil
}

// statusCodeSummaryWire mirrors StatusCodeSummary's "data" (reports.status.summary).
// Statistics is deprecated in the spec and not decoded here.
type statusCodeSummaryWire struct {
	Charts struct {
		StatusCode []pieSliceWire `json:"status_code"`
	} `json:"charts"`
}

// GetArvanCloudStatusCodeSummary returns an overview of the status-code pie
// chart for a domain (reports.status.summary).
func (p *Provider) GetArvanCloudStatusCodeSummary(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudStatusCodeSummary, error) {
	path := statusCodeSummaryPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true}).Encode()
	var wire statusCodeSummaryWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud status code summary for domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudStatusCodeSummary{Chart: toPieSlicesDomain(wire.Charts.StatusCode)}, nil
}

// --- Error logs ----------------------------------------------------------

// errorLogWire mirrors the ErrorLog schema.
type errorLogWire struct {
	Name      string `json:"name"`
	Count     int64  `json:"count"`
	Upstreams []struct {
		Address string `json:"address"`
		Count   int64  `json:"count"`
	} `json:"upstreams"`
}

// ListArvanCloudErrorLogs returns the list of errors for a domain
// (reports.error-logs).
func (p *Provider) ListArvanCloudErrorLogs(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) ([]domain.ArvanCloudErrorLog, error) {
	path := errorLogsPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true}).Encode()
	var items []errorLogWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud error logs for domain %q: %w", domainName, err)
	}
	out := make([]domain.ArvanCloudErrorLog, len(items))
	for i, l := range items {
		upstreams := make([]domain.ArvanCloudErrorLogUpstream, len(l.Upstreams))
		for j, u := range l.Upstreams {
			upstreams[j] = domain.ArvanCloudErrorLogUpstream{Address: u.Address, Count: u.Count}
		}
		out[i] = domain.ArvanCloudErrorLog{Name: l.Name, Count: l.Count, Upstreams: upstreams}
	}
	return out, nil
}

// errorLogChartWire mirrors ErrorLogChart's "data" (reports.error-logs.chart).
// Statistics.StatusCodes is map[string]int64 per the spec's own description
// of its otherwise-untyped "additionalProperties: true" field: "<key, value>
// where key is error and value is its count" — the most conservative
// reading of an undocumented shape, matching how AC14 handled an
// undocumented status enum (documented here rather than guessed silently).
type errorLogChartWire struct {
	Statistics struct {
		StatusCodes map[string]int64 `json:"status_codes"`
	} `json:"statistics"`
	Charts struct {
		StatusCode reportChartWire `json:"status_code"`
	} `json:"charts"`
}

// GetArvanCloudErrorLogsChart returns a chart view of errors for a domain
// (reports.error-logs.chart).
func (p *Provider) GetArvanCloudErrorLogsChart(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudErrorLogsChart, error) {
	path := errorLogsChartPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true}).Encode()
	var wire errorLogChartWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud error logs chart for domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudErrorLogsChart{
		Statistics: wire.Statistics.StatusCodes,
		Chart:      toReportChartDomain(wire.Charts.StatusCode),
	}, nil
}

// GetArvanCloudErrorLogDetail returns the detail of one error for a domain
// (reports.error-log-details, deprecated in the spec but still implemented).
// The response's "data" has no confirmed shape (a bare `type: object` with
// no declared properties) — see domain.ArvanCloudErrorLogDetail's own doc
// comment for why this keeps it as raw JSON rather than guessing a shape.
func (p *Provider) GetArvanCloudErrorLogDetail(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudErrorLogDetail, error) {
	path := errorLogDetailsPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true, errorParam: true}).Encode()
	var raw json.RawMessage
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &raw); err != nil {
		return nil, fmt.Errorf("getting arvancloud error log detail for domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudErrorLogDetail{Raw: raw}, nil
}

// --- DNS reports ---------------------------------------------------------

// dnsRequestReportWire mirrors DnsRequestReport's "data" (reports.dns.requests).
type dnsRequestReportWire struct {
	Statistics struct {
		Total int64  `json:"total"`
		Top   string `json:"top"`
	} `json:"statistics"`
	Charts struct {
		Requests reportChartWire `json:"requests"`
	} `json:"charts"`
}

// GetArvanCloudDnsRequestsReport returns the DNS request report for a domain
// (reports.dns.requests).
func (p *Provider) GetArvanCloudDnsRequestsReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudDnsRequestsReport, error) {
	path := dnsRequestsReportPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true}).Encode()
	var wire dnsRequestReportWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud dns requests report for domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudDnsRequestsReport{
		Total: wire.Statistics.Total, Top: wire.Statistics.Top, Chart: toReportChartDomain(wire.Charts.Requests),
	}, nil
}

// dnsGeoReportWire mirrors DnsGeoReport's "data" (reports.dns.geo).
// Statistics is deprecated in the spec and not decoded here.
type dnsGeoReportWire struct {
	Charts struct {
		Requests map[string]geoMapEntryWire `json:"requests"`
	} `json:"charts"`
	Lists []struct {
		Country  string `json:"country"`
		Name     string `json:"name"`
		Code     string `json:"code"`
		Requests int64  `json:"requests"`
	} `json:"lists"`
}

// GetArvanCloudDnsGeoReport returns DNS requests as a geo-map for a domain
// (reports.dns.geo).
func (p *Provider) GetArvanCloudDnsGeoReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudDnsGeoReport, error) {
	path := dnsGeoReportPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true}).Encode()
	var wire dnsGeoReportWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud dns geo report for domain %q: %w", domainName, err)
	}
	lists := make([]domain.ArvanCloudDnsGeoCountry, len(wire.Lists))
	for i, l := range wire.Lists {
		lists[i] = domain.ArvanCloudDnsGeoCountry{Country: l.Country, Name: l.Name, Code: l.Code, Requests: l.Requests}
	}
	return &domain.ArvanCloudDnsGeoReport{RequestsChart: toGeoMapDomain(wire.Charts.Requests), Lists: lists}, nil
}

// --- Attack reports --------------------------------------------------------

// attackReportWire mirrors AttackReport's "data" (reports.attacks.show).
// Note the capitalized "Attacks" wire key under statistics — that is exactly
// how the spec declares it (a spec inconsistency versus its own snake_case
// convention elsewhere, kept as-is here since this adapter decodes what the
// provider actually sends).
type attackReportWire struct {
	Statistics struct {
		Attacks struct {
			TotalAttacks int64  `json:"total_attacks"`
			TopAttacks   string `json:"top_attacks"`
		} `json:"Attacks"`
	} `json:"statistics"`
	Charts struct {
		Attacks reportChartWire `json:"attacks"`
	} `json:"charts"`
}

// GetArvanCloudAttackReport returns the attack report for a domain
// (reports.attacks.show). Only Period is accepted by this endpoint per the
// spec — no since/until/subdomain.
func (p *Provider) GetArvanCloudAttackReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudAttackReport, error) {
	path := attackReportPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{}).Encode()
	var wire attackReportWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud attack report for domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudAttackReport{
		TotalAttacks: wire.Statistics.Attacks.TotalAttacks, TopAttacks: wire.Statistics.Attacks.TopAttacks,
		Chart: toReportChartDomain(wire.Charts.Attacks),
	}, nil
}

// attackReportItemWire mirrors the AttackReportItem schema.
type attackReportItemWire struct {
	AttackerIP      string   `json:"attacker_ip"`
	AttackerCountry string   `json:"attacker_country"`
	Method          string   `json:"method"`
	URI             string   `json:"uri"`
	Host            []string `json:"host"`
	Timestamp       string   `json:"timestamp"`
	URIArgs         string   `json:"uri_args"`
	Cookie          string   `json:"cookie"`
	Alerts          []string `json:"alerts"`
	UserAgent       []string `json:"user_agent"`
}

// attacksListEnvelope mirrors the PaginatedResponse shape
// reports.attacks.index returns — same top-level data+meta shape as
// highRequestIPsEnvelope above.
type attacksListEnvelope struct {
	Data []attackReportItemWire    `json:"data"`
	Meta paginatedResponseMetaWire `json:"meta"`
}

// ListArvanCloudAttacks returns a page of attack details for a domain
// (reports.attacks.index). Period and paging only — no since/until/subdomain.
func (p *Provider) ListArvanCloudAttacks(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) ([]domain.ArvanCloudAttackReportItem, domain.ArvanCloudReportPageMeta, error) {
	path := attacksListPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{paging: true}).Encode()
	raw, err := p.client.doRawGET(ctx, creds, path, "application/json")
	if err != nil {
		return nil, domain.ArvanCloudReportPageMeta{}, fmt.Errorf("listing arvancloud attacks for domain %q: %w", domainName, err)
	}
	var envelope attacksListEnvelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, domain.ArvanCloudReportPageMeta{}, fmt.Errorf("decoding arvancloud attacks list for domain %q: %w", domainName, err)
		}
	}
	out := make([]domain.ArvanCloudAttackReportItem, len(envelope.Data))
	for i, a := range envelope.Data {
		out[i] = domain.ArvanCloudAttackReportItem{
			AttackerIP: a.AttackerIP, AttackerCountry: a.AttackerCountry, Method: a.Method, URI: a.URI,
			Host: a.Host, Timestamp: a.Timestamp, URIArgs: a.URIArgs, Cookie: a.Cookie, Alerts: a.Alerts, UserAgent: a.UserAgent,
		}
	}
	return out, toReportPageMetaDomain(envelope.Meta), nil
}

// attackerWire mirrors reports.attacks.attackers' inline item schema.
type attackerWire struct {
	IP    string `json:"ip"`
	Count int64  `json:"count"`
}

// ListArvanCloudAttackers returns attacker info for a domain
// (reports.attacks.attackers). Period only.
func (p *Provider) ListArvanCloudAttackers(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) ([]domain.ArvanCloudAttacker, error) {
	path := attackersPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{}).Encode()
	var items []attackerWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud attackers for domain %q: %w", domainName, err)
	}
	out := make([]domain.ArvanCloudAttacker, len(items))
	for i, a := range items {
		out[i] = domain.ArvanCloudAttacker{IP: a.IP, Count: a.Count}
	}
	return out, nil
}

// attackReportMapWire mirrors AttackReportMap's "data" (reports.attacks.map).
// Statistics is deprecated in the spec and not decoded here.
type attackReportMapWire struct {
	Charts struct {
		Attacks map[string]geoMapEntryWire `json:"attacks"`
	} `json:"charts"`
	Lists []struct {
		Country string `json:"country"`
		Name    string `json:"name"`
		Code    string `json:"code"`
		Attack  int64  `json:"attack"`
	} `json:"lists"`
}

// GetArvanCloudAttackMap returns a geo-map of attacks for a domain
// (reports.attacks.map). Period only.
func (p *Provider) GetArvanCloudAttackMap(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudAttackMapReport, error) {
	path := attackMapPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{}).Encode()
	var wire attackReportMapWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud attack map for domain %q: %w", domainName, err)
	}
	lists := make([]domain.ArvanCloudAttackMapCountry, len(wire.Lists))
	for i, l := range wire.Lists {
		lists[i] = domain.ArvanCloudAttackMapCountry{Country: l.Country, Name: l.Name, Code: l.Code, Attack: l.Attack}
	}
	return &domain.ArvanCloudAttackMapReport{Chart: toGeoMapDomain(wire.Charts.Attacks), Lists: lists}, nil
}

// attackedURIWire mirrors the AttackReportUri schema.
type attackedURIWire struct {
	URI   string `json:"uri"`
	Count int64  `json:"count"`
}

// ListArvanCloudAttackedURIs returns the list of URLs under attack for a
// domain (reports.attacks.uri). Period only.
func (p *Provider) ListArvanCloudAttackedURIs(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) ([]domain.ArvanCloudAttackedURI, error) {
	path := attackedURIsPath(domainName) + "?" + arvanCloudReportQueryValues(query, reportQueryOptions{}).Encode()
	var items []attackedURIWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud attacked uris for domain %q: %w", domainName, err)
	}
	out := make([]domain.ArvanCloudAttackedURI, len(items))
	for i, u := range items {
		out[i] = domain.ArvanCloudAttackedURI{URI: u.URI, Count: u.Count}
	}
	return out, nil
}

// --- Transport layer proxy traffic ------------------------------------------

// transportLayerProxyTrafficWire mirrors TransportLayerProxyTrafficCharts'
// "data" (reports.transport_layer_proxies.traffics).
type transportLayerProxyTrafficWire struct {
	Charts struct {
		Traffics reportChartWire `json:"traffics"`
	} `json:"charts"`
}

// GetArvanCloudTransportLayerProxyTraffic returns traffic for one Transport
// Layer Proxy (reports.transport_layer_proxies.traffics). See
// ports.ArvanCloudProvider's own doc comment for why transportLayerProxyID
// is a caller-supplied opaque ID.
func (p *Provider) GetArvanCloudTransportLayerProxyTraffic(ctx context.Context, creds domain.ProviderCredentials, domainName, transportLayerProxyID string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudTransportLayerProxyTraffic, error) {
	path := transportLayerProxyTrafficPath(domainName, transportLayerProxyID) + "?" +
		arvanCloudReportQueryValues(query, reportQueryOptions{sinceUntil: true}).Encode()
	var wire transportLayerProxyTrafficWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud transport layer proxy %q traffic for domain %q: %w", transportLayerProxyID, domainName, err)
	}
	return &domain.ArvanCloudTransportLayerProxyTraffic{Chart: toReportChartDomain(wire.Charts.Traffics)}, nil
}

// DownloadArvanCloudDomainsReport returns a CSV export of the domains report
// (domains.reports.download, text/csv — not JSON, and not scoped to a single
// domain, unlike every other method in this file). Follows the same
// "return the raw body" convention ExportArvanCloudDNSRecords uses for its
// own non-JSON response.
func (p *Provider) DownloadArvanCloudDomainsReport(ctx context.Context, creds domain.ProviderCredentials) (string, error) {
	data, err := p.client.doRawGET(ctx, creds, domainsReportDownloadPath, "text/csv")
	if err != nil {
		return "", fmt.Errorf("downloading arvancloud domains report: %w", err)
	}
	return string(data), nil
}

// --- Aggregated Reports (account-wide) --------------------------------------

// aggregatedReportDetailWire mirrors the AggregatedReportsResponse schema.
// ASN/POP are nullable per the spec.
type aggregatedReportDetailWire struct {
	Domain          string  `json:"domain"`
	TotalDownstream int64   `json:"total_downstream"`
	TotalUpstream   int64   `json:"total_upstream"`
	TotalRequests   int64   `json:"total_requests"`
	ASN             *int64  `json:"asn"`
	POP             *string `json:"pop"`
}

// aggregatedReportDetailsEnvelope mirrors the PaginatedResponse shape
// reports.aggregated.details returns — same top-level data+meta shape as
// highRequestIPsEnvelope above.
type aggregatedReportDetailsEnvelope struct {
	Data []aggregatedReportDetailWire `json:"data"`
	Meta paginatedResponseMetaWire    `json:"meta"`
}

// ListArvanCloudAggregatedReportDetails returns a page of aggregated reports
// across domains (reports.aggregated.details).
func (p *Provider) ListArvanCloudAggregatedReportDetails(ctx context.Context, creds domain.ProviderCredentials, query domain.ArvanCloudAggregatedReportQuery) ([]domain.ArvanCloudAggregatedReportDetail, domain.ArvanCloudReportPageMeta, error) {
	path := aggregatedReportDetailsPath + "?" + aggregatedQueryValues(query, aggregatedQueryOptions{filters: true, paging: true}).Encode()
	raw, err := p.client.doRawGET(ctx, creds, path, "application/json")
	if err != nil {
		return nil, domain.ArvanCloudReportPageMeta{}, fmt.Errorf("listing arvancloud aggregated report details: %w", err)
	}
	var envelope aggregatedReportDetailsEnvelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, domain.ArvanCloudReportPageMeta{}, fmt.Errorf("decoding arvancloud aggregated report details: %w", err)
		}
	}
	out := make([]domain.ArvanCloudAggregatedReportDetail, len(envelope.Data))
	for i, d := range envelope.Data {
		detail := domain.ArvanCloudAggregatedReportDetail{
			Domain: d.Domain, TotalDownstream: d.TotalDownstream, TotalUpstream: d.TotalUpstream, TotalRequests: d.TotalRequests,
		}
		if d.ASN != nil {
			detail.ASN = *d.ASN
		}
		if d.POP != nil {
			detail.POP = *d.POP
		}
		out[i] = detail
	}
	return out, toReportPageMetaDomain(envelope.Meta), nil
}

// aggregatedReportChartsEnvelopeWire mirrors reports.aggregated.charts'
// response, which nests the chart one level deeper than usual — under
// "data.charts" (the AggregatedReportsChartsResponse schema) rather than
// "data" itself. AggregatedReportsChartsResponse is otherwise exactly the
// {title, categories[], series[]} shape reportChartWire already models, so
// it is reused directly rather than declaring a near-duplicate type.
type aggregatedReportChartsEnvelopeWire struct {
	Charts reportChartWire `json:"charts"`
}

// GetArvanCloudAggregatedReportCharts returns charts of aggregated reports
// across domains (reports.aggregated.charts).
func (p *Provider) GetArvanCloudAggregatedReportCharts(ctx context.Context, creds domain.ProviderCredentials, query domain.ArvanCloudAggregatedReportQuery) (*domain.ArvanCloudReportChart, error) {
	path := aggregatedReportChartsPath + "?" + aggregatedQueryValues(query, aggregatedQueryOptions{reportType: true, filters: true}).Encode()
	var wire aggregatedReportChartsEnvelopeWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud aggregated report charts: %w", err)
	}
	chart := toReportChartDomain(wire.Charts)
	return &chart, nil
}

// aggregatedReportFiltersWire mirrors the AggregatedReportsFilterResponse
// schema.
type aggregatedReportFiltersWire struct {
	Pops []struct {
		Pop   string `json:"pop"`
		Label string `json:"label"`
	} `json:"pops"`
	Asns []int64 `json:"asns"`
}

// GetArvanCloudAggregatedReportFilters returns the combined filtering data
// (pops/asns) available across domains (reports.aggregated.filters).
func (p *Provider) GetArvanCloudAggregatedReportFilters(ctx context.Context, creds domain.ProviderCredentials, query domain.ArvanCloudAggregatedReportQuery) (*domain.ArvanCloudAggregatedReportFilters, error) {
	path := aggregatedReportFiltersPath + "?" + aggregatedQueryValues(query, aggregatedQueryOptions{}).Encode()
	var wire aggregatedReportFiltersWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud aggregated report filters: %w", err)
	}
	pops := make([]domain.ArvanCloudAggregatedReportFilterPOP, len(wire.Pops))
	for i, pop := range wire.Pops {
		pops[i] = domain.ArvanCloudAggregatedReportFilterPOP{POP: pop.Pop, Label: pop.Label}
	}
	return &domain.ArvanCloudAggregatedReportFilters{POPs: pops, Asns: wire.Asns}, nil
}
