package parspack

import (
	"context"
	"fmt"
	"net/url"

	"github.com/javadib/do0ps/internal/core/domain"
)

// CDN-edge load-balance pools and their backend servers (issue #24), wired
// to the real CDN API. Base path is confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml's "Load Balance" tag
// (lines ~5027-6677), relative to Client.cdnBaseURL and reusing cdn.go's
// zonesBasePath ("external/api/v1/zones"), i.e.
// https://my.parspack.com/cdnapi/external/api/v1/zones/{zone_uuid}/load-balance
// and .../load-balance-server.
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real Parspack responses correctly. Nothing above the
// adapter boundary ever sees these — every method translates to/from
// internal/core/domain types.
const (
	loadBalancePathSuffix       = "/load-balance"
	loadBalanceServerPathSuffix = "/load-balance-server"
)

// loadBalanceServerWire mirrors the server objects nested under a load
// balance's "servers" field (list/get) and the standalone
// load-balance-server list/get responses.
type loadBalanceServerWire struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IP           string `json:"ip"`
	HTTPPort     int    `json:"http_port"`
	HTTPSPort    int    `json:"https_port"`
	Weight       int    `json:"weight"`
	RecoveryTime int    `json:"recovery_time"`
	Group        string `json:"group"`
	Active       bool   `json:"active"`
}

func toDomainLoadBalanceServer(w loadBalanceServerWire) domain.CDNLoadBalanceServer {
	return domain.CDNLoadBalanceServer{
		ID: w.ID, Name: w.Name, IP: w.IP, HTTPPort: w.HTTPPort, HTTPSPort: w.HTTPSPort,
		Weight: w.Weight, RecoveryTime: w.RecoveryTime, Group: w.Group, Active: w.Active,
	}
}

func toDomainLoadBalanceServers(wire []loadBalanceServerWire) []domain.CDNLoadBalanceServer {
	servers := make([]domain.CDNLoadBalanceServer, len(wire))
	for i, w := range wire {
		servers[i] = toDomainLoadBalanceServer(w)
	}
	return servers
}

// loadBalanceWire mirrors GET .../load-balance (list items) and GET
// .../load-balance/{id} (single object) — same shape either way.
type loadBalanceWire struct {
	ID                      string                  `json:"id"`
	Name                    string                  `json:"name"`
	Enabled                 bool                    `json:"enabled"`
	RetryCount              int                     `json:"retry_count"`
	EnableCookiePersist     bool                    `json:"enable_cookie_persist"`
	ServerFailCountToBeDown int                     `json:"server_fail_count_to_be_down"`
	Method                  string                  `json:"method"`
	DownServersRecoveryTime int                     `json:"down_servers_recovery_time"`
	CookiePersistExpireTime int                     `json:"cookie_persist_expire_time"`
	Servers                 []loadBalanceServerWire `json:"servers"`
}

func toDomainLoadBalance(w loadBalanceWire) domain.CDNLoadBalance {
	return domain.CDNLoadBalance{
		ID: w.ID, Name: w.Name, Enabled: w.Enabled, RetryCount: w.RetryCount,
		EnableCookiePersist: w.EnableCookiePersist, ServerFailCountToBeDown: w.ServerFailCountToBeDown,
		Method: w.Method, DownServersRecoveryTime: w.DownServersRecoveryTime,
		CookiePersistExpireTime: w.CookiePersistExpireTime, Servers: toDomainLoadBalanceServers(w.Servers),
	}
}

// ListCDNLoadBalances returns every load-balance pool configured in a zone.
func (c *Client) ListCDNLoadBalances(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNLoadBalance, error) {
	var items []loadBalanceWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+loadBalancePathSuffix, nil, &items); err != nil {
		return nil, fmt.Errorf("list CDN load balances of zone %s: %w", zoneUUID, err)
	}

	balances := make([]domain.CDNLoadBalance, len(items))
	for i := range items {
		balances[i] = toDomainLoadBalance(items[i])
	}
	return balances, nil
}

// loadBalanceServerCreateWire is the shape of one server object nested
// inside POST .../load-balance's "servers" field. The spec documents that
// field's type as "object" with an empty-array example; the item properties
// it lists (name, ip, http_port, https_port, weight, recovery_time, group,
// active) exactly match the GET/show server shape minus "id" (assigned by
// the provider), so it is treated as an array of these here.
type loadBalanceServerCreateWire struct {
	Name         string `json:"name,omitempty"`
	IP           string `json:"ip"`
	HTTPPort     int    `json:"http_port,omitempty"`
	HTTPSPort    int    `json:"https_port,omitempty"`
	Weight       int    `json:"weight,omitempty"`
	RecoveryTime int    `json:"recovery_time,omitempty"`
	Group        string `json:"group,omitempty"`
	Active       bool   `json:"active"`
}

func toLoadBalanceServerCreateWire(s domain.CDNLoadBalanceServer) loadBalanceServerCreateWire {
	return loadBalanceServerCreateWire{
		Name: s.Name, IP: s.IP, HTTPPort: s.HTTPPort, HTTPSPort: s.HTTPSPort,
		Weight: s.Weight, RecoveryTime: s.RecoveryTime, Group: s.Group, Active: s.Active,
	}
}

// loadBalanceCreateRequest is the body of POST .../load-balance.
type loadBalanceCreateRequest struct {
	Name                    string                        `json:"name"`
	Enabled                 bool                          `json:"enabled"`
	RetryCount              int                           `json:"retry_count,omitempty"`
	EnableCookiePersist     bool                          `json:"enable_cookie_persist,omitempty"`
	ServerFailCountToBeDown int                           `json:"server_fail_count_to_be_down,omitempty"`
	Method                  string                        `json:"method,omitempty"`
	DownServersRecoveryTime int                           `json:"down_servers_recovery_time,omitempty"`
	CookiePersistExpireTime int                           `json:"cookie_persist_expire_time,omitempty"`
	Servers                 []loadBalanceServerCreateWire `json:"servers"`
}

// CreateCDNLoadBalance creates a new load-balance pool in a zone. The
// provider's store endpoint returns an empty "data" array on success (no
// generated ID or echoed fields, confirmed against the spec's 201 example)
// — unlike CreateCDNZone, there is nothing to decode from the response, so
// the returned pool is an echo of what was sent, with no ID populated. Call
// ListCDNLoadBalances/GetCDNLoadBalance by name afterward to learn the
// provider-assigned ID.
func (c *Client) CreateCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, lb domain.CDNLoadBalance) (*domain.CDNLoadBalance, error) {
	servers := make([]loadBalanceServerCreateWire, len(lb.Servers))
	for i, s := range lb.Servers {
		servers[i] = toLoadBalanceServerCreateWire(s)
	}

	reqBody := loadBalanceCreateRequest{
		Name: lb.Name, Enabled: lb.Enabled, RetryCount: lb.RetryCount,
		EnableCookiePersist: lb.EnableCookiePersist, ServerFailCountToBeDown: lb.ServerFailCountToBeDown,
		Method: lb.Method, DownServersRecoveryTime: lb.DownServersRecoveryTime,
		CookiePersistExpireTime: lb.CookiePersistExpireTime, Servers: servers,
	}

	if err := c.doCDNJSON(ctx, creds, "POST", zonesBasePath+"/"+zoneUUID+loadBalancePathSuffix, reqBody, nil); err != nil {
		return nil, fmt.Errorf("creating CDN load balance %q in zone %s: %w", lb.Name, zoneUUID, err)
	}

	created := lb
	return &created, nil
}

// GetCDNLoadBalance returns a single load-balance pool by ID.
func (c *Client) GetCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNLoadBalance, error) {
	var w loadBalanceWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+loadBalancePathSuffix+"/"+id, nil, &w); err != nil {
		return nil, fmt.Errorf("get CDN load balance %s in zone %s: %w", id, zoneUUID, err)
	}
	lb := toDomainLoadBalance(w)
	return &lb, nil
}

// loadBalanceUpdateRequest is the body of PUT .../load-balance/{id}. Unlike
// create, the spec's update body has no "servers" field — a pool's backend
// servers are managed individually through the load-balance-server
// endpoints, not by replacing the whole list here.
type loadBalanceUpdateRequest struct {
	Name                    string `json:"name"`
	Enabled                 bool   `json:"enabled"`
	RetryCount              int    `json:"retry_count,omitempty"`
	EnableCookiePersist     bool   `json:"enable_cookie_persist,omitempty"`
	ServerFailCountToBeDown int    `json:"server_fail_count_to_be_down,omitempty"`
	Method                  string `json:"method,omitempty"`
	DownServersRecoveryTime int    `json:"down_servers_recovery_time,omitempty"`
	CookiePersistExpireTime int    `json:"cookie_persist_expire_time,omitempty"`
}

// UpdateCDNLoadBalance updates a load-balance pool's configuration by ID.
// As with create, the provider's update endpoint returns an empty "data"
// array on success, so the returned pool is an echo of what was sent.
func (c *Client) UpdateCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string, lb domain.CDNLoadBalance) (*domain.CDNLoadBalance, error) {
	reqBody := loadBalanceUpdateRequest{
		Name: lb.Name, Enabled: lb.Enabled, RetryCount: lb.RetryCount,
		EnableCookiePersist: lb.EnableCookiePersist, ServerFailCountToBeDown: lb.ServerFailCountToBeDown,
		Method: lb.Method, DownServersRecoveryTime: lb.DownServersRecoveryTime,
		CookiePersistExpireTime: lb.CookiePersistExpireTime,
	}

	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+loadBalancePathSuffix+"/"+id, reqBody, nil); err != nil {
		return nil, fmt.Errorf("updating CDN load balance %s in zone %s: %w", id, zoneUUID, err)
	}

	updated := lb
	updated.ID = id
	return &updated, nil
}

// DeleteCDNLoadBalance removes a load-balance pool by ID.
func (c *Client) DeleteCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", zonesBasePath+"/"+zoneUUID+loadBalancePathSuffix+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("deleting CDN load balance %s in zone %s: %w", id, zoneUUID, err)
	}
	return nil
}

// ListCDNLoadBalanceServers returns every backend server of one load-balance
// pool. loadBalanceID is required by the provider's index endpoint (a query
// parameter, confirmed against the spec) — it is the only place this whole
// resource is filtered by pool, since the server objects themselves carry no
// load-balance foreign key (domain.CDNLoadBalanceServer's doc comment).
func (c *Client) ListCDNLoadBalanceServers(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, loadBalanceID string) ([]domain.CDNLoadBalanceServer, error) {
	path := zonesBasePath + "/" + zoneUUID + loadBalanceServerPathSuffix + "?load_balance_id=" + url.QueryEscape(loadBalanceID)
	var items []loadBalanceServerWire
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &items); err != nil {
		return nil, fmt.Errorf("list CDN load balance servers of pool %s in zone %s: %w", loadBalanceID, zoneUUID, err)
	}
	return toDomainLoadBalanceServers(items), nil
}

// CreateCDNLoadBalanceServer creates a new backend server in a zone. As with
// CreateCDNLoadBalance, the provider's store endpoint returns an empty
// "data" array on success, so the returned server is an echo of what was
// sent, with no ID populated. Confirmed against the spec, the create request
// body has no "name" field (unlike update, which does) and no load-balance
// foreign key — see domain.CDNLoadBalanceServer's doc comment.
func (c *Client) CreateCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, srv domain.CDNLoadBalanceServer) (*domain.CDNLoadBalanceServer, error) {
	reqBody := struct {
		IP           string `json:"ip"`
		HTTPPort     int    `json:"http_port,omitempty"`
		HTTPSPort    int    `json:"https_port,omitempty"`
		Weight       int    `json:"weight,omitempty"`
		RecoveryTime int    `json:"recovery_time,omitempty"`
		Group        string `json:"group,omitempty"`
		Active       bool   `json:"active"`
	}{
		IP: srv.IP, HTTPPort: srv.HTTPPort, HTTPSPort: srv.HTTPSPort,
		Weight: srv.Weight, RecoveryTime: srv.RecoveryTime, Group: srv.Group, Active: srv.Active,
	}

	if err := c.doCDNJSON(ctx, creds, "POST", zonesBasePath+"/"+zoneUUID+loadBalanceServerPathSuffix, reqBody, nil); err != nil {
		return nil, fmt.Errorf("creating CDN load balance server in zone %s: %w", zoneUUID, err)
	}

	created := srv
	return &created, nil
}

// GetCDNLoadBalanceServer returns a single backend server by ID.
func (c *Client) GetCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNLoadBalanceServer, error) {
	var w loadBalanceServerWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+loadBalanceServerPathSuffix+"/"+id, nil, &w); err != nil {
		return nil, fmt.Errorf("get CDN load balance server %s in zone %s: %w", id, zoneUUID, err)
	}
	srv := toDomainLoadBalanceServer(w)
	return &srv, nil
}

// UpdateCDNLoadBalanceServer updates a backend server's configuration by ID.
// Unlike create, the spec's update body does accept "name". As with
// UpdateCDNLoadBalance, the provider's update endpoint returns an empty
// "data" array on success, so the returned server is an echo of what was
// sent.
func (c *Client) UpdateCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string, srv domain.CDNLoadBalanceServer) (*domain.CDNLoadBalanceServer, error) {
	reqBody := struct {
		Name         string `json:"name"`
		IP           string `json:"ip"`
		HTTPPort     int    `json:"http_port,omitempty"`
		HTTPSPort    int    `json:"https_port,omitempty"`
		Weight       int    `json:"weight,omitempty"`
		RecoveryTime int    `json:"recovery_time,omitempty"`
		Group        string `json:"group"`
		Active       bool   `json:"active"`
	}{
		Name: srv.Name, IP: srv.IP, HTTPPort: srv.HTTPPort, HTTPSPort: srv.HTTPSPort,
		Weight: srv.Weight, RecoveryTime: srv.RecoveryTime, Group: srv.Group, Active: srv.Active,
	}

	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+loadBalanceServerPathSuffix+"/"+id, reqBody, nil); err != nil {
		return nil, fmt.Errorf("updating CDN load balance server %s in zone %s: %w", id, zoneUUID, err)
	}

	updated := srv
	updated.ID = id
	return &updated, nil
}

// DeleteCDNLoadBalanceServer removes a backend server by ID.
func (c *Client) DeleteCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", zonesBasePath+"/"+zoneUUID+loadBalanceServerPathSuffix+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("deleting CDN load balance server %s in zone %s: %w", id, zoneUUID, err)
	}
	return nil
}
