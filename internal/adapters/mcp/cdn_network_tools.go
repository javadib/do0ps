package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
)

// --- HTTPS convertor -------------------------------------------------------

func getCDNHTTPSConvertorTool(uc *app.GetCDNHTTPSConvertor) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_https_convertor",
		Description: "Get whether Parspack automatically rewrites HTTP links to HTTPS for a CDN zone. This is a " +
			"fast operation: the result is returned within this call.",
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

			setting, err := uc.Execute(ctx, app.GetCDNHTTPSConvertorInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": setting.Enabled}, nil
		},
	}
}

type updateCDNHTTPSConvertorArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Enabled  bool   `json:"enabled"`
}

func updateCDNHTTPSConvertorTool(uc *app.UpdateCDNHTTPSConvertor) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["enabled"] = map[string]any{
		"type":        "boolean",
		"description": "Whether to automatically rewrite HTTP links to HTTPS for this zone.",
	}

	return Tool{
		Name: "update_cdn_https_convertor",
		Description: "Set whether Parspack automatically rewrites HTTP links to HTTPS for a CDN zone. This is a " +
			"fast operation: the setting now in effect is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNHTTPSConvertorArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			setting, err := uc.Execute(ctx, app.UpdateCDNHTTPSConvertorInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Enabled: args.Enabled,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": setting.Enabled}, nil
		},
	}
}

// --- Edge-to-upstream connection --------------------------------------------

func getCDNEdgeToUpstreamConnectionTool(uc *app.GetCDNEdgeToUpstreamConnection) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_edge_to_upstream_connection",
		Description: "Get the protocol Parspack's edge nodes currently use when connecting to the origin server " +
			"for a CDN zone. This is a fast operation: the result is returned within this call.",
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

			setting, err := uc.Execute(ctx, app.GetCDNEdgeToUpstreamConnectionInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "type": setting.Type}, nil
		},
	}
}

type updateCDNEdgeToUpstreamConnectionArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Type     string `json:"type"`
}

func updateCDNEdgeToUpstreamConnectionTool(uc *app.UpdateCDNEdgeToUpstreamConnection) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["type"] = map[string]any{
		"type":        "string",
		"enum":        []string{"auto", "http", "https"},
		"description": "Protocol Parspack's edge nodes should use when connecting to the origin server. \"auto\" mirrors the visitor's protocol.",
	}

	return Tool{
		Name: "update_cdn_edge_to_upstream_connection",
		Description: "Set the protocol Parspack's edge nodes use when connecting to the origin server for a CDN " +
			"zone. This is a fast operation: the setting now in effect is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "type"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNEdgeToUpstreamConnectionArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			setting, err := uc.Execute(ctx, app.UpdateCDNEdgeToUpstreamConnectionInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Type: args.Type,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "type": setting.Type}, nil
		},
	}
}

// --- WWW redirection ---------------------------------------------------

func getCDNWWWRedirectionTool(uc *app.GetCDNWWWRedirection) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_www_redirection",
		Description: "Get the current www/non-www redirection mode for a CDN zone. This is a fast operation: the " +
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

			setting, err := uc.Execute(ctx, app.GetCDNWWWRedirectionInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "www_redirection": setting.Mode}, nil
		},
	}
}

type updateCDNWWWRedirectionArgs struct {
	credentialArgs
	ZoneUUID       string `json:"zone_uuid"`
	WWWRedirection string `json:"www_redirection"`
}

func updateCDNWWWRedirectionTool(uc *app.UpdateCDNWWWRedirection) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["www_redirection"] = map[string]any{
		"type": "string",
		"enum": []string{"none", "redirect-to-www", "redirect-from-www"},
		"description": "Redirection mode: \"none\" for no redirect, \"redirect-to-www\" to send the bare domain to " +
			"its www subdomain, \"redirect-from-www\" to send the www subdomain to the bare domain.",
	}

	return Tool{
		Name: "update_cdn_www_redirection",
		Description: "Set the www/non-www redirection mode for a CDN zone. This is a fast operation: the setting " +
			"now in effect is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "www_redirection"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNWWWRedirectionArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			setting, err := uc.Execute(ctx, app.UpdateCDNWWWRedirectionInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Mode: args.WWWRedirection,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "www_redirection": setting.Mode}, nil
		},
	}
}

// --- WebSocket -----------------------------------------------------------

func getCDNWebSocketTool(uc *app.GetCDNWebSocket) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_web_socket",
		Description: "Get whether WebSocket connections are currently allowed through a CDN zone. This is a fast " +
			"operation: the result is returned within this call.",
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

			setting, err := uc.Execute(ctx, app.GetCDNWebSocketInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": setting.Enabled}, nil
		},
	}
}

type updateCDNWebSocketArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Enabled  bool   `json:"enabled"`
}

func updateCDNWebSocketTool(uc *app.UpdateCDNWebSocket) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["enabled"] = map[string]any{
		"type":        "boolean",
		"description": "Whether to allow WebSocket connections through this zone.",
	}

	return Tool{
		Name: "update_cdn_web_socket",
		Description: "Set whether WebSocket connections are allowed through a CDN zone. This is a fast operation: " +
			"the setting now in effect is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNWebSocketArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			setting, err := uc.Execute(ctx, app.UpdateCDNWebSocketInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Enabled: args.Enabled,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": setting.Enabled}, nil
		},
	}
}
