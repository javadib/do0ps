package domain

// The types below model the "Cache Management" tag of Parspack's CDN API
// (issue #24), confirmed against lines 1125-2072 of
// docs/api-specs/parspack-cdn.openapi.yaml. Every operation is scoped to a
// CDN zone (zone_uuid), consistent with the rest of the CDN surface
// (AGENTS.md 4.1, 4.5).
//
// The spec only exposes PUT for cache/ttl, cache/rule and cache/user-agent
// (no matching GET per-setting) and only GET for cache/settings (the
// aggregate view, no matching PUT) — so, unlike CDNZone's symmetric
// get/update pairs, these settings are write-only individually and
// read-only in aggregate. There is also no DELETE for a single
// cache/{id} entry in this block of the spec; only GET (show) and the
// zone-wide DELETE .../cache ("Destroy Cache") exist.

// cdnCacheRules is the fixed enum confirmed against the cache/rule request
// body and the cache_rule field of cache/settings' response.
var cdnCacheRules = []string{
	"cdn-no-caching", "cdn-static-caching", "cdn-smart-caching", "cdn-always-caching",
}

// ValidCDNCacheRule reports whether s is one of the cache rules the CDN
// API's cache/rule endpoint accepts.
func ValidCDNCacheRule(s string) bool { return contains(cdnCacheRules, s) }

// CDNCacheTTLSetting is the edge cache TTL of a zone (PUT
// .../cache/ttl). EdgeCacheTTLSeconds must be one of the fixed values
// ValidDNSRecordTTL also validates — the CDN API's spec documents the exact
// same TTL enum for DNS records and edge cache.
type CDNCacheTTLSetting struct {
	EdgeCacheTTLSeconds int
}

// CDNCacheRuleSetting is the zone-wide cache rule (PUT .../cache/rule).
type CDNCacheRuleSetting struct {
	CacheRule string // one of ValidCDNCacheRule's values, e.g. "cdn-smart-caching"
}

// CDNCacheUserAgentSetting is whether the CDN caches content separately per
// User-Agent header (PUT .../cache/user-agent).
type CDNCacheUserAgentSetting struct {
	Enabled bool
}

// CDNCacheSettings is the aggregate cache configuration of a zone (GET
// .../cache/settings). It is read-only as a whole; individual fields are
// changed through their own endpoints (cache/ttl, cache/rule,
// cache/user-agent) — the spec exposes no PUT for this combined resource.
type CDNCacheSettings struct {
	DeveloperMode           bool
	MaintenanceMode         bool
	IgnoreQueryString       bool
	CacheRule               string
	EdgeCacheTTLSeconds     int
	OriginOffline           bool
	EnableCachePerUserAgent bool
}

// CDNCacheEntry is one cache-clear ("purge") operation tracked against a
// zone, confirmed against GET .../cache (list, "Index Cache" in the spec)
// and GET .../cache/{id} (show, "Show Cache Management" in the spec).
// DELETE .../cache ("Destroy Cache") triggers a new one, but its own
// response carries no id — a caller that wants to track progress must list
// or show separately (see the PurgeCDNCache use case's doc comment).
type CDNCacheEntry struct {
	ID              string
	Operation       string // e.g. "Purge All"
	Status          string // e.g. "none"; the spec does not enumerate every value
	CreatedTime     string // as reported by the provider, e.g. "2023-07-17 08:48:15"
	SuccessProgress int    // 0-100
}
