package mcp

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud Reports (per-domain) and Aggregated Reports (account-wide)
// tools (issue #75): pure GET traffic/security/DNS analytics. All fast
// operations (AGENTS.md 4.3): every tool below returns its result within the
// call, with no operation_id to poll afterward.

// --- Shared query property/args helpers -------------------------------

// arvanCloudReportQueryOptions selects which of the shared report query
// properties/args a given tool below actually exposes, matching exactly
// which subset the underlying endpoint honors per
// domain.ArvanCloudReportQuery's own field comments — period is always
// exposed when set.
type arvanCloudReportQueryOptions struct {
	sinceUntil bool
	subdomain  bool
	paging     bool
	errorParam bool
}

// arvanCloudReportPeriodProperty describes the "period" parameter every
// per-domain Reports tool below accepts.
func arvanCloudReportPeriodProperty() map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{"5m", "1h", "3h", "6h", "12h", "24h", "7d", "30d"},
		"description": "A period ending now to report over, e.g. \"1h\" for the last hour or \"24h\" for the last " +
			"day. \"5m\" is available only for enterprise domains. Omit to let ArvanCloud apply its own default window.",
	}
}

// arvanCloudReportQueryProperties adds the query properties opts selects to
// props. Every property documents its time-range format explicitly per
// AGENTS.md 5.
func arvanCloudReportQueryProperties(props map[string]any, opts arvanCloudReportQueryOptions) {
	props["period"] = arvanCloudReportPeriodProperty()
	if opts.sinceUntil {
		props["since"] = map[string]any{
			"type": "string",
			"description": "Report window start, as an ISO 8601 UTC timestamp, e.g. \"2026-08-01T00:00:00Z\". " +
				"May be combined with \"period\". Omit to let ArvanCloud apply its own default.",
		}
		props["until"] = map[string]any{
			"type": "string",
			"description": "Report window end, as an ISO 8601 UTC timestamp, e.g. \"2026-08-21T00:00:00Z\". " +
				"Omit to let ArvanCloud apply its own default.",
		}
	}
	if opts.subdomain {
		props["subdomain"] = map[string]any{
			"type": "string",
			"description": "Report on one subdomain only, e.g. \"www\" or \"api\". Use \"@\" for the root domain. " +
				"Omit to report on the whole domain.",
		}
	}
	if opts.paging {
		props["per_page"] = map[string]any{"type": "integer", "description": "How many results per page. Omit for ArvanCloud's own default."}
		props["page"] = map[string]any{"type": "integer", "description": "Which page to return, 1-indexed. Omit for page 1."}
	}
	if opts.errorParam {
		props["error_message"] = map[string]any{"type": "string", "description": "An error message to search for, e.g. \"upstream timed out\"."}
	}
}

// arvanCloudReportQueryArgs decodes the query fields every per-domain
// Reports tool below shares. Fields a given tool's own InputSchema does not
// advertise simply stay at their zero value, which the adapter then omits
// from the outbound request (see arvancloud/reports.go's
// arvanCloudReportQueryValues and its per-endpoint reportQueryOptions).
type arvanCloudReportQueryArgs struct {
	Period       string `json:"period"`
	Since        string `json:"since"`
	Until        string `json:"until"`
	Subdomain    string `json:"subdomain"`
	PerPage      int    `json:"per_page"`
	Page         int    `json:"page"`
	ErrorMessage string `json:"error_message"`
}

func (a arvanCloudReportQueryArgs) toDomain() domain.ArvanCloudReportQuery {
	return domain.ArvanCloudReportQuery{
		Period: a.Period, Since: a.Since, Until: a.Until, Subdomain: a.Subdomain,
		PerPage: a.PerPage, Page: a.Page, Error: a.ErrorMessage,
	}
}

// arvanCloudDomainReportArgs is decoded by every per-domain report tool
// below.
type arvanCloudDomainReportArgs struct {
	arvanCloudDomainNameArgs
	arvanCloudReportQueryArgs
}

// --- Shared result-rendering helpers ------------------------------------

// arvanCloudReportChartToMap renders a domain.ArvanCloudReportChart.
func arvanCloudReportChartToMap(c domain.ArvanCloudReportChart) map[string]any {
	series := make([]map[string]any, len(c.Series))
	for i, s := range c.Series {
		series[i] = map[string]any{"name": s.Name, "data": s.Data}
	}
	return map[string]any{"title": c.Title, "categories": c.Categories, "series": series}
}

// arvanCloudPieSlicesToMaps renders a []domain.ArvanCloudPieSlice.
func arvanCloudPieSlicesToMaps(items []domain.ArvanCloudPieSlice) []map[string]any {
	out := make([]map[string]any, len(items))
	for i, s := range items {
		out[i] = map[string]any{"name": s.Name, "value": s.Value}
	}
	return out
}

// arvanCloudGeoMapToMaps renders a country-code-keyed
// map[string]domain.ArvanCloudGeoMapEntry as a sorted (by country code)
// slice, for a deterministic MCP response.
func arvanCloudGeoMapToMaps(m map[string]domain.ArvanCloudGeoMapEntry) []map[string]any {
	codes := make([]string, 0, len(m))
	for code := range m {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	out := make([]map[string]any, len(codes))
	for i, code := range codes {
		e := m[code]
		out[i] = map[string]any{"country_code": code, "fill_key": e.FillKey, "name": e.Name, "value": e.Value}
	}
	return out
}

// arvanCloudReportPageMetaToMap renders a domain.ArvanCloudReportPageMeta.
func arvanCloudReportPageMetaToMap(m domain.ArvanCloudReportPageMeta) map[string]any {
	return map[string]any{
		"current_page": m.CurrentPage, "from": m.From, "last_page": m.LastPage,
		"per_page": m.PerPage, "to": m.To, "total": m.Total,
	}
}

// --- Traffic reports -----------------------------------------------------

func getArvanCloudTrafficReportTool(uc *app.GetArvanCloudTrafficReport) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true, subdomain: true})

	return Tool{
		Name:        "get_arvancloud_traffic_report",
		Description: "Get a domain's traffic report: total/saved/bypassed bandwidth and request counts, plus a time-series chart. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			report, err := uc.Execute(ctx, app.GetArvanCloudTrafficReportInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"traffic_statistics": map[string]any{"saved": report.TrafficStatistics.Saved, "bypass": report.TrafficStatistics.Bypass, "top": report.TrafficStatistics.Top, "total": report.TrafficStatistics.Total},
				"request_statistics": map[string]any{"saved": report.RequestStatistics.Saved, "bypass": report.RequestStatistics.Bypass, "top": report.RequestStatistics.Top, "total": report.RequestStatistics.Total},
				"traffic_chart":      arvanCloudReportChartToMap(report.TrafficChart),
				"request_chart":      arvanCloudReportChartToMap(report.RequestChart),
			}, nil
		},
	}
}

func getArvanCloudTrafficSavedReportTool(uc *app.GetArvanCloudTrafficSavedReport) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true, subdomain: true})

	return Tool{
		Name:        "get_arvancloud_traffic_saved_report",
		Description: "Get a domain's bandwidth-saved-by-caching pie chart, for traffic volume and request count. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			report, err := uc.Execute(ctx, app.GetArvanCloudTrafficSavedReportInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"traffic_statistics": map[string]any{"saved": report.TrafficStatistics.Saved, "total": report.TrafficStatistics.Total},
				"request_statistics": map[string]any{"saved": report.RequestStatistics.Saved, "total": report.RequestStatistics.Total},
				"traffic_chart":      arvanCloudPieSlicesToMaps(report.TrafficChart),
				"request_chart":      arvanCloudPieSlicesToMaps(report.RequestChart),
			}, nil
		},
	}
}

func getArvanCloudTrafficMapTool(uc *app.GetArvanCloudTrafficMap) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true, subdomain: true})

	return Tool{
		Name:        "get_arvancloud_traffic_map",
		Description: "Get a domain's traffic report broken down by country, as a geo-map plus a per-country list. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			report, err := uc.Execute(ctx, app.GetArvanCloudTrafficMapInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			lists := make([]map[string]any, len(report.Lists))
			for i, l := range report.Lists {
				lists[i] = map[string]any{"country": l.Country, "code": l.Code, "requests": l.Requests, "traffics": l.Traffics}
			}
			return map[string]any{
				"requests_map": arvanCloudGeoMapToMaps(report.RequestsChart),
				"traffics_map": arvanCloudGeoMapToMaps(report.TrafficsChart),
				"countries":    lists,
			}, nil
		},
	}
}

// --- Visitors --------------------------------------------------------------

func getArvanCloudVisitorsReportTool(uc *app.GetArvanCloudVisitorsReport) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true, subdomain: true})

	return Tool{
		Name:        "get_arvancloud_visitors_report",
		Description: "Get a domain's visitor report: total/top visitor counts, plus a time-series chart. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			report, err := uc.Execute(ctx, app.GetArvanCloudVisitorsReportInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"top_visitors":   report.TopVisitors,
				"total_visitors": report.TotalVisitors,
				"chart":          arvanCloudReportChartToMap(report.Chart),
			}, nil
		},
	}
}

func getArvanCloudHighRequestIPsTool(uc *app.ListArvanCloudHighRequestIPs) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true, paging: true})

	return Tool{
		Name:        "get_arvancloud_high_request_ips",
		Description: "Get a page of the IPs sending the most requests to a domain. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			result, err := uc.Execute(ctx, app.ListArvanCloudHighRequestIPsInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(result.IPs))
			for i, ip := range result.IPs {
				out[i] = map[string]any{"ip": ip.IP, "request_count": ip.RequestCount}
			}
			return map[string]any{"ips": out, "page": arvanCloudReportPageMetaToMap(result.Page)}, nil
		},
	}
}

// --- Response time -----------------------------------------------------

func getArvanCloudResponseTimeReportTool(uc *app.GetArvanCloudResponseTimeReport) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true, subdomain: true})

	return Tool{
		Name:        "get_arvancloud_response_time_report",
		Description: "Get a domain's server response-time report, as a time-series chart (milliseconds). This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			report, err := uc.Execute(ctx, app.GetArvanCloudResponseTimeReportInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return map[string]any{"chart": arvanCloudReportChartToMap(report.Chart)}, nil
		},
	}
}

// --- Status code reports -----------------------------------------------

func getArvanCloudStatusCodeReportTool(uc *app.GetArvanCloudStatusCodeReport) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true})

	return Tool{
		Name:        "get_arvancloud_status_code_report",
		Description: "Get a domain's response status codes broken down by class (2xx/3xx/4xx/5xx), plus a time-series chart. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			report, err := uc.Execute(ctx, app.GetArvanCloudStatusCodeReportInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"statistics": map[string]any{
					"2xx": report.Statistics.TwoXX, "3xx": report.Statistics.ThreeXX,
					"4xx": report.Statistics.FourXX, "5xx": report.Statistics.FiveXX,
				},
				"chart": arvanCloudReportChartToMap(report.Chart),
			}, nil
		},
	}
}

func getArvanCloudStatusCodeSummaryTool(uc *app.GetArvanCloudStatusCodeSummary) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true})

	return Tool{
		Name:        "get_arvancloud_status_code_summary",
		Description: "Get an overview pie chart of a domain's response status codes. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			summary, err := uc.Execute(ctx, app.GetArvanCloudStatusCodeSummaryInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return map[string]any{"chart": arvanCloudPieSlicesToMaps(summary.Chart)}, nil
		},
	}
}

// --- Error logs ------------------------------------------------------------

func listArvanCloudErrorLogsTool(uc *app.ListArvanCloudErrorLogs) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true})

	return Tool{
		Name:        "list_arvancloud_error_logs",
		Description: "List the errors a domain has logged, with per-error and per-upstream counts. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			logs, err := uc.Execute(ctx, app.ListArvanCloudErrorLogsInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(logs))
			for i, l := range logs {
				upstreams := make([]map[string]any, len(l.Upstreams))
				for j, u := range l.Upstreams {
					upstreams[j] = map[string]any{"address": u.Address, "count": u.Count}
				}
				out[i] = map[string]any{"name": l.Name, "count": l.Count, "upstreams": upstreams}
			}
			return map[string]any{"error_logs": out}, nil
		},
	}
}

func getArvanCloudErrorLogsChartTool(uc *app.GetArvanCloudErrorLogsChart) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true})

	return Tool{
		Name:        "get_arvancloud_error_logs_chart",
		Description: "Get a chart view of a domain's errors over time, plus each error's total count. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			chart, err := uc.Execute(ctx, app.GetArvanCloudErrorLogsChartInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return map[string]any{"statistics": chart.Statistics, "chart": arvanCloudReportChartToMap(chart.Chart)}, nil
		},
	}
}

func getArvanCloudErrorLogDetailsTool(uc *app.GetArvanCloudErrorLogDetail) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true, errorParam: true})

	return Tool{
		Name: "get_arvancloud_error_log_details",
		Description: "Get the detail of one error message for a domain. Deprecated by ArvanCloud with no documented " +
			"replacement, but still available. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "error_message"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			detail, err := uc.Execute(ctx, app.GetArvanCloudErrorLogDetailInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			// detail.Raw has no confirmed shape per the spec (see
			// domain.ArvanCloudErrorLogDetail's own doc comment) — passed
			// through as-is rather than guessed at.
			var raw2 any
			if len(detail.Raw) > 0 {
				if err := json.Unmarshal(detail.Raw, &raw2); err != nil {
					return nil, err
				}
			}
			return map[string]any{"detail": raw2}, nil
		},
	}
}

// --- DNS reports ---------------------------------------------------------

func getArvanCloudDnsRequestsReportTool(uc *app.GetArvanCloudDnsRequestsReport) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true})

	return Tool{
		Name:        "get_arvancloud_dns_requests_report",
		Description: "Get a domain's DNS request report: total/top request counts, plus a time-series chart. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			report, err := uc.Execute(ctx, app.GetArvanCloudDnsRequestsReportInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return map[string]any{"total": report.Total, "top": report.Top, "chart": arvanCloudReportChartToMap(report.Chart)}, nil
		},
	}
}

func getArvanCloudDnsGeoReportTool(uc *app.GetArvanCloudDnsGeoReport) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true})

	return Tool{
		Name:        "get_arvancloud_dns_geo_report",
		Description: "Get a domain's DNS requests broken down by country, as a geo-map plus a per-country list. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			report, err := uc.Execute(ctx, app.GetArvanCloudDnsGeoReportInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			lists := make([]map[string]any, len(report.Lists))
			for i, l := range report.Lists {
				lists[i] = map[string]any{"country": l.Country, "name": l.Name, "code": l.Code, "requests": l.Requests}
			}
			return map[string]any{"requests_map": arvanCloudGeoMapToMaps(report.RequestsChart), "countries": lists}, nil
		},
	}
}

// --- Attack reports ------------------------------------------------------

// arvanCloudAttackQueryProperties adds only "period" — the five attacks/*
// endpoints below accept no other query parameter per the spec.
func arvanCloudAttackQueryProperties(props map[string]any) {
	props["period"] = arvanCloudReportPeriodProperty()
}

func getArvanCloudAttackReportTool(uc *app.GetArvanCloudAttackReport) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudAttackQueryProperties(props)

	return Tool{
		Name:        "get_arvancloud_attack_report",
		Description: "Get a domain's attack report: total attack count, plus a time-series chart. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			report, err := uc.Execute(ctx, app.GetArvanCloudAttackReportInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"total_attacks": report.TotalAttacks, "top_attacks": report.TopAttacks,
				"chart": arvanCloudReportChartToMap(report.Chart),
			}, nil
		},
	}
}

func listArvanCloudAttacksTool(uc *app.ListArvanCloudAttacks) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudAttackQueryProperties(props)
	props["per_page"] = map[string]any{"type": "integer", "description": "How many attacks per page. Omit for ArvanCloud's own default."}
	props["page"] = map[string]any{"type": "integer", "description": "Which page to return, 1-indexed. Omit for page 1."}

	return Tool{
		Name:        "list_arvancloud_attacks",
		Description: "Get a page of detailed attack events against a domain (attacker IP/country, method, URI, timestamp, alerts). This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			result, err := uc.Execute(ctx, app.ListArvanCloudAttacksInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(result.Attacks))
			for i, a := range result.Attacks {
				out[i] = map[string]any{
					"attacker_ip": a.AttackerIP, "attacker_country": a.AttackerCountry, "method": a.Method,
					"uri": a.URI, "host": a.Host, "timestamp": a.Timestamp, "uri_args": a.URIArgs,
					"cookie": a.Cookie, "alerts": a.Alerts, "user_agent": a.UserAgent,
				}
			}
			return map[string]any{"attacks": out, "page": arvanCloudReportPageMetaToMap(result.Page)}, nil
		},
	}
}

func listArvanCloudAttackersTool(uc *app.ListArvanCloudAttackers) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudAttackQueryProperties(props)

	return Tool{
		Name:        "list_arvancloud_attackers",
		Description: "List attacker IPs against a domain, with each one's attack count. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			attackers, err := uc.Execute(ctx, app.ListArvanCloudAttackersInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(attackers))
			for i, a := range attackers {
				out[i] = map[string]any{"ip": a.IP, "count": a.Count}
			}
			return map[string]any{"attackers": out}, nil
		},
	}
}

func getArvanCloudAttackMapTool(uc *app.GetArvanCloudAttackMap) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudAttackQueryProperties(props)

	return Tool{
		Name:        "get_arvancloud_attack_map",
		Description: "Get a domain's attacks broken down by country, as a geo-map plus a per-country list. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			report, err := uc.Execute(ctx, app.GetArvanCloudAttackMapInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			lists := make([]map[string]any, len(report.Lists))
			for i, l := range report.Lists {
				lists[i] = map[string]any{"country": l.Country, "name": l.Name, "code": l.Code, "attack": l.Attack}
			}
			return map[string]any{"map": arvanCloudGeoMapToMaps(report.Chart), "countries": lists}, nil
		},
	}
}

func listArvanCloudAttackedURIsTool(uc *app.ListArvanCloudAttackedURIs) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudAttackQueryProperties(props)

	return Tool{
		Name:        "list_arvancloud_attacked_uris",
		Description: "List the URLs under attack for a domain, with each one's hit count. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			uris, err := uc.Execute(ctx, app.ListArvanCloudAttackedURIsInput{Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(uris))
			for i, u := range uris {
				out[i] = map[string]any{"uri": u.URI, "count": u.Count}
			}
			return map[string]any{"attacked_uris": out}, nil
		},
	}
}

// --- Transport layer proxy traffic ------------------------------------------

func getArvanCloudTransportLayerProxyTrafficTool(uc *app.GetArvanCloudTransportLayerProxyTraffic) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["transport_layer_proxy_id"] = map[string]any{
		"type": "string",
		"description": "The Transport Layer Proxy's ID (a UUID). ArvanCloud's API does not expose a way to list " +
			"Transport Layer Proxies, so this ID must come from wherever the proxy was originally configured " +
			"(e.g. the ArvanCloud control panel).",
	}
	arvanCloudReportQueryProperties(props, arvanCloudReportQueryOptions{sinceUntil: true})

	return Tool{
		Name:        "get_arvancloud_transport_layer_proxy_traffic",
		Description: "Get the traffic report for one Transport Layer Proxy on a domain, as a time-series chart. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "transport_layer_proxy_id"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainReportArgs
				TransportLayerProxyID string `json:"transport_layer_proxy_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			traffic, err := uc.Execute(ctx, app.GetArvanCloudTransportLayerProxyTrafficInput{
				Credentials: args.domain(), Domain: args.Domain, TransportLayerProxyID: args.TransportLayerProxyID, Query: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"chart": arvanCloudReportChartToMap(traffic.Chart)}, nil
		},
	}
}

// --- Domains report download -----------------------------------------------

func downloadArvanCloudDomainsReportTool(uc *app.DownloadArvanCloudDomainsReport) Tool {
	props := credentialProperties()

	return Tool{
		Name: "download_arvancloud_domains_report",
		Description: "Download a CSV report of every domain visible to these credentials, across the whole " +
			"account — NOT scoped to a single domain, unlike every other tool in this group. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args credentialArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			csv, err := uc.Execute(ctx, app.DownloadArvanCloudDomainsReportInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}
			return map[string]any{"csv": csv}, nil
		},
	}
}

// --- Aggregated Reports (account-wide) --------------------------------------

// arvanCloudAggregatedReportQueryArgs decodes the query fields shared by all
// three Aggregated Reports tools below.
type arvanCloudAggregatedReportQueryArgs struct {
	credentialArgs
	Domains      string `json:"domains"`
	ReportType   string `json:"report_type"`
	CategoryType string `json:"category_type"`
	Pops         string `json:"pops"`
	Asns         string `json:"asns"`
	Period       string `json:"period"`
	PerPage      int    `json:"per_page"`
	Page         int    `json:"page"`
}

func (a arvanCloudAggregatedReportQueryArgs) toDomain() domain.ArvanCloudAggregatedReportQuery {
	return domain.ArvanCloudAggregatedReportQuery{
		Domains: a.Domains, ReportType: a.ReportType, CategoryType: a.CategoryType,
		Pops: a.Pops, Asns: a.Asns, Period: a.Period, PerPage: a.PerPage, Page: a.Page,
	}
}

func arvanCloudAggregatedDomainsProperty() map[string]any {
	return map[string]any{
		"type": "string",
		"description": "Comma-separated domain names to include, e.g. \"example1.com,example2.com\". " +
			"Omit to include every domain visible to these credentials.",
	}
}

func listArvanCloudAggregatedReportDetailsTool(uc *app.ListArvanCloudAggregatedReportDetails) Tool {
	props := credentialProperties()
	props["domains"] = arvanCloudAggregatedDomainsProperty()
	props["category_type"] = map[string]any{"type": "string", "enum": []string{"pop", "asn"}, "description": "Which dimension to bucket the report by."}
	props["pops"] = map[string]any{"type": "string", "description": "Comma-separated PoP site names to filter by, e.g. \"thr-r1c,thr-mci\"."}
	props["asns"] = map[string]any{"type": "string", "description": "Comma-separated ASN numbers to filter by, e.g. \"1435,7846\"."}
	props["period"] = arvanCloudReportPeriodProperty()
	props["per_page"] = map[string]any{"type": "integer", "description": "How many rows per page. Omit for ArvanCloud's own default."}
	props["page"] = map[string]any{"type": "integer", "description": "Which page to return, 1-indexed. Omit for page 1."}

	return Tool{
		Name:        "list_arvancloud_aggregated_report_details",
		Description: "Get a page of account-wide aggregated traffic/request reports, one row per domain (or per PoP/ASN bucket). This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudAggregatedReportQueryArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			result, err := uc.Execute(ctx, app.ListArvanCloudAggregatedReportDetailsInput{Credentials: args.domain(), Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(result.Details))
			for i, d := range result.Details {
				out[i] = map[string]any{
					"domain": d.Domain, "total_downstream": d.TotalDownstream, "total_upstream": d.TotalUpstream,
					"total_requests": d.TotalRequests, "asn": d.ASN, "pop": d.POP,
				}
			}
			return map[string]any{"details": out, "page": arvanCloudReportPageMetaToMap(result.Page)}, nil
		},
	}
}

func getArvanCloudAggregatedReportChartsTool(uc *app.GetArvanCloudAggregatedReportCharts) Tool {
	props := credentialProperties()
	props["domains"] = arvanCloudAggregatedDomainsProperty()
	props["report_type"] = map[string]any{"type": "string", "enum": []string{"traffic", "requests"}, "description": "Which metric to chart."}
	props["category_type"] = map[string]any{"type": "string", "enum": []string{"pop", "asn"}, "description": "Which dimension to bucket the chart by."}
	props["pops"] = map[string]any{"type": "string", "description": "Comma-separated PoP site names to filter by, e.g. \"thr-r1c,thr-mci\"."}
	props["asns"] = map[string]any{"type": "string", "description": "Comma-separated ASN numbers to filter by, e.g. \"1435,7846\"."}
	props["period"] = arvanCloudReportPeriodProperty()

	return Tool{
		Name:        "get_arvancloud_aggregated_report_charts",
		Description: "Get an account-wide aggregated traffic/request chart across domains, as a time series. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudAggregatedReportQueryArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			chart, err := uc.Execute(ctx, app.GetArvanCloudAggregatedReportChartsInput{Credentials: args.domain(), Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return arvanCloudReportChartToMap(*chart), nil
		},
	}
}

func getArvanCloudAggregatedReportFiltersTool(uc *app.GetArvanCloudAggregatedReportFilters) Tool {
	props := credentialProperties()
	props["domains"] = arvanCloudAggregatedDomainsProperty()

	return Tool{
		Name:        "get_arvancloud_aggregated_report_filters",
		Description: "Get the PoP and ASN filter dimensions available for the aggregated report tools, across the given domains. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudAggregatedReportQueryArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			filters, err := uc.Execute(ctx, app.GetArvanCloudAggregatedReportFiltersInput{Credentials: args.domain(), Query: args.toDomain()})
			if err != nil {
				return nil, err
			}
			pops := make([]map[string]any, len(filters.POPs))
			for i, p := range filters.POPs {
				pops[i] = map[string]any{"pop": p.POP, "label": p.Label}
			}
			return map[string]any{"pops": pops, "asns": filters.Asns}, nil
		},
	}
}
