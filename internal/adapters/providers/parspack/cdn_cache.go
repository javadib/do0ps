package parspack

import (
	"context"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Cache Management (issue #24), wired to the real CDN API. Base paths are
// confirmed against lines 1125-2072 of
// docs/api-specs/parspack-cdn.openapi.yaml's "Cache Management" tag,
// relative to Client.cdnBaseURL — the same zonesBasePath ("zones") this
// package's cdn.go already uses for zone/DNS endpoints.
//
// Every PUT here (cache/ttl, cache/rule, cache/user-agent) returns an empty
// "data" array on success, per the spec's documented 200 examples — there is
// nothing to decode, so each method echoes back the value it sent rather
// than re-fetching it. This mirrors cdn.go's UpdateDNSRecord, which does the
// same for the same reason.

// cacheTTLUpdateRequest is the body of PUT .../cache/ttl.
type cacheTTLUpdateRequest struct {
	EdgeCacheTTL int `json:"edge_cache_ttl"`
}

// UpdateCDNCacheTTL sets the edge cache TTL for a zone.
func (c *Client) UpdateCDNCacheTTL(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, ttlSeconds int) (*domain.CDNCacheTTLSetting, error) {
	body := cacheTTLUpdateRequest{EdgeCacheTTL: ttlSeconds}
	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+"/cache/ttl", body, nil); err != nil {
		return nil, fmt.Errorf("update cache TTL for zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNCacheTTLSetting{EdgeCacheTTLSeconds: ttlSeconds}, nil
}

// cacheRuleUpdateRequest is the body of PUT .../cache/rule.
type cacheRuleUpdateRequest struct {
	CacheRule string `json:"cache_rule"`
}

// UpdateCDNCacheRule sets the zone-wide cache rule (e.g. "cdn-smart-caching").
func (c *Client) UpdateCDNCacheRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, cacheRule string) (*domain.CDNCacheRuleSetting, error) {
	body := cacheRuleUpdateRequest{CacheRule: cacheRule}
	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+"/cache/rule", body, nil); err != nil {
		return nil, fmt.Errorf("update cache rule for zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNCacheRuleSetting{CacheRule: cacheRule}, nil
}

// cacheUserAgentUpdateRequest is the body of PUT .../cache/user-agent.
type cacheUserAgentUpdateRequest struct {
	Enabled bool `json:"enabled"`
}

// UpdateCDNCacheUserAgentSetting enables or disables caching content
// separately per User-Agent header for a zone (operationId
// updateCachePerUserAgent in the spec).
func (c *Client) UpdateCDNCacheUserAgentSetting(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (*domain.CDNCacheUserAgentSetting, error) {
	body := cacheUserAgentUpdateRequest{Enabled: enabled}
	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+"/cache/user-agent", body, nil); err != nil {
		return nil, fmt.Errorf("update cache per-user-agent setting for zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNCacheUserAgentSetting{Enabled: enabled}, nil
}

// cacheSettingsWire mirrors GET .../cache/settings' "data" object exactly.
type cacheSettingsWire struct {
	DeveloperMode           bool   `json:"developer_mode"`
	MaintenanceMode         bool   `json:"maintenance_mode"`
	IgnoreQueryString       bool   `json:"ignore_query_string"`
	CacheRule               string `json:"cache_rule"`
	EdgeCacheTTL            int    `json:"edge_cache_ttl"`
	OriginOffline           bool   `json:"origin_offline"`
	EnableCachePerUserAgent bool   `json:"enable_cache_per_user_agent"`
}

func toDomainCacheSettings(w cacheSettingsWire) domain.CDNCacheSettings {
	return domain.CDNCacheSettings{
		DeveloperMode:           w.DeveloperMode,
		MaintenanceMode:         w.MaintenanceMode,
		IgnoreQueryString:       w.IgnoreQueryString,
		CacheRule:               w.CacheRule,
		EdgeCacheTTLSeconds:     w.EdgeCacheTTL,
		OriginOffline:           w.OriginOffline,
		EnableCachePerUserAgent: w.EnableCachePerUserAgent,
	}
}

// GetCDNCacheSettings returns the aggregate cache configuration of a zone
// (operationId indexCacheSettings in the spec — "index" here means "show
// the one settings resource", not a list).
func (c *Client) GetCDNCacheSettings(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNCacheSettings, error) {
	var wire cacheSettingsWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+"/cache/settings", nil, &wire); err != nil {
		return nil, fmt.Errorf("get cache settings for zone %s: %w", zoneUUID, err)
	}
	settings := toDomainCacheSettings(wire)
	return &settings, nil
}

// cacheEntryWire mirrors one entry of GET .../cache and the object of
// GET .../cache/{id}.
type cacheEntryWire struct {
	ID              string `json:"id"`
	Operation       string `json:"operation"`
	Status          string `json:"status"`
	CreatedTime     string `json:"created_time"`
	SuccessProgress int    `json:"success_progress"`
}

func toDomainCacheEntry(w cacheEntryWire) domain.CDNCacheEntry {
	return domain.CDNCacheEntry{
		ID:              w.ID,
		Operation:       w.Operation,
		Status:          w.Status,
		CreatedTime:     w.CreatedTime,
		SuccessProgress: w.SuccessProgress,
	}
}

// ListCDNCacheEntries lists the cache-clear ("purge") operations tracked
// against a zone (operationId indexCache, "Index Cache" in the spec).
func (c *Client) ListCDNCacheEntries(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNCacheEntry, error) {
	var items []cacheEntryWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+"/cache", nil, &items); err != nil {
		return nil, fmt.Errorf("list cache entries for zone %s: %w", zoneUUID, err)
	}
	entries := make([]domain.CDNCacheEntry, len(items))
	for i := range items {
		entries[i] = toDomainCacheEntry(items[i])
	}
	return entries, nil
}

// PurgeCDNCache clears a zone's cached content at the edge. The spec calls
// this "Destroy Cache" (operationId destroyCache, DELETE .../cache); its
// response carries an empty "data" array, so — unlike DeleteCDNZone, whose
// resource identity is the zone UUID the caller already has — no id for the
// resulting purge job is returned here. A caller that wants to track its
// progress must call ListCDNCacheEntries afterward and match on the newest
// "Purge All" entry, or GetCDNCacheEntry once that entry's id is known.
func (c *Client) PurgeCDNCache(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", zonesBasePath+"/"+zoneUUID+"/cache", nil, nil); err != nil {
		return fmt.Errorf("purge cache for zone %s: %w", zoneUUID, err)
	}
	return nil
}

// GetCDNCacheEntry returns one cache-clear operation by id (operationId
// showCacheManagement, "Show Cache Management" in the spec).
func (c *Client) GetCDNCacheEntry(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNCacheEntry, error) {
	var wire cacheEntryWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+"/cache/"+id, nil, &wire); err != nil {
		return nil, fmt.Errorf("get cache entry %s for zone %s: %w", id, zoneUUID, err)
	}
	entry := toDomainCacheEntry(wire)
	return &entry, nil
}
