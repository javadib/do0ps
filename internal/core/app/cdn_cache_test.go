package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// fakeCDNCacheProvider implements only the local cdnCacheProvider interface
// cdn_cache.go's use cases depend on (house style: cdn_cache.go's provider
// field is a small local interface, not ports.ParspackProvider, since the
// central ports.go integration for these methods is a follow-up — see
// AGENTS.md's note in that file). It is intentionally separate from
// app_test.go's shared fakeProvider, which only implements
// ports.ParspackProvider and therefore has none of these methods.
type fakeCDNCacheProvider struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	ttlSetting             *domain.CDNCacheTTLSetting
	ttlErr                 error
	ruleSetting            *domain.CDNCacheRuleSetting
	ruleErr                error
	uaSetting              *domain.CDNCacheUserAgentSetting
	uaErr                  error
	settings               *domain.CDNCacheSettings
	settingsErr            error
	entries                []domain.CDNCacheEntry
	entriesErr             error
	purgeErr               error
	entry                  *domain.CDNCacheEntry
	entryErr               error

	gotZoneUUID string
	gotTTL      int
	gotRule     string
	gotEnabled  bool
	gotID       string
}

func (p *fakeCDNCacheProvider) UpdateCDNCacheTTL(_ context.Context, _ domain.ProviderCredentials, zoneUUID string, ttlSeconds int) (*domain.CDNCacheTTLSetting, error) {
	p.gotZoneUUID, p.gotTTL = zoneUUID, ttlSeconds
	if p.ttlErr != nil {
		return nil, p.ttlErr
	}
	if p.ttlSetting != nil {
		return p.ttlSetting, nil
	}
	return &domain.CDNCacheTTLSetting{EdgeCacheTTLSeconds: ttlSeconds}, nil
}

func (p *fakeCDNCacheProvider) UpdateCDNCacheRule(_ context.Context, _ domain.ProviderCredentials, zoneUUID, cacheRule string) (*domain.CDNCacheRuleSetting, error) {
	p.gotZoneUUID, p.gotRule = zoneUUID, cacheRule
	if p.ruleErr != nil {
		return nil, p.ruleErr
	}
	if p.ruleSetting != nil {
		return p.ruleSetting, nil
	}
	return &domain.CDNCacheRuleSetting{CacheRule: cacheRule}, nil
}

func (p *fakeCDNCacheProvider) UpdateCDNCacheUserAgentSetting(_ context.Context, _ domain.ProviderCredentials, zoneUUID string, enabled bool) (*domain.CDNCacheUserAgentSetting, error) {
	p.gotZoneUUID, p.gotEnabled = zoneUUID, enabled
	if p.uaErr != nil {
		return nil, p.uaErr
	}
	if p.uaSetting != nil {
		return p.uaSetting, nil
	}
	return &domain.CDNCacheUserAgentSetting{Enabled: enabled}, nil
}

func (p *fakeCDNCacheProvider) GetCDNCacheSettings(_ context.Context, _ domain.ProviderCredentials, zoneUUID string) (*domain.CDNCacheSettings, error) {
	p.gotZoneUUID = zoneUUID
	if p.settingsErr != nil {
		return nil, p.settingsErr
	}
	return p.settings, nil
}

func (p *fakeCDNCacheProvider) ListCDNCacheEntries(_ context.Context, _ domain.ProviderCredentials, zoneUUID string) ([]domain.CDNCacheEntry, error) {
	p.gotZoneUUID = zoneUUID
	if p.entriesErr != nil {
		return nil, p.entriesErr
	}
	return p.entries, nil
}

func (p *fakeCDNCacheProvider) PurgeCDNCache(_ context.Context, _ domain.ProviderCredentials, zoneUUID string) error {
	p.gotZoneUUID = zoneUUID
	return p.purgeErr
}

func (p *fakeCDNCacheProvider) GetCDNCacheEntry(_ context.Context, _ domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNCacheEntry, error) {
	p.gotZoneUUID, p.gotID = zoneUUID, id
	if p.entryErr != nil {
		return nil, p.entryErr
	}
	return p.entry, nil
}

func TestUpdateCDNCacheTTLSuccess(t *testing.T) {
	provider := &fakeCDNCacheProvider{}
	uc := app.NewUpdateCDNCacheTTL(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.UpdateCDNCacheTTLInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", TTLSeconds: 10800,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if setting.EdgeCacheTTLSeconds != 10800 {
		t.Errorf("EdgeCacheTTLSeconds = %d, want 10800", setting.EdgeCacheTTLSeconds)
	}
	if provider.gotZoneUUID != "zone-1" || provider.gotTTL != 10800 {
		t.Errorf("provider got zone %q ttl %d, want zone-1 10800", provider.gotZoneUUID, provider.gotTTL)
	}
}

func TestUpdateCDNCacheTTLInvalidTTL(t *testing.T) {
	uc := app.NewUpdateCDNCacheTTL(&inlineQueue{}, &fakeCDNCacheProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNCacheTTLInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", TTLSeconds: 42,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNCacheTTLMissingZoneUUID(t *testing.T) {
	uc := app.NewUpdateCDNCacheTTL(&inlineQueue{}, &fakeCDNCacheProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNCacheTTLInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, TTLSeconds: 3600})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNCacheRuleSuccess(t *testing.T) {
	provider := &fakeCDNCacheProvider{}
	uc := app.NewUpdateCDNCacheRule(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.UpdateCDNCacheRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", CacheRule: "cdn-smart-caching",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if setting.CacheRule != "cdn-smart-caching" {
		t.Errorf("CacheRule = %q, want cdn-smart-caching", setting.CacheRule)
	}
}

func TestUpdateCDNCacheRuleInvalidRule(t *testing.T) {
	uc := app.NewUpdateCDNCacheRule(&inlineQueue{}, &fakeCDNCacheProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNCacheRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", CacheRule: "not-a-real-rule",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNCacheUserAgentSettingSuccess(t *testing.T) {
	provider := &fakeCDNCacheProvider{}
	uc := app.NewUpdateCDNCacheUserAgentSetting(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.UpdateCDNCacheUserAgentSettingInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !setting.Enabled {
		t.Errorf("Enabled = %v, want true", setting.Enabled)
	}
}

func TestGetCDNCacheSettingsSuccess(t *testing.T) {
	provider := &fakeCDNCacheProvider{settings: &domain.CDNCacheSettings{
		CacheRule: "cdn-static-caching", EdgeCacheTTLSeconds: 3600, OriginOffline: true, EnableCachePerUserAgent: true,
	}}
	uc := app.NewGetCDNCacheSettings(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetCDNCacheSettingsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if settings.CacheRule != "cdn-static-caching" || settings.EdgeCacheTTLSeconds != 3600 {
		t.Errorf("settings = %+v, want cache_rule cdn-static-caching, edge_cache_ttl 3600", settings)
	}
}

func TestGetCDNCacheSettingsMissingZoneUUID(t *testing.T) {
	uc := app.NewGetCDNCacheSettings(&inlineQueue{}, &fakeCDNCacheProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNCacheSettingsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestListCDNCacheEntriesSuccess(t *testing.T) {
	provider := &fakeCDNCacheProvider{entries: []domain.CDNCacheEntry{
		{ID: "OaB1AVVr", Operation: "Purge All", Status: "none"},
	}}
	uc := app.NewListCDNCacheEntries(&inlineQueue{}, provider)

	entries, err := uc.Execute(context.Background(), app.ListCDNCacheEntriesInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "OaB1AVVr" {
		t.Errorf("entries = %+v, want a single entry with id OaB1AVVr", entries)
	}
}

func TestPurgeCDNCacheSuccess(t *testing.T) {
	provider := &fakeCDNCacheProvider{}
	uc := app.NewPurgeCDNCache(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.PurgeCDNCacheInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.gotZoneUUID != "zone-1" {
		t.Errorf("provider got zone %q, want zone-1", provider.gotZoneUUID)
	}
}

func TestPurgeCDNCacheProviderError(t *testing.T) {
	provider := &fakeCDNCacheProvider{purgeErr: fmt.Errorf("zone %s: %w", "zone-1", domain.ErrNotFound)}
	uc := app.NewPurgeCDNCache(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.PurgeCDNCacheInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetCDNCacheEntrySuccess(t *testing.T) {
	provider := &fakeCDNCacheProvider{entry: &domain.CDNCacheEntry{ID: "AbCdEfGh", Operation: "Purge All"}}
	uc := app.NewGetCDNCacheEntry(&inlineQueue{}, provider)

	entry, err := uc.Execute(context.Background(), app.GetCDNCacheEntryInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", ID: "AbCdEfGh"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if entry.ID != "AbCdEfGh" {
		t.Errorf("ID = %q, want AbCdEfGh", entry.ID)
	}
	if provider.gotID != "AbCdEfGh" {
		t.Errorf("provider got id %q, want AbCdEfGh", provider.gotID)
	}
}

func TestGetCDNCacheEntryMissingID(t *testing.T) {
	uc := app.NewGetCDNCacheEntry(&inlineQueue{}, &fakeCDNCacheProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNCacheEntryInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
