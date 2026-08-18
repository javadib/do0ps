package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// UseCases collects the application entry points this adapter exposes.
type UseCases struct {
	ProvisionServer    *app.ProvisionServer
	ListServers        *app.ListServers
	GetServer          *app.GetServer
	DeleteServer       *app.DeleteServer
	GetOperationStatus *app.GetOperationStatus

	CreateCDNZone        *app.CreateCDNZone
	ListCDNZones         *app.ListCDNZones
	GetCDNZone           *app.GetCDNZone
	DeleteCDNZone        *app.DeleteCDNZone
	ListCDNZonePlans     *app.ListCDNZonePlans
	GetNameserverRecords *app.GetNameserverRecords
	ListDNSRecords       *app.ListDNSRecords
	CreateDNSRecord      *app.CreateDNSRecord
	UpdateDNSRecord      *app.UpdateDNSRecord
	DeleteDNSRecord      *app.DeleteDNSRecord
}

// credentialProperties are repeated on every provider-touching tool: the
// chatbot session holds the user's provider credentials and passes them on
// each call, since this server stores none (AGENTS.md 4.2).
func credentialProperties() map[string]any {
	return map[string]any{
		"api_key": map[string]any{
			"type":        "string",
			"description": "Provider API key from the user's provider account. Never stored by this server; supply it on every call.",
		},
		"secret_key": map[string]any{
			"type":        "string",
			"description": "Provider secret key, if the provider issues a key pair. Leave empty when the provider uses a single API key.",
		},
	}
}

type credentialArgs struct {
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
}

func (a credentialArgs) domain() domain.ProviderCredentials {
	return domain.ProviderCredentials{APIKey: a.APIKey, SecretKey: a.SecretKey}
}

// Tools builds the tool set backed by the given use cases.
func Tools(uc UseCases) []Tool {
	return []Tool{
		PingTool(),
		createServerTool(uc.ProvisionServer),
		listServersTool(uc.ListServers),
		getServerTool(uc.GetServer),
		deleteServerTool(uc.DeleteServer),
		getOperationStatusTool(uc.GetOperationStatus),
		createCDNZoneTool(uc.CreateCDNZone),
		listCDNZonesTool(uc.ListCDNZones),
		getCDNZoneTool(uc.GetCDNZone),
		deleteCDNZoneTool(uc.DeleteCDNZone),
		listCDNZonePlansTool(uc.ListCDNZonePlans),
		getNameserverRecordsTool(uc.GetNameserverRecords),
		listDNSRecordsTool(uc.ListDNSRecords),
		createDNSRecordTool(uc.CreateDNSRecord),
		updateDNSRecordTool(uc.UpdateDNSRecord),
		deleteDNSRecordTool(uc.DeleteDNSRecord),
	}
}

// PingTool is a trivial built-in tool with no use case or provider behind it.
// It proves the full MCP transport round-trip end-to-end — a client can
// connect, list tools, and call one successfully — before any real business
// tool exists (AGENTS.md 5).
func PingTool() Tool {
	return Tool{
		Name:        "ping",
		Description: "Health-check tool with no side effects. Returns \"pong\" to confirm the MCP server is reachable.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
			return map[string]any{"message": "pong"}, nil
		},
	}
}

type createServerArgs struct {
	credentialArgs
	Name     string   `json:"name"`
	Region   string   `json:"region"`
	Image    string   `json:"image"`
	PlanID   string   `json:"plan_id"`
	CPUCores int      `json:"cpu_cores"`
	RAMMB    int      `json:"ram_mb"`
	DiskGB   int      `json:"disk_gb"`
	SSHKeys  []string `json:"ssh_keys"`
}

func createServerTool(uc *app.ProvisionServer) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Hostname for the new server, e.g. \"web-01\". Must be unique within the account; it is also how a retry recognizes an already-created server.",
	}
	props["region"] = map[string]any{
		"type":        "string",
		"description": "Provider datacenter region, e.g. \"tehran\". Omit to use the provider default.",
	}
	props["image"] = map[string]any{
		"type":        "string",
		"description": "Operating system image, e.g. \"ubuntu-24.04\".",
	}
	props["plan_id"] = map[string]any{
		"type":        "string",
		"description": "Provider plan/flavor identifier. Supply this when the user names a plan; otherwise give cpu_cores, ram_mb and disk_gb instead.",
	}
	props["cpu_cores"] = map[string]any{
		"type":        "integer",
		"description": "Number of virtual CPU cores, e.g. 2.",
		"minimum":     1,
	}
	props["ram_mb"] = map[string]any{
		"type":        "integer",
		"description": "RAM in megabytes, e.g. 2048 for 2GB.",
		"minimum":     512,
	}
	props["disk_gb"] = map[string]any{
		"type":        "integer",
		"description": "Disk size in gigabytes, e.g. 40.",
		"minimum":     10,
	}
	props["ssh_keys"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "IDs or fingerprints of SSH keys already registered with the provider, to be installed on the new server.",
	}

	return Tool{
		Name: "create_server",
		Description: "Provision a new server (VPS) at Parspack. This is a long operation: it returns immediately with " +
			"an operation_id and status \"pending\". Poll get_operation_status with that id to learn when the server is ready.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createServerArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			out, err := uc.Execute(ctx, app.ProvisionServerInput{
				Credentials: args.domain(),
				Spec: domain.ServerSpec{
					Name:     args.Name,
					Region:   args.Region,
					Image:    args.Image,
					PlanID:   args.PlanID,
					CPUCores: args.CPUCores,
					RAMMB:    args.RAMMB,
					DiskGB:   args.DiskGB,
					SSHKeys:  args.SSHKeys,
				},
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"operation_id": out.OperationID,
				"status":       out.Status.String(),
				"note":         "Server provisioning runs in the background. Call get_operation_status with this operation_id to check progress.",
			}, nil
		},
	}
}

// serverToMap renders a domain.Server the way every server-returning tool
// reports it back to the caller.
func serverToMap(srv domain.Server) map[string]any {
	return map[string]any{
		"id":           srv.ID,
		"name":         srv.Name,
		"status":       srv.Status.String(),
		"region":       srv.Region,
		"image":        srv.Image,
		"plan_id":      srv.PlanID,
		"cpu_cores":    srv.CPUCores,
		"ram_mb":       srv.RAMMB,
		"disk_gb":      srv.DiskGB,
		"ipv4":         srv.IPv4,
		"ipv4_private": srv.IPv4Private,
		"ipv6":         srv.IPv6,
		"vpc_uuid":     srv.VPCUUID,
		"created_at":   srv.CreatedAt,
	}
}

func listServersTool(uc *app.ListServers) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_servers",
		Description: "List every server (VPS) at Parspack visible to the given credentials. This is a fast operation: " +
			"the list is returned within this call.",
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

			servers, err := uc.Execute(ctx, app.ListServersInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(servers))
			for i, srv := range servers {
				out[i] = serverToMap(srv)
			}
			return map[string]any{"servers": out}, nil
		},
	}
}

type serverIDArgs struct {
	credentialArgs
	ServerID string `json:"server_id"`
}

func getServerTool(uc *app.GetServer) Tool {
	props := credentialProperties()
	props["server_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the server to look up, as returned by create_server or list_servers.",
	}

	return Tool{
		Name: "get_server",
		Description: "Get the current state of one server (VPS) at Parspack by its provider ID. This is a fast " +
			"operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "server_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args serverIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			srv, err := uc.Execute(ctx, app.GetServerInput{
				Credentials: args.domain(),
				ServerID:    args.ServerID,
			})
			if err != nil {
				return nil, err
			}
			return serverToMap(*srv), nil
		},
	}
}

func deleteServerTool(uc *app.DeleteServer) Tool {
	props := credentialProperties()
	props["server_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the server to delete, as returned by create_server or list_servers.",
	}

	return Tool{
		Name: "delete_server",
		Description: "Permanently delete a server (VPS) at Parspack by its provider ID. This is a fast operation " +
			"and cannot be undone. Deleting a server that no longer exists is treated as already done rather than " +
			"an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "server_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args serverIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteServerInput{
				Credentials: args.domain(),
				ServerID:    args.ServerID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "server_id": args.ServerID}, nil
		},
	}
}

// cdnZoneToMap renders a domain.CDNZone the way every zone-returning tool
// reports it back to the caller.
func cdnZoneToMap(zone domain.CDNZone) map[string]any {
	return map[string]any{
		"zone_uuid":      zone.UUID,
		"id":             zone.ID,
		"domain":         zone.Domain,
		"status":         zone.Status,
		"plan":           zone.Plan,
		"billing_cycle":  zone.BillingCycle,
		"proxy":          zone.Proxy,
		"ns_status":      zone.NSStatus,
		"remaining_days": zone.RemainingDays,
		"expire_at":      zone.ExpireAt,
	}
}

type createCDNZoneArgs struct {
	credentialArgs
	Domain        string `json:"domain"`
	Plan          string `json:"plan"`
	BillingCycle  string `json:"billing_cycle"`
	PromotionCode string `json:"promotion_code"`
}

func createCDNZoneTool(uc *app.CreateCDNZone) Tool {
	props := credentialProperties()
	props["domain"] = map[string]any{
		"type":        "string",
		"description": "The domain to onboard onto Parspack's CDN, e.g. \"example.com\".",
	}
	props["plan"] = map[string]any{
		"type":        "string",
		"enum":        []string{"free", "standard", "premium", "professional"},
		"description": "CDN subscription plan. Use list_cdn_plans to see pricing for each.",
	}
	props["billing_cycle"] = map[string]any{
		"type":        "string",
		"enum":        []string{"free", "monthly", "quarterly", "semiannually", "annually"},
		"description": "Billing cycle for the plan, e.g. \"monthly\". Use \"free\" only with the free plan.",
	}
	props["promotion_code"] = map[string]any{
		"type":        "string",
		"description": "Optional promotion/discount code to apply to the order.",
	}

	return Tool{
		Name: "create_cdn_zone",
		Description: "Onboard a new domain onto Parspack's CDN (creates a \"zone\"). This is a fast operation: the " +
			"created zone, including its status, is returned within this call. Default MX and NS records are created " +
			"automatically. After this call, use get_nameserver_records to learn which nameservers the domain " +
			"registrar must be pointed at.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "plan", "billing_cycle"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createCDNZoneArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			zone, err := uc.Execute(ctx, app.CreateCDNZoneInput{
				Credentials: args.domain(),
				Spec: domain.CDNZoneSpec{
					Domain:        args.Domain,
					Plan:          args.Plan,
					BillingCycle:  args.BillingCycle,
					PromotionCode: args.PromotionCode,
				},
			})
			if err != nil {
				return nil, err
			}
			return cdnZoneToMap(*zone), nil
		},
	}
}

func listCDNZonesTool(uc *app.ListCDNZones) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_cdn_zones",
		Description: "List every CDN zone at Parspack visible to the given credentials. This is a fast operation: " +
			"the list is returned within this call.",
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

			zones, err := uc.Execute(ctx, app.ListCDNZonesInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(zones))
			for i, zone := range zones {
				out[i] = cdnZoneToMap(zone)
			}
			return map[string]any{"zones": out}, nil
		},
	}
}

type zoneUUIDArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
}

func zoneUUIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The zone's UUID, as returned by create_cdn_zone or list_cdn_zones.",
	}
}

func getCDNZoneTool(uc *app.GetCDNZone) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_zone",
		Description: "Get the current state of one CDN zone at Parspack by its UUID. This is a fast operation: the " +
			"result is returned within this call.",
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

			zone, err := uc.Execute(ctx, app.GetCDNZoneInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return cdnZoneToMap(*zone), nil
		},
	}
}

func deleteCDNZoneTool(uc *app.DeleteCDNZone) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "delete_cdn_zone",
		Description: "Permanently remove a domain from Parspack's CDN by its zone UUID. This is a fast operation " +
			"and cannot be undone. Deleting a zone that no longer exists is treated as already done rather than an " +
			"error.",
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

			if err := uc.Execute(ctx, app.DeleteCDNZoneInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID}, nil
		},
	}
}

func listCDNZonePlansTool(uc *app.ListCDNZonePlans) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_cdn_plans",
		Description: "List the CDN subscription plans Parspack offers and their pricing per billing cycle. Use this " +
			"before create_cdn_zone to help the user pick a plan. This is a fast operation.",
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

			plans, err := uc.Execute(ctx, app.ListCDNZonePlansInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(plans))
			for i, p := range plans {
				out[i] = map[string]any{
					"plan":         p.Plan,
					"currency":     p.Currency,
					"monthly":      p.Monthly,
					"quarterly":    p.Quarterly,
					"semiannually": p.Semiannually,
					"annually":     p.Annually,
				}
			}
			return map[string]any{"plans": out}, nil
		},
	}
}

func getNameserverRecordsTool(uc *app.GetNameserverRecords) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_nameserver_records",
		Description: "Get the nameservers a CDN zone's domain registrar must be pointed at, plus (when known) the " +
			"nameservers currently configured there. This is a fast operation.",
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

			ns, err := uc.Execute(ctx, app.GetNameserverRecordsInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"ns1": ns.NS1, "ns2": ns.NS2, "ns3": ns.NS3, "ns4": ns.NS4,
				"current_ns": ns.CurrentNS,
			}, nil
		},
	}
}

// dnsRecordValueProperties are the fields of a single DNS record value,
// shared by create_dns_record and update_dns_record: content is always
// required, the rest are only meaningful for specific record types.
func dnsRecordValueProperties(props map[string]any) {
	props["content"] = map[string]any{
		"type":        "string",
		"description": "Record value: an IPv4 address for A, a hostname for CNAME/MX/NS, arbitrary text for TXT.",
	}
	props["port"] = map[string]any{
		"type":        "integer",
		"description": "Target port. Only meaningful for SRV records, e.g. 5060.",
	}
	props["weight"] = map[string]any{
		"type":        "integer",
		"description": "Relative weight among records with the same priority. Only meaningful for SRV records.",
	}
	props["priority"] = map[string]any{
		"type":        "integer",
		"description": "Priority, lower is preferred. Only meaningful for MX and SRV records, e.g. 10.",
	}
	props["flags"] = map[string]any{
		"type":        "integer",
		"description": "CAA flags. Only meaningful for CAA records; the provider currently only accepts 0.",
	}
	props["tag"] = map[string]any{
		"type":        "string",
		"enum":        []string{"issue", "issuewild", "iodef"},
		"description": "CAA tag naming what the value authorizes. Only meaningful for CAA records.",
	}
}

func dnsRecordTypeAndTTLProperties(props map[string]any) {
	props["type"] = map[string]any{
		"type":        "string",
		"enum":        []string{"A", "CNAME", "MX", "TXT", "NS", "SRV", "CAA"},
		"description": "DNS record type, e.g. \"A\" for an IPv4 address.",
	}
	props["ttl"] = map[string]any{
		"type": "integer",
		"description": "Time to live in seconds, e.g. 3600 for one hour. Must be one of Parspack's supported " +
			"values (1, 5, 30, 60, 300, 3600, 86400, ...) — an arbitrary value is rejected.",
		"enum": []int{1, 2, 5, 10, 30, 60, 180, 300, 600, 900, 1800, 2700, 3600,
			10800, 18000, 36000, 43200, 86400, 259200, 604800, 864000, 1296000, 2592000},
	}
	props["proxy"] = map[string]any{
		"type":        "string",
		"enum":        []string{"direct", "cdn-no-caching", "cdn-static-caching", "cdn-smart-caching", "cdn-always-caching"},
		"description": "CDN proxy mode for the record. Use \"direct\" to bypass the CDN and resolve straight to content.",
	}
}

// dnsRecordArgs is shared by create_dns_record and update_dns_record: both
// take a zone-scoped host+type plus exactly one value.
type dnsRecordArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Host     string `json:"host"`
	Type     string `json:"type"`
	TTL      int    `json:"ttl"`
	Proxy    string `json:"proxy"`
	Content  string `json:"content"`
	Port     int    `json:"port"`
	Weight   int    `json:"weight"`
	Priority int    `json:"priority"`
	Flags    int    `json:"flags"`
	Tag      string `json:"tag"`
}

func (a dnsRecordArgs) toDomainRecord() (domain.DNSRecord, error) {
	recordType, err := domain.ParseDNSRecordType(a.Type)
	if err != nil {
		return domain.DNSRecord{}, fmt.Errorf("record type %q is not supported: %w", a.Type, err)
	}
	proxy, err := domain.ParseDNSRecordProxy(a.Proxy)
	if err != nil {
		return domain.DNSRecord{}, fmt.Errorf("proxy mode %q is not supported: %w", a.Proxy, err)
	}

	return domain.DNSRecord{
		ZoneUUID: a.ZoneUUID,
		Host:     a.Host,
		Type:     recordType,
		TTL:      a.TTL,
		Proxy:    proxy,
		Values: []domain.DNSRecordValue{{
			Content: a.Content, Port: a.Port, Weight: a.Weight, Priority: a.Priority, Flags: a.Flags, Tag: a.Tag,
		}},
	}, nil
}

// dnsRecordToMap renders a domain.DNSRecord (with its first value, the
// common case of one value per host+type) the way create/update_dns_record
// report it back to the caller.
func dnsRecordToMap(rec domain.DNSRecord) map[string]any {
	out := map[string]any{
		"zone_uuid": rec.ZoneUUID,
		"host":      rec.Host,
		"type":      rec.Type.String(),
		"ttl":       rec.TTL,
		"proxy":     rec.Proxy.String(),
	}
	if len(rec.Values) > 0 {
		out["content"] = rec.Values[0].Content
		out["priority"] = rec.Values[0].Priority
	}
	return out
}

func listDNSRecordsTool(uc *app.ListDNSRecords) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_dns_records",
		Description: "List every DNS record of a Parspack CDN zone. Records are grouped by host and type: a single " +
			"entry's values array can hold more than one value, e.g. multiple NS records for the zone apex. This is " +
			"a fast operation.",
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

			records, err := uc.Execute(ctx, app.ListDNSRecordsInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(records))
			for i, rec := range records {
				values := make([]map[string]any, len(rec.Values))
				for j, v := range rec.Values {
					values[j] = map[string]any{
						"content": v.Content, "port": v.Port, "weight": v.Weight,
						"priority": v.Priority, "flags": v.Flags, "tag": v.Tag,
					}
				}
				out[i] = map[string]any{
					"host": rec.Host, "type": rec.Type.String(), "ttl": rec.TTL,
					"proxy": rec.Proxy.String(), "values": values,
				}
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "records": out}, nil
		},
	}
}

func createDNSRecordTool(uc *app.CreateDNSRecord) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["host"] = map[string]any{
		"type":        "string",
		"description": "Record host relative to the zone, e.g. \"api\" for api.example.com. Use \"@\" for the zone apex.",
	}
	dnsRecordTypeAndTTLProperties(props)
	dnsRecordValueProperties(props)

	return Tool{
		Name: "create_dns_record",
		Description: "Create a DNS record in a Parspack CDN zone. This is a fast operation: the created record is " +
			"returned within this call. Creating a record with the same host and type as an existing one appends a " +
			"new value instead of erroring, e.g. to add a second NS record for the same host.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "host", "type", "ttl", "proxy", "content"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args dnsRecordArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			rec, err := args.toDomainRecord()
			if err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateDNSRecordInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				Record:      rec,
			})
			if err != nil {
				return nil, err
			}
			return dnsRecordToMap(*created), nil
		},
	}
}

func updateDNSRecordTool(uc *app.UpdateDNSRecord) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["host"] = map[string]any{
		"type":        "string",
		"description": "Host of the existing record to update, e.g. \"api\". Use \"@\" for the zone apex.",
	}
	dnsRecordTypeAndTTLProperties(props)
	dnsRecordValueProperties(props)

	return Tool{
		Name: "update_dns_record",
		Description: "Update an existing DNS record (matched by zone, host and type) in a Parspack CDN zone: TTL, " +
			"proxy mode and value can change, but this cannot add or remove values under that host and type. This " +
			"is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "host", "type", "ttl", "proxy", "content"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args dnsRecordArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			rec, err := args.toDomainRecord()
			if err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateDNSRecordInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				Record:      rec,
			})
			if err != nil {
				return nil, err
			}
			return dnsRecordToMap(*updated), nil
		},
	}
}

type deleteDNSRecordArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Host     string `json:"host"`
	Type     string `json:"type"`
	Content  string `json:"content"`
}

func deleteDNSRecordTool(uc *app.DeleteDNSRecord) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["host"] = map[string]any{
		"type":        "string",
		"description": "Host of the record to delete, e.g. \"api\". Use \"@\" for the zone apex.",
	}
	props["type"] = map[string]any{
		"type":        "string",
		"enum":        []string{"A", "CNAME", "MX", "TXT", "NS", "SRV", "CAA"},
		"description": "Type of the record to delete.",
	}
	props["content"] = map[string]any{
		"type": "string",
		"description": "The specific value to remove, e.g. \"1.2.3.4\" for one A record among several under the " +
			"same host. Omit to delete every value under this host and type.",
	}

	return Tool{
		Name: "delete_dns_record",
		Description: "Delete a DNS record (or one value of it) from a Parspack CDN zone by host and type. This is a " +
			"fast operation and cannot be undone. Deleting a record that no longer exists is treated as already " +
			"done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "host", "type"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args deleteDNSRecordArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			recordType, err := domain.ParseDNSRecordType(args.Type)
			if err != nil {
				return nil, fmt.Errorf("record type %q is not supported: %w", args.Type, err)
			}

			if err := uc.Execute(ctx, app.DeleteDNSRecordInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				Host:        args.Host,
				Type:        recordType,
				Content:     args.Content,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID, "host": args.Host, "type": args.Type}, nil
		},
	}
}

type getOperationStatusArgs struct {
	credentialArgs
	OperationID string `json:"operation_id"`
}

func getOperationStatusTool(uc *app.GetOperationStatus) Tool {
	props := credentialProperties()
	props["operation_id"] = map[string]any{
		"type":        "string",
		"description": "The operation_id returned by a long operation such as create_server.",
	}

	return Tool{
		Name: "get_operation_status",
		Description: "Check the progress of a long operation started earlier, such as create_server. Returns status " +
			"pending, running, succeeded or failed, plus the result once it succeeded. Passing the provider credentials " +
			"is recommended: if the server restarted while the operation was in flight, they let this call confirm with " +
			"the provider whether the resource was actually created.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"operation_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args getOperationStatusArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			op, err := uc.Execute(ctx, app.GetOperationStatusInput{
				OperationID: args.OperationID,
				Credentials: args.domain(),
			})
			if err != nil {
				return nil, err
			}

			out := map[string]any{
				"operation_id": op.ID,
				"status":       op.Status.String(),
				"updated_at":   op.UpdatedAt,
			}
			if len(op.Result) > 0 {
				out["result"] = json.RawMessage(op.Result)
			}
			if op.Error != "" {
				out["error"] = op.Error
			}
			return out, nil
		},
	}
}

// decodeArgs rejects malformed tool arguments at the boundary, so use cases
// can trust what they receive.
func decodeArgs(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return fmt.Errorf("tool arguments are required: %w", domain.ErrInvalidInput)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decoding tool arguments: %w: %v", domain.ErrInvalidInput, err)
	}
	return nil
}
