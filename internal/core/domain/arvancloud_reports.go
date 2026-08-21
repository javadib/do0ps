package domain

import "encoding/json"

// The types below model ArvanCloud's Reports (per-domain) and Aggregated
// Reports (account-wide) capabilities (issue #75), confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Reports" and "Aggregated Reports"
// tags. All 23 operations across both tags are read-only GET endpoints — no
// create/update/delete anywhere in this file.
//
// Three shapes recur across most of the individual report schemas, and are
// modeled once here rather than once per endpoint:
//
//   - ArvanCloudReportChart: the {title/name, categories[], series[{name,
//     data[]}]} time-series shape (e.g. TrafficCharts.traffics, VisitorsCharts,
//     ErrorLogChart.charts.status_code, TransportLayerProxyTrafficCharts,
//     AggregatedReportsChartsResponse). Confirmed to recur across traffics,
//     visitors, response-time, status (categories/series, keyed "name" not
//     "title" on the wire, translated by the adapter), dns-requests, attacks,
//     error-logs/chart, transport-layer-proxies/traffics and the aggregated
//     charts endpoint — but NOT traffics/saved or status/summary, whose own
//     charts are the {name, y} pie shape instead (see ArvanCloudPieSlice
//     below). Series values are kept as float64
//     because the spec declares some as integer/int64 (byte/request counts)
//     and others as number/double (response-time milliseconds) — float64
//     round-trips both without a second parallel type, at the cost of exact
//     int64 precision above 2^53, which no report endpoint here plausibly
//     reaches.
//   - ArvanCloudPieSlice: the {name, y} pie-chart shape (SavedTrafficsCharts,
//     StatusCodeSummary.charts.status_code).
//   - ArvanCloudGeoMapEntry: the country-code -> {fillKey, name, value} map
//     shape shared by TrafficsMap.charts, DnsGeoReport.charts.requests and
//     AttackReportMap.charts.attacks.
//
// The per-endpoint "statistics" blocks and "lists" arrays are NOT forced into
// one shared shape: each declares its own small, genuinely different field
// set (e.g. TrafficStatistics' saved/bypass/top/total vs.
// StatusCodeReport.statistics.status_codes' 2xx_sum/3xx_sum/4xx_sum/5xx_sum),
// so each gets its own type below, the same granularity
// cdn_report_ssl.go/arvancloud_healthcheck.go use for their own
// endpoint-specific response fields.

// --- Shared query and building-block types ---------------------------------

// ArvanCloudReportQuery is the filter accepted by every per-domain Reports
// endpoint in this file. Not every field applies to every endpoint — each
// field's own comment says which do, the same "shared struct, endpoint-
// specific subset" convention CDNLogQuery and
// ArvanCloudHealthCheckReportQuery use.
type ArvanCloudReportQuery struct {
	// Period selects a period ending now. Must satisfy
	// ValidArvanCloudHealthCheckReportPeriod (the identical eight-value enum
	// this file's Reports tag and the Active Health Check reports share —
	// reused rather than redeclared). Honored by every endpoint in this file
	// except domains.reports.download (no parameters at all) and the three
	// Aggregated Reports endpoints (see ArvanCloudAggregatedReportQuery).
	Period string
	// Since/Until bound the report window explicitly, ISO 8601 UTC (e.g.
	// "1988-07-11T00:00:00Z"). Honored by traffics, traffics/saved,
	// traffics/map, visitors, response-time, high-request-ips, status,
	// status/summary, error-logs, error-logs/chart, error-log-details,
	// dns-requests, dns-geo and the transport-layer-proxy traffic report.
	// NOT honored by any of the five attacks/* endpoints, which the spec
	// declares with Period only.
	Since string
	Until string
	// Subdomain selects one subdomain to report on ("@" means the root
	// domain), the spec's "filter[subdomain]" query parameter. Honored only
	// by traffics, traffics/saved, traffics/map, visitors and response-time —
	// the five endpoints the spec lists FilterSubdomain on.
	Subdomain string
	// PerPage/Page paginate a result. Zero leaves both to the provider's own
	// defaults. Honored only by high-request-ips and attacks/list (the two
	// per-domain Reports endpoints the spec paginates).
	PerPage int
	Page    int
	// Error is a free-text error message to search for. Honored only by the
	// deprecated error-log-details endpoint.
	Error string
}

// ArvanCloudReportChart is the generic time-series chart shape recurring
// across most Reports/Aggregated Reports endpoints — see this file's package
// comment for which endpoints use it and why Series' values are float64.
type ArvanCloudReportChart struct {
	// Title is the chart's title/name as reported by the provider (the wire
	// field is "title" on most endpoints, "name" on status.index's chart —
	// the adapter normalizes both onto this one field).
	Title      string
	Categories []string
	Series     []ArvanCloudReportChartSeries
}

// ArvanCloudReportChartSeries is one named data series within an
// ArvanCloudReportChart.
type ArvanCloudReportChartSeries struct {
	Name string
	Data []float64
}

// ArvanCloudPieSlice is one slice of a pie-chart-shaped report (the wire
// shape is {"name": ..., "y": ...}).
type ArvanCloudPieSlice struct {
	Name  string
	Value int64
}

// ArvanCloudGeoMapEntry is one country's entry in a geo-bucketed map report
// (a country-code-keyed object on the wire, e.g. {"IRN": {"fillKey": 4,
// "name": "Iran (Islamic Republic of)", "value": 12456789}}).
type ArvanCloudGeoMapEntry struct {
	CountryCode string
	FillKey     int64
	Name        string
	Value       int64
}

// ArvanCloudReportPageMeta is the pagination info attached to a paginated
// Reports/Aggregated Reports result (the spec's PaginatedResponseMeta
// schema), the same shape ArvanCloudHealthCheckReportPageMeta and
// ArvanCloudPageRulePageMeta give their own respective endpoints.
type ArvanCloudReportPageMeta struct {
	CurrentPage int
	From        int
	LastPage    int
	PerPage     int
	To          int
	Total       int
}

// --- Traffic reports ---------------------------------------------------

// ArvanCloudTrafficStat is one of TrafficStatistics.traffics/.requests
// (reports.traffics.total's statistics block): Saved/Top/Total are always
// present per the spec; Bypass is not marked required.
type ArvanCloudTrafficStat struct {
	Saved  int64
	Bypass int64
	Top    string
	Total  int64
}

// ArvanCloudTrafficReport is GetArvanCloudTrafficReport's result
// (reports.traffics.total, the Traffics schema).
type ArvanCloudTrafficReport struct {
	TrafficStatistics ArvanCloudTrafficStat
	RequestStatistics ArvanCloudTrafficStat
	TrafficChart      ArvanCloudReportChart
	RequestChart      ArvanCloudReportChart
}

// ArvanCloudSavedStat is one of SavedTrafficsStatistics.traffic/.request
// (reports.traffics.saved's statistics block): just saved/total, unlike
// ArvanCloudTrafficStat above.
type ArvanCloudSavedStat struct {
	Saved int64
	Total int64
}

// ArvanCloudTrafficSavedReport is GetArvanCloudTrafficSavedReport's result
// (reports.traffics.saved, the SavedTrafficsData schema): a bandwidth-saved
// pie chart plus its underlying totals.
type ArvanCloudTrafficSavedReport struct {
	TrafficStatistics ArvanCloudSavedStat
	RequestStatistics ArvanCloudSavedStat
	// TrafficChart/RequestChart are pie slices: hit vs. miss, for traffic
	// volume and request count respectively.
	TrafficChart []ArvanCloudPieSlice
	RequestChart []ArvanCloudPieSlice
}

// ArvanCloudTrafficMapCountry is one entry of TrafficsMap.lists
// (reports.traffics.map's CountryList schema).
type ArvanCloudTrafficMapCountry struct {
	Country  string
	Code     string
	Requests int64
	Traffics int64
}

// ArvanCloudTrafficMapReport is GetArvanCloudTrafficMap's result
// (reports.traffics.map, the MapTrafficsData schema): traffic as a geo-map.
// TrafficsMap.statistics is deprecated and not modeled here — Lists is its
// non-deprecated replacement per the spec.
type ArvanCloudTrafficMapReport struct {
	// RequestsChart/TrafficsChart are keyed by 3-letter country code (the
	// spec's own example: "IRN").
	RequestsChart map[string]ArvanCloudGeoMapEntry
	TrafficsChart map[string]ArvanCloudGeoMapEntry
	Lists         []ArvanCloudTrafficMapCountry
}

// --- Visitors ------------------------------------------------------------

// ArvanCloudVisitorsReport is GetArvanCloudVisitorsReport's result
// (reports.visitors.index, the VisitorsData schema).
type ArvanCloudVisitorsReport struct {
	TopVisitors   string
	TotalVisitors int64
	Chart         ArvanCloudReportChart
}

// ArvanCloudHighRequestIP is one entry of ListArvanCloudHighRequestIPs'
// result (reports.visitors.high-request-ips, the HighRequestedIp schema).
type ArvanCloudHighRequestIP struct {
	IP           string
	RequestCount int64
}

// --- Response time ---------------------------------------------------------

// ArvanCloudResponseTimeReport is GetArvanCloudResponseTimeReport's result
// (reports.response-time.index, the ResponseTime schema). The spec declares
// only a "charts.ir" chart on this schema — no statistics block, unlike most
// of the other reports in this file.
type ArvanCloudResponseTimeReport struct {
	Chart ArvanCloudReportChart
}

// --- Status code reports ----------------------------------------------------

// ArvanCloudStatusCodeStat is StatusCodeReport.statistics.status_codes
// (reports.status.index's statistics block): request counts bucketed by
// status code class.
type ArvanCloudStatusCodeStat struct {
	TwoXX   int64
	ThreeXX int64
	FourXX  int64
	FiveXX  int64
}

// ArvanCloudStatusCodeReport is GetArvanCloudStatusCodeReport's result
// (reports.status.index, the StatusCodeReport schema): a status-code pie
// chart, reported as a time series (the chart itself carries categories/
// series, not name/y pairs — see ArvanCloudStatusCodeSummary below for the
// actual pie-chart endpoint despite this one's "pie chart" summary text).
type ArvanCloudStatusCodeReport struct {
	Statistics ArvanCloudStatusCodeStat
	Chart      ArvanCloudReportChart
}

// ArvanCloudStatusCodeSummary is GetArvanCloudStatusCodeSummary's result
// (reports.status.summary, the StatusCodeSummary schema). Statistics is
// deprecated on the wire and not modeled here.
type ArvanCloudStatusCodeSummary struct {
	Chart []ArvanCloudPieSlice
}

// --- Error logs --------------------------------------------------------

// ArvanCloudErrorLogUpstream is one entry of ArvanCloudErrorLog.Upstreams.
type ArvanCloudErrorLogUpstream struct {
	Address string
	Count   int64
}

// ArvanCloudErrorLog is one entry of ListArvanCloudErrorLogs' result
// (reports.error-logs, the ErrorLog schema).
type ArvanCloudErrorLog struct {
	Name      string
	Count     int64
	Upstreams []ArvanCloudErrorLogUpstream
}

// ArvanCloudErrorLogsChart is GetArvanCloudErrorLogsChart's result
// (reports.error-logs.chart, the ErrorLogChart schema). Statistics is
// map[string]int64 per the spec's own description of its "additionalProperties:
// true" field: "<key, value> where key is error and value is its count" —
// the most conservative reading of an otherwise untyped field (the same
// documented-inference approach AC14 used for an undocumented enum).
type ArvanCloudErrorLogsChart struct {
	Statistics map[string]int64
	Chart      ArvanCloudReportChart
}

// ArvanCloudErrorLogDetail is GetArvanCloudErrorLogDetail's result
// (reports.error-log-details, deprecated in the spec). The spec declares
// this endpoint's "data" as a bare `type: object` with no properties at all —
// genuinely no shape to model. Kept as raw JSON rather than guessed at,
// matching cdn_rules.go's ValueDetail field for the same situation: an
// endpoint whose response has no confirmed stable shape.
type ArvanCloudErrorLogDetail struct {
	Raw json.RawMessage
}

// --- DNS reports -------------------------------------------------------

// ArvanCloudDnsRequestsReport is GetArvanCloudDnsRequestsReport's result
// (reports.dns.requests, the DnsRequestReport schema).
type ArvanCloudDnsRequestsReport struct {
	Total int64
	Top   string
	Chart ArvanCloudReportChart
}

// ArvanCloudDnsGeoCountry is one entry of DnsGeoReport.lists.
type ArvanCloudDnsGeoCountry struct {
	Country  string
	Name     string
	Code     string
	Requests int64
}

// ArvanCloudDnsGeoReport is GetArvanCloudDnsGeoReport's result
// (reports.dns.geo, the DnsGeoReport schema): DNS requests as a geo-map.
// Statistics is deprecated on the wire and not modeled here.
type ArvanCloudDnsGeoReport struct {
	// RequestsChart is keyed by 3-letter country code, same convention as
	// ArvanCloudTrafficMapReport.RequestsChart.
	RequestsChart map[string]ArvanCloudGeoMapEntry
	Lists         []ArvanCloudDnsGeoCountry
}

// --- Attack reports ------------------------------------------------------

// ArvanCloudAttackReport is GetArvanCloudAttackReport's result
// (reports.attacks.show, the AttackReport schema).
type ArvanCloudAttackReport struct {
	TotalAttacks int64
	TopAttacks   string
	Chart        ArvanCloudReportChart
}

// ArvanCloudAttackReportItem is one entry of ListArvanCloudAttacks' result
// (reports.attacks.index, the AttackReportItem schema).
type ArvanCloudAttackReportItem struct {
	AttackerIP      string
	AttackerCountry string
	// Method is one of GET/POST/PUT/PATCH/DELETE/HEAD/OPTION per the spec's
	// enum.
	Method    string
	URI       string
	Host      []string
	Timestamp string
	URIArgs   string
	Cookie    string
	Alerts    []string
	UserAgent []string
}

// ArvanCloudAttacker is one entry of ListArvanCloudAttackers' result
// (reports.attacks.attackers, an inline schema: {"ip", "count"}).
type ArvanCloudAttacker struct {
	IP    string
	Count int64
}

// ArvanCloudAttackMapCountry is one entry of AttackReportMap.lists. Per the
// spec's own field descriptions, "Country" here is documented as the
// 2-letter code and "Code" as the 3-letter code — kept exactly as the spec
// names them, even though this reads as swapped relative to
// ArvanCloudDnsGeoCountry/ArvanCloudTrafficMapCountry's Country=name,
// Code=2-letter convention; this is the spec's own inconsistency, not this
// adapter's.
type ArvanCloudAttackMapCountry struct {
	Country string
	Name    string
	Code    string
	Attack  int64
}

// ArvanCloudAttackMapReport is GetArvanCloudAttackMap's result
// (reports.attacks.map, the AttackReportMap schema). Statistics is
// deprecated on the wire and not modeled here.
type ArvanCloudAttackMapReport struct {
	// Chart is keyed by 3-letter country code, same convention as
	// ArvanCloudTrafficMapReport.RequestsChart.
	Chart map[string]ArvanCloudGeoMapEntry
	Lists []ArvanCloudAttackMapCountry
}

// ArvanCloudAttackedURI is one entry of ListArvanCloudAttackedURIs' result
// (reports.attacks.uri, the AttackReportUri schema).
type ArvanCloudAttackedURI struct {
	URI   string
	Count int64
}

// --- Transport layer proxy traffic ------------------------------------------

// ArvanCloudTransportLayerProxyTraffic is
// GetArvanCloudTransportLayerProxyTraffic's result
// (reports.transport_layer_proxies.traffics, the
// TransportLayerProxyTrafficCharts schema). transportLayerProxyId is a
// caller-supplied opaque ID (see
// ports.ArvanCloudProvider.GetArvanCloudTransportLayerProxyTraffic's doc
// comment): the spec has no create/list endpoint for this resource type
// anywhere else, so this port does not invent CRUD for it.
type ArvanCloudTransportLayerProxyTraffic struct {
	Chart ArvanCloudReportChart
}

// --- Aggregated Reports (account-wide) --------------------------------------

// ArvanCloudAggregatedReportQuery is the filter shared by all three
// Aggregated Reports endpoints. Not every field applies to every endpoint —
// each field's own comment says which do. Deliberately a separate type from
// ArvanCloudReportQuery: these three endpoints are account-wide (no domain
// path segment) and take a different, unrelated field set (comma-separated
// domain names instead of one domain in the URL, category/pop/asn filters
// instead of a subdomain filter).
type ArvanCloudAggregatedReportQuery struct {
	// Domains is a comma-separated list of domain names to include, e.g.
	// "example1.com,example2.com". Empty means every domain visible to the
	// credentials. Honored by all three endpoints.
	Domains string
	// ReportType filters reports.aggregated.charts to "traffic" or
	// "requests". Ignored by the other two endpoints (the spec does not
	// declare this parameter on them).
	ReportType string
	// CategoryType selects "pop" or "asn" as the report's bucketing
	// dimension. Honored by details and charts; ignored by filters.
	CategoryType string
	// Pops/Asns are comma-separated filters, e.g. "thr-r1c,thr-mci" and
	// "1435,7846" respectively. Honored by details and charts; ignored by
	// filters.
	Pops string
	Asns string
	// Period selects a period ending now. Must satisfy
	// ValidArvanCloudHealthCheckReportPeriod, same enum as
	// ArvanCloudReportQuery.Period. Honored by details and charts; ignored
	// by filters.
	Period string
	// PerPage/Page paginate a result. Zero leaves both to the provider's own
	// defaults. Honored only by details (reports.aggregated.details is the
	// only paginated endpoint of the three).
	PerPage int
	Page    int
}

// ArvanCloudAggregatedReportDetail is one entry of
// ListArvanCloudAggregatedReportDetails' result (reports.aggregated.details,
// the AggregatedReportsResponse schema). ASN/POP are nullable per the spec —
// zero value (0 / "") means the provider reported none for this bucket.
type ArvanCloudAggregatedReportDetail struct {
	Domain          string
	TotalDownstream int64
	TotalUpstream   int64
	TotalRequests   int64
	ASN             int64
	POP             string
}

// ArvanCloudAggregatedReportFilterPOP is one entry of
// ArvanCloudAggregatedReportFilters.POPs.
type ArvanCloudAggregatedReportFilterPOP struct {
	POP   string
	Label string
}

// ArvanCloudAggregatedReportFilters is
// GetArvanCloudAggregatedReportFilters' result (reports.aggregated.filters,
// the AggregatedReportsFilterResponse schema): the filter dimensions
// available to reports.aggregated.details/.charts' own pops/asns
// parameters.
type ArvanCloudAggregatedReportFilters struct {
	POPs []ArvanCloudAggregatedReportFilterPOP
	Asns []int64
}
