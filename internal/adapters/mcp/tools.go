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
	RegisterSSHKey     *app.RegisterSSHKey
	ListSSHKeys        *app.ListSSHKeys
	DeleteSSHKey       *app.DeleteSSHKey
	SetupDNS           *app.SetupDNS
	GetOperationStatus *app.GetOperationStatus
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
		registerSSHKeyTool(uc.RegisterSSHKey),
		listSSHKeysTool(uc.ListSSHKeys),
		deleteSSHKeyTool(uc.DeleteSSHKey),
		createDNSRecordTool(uc.SetupDNS),
		getOperationStatusTool(uc.GetOperationStatus),
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

type registerSSHKeyArgs struct {
	credentialArgs
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

func registerSSHKeyTool(uc *app.RegisterSSHKey) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Human-readable label for the key, e.g. \"laptop\" or \"ci-runner\". Must be unique within the account.",
	}
	props["public_key"] = map[string]any{
		"type":        "string",
		"description": "The public key contents, e.g. \"ssh-ed25519 AAAAC3... user@host\". Sent to the provider as-is.",
	}

	return Tool{
		Name: "register_ssh_key",
		Description: "Register an SSH public key with the provider so it can be installed on new servers via " +
			"create_server's ssh_keys parameter. This is a fast operation: the created key (with its provider id " +
			"and fingerprint) is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name", "public_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args registerSSHKeyArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			key, err := uc.Execute(ctx, app.RegisterSSHKeyInput{
				Credentials: args.domain(),
				Key:         domain.SSHKey{Name: args.Name, PublicKey: args.PublicKey},
			})
			if err != nil {
				return nil, err
			}
			return sshKeyToMap(*key), nil
		},
	}
}

func listSSHKeysTool(uc *app.ListSSHKeys) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_ssh_keys",
		Description: "List every SSH key registered with the provider for the given credentials. This is a fast " +
			"operation: the list is returned within this call.",
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

			keys, err := uc.Execute(ctx, app.ListSSHKeysInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(keys))
			for i, key := range keys {
				out[i] = sshKeyToMap(key)
			}
			return map[string]any{"ssh_keys": out}, nil
		},
	}
}

type sshKeyIDArgs struct {
	credentialArgs
	KeyID string `json:"key_id"`
}

func deleteSSHKeyTool(uc *app.DeleteSSHKey) Tool {
	props := credentialProperties()
	props["key_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID (or fingerprint) of the key to delete, as returned by register_ssh_key or list_ssh_keys.",
	}

	return Tool{
		Name: "delete_ssh_key",
		Description: "Permanently delete a registered SSH key by its provider ID or fingerprint. This is a fast " +
			"operation and cannot be undone. Deleting a key that no longer exists is treated as already done rather " +
			"than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "key_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args sshKeyIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteSSHKeyInput{
				Credentials: args.domain(),
				KeyID:       args.KeyID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "key_id": args.KeyID}, nil
		},
	}
}

// sshKeyToMap renders a domain.SSHKey the way every key-returning tool reports
// it back to the caller.
func sshKeyToMap(key domain.SSHKey) map[string]any {
	return map[string]any{
		"id":          key.ID,
		"name":        key.Name,
		"fingerprint": key.Fingerprint,
		"public_key":  key.PublicKey,
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
