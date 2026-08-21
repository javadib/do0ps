package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud Active Health Check tools (issue #70): a domain-scoped monitor
// that periodically probes an origin (in practice, a Load Balancing pool —
// the arvancloud_loadbalancer_tools.go tools, issue #69) over TCP or
// HTTP(S) and reports whether it is reachable. All fast operations
// (AGENTS.md 4.3): every tool below returns its result within the call, with
// no operation_id to poll afterward.
const arvanCloudHealthCheckVsLBNote = "This monitors an origin (in practice, a Load Balancing pool — " +
	"create_arvancloud_load_balancer and friends) and is a separate resource from the load balancer/pool/origin " +
	"themselves; a pool's own health_check field is only a read-only reference to a check created here."

// arvanCloudHealthCheckExpectedHeaderToMap renders one expected_headers
// entry.
func arvanCloudHealthCheckExpectedHeaderToMap(h domain.ArvanCloudHealthCheckExpectedHeader) map[string]any {
	return map[string]any{"key": h.Key, "value": h.Value}
}

// arvanCloudHealthCheckExpectedResponseToMap renders an
// domain.ArvanCloudHealthCheckExpectedResponse.
func arvanCloudHealthCheckExpectedResponseToMap(r domain.ArvanCloudHealthCheckExpectedResponse) map[string]any {
	headers := make([]map[string]any, len(r.ExpectedHeaders))
	for i, h := range r.ExpectedHeaders {
		headers[i] = arvanCloudHealthCheckExpectedHeaderToMap(h)
	}
	return map[string]any{
		"codes":            r.Codes,
		"headers":          r.Headers,
		"expected_headers": headers,
		"body":             r.Body,
	}
}

// arvanCloudHealthCheckSentHeaderToMap renders one sent_headers entry.
func arvanCloudHealthCheckSentHeaderToMap(h domain.ArvanCloudHealthCheckSentHeader) map[string]any {
	return map[string]any{"key": h.Key, "value": h.Value}
}

// arvanCloudHealthCheckRequestConfigToMap renders whichever branch of a
// domain.ArvanCloudHealthCheckRequestConfig is set. At most one of
// "tcp_config"/"http_config" is present in the result, matching the two
// input properties create/update_arvancloud_health_check accept.
func arvanCloudHealthCheckRequestConfigToMap(cfg domain.ArvanCloudHealthCheckRequestConfig) map[string]any {
	out := map[string]any{}
	if cfg.TCP != nil {
		out["tcp_config"] = map[string]any{
			"port":       cfg.TCP.Port,
			"timeout_ms": cfg.TCP.TimeoutMS,
		}
	}
	if cfg.HTTP != nil {
		h := cfg.HTTP
		sentHeaders := make([]map[string]any, len(h.SentHeaders))
		for i, sh := range h.SentHeaders {
			sentHeaders[i] = arvanCloudHealthCheckSentHeaderToMap(sh)
		}
		out["http_config"] = map[string]any{
			"method":            string(h.Method),
			"port":              h.Port,
			"path":              h.Path,
			"allow_insecure":    h.AllowInsecure,
			"expected_response": arvanCloudHealthCheckExpectedResponseToMap(h.ExpectedResponse),
			"headers":           h.Headers,
			"sent_headers":      sentHeaders,
			"timeout_ms":        h.TimeoutMS,
		}
	}
	return out
}

// arvanCloudHealthCheckZoneToMap renders one domain.ArvanCloudHealthCheckZone
// (a check's own Zones entry).
func arvanCloudHealthCheckZoneToMap(z domain.ArvanCloudHealthCheckZone) map[string]any {
	return map[string]any{"id": z.ID, "name": z.Name, "monitoring_level": z.MonitoringLevel}
}

// arvanCloudHealthCheckZoneNameToMap renders one
// domain.ArvanCloudHealthCheckZoneName (a zone-LISTING endpoint's entry).
func arvanCloudHealthCheckZoneNameToMap(z domain.ArvanCloudHealthCheckZoneName) map[string]any {
	return map[string]any{"id": z.ID, "name": z.Name}
}

// arvanCloudHealthCheckToMap renders a domain.ArvanCloudHealthCheck the way
// every check-returning tool reports it back to the caller.
func arvanCloudHealthCheckToMap(hc domain.ArvanCloudHealthCheck) map[string]any {
	zones := make([]map[string]any, len(hc.Zones))
	for i, z := range hc.Zones {
		zones[i] = arvanCloudHealthCheckZoneToMap(z)
	}
	out := map[string]any{
		"id":                    hc.ID,
		"name":                  hc.Name,
		"description":           hc.Description,
		"origin":                hc.Origin,
		"origin_type":           string(hc.OriginType),
		"upstreams":             hc.Upstreams,
		"interval_ms":           hc.IntervalMS,
		"threshold":             hc.Threshold,
		"type":                  string(hc.Type),
		"status":                hc.Status,
		"retries":               hc.Retries,
		"zones":                 zones,
		"monitoring_updated_at": hc.MonitoringUpdatedAt,
	}
	for k, v := range arvanCloudHealthCheckRequestConfigToMap(hc.RequestConfig) {
		out[k] = v
	}
	return out
}

// --- Shared argument/property helpers ---------------------------------

// arvanCloudHealthCheckZoneArgs decodes one zones[] entry.
type arvanCloudHealthCheckZoneArgs struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	MonitoringLevel string `json:"monitoring_level"`
}

func (a arvanCloudHealthCheckZoneArgs) toDomain() domain.ArvanCloudHealthCheckZone {
	return domain.ArvanCloudHealthCheckZone{ID: a.ID, Name: a.Name, MonitoringLevel: a.MonitoringLevel}
}

// arvanCloudHealthCheckExpectedHeaderArgs decodes one
// expected_response.expected_headers[] entry.
type arvanCloudHealthCheckExpectedHeaderArgs struct {
	Key   string   `json:"key"`
	Value []string `json:"value"`
}

// arvanCloudHealthCheckExpectedResponseArgs decodes http_config.expected_response.
type arvanCloudHealthCheckExpectedResponseArgs struct {
	Codes           []int                                     `json:"codes"`
	Headers         map[string][]string                       `json:"headers"`
	ExpectedHeaders []arvanCloudHealthCheckExpectedHeaderArgs `json:"expected_headers"`
	Body            string                                    `json:"body"`
}

func (a arvanCloudHealthCheckExpectedResponseArgs) toDomain() domain.ArvanCloudHealthCheckExpectedResponse {
	headers := make([]domain.ArvanCloudHealthCheckExpectedHeader, len(a.ExpectedHeaders))
	for i, h := range a.ExpectedHeaders {
		headers[i] = domain.ArvanCloudHealthCheckExpectedHeader{Key: h.Key, Value: h.Value}
	}
	return domain.ArvanCloudHealthCheckExpectedResponse{
		Codes: a.Codes, Headers: a.Headers, ExpectedHeaders: headers, Body: a.Body,
	}
}

// arvanCloudHealthCheckSentHeaderArgs decodes one http_config.sent_headers[]
// entry.
type arvanCloudHealthCheckSentHeaderArgs struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// arvanCloudHealthCheckTCPConfigArgs decodes tcp_config.
type arvanCloudHealthCheckTCPConfigArgs struct {
	Port      int `json:"port"`
	TimeoutMS int `json:"timeout_ms"`
}

func (a *arvanCloudHealthCheckTCPConfigArgs) toDomain() *domain.ArvanCloudHealthCheckTCPConfig {
	if a == nil {
		return nil
	}
	return &domain.ArvanCloudHealthCheckTCPConfig{Port: a.Port, TimeoutMS: a.TimeoutMS}
}

// arvanCloudHealthCheckHTTPConfigArgs decodes http_config.
type arvanCloudHealthCheckHTTPConfigArgs struct {
	Method           string                                    `json:"method"`
	Port             int                                       `json:"port"`
	Path             string                                    `json:"path"`
	AllowInsecure    bool                                      `json:"allow_insecure"`
	ExpectedResponse arvanCloudHealthCheckExpectedResponseArgs `json:"expected_response"`
	Headers          map[string]string                         `json:"headers"`
	SentHeaders      []arvanCloudHealthCheckSentHeaderArgs     `json:"sent_headers"`
	TimeoutMS        int                                       `json:"timeout_ms"`
}

func (a *arvanCloudHealthCheckHTTPConfigArgs) toDomain() *domain.ArvanCloudHealthCheckHTTPConfig {
	if a == nil {
		return nil
	}
	sentHeaders := make([]domain.ArvanCloudHealthCheckSentHeader, len(a.SentHeaders))
	for i, h := range a.SentHeaders {
		sentHeaders[i] = domain.ArvanCloudHealthCheckSentHeader{Key: h.Key, Value: h.Value}
	}
	return &domain.ArvanCloudHealthCheckHTTPConfig{
		Method:           domain.ArvanCloudHealthCheckHTTPMethod(a.Method),
		Port:             a.Port,
		Path:             a.Path,
		AllowInsecure:    a.AllowInsecure,
		ExpectedResponse: a.ExpectedResponse.toDomain(),
		Headers:          a.Headers,
		SentHeaders:      sentHeaders,
		TimeoutMS:        a.TimeoutMS,
	}
}

func arvanCloudHealthCheckIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The health check's provider-assigned ID (a UUID), as returned by create_arvancloud_health_check or list_arvancloud_health_checks.",
	}
}

// arvanCloudHealthCheckExpectedResponseProperty describes http_config.expected_response.
func arvanCloudHealthCheckExpectedResponseProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "What counts as a passing HTTP/HTTPS probe.",
		"properties": map[string]any{
			"codes": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "integer"},
				"description": "HTTP status codes considered healthy, e.g. [200, 204]. Leave empty for ArvanCloud's own default.",
			},
			"headers": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"description":          "Response headers required, as header name -> list of accepted values.",
			},
			"expected_headers": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":   map[string]any{"type": "string"},
						"value": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
				},
				"description": "Response headers required, as an explicit list of key/accepted-values pairs (an alternative shape to \"headers\" above).",
			},
			"body": map[string]any{"type": "string", "description": "A substring the response body must contain. Leave empty for no body check."},
		},
	}
}

// arvanCloudHealthCheckHTTPConfigProperty describes http_config, the probe
// configuration used when type is "HTTP" or "HTTPS".
func arvanCloudHealthCheckHTTPConfigProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "REQUIRED when type is \"HTTP\" or \"HTTPS\"; ignored when type is \"TCP\". The HTTP(S) probe configuration.",
		"properties": map[string]any{
			"method":            map[string]any{"type": "string", "enum": []string{"HEAD", "GET", "POST", "PUT"}, "description": "REQUIRED. The HTTP method to probe with."},
			"port":              map[string]any{"type": "integer", "description": "REQUIRED. The TCP port to probe, e.g. 443. Range: 1-65535."},
			"path":              map[string]any{"type": "string", "description": "REQUIRED. The request path to probe, e.g. \"/healthz\"."},
			"allow_insecure":    map[string]any{"type": "boolean", "description": "Skip TLS certificate verification (HTTPS only)."},
			"expected_response": arvanCloudHealthCheckExpectedResponseProperty(),
			"headers": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
				"description":          "Request headers sent with the probe, as header name -> single value.",
			},
			"sent_headers": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":   map[string]any{"type": "string"},
						"value": map[string]any{"type": "string"},
					},
				},
				"description": "Request headers sent with the probe, as an explicit list of key/value pairs (an alternative shape to \"headers\" above).",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"description": "REQUIRED. Probe timeout, in MILLISECONDS, e.g. 5000 for 5 seconds. Range: 1-30000.",
			},
		},
	}
}

// arvanCloudHealthCheckTCPConfigProperty describes tcp_config, the probe
// configuration used when type is "TCP".
func arvanCloudHealthCheckTCPConfigProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "REQUIRED when type is \"TCP\"; ignored when type is \"HTTP\" or \"HTTPS\". The TCP probe configuration.",
		"properties": map[string]any{
			"port":       map[string]any{"type": "integer", "description": "REQUIRED. The TCP port to probe, e.g. 5432. Range: 1-65535."},
			"timeout_ms": map[string]any{"type": "integer", "description": "REQUIRED. Probe timeout, in MILLISECONDS, e.g. 3000 for 3 seconds. Range: 1-10000."},
		},
	}
}

// arvanCloudHealthCheckProperties adds the field set shared by
// create_arvancloud_health_check and update_arvancloud_health_check to
// props.
func arvanCloudHealthCheckProperties(props map[string]any) {
	props["name"] = map[string]any{"type": "string", "description": "REQUIRED. A caller-supplied label for the check."}
	props["description"] = map[string]any{"type": "string", "description": "An optional note about what this check monitors."}
	props["origin"] = map[string]any{
		"type": "string",
		"description": "REQUIRED. What this check monitors — in practice, a Load Balancing pool's ID " +
			"(get/list_arvancloud_lb_pool return it). " + arvanCloudHealthCheckVsLBNote,
	}
	props["origin_type"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudHealthCheckOriginTypePool)},
		"description": "What kind of resource \"origin\" addresses. Defaults to \"pool\" (currently the only value ArvanCloud accepts) when omitted.",
	}
	props["upstreams"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "The specific upstream addresses within \"origin\" to probe, e.g. [\"1.1.1.1\"].",
	}
	props["interval_ms"] = map[string]any{
		"type":        "integer",
		"enum":        []int{30000, 60000, 120000},
		"description": "REQUIRED. How often to probe, in MILLISECONDS — one of 30000 (30s), 60000 (60s) or 120000 (120s). NOT seconds.",
	}
	props["threshold"] = map[string]any{
		"type":        "integer",
		"description": "REQUIRED. How many consecutive failed probes before an upstream is marked unhealthy. Must be at least 1.",
	}
	props["type"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudHealthCheckTCP), string(domain.ArvanCloudHealthCheckHTTP), string(domain.ArvanCloudHealthCheckHTTPS)},
		"description": "REQUIRED. The probe protocol. Selects which of tcp_config/http_config is required.",
	}
	props["status"] = map[string]any{"type": "boolean", "description": "Whether the check is active. Defaults to true when omitted."}
	props["retries"] = map[string]any{
		"type":        "integer",
		"description": "Immediate retries a timed-out probe gets before counting as failed, 0-10. Optional — omit to let ArvanCloud apply its own default.",
	}
	props["zones"] = map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":               map[string]any{"type": "string", "description": "A zone ID from list_arvancloud_domain_health_check_zones/list_arvancloud_health_check_zones. If omitted, \"name\" is used as the ID."},
				"name":             map[string]any{"type": "string", "description": "The zone's name, from the same list tools."},
				"monitoring_level": map[string]any{"type": "string", "enum": []string{"critical", "non-critical", "quiet-critical", "off"}},
			},
		},
		"description": "Which check-execution zones to probe from, and how strictly each one's result counts. Leave empty for ArvanCloud's own default zone set.",
	}
	props["tcp_config"] = arvanCloudHealthCheckTCPConfigProperty()
	props["http_config"] = arvanCloudHealthCheckHTTPConfigProperty()
}

// arvanCloudHealthCheckArgs is embedded by create/update_arvancloud_health_check.
type arvanCloudHealthCheckArgs struct {
	arvanCloudDomainNameArgs
	Name        string                               `json:"name"`
	Description string                               `json:"description"`
	Origin      string                               `json:"origin"`
	OriginType  string                               `json:"origin_type"`
	Upstreams   []string                             `json:"upstreams"`
	IntervalMS  int                                  `json:"interval_ms"`
	Threshold   int                                  `json:"threshold"`
	Type        string                               `json:"type"`
	Status      *bool                                `json:"status"`
	Retries     int                                  `json:"retries"`
	Zones       []arvanCloudHealthCheckZoneArgs      `json:"zones"`
	TCPConfig   *arvanCloudHealthCheckTCPConfigArgs  `json:"tcp_config"`
	HTTPConfig  *arvanCloudHealthCheckHTTPConfigArgs `json:"http_config"`
}

func (a arvanCloudHealthCheckArgs) toDomain() domain.ArvanCloudHealthCheck {
	status := true
	if a.Status != nil {
		status = *a.Status
	}
	originType := domain.ArvanCloudHealthCheckOriginType(a.OriginType)
	if originType == "" {
		originType = domain.ArvanCloudHealthCheckOriginTypePool
	}
	zones := make([]domain.ArvanCloudHealthCheckZone, len(a.Zones))
	for i, z := range a.Zones {
		zones[i] = z.toDomain()
	}
	return domain.ArvanCloudHealthCheck{
		Name:        a.Name,
		Description: a.Description,
		Origin:      a.Origin,
		OriginType:  originType,
		Upstreams:   a.Upstreams,
		IntervalMS:  a.IntervalMS,
		Threshold:   a.Threshold,
		Type:        domain.ArvanCloudHealthCheckType(a.Type),
		Status:      status,
		Retries:     a.Retries,
		Zones:       zones,
		RequestConfig: domain.ArvanCloudHealthCheckRequestConfig{
			TCP:  a.TCPConfig.toDomain(),
			HTTP: a.HTTPConfig.toDomain(),
		},
	}
}

// --- Health checks ------------------------------------------------------

func listArvanCloudHealthChecksTool(uc *app.ListArvanCloudHealthChecks) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name:        "list_arvancloud_health_checks",
		Description: "List every active health check configured for a domain. " + arvanCloudHealthCheckVsLBNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			checks, err := uc.Execute(ctx, app.ListArvanCloudHealthChecksInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(checks))
			for i, c := range checks {
				out[i] = arvanCloudHealthCheckToMap(c)
			}
			return map[string]any{"health_checks": out}, nil
		},
	}
}

func createArvanCloudHealthCheckTool(uc *app.CreateArvanCloudHealthCheck) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudHealthCheckProperties(props)

	return Tool{
		Name: "create_arvancloud_health_check",
		Description: "Create a new active health check: periodically probe an origin (in practice, a Load " +
			"Balancing pool) over TCP or HTTP(S) and track whether it is reachable. " + arvanCloudHealthCheckVsLBNote +
			" This is a fast operation: the created check, including its provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "name", "origin", "interval_ms", "threshold", "type"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudHealthCheckArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudHealthCheckInput{
				Credentials: args.domain(), Domain: args.Domain, Check: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudHealthCheckToMap(*created), nil
		},
	}
}

func getArvanCloudHealthCheckTool(uc *app.GetArvanCloudHealthCheck) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudHealthCheckIDProperty()

	return Tool{
		Name:        "get_arvancloud_health_check",
		Description: "Get the current state of one active health check by ID. " + arvanCloudHealthCheckVsLBNote + " This is a fast operation.",
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

			found, err := uc.Execute(ctx, app.GetArvanCloudHealthCheckInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudHealthCheckToMap(*found), nil
		},
	}
}

func updateArvanCloudHealthCheckTool(uc *app.UpdateArvanCloudHealthCheck) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudHealthCheckIDProperty()
	arvanCloudHealthCheckProperties(props)

	return Tool{
		Name: "update_arvancloud_health_check",
		Description: "Update an active health check. This replaces the check's fields with the given values — pass " +
			"every field you want to keep, not only the ones changing. " + arvanCloudHealthCheckVsLBNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "name", "origin", "interval_ms", "threshold", "type"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudHealthCheckArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudHealthCheckInput{
				Credentials: args.domain(), Domain: args.Domain, ID: args.ID, Check: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudHealthCheckToMap(*updated), nil
		},
	}
}

func deleteArvanCloudHealthCheckTool(uc *app.DeleteArvanCloudHealthCheck) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudHealthCheckIDProperty()

	return Tool{
		Name: "delete_arvancloud_health_check",
		Description: "Permanently delete an active health check by ID. " + arvanCloudHealthCheckVsLBNote +
			" This is a fast operation and cannot be undone. Deleting a check that no longer exists is treated as already done rather than an error.",
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

			if err := uc.Execute(ctx, app.DeleteArvanCloudHealthCheckInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

// --- Zones ----------------------------------------------------------------

func listArvanCloudDomainHealthCheckZonesTool(uc *app.ListArvanCloudDomainHealthCheckZones) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_domain_health_check_zones",
		Description: "List the check-execution zones available to a domain, to choose from when setting an " +
			"active health check's \"zones\". This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			zones, err := uc.Execute(ctx, app.ListArvanCloudDomainHealthCheckZonesInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(zones))
			for i, z := range zones {
				out[i] = arvanCloudHealthCheckZoneNameToMap(z)
			}
			return map[string]any{"zones": out}, nil
		},
	}
}

func listArvanCloudHealthCheckZonesTool(uc *app.ListArvanCloudHealthCheckZones) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_arvancloud_health_check_zones",
		Description: "List the account-independent, global set of check-execution zones. Deprecated by ArvanCloud " +
			"in favor of list_arvancloud_domain_health_check_zones, but still available. This is a fast operation.",
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

			zones, err := uc.Execute(ctx, app.ListArvanCloudHealthCheckZonesInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(zones))
			for i, z := range zones {
				out[i] = arvanCloudHealthCheckZoneNameToMap(z)
			}
			return map[string]any{"zones": out}, nil
		},
	}
}

// --- Reports ----------------------------------------------------------------

// arvanCloudHealthCheckReportProperties adds the filter fields shared by
// get_arvancloud_health_check_summary and get_arvancloud_health_check_details
// to props. includePaging additionally adds type/per_page/page, meaningful
// only for the details report.
func arvanCloudHealthCheckReportProperties(props map[string]any, includePaging bool) {
	props["name"] = map[string]any{"type": "string", "description": "REQUIRED. The health check's name to report on."}
	props["upstream"] = map[string]any{"type": "string", "description": "REQUIRED. Which upstream (within the check's origin) to report on."}
	props["period"] = map[string]any{
		"type":        "string",
		"enum":        []string{"5m", "1h", "3h", "6h", "12h", "24h", "7d", "30d"},
		"description": "A period ending now to report over, e.g. \"1h\" for the last hour. \"5m\" is available only for enterprise domains.",
	}
	props["since"] = map[string]any{"type": "string", "description": "Report window start, ISO 8601 UTC, e.g. \"2026-08-01T00:00:00Z\". May be combined with \"period\"."}
	props["until"] = map[string]any{"type": "string", "description": "Report window end, ISO 8601 UTC, e.g. \"2026-08-21T00:00:00Z\"."}
	props["direction"] = map[string]any{"type": "string", "enum": []string{"asc", "desc"}, "description": "Sort direction for the report entries."}
	if includePaging {
		props["type"] = map[string]any{
			"type":        "string",
			"enum":        []string{"all", "success", "error"},
			"description": "Filter probe results by outcome. Defaults to \"all\" when omitted.",
		}
		props["per_page"] = map[string]any{"type": "integer", "description": "How many probe results per page. Omit for ArvanCloud's own default."}
		props["page"] = map[string]any{"type": "integer", "description": "Which page to return, 1-indexed. Omit for page 1."}
	}
}

// arvanCloudHealthCheckReportArgs decodes the filter fields shared by both
// report tools.
type arvanCloudHealthCheckReportArgs struct {
	arvanCloudDomainNameArgs
	Name      string `json:"name"`
	Upstream  string `json:"upstream"`
	Period    string `json:"period"`
	Since     string `json:"since"`
	Until     string `json:"until"`
	Direction string `json:"direction"`
	Type      string `json:"type"`
	PerPage   int    `json:"per_page"`
	Page      int    `json:"page"`
}

func (a arvanCloudHealthCheckReportArgs) toDomain() domain.ArvanCloudHealthCheckReportQuery {
	return domain.ArvanCloudHealthCheckReportQuery{
		Name: a.Name, Upstream: a.Upstream, Period: a.Period, Since: a.Since, Until: a.Until,
		Direction: a.Direction, Type: a.Type, PerPage: a.PerPage, Page: a.Page,
	}
}

func arvanCloudHealthCheckReportSummaryToMap(s domain.ArvanCloudHealthCheckReportSummary) map[string]any {
	details := make([]map[string]any, len(s.Details))
	for i, d := range s.Details {
		details[i] = map[string]any{"date": d.Date, "status": d.Status}
	}
	return map[string]any{
		"zone": s.Zone, "status": s.Status, "total": s.Total, "failed": s.Failed, "details": details,
	}
}

func getArvanCloudHealthCheckSummaryTool(uc *app.GetArvanCloudHealthCheckSummary) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudHealthCheckReportProperties(props, false)

	return Tool{
		Name:        "get_arvancloud_health_check_summary",
		Description: "Get a health check's per-zone summary monitoring report — not paginated, the full per-zone breakdown is returned in one call. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "name", "upstream"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudHealthCheckReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			report, err := uc.Execute(ctx, app.GetArvanCloudHealthCheckSummaryInput{
				Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(report))
			for i, s := range report {
				out[i] = arvanCloudHealthCheckReportSummaryToMap(s)
			}
			return map[string]any{"summary": out}, nil
		},
	}
}

func getArvanCloudHealthCheckDetailsTool(uc *app.GetArvanCloudHealthCheckDetails) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudHealthCheckReportProperties(props, true)

	return Tool{
		Name:        "get_arvancloud_health_check_details",
		Description: "Get a page of a health check's individual probe results. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "name", "upstream"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudHealthCheckReportArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			result, err := uc.Execute(ctx, app.GetArvanCloudHealthCheckDetailsInput{
				Credentials: args.domain(), Domain: args.Domain, Query: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(result.Details))
			for i, d := range result.Details {
				out[i] = map[string]any{
					"date": d.Date, "zone": d.Zone, "upstream": d.Upstream, "status": d.Status, "message": d.Message,
				}
			}
			return map[string]any{
				"details": out,
				"page": map[string]any{
					"current_page": result.Page.CurrentPage, "from": result.Page.From, "last_page": result.Page.LastPage,
					"per_page": result.Page.PerPage, "to": result.Page.To, "total": result.Page.Total,
				},
			}, nil
		},
	}
}
