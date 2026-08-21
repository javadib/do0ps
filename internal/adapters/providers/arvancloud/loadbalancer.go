package arvancloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Load Balancing (issue #69), wired to the real CDN API: CDN edge-level
// traffic distribution across origin pools, a 3-level resource hierarchy
// (load balancer -> pools -> origins) kept in one file rather than split
// three ways, matching the "keep it as one file" precedent of this package's
// own ratelimit.go/ddos.go and Parspack's cdn_loadbalance.go-style adapters —
// LB/pool/origin share request/response plumbing throughout. Base paths are
// confirmed against docs/api-specs/arvancloud-cdn-4.0.yml's "Load Balancing"
// tag, relative to domainPath (defined in domain.go) — i.e.
// https://napi.arvancloud.ir/cdn/4.0/domains/{domain}/load-balancers/... .
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real ArvanCloud responses correctly. Nothing above
// the adapter boundary ever sees them — every method here translates to/from
// internal/core/domain types.

const (
	globalLBRegionsPath  = "load-balancers/regions"
	lbRegionsPathSuffix  = "/load-balancers/regions"
	lbSettingsPathSuffix = "/load-balancers/settings"
	loadBalancersSuffix  = "/load-balancers"
)

func domainLBRegionsPath(domainName string) string {
	return domainPath(domainName) + lbRegionsPathSuffix
}
func lbSettingsPath(domainName string) string {
	return domainPath(domainName) + lbSettingsPathSuffix
}
func loadBalancersPath(domainName string) string {
	return domainPath(domainName) + loadBalancersSuffix
}
func loadBalancerPath(domainName, id string) string {
	return loadBalancersPath(domainName) + "/" + id
}
func lbPrioritizePath(domainName, loadBalancerID string) string {
	return loadBalancerPath(domainName, loadBalancerID) + "/prioritize"
}
func lbPoolsPath(domainName, loadBalancerID string) string {
	return loadBalancerPath(domainName, loadBalancerID) + "/pools"
}
func lbPoolPath(domainName, loadBalancerID, poolID string) string {
	return lbPoolsPath(domainName, loadBalancerID) + "/" + poolID
}
func lbPoolOriginsPath(domainName, loadBalancerID, poolID string) string {
	return lbPoolPath(domainName, loadBalancerID, poolID) + "/origins"
}
func lbPoolOriginPath(domainName, loadBalancerID, poolID, originID string) string {
	return lbPoolOriginsPath(domainName, loadBalancerID, poolID) + "/" + originID
}

// --- Regions ----------------------------------------------------------------

// lbRegionWire mirrors LoadBalancerRegion.
type lbRegionWire struct {
	ID     string `json:"id"`
	Region string `json:"region"`
	Name   string `json:"name"`
}

func toLBRegionDomain(w lbRegionWire) domain.ArvanCloudLoadBalancerRegion {
	return domain.ArvanCloudLoadBalancerRegion{ID: w.ID, Region: w.Region, Name: w.Name}
}

// ListArvanCloudLBRegions returns every region load-balancer pools can be
// scoped to, account-independent. The spec marks this endpoint's operationId
// (load-balancers.regions.index) deprecated, but it is still implemented.
func (p *Provider) ListArvanCloudLBRegions(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudLoadBalancerRegion, error) {
	var items []lbRegionWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, globalLBRegionsPath, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud load balancer regions: %w", err)
	}
	regions := make([]domain.ArvanCloudLoadBalancerRegion, len(items))
	for i := range items {
		regions[i] = toLBRegionDomain(items[i])
	}
	return regions, nil
}

// ListArvanCloudDomainLBRegions returns the regions load-balancer pools on
// domainName can be scoped to.
func (p *Provider) ListArvanCloudDomainLBRegions(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudLoadBalancerRegion, error) {
	var items []lbRegionWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, domainLBRegionsPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud load balancer regions for domain %q: %w", domainName, err)
	}
	regions := make([]domain.ArvanCloudLoadBalancerRegion, len(items))
	for i := range items {
		regions[i] = toLBRegionDomain(items[i])
	}
	return regions, nil
}

// --- Settings -----------------------------------------------------------

// lbSettingWire mirrors LoadBalancerSetting.
type lbSettingWire struct {
	Method               string           `json:"method,omitempty"`
	NextUpstreamTCP      string           `json:"next_upstream_tcp,omitempty"`
	NextUpstreamTCPCodes map[string][]int `json:"next_upstream_tcp_codes,omitempty"`
	Protocol             string           `json:"protocol,omitempty"`
	GRPCStatus           bool             `json:"grpc_status,omitempty"`
	Keepalive            string           `json:"keepalive,omitempty"`
	MaxFails             int              `json:"max_fails,omitempty"`
	FailTimeout          string           `json:"fail_timeout,omitempty"`
}

func toLBSettingsDomain(w lbSettingWire) domain.ArvanCloudLoadBalancerSettings {
	return domain.ArvanCloudLoadBalancerSettings{
		Method:               domain.ArvanCloudPoolMethod(w.Method),
		NextUpstreamTCP:      domain.ArvanCloudLBToggle(w.NextUpstreamTCP),
		NextUpstreamTCPCodes: w.NextUpstreamTCPCodes,
		Protocol:             domain.ArvanCloudOriginProtocol(w.Protocol),
		GRPCStatus:           w.GRPCStatus,
		Keepalive:            domain.ArvanCloudLBToggle(w.Keepalive),
		MaxFails:             w.MaxFails,
		FailTimeout:          w.FailTimeout,
	}
}

// lbSettingsRequestBody builds the JSON body for a settings PATCH as a plain
// map — the same reason ddosSettingsRequestBody/rateLimitSettingsRequestBody
// do: an explicit `false` on grpc_status must reach the provider rather than
// being dropped by encoding/json's omitempty. MaxFails is the one field kept
// out even when explicitly meaningful at zero (see
// domain.ArvanCloudLoadBalancerSettings.MaxFails's doc comment on why).
func lbSettingsRequestBody(s domain.ArvanCloudLoadBalancerSettings) map[string]any {
	body := map[string]any{
		"grpc_status": s.GRPCStatus,
	}
	if s.Method != "" {
		body["method"] = string(s.Method)
	}
	if s.NextUpstreamTCP != "" {
		body["next_upstream_tcp"] = string(s.NextUpstreamTCP)
	}
	if len(s.NextUpstreamTCPCodes) > 0 {
		body["next_upstream_tcp_codes"] = s.NextUpstreamTCPCodes
	}
	if s.Protocol != "" {
		body["protocol"] = string(s.Protocol)
	}
	if s.Keepalive != "" {
		body["keepalive"] = string(s.Keepalive)
	}
	if s.MaxFails > 0 {
		body["max_fails"] = s.MaxFails
	}
	if s.FailTimeout != "" {
		body["fail_timeout"] = s.FailTimeout
	}
	return body
}

// GetArvanCloudLBSettings returns domainName's global load-balancing
// defaults.
func (p *Provider) GetArvanCloudLBSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudLoadBalancerSettings, error) {
	var wire lbSettingWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, lbSettingsPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud load balancer settings for domain %q: %w", domainName, err)
	}
	settings := toLBSettingsDomain(wire)
	return &settings, nil
}

// UpdateArvanCloudLBSettings changes domainName's global load-balancing
// defaults and returns them as stored afterward.
func (p *Provider) UpdateArvanCloudLBSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudLoadBalancerSettings) (*domain.ArvanCloudLoadBalancerSettings, error) {
	body := lbSettingsRequestBody(settings)
	var wire lbSettingWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, lbSettingsPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud load balancer settings for domain %q: %w", domainName, err)
	}
	updated := toLBSettingsDomain(wire)
	return &updated, nil
}

// --- Origins ------------------------------------------------------------

// lbOriginWire mirrors LoadBalancerOrigin, decode-only like
// rateLimitRuleWire: a request body is built separately (originRequestBody)
// as a plain map, so explicit zero values (status: false) reach the provider
// rather than being dropped by encoding/json's omitempty.
type lbOriginWire struct {
	ID                string `json:"id,omitempty"`
	Name              string `json:"name,omitempty"`
	HealthCheckStatus string `json:"health_check_status,omitempty"`
	Status            *bool  `json:"status,omitempty"`
	Address           string `json:"address,omitempty"`
	Port              int    `json:"port,omitempty"`
	Weight            int    `json:"weight,omitempty"`
	Protocol          string `json:"protocol,omitempty"`
	HostHeader        string `json:"host_header,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

func toLBOriginDomain(w lbOriginWire) domain.ArvanCloudLoadBalancerOrigin {
	origin := domain.ArvanCloudLoadBalancerOrigin{
		ID:                w.ID,
		Name:              w.Name,
		HealthCheckStatus: domain.ArvanCloudOriginHealthCheckStatus(w.HealthCheckStatus),
		Address:           w.Address,
		Port:              w.Port,
		Weight:            w.Weight,
		Protocol:          domain.ArvanCloudOriginProtocol(w.Protocol),
		HostHeader:        w.HostHeader,
		CreatedAt:         w.CreatedAt,
		UpdatedAt:         w.UpdatedAt,
	}
	if w.Status != nil {
		origin.Status = *w.Status
	}
	return origin
}

// originRequestBody builds the JSON body for an origin create/update, and the
// per-origin element of a pool's writable "origins" array
// (LoadBalancerOriginStore, used identically by
// load-balancers.pools.origins.store/update and embedded in
// LoadBalancerPoolStoreWithOrigin.origins). status/protocol/address/port/
// weight are always sent — the spec's own required list for this schema —
// so an explicit status:false or a legitimately-zero-adjacent value still
// reaches the provider.
func originRequestBody(o domain.ArvanCloudLoadBalancerOrigin) map[string]any {
	body := map[string]any{
		"status":   o.Status,
		"protocol": string(o.Protocol),
		"address":  o.Address,
		"port":     o.Port,
		"weight":   o.Weight,
	}
	if o.Name != "" {
		body["name"] = o.Name
	}
	if o.HostHeader != "" {
		body["host_header"] = o.HostHeader
	}
	return body
}

// ListArvanCloudLBPoolOrigins returns every origin in a pool.
func (p *Provider) ListArvanCloudLBPoolOrigins(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string) ([]domain.ArvanCloudLoadBalancerOrigin, error) {
	var items []lbOriginWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, lbPoolOriginsPath(domainName, loadBalancerID, poolID), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud load balancer pool origins (domain %q, lb %q, pool %q): %w", domainName, loadBalancerID, poolID, err)
	}
	origins := make([]domain.ArvanCloudLoadBalancerOrigin, len(items))
	for i := range items {
		origins[i] = toLBOriginDomain(items[i])
	}
	return origins, nil
}

// CreateArvanCloudLBPoolOrigin creates a new origin in a pool.
func (p *Provider) CreateArvanCloudLBPoolOrigin(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string, origin domain.ArvanCloudLoadBalancerOrigin) (*domain.ArvanCloudLoadBalancerOrigin, error) {
	body := originRequestBody(origin)
	var wire lbOriginWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, lbPoolOriginsPath(domainName, loadBalancerID, poolID), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud load balancer pool origin (domain %q, lb %q, pool %q): %w", domainName, loadBalancerID, poolID, err)
	}
	created := toLBOriginDomain(wire)
	return &created, nil
}

// GetArvanCloudLBPoolOrigin returns a single origin by id.
func (p *Provider) GetArvanCloudLBPoolOrigin(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID, originID string) (*domain.ArvanCloudLoadBalancerOrigin, error) {
	var wire lbOriginWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, lbPoolOriginPath(domainName, loadBalancerID, poolID, originID), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud load balancer pool origin %q (domain %q, lb %q, pool %q): %w", originID, domainName, loadBalancerID, poolID, err)
	}
	found := toLBOriginDomain(wire)
	return &found, nil
}

// UpdateArvanCloudLBPoolOrigin updates an origin and returns it as stored
// afterward.
func (p *Provider) UpdateArvanCloudLBPoolOrigin(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID, originID string, origin domain.ArvanCloudLoadBalancerOrigin) (*domain.ArvanCloudLoadBalancerOrigin, error) {
	body := originRequestBody(origin)
	var wire lbOriginWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, lbPoolOriginPath(domainName, loadBalancerID, poolID, originID), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud load balancer pool origin %q (domain %q, lb %q, pool %q): %w", originID, domainName, loadBalancerID, poolID, err)
	}
	updated := toLBOriginDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudLBPoolOrigin removes an origin by id.
func (p *Provider) DeleteArvanCloudLBPoolOrigin(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID, originID string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, lbPoolOriginPath(domainName, loadBalancerID, poolID, originID), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud load balancer pool origin %q (domain %q, lb %q, pool %q): %w", originID, domainName, loadBalancerID, poolID, err)
	}
	return nil
}

// --- Pools ----------------------------------------------------------------

// lbRegionCodeWire mirrors LoadBalancerPool.regions' response shape: an
// array of full LoadBalancerRegion objects, decoded here only for its
// "region" code — see domain.ArvanCloudLoadBalancerPool.Regions's doc
// comment for the confirmed request/response asymmetry (the request side
// takes plain 3-letter code strings instead).
type lbRegionCodeWire struct {
	Region string `json:"region"`
}

// lbPoolWire mirrors LoadBalancerPool, decode-only like lbOriginWire: a
// request body is built separately (poolRequestBody) as a plain map.
type lbPoolWire struct {
	ID                   string             `json:"id,omitempty"`
	Name                 string             `json:"name,omitempty"`
	Description          string             `json:"description,omitempty"`
	Status               *bool              `json:"status,omitempty"`
	Priority             int                `json:"priority,omitempty"`
	Method               string             `json:"method,omitempty"`
	Keepalive            string             `json:"keepalive,omitempty"`
	NextUpstreamTCP      string             `json:"next_upstream_tcp,omitempty"`
	NextUpstreamTCPCodes map[string][]int   `json:"next_upstream_tcp_codes,omitempty"`
	Regions              []lbRegionCodeWire `json:"regions,omitempty"`
	Origins              []lbOriginWire     `json:"origins,omitempty"`
	MonitoringStatus     string             `json:"monitoring_status,omitempty"`
	HealthCheck          json.RawMessage    `json:"health_check,omitempty"`
	CreatedAt            string             `json:"created_at,omitempty"`
	UpdatedAt            string             `json:"updated_at,omitempty"`
}

func toLBPoolDomain(w lbPoolWire) domain.ArvanCloudLoadBalancerPool {
	pool := domain.ArvanCloudLoadBalancerPool{
		ID:                   w.ID,
		Name:                 w.Name,
		Description:          w.Description,
		Priority:             w.Priority,
		Method:               domain.ArvanCloudPoolMethod(w.Method),
		Keepalive:            domain.ArvanCloudLBToggle(w.Keepalive),
		NextUpstreamTCP:      domain.ArvanCloudLBToggle(w.NextUpstreamTCP),
		NextUpstreamTCPCodes: w.NextUpstreamTCPCodes,
		MonitoringStatus:     w.MonitoringStatus,
		HealthCheck:          w.HealthCheck,
		CreatedAt:            w.CreatedAt,
		UpdatedAt:            w.UpdatedAt,
	}
	if w.Status != nil {
		pool.Status = *w.Status
	}
	if len(w.Regions) > 0 {
		pool.Regions = make([]string, len(w.Regions))
		for i, r := range w.Regions {
			pool.Regions[i] = r.Region
		}
	}
	if len(w.Origins) > 0 {
		pool.Origins = make([]domain.ArvanCloudLoadBalancerOrigin, len(w.Origins))
		for i := range w.Origins {
			pool.Origins[i] = toLBOriginDomain(w.Origins[i])
		}
	}
	return pool
}

// poolRequestBody builds the JSON body for a pool create/update
// (LoadBalancerPoolStoreWithOrigin when includeOrigins is true —
// load-balancers.pools.store and .update/PUT — or
// LoadBalancerPoolStoreWithoutOrigin when false —
// load-balancers.pools.updatePool/PATCH). name/status/method/keepalive/
// next_upstream_tcp are always sent, matching the with-origin schema's own
// required list; the without-origin (PATCH) schema requires none of them,
// but sending the same full field set there is deliberate: PATCH here means
// "update the pool's settings, not its origins" (see
// ports.ArvanCloudProvider.UpdateArvanCloudLBPoolSettings's doc comment),
// not "send only the fields that changed".
func poolRequestBody(pool domain.ArvanCloudLoadBalancerPool, includeOrigins bool) map[string]any {
	body := map[string]any{
		"name":              pool.Name,
		"status":            pool.Status,
		"method":            string(pool.Method),
		"keepalive":         string(pool.Keepalive),
		"next_upstream_tcp": string(pool.NextUpstreamTCP),
	}
	if pool.Description != "" {
		body["description"] = pool.Description
	}
	if pool.Priority != 0 {
		body["priority"] = pool.Priority
	}
	if len(pool.NextUpstreamTCPCodes) > 0 {
		body["next_upstream_tcp_codes"] = pool.NextUpstreamTCPCodes
	}
	if len(pool.Regions) > 0 {
		body["regions"] = pool.Regions
	}
	if includeOrigins {
		origins := make([]map[string]any, len(pool.Origins))
		for i, o := range pool.Origins {
			origins[i] = originRequestBody(o)
		}
		body["origins"] = origins
	}
	return body
}

// ListArvanCloudLBPools returns every pool of a load balancer.
func (p *Provider) ListArvanCloudLBPools(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID string) ([]domain.ArvanCloudLoadBalancerPool, error) {
	var items []lbPoolWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, lbPoolsPath(domainName, loadBalancerID), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud load balancer pools (domain %q, lb %q): %w", domainName, loadBalancerID, err)
	}
	pools := make([]domain.ArvanCloudLoadBalancerPool, len(items))
	for i := range items {
		pools[i] = toLBPoolDomain(items[i])
	}
	return pools, nil
}

// CreateArvanCloudLBPool creates a new pool, optionally with its initial set
// of origins in the same call (LoadBalancerPoolStoreWithOrigin).
func (p *Provider) CreateArvanCloudLBPool(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID string, pool domain.ArvanCloudLoadBalancerPool) (*domain.ArvanCloudLoadBalancerPool, error) {
	body := poolRequestBody(pool, true)
	var wire lbPoolWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, lbPoolsPath(domainName, loadBalancerID), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud load balancer pool (domain %q, lb %q): %w", domainName, loadBalancerID, err)
	}
	created := toLBPoolDomain(wire)
	return &created, nil
}

// GetArvanCloudLBPool returns a single pool by id.
func (p *Provider) GetArvanCloudLBPool(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string) (*domain.ArvanCloudLoadBalancerPool, error) {
	var wire lbPoolWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, lbPoolPath(domainName, loadBalancerID, poolID), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud load balancer pool %q (domain %q, lb %q): %w", poolID, domainName, loadBalancerID, err)
	}
	found := toLBPoolDomain(wire)
	return &found, nil
}

// UpdateArvanCloudLBPoolWithOrigins (PUT) replaces the pool AND its full set
// of origins in one call. See ports.ArvanCloudProvider's doc comment for why
// this is a distinct method from UpdateArvanCloudLBPoolSettings rather than
// one method with a flag.
func (p *Provider) UpdateArvanCloudLBPoolWithOrigins(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string, pool domain.ArvanCloudLoadBalancerPool) (*domain.ArvanCloudLoadBalancerPool, error) {
	body := poolRequestBody(pool, true)
	var wire lbPoolWire
	if err := p.client.doJSON(ctx, creds, http.MethodPut, lbPoolPath(domainName, loadBalancerID, poolID), body, &wire); err != nil {
		return nil, fmt.Errorf("replacing arvancloud load balancer pool %q with origins (domain %q, lb %q): %w", poolID, domainName, loadBalancerID, err)
	}
	updated := toLBPoolDomain(wire)
	return &updated, nil
}

// UpdateArvanCloudLBPoolSettings (PATCH) updates the pool's own settings
// only, leaving its existing origins untouched. See
// ports.ArvanCloudProvider's doc comment for why this is a distinct method
// from UpdateArvanCloudLBPoolWithOrigins rather than one method with a flag.
func (p *Provider) UpdateArvanCloudLBPoolSettings(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string, pool domain.ArvanCloudLoadBalancerPool) (*domain.ArvanCloudLoadBalancerPool, error) {
	body := poolRequestBody(pool, false)
	var wire lbPoolWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, lbPoolPath(domainName, loadBalancerID, poolID), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud load balancer pool %q settings (domain %q, lb %q): %w", poolID, domainName, loadBalancerID, err)
	}
	updated := toLBPoolDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudLBPool removes a pool by id.
func (p *Provider) DeleteArvanCloudLBPool(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, lbPoolPath(domainName, loadBalancerID, poolID), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud load balancer pool %q (domain %q, lb %q): %w", poolID, domainName, loadBalancerID, err)
	}
	return nil
}

// lbPrioritizePoolRequestWire mirrors PrioritizePoolAfter/PrioritizePoolBefore:
// exactly one of AfterPoolID/BeforePoolID is sent per call — a relative
// "move this pool before/after that pool" request, NOT the reordered-array
// reprioritize shape the rule engines elsewhere in this package use
// (reprioritizeRuleRequestWire in firewall.go). Confirmed against the spec's
// PrioritizePool schema (issue #69's explicit warning not to assume the two
// shapes match).
type lbPrioritizePoolRequestWire struct {
	PoolID       string `json:"pool_id"`
	AfterPoolID  string `json:"after_pool_id,omitempty"`
	BeforePoolID string `json:"before_pool_id,omitempty"`
}

// lbWire mirrors LoadBalancer, decode-only like lbPoolWire.
type lbWire struct {
	ID          string       `json:"id,omitempty"`
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	Status      *bool        `json:"status,omitempty"`
	Method      string       `json:"method,omitempty"`
	TimeSlice   string       `json:"time_slice,omitempty"`
	Pools       []lbPoolWire `json:"pools,omitempty"`
	CreatedAt   string       `json:"created_at,omitempty"`
	UpdatedAt   string       `json:"updated_at,omitempty"`
}

func toLBDomain(w lbWire) domain.ArvanCloudLoadBalancer {
	lb := domain.ArvanCloudLoadBalancer{
		ID:          w.ID,
		Name:        w.Name,
		Description: w.Description,
		Method:      domain.ArvanCloudLoadBalancerMethod(w.Method),
		TimeSlice:   w.TimeSlice,
		CreatedAt:   w.CreatedAt,
		UpdatedAt:   w.UpdatedAt,
	}
	if w.Status != nil {
		lb.Status = *w.Status
	}
	if len(w.Pools) > 0 {
		lb.Pools = make([]domain.ArvanCloudLoadBalancerPool, len(w.Pools))
		for i := range w.Pools {
			lb.Pools[i] = toLBPoolDomain(w.Pools[i])
		}
	}
	return lb
}

// loadBalancerRequestBody builds the JSON body shared by
// CreateArvanCloudLoadBalancer (LoadBalancerStore) and
// UpdateArvanCloudLoadBalancer (LoadBalancer) — both schemas share the same
// writable field set (name/description/status/method/time_slice); everything
// else on the LoadBalancer schema (id/pools/created_at/updated_at) is
// readOnly.
func loadBalancerRequestBody(lb domain.ArvanCloudLoadBalancer) map[string]any {
	body := map[string]any{
		"name":   lb.Name,
		"status": lb.Status,
		"method": string(lb.Method),
	}
	if lb.Description != "" {
		body["description"] = lb.Description
	}
	if lb.TimeSlice != "" {
		body["time_slice"] = lb.TimeSlice
	}
	return body
}

// ListArvanCloudLoadBalancers returns every load balancer on domainName.
func (p *Provider) ListArvanCloudLoadBalancers(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudLoadBalancer, error) {
	var items []lbWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, loadBalancersPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud load balancers for domain %q: %w", domainName, err)
	}
	lbs := make([]domain.ArvanCloudLoadBalancer, len(items))
	for i := range items {
		lbs[i] = toLBDomain(items[i])
	}
	return lbs, nil
}

// CreateArvanCloudLoadBalancer creates a new load balancer. This is a single
// synchronous call: the store endpoint returns the created resource
// directly, so there is no further "provisioning" state for this adapter to
// poll (ports.ArvanCloudProvider, AGENTS.md 4.3) — unlike Parspack's
// cloud-server CreateLoadBalancer.
func (p *Provider) CreateArvanCloudLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, domainName string, lb domain.ArvanCloudLoadBalancer) (*domain.ArvanCloudLoadBalancer, error) {
	body := loadBalancerRequestBody(lb)
	var wire lbWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, loadBalancersPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud load balancer on domain %q: %w", domainName, err)
	}
	created := toLBDomain(wire)
	return &created, nil
}

// GetArvanCloudLoadBalancer returns a single load balancer by id.
func (p *Provider) GetArvanCloudLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudLoadBalancer, error) {
	var wire lbWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, loadBalancerPath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud load balancer %q on domain %q: %w", id, domainName, err)
	}
	found := toLBDomain(wire)
	return &found, nil
}

// UpdateArvanCloudLoadBalancer updates a load balancer and returns it as
// stored afterward.
func (p *Provider) UpdateArvanCloudLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, lb domain.ArvanCloudLoadBalancer) (*domain.ArvanCloudLoadBalancer, error) {
	body := loadBalancerRequestBody(lb)
	var wire lbWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, loadBalancerPath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud load balancer %q on domain %q: %w", id, domainName, err)
	}
	updated := toLBDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudLoadBalancer removes a load balancer by id.
func (p *Provider) DeleteArvanCloudLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, loadBalancerPath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud load balancer %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// ReprioritizeArvanCloudLBPool moves poolID relative to
// afterPoolID/beforePoolID within loadBalancerID's pool set and returns the
// load balancer as stored afterward — load-balancers.prioritize_pool's 200
// response is the LoadBalancer resource itself (LoadBalancerResponse),
// unlike the rule-engine reprioritize endpoints elsewhere in this package,
// which return no data.
func (p *Provider) ReprioritizeArvanCloudLBPool(ctx context.Context, creds domain.ProviderCredentials, domainName, loadBalancerID, poolID, afterPoolID, beforePoolID string) (*domain.ArvanCloudLoadBalancer, error) {
	body := lbPrioritizePoolRequestWire{PoolID: poolID, AfterPoolID: afterPoolID, BeforePoolID: beforePoolID}
	var wire lbWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, lbPrioritizePath(domainName, loadBalancerID), body, &wire); err != nil {
		return nil, fmt.Errorf("reprioritizing arvancloud load balancer pool %q (domain %q, lb %q): %w", poolID, domainName, loadBalancerID, err)
	}
	updated := toLBDomain(wire)
	return &updated, nil
}
