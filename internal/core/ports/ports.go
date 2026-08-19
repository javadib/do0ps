// Package ports declares the interfaces the application core depends on.
//
// Every interface here is owned by core and implemented by an adapter under
// internal/adapters. Core never imports an adapter package; the composition
// root in cmd/server wires concrete implementations into the use cases.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Task is a unit of fast provider work executed on a worker goroutine.
type Task func(ctx context.Context) (json.RawMessage, error)

// JobHandler executes a persisted long-running job. Handlers are implemented
// by use cases in internal/core/app and registered with the queue adapter at
// wiring time, keyed by domain.JobType.
type JobHandler func(ctx context.Context, job *domain.Job) (json.RawMessage, error)

// Queue schedules provider work. Implemented by internal/adapters/queue with
// Go channels and a bounded worker pool.
type Queue interface {
	// Dispatch runs a fast operation on a worker and blocks until it returns.
	// The caller's context bounds the wait, so a saturated pool surfaces as a
	// deadline error rather than an indefinite hang.
	Dispatch(ctx context.Context, task Task) (json.RawMessage, error)

	// Submit schedules an already-persisted job for background execution and
	// returns as soon as it is accepted. Used for long operations, where the
	// MCP caller polls progress through an operation ID instead of waiting.
	Submit(ctx context.Context, job *domain.Job) error
}

// JobRepository persists job state. Implemented by internal/adapters/sqlite.
type JobRepository interface {
	// Create persists a new job. The caller assigns Job.ID beforehand (see
	// ports.IDGenerator); Create returns an error if that ID is already in use.
	Create(ctx context.Context, job *domain.Job) error

	// Get returns the job with the given ID, or domain.ErrNotFound if none
	// exists.
	Get(ctx context.Context, id string) (*domain.Job, error)

	// Update overwrites a persisted job's mutable fields (status, attempts,
	// next_retry_at, result, error, updated_at) by ID. Returns domain.ErrNotFound
	// if the job was never created.
	Update(ctx context.Context, job *domain.Job) error

	// ListUnfinished returns every job still in pending or running state. Used
	// once at startup for recovery; running jobs must be reconciled against the
	// provider before they are retried (see AGENTS.md 4.4).
	ListUnfinished(ctx context.Context) ([]*domain.Job, error)

	// ListDue returns at most limit pending jobs whose next_retry_at has passed.
	ListDue(ctx context.Context, now time.Time, limit int) ([]*domain.Job, error)
}

// ParspackProvider is the port for the Parspack API, listing exactly the
// operations core needs. Each provider gets its own dedicated port for now —
// a shared provider interface is deliberately deferred until two or three
// providers make the real overlap visible (see AGENTS.md 4.1).
//
// Credentials are passed per call: they belong to the chatbot session, never
// to this server. Create-style methods are not expected to be idempotent on
// their own — callers that must not duplicate a resource on retry (recovery
// after a crash, see AGENTS.md 4.4) are expected to check first, e.g. via
// FindServerByName, rather than relying on the provider to deduplicate.
type ParspackProvider interface {
	// VM lifecycle. CreateServer is a long operation (AGENTS.md 4.3); the
	// others are fast.
	CreateServer(ctx context.Context, creds domain.ProviderCredentials, spec domain.ServerSpec) (*domain.Server, error)
	GetServer(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.Server, error)
	ListServers(ctx context.Context, creds domain.ProviderCredentials) ([]domain.Server, error)

	// DeleteServer removes a server by provider ID. Deleting an ID that no
	// longer exists returns domain.ErrNotFound, so callers can treat delete as
	// idempotent by tolerating that specific error.
	DeleteServer(ctx context.Context, creds domain.ProviderCredentials, id string) error

	// FindServerByName supports crash recovery: before retrying a create-style
	// job found in running state, core asks whether the resource already exists.
	// Returns domain.ErrNotFound when it does not.
	FindServerByName(ctx context.Context, creds domain.ProviderCredentials, name string) (*domain.Server, error)

	// SSH key management. All fast operations. Keys are referenced by
	// ServerSpec.SSHKeys at server-creation time.
	CreateSSHKey(ctx context.Context, creds domain.ProviderCredentials, key domain.SSHKey) (*domain.SSHKey, error)
	ListSSHKeys(ctx context.Context, creds domain.ProviderCredentials) ([]domain.SSHKey, error)
	// DeleteSSHKey removes a key by provider ID. As with DeleteServer, an
	// already-absent ID reports domain.ErrNotFound rather than succeeding
	// silently, so callers decide for themselves whether that counts as done.
	DeleteSSHKey(ctx context.Context, creds domain.ProviderCredentials, id string) error

	// Reserved IP management. All fast operations. A Reserved IP exists
	// independently of any server (the two-resource split of
	// terraform-provider-abrha's "reserved_ip" and "reserved_ip_assignment"):
	// AssignIPToServer/UnassignIP only attach or detach it, they never create
	// or destroy the address itself.
	ReserveIP(ctx context.Context, creds domain.ProviderCredentials, region string) (*domain.ReservedIP, error)
	// ReleaseIP removes a reserved IP. As with DeleteServer, an already-absent
	// address reports domain.ErrNotFound so callers decide whether to treat a
	// repeat release as already done.
	ReleaseIP(ctx context.Context, creds domain.ProviderCredentials, ip string) error
	AssignIPToServer(ctx context.Context, creds domain.ProviderCredentials, ip, serverID string) (*domain.ReservedIP, error)
	UnassignIP(ctx context.Context, creds domain.ProviderCredentials, ip string) (*domain.ReservedIP, error)

	// Firewall management. All fast operations (AGENTS.md 4.3). These are the
	// cloud-server/VM-network-level firewalls of the Abrha-based cloud-server
	// API (base URL .../cserver, path api/public/v1/firewalls), NOT the CDN
	// API's edge-level firewall concept (tracked separately in issue #24) —
	// the two live on different API surfaces and must not be conflated.
	//
	// Create-style methods are not expected to be idempotent on their own;
	// callers that must not duplicate a firewall on retry are expected to
	// check first via ListFirewalls (AGENTS.md 4.4).
	CreateFirewall(ctx context.Context, creds domain.ProviderCredentials, fw domain.Firewall) (*domain.Firewall, error)
	GetFirewall(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.Firewall, error)
	ListFirewalls(ctx context.Context, creds domain.ProviderCredentials) ([]domain.Firewall, error)
	UpdateFirewall(ctx context.Context, creds domain.ProviderCredentials, id string, fw domain.Firewall) (*domain.Firewall, error)

	// DeleteFirewall removes a firewall by provider ID. As with DeleteServer,
	// an already-absent ID reports domain.ErrNotFound rather than succeeding
	// silently, so callers decide for themselves whether that counts as done.
	DeleteFirewall(ctx context.Context, creds domain.ProviderCredentials, id string) error

	// SSL certificate ordering workflow (AGENTS.md 4.5's SSL surface). All
	// fast operations: each is a single HTTP round trip, even though driving
	// a certificate to issuance takes a caller several separate calls
	// (create order, process, verify challenge, poll certificate).
	ListSSLProducts(ctx context.Context, creds domain.ProviderCredentials) ([]domain.SSLProduct, error)
	CreateSSLOrder(ctx context.Context, creds domain.ProviderCredentials, spec domain.SSLOrderSpec) (*domain.SSLOrder, error)
	// ProcessSSLOrder submits the CSR and contact details for a paid order
	// and returns the domain-ownership challenges to complete next.
	ProcessSSLOrder(ctx context.Context, creds domain.ProviderCredentials, orderID, csr string, contact domain.SSLContact) (*domain.SSLChallengeSet, error)
	// GetSSLChallenge re-shows the challenges of an already-processed order.
	GetSSLChallenge(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.SSLChallengeSet, error)
	// ReloadSSLChallenge switches the verification method, invalidating any
	// previously shown challenge tokens. emailPrefix is only meaningful when
	// method is "ADMIN".
	ReloadSSLChallenge(ctx context.Context, creds domain.ProviderCredentials, orderID, method, emailPrefix string) (*domain.SSLChallengeSet, error)
	// VerifySSLChallenge checks the completed challenge for method and, on
	// success, returns the certificate if it is ready immediately.
	VerifySSLChallenge(ctx context.Context, creds domain.ProviderCredentials, orderID, method string) (*domain.SSLVerifyResult, error)
	GetSSLCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.SSLCertificate, error)
	ReissueSSLCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID, csr string) (*domain.SSLCertificate, error)

	// CDN zone management (issue #19). CreateCDNZone is a fast operation: the
	// provider's order endpoint returns a final zone_id and status
	// synchronously — there is no further "provisioning" state to poll, unlike
	// CreateServer (AGENTS.md 4.3).
	CreateCDNZone(ctx context.Context, creds domain.ProviderCredentials, spec domain.CDNZoneSpec) (*domain.CDNZone, error)
	ListCDNZones(ctx context.Context, creds domain.ProviderCredentials) ([]domain.CDNZone, error)
	GetCDNZone(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNZone, error)
	// DeleteCDNZone removes a zone by UUID. As with DeleteServer, an
	// already-absent zone reports domain.ErrNotFound rather than succeeding
	// silently, so callers decide for themselves whether that counts as done.
	DeleteCDNZone(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) error
	ListCDNZonePlans(ctx context.Context, creds domain.ProviderCredentials) ([]domain.CDNZonePlanPricing, error)
	GetNameserverRecords(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.NameserverRecords, error)

	// DNS records, scoped to a CDN zone — Parspack has no standalone DNS
	// product (AGENTS.md 4.1). All fast operations.
	ListDNSRecords(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.DNSRecord, error)
	CreateDNSRecord(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rec domain.DNSRecord) (*domain.DNSRecord, error)
	UpdateDNSRecord(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rec domain.DNSRecord) (*domain.DNSRecord, error)
	// DeleteDNSRecord removes a record from a zone by host+type. When content
	// is empty every value under that host+type is deleted; otherwise only the
	// value matching content is removed (a host+type can hold more than one
	// value, e.g. multiple NS records).
	DeleteDNSRecord(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, host string, recordType domain.DNSRecordType, content string) error

	// LoadBalancer management (cloud-server/VM-network level, NOT the CDN
	// API's separate edge-level Load Balance concept — issue #24). Create is a
	// long operation: the load balancer starts in "new" status and reaches
	// "active" after provisioning; the others are fast.
	CreateLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, lb domain.LoadBalancer) (*domain.LoadBalancer, error)
	GetLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.LoadBalancer, error)
	ListLoadBalancers(ctx context.Context, creds domain.ProviderCredentials) ([]domain.LoadBalancer, error)

	// UpdateLoadBalancer replaces a load balancer's configuration by provider
	// ID.
	UpdateLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, id string, lb domain.LoadBalancer) (*domain.LoadBalancer, error)

	// DeleteLoadBalancer removes a load balancer by provider ID. As with
	// DeleteServer, an already-absent ID reports domain.ErrNotFound.
	DeleteLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, id string) error

	// FindLoadBalancerByName supports crash recovery for provisioning jobs,
	// mirroring FindServerByName.
	FindLoadBalancerByName(ctx context.Context, creds domain.ProviderCredentials, name string) (*domain.LoadBalancer, error)

	// VM snapshot management. CreateVMSnapshot and RestoreVM are long
	// operations: they start an async VM action and return it in
	// "in-progress" state; callers poll GetVMAction until it reaches a
	// terminal state (see AGENTS.md 4.3). ListVMSnapshots and
	// DeleteVMSnapshot are fast.
	CreateVMSnapshot(ctx context.Context, creds domain.ProviderCredentials, serverID, name string) (*domain.VMAction, error)
	GetVMAction(ctx context.Context, creds domain.ProviderCredentials, serverID, actionID string) (*domain.VMAction, error)
	ListVMSnapshots(ctx context.Context, creds domain.ProviderCredentials) ([]domain.VMSnapshot, error)
	// DeleteVMSnapshot removes a snapshot by provider ID. As with
	// DeleteServer, an already-absent ID reports domain.ErrNotFound rather
	// than succeeding silently, so callers decide for themselves whether that
	// counts as done.
	DeleteVMSnapshot(ctx context.Context, creds domain.ProviderCredentials, id string) error
	// RestoreVM wipes the disk of the given server and replaces it with the
	// given snapshot's contents.
	RestoreVM(ctx context.Context, creds domain.ProviderCredentials, serverID, snapshotID string) (*domain.VMAction, error)
	// CreateVPC provisions an isolated private network. The input VPC carries
	// Name, Region, Description and IPRange; the returned copy carries the
	// provider-assigned ID and default flag.
	CreateVPC(ctx context.Context, creds domain.ProviderCredentials, vpc domain.VPC) (*domain.VPC, error)
	GetVPC(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.VPC, error)
	ListVPCs(ctx context.Context, creds domain.ProviderCredentials) ([]domain.VPC, error)

	// DeleteVPC removes a VPC by provider ID. Deleting an ID the provider no
	// longer has must be tolerated by callers: it returns domain.ErrNotFound.
	DeleteVPC(ctx context.Context, creds domain.ProviderCredentials, id string) error

	// CDN zone settings/toggles (issue #24): antivirus, DNSSEC, asset
	// optimization, developer mode, maintenance mode, query-string caching
	// behavior, and origin-offline handling. All fast operations. Developer
	// mode, maintenance mode, query string and origin offline have no
	// dedicated single-setting GET documented by the CDN API spec — their
	// current value is available via GetCDNCacheSettings instead, so only
	// their Update methods exist here.
	GetCDNAntivirusStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (bool, error)
	UpdateCDNAntivirusStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error)
	GetCDNDNSSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNDNSSecStatus, error)
	UpdateCDNDNSSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (*domain.CDNDNSSecStatus, error)
	GetCDNOptimizationStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNOptimizationStatus, error)
	UpdateCDNOptimization(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, status domain.CDNOptimizationStatus) (*domain.CDNOptimizationStatus, error)
	UpdateCDNDeveloperMode(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error)
	UpdateCDNMaintenanceMode(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error)
	UpdateCDNQueryStringSetting(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error)
	UpdateCDNOriginOffline(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error)

	// Bulklist management (issue #24). Bulklist is a user-level resource (no
	// zone_uuid), unlike CDN zones and DNS records — a reusable IP/country
	// list other CDN features (e.g. firewall rules) reference by ID. All
	// fast operations.
	ListCDNBulklists(ctx context.Context, creds domain.ProviderCredentials) ([]domain.CDNBulklist, error)
	CreateCDNBulklist(ctx context.Context, creds domain.ProviderCredentials, spec domain.CDNBulklistSpec) (*domain.CDNBulklist, error)
	GetCDNBulklist(ctx context.Context, creds domain.ProviderCredentials, bulklistID string) (*domain.CDNBulklist, error)
	UpdateCDNBulklist(ctx context.Context, creds domain.ProviderCredentials, bulklistID string, spec domain.CDNBulklistSpec) (*domain.CDNBulklist, error)
	// DeleteCDNBulklist removes a bulklist by ID. As with DeleteServer, an
	// already-absent ID reports domain.ErrNotFound rather than succeeding
	// silently, so callers decide for themselves whether that counts as done.
	DeleteCDNBulklist(ctx context.Context, creds domain.ProviderCredentials, bulklistID string) error
	// ListCDNFirewallCountries returns the read-only country reference list
	// used to populate country-based firewall rules for a zone.
	ListCDNFirewallCountries(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNCountry, error)

	// Cache Management (issue #24), scoped to a CDN zone. All fast
	// operations — none of the documented 200 responses carries a
	// job/status/pending field to poll. The spec exposes PUT only for
	// cache/ttl, cache/rule and cache/user-agent (no matching GET per
	// setting) and GET only for cache/settings (an aggregate view that also
	// reports developer_mode/maintenance_mode/ignore_query_string/
	// origin_offline — see the zone-settings group above).
	UpdateCDNCacheTTL(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, ttlSeconds int) (*domain.CDNCacheTTLSetting, error)
	UpdateCDNCacheRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, cacheRule string) (*domain.CDNCacheRuleSetting, error)
	UpdateCDNCacheUserAgentSetting(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (*domain.CDNCacheUserAgentSetting, error)
	GetCDNCacheSettings(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNCacheSettings, error)
	ListCDNCacheEntries(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNCacheEntry, error)
	// PurgeCDNCache ("Destroy Cache" in the spec) returns no id to poll —
	// callers that want progress use ListCDNCacheEntries/GetCDNCacheEntry
	// afterward.
	PurgeCDNCache(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) error
	// GetCDNCacheEntry ("Show Cache Management" in the spec). Note: the spec
	// has no DELETE .../cache/{id}, so there is no DeleteCDNCacheEntry.
	GetCDNCacheEntry(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNCacheEntry, error)

	// CDN-edge firewall (issue #24): access-management rules, IP-reputation
	// blocking and DDoS mitigation actions, scoped to a CDN zone_uuid. These
	// are entirely distinct from CreateFirewall/GetFirewall/ListFirewalls/
	// UpdateFirewall/DeleteFirewall above, which are the cloud-server/
	// VM-network-level firewall (issue #11) — unrelated to the CDN edge. All
	// fast operations.
	ListCDNAccessRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNAccessRule, error)
	CreateCDNAccessRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNAccessRule) (*domain.CDNAccessRule, error)
	GetCDNAccessRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNAccessRule, error)
	UpdateCDNAccessRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNAccessRule) (*domain.CDNAccessRule, error)
	// DeleteCDNAccessRule removes a rule by ID. As with DeleteServer, an
	// already-absent rule reports domain.ErrNotFound rather than succeeding
	// silently.
	DeleteCDNAccessRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error
	GetCDNIPReputation(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNIPReputationSettings, error)
	UpdateCDNIPReputation(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, settings domain.CDNIPReputationSettings) (*domain.CDNIPReputationSettings, error)
	GetCDNDDoSActions(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNDDoSActionSettings, error)
	UpdateCDNDDoSActions(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, settings domain.CDNDDoSActionSettings) (*domain.CDNDDoSActionSettings, error)

	// CDN-edge load-balance pools and their backend servers (issue #24). All
	// fast operations: none of the create/update/delete responses carry a
	// status field to poll — unlike the cloud-server LoadBalancer above,
	// which is a completely different resource; every method here is
	// prefixed CDN to keep the two apart.
	//
	// CreateCDNLoadBalance/CreateCDNLoadBalanceServer's store endpoints
	// return an empty data array with no generated ID, so the returned
	// resource echoes what was sent with no ID populated — callers must
	// list/get afterward to learn it.
	//
	// ListCDNLoadBalanceServers requires loadBalanceID as a query-parameter
	// filter (the provider's only supported way to scope servers to a
	// pool): none of this resource's own request/response bodies carry a
	// load-balance foreign key.
	ListCDNLoadBalances(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNLoadBalance, error)
	CreateCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, lb domain.CDNLoadBalance) (*domain.CDNLoadBalance, error)
	GetCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNLoadBalance, error)
	UpdateCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string, lb domain.CDNLoadBalance) (*domain.CDNLoadBalance, error)
	// DeleteCDNLoadBalance removes a load-balance pool by ID. As with
	// DeleteServer, an already-absent ID reports domain.ErrNotFound.
	DeleteCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) error
	ListCDNLoadBalanceServers(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, loadBalanceID string) ([]domain.CDNLoadBalanceServer, error)
	CreateCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, srv domain.CDNLoadBalanceServer) (*domain.CDNLoadBalanceServer, error)
	GetCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNLoadBalanceServer, error)
	UpdateCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string, srv domain.CDNLoadBalanceServer) (*domain.CDNLoadBalanceServer, error)
	// DeleteCDNLoadBalanceServer removes a backend server by ID. As with
	// DeleteServer, an already-absent ID reports domain.ErrNotFound.
	DeleteCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) error

	// ModSec (Web Application Firewall custom rules), scoped to a CDN zone
	// like DNS records (AGENTS.md 4.1). All fast operations (issue #24).
	// Two provider response quirks callers should know: CreateCDNModSecData
	// and CreateCDNModSecRule's response carries no id — call the matching
	// List method afterward to discover it; GetCDNModSecRule's response
	// omits the rule's name/status (unlike ListCDNModSecRules).
	GetCDNModSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNModSecStatus, error)
	// UpdateCDNModSecStatus replaces the set of selected ModSec rule-set ids
	// (standard and/or custom) for the zone; pass an empty slice to clear
	// every selection. The provider's response carries no data, so this
	// re-fetches and returns the zone's status afterward.
	UpdateCDNModSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, selectedRuleIDs []string) (*domain.CDNModSecStatus, error)
	ListCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNModSecData, error)
	CreateCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, data domain.CDNModSecData) (*domain.CDNModSecData, error)
	GetCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNModSecData, error)
	UpdateCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string, data domain.CDNModSecData) (*domain.CDNModSecData, error)
	// DeleteCDNModSecData removes a ModSec data value by id. As with
	// DeleteCDNZone, an already-absent value reports domain.ErrNotFound
	// rather than succeeding silently.
	DeleteCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) error
	ListCDNModSecRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNModSecRule, error)
	CreateCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNModSecRule) (*domain.CDNModSecRule, error)
	GetCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNModSecRule, error)
	UpdateCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNModSecRule) (*domain.CDNModSecRule, error)
	// DeleteCDNModSecRule removes a custom rule by id. As with
	// DeleteCDNZone, an already-absent rule reports domain.ErrNotFound
	// rather than succeeding silently.
	DeleteCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error

	// Zone-level network settings (issue #24): HTTPS convertor, edge-to-
	// upstream connection protocol, www redirection, and WebSocket support.
	// All fast operations.
	GetCDNHTTPSConvertor(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNHTTPSConvertorSetting, error)
	UpdateCDNHTTPSConvertor(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNHTTPSConvertorSetting) (*domain.CDNHTTPSConvertorSetting, error)
	GetCDNEdgeToUpstreamConnection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNEdgeToUpstreamConnectionSetting, error)
	UpdateCDNEdgeToUpstreamConnection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNEdgeToUpstreamConnectionSetting) (*domain.CDNEdgeToUpstreamConnectionSetting, error)
	GetCDNWWWRedirection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNWWWRedirectionSetting, error)
	UpdateCDNWWWRedirection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNWWWRedirectionSetting) (*domain.CDNWWWRedirectionSetting, error)
	GetCDNWebSocket(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNWebSocketSetting, error)
	UpdateCDNWebSocket(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNWebSocketSetting) (*domain.CDNWebSocketSetting, error)

	// CDN Origin Rules (issue #24): route matching traffic in a zone to a
	// specific origin (a fixed IP, a port, or a load balancer). All fast
	// operations. Create/Update do not echo the rule's ID in the response —
	// callers should follow up with ListCDNOriginRules to discover it.
	ListCDNOriginRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNOriginRule, error)
	CreateCDNOriginRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNOriginRule) (*domain.CDNOriginRule, error)
	GetCDNOriginRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNOriginRule, error)
	UpdateCDNOriginRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNOriginRule) (*domain.CDNOriginRule, error)
	// DeleteCDNOriginRule removes an origin rule by ID. As with
	// DeleteServer, an already-absent ID reports domain.ErrNotFound.
	DeleteCDNOriginRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error
	// ToggleCDNOriginRule enables or disables a rule without deleting it.
	ToggleCDNOriginRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, enabled bool) error

	// CDN Page Rules (issue #24): apply per-URL/user-agent settings
	// overrides (caching, firewall, headers, cookies, redirects, ...) to
	// matching traffic in a zone. All fast operations. No toggle-status
	// endpoint exists for this rule engine — a page rule is removed, not
	// disabled, to stop applying it.
	ListCDNPageRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNPageRule, error)
	CreateCDNPageRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNPageRule) (*domain.CDNPageRule, error)
	GetCDNPageRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNPageRule, error)
	UpdateCDNPageRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNPageRule) (*domain.CDNPageRule, error)
	// DeleteCDNPageRule removes a page rule by ID; an already-absent ID
	// reports domain.ErrNotFound, same contract as DeleteCDNOriginRule.
	DeleteCDNPageRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error

	// CDN Transform Rules (issue #24): rewrite request/response headers on
	// matching traffic in a zone, matched by the same nested condition tree
	// shape as Origin Rules. All fast operations.
	ListCDNTransformRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNTransformRule, error)
	CreateCDNTransformRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNTransformRule) (*domain.CDNTransformRule, error)
	GetCDNTransformRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNTransformRule, error)
	UpdateCDNTransformRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNTransformRule) (*domain.CDNTransformRule, error)
	// DeleteCDNTransformRule removes a transform rule by ID; an
	// already-absent ID reports domain.ErrNotFound, same contract as
	// DeleteCDNOriginRule.
	DeleteCDNTransformRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error
	// ToggleCDNTransformRule enables or disables a rule without deleting it.
	ToggleCDNTransformRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, enabled bool) error

	// Rate Limit Rules and the Upstream Errors toggle, two more corners of
	// the CDN edge firewall (issue #24), both scoped to a CDN zone like DNS
	// records (AGENTS.md 4.1). All fast operations. The store and update
	// endpoints do not return the affected resource in their response body,
	// so CreateCDNRateLimitRule/UpdateCDNRateLimitRule/
	// UpdateCDNUpstreamErrors echo back the request instead of a
	// provider-reported value; call ListCDNRateLimitRules afterward to
	// discover a newly created rule's ID.
	ListCDNRateLimitRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNRateLimitRule, error)
	CreateCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNRateLimitRule) (*domain.CDNRateLimitRule, error)
	GetCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNRateLimitRule, error)
	UpdateCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNRateLimitRule) (*domain.CDNRateLimitRule, error)
	// DeleteCDNRateLimitRule removes a rule by ID. As with DeleteServer, an
	// already-absent ID reports domain.ErrNotFound rather than succeeding
	// silently.
	DeleteCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error
	// UpdateCDNRateLimitRulePriority reorders a rule's evaluation priority
	// relative to the zone's other rate limit rules; lower values are
	// evaluated first.
	UpdateCDNRateLimitRulePriority(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, priority int) error
	GetCDNUpstreamErrors(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNUpstreamErrorSettings, error)
	UpdateCDNUpstreamErrors(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (*domain.CDNUpstreamErrorSettings, error)

	// CDN report/analytics (read-only) and CDN-zone-level SSL settings
	// (issue #24). All fast operations. Distinct from the SSL certificate
	// ORDERING methods above (ListSSLProducts..ReissueSSLCertificate, the
	// separate sslv2 surface): these are scoped to a CDN zone on the cdnapi
	// surface.
	GetCDNAccessLog(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, query domain.CDNLogQuery) (*domain.CDNAccessLogPage, error)
	GetCDNSecurityLog(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, query domain.CDNLogQuery) (*domain.CDNSecurityLogPage, error)
	GetCDNErrorLog(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, query domain.CDNLogQuery) (*domain.CDNErrorLogPage, error)
	GetCDNWAFLog(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, query domain.CDNLogQuery) (*domain.CDNWAFLogPage, error)
	GetCDNTopVisitors(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, start, end string) ([]domain.CDNTopVisitor, error)
	GetCDNMonthlyTrafficUsage(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNTrafficUsage, error)
	GetCDNMinTLSVersion(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (domain.CDNMinTLSVersion, error)
	UpdateCDNMinTLSVersion(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, version domain.CDNMinTLSVersion) error
	// ListCDNCertificates is read-only: lists certificates already attached
	// to a zone. Ordering a new certificate remains the separate SSL-surface
	// workflow above (issue #18).
	ListCDNCertificates(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, perPage, page int, domainFilter string) ([]domain.CDNCertificate, error)
	GetCDNHSTS(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNHSTSSettings, error)
	UpdateCDNHSTS(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, settings domain.CDNHSTSSettings) error
}

// Clock reports the current time. Injected so use cases stay deterministic
// under test.
type Clock interface {
	Now() time.Time
}

// IDGenerator produces operation and job identifiers.
type IDGenerator interface {
	NewID() string
}
