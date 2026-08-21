package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud Load Balancing tools (issue #69): CDN edge-level traffic
// distribution across origin pools — a 3-level resource hierarchy (load
// balancer -> pools -> origins). All fast operations (AGENTS.md 4.3): every
// tool below returns its result within the call, with no operation_id to
// poll afterward — creating a load balancer configures routing rules, it
// does not provision new infrastructure, unlike a cloud-server load balancer
// (create_load_balancer elsewhere in this package, a long operation).
const arvanCloudLBNamingNote = "This is ArvanCloud's CDN edge-level load balancer (traffic distribution across " +
	"origin pools at the edge) — unrelated to create_load_balancer/get_load_balancer and friends elsewhere in " +
	"this tool set, which manage a cloud-server/VM-network-level load balancer on a different resource entirely."

// --- Shared property/arg helpers -------------------------------------------

func arvanCloudLoadBalancerIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The load balancer's provider-assigned ID (a UUID), as returned by create_arvancloud_load_balancer or list_arvancloud_load_balancers.",
	}
}

func arvanCloudLBPoolIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The pool's provider-assigned ID (a UUID), as returned by create_arvancloud_lb_pool or list_arvancloud_lb_pools.",
	}
}

func arvanCloudLBOriginIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The origin's provider-assigned ID (a UUID), as returned by create_arvancloud_lb_pool_origin or list_arvancloud_lb_pool_origins.",
	}
}

// arvanCloudLBIDArgs is embedded by every tool below scoped to exactly one
// load balancer by domain + id.
type arvanCloudLBIDArgs struct {
	arvanCloudDomainNameArgs
	ID string `json:"id"`
}

// arvanCloudLBPoolListArgs is embedded by every tool below that lists or
// creates a pool, scoped to exactly one load balancer by domain + id.
type arvanCloudLBPoolListArgs struct {
	arvanCloudDomainNameArgs
	LoadBalancerID string `json:"load_balancer_id"`
}

func arvanCloudLoadBalancerIDArgProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The load balancer's ID (a UUID) whose pools this call operates on, as returned by create_arvancloud_load_balancer or list_arvancloud_load_balancers.",
	}
}

// arvanCloudLBPoolIDArgs is embedded by every tool below scoped to exactly
// one pool by domain + load balancer + pool id.
type arvanCloudLBPoolIDArgs struct {
	arvanCloudLBPoolListArgs
	PoolID string `json:"pool_id"`
}

// arvanCloudLBPoolOriginListArgs is embedded by every tool below that lists
// or creates an origin, scoped to exactly one pool.
type arvanCloudLBPoolOriginListArgs = arvanCloudLBPoolIDArgs

// arvanCloudLBPoolOriginIDArgs is embedded by every tool below scoped to
// exactly one origin by domain + load balancer + pool + origin id.
type arvanCloudLBPoolOriginIDArgs struct {
	arvanCloudLBPoolIDArgs
	OriginID string `json:"origin_id"`
}

// --- Renderers ----------------------------------------------------------

func arvanCloudLBRegionToMap(r domain.ArvanCloudLoadBalancerRegion) map[string]any {
	return map[string]any{
		"id":     r.ID,
		"region": r.Region,
		"name":   r.Name,
	}
}

func arvanCloudLBSettingsToMap(s domain.ArvanCloudLoadBalancerSettings) map[string]any {
	return map[string]any{
		"method":                  string(s.Method),
		"next_upstream_tcp":       string(s.NextUpstreamTCP),
		"next_upstream_tcp_codes": s.NextUpstreamTCPCodes,
		"protocol":                string(s.Protocol),
		"grpc_status":             s.GRPCStatus,
		"keepalive":               string(s.Keepalive),
		"max_fails":               s.MaxFails,
		"fail_timeout":            s.FailTimeout,
	}
}

func arvanCloudLBOriginToMap(o domain.ArvanCloudLoadBalancerOrigin) map[string]any {
	return map[string]any{
		"id":                  o.ID,
		"name":                o.Name,
		"health_check_status": string(o.HealthCheckStatus),
		"status":              o.Status,
		"address":             o.Address,
		"port":                o.Port,
		"weight":              o.Weight,
		"protocol":            string(o.Protocol),
		"host_header":         o.HostHeader,
		"created_at":          o.CreatedAt,
		"updated_at":          o.UpdatedAt,
	}
}

func arvanCloudLBPoolToMap(p domain.ArvanCloudLoadBalancerPool) map[string]any {
	origins := make([]map[string]any, len(p.Origins))
	for i, o := range p.Origins {
		origins[i] = arvanCloudLBOriginToMap(o)
	}
	return map[string]any{
		"id":                      p.ID,
		"name":                    p.Name,
		"description":             p.Description,
		"status":                  p.Status,
		"priority":                p.Priority,
		"method":                  string(p.Method),
		"keepalive":               string(p.Keepalive),
		"next_upstream_tcp":       string(p.NextUpstreamTCP),
		"next_upstream_tcp_codes": p.NextUpstreamTCPCodes,
		"regions":                 p.Regions,
		"origins":                 origins,
		"monitoring_status":       p.MonitoringStatus,
		"health_check":            p.HealthCheck,
		"created_at":              p.CreatedAt,
		"updated_at":              p.UpdatedAt,
	}
}

func arvanCloudLoadBalancerToMap(lb domain.ArvanCloudLoadBalancer) map[string]any {
	pools := make([]map[string]any, len(lb.Pools))
	for i, p := range lb.Pools {
		pools[i] = arvanCloudLBPoolToMap(p)
	}
	return map[string]any{
		"id":          lb.ID,
		"name":        lb.Name,
		"description": lb.Description,
		"status":      lb.Status,
		"method":      string(lb.Method),
		"time_slice":  lb.TimeSlice,
		"pools":       pools,
		"created_at":  lb.CreatedAt,
		"updated_at":  lb.UpdatedAt,
	}
}

// --- Regions ----------------------------------------------------------------

func listArvanCloudLBRegionsTool(uc *app.ListArvanCloudLBRegions) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_arvancloud_lb_regions",
		Description: "List every region ArvanCloud load-balancer pools can be scoped to, account-independent " +
			"(the same list regardless of domain). " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args credentialArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			regions, err := uc.Execute(ctx, app.ListArvanCloudLBRegionsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(regions))
			for i, r := range regions {
				out[i] = arvanCloudLBRegionToMap(r)
			}
			return map[string]any{"regions": out}, nil
		},
	}
}

func listArvanCloudDomainLBRegionsTool(uc *app.ListArvanCloudDomainLBRegions) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_domain_lb_regions",
		Description: "List the regions ArvanCloud load-balancer pools on a specific domain can be scoped to. " +
			arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			regions, err := uc.Execute(ctx, app.ListArvanCloudDomainLBRegionsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(regions))
			for i, r := range regions {
				out[i] = arvanCloudLBRegionToMap(r)
			}
			return map[string]any{"regions": out}, nil
		},
	}
}

// --- Settings -----------------------------------------------------------

// arvanCloudLBSettingsProperties adds the field set shared by
// get/update_arvancloud_lb_settings's input to props. Units are stated
// explicitly per AGENTS.md 5: max_fails counts consecutive failures,
// fail_timeout is a human-friendly duration string like "45s".
func arvanCloudLBSettingsProperties(props map[string]any) {
	props["method"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudPoolMethodClusterRR), string(domain.ArvanCloudPoolMethodClusterCHash)},
		"description": "Domain-wide default pool traffic-distribution strategy: \"cluster_rr\" (round-robin across origins) or \"cluster_chash\" (consistent hashing). No \"failover\" here — that only applies at the load-balancer level (see create_arvancloud_load_balancer's method).",
	}
	props["next_upstream_tcp"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudLBOn), string(domain.ArvanCloudLBOff)},
		"description": "Domain-wide default: try another origin when the first one fails at the TCP level, if \"on\".",
	}
	props["next_upstream_tcp_codes"] = map[string]any{
		"type": "object",
		"description": "Domain-wide default: upstream HTTP status codes, per request method, that trigger falling " +
			"over to the next origin (only applies when next_upstream_tcp is \"on\"). Keys are HTTP methods in " +
			"lowercase (\"head\", \"get\", \"post\", \"put\", \"delete\", \"options\", \"patch\"); values are arrays " +
			"of status codes from [500, 502, 503, 504, 403, 404, 429]. Example: {\"get\": [502, 503]}.",
		"additionalProperties": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "integer", "enum": []int{500, 502, 503, 504, 403, 404, 429}},
		},
	}
	props["protocol"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudOriginProtocolAuto), string(domain.ArvanCloudOriginProtocolHTTP), string(domain.ArvanCloudOriginProtocolHTTPS)},
		"description": "Domain-wide default protocol used to reach an origin when the origin itself does not override it. \"auto\" matches the incoming request's protocol.",
	}
	props["grpc_status"] = map[string]any{
		"type":        "boolean",
		"description": "Turn on gRPC proxying for the domain. Requires upstream services to actually support gRPC; when off, standard HTTP proxying is used.",
	}
	props["keepalive"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudLBOn), string(domain.ArvanCloudLBOff)},
		"description": "Domain-wide default: keep upstream connections to origins open between requests, if \"on\".",
	}
	props["max_fails"] = map[string]any{
		"type":        "integer",
		"description": "How many consecutive FAILURES mark an origin down before it is skipped, e.g. 3. Range 0-10000. Omit to let ArvanCloud apply its own default (a value of exactly 0 cannot be sent explicitly through this tool).",
	}
	props["fail_timeout"] = map[string]any{
		"type":        "string",
		"description": "How long a failed origin stays excluded before being retried, as a human-friendly duration string, e.g. \"45s\" for 45 seconds or \"2m\" for two minutes. Defaults to \"10s\" when omitted.",
	}
}

// arvanCloudLBSettingsArgs decodes get/update_arvancloud_lb_settings's
// shared field set.
type arvanCloudLBSettingsArgs struct {
	Method               string           `json:"method"`
	NextUpstreamTCP      string           `json:"next_upstream_tcp"`
	NextUpstreamTCPCodes map[string][]int `json:"next_upstream_tcp_codes"`
	Protocol             string           `json:"protocol"`
	GRPCStatus           bool             `json:"grpc_status"`
	Keepalive            string           `json:"keepalive"`
	MaxFails             int              `json:"max_fails"`
	FailTimeout          string           `json:"fail_timeout"`
}

func (a arvanCloudLBSettingsArgs) toDomain() domain.ArvanCloudLoadBalancerSettings {
	return domain.ArvanCloudLoadBalancerSettings{
		Method:               domain.ArvanCloudPoolMethod(a.Method),
		NextUpstreamTCP:      domain.ArvanCloudLBToggle(a.NextUpstreamTCP),
		NextUpstreamTCPCodes: a.NextUpstreamTCPCodes,
		Protocol:             domain.ArvanCloudOriginProtocol(a.Protocol),
		GRPCStatus:           a.GRPCStatus,
		Keepalive:            domain.ArvanCloudLBToggle(a.Keepalive),
		MaxFails:             a.MaxFails,
		FailTimeout:          a.FailTimeout,
	}
}

func getArvanCloudLBSettingsTool(uc *app.GetArvanCloudLBSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_lb_settings",
		Description: "Get a domain's global load-balancing defaults (method, keepalive, next-upstream retry " +
			"behavior, protocol, gRPC proxying, failure handling), applied to every load balancer on the domain " +
			"unless a pool overrides a field itself. " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudLBSettingsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudLBSettingsToMap(*found), nil
		},
	}
}

func updateArvanCloudLBSettingsTool(uc *app.UpdateArvanCloudLBSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudLBSettingsProperties(props)

	return Tool{
		Name: "update_arvancloud_lb_settings",
		Description: "Update a domain's global load-balancing defaults. " + arvanCloudLBNamingNote + " This is a " +
			"fast operation: the updated settings are returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				arvanCloudLBSettingsArgs
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudLBSettingsInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				Settings:    args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLBSettingsToMap(*updated), nil
		},
	}
}

// --- Load balancers -------------------------------------------------------

// arvanCloudLoadBalancerProperties adds the field set shared by
// create_arvancloud_load_balancer and update_arvancloud_load_balancer to
// props.
func arvanCloudLoadBalancerProperties(props map[string]any) {
	props["name"] = map[string]any{
		"type":        "string",
		"description": "REQUIRED. A label for the load balancer, letters/digits/hyphens only, e.g. \"lb1\".",
	}
	props["description"] = map[string]any{"type": "string", "description": "An optional note about what this load balancer is for."}
	props["status"] = map[string]any{"type": "boolean", "description": "Whether the load balancer is active. Defaults to true when omitted."}
	props["method"] = map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudLoadBalancerMethodFailover),
			string(domain.ArvanCloudLoadBalancerMethodClusterRR),
			string(domain.ArvanCloudLoadBalancerMethodClusterCHash),
		},
		"description": "REQUIRED. How traffic is distributed across this load balancer's pools: \"failover\" (send all traffic to the highest-priority healthy pool), \"cluster_rr\" (round-robin between pools, switching every time_slice) or \"cluster_chash\" (consistent hashing across pools).",
	}
	props["time_slice"] = map[string]any{
		"type":        "string",
		"description": "How long a pool stays selected under \"cluster_rr\" before switching to the next one, as a human-friendly duration string, e.g. \"30s\". Meaningful only for method \"cluster_rr\". Defaults to \"0s\" when omitted.",
	}
}

// arvanCloudLoadBalancerArgs decodes create/update_arvancloud_load_balancer's
// shared field set.
type arvanCloudLoadBalancerArgs struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      *bool  `json:"status"`
	Method      string `json:"method"`
	TimeSlice   string `json:"time_slice"`
}

func (a arvanCloudLoadBalancerArgs) toDomain() domain.ArvanCloudLoadBalancer {
	status := true
	if a.Status != nil {
		status = *a.Status
	}
	return domain.ArvanCloudLoadBalancer{
		Name:        a.Name,
		Description: a.Description,
		Status:      status,
		Method:      domain.ArvanCloudLoadBalancerMethod(a.Method),
		TimeSlice:   a.TimeSlice,
	}
}

func listArvanCloudLoadBalancersTool(uc *app.ListArvanCloudLoadBalancers) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name:        "list_arvancloud_load_balancers",
		Description: "List every load balancer configured for a domain. " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			lbs, err := uc.Execute(ctx, app.ListArvanCloudLoadBalancersInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(lbs))
			for i, lb := range lbs {
				out[i] = arvanCloudLoadBalancerToMap(lb)
			}
			return map[string]any{"load_balancers": out}, nil
		},
	}
}

func createArvanCloudLoadBalancerTool(uc *app.CreateArvanCloudLoadBalancer) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudLoadBalancerProperties(props)

	return Tool{
		Name: "create_arvancloud_load_balancer",
		Description: "Create a new load balancer on a domain: CDN edge-level traffic distribution across origin " +
			"pools you add afterward with create_arvancloud_lb_pool. " + arvanCloudLBNamingNote + " Creating a load " +
			"balancer configures routing rules — it does not provision new infrastructure. This is a fast " +
			"operation: the created load balancer, including its provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "name", "method"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				arvanCloudLoadBalancerArgs
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudLoadBalancerInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				LB:          args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLoadBalancerToMap(*created), nil
		},
	}
}

func getArvanCloudLoadBalancerTool(uc *app.GetArvanCloudLoadBalancer) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudLoadBalancerIDProperty()

	return Tool{
		Name:        "get_arvancloud_load_balancer",
		Description: "Get the current state of one load balancer by ID, including its pools. " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudLBIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudLoadBalancerInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudLoadBalancerToMap(*found), nil
		},
	}
}

func updateArvanCloudLoadBalancerTool(uc *app.UpdateArvanCloudLoadBalancer) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudLoadBalancerIDProperty()
	arvanCloudLoadBalancerProperties(props)

	return Tool{
		Name: "update_arvancloud_load_balancer",
		Description: "Update a load balancer. This replaces the load balancer's fields with the given values — " +
			"pass every field you want to keep, not only the ones changing; pools are managed separately and are " +
			"never touched by this call. " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "name", "method"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudLBIDArgs
				arvanCloudLoadBalancerArgs
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudLoadBalancerInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				ID:          args.ID,
				LB:          args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLoadBalancerToMap(*updated), nil
		},
	}
}

func deleteArvanCloudLoadBalancerTool(uc *app.DeleteArvanCloudLoadBalancer) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudLoadBalancerIDProperty()

	return Tool{
		Name: "delete_arvancloud_load_balancer",
		Description: "Permanently delete a load balancer by ID, along with its pools and origins. " + arvanCloudLBNamingNote +
			" This is a fast operation and cannot be undone. Deleting a load balancer that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudLBIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudLoadBalancerInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

// --- Pools ------------------------------------------------------------------

// arvanCloudLBOriginStoreProperty describes one element of a pool's writable
// "origins" array, used by create_arvancloud_lb_pool and
// replace_arvancloud_lb_pool_with_origins — the same per-origin shape
// create_arvancloud_lb_pool_origin/update_arvancloud_lb_pool_origin use for
// a single origin (arvanCloudLBOriginProperties), duplicated here as a
// nested object schema since JSON Schema has no $ref support in this tool
// registry.
func arvanCloudLBOriginStoreProperty() map[string]any {
	itemProps := map[string]any{}
	arvanCloudLBOriginProperties(itemProps)
	return map[string]any{
		"type":        "array",
		"description": "The pool's origins. Only meaningful for create_arvancloud_lb_pool and replace_arvancloud_lb_pool_with_origins — any existing origin not included here is removed by those two calls.",
		"items":       map[string]any{"type": "object", "properties": itemProps, "required": []string{"status", "protocol", "address", "port", "weight"}},
	}
}

// arvanCloudLBPoolProperties adds the field set shared by
// create_arvancloud_lb_pool, replace_arvancloud_lb_pool_with_origins and
// update_arvancloud_lb_pool_settings to props. Units are stated explicitly
// per AGENTS.md 5.
func arvanCloudLBPoolProperties(props map[string]any) {
	props["name"] = map[string]any{"type": "string", "description": "REQUIRED. A label for the pool."}
	props["description"] = map[string]any{"type": "string", "description": "An optional note about what this pool is for."}
	props["status"] = map[string]any{"type": "boolean", "description": "Whether the pool is active. Defaults to true when omitted."}
	props["priority"] = map[string]any{
		"type":        "integer",
		"description": "Orders this pool relative to its siblings; 0 means the default pool. Use reprioritize_arvancloud_lb_pool to reorder pools relative to each other instead of setting this directly.",
	}
	props["method"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudPoolMethodClusterRR), string(domain.ArvanCloudPoolMethodClusterCHash)},
		"description": "REQUIRED. How traffic is distributed across this pool's origins: \"cluster_rr\" (round-robin) or \"cluster_chash\" (consistent hashing). No \"failover\" here — that only applies at the load-balancer level.",
	}
	props["keepalive"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudLBOn), string(domain.ArvanCloudLBOff)},
		"description": "Keep upstream connections to this pool's origins open between requests, if \"on\". Defaults to \"off\" when omitted.",
	}
	props["next_upstream_tcp"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudLBOn), string(domain.ArvanCloudLBOff)},
		"description": "Try another origin in this pool when the first one fails at the TCP level, if \"on\". Defaults to \"off\" when omitted.",
	}
	props["next_upstream_tcp_codes"] = map[string]any{
		"type": "object",
		"description": "Upstream HTTP status codes, per request method, that trigger falling over to the next " +
			"origin (only applies when next_upstream_tcp is \"on\"). Keys are HTTP methods in lowercase (\"head\", " +
			"\"get\", \"post\", \"put\", \"delete\", \"options\", \"patch\"); values are arrays of status codes from " +
			"[500, 502, 503, 504, 403, 404, 429]. Example: {\"get\": [502, 503]}.",
		"additionalProperties": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "integer", "enum": []int{500, 502, 503, 504, 403, 404, 429}},
		},
	}
	props["regions"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Restrict this pool to specific region codes (3 uppercase letters, e.g. \"LAH\"), from list_arvancloud_lb_regions/list_arvancloud_domain_lb_regions. Leave empty to allow every region.",
	}
	props["origins"] = arvanCloudLBOriginStoreProperty()
}

// arvanCloudLBPoolArgs decodes create/replace/update_arvancloud_lb_pool*'s
// shared field set.
type arvanCloudLBPoolArgs struct {
	Name                 string                   `json:"name"`
	Description          string                   `json:"description"`
	Status               *bool                    `json:"status"`
	Priority             int                      `json:"priority"`
	Method               string                   `json:"method"`
	Keepalive            string                   `json:"keepalive"`
	NextUpstreamTCP      string                   `json:"next_upstream_tcp"`
	NextUpstreamTCPCodes map[string][]int         `json:"next_upstream_tcp_codes"`
	Regions              []string                 `json:"regions"`
	Origins              []arvanCloudLBOriginArgs `json:"origins"`
}

func (a arvanCloudLBPoolArgs) toDomain() domain.ArvanCloudLoadBalancerPool {
	status := true
	if a.Status != nil {
		status = *a.Status
	}
	origins := make([]domain.ArvanCloudLoadBalancerOrigin, len(a.Origins))
	for i, o := range a.Origins {
		origins[i] = o.toDomain()
	}
	return domain.ArvanCloudLoadBalancerPool{
		Name:                 a.Name,
		Description:          a.Description,
		Status:               status,
		Priority:             a.Priority,
		Method:               domain.ArvanCloudPoolMethod(a.Method),
		Keepalive:            domain.ArvanCloudLBToggle(a.Keepalive),
		NextUpstreamTCP:      domain.ArvanCloudLBToggle(a.NextUpstreamTCP),
		NextUpstreamTCPCodes: a.NextUpstreamTCPCodes,
		Regions:              a.Regions,
		Origins:              origins,
	}
}

func listArvanCloudLBPoolsTool(uc *app.ListArvanCloudLBPools) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()

	return Tool{
		Name:        "list_arvancloud_lb_pools",
		Description: "List every pool of a load balancer, including their origins. " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudLBPoolListArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			pools, err := uc.Execute(ctx, app.ListArvanCloudLBPoolsInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID,
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(pools))
			for i, p := range pools {
				out[i] = arvanCloudLBPoolToMap(p)
			}
			return map[string]any{"pools": out}, nil
		},
	}
}

func createArvanCloudLBPoolTool(uc *app.CreateArvanCloudLBPool) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	arvanCloudLBPoolProperties(props)

	return Tool{
		Name: "create_arvancloud_lb_pool",
		Description: "Create a new pool on a load balancer, optionally with its initial set of origins in the " +
			"same call. " + arvanCloudLBNamingNote + " This is a fast operation: the created pool, including its " +
			"provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "name", "method"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudLBPoolListArgs
				arvanCloudLBPoolArgs
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudLBPoolInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID,
				Pool: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLBPoolToMap(*created), nil
		},
	}
}

func reprioritizeArvanCloudLBPoolTool(uc *app.ReprioritizeArvanCloudLBPool) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	props["pool_id"] = map[string]any{"type": "string", "description": "REQUIRED. The ID of the pool to move."}
	props["after_pool_id"] = map[string]any{
		"type":        "string",
		"description": "Move pool_id to just after this pool (lower priority). Give exactly one of after_pool_id/before_pool_id, not both.",
	}
	props["before_pool_id"] = map[string]any{
		"type":        "string",
		"description": "Move pool_id to just before this pool (higher priority). Give exactly one of after_pool_id/before_pool_id, not both.",
	}

	return Tool{
		Name: "reprioritize_arvancloud_lb_pool",
		Description: "Change the priority order of a load balancer's pools by moving one pool relative to " +
			"another. Give exactly one of after_pool_id/before_pool_id, not both — this is a relative move, not a " +
			"full reordered list. " + arvanCloudLBNamingNote + " This is a fast operation: returns the load " +
			"balancer, including its pools in their new order, within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "pool_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudLBPoolListArgs
				PoolID       string `json:"pool_id"`
				AfterPoolID  string `json:"after_pool_id"`
				BeforePoolID string `json:"before_pool_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.ReprioritizeArvanCloudLBPoolInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID,
				PoolID: args.PoolID, AfterPoolID: args.AfterPoolID, BeforePoolID: args.BeforePoolID,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLoadBalancerToMap(*updated), nil
		},
	}
}

func getArvanCloudLBPoolTool(uc *app.GetArvanCloudLBPool) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	props["pool_id"] = arvanCloudLBPoolIDProperty()

	return Tool{
		Name:        "get_arvancloud_lb_pool",
		Description: "Get the current state of one pool by ID, including its origins. " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "pool_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudLBPoolIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudLBPoolInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID, PoolID: args.PoolID,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLBPoolToMap(*found), nil
		},
	}
}

func replaceArvanCloudLBPoolWithOriginsTool(uc *app.ReplaceArvanCloudLBPoolWithOrigins) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	props["pool_id"] = arvanCloudLBPoolIDProperty()
	arvanCloudLBPoolProperties(props)

	return Tool{
		Name: "replace_arvancloud_lb_pool_with_origins",
		Description: "Replace a pool's settings AND its full set of origins in one call — any existing origin not " +
			"included in the origins array here is REMOVED. Use this when you want to set the pool's complete " +
			"origin list at once; use update_arvancloud_lb_pool_settings instead when you only want to change the " +
			"pool's own settings and leave its existing origins untouched. " + arvanCloudLBNamingNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "pool_id", "name", "method"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudLBPoolIDArgs
				arvanCloudLBPoolArgs
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudLBPoolInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID, PoolID: args.PoolID,
				Pool: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLBPoolToMap(*updated), nil
		},
	}
}

func updateArvanCloudLBPoolSettingsTool(uc *app.UpdateArvanCloudLBPoolSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	props["pool_id"] = arvanCloudLBPoolIDProperty()
	arvanCloudLBPoolProperties(props)
	props["origins"] = map[string]any{
		"type":        "array",
		"description": "IGNORED by this tool — this call never touches a pool's origins. Use create_arvancloud_lb_pool_origin/update_arvancloud_lb_pool_origin/delete_arvancloud_lb_pool_origin to manage them individually, or replace_arvancloud_lb_pool_with_origins to set the whole list at once.",
		"items":       map[string]any{"type": "object"},
	}

	return Tool{
		Name: "update_arvancloud_lb_pool_settings",
		Description: "Update a pool's own settings (name, status, priority, method, keepalive, next-upstream " +
			"retry behavior, regions) WITHOUT touching its existing origins. Pass every field you want to keep, " +
			"not only the ones changing. Use replace_arvancloud_lb_pool_with_origins instead when you also want " +
			"to change the pool's origins. " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "pool_id", "name", "method"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudLBPoolIDArgs
				arvanCloudLBPoolArgs
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			pool := args.toDomain()
			pool.Origins = nil // this call never touches origins; see the tool's own description.

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudLBPoolInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID, PoolID: args.PoolID,
				Pool: pool,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLBPoolToMap(*updated), nil
		},
	}
}

func deleteArvanCloudLBPoolTool(uc *app.DeleteArvanCloudLBPool) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	props["pool_id"] = arvanCloudLBPoolIDProperty()

	return Tool{
		Name: "delete_arvancloud_lb_pool",
		Description: "Permanently delete a pool by ID, along with its origins. " + arvanCloudLBNamingNote +
			" This is a fast operation and cannot be undone. Deleting a pool that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "pool_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudLBPoolIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudLBPoolInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID, PoolID: args.PoolID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "load_balancer_id": args.LoadBalancerID, "pool_id": args.PoolID}, nil
		},
	}
}

// --- Origins ------------------------------------------------------------

// arvanCloudLBOriginProperties adds the field set shared by
// create_arvancloud_lb_pool_origin/update_arvancloud_lb_pool_origin (and,
// nested, a pool's own writable "origins" array) to props. Units are stated
// explicitly per AGENTS.md 5: port is a TCP port number, weight is a
// relative traffic share.
func arvanCloudLBOriginProperties(props map[string]any) {
	props["name"] = map[string]any{"type": "string", "description": "An optional label for the origin."}
	props["status"] = map[string]any{"type": "boolean", "description": "REQUIRED. Whether this origin currently receives traffic (admin enable/disable, independent of its live health check)."}
	props["address"] = map[string]any{"type": "string", "description": "REQUIRED. The origin's IP address or hostname, e.g. \"203.0.113.10\" or \"backend1.internal.example.com\"."}
	props["port"] = map[string]any{
		"type":        "integer",
		"description": "REQUIRED. The TCP PORT to connect to on address, e.g. 443 for HTTPS or 80 for HTTP. Range 1-65535.",
	}
	props["weight"] = map[string]any{
		"type":        "integer",
		"description": "REQUIRED. This origin's relative share of traffic within the pool, e.g. 100. Range 1-1000; higher gets proportionally more traffic.",
	}
	props["protocol"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudOriginProtocolAuto), string(domain.ArvanCloudOriginProtocolHTTP), string(domain.ArvanCloudOriginProtocolHTTPS)},
		"description": "REQUIRED. The protocol used to reach this origin. \"auto\" matches the incoming request's protocol. Defaults to \"auto\" when omitted.",
	}
	props["host_header"] = map[string]any{"type": "string", "description": "Override the Host header sent to this origin. Leave empty to use ArvanCloud's own default."}
}

// arvanCloudLBOriginArgs decodes one origin's field set, used both for a
// standalone origin call and as an element of a pool's "origins" array.
type arvanCloudLBOriginArgs struct {
	Name       string `json:"name"`
	Status     *bool  `json:"status"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Weight     int    `json:"weight"`
	Protocol   string `json:"protocol"`
	HostHeader string `json:"host_header"`
}

func (a arvanCloudLBOriginArgs) toDomain() domain.ArvanCloudLoadBalancerOrigin {
	status := true
	if a.Status != nil {
		status = *a.Status
	}
	protocol := domain.ArvanCloudOriginProtocol(a.Protocol)
	if protocol == "" {
		protocol = domain.ArvanCloudOriginProtocolAuto
	}
	return domain.ArvanCloudLoadBalancerOrigin{
		Name:       a.Name,
		Status:     status,
		Address:    a.Address,
		Port:       a.Port,
		Weight:     a.Weight,
		Protocol:   protocol,
		HostHeader: a.HostHeader,
	}
}

func listArvanCloudLBPoolOriginsTool(uc *app.ListArvanCloudLBPoolOrigins) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	props["pool_id"] = arvanCloudLBPoolIDProperty()

	return Tool{
		Name:        "list_arvancloud_lb_pool_origins",
		Description: "List every origin in a pool. " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "pool_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudLBPoolOriginListArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			origins, err := uc.Execute(ctx, app.ListArvanCloudLBPoolOriginsInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID, PoolID: args.PoolID,
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(origins))
			for i, o := range origins {
				out[i] = arvanCloudLBOriginToMap(o)
			}
			return map[string]any{"origins": out}, nil
		},
	}
}

func createArvanCloudLBPoolOriginTool(uc *app.CreateArvanCloudLBPoolOrigin) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	props["pool_id"] = arvanCloudLBPoolIDProperty()
	arvanCloudLBOriginProperties(props)

	return Tool{
		Name: "create_arvancloud_lb_pool_origin",
		Description: "Create a new origin in a pool: a backend server the edge proxies traffic to. " +
			arvanCloudLBNamingNote + " This is a fast operation: the created origin, including its " +
			"provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "pool_id", "status", "address", "port", "weight", "protocol"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudLBPoolIDArgs
				arvanCloudLBOriginArgs
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudLBPoolOriginInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID, PoolID: args.PoolID,
				Origin: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLBOriginToMap(*created), nil
		},
	}
}

func getArvanCloudLBPoolOriginTool(uc *app.GetArvanCloudLBPoolOrigin) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	props["pool_id"] = arvanCloudLBPoolIDProperty()
	props["origin_id"] = arvanCloudLBOriginIDProperty()

	return Tool{
		Name:        "get_arvancloud_lb_pool_origin",
		Description: "Get the current state of one origin by ID, including its live health-check status. " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "pool_id", "origin_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudLBPoolOriginIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudLBPoolOriginInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID, PoolID: args.PoolID, OriginID: args.OriginID,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLBOriginToMap(*found), nil
		},
	}
}

func updateArvanCloudLBPoolOriginTool(uc *app.UpdateArvanCloudLBPoolOrigin) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	props["pool_id"] = arvanCloudLBPoolIDProperty()
	props["origin_id"] = arvanCloudLBOriginIDProperty()
	arvanCloudLBOriginProperties(props)

	return Tool{
		Name: "update_arvancloud_lb_pool_origin",
		Description: "Update an origin. This replaces the origin's fields with the given values — pass every " +
			"field you want to keep, not only the ones changing. " + arvanCloudLBNamingNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "pool_id", "origin_id", "status", "address", "port", "weight", "protocol"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudLBPoolOriginIDArgs
				arvanCloudLBOriginArgs
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudLBPoolOriginInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID, PoolID: args.PoolID, OriginID: args.OriginID,
				Origin: args.toDomain(),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudLBOriginToMap(*updated), nil
		},
	}
}

func deleteArvanCloudLBPoolOriginTool(uc *app.DeleteArvanCloudLBPoolOrigin) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["load_balancer_id"] = arvanCloudLoadBalancerIDArgProperty()
	props["pool_id"] = arvanCloudLBPoolIDProperty()
	props["origin_id"] = arvanCloudLBOriginIDProperty()

	return Tool{
		Name: "delete_arvancloud_lb_pool_origin",
		Description: "Permanently delete an origin by ID. " + arvanCloudLBNamingNote +
			" This is a fast operation and cannot be undone. Deleting an origin that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "load_balancer_id", "pool_id", "origin_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudLBPoolOriginIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudLBPoolOriginInput{
				Credentials: args.domain(), Domain: args.Domain, LoadBalancerID: args.LoadBalancerID, PoolID: args.PoolID, OriginID: args.OriginID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{
				"deleted": true, "domain": args.Domain, "load_balancer_id": args.LoadBalancerID,
				"pool_id": args.PoolID, "origin_id": args.OriginID,
			}, nil
		},
	}
}
