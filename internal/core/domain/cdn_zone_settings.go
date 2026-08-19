package domain

// The types below model the CDN zone-level settings/toggles that sit outside
// issue #19's zone/order/DNS scope (AGENTS.md 4.5, issue #24): antivirus,
// DNSSEC, asset optimization, developer mode, maintenance mode, query-string
// caching behavior, and origin-offline handling. Field sets are confirmed
// against docs/api-specs/parspack-cdn.openapi.yaml's Antivirus, Dns,
// Optimization and "Service Settings" tags.
//
// Most of these settings are a bare enabled/disabled flag, so no domain type
// is needed for them — the adapter and use case methods pass/return a plain
// bool. Only DNSSEC and Optimization return more than a flag, so only those
// two get a dedicated struct here.

// CDNDNSSecStatus is a CDN zone's DNSSEC status, confirmed against
// GET/PUT .../dns-sec. Value carries the DS record the domain registrar must
// be given once DNSSEC is enabled; it is empty while DNSSEC is disabled.
type CDNDNSSecStatus struct {
	Enabled bool
	Value   string // e.g. "example.com. 3600 IN DS 12345 8 2 ABCD"
}

// CDNOptimizationStatus is a CDN zone's asset optimization configuration,
// confirmed against GET .../optimization/status and PUT
// .../optimization/update. The provider nests the three minification flags
// under a "website_minification" object; this type flattens them since
// there is no other use for that nesting on this side of the adapter
// boundary.
type CDNOptimizationStatus struct {
	ImageMinification bool
	WebPConversion    bool
	MinifyHTML        bool
	MinifyCSS         bool
	MinifyJS          bool
}
