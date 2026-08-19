package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// Cache Management tools (issue #24). Every tool here is FAST: the
// Parspack CDN API returns "{success, message, data}" synchronously for
// every Cache Management endpoint (see internal/core/app/cdn_cache.go's doc
// comment for the full reasoning) — none of these tool calls returns an
// operation_id to poll with get_operation_status.

// cdnCacheTTLEnum is the fixed TTL enum shared with dns_record.go's
// dnsRecordTypeAndTTLProperties — the CDN API documents the same values for
// both DNS record TTL and edge cache TTL.
var cdnCacheTTLEnum = []int{
	1, 2, 5, 10, 30, 60, 180, 300, 600, 900, 1800, 2700, 3600,
	10800, 18000, 36000, 43200, 86400, 259200, 604800, 864000, 1296000, 2592000,
}

// cdnCacheRuleEnum is the fixed cache_rule enum shared by update_cdn_cache_rule
// and the cache_rule field reported by get_cdn_cache_settings.
var cdnCacheRuleEnum = []string{"cdn-no-caching", "cdn-static-caching", "cdn-smart-caching", "cdn-always-caching"}

// cacheTTLArgs is the input of update_cdn_cache_ttl.
type cacheTTLArgs struct {
	credentialArgs
	ZoneUUID   string `json:"zone_uuid"`
	TTLSeconds int    `json:"edge_cache_ttl"`
}

func updateCDNCacheTTLTool(uc *app.UpdateCDNCacheTTL) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["edge_cache_ttl"] = map[string]any{
		"type":        "integer",
		"description": "Edge cache TTL in seconds, e.g. 10800 for 3 hours. Must be one of Parspack's supported values — an arbitrary value is rejected.",
		"enum":        cdnCacheTTLEnum,
	}

	return Tool{
		Name: "update_cdn_cache_ttl",
		Description: "Set the edge cache TTL of a Parspack CDN zone. This is a fast operation: the provider's " +
			"response carries no separate job to poll, so the setting is confirmed within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "edge_cache_ttl"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cacheTTLArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			setting, err := uc.Execute(ctx, app.UpdateCDNCacheTTLInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, TTLSeconds: args.TTLSeconds,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "edge_cache_ttl": setting.EdgeCacheTTLSeconds}, nil
		},
	}
}

// cacheRuleArgs is the input of update_cdn_cache_rule.
type cacheRuleArgs struct {
	credentialArgs
	ZoneUUID  string `json:"zone_uuid"`
	CacheRule string `json:"cache_rule"`
}

func updateCDNCacheRuleTool(uc *app.UpdateCDNCacheRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["cache_rule"] = map[string]any{
		"type":        "string",
		"enum":        cdnCacheRuleEnum,
		"description": "Zone-wide cache rule, e.g. \"cdn-smart-caching\" to let the CDN decide what to cache.",
	}

	return Tool{
		Name: "update_cdn_cache_rule",
		Description: "Set the zone-wide cache rule of a Parspack CDN zone. This is a fast operation: the " +
			"provider's response carries no separate job to poll, so the setting is confirmed within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "cache_rule"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cacheRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			setting, err := uc.Execute(ctx, app.UpdateCDNCacheRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, CacheRule: args.CacheRule,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "cache_rule": setting.CacheRule}, nil
		},
	}
}

// cacheUserAgentArgs is the input of update_cdn_cache_user_agent.
type cacheUserAgentArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Enabled  bool   `json:"enabled"`
}

func updateCDNCacheUserAgentTool(uc *app.UpdateCDNCacheUserAgentSetting) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["enabled"] = map[string]any{
		"type":        "boolean",
		"description": "Whether to cache content separately per User-Agent header (e.g. serve different cached versions to mobile vs. desktop).",
	}

	return Tool{
		Name: "update_cdn_cache_user_agent",
		Description: "Enable or disable per-user-agent caching for a Parspack CDN zone. This is a fast operation: " +
			"the provider's response carries no separate job to poll, so the setting is confirmed within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cacheUserAgentArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			setting, err := uc.Execute(ctx, app.UpdateCDNCacheUserAgentSettingInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Enabled: args.Enabled,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "enabled": setting.Enabled}, nil
		},
	}
}

func getCDNCacheSettingsTool(uc *app.GetCDNCacheSettings) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_cache_settings",
		Description: "Get the aggregate cache configuration of a Parspack CDN zone (developer mode, maintenance " +
			"mode, ignore-query-string, cache rule, edge cache TTL, origin-offline handling, per-user-agent " +
			"caching). This is a fast, read-only operation; use update_cdn_cache_ttl, update_cdn_cache_rule or " +
			"update_cdn_cache_user_agent to change one of these fields.",
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

			settings, err := uc.Execute(ctx, app.GetCDNCacheSettingsInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"zone_uuid":                   args.ZoneUUID,
				"developer_mode":              settings.DeveloperMode,
				"maintenance_mode":            settings.MaintenanceMode,
				"ignore_query_string":         settings.IgnoreQueryString,
				"cache_rule":                  settings.CacheRule,
				"edge_cache_ttl":              settings.EdgeCacheTTLSeconds,
				"origin_offline":              settings.OriginOffline,
				"enable_cache_per_user_agent": settings.EnableCachePerUserAgent,
			}, nil
		},
	}
}

// cdnCacheEntryToMap renders a domain.CDNCacheEntry the way list/get
// cache-entry tools report it back to the caller.
func cdnCacheEntryToMap(entry domain.CDNCacheEntry) map[string]any {
	return map[string]any{
		"id":               entry.ID,
		"operation":        entry.Operation,
		"status":           entry.Status,
		"created_time":     entry.CreatedTime,
		"success_progress": entry.SuccessProgress,
	}
}

func listCDNCacheEntriesTool(uc *app.ListCDNCacheEntries) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_cdn_cache_entries",
		Description: "List the cache-clear (\"purge\") operations tracked against a Parspack CDN zone, most recent " +
			"first. Use this after purge_cdn_cache to find the id of the purge job it triggered, then " +
			"get_cdn_cache_entry to poll its progress. This is a fast operation.",
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

			entries, err := uc.Execute(ctx, app.ListCDNCacheEntriesInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(entries))
			for i, e := range entries {
				out[i] = cdnCacheEntryToMap(e)
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "entries": out}, nil
		},
	}
}

func purgeCDNCacheTool(uc *app.PurgeCDNCache) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "purge_cdn_cache",
		Description: "Clear all cached content at the edge for a Parspack CDN zone (the provider calls this " +
			"\"Destroy Cache\"). This is a fast operation: the call itself returns as soon as the provider accepts " +
			"the request, but the resulting purge does not report an id here — use list_cdn_cache_entries " +
			"afterward to find and track it, since content may still be repopulating at the edge for a short " +
			"time after this call returns.",
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

			if err := uc.Execute(ctx, app.PurgeCDNCacheInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID}); err != nil {
				return nil, err
			}
			return map[string]any{"purged": true, "zone_uuid": args.ZoneUUID}, nil
		},
	}
}

// cacheEntryIDArgs is the input of get_cdn_cache_entry.
type cacheEntryIDArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	ID       string `json:"id"`
}

func getCDNCacheEntryTool(uc *app.GetCDNCacheEntry) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["id"] = map[string]any{
		"type":        "string",
		"description": "The cache entry's id, as returned by list_cdn_cache_entries, e.g. \"OaB1AVVr\".",
	}

	return Tool{
		Name: "get_cdn_cache_entry",
		Description: "Get one cache-clear (\"purge\") operation of a Parspack CDN zone by id, including its " +
			"status and completion progress. Use this to poll the operation list_cdn_cache_entries surfaced after " +
			"purge_cdn_cache. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args cacheEntryIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			entry, err := uc.Execute(ctx, app.GetCDNCacheEntryInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, ID: args.ID,
			})
			if err != nil {
				return nil, err
			}
			out := cdnCacheEntryToMap(*entry)
			out["zone_uuid"] = args.ZoneUUID
			return out, nil
		},
	}
}
