package domain

// The types below model the CDN-edge load-balance pool feature of Parspack's
// CDN API (issue #24), confirmed against docs/api-specs/parspack-cdn.openapi.yaml's
// "Load Balance" tag (lines ~5027-6677): a CDNLoadBalance is a pool of
// backend CDNLoadBalanceServer entries, scoped to a CDN zone (zone_uuid),
// that DNS records can point at via DNSRecord.LoadBalanceID.
//
// This is a completely different resource from the cloud-server/VM-network
// level LoadBalancer type elsewhere in this package (issue #12): that one
// lives on the Abrha-based cloud-server API surface (.../cserver) and
// balances traffic across VMs; this one lives on the CDN surface
// (.../cdnapi) and balances traffic the CDN edge forwards to origin servers.
// Every type and function here is prefixed CDN to keep the two apart — see
// ports.ParspackProvider's LoadBalancer doc comment for the same warning
// from the other side.

// CDNLoadBalance is a load-balance pool configured inside a CDN zone.
type CDNLoadBalance struct {
	ID                      string
	Name                    string
	Enabled                 bool
	RetryCount              int
	EnableCookiePersist     bool
	ServerFailCountToBeDown int
	Method                  string // "round_robin" or "c-hash"
	DownServersRecoveryTime int    // seconds
	CookiePersistExpireTime int    // seconds
	Servers                 []CDNLoadBalanceServer
}

// CDNLoadBalanceServer is one backend server inside a CDNLoadBalance pool.
//
// The CDN API's create/get/update/delete endpoints for this resource are all
// zone-scoped only (.../zones/{zone_uuid}/load-balance-server); confirmed
// against the spec, none of their request or response bodies carry a
// load-balance foreign key — the only place a load balance's ID appears in
// this part of the API is as a required query parameter filtering
// ListCDNLoadBalanceServers. A server only becomes visible as part of a
// specific pool through that pool's own "servers" list
// (CDNLoadBalance.Servers, returned by ListCDNLoadBalances/GetCDNLoadBalance).
type CDNLoadBalanceServer struct {
	ID           string
	Name         string
	IP           string
	HTTPPort     int
	HTTPSPort    int
	Weight       int
	RecoveryTime int    // seconds
	Group        string // "primary" or "backup"
	Active       bool
}

// cdnLoadBalanceMethods and cdnLoadBalanceServerGroups are the enums
// confirmed against the load-balance and load-balance-server request bodies.
var (
	cdnLoadBalanceMethods      = []string{"round_robin", "c-hash"}
	cdnLoadBalanceServerGroups = []string{"primary", "backup"}
)

// ValidCDNLoadBalanceMethod reports whether s is one of the balancing
// methods the CDN API's load-balance endpoints accept.
func ValidCDNLoadBalanceMethod(s string) bool { return contains(cdnLoadBalanceMethods, s) }

// ValidCDNLoadBalanceServerGroup reports whether s is one of the server
// groups the CDN API's load-balance-server endpoints accept.
func ValidCDNLoadBalanceServerGroup(s string) bool { return contains(cdnLoadBalanceServerGroups, s) }
