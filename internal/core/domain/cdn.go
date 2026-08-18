package domain

// The types below model Parspack's CDN API surface (AGENTS.md 4.5): a CDN
// "zone" is a CDN-onboarded domain, and DNS records live inside a zone, not
// as a standalone resource (AGENTS.md 4.1). Field sets are confirmed against
// docs/api-specs/cdn-api.openapi's Service/Order/Dns tags — issue #19's scope
// is deliberately limited to zone/order management and DNS records; the
// other 18 CDN tags (Firewall, Load Balance, Cache, WAF, ...) are tracked in
// issue #24.

// CDNZone is a domain onboarded onto Parspack's CDN. UUID is the identifier
// every zone-scoped endpoint (including DNS records) is keyed on; ID is the
// shorter display id the list endpoint also reports.
type CDNZone struct {
	UUID          string
	ID            string
	Domain        string
	Status        string // e.g. "active", "setup_required", "payment_pending", "pending"
	Plan          string // "free", "standard", "premium", "professional"
	BillingCycle  string // "free", "monthly", "quarterly", "semiannually", "annually"
	Proxy         bool
	NSStatus      bool // whether the domain's nameservers already point at Parspack
	RemainingDays int
	ExpireAt      string // as reported by the provider, e.g. "2024-01-13"
}

// CDNZoneSpec is the normalized request to onboard a new domain onto the CDN
// (POST /external/api/v2/orders). Records is optional: the provider creates
// default MX/NS records for the zone when none are supplied.
type CDNZoneSpec struct {
	Domain        string
	Plan          string
	BillingCycle  string
	PromotionCode string
	Records       []DNSRecord
}

// cdnZonePlans and cdnBillingCycles are the enums confirmed against the
// Order endpoints (create-zone request body and list-packages response).
var (
	cdnZonePlans     = []string{"free", "standard", "premium", "professional"}
	cdnBillingCycles = []string{"free", "monthly", "quarterly", "semiannually", "annually"}
)

// ValidCDNZonePlan reports whether s is one of the plans the CDN order API
// accepts.
func ValidCDNZonePlan(s string) bool { return contains(cdnZonePlans, s) }

// ValidCDNBillingCycle reports whether s is one of the billing cycles the
// CDN order API accepts.
func ValidCDNBillingCycle(s string) bool { return contains(cdnBillingCycles, s) }

func contains(values []string, s string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}

// CDNZonePlanPricing is one entry of list_cdn_plans, confirmed against
// GET /external/api/v1/orders/packages.
type CDNZonePlanPricing struct {
	Plan         string
	Currency     string
	Monthly      int
	Quarterly    int
	Semiannually int
	Annually     int
}

// NameserverRecords are the nameservers a zone's domain registrar must be
// pointed at, confirmed against GET .../orders/{zone_uuid}/ns-records.
// CurrentNS reports what the registrar currently has configured, when the
// provider knows it, so a caller can tell whether the domain still needs to
// be repointed.
type NameserverRecords struct {
	NS1, NS2, NS3, NS4 string
	CurrentNS          []string
}

// DNSRecordType is the type of a DNS record. Confirmed against the CDN API's
// dns-records endpoints: A, CNAME, MX, TXT, NS, SRV and CAA are the only
// types the provider accepts for a zone's records.
type DNSRecordType int

const (
	DNSRecordTypeUnknown DNSRecordType = iota
	DNSRecordTypeA
	DNSRecordTypeCNAME
	DNSRecordTypeMX
	DNSRecordTypeTXT
	DNSRecordTypeNS
	DNSRecordTypeSRV
	DNSRecordTypeCAA
)

// String returns the canonical DNS record type name.
func (t DNSRecordType) String() string {
	switch t {
	case DNSRecordTypeA:
		return "A"
	case DNSRecordTypeCNAME:
		return "CNAME"
	case DNSRecordTypeMX:
		return "MX"
	case DNSRecordTypeTXT:
		return "TXT"
	case DNSRecordTypeNS:
		return "NS"
	case DNSRecordTypeSRV:
		return "SRV"
	case DNSRecordTypeCAA:
		return "CAA"
	default:
		return "UNKNOWN"
	}
}

// ParseDNSRecordType converts a record type name into a DNSRecordType,
// rejecting anything outside the confirmed enum so a bad value fails fast
// instead of reaching the provider.
func ParseDNSRecordType(s string) (DNSRecordType, error) {
	switch s {
	case "A":
		return DNSRecordTypeA, nil
	case "CNAME":
		return DNSRecordTypeCNAME, nil
	case "MX":
		return DNSRecordTypeMX, nil
	case "TXT":
		return DNSRecordTypeTXT, nil
	case "NS":
		return DNSRecordTypeNS, nil
	case "SRV":
		return DNSRecordTypeSRV, nil
	case "CAA":
		return DNSRecordTypeCAA, nil
	default:
		return DNSRecordTypeUnknown, ErrInvalidInput
	}
}

// DNSRecordProxy is the CDN proxy mode applied to a record, confirmed against
// the dns-records request body's "proxy" enum.
type DNSRecordProxy int

const (
	DNSRecordProxyUnknown DNSRecordProxy = iota
	DNSRecordProxyDirect
	DNSRecordProxyCDNNoCaching
	DNSRecordProxyCDNStaticCaching
	DNSRecordProxyCDNSmartCaching
	DNSRecordProxyCDNAlwaysCaching
)

// String returns the wire value of the proxy mode.
func (p DNSRecordProxy) String() string {
	switch p {
	case DNSRecordProxyDirect:
		return "direct"
	case DNSRecordProxyCDNNoCaching:
		return "cdn-no-caching"
	case DNSRecordProxyCDNStaticCaching:
		return "cdn-static-caching"
	case DNSRecordProxyCDNSmartCaching:
		return "cdn-smart-caching"
	case DNSRecordProxyCDNAlwaysCaching:
		return "cdn-always-caching"
	default:
		return "unknown"
	}
}

// ParseDNSRecordProxy converts a proxy mode name into a DNSRecordProxy,
// rejecting anything outside the confirmed enum.
func ParseDNSRecordProxy(s string) (DNSRecordProxy, error) {
	switch s {
	case "direct":
		return DNSRecordProxyDirect, nil
	case "cdn-no-caching":
		return DNSRecordProxyCDNNoCaching, nil
	case "cdn-static-caching":
		return DNSRecordProxyCDNStaticCaching, nil
	case "cdn-smart-caching":
		return DNSRecordProxyCDNSmartCaching, nil
	case "cdn-always-caching":
		return DNSRecordProxyCDNAlwaysCaching, nil
	default:
		return DNSRecordProxyUnknown, ErrInvalidInput
	}
}

// validDNSRecordTTLs is the fixed TTL enum confirmed against the dns-records
// request body — arbitrary TTLs are rejected by the provider, so this lets
// callers fail fast instead of relying on its 422 response.
var validDNSRecordTTLs = []int{
	1, 2, 5, 10, 30, 60, 180, 300, 600, 900, 1800, 2700, 3600,
	10800, 18000, 36000, 43200, 86400, 259200, 604800, 864000, 1296000, 2592000,
}

// ValidDNSRecordTTL reports whether ttl is one of the values the CDN API's
// dns-records endpoints accept.
func ValidDNSRecordTTL(ttl int) bool {
	for _, v := range validDNSRecordTTLs {
		if v == ttl {
			return true
		}
	}
	return false
}

// DNSRecordValue is one value under a DNSRecord's host+type — the CDN API
// nests these under "record" (single value, create) or "records" (list,
// index/update) because a create with a repeated host+type appends a value
// rather than erroring (e.g. multiple NS or MX records for the same host).
type DNSRecordValue struct {
	Content  string
	Port     int    // SRV only
	Weight   int    // SRV only
	Priority int    // MX, SRV only
	Flags    int    // CAA only
	Tag      string // CAA only: "issue", "issuewild", "iodef"
	Disabled bool
}

// DNSRecord is a DNS record inside a CDN zone. It always belongs to a zone
// (ZoneUUID) — Parspack has no standalone DNS product (AGENTS.md 4.1).
type DNSRecord struct {
	ZoneUUID      string
	Host          string // "@" means the zone apex
	Type          DNSRecordType
	TTL           int
	Proxy         DNSRecordProxy
	LoadBalanceID string // optional; empty when the record is not tied to a CDN load balancer
	Values        []DNSRecordValue
}
