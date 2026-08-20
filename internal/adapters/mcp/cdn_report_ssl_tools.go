package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// This file covers issue #24's CDN report/analytics (read-only) and
// CDN-zone-level SSL settings tools: access/security/error/WAF logs, top
// visitors, monthly traffic usage, minimum TLS version, attached
// certificates (read-only listing), and HSTS. Every tool here is a fast
// operation, stated in its Description per the house style (cdn_tools.go).
//
// These CDN-zone-level SSL SETTINGS tools are unrelated to the SSL
// certificate ORDERING tools (list_ssl_products, create_ssl_order, ...,
// ssl_tools.go, issue #18): those talk to the separate sslv2 API surface.
// list_cdn_certificates only reads certificates already attached to a zone
// on the CDN surface — it does not order or issue anything.

// cdnLogPageMetaProperty documents the pagination metadata every log tool
// returns.
func cdnLogPageMetaToMap(meta domain.CDNLogPageMeta) map[string]any {
	return map[string]any{
		"current_page": meta.CurrentPage,
		"last_page":    meta.LastPage,
		"per_page":     meta.PerPage,
		"total":        meta.Total,
	}
}

// cdnLogQueryProperties are the filter parameters shared by every CDN log
// tool (access, security, error, WAF).
func cdnLogQueryProperties(props map[string]any) {
	props["page"] = map[string]any{
		"type":        "integer",
		"minimum":     1,
		"description": "Page number, starting at 1. Omit for the first page.",
	}
	props["step"] = map[string]any{
		"type":        "integer",
		"enum":        []int{10, 25, 50, 100},
		"description": "Number of log entries per page. Omit to use the provider default.",
	}
	props["from"] = map[string]any{
		"type":        "string",
		"description": "Start date of the query range, format YYYY-MM-DD, e.g. \"2024-05-20\". Omit for no lower bound.",
	}
	props["to"] = map[string]any{
		"type":        "string",
		"description": "End date of the query range, format YYYY-MM-DD, e.g. \"2024-05-23\". Omit for no upper bound.",
	}
	props["uri"] = map[string]any{
		"type":        "string",
		"description": "Filter to entries whose request URI matches this value, e.g. \"/robots.txt\".",
	}
	props["status_code"] = map[string]any{
		"type":        "integer",
		"minimum":     1,
		"maximum":     65535,
		"description": "Filter to entries with this HTTP status code, e.g. 404.",
	}
	props["user_agent"] = map[string]any{
		"type":        "string",
		"description": "Filter to entries whose User-Agent header matches this value.",
	}
	props["method"] = map[string]any{
		"type":        "string",
		"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "CONNECT", "TRACE"},
		"description": "Filter to entries with this HTTP method. Case-sensitive on the provider side.",
	}
	props["ray_id"] = map[string]any{
		"type":        "string",
		"description": "Filter to the entry with this specific ray ID (the provider's per-request trace identifier).",
	}
	props["target_domain"] = map[string]any{
		"type":        "string",
		"description": "Filter to entries for this target domain, e.g. \"example.com\".",
	}
}

// cdnLogQueryArgs is embedded by every CDN log tool's args struct.
type cdnLogQueryArgs struct {
	credentialArgs
	ZoneUUID     string `json:"zone_uuid"`
	Page         int    `json:"page"`
	Step         int    `json:"step"`
	From         string `json:"from"`
	To           string `json:"to"`
	URI          string `json:"uri"`
	StatusCode   int    `json:"status_code"`
	UserAgent    string `json:"user_agent"`
	Method       string `json:"method"`
	RayID        string `json:"ray_id"`
	TargetDomain string `json:"target_domain"`
}

func (a cdnLogQueryArgs) toQuery() domain.CDNLogQuery {
	return domain.CDNLogQuery{
		Page: a.Page, Step: a.Step, From: a.From, To: a.To, URI: a.URI,
		StatusCode: a.StatusCode, UserAgent: a.UserAgent, Method: a.Method,
		RayID: a.RayID, TargetDomain: a.TargetDomain,
	}
}

type getCDNAccessLogArgs struct {
	cdnLogQueryArgs
	WCDNState string `json:"wcdn_state"`
}

func getCDNAccessLogTool(uc *app.GetCDNAccessLog) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	cdnLogQueryProperties(props)
	props["wcdn_state"] = map[string]any{
		"type":        "string",
		"description": "Filter to entries with this CDN cache state, e.g. \"hit\" or \"miss\". Access log only.",
	}

	return Tool{
		Name: "get_cdn_access_log",
		Description: "Get one page of a Parspack CDN zone's access log (every request served through the CDN). " +
			"This is a fast, read-only operation: the page is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args getCDNAccessLogArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			query := args.toQuery()
			query.WCDNState = args.WCDNState

			page, err := uc.Execute(ctx, app.GetCDNAccessLogInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Query: query,
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(page.Records))
			for i, r := range page.Records {
				out[i] = map[string]any{
					"id": r.ID, "date": r.Date, "timestamp": r.Timestamp, "host": r.Host,
					"user_host": r.UserHost, "target_domain": r.TargetDomain, "uri": r.URI,
					"method": r.Method, "scheme": r.Scheme, "status_code": r.StatusCode, "level": r.Level,
					"ray_id": r.RayID, "edge_id": r.EdgeID, "source": r.Source, "wcdn_state": r.WCDNState,
					"log_type": r.LogType, "byte_in": r.ByteIn, "byte_out": r.ByteOut, "remote_ip": r.RemoteIP,
					"zid": r.ZID, "connection_duration": r.ConnectionDuration, "delivery_duration": r.DeliveryDuration,
					"hosting_wait_duration": r.HostingWaitDuration, "total_duration": r.TotalDuration,
					"user_agent": r.UserAgent, "facility": r.Facility,
				}
			}
			return map[string]any{"records": out, "meta": cdnLogPageMetaToMap(page.Meta)}, nil
		},
	}
}

func getCDNSecurityLogTool(uc *app.GetCDNSecurityLog) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	cdnLogQueryProperties(props)

	return Tool{
		Name: "get_cdn_security_log",
		Description: "Get one page of a Parspack CDN zone's security log (requests blocked or flagged by the " +
			"provider's security layer). This is a fast, read-only operation: the page is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cdnLogQueryArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			page, err := uc.Execute(ctx, app.GetCDNSecurityLogInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Query: args.toQuery(),
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(page.Records))
			for i, r := range page.Records {
				out[i] = map[string]any{
					"id": r.ID, "date": r.Date, "timestamp": r.Timestamp, "host": r.Host,
					"user_host": r.UserHost, "target_domain": r.TargetDomain, "uri": r.URI,
					"method": r.Method, "scheme": r.Scheme, "status_code": r.StatusCode, "level": r.Level,
					"ray_id": r.RayID, "edge_id": r.EdgeID, "source": r.Source, "log_type": r.LogType,
					"security_type": r.SecurityType, "security_message": r.SecurityMessage,
					"request_time": r.RequestTime, "remote_ip": r.RemoteIP, "zid": r.ZID,
					"additional_logs": r.AdditionalLogs, "user_agent": r.UserAgent, "facility": r.Facility,
				}
			}
			return map[string]any{"records": out, "meta": cdnLogPageMetaToMap(page.Meta)}, nil
		},
	}
}

func getCDNErrorLogTool(uc *app.GetCDNErrorLog) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	cdnLogQueryProperties(props)

	return Tool{
		Name: "get_cdn_error_log",
		Description: "Get one page of a Parspack CDN zone's error log (requests that failed at the origin or edge). " +
			"This is a fast, read-only operation: the page is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cdnLogQueryArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			page, err := uc.Execute(ctx, app.GetCDNErrorLogInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Query: args.toQuery(),
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(page.Records))
			for i, r := range page.Records {
				out[i] = map[string]any{
					"id": r.ID, "date": r.Date, "timestamp": r.Timestamp, "host": r.Host,
					"user_host": r.UserHost, "uri": r.URI, "method": r.Method, "scheme": r.Scheme,
					"status_code": r.StatusCode, "level": r.Level, "ray_id": r.RayID, "edge_id": r.EdgeID,
					"source": r.Source, "log_type": r.LogType, "error_type": r.ErrorType,
					"request_time": r.RequestTime, "remote_ip": r.RemoteIP, "zid": r.ZID,
					"user_agent": r.UserAgent, "facility": r.Facility,
				}
			}
			return map[string]any{"records": out, "meta": cdnLogPageMetaToMap(page.Meta)}, nil
		},
	}
}

func getCDNWAFLogTool(uc *app.GetCDNWAFLog) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	cdnLogQueryProperties(props)

	return Tool{
		Name: "get_cdn_waf_log",
		Description: "Get one page of a Parspack CDN zone's WAF (Web Application Firewall) log, including which WAF " +
			"rule matched each blocked request. This is a fast, read-only operation: the page is returned within " +
			"this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cdnLogQueryArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			page, err := uc.Execute(ctx, app.GetCDNWAFLogInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Query: args.toQuery(),
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(page.Records))
			for i, r := range page.Records {
				details := make([]map[string]any, len(r.AdditionalLogs))
				for j, d := range r.AdditionalLogs {
					details[j] = map[string]any{"message": d.Message, "match": d.Match, "data": d.Data}
				}
				out[i] = map[string]any{
					"id": r.ID, "date": r.Date, "timestamp": r.Timestamp, "host": r.Host,
					"user_host": r.UserHost, "target_domain": r.TargetDomain, "uri": r.URI,
					"method": r.Method, "scheme": r.Scheme, "status_code": r.StatusCode, "level": r.Level,
					"ray_id": r.RayID, "edge_id": r.EdgeID, "source": r.Source, "log_type": r.LogType,
					"security_type": r.SecurityType, "security_message": r.SecurityMessage,
					"request_time": r.RequestTime, "remote_ip": r.RemoteIP, "zid": r.ZID,
					"additional_logs": details, "user_agent": r.UserAgent, "facility": r.Facility,
				}
			}
			return map[string]any{"records": out, "meta": cdnLogPageMetaToMap(page.Meta)}, nil
		},
	}
}

type cdnTopVisitorsArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

func getCDNTopVisitorsTool(uc *app.GetCDNTopVisitors) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["start"] = map[string]any{
		"type":        "string",
		"description": "Start date of the query range, format YYYY-MM-DD, e.g. \"2024-05-20\". Required.",
	}
	props["end"] = map[string]any{
		"type":        "string",
		"description": "End date of the query range, format YYYY-MM-DD, e.g. \"2024-05-25\". Required.",
	}

	return Tool{
		Name: "get_cdn_top_visitors",
		Description: "Get the top visitor IPs (by request count) for a Parspack CDN zone within a date range. This " +
			"is a fast, read-only operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "start", "end"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cdnTopVisitorsArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			visitors, err := uc.Execute(ctx, app.GetCDNTopVisitorsInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Start: args.Start, End: args.End,
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(visitors))
			for i, v := range visitors {
				out[i] = map[string]any{"ip": v.IP, "count": v.Count}
			}
			return map[string]any{"visitors": out}, nil
		},
	}
}

func getCDNMonthlyTrafficUsageTool(uc *app.GetCDNMonthlyTrafficUsage) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_monthly_traffic_usage",
		Description: "Get a Parspack CDN zone's traffic usage for the current billing month, plus the plan's traffic " +
			"limit. This is a fast, read-only operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args zoneUUIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			usage, err := uc.Execute(ctx, app.GetCDNMonthlyTrafficUsageInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"received_bytes": usage.ReceivedBytes,
				"traffic_limit":  usage.TrafficLimit,
			}, nil
		},
	}
}

func getCDNMinTLSVersionTool(uc *app.GetCDNMinTLSVersion) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_min_tls_version",
		Description: "Get the minimum TLS version a Parspack CDN zone currently accepts for HTTPS connections. This " +
			"is a fast operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args zoneUUIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			version, err := uc.Execute(ctx, app.GetCDNMinTLSVersionInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "min_tls_version": string(version)}, nil
		},
	}
}

func minTLSVersionProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{"1.0", "1.1", "1.2", "1.3"},
		"description": "Minimum TLS version the zone should accept, e.g. \"1.2\". Connections below this version are rejected.",
	}
}

type updateCDNMinTLSVersionArgs struct {
	credentialArgs
	ZoneUUID      string `json:"zone_uuid"`
	MinTLSVersion string `json:"min_tls_version"`
}

func updateCDNMinTLSVersionTool(uc *app.UpdateCDNMinTLSVersion) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["min_tls_version"] = minTLSVersionProperty()

	return Tool{
		Name: "update_cdn_min_tls_version",
		Description: "Set the minimum TLS version a Parspack CDN zone accepts for HTTPS connections. This is a fast " +
			"operation: the applied value is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "min_tls_version"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNMinTLSVersionArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			version, err := uc.Execute(ctx, app.UpdateCDNMinTLSVersionInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID,
				MinTLSVersion: domain.CDNMinTLSVersion(args.MinTLSVersion),
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "min_tls_version": string(version)}, nil
		},
	}
}

type listCDNCertificatesArgs struct {
	credentialArgs
	ZoneUUID     string `json:"zone_uuid"`
	PerPage      int    `json:"per_page"`
	Page         int    `json:"page"`
	DomainFilter string `json:"domain_filter"`
}

func cdnCertificateDetailToMap(d *domain.CDNCertificateDetail) map[string]any {
	if d == nil {
		return nil
	}
	return map[string]any{
		"expiration_time": d.ExpirationTime,
		"active":          d.Active,
		"certificate":     d.Certificate,
		"private_key":     d.PrivateKey,
		"ca_bundle":       d.CABundle,
	}
}

func listCDNCertificatesTool(uc *app.ListCDNCertificates) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["per_page"] = map[string]any{
		"type":        "integer",
		"minimum":     1,
		"maximum":     100,
		"description": "Number of certificates per page. Omit to use the provider default.",
	}
	props["page"] = map[string]any{
		"type":        "integer",
		"minimum":     1,
		"description": "Page number, starting at 1. Omit for the first page.",
	}
	props["domain_filter"] = map[string]any{
		"type": "string",
		"description": "Optional domain substring filter, e.g. \"api\". Matches certificates whose domain contains " +
			"this value.",
	}

	return Tool{
		Name: "list_cdn_certificates",
		Description: "List the SSL/TLS certificates currently attached to a Parspack CDN zone. This is a fast, " +
			"read-only operation: the list is returned within this call. To order or issue a NEW certificate, use " +
			"list_ssl_products and create_ssl_order instead — this tool only reports what is already attached.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args listCDNCertificatesArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			certs, err := uc.Execute(ctx, app.ListCDNCertificatesInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID,
				PerPage: args.PerPage, Page: args.Page, DomainFilter: args.DomainFilter,
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(certs))
			for i, c := range certs {
				out[i] = map[string]any{
					"domain":      c.Domain,
					"status":      c.Status,
					"ssl_type":    c.SSLType,
					"letsencrypt": cdnCertificateDetailToMap(c.LetsEncrypt),
					"custom":      cdnCertificateDetailToMap(c.Custom),
				}
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "certificates": out}, nil
		},
	}
}

func cdnHSTSToMap(zoneUUID string, s domain.CDNHSTSSettings) map[string]any {
	return map[string]any{
		"zone_uuid": zoneUUID,
		"enabled":   s.Enabled,
		"max_age":   s.MaxAgeSeconds,
	}
}

func getCDNHSTSTool(uc *app.GetCDNHSTS) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_hsts",
		Description: "Get a Parspack CDN zone's current HTTP Strict Transport Security (HSTS) configuration. This " +
			"is a fast operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args zoneUUIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			settings, err := uc.Execute(ctx, app.GetCDNHSTSInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return cdnHSTSToMap(args.ZoneUUID, *settings), nil
		},
	}
}

type updateCDNHSTSArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Enabled  bool   `json:"enabled"`
	MaxAge   int    `json:"max_age"`
}

func updateCDNHSTSTool(uc *app.UpdateCDNHSTS) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["enabled"] = map[string]any{
		"type":        "boolean",
		"description": "Whether HSTS should be enabled for this zone.",
	}
	props["max_age"] = map[string]any{
		"type":        "integer",
		"minimum":     0,
		"maximum":     31536000,
		"description": "HSTS max-age in seconds, e.g. 3600 for one hour. Maximum is 31536000 (one year).",
	}

	return Tool{
		Name: "update_cdn_hsts",
		Description: "Set a Parspack CDN zone's HTTP Strict Transport Security (HSTS) configuration. This is a fast " +
			"operation: the applied settings are returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled", "max_age"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNHSTSArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			settings, err := uc.Execute(ctx, app.UpdateCDNHSTSInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID,
				Settings: domain.CDNHSTSSettings{Enabled: args.Enabled, MaxAgeSeconds: args.MaxAge},
			})
			if err != nil {
				return nil, err
			}
			return cdnHSTSToMap(args.ZoneUUID, *settings), nil
		},
	}
}
