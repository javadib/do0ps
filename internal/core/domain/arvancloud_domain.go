package domain

// ArvanCloudDomainSpec is the input to onboard a new domain onto ArvanCloud's
// CDN (POST /domains/dns-service, the DomainStore request schema in
// docs/api-specs/arvancloud-cdn-4.0.yml, issue #62).
type ArvanCloudDomainSpec struct {
	// Name is the domain to onboard, e.g. "example.com". Required.
	Name string

	// DomainType selects the onboarding mode: ArvanCloudDomainTypeFull
	// (NS-based — the registrar points its nameservers at ArvanCloud) or
	// ArvanCloudDomainTypePartial (CNAME-based — only a subdomain is routed
	// through ArvanCloud, e.g. because the rest of the domain's DNS is hosted
	// elsewhere). Left empty, the CDN API applies its own documented default
	// ("full").
	DomainType string

	// PlanLevel is the CDN plan the domain is created on. Subdomains
	// (DomainType ArvanCloudDomainTypePartial) require ArvanCloudPlanGrowth
	// or higher.
	PlanLevel ArvanCloudPlanLevel

	// ImportDNSRecords, when true, has ArvanCloud automatically create A
	// records for the root (@) and www, and attempt to detect and add a
	// wildcard (*) record from DNS resolution. The CDN API defaults this to
	// true when the request omits the field entirely; this port always
	// receives an explicit value, because the "omitted means true" decision
	// is made once at the MCP tool boundary
	// (internal/adapters/mcp/arvancloud_domain_tools.go), not repeated here.
	ImportDNSRecords bool
}
