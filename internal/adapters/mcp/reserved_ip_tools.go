package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// reservedIPToMap renders a domain.ReservedIP the way every reserved-IP tool
// reports it back to the caller.
func reservedIPToMap(ip domain.ReservedIP) map[string]any {
	return map[string]any{
		"ip_address": ip.IPAddress,
		"region":     ip.Region,
		"server_id":  ip.ServerID,
		"urn":        ip.URN,
	}
}

func reserveIPTool(uc *app.ReserveIP) Tool {
	props := credentialProperties()
	props["region"] = map[string]any{
		"type":        "string",
		"description": "Provider region to reserve the address in, e.g. \"tehran\". The address stays billed from reservation even while it is not attached to any server.",
	}

	return Tool{
		Name: "reserve_ip",
		Description: "Reserve a static public IPv4 address at Parspack, unassigned to any server. This is a fast " +
			"operation: the reserved address is returned within this call. Attach it to a server later with " +
			"assign_ip_to_server.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "region"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				Region string `json:"region"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			ip, err := uc.Execute(ctx, app.ReserveIPInput{
				Credentials: args.domain(),
				Region:      args.Region,
			})
			if err != nil {
				return nil, err
			}
			return reservedIPToMap(*ip), nil
		},
	}
}

func releaseIPTool(uc *app.ReleaseIP) Tool {
	props := credentialProperties()
	props["ip_address"] = map[string]any{
		"type":        "string",
		"description": "The reserved IP address to release, e.g. \"203.0.113.10\", as returned by reserve_ip.",
	}

	return Tool{
		Name: "release_ip",
		Description: "Release a reserved IP address at Parspack, returning it to the pool and stopping billing. This " +
			"is a fast operation and cannot be undone. Releasing an address that no longer exists is treated as " +
			"already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "ip_address"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				IPAddress string `json:"ip_address"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.ReleaseIPInput{
				Credentials: args.domain(),
				IPAddress:   args.IPAddress,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"released": true, "ip_address": args.IPAddress}, nil
		},
	}
}

func assignIPToServerTool(uc *app.AssignIPToServer) Tool {
	props := credentialProperties()
	props["ip_address"] = map[string]any{
		"type":        "string",
		"description": "The reserved IP address to attach, e.g. \"203.0.113.10\", as returned by reserve_ip.",
	}
	props["server_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the server to attach the address to, as returned by create_server or list_servers.",
	}

	return Tool{
		Name: "assign_ip_to_server",
		Description: "Attach an existing reserved IP address to a server at Parspack. This is a fast operation: " +
			"the address's updated state is returned within this call. The address itself stays reserved and can " +
			"be reassigned later.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "ip_address", "server_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				IPAddress string `json:"ip_address"`
				ServerID  string `json:"server_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			ip, err := uc.Execute(ctx, app.AssignIPToServerInput{
				Credentials: args.domain(),
				IPAddress:   args.IPAddress,
				ServerID:    args.ServerID,
			})
			if err != nil {
				return nil, err
			}
			return reservedIPToMap(*ip), nil
		},
	}
}

func unassignIPTool(uc *app.UnassignIP) Tool {
	props := credentialProperties()
	props["ip_address"] = map[string]any{
		"type":        "string",
		"description": "The reserved IP address to detach, e.g. \"203.0.113.10\".",
	}

	return Tool{
		Name: "unassign_ip",
		Description: "Detach a reserved IP address from the server it is currently attached to at Parspack. This " +
			"is a fast operation: the address's updated state is returned within this call. The address itself " +
			"stays reserved and keeps being billed.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "ip_address"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				IPAddress string `json:"ip_address"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			ip, err := uc.Execute(ctx, app.UnassignIPInput{
				Credentials: args.domain(),
				IPAddress:   args.IPAddress,
			})
			if err != nil {
				return nil, err
			}
			return reservedIPToMap(*ip), nil
		},
	}
}
