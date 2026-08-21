package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud Page Rules, Response Transforms, Redirect (www-redirect) and
// Host Header Whitelist tools (issue #71) — see
// domain/arvancloud_rules.go's package comment for what each resource is.
// All fast operations (AGENTS.md 4.3): every tool below returns its result
// within the call, with no operation_id to poll afterward.
//
// Schema design for create/update_arvancloud_page_rule (issue #71's own
// question to resolve, given the field count): the input schema is split
// into the SAME six named sub-objects the domain type itself groups fields
// into — matching/caching/security/routing/headers/other — rather than one
// flat ~40-property object, and rather than an arbitrary "common fields plus
// one advanced catch-all" split. Mirroring the domain grouping means the
// calling chatbot sees the same structure this codebase's Go code and doc
// comments already describe (e.g. "cache_200 is under caching"), and each
// sub-object's own required/enum constraints stay local to it instead of
// competing for attention in one giant flat property list.

// --- Shared render helpers -------------------------------------------------

func arvanCloudAccelerationToMap(a domain.ArvanCloudAccelerationSettings) map[string]any {
	extensions := make([]string, len(a.Extensions))
	for i, e := range a.Extensions {
		extensions[i] = string(e)
	}
	return map[string]any{"status": string(a.Status), "extensions": extensions}
}

func arvanCloudPageRuleImageResizeToMap(r domain.ArvanCloudPageRuleImageResize) map[string]any {
	return map[string]any{"status": string(r.Status), "height_by": r.HeightBy, "width_by": r.WidthBy, "mode": string(r.Mode)}
}

func arvanCloudUpstreamTimeoutToMap(t domain.ArvanCloudUpstreamTimeout) map[string]any {
	return map[string]any{
		"connect_timeout_seconds": t.ConnectTimeoutSeconds, "read_timeout_seconds": t.ReadTimeoutSeconds, "send_timeout_seconds": t.SendTimeoutSeconds,
	}
}

func arvanCloudPageRuleRedirectToMap(r domain.ArvanCloudPageRuleRedirect) map[string]any {
	return map[string]any{"enable": r.Enable, "status_code": r.StatusCode, "url": r.URL}
}

func arvanCloudPageRuleHeaderEntryToMap(h domain.ArvanCloudPageRuleHeaderEntry) map[string]any {
	return map[string]any{"name": h.Name, "value": h.Value}
}

func arvanCloudPageRuleHeaderEntriesToMaps(items []domain.ArvanCloudPageRuleHeaderEntry) []map[string]any {
	out := make([]map[string]any, len(items))
	for i, h := range items {
		out[i] = arvanCloudPageRuleHeaderEntryToMap(h)
	}
	return out
}

func arvanCloudPageRuleExceptionHeaderEntryToMap(h domain.ArvanCloudPageRuleExceptionHeaderEntry) map[string]any {
	return map[string]any{"name": h.Name, "value": h.Value, "is_var": h.IsVar}
}

func arvanCloudPageRuleExceptionHeaderEntriesToMaps(items []domain.ArvanCloudPageRuleExceptionHeaderEntry) []map[string]any {
	out := make([]map[string]any, len(items))
	for i, h := range items {
		out[i] = arvanCloudPageRuleExceptionHeaderEntryToMap(h)
	}
	return out
}

// arvanCloudPageRuleToMap renders a domain.ArvanCloudPageRule flat (using
// the wire's own flat field names), the way every page-rule-returning tool
// reports it back to the caller — the grouped Go/JSON-Schema shape is an
// input-side organizing device only (see this file's package comment); the
// result is easier to read flat.
func arvanCloudPageRuleToMap(pr domain.ArvanCloudPageRule) map[string]any {
	slinkMD5 := make([]string, len(pr.Security.SlinkMD5))
	for i, v := range pr.Security.SlinkMD5 {
		slinkMD5[i] = string(v)
	}
	return map[string]any{
		"id": pr.ID, "domain_id": pr.DomainID,
		"url_type": string(pr.Matching.URLType), "url": pr.Matching.URL, "seq": pr.Matching.Seq,
		"is_protected": pr.Matching.IsProtected, "status": pr.Matching.Status,
		"cache_level": string(pr.Caching.CacheLevel), "cache_200": string(pr.Caching.Cache200), "cache_any": string(pr.Caching.CacheAny),
		"cache_browser": string(pr.Caching.CacheBrowser), "cache_cookie": pr.Caching.CacheCookie,
		"cache_device_type": pr.Caching.CacheDeviceType, "cache_args": pr.Caching.CacheArgs, "cache_arg": pr.Caching.CacheArg,
		"cache_scheme": pr.Caching.CacheScheme, "cache_ignore_sc": pr.Caching.CacheIgnoreSC,
		"cache_ignore_vary": pr.Caching.CacheIgnoreVary, "cache_ignore_cc": pr.Caching.CacheIgnoreCC,
		"waf_status": pr.Security.WAFStatus, "fw_status": pr.Security.FWStatus, "slink_status": pr.Security.SlinkStatus,
		"slink_secret": pr.Security.SlinkSecret, "slink_md5": slinkMD5,
		"redirect": arvanCloudPageRuleRedirectToMap(pr.Routing.Redirect), "rewrite_url": pr.Routing.RewriteURL,
		"load_balancer": pr.Routing.LoadBalancer, "edge_compute_id": pr.Routing.EdgeComputeID,
		"cluster_status": pr.Routing.ClusterStatus, "cluster_id": pr.Routing.ClusterID,
		"cors_header": pr.Headers.CORSHeader, "req_custom_headers": arvanCloudPageRuleHeaderEntriesToMaps(pr.Headers.ReqCustomHeaders),
		"res_custom_headers": arvanCloudPageRuleHeaderEntriesToMaps(pr.Headers.ResCustomHeaders),
		"req_hide_headers":   pr.Headers.ReqHideHeaders, "res_hide_headers": pr.Headers.ResHideHeaders,
		"custom_host_header": pr.Headers.CustomHostHeader,
		"acceleration":       arvanCloudAccelerationToMap(pr.Other.Acceleration),
		"image_resize":       arvanCloudPageRuleImageResizeToMap(pr.Other.ImageResize),
		"upstream_timeout":   arvanCloudUpstreamTimeoutToMap(pr.Other.UpstreamTimeout),
		"created_at":         pr.CreatedAt, "updated_at": pr.UpdatedAt,
	}
}

func arvanCloudPageRuleSummaryToMap(pr domain.ArvanCloudPageRuleSummary) map[string]any {
	return map[string]any{
		"id": pr.ID, "domain_id": pr.DomainID, "seq": pr.Seq, "url_type": string(pr.URLType), "is_protected": pr.IsProtected,
		"url": pr.URL, "cache_level": string(pr.CacheLevel), "waf_status": pr.WAFStatus, "fw_status": pr.FWStatus,
		"acceleration": arvanCloudAccelerationToMap(pr.Acceleration), "slink_status": pr.SlinkStatus, "status": pr.Status,
		"created_at": pr.CreatedAt, "updated_at": pr.UpdatedAt,
	}
}

func arvanCloudPageRulePageMetaToMap(m domain.ArvanCloudPageRulePageMeta) map[string]any {
	return map[string]any{
		"current_page": m.CurrentPage, "from": m.From, "last_page": m.LastPage, "per_page": m.PerPage, "to": m.To, "total": m.Total,
	}
}

func arvanCloudPageRuleExceptionsToMap(e domain.ArvanCloudPageRuleExceptions) map[string]any {
	slinkMD5 := make([]string, len(e.SlinkMD5))
	for i, v := range e.SlinkMD5 {
		slinkMD5[i] = string(v)
	}
	return map[string]any{
		"url": e.URL, "cache_level": string(e.CacheLevel), "waf_status": e.WAFStatus, "fw_status": e.FWStatus,
		"acceleration": arvanCloudAccelerationToMap(e.Acceleration), "slink_status": e.SlinkStatus, "status": e.Status,
		"cache_200": string(e.Cache200), "cache_any": string(e.CacheAny), "cache_cookie": e.CacheCookie,
		"cache_args": e.CacheArgs, "cache_arg": e.CacheArg, "cache_scheme": e.CacheScheme, "cache_browser": string(e.CacheBrowser),
		"cache_ignore_sc": e.CacheIgnoreSC, "cache_ignore_vary": e.CacheIgnoreVary, "cache_ignore_cc": e.CacheIgnoreCC,
		"cors_header": e.CORSHeader, "rewrite_url": e.RewriteURL, "slink_secret": e.SlinkSecret, "slink_md5": slinkMD5,
		"cluster_status": e.ClusterStatus, "cluster_id": e.ClusterID, "edge_compute_id": e.EdgeComputeID,
		"upstream_timeout": arvanCloudUpstreamTimeoutToMap(e.UpstreamTimeout), "req_custom_headers": arvanCloudPageRuleExceptionHeaderEntriesToMaps(e.ReqCustomHeaders),
		"cache_device_type": e.CacheDeviceType, "image_resize": arvanCloudPageRuleImageResizeToMap(e.ImageResize),
		"load_balancer": e.LoadBalancer, "res_custom_headers": arvanCloudPageRuleExceptionHeaderEntriesToMaps(e.ResCustomHeaders),
		"req_hide_headers": e.ReqHideHeaders, "res_hide_headers": e.ResHideHeaders, "custom_host_header": e.CustomHostHeader,
		"redirect": arvanCloudPageRuleRedirectToMap(e.Redirect),
	}
}

// --- Input argument groups (mirroring the domain's own field grouping) ----

type arvanCloudPageRuleMatchingArgs struct {
	URLType string `json:"url_type"`
	URL     string `json:"url"`
	Seq     int    `json:"seq"`
	Status  *bool  `json:"status"`
}

type arvanCloudPageRuleCachingArgs struct {
	CacheLevel      string `json:"cache_level"`
	Cache200        string `json:"cache_200"`
	CacheAny        string `json:"cache_any"`
	CacheBrowser    string `json:"cache_browser"`
	CacheCookie     string `json:"cache_cookie"`
	CacheDeviceType bool   `json:"cache_device_type"`
	CacheArgs       *bool  `json:"cache_args"`
	CacheArg        string `json:"cache_arg"`
	CacheScheme     *bool  `json:"cache_scheme"`
	CacheIgnoreSC   bool   `json:"cache_ignore_sc"`
	CacheIgnoreVary *bool  `json:"cache_ignore_vary"`
	CacheIgnoreCC   *bool  `json:"cache_ignore_cc"`
}

type arvanCloudPageRuleSecurityArgs struct {
	WAFStatus   *bool    `json:"waf_status"`
	FWStatus    *bool    `json:"fw_status"`
	SlinkStatus bool     `json:"slink_status"`
	SlinkSecret string   `json:"slink_secret"`
	SlinkMD5    []string `json:"slink_md5"`
}

type arvanCloudPageRuleRedirectArgs struct {
	Enable     bool   `json:"enable"`
	StatusCode int    `json:"status_code"`
	URL        string `json:"url"`
}

func (a arvanCloudPageRuleRedirectArgs) toDomain() domain.ArvanCloudPageRuleRedirect {
	return domain.ArvanCloudPageRuleRedirect{Enable: a.Enable, StatusCode: a.StatusCode, URL: a.URL}
}

type arvanCloudPageRuleRoutingArgs struct {
	Redirect      arvanCloudPageRuleRedirectArgs `json:"redirect"`
	RewriteURL    string                         `json:"rewrite_url"`
	LoadBalancer  string                         `json:"load_balancer"`
	EdgeComputeID string                         `json:"edge_compute_id"`
	ClusterStatus bool                           `json:"cluster_status"`
	ClusterID     string                         `json:"cluster_id"`
}

type arvanCloudPageRuleHeaderEntryArgs struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (a arvanCloudPageRuleHeaderEntryArgs) toDomain() domain.ArvanCloudPageRuleHeaderEntry {
	return domain.ArvanCloudPageRuleHeaderEntry{Name: a.Name, Value: a.Value}
}

func arvanCloudPageRuleHeaderEntryArgsToDomain(items []arvanCloudPageRuleHeaderEntryArgs) []domain.ArvanCloudPageRuleHeaderEntry {
	out := make([]domain.ArvanCloudPageRuleHeaderEntry, len(items))
	for i, h := range items {
		out[i] = h.toDomain()
	}
	return out
}

type arvanCloudPageRuleHeadersArgs struct {
	CORSHeader       string                              `json:"cors_header"`
	ReqCustomHeaders []arvanCloudPageRuleHeaderEntryArgs `json:"req_custom_headers"`
	ResCustomHeaders []arvanCloudPageRuleHeaderEntryArgs `json:"res_custom_headers"`
	ReqHideHeaders   []string                            `json:"req_hide_headers"`
	ResHideHeaders   []string                            `json:"res_hide_headers"`
	CustomHostHeader string                              `json:"custom_host_header"`
}

type arvanCloudAccelerationArgs struct {
	Status     string   `json:"status"`
	Extensions []string `json:"extensions"`
}

func (a arvanCloudAccelerationArgs) toDomain() domain.ArvanCloudAccelerationSettings {
	extensions := make([]domain.ArvanCloudAccelerationExtension, len(a.Extensions))
	for i, e := range a.Extensions {
		extensions[i] = domain.ArvanCloudAccelerationExtension(e)
	}
	return domain.ArvanCloudAccelerationSettings{Status: domain.ArvanCloudAccelerationStatus(a.Status), Extensions: extensions}
}

type arvanCloudPageRuleImageResizeArgs struct {
	Status   string `json:"status"`
	HeightBy string `json:"height_by"`
	WidthBy  string `json:"width_by"`
	Mode     string `json:"mode"`
}

func (a arvanCloudPageRuleImageResizeArgs) toDomain() domain.ArvanCloudPageRuleImageResize {
	return domain.ArvanCloudPageRuleImageResize{
		Status: domain.ArvanCloudPageRuleImageResizeStatus(a.Status), HeightBy: a.HeightBy, WidthBy: a.WidthBy,
		Mode: domain.ArvanCloudImageResizeMode(a.Mode),
	}
}

type arvanCloudUpstreamTimeoutArgs struct {
	ConnectTimeoutSeconds int `json:"connect_timeout_seconds"`
	ReadTimeoutSeconds    int `json:"read_timeout_seconds"`
	SendTimeoutSeconds    int `json:"send_timeout_seconds"`
}

func (a arvanCloudUpstreamTimeoutArgs) toDomain() domain.ArvanCloudUpstreamTimeout {
	return domain.ArvanCloudUpstreamTimeout{
		ConnectTimeoutSeconds: a.ConnectTimeoutSeconds, ReadTimeoutSeconds: a.ReadTimeoutSeconds, SendTimeoutSeconds: a.SendTimeoutSeconds,
	}
}

type arvanCloudPageRuleOtherArgs struct {
	Acceleration    arvanCloudAccelerationArgs        `json:"acceleration"`
	ImageResize     arvanCloudPageRuleImageResizeArgs `json:"image_resize"`
	UpstreamTimeout arvanCloudUpstreamTimeoutArgs     `json:"upstream_timeout"`
}

// arvanCloudPageRuleArgs decodes create/update_arvancloud_page_rule's
// grouped input (see this file's package comment on the schema design).
type arvanCloudPageRuleArgs struct {
	arvanCloudDomainNameArgs
	Matching arvanCloudPageRuleMatchingArgs `json:"matching"`
	Caching  arvanCloudPageRuleCachingArgs  `json:"caching"`
	Security arvanCloudPageRuleSecurityArgs `json:"security"`
	Routing  arvanCloudPageRuleRoutingArgs  `json:"routing"`
	Headers  arvanCloudPageRuleHeadersArgs  `json:"headers"`
	Other    arvanCloudPageRuleOtherArgs    `json:"other"`
}

func (a arvanCloudPageRuleArgs) toDomain() domain.ArvanCloudPageRule {
	status := true
	if a.Matching.Status != nil {
		status = *a.Matching.Status
	}
	wafStatus := true
	if a.Security.WAFStatus != nil {
		wafStatus = *a.Security.WAFStatus
	}
	fwStatus := true
	if a.Security.FWStatus != nil {
		fwStatus = *a.Security.FWStatus
	}
	cacheArgs := true
	if a.Caching.CacheArgs != nil {
		cacheArgs = *a.Caching.CacheArgs
	}
	cacheScheme := true
	if a.Caching.CacheScheme != nil {
		cacheScheme = *a.Caching.CacheScheme
	}
	cacheIgnoreVary := true
	if a.Caching.CacheIgnoreVary != nil {
		cacheIgnoreVary = *a.Caching.CacheIgnoreVary
	}
	cacheIgnoreCC := true
	if a.Caching.CacheIgnoreCC != nil {
		cacheIgnoreCC = *a.Caching.CacheIgnoreCC
	}
	slinkMD5 := make([]domain.ArvanCloudPageRuleSlinkMD5Field, len(a.Security.SlinkMD5))
	for i, v := range a.Security.SlinkMD5 {
		slinkMD5[i] = domain.ArvanCloudPageRuleSlinkMD5Field(v)
	}

	return domain.ArvanCloudPageRule{
		Matching: domain.ArvanCloudPageRuleMatching{
			URLType: domain.ArvanCloudPageRuleURLType(a.Matching.URLType), URL: a.Matching.URL, Seq: a.Matching.Seq, Status: status,
		},
		Caching: domain.ArvanCloudPageRuleCaching{
			CacheLevel: domain.ArvanCloudPageRuleCacheLevel(a.Caching.CacheLevel),
			Cache200:   domain.ArvanCloudCacheTTL(a.Caching.Cache200), CacheAny: domain.ArvanCloudCacheTTL(a.Caching.CacheAny),
			CacheBrowser: domain.ArvanCloudCacheTTL(a.Caching.CacheBrowser), CacheCookie: a.Caching.CacheCookie,
			CacheDeviceType: a.Caching.CacheDeviceType, CacheArgs: cacheArgs, CacheArg: a.Caching.CacheArg,
			CacheScheme: cacheScheme, CacheIgnoreSC: a.Caching.CacheIgnoreSC, CacheIgnoreVary: cacheIgnoreVary, CacheIgnoreCC: cacheIgnoreCC,
		},
		Security: domain.ArvanCloudPageRuleSecurity{
			WAFStatus: wafStatus, FWStatus: fwStatus, SlinkStatus: a.Security.SlinkStatus,
			SlinkSecret: a.Security.SlinkSecret, SlinkMD5: slinkMD5,
		},
		Routing: domain.ArvanCloudPageRuleRouting{
			Redirect: a.Routing.Redirect.toDomain(), RewriteURL: a.Routing.RewriteURL, LoadBalancer: a.Routing.LoadBalancer,
			EdgeComputeID: a.Routing.EdgeComputeID, ClusterStatus: a.Routing.ClusterStatus, ClusterID: a.Routing.ClusterID,
		},
		Headers: domain.ArvanCloudPageRuleHeaders{
			CORSHeader: a.Headers.CORSHeader, ReqCustomHeaders: arvanCloudPageRuleHeaderEntryArgsToDomain(a.Headers.ReqCustomHeaders),
			ResCustomHeaders: arvanCloudPageRuleHeaderEntryArgsToDomain(a.Headers.ResCustomHeaders),
			ReqHideHeaders:   a.Headers.ReqHideHeaders, ResHideHeaders: a.Headers.ResHideHeaders, CustomHostHeader: a.Headers.CustomHostHeader,
		},
		Other: domain.ArvanCloudPageRuleOther{
			Acceleration: a.Other.Acceleration.toDomain(), ImageResize: a.Other.ImageResize.toDomain(),
			UpstreamTimeout: a.Other.UpstreamTimeout.toDomain(),
		},
	}
}

// --- JSON Schema property builders -----------------------------------------

func arvanCloudPageRuleIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The page rule's provider-assigned ID (a UUID), as returned by create_arvancloud_page_rule or list_arvancloud_page_rules.",
	}
}

func arvanCloudCacheTTLDescription(extra string) string {
	base := "One of the fixed TTL strings ArvanCloud accepts: \"0s\",\"1s\",...,\"10s\",\"30s\",\"1m\",\"3m\",\"5m\",\"10m\",\"30m\"," +
		"\"45m\",\"1h\",\"3h\",\"5h\",\"10h\",\"12h\",\"24h\",\"3d\",\"7d\",\"10d\",\"15d\",\"30d\""
	if extra != "" {
		base += extra
	}
	return base + ". Example: \"1h\" for one hour."
}

func arvanCloudPageRuleMatchingProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Which requests this rule applies to, and whether it is active.",
		"properties": map[string]any{
			"url_type": map[string]any{
				"type": "string", "enum": []string{"default", "index", "directory", "extension", "page", "regex"},
				"description": "Deprecated by ArvanCloud in favor of matching on \"url\" alone, but still accepted. Defaults to \"default\".",
			},
			"url":    map[string]any{"type": "string", "description": "REQUIRED. The URL match pattern, e.g. \"/api/*\". Its exact shape depends on url_type."},
			"seq":    map[string]any{"type": "integer", "description": "This rule's priority/order among the domain's page rules; lower values are evaluated first."},
			"status": map[string]any{"type": "boolean", "description": "Whether the rule is active. Defaults to true when omitted."},
		},
		"required": []string{"url"},
	}
}

func arvanCloudPageRuleCachingProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Caching behavior for requests matching this rule.",
		"properties": map[string]any{
			"cache_level": map[string]any{"type": "string", "enum": []string{"off", "uri", "query_string"}, "description": "Defaults to \"query_string\" when omitted."},
			"cache_200":   map[string]any{"type": "string", "description": "How long to cache a 200 response. " + arvanCloudCacheTTLDescription("") + " Defaults to \"30m\" when omitted."},
			"cache_any":   map[string]any{"type": "string", "description": "How long to cache any response. " + arvanCloudCacheTTLDescription("") + " Defaults to \"0s\" when omitted."},
			"cache_browser": map[string]any{
				"type": "string", "description": "The browser-facing Cache-Control TTL. " +
					arvanCloudCacheTTLDescription(", plus the extra value \"default\" (defer to ArvanCloud's own default)") + " Defaults to \"default\" when omitted.",
			},
			"cache_cookie":      map[string]any{"type": "string", "description": "Comma-separated cookie names to vary the cache on."},
			"cache_device_type": map[string]any{"type": "boolean", "description": "Vary the cache by device type. Defaults to false."},
			"cache_args":        map[string]any{"type": "boolean", "description": "Vary the cache by query-string arguments. Defaults to true."},
			"cache_arg":         map[string]any{"type": "string", "description": "\"&\"-separated query-string argument names to vary the cache on, e.g. \"filter&sort\"."},
			"cache_scheme":      map[string]any{"type": "boolean", "description": "Deprecated by ArvanCloud but still accepted. Defaults to true."},
			"cache_ignore_sc":   map[string]any{"type": "boolean", "description": "Ignore the default Set-Cookie-header caching behavior. Defaults to false."},
			"cache_ignore_vary": map[string]any{"type": "boolean", "description": "Ignore the default Vary-header caching behavior. Defaults to true."},
			"cache_ignore_cc":   map[string]any{"type": "boolean", "description": "Defaults to true."},
		},
	}
}

func arvanCloudPageRuleSecurityProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "WAF/firewall/secure-link toggles for requests matching this rule.",
		"properties": map[string]any{
			"waf_status":   map[string]any{"type": "boolean", "description": "Turn the Web Application Firewall on for matching requests. Defaults to true."},
			"fw_status":    map[string]any{"type": "boolean", "description": "Deprecated by ArvanCloud but still accepted. Defaults to true."},
			"slink_status": map[string]any{"type": "boolean", "description": "Turn on secure-link (token-authenticated URLs) for this rule. Defaults to false."},
			"slink_secret": map[string]any{"type": "string", "description": "Shared secret used to compute the secure-link token. Only meaningful when slink_status is true."},
			"slink_md5": map[string]any{
				"type": "array", "items": map[string]any{"type": "string", "enum": []string{"remote_addr", "file", "expires", "url", "uri"}},
				"description": "Which request attributes are hashed into the secure-link token. Defaults to [\"remote_addr\",\"file\",\"expires\"] when omitted.",
			},
		},
	}
}

func arvanCloudPageRuleRoutingProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Redirect, rewrite, and re-routing behavior for requests matching this rule.",
		"properties": map[string]any{
			"redirect": map[string]any{
				"type":        "object",
				"description": "A rule-scoped HTTP redirect. Cannot be combined with rewrite_url or a secure link.",
				"properties": map[string]any{
					"enable":      map[string]any{"type": "boolean", "description": "Turn the redirect on. Defaults to false."},
					"status_code": map[string]any{"type": "integer", "enum": []int{301, 302, 307}, "description": "Defaults to 301 when omitted."},
					"url":         map[string]any{"type": "string", "description": "The redirect target URL."},
				},
			},
			"rewrite_url":     map[string]any{"type": "string", "description": "Rewrite the request path before it reaches the origin. Cannot be combined with redirect or a secure link."},
			"load_balancer":   map[string]any{"type": "string", "description": "Name or ID of a Load Balancing resource (create_arvancloud_load_balancer) to route matching traffic to."},
			"edge_compute_id": map[string]any{"type": "string", "description": "UUID of an edge compute function to route matching traffic to. Marked alpha by ArvanCloud."},
			"cluster_status":  map[string]any{"type": "boolean", "description": "Deprecated by ArvanCloud but still accepted."},
			"cluster_id":      map[string]any{"type": "string", "description": "Deprecated by ArvanCloud but still accepted."},
		},
	}
}

func arvanCloudPageRuleHeaderEntryProperty() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string", "description": "REQUIRED. The header name, e.g. \"X-Custom-Header\"."},
			"value": map[string]any{"type": "string", "description": "REQUIRED. The header value."},
		},
	}
}

func arvanCloudPageRuleHeadersProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Request/response header manipulation for requests matching this rule.",
		"properties": map[string]any{
			"cors_header":        map[string]any{"type": "string", "description": "The CORS response header value this rule applies. Omit or \"-\" for none."},
			"req_custom_headers": map[string]any{"type": "array", "items": arvanCloudPageRuleHeaderEntryProperty(), "description": "Headers to add to the request sent upstream."},
			"res_custom_headers": map[string]any{"type": "array", "items": arvanCloudPageRuleHeaderEntryProperty(), "description": "Headers to add to the response sent to the client."},
			"req_hide_headers":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Header names to strip from the request sent upstream."},
			"res_hide_headers":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Header names to strip from the response sent to the client."},
			"custom_host_header": map[string]any{"type": "string", "format": "hostname", "description": "Override the Host header sent to the origin for matching requests."},
		},
	}
}

func arvanCloudAccelerationProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "This rule's acceleration override.",
		"properties": map[string]any{
			"status": map[string]any{"type": "string", "enum": []string{"inherit", "on", "off"}, "description": "\"inherit\" defers to the domain's own acceleration setting."},
			"extensions": map[string]any{
				"type": "array", "items": map[string]any{"type": "string", "enum": []string{"css", "gif", "jpeg", "js", "png"}},
				"description": "File extensions acceleration applies to, e.g. [\"css\",\"js\"].",
			},
		},
	}
}

func arvanCloudPageRuleImageResizeProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "This rule's image-resize override.",
		"properties": map[string]any{
			"status":    map[string]any{"type": "string", "enum": []string{"on", "off", "inherit"}, "description": "Defaults to \"off\" when omitted."},
			"height_by": map[string]any{"type": "string", "description": "Query-string argument read for the target height, e.g. \"height\" (the default)."},
			"width_by":  map[string]any{"type": "string", "description": "Query-string argument read for the target width, e.g. \"width\" (the default)."},
			"mode":      map[string]any{"type": "string", "enum": []string{"freely", "short-side", "long-side"}},
		},
	}
}

func arvanCloudUpstreamTimeoutProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "This rule's upstream connect/read/send timeouts, all in SECONDS.",
		"properties": map[string]any{
			"connect_timeout_seconds": map[string]any{"type": "integer", "description": "Defaults to 30 seconds when omitted."},
			"read_timeout_seconds":    map[string]any{"type": "integer", "description": "Defaults to 100 seconds when omitted."},
			"send_timeout_seconds":    map[string]any{"type": "integer", "description": "Defaults to 300 seconds when omitted."},
		},
	}
}

func arvanCloudPageRuleOtherProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Acceleration, image-resize and upstream-timeout overrides that do not fit the other groups.",
		"properties": map[string]any{
			"acceleration":     arvanCloudAccelerationProperty(),
			"image_resize":     arvanCloudPageRuleImageResizeProperty(),
			"upstream_timeout": arvanCloudUpstreamTimeoutProperty(),
		},
	}
}

// arvanCloudPageRuleGroupProperties adds the six grouped sub-objects shared
// by create/update_arvancloud_page_rule to props (see this file's package
// comment on the schema design).
func arvanCloudPageRuleGroupProperties(props map[string]any) {
	props["matching"] = arvanCloudPageRuleMatchingProperty()
	props["caching"] = arvanCloudPageRuleCachingProperty()
	props["security"] = arvanCloudPageRuleSecurityProperty()
	props["routing"] = arvanCloudPageRuleRoutingProperty()
	props["headers"] = arvanCloudPageRuleHeadersProperty()
	props["other"] = arvanCloudPageRuleOtherProperty()
}

// --- Page Rules -------------------------------------------------------

func listArvanCloudPageRulesTool(uc *app.ListArvanCloudPageRules) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["search"] = map[string]any{"type": "string", "description": "Filter by a free-text search term."}
	props["per_page"] = map[string]any{"type": "integer", "description": "How many rules per page. Omit for ArvanCloud's own default."}
	props["page"] = map[string]any{"type": "integer", "description": "Which page to return, 1-indexed. Omit for page 1."}
	props["order"] = map[string]any{"type": "string", "enum": []string{"asc", "desc"}, "description": "Sort by seq. Defaults to \"desc\" when omitted."}

	return Tool{
		Name:        "list_arvancloud_page_rules",
		Description: "List a domain's edge rewrite/routing page rules, in summary form. This is a fast operation.",
		InputSchema: map[string]any{
			"type": "object", "properties": props, "required": []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Search  string `json:"search"`
				PerPage int    `json:"per_page"`
				Page    int    `json:"page"`
				Order   string `json:"order"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			result, err := uc.Execute(ctx, app.ListArvanCloudPageRulesInput{
				Credentials: args.domain(), Domain: args.Domain,
				Query: domain.ArvanCloudPageRuleListQuery{Search: args.Search, PerPage: args.PerPage, Page: args.Page, Order: args.Order},
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(result.Rules))
			for i, r := range result.Rules {
				out[i] = arvanCloudPageRuleSummaryToMap(r)
			}
			return map[string]any{"page_rules": out, "page": arvanCloudPageRulePageMetaToMap(result.Page)}, nil
		},
	}
}

func createArvanCloudPageRuleTool(uc *app.CreateArvanCloudPageRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudPageRuleGroupProperties(props)

	return Tool{
		Name: "create_arvancloud_page_rule",
		Description: "Create a new edge rewrite/routing page rule for a domain. Fields are grouped into matching " +
			"(which requests this rule applies to), caching, security (WAF/secure-link), routing (redirect/rewrite/" +
			"load balancer), headers, and other (acceleration/image-resize/upstream-timeout) — pass only the groups " +
			"you need; every field within a group is itself optional except matching.url. This is a fast operation: " +
			"the created rule, including its provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type": "object", "properties": props, "required": []string{"api_key", "domain", "matching"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudPageRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudPageRuleInput{Credentials: args.domain(), Domain: args.Domain, Rule: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return arvanCloudPageRuleToMap(*created), nil
		},
	}
}

func getArvanCloudPageRuleTool(uc *app.GetArvanCloudPageRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudPageRuleIDProperty()

	return Tool{
		Name:        "get_arvancloud_page_rule",
		Description: "Get the current state of one page rule by ID. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "id"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudPageRuleInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudPageRuleToMap(*found), nil
		},
	}
}

func updateArvanCloudPageRuleTool(uc *app.UpdateArvanCloudPageRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudPageRuleIDProperty()
	arvanCloudPageRuleGroupProperties(props)

	return Tool{
		Name: "update_arvancloud_page_rule",
		Description: "Replace a page rule's fields. This is a full replace, the same grouped schema as " +
			"create_arvancloud_page_rule — pass every group/field you want to keep, not only the ones changing. " +
			"This is a fast operation.",
		InputSchema: map[string]any{
			"type": "object", "properties": props, "required": []string{"api_key", "domain", "id", "matching"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudPageRuleArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudPageRuleInput{
				Credentials: args.domain(), Domain: args.Domain, ID: args.ID, Rule: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudPageRuleToMap(*updated), nil
		},
	}
}

func setArvanCloudPageRuleStatusTool(uc *app.SetArvanCloudPageRuleStatus) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudPageRuleIDProperty()
	props["status"] = map[string]any{"type": "boolean", "description": "REQUIRED. Whether the rule should be active."}

	return Tool{
		Name: "set_arvancloud_page_rule_status",
		Description: "Toggle ONLY a page rule's active/inactive status — a narrower, separate operation from " +
			"update_arvancloud_page_rule's full replace. This is a fast operation; call get_arvancloud_page_rule " +
			"afterward for the rule's full state.",
		InputSchema: map[string]any{
			"type": "object", "properties": props, "required": []string{"api_key", "domain", "id", "status"},
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

			if err := uc.Execute(ctx, app.SetArvanCloudPageRuleStatusInput{
				Credentials: args.domain(), Domain: args.Domain, ID: args.ID, Status: args.Status,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"domain": args.Domain, "id": args.ID, "status": args.Status}, nil
		},
	}
}

func deleteArvanCloudPageRuleTool(uc *app.DeleteArvanCloudPageRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudPageRuleIDProperty()

	return Tool{
		Name: "delete_arvancloud_page_rule",
		Description: "Permanently delete a page rule by ID. This is a fast operation and cannot be undone. " +
			"Deleting a rule that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "id"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudPageRuleInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

func purgeArvanCloudPageRuleCacheTool(uc *app.PurgeArvanCloudPageRuleCache) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudPageRuleIDProperty()

	return Tool{
		Name:        "purge_arvancloud_page_rule_cache",
		Description: "Purge cached content for every URL matching this page rule. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "id"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.PurgeArvanCloudPageRuleCacheInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"purged": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

// --- Page Rule exceptions ("diff") -----------------------------------------

type arvanCloudPageRuleExceptionHeaderEntryArgs struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	IsVar string `json:"is_var"`
}

func (a arvanCloudPageRuleExceptionHeaderEntryArgs) toDomain() domain.ArvanCloudPageRuleExceptionHeaderEntry {
	return domain.ArvanCloudPageRuleExceptionHeaderEntry{Name: a.Name, Value: a.Value, IsVar: a.IsVar}
}

func arvanCloudPageRuleExceptionHeaderEntryArgsToDomain(items []arvanCloudPageRuleExceptionHeaderEntryArgs) []domain.ArvanCloudPageRuleExceptionHeaderEntry {
	out := make([]domain.ArvanCloudPageRuleExceptionHeaderEntry, len(items))
	for i, h := range items {
		out[i] = h.toDomain()
	}
	return out
}

// arvanCloudPageRuleExceptionsArgs decodes
// update_arvancloud_page_rule_exceptions' input. Deliberately flat (unlike
// arvanCloudPageRuleArgs' grouped shape) since PageRuleDiff is already a
// smaller, sparse-override object — see
// domain.ArvanCloudPageRuleExceptions' own doc comment on why only non-zero
// fields here become overrides.
type arvanCloudPageRuleExceptionsArgs struct {
	arvanCloudDomainNameArgs
	ID               string                                       `json:"id"`
	URL              string                                       `json:"url"`
	CacheLevel       string                                       `json:"cache_level"`
	WAFStatus        bool                                         `json:"waf_status"`
	FWStatus         bool                                         `json:"fw_status"`
	Acceleration     arvanCloudAccelerationArgs                   `json:"acceleration"`
	SlinkStatus      bool                                         `json:"slink_status"`
	Status           bool                                         `json:"status"`
	Cache200         string                                       `json:"cache_200"`
	CacheAny         string                                       `json:"cache_any"`
	CacheCookie      string                                       `json:"cache_cookie"`
	CacheArgs        bool                                         `json:"cache_args"`
	CacheArg         string                                       `json:"cache_arg"`
	CacheScheme      bool                                         `json:"cache_scheme"`
	CacheBrowser     string                                       `json:"cache_browser"`
	CacheIgnoreSC    bool                                         `json:"cache_ignore_sc"`
	CacheIgnoreVary  bool                                         `json:"cache_ignore_vary"`
	CacheIgnoreCC    bool                                         `json:"cache_ignore_cc"`
	CORSHeader       string                                       `json:"cors_header"`
	RewriteURL       string                                       `json:"rewrite_url"`
	SlinkSecret      string                                       `json:"slink_secret"`
	SlinkMD5         []string                                     `json:"slink_md5"`
	ClusterStatus    bool                                         `json:"cluster_status"`
	ClusterID        string                                       `json:"cluster_id"`
	EdgeComputeID    string                                       `json:"edge_compute_id"`
	UpstreamTimeout  arvanCloudUpstreamTimeoutArgs                `json:"upstream_timeout"`
	ReqCustomHeaders []arvanCloudPageRuleExceptionHeaderEntryArgs `json:"req_custom_headers"`
	CacheDeviceType  bool                                         `json:"cache_device_type"`
	ImageResize      arvanCloudPageRuleImageResizeArgs            `json:"image_resize"`
	LoadBalancer     string                                       `json:"load_balancer"`
	ResCustomHeaders []arvanCloudPageRuleExceptionHeaderEntryArgs `json:"res_custom_headers"`
	ReqHideHeaders   []string                                     `json:"req_hide_headers"`
	ResHideHeaders   []string                                     `json:"res_hide_headers"`
	CustomHostHeader string                                       `json:"custom_host_header"`
	Redirect         arvanCloudPageRuleRedirectArgs               `json:"redirect"`
}

func (a arvanCloudPageRuleExceptionsArgs) toDomain() domain.ArvanCloudPageRuleExceptions {
	slinkMD5 := make([]domain.ArvanCloudPageRuleSlinkMD5Field, len(a.SlinkMD5))
	for i, v := range a.SlinkMD5 {
		slinkMD5[i] = domain.ArvanCloudPageRuleSlinkMD5Field(v)
	}
	return domain.ArvanCloudPageRuleExceptions{
		URL: a.URL, CacheLevel: domain.ArvanCloudPageRuleCacheLevel(a.CacheLevel), WAFStatus: a.WAFStatus, FWStatus: a.FWStatus,
		Acceleration: a.Acceleration.toDomain(), SlinkStatus: a.SlinkStatus, Status: a.Status,
		Cache200: domain.ArvanCloudCacheTTL(a.Cache200), CacheAny: domain.ArvanCloudCacheTTL(a.CacheAny), CacheCookie: a.CacheCookie,
		CacheArgs: a.CacheArgs, CacheArg: a.CacheArg, CacheScheme: a.CacheScheme, CacheBrowser: domain.ArvanCloudCacheTTL(a.CacheBrowser),
		CacheIgnoreSC: a.CacheIgnoreSC, CacheIgnoreVary: a.CacheIgnoreVary, CacheIgnoreCC: a.CacheIgnoreCC,
		CORSHeader: a.CORSHeader, RewriteURL: a.RewriteURL, SlinkSecret: a.SlinkSecret, SlinkMD5: slinkMD5,
		ClusterStatus: a.ClusterStatus, ClusterID: a.ClusterID, EdgeComputeID: a.EdgeComputeID,
		UpstreamTimeout: a.UpstreamTimeout.toDomain(), ReqCustomHeaders: arvanCloudPageRuleExceptionHeaderEntryArgsToDomain(a.ReqCustomHeaders),
		CacheDeviceType: a.CacheDeviceType, ImageResize: a.ImageResize.toDomain(), LoadBalancer: a.LoadBalancer,
		ResCustomHeaders: arvanCloudPageRuleExceptionHeaderEntryArgsToDomain(a.ResCustomHeaders),
		ReqHideHeaders:   a.ReqHideHeaders, ResHideHeaders: a.ResHideHeaders, CustomHostHeader: a.CustomHostHeader,
		Redirect: a.Redirect.toDomain(),
	}
}

func getArvanCloudPageRuleExceptionsTool(uc *app.GetArvanCloudPageRuleExceptions) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudPageRuleIDProperty()

	return Tool{
		Name: "get_arvancloud_page_rule_exceptions",
		Description: "Get a page rule's \"exceptions\" — an override layer on top of the rule's own fields. " +
			"This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "id"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudPageRuleExceptionsInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudPageRuleExceptionsToMap(*found), nil
		},
	}
}

func updateArvanCloudPageRuleExceptionsTool(uc *app.UpdateArvanCloudPageRuleExceptions) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudPageRuleIDProperty()
	props["url"] = map[string]any{"type": "string", "description": "Override the rule's URL match pattern."}
	props["cache_level"] = map[string]any{"type": "string", "enum": []string{"off", "uri", "query_string"}}
	props["waf_status"] = map[string]any{"type": "boolean"}
	props["fw_status"] = map[string]any{"type": "boolean", "description": "Deprecated by ArvanCloud but still accepted."}
	props["acceleration"] = arvanCloudAccelerationProperty()
	props["slink_status"] = map[string]any{"type": "boolean"}
	props["status"] = map[string]any{"type": "boolean"}
	props["cache_200"] = map[string]any{"type": "string", "description": arvanCloudCacheTTLDescription("")}
	props["cache_any"] = map[string]any{"type": "string", "description": arvanCloudCacheTTLDescription("")}
	props["cache_cookie"] = map[string]any{"type": "string"}
	props["cache_args"] = map[string]any{"type": "boolean"}
	props["cache_arg"] = map[string]any{"type": "string"}
	props["cache_scheme"] = map[string]any{"type": "boolean", "description": "Deprecated by ArvanCloud but still accepted."}
	props["cache_browser"] = map[string]any{"type": "string", "description": arvanCloudCacheTTLDescription(", plus \"default\"")}
	props["cache_ignore_sc"] = map[string]any{"type": "boolean"}
	props["cache_ignore_vary"] = map[string]any{"type": "boolean"}
	props["cache_ignore_cc"] = map[string]any{"type": "boolean"}
	props["cors_header"] = map[string]any{"type": "string"}
	props["rewrite_url"] = map[string]any{"type": "string"}
	props["slink_secret"] = map[string]any{"type": "string"}
	props["slink_md5"] = map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"remote_addr", "file", "expires", "url", "uri"}}}
	props["cluster_status"] = map[string]any{"type": "boolean", "description": "Deprecated by ArvanCloud but still accepted."}
	props["cluster_id"] = map[string]any{"type": "string", "description": "Deprecated by ArvanCloud but still accepted."}
	props["edge_compute_id"] = map[string]any{"type": "string"}
	props["upstream_timeout"] = arvanCloudUpstreamTimeoutProperty()
	props["req_custom_headers"] = map[string]any{
		"type": "array", "description": "Headers to add to the request sent upstream.",
		"items": map[string]any{
			"type": "object", "properties": map[string]any{
				"name": map[string]any{"type": "string"}, "value": map[string]any{"type": "string"},
				"is_var": map[string]any{"type": "string", "enum": []string{"true", "false"}, "description": "ArvanCloud's own string-typed boolean; leave empty to omit."},
			},
		},
	}
	props["cache_device_type"] = map[string]any{"type": "boolean"}
	props["image_resize"] = arvanCloudPageRuleImageResizeProperty()
	props["load_balancer"] = map[string]any{"type": "string"}
	props["res_custom_headers"] = map[string]any{
		"type": "array", "description": "Headers to add to the response sent to the client.",
		"items": map[string]any{
			"type": "object", "properties": map[string]any{
				"name": map[string]any{"type": "string"}, "value": map[string]any{"type": "string"},
				"is_var": map[string]any{"type": "string", "enum": []string{"true", "false"}},
			},
		},
	}
	props["req_hide_headers"] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	props["res_hide_headers"] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	props["custom_host_header"] = map[string]any{"type": "string", "format": "hostname"}
	props["redirect"] = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"enable": map[string]any{"type": "boolean"}, "status_code": map[string]any{"type": "integer", "enum": []int{301, 302, 307}},
			"url": map[string]any{"type": "string"},
		},
	}

	return Tool{
		Name: "update_arvancloud_page_rule_exceptions",
		Description: "Set a page rule's \"exceptions\" — an override layer on top of the rule's own fields. Only " +
			"fields given a non-default (non-empty/non-zero/true) value become overrides; a field left at its " +
			"default is treated as \"no override for this field\", not as an explicit false/empty override — " +
			"there is currently no way to override a boolean field to false through this tool. This is a fast " +
			"operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "id"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudPageRuleExceptionsArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudPageRuleExceptionsInput{
				Credentials: args.domain(), Domain: args.Domain, ID: args.ID, Exceptions: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudPageRuleExceptionsToMap(*updated), nil
		},
	}
}

// --- Response Transforms ---------------------------------------------------

func arvanCloudResponseTransformIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The response transform's provider-assigned ID (a UUID), as returned by create_arvancloud_response_transform or list_arvancloud_response_transforms.",
	}
}

func arvanCloudResponseTransformToMap(rt domain.ArvanCloudResponseTransform) map[string]any {
	steps := make([]map[string]any, len(rt.Transforms))
	for i, s := range rt.Transforms {
		actions := make([]map[string]any, len(s.Actions))
		for j, a := range s.Actions {
			actions[j] = map[string]any{"type": string(a.Type), "mode": string(a.Mode), "key": string(a.Key), "value": a.Value}
		}
		steps[i] = map[string]any{"condition": s.Condition, "actions": actions}
	}
	return map[string]any{
		"id": rt.ID, "name": rt.Name, "description": rt.Description, "transforms": steps,
		"created_at": rt.CreatedAt, "updated_at": rt.UpdatedAt,
	}
}

type arvanCloudResponseTransformActionArgs struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type arvanCloudResponseTransformStepArgs struct {
	Condition string                                  `json:"condition"`
	Actions   []arvanCloudResponseTransformActionArgs `json:"actions"`
}

type arvanCloudResponseTransformArgs struct {
	arvanCloudDomainNameArgs
	Name        string                                `json:"name"`
	Description string                                `json:"description"`
	Transforms  []arvanCloudResponseTransformStepArgs `json:"transforms"`
}

func (a arvanCloudResponseTransformArgs) toDomain() domain.ArvanCloudResponseTransform {
	steps := make([]domain.ArvanCloudResponseTransformStep, len(a.Transforms))
	for i, s := range a.Transforms {
		actions := make([]domain.ArvanCloudResponseTransformAction, len(s.Actions))
		for j, act := range s.Actions {
			actions[j] = domain.ArvanCloudResponseTransformAction{
				Type: domain.ArvanCloudResponseTransformActionAddHeader, Mode: domain.ArvanCloudResponseTransformModeSet,
				Key: domain.ArvanCloudResponseTransformHeaderKey(act.Key), Value: act.Value,
			}
		}
		steps[i] = domain.ArvanCloudResponseTransformStep{Condition: s.Condition, Actions: actions}
	}
	return domain.ArvanCloudResponseTransform{Name: a.Name, Description: a.Description, Transforms: steps}
}

func arvanCloudResponseTransformActionProperty() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{
				"type": "string", "enum": []string{
					"Access-Control-Allow-Origin", "Access-Control-Expose-Headers", "Access-Control-Max-Age",
					"Access-Control-Allow-Credentials", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers",
				},
				"description": "REQUIRED. The CORS response header this action sets.",
			},
			"value": map[string]any{
				"type": "string",
				"description": "REQUIRED. Must resolve to a string. Allowed forms: http.request.headers[\"origin\"], " +
					"http.request.headers[\"host\"], or to_string(\"literal\"), e.g. to_string(\"*\").",
			},
		},
		"required": []string{"key", "value"},
	}
}

func arvanCloudResponseTransformStepsProperty() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"condition": map[string]any{
					"type": "string",
					"description": "REQUIRED. A Wireshark-like filter expression, 3-5000 characters, e.g. " +
						"\"http.request.method == \\\"GET\\\"\". Validated against a plan-based field allowlist.",
				},
				"actions": map[string]any{
					"type": "array", "items": arvanCloudResponseTransformActionProperty(),
					"description": "REQUIRED, at least one. Actions applied in order when condition matches.",
				},
			},
			"required": []string{"condition", "actions"},
		},
		"description": "REQUIRED, at least one step.",
	}
}

func listArvanCloudResponseTransformsTool(uc *app.ListArvanCloudResponseTransforms) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["name"] = map[string]any{"type": "string", "description": "Filter by a substring match against the preset name."}
	props["per_page"] = map[string]any{"type": "integer", "description": "How many presets per page. Omit for ArvanCloud's own default."}
	props["page"] = map[string]any{"type": "integer", "description": "Which page to return, 1-indexed. Omit for page 1."}

	return Tool{
		Name:        "list_arvancloud_response_transforms",
		Description: "List a domain's response-transform presets. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Name    string `json:"name"`
				PerPage int    `json:"per_page"`
				Page    int    `json:"page"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			result, err := uc.Execute(ctx, app.ListArvanCloudResponseTransformsInput{
				Credentials: args.domain(), Domain: args.Domain,
				Query: domain.ArvanCloudResponseTransformListQuery{Name: args.Name, PerPage: args.PerPage, Page: args.Page},
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(result.Transforms))
			for i, t := range result.Transforms {
				out[i] = arvanCloudResponseTransformToMap(t)
			}
			return map[string]any{"response_transforms": out, "page": arvanCloudPageRulePageMetaToMap(result.Page)}, nil
		},
	}
}

func createArvanCloudResponseTransformTool(uc *app.CreateArvanCloudResponseTransform) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["name"] = map[string]any{"type": "string", "description": "REQUIRED. A unique label for the preset within the domain, e.g. \"cors-allow-all\". Letters, digits, underscore and hyphen only."}
	props["description"] = map[string]any{"type": "string", "description": "An optional note about what this preset does."}
	props["transforms"] = arvanCloudResponseTransformStepsProperty()

	return Tool{
		Name: "create_arvancloud_response_transform",
		Description: "Create a new response-transform preset: a named, ordered set of condition+action steps that " +
			"add/replace CORS response headers on matching responses. This is a fast operation: the created preset, " +
			"including its provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "name", "transforms"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudResponseTransformArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudResponseTransformInput{Credentials: args.domain(), Domain: args.Domain, Transform: args.toDomain()})
			if err != nil {
				return nil, err
			}
			return arvanCloudResponseTransformToMap(*created), nil
		},
	}
}

func getArvanCloudResponseTransformTool(uc *app.GetArvanCloudResponseTransform) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudResponseTransformIDProperty()

	return Tool{
		Name:        "get_arvancloud_response_transform",
		Description: "Get one response-transform preset by ID. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "id"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudResponseTransformInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudResponseTransformToMap(*found), nil
		},
	}
}

func updateArvanCloudResponseTransformTool(uc *app.UpdateArvanCloudResponseTransform) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudResponseTransformIDProperty()
	props["name"] = map[string]any{"type": "string", "description": "REQUIRED. The preset's (possibly new) name."}
	props["description"] = map[string]any{"type": "string"}
	props["transforms"] = arvanCloudResponseTransformStepsProperty()

	return Tool{
		Name: "update_arvancloud_response_transform",
		Description: "Update a response-transform preset. Pass every field you want to keep, not only the ones " +
			"changing. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "id", "name", "transforms"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudResponseTransformArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudResponseTransformInput{
				Credentials: args.domain(), Domain: args.Domain, ID: args.ID, Transform: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudResponseTransformToMap(*updated), nil
		},
	}
}

func deleteArvanCloudResponseTransformTool(uc *app.DeleteArvanCloudResponseTransform) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudResponseTransformIDProperty()

	return Tool{
		Name: "delete_arvancloud_response_transform",
		Description: "Permanently delete a response-transform preset by ID. This is a fast operation and cannot be " +
			"undone. Deleting a preset that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "id"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudResponseTransformInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

// --- Redirect (www-redirect) ------------------------------------------------

func getArvanCloudWWWRedirectTool(uc *app.GetArvanCloudWWWRedirect) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name:        "get_arvancloud_www_redirect",
		Description: "Get a domain's www-redirect setting (whether/how the bare domain and its \"www.\" subdomain redirect to each other). This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			settings, err := uc.Execute(ctx, app.GetArvanCloudWWWRedirectInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return map[string]any{"mode": string(settings.Mode)}, nil
		},
	}
}

func updateArvanCloudWWWRedirectTool(uc *app.UpdateArvanCloudWWWRedirect) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["mode"] = map[string]any{
		"type": "string", "enum": []string{"off", "www", "root"},
		"description": "REQUIRED. \"off\" disables the redirect; \"www\" redirects the bare domain to its \"www.\" " +
			"subdomain; \"root\" redirects the \"www.\" subdomain to the bare domain.",
	}

	return Tool{
		Name:        "update_arvancloud_www_redirect",
		Description: "Set a domain's www-redirect setting. This is a fast operation; call get_arvancloud_www_redirect afterward to confirm.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "mode"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Mode string `json:"mode"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.UpdateArvanCloudWWWRedirectInput{
				Credentials: args.domain(), Domain: args.Domain, Settings: domain.ArvanCloudWWWRedirectSettings{Mode: domain.ArvanCloudWWWRedirectMode(args.Mode)},
			}); err != nil {
				return nil, err
			}
			return map[string]any{"domain": args.Domain, "mode": args.Mode}, nil
		},
	}
}

// --- Host Header Whitelist --------------------------------------------------

func arvanCloudHostHeaderWhitelistToMap(w domain.ArvanCloudHostHeaderWhitelist) map[string]any {
	return map[string]any{"target_accounts": w.TargetAccounts, "globally_whitelisted": w.GloballyWhitelisted}
}

func arvanCloudHostHeaderWhitelistTargetAccountProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "REQUIRED. The target CDN account's UUID to allow/remove using this domain as HTTP Host.",
	}
}

func getArvanCloudHostHeaderWhitelistTool(uc *app.GetArvanCloudHostHeaderWhitelist) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_host_header_whitelist",
		Description: "Get a domain's Host header whitelist state: whether it is on the global Host allowlist, and " +
			"(if not) which target CDN accounts may use it as HTTP Host. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudHostHeaderWhitelistInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudHostHeaderWhitelistToMap(*found), nil
		},
	}
}

func addArvanCloudHostHeaderWhitelistEntryTool(uc *app.AddArvanCloudHostHeaderWhitelistEntry) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["target_account"] = arvanCloudHostHeaderWhitelistTargetAccountProperty()

	return Tool{
		Name: "add_arvancloud_host_header_whitelist_entry",
		Description: "Whitelist a target CDN account so it may use this domain as HTTP Host. Rejected while the " +
			"domain is on the global Host allowlist. Rate limited to 30 requests per minute on target_account. " +
			"This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "target_account"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				TargetAccount string `json:"target_account"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.AddArvanCloudHostHeaderWhitelistEntryInput{Credentials: args.domain(), Domain: args.Domain, TargetAccount: args.TargetAccount})
			if err != nil {
				return nil, err
			}
			return arvanCloudHostHeaderWhitelistToMap(*updated), nil
		},
	}
}

func setArvanCloudHostHeaderWhitelistSettingsTool(uc *app.SetArvanCloudHostHeaderWhitelistSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["global"] = map[string]any{
		"type": "boolean",
		"description": "REQUIRED. If true, adds the domain to the global Host allowlist (any account may use it as " +
			"HTTP Host). If false, removes the global entry. Does not modify the per-account whitelist.",
	}

	return Tool{
		Name:        "set_arvancloud_host_header_whitelist_settings",
		Description: "Set or clear a domain's global Host allowlist entry. This is a fast operation.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "global"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Global bool `json:"global"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.SetArvanCloudHostHeaderWhitelistSettingsInput{Credentials: args.domain(), Domain: args.Domain, Global: args.Global})
			if err != nil {
				return nil, err
			}
			return arvanCloudHostHeaderWhitelistToMap(*updated), nil
		},
	}
}

func removeArvanCloudHostHeaderWhitelistEntryTool(uc *app.RemoveArvanCloudHostHeaderWhitelistEntry) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["target_account"] = arvanCloudHostHeaderWhitelistTargetAccountProperty()

	return Tool{
		Name: "remove_arvancloud_host_header_whitelist_entry",
		Description: "Remove this domain from the whitelist for the given target CDN account. Rate limited to 30 " +
			"requests per minute on target_account. This is a fast operation. Unlike most delete-style tools in " +
			"this server, a target account that was never whitelisted is reported back by ArvanCloud as an error " +
			"rather than treated as already done.",
		InputSchema: map[string]any{"type": "object", "properties": props, "required": []string{"api_key", "domain", "target_account"}},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				TargetAccount string `json:"target_account"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.RemoveArvanCloudHostHeaderWhitelistEntryInput{Credentials: args.domain(), Domain: args.Domain, TargetAccount: args.TargetAccount})
			if err != nil {
				return nil, err
			}
			return arvanCloudHostHeaderWhitelistToMap(*updated), nil
		},
	}
}
