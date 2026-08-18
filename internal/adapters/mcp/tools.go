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
	SetupDNS           *app.SetupDNS
	GetOperationStatus *app.GetOperationStatus

	CreateFirewall *app.CreateFirewall
	GetFirewall    *app.GetFirewall
	ListFirewalls  *app.ListFirewalls
	UpdateFirewall *app.UpdateFirewall
	DeleteFirewall *app.DeleteFirewall
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
		createDNSRecordTool(uc.SetupDNS),
		getOperationStatusTool(uc.GetOperationStatus),
		createFirewallTool(uc.CreateFirewall),
		getFirewallTool(uc.GetFirewall),
		listFirewallsTool(uc.ListFirewalls),
		updateFirewallTool(uc.UpdateFirewall),
		deleteFirewallTool(uc.DeleteFirewall),
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

type createDNSRecordArgs struct {
	credentialArgs
	Zone     string `json:"zone"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority"`
}

func createDNSRecordTool(uc *app.SetupDNS) Tool {
	props := credentialProperties()
	props["zone"] = map[string]any{
		"type":        "string",
		"description": "The DNS zone (domain) the record belongs to, e.g. \"example.com\".",
	}
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Record name relative to the zone, e.g. \"api\" for api.example.com. Use \"@\" for the zone apex.",
	}
	props["type"] = map[string]any{
		"type":        "string",
		"enum":        []string{"A", "AAAA", "CNAME", "TXT", "MX", "NS", "SRV"},
		"description": "DNS record type, e.g. \"A\" for an IPv4 address.",
	}
	props["value"] = map[string]any{
		"type":        "string",
		"description": "Record value: an IPv4 address for A, a hostname for CNAME, arbitrary text for TXT.",
	}
	props["ttl"] = map[string]any{
		"type":        "integer",
		"description": "Time to live in seconds, e.g. 3600 for one hour. Omit to use the provider default.",
		"minimum":     60,
	}
	props["priority"] = map[string]any{
		"type":        "integer",
		"description": "Priority, e.g. 10. Only meaningful for MX and SRV records.",
	}

	return Tool{
		Name: "create_dns_record",
		Description: "Create a DNS record in a Parspack-hosted zone. This is a fast operation: the created record is " +
			"returned within this call. The zone is looked up by domain name, so no zone id is needed.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone", "name", "type", "value"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createDNSRecordArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			recordType, err := domain.ParseDNSRecordType(args.Type)
			if err != nil {
				return nil, fmt.Errorf("record type %q is not supported: %w", args.Type, err)
			}

			rec, err := uc.Execute(ctx, app.SetupDNSInput{
				Credentials: args.domain(),
				ZoneName:    args.Zone,
				Record: domain.DNSRecord{
					Name:     args.Name,
					Type:     recordType,
					Value:    args.Value,
					TTL:      args.TTL,
					Priority: args.Priority,
				},
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"id":    rec.ID,
				"zone":  args.Zone,
				"name":  rec.Name,
				"type":  rec.Type.String(),
				"value": rec.Value,
				"ttl":   rec.TTL,
			}, nil
		},
	}
}

type firewallRuleArgs struct {
	Protocol  string   `json:"protocol"`
	PortRange string   `json:"port_range"`
	Addresses []string `json:"addresses"`
}

type firewallArgs struct {
	credentialArgs
	Name          string             `json:"name"`
	ServerIDs     []string           `json:"server_ids"`
	InboundRules  []firewallRuleArgs `json:"inbound_rules"`
	OutboundRules []firewallRuleArgs `json:"outbound_rules"`
}

type firewallIDArgs struct {
	credentialArgs
	FirewallID string `json:"firewall_id"`
}

type updateFirewallArgs struct {
	firewallArgs
	FirewallID string `json:"firewall_id"`
}

func (a firewallArgs) firewall() domain.Firewall {
	fw := domain.Firewall{
		Name:      a.Name,
		ServerIDs: a.ServerIDs,
	}
	for _, r := range a.InboundRules {
		fw.InboundRules = append(fw.InboundRules, firewallRuleArgsToDomain(r))
	}
	for _, r := range a.OutboundRules {
		fw.OutboundRules = append(fw.OutboundRules, firewallRuleArgsToDomain(r))
	}
	return fw
}

func firewallRuleArgsToDomain(r firewallRuleArgs) domain.FirewallRule {
	return domain.FirewallRule{Protocol: r.Protocol, PortRange: r.PortRange, Addresses: r.Addresses}
}

// firewallRuleProperties is the JSON Schema for one inbound/outbound rule
// block, shared by create_firewall and update_firewall.
func firewallRuleProperties() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"protocol": map[string]any{
				"type":        "string",
				"enum":        []string{"tcp", "udp", "icmp"},
				"description": "The type of traffic the rule allows: \"tcp\", \"udp\", or \"icmp\".",
			},
			"port_range": map[string]any{
				"type":        "string",
				"description": "A single port, a range like \"8000-9000\", or \"1-65535\" for all ports. Required for tcp and udp rules, ignored for icmp.",
			},
			"addresses": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Source addresses (CIDRs or single IPs) for an inbound rule, or destination addresses for an outbound rule. Omit to mean all addresses.",
			},
		},
		"required": []string{"protocol"},
	}
}

func createFirewallTool(uc *app.CreateFirewall) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Human-readable firewall name, e.g. \"only-22-80-and-443\".",
	}
	props["server_ids"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "IDs of the servers (VMs) the firewall is applied to, as returned by create_server or list_servers. Omit to create the firewall without attaching any server yet.",
	}
	props["inbound_rules"] = map[string]any{
		"type":        "array",
		"items":       firewallRuleProperties(),
		"description": "Inbound rules: traffic allowed INTO the attached servers. Each rule lists its source addresses under \"addresses\".",
	}
	props["outbound_rules"] = map[string]any{
		"type":        "array",
		"items":       firewallRuleProperties(),
		"description": "Outbound rules: traffic allowed OUT of the attached servers. Each rule lists its destination addresses under \"addresses\".",
	}

	return Tool{
		Name: "create_firewall",
		Description: "Create a new rules-based network firewall at Parspack and optionally attach it to servers. This is a " +
			"fast operation: the created firewall is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args firewallArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			fw, err := uc.Execute(ctx, app.CreateFirewallInput{
				Credentials: args.domain(),
				Firewall:    args.firewall(),
			})
			if err != nil {
				return nil, err
			}
			return firewallToMap(*fw), nil
		},
	}
}

func getFirewallTool(uc *app.GetFirewall) Tool {
	props := credentialProperties()
	props["firewall_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the firewall to look up, as returned by create_firewall or list_firewalls.",
	}

	return Tool{
		Name: "get_firewall",
		Description: "Get the current state of one firewall at Parspack by its provider ID. This is a fast " +
			"operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "firewall_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args firewallIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			fw, err := uc.Execute(ctx, app.GetFirewallInput{
				Credentials: args.domain(),
				FirewallID:  args.FirewallID,
			})
			if err != nil {
				return nil, err
			}
			return firewallToMap(*fw), nil
		},
	}
}

func listFirewallsTool(uc *app.ListFirewalls) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_firewalls",
		Description: "List every firewall at Parspack visible to the given credentials. This is a fast operation: " +
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

			firewalls, err := uc.Execute(ctx, app.ListFirewallsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(firewalls))
			for i, fw := range firewalls {
				out[i] = firewallToMap(fw)
			}
			return map[string]any{"firewalls": out}, nil
		},
	}
}

func updateFirewallTool(uc *app.UpdateFirewall) Tool {
	props := credentialProperties()
	props["firewall_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the firewall to update, as returned by create_firewall or list_firewalls.",
	}
	props["name"] = map[string]any{
		"type":        "string",
		"description": "New human-readable firewall name, e.g. \"only-22-80-and-443\".",
	}
	props["server_ids"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "IDs of the servers (VMs) the firewall should be applied to, replacing the previous set.",
	}
	props["inbound_rules"] = map[string]any{
		"type":        "array",
		"items":       firewallRuleProperties(),
		"description": "Inbound rules: traffic allowed INTO the attached servers. Replaces the previous inbound rules. Each rule lists its source addresses under \"addresses\".",
	}
	props["outbound_rules"] = map[string]any{
		"type":        "array",
		"items":       firewallRuleProperties(),
		"description": "Outbound rules: traffic allowed OUT of the attached servers. Replaces the previous outbound rules. Each rule lists its destination addresses under \"addresses\".",
	}

	return Tool{
		Name: "update_firewall",
		Description: "Replace the configuration of an existing firewall at Parspack (rules, server attachments, and " +
			"name). This is a fast operation: the updated firewall is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "firewall_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateFirewallArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			fw, err := uc.Execute(ctx, app.UpdateFirewallInput{
				Credentials: args.domain(),
				FirewallID:  args.FirewallID,
				Firewall:    args.firewall(),
			})
			if err != nil {
				return nil, err
			}
			return firewallToMap(*fw), nil
		},
	}
}

func deleteFirewallTool(uc *app.DeleteFirewall) Tool {
	props := credentialProperties()
	props["firewall_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the firewall to delete, as returned by create_firewall or list_firewalls.",
	}

	return Tool{
		Name: "delete_firewall",
		Description: "Permanently delete a firewall at Parspack by its provider ID. This is a fast operation and " +
			"cannot be undone. Deleting a firewall that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "firewall_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args firewallIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteFirewallInput{
				Credentials: args.domain(),
				FirewallID:  args.FirewallID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "firewall_id": args.FirewallID}, nil
		},
	}
}

// firewallToMap renders a domain.Firewall the way every firewall-returning
// tool reports it back to the caller.
func firewallToMap(fw domain.Firewall) map[string]any {
	inbound := make([]map[string]any, len(fw.InboundRules))
	for i, r := range fw.InboundRules {
		inbound[i] = firewallRuleToMap(r)
	}
	outbound := make([]map[string]any, len(fw.OutboundRules))
	for i, r := range fw.OutboundRules {
		outbound[i] = firewallRuleToMap(r)
	}
	return map[string]any{
		"id":             fw.ID,
		"name":           fw.Name,
		"status":         fw.Status,
		"server_ids":     fw.ServerIDs,
		"inbound_rules":  inbound,
		"outbound_rules": outbound,
		"created_at":     fw.CreatedAt,
	}
}

func firewallRuleToMap(r domain.FirewallRule) map[string]any {
	return map[string]any{
		"protocol":   r.Protocol,
		"port_range": r.PortRange,
		"addresses":  r.Addresses,
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
