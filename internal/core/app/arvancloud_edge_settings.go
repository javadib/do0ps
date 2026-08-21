package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the Caching, Acceleration/Image Resize and Custom Pages
// use cases for ArvanCloud (issue #72): domain "edge-behavior settings" that
// don't fit the rule-engine pattern of AC5-AC8 or the routing-rule pattern
// of AC11 — see domain/arvancloud_edge_settings.go's package comment. Every
// one of them is a fast operation (ports.ArvanCloudProvider, AGENTS.md 4.3):
// each dispatches onto the queue and blocks for the result within the same
// tool call — PurgeArvanCloudCache included, despite its response wording
// ("Successfully queued purge"); see ports.ArvanCloudProvider's own doc
// comment for why.
//
// arvanCloudDomainInput (arvancloud_rules.go) is reused here rather than
// redefined: every use case below is scoped to exactly one domain by name,
// the same shape that type already covers.

// --- Caching ---------------------------------------------------------------

// validateArvanCloudCachingSettingsInput checks CachingSettings' four
// enum-typed fields, all optional: an empty field is left unvalidated (it
// means "let the provider apply its own default"), matching every other
// optional-enum validator in this package (e.g.
// validateArvanCloudPageRuleMatching's URLType check).
func validateArvanCloudCachingSettingsInput(s domain.ArvanCloudCachingSettings) error {
	if s.CacheStatus != "" && !domain.ValidArvanCloudPageRuleCacheLevel(string(s.CacheStatus)) {
		return fmt.Errorf("cache_status %q is not one of off/uri/query_string: %w", s.CacheStatus, domain.ErrInvalidInput)
	}
	if s.CachePage200 != "" && !domain.ValidArvanCloudCacheTTL(string(s.CachePage200)) {
		return fmt.Errorf("cache_page_200 %q is not one of the accepted TTL values: %w", s.CachePage200, domain.ErrInvalidInput)
	}
	if s.CachePageAny != "" && !domain.ValidArvanCloudCacheTTL(string(s.CachePageAny)) {
		return fmt.Errorf("cache_page_any %q is not one of the accepted TTL values: %w", s.CachePageAny, domain.ErrInvalidInput)
	}
	if s.CacheBrowser != "" && !domain.ValidArvanCloudCacheBrowserTTL(string(s.CacheBrowser)) {
		return fmt.Errorf("cache_browser %q is not one of the accepted TTL values or \"default\": %w", s.CacheBrowser, domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudCachingSettingsInput identifies the domain whose caching
// settings to look up.
type GetArvanCloudCachingSettingsInput = arvanCloudDomainInput

// GetArvanCloudCachingSettings is a fast operation.
type GetArvanCloudCachingSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudCachingSettings builds the use case from its ports.
func NewGetArvanCloudCachingSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudCachingSettings {
	return &GetArvanCloudCachingSettings{queue: queue, provider: provider}
}

// Execute returns the domain's current cache-behavior settings.
func (uc *GetArvanCloudCachingSettings) Execute(ctx context.Context, in GetArvanCloudCachingSettingsInput) (*domain.ArvanCloudCachingSettings, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		settings, err := uc.provider.GetArvanCloudCachingSettings(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud caching settings for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(settings)
	})
	if err != nil {
		return nil, err
	}

	var settings domain.ArvanCloudCachingSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decoding arvancloud caching settings: %w", err)
	}
	return &settings, nil
}

// UpdateArvanCloudCachingSettingsInput identifies the domain and its new
// cache-behavior settings.
type UpdateArvanCloudCachingSettingsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Settings    domain.ArvanCloudCachingSettings
}

// UpdateArvanCloudCachingSettings is a fast operation.
type UpdateArvanCloudCachingSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudCachingSettings builds the use case from its ports.
func NewUpdateArvanCloudCachingSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudCachingSettings {
	return &UpdateArvanCloudCachingSettings{queue: queue, provider: provider}
}

// Execute validates the request and updates the settings. The endpoint's
// response carries no data, so Execute returns only an error; a caller
// wanting the settings' state afterward calls GetArvanCloudCachingSettings.
func (uc *UpdateArvanCloudCachingSettings) Execute(ctx context.Context, in UpdateArvanCloudCachingSettingsInput) error {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return err
	}
	if err := validateArvanCloudCachingSettingsInput(in.Settings); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UpdateArvanCloudCachingSettings(ctx, in.Credentials, in.Domain, in.Settings); err != nil {
			return nil, fmt.Errorf("updating arvancloud caching settings for domain %q: %w", in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Cache purge -----------------------------------------------------------

// validateArvanCloudCachePurgeInput checks CachingPurge's required-when
// relationships: Mode is required and must be one of
// domain.ValidArvanCloudCachePurgeMode's values; URLs is required (1-50
// entries) when Mode is "individual"; Tags is required (1-100 entries, each
// at most 32 ASCII characters) when Mode is "tags" — all per the spec's own
// CachingPurge schema constraints.
func validateArvanCloudCachePurgeInput(purge domain.ArvanCloudCachePurgeRequest) error {
	if !domain.ValidArvanCloudCachePurgeMode(string(purge.Mode)) {
		return fmt.Errorf("purge mode %q is not one of \"all\", \"individual\" or \"tags\": %w", purge.Mode, domain.ErrInvalidInput)
	}
	switch purge.Mode {
	case domain.ArvanCloudCachePurgeIndividual:
		if len(purge.URLs) == 0 {
			return fmt.Errorf("purge_urls is required when purge mode is \"individual\": %w", domain.ErrInvalidInput)
		}
		if len(purge.URLs) > 50 {
			return fmt.Errorf("purge_urls may contain at most 50 entries, got %d: %w", len(purge.URLs), domain.ErrInvalidInput)
		}
	case domain.ArvanCloudCachePurgeTags:
		if len(purge.Tags) == 0 {
			return fmt.Errorf("purge_tags is required when purge mode is \"tags\": %w", domain.ErrInvalidInput)
		}
		if len(purge.Tags) > 100 {
			return fmt.Errorf("purge_tags may contain at most 100 entries, got %d: %w", len(purge.Tags), domain.ErrInvalidInput)
		}
		for _, tag := range purge.Tags {
			if len(tag) > 32 {
				return fmt.Errorf("purge_tags entry %q is longer than 32 characters: %w", tag, domain.ErrInvalidInput)
			}
		}
	}
	return nil
}

// PurgeArvanCloudCacheInput identifies the domain and what to purge.
type PurgeArvanCloudCacheInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Purge       domain.ArvanCloudCachePurgeRequest
}

// PurgeArvanCloudCache is a fast operation. See
// ports.ArvanCloudProvider's own doc comment for why, despite the endpoint's
// "queued" response wording.
type PurgeArvanCloudCache struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewPurgeArvanCloudCache builds the use case from its ports.
func NewPurgeArvanCloudCache(queue ports.Queue, provider ports.ArvanCloudProvider) *PurgeArvanCloudCache {
	return &PurgeArvanCloudCache{queue: queue, provider: provider}
}

// Execute validates the request and triggers the purge.
func (uc *PurgeArvanCloudCache) Execute(ctx context.Context, in PurgeArvanCloudCacheInput) error {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return err
	}
	if err := validateArvanCloudCachePurgeInput(in.Purge); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.PurgeArvanCloudCache(ctx, in.Credentials, in.Domain, in.Purge); err != nil {
			return nil, fmt.Errorf("purging arvancloud cache for domain %q: %w", in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Purge tags --------------------------------------------------------

// ListArvanCloudPurgeTagsInput identifies the domain whose purge-tag history
// to list.
type ListArvanCloudPurgeTagsInput = arvanCloudDomainInput

// ListArvanCloudPurgeTags is a fast operation.
type ListArvanCloudPurgeTags struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudPurgeTags builds the use case from its ports.
func NewListArvanCloudPurgeTags(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudPurgeTags {
	return &ListArvanCloudPurgeTags{queue: queue, provider: provider}
}

// Execute returns the domain's previously-purged cache tag history.
func (uc *ListArvanCloudPurgeTags) Execute(ctx context.Context, in ListArvanCloudPurgeTagsInput) ([]domain.ArvanCloudPurgeTag, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		tags, err := uc.provider.ListArvanCloudPurgeTags(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud purge tags for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(tags)
	})
	if err != nil {
		return nil, err
	}

	var tags []domain.ArvanCloudPurgeTag
	if err := json.Unmarshal(raw, &tags); err != nil {
		return nil, fmt.Errorf("decoding arvancloud purge tag list: %w", err)
	}
	return tags, nil
}

// DeleteArvanCloudPurgeTagInput identifies the domain and the tag to remove.
type DeleteArvanCloudPurgeTagInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Tag         string
}

// DeleteArvanCloudPurgeTag is a fast operation.
type DeleteArvanCloudPurgeTag struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudPurgeTag builds the use case from its ports.
func NewDeleteArvanCloudPurgeTag(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudPurgeTag {
	return &DeleteArvanCloudPurgeTag{queue: queue, provider: provider}
}

// Execute deletes the tag.
func (uc *DeleteArvanCloudPurgeTag) Execute(ctx context.Context, in DeleteArvanCloudPurgeTagInput) error {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return err
	}
	if in.Tag == "" {
		return fmt.Errorf("tag is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudPurgeTag(ctx, in.Credentials, in.Domain, in.Tag); err != nil {
			return nil, fmt.Errorf("deleting arvancloud purge tag %q for domain %q: %w", in.Tag, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Image Resize ------------------------------------------------------

// validateArvanCloudImageResizeSettingsInput checks ImageResizeSettings'
// two enum-typed fields.
func validateArvanCloudImageResizeSettingsInput(s domain.ArvanCloudImageResizeSettings) error {
	if !domain.ValidArvanCloudImageResizeStatus(string(s.Status)) {
		return fmt.Errorf("status %q is not one of \"on\" or \"off\": %w", s.Status, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudImageResizeMode(string(s.Mode)) {
		return fmt.Errorf("mode %q is not one of freely/short-side/long-side: %w", s.Mode, domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudImageResizeSettingsInput identifies the domain whose
// image-resize settings to look up.
type GetArvanCloudImageResizeSettingsInput = arvanCloudDomainInput

// GetArvanCloudImageResizeSettings is a fast operation.
type GetArvanCloudImageResizeSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudImageResizeSettings builds the use case from its ports.
func NewGetArvanCloudImageResizeSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudImageResizeSettings {
	return &GetArvanCloudImageResizeSettings{queue: queue, provider: provider}
}

// Execute returns the domain's current image-resize settings.
func (uc *GetArvanCloudImageResizeSettings) Execute(ctx context.Context, in GetArvanCloudImageResizeSettingsInput) (*domain.ArvanCloudImageResizeSettings, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudImageResizeSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudImageResizeSettings, error) {
		found, err := uc.provider.GetArvanCloudImageResizeSettings(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud image-resize settings for domain %q: %w", in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudImageResizeSettingsInput identifies the domain and its new
// image-resize settings.
type UpdateArvanCloudImageResizeSettingsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Settings    domain.ArvanCloudImageResizeSettings
}

// UpdateArvanCloudImageResizeSettings is a fast operation.
type UpdateArvanCloudImageResizeSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudImageResizeSettings builds the use case from its ports.
func NewUpdateArvanCloudImageResizeSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudImageResizeSettings {
	return &UpdateArvanCloudImageResizeSettings{queue: queue, provider: provider}
}

// Execute validates the request and updates the settings, returning them as
// stored afterward.
func (uc *UpdateArvanCloudImageResizeSettings) Execute(ctx context.Context, in UpdateArvanCloudImageResizeSettingsInput) (*domain.ArvanCloudImageResizeSettings, error) {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudImageResizeSettingsInput(in.Settings); err != nil {
		return nil, err
	}

	return dispatchArvanCloudImageResizeSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudImageResizeSettings, error) {
		updated, err := uc.provider.UpdateArvanCloudImageResizeSettings(ctx, in.Credentials, in.Domain, in.Settings)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud image-resize settings for domain %q: %w", in.Domain, err)
		}
		return updated, nil
	})
}

// --- Acceleration --------------------------------------------------------

// validateArvanCloudAccelerationSettingsInput checks AccelerationSettings'
// Status/Extensions fields, reusing the full 3-value ValidArvanCloudAccelerationStatus
// (which still accepts "inherit") rather than a standalone-endpoint-narrowed
// 2-value check — the same choice rules.go's use cases already made for
// PageRule.acceleration/PageRuleDiff.acceleration (see
// arvancloud_rules.go's validateArvanCloudPageRuleOther): one Go type/
// validator pair covers every context this field appears in, and the
// provider itself rejects "inherit" on the standalone endpoint with a 422
// if it is ever sent there.
func validateArvanCloudAccelerationSettingsInput(s domain.ArvanCloudAccelerationSettings) error {
	if s.Status != "" && !domain.ValidArvanCloudAccelerationStatus(string(s.Status)) {
		return fmt.Errorf("acceleration status %q is not one of inherit/on/off: %w", s.Status, domain.ErrInvalidInput)
	}
	for _, ext := range s.Extensions {
		if !domain.ValidArvanCloudAccelerationExtension(string(ext)) {
			return fmt.Errorf("acceleration extension %q is not one of css/gif/jpeg/js/png: %w", ext, domain.ErrInvalidInput)
		}
	}
	return nil
}

// GetArvanCloudAccelerationSettingsInput identifies the domain whose
// acceleration settings to look up.
type GetArvanCloudAccelerationSettingsInput = arvanCloudDomainInput

// GetArvanCloudAccelerationSettings is a fast operation.
type GetArvanCloudAccelerationSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudAccelerationSettings builds the use case from its ports.
func NewGetArvanCloudAccelerationSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudAccelerationSettings {
	return &GetArvanCloudAccelerationSettings{queue: queue, provider: provider}
}

// Execute returns the domain's current acceleration settings.
func (uc *GetArvanCloudAccelerationSettings) Execute(ctx context.Context, in GetArvanCloudAccelerationSettingsInput) (*domain.ArvanCloudAccelerationSettings, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudAccelerationSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudAccelerationSettings, error) {
		found, err := uc.provider.GetArvanCloudAccelerationSettings(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud acceleration settings for domain %q: %w", in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudAccelerationSettingsInput identifies the domain and its
// new acceleration settings.
type UpdateArvanCloudAccelerationSettingsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Settings    domain.ArvanCloudAccelerationSettings
}

// UpdateArvanCloudAccelerationSettings is a fast operation.
type UpdateArvanCloudAccelerationSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudAccelerationSettings builds the use case from its
// ports.
func NewUpdateArvanCloudAccelerationSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudAccelerationSettings {
	return &UpdateArvanCloudAccelerationSettings{queue: queue, provider: provider}
}

// Execute validates the request and updates the settings, returning them as
// stored afterward.
func (uc *UpdateArvanCloudAccelerationSettings) Execute(ctx context.Context, in UpdateArvanCloudAccelerationSettingsInput) (*domain.ArvanCloudAccelerationSettings, error) {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudAccelerationSettingsInput(in.Settings); err != nil {
		return nil, err
	}

	return dispatchArvanCloudAccelerationSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudAccelerationSettings, error) {
		updated, err := uc.provider.UpdateArvanCloudAccelerationSettings(ctx, in.Credentials, in.Domain, in.Settings)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud acceleration settings for domain %q: %w", in.Domain, err)
		}
		return updated, nil
	})
}

// --- Custom Pages ----------------------------------------------------------

// ListArvanCloudCustomPagesInput identifies the domain whose custom pages
// to list.
type ListArvanCloudCustomPagesInput = arvanCloudDomainInput

// ListArvanCloudCustomPages is a fast operation.
type ListArvanCloudCustomPages struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudCustomPages builds the use case from its ports.
func NewListArvanCloudCustomPages(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudCustomPages {
	return &ListArvanCloudCustomPages{queue: queue, provider: provider}
}

// Execute returns the domain's whole named-slot custom-pages object.
func (uc *ListArvanCloudCustomPages) Execute(ctx context.Context, in ListArvanCloudCustomPagesInput) (*domain.ArvanCloudCustomPages, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		pages, err := uc.provider.ListArvanCloudCustomPages(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud custom pages for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(pages)
	})
	if err != nil {
		return nil, err
	}

	var pages domain.ArvanCloudCustomPages
	if err := json.Unmarshal(raw, &pages); err != nil {
		return nil, fmt.Errorf("decoding arvancloud custom pages: %w", err)
	}
	return &pages, nil
}

// validateArvanCloudCustomPageUpdateInput checks UpdateArvanCloudCustomPage's
// request: Page is required and must be one of
// domain.ValidArvanCloudCustomPageName's nine values; Type, when given, must
// satisfy domain.ValidArvanCloudCustomPageType; and the field the chosen
// Type actually needs is required — URL for "url", file content for "file"
// — per the spec's own CustomPageUpdate description.
func validateArvanCloudCustomPageUpdateInput(u domain.ArvanCloudCustomPageUpdate) error {
	if u.Page == "" {
		return fmt.Errorf("page is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudCustomPageName(string(u.Page)) {
		return fmt.Errorf("page %q is not one of the nine named custom-page slots: %w", u.Page, domain.ErrInvalidInput)
	}
	if u.Type != "" && !domain.ValidArvanCloudCustomPageType(string(u.Type)) {
		return fmt.Errorf("type %q is not one of off/url/file: %w", u.Type, domain.ErrInvalidInput)
	}
	if u.Type == domain.ArvanCloudCustomPageTypeURL && u.URL == "" {
		return fmt.Errorf("url is required when type is \"url\": %w", domain.ErrInvalidInput)
	}
	if u.Type == domain.ArvanCloudCustomPageTypeFile && len(u.FileContent) == 0 {
		return fmt.Errorf("file content is required when type is \"file\": %w", domain.ErrInvalidInput)
	}
	return nil
}

// UpdateArvanCloudCustomPageInput identifies the domain and the slot update.
type UpdateArvanCloudCustomPageInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Update      domain.ArvanCloudCustomPageUpdate
}

// UpdateArvanCloudCustomPage is a fast operation.
type UpdateArvanCloudCustomPage struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudCustomPage builds the use case from its ports.
func NewUpdateArvanCloudCustomPage(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudCustomPage {
	return &UpdateArvanCloudCustomPage{queue: queue, provider: provider}
}

// Execute validates the request and updates the named slot. The endpoint's
// response carries no data, so Execute returns only an error; a caller
// wanting the slot's state afterward calls ListArvanCloudCustomPages.
func (uc *UpdateArvanCloudCustomPage) Execute(ctx context.Context, in UpdateArvanCloudCustomPageInput) error {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return err
	}
	if err := validateArvanCloudCustomPageUpdateInput(in.Update); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UpdateArvanCloudCustomPage(ctx, in.Credentials, in.Domain, in.Update); err != nil {
			return nil, fmt.Errorf("updating arvancloud custom page %q for domain %q: %w", in.Update.Page, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// arvanCloudCustomPageFileIDInput is embedded by every use case below that
// is scoped to exactly one custom-page file by domain + id.
type arvanCloudCustomPageFileIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	FileID      string
}

func (in arvanCloudCustomPageFileIDInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.FileID == "" {
		return fmt.Errorf("file_id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudCustomPageFileInput identifies the custom-page file to look
// up.
type GetArvanCloudCustomPageFileInput = arvanCloudCustomPageFileIDInput

// GetArvanCloudCustomPageFile is a fast operation.
type GetArvanCloudCustomPageFile struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudCustomPageFile builds the use case from its ports.
func NewGetArvanCloudCustomPageFile(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudCustomPageFile {
	return &GetArvanCloudCustomPageFile{queue: queue, provider: provider}
}

// Execute returns one custom-page file's details, including its HTML
// content.
func (uc *GetArvanCloudCustomPageFile) Execute(ctx context.Context, in GetArvanCloudCustomPageFileInput) (*domain.ArvanCloudCustomPageFile, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		found, err := uc.provider.GetArvanCloudCustomPageFile(ctx, in.Credentials, in.Domain, in.FileID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud custom page file %q for domain %q: %w", in.FileID, in.Domain, err)
		}
		return json.Marshal(found)
	})
	if err != nil {
		return nil, err
	}

	var found domain.ArvanCloudCustomPageFile
	if err := json.Unmarshal(raw, &found); err != nil {
		return nil, fmt.Errorf("decoding arvancloud custom page file: %w", err)
	}
	return &found, nil
}

// UpdateArvanCloudCustomPageFileInput identifies the custom-page file to
// update. Active is nil to leave the file's active flag untouched;
// FileContent, when non-empty, replaces the file's content.
type UpdateArvanCloudCustomPageFileInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	FileID      string
	Active      *bool
	FileName    string
	FileContent []byte
}

// UpdateArvanCloudCustomPageFile is a fast operation.
type UpdateArvanCloudCustomPageFile struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudCustomPageFile builds the use case from its ports.
func NewUpdateArvanCloudCustomPageFile(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudCustomPageFile {
	return &UpdateArvanCloudCustomPageFile{queue: queue, provider: provider}
}

// Execute updates the file. At least one of Active/FileContent must be
// given — an empty update has nothing for the provider to change. The
// endpoint's response carries no data, so Execute returns only an error; a
// caller wanting the file's state afterward calls
// GetArvanCloudCustomPageFile.
func (uc *UpdateArvanCloudCustomPageFile) Execute(ctx context.Context, in UpdateArvanCloudCustomPageFileInput) error {
	if err := (arvanCloudCustomPageFileIDInput{Credentials: in.Credentials, Domain: in.Domain, FileID: in.FileID}).validate(); err != nil {
		return err
	}
	if in.Active == nil && len(in.FileContent) == 0 {
		return fmt.Errorf("at least one of active or file content is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UpdateArvanCloudCustomPageFile(ctx, in.Credentials, in.Domain, in.FileID, in.Active, in.FileName, in.FileContent); err != nil {
			return nil, fmt.Errorf("updating arvancloud custom page file %q for domain %q: %w", in.FileID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// DeleteArvanCloudCustomPageFileInput identifies the custom-page file to
// remove.
type DeleteArvanCloudCustomPageFileInput = arvanCloudCustomPageFileIDInput

// DeleteArvanCloudCustomPageFile is a fast operation. Unlike this package's
// other Delete use cases, an already-absent file is NOT normalized to
// success: the provider's own rejection of deleting a currently-active file
// (a 400) needs to reach the caller as-is, and a plain 404 here is
// propagated too rather than guessed at — see
// ports.ArvanCloudProvider.DeleteArvanCloudCustomPageFile's own doc comment.
type DeleteArvanCloudCustomPageFile struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudCustomPageFile builds the use case from its ports.
func NewDeleteArvanCloudCustomPageFile(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudCustomPageFile {
	return &DeleteArvanCloudCustomPageFile{queue: queue, provider: provider}
}

// Execute deletes the file.
func (uc *DeleteArvanCloudCustomPageFile) Execute(ctx context.Context, in DeleteArvanCloudCustomPageFileInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudCustomPageFile(ctx, in.Credentials, in.Domain, in.FileID); err != nil {
			return nil, fmt.Errorf("deleting arvancloud custom page file %q for domain %q: %w", in.FileID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Dispatch helpers -------------------------------------------------

// dispatchArvanCloudImageResizeSettings runs fn on the queue and decodes its
// result back into a *domain.ArvanCloudImageResizeSettings.
func dispatchArvanCloudImageResizeSettings(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudImageResizeSettings, error),
) (*domain.ArvanCloudImageResizeSettings, error) {
	raw, err := queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudImageResizeSettings
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud image-resize settings: %w", err)
	}
	return &result, nil
}

// dispatchArvanCloudAccelerationSettings runs fn on the queue and decodes
// its result back into a *domain.ArvanCloudAccelerationSettings.
func dispatchArvanCloudAccelerationSettings(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudAccelerationSettings, error),
) (*domain.ArvanCloudAccelerationSettings, error) {
	raw, err := queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudAccelerationSettings
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud acceleration settings: %w", err)
	}
	return &result, nil
}
