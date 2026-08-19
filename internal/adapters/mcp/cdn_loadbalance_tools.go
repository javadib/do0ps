package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// loadBalanceIDProperty is shared by every tool that operates on an existing
// CDN load-balance pool.
func cdnLoadBalanceIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The load balance pool's provider ID, as returned by list_cdn_load_balances or get_cdn_load_balance.",
	}
}

// cdnLoadBalanceServerConfigProperties is the JSON Schema for one backend
// server, shared by create_cdn_load_balance's nested "servers" array and
// create_cdn_load_balance_server's/update_cdn_load_balance_server's
// top-level fields.
func cdnLoadBalanceServerConfigProperties() map[string]any {
	return map[string]any{
		"name": map[string]any{
			"type": "string",
			"description": "Human-readable label for the server, e.g. \"origin-1\". Only used when nested inside " +
				"create_cdn_load_balance's servers array or by update_cdn_load_balance_server — the provider's " +
				"standalone create_cdn_load_balance_server call does not accept a name.",
		},
		"ip": map[string]any{
			"type":        "string",
			"description": "IP address of the backend (origin) server, e.g. \"203.0.113.10\".",
		},
		"http_port": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     65535,
			"description": "HTTP port on the backend server, e.g. 80.",
		},
		"https_port": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     65535,
			"description": "HTTPS port on the backend server, e.g. 443.",
		},
		"weight": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"description": "Relative traffic weight among servers in the same group, e.g. 10.",
		},
		"recovery_time": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"description": "Seconds before a server marked down is retried, e.g. 120.",
		},
		"group": map[string]any{
			"type":        "string",
			"enum":        []string{"primary", "backup"},
			"description": "Whether the server is in the primary or backup group.",
		},
		"active": map[string]any{
			"type":        "boolean",
			"description": "Whether the server is enabled to receive traffic.",
		},
	}
}

type cdnLoadBalanceServerConfigArgs struct {
	Name         string `json:"name"`
	IP           string `json:"ip"`
	HTTPPort     int    `json:"http_port"`
	HTTPSPort    int    `json:"https_port"`
	Weight       int    `json:"weight"`
	RecoveryTime int    `json:"recovery_time"`
	Group        string `json:"group"`
	Active       bool   `json:"active"`
}

func (a cdnLoadBalanceServerConfigArgs) server() domain.CDNLoadBalanceServer {
	return domain.CDNLoadBalanceServer{
		Name: a.Name, IP: a.IP, HTTPPort: a.HTTPPort, HTTPSPort: a.HTTPSPort,
		Weight: a.Weight, RecoveryTime: a.RecoveryTime, Group: a.Group, Active: a.Active,
	}
}

// cdnLoadBalanceServerToMap renders a domain.CDNLoadBalanceServer the way
// every server-returning tool reports it back to the caller.
func cdnLoadBalanceServerToMap(srv domain.CDNLoadBalanceServer) map[string]any {
	return map[string]any{
		"id":            srv.ID,
		"name":          srv.Name,
		"ip":            srv.IP,
		"http_port":     srv.HTTPPort,
		"https_port":    srv.HTTPSPort,
		"weight":        srv.Weight,
		"recovery_time": srv.RecoveryTime,
		"group":         srv.Group,
		"active":        srv.Active,
	}
}

// cdnLoadBalanceToMap renders a domain.CDNLoadBalance the way every
// pool-returning tool reports it back to the caller.
func cdnLoadBalanceToMap(lb domain.CDNLoadBalance) map[string]any {
	servers := make([]map[string]any, len(lb.Servers))
	for i, s := range lb.Servers {
		servers[i] = cdnLoadBalanceServerToMap(s)
	}
	return map[string]any{
		"id":                           lb.ID,
		"name":                         lb.Name,
		"enabled":                      lb.Enabled,
		"retry_count":                  lb.RetryCount,
		"enable_cookie_persist":        lb.EnableCookiePersist,
		"server_fail_count_to_be_down": lb.ServerFailCountToBeDown,
		"method":                       lb.Method,
		"down_servers_recovery_time":   lb.DownServersRecoveryTime,
		"cookie_persist_expire_time":   lb.CookiePersistExpireTime,
		"servers":                      servers,
	}
}

// cdnLoadBalanceConfigProperties is the JSON Schema for the mutable
// configuration shared by create_cdn_load_balance and
// update_cdn_load_balance.
func cdnLoadBalanceConfigProperties() map[string]any {
	return map[string]any{
		"name": map[string]any{
			"type":        "string",
			"description": "Human-readable pool name, e.g. \"lb2\". Must be unique within the zone.",
		},
		"enabled": map[string]any{
			"type":        "boolean",
			"description": "Whether the pool is active.",
		},
		"retry_count": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"description": "Number of times to retry a failed request against another backend server, e.g. 3.",
		},
		"enable_cookie_persist": map[string]any{
			"type":        "boolean",
			"description": "Whether to pin a client to the same backend server via a cookie.",
		},
		"server_fail_count_to_be_down": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"description": "Number of failed requests before a backend server is marked down, e.g. 5.",
		},
		"method": map[string]any{
			"type":        "string",
			"enum":        []string{"round_robin", "c-hash"},
			"description": "Balancing method across backend servers.",
		},
		"down_servers_recovery_time": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"description": "Seconds before a down backend server is retried, e.g. 60.",
		},
		"cookie_persist_expire_time": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"description": "Seconds before a cookie-persist session expires, e.g. 3600. Only meaningful when enable_cookie_persist is true.",
		},
	}
}

type createCDNLoadBalanceArgs struct {
	credentialArgs
	ZoneUUID                string                           `json:"zone_uuid"`
	Name                    string                           `json:"name"`
	Enabled                 bool                             `json:"enabled"`
	RetryCount              int                              `json:"retry_count"`
	EnableCookiePersist     bool                             `json:"enable_cookie_persist"`
	ServerFailCountToBeDown int                              `json:"server_fail_count_to_be_down"`
	Method                  string                           `json:"method"`
	DownServersRecoveryTime int                              `json:"down_servers_recovery_time"`
	CookiePersistExpireTime int                              `json:"cookie_persist_expire_time"`
	Servers                 []cdnLoadBalanceServerConfigArgs `json:"servers"`
}

func (a createCDNLoadBalanceArgs) loadBalance() domain.CDNLoadBalance {
	lb := domain.CDNLoadBalance{
		Name: a.Name, Enabled: a.Enabled, RetryCount: a.RetryCount,
		EnableCookiePersist: a.EnableCookiePersist, ServerFailCountToBeDown: a.ServerFailCountToBeDown,
		Method: a.Method, DownServersRecoveryTime: a.DownServersRecoveryTime,
		CookiePersistExpireTime: a.CookiePersistExpireTime,
	}
	for _, s := range a.Servers {
		lb.Servers = append(lb.Servers, s.server())
	}
	return lb
}

func createCDNLoadBalanceTool(uc *app.CreateCDNLoadBalance) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	for k, v := range cdnLoadBalanceConfigProperties() {
		props[k] = v
	}
	props["servers"] = map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":       "object",
			"properties": cdnLoadBalanceServerConfigProperties(),
			"required":   []string{"ip", "active"},
		},
		"description": "Backend servers to seed the pool with. Extra servers can be added later with create_cdn_load_balance_server.",
	}

	return Tool{
		Name: "create_cdn_load_balance",
		Description: "Create a CDN-edge load-balance pool in a Parspack CDN zone (distinct from the cloud-server-level " +
			"load balancer created by create_load_balancer). This is a fast operation: the provider's create endpoint " +
			"returns synchronously with no further status to poll. Note: the provider's response does not include the " +
			"pool's generated ID — call list_cdn_load_balances or get_cdn_load_balance afterward (matching by name) to " +
			"learn it, e.g. before pointing a DNS record at it via create_dns_record's load_balance_id.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "name", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createCDNLoadBalanceArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			lb, err := uc.Execute(ctx, app.CreateCDNLoadBalanceInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, LoadBalance: args.loadBalance(),
			})
			if err != nil {
				return nil, err
			}
			return cdnLoadBalanceToMap(*lb), nil
		},
	}
}

func listCDNLoadBalancesTool(uc *app.ListCDNLoadBalances) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_cdn_load_balances",
		Description: "List every CDN-edge load-balance pool configured in a Parspack CDN zone. This is a fast " +
			"operation: the list is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args zoneUUIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			balances, err := uc.Execute(ctx, app.ListCDNLoadBalancesInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(balances))
			for i, lb := range balances {
				out[i] = cdnLoadBalanceToMap(lb)
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "load_balances": out}, nil
		},
	}
}

type cdnLoadBalanceIDArgs struct {
	credentialArgs
	ZoneUUID      string `json:"zone_uuid"`
	LoadBalanceID string `json:"load_balance_id"`
}

func getCDNLoadBalanceTool(uc *app.GetCDNLoadBalance) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["load_balance_id"] = cdnLoadBalanceIDProperty()

	return Tool{
		Name: "get_cdn_load_balance",
		Description: "Get the current state of one CDN-edge load-balance pool by its provider ID. This is a fast " +
			"operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "load_balance_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cdnLoadBalanceIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			lb, err := uc.Execute(ctx, app.GetCDNLoadBalanceInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, LoadBalanceID: args.LoadBalanceID,
			})
			if err != nil {
				return nil, err
			}
			return cdnLoadBalanceToMap(*lb), nil
		},
	}
}

type updateCDNLoadBalanceArgs struct {
	credentialArgs
	ZoneUUID                string `json:"zone_uuid"`
	LoadBalanceID           string `json:"load_balance_id"`
	Name                    string `json:"name"`
	Enabled                 bool   `json:"enabled"`
	RetryCount              int    `json:"retry_count"`
	EnableCookiePersist     bool   `json:"enable_cookie_persist"`
	ServerFailCountToBeDown int    `json:"server_fail_count_to_be_down"`
	Method                  string `json:"method"`
	DownServersRecoveryTime int    `json:"down_servers_recovery_time"`
	CookiePersistExpireTime int    `json:"cookie_persist_expire_time"`
}

func (a updateCDNLoadBalanceArgs) loadBalance() domain.CDNLoadBalance {
	return domain.CDNLoadBalance{
		Name: a.Name, Enabled: a.Enabled, RetryCount: a.RetryCount,
		EnableCookiePersist: a.EnableCookiePersist, ServerFailCountToBeDown: a.ServerFailCountToBeDown,
		Method: a.Method, DownServersRecoveryTime: a.DownServersRecoveryTime,
		CookiePersistExpireTime: a.CookiePersistExpireTime,
	}
}

func updateCDNLoadBalanceTool(uc *app.UpdateCDNLoadBalance) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["load_balance_id"] = cdnLoadBalanceIDProperty()
	for k, v := range cdnLoadBalanceConfigProperties() {
		props[k] = v
	}

	return Tool{
		Name: "update_cdn_load_balance",
		Description: "Replace the configuration of an existing CDN-edge load-balance pool by its provider ID. This " +
			"is a fast operation: the provider's update endpoint returns synchronously with no further status to poll. " +
			"This cannot change the pool's backend servers — manage those individually with " +
			"create_cdn_load_balance_server/update_cdn_load_balance_server/delete_cdn_load_balance_server.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "load_balance_id", "name", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNLoadBalanceArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			lb, err := uc.Execute(ctx, app.UpdateCDNLoadBalanceInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, LoadBalanceID: args.LoadBalanceID,
				LoadBalance: args.loadBalance(),
			})
			if err != nil {
				return nil, err
			}
			return cdnLoadBalanceToMap(*lb), nil
		},
	}
}

func deleteCDNLoadBalanceTool(uc *app.DeleteCDNLoadBalance) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["load_balance_id"] = cdnLoadBalanceIDProperty()

	return Tool{
		Name: "delete_cdn_load_balance",
		Description: "Permanently delete a CDN-edge load-balance pool by its provider ID. This is a fast operation " +
			"and cannot be undone. Deleting a pool that no longer exists is treated as already done rather than an " +
			"error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "load_balance_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cdnLoadBalanceIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteCDNLoadBalanceInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, LoadBalanceID: args.LoadBalanceID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID, "load_balance_id": args.LoadBalanceID}, nil
		},
	}
}

type listCDNLoadBalanceServersArgs struct {
	credentialArgs
	ZoneUUID      string `json:"zone_uuid"`
	LoadBalanceID string `json:"load_balance_id"`
}

func listCDNLoadBalanceServersTool(uc *app.ListCDNLoadBalanceServers) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["load_balance_id"] = cdnLoadBalanceIDProperty()

	return Tool{
		Name: "list_cdn_load_balance_servers",
		Description: "List every backend server of one CDN-edge load-balance pool. This is a fast operation: the " +
			"list is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "load_balance_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args listCDNLoadBalanceServersArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			servers, err := uc.Execute(ctx, app.ListCDNLoadBalanceServersInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, LoadBalanceID: args.LoadBalanceID,
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(servers))
			for i, srv := range servers {
				out[i] = cdnLoadBalanceServerToMap(srv)
			}
			return map[string]any{
				"zone_uuid": args.ZoneUUID, "load_balance_id": args.LoadBalanceID, "servers": out,
			}, nil
		},
	}
}

type createCDNLoadBalanceServerArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	cdnLoadBalanceServerConfigArgs
}

func createCDNLoadBalanceServerTool(uc *app.CreateCDNLoadBalanceServer) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	for k, v := range cdnLoadBalanceServerConfigProperties() {
		props[k] = v
	}

	return Tool{
		Name: "create_cdn_load_balance_server",
		Description: "Create a backend server for a Parspack CDN zone. This is a fast operation: the provider's " +
			"create endpoint returns synchronously with no further status to poll. Important: the provider's create " +
			"call is zone-scoped only, not pool-scoped — it does not accept a load_balance_id and its response does " +
			"not include the server's generated ID. To attach it to a specific pool, seed it directly in that pool's " +
			"servers array via create_cdn_load_balance, or call list_cdn_load_balance_servers afterward (filtered by " +
			"load_balance_id) to find it by IP once assigned.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "ip", "active"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createCDNLoadBalanceServerArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			srv, err := uc.Execute(ctx, app.CreateCDNLoadBalanceServerInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Server: args.server(),
			})
			if err != nil {
				return nil, err
			}
			return cdnLoadBalanceServerToMap(*srv), nil
		},
	}
}

type cdnLoadBalanceServerIDArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	ServerID string `json:"server_id"`
}

func getCDNLoadBalanceServerTool(uc *app.GetCDNLoadBalanceServer) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["server_id"] = map[string]any{
		"type":        "string",
		"description": "The backend server's provider ID, as returned by list_cdn_load_balance_servers.",
	}

	return Tool{
		Name: "get_cdn_load_balance_server",
		Description: "Get the current state of one CDN-edge load-balance backend server by its provider ID. This is " +
			"a fast operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "server_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cdnLoadBalanceServerIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			srv, err := uc.Execute(ctx, app.GetCDNLoadBalanceServerInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, ServerID: args.ServerID,
			})
			if err != nil {
				return nil, err
			}
			return cdnLoadBalanceServerToMap(*srv), nil
		},
	}
}

type updateCDNLoadBalanceServerArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	ServerID string `json:"server_id"`
	cdnLoadBalanceServerConfigArgs
}

func updateCDNLoadBalanceServerTool(uc *app.UpdateCDNLoadBalanceServer) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["server_id"] = map[string]any{
		"type":        "string",
		"description": "The backend server's provider ID, as returned by list_cdn_load_balance_servers.",
	}
	for k, v := range cdnLoadBalanceServerConfigProperties() {
		props[k] = v
	}

	return Tool{
		Name: "update_cdn_load_balance_server",
		Description: "Replace the configuration of an existing CDN-edge load-balance backend server by its provider " +
			"ID. This is a fast operation: the provider's update endpoint returns synchronously with no further " +
			"status to poll. Unlike create_cdn_load_balance_server, this accepts a name.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "server_id", "name", "ip", "group", "active"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNLoadBalanceServerArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			srv, err := uc.Execute(ctx, app.UpdateCDNLoadBalanceServerInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, ServerID: args.ServerID,
				Server: args.server(),
			})
			if err != nil {
				return nil, err
			}
			return cdnLoadBalanceServerToMap(*srv), nil
		},
	}
}

func deleteCDNLoadBalanceServerTool(uc *app.DeleteCDNLoadBalanceServer) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["server_id"] = map[string]any{
		"type":        "string",
		"description": "The backend server's provider ID, as returned by list_cdn_load_balance_servers.",
	}

	return Tool{
		Name: "delete_cdn_load_balance_server",
		Description: "Permanently delete a CDN-edge load-balance backend server by its provider ID. This is a fast " +
			"operation and cannot be undone. Deleting a server that no longer exists is treated as already done " +
			"rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "server_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cdnLoadBalanceServerIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteCDNLoadBalanceServerInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, ServerID: args.ServerID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID, "server_id": args.ServerID}, nil
		},
	}
}
