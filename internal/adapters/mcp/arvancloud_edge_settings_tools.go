package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud Caching, Acceleration/Image Resize and Custom Pages tools
// (issue #72) — see domain/arvancloud_edge_settings.go's package comment for
// what each resource is. All fast operations (AGENTS.md 4.3): every tool
// below returns its result within the call, with no operation_id to poll
// afterward — including purge_arvancloud_cache, despite the underlying
// endpoint's own "queued" response wording (see
// ports.ArvanCloudProvider's doc comment for why).
//
// arvanCloudAccelerationToMap (arvancloud_rules_tools.go, AC11) is reused
// below for get/update_arvancloud_acceleration_settings rather than a second
// render helper, matching how domain.ArvanCloudAccelerationSettings itself
// is reused rather than redefined.

// --- Caching -----------------------------------------------------------

func arvanCloudCachingSettingsToMap(s domain.ArvanCloudCachingSettings) map[string]any {
	return map[string]any{
		"cache_developer_mode":    s.CacheDeveloperMode,
		"cache_consistent_uptime": s.CacheConsistentUptime,
		"cache_max_size_bytes":    s.CacheMaxSizeBytes,
		"cache_status":            string(s.CacheStatus),
		"cache_page_200":          string(s.CachePage200),
		"cache_page_any":          string(s.CachePageAny),
		"cache_browser":           string(s.CacheBrowser),
		"cache_scheme":            s.CacheScheme,
		"cache_ignore_sc":         s.CacheIgnoreSC,
		"cache_cookie":            s.CacheCookie,
		"cache_args":              s.CacheArgs,
		"cache_arg":               s.CacheArg,
	}
}

// arvanCloudCacheTTLProperty is the shared 29-value TTL enum
// (domain.ArvanCloudCacheTTL) used by cache_page_200/cache_page_any, plus
// their "default" sentinel counterpart for cache_browser.
func arvanCloudCacheTTLProperty(description string, allowDefault bool) map[string]any {
	values := []string{
		"0s", "1s", "2s", "3s", "4s", "5s", "6s", "7s", "8s", "9s", "10s", "30s",
		"1m", "3m", "5m", "10m", "30m", "45m",
		"1h", "3h", "5h", "10h", "12h", "24h",
		"3d", "7d", "10d", "15d", "30d",
	}
	if allowDefault {
		values = append([]string{"default"}, values...)
	}
	return map[string]any{"type": "string", "enum": values, "description": description}
}

func getArvanCloudCachingSettingsTool(uc *app.GetArvanCloudCachingSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_caching_settings",
		Description: "Get a domain's cache-behavior configuration: developer mode, max cacheable size, cache mode " +
			"(off/uri/query_string), TTLs for 200/any responses and the browser Cache-Control header, and which " +
			"cookies/query-string arguments vary the cache. This is a fast operation.",
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

			found, err := uc.Execute(ctx, app.GetArvanCloudCachingSettingsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudCachingSettingsToMap(*found), nil
		},
	}
}

func updateArvanCloudCachingSettingsTool(uc *app.UpdateArvanCloudCachingSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["cache_developer_mode"] = map[string]any{"type": "boolean", "description": "Disable caching for the requesting IP only, useful while developing against a cached origin."}
	props["cache_consistent_uptime"] = map[string]any{"type": "boolean", "description": "Enable the provider's \"consistent uptime\" caching mode."}
	props["cache_max_size_bytes"] = map[string]any{
		"type":        "integer",
		"description": "Maximum size of cacheable content, in BYTES, e.g. 104857600 for 100 MB. Provider maximum is 2147483648 (2 GB). Omit to keep the provider default.",
	}
	props["cache_status"] = map[string]any{
		"type": "string", "enum": []string{"off", "uri", "query_string"},
		"description": "Domain-wide cache mode. Omit to keep the provider default.",
	}
	props["cache_page_200"] = arvanCloudCacheTTLProperty("How long to cache a 200 response. Omit to keep the provider default (\"30m\").", false)
	props["cache_page_any"] = arvanCloudCacheTTLProperty("How long to cache any response. Omit to keep the provider default (\"0s\").", false)
	props["cache_browser"] = arvanCloudCacheTTLProperty("Browser-facing Cache-Control TTL. Omit to keep the provider default (\"default\").", true)
	props["cache_scheme"] = map[string]any{"type": "boolean", "description": "Deprecated by the provider but still settable: whether to consider the request scheme (HTTP/HTTPS) in caching."}
	props["cache_ignore_sc"] = map[string]any{"type": "boolean", "description": "Ignore the default Set-Cookie-header caching behavior."}
	props["cache_cookie"] = map[string]any{"type": "string", "description": "Comma-separated cookie names to vary the cache on, e.g. \"session,lang\". Omit for none."}
	props["cache_args"] = map[string]any{"type": "boolean", "description": "Vary the cache by query-string arguments."}
	props["cache_arg"] = map[string]any{"type": "string", "description": "\"&\"-separated query-string argument names to vary the cache on, e.g. \"filter&sort\". Omit for none/all, depending on cache_args."}

	return Tool{
		Name: "update_arvancloud_caching_settings",
		Description: "Update a domain's cache-behavior configuration. Every field is optional — an omitted field " +
			"keeps the provider's own current/default value. This is a fast operation; call get_arvancloud_caching_settings " +
			"afterward to confirm.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				CacheDeveloperMode    bool   `json:"cache_developer_mode"`
				CacheConsistentUptime bool   `json:"cache_consistent_uptime"`
				CacheMaxSizeBytes     int64  `json:"cache_max_size_bytes"`
				CacheStatus           string `json:"cache_status"`
				CachePage200          string `json:"cache_page_200"`
				CachePageAny          string `json:"cache_page_any"`
				CacheBrowser          string `json:"cache_browser"`
				CacheScheme           bool   `json:"cache_scheme"`
				CacheIgnoreSC         bool   `json:"cache_ignore_sc"`
				CacheCookie           string `json:"cache_cookie"`
				CacheArgs             bool   `json:"cache_args"`
				CacheArg              string `json:"cache_arg"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.UpdateArvanCloudCachingSettingsInput{
				Credentials: args.domain(), Domain: args.Domain,
				Settings: domain.ArvanCloudCachingSettings{
					CacheDeveloperMode:    args.CacheDeveloperMode,
					CacheConsistentUptime: args.CacheConsistentUptime,
					CacheMaxSizeBytes:     args.CacheMaxSizeBytes,
					CacheStatus:           domain.ArvanCloudPageRuleCacheLevel(args.CacheStatus),
					CachePage200:          domain.ArvanCloudCacheTTL(args.CachePage200),
					CachePageAny:          domain.ArvanCloudCacheTTL(args.CachePageAny),
					CacheBrowser:          domain.ArvanCloudCacheTTL(args.CacheBrowser),
					CacheScheme:           args.CacheScheme,
					CacheIgnoreSC:         args.CacheIgnoreSC,
					CacheCookie:           args.CacheCookie,
					CacheArgs:             args.CacheArgs,
					CacheArg:              args.CacheArg,
				},
			}); err != nil {
				return nil, err
			}
			return map[string]any{"domain": args.Domain}, nil
		},
	}
}

func purgeArvanCloudCacheTool(uc *app.PurgeArvanCloudCache) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["purge"] = map[string]any{
		"type": "string",
		"enum": []string{"all", "individual", "tags"},
		"description": "REQUIRED. \"all\" purges every cached object for the domain; \"individual\" purges only " +
			"purge_urls; \"tags\" (deprecated, Professional plan or higher) purges only purge_tags.",
	}
	props["purge_urls"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string", "format": "uri"},
		"description": "URLs to purge. REQUIRED (1-50 entries) when purge is \"individual\"; ignored otherwise.",
	}
	props["purge_tags"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Cache tags to purge, each at most 32 ASCII characters. REQUIRED (1-100 entries) when purge is \"tags\"; ignored otherwise. Deprecated by the provider.",
	}

	return Tool{
		Name: "purge_arvancloud_cache",
		Description: "Purge a domain's cached content, either the whole site, specific URLs, or specific cache " +
			"tags. This is a fast operation: the call itself returns immediately, even though the actual purge " +
			"propagates asynchronously afterward on ArvanCloud's own side, with nothing exposed here to poll for it.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "purge"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Purge     string   `json:"purge"`
				PurgeURLs []string `json:"purge_urls"`
				PurgeTags []string `json:"purge_tags"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.PurgeArvanCloudCacheInput{
				Credentials: args.domain(), Domain: args.Domain,
				Purge: domain.ArvanCloudCachePurgeRequest{
					Mode: domain.ArvanCloudCachePurgeMode(args.Purge), URLs: args.PurgeURLs, Tags: args.PurgeTags,
				},
			}); err != nil {
				return nil, err
			}
			return map[string]any{"purged": true, "domain": args.Domain}, nil
		},
	}
}

func arvanCloudPurgeTagToMap(t domain.ArvanCloudPurgeTag) map[string]any {
	return map[string]any{"tag": t.Tag, "created_at": t.CreatedAt}
}

func listArvanCloudPurgeTagsTool(uc *app.ListArvanCloudPurgeTags) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name:        "list_arvancloud_purge_tags",
		Description: "List a domain's previously-purged cache tag history. This is a fast operation.",
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

			tags, err := uc.Execute(ctx, app.ListArvanCloudPurgeTagsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(tags))
			for i, t := range tags {
				out[i] = arvanCloudPurgeTagToMap(t)
			}
			return map[string]any{"tags": out}, nil
		},
	}
}

func deleteArvanCloudPurgeTagTool(uc *app.DeleteArvanCloudPurgeTag) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["tag"] = map[string]any{"type": "string", "description": "The tag value to remove from the purge-tag history."}

	return Tool{
		Name:        "delete_arvancloud_purge_tag",
		Description: "Remove one tag from a domain's purge-tag history. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "tag"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Tag string `json:"tag"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudPurgeTagInput{Credentials: args.domain(), Domain: args.Domain, Tag: args.Tag}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "tag": args.Tag}, nil
		},
	}
}

// --- Image Resize --------------------------------------------------------

func arvanCloudImageResizeSettingsToMap(s domain.ArvanCloudImageResizeSettings) map[string]any {
	return map[string]any{
		"status":     string(s.Status),
		"height_by":  s.HeightBy,
		"width_by":   s.WidthBy,
		"mode":       string(s.Mode),
		"mode_by":    s.ModeBy,
		"quality_by": s.QualityBy,
	}
}

func getArvanCloudImageResizeSettingsTool(uc *app.GetArvanCloudImageResizeSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name:        "get_arvancloud_image_resize_settings",
		Description: "Get a domain's image-resize (image transformation on the fly) settings. This is a fast operation.",
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

			found, err := uc.Execute(ctx, app.GetArvanCloudImageResizeSettingsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudImageResizeSettingsToMap(*found), nil
		},
	}
}

func updateArvanCloudImageResizeSettingsTool(uc *app.UpdateArvanCloudImageResizeSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["status"] = map[string]any{"type": "string", "enum": []string{"on", "off"}, "description": "Turn image resizing on or off for this domain. Defaults to \"off\" when omitted."}
	props["height_by"] = map[string]any{"type": "string", "description": "Query-string argument read for the target height, e.g. \"height\" (the default)."}
	props["width_by"] = map[string]any{"type": "string", "description": "Query-string argument read for the target width, e.g. \"width\" (the default)."}
	props["mode"] = map[string]any{
		"type": "string", "enum": []string{"freely", "short-side", "long-side"},
		"description": "Whether to preserve aspect ratio based on the short/long side of the image after transforming. Defaults to \"freely\" when omitted.",
	}
	props["mode_by"] = map[string]any{"type": "string", "description": "Query-string variable name for overriding mode per request (acceptable request-time values: \"f\", \"s\", \"l\")."}
	props["quality_by"] = map[string]any{"type": "string", "description": "Query-string variable name for setting image quality per request (acceptable request-time values: 1-100)."}

	return Tool{
		Name: "update_arvancloud_image_resize_settings",
		Description: "Update a domain's image-resize settings. This is a fast operation: the updated settings are " +
			"returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Status    string `json:"status"`
				HeightBy  string `json:"height_by"`
				WidthBy   string `json:"width_by"`
				Mode      string `json:"mode"`
				ModeBy    string `json:"mode_by"`
				QualityBy string `json:"quality_by"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudImageResizeSettingsInput{
				Credentials: args.domain(), Domain: args.Domain,
				Settings: domain.ArvanCloudImageResizeSettings{
					Status: domain.ArvanCloudImageResizeStatus(args.Status), HeightBy: args.HeightBy, WidthBy: args.WidthBy,
					Mode: domain.ArvanCloudImageResizeMode(args.Mode), ModeBy: args.ModeBy, QualityBy: args.QualityBy,
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudImageResizeSettingsToMap(*updated), nil
		},
	}
}

// --- Acceleration --------------------------------------------------------

func getArvanCloudAccelerationSettingsTool(uc *app.GetArvanCloudAccelerationSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name:        "get_arvancloud_acceleration_settings",
		Description: "Get a domain's front-end acceleration settings (which static-file extensions are accelerated). This is a fast operation.",
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

			found, err := uc.Execute(ctx, app.GetArvanCloudAccelerationSettingsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudAccelerationToMap(*found), nil
		},
	}
}

func updateArvanCloudAccelerationSettingsTool(uc *app.UpdateArvanCloudAccelerationSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["status"] = map[string]any{
		"type": "string", "enum": []string{"on", "off"},
		"description": "Turn acceleration on or off for this domain. Only \"on\"/\"off\" are accepted here " +
			"(\"inherit\" is meaningful only for a Page Rule's own acceleration override, not this domain-wide setting).",
	}
	props["extensions"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string", "enum": []string{"css", "gif", "jpeg", "js", "png"}},
		"description": "File extensions acceleration applies to, e.g. [\"css\",\"js\"]. Empty means none.",
	}

	return Tool{
		Name: "update_arvancloud_acceleration_settings",
		Description: "Update a domain's front-end acceleration settings. This is a fast operation: the updated " +
			"settings are returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "status"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Status     string   `json:"status"`
				Extensions []string `json:"extensions"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			extensions := make([]domain.ArvanCloudAccelerationExtension, len(args.Extensions))
			for i, e := range args.Extensions {
				extensions[i] = domain.ArvanCloudAccelerationExtension(e)
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudAccelerationSettingsInput{
				Credentials: args.domain(), Domain: args.Domain,
				Settings: domain.ArvanCloudAccelerationSettings{Status: domain.ArvanCloudAccelerationStatus(args.Status), Extensions: extensions},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudAccelerationToMap(*updated), nil
		},
	}
}

// --- Custom Pages ----------------------------------------------------------

func arvanCloudCustomPageFileToMap(f domain.ArvanCloudCustomPageFile) map[string]any {
	return map[string]any{"id": f.ID, "name": f.Name, "active": f.Active}
}

func arvanCloudCustomPageToMap(p domain.ArvanCloudCustomPage) map[string]any {
	files := make([]map[string]any, len(p.Files))
	for i, f := range p.Files {
		files[i] = arvanCloudCustomPageFileToMap(f)
	}
	return map[string]any{
		"status_code": int(p.StatusCode),
		"type":        string(p.Type),
		"url":         p.URL,
		"files":       files,
	}
}

func arvanCloudCustomPagesToMap(pages domain.ArvanCloudCustomPages) map[string]any {
	return map[string]any{
		"under_construction":  arvanCloudCustomPageToMap(pages.UnderConstruction),
		"firewall_error":      arvanCloudCustomPageToMap(pages.FirewallError),
		"waf_protection":      arvanCloudCustomPageToMap(pages.WAFProtection),
		"rate_limit_exceeded": arvanCloudCustomPageToMap(pages.RateLimitExceeded),
		"secure_link_expired": arvanCloudCustomPageToMap(pages.SecureLinkExpired),
		"secure_link_invalid": arvanCloudCustomPageToMap(pages.SecureLinkInvalid),
		"error_500":           arvanCloudCustomPageToMap(pages.Error500),
		"ddos_js":             arvanCloudCustomPageToMap(pages.DdosJS),
		"ddos_captcha":        arvanCloudCustomPageToMap(pages.DdosCaptcha),
	}
}

func arvanCloudCustomPageNameProperty() map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			"under_construction", "firewall_error", "waf_protection", "rate_limit_exceeded",
			"secure_link_expired", "secure_link_invalid", "error_500", "ddos_js", "ddos_captcha",
		},
		"description": "Which of the domain's nine named custom-page slots to target. ddos_js/ddos_captcha may only be used with type \"file\".",
	}
}

func listArvanCloudCustomPagesTool(uc *app.ListArvanCloudCustomPages) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_custom_pages",
		Description: "Get all nine of a domain's named custom-page slots (what is served instead of ArvanCloud's " +
			"own default content for a WAF block, a rate-limit block, an expired secure link, and so on). This is a fast operation.",
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

			pages, err := uc.Execute(ctx, app.ListArvanCloudCustomPagesInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudCustomPagesToMap(*pages), nil
		},
	}
}

func updateArvanCloudCustomPagesTool(uc *app.UpdateArvanCloudCustomPage) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["page"] = arvanCloudCustomPageNameProperty()
	props["type"] = map[string]any{
		"type": "string", "enum": []string{"off", "url", "file"},
		"description": "REQUIRED. \"off\" disables this slot (ArvanCloud's own default content is served instead); " +
			"\"url\" redirects to the given url; \"file\" serves an uploaded HTML file given as file_content.",
	}
	props["url"] = map[string]any{"type": "string", "format": "uri", "description": "The redirect target. REQUIRED when type is \"url\"; ignored otherwise."}
	props["file_name"] = map[string]any{"type": "string", "description": "A human-readable name for the uploaded file, e.g. \"blocked.html\". Only used when type is \"file\"."}
	props["file_content"] = map[string]any{
		"type":        "string",
		"description": "The full HTML content to upload as a new file for this slot. REQUIRED when type is \"file\" — each call with type \"file\" creates a NEW file entry rather than editing an existing one; ignored otherwise.",
	}

	return Tool{
		Name: "update_arvancloud_custom_pages",
		Description: "Update ONE of a domain's nine named custom-page slots (select it with page) — despite the " +
			"plural tool name, this call targets exactly one slot per call, matching the provider's own API shape. " +
			"This is a fast operation; call list_arvancloud_custom_pages afterward to confirm.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "page", "type"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Page        string `json:"page"`
				Type        string `json:"type"`
				URL         string `json:"url"`
				FileName    string `json:"file_name"`
				FileContent string `json:"file_content"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.UpdateArvanCloudCustomPageInput{
				Credentials: args.domain(), Domain: args.Domain,
				Update: domain.ArvanCloudCustomPageUpdate{
					Page: domain.ArvanCloudCustomPageName(args.Page), Type: domain.ArvanCloudCustomPageType(args.Type),
					URL: args.URL, FileName: args.FileName, FileContent: []byte(args.FileContent),
				},
			}); err != nil {
				return nil, err
			}
			return map[string]any{"domain": args.Domain, "page": args.Page}, nil
		},
	}
}

func arvanCloudCustomPageFileIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The custom-page file's provider-assigned ID (a UUID), as returned in list_arvancloud_custom_pages' files or update_arvancloud_custom_pages.",
	}
}

func getArvanCloudCustomPageFileTool(uc *app.GetArvanCloudCustomPageFile) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["file_id"] = arvanCloudCustomPageFileIDProperty()

	return Tool{
		Name:        "get_arvancloud_custom_page_file",
		Description: "Get one custom-page file's details, including its HTML content. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "file_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				FileID string `json:"file_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudCustomPageFileInput{Credentials: args.domain(), Domain: args.Domain, FileID: args.FileID})
			if err != nil {
				return nil, err
			}
			out := arvanCloudCustomPageFileToMap(*found)
			out["value"] = found.Value
			return out, nil
		},
	}
}

func updateArvanCloudCustomPageFileTool(uc *app.UpdateArvanCloudCustomPageFile) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["file_id"] = arvanCloudCustomPageFileIDProperty()
	props["active"] = map[string]any{"type": "boolean", "description": "Set which file is served for its slot. Omit to leave unchanged."}
	props["file_name"] = map[string]any{"type": "string", "description": "A human-readable name for the new content, e.g. \"blocked.html\". Only used when file_content is given."}
	props["file_content"] = map[string]any{"type": "string", "description": "New HTML content to replace this file's content with. Omit to leave the content unchanged."}

	return Tool{
		Name: "update_arvancloud_custom_page_file",
		Description: "Update one already-uploaded custom-page file in place: mark it active/inactive and/or " +
			"replace its HTML content. Give at least one of active/file_content. This is a fast operation; call " +
			"get_arvancloud_custom_page_file afterward to confirm.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "file_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				FileID      string `json:"file_id"`
				Active      *bool  `json:"active"`
				FileName    string `json:"file_name"`
				FileContent string `json:"file_content"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.UpdateArvanCloudCustomPageFileInput{
				Credentials: args.domain(), Domain: args.Domain, FileID: args.FileID,
				Active: args.Active, FileName: args.FileName, FileContent: []byte(args.FileContent),
			}); err != nil {
				return nil, err
			}
			return map[string]any{"domain": args.Domain, "file_id": args.FileID}, nil
		},
	}
}

func deleteArvanCloudCustomPageFileTool(uc *app.DeleteArvanCloudCustomPageFile) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["file_id"] = arvanCloudCustomPageFileIDProperty()

	return Tool{
		Name: "delete_arvancloud_custom_page_file",
		Description: "Permanently delete one custom-page file by ID. The provider refuses to delete a file that is " +
			"currently active for its slot. This is a fast operation and cannot be undone.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "file_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				FileID string `json:"file_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudCustomPageFileInput{Credentials: args.domain(), Domain: args.Domain, FileID: args.FileID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "file_id": args.FileID}, nil
		},
	}
}
