package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// CDN zone-level settings/toggles beyond issue #19's zone/order/DNS scope
// (issue #24): antivirus, DNSSEC, asset optimization, developer mode,
// maintenance mode, query-string caching behavior, and origin-offline
// handling. Every tool here is a fast operation (AGENTS.md 4.3): the result
// is returned within the same tool call, no polling involved.
//
// Developer mode, maintenance mode, query-string and origin-offline have no
// dedicated single-setting GET documented in the CDN API spec — only their
// update tools are exposed here. See
// internal/adapters/providers/parspack/cdn_zone_settings.go's top-level
// comment for the full explanation.

// enabledArgs is the zone-scoped {api_key, secret_key, zone_uuid, enabled}
// shape shared by every plain on/off setting's update tool.
type enabledArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Enabled  bool   `json:"enabled"`
}

func enabledUpdateProperties(description string) map[string]any {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["enabled"] = map[string]any{
		"type":        "boolean",
		"description": description,
	}
	return props
}

func getCDNAntivirusStatusTool(uc *app.GetCDNAntivirusStatus) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_antivirus_status",
		Description: "Get whether antivirus scanning is enabled for a Parspack CDN zone. This is a fast " +
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

			enabled, err := uc.Execute(ctx, app.GetCDNAntivirusStatusInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": enabled}, nil
		},
	}
}

func updateCDNAntivirusStatusTool(uc *app.UpdateCDNAntivirusStatus) Tool {
	props := enabledUpdateProperties("Whether antivirus scanning should be enabled for the zone.")

	return Tool{
		Name: "update_cdn_antivirus_status",
		Description: "Enable or disable antivirus scanning for a Parspack CDN zone. This is a fast operation: " +
			"the applied value is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args enabledArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			enabled, err := uc.Execute(ctx, app.UpdateCDNAntivirusStatusInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Enabled: args.Enabled,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": enabled}, nil
		},
	}
}

func dnsSecStatusToMap(zoneUUID string, status domain.CDNDNSSecStatus) map[string]any {
	return map[string]any{"zone_uuid": zoneUUID, "enabled": status.Enabled, "ds_record": status.Value}
}

func getCDNDNSSecStatusTool(uc *app.GetCDNDNSSecStatus) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_dnssec_status",
		Description: "Get the DNSSEC status of a Parspack CDN zone, including the DS record the domain " +
			"registrar must be given once DNSSEC is enabled. This is a fast operation: the result is returned " +
			"within this call.",
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

			status, err := uc.Execute(ctx, app.GetCDNDNSSecStatusInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return dnsSecStatusToMap(args.ZoneUUID, *status), nil
		},
	}
}

func updateCDNDNSSecStatusTool(uc *app.UpdateCDNDNSSecStatus) Tool {
	props := enabledUpdateProperties("Whether DNSSEC should be enabled for the zone.")

	return Tool{
		Name: "update_cdn_dnssec_status",
		Description: "Enable or disable DNSSEC for a Parspack CDN zone. This is a fast operation: the resulting " +
			"status, including the DS record when DNSSEC is enabled, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args enabledArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			status, err := uc.Execute(ctx, app.UpdateCDNDNSSecStatusInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Enabled: args.Enabled,
			})
			if err != nil {
				return nil, err
			}
			return dnsSecStatusToMap(args.ZoneUUID, *status), nil
		},
	}
}

// optimizationArgs is the input shape shared by get and update tools'
// output, and update's input: the three flat flags plus the nested
// website-minification trio.
type optimizationArgs struct {
	credentialArgs
	ZoneUUID                string `json:"zone_uuid"`
	ImageMinificationStatus bool   `json:"image_minification_status"`
	WebPConversionStatus    bool   `json:"webp_conversion_status"`
	MinifyHTML              bool   `json:"minify_html"`
	MinifyCSS               bool   `json:"minify_css"`
	MinifyJS                bool   `json:"minify_js"`
}

func (a optimizationArgs) toDomainStatus() domain.CDNOptimizationStatus {
	return domain.CDNOptimizationStatus{
		ImageMinification: a.ImageMinificationStatus,
		WebPConversion:    a.WebPConversionStatus,
		MinifyHTML:        a.MinifyHTML,
		MinifyCSS:         a.MinifyCSS,
		MinifyJS:          a.MinifyJS,
	}
}

func optimizationStatusToMap(zoneUUID string, status domain.CDNOptimizationStatus) map[string]any {
	return map[string]any{
		"zone_uuid":                 zoneUUID,
		"image_minification_status": status.ImageMinification,
		"webp_conversion_status":    status.WebPConversion,
		"minify_html":               status.MinifyHTML,
		"minify_css":                status.MinifyCSS,
		"minify_js":                 status.MinifyJS,
	}
}

func getCDNOptimizationStatusTool(uc *app.GetCDNOptimizationStatus) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_optimization_status",
		Description: "Get a Parspack CDN zone's asset optimization configuration: image minification, WebP " +
			"conversion, and per-asset-type (HTML/CSS/JS) website minification. This is a fast operation: the " +
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

			status, err := uc.Execute(ctx, app.GetCDNOptimizationStatusInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return optimizationStatusToMap(args.ZoneUUID, *status), nil
		},
	}
}

func updateCDNOptimizationTool(uc *app.UpdateCDNOptimization) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["image_minification_status"] = map[string]any{
		"type":        "boolean",
		"description": "Whether images served through the CDN should be minified/compressed.",
	}
	props["webp_conversion_status"] = map[string]any{
		"type":        "boolean",
		"description": "Whether eligible images should be converted to WebP for supporting clients.",
	}
	props["minify_html"] = map[string]any{
		"type":        "boolean",
		"description": "Whether HTML responses should be minified.",
	}
	props["minify_css"] = map[string]any{
		"type":        "boolean",
		"description": "Whether CSS responses should be minified.",
	}
	props["minify_js"] = map[string]any{
		"type":        "boolean",
		"description": "Whether JavaScript responses should be minified.",
	}

	return Tool{
		Name: "update_cdn_optimization",
		Description: "Replace a Parspack CDN zone's asset optimization configuration (image minification, WebP " +
			"conversion, and per-asset-type website minification). This is a fast operation: the applied " +
			"configuration is returned within this call. All fields are required — this replaces the whole " +
			"configuration, it does not merge with the previous one.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required": []string{
				"api_key", "zone_uuid", "image_minification_status", "webp_conversion_status",
				"minify_html", "minify_css", "minify_js",
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args optimizationArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			status, err := uc.Execute(ctx, app.UpdateCDNOptimizationInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Status: args.toDomainStatus(),
			})
			if err != nil {
				return nil, err
			}
			return optimizationStatusToMap(args.ZoneUUID, *status), nil
		},
	}
}

func updateCDNDeveloperModeTool(uc *app.UpdateCDNDeveloperMode) Tool {
	props := enabledUpdateProperties("Whether developer mode (bypasses CDN caching for troubleshooting) should be enabled for the zone.")

	return Tool{
		Name: "update_cdn_developer_mode",
		Description: "Enable or disable developer mode for a Parspack CDN zone — while on, the CDN bypasses " +
			"caching so origin changes are visible immediately. This is a fast operation: the applied value is " +
			"returned within this call. There is no dedicated tool to read the current value back: the CDN API " +
			"does not document a single-setting GET for this toggle.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args enabledArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			enabled, err := uc.Execute(ctx, app.UpdateCDNDeveloperModeInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Enabled: args.Enabled,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": enabled}, nil
		},
	}
}

func updateCDNMaintenanceModeTool(uc *app.UpdateCDNMaintenanceMode) Tool {
	props := enabledUpdateProperties("Whether maintenance mode (shows visitors a maintenance page) should be enabled for the zone.")

	return Tool{
		Name: "update_cdn_maintenance_mode",
		Description: "Enable or disable maintenance mode for a Parspack CDN zone. This is a fast operation: the " +
			"applied value is returned within this call. There is no dedicated tool to read the current value " +
			"back: the CDN API does not document a single-setting GET for this toggle.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args enabledArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			enabled, err := uc.Execute(ctx, app.UpdateCDNMaintenanceModeInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Enabled: args.Enabled,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": enabled}, nil
		},
	}
}

func updateCDNQueryStringSettingTool(uc *app.UpdateCDNQueryStringSetting) Tool {
	props := enabledUpdateProperties(
		"Whether the CDN should ignore query strings when caching (true caches one response per URL path " +
			"regardless of its query string; false caches a separate response per distinct query string).")

	return Tool{
		Name: "update_cdn_query_string_setting",
		Description: "Update a Parspack CDN zone's query-string caching behavior. This is a fast operation: the " +
			"applied value is returned within this call. There is no dedicated tool to read the current value " +
			"back: the CDN API does not document a single-setting GET for this toggle.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args enabledArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			enabled, err := uc.Execute(ctx, app.UpdateCDNQueryStringSettingInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Enabled: args.Enabled,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": enabled}, nil
		},
	}
}

func updateCDNOriginOfflineTool(uc *app.UpdateCDNOriginOffline) Tool {
	props := enabledUpdateProperties(
		"Whether the CDN should keep serving cached content for the zone when its origin server is unreachable.")

	return Tool{
		Name: "update_cdn_origin_offline",
		Description: "Enable or disable origin-offline handling for a Parspack CDN zone. This is a fast " +
			"operation: the applied value is returned within this call. There is no dedicated tool to read the " +
			"current value back: the CDN API does not document a single-setting GET for this toggle.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args enabledArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			enabled, err := uc.Execute(ctx, app.UpdateCDNOriginOfflineInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Enabled: args.Enabled,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": enabled}, nil
		},
	}
}
