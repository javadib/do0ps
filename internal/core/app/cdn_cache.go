package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// Cache Management use cases (issue #24), scoped to a CDN zone like the rest
// of the CDN surface (AGENTS.md 4.1). Every operation here is FAST: the
// Parspack CDN API in docs/api-specs/parspack-cdn.openapi.yaml returns
// "{success, message, data}" synchronously for every Cache Management
// endpoint (lines 1125-2072) — none of the documented 200 responses carry a
// job/status/pending field to poll, so unlike CreateServer or CreateSnapshot
// there is no long-running provisioning state here to track through
// ports.JobRepository. Each use case therefore calls queue.Dispatch and
// returns synchronously, exactly like the CDN zone/DNS use cases in
// cdn_zone.go.
//

func validateZoneUUID(zoneUUID string) error {
	if zoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// UpdateCDNCacheTTLInput identifies the zone and the new edge cache TTL.
type UpdateCDNCacheTTLInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	TTLSeconds  int
}

// UpdateCDNCacheTTL is a fast operation (see this file's doc comment): the
// provider's response carries no data to poll, so the result returns within
// this call.
type UpdateCDNCacheTTL struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNCacheTTL builds the use case from its ports.
func NewUpdateCDNCacheTTL(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNCacheTTL {
	return &UpdateCDNCacheTTL{queue: queue, provider: provider}
}

// Execute validates the request and sets the zone's edge cache TTL.
func (uc *UpdateCDNCacheTTL) Execute(ctx context.Context, in UpdateCDNCacheTTLInput) (*domain.CDNCacheTTLSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}
	if !domain.ValidDNSRecordTTL(in.TTLSeconds) {
		return nil, fmt.Errorf("edge_cache_ttl %d is not one of the TTLs Parspack offers: %w", in.TTLSeconds, domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.UpdateCDNCacheTTL(ctx, in.Credentials, in.ZoneUUID, in.TTLSeconds)
		if err != nil {
			return nil, fmt.Errorf("updating cache TTL for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNCacheTTLSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding cache TTL setting: %w", err)
	}
	return &setting, nil
}

// UpdateCDNCacheRuleInput identifies the zone and the new cache rule.
type UpdateCDNCacheRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	CacheRule   string
}

// UpdateCDNCacheRule is a fast operation (see this file's doc comment): the
// provider's response carries no data to poll, so the result returns within
// this call.
type UpdateCDNCacheRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNCacheRule builds the use case from its ports.
func NewUpdateCDNCacheRule(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNCacheRule {
	return &UpdateCDNCacheRule{queue: queue, provider: provider}
}

// Execute validates the request and sets the zone's cache rule.
func (uc *UpdateCDNCacheRule) Execute(ctx context.Context, in UpdateCDNCacheRuleInput) (*domain.CDNCacheRuleSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}
	if !domain.ValidCDNCacheRule(in.CacheRule) {
		return nil, fmt.Errorf("cache_rule %q is not one of the rules Parspack offers: %w", in.CacheRule, domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.UpdateCDNCacheRule(ctx, in.Credentials, in.ZoneUUID, in.CacheRule)
		if err != nil {
			return nil, fmt.Errorf("updating cache rule for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNCacheRuleSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding cache rule setting: %w", err)
	}
	return &setting, nil
}

// UpdateCDNCacheUserAgentSettingInput identifies the zone and the new
// per-user-agent caching status.
type UpdateCDNCacheUserAgentSettingInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Enabled     bool
}

// UpdateCDNCacheUserAgentSetting is a fast operation (see this file's doc
// comment): the provider's response carries no data to poll, so the result
// returns within this call.
type UpdateCDNCacheUserAgentSetting struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNCacheUserAgentSetting builds the use case from its ports.
func NewUpdateCDNCacheUserAgentSetting(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNCacheUserAgentSetting {
	return &UpdateCDNCacheUserAgentSetting{queue: queue, provider: provider}
}

// Execute validates the request and sets the zone's per-user-agent caching
// status.
func (uc *UpdateCDNCacheUserAgentSetting) Execute(ctx context.Context, in UpdateCDNCacheUserAgentSettingInput) (*domain.CDNCacheUserAgentSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.UpdateCDNCacheUserAgentSetting(ctx, in.Credentials, in.ZoneUUID, in.Enabled)
		if err != nil {
			return nil, fmt.Errorf("updating cache per-user-agent setting for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNCacheUserAgentSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding cache per-user-agent setting: %w", err)
	}
	return &setting, nil
}

// GetCDNCacheSettingsInput identifies the zone whose aggregate cache
// configuration to look up.
type GetCDNCacheSettingsInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNCacheSettings is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNCacheSettings struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNCacheSettings builds the use case from its ports.
func NewGetCDNCacheSettings(queue ports.Queue, provider ports.ParspackProvider) *GetCDNCacheSettings {
	return &GetCDNCacheSettings{queue: queue, provider: provider}
}

// Execute returns the aggregate cache configuration of a zone.
func (uc *GetCDNCacheSettings) Execute(ctx context.Context, in GetCDNCacheSettingsInput) (*domain.CDNCacheSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		settings, err := uc.provider.GetCDNCacheSettings(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting cache settings for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(settings)
	})
	if err != nil {
		return nil, err
	}

	var settings domain.CDNCacheSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decoding cache settings: %w", err)
	}
	return &settings, nil
}

// ListCDNCacheEntriesInput identifies the zone whose cache-clear operations
// to list.
type ListCDNCacheEntriesInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// ListCDNCacheEntries is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type ListCDNCacheEntries struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListCDNCacheEntries builds the use case from its ports.
func NewListCDNCacheEntries(queue ports.Queue, provider ports.ParspackProvider) *ListCDNCacheEntries {
	return &ListCDNCacheEntries{queue: queue, provider: provider}
}

// Execute returns every cache-clear operation tracked against a zone.
func (uc *ListCDNCacheEntries) Execute(ctx context.Context, in ListCDNCacheEntriesInput) ([]domain.CDNCacheEntry, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		entries, err := uc.provider.ListCDNCacheEntries(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing cache entries for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(entries)
	})
	if err != nil {
		return nil, err
	}

	var entries []domain.CDNCacheEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("decoding cache entry list: %w", err)
	}
	return entries, nil
}

// PurgeCDNCacheInput identifies the zone whose cache to clear.
type PurgeCDNCacheInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// PurgeCDNCache is a fast operation (see this file's doc comment): the
// provider's response carries no data, so the call returns as soon as the
// provider accepts it. The resulting purge job's progress is not tracked
// through ports.JobRepository — use ListCDNCacheEntries or
// GetCDNCacheEntry afterward to check on it (ports.ParspackProvider's
// PurgeCDNCache doc comment).
type PurgeCDNCache struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewPurgeCDNCache builds the use case from its ports.
func NewPurgeCDNCache(queue ports.Queue, provider ports.ParspackProvider) *PurgeCDNCache {
	return &PurgeCDNCache{queue: queue, provider: provider}
}

// Execute clears the zone's cached content at the edge.
func (uc *PurgeCDNCache) Execute(ctx context.Context, in PurgeCDNCacheInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.PurgeCDNCache(ctx, in.Credentials, in.ZoneUUID); err != nil {
			return nil, fmt.Errorf("purging cache for zone %s: %w", in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// GetCDNCacheEntryInput identifies the zone and the cache-clear operation to
// look up.
type GetCDNCacheEntryInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	ID          string
}

// GetCDNCacheEntry is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNCacheEntry struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNCacheEntry builds the use case from its ports.
func NewGetCDNCacheEntry(queue ports.Queue, provider ports.ParspackProvider) *GetCDNCacheEntry {
	return &GetCDNCacheEntry{queue: queue, provider: provider}
}

// Execute returns one cache-clear operation of a zone by id.
func (uc *GetCDNCacheEntry) Execute(ctx context.Context, in GetCDNCacheEntryInput) (*domain.CDNCacheEntry, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		entry, err := uc.provider.GetCDNCacheEntry(ctx, in.Credentials, in.ZoneUUID, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting cache entry %s for zone %s: %w", in.ID, in.ZoneUUID, err)
		}
		return json.Marshal(entry)
	})
	if err != nil {
		return nil, err
	}

	var entry domain.CDNCacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, fmt.Errorf("decoding cache entry: %w", err)
	}
	return &entry, nil
}
