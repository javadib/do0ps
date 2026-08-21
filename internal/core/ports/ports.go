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

// JobSettled is called once a job reaches a terminal state, so the use case
// that owns it can release whatever it holds for the operation in memory —
// above all the caller's credentials, which a retry still needs and which are
// never persisted. Registered alongside the handler at wiring time.
type JobSettled func(jobID string)

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

// ArvanCloudProvider is the port for the ArvanCloud CDN API
// (docs/api-specs/arvancloud-cdn-4.0.yml). It is a second dedicated
// per-provider port, not a shared interface with ParspackProvider: AGENTS.md
// 4.1 defers that unification until two or three providers make the real
// overlap visible, and the two APIs' field-level shapes are not close enough
// to force it from two.
//
// The same call conventions as ParspackProvider apply:
//
//   - Credentials are passed per call. They belong to the chatbot session and
//     are never held by this server (AGENTS.md 4.2).
//   - Every method's doc comment states whether it is a fast or a long
//     operation (AGENTS.md 4.3), so the use case above it knows whether to
//     block on the queue or hand back an operation ID.
//   - Create-style methods are not assumed idempotent. A caller that must not
//     duplicate a resource on retry (crash recovery, AGENTS.md 4.4) checks
//     first, e.g. by listing.
//
// One convention differs, and it is the reason this port cannot simply mirror
// the Parspack one: ArvanCloud addresses sub-resources by the domain NAME, so
// methods take a `domain string` (e.g. "example.com") where the Parspack port
// takes a zoneUUID.
//
// The interface started deliberately empty (issue #61); this is the first
// capability to extend it. Other CDN capabilities (DNS records, firewall,
// WAF, load balancing, ...) each land in their own issue and extend this
// interface the same way.
type ArvanCloudProvider interface {
	// Domain lifecycle (issue #62): onboarding a domain onto ArvanCloud's
	// CDN, listing/inspecting it, and removing it. All fast operations — the
	// CDN API answers each of these in one synchronous round trip.
	//
	// CreateDomain is not assumed idempotent (see above); a caller that must
	// not duplicate a domain on retry checks first, e.g. via ListDomains.
	ListDomains(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudDomain, error)
	CreateDomain(ctx context.Context, creds domain.ProviderCredentials, spec domain.ArvanCloudDomainSpec) (*domain.ArvanCloudDomain, error)
	GetDomain(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error)
	// DeleteDomain removes a domain by name. As with ParspackProvider's
	// DeleteServer/DeleteCDNZone, an already-absent domain reports
	// domain.ErrNotFound rather than succeeding silently, so callers decide
	// for themselves whether that counts as done.
	DeleteDomain(ctx context.Context, creds domain.ProviderCredentials, domainName string) error

	// NS Setup (issue #62), for a "full" (NS-based) domain: the registrar
	// points its nameservers at ArvanCloud, which then owns DNS resolution
	// for the whole domain. All fast operations. Each returns the domain's
	// nameserver-related fields only — the CDN API's set/reset/optional-keys
	// endpoints do not echo the rest of the domain resource.
	SetNSKeys(ctx context.Context, creds domain.ProviderCredentials, domainName string, nsKeys []string) (*domain.ArvanCloudDomain, error)
	ResetNSKeys(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error)
	// CheckNSStatus reports whether the registrar has actually been
	// repointed: the returned domain's NSKeys is what ArvanCloud expects,
	// CurrentNS is what it currently sees configured at the registrar.
	CheckNSStatus(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error)
	// UseOptionalNSKeys switches the domain to ArvanCloud's alternate NS key
	// set (e.g. when the primary set is rejected by a registrar).
	UseOptionalNSKeys(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error)

	// CNAME Setup (issue #62), for a "partial" domain: only a subdomain's
	// traffic is routed through ArvanCloud via a CNAME record, while the rest
	// of the domain's DNS stays wherever it already is hosted. This is the
	// mode for a caller who says something like "my domain's DNS is hosted
	// elsewhere, I just want the CDN on a subdomain" — NS Setup above would
	// instead require moving the whole domain's DNS to ArvanCloud. All fast
	// operations; each returns the domain resource as ArvanCloud reports it
	// after the change.
	SetCnameTarget(ctx context.Context, creds domain.ProviderCredentials, domainName, address string) (*domain.ArvanCloudDomain, error)
	ResetCnameTarget(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error)
	ConvertToCnameSetup(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error)
	// CheckCnameStatus reports whether the CNAME has been activated yet.
	CheckCnameStatus(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error)

	// Other domain actions (issue #62). All fast operations.
	//
	// CloneDomainConfig copies another domain's CDN configuration (cache
	// rules, firewall, ...) onto domainName; fromDomain is the domain the
	// configuration is copied FROM.
	CloneDomainConfig(ctx context.Context, creds domain.ProviderCredentials, domainName, fromDomain string) error
	// RegenerateDomainConfig re-publishes the domain's current configuration
	// to the edge servers. The call itself returns immediately; the actual
	// propagation to edge servers happens asynchronously on ArvanCloud's own
	// side afterward, with nothing this port exposes to poll for it.
	RegenerateDomainConfig(ctx context.Context, creds domain.ProviderCredentials, domainName string) error
	// HoldDomain pauses CDN service for the domain; UnholdDomain resumes it.
	HoldDomain(ctx context.Context, creds domain.ProviderCredentials, domainName string) error
	UnholdDomain(ctx context.Context, creds domain.ProviderCredentials, domainName string) error

	// DNS records, scoped to a domain by name — same DNS-inside-domain model
	// as the methods above (AGENTS.md 4.1), with one addressing difference
	// from Parspack: a record here is identified by a per-record UUID, not
	// by host+type (issue #63). All fast operations.
	ListArvanCloudDNSRecords(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudDNSRecord, error)
	CreateArvanCloudDNSRecord(ctx context.Context, creds domain.ProviderCredentials, domainName string, rec domain.ArvanCloudDNSRecord) (*domain.ArvanCloudDNSRecord, error)
	GetArvanCloudDNSRecord(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudDNSRecord, error)
	UpdateArvanCloudDNSRecord(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rec domain.ArvanCloudDNSRecord) (*domain.ArvanCloudDNSRecord, error)
	// DeleteArvanCloudDNSRecord removes a record by id. As with DeleteDomain,
	// an already-absent record reports domain.ErrNotFound rather than
	// succeeding silently. A record the provider refuses to delete (a
	// protected record, or one still referenced elsewhere — BaseDnsRecord's
	// is_protected/usage) is not pre-checked client-side; whatever error the
	// provider returns is propagated as-is (issue #63's scope note).
	DeleteArvanCloudDNSRecord(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error
	// ToggleArvanCloudDNSRecordCloud flips the CDN-proxy ("cloud") status of
	// one record without changing anything else about it.
	ToggleArvanCloudDNSRecordCloud(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, cloud bool) (*domain.ArvanCloudDNSRecord, error)
	// ImportArvanCloudDNSRecords bulk-creates records from a BIND zone file.
	// Unlike every other method on this port, the request body is not JSON
	// (the spec declares dns-records.import as multipart/form-data).
	ImportArvanCloudDNSRecords(ctx context.Context, creds domain.ProviderCredentials, domainName string, zoneFile []byte) error
	// ExportArvanCloudDNSRecords returns the domain's records as a BIND zone
	// file. Unlike every other method on this port, the response body is not
	// JSON (the spec declares dns-records.export's 200 response as
	// text/plain).
	ExportArvanCloudDNSRecords(ctx context.Context, creds domain.ProviderCredentials, domainName string) (string, error)

	// DNSSEC, scoped to a domain by name (issue #63). Both fast operations.
	GetArvanCloudDNSSecStatus(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDNSSecStatus, error)
	UpdateArvanCloudDNSSecStatus(ctx context.Context, creds domain.ProviderCredentials, domainName string, enable, rotate bool) (*domain.ArvanCloudDNSSecStatus, error)

	// Secondary DNS, scoped to a domain by name (issue #63): ArvanCloud
	// transfers zone data from another primary nameserver via AXFR/IXFR. All
	// fast operations. RemoveArvanCloudSecondaryDNS tolerates a domain that
	// no longer has a Secondary DNS config the same way DeleteDomain
	// tolerates an already-absent domain: an already-absent config reports
	// domain.ErrNotFound rather than succeeding silently.
	GetArvanCloudSecondaryDNS(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudSecondaryDNSConfig, error)
	SetArvanCloudSecondaryDNS(ctx context.Context, creds domain.ProviderCredentials, domainName string, config domain.ArvanCloudSecondaryDNSConfig) (*domain.ArvanCloudSecondaryDNSConfig, error)
	RemoveArvanCloudSecondaryDNS(ctx context.Context, creds domain.ProviderCredentials, domainName string) error

	// Lists ("dynamic-fields", issue #64): a reusable, account-scoped
	// collection of values that other CDN capabilities (firewall, WAF, DDoS
	// protection, rate limiting — AC5-AC8) reference by ID from their own
	// filter/source fields. Unlike every other capability on this port,
	// Lists are account-scoped, not scoped to a domain by name. All fast
	// operations.
	//
	// ListArvanCloudDynamicFields returns every list visible to the given
	// credentials, unfiltered — the spec's optional scope/type/name query
	// parameters are not exposed by this port, matching ListDomains' own
	// choice to keep listing unfiltered (issue #62).
	ListArvanCloudDynamicFields(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudDynamicField, error)
	CreateArvanCloudDynamicField(ctx context.Context, creds domain.ProviderCredentials, field domain.ArvanCloudDynamicField) (*domain.ArvanCloudDynamicField, error)
	GetArvanCloudDynamicField(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.ArvanCloudDynamicField, error)
	// UpdateArvanCloudDynamicField changes a list's description and/or type.
	// The spec's lists.update operation is marked deprecated but has no
	// documented replacement, so it is still implemented here (see
	// docs/api-specs/arvancloud-cdn-4.0.yml's DynamicFieldUpdateRequest).
	UpdateArvanCloudDynamicField(ctx context.Context, creds domain.ProviderCredentials, id string, description string, fieldType domain.ArvanCloudDynamicFieldType) (*domain.ArvanCloudDynamicField, error)
	// DeleteArvanCloudDynamicField removes a list by id. As with
	// DeleteDomain, an already-absent list reports domain.ErrNotFound rather
	// than succeeding silently.
	DeleteArvanCloudDynamicField(ctx context.Context, creds domain.ProviderCredentials, id string) error
	// AddArvanCloudDynamicFieldItems appends items to a list. The endpoint's
	// response carries no data to translate — only a confirmation message —
	// so there is nothing for this method to return but the error; a caller
	// that needs the newly-assigned item IDs calls GetArvanCloudDynamicField
	// afterward.
	AddArvanCloudDynamicFieldItems(ctx context.Context, creds domain.ProviderCredentials, id string, values []domain.ArvanCloudDynamicFieldValue) error
	// RemoveArvanCloudDynamicFieldItem removes one item from a list by the
	// item's own id (domain.ArvanCloudDynamicFieldValue.ID), not an index.
	RemoveArvanCloudDynamicFieldItem(ctx context.Context, creds domain.ProviderCredentials, id, itemID string) error

	// Firewall — domain-level, scoped to a domain by name (issue #65): the
	// CDN edge-level L7 firewall, evaluated per-request against each rule's
	// filter_expr. Naming collision warning: this has no relationship to any
	// future ArvanCloud IaaS/cloud-server firewall (AGENTS.md 4.1/4.5, see
	// domain/arvancloud_firewall.go's package comment). All fast operations.
	//
	// CreateArvanCloudFirewallRule is not assumed idempotent (see this
	// interface's own doc comment above); a caller that must not duplicate a
	// rule on retry checks first, e.g. via ListArvanCloudFirewallRules.
	GetArvanCloudFirewallSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudFirewallSettings, error)
	UpdateArvanCloudFirewallSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudFirewallSettings) (*domain.ArvanCloudFirewallSettings, error)
	ListArvanCloudFirewallRules(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudFirewallRule, error)
	CreateArvanCloudFirewallRule(ctx context.Context, creds domain.ProviderCredentials, domainName string, rule domain.ArvanCloudFirewallRule) (*domain.ArvanCloudFirewallRule, error)
	GetArvanCloudFirewallRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudFirewallRule, error)
	UpdateArvanCloudFirewallRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rule domain.ArvanCloudFirewallRule) (*domain.ArvanCloudFirewallRule, error)
	// DeleteArvanCloudFirewallRule removes a rule by id. As with
	// DeleteDomain, an already-absent rule reports domain.ErrNotFound rather
	// than succeeding silently.
	DeleteArvanCloudFirewallRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error
	// ReprioritizeArvanCloudFirewallRules moves ruleID to a new position
	// relative to another rule: exactly one of afterRuleID/beforeRuleID
	// should be given, per the spec's ReprioritizeRuleRequest description
	// ("You should only provide either after_rule_id or before_rule_id (and
	// not both of them)").
	ReprioritizeArvanCloudFirewallRules(ctx context.Context, creds domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error

	// Firewall — account-level (issue #65): the same rule shape as the
	// domain-level firewall above, but applied across a settable subset of
	// the account's domains (domain.ArvanCloudAccountFirewallRule's
	// DomainSelectionType/DomainIDs) instead of one domain named in the URL
	// path. Unlike every domain-scoped method above, none of these take a
	// domainName. All fast operations.
	//
	// ListArvanCloudAccountFirewallValidDomains lists the account's active
	// enterprise domains eligible to be targeted by DomainIDs on create or
	// via Attach/DetachArvanCloudAccountFirewallDomains below.
	ListArvanCloudAccountFirewallValidDomains(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudAccountFirewallValidDomain, error)
	ListArvanCloudAccountFirewallRules(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudAccountFirewallRule, error)
	// CreateArvanCloudAccountFirewallRule is not assumed idempotent, same as
	// CreateArvanCloudFirewallRule above.
	CreateArvanCloudAccountFirewallRule(ctx context.Context, creds domain.ProviderCredentials, rule domain.ArvanCloudAccountFirewallRule) (*domain.ArvanCloudAccountFirewallRule, error)
	GetArvanCloudAccountFirewallRule(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.ArvanCloudAccountFirewallRule, error)
	UpdateArvanCloudAccountFirewallRule(ctx context.Context, creds domain.ProviderCredentials, id string, rule domain.ArvanCloudAccountFirewallRule) (*domain.ArvanCloudAccountFirewallRule, error)
	// DeleteArvanCloudAccountFirewallRule removes a rule by id. As with
	// DeleteDomain, an already-absent rule reports domain.ErrNotFound rather
	// than succeeding silently.
	DeleteArvanCloudAccountFirewallRule(ctx context.Context, creds domain.ProviderCredentials, id string) error
	// AttachArvanCloudAccountFirewallDomains/DetachArvanCloudAccountFirewallDomains
	// add or remove domains from an "include"/"exclude" rule's DomainIDs
	// without resubmitting the whole rule. Both return the rule as stored
	// afterward.
	AttachArvanCloudAccountFirewallDomains(ctx context.Context, creds domain.ProviderCredentials, id string, domainIDs []string) (*domain.ArvanCloudAccountFirewallRule, error)
	DetachArvanCloudAccountFirewallDomains(ctx context.Context, creds domain.ProviderCredentials, id string, domainIDs []string) (*domain.ArvanCloudAccountFirewallRule, error)
	// ReprioritizeArvanCloudAccountFirewallRules moves ruleID to a new
	// position, same "exactly one of after/before" contract as
	// ReprioritizeArvanCloudFirewallRules above.
	ReprioritizeArvanCloudAccountFirewallRules(ctx context.Context, creds domain.ProviderCredentials, ruleID, afterRuleID, beforeRuleID string) error

	// WAF (issue #66): ArvanCloud's managed rule-set engine (OWASP-style
	// packages/presets a domain subscribes to), distinct from the CDN edge
	// Firewall above — see domain/arvancloud_waf.go's package comment for
	// the naming-collision warning. All fast operations.
	//
	// Global (account-independent reference data): read-only presets and
	// package catalog, shared across every account.
	ListArvanCloudWafPresets(ctx context.Context, creds domain.ProviderCredentials) (*domain.ArvanCloudWafPresetsAndPackages, error)
	GetArvanCloudWafPackage(ctx context.Context, creds domain.ProviderCredentials, packageID string) (*domain.ArvanCloudWafPackage, error)
	GetArvanCloudWafPackageRules(ctx context.Context, creds domain.ProviderCredentials, packageID string) ([]domain.ArvanCloudWafPackageRule, error)

	// Per-domain WAF configuration, scoped to a domain by name.
	GetArvanCloudWafSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudWafSettings, error)
	UpdateArvanCloudWafSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudWafSettings) (*domain.ArvanCloudWafSettings, error)
	// ReconfigureArvanCloudWaf applies presetID to domainName in one call,
	// removing every WAF package currently installed on the domain and
	// replacing it with the preset's own set (per the spec's
	// waf.reconfigure description). This is the tool for a request like
	// "turn on OWASP-style protection" or "block SQL injection attempts" —
	// see domain/arvancloud_waf.go's package comment.
	ReconfigureArvanCloudWaf(ctx context.Context, creds domain.ProviderCredentials, domainName, presetID string) error
	// ReprioritizeArvanCloudWafRules moves ruleID to a new position relative
	// to another custom rule: exactly one of afterRuleID/beforeRuleID should
	// be given, same contract as ReprioritizeArvanCloudFirewallRules above.
	ReprioritizeArvanCloudWafRules(ctx context.Context, creds domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error
	// ReprioritizeArvanCloudWafPackages moves packageID to a new position
	// relative to another installed package: exactly one of
	// afterPackageID/beforePackageID should be given.
	ReprioritizeArvanCloudWafPackages(ctx context.Context, creds domain.ProviderCredentials, domainName, packageID, afterPackageID, beforePackageID string) error

	// Per-domain WAF custom rules (waf.rules.*), the thin exception layer on
	// top of the managed packages — see domain/arvancloud_waf.go's package
	// comment for how this differs from the CDN edge Firewall's rules.
	//
	// CreateArvanCloudWafRule is not assumed idempotent (see this
	// interface's own doc comment above); a caller that must not duplicate a
	// rule on retry checks first, e.g. via ListArvanCloudWafRules.
	ListArvanCloudWafRules(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudWafRule, error)
	CreateArvanCloudWafRule(ctx context.Context, creds domain.ProviderCredentials, domainName string, rule domain.ArvanCloudWafRule) (*domain.ArvanCloudWafRule, error)
	GetArvanCloudWafRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudWafRule, error)
	UpdateArvanCloudWafRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rule domain.ArvanCloudWafRule) (*domain.ArvanCloudWafRule, error)
	// DeleteArvanCloudWafRule removes a custom rule by id. As with
	// DeleteDomain, an already-absent rule reports domain.ErrNotFound rather
	// than succeeding silently.
	DeleteArvanCloudWafRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error

	// Per-domain WAF package subscriptions (waf.packages.*): which managed
	// packages are installed on a domain's WAF, and their per-package
	// configuration (disabled rules/rulesets, params).
	ListArvanCloudWafDomainPackages(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudWafPackage, error)
	// InstallArvanCloudWafPackage subscribes domainName to the global
	// package identified by packageID (DomainWafPackageStore's only field).
	// Not assumed idempotent, same caveat as CreateArvanCloudWafRule above.
	InstallArvanCloudWafPackage(ctx context.Context, creds domain.ProviderCredentials, domainName, packageID string) (*domain.ArvanCloudWafPackage, error)
	GetArvanCloudWafDomainPackage(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudWafPackage, error)
	// UpdateArvanCloudWafDomainPackage changes an installed package's own
	// configuration, e.g. toggling IsEnabled or its DisabledRules/
	// DisabledRulesets/Params.
	UpdateArvanCloudWafDomainPackage(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, pkg domain.ArvanCloudWafPackage) (*domain.ArvanCloudWafPackage, error)
	// UninstallArvanCloudWafPackage removes an installed package from the
	// domain by id. As with DeleteDomain, an already-absent package reports
	// domain.ErrNotFound rather than succeeding silently.
	UninstallArvanCloudWafPackage(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error

	// DDoS Protection (issue #67): a per-domain challenge engine
	// (cookie/JavaScript/CAPTCHA-based), distinct from both the CDN edge
	// Firewall and the WAF managed rule-set engine above — see
	// domain/arvancloud_ddos.go's package comment for the naming-collision
	// warning. All fast operations.
	//
	// GetArvanCloudDdosSettings/UpdateArvanCloudDdosSettings manage the
	// domain's challenge mechanism (ProtectionMode) and, when it is
	// "captcha", the CAPTCHA vendor configuration. settings.SecretKey is
	// caller-supplied CAPTCHA provider material, not an ArvanCloud
	// credential — treated as sensitive by the adapter (never logged, never
	// embedded in an error message), see
	// domain.ArvanCloudDdosSettings.SecretKey's doc comment.
	GetArvanCloudDdosSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDdosSettings, error)
	UpdateArvanCloudDdosSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudDdosSettings) (*domain.ArvanCloudDdosSettings, error)

	// Per-domain DDoS rules (ddos.rules.*): a caller-authored exemption or
	// enforcement layered on top of the domain's DDoS challenge settings.
	//
	// CreateArvanCloudDdosRule is not assumed idempotent (see this
	// interface's own doc comment above); a caller that must not duplicate a
	// rule on retry checks first, e.g. via ListArvanCloudDdosRules.
	ListArvanCloudDdosRules(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudDdosRule, error)
	CreateArvanCloudDdosRule(ctx context.Context, creds domain.ProviderCredentials, domainName string, rule domain.ArvanCloudDdosRule) (*domain.ArvanCloudDdosRule, error)
	GetArvanCloudDdosRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudDdosRule, error)
	UpdateArvanCloudDdosRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rule domain.ArvanCloudDdosRule) (*domain.ArvanCloudDdosRule, error)
	// DeleteArvanCloudDdosRule removes a rule by id. As with DeleteDomain, an
	// already-absent rule reports domain.ErrNotFound rather than succeeding
	// silently.
	DeleteArvanCloudDdosRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error
	// ReprioritizeArvanCloudDdosRules moves ruleID to a new position relative
	// to another rule: exactly one of afterRuleID/beforeRuleID should be
	// given, same contract as ReprioritizeArvanCloudFirewallRules above.
	ReprioritizeArvanCloudDdosRules(ctx context.Context, creds domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error

	// Rate Limiting (issue #68): per-domain request-rate settings plus a rule
	// engine that throttles or blocks traffic exceeding a configured rate —
	// distinct from the CDN edge Firewall, the WAF managed rule-set engine
	// and the dedicated DDoS challenge engine above, see
	// domain/arvancloud_ratelimit.go's package comment. All fast operations.
	//
	// GetArvanCloudRateLimitSettings/UpdateArvanCloudRateLimitSettings manage
	// the domain's automatic rate-based DDoS detection toggle and its
	// globally exempted sources.
	GetArvanCloudRateLimitSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudRateLimitSettings, error)
	UpdateArvanCloudRateLimitSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudRateLimitSettings) (*domain.ArvanCloudRateLimitSettings, error)

	// Per-domain rate-limit rules (rate-limiting.rules.*): throttle or block
	// traffic matching a URL pattern once a source exceeds a configured
	// request rate.
	//
	// CreateArvanCloudRateLimitRule is not assumed idempotent (see this
	// interface's own doc comment above); a caller that must not duplicate a
	// rule on retry checks first, e.g. via ListArvanCloudRateLimitRules.
	ListArvanCloudRateLimitRules(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudRateLimitRule, error)
	CreateArvanCloudRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, domainName string, rule domain.ArvanCloudRateLimitRule) (*domain.ArvanCloudRateLimitRule, error)
	GetArvanCloudRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudRateLimitRule, error)
	UpdateArvanCloudRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rule domain.ArvanCloudRateLimitRule) (*domain.ArvanCloudRateLimitRule, error)
	// DeleteArvanCloudRateLimitRule removes a rule by id. As with
	// DeleteArvanCloudDdosRule, an already-absent rule reports
	// domain.ErrNotFound rather than succeeding silently.
	DeleteArvanCloudRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error
	// ReprioritizeArvanCloudRateLimitRules moves ruleID to a new position
	// relative to another rule: exactly one of afterRuleID/beforeRuleID
	// should be given, same contract as ReprioritizeArvanCloudFirewallRules
	// above.
	ReprioritizeArvanCloudRateLimitRules(ctx context.Context, creds domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error

	// Load Balancing (issue #69): ArvanCloud's CDN edge-level traffic
	// distribution across origin pools — a 3-level resource hierarchy (load
	// balancer -> pools -> origins), see domain/arvancloud_loadbalancer.go's
	// package comment for the naming-collision warning against
	// domain.LoadBalancer (this port's own CreateLoadBalancer/GetLoadBalancer/
	// ... above, the cloud-server/VM-network-level resource). All fast
	// operations: creating a load balancer configures routing rules, it does
	// not provision new infrastructure, unlike CreateLoadBalancer above
	// (a long operation, AGENTS.md 4.3) — do not copy that polling pattern
	// here.
	//
	// ListArvanCloudLBRegions is account-independent (no domainName); the
	// spec marks its operationId (load-balancers.regions.index) deprecated,
	// but it is still implemented. ListArvanCloudDomainLBRegions is the
	// per-domain equivalent (load-balancers.regions). Both are exposed since
	// the spec does not document the two as always returning identical data.
	ListArvanCloudLBRegions(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudLoadBalancerRegion, error)
	ListArvanCloudDomainLBRegions(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudLoadBalancerRegion, error)

	// GetArvanCloudLBSettings/UpdateArvanCloudLBSettings manage a domain's
	// global load-balancing defaults, applied to every load balancer on the
	// domain unless a pool overrides a field itself.
	GetArvanCloudLBSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudLoadBalancerSettings, error)
	UpdateArvanCloudLBSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudLoadBalancerSettings) (*domain.ArvanCloudLoadBalancerSettings, error)

	// Load balancers, scoped to a domain by name.
	//
	// CreateArvanCloudLoadBalancer is not assumed idempotent (see this
	// interface's own doc comment above); a caller that must not duplicate a
	// load balancer on retry checks first, e.g. via
	// ListArvanCloudLoadBalancers.
	ListArvanCloudLoadBalancers(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudLoadBalancer, error)
	CreateArvanCloudLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, domainName string, lb domain.ArvanCloudLoadBalancer) (*domain.ArvanCloudLoadBalancer, error)
	GetArvanCloudLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudLoadBalancer, error)
	UpdateArvanCloudLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, lb domain.ArvanCloudLoadBalancer) (*domain.ArvanCloudLoadBalancer, error)
	// DeleteArvanCloudLoadBalancer removes a load balancer by id. As with
	// DeleteDomain, an already-absent load balancer reports
	// domain.ErrNotFound rather than succeeding silently.
	DeleteArvanCloudLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error

	// Pools, scoped to a load balancer by id.
	//
	// CreateArvanCloudLBPool is not assumed idempotent, same caveat as
	// CreateArvanCloudLoadBalancer above.
	ListArvanCloudLBPools(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID string) ([]domain.ArvanCloudLoadBalancerPool, error)
	CreateArvanCloudLBPool(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID string, pool domain.ArvanCloudLoadBalancerPool) (*domain.ArvanCloudLoadBalancerPool, error)
	// ReprioritizeArvanCloudLBPool moves poolID relative to
	// afterPoolID/beforePoolID within loadBalancerID's pool set: exactly one
	// of the two should be given, per the spec's PrioritizePool schema
	// (PrioritizePoolAfter/PrioritizePoolBefore's pool_id/after_pool_id and
	// pool_id/before_pool_id) — a relative "move this pool before/after that
	// pool" operation, NOT a full reordered-array reprioritize like the rule
	// engines above (issue #69's explicit warning: do not copy that
	// pattern). Returns the load balancer as stored afterward
	// (load-balancers.prioritize_pool's 200 response is the LoadBalancer
	// resource itself, unlike the rule-engine reprioritize methods above,
	// which return no data).
	ReprioritizeArvanCloudLBPool(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID, afterPoolID, beforePoolID string) (*domain.ArvanCloudLoadBalancer, error)
	GetArvanCloudLBPool(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string) (*domain.ArvanCloudLoadBalancerPool, error)
	// UpdateArvanCloudLBPoolWithOrigins (PUT, load-balancers.pools.update)
	// replaces the pool AND its full set of origins in one call — any
	// existing origin not included in pool.Origins is removed.
	// UpdateArvanCloudLBPoolSettings (PATCH, load-balancers.pools.updatePool)
	// updates the pool's own settings only and leaves its existing origins
	// untouched. These are two distinct update semantics for the same
	// resource, kept as separate methods on purpose: silently picking the
	// wrong one either wipes origins the caller didn't intend to touch (PUT
	// for a settings-only change) or fails to update origins the caller
	// expected to change (PATCH expecting origin changes to apply) — issue
	// #69's acceptance criteria calls this out as the highest-risk part of
	// this port.
	UpdateArvanCloudLBPoolWithOrigins(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string, pool domain.ArvanCloudLoadBalancerPool) (*domain.ArvanCloudLoadBalancerPool, error)
	UpdateArvanCloudLBPoolSettings(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string, pool domain.ArvanCloudLoadBalancerPool) (*domain.ArvanCloudLoadBalancerPool, error)
	// DeleteArvanCloudLBPool removes a pool by id. As with DeleteDomain, an
	// already-absent pool reports domain.ErrNotFound rather than succeeding
	// silently.
	DeleteArvanCloudLBPool(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string) error

	// Origins, scoped to a pool by id.
	//
	// CreateArvanCloudLBPoolOrigin is not assumed idempotent, same caveat as
	// CreateArvanCloudLoadBalancer above.
	ListArvanCloudLBPoolOrigins(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string) ([]domain.ArvanCloudLoadBalancerOrigin, error)
	CreateArvanCloudLBPoolOrigin(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string, origin domain.ArvanCloudLoadBalancerOrigin) (*domain.ArvanCloudLoadBalancerOrigin, error)
	GetArvanCloudLBPoolOrigin(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID, originID string) (*domain.ArvanCloudLoadBalancerOrigin, error)
	UpdateArvanCloudLBPoolOrigin(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID, originID string, origin domain.ArvanCloudLoadBalancerOrigin) (*domain.ArvanCloudLoadBalancerOrigin, error)
	// DeleteArvanCloudLBPoolOrigin removes an origin by id. As with
	// DeleteDomain, an already-absent origin reports domain.ErrNotFound
	// rather than succeeding silently.
	DeleteArvanCloudLBPoolOrigin(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID, originID string) error

	// Active Health Check (issue #70): a domain-scoped monitor that
	// periodically probes an origin (in practice, a Load Balancing pool,
	// #69) over TCP or HTTP(S) and reports whether it is reachable.
	// Conceptually related to Load Balancing but its own resource family —
	// see domain/arvancloud_healthcheck.go's package comment. All fast
	// operations: every one of these API calls (create/read/update/delete a
	// check definition, list zones, or read a report) returns synchronously
	// — the check itself runs continuously on ArvanCloud's infrastructure,
	// but there is no operation_id polling pattern here.
	//
	// CreateArvanCloudHealthCheck is not assumed idempotent (see this
	// interface's own doc comment); a caller that must not duplicate a check
	// on retry checks first, e.g. via ListArvanCloudHealthChecks.
	ListArvanCloudHealthChecks(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudHealthCheck, error)
	CreateArvanCloudHealthCheck(ctx context.Context, creds domain.ProviderCredentials, domainName string, hc domain.ArvanCloudHealthCheck) (*domain.ArvanCloudHealthCheck, error)
	GetArvanCloudHealthCheck(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudHealthCheck, error)
	UpdateArvanCloudHealthCheck(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, hc domain.ArvanCloudHealthCheck) (*domain.ArvanCloudHealthCheck, error)
	// DeleteArvanCloudHealthCheck removes a check by id. As with
	// DeleteArvanCloudLBPoolOrigin, an already-absent check reports
	// domain.ErrNotFound rather than succeeding silently.
	DeleteArvanCloudHealthCheck(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error
	// ListArvanCloudDomainHealthCheckZones lists the check-execution zones
	// available to domainName (health-checks.regions.index, despite the
	// operationId saying "regions" — the path and response say "zones"; the
	// spec's own inconsistency, per issue #70). ListArvanCloudHealthCheckZones
	// is the global, account-independent equivalent
	// (health-checks.zones.index, deprecated in the spec but still
	// implemented, the same "implement both, do not assume they always
	// agree" convention ListArvanCloudLBRegions/ListArvanCloudDomainLBRegions
	// established for #69).
	ListArvanCloudDomainHealthCheckZones(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudHealthCheckZoneName, error)
	ListArvanCloudHealthCheckZones(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudHealthCheckZoneName, error)
	// GetArvanCloudHealthCheckSummary/GetArvanCloudHealthCheckDetails read a
	// single health check's monitoring reports
	// (active-health-check.reports.summary/.details). Only Details is
	// paginated — Summary returns its full per-zone breakdown in one call.
	GetArvanCloudHealthCheckSummary(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudHealthCheckReportQuery) ([]domain.ArvanCloudHealthCheckReportSummary, error)
	GetArvanCloudHealthCheckDetails(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudHealthCheckReportQuery) ([]domain.ArvanCloudHealthCheckReportDetail, domain.ArvanCloudHealthCheckReportPageMeta, error)

	// Page Rules (issue #71): domain-scoped edge rewrite/routing rules, the
	// largest and most complex single resource in the ArvanCloud CDN spec by
	// field count — see domain/arvancloud_rules.go's package comment. All
	// fast operations.
	//
	// CreateArvanCloudPageRule is not assumed idempotent (see this
	// interface's own doc comment); a caller that must not duplicate a rule
	// on retry checks first, e.g. via ListArvanCloudPageRules.
	ListArvanCloudPageRules(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudPageRuleListQuery) ([]domain.ArvanCloudPageRuleSummary, domain.ArvanCloudPageRulePageMeta, error)
	CreateArvanCloudPageRule(ctx context.Context, creds domain.ProviderCredentials, domainName string, rule domain.ArvanCloudPageRule) (*domain.ArvanCloudPageRule, error)
	GetArvanCloudPageRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudPageRule, error)
	// UpdateArvanCloudPageRule replaces a page rule's fields via PUT
	// (page-rules.update).
	UpdateArvanCloudPageRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rule domain.ArvanCloudPageRule) (*domain.ArvanCloudPageRule, error)
	// SetArvanCloudPageRuleStatus toggles ONLY a page rule's status via PATCH
	// (page-rules.status.update) — a separate, narrower endpoint from
	// UpdateArvanCloudPageRule's full PUT replace. The endpoint's response
	// carries no data (just a confirmation message), so this returns only an
	// error; a caller wanting the rule's full state afterward calls
	// GetArvanCloudPageRule.
	SetArvanCloudPageRuleStatus(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, status bool) error
	// DeleteArvanCloudPageRule removes a page rule by id. As with
	// DeleteArvanCloudHealthCheck, an already-absent rule reports
	// domain.ErrNotFound rather than succeeding silently.
	DeleteArvanCloudPageRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error
	// PurgeArvanCloudPageRuleCache purges cached content for URLs matching
	// this page rule (page-rules.purge). The endpoint's response carries no
	// data, so this returns only an error.
	PurgeArvanCloudPageRuleCache(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error
	// GetArvanCloudPageRuleExceptions/UpdateArvanCloudPageRuleExceptions read
	// and replace a page rule's "exceptions" override layer
	// (page-rules.diff.show / page-rules.diff.update) — see
	// domain.ArvanCloudPageRuleExceptions' own doc comment.
	GetArvanCloudPageRuleExceptions(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudPageRuleExceptions, error)
	UpdateArvanCloudPageRuleExceptions(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, exceptions domain.ArvanCloudPageRuleExceptions) (*domain.ArvanCloudPageRuleExceptions, error)

	// Response Transforms (issue #71): domain-scoped, named presets of
	// condition+action steps that add/replace CORS response headers on
	// matching responses. All fast operations.
	//
	// CreateArvanCloudResponseTransform is not assumed idempotent (see this
	// interface's own doc comment); a caller that must not duplicate a
	// preset on retry checks first, e.g. via ListArvanCloudResponseTransforms.
	ListArvanCloudResponseTransforms(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudResponseTransformListQuery) ([]domain.ArvanCloudResponseTransform, domain.ArvanCloudResponseTransformPageMeta, error)
	CreateArvanCloudResponseTransform(ctx context.Context, creds domain.ProviderCredentials, domainName string, rt domain.ArvanCloudResponseTransform) (*domain.ArvanCloudResponseTransform, error)
	GetArvanCloudResponseTransform(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudResponseTransform, error)
	// UpdateArvanCloudResponseTransform updates a preset via PATCH
	// (response_transforms.update); per the spec, omitting Transforms
	// changes only name/description.
	UpdateArvanCloudResponseTransform(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rt domain.ArvanCloudResponseTransform) (*domain.ArvanCloudResponseTransform, error)
	// DeleteArvanCloudResponseTransform removes a preset by id. As with
	// DeleteArvanCloudPageRule, an already-absent preset reports
	// domain.ErrNotFound rather than succeeding silently.
	DeleteArvanCloudResponseTransform(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error

	// Redirect (issue #71): a domain's www-redirect setting
	// (/domains/{domain}/settings/www-redirect). A fast operation.
	GetArvanCloudWWWRedirect(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudWWWRedirectSettings, error)
	// UpdateArvanCloudWWWRedirect's endpoint response carries no data (just a
	// confirmation message), so this returns only an error; a caller wanting
	// the setting's state afterward calls GetArvanCloudWWWRedirect.
	UpdateArvanCloudWWWRedirect(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudWWWRedirectSettings) error

	// Host Header Whitelist (issue #71): controls which CDN accounts (or,
	// when globally whitelisted, any account) may use a domain as the HTTP
	// Host header for requests proxied to it. All fast operations.
	GetArvanCloudHostHeaderWhitelist(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudHostHeaderWhitelist, error)
	// AddArvanCloudHostHeaderWhitelistEntry adds one target-account entry.
	// Rejected by the provider with a 422 while the domain is globally
	// whitelisted (per the spec's own description) — this adapter does not
	// pre-check that client-side, since GloballyWhitelisted can change
	// between a caller's read and write.
	AddArvanCloudHostHeaderWhitelistEntry(ctx context.Context, creds domain.ProviderCredentials, domainName, targetAccount string) (*domain.ArvanCloudHostHeaderWhitelist, error)
	// SetArvanCloudHostHeaderWhitelistSettings sets or clears the domain's
	// global Host allowlist entry. Does not modify the per-account entries.
	SetArvanCloudHostHeaderWhitelistSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, global bool) (*domain.ArvanCloudHostHeaderWhitelist, error)
	// RemoveArvanCloudHostHeaderWhitelistEntry removes one target-account
	// entry. Unlike this interface's other Delete/Remove methods, a missing
	// row is NOT normalized to success here: the spec's own 404 response for
	// this endpoint is ambiguous between "domain not found" and "row not
	// found" (two different response shapes on the same status,
	// HostHeaderWhitelistRemoveNotFoundResponse vs. MessageResponse), so this
	// adapter surfaces whatever the provider reports rather than guessing
	// which case applies.
	RemoveArvanCloudHostHeaderWhitelistEntry(ctx context.Context, creds domain.ProviderCredentials, domainName, targetAccount string) (*domain.ArvanCloudHostHeaderWhitelist, error)

	// Caching (issue #72): a domain's cache-behavior settings, cache purging,
	// and the deprecated-but-still-implemented purge-tag history endpoints.
	// All fast operations — PurgeArvanCloudCache included: like
	// RegenerateDomainConfig above, the call itself returns immediately even
	// though the actual purge propagates asynchronously afterward on
	// ArvanCloud's own side, with nothing this port exposes to poll for it
	// (confirmed against the spec's caching.purge response, a bare
	// confirmation message with no operation id to poll — see
	// domain.ArvanCloudCachePurgeRequest's doc comment).
	//
	// UpdateArvanCloudCachingSettings's endpoint response carries no data
	// (just a confirmation message), so it returns only an error; a caller
	// wanting the settings' state afterward calls
	// GetArvanCloudCachingSettings. PurgeArvanCloudCache does NOT call the
	// deprecated DELETE /domains/{domain}/caching endpoint
	// (caching.deprecated_purge) — only POST /domains/{domain}/caching/purge
	// (caching.purge), per the spec's own deprecation notice and issue #72's
	// explicit scope note.
	GetArvanCloudCachingSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudCachingSettings, error)
	UpdateArvanCloudCachingSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudCachingSettings) error
	PurgeArvanCloudCache(ctx context.Context, creds domain.ProviderCredentials, domainName string, purge domain.ArvanCloudCachePurgeRequest) error
	// ListArvanCloudPurgeTags returns a domain's previously-purged cache tag
	// history, one domain.ArvanCloudPurgeTag entry per tag (see that type's
	// own doc comment for how it denormalizes the provider's single-object
	// response).
	ListArvanCloudPurgeTags(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudPurgeTag, error)
	// DeleteArvanCloudPurgeTag removes one tag from the purge-tag history by
	// its value.
	DeleteArvanCloudPurgeTag(ctx context.Context, creds domain.ProviderCredentials, domainName, tag string) error

	// Acceleration / Image Resize (issue #72): standalone domain-scoped
	// settings resources — image transformation-on-the-fly and general
	// front-end acceleration, respectively. All fast operations.
	//
	// GetArvanCloudAccelerationSettings/UpdateArvanCloudAccelerationSettings
	// reuse domain.ArvanCloudAccelerationSettings from
	// arvancloud_acceleration.go (AC11/issue #71) rather than a second type —
	// see that type's own doc comment, which documents this exact reuse.
	GetArvanCloudImageResizeSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudImageResizeSettings, error)
	UpdateArvanCloudImageResizeSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudImageResizeSettings) (*domain.ArvanCloudImageResizeSettings, error)
	GetArvanCloudAccelerationSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudAccelerationSettings, error)
	UpdateArvanCloudAccelerationSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudAccelerationSettings) (*domain.ArvanCloudAccelerationSettings, error)

	// Custom Pages (issue #72): a domain's nine named custom-page slots
	// (what is served instead of ArvanCloud's own default content for a WAF
	// block, a rate-limit block, an expired secure link, ...), plus the
	// individual uploaded-file resources those slots can hold. All fast
	// operations.
	//
	// ListArvanCloudCustomPages returns the domain's whole named-slot object
	// (custom-pages.show — despite the operationId saying "show", the
	// response is a list/index by shape; see domain.ArvanCloudCustomPages'
	// own doc comment).
	ListArvanCloudCustomPages(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudCustomPages, error)
	// UpdateArvanCloudCustomPage updates exactly ONE named slot per call
	// (domain.ArvanCloudCustomPageUpdate.Page selects which) despite being
	// POST to the /custom-pages collection endpoint — see that type's own
	// doc comment. The endpoint's response carries no data, so this returns
	// only an error; a caller wanting the slot's state afterward calls
	// ListArvanCloudCustomPages.
	UpdateArvanCloudCustomPage(ctx context.Context, creds domain.ProviderCredentials, domainName string, update domain.ArvanCloudCustomPageUpdate) error
	GetArvanCloudCustomPageFile(ctx context.Context, creds domain.ProviderCredentials, domainName, fileID string) (*domain.ArvanCloudCustomPageFile, error)
	// UpdateArvanCloudCustomPageFile updates one already-uploaded file entry
	// by id: active is nil to leave the file's active flag untouched, or a
	// pointer to set it explicitly; fileName/fileContent, when fileContent is
	// non-empty, replace the file's content (the spec's own description:
	// "Each upload creates a new file entry" applies to custom-pages.update,
	// not this endpoint, which updates the existing entry addressed by
	// fileID in place). The endpoint's response carries no data, so this
	// returns only an error; a caller wanting the file's state afterward
	// calls GetArvanCloudCustomPageFile.
	UpdateArvanCloudCustomPageFile(ctx context.Context, creds domain.ProviderCredentials, domainName, fileID string, active *bool, fileName string, fileContent []byte) error
	// DeleteArvanCloudCustomPageFile removes one file entry by id. Unlike
	// this interface's other Delete/Remove methods, the provider rejects
	// deleting the currently-active file for a slot (a 400, per the spec's
	// own "Cannot delete active file" response) — this adapter does not
	// pre-check that client-side, since which file is active can change
	// between a caller's read and delete; whatever the provider reports is
	// propagated as-is.
	DeleteArvanCloudCustomPageFile(ctx context.Context, creds domain.ProviderCredentials, domainName, fileID string) error

	// SSL/TLS — domain-scoped settings plus uploaded/managed certificates
	// (issue #73): a domain's SSL settings, the certificates attached to it,
	// and the account-side workflow that drives an ArvanCloud-managed
	// certificate to issuance. Deliberately NOT the account-scoped Certum
	// certificate ordering workflow (a distinct purchase/issuance product,
	// tracked separately in issue #74/AC14) — see domain/arvancloud_ssl.go's
	// package comment for the same base-concern split this project already
	// made for Parspack's own SSL surfaces (AGENTS.md 4.5, issue #18).
	//
	// All fast operations except IssueArvanCloudManagedCertificate, which
	// starts a CertificateOrder in a non-terminal status
	// (domain.ArvanCloudCertificateOrderStatus) that ArvanCloud drives toward
	// "valid" or "killed" over time — a long operation (AGENTS.md 4.3), the
	// same shape as Parspack's CreateSSLOrder/ProcessSSLOrder flow.
	GetArvanCloudSslSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudSslSettings, error)
	UpdateArvanCloudSslSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudSslSettings) (*domain.ArvanCloudSslSettings, error)

	ListArvanCloudCertificates(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudCertificate, error)
	// UploadArvanCloudCertificate stores a customer-owned certificate
	// (ssl.cert.store). The endpoint's response carries no data (just a
	// confirmation message), so this returns only an error; a caller wanting
	// the new certificate's provider-assigned ID calls
	// ListArvanCloudCertificates afterward. certificatePEM/privateKeyPEM are
	// sent exactly as given, multipart/form-data per the spec's
	// CertificateStore schema — privateKeyPEM is caller-supplied sensitive
	// material, never logged and never persisted beyond this call (AGENTS.md
	// 4.2's credential-handling principle extended to this field, per issue
	// #73's explicit scope note).
	UploadArvanCloudCertificate(ctx context.Context, creds domain.ProviderCredentials, domainName string, certificatePEM, privateKeyPEM []byte) error
	GetArvanCloudCertificate(ctx context.Context, creds domain.ProviderCredentials, domainName, certificateID string) (*domain.ArvanCloudCertificate, error)
	// DeleteArvanCloudCertificate removes an unused certificate by id. As
	// with DeleteDomain, an already-absent certificate reports
	// domain.ErrNotFound rather than succeeding silently.
	DeleteArvanCloudCertificate(ctx context.Context, creds domain.ProviderCredentials, domainName, certificateID string) error
	// RevokeArvanCloudCertificate revokes a certificate for security reasons
	// (ssl.cert.revoke). The endpoint's response carries no data, so this
	// returns only an error.
	RevokeArvanCloudCertificate(ctx context.Context, creds domain.ProviderCredentials, domainName, certificateID string) error

	// ListArvanCloudSslOrders returns the domain's managed-certificate order
	// history (ssl.cert.order.index), newest activity included — the same
	// data a caller polls to track an order IssueArvanCloudManagedCertificate
	// started, and what crash recovery consults to reconcile an interrupted
	// issuance job (AGENTS.md 4.4) instead of calling
	// IssueArvanCloudManagedCertificate a second time.
	ListArvanCloudSslOrders(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudCertificateOrder, error)
	// IssueArvanCloudManagedCertificate requests issuance of a managed
	// (ArvanCloud-issued, free/automatic) certificate for the domain
	// (ssl.cert.issue) and returns the order it started. This is a LONG
	// operation (AGENTS.md 4.3) — see this interface's own doc comment above
	// and domain.ArvanCloudCertificateOrderStatus. Not assumed idempotent by
	// itself; the use case above it (app.IssueArvanCloudManagedCertificate)
	// is responsible for not calling this a second time for a domain that
	// already has an order in flight, checking ListArvanCloudSslOrders first.
	IssueArvanCloudManagedCertificate(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudCertificateOrder, error)
	// RetryArvanCloudSslOrder retries a previously "killed" order
	// (ssl.cert.order.retry) — the manual-intervention path
	// domain.ArvanCloudCertificateOrderStatusKilled's own doc comment
	// describes. The endpoint's response carries no data, so this returns
	// only an error; a caller wanting the retried order's state afterward
	// calls ListArvanCloudSslOrders.
	RetryArvanCloudSslOrder(ctx context.Context, creds domain.ProviderCredentials, domainName string) error

	// Account-level Certum certificate ordering (issue #74/AC14): a paid,
	// CA-issued certificate product purchased against the account and
	// installed onto one or more domains. Deliberately NOT the domain-scoped
	// SSL/TLS settings and managed/uploaded certificate workflow above (issue
	// #73) — see domain/arvancloud_account_ssl.go's package comment.
	//
	// All fast operations except IssueArvanCloudAccountCertificate, which
	// starts an AccountCertificateOrder in a non-terminal status
	// (domain.ArvanCloudAccountCertificateOrderStatus) — a long operation
	// (AGENTS.md 4.3), the same shape as IssueArvanCloudManagedCertificate
	// above.
	ListArvanCloudCertificateProducts(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudCertificateProduct, error)
	// IssueArvanCloudAccountCertificate purchases and requests issuance of a
	// Certum certificate (account_certificate.issue) and returns the order it
	// started. This is a LONG operation (AGENTS.md 4.3) — see this
	// interface's own doc comment above and
	// domain.ArvanCloudAccountCertificateOrderStatus. Not assumed idempotent
	// by itself; the use case above it
	// (app.IssueArvanCloudAccountCertificate) is responsible for not calling
	// this a second time for a request that already has an order in flight,
	// checking ListArvanCloudAccountCertificateOrders first.
	IssueArvanCloudAccountCertificate(ctx context.Context, creds domain.ProviderCredentials, req domain.ArvanCloudCertificateOrderIssueRequest) (*domain.ArvanCloudAccountCertificateOrder, error)
	// ListArvanCloudAccountCertificateOrders returns the account's full
	// Certum order history (account_certificate.order.index) — the data a
	// caller polls to track an order IssueArvanCloudAccountCertificate
	// started, and what crash recovery consults to reconcile an interrupted
	// issuance job (AGENTS.md 4.4) instead of calling
	// IssueArvanCloudAccountCertificate a second time. Unlike
	// ListArvanCloudSslOrders above, this list is account-wide, not scoped to
	// one domain — there is no per-domain filter on this endpoint.
	ListArvanCloudAccountCertificateOrders(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudAccountCertificateOrder, error)
	// GetArvanCloudAccountCertificateOrder returns one order by its ID
	// (account_certificate.show). keepPrivateKey mirrors the endpoint's own
	// keep_private_key query parameter: false permanently deletes the
	// provider-held private key ArvanCloud/Certum generated for this order
	// after this call, so it can no longer be retrieved or viewed — nil
	// leaves the provider's own default (true) in effect.
	GetArvanCloudAccountCertificateOrder(ctx context.Context, creds domain.ProviderCredentials, orderID string, keepPrivateKey *bool) (*domain.ArvanCloudAccountCertificateOrder, error)
	// RevokeArvanCloudAccountCertificate revokes an order's certificate
	// (account_certificate.revoke) and returns the order as updated.
	RevokeArvanCloudAccountCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.ArvanCloudAccountCertificateOrder, error)
	// ReissueArvanCloudAccountCertificate reissues an order's certificate
	// (account_certificate.reissue) — e.g. after a key compromise or CN
	// change, or as the manual recovery path for an order that reached
	// domain.ArvanCloudAccountCertificateOrderStatusKilled (there is no
	// order-level "retry" endpoint at the account level, unlike issue #73's
	// ssl.cert.order.retry) — and returns the order as updated.
	ReissueArvanCloudAccountCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.ArvanCloudAccountCertificateOrder, error)
	// InstallArvanCloudAccountCertificate installs an issued certificate onto
	// edge servers (account_certificate.install) — the step that actually
	// activates it for serving traffic; issuance alone does not. This is a
	// FAST operation (AGENTS.md 4.3): see
	// domain.ArvanCloudCertificateInstallResult's own doc comment for why.
	InstallArvanCloudAccountCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.ArvanCloudCertificateInstallResult, error)

	// Reports (per-domain) and Aggregated Reports (account-wide) (issue #75):
	// pure GET traffic/security/DNS analytics, confirmed against
	// docs/api-specs/arvancloud-cdn-4.0.yml's "Reports" and "Aggregated
	// Reports" tags. All fast operations (AGENTS.md 4.3) — every one of
	// these API calls returns synchronously, with no operation_id to poll
	// afterward. See domain/arvancloud_reports.go's package comment for the
	// shared response building blocks (ArvanCloudReportChart,
	// ArvanCloudPieSlice, ArvanCloudGeoMapEntry) most of the methods below
	// return pieces of.
	//
	// Every per-domain report's query parameters live on one shared
	// domain.ArvanCloudReportQuery — see that type's own field comments for
	// exactly which endpoints honor which field, since the spec does not
	// apply the same parameter set to every endpoint here.
	GetArvanCloudTrafficReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudTrafficReport, error)
	GetArvanCloudTrafficSavedReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudTrafficSavedReport, error)
	GetArvanCloudTrafficMap(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudTrafficMapReport, error)
	GetArvanCloudVisitorsReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudVisitorsReport, error)
	// ListArvanCloudHighRequestIPs is the one paginated report among the
	// non-attack per-domain endpoints (reports.visitors.high-request-ips).
	ListArvanCloudHighRequestIPs(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) ([]domain.ArvanCloudHighRequestIP, domain.ArvanCloudReportPageMeta, error)
	GetArvanCloudResponseTimeReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudResponseTimeReport, error)
	GetArvanCloudStatusCodeReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudStatusCodeReport, error)
	GetArvanCloudStatusCodeSummary(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudStatusCodeSummary, error)
	ListArvanCloudErrorLogs(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) ([]domain.ArvanCloudErrorLog, error)
	GetArvanCloudErrorLogsChart(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudErrorLogsChart, error)
	// GetArvanCloudErrorLogDetail is deprecated in the spec
	// (reports.error-log-details) but still implemented, the same
	// "implement it anyway" convention this port already applies to other
	// deprecated-but-live endpoints (e.g. ListArvanCloudLBRegions).
	// query.Error is the error message to search for; see
	// domain.ArvanCloudErrorLogDetail's own doc comment for why its result
	// has no confirmed shape beyond raw JSON.
	GetArvanCloudErrorLogDetail(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudErrorLogDetail, error)
	GetArvanCloudDnsRequestsReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudDnsRequestsReport, error)
	GetArvanCloudDnsGeoReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudDnsGeoReport, error)
	GetArvanCloudAttackReport(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudAttackReport, error)
	// ListArvanCloudAttacks is the other paginated report among the
	// per-domain endpoints (reports.attacks.index).
	ListArvanCloudAttacks(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) ([]domain.ArvanCloudAttackReportItem, domain.ArvanCloudReportPageMeta, error)
	ListArvanCloudAttackers(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) ([]domain.ArvanCloudAttacker, error)
	GetArvanCloudAttackMap(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudAttackMapReport, error)
	ListArvanCloudAttackedURIs(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudReportQuery) ([]domain.ArvanCloudAttackedURI, error)
	// GetArvanCloudTransportLayerProxyTraffic reports traffic for one
	// Transport Layer Proxy (reports.transport_layer_proxies.traffics).
	// transportLayerProxyID is a caller-supplied opaque ID: the spec
	// declares no create/list endpoint anywhere for this resource type, so
	// this port does not invent CRUD for it just to backfill an ID lookup —
	// see domain.ArvanCloudTransportLayerProxyTraffic's own doc comment.
	GetArvanCloudTransportLayerProxyTraffic(ctx context.Context, creds domain.ProviderCredentials, domainName, transportLayerProxyID string, query domain.ArvanCloudReportQuery) (*domain.ArvanCloudTransportLayerProxyTraffic, error)
	// DownloadArvanCloudDomainsReport returns a CSV export of the domains
	// report (domains.reports.download) — unlike every other method above,
	// it is NOT scoped to a single domain (no domainName parameter): the
	// spec declares it with no parameters at all, across every domain
	// visible to the credentials. The response body is not JSON (the spec
	// declares its 200 response as text/csv), the same "return the raw
	// body" convention ExportArvanCloudDNSRecords uses for its own
	// text/plain response.
	DownloadArvanCloudDomainsReport(ctx context.Context, creds domain.ProviderCredentials) (string, error)

	// Aggregated Reports, account-wide (no domain path segment at all,
	// unlike every method above). Every query parameter lives on
	// domain.ArvanCloudAggregatedReportQuery — see that type's own field
	// comments for which of the three endpoints below honor which field.
	ListArvanCloudAggregatedReportDetails(ctx context.Context, creds domain.ProviderCredentials, query domain.ArvanCloudAggregatedReportQuery) ([]domain.ArvanCloudAggregatedReportDetail, domain.ArvanCloudReportPageMeta, error)
	// GetArvanCloudAggregatedReportCharts returns the single chart the
	// spec's reports.aggregated.charts nests under data.charts.
	GetArvanCloudAggregatedReportCharts(ctx context.Context, creds domain.ProviderCredentials, query domain.ArvanCloudAggregatedReportQuery) (*domain.ArvanCloudReportChart, error)
	GetArvanCloudAggregatedReportFilters(ctx context.Context, creds domain.ProviderCredentials, query domain.ArvanCloudAggregatedReportQuery) (*domain.ArvanCloudAggregatedReportFilters, error)

	// Log Forwarders and Metric Exporters (issue #76): both push data to an
	// external system (S3-compatible storage, Datadog, Kafka, syslog, ...)
	// rather than exposing it through this project's own Reports tools
	// above. See domain/arvancloud_observability.go's package comment for
	// how the type/data_fields/settings per-variant shape variance and the
	// metric exporter list-vs-CRUD scoping asymmetry are resolved. All fast
	// operations (AGENTS.md 4.3).

	// ListArvanCloudLogForwarders lists domainName's log forwarders
	// (log-forwarders.index), filtered/paginated per query.
	ListArvanCloudLogForwarders(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudLogForwarderListQuery) ([]domain.ArvanCloudLogForwarder, domain.ArvanCloudReportPageMeta, error)
	// CreateArvanCloudLogForwarder creates a new log forwarder
	// (log-forwarders.store).
	CreateArvanCloudLogForwarder(ctx context.Context, creds domain.ProviderCredentials, domainName string, forwarder domain.ArvanCloudLogForwarder) (*domain.ArvanCloudLogForwarder, error)
	// GetArvanCloudLogForwarder returns a single log forwarder by id
	// (log-forwarders.show).
	GetArvanCloudLogForwarder(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudLogForwarder, error)
	// UpdateArvanCloudLogForwarder replaces a log forwarder's fields
	// (log-forwarders.update) and returns it as stored afterward.
	UpdateArvanCloudLogForwarder(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, forwarder domain.ArvanCloudLogForwarder) (*domain.ArvanCloudLogForwarder, error)
	// DeleteArvanCloudLogForwarder removes a log forwarder by id
	// (log-forwarders.destroy).
	DeleteArvanCloudLogForwarder(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error
	// SetArvanCloudLogForwarderStatus enables or disables a log forwarder
	// (log-forwarders.update.status) and returns it as stored afterward.
	SetArvanCloudLogForwarderStatus(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, status bool) (*domain.ArvanCloudLogForwarder, error)

	// ListArvanCloudMetricExporters lists metric exporters across the whole
	// account (metric-exporters.index) — NOT scoped to a single domain,
	// unlike every other method in this group; see
	// domain/arvancloud_observability.go's package comment.
	ListArvanCloudMetricExporters(ctx context.Context, creds domain.ProviderCredentials, query domain.ArvanCloudMetricExporterListQuery) ([]domain.ArvanCloudMetricExporter, domain.ArvanCloudReportPageMeta, error)
	// ListArvanCloudMetricExporterTypes returns the catalog of metric groups
	// and individual metrics available to choose from when creating a
	// metric exporter (metric-exporters.metrics.index) — also
	// account-wide, no domain parameter.
	ListArvanCloudMetricExporterTypes(ctx context.Context, creds domain.ProviderCredentials) (*domain.ArvanCloudMetricExporterMetrics, error)
	// CreateArvanCloudMetricExporter creates a new metric exporter, scoped
	// to domainName (metric-exporters.store).
	CreateArvanCloudMetricExporter(ctx context.Context, creds domain.ProviderCredentials, domainName string, exporter domain.ArvanCloudMetricExporter) (*domain.ArvanCloudMetricExporter, error)
	// GetArvanCloudMetricExporter returns a single metric exporter by id
	// (metric-exporters.show).
	GetArvanCloudMetricExporter(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudMetricExporter, error)
	// UpdateArvanCloudMetricExporter replaces a metric exporter's fields
	// (metric-exporters.update) and returns it as stored afterward.
	UpdateArvanCloudMetricExporter(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, exporter domain.ArvanCloudMetricExporter) (*domain.ArvanCloudMetricExporter, error)
	// DeleteArvanCloudMetricExporter removes a metric exporter by id
	// (metric-exporters.destroy).
	DeleteArvanCloudMetricExporter(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error
	// SetArvanCloudMetricExporterStatus enables or disables a metric
	// exporter (metric-exporters.update.status) and returns it as stored
	// afterward.
	SetArvanCloudMetricExporterStatus(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, status bool) (*domain.ArvanCloudMetricExporter, error)
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
