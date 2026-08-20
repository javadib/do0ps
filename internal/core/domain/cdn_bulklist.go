package domain

// The types below model two more of Parspack's CDN API surfaces (AGENTS.md
// 4.5, issue #24): Bulklist and Country. Field sets are confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml's Bulklist and Country tags.
//
// A bulklist is a reusable list of values (IPs, countries, user agents or
// referrers) that other CDN features — firewall rules, for one — reference
// by ID instead of inlining the values themselves. Unlike CDNZone and
// DNSRecord, a bulklist is scoped to the user's account, not to a zone: its
// endpoints carry no zone_uuid.
//
// Country is a read-only reference list of countries, used to populate
// country-based firewall rules for a zone.

// CDNBulklistItem is one value inside a bulklist, as reported by the
// list/get endpoints. ValueDetail is a human-readable label the provider
// attaches for some types (e.g. a country name alongside a country-code
// value); it is empty when the provider has none to offer.
type CDNBulklistItem struct {
	Value       string
	ValueDetail string
}

// CDNBulklist is a reusable IP/country/user-agent/referral list, scoped to
// the user's account rather than to a CDN zone.
type CDNBulklist struct {
	ID    string
	Name  string
	Type  string // "ip", "user_agent", "country", "referral"
	Items []CDNBulklistItem
}

// CDNBulklistSpec is the normalized request to create or update a bulklist.
// Items are plain values (e.g. "192.168.0.1" or a country code) — the
// provider only reports a value_detail on read, it does not accept one on
// write.
type CDNBulklistSpec struct {
	Name  string
	Type  string
	Items []string
}

// cdnBulklistTypes is the enum confirmed against the Bulklist API's
// create/update request body.
var cdnBulklistTypes = []string{"ip", "user_agent", "country", "referral"}

// ValidCDNBulklistType reports whether s is one of the types the Bulklist
// API accepts.
func ValidCDNBulklistType(s string) bool { return contains(cdnBulklistTypes, s) }

// CDNCountry is one entry of the CDN firewall country reference list
// (GET /zones/{zone_uuid}/firewalls/countries), used to populate
// country-based firewall rules. Code is the provider's opaque country
// identifier (e.g. "1"), not an ISO code.
type CDNCountry struct {
	Code string
	Name string
}
