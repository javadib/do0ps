package domain

// The types below model the "Network" tag of Parspack's CDN API surface
// (AGENTS.md 4.5, issue #24's slice of the 18 CDN tags left out of issue
// #19's scope): four zone-level settings pairs, each a simple get/update
// toggle or enum. Confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml, lines 8223-9279.

// CDNHTTPSConvertorSetting controls whether Parspack automatically rewrites
// HTTP links to HTTPS in a zone's served content. Confirmed against GET/PUT
// /external/api/v1/zones/{zone_uuid}/https-convertor.
type CDNHTTPSConvertorSetting struct {
	Enabled bool
}

// CDNEdgeToUpstreamConnectionSetting controls which protocol Parspack's edge
// nodes use when connecting to the origin server for a zone. Confirmed
// against GET/PUT /external/api/v1/zones/{zone_uuid}/edge-to-upstream-connection.
type CDNEdgeToUpstreamConnectionSetting struct {
	Type string // "auto", "http", or "https"
}

// cdnEdgeToUpstreamConnectionTypes is the enum confirmed against the
// edge-to-upstream-connection update request body.
var cdnEdgeToUpstreamConnectionTypes = []string{"auto", "http", "https"}

// ValidCDNEdgeToUpstreamConnectionType reports whether s is one of the
// connection types the edge-to-upstream-connection endpoint accepts.
func ValidCDNEdgeToUpstreamConnectionType(s string) bool {
	return contains(cdnEdgeToUpstreamConnectionTypes, s)
}

// CDNWWWRedirectionSetting controls whether Parspack redirects a zone's
// traffic between its www and non-www hostnames. Confirmed against GET/PUT
// /external/api/v1/zones/{zone_uuid}/www-redirection.
type CDNWWWRedirectionSetting struct {
	Mode string // "none", "redirect-to-www", or "redirect-from-www"
}

// cdnWWWRedirectionModes is the enum confirmed against the www-redirection
// update request body's "www_redirection" field.
var cdnWWWRedirectionModes = []string{"none", "redirect-to-www", "redirect-from-www"}

// ValidCDNWWWRedirectionMode reports whether s is one of the modes the
// www-redirection endpoint accepts.
func ValidCDNWWWRedirectionMode(s string) bool {
	return contains(cdnWWWRedirectionModes, s)
}

// CDNWebSocketSetting controls whether WebSocket connections are allowed
// through a zone. Confirmed against GET/PUT
// /external/api/v1/zones/{zone_uuid}/web-socket.
type CDNWebSocketSetting struct {
	Enabled bool
}
