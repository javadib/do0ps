package domain

import "encoding/json"

// The types below model ArvanCloud's Load Balancing capability (issue #69):
// CDN edge-level traffic distribution across origin pools, confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Load Balancing" tag (the
// load-balancers.*, load-balancers.pools.* and
// load-balancers.pools.origins.* operationIds) and the
// LoadBalancer/LoadBalancerPool/LoadBalancerOrigin/LoadBalancerRegion
// schemas. A load balancer is a 3-level resource hierarchy: a
// ArvanCloudLoadBalancer holds ArvanCloudLoadBalancerPool entries, each of
// which holds ArvanCloudLoadBalancerOrigin entries the edge actually proxies
// traffic to.
//
// Naming collision warning, the same shape as domain/cdn_loadbalance.go's own
// warning: this is ArvanCloud's CDN edge-level load balancer, completely
// unrelated to any future ArvanCloud IaaS/cloud-server load balancer and to
// domain.LoadBalancer elsewhere in this package, which models Parspack's
// cloud-server/VM-network-level load balancer (see
// ports.ParspackProvider.CreateLoadBalancer's doc comment, ports.go line
// ~182). Every type here is prefixed ArvanCloudLoadBalancer (or
// ArvanCloud<something>, for the narrower enums) to keep the two apart —
// nothing here is unified with domain.LoadBalancer, and nothing should be.

// ArvanCloudLoadBalancerMethod is a load balancer's traffic-distribution
// strategy across its pools (LoadBalancer.method / LoadBalancerStore.method).
type ArvanCloudLoadBalancerMethod string

const (
	// ArvanCloudLoadBalancerMethodFailover sends all traffic to the
	// highest-priority healthy pool, falling over to the next one only when
	// it becomes unhealthy.
	ArvanCloudLoadBalancerMethodFailover ArvanCloudLoadBalancerMethod = "failover"
	// ArvanCloudLoadBalancerMethodClusterRR round-robins traffic across
	// pools, switching pools once per LoadBalancer.TimeSlice.
	ArvanCloudLoadBalancerMethodClusterRR ArvanCloudLoadBalancerMethod = "cluster_rr"
	// ArvanCloudLoadBalancerMethodClusterCHash distributes traffic across
	// pools by consistent hashing.
	ArvanCloudLoadBalancerMethodClusterCHash ArvanCloudLoadBalancerMethod = "cluster_chash"
)

var arvanCloudLoadBalancerMethods = []string{
	string(ArvanCloudLoadBalancerMethodFailover),
	string(ArvanCloudLoadBalancerMethodClusterRR),
	string(ArvanCloudLoadBalancerMethodClusterCHash),
}

// ValidArvanCloudLoadBalancerMethod reports whether s is one of
// LoadBalancer.method's three values.
func ValidArvanCloudLoadBalancerMethod(s string) bool {
	return contains(arvanCloudLoadBalancerMethods, s)
}

// ArvanCloudPoolMethod is a pool's own traffic-distribution strategy across
// its origins (LoadBalancerPool.method).
//
// Confirmed against the spec: a narrower enum than
// ArvanCloudLoadBalancerMethod above — exactly two values, "cluster_rr" and
// "cluster_chash", with NO "failover" — the same asymmetry issue #69 flags
// explicitly rather than assuming it is a spec typo. A load balancer picks
// which pool gets traffic (possibly by failing over between pools); a pool
// itself only ever spreads traffic across its own origins by round-robin or
// consistent hashing, never by failover.
type ArvanCloudPoolMethod string

const (
	// ArvanCloudPoolMethodClusterRR round-robins traffic across the pool's
	// origins.
	ArvanCloudPoolMethodClusterRR ArvanCloudPoolMethod = "cluster_rr"
	// ArvanCloudPoolMethodClusterCHash distributes traffic across the pool's
	// origins by consistent hashing.
	ArvanCloudPoolMethodClusterCHash ArvanCloudPoolMethod = "cluster_chash"
)

var arvanCloudPoolMethods = []string{
	string(ArvanCloudPoolMethodClusterRR),
	string(ArvanCloudPoolMethodClusterCHash),
}

// ValidArvanCloudPoolMethod reports whether s is one of
// LoadBalancerPool.method's two values.
func ValidArvanCloudPoolMethod(s string) bool { return contains(arvanCloudPoolMethods, s) }

// ArvanCloudLBToggle is the plain "on"/"off" string enum shared by
// LoadBalancerPool.keepalive, LoadBalancerPool.next_upstream_tcp and their
// LoadBalancerSetting equivalents — the same value set and meaning in both
// places, so this one type covers all four fields rather than four separate
// ones.
type ArvanCloudLBToggle string

const (
	ArvanCloudLBOn  ArvanCloudLBToggle = "on"
	ArvanCloudLBOff ArvanCloudLBToggle = "off"
)

var arvanCloudLBToggles = []string{string(ArvanCloudLBOn), string(ArvanCloudLBOff)}

// ValidArvanCloudLBToggle reports whether s is "on" or "off".
func ValidArvanCloudLBToggle(s string) bool { return contains(arvanCloudLBToggles, s) }

// ArvanCloudOriginProtocol is the protocol a load balancer's origin (or the
// domain's LB settings default) is proxied over (LoadBalancerOrigin.protocol
// / LoadBalancerSetting.protocol).
type ArvanCloudOriginProtocol string

const (
	// ArvanCloudOriginProtocolAuto matches the protocol of the incoming
	// request. The spec's own default when the field is omitted.
	ArvanCloudOriginProtocolAuto  ArvanCloudOriginProtocol = "auto"
	ArvanCloudOriginProtocolHTTP  ArvanCloudOriginProtocol = "http"
	ArvanCloudOriginProtocolHTTPS ArvanCloudOriginProtocol = "https"
)

var arvanCloudOriginProtocols = []string{
	string(ArvanCloudOriginProtocolAuto),
	string(ArvanCloudOriginProtocolHTTP),
	string(ArvanCloudOriginProtocolHTTPS),
}

// ValidArvanCloudOriginProtocol reports whether s is one of
// LoadBalancerOrigin.protocol's three values.
func ValidArvanCloudOriginProtocol(s string) bool { return contains(arvanCloudOriginProtocols, s) }

// ArvanCloudOriginHealthCheckStatus is an origin's live health-check status
// as reported by ArvanCloud's own health-check subsystem
// (LoadBalancerOrigin.health_check_status). Read-only: never sent on
// create/update, only ever reported back.
type ArvanCloudOriginHealthCheckStatus string

const (
	ArvanCloudOriginHealthCheckOff       ArvanCloudOriginHealthCheckStatus = "off"
	ArvanCloudOriginHealthCheckHealthy   ArvanCloudOriginHealthCheckStatus = "healthy"
	ArvanCloudOriginHealthCheckUnhealthy ArvanCloudOriginHealthCheckStatus = "unhealthy"
	ArvanCloudOriginHealthCheckUnstable  ArvanCloudOriginHealthCheckStatus = "unstable"
	ArvanCloudOriginHealthCheckNoData    ArvanCloudOriginHealthCheckStatus = "no-data"
)

// ArvanCloudLoadBalancerRegion is a region a load balancer's pools can be
// scoped to (the LoadBalancerRegion schema): GetArvanCloudLBRegions returns
// the account-independent, global list; GetArvanCloudDomainLBRegions returns
// the per-domain equivalent — the spec does not document the two as always
// identical, so both are exposed rather than assumed interchangeable (issue
// #69).
type ArvanCloudLoadBalancerRegion struct {
	// ID is the provider-assigned UUID.
	ID string
	// Region is the short region code, e.g. "LAH", used elsewhere (e.g.
	// ArvanCloudLoadBalancerPool.Regions) to reference this region.
	Region string
	// Name is the region's human-readable name. Read-only.
	Name string
}

// ArvanCloudLoadBalancerOrigin is one backend an ArvanCloudLoadBalancerPool
// proxies traffic to (/domains/{domain}/load-balancers/{id}/pools/{id}/origins[/{id}],
// the LoadBalancerOrigin/LoadBalancerOriginStore schemas).
type ArvanCloudLoadBalancerOrigin struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string
	// Name is a caller-supplied label for the origin.
	Name string
	// HealthCheckStatus is read-only: the origin's live health as reported by
	// ArvanCloud's own health-check subsystem. Never sent on create/update.
	HealthCheckStatus ArvanCloudOriginHealthCheckStatus
	// Status is the admin enable/disable switch: whether this origin
	// currently receives traffic, independent of HealthCheckStatus.
	Status bool
	// Address is the origin's IP or hostname.
	Address string
	// Port is the TCP port to connect to on Address, e.g. 443. Spec range:
	// 1-65535.
	Port int
	// Weight is this origin's relative share of traffic within the pool,
	// e.g. 100. Spec range: 1-1000; higher gets proportionally more traffic.
	Weight int
	// Protocol is the protocol used to reach this origin. Must be one of
	// ValidArvanCloudOriginProtocol's values; defaults to "auto" when left
	// unset.
	Protocol ArvanCloudOriginProtocol
	// HostHeader overrides the Host header sent to this origin. Empty leaves
	// it unset, letting the provider apply its own default.
	HostHeader string
	// CreatedAt/UpdatedAt are provider-reported timestamps. Read-only.
	CreatedAt string
	UpdatedAt string
}

// ArvanCloudLoadBalancerPool is a set of ArvanCloudLoadBalancerOrigin
// entries an ArvanCloudLoadBalancer distributes traffic to
// (/domains/{domain}/load-balancers/{id}/pools[/{id}], the
// LoadBalancerPool/LoadBalancerPoolStoreWithOrigin/
// LoadBalancerPoolStoreWithoutOrigin schemas).
type ArvanCloudLoadBalancerPool struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string
	// Name is a caller-supplied label for the pool.
	Name string
	// Description is a caller-supplied note about the pool.
	Description string
	// Status is the admin enable/disable switch for the whole pool.
	Status bool
	// Priority orders this pool relative to its siblings; zero means the
	// default pool (the spec's own description). Callers reordering pools
	// relative to each other use ReprioritizeArvanCloudLBPool instead of
	// setting Priority directly.
	Priority int
	// Method is this pool's own traffic-distribution strategy across its
	// origins. Must be one of ValidArvanCloudPoolMethod's values — narrower
	// than ArvanCloudLoadBalancer.Method's enum, see
	// ArvanCloudPoolMethod's doc comment for why.
	Method ArvanCloudPoolMethod
	// Keepalive turns on upstream keepalive connections to this pool's
	// origins. Must be one of ValidArvanCloudLBToggle's values; defaults to
	// "off" when left unset.
	Keepalive ArvanCloudLBToggle
	// NextUpstreamTCP turns on retrying another origin in this pool when the
	// first one fails at the TCP level. Must be one of
	// ValidArvanCloudLBToggle's values; defaults to "off" when left unset.
	NextUpstreamTCP ArvanCloudLBToggle
	// NextUpstreamTCPCodes is which upstream HTTP status codes, per request
	// method (keys: "head"/"get"/"post"/"put"/"delete"/"options"/"patch"),
	// trigger falling over to the next origin when NextUpstreamTCP is "on".
	// Spec values are drawn from [500, 502, 503, 504, 403, 404, 429].
	NextUpstreamTCPCodes map[string][]int
	// Regions restricts this pool to specific ArvanCloudLoadBalancerRegion
	// codes (e.g. "LAH"), an empty slice meaning every region. Confirmed
	// asymmetry against the spec: the request shape
	// (LoadBalancerPoolStoreWithoutOrigin.regions) is a plain array of
	// 3-letter region-code strings, while the response shape
	// (LoadBalancerPool.regions) is an array of full LoadBalancerRegion
	// objects — this type always holds the codes, matching the request
	// shape; a caller that needs a region's full name looks it up via
	// ListArvanCloudLBRegions/ListArvanCloudDomainLBRegions.
	Regions []string
	// Origins is this pool's origin set. Read-only on every method except
	// CreateArvanCloudLBPool and UpdateArvanCloudLBPoolWithOrigins (PUT),
	// which are the only two operations that ever write to it — see this
	// port's own doc comment on the PUT-vs-PATCH distinction
	// (ports.ArvanCloudProvider.UpdateArvanCloudLBPoolWithOrigins /
	// UpdateArvanCloudLBPoolSettings).
	Origins []ArvanCloudLoadBalancerOrigin
	// MonitoringStatus is read-only: the pool's aggregate health-check
	// status ("off"/"no-data"/"healthy"/"unhealthy"), reported by
	// ArvanCloud's health-check subsystem.
	MonitoringStatus string
	// HealthCheck is a read-only, opaque reference to the pool's configured
	// health check — the HealthCheck resource itself is issue #70/AC10's
	// scope, not this one's. Kept here only so a caller inspecting a pool
	// can see that a health check exists, decoded as raw JSON rather than a
	// typed structure until #70 implements it.
	HealthCheck json.RawMessage
	// CreatedAt/UpdatedAt are provider-reported timestamps. Read-only.
	CreatedAt string
	UpdatedAt string
}

// ArvanCloudLoadBalancer is CDN edge-level traffic distribution across a set
// of ArvanCloudLoadBalancerPool entries
// (/domains/{domain}/load-balancers[/{id}], the LoadBalancer/LoadBalancerStore
// schemas). See this file's package comment for the naming-collision warning
// against domain.LoadBalancer.
type ArvanCloudLoadBalancer struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string
	// Name is a caller-supplied label, restricted to letters, digits and
	// hyphens by the spec's own pattern, e.g. "lb1".
	Name string
	// Description is a caller-supplied note about the load balancer.
	Description string
	// Status is the admin enable/disable switch for the whole load balancer.
	Status bool
	// Method is how traffic is distributed across Pools. Must be one of
	// ValidArvanCloudLoadBalancerMethod's values.
	Method ArvanCloudLoadBalancerMethod
	// TimeSlice is a human-friendly duration string (e.g. "30s") for how
	// long a pool is uninterruptedly selected under
	// ArvanCloudLoadBalancerMethodClusterRR, before switching to the next
	// one. Meaningful only for that method. Defaults to "0s" when left
	// unset.
	TimeSlice string
	// Pools is this load balancer's pool set. Read-only: pools are managed
	// through the dedicated pool methods
	// (ListArvanCloudLBPools/CreateArvanCloudLBPool/...), never written
	// through this field.
	Pools []ArvanCloudLoadBalancerPool
	// CreatedAt/UpdatedAt are provider-reported timestamps. Read-only.
	CreatedAt string
	UpdatedAt string
}

// ArvanCloudLoadBalancerSettings is a domain's global load-balancing defaults
// (/domains/{domain}/load-balancers/settings, the LoadBalancerSetting
// schema), applied to every load balancer on the domain unless a pool
// overrides a field itself.
type ArvanCloudLoadBalancerSettings struct {
	// Method is the domain-wide default pool traffic-distribution strategy.
	// Must be one of ValidArvanCloudPoolMethod's values — the same narrower
	// enum as ArvanCloudLoadBalancerPool.Method, confirmed against the spec's
	// LoadBalancerSetting.method (no "failover" here either).
	Method ArvanCloudPoolMethod
	// NextUpstreamTCP is the domain-wide default for
	// ArvanCloudLoadBalancerPool.NextUpstreamTCP. Must be one of
	// ValidArvanCloudLBToggle's values; defaults to "off" when left unset.
	NextUpstreamTCP ArvanCloudLBToggle
	// NextUpstreamTCPCodes is the domain-wide default for
	// ArvanCloudLoadBalancerPool.NextUpstreamTCPCodes.
	NextUpstreamTCPCodes map[string][]int
	// Protocol is the domain-wide default protocol used to reach an origin
	// when the origin itself does not override it. Must be one of
	// ValidArvanCloudOriginProtocol's values.
	Protocol ArvanCloudOriginProtocol
	// GRPCStatus turns on gRPC proxying for the domain; requires upstream
	// services to actually support gRPC (the spec's own description).
	// Disabled/unset falls back to standard HTTP proxying.
	GRPCStatus bool
	// Keepalive is the domain-wide default for
	// ArvanCloudLoadBalancerPool.Keepalive. Must be one of
	// ValidArvanCloudLBToggle's values; defaults to "off" when left unset.
	Keepalive ArvanCloudLBToggle
	// MaxFails is how many consecutive failures mark an origin down before
	// it is skipped (spec range: 0-10000). The spec's own description reads
	// zero as "disable the failing strategy" rather than "unset" — but that
	// is indistinguishable from Go's own int zero value, the same limitation
	// domain.ArvanCloudRateLimitRule.Burst documents — so this adapter
	// treats zero as "leave it unset, let the provider apply its own
	// default" rather than sending an explicit 0, matching Burst's own
	// choice.
	MaxFails int
	// FailTimeout is a human-friendly duration string (e.g. "45s") for how
	// long an origin marked down by MaxFails stays excluded before being
	// retried. Defaults to "10s" when left unset.
	FailTimeout string
}
