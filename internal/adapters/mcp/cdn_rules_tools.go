package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// Origin Rule, Page Rule and Transform Rule tools (issue #24). Every tool
// here is a fast operation: the CDN rule engines are single HTTP round trips
// with no long-running provisioning state, so each Description says so and
// no tool returns an operation_id.

// ---------------------------------------------------------------------------
// Shared condition-tree ("rule_items") schema and (de)serialization, used by
// Origin Rules and Transform Rules — Page Rules has no condition tree.

// ruleConditionArgs is one entry of a rule_items condition group.
type ruleConditionArgs struct {
	Field     string          `json:"field"`
	Operation string          `json:"operation"`
	Value     json.RawMessage `json:"value"`
}

func toDomainConditionGroups(groups [][]ruleConditionArgs) [][]domain.CDNRuleCondition {
	out := make([][]domain.CDNRuleCondition, len(groups))
	for i, group := range groups {
		conditions := make([]domain.CDNRuleCondition, len(group))
		for j, c := range group {
			conditions[j] = domain.CDNRuleCondition{Field: c.Field, Operation: c.Operation, Value: c.Value}
		}
		out[i] = conditions
	}
	return out
}

// ruleConditionToMap renders one condition, including the read-only fields
// the provider only populates on get/list (ValueDetail, Bulklist*).
func ruleConditionToMap(c domain.CDNRuleCondition) map[string]any {
	m := map[string]any{"field": c.Field, "operation": c.Operation}
	if len(c.Value) > 0 {
		m["value"] = c.Value
	}
	if len(c.ValueDetail) > 0 {
		m["value_detail"] = c.ValueDetail
	}
	if c.BulklistID != "" {
		m["bulklist"] = map[string]any{"id": c.BulklistID, "name": c.BulklistName, "type": c.BulklistType}
	}
	return m
}

func ruleItemsToSlice(groups [][]domain.CDNRuleCondition) []any {
	out := make([]any, len(groups))
	for i, group := range groups {
		conditions := make([]map[string]any, len(group))
		for j, c := range group {
			conditions[j] = ruleConditionToMap(c)
		}
		out[i] = conditions
	}
	return out
}

// ruleConditionProperty is the JSON Schema for one condition inside a
// rule_items group.
func ruleConditionProperty() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":        "string",
				"description": "Request attribute to match, e.g. \"full_url\", \"country_code\", or \"user_agent\". Not a fixed enum — the provider validates it.",
			},
			"operation": map[string]any{
				"type":        "string",
				"description": "How to compare field against value, e.g. \"contains\", \"equals\", or \"list\" for matching against an array of values. Not a fixed enum — the provider validates it.",
			},
			"value": map[string]any{
				"description": "Value(s) to compare against: a single string/number for most operations, or an array of them for a \"list\"-style operation, e.g. \"curl\" or [353, 840].",
			},
		},
		"required": []string{"field", "operation"},
	}
}

// ruleItemsProperty is the JSON Schema for a rule's full condition tree.
func ruleItemsProperty() map[string]any {
	return map[string]any{
		"type": "array",
		"description": "Nested condition groups deciding which traffic this rule matches: the outer array is ORed " +
			"together, and the conditions inside each inner array are ANDed. E.g. [[{full_url contains " +
			"\"/admin\"}, {country_code equals 1}]] matches traffic under /admin AND from country 1.",
		"items": map[string]any{
			"type":  "array",
			"items": ruleConditionProperty(),
		},
	}
}

// ---------------------------------------------------------------------------
// Origin Rules — route matching traffic in a zone to a specific origin.

func originRuleIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The origin rule's ID, as returned by create_cdn_origin_rule or list_cdn_origin_rules.",
	}
}

func originRuleToMap(rule domain.CDNOriginRule) map[string]any {
	return map[string]any{
		"id":                rule.ID,
		"name":              rule.Name,
		"type":              rule.Type,
		"upstream_ip":       rule.UpstreamIP,
		"port":              rule.Port,
		"load_balance_id":   rule.LoadBalanceID,
		"load_balance_name": rule.LoadBalanceName,
		"enabled":           rule.Enabled,
		"priority":          rule.Priority,
		"rule_items":        ruleItemsToSlice(rule.RuleItems),
	}
}

type originRuleWriteArgs struct {
	credentialArgs
	ZoneUUID      string                `json:"zone_uuid"`
	Name          string                `json:"name"`
	Type          string                `json:"type"`
	UpstreamIP    string                `json:"upstream_ip"`
	Port          int                   `json:"port"`
	LoadBalanceID string                `json:"load_balance_id"`
	RuleItems     [][]ruleConditionArgs `json:"rule_items"`
}

// updateOriginRuleArgs extends originRuleWriteArgs with the path ID instead
// of also embedding originRuleIDArgs, which would embed credentialArgs a
// second time and make args.domain() an ambiguous selector.
type updateOriginRuleArgs struct {
	originRuleWriteArgs
	RuleID string `json:"origin_rule_id"`
}

func (a originRuleWriteArgs) toDomain() domain.CDNOriginRule {
	return domain.CDNOriginRule{
		Name: a.Name, Type: a.Type, UpstreamIP: a.UpstreamIP, Port: a.Port, LoadBalanceID: a.LoadBalanceID,
		RuleItems: toDomainConditionGroups(a.RuleItems),
	}
}

// originRuleWriteProperties is the JSON Schema shared by create_cdn_origin_rule
// and update_cdn_origin_rule.
func originRuleWriteProperties() map[string]any {
	return map[string]any{
		"name": map[string]any{
			"type":        "string",
			"description": "Human-readable origin rule name, e.g. \"upstream-rule\".",
		},
		"type": map[string]any{
			"type":        "string",
			"enum":        []string{"upstream", "port", "load_balance"},
			"description": "Where matching traffic is routed: a fixed IP (\"upstream\"), a port (\"port\"), or a configured load balancer (\"load_balance\").",
		},
		"upstream_ip": map[string]any{
			"type":        "string",
			"description": "Origin IP address to route matching traffic to. Required when type is \"upstream\", e.g. \"1.1.1.1\".",
		},
		"port": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     65535,
			"description": "Origin port to route matching traffic to. Required when type is \"port\", e.g. 8080.",
		},
		"load_balance_id": map[string]any{
			"type":        "string",
			"description": "ID of the CDN load balancer to route matching traffic to. Required when type is \"load_balance\".",
		},
		"rule_items": ruleItemsProperty(),
	}
}

func listCDNOriginRulesTool(uc *app.ListCDNOriginRules) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_cdn_origin_rules",
		Description: "List every origin rule of a Parspack CDN zone. Origin rules route matching traffic to a " +
			"specific origin (a fixed IP, a port, or a load balancer). This is a fast operation.",
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

			rules, err := uc.Execute(ctx, app.ListCDNOriginRulesInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, rule := range rules {
				out[i] = originRuleToMap(rule)
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "origin_rules": out}, nil
		},
	}
}

func createCDNOriginRuleTool(uc *app.CreateCDNOriginRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	for k, v := range originRuleWriteProperties() {
		props[k] = v
	}

	return Tool{
		Name: "create_cdn_origin_rule",
		Description: "Create a new origin rule in a Parspack CDN zone, routing matching traffic to a specific " +
			"origin. This is a fast operation. The provider does not return the new rule's ID in the response — " +
			"call list_cdn_origin_rules afterward to discover it.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "name", "rule_items"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args originRuleWriteArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.CreateCDNOriginRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Rule: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return originRuleToMap(*rule), nil
		},
	}
}

type originRuleIDArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	RuleID   string `json:"origin_rule_id"`
}

func getCDNOriginRuleTool(uc *app.GetCDNOriginRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["origin_rule_id"] = originRuleIDProperty()

	return Tool{
		Name: "get_cdn_origin_rule",
		Description: "Get the current state of one origin rule in a Parspack CDN zone by its ID, including its " +
			"full condition tree. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "origin_rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args originRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.GetCDNOriginRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID,
			})
			if err != nil {
				return nil, err
			}
			return originRuleToMap(*rule), nil
		},
	}
}

func updateCDNOriginRuleTool(uc *app.UpdateCDNOriginRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["origin_rule_id"] = originRuleIDProperty()
	for k, v := range originRuleWriteProperties() {
		props[k] = v
	}

	return Tool{
		Name: "update_cdn_origin_rule",
		Description: "Replace the configuration of an existing origin rule in a Parspack CDN zone by its ID. This " +
			"is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "origin_rule_id", "name", "rule_items"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateOriginRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.UpdateCDNOriginRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID,
				Rule: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return originRuleToMap(*rule), nil
		},
	}
}

func deleteCDNOriginRuleTool(uc *app.DeleteCDNOriginRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["origin_rule_id"] = originRuleIDProperty()

	return Tool{
		Name: "delete_cdn_origin_rule",
		Description: "Permanently delete an origin rule from a Parspack CDN zone by its ID. This is a fast " +
			"operation and cannot be undone. Deleting a rule that no longer exists is treated as already done " +
			"rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "origin_rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args originRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteCDNOriginRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID, "origin_rule_id": args.RuleID}, nil
		},
	}
}

type toggleOriginRuleArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	RuleID   string `json:"origin_rule_id"`
	Enabled  bool   `json:"enabled"`
}

func toggleCDNOriginRuleTool(uc *app.ToggleCDNOriginRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["origin_rule_id"] = originRuleIDProperty()
	props["enabled"] = map[string]any{
		"type":        "boolean",
		"description": "true to enable the rule, false to disable it without deleting it.",
	}

	return Tool{
		Name: "toggle_cdn_origin_rule",
		Description: "Enable or disable an origin rule in a Parspack CDN zone by its ID, without deleting it. " +
			"This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "origin_rule_id", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args toggleOriginRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.ToggleCDNOriginRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID, Enabled: args.Enabled,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "origin_rule_id": args.RuleID, "enabled": args.Enabled}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// Page Rules — apply per-URL/user-agent settings overrides to matching
// traffic. No toggle tool exists for this rule engine: the provider exposes
// no toggle-status endpoint for page rules.

func pageRuleIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The page rule's ID, as returned by create_cdn_page_rule or list_cdn_page_rules.",
	}
}

func pageRuleToMap(rule domain.CDNPageRule) map[string]any {
	cachePolicy := ""
	if rule.CachePolicy != domain.DNSRecordProxyUnknown {
		cachePolicy = rule.CachePolicy.String()
	}

	var urlRedirection map[string]any
	if rule.URLRedirection != nil {
		urlRedirection = map[string]any{"http_code": rule.URLRedirection.HTTPCode, "url": rule.URLRedirection.URL}
	}
	var portDestination map[string]any
	if rule.PortDestination != nil {
		portDestination = map[string]any{"http": rule.PortDestination.HTTP, "https": rule.PortDestination.HTTPS}
	}
	cookies := make([]map[string]any, len(rule.AdditionCookies))
	for i, ck := range rule.AdditionCookies {
		cookies[i] = map[string]any{
			"key": ck.Key, "value": ck.Value, "expire_seconds": ck.ExpireSeconds, "path": ck.Path,
			"http_only": ck.HTTPOnly, "secure": ck.Secure,
		}
	}

	return map[string]any{
		"id":                  rule.ID,
		"name":                rule.Name,
		"type":                rule.Type,
		"operator":            rule.Operator,
		"value":               rule.Value,
		"minify":              rule.Minify,
		"cache_policy":        cachePolicy,
		"cache_ttl_seconds":   rule.CacheTTLSeconds,
		"firewall_status":     rule.FirewallStatus,
		"firewall_except_ips": rule.FirewallExceptIPs,
		"waf_is_active":       rule.WAFIsActive,
		"removable_headers":   rule.RemovableHeaders,
		"addition_headers":    rule.AdditionHeaders,
		"url_redirection":     urlRedirection,
		"port_destination":    portDestination,
		"addition_cookies":    cookies,
		"priority":            rule.Priority,
	}
}

type urlRedirectionArgs struct {
	HTTPCode int    `json:"http_code"`
	URL      string `json:"url"`
}

type portDestinationArgs struct {
	HTTP  int `json:"http"`
	HTTPS int `json:"https"`
}

type pageCookieArgs struct {
	Key           string `json:"key"`
	Value         string `json:"value"`
	ExpireSeconds int    `json:"expire_seconds"`
	Path          string `json:"path"`
	HTTPOnly      bool   `json:"http_only"`
	Secure        bool   `json:"secure"`
}

type pageRuleWriteArgs struct {
	credentialArgs
	ZoneUUID          string               `json:"zone_uuid"`
	Name              string               `json:"name"`
	Type              string               `json:"type"`
	Operator          string               `json:"operator"`
	Value             string               `json:"value"`
	Minify            bool                 `json:"minify"`
	CachePolicy       string               `json:"cache_policy"`
	CacheTTLSeconds   int                  `json:"cache_ttl_seconds"`
	FirewallStatus    bool                 `json:"firewall_status"`
	FirewallExceptIPs []string             `json:"firewall_except_ips"`
	WAFIsActive       bool                 `json:"waf_is_active"`
	RemovableHeaders  []string             `json:"removable_headers"`
	URLRedirection    *urlRedirectionArgs  `json:"url_redirection"`
	AdditionHeaders   map[string]string    `json:"addition_headers"`
	PortDestination   *portDestinationArgs `json:"port_destination"`
	AdditionCookies   []pageCookieArgs     `json:"addition_cookies"`
}

func (a pageRuleWriteArgs) toDomain() (domain.CDNPageRule, error) {
	rule := domain.CDNPageRule{
		Name: a.Name, Type: a.Type, Operator: a.Operator, Value: a.Value, Minify: a.Minify,
		CacheTTLSeconds: a.CacheTTLSeconds, FirewallStatus: a.FirewallStatus, FirewallExceptIPs: a.FirewallExceptIPs,
		WAFIsActive: a.WAFIsActive, RemovableHeaders: a.RemovableHeaders, AdditionHeaders: a.AdditionHeaders,
	}
	if a.CachePolicy != "" {
		policy, err := domain.ParseDNSRecordProxy(a.CachePolicy)
		if err != nil {
			return domain.CDNPageRule{}, fmt.Errorf("cache_policy %q is not supported: %w", a.CachePolicy, err)
		}
		rule.CachePolicy = policy
	}
	if a.URLRedirection != nil {
		rule.URLRedirection = &domain.CDNPageRuleRedirect{HTTPCode: a.URLRedirection.HTTPCode, URL: a.URLRedirection.URL}
	}
	if a.PortDestination != nil {
		rule.PortDestination = &domain.CDNPagePortDestination{HTTP: a.PortDestination.HTTP, HTTPS: a.PortDestination.HTTPS}
	}
	for _, ck := range a.AdditionCookies {
		rule.AdditionCookies = append(rule.AdditionCookies, domain.CDNPageRuleCookie{
			Key: ck.Key, Value: ck.Value, ExpireSeconds: ck.ExpireSeconds, Path: ck.Path,
			HTTPOnly: ck.HTTPOnly, Secure: ck.Secure,
		})
	}
	return rule, nil
}

// updatePageRuleArgs extends pageRuleWriteArgs with the path ID instead of
// also embedding pageRuleIDArgs, which would embed credentialArgs a second
// time and make args.domain() an ambiguous selector.
type updatePageRuleArgs struct {
	pageRuleWriteArgs
	RuleID string `json:"page_rule_id"`
}

// pageRuleWriteProperties is the JSON Schema shared by create_cdn_page_rule
// and update_cdn_page_rule.
func pageRuleWriteProperties() map[string]any {
	return map[string]any{
		"name": map[string]any{
			"type":        "string",
			"description": "Human-readable page rule name, e.g. \"cache-assets\".",
		},
		"type": map[string]any{
			"type":        "string",
			"enum":        []string{"url", "user_agent"},
			"description": "What request attribute to match against value: the request URL (\"url\") or the client's User-Agent header (\"user_agent\").",
		},
		"operator": map[string]any{
			"type":        "string",
			"enum":        []string{"pattern", "equals", "not_equals", "contains", "not_contains"},
			"description": "How to compare the matched attribute against value, e.g. \"pattern\" for a wildcard URL pattern match.",
		},
		"value": map[string]any{
			"type":        "string",
			"description": "The pattern or string to match, e.g. \"*example.com/assets/**\" for a url pattern.",
		},
		"minify": map[string]any{
			"type":        "boolean",
			"description": "Minify matching HTML/CSS/JS responses. Defaults to false.",
		},
		"cache_policy": map[string]any{
			"type":        "string",
			"enum":        []string{"direct", "cdn-no-caching", "cdn-static-caching", "cdn-smart-caching", "cdn-always-caching"},
			"description": "CDN caching mode applied to matching traffic, overriding the zone default. Same enum as a DNS record's proxy mode.",
		},
		"cache_ttl_seconds": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"maximum":     86400,
			"description": "Cache TTL in seconds for matching responses, e.g. 3600 for one hour.",
		},
		"firewall_status": map[string]any{
			"type":        "boolean",
			"description": "Whether the zone's firewall applies to matching traffic. Required.",
		},
		"firewall_except_ips": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"maxItems":    100,
			"description": "IP addresses/CIDRs exempted from the firewall for matching traffic, e.g. \"192.168.0.1\".",
		},
		"waf_is_active": map[string]any{
			"type":        "boolean",
			"description": "Whether the Web Application Firewall (WAF) applies to matching traffic.",
		},
		"removable_headers": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"maxItems":    50,
			"description": "Response header names to strip from matching traffic, e.g. \"X-Powered-By\".",
		},
		"url_redirection": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"http_code": map[string]any{
					"type": "integer", "enum": []int{301, 302},
					"description": "HTTP redirect status code.",
				},
				"url": map[string]any{"type": "string", "description": "Destination URL to redirect matching traffic to."},
			},
			"description": "Redirect matching traffic to another URL. Omit for no redirect.",
		},
		"addition_headers": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{"type": "string"},
			"description":          "Extra response headers to add to matching traffic, keyed by header name, e.g. {\"access-control-allow-origin\": \"*\"}. Up to 50 entries.",
		},
		"port_destination": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"http":  map[string]any{"type": "integer", "minimum": 1, "maximum": 65535, "description": "Origin HTTP port override."},
				"https": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535, "description": "Origin HTTPS port override."},
			},
			"description": "Override the origin port(s) matching traffic is forwarded to. Omit to use the zone default.",
		},
		"addition_cookies": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key":            map[string]any{"type": "string", "description": "Cookie name."},
					"value":          map[string]any{"type": "string", "description": "Cookie value."},
					"expire_seconds": map[string]any{"type": "integer", "minimum": 0, "maximum": 86400, "description": "Seconds until the cookie expires."},
					"path":           map[string]any{"type": "string", "description": "Cookie path, e.g. \"/\"."},
					"http_only":      map[string]any{"type": "boolean", "description": "Mark the cookie HttpOnly."},
					"secure":         map[string]any{"type": "boolean", "description": "Mark the cookie Secure."},
				},
				"required": []string{"key", "value"},
			},
			"maxItems":    20,
			"description": "Cookies to inject into matching responses.",
		},
	}
}

func listCDNPageRulesTool(uc *app.ListCDNPageRules) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_cdn_page_rules",
		Description: "List every page rule of a Parspack CDN zone. Page rules apply settings overrides (caching, " +
			"firewall, headers, cookies, redirects, ...) to traffic matching a URL or User-Agent pattern. This is a " +
			"fast operation.",
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

			rules, err := uc.Execute(ctx, app.ListCDNPageRulesInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, rule := range rules {
				out[i] = pageRuleToMap(rule)
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "page_rules": out}, nil
		},
	}
}

func createCDNPageRuleTool(uc *app.CreateCDNPageRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	for k, v := range pageRuleWriteProperties() {
		props[k] = v
	}

	return Tool{
		Name: "create_cdn_page_rule",
		Description: "Create a new page rule in a Parspack CDN zone, applying settings overrides to traffic " +
			"matching a URL or User-Agent pattern. This is a fast operation. The provider does not return the new " +
			"rule's ID in the response — call list_cdn_page_rules afterward to discover it.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "name", "value", "firewall_status"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args pageRuleWriteArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			rule, err := args.toDomain()
			if err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateCDNPageRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Rule: rule,
			})
			if err != nil {
				return nil, err
			}
			return pageRuleToMap(*created), nil
		},
	}
}

type pageRuleIDArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	RuleID   string `json:"page_rule_id"`
}

func getCDNPageRuleTool(uc *app.GetCDNPageRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["page_rule_id"] = pageRuleIDProperty()

	return Tool{
		Name: "get_cdn_page_rule",
		Description: "Get the current state of one page rule in a Parspack CDN zone by its ID. This is a fast " +
			"operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "page_rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args pageRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.GetCDNPageRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID,
			})
			if err != nil {
				return nil, err
			}
			return pageRuleToMap(*rule), nil
		},
	}
}

func updateCDNPageRuleTool(uc *app.UpdateCDNPageRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["page_rule_id"] = pageRuleIDProperty()
	for k, v := range pageRuleWriteProperties() {
		props[k] = v
	}

	return Tool{
		Name: "update_cdn_page_rule",
		Description: "Replace the configuration of an existing page rule in a Parspack CDN zone by its ID. This " +
			"is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "page_rule_id", "name", "value", "firewall_status"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updatePageRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			rule, err := args.toDomain()
			if err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateCDNPageRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID, Rule: rule,
			})
			if err != nil {
				return nil, err
			}
			return pageRuleToMap(*updated), nil
		},
	}
}

func deleteCDNPageRuleTool(uc *app.DeleteCDNPageRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["page_rule_id"] = pageRuleIDProperty()

	return Tool{
		Name: "delete_cdn_page_rule",
		Description: "Permanently delete a page rule from a Parspack CDN zone by its ID. This is a fast operation " +
			"and cannot be undone. Deleting a rule that no longer exists is treated as already done rather than an " +
			"error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "page_rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args pageRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteCDNPageRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID, "page_rule_id": args.RuleID}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// Transform Rules — rewrite request/response headers on matching traffic.

func transformRuleIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The transform rule's ID, as returned by create_cdn_transform_rule or list_cdn_transform_rules.",
	}
}

func headerActionToMap(a domain.CDNTransformHeaderAction) map[string]any {
	return map[string]any{"header_name": a.HeaderName, "header_value": a.HeaderValue, "action": a.Action}
}

func transformRuleToMap(rule domain.CDNTransformRule) map[string]any {
	reqHeaders := make([]map[string]any, len(rule.RequestHeaders))
	for i, h := range rule.RequestHeaders {
		reqHeaders[i] = headerActionToMap(h)
	}
	respHeaders := make([]map[string]any, len(rule.ResponseHeaders))
	for i, h := range rule.ResponseHeaders {
		respHeaders[i] = headerActionToMap(h)
	}

	return map[string]any{
		"id":               rule.ID,
		"name":             rule.Name,
		"enabled":          rule.Enabled,
		"priority":         rule.Priority,
		"request_headers":  reqHeaders,
		"response_headers": respHeaders,
		"rule_items":       ruleItemsToSlice(rule.RuleItems),
	}
}

type headerActionArgs struct {
	HeaderName  string `json:"header_name"`
	HeaderValue string `json:"header_value"`
	Action      string `json:"action"`
}

func toDomainHeaderActionArgs(args []headerActionArgs) []domain.CDNTransformHeaderAction {
	out := make([]domain.CDNTransformHeaderAction, len(args))
	for i, a := range args {
		out[i] = domain.CDNTransformHeaderAction{HeaderName: a.HeaderName, HeaderValue: a.HeaderValue, Action: a.Action}
	}
	return out
}

// headerActionProperty is the JSON Schema for one request/response header
// entry, shared by create_cdn_transform_rule and update_cdn_transform_rule.
func headerActionProperty() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"header_name": map[string]any{
				"type":        "string",
				"description": "Name of the header to modify or delete, e.g. \"X-Frame-Options\".",
			},
			"header_value": map[string]any{
				"type":        "string",
				"description": "New header value. Required when action is \"modify\"; ignored when action is \"delete\".",
			},
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"modify", "delete"},
				"description": "Whether to set (\"modify\") or remove (\"delete\") the header.",
			},
		},
		"required": []string{"header_name", "action"},
	}
}

type transformRuleWriteArgs struct {
	credentialArgs
	ZoneUUID        string                `json:"zone_uuid"`
	Name            string                `json:"name"`
	RequestHeaders  []headerActionArgs    `json:"request_headers"`
	ResponseHeaders []headerActionArgs    `json:"response_headers"`
	RuleItems       [][]ruleConditionArgs `json:"rule_items"`
}

// updateTransformRuleArgs extends transformRuleWriteArgs with the path ID
// instead of also embedding transformRuleIDArgs, which would embed
// credentialArgs a second time and make args.domain() an ambiguous selector.
type updateTransformRuleArgs struct {
	transformRuleWriteArgs
	RuleID string `json:"transform_rule_id"`
}

func (a transformRuleWriteArgs) toDomain() domain.CDNTransformRule {
	return domain.CDNTransformRule{
		Name:            a.Name,
		RequestHeaders:  toDomainHeaderActionArgs(a.RequestHeaders),
		ResponseHeaders: toDomainHeaderActionArgs(a.ResponseHeaders),
		RuleItems:       toDomainConditionGroups(a.RuleItems),
	}
}

// transformRuleWriteProperties is the JSON Schema shared by
// create_cdn_transform_rule and update_cdn_transform_rule.
func transformRuleWriteProperties() map[string]any {
	return map[string]any{
		"name": map[string]any{
			"type":        "string",
			"description": "Human-readable transform rule name, e.g. \"header-modify-rule\".",
		},
		"request_headers": map[string]any{
			"type":        "array",
			"items":       headerActionProperty(),
			"maxItems":    20,
			"description": "Modify or delete headers on the matching REQUEST before it reaches the origin.",
		},
		"response_headers": map[string]any{
			"type":        "array",
			"items":       headerActionProperty(),
			"maxItems":    20,
			"description": "Modify or delete headers on the RESPONSE before it reaches the client.",
		},
		"rule_items": ruleItemsProperty(),
	}
}

func listCDNTransformRulesTool(uc *app.ListCDNTransformRules) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_cdn_transform_rules",
		Description: "List every transform rule of a Parspack CDN zone. Transform rules rewrite request/response " +
			"headers on matching traffic. This is a fast operation.",
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

			rules, err := uc.Execute(ctx, app.ListCDNTransformRulesInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, rule := range rules {
				out[i] = transformRuleToMap(rule)
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "transform_rules": out}, nil
		},
	}
}

func createCDNTransformRuleTool(uc *app.CreateCDNTransformRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	for k, v := range transformRuleWriteProperties() {
		props[k] = v
	}

	return Tool{
		Name: "create_cdn_transform_rule",
		Description: "Create a new transform rule in a Parspack CDN zone, rewriting request/response headers on " +
			"matching traffic. This is a fast operation. The provider does not return the new rule's ID in the " +
			"response — call list_cdn_transform_rules afterward to discover it.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "name", "rule_items"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args transformRuleWriteArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.CreateCDNTransformRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Rule: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return transformRuleToMap(*rule), nil
		},
	}
}

type transformRuleIDArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	RuleID   string `json:"transform_rule_id"`
}

func getCDNTransformRuleTool(uc *app.GetCDNTransformRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["transform_rule_id"] = transformRuleIDProperty()

	return Tool{
		Name: "get_cdn_transform_rule",
		Description: "Get the current state of one transform rule in a Parspack CDN zone by its ID, including its " +
			"header rewrites and full condition tree. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "transform_rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args transformRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.GetCDNTransformRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID,
			})
			if err != nil {
				return nil, err
			}
			return transformRuleToMap(*rule), nil
		},
	}
}

func updateCDNTransformRuleTool(uc *app.UpdateCDNTransformRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["transform_rule_id"] = transformRuleIDProperty()
	for k, v := range transformRuleWriteProperties() {
		props[k] = v
	}

	return Tool{
		Name: "update_cdn_transform_rule",
		Description: "Replace the configuration of an existing transform rule in a Parspack CDN zone by its ID. " +
			"This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "transform_rule_id", "name", "rule_items"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateTransformRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.UpdateCDNTransformRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID,
				Rule: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return transformRuleToMap(*rule), nil
		},
	}
}

func deleteCDNTransformRuleTool(uc *app.DeleteCDNTransformRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["transform_rule_id"] = transformRuleIDProperty()

	return Tool{
		Name: "delete_cdn_transform_rule",
		Description: "Permanently delete a transform rule from a Parspack CDN zone by its ID. This is a fast " +
			"operation and cannot be undone. Deleting a rule that no longer exists is treated as already done " +
			"rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "transform_rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args transformRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteCDNTransformRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID, "transform_rule_id": args.RuleID}, nil
		},
	}
}

type toggleTransformRuleArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	RuleID   string `json:"transform_rule_id"`
	Enabled  bool   `json:"enabled"`
}

func toggleCDNTransformRuleTool(uc *app.ToggleCDNTransformRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["transform_rule_id"] = transformRuleIDProperty()
	props["enabled"] = map[string]any{
		"type":        "boolean",
		"description": "true to enable the rule, false to disable it without deleting it.",
	}

	return Tool{
		Name: "toggle_cdn_transform_rule",
		Description: "Enable or disable a transform rule in a Parspack CDN zone by its ID, without deleting it. " +
			"This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "transform_rule_id", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args toggleTransformRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.ToggleCDNTransformRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID, Enabled: args.Enabled,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "transform_rule_id": args.RuleID, "enabled": args.Enabled}, nil
		},
	}
}
