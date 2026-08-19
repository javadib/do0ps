package domain

// The types below model two more corners of Parspack's CDN edge-firewall
// surface (issue #24, AGENTS.md 4.5): Rate Limit Rules and the zone-wide
// Upstream Errors toggle. Field sets are confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml's "Rate Limit Rule" and "Upstream
// Errors" tags. Both are scoped to a CDN zone (zone_uuid), same as DNS
// records (AGENTS.md 4.1) — neither is a standalone product.

// CDNRateLimitWhitelistIP is one address exempt from a rate limit rule, as
// reported back by the provider on list/get. ID and RateLimitRuleID are
// provider-assigned and read-only; a create/update request instead sends
// this rule's whitelist as a plain list of IP strings (see
// CDNRateLimitRule.WhitelistIPs and the adapter's wire translation).
type CDNRateLimitWhitelistIP struct {
	ID              int
	IP              string
	RateLimitRuleID int
}

// CDNRateLimitRule is one entry of a CDN zone's edge-firewall rate limiting,
// confirmed against the "Rate Limit Rule" tag's index/show/store/update
// endpoints. It matches requests against Value (e.g. a URL glob) and applies
// two independent thresholds:
//   - "static": a fixed request budget per StaticInterval.
//   - "dynamic": a budget that additionally reacts to traffic patterns
//     (used together with IPReputationEnabled).
//
// StaticRequests and DynamicRequests are only ever reported by the provider
// on read (list/get) — the store/update request bodies do not accept them,
// so they are left at their zero value when building a rule to send.
type CDNRateLimitRule struct {
	ID                  string
	Name                string
	Value               string
	Enabled             bool
	Priority            int
	StaticIntervalType  string // "second" or "minute"
	StaticInterval      int
	StaticRequests      int    // read-only: reported by the provider, not settable
	DynamicIntervalType string // "second", "minute", "hour" or "day"
	DynamicInterval     int
	DynamicRequests     int // read-only: reported by the provider, not settable
	IPReputationEnabled bool
	Challenge           string // "js", "block", "captcha", "allow" or "bypass"
	TrustTime           int    // seconds a passed challenge is trusted before re-challenging
	AttackBanTime       int    // seconds an offending client is banned for
	WhitelistIPs        []CDNRateLimitWhitelistIP
}

// cdnRateLimitStaticIntervalTypes, cdnRateLimitDynamicIntervalTypes and
// cdnRateLimitChallenges are the enums confirmed against the Rate Limit
// Rule store/update request bodies.
var (
	cdnRateLimitStaticIntervalTypes  = []string{"second", "minute"}
	cdnRateLimitDynamicIntervalTypes = []string{"second", "minute", "hour", "day"}
	cdnRateLimitChallenges           = []string{"js", "block", "captcha", "allow", "bypass"}
)

// ValidCDNRateLimitStaticIntervalType reports whether s is one of the static
// interval types the rate limit rule API accepts.
func ValidCDNRateLimitStaticIntervalType(s string) bool {
	return contains(cdnRateLimitStaticIntervalTypes, s)
}

// ValidCDNRateLimitDynamicIntervalType reports whether s is one of the
// dynamic interval types the rate limit rule API accepts.
func ValidCDNRateLimitDynamicIntervalType(s string) bool {
	return contains(cdnRateLimitDynamicIntervalTypes, s)
}

// ValidCDNRateLimitChallenge reports whether s is one of the challenge
// actions the rate limit rule API accepts.
func ValidCDNRateLimitChallenge(s string) bool { return contains(cdnRateLimitChallenges, s) }

// CDNUpstreamErrorSettings is a CDN zone's "Upstream Errors" setting,
// confirmed against the "Upstream Errors" tag's single GET/PUT resource —
// there is no per-status-code custom error page configuration on this
// endpoint, only a single zone-wide enabled flag.
type CDNUpstreamErrorSettings struct {
	Enabled bool
}
