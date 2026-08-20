package domain

// The types below model ArvanCloud's CDN API surface
// (docs/api-specs/arvancloud-cdn-4.0.yml, "ArvanCloud CDN Services" 4.180.1),
// the second provider this project implements after Parspack (AGENTS.md 4.1).
//
// Two shape differences with Parspack drive everything in this file and the
// arvancloud_*.go files that follow it:
//
//  1. Resources are addressed by the domain NAME, not by a UUID. A domain does
//     carry an internal id, but every sub-resource path is
//     /domains/{domain}/... with {domain} being e.g. "example.com". There is
//     deliberately no UUID-keyed abstraction mirroring CDNZone here.
//  2. Every ArvanCloud type carries an ArvanCloud prefix, even where a
//     same-named Parspack type already exists in this package. The field-level
//     shapes are not compatible (ArvanCloud DNS records, for one, have a
//     "cloud" flag and 13 record types where Parspack has a 5-value proxy enum
//     and 7 types), so reusing a name would either corrupt Parspack's meaning
//     or force a union type. The shared-port unification AGENTS.md 4.1 leaves
//     open is revisited once a third provider makes the real overlap visible —
//     not guessed at from two.

// ArvanCloudPlanLevel is a domain's CDN plan, sent and reported as an integer
// (the spec's Plan schema, 0..4). Subdomains require Growth or higher.
type ArvanCloudPlanLevel int

// The plan levels the CDN API accepts, confirmed against the Plan schema.
const (
	ArvanCloudPlanTraffic      ArvanCloudPlanLevel = 0
	ArvanCloudPlanBasic        ArvanCloudPlanLevel = 1
	ArvanCloudPlanGrowth       ArvanCloudPlanLevel = 2
	ArvanCloudPlanProfessional ArvanCloudPlanLevel = 3
	ArvanCloudPlanEnterprise   ArvanCloudPlanLevel = 4
)

// String renders the plan the way ArvanCloud's panel names it, so a tool
// result reads as "growth" rather than "2".
func (p ArvanCloudPlanLevel) String() string {
	switch p {
	case ArvanCloudPlanTraffic:
		return "traffic"
	case ArvanCloudPlanBasic:
		return "basic"
	case ArvanCloudPlanGrowth:
		return "growth"
	case ArvanCloudPlanProfessional:
		return "professional"
	case ArvanCloudPlanEnterprise:
		return "enterprise"
	default:
		return "unknown"
	}
}

// Valid reports whether p is one of the plan levels the CDN API accepts.
func (p ArvanCloudPlanLevel) Valid() bool {
	return p >= ArvanCloudPlanTraffic && p <= ArvanCloudPlanEnterprise
}

// The onboarding modes a domain can use, per the Domain schema's "type" enum.
// A full domain points its nameservers at ArvanCloud; a partial one is
// onboarded through a CNAME instead, which is how subdomains are served.
const (
	ArvanCloudDomainTypeFull    = "full"
	ArvanCloudDomainTypePartial = "partial"
)

// The lifecycle states a domain reports, per the Domain schema's "status"
// enum. Only "active" means traffic is actually being served.
const (
	ArvanCloudDomainStatusInitializing = "initializing"
	ArvanCloudDomainStatusPending      = "pending"
	ArvanCloudDomainStatusActive       = "active"
	ArvanCloudDomainStatusMoved        = "moved"
)

var (
	arvanCloudDomainTypes    = []string{ArvanCloudDomainTypeFull, ArvanCloudDomainTypePartial}
	arvanCloudDomainStatuses = []string{
		ArvanCloudDomainStatusInitializing,
		ArvanCloudDomainStatusPending,
		ArvanCloudDomainStatusActive,
		ArvanCloudDomainStatusMoved,
	}
)

// ValidArvanCloudDomainType reports whether s is one of the onboarding modes
// the CDN API accepts.
func ValidArvanCloudDomainType(s string) bool { return contains(arvanCloudDomainTypes, s) }

// ValidArvanCloudDomainStatus reports whether s is one of the lifecycle
// states the CDN API reports.
func ValidArvanCloudDomainStatus(s string) bool { return contains(arvanCloudDomainStatuses, s) }

// ArvanCloudDomain is a domain onboarded onto ArvanCloud's CDN — the
// equivalent of a CDN zone, and the resource every other ArvanCloud
// capability is scoped to.
//
// Name, not ID, is what addresses this domain's sub-resources: the API keys
// every /domains/{domain}/... path on the domain name. ID is the account-side
// UUID the provider reports, kept because a few endpoints (domain transfer,
// audit trails) refer to it, never because a caller needs it to reach a
// sub-resource.
type ArvanCloudDomain struct {
	// ID is the provider-side UUID of the domain record.
	ID string

	// Name is the domain name itself, e.g. "example.com". The spec still
	// reports a deprecated "domain" field alongside "name"; adapters read
	// "name" and fall back to "domain" only for older responses.
	Name string

	// PlanLevel is the CDN plan the domain is on.
	PlanLevel ArvanCloudPlanLevel

	// Type is the onboarding mode: "full" (NS-based) or "partial"
	// (CNAME-based).
	Type string

	// Status is the lifecycle state: "initializing", "pending", "active" or
	// "moved".
	Status string

	// NSKeys are the nameservers the domain's registrar must be pointed at
	// for a full-type domain. Always a pair when present.
	NSKeys []string

	// CurrentNS is what the registrar currently has configured, as far as
	// ArvanCloud can see it — the field a caller compares against NSKeys to
	// tell whether the domain still needs repointing.
	CurrentNS []string

	// CnameTarget is the record a partial-type domain must CNAME to.
	CnameTarget string

	// CustomCname is the caller-chosen CNAME record replacing the default
	// CnameTarget, empty when none is set.
	CustomCname string

	// DNSCloud reports whether ArvanCloud is serving the domain's DNS.
	DNSCloud bool

	// CreatedAt and UpdatedAt are provider-reported timestamps, kept as the
	// RFC 3339 strings the API returns rather than parsed times: they are
	// passed through to the caller, never computed with.
	CreatedAt string
	UpdatedAt string
}
