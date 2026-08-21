package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// fakeArvanCloudEdgeSettingsProvider embeds the port so a test only needs to
// override the methods it actually exercises, the same pattern as
// fakeArvanCloudDdosProvider.
type fakeArvanCloudEdgeSettingsProvider struct {
	ports.ArvanCloudProvider

	cachingSettings        domain.ArvanCloudCachingSettings
	updatedCachingSettings domain.ArvanCloudCachingSettings
	updatedCachingDomain   string

	purgedDomain string
	purgedPurge  domain.ArvanCloudCachePurgeRequest

	purgeTags []domain.ArvanCloudPurgeTag

	deletedPurgeTagDomain string
	deletedPurgeTag       string

	imageResizeSettings        domain.ArvanCloudImageResizeSettings
	updatedImageResizeSettings domain.ArvanCloudImageResizeSettings

	accelerationSettings        domain.ArvanCloudAccelerationSettings
	updatedAccelerationSettings domain.ArvanCloudAccelerationSettings

	customPages domain.ArvanCloudCustomPages

	updatedCustomPage domain.ArvanCloudCustomPageUpdate

	customPageFile domain.ArvanCloudCustomPageFile

	updatedFileID      string
	updatedFileActive  *bool
	updatedFileName    string
	updatedFileContent []byte

	deletedFileID string
}

func (p *fakeArvanCloudEdgeSettingsProvider) GetArvanCloudCachingSettings(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudCachingSettings, error) {
	settings := p.cachingSettings
	return &settings, nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) UpdateArvanCloudCachingSettings(_ context.Context, _ domain.ProviderCredentials, domainName string, settings domain.ArvanCloudCachingSettings) error {
	p.updatedCachingDomain = domainName
	p.updatedCachingSettings = settings
	return nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) PurgeArvanCloudCache(_ context.Context, _ domain.ProviderCredentials, domainName string, purge domain.ArvanCloudCachePurgeRequest) error {
	p.purgedDomain = domainName
	p.purgedPurge = purge
	return nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) ListArvanCloudPurgeTags(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudPurgeTag, error) {
	return p.purgeTags, nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) DeleteArvanCloudPurgeTag(_ context.Context, _ domain.ProviderCredentials, domainName, tag string) error {
	p.deletedPurgeTagDomain = domainName
	p.deletedPurgeTag = tag
	return nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) GetArvanCloudImageResizeSettings(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudImageResizeSettings, error) {
	settings := p.imageResizeSettings
	return &settings, nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) UpdateArvanCloudImageResizeSettings(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.ArvanCloudImageResizeSettings) (*domain.ArvanCloudImageResizeSettings, error) {
	p.updatedImageResizeSettings = settings
	return &settings, nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) GetArvanCloudAccelerationSettings(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudAccelerationSettings, error) {
	settings := p.accelerationSettings
	return &settings, nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) UpdateArvanCloudAccelerationSettings(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.ArvanCloudAccelerationSettings) (*domain.ArvanCloudAccelerationSettings, error) {
	p.updatedAccelerationSettings = settings
	return &settings, nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) ListArvanCloudCustomPages(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudCustomPages, error) {
	pages := p.customPages
	return &pages, nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) UpdateArvanCloudCustomPage(_ context.Context, _ domain.ProviderCredentials, _ string, update domain.ArvanCloudCustomPageUpdate) error {
	p.updatedCustomPage = update
	return nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) GetArvanCloudCustomPageFile(context.Context, domain.ProviderCredentials, string, string) (*domain.ArvanCloudCustomPageFile, error) {
	file := p.customPageFile
	return &file, nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) UpdateArvanCloudCustomPageFile(_ context.Context, _ domain.ProviderCredentials, _ string, fileID string, active *bool, fileName string, fileContent []byte) error {
	p.updatedFileID = fileID
	p.updatedFileActive = active
	p.updatedFileName = fileName
	p.updatedFileContent = fileContent
	return nil
}

func (p *fakeArvanCloudEdgeSettingsProvider) DeleteArvanCloudCustomPageFile(_ context.Context, _ domain.ProviderCredentials, _ string, fileID string) error {
	p.deletedFileID = fileID
	return nil
}

// --- Caching ---------------------------------------------------------------

func TestGetArvanCloudCachingSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{cachingSettings: domain.ArvanCloudCachingSettings{
		CacheDeveloperMode: true, CacheStatus: domain.ArvanCloudPageRuleCacheLevelURI,
	}}
	uc := app.NewGetArvanCloudCachingSettings(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetArvanCloudCachingSettingsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !settings.CacheDeveloperMode || settings.CacheStatus != domain.ArvanCloudPageRuleCacheLevelURI {
		t.Errorf("settings = %+v, want the fake's settings", settings)
	}
}

func TestGetArvanCloudCachingSettingsMissingDomain(t *testing.T) {
	uc := app.NewGetArvanCloudCachingSettings(&inlineQueue{}, &fakeArvanCloudEdgeSettingsProvider{})
	if _, err := uc.Execute(context.Background(), app.GetArvanCloudCachingSettingsInput{Credentials: validArvanCloudCreds()}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateArvanCloudCachingSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{}
	uc := app.NewUpdateArvanCloudCachingSettings(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.UpdateArvanCloudCachingSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudCachingSettings{CacheStatus: domain.ArvanCloudPageRuleCacheLevelQueryString, CachePage200: domain.ArvanCloudCacheTTL1h},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.updatedCachingDomain != "example.com" || provider.updatedCachingSettings.CachePage200 != domain.ArvanCloudCacheTTL1h {
		t.Errorf("provider saw %q / %+v, want example.com / the given settings", provider.updatedCachingDomain, provider.updatedCachingSettings)
	}
}

func TestUpdateArvanCloudCachingSettingsInvalidEnum(t *testing.T) {
	uc := app.NewUpdateArvanCloudCachingSettings(&inlineQueue{}, &fakeArvanCloudEdgeSettingsProvider{})

	tests := []domain.ArvanCloudCachingSettings{
		{CacheStatus: "bogus"},
		{CachePage200: "bogus"},
		{CachePageAny: "bogus"},
		{CacheBrowser: "bogus"},
	}
	for _, settings := range tests {
		err := uc.Execute(context.Background(), app.UpdateArvanCloudCachingSettingsInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com", Settings: settings,
		})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("Execute(%+v) error = %v, want domain.ErrInvalidInput", settings, err)
		}
	}
}

// --- Cache purge -----------------------------------------------------------

func TestPurgeArvanCloudCacheSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{}
	uc := app.NewPurgeArvanCloudCache(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.PurgeArvanCloudCacheInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Purge: domain.ArvanCloudCachePurgeRequest{Mode: domain.ArvanCloudCachePurgeAll},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.purgedDomain != "example.com" || provider.purgedPurge.Mode != domain.ArvanCloudCachePurgeAll {
		t.Errorf("provider saw %q / %+v", provider.purgedDomain, provider.purgedPurge)
	}
}

// TestPurgeArvanCloudCacheRequiredWhenFields proves the required-when
// relationships CachingPurge's schema declares: purge_urls is required for
// "individual", purge_tags for "tags".
func TestPurgeArvanCloudCacheRequiredWhenFields(t *testing.T) {
	uc := app.NewPurgeArvanCloudCache(&inlineQueue{}, &fakeArvanCloudEdgeSettingsProvider{})

	tests := []struct {
		name  string
		purge domain.ArvanCloudCachePurgeRequest
	}{
		{"invalid mode", domain.ArvanCloudCachePurgeRequest{Mode: "bogus"}},
		{"individual without urls", domain.ArvanCloudCachePurgeRequest{Mode: domain.ArvanCloudCachePurgeIndividual}},
		{"tags without tags", domain.ArvanCloudCachePurgeRequest{Mode: domain.ArvanCloudCachePurgeTags}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := uc.Execute(context.Background(), app.PurgeArvanCloudCacheInput{
				Credentials: validArvanCloudCreds(), Domain: "example.com", Purge: tt.purge,
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

// --- Purge tags --------------------------------------------------------

func TestListArvanCloudPurgeTagsSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{purgeTags: []domain.ArvanCloudPurgeTag{{Tag: "tag-a"}, {Tag: "tag-b"}}}
	uc := app.NewListArvanCloudPurgeTags(&inlineQueue{}, provider)

	tags, err := uc.Execute(context.Background(), app.ListArvanCloudPurgeTagsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(tags) != 2 {
		t.Errorf("tags = %+v, want 2 entries", tags)
	}
}

func TestDeleteArvanCloudPurgeTagSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{}
	uc := app.NewDeleteArvanCloudPurgeTag(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudPurgeTagInput{Credentials: validArvanCloudCreds(), Domain: "example.com", Tag: "tag-a"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedPurgeTagDomain != "example.com" || provider.deletedPurgeTag != "tag-a" {
		t.Errorf("provider saw %q / %q", provider.deletedPurgeTagDomain, provider.deletedPurgeTag)
	}
}

func TestDeleteArvanCloudPurgeTagMissingTag(t *testing.T) {
	uc := app.NewDeleteArvanCloudPurgeTag(&inlineQueue{}, &fakeArvanCloudEdgeSettingsProvider{})
	err := uc.Execute(context.Background(), app.DeleteArvanCloudPurgeTagInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// --- Image Resize --------------------------------------------------------

func TestGetArvanCloudImageResizeSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{imageResizeSettings: domain.ArvanCloudImageResizeSettings{Status: domain.ArvanCloudImageResizeStatusOn}}
	uc := app.NewGetArvanCloudImageResizeSettings(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetArvanCloudImageResizeSettingsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if settings.Status != domain.ArvanCloudImageResizeStatusOn {
		t.Errorf("settings.Status = %q, want %q", settings.Status, domain.ArvanCloudImageResizeStatusOn)
	}
}

func TestUpdateArvanCloudImageResizeSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{}
	uc := app.NewUpdateArvanCloudImageResizeSettings(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudImageResizeSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudImageResizeSettings{Status: domain.ArvanCloudImageResizeStatusOff, Mode: domain.ArvanCloudImageResizeModeShortSide},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.Mode != domain.ArvanCloudImageResizeModeShortSide {
		t.Errorf("updated.Mode = %q, want %q", updated.Mode, domain.ArvanCloudImageResizeModeShortSide)
	}
}

func TestUpdateArvanCloudImageResizeSettingsInvalidEnum(t *testing.T) {
	uc := app.NewUpdateArvanCloudImageResizeSettings(&inlineQueue{}, &fakeArvanCloudEdgeSettingsProvider{})

	tests := []domain.ArvanCloudImageResizeSettings{
		{Status: "bogus"},
		{Status: "inherit"}, // valid for PageRuleImageResize, NOT the base ImageResize schema
		{Mode: "bogus"},
	}
	for _, settings := range tests {
		_, err := uc.Execute(context.Background(), app.UpdateArvanCloudImageResizeSettingsInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com", Settings: settings,
		})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("Execute(%+v) error = %v, want domain.ErrInvalidInput", settings, err)
		}
	}
}

// --- Acceleration --------------------------------------------------------

func TestGetArvanCloudAccelerationSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{accelerationSettings: domain.ArvanCloudAccelerationSettings{Status: domain.ArvanCloudAccelerationOn}}
	uc := app.NewGetArvanCloudAccelerationSettings(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetArvanCloudAccelerationSettingsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if settings.Status != domain.ArvanCloudAccelerationOn {
		t.Errorf("settings.Status = %q, want %q", settings.Status, domain.ArvanCloudAccelerationOn)
	}
}

func TestUpdateArvanCloudAccelerationSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{}
	uc := app.NewUpdateArvanCloudAccelerationSettings(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudAccelerationSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudAccelerationSettings{Status: domain.ArvanCloudAccelerationOff, Extensions: []domain.ArvanCloudAccelerationExtension{domain.ArvanCloudAccelerationExtensionCSS}},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.Status != domain.ArvanCloudAccelerationOff {
		t.Errorf("updated.Status = %q, want %q", updated.Status, domain.ArvanCloudAccelerationOff)
	}
}

func TestUpdateArvanCloudAccelerationSettingsInvalidExtension(t *testing.T) {
	uc := app.NewUpdateArvanCloudAccelerationSettings(&inlineQueue{}, &fakeArvanCloudEdgeSettingsProvider{})
	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudAccelerationSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudAccelerationSettings{Status: domain.ArvanCloudAccelerationOn, Extensions: []domain.ArvanCloudAccelerationExtension{"bogus"}},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// --- Custom Pages ----------------------------------------------------------

func TestListArvanCloudCustomPagesSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{customPages: domain.ArvanCloudCustomPages{
		FirewallError: domain.ArvanCloudCustomPage{Type: domain.ArvanCloudCustomPageTypeURL, URL: "https://example.com/blocked"},
	}}
	uc := app.NewListArvanCloudCustomPages(&inlineQueue{}, provider)

	pages, err := uc.Execute(context.Background(), app.ListArvanCloudCustomPagesInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if pages.FirewallError.URL != "https://example.com/blocked" {
		t.Errorf("FirewallError.URL = %q, want the fake's value", pages.FirewallError.URL)
	}
}

func TestUpdateArvanCloudCustomPageSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{}
	uc := app.NewUpdateArvanCloudCustomPage(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.UpdateArvanCloudCustomPageInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Update: domain.ArvanCloudCustomPageUpdate{Page: domain.ArvanCloudCustomPageFirewallError, Type: domain.ArvanCloudCustomPageTypeURL, URL: "https://example.com/blocked"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.updatedCustomPage.Page != domain.ArvanCloudCustomPageFirewallError {
		t.Errorf("provider saw page %q", provider.updatedCustomPage.Page)
	}
}

// TestUpdateArvanCloudCustomPageRequiredWhenFields proves the required-when
// relationships CustomPageUpdate implies: url required for type "url", file
// content required for type "file".
func TestUpdateArvanCloudCustomPageRequiredWhenFields(t *testing.T) {
	uc := app.NewUpdateArvanCloudCustomPage(&inlineQueue{}, &fakeArvanCloudEdgeSettingsProvider{})

	tests := []struct {
		name   string
		update domain.ArvanCloudCustomPageUpdate
	}{
		{"missing page", domain.ArvanCloudCustomPageUpdate{Type: domain.ArvanCloudCustomPageTypeOff}},
		{"invalid page", domain.ArvanCloudCustomPageUpdate{Page: "bogus", Type: domain.ArvanCloudCustomPageTypeOff}},
		{"invalid type", domain.ArvanCloudCustomPageUpdate{Page: domain.ArvanCloudCustomPageError500, Type: "bogus"}},
		{"url type without url", domain.ArvanCloudCustomPageUpdate{Page: domain.ArvanCloudCustomPageFirewallError, Type: domain.ArvanCloudCustomPageTypeURL}},
		{"file type without content", domain.ArvanCloudCustomPageUpdate{Page: domain.ArvanCloudCustomPageDdosJS, Type: domain.ArvanCloudCustomPageTypeFile}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := uc.Execute(context.Background(), app.UpdateArvanCloudCustomPageInput{
				Credentials: validArvanCloudCreds(), Domain: "example.com", Update: tt.update,
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

func TestGetArvanCloudCustomPageFileSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{customPageFile: domain.ArvanCloudCustomPageFile{ID: "file-1", Value: "<html></html>"}}
	uc := app.NewGetArvanCloudCustomPageFile(&inlineQueue{}, provider)

	found, err := uc.Execute(context.Background(), app.GetArvanCloudCustomPageFileInput{Credentials: validArvanCloudCreds(), Domain: "example.com", FileID: "file-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if found.Value != "<html></html>" {
		t.Errorf("found.Value = %q, want the fake's value", found.Value)
	}
}

func TestGetArvanCloudCustomPageFileMissingFileID(t *testing.T) {
	uc := app.NewGetArvanCloudCustomPageFile(&inlineQueue{}, &fakeArvanCloudEdgeSettingsProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudCustomPageFileInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateArvanCloudCustomPageFileSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{}
	uc := app.NewUpdateArvanCloudCustomPageFile(&inlineQueue{}, provider)

	active := true
	err := uc.Execute(context.Background(), app.UpdateArvanCloudCustomPageFileInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", FileID: "file-1", Active: &active,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.updatedFileID != "file-1" || provider.updatedFileActive == nil || !*provider.updatedFileActive {
		t.Errorf("provider saw fileID=%q active=%v", provider.updatedFileID, provider.updatedFileActive)
	}
}

// TestUpdateArvanCloudCustomPageFileRequiresSomeChange proves an update with
// neither active nor file content given is rejected: it has nothing for the
// provider to change.
func TestUpdateArvanCloudCustomPageFileRequiresSomeChange(t *testing.T) {
	uc := app.NewUpdateArvanCloudCustomPageFile(&inlineQueue{}, &fakeArvanCloudEdgeSettingsProvider{})
	err := uc.Execute(context.Background(), app.UpdateArvanCloudCustomPageFileInput{Credentials: validArvanCloudCreds(), Domain: "example.com", FileID: "file-1"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteArvanCloudCustomPageFileSuccess(t *testing.T) {
	provider := &fakeArvanCloudEdgeSettingsProvider{}
	uc := app.NewDeleteArvanCloudCustomPageFile(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudCustomPageFileInput{Credentials: validArvanCloudCreds(), Domain: "example.com", FileID: "file-1"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedFileID != "file-1" {
		t.Errorf("provider saw deleted fileID = %q, want file-1", provider.deletedFileID)
	}
}

func TestDeleteArvanCloudCustomPageFileMissingFileID(t *testing.T) {
	uc := app.NewDeleteArvanCloudCustomPageFile(&inlineQueue{}, &fakeArvanCloudEdgeSettingsProvider{})
	err := uc.Execute(context.Background(), app.DeleteArvanCloudCustomPageFileInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}
