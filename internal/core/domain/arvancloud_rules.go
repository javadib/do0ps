package domain

// The types below model four ArvanCloud CDN capabilities grouped by issue
// #71 (AC11) as "edge routing/rewrite rule engines": Page Rules, Response
// Transforms, Redirect (www-redirect) and Host Header Whitelist. Confirmed
// against docs/api-specs/arvancloud-cdn-4.0.yml's "Page Rule", "Response
// Transforms", "Redirect" and "Host header whitelist" tags and the
// PageRuleSummary/PageRule/PageRuleDiff, ResponseTransform/
// ProxyResponseTransformStep/ProxyResponseTransformAddHeaderAction, Redirect
// and HostHeaderWhitelist* schemas.
//
// Page Rule wire shape note: the spec's PageRule schema is FLAT (an allOf of
// PageRuleSummary plus ~30 more top-level properties, no nesting beyond a
// handful of sub-objects like acceleration/image_resize/upstream_timeout/
// redirect). ArvanCloudPageRule below is still organized into named
// sub-structs by logical group (Matching/Caching/Security/Routing/Headers/
// Other), per issue #71's own code-quality recommendation — this is a Go-side
// grouping only. The secondary adapter
// (internal/adapters/providers/arvancloud/rules.go) is what actually
// flattens/unflattens between this grouped Go shape and the flat wire JSON,
// the same convention every other ArvanCloud domain type in this package
// already follows (e.g. ArvanCloudHealthCheck's wire counterpart lives in
// healthcheck.go, not as JSON tags on the domain type itself) — domain types
// carry no json tags anywhere in this package, and this file does not break
// that.

// --- Shared cache TTL enum -----------------------------------------------

// ArvanCloudCacheTTL is the TTL-duration enum shared by PageRule.cache_200,
// PageRule.cache_any and PageRule.cache_browser (and their PageRuleDiff
// equivalents) — all four fields declare the exact same 30-value string enum
// in the spec ("0s".."30d"), so this one Go type/validator pair covers all of
// them rather than three (or four) near-identical enums (issue #71's own
// instruction).
//
// cache_browser additionally accepts the sentinel value "default", which
// none of the other three fields declare — ValidArvanCloudCacheBrowserTTL
// layers that one extra value on top of the same underlying list rather than
// defining a second, mostly-duplicate enum.
type ArvanCloudCacheTTL string

const (
	ArvanCloudCacheTTL0s  ArvanCloudCacheTTL = "0s"
	ArvanCloudCacheTTL1s  ArvanCloudCacheTTL = "1s"
	ArvanCloudCacheTTL2s  ArvanCloudCacheTTL = "2s"
	ArvanCloudCacheTTL3s  ArvanCloudCacheTTL = "3s"
	ArvanCloudCacheTTL4s  ArvanCloudCacheTTL = "4s"
	ArvanCloudCacheTTL5s  ArvanCloudCacheTTL = "5s"
	ArvanCloudCacheTTL6s  ArvanCloudCacheTTL = "6s"
	ArvanCloudCacheTTL7s  ArvanCloudCacheTTL = "7s"
	ArvanCloudCacheTTL8s  ArvanCloudCacheTTL = "8s"
	ArvanCloudCacheTTL9s  ArvanCloudCacheTTL = "9s"
	ArvanCloudCacheTTL10s ArvanCloudCacheTTL = "10s"
	ArvanCloudCacheTTL30s ArvanCloudCacheTTL = "30s"
	ArvanCloudCacheTTL1m  ArvanCloudCacheTTL = "1m"
	ArvanCloudCacheTTL3m  ArvanCloudCacheTTL = "3m"
	ArvanCloudCacheTTL5m  ArvanCloudCacheTTL = "5m"
	ArvanCloudCacheTTL10m ArvanCloudCacheTTL = "10m"
	ArvanCloudCacheTTL30m ArvanCloudCacheTTL = "30m"
	ArvanCloudCacheTTL45m ArvanCloudCacheTTL = "45m"
	ArvanCloudCacheTTL1h  ArvanCloudCacheTTL = "1h"
	ArvanCloudCacheTTL3h  ArvanCloudCacheTTL = "3h"
	ArvanCloudCacheTTL5h  ArvanCloudCacheTTL = "5h"
	ArvanCloudCacheTTL10h ArvanCloudCacheTTL = "10h"
	ArvanCloudCacheTTL12h ArvanCloudCacheTTL = "12h"
	ArvanCloudCacheTTL24h ArvanCloudCacheTTL = "24h"
	ArvanCloudCacheTTL3d  ArvanCloudCacheTTL = "3d"
	ArvanCloudCacheTTL7d  ArvanCloudCacheTTL = "7d"
	ArvanCloudCacheTTL10d ArvanCloudCacheTTL = "10d"
	ArvanCloudCacheTTL15d ArvanCloudCacheTTL = "15d"
	ArvanCloudCacheTTL30d ArvanCloudCacheTTL = "30d"

	// ArvanCloudCacheTTLDefault is cache_browser's own extra sentinel value,
	// not shared by cache_200/cache_any. See ValidArvanCloudCacheBrowserTTL.
	ArvanCloudCacheTTLDefault ArvanCloudCacheTTL = "default"
)

var arvanCloudCacheTTLs = []string{
	string(ArvanCloudCacheTTL0s), string(ArvanCloudCacheTTL1s), string(ArvanCloudCacheTTL2s),
	string(ArvanCloudCacheTTL3s), string(ArvanCloudCacheTTL4s), string(ArvanCloudCacheTTL5s),
	string(ArvanCloudCacheTTL6s), string(ArvanCloudCacheTTL7s), string(ArvanCloudCacheTTL8s),
	string(ArvanCloudCacheTTL9s), string(ArvanCloudCacheTTL10s), string(ArvanCloudCacheTTL30s),
	string(ArvanCloudCacheTTL1m), string(ArvanCloudCacheTTL3m), string(ArvanCloudCacheTTL5m),
	string(ArvanCloudCacheTTL10m), string(ArvanCloudCacheTTL30m), string(ArvanCloudCacheTTL45m),
	string(ArvanCloudCacheTTL1h), string(ArvanCloudCacheTTL3h), string(ArvanCloudCacheTTL5h),
	string(ArvanCloudCacheTTL10h), string(ArvanCloudCacheTTL12h), string(ArvanCloudCacheTTL24h),
	string(ArvanCloudCacheTTL3d), string(ArvanCloudCacheTTL7d), string(ArvanCloudCacheTTL10d),
	string(ArvanCloudCacheTTL15d), string(ArvanCloudCacheTTL30d),
}

// ValidArvanCloudCacheTTL reports whether s is one of the 29-value TTL enum
// shared by cache_200 and cache_any (and their PageRuleDiff equivalents).
func ValidArvanCloudCacheTTL(s string) bool { return contains(arvanCloudCacheTTLs, s) }

// ValidArvanCloudCacheBrowserTTL reports whether s is valid for
// cache_browser: the same 29-value TTL enum ValidArvanCloudCacheTTL checks,
// plus the one extra sentinel "default" cache_browser alone accepts.
func ValidArvanCloudCacheBrowserTTL(s string) bool {
	return s == string(ArvanCloudCacheTTLDefault) || ValidArvanCloudCacheTTL(s)
}

// --- Page Rule enums -------------------------------------------------------

// ArvanCloudPageRuleURLType is PageRuleSummary.url_type's enum. Deprecated in
// the spec in favor of IsProtected, but still a real field the API accepts
// and returns, so it is still modeled here.
type ArvanCloudPageRuleURLType string

const (
	ArvanCloudPageRuleURLTypeDefault   ArvanCloudPageRuleURLType = "default"
	ArvanCloudPageRuleURLTypeIndex     ArvanCloudPageRuleURLType = "index"
	ArvanCloudPageRuleURLTypeDirectory ArvanCloudPageRuleURLType = "directory"
	ArvanCloudPageRuleURLTypeExtension ArvanCloudPageRuleURLType = "extension"
	ArvanCloudPageRuleURLTypePage      ArvanCloudPageRuleURLType = "page"
	ArvanCloudPageRuleURLTypeRegex     ArvanCloudPageRuleURLType = "regex"
)

var arvanCloudPageRuleURLTypes = []string{
	string(ArvanCloudPageRuleURLTypeDefault), string(ArvanCloudPageRuleURLTypeIndex),
	string(ArvanCloudPageRuleURLTypeDirectory), string(ArvanCloudPageRuleURLTypeExtension),
	string(ArvanCloudPageRuleURLTypePage), string(ArvanCloudPageRuleURLTypeRegex),
}

// ValidArvanCloudPageRuleURLType reports whether s is one of
// PageRuleSummary.url_type's six values, or empty (the field is optional;
// the provider applies its own "default" when omitted).
func ValidArvanCloudPageRuleURLType(s string) bool {
	if s == "" {
		return true
	}
	return contains(arvanCloudPageRuleURLTypes, s)
}

// ArvanCloudPageRuleCacheLevel is PageRule.cache_level's enum.
type ArvanCloudPageRuleCacheLevel string

const (
	ArvanCloudPageRuleCacheLevelOff         ArvanCloudPageRuleCacheLevel = "off"
	ArvanCloudPageRuleCacheLevelURI         ArvanCloudPageRuleCacheLevel = "uri"
	ArvanCloudPageRuleCacheLevelQueryString ArvanCloudPageRuleCacheLevel = "query_string"
)

var arvanCloudPageRuleCacheLevels = []string{
	string(ArvanCloudPageRuleCacheLevelOff), string(ArvanCloudPageRuleCacheLevelURI), string(ArvanCloudPageRuleCacheLevelQueryString),
}

// ValidArvanCloudPageRuleCacheLevel reports whether s is one of
// PageRule.cache_level's three values, or empty (provider default applies).
func ValidArvanCloudPageRuleCacheLevel(s string) bool {
	if s == "" {
		return true
	}
	return contains(arvanCloudPageRuleCacheLevels, s)
}

// ArvanCloudPageRuleSlinkMD5Field is one entry of PageRule.slink_md5: which
// request attributes are hashed into a secure-link token.
type ArvanCloudPageRuleSlinkMD5Field string

const (
	ArvanCloudPageRuleSlinkMD5RemoteAddr ArvanCloudPageRuleSlinkMD5Field = "remote_addr"
	ArvanCloudPageRuleSlinkMD5File       ArvanCloudPageRuleSlinkMD5Field = "file"
	ArvanCloudPageRuleSlinkMD5Expires    ArvanCloudPageRuleSlinkMD5Field = "expires"
	ArvanCloudPageRuleSlinkMD5URL        ArvanCloudPageRuleSlinkMD5Field = "url"
	ArvanCloudPageRuleSlinkMD5URI        ArvanCloudPageRuleSlinkMD5Field = "uri"
)

var arvanCloudPageRuleSlinkMD5Fields = []string{
	string(ArvanCloudPageRuleSlinkMD5RemoteAddr), string(ArvanCloudPageRuleSlinkMD5File),
	string(ArvanCloudPageRuleSlinkMD5Expires), string(ArvanCloudPageRuleSlinkMD5URL), string(ArvanCloudPageRuleSlinkMD5URI),
}

// ValidArvanCloudPageRuleSlinkMD5Field reports whether s is one of
// PageRule.slink_md5's five item values.
func ValidArvanCloudPageRuleSlinkMD5Field(s string) bool {
	return contains(arvanCloudPageRuleSlinkMD5Fields, s)
}

// ArvanCloudPageRuleImageResizeStatus is PageRuleImageResize.status's enum —
// a narrower override of the base ImageResize.status ("on"/"off") that adds
// "inherit", meaningful only in the PageRule/PageRuleDiff context.
type ArvanCloudPageRuleImageResizeStatus string

const (
	ArvanCloudPageRuleImageResizeOn      ArvanCloudPageRuleImageResizeStatus = "on"
	ArvanCloudPageRuleImageResizeOff     ArvanCloudPageRuleImageResizeStatus = "off"
	ArvanCloudPageRuleImageResizeInherit ArvanCloudPageRuleImageResizeStatus = "inherit"
)

var arvanCloudPageRuleImageResizeStatuses = []string{
	string(ArvanCloudPageRuleImageResizeOn), string(ArvanCloudPageRuleImageResizeOff), string(ArvanCloudPageRuleImageResizeInherit),
}

// ValidArvanCloudPageRuleImageResizeStatus reports whether s is one of
// PageRuleImageResize.status's three values, or empty (provider default
// applies).
func ValidArvanCloudPageRuleImageResizeStatus(s string) bool {
	if s == "" {
		return true
	}
	return contains(arvanCloudPageRuleImageResizeStatuses, s)
}

// ArvanCloudImageResizeMode is ImageResize.mode's enum, used by both
// PageRule.image_resize and the standalone image-resize endpoint (not this
// issue's scope, but the same schema).
type ArvanCloudImageResizeMode string

const (
	ArvanCloudImageResizeModeFreely    ArvanCloudImageResizeMode = "freely"
	ArvanCloudImageResizeModeShortSide ArvanCloudImageResizeMode = "short-side"
	ArvanCloudImageResizeModeLongSide  ArvanCloudImageResizeMode = "long-side"
)

var arvanCloudImageResizeModes = []string{
	string(ArvanCloudImageResizeModeFreely), string(ArvanCloudImageResizeModeShortSide), string(ArvanCloudImageResizeModeLongSide),
}

// ValidArvanCloudImageResizeMode reports whether s is one of
// ImageResize.mode's three values, or empty (provider default applies).
func ValidArvanCloudImageResizeMode(s string) bool {
	if s == "" {
		return true
	}
	return contains(arvanCloudImageResizeModes, s)
}

// ArvanCloudPageRuleRedirectStatusCode is PageRuleRedirect.status_code's
// fixed integer enum.
var arvanCloudPageRuleRedirectStatusCodes = []int{301, 302, 307}

// ValidArvanCloudPageRuleRedirectStatusCode reports whether n is one of
// PageRuleRedirect.status_code's three values, or zero (provider default,
// 301, applies).
func ValidArvanCloudPageRuleRedirectStatusCode(n int) bool {
	if n == 0 {
		return true
	}
	for _, v := range arvanCloudPageRuleRedirectStatusCodes {
		if v == n {
			return true
		}
	}
	return false
}

// --- Page Rule sub-structs (Go-side grouping only; see file comment) ------

// ArvanCloudPageRuleImageResize is PageRule.image_resize / the inline
// image_resize shape on PageRuleDiff (PageRuleImageResize schema — an allOf
// of the base ImageResize schema with a widened status enum).
type ArvanCloudPageRuleImageResize struct {
	// Status must be one of ValidArvanCloudPageRuleImageResizeStatus's
	// values. Defaults to "off" when left unset.
	Status ArvanCloudPageRuleImageResizeStatus
	// HeightBy/WidthBy are the query-string argument names read for the
	// target height/width, e.g. "height"/"width". Pattern: [a-z0-9_]+, max 20
	// chars. Default "height"/"width" when left unset.
	HeightBy string
	WidthBy  string
	// Mode must be one of ValidArvanCloudImageResizeMode's values, or empty.
	Mode ArvanCloudImageResizeMode
}

// ArvanCloudUpstreamTimeout is PageRule.upstream_timeout (the UpstreamTimeout
// schema), all fields in SECONDS.
type ArvanCloudUpstreamTimeout struct {
	// ConnectTimeoutSeconds defaults to 30 when left unset (zero).
	ConnectTimeoutSeconds int
	// ReadTimeoutSeconds defaults to 100 when left unset (zero).
	ReadTimeoutSeconds int
	// SendTimeoutSeconds defaults to 300 when left unset (zero).
	SendTimeoutSeconds int
}

// ArvanCloudPageRuleRedirect is PageRule.redirect (the PageRuleRedirect
// schema): a rule-scoped HTTP redirect. Per the spec's own description on
// PageRule.rewrite_url, a rule cannot combine a redirect with URL rewriting
// or a secure link — this adapter does not enforce that combination
// client-side, since it is a cross-field business rule the provider itself
// validates and reports back on a 422.
type ArvanCloudPageRuleRedirect struct {
	// Enable turns the redirect on. Defaults to false when left unset.
	Enable bool
	// StatusCode must satisfy ValidArvanCloudPageRuleRedirectStatusCode
	// (301/302/307, or 0 to let the provider apply its own default of 301).
	StatusCode int
	// URL is the redirect target.
	URL string
}

// ArvanCloudPageRuleHeaderEntry is one entry of PageRule.req_custom_headers /
// PageRule.res_custom_headers: a header name/value pair added by the rule.
type ArvanCloudPageRuleHeaderEntry struct {
	Name  string
	Value string
}

// ArvanCloudPageRuleMatching groups the fields that decide which requests a
// page rule applies to and whether it is currently active.
type ArvanCloudPageRuleMatching struct {
	// URLType must satisfy ValidArvanCloudPageRuleURLType. Deprecated by the
	// provider in favor of IsProtected, but still accepted/returned.
	URLType ArvanCloudPageRuleURLType
	// URL is the match pattern. Its exact shape (a literal path, a glob, a
	// regex, ...) depends on URLType.
	URL string
	// Seq is this rule's priority/order among the domain's page rules. Lower
	// values are evaluated first per the spec's own default sort.
	Seq int
	// IsProtected is read-only: a protected rule cannot be modified or
	// deleted by the caller. Never sent on create/update.
	IsProtected bool
	// Status is the admin enable/disable switch for the whole rule. Defaults
	// to true when left unset.
	Status bool
}

// ArvanCloudPageRuleCaching groups every caching-behavior field.
type ArvanCloudPageRuleCaching struct {
	// CacheLevel must satisfy ValidArvanCloudPageRuleCacheLevel. Defaults to
	// "query_string" when left unset.
	CacheLevel ArvanCloudPageRuleCacheLevel
	// Cache200/CacheAny/CacheBrowser are the shared TTL enum (see this file's
	// package comment): how long to cache a 200 response, any response, and
	// the browser-facing cache-control TTL, respectively. Cache200/CacheAny
	// must satisfy ValidArvanCloudCacheTTL; CacheBrowser must satisfy
	// ValidArvanCloudCacheBrowserTTL (which additionally allows "default").
	// Defaults: Cache200 "30m", CacheAny "0s", CacheBrowser "default".
	Cache200     ArvanCloudCacheTTL
	CacheAny     ArvanCloudCacheTTL
	CacheBrowser ArvanCloudCacheTTL
	// CacheCookie is a comma-separated list of cookie names to vary the cache
	// on. Empty means none.
	CacheCookie string
	// CacheDeviceType varies the cache by device type. Defaults to false.
	CacheDeviceType bool
	// CacheArgs varies the cache by query-string arguments. Defaults to
	// true.
	CacheArgs bool
	// CacheArg is a "&"-separated list of specific query-string argument
	// names to vary the cache on, e.g. "filter&sort". Empty means none/all,
	// depending on CacheArgs.
	CacheArg string
	// CacheScheme is deprecated by the provider but still a real field.
	// Defaults to true when left unset.
	CacheScheme bool
	// CacheIgnoreSC ignores the default Set-Cookie-header caching behavior.
	// Defaults to false.
	CacheIgnoreSC bool
	// CacheIgnoreVary ignores the default Vary-header caching behavior.
	// Defaults to true.
	CacheIgnoreVary bool
	// CacheIgnoreCC defaults to true when left unset.
	CacheIgnoreCC bool
}

// ArvanCloudPageRuleSecurity groups the WAF/firewall/secure-link toggles.
type ArvanCloudPageRuleSecurity struct {
	// WAFStatus turns the Web Application Firewall on for requests matching
	// this rule. Defaults to true when left unset.
	WAFStatus bool
	// FWStatus is deprecated by the provider but still a real field.
	// Defaults to true when left unset.
	FWStatus bool
	// SlinkStatus turns secure-link (token-authenticated URLs) on for this
	// rule. Defaults to false when left unset.
	SlinkStatus bool
	// SlinkSecret is the shared secret used to compute the secure-link
	// token. Only meaningful when SlinkStatus is true.
	SlinkSecret string
	// SlinkMD5 selects which request attributes are hashed into the
	// secure-link token; each entry must satisfy
	// ValidArvanCloudPageRuleSlinkMD5Field. Defaults to
	// [remote_addr, file, expires] when left unset (nil).
	SlinkMD5 []ArvanCloudPageRuleSlinkMD5Field
}

// ArvanCloudPageRuleRouting groups the fields that redirect, rewrite, or
// re-route matching requests to a different backend.
type ArvanCloudPageRuleRouting struct {
	// Redirect is a rule-scoped HTTP redirect. Zero value (Enable: false)
	// means no redirect.
	Redirect ArvanCloudPageRuleRedirect
	// RewriteURL rewrites the request path before it reaches the origin. Per
	// the spec, cannot be combined with Redirect or secure link (see
	// ArvanCloudPageRuleRedirect's doc comment).
	RewriteURL string
	// LoadBalancer is the name or ID of a Load Balancing resource (#69/AC9)
	// this rule routes matching traffic to. Empty means none.
	LoadBalancer string
	// EdgeComputeID is the UUID of an edge compute function this rule routes
	// to. Marked `alpha` by the provider. Empty means none.
	EdgeComputeID string
	// ClusterStatus/ClusterID are deprecated by the provider but still real,
	// gettable/settable fields.
	ClusterStatus bool
	ClusterID     string
}

// ArvanCloudPageRuleHeaders groups every request/response header
// manipulation field.
type ArvanCloudPageRuleHeaders struct {
	// CORSHeader sets the CORS response header value this rule applies. "-"
	// (the spec's own default) means unset/not applied.
	CORSHeader string
	// ReqCustomHeaders/ResCustomHeaders add the given name/value headers to
	// the request sent upstream / the response sent to the client,
	// respectively.
	ReqCustomHeaders []ArvanCloudPageRuleHeaderEntry
	ResCustomHeaders []ArvanCloudPageRuleHeaderEntry
	// ReqHideHeaders/ResHideHeaders strip the named headers from the request
	// sent upstream / the response sent to the client, respectively.
	ReqHideHeaders []string
	ResHideHeaders []string
	// CustomHostHeader overrides the Host header sent to the origin for
	// requests matching this rule. Empty leaves it unset.
	CustomHostHeader string
}

// ArvanCloudPageRuleOther groups the remaining fields that do not fit
// Matching/Caching/Security/Routing/Headers.
type ArvanCloudPageRuleOther struct {
	// Acceleration is this rule's acceleration override. See
	// ArvanCloudAccelerationSettings' own doc comment (arvancloud_acceleration.go)
	// for why it is a shared, standalone type.
	Acceleration ArvanCloudAccelerationSettings
	// ImageResize is this rule's image-resize override.
	ImageResize ArvanCloudPageRuleImageResize
	// UpstreamTimeout is this rule's upstream connect/read/send timeouts.
	UpstreamTimeout ArvanCloudUpstreamTimeout
}

// ArvanCloudPageRule is a domain-scoped edge rewrite/routing rule
// (/domains/{domain}/page-rules[/{id}], the PageRule schema — an allOf of
// PageRuleSummary plus roughly 30 more flat properties; see this file's
// package comment for the flat-wire-vs-grouped-Go-struct note). The largest
// and most complex single resource in the ArvanCloud CDN spec by field
// count.
type ArvanCloudPageRule struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string
	// DomainID is the provider-assigned UUID of the parent domain. Read-only.
	DomainID string

	Matching ArvanCloudPageRuleMatching
	Caching  ArvanCloudPageRuleCaching
	Security ArvanCloudPageRuleSecurity
	Routing  ArvanCloudPageRuleRouting
	Headers  ArvanCloudPageRuleHeaders
	Other    ArvanCloudPageRuleOther

	// CreatedAt/UpdatedAt are provider-reported timestamps. Read-only.
	CreatedAt string
	UpdatedAt string
}

// ArvanCloudPageRuleSummary is one entry of ListArvanCloudPageRules' result
// (the PageRuleSummary schema): the subset of ArvanCloudPageRule's fields
// the list endpoint actually returns, confirmed against the spec rather than
// assumed to be the full PageRule shape (page-rules.index's response items
// are typed PageRuleSummary, not PageRule).
type ArvanCloudPageRuleSummary struct {
	ID           string
	DomainID     string
	Seq          int
	URLType      ArvanCloudPageRuleURLType
	IsProtected  bool
	URL          string
	CacheLevel   ArvanCloudPageRuleCacheLevel
	WAFStatus    bool
	FWStatus     bool
	Acceleration ArvanCloudAccelerationSettings
	SlinkStatus  bool
	Status       bool
	CreatedAt    string
	UpdatedAt    string
}

// ArvanCloudPageRuleListQuery filters/paginates ListArvanCloudPageRules
// (page-rules.index's query parameters).
type ArvanCloudPageRuleListQuery struct {
	// Search filters by a free-text search term. Empty means no filter.
	Search string
	// PerPage/Page paginate the result. Zero leaves both to the provider's
	// own defaults.
	PerPage int
	Page    int
	// Order sorts by Seq, "asc" or "desc". Empty lets the provider apply its
	// own default ("desc").
	Order string
}

// ArvanCloudPageRulePageMeta is the pagination info attached to
// ListArvanCloudPageRules' result, the same shape
// ArvanCloudHealthCheckReportPageMeta gives the health-check details report.
type ArvanCloudPageRulePageMeta struct {
	CurrentPage int
	From        int
	LastPage    int
	PerPage     int
	To          int
	Total       int
}

// --- Page Rule exceptions ("diff") -----------------------------------------

// ArvanCloudPageRuleExceptionHeaderEntry is one entry of
// PageRuleDiff.req_custom_headers / .res_custom_headers: like
// ArvanCloudPageRuleHeaderEntry, but the diff variant additionally carries
// IsVar, matching the spec's own (string-typed, not boolean-typed) field.
type ArvanCloudPageRuleExceptionHeaderEntry struct {
	Name  string
	Value string
	// IsVar must be "true", "false", or empty (omitted). Modeled as a string
	// rather than a bool because the spec itself types
	// PageRuleDiff.req_custom_headers[].is_var as a two-value STRING enum
	// (["true", "false"]), not a JSON boolean — an odd but explicit choice
	// this adapter mirrors rather than silently reinterprets, so a caller
	// reading this field back sees exactly what the provider sent.
	IsVar string
}

// ValidArvanCloudPageRuleExceptionIsVar reports whether s is "true", "false",
// or empty (omitted).
func ValidArvanCloudPageRuleExceptionIsVar(s string) bool {
	return s == "" || s == "true" || s == "false"
}

// ArvanCloudPageRuleExceptions is a page rule's "exceptions" — a sparse
// override layer on top of the parent rule's own fields
// (/domains/{domain}/page-rules/{id}/diff, the PageRuleDiff schema).
//
// Unlike ArvanCloudPageRule (a full-replace resource: every field is always
// sent on create/update), PageRuleDiff declares no non-empty default on any
// of its properties — every field here is an OPTIONAL override, and
// UpdateArvanCloudPageRuleExceptions only sends the fields left at a
// non-zero value; a zero-valued field (false, "", 0, an empty slice) is
// omitted from the request rather than clearing the corresponding override.
// This is the same "zero is ambiguous between unset and a real zero value"
// limitation domain.ArvanCloudLoadBalancerSettings.MaxFails documents,
// applied to every field of this struct rather than just one: there is
// currently no way to explicitly override a boolean field to false through
// this adapter. A caller who genuinely needs that clears the underlying
// PageRule field itself instead of using an exception for it.
//
// Spec note: PageRuleDiff omits url_type/is_protected/domain_id/seq compared
// to PageRule/PageRuleSummary — those identify/order the parent rule itself
// and are not overridable per-exception.
type ArvanCloudPageRuleExceptions struct {
	URL              string
	CacheLevel       ArvanCloudPageRuleCacheLevel
	WAFStatus        bool
	FWStatus         bool
	Acceleration     ArvanCloudAccelerationSettings
	SlinkStatus      bool
	Status           bool
	Cache200         ArvanCloudCacheTTL
	CacheAny         ArvanCloudCacheTTL
	CacheCookie      string
	CacheArgs        bool
	CacheArg         string
	CacheScheme      bool
	CacheBrowser     ArvanCloudCacheTTL
	CacheIgnoreSC    bool
	CacheIgnoreVary  bool
	CacheIgnoreCC    bool
	CORSHeader       string
	RewriteURL       string
	SlinkSecret      string
	SlinkMD5         []ArvanCloudPageRuleSlinkMD5Field
	ClusterStatus    bool
	ClusterID        string
	EdgeComputeID    string
	UpstreamTimeout  ArvanCloudUpstreamTimeout
	ReqCustomHeaders []ArvanCloudPageRuleExceptionHeaderEntry
	CacheDeviceType  bool
	ImageResize      ArvanCloudPageRuleImageResize
	LoadBalancer     string
	ResCustomHeaders []ArvanCloudPageRuleExceptionHeaderEntry
	ReqHideHeaders   []string
	ResHideHeaders   []string
	CustomHostHeader string
	Redirect         ArvanCloudPageRuleRedirect
}

// --- Response Transforms ---------------------------------------------------

// ArvanCloudResponseTransformActionType is
// ProxyResponseTransformAddHeaderAction.type's enum. The spec currently
// declares exactly one value, "add_header" — kept as its own field (the same
// "expose as a field rather than hardcode" convention
// ArvanCloudHealthCheckOriginType established for a single-value enum) since
// the spec does not say whether more action types will be added later.
type ArvanCloudResponseTransformActionType string

// ArvanCloudResponseTransformActionAddHeader is the only value the spec
// currently declares for a transform action's Type.
const ArvanCloudResponseTransformActionAddHeader ArvanCloudResponseTransformActionType = "add_header"

var arvanCloudResponseTransformActionTypes = []string{string(ArvanCloudResponseTransformActionAddHeader)}

// ValidArvanCloudResponseTransformActionType reports whether s is one of the
// spec's declared action types.
func ValidArvanCloudResponseTransformActionType(s string) bool {
	return contains(arvanCloudResponseTransformActionTypes, s)
}

// ArvanCloudResponseTransformActionMode is
// ProxyResponseTransformAddHeaderAction.mode's enum. Like Type above, the
// spec currently declares exactly one value, "set".
type ArvanCloudResponseTransformActionMode string

// ArvanCloudResponseTransformModeSet is the only value the spec currently
// declares for a transform action's Mode: add the header, or replace it if
// already present.
const ArvanCloudResponseTransformModeSet ArvanCloudResponseTransformActionMode = "set"

var arvanCloudResponseTransformActionModes = []string{string(ArvanCloudResponseTransformModeSet)}

// ValidArvanCloudResponseTransformActionMode reports whether s is one of the
// spec's declared action modes.
func ValidArvanCloudResponseTransformActionMode(s string) bool {
	return contains(arvanCloudResponseTransformActionModes, s)
}

// ArvanCloudResponseTransformHeaderKey is
// ProxyResponseTransformAddHeaderAction.key's enum: the response headers a
// transform action is allowed to set. All six declared values are CORS
// headers per the spec's own description.
type ArvanCloudResponseTransformHeaderKey string

const (
	ArvanCloudResponseTransformHeaderAllowOrigin      ArvanCloudResponseTransformHeaderKey = "Access-Control-Allow-Origin"
	ArvanCloudResponseTransformHeaderExposeHeaders    ArvanCloudResponseTransformHeaderKey = "Access-Control-Expose-Headers"
	ArvanCloudResponseTransformHeaderMaxAge           ArvanCloudResponseTransformHeaderKey = "Access-Control-Max-Age"
	ArvanCloudResponseTransformHeaderAllowCredentials ArvanCloudResponseTransformHeaderKey = "Access-Control-Allow-Credentials"
	ArvanCloudResponseTransformHeaderAllowMethods     ArvanCloudResponseTransformHeaderKey = "Access-Control-Allow-Methods"
	ArvanCloudResponseTransformHeaderAllowHeaders     ArvanCloudResponseTransformHeaderKey = "Access-Control-Allow-Headers"
)

var arvanCloudResponseTransformHeaderKeys = []string{
	string(ArvanCloudResponseTransformHeaderAllowOrigin), string(ArvanCloudResponseTransformHeaderExposeHeaders),
	string(ArvanCloudResponseTransformHeaderMaxAge), string(ArvanCloudResponseTransformHeaderAllowCredentials),
	string(ArvanCloudResponseTransformHeaderAllowMethods), string(ArvanCloudResponseTransformHeaderAllowHeaders),
}

// ValidArvanCloudResponseTransformHeaderKey reports whether s is one of the
// six CORS response headers a transform action may set.
func ValidArvanCloudResponseTransformHeaderKey(s string) bool {
	return contains(arvanCloudResponseTransformHeaderKeys, s)
}

// ArvanCloudResponseTransformAction is one action of a
// ArvanCloudResponseTransformStep (the ProxyResponseTransformAddHeaderAction
// schema): set or replace one CORS response header when the step's Condition
// matches.
type ArvanCloudResponseTransformAction struct {
	// Type must satisfy ValidArvanCloudResponseTransformActionType.
	Type ArvanCloudResponseTransformActionType
	// Mode must satisfy ValidArvanCloudResponseTransformActionMode.
	Mode ArvanCloudResponseTransformActionMode
	// Key must satisfy ValidArvanCloudResponseTransformHeaderKey.
	Key ArvanCloudResponseTransformHeaderKey
	// Value must resolve to a string per the spec's own grammar: one of
	// http.request.headers["origin"], http.request.headers["host"], or
	// to_string("literal").
	Value string
}

// ArvanCloudResponseTransformStep is one entry of
// ArvanCloudResponseTransform.Transforms (the ProxyResponseTransformStep
// schema): a Wireshark-like filter condition plus the actions applied when
// it matches.
type ArvanCloudResponseTransformStep struct {
	// Condition is a Wireshark-like filter expression, 3-5000 characters,
	// validated against the same plan-based field allowlist as domain
	// firewall rules' filter_expr (per the spec's own description). This
	// adapter does not attempt to validate the expression grammar
	// client-side — only its length — since it depends on the domain's plan.
	Condition string
	// Actions is applied in order when Condition matches. At least one
	// required.
	Actions []ArvanCloudResponseTransformAction
}

// ArvanCloudResponseTransform is a domain-scoped response-transform preset
// (/domains/{domain}/response-transforms[/{id}], the ResponseTransform/
// ResponseTransformStore/ResponseTransformUpdate schemas): a named, ordered
// set of condition+action steps applied to matching responses.
type ArvanCloudResponseTransform struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string
	// Name is a caller-supplied label, unique within the domain. Pattern:
	// ^[\w-]+$, max 255 chars.
	Name string
	// Description is a caller-supplied note about the preset. Empty means
	// none.
	Description string
	// Transforms is this preset's ordered step list.
	Transforms []ArvanCloudResponseTransformStep
	// CreatedAt/UpdatedAt are provider-reported timestamps. Read-only.
	CreatedAt string
	UpdatedAt string
}

// ArvanCloudResponseTransformListQuery filters/paginates
// ListArvanCloudResponseTransforms (response_transforms.index's query
// parameters).
type ArvanCloudResponseTransformListQuery struct {
	// Name filters by a substring match against the preset name. Empty means
	// no filter.
	Name string
	// PerPage/Page paginate the result. Zero leaves both to the provider's
	// own defaults.
	PerPage int
	Page    int
}

// ArvanCloudResponseTransformPageMeta is the pagination info attached to
// ListArvanCloudResponseTransforms' result, the same shape
// ArvanCloudPageRulePageMeta gives the page-rule list.
type ArvanCloudResponseTransformPageMeta = ArvanCloudPageRulePageMeta

// --- Redirect (www-redirect) ------------------------------------------------

// ArvanCloudWWWRedirectMode is ArvanCloudWWWRedirectSettings.Mode's enum
// (Redirect.f_redirect_to_www).
type ArvanCloudWWWRedirectMode string

const (
	// ArvanCloudWWWRedirectOff disables the www redirect.
	ArvanCloudWWWRedirectOff ArvanCloudWWWRedirectMode = "off"
	// ArvanCloudWWWRedirectToWWW redirects the bare domain to its "www."
	// subdomain.
	ArvanCloudWWWRedirectToWWW ArvanCloudWWWRedirectMode = "www"
	// ArvanCloudWWWRedirectToRoot redirects the "www." subdomain to the bare
	// domain.
	ArvanCloudWWWRedirectToRoot ArvanCloudWWWRedirectMode = "root"
)

var arvanCloudWWWRedirectModes = []string{
	string(ArvanCloudWWWRedirectOff), string(ArvanCloudWWWRedirectToWWW), string(ArvanCloudWWWRedirectToRoot),
}

// ValidArvanCloudWWWRedirectMode reports whether s is one of
// Redirect.f_redirect_to_www's three values.
func ValidArvanCloudWWWRedirectMode(s string) bool { return contains(arvanCloudWWWRedirectModes, s) }

// ArvanCloudWWWRedirectSettings is a domain's www-redirect configuration
// (/domains/{domain}/settings/www-redirect, the Redirect schema).
type ArvanCloudWWWRedirectSettings struct {
	// Mode must satisfy ValidArvanCloudWWWRedirectMode.
	Mode ArvanCloudWWWRedirectMode
}

// --- Host Header Whitelist --------------------------------------------------

// ArvanCloudHostHeaderWhitelist is a domain's Host-header-whitelist state
// (/domains/{domain}/host-header-whitelists[/...], the HostHeaderWhitelistData
// schema): whether the domain is on the global Host allowlist, and — when it
// is not — which target CDN accounts may use it as their HTTP Host header.
type ArvanCloudHostHeaderWhitelist struct {
	// TargetAccounts is the sorted list of target CDN account UUIDs allowed
	// to use this domain as Host. Always empty when GloballyWhitelisted is
	// true (per the spec's own description).
	TargetAccounts []string
	// GloballyWhitelisted reports whether the domain is on the global Host
	// allowlist: if true, any account may use it as Host, and
	// TargetAccounts is not consulted.
	GloballyWhitelisted bool
}
