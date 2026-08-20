package domain

import "strings"

// The types below model ArvanCloud's DNS record, DNSSEC and Secondary DNS
// capabilities (issue #63), scoped to a domain by name exactly like
// arvancloud.go and arvancloud_domain.go — DNS is not a standalone product
// for ArvanCloud either (AGENTS.md 4.1). Confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "DNS Management" and
// "Secondary DNS" tags: the dns-records.* and domains.secondary_dns.*
// operationIds, and the BaseDnsRecord/DnsRecord/DnsSec/SecondaryDNSData
// schemas.
//
// This is deliberately a fresh type, not an extension of DNSRecord
// (Parspack's CDN-zone-scoped record from cdn.go): the shapes diverge too far
// to share one Go type. A record here is addressed by a per-record UUID
// (unlike Parspack, where host+type addresses a record and a repeated create
// appends a value); the TTL enum is different; "cloud" is a binary CDN-proxy
// flag rather than Parspack's 5-value proxy enum; and, most importantly, the
// record's "value" shape depends on its type across 13 different record
// types instead of Parspack's 7. See ArvanCloudDNSRecordValue's doc comment
// for how that union is modeled.

// ArvanCloudDNSRecordType is the type of an ArvanCloud DNS record. Confirmed
// against the spec's DnsRecord oneOf (A-Record, AAAA-Record, ..., CAA-Record)
// — these 13 are the only types the CDN API accepts for a domain's records.
type ArvanCloudDNSRecordType int

const (
	ArvanCloudDNSRecordTypeUnknown ArvanCloudDNSRecordType = iota
	ArvanCloudDNSRecordTypeA
	ArvanCloudDNSRecordTypeAAAA
	ArvanCloudDNSRecordTypeCNAME
	ArvanCloudDNSRecordTypeANAME
	ArvanCloudDNSRecordTypeMX
	ArvanCloudDNSRecordTypeSRV
	ArvanCloudDNSRecordTypeTXT
	ArvanCloudDNSRecordTypeSPF
	ArvanCloudDNSRecordTypeDKIM
	ArvanCloudDNSRecordTypeNS
	ArvanCloudDNSRecordTypePTR
	ArvanCloudDNSRecordTypeTLSA
	ArvanCloudDNSRecordTypeCAA
)

// String returns the record type's wire value — the lowercase string the CDN
// API sends and expects in the record's "type" field (e.g. the A-Record
// schema's "type" enum default, "a").
func (t ArvanCloudDNSRecordType) String() string {
	switch t {
	case ArvanCloudDNSRecordTypeA:
		return "a"
	case ArvanCloudDNSRecordTypeAAAA:
		return "aaaa"
	case ArvanCloudDNSRecordTypeCNAME:
		return "cname"
	case ArvanCloudDNSRecordTypeANAME:
		return "aname"
	case ArvanCloudDNSRecordTypeMX:
		return "mx"
	case ArvanCloudDNSRecordTypeSRV:
		return "srv"
	case ArvanCloudDNSRecordTypeTXT:
		return "txt"
	case ArvanCloudDNSRecordTypeSPF:
		return "spf"
	case ArvanCloudDNSRecordTypeDKIM:
		return "dkim"
	case ArvanCloudDNSRecordTypeNS:
		return "ns"
	case ArvanCloudDNSRecordTypePTR:
		return "ptr"
	case ArvanCloudDNSRecordTypeTLSA:
		return "tlsa"
	case ArvanCloudDNSRecordTypeCAA:
		return "caa"
	default:
		return "unknown"
	}
}

// ParseArvanCloudDNSRecordType converts a record type name into an
// ArvanCloudDNSRecordType, rejecting anything outside the confirmed 13-value
// enum so a bad or misspelled type fails fast here instead of reaching the
// provider and coming back as a 422. Matching is case-insensitive: a caller
// (or the calling chatbot) writing "A" or "CNAME" is as valid as the wire's
// own lowercase form.
func ParseArvanCloudDNSRecordType(s string) (ArvanCloudDNSRecordType, error) {
	switch strings.ToLower(s) {
	case "a":
		return ArvanCloudDNSRecordTypeA, nil
	case "aaaa":
		return ArvanCloudDNSRecordTypeAAAA, nil
	case "cname":
		return ArvanCloudDNSRecordTypeCNAME, nil
	case "aname":
		return ArvanCloudDNSRecordTypeANAME, nil
	case "mx":
		return ArvanCloudDNSRecordTypeMX, nil
	case "srv":
		return ArvanCloudDNSRecordTypeSRV, nil
	case "txt":
		return ArvanCloudDNSRecordTypeTXT, nil
	case "spf":
		return ArvanCloudDNSRecordTypeSPF, nil
	case "dkim":
		return ArvanCloudDNSRecordTypeDKIM, nil
	case "ns":
		return ArvanCloudDNSRecordTypeNS, nil
	case "ptr":
		return ArvanCloudDNSRecordTypePTR, nil
	case "tlsa":
		return ArvanCloudDNSRecordTypeTLSA, nil
	case "caa":
		return ArvanCloudDNSRecordTypeCAA, nil
	default:
		return ArvanCloudDNSRecordTypeUnknown, ErrInvalidInput
	}
}

// validArvanCloudDNSRecordTTLs is the fixed TTL enum confirmed against
// BaseDnsRecord's "ttl" property — completely different from Parspack's
// validDNSRecordTTLs in cdn.go, so it is deliberately not shared with it.
var validArvanCloudDNSRecordTTLs = []int{
	120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400, 172800, 432000,
}

// ValidArvanCloudDNSRecordTTL reports whether ttl is one of the values the
// CDN API's DNS record endpoints accept for an ArvanCloud domain.
func ValidArvanCloudDNSRecordTTL(ttl int) bool {
	for _, v := range validArvanCloudDNSRecordTTLs {
		if v == ttl {
			return true
		}
	}
	return false
}

// The upstream_https modes a record accepts, confirmed against
// BaseDnsRecord's "upstream_https" enum.
const (
	ArvanCloudUpstreamHTTPSDefault = "default"
	ArvanCloudUpstreamHTTPSAuto    = "auto"
	ArvanCloudUpstreamHTTPSHTTP    = "http"
	ArvanCloudUpstreamHTTPSHTTPS   = "https"
)

var arvanCloudUpstreamHTTPSModes = []string{
	ArvanCloudUpstreamHTTPSDefault, ArvanCloudUpstreamHTTPSAuto, ArvanCloudUpstreamHTTPSHTTP, ArvanCloudUpstreamHTTPSHTTPS,
}

// ValidArvanCloudUpstreamHTTPS reports whether s is one of the
// upstream_https modes the CDN API accepts. An empty string is not
// validated here — it means "leave this field unset", left to the caller to
// decide whether that is acceptable.
func ValidArvanCloudUpstreamHTTPS(s string) bool { return contains(arvanCloudUpstreamHTTPSModes, s) }

// The ip_filter_mode sub-fields' enums, confirmed against
// DnsRecordIpFilterMode.
const (
	ArvanCloudIPFilterCountSingle = "single"
	ArvanCloudIPFilterCountMulti  = "multi"

	ArvanCloudIPFilterOrderNone     = "none"
	ArvanCloudIPFilterOrderWeighted = "weighted"
	ArvanCloudIPFilterOrderRR       = "rr"

	ArvanCloudIPFilterGeoNone     = "none"
	ArvanCloudIPFilterGeoLocation = "location"
	ArvanCloudIPFilterGeoCountry  = "country"
)

var (
	arvanCloudIPFilterCounts = []string{ArvanCloudIPFilterCountSingle, ArvanCloudIPFilterCountMulti}
	arvanCloudIPFilterOrders = []string{ArvanCloudIPFilterOrderNone, ArvanCloudIPFilterOrderWeighted, ArvanCloudIPFilterOrderRR}
	arvanCloudIPFilterGeos   = []string{ArvanCloudIPFilterGeoNone, ArvanCloudIPFilterGeoLocation, ArvanCloudIPFilterGeoCountry}
)

// ValidArvanCloudIPFilterCount reports whether s is one of
// DnsRecordIpFilterMode's "count" values. An empty string means "unset".
func ValidArvanCloudIPFilterCount(s string) bool { return contains(arvanCloudIPFilterCounts, s) }

// ValidArvanCloudIPFilterOrder reports whether s is one of
// DnsRecordIpFilterMode's "order" values. An empty string means "unset".
func ValidArvanCloudIPFilterOrder(s string) bool { return contains(arvanCloudIPFilterOrders, s) }

// ValidArvanCloudIPFilterGeo reports whether s is one of
// DnsRecordIpFilterMode's "geo_filter" values. An empty string means
// "unset".
func ValidArvanCloudIPFilterGeo(s string) bool { return contains(arvanCloudIPFilterGeos, s) }

// ArvanCloudDNSRecordIPFilterMode controls how multiple A/AAAA values under
// one record are selected and geo-targeted (BaseDnsRecord's
// "ip_filter_mode"). Every field is optional; the zero value (all empty
// strings) means "leave this to the provider's default".
type ArvanCloudDNSRecordIPFilterMode struct {
	// Count is "single" or "multi": whether the provider answers with one IP
	// or every matching IP.
	Count string
	// Order is "none", "weighted" or "rr" (round-robin): how multiple
	// candidate IPs are ordered before Count is applied.
	Order string
	// GeoFilter is "none", "location" or "country": whether IP selection is
	// narrowed by the resolver's geographic origin.
	GeoFilter string
}

// The tags a CAA record's value may carry, confirmed against
// CAA-RecordValue's "tag" enum.
const (
	ArvanCloudCAATagIssue     = "issue"
	ArvanCloudCAATagIssueWild = "issuewild"
	ArvanCloudCAATagIODEF     = "iodef"
)

var arvanCloudCAATags = []string{ArvanCloudCAATagIssue, ArvanCloudCAATagIssueWild, ArvanCloudCAATagIODEF}

// ValidArvanCloudCAATag reports whether s is one of the tags a CAA record's
// value accepts.
func ValidArvanCloudCAATag(s string) bool { return contains(arvanCloudCAATags, s) }

// The host_header modes CNAME and ANAME records accept, confirmed against
// CNAME-RecordValue/ANAME-RecordValue's "host_header" enum.
const (
	ArvanCloudHostHeaderSource = "source"
	ArvanCloudHostHeaderDest   = "dest"
)

var arvanCloudHostHeaders = []string{ArvanCloudHostHeaderSource, ArvanCloudHostHeaderDest}

// ValidArvanCloudHostHeader reports whether s is one of the host_header
// modes a CNAME or ANAME record's value accepts. An empty string means
// "unset" (the provider applies its own default).
func ValidArvanCloudHostHeader(s string) bool { return contains(arvanCloudHostHeaders, s) }

// ArvanCloudDNSRecordValue is one value entry of an ArvanCloudDNSRecord.
//
// The CDN API's "value" shape depends on the record's Type (13 different
// per-type schemas — see this file's package-level comment), which this
// struct covers as one flattened union: every field below belongs to a
// specific subset of record types, documented per field, and only those
// fields are meaningful for a given record's Type. This keeps the domain
// layer provider-shape-aware only at the level ArvanCloud's own spec already
// is (a fixed 13-type union), while leaving wire concerns — encoding this as
// an array for A/AAAA vs. a single JSON object for every other type, and
// picking the right field names — entirely to
// internal/adapters/providers/arvancloud/dns.go, per AGENTS.md 4.0's
// layering rule.
//
// A and AAAA records carry one ArvanCloudDNSRecordValue per IP (multiple
// weighted/geo-targeted addresses under one record); every other type
// carries exactly one.
type ArvanCloudDNSRecordValue struct {
	// IP is the address. A/AAAA only.
	IP string
	// Port is a target port. A, AAAA, CNAME, ANAME, SRV.
	Port int
	// Weight influences selection among multiple values. A, AAAA, SRV.
	Weight int
	// OriginalWeight is read-only: reported only when Health Check has
	// changed Weight from what was set. A, AAAA.
	OriginalWeight int
	// Country is an ISO 3166 alpha-2 code for geo-targeting. A, AAAA.
	Country string

	// Host is the target hostname. CNAME ("host"), MX ("host"), NS ("host") —
	// three different record types reuse this Go field because the spec
	// itself reuses the JSON key "host" across A-RecordValue-like value
	// schemas that are not this record's own value; the adapter reads the
	// right one for whichever Type this value belongs to.
	Host string
	// HostHeader is "source" or "dest" (ArvanCloudHostHeaderSource /
	// ArvanCloudHostHeaderDest). CNAME, ANAME.
	HostHeader string
	// Location is the target FQDN. ANAME only (the spec's "location" field,
	// analogous to CNAME's "host").
	Location string

	// Priority orders MX/SRV targets, lower first. MX, SRV.
	Priority int

	// Target is the service hostname. SRV only.
	Target string

	// Text is the record's text payload, max 500 chars per the spec. TXT,
	// SPF, DKIM (the spec aliases SPF/DKIM's value schema onto TXT's).
	Text string

	// Domain is the pointer target. PTR only; optional per the spec (PTR is
	// the one type with no required value sub-fields).
	Domain string

	// Usage, Selector, MatchingType and Certificate are TLSA's four fields,
	// all required per the spec despite their short max lengths. TLSA only.
	Usage        string
	Selector     string
	MatchingType string
	Certificate  string

	// CAAValue is CAA's "value" field: a domain string naming the
	// certificate authority allowed to issue for this name (spec property
	// name is literally "value", renamed here to avoid colliding with this
	// type's own name). CAA only.
	CAAValue string
	// Tag is CAA's authorization tag: "issue", "issuewild" or "iodef"
	// (ArvanCloudCAATagIssue / ArvanCloudCAATagIssueWild /
	// ArvanCloudCAATagIODEF). CAA only.
	Tag string
}

// ArvanCloudDNSRecord is a DNS record scoped to an ArvanCloud domain by name
// (GET/POST/PUT/DELETE /domains/{domain}/dns-records[/{id}], the
// DnsRecordGeneric response / DnsRecord request schemas).
type ArvanCloudDNSRecord struct {
	// ID is the provider-assigned UUID. Read-only; empty until the record has
	// been created.
	ID string

	// Name is the record's hostname, max 250 chars per the spec (e.g. "@" for
	// the zone apex, "www", or a fully-qualified subdomain).
	Name string

	// Type selects which of the 13 value shapes Values must satisfy.
	Type ArvanCloudDNSRecordType

	// TTL must be one of ValidArvanCloudDNSRecordTTL's fixed values.
	TTL int

	// Cloud is ArvanCloud's CDN-proxy toggle for this record — the
	// equivalent of Parspack's 5-value DNSRecordProxy enum, but binary here:
	// true routes traffic through ArvanCloud's CDN, false answers with the
	// record's raw value.
	Cloud bool

	// UpstreamHTTPS is one of ArvanCloudUpstreamHTTPSDefault/Auto/HTTP/HTTPS,
	// or empty to leave it unset.
	UpstreamHTTPS string

	// IPFilterMode controls multi-value A/AAAA selection and geo-targeting.
	// Meaningful mainly for A/AAAA records; the zero value leaves it unset.
	IPFilterMode ArvanCloudDNSRecordIPFilterMode

	// IsProtected is read-only: true means the provider refuses to modify or
	// delete this record. Callers see that refusal as a normal
	// ErrInvalidInput/ErrProviderUnavailable from the provider — this field
	// is informational only, never enforced client-side (issue #63).
	IsProtected bool

	// Usage is read-only: non-empty when something else (e.g. an issued TLS
	// certificate, per the spec's "certificate-issuance" value) references
	// this record. A referenced record may not simply be deletable; that
	// too surfaces as whatever error the provider returns, not a local
	// pre-check.
	Usage []string

	// Values holds this record's type-specific data — one entry per IP for
	// A/AAAA, exactly one entry for every other type. See
	// ArvanCloudDNSRecordValue's doc comment.
	Values []ArvanCloudDNSRecordValue

	// CreatedAt and UpdatedAt are provider-reported RFC 3339 timestamps,
	// passed through as-is like ArvanCloudDomain's.
	CreatedAt string
	UpdatedAt string
}

// ArvanCloudSecondaryDNSSkippedRecord is one entry of a Secondary DNS
// config's read-only "errors.skipped_records" — a record the provider could
// not carry over from the secondary nameserver.
type ArvanCloudSecondaryDNSSkippedRecord struct {
	Name  string
	Type  string
	Value string
}

// ArvanCloudSecondaryDNSConfig is a domain's Secondary DNS configuration
// (GET/POST/DELETE /domains/{domain}/secondary-dns, the SecondaryDNSData
// schema): ArvanCloud acts as a secondary nameserver, transferring zone data
// from Nameserver via AXFR/IXFR.
type ArvanCloudSecondaryDNSConfig struct {
	// Status is whether Secondary DNS is enabled for the domain.
	Status bool

	// Nameserver is the primary nameserver ArvanCloud transfers the zone
	// from.
	Nameserver string

	// SOASerial is the last transferred zone's SOA serial. Read-only.
	SOASerial string

	// ErrorMessage and SkippedRecords are the read-only "errors" object's
	// contents, reported when the last transfer only partially succeeded.
	// ErrorMessage is empty when there was no error to report.
	ErrorMessage   string
	SkippedRecords []ArvanCloudSecondaryDNSSkippedRecord

	// CreatedAt and UpdatedAt are provider-reported RFC 3339 timestamps.
	CreatedAt string
	UpdatedAt string
}

// ArvanCloudDNSSecStatus is a domain's DNSSEC status (GET
// /domains/{domain}/dns-records/dnssec and PUT .../dnssec/actions, the DnsSec
// schema).
type ArvanCloudDNSSecStatus struct {
	// Enabled reports whether DNSSEC is turned on for the domain.
	Enabled bool
	// DS is the DS record the domain's registrar must publish for DNSSEC to
	// validate, empty when DNSSEC is disabled or the provider has not
	// generated one yet.
	DS string
}
