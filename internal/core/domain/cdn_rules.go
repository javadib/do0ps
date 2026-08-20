package domain

import "encoding/json"

// The types below model three of the CDN rule engines Parspack exposes on a
// zone (AGENTS.md 4.5, issue #24): Origin Rules (route a request to a
// specific origin), Page Rules (apply per-URL/user-agent settings overrides),
// and Transform Rules (rewrite request/response headers). Field sets are
// confirmed against docs/api-specs/parspack-cdn.openapi.yaml's "Origin
// Rules", "Page Rules" and "Transform Rules" tags.
//
// Origin Rules and Transform Rules both match traffic with a nested
// condition tree ("rule_items" in the spec); that shape is genuinely shared,
// so it is factored out once as CDNRuleCondition rather than duplicated.
// Page Rules match with a single flat type/operator/value instead — the spec
// gives it no rule_items field at all — so it is not forced into the same
// shape.

// CDNRuleCondition is one condition inside a rule's condition tree, shared by
// Origin Rules and Transform Rules. A rule's full tree is [][]CDNRuleCondition:
// the outer slice is ORed, and the conditions inside each inner slice are
// ANDed (confirmed by the spec's rule_items example and description).
//
// Field and Operation are not a closed enum in the spec (documented only as
// free-form strings via examples such as "full_url"/"contains",
// "country_code"/"equals", "user_agent"/"contains"), so they are not
// validated against a fixed list here — an unsupported combination is
// rejected by the provider with a 422.
//
// Value carries whatever JSON shape the operation expects (a single string,
// a number, or an array for "list"-style operations), so it is kept as raw
// JSON rather than forced into one Go type. ValueDetail, BulklistID,
// BulklistName and BulklistType are read-only: the provider only populates
// them on a get/list response (ValueDetail is a human-readable label for
// Value; the Bulklist* fields are set instead of Value when Operation is
// "from_list", referencing a named list of values managed elsewhere on the
// account).
type CDNRuleCondition struct {
	Field        string
	Operation    string
	Value        json.RawMessage
	ValueDetail  json.RawMessage
	BulklistID   string
	BulklistName string
	BulklistType string
}

// cdnOriginRuleTypes is the enum confirmed against the Origin Rules
// create/update request body's "type" field.
var cdnOriginRuleTypes = []string{"upstream", "port", "load_balance"}

// ValidCDNOriginRuleType reports whether s is one of the origin rule types
// the CDN API accepts.
func ValidCDNOriginRuleType(s string) bool { return contains(cdnOriginRuleTypes, s) }

// CDNOriginRule routes matching traffic in a zone to a specific origin:
// a fixed upstream IP, a port, or a configured load balancer, depending on
// Type. Only the field matching Type is meaningful (e.g. UpstreamIP is
// required and only used when Type is "upstream"); the API returns null for
// the other two on read.
//
// LoadBalanceName is read-only: the provider reports it (alongside
// LoadBalanceID) on get/list, but only accepts LoadBalanceID on write.
type CDNOriginRule struct {
	ID              string
	Name            string
	Type            string // "upstream", "port", or "load_balance"
	UpstreamIP      string // required when Type is "upstream"
	Port            int    // required when Type is "port"
	LoadBalanceID   string // required when Type is "load_balance"
	LoadBalanceName string // read-only
	Enabled         bool
	Priority        int // read-only: position among the zone's origin rules
	RuleItems       [][]CDNRuleCondition
}

// cdnPageRuleTypes and cdnPageRuleOperators are the enums confirmed against
// the Page Rules create/update request body's "type" and "operator" fields.
var (
	cdnPageRuleTypes     = []string{"url", "user_agent"}
	cdnPageRuleOperators = []string{"pattern", "equals", "not_equals", "contains", "not_contains"}
)

// ValidCDNPageRuleType reports whether s is one of the match types the CDN
// API's page rules accept.
func ValidCDNPageRuleType(s string) bool { return contains(cdnPageRuleTypes, s) }

// ValidCDNPageRuleOperator reports whether s is one of the match operators
// the CDN API's page rules accept.
func ValidCDNPageRuleOperator(s string) bool { return contains(cdnPageRuleOperators, s) }

// cdnPageRuleRedirectCodes is the enum confirmed against the Page Rules
// request body's "url_redirection.http_code" field.
var cdnPageRuleRedirectCodes = []int{301, 302}

// ValidCDNPageRuleRedirectCode reports whether code is one of the HTTP
// redirect status codes the CDN API's page rules accept.
func ValidCDNPageRuleRedirectCode(code int) bool {
	for _, c := range cdnPageRuleRedirectCodes {
		if c == code {
			return true
		}
	}
	return false
}

// CDNPageRuleRedirect is a page rule's optional URL redirection action
// ("url_redirection" in the spec).
type CDNPageRuleRedirect struct {
	HTTPCode int // 301 or 302
	URL      string
}

// CDNPagePortDestination is a page rule's optional origin port override
// ("port_destination" in the spec). A zero value means "not set" for that
// protocol.
type CDNPagePortDestination struct {
	HTTP  int
	HTTPS int
}

// CDNPageRuleCookie is one entry of a page rule's "addition_cookies": a
// cookie the CDN injects into matching responses.
type CDNPageRuleCookie struct {
	Key           string
	Value         string
	ExpireSeconds int // "expire_date" in the spec, despite the name it is seconds until expiry
	Path          string
	HTTPOnly      bool
	Secure        bool
}

// CDNPageRule matches traffic in a zone by a single condition (Type +
// Operator against Value, e.g. URL "pattern" match) and applies a bundle of
// setting overrides to what matches: caching, minification, firewall/WAF
// toggles, header rewriting, cookie injection, an optional redirect, and an
// optional origin port override. Unlike Origin/Transform Rules it has no
// nested condition tree — the spec gives page-rules a flat single match
// instead of "rule_items" — and no toggle-status endpoint, so there is no
// Enabled field: a page rule is removed (not disabled) to stop applying it.
//
// CachePolicy reuses DNSRecordProxy: the spec's "cache_policy" enum
// ("direct", "cdn-no-caching", "cdn-static-caching", "cdn-smart-caching",
// "cdn-always-caching") is identical to a DNS record's CDN proxy mode, so
// this is a genuinely shared shape rather than a duplicated one.
type CDNPageRule struct {
	ID                string
	Name              string
	Type              string // "url" or "user_agent"
	Operator          string // "pattern", "equals", "not_equals", "contains", "not_contains"
	Value             string
	Minify            bool
	CachePolicy       DNSRecordProxy
	CacheTTLSeconds   int // 0-86400
	FirewallStatus    bool
	FirewallExceptIPs []string
	WAFIsActive       bool
	RemovableHeaders  []string
	URLRedirection    *CDNPageRuleRedirect
	AdditionHeaders   map[string]string
	PortDestination   *CDNPagePortDestination
	AdditionCookies   []CDNPageRuleCookie
	Priority          int // read-only: position among the zone's page rules
}

// cdnTransformHeaderActions is the enum confirmed against the Transform
// Rules request body's "transform_rule_values.*.action" field.
var cdnTransformHeaderActions = []string{"modify", "delete"}

// ValidCDNTransformHeaderAction reports whether s is one of the actions the
// CDN API's transform rules accept for a header entry.
func ValidCDNTransformHeaderAction(s string) bool { return contains(cdnTransformHeaderActions, s) }

// CDNTransformHeaderAction is one entry of a transform rule's request or
// response header list: modify (set) or delete a named header on matching
// traffic. HeaderValue is required when Action is "modify" and ignored when
// Action is "delete".
type CDNTransformHeaderAction struct {
	HeaderName  string
	HeaderValue string
	Action      string // "modify" or "delete"
}

// CDNTransformRule rewrites request and/or response headers on traffic in a
// zone matched by a nested condition tree (RuleItems), the same shape Origin
// Rules use — see CDNRuleCondition's doc comment for how outer/inner groups
// combine.
type CDNTransformRule struct {
	ID              string
	Name            string
	Enabled         bool
	Priority        int // read-only: position among the zone's transform rules
	RequestHeaders  []CDNTransformHeaderAction
	ResponseHeaders []CDNTransformHeaderAction
	RuleItems       [][]CDNRuleCondition
}
