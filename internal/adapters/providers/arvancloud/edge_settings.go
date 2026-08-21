package arvancloud

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Caching, Acceleration/Image Resize and Custom Pages (issue #72), wired to
// the real CDN API. Base paths are confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Caching" and "Custom Pages" tags
// (Acceleration/Image Resize share the "Acceleration" tag with AC11's
// PageRule sub-objects), relative to domainPath (defined in domain.go).
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real ArvanCloud responses correctly. Nothing above
// the adapter boundary ever sees them — every method here translates to/from
// internal/core/domain types. Acceleration reuses accelerationWire/
// toAccelerationDomain/accelerationRequestBody from rules.go rather than a
// second copy — same package, same wire shape (see
// domain.ArvanCloudAccelerationSettings' own doc comment).

const (
	cachingPathSuffix      = "/caching"
	purgeTagsPathSuffix    = "/purge-tags"
	imageResizePathSuffix  = "/image-resize"
	accelerationPathSuffix = "/acceleration"
	customPagesPathSuffix  = "/custom-pages"
)

func cachingPath(domainName string) string      { return domainPath(domainName) + cachingPathSuffix }
func cachingPurgePath(domainName string) string { return cachingPath(domainName) + "/purge" }
func purgeTagsPath(domainName string) string    { return domainPath(domainName) + purgeTagsPathSuffix }
func imageResizePath(domainName string) string  { return domainPath(domainName) + imageResizePathSuffix }
func accelerationPath(domainName string) string {
	return domainPath(domainName) + accelerationPathSuffix
}
func customPagesPath(domainName string) string { return domainPath(domainName) + customPagesPathSuffix }
func customPageFilePath(domainName, fileID string) string {
	return customPagesPath(domainName) + "/" + fileID
}

// --- Caching ---------------------------------------------------------------

// cachingSettingsWire mirrors CacheSettings, decode-only for the same reason
// as ddosSettingsWire: the update request is built separately
// (cachingSettingsRequestBody) as a plain map, so an explicit `false` on a
// boolean toggle reaches the provider rather than being dropped by
// encoding/json's omitempty.
type cachingSettingsWire struct {
	CacheDeveloperMode    bool   `json:"cache_developer_mode"`
	CacheConsistentUptime bool   `json:"cache_consistent_uptime"`
	CacheMaxSize          int64  `json:"cache_max_size"`
	CacheStatus           string `json:"cache_status,omitempty"`
	CachePage200          string `json:"cache_page_200,omitempty"`
	CachePageAny          string `json:"cache_page_any,omitempty"`
	CacheBrowser          string `json:"cache_browser,omitempty"`
	CacheScheme           bool   `json:"cache_scheme"`
	CacheIgnoreSC         bool   `json:"cache_ignore_sc"`
	CacheCookie           string `json:"cache_cookie,omitempty"`
	CacheArgs             bool   `json:"cache_args"`
	CacheArg              string `json:"cache_arg,omitempty"`
}

func toCachingSettingsDomain(w cachingSettingsWire) domain.ArvanCloudCachingSettings {
	return domain.ArvanCloudCachingSettings{
		CacheDeveloperMode:    w.CacheDeveloperMode,
		CacheConsistentUptime: w.CacheConsistentUptime,
		CacheMaxSizeBytes:     w.CacheMaxSize,
		CacheStatus:           domain.ArvanCloudPageRuleCacheLevel(w.CacheStatus),
		CachePage200:          domain.ArvanCloudCacheTTL(w.CachePage200),
		CachePageAny:          domain.ArvanCloudCacheTTL(w.CachePageAny),
		CacheBrowser:          domain.ArvanCloudCacheTTL(w.CacheBrowser),
		CacheScheme:           w.CacheScheme,
		CacheIgnoreSC:         w.CacheIgnoreSC,
		CacheCookie:           w.CacheCookie,
		CacheArgs:             w.CacheArgs,
		CacheArg:              w.CacheArg,
	}
}

// cachingSettingsRequestBody builds the JSON body for a caching settings
// PATCH. Booleans are always sent (matching ddosSettingsRequestBody's own
// convention, since none of them are readOnly); the size and every
// enum/string field are sent only when non-zero/non-empty, leaving the
// provider's own default in place otherwise.
func cachingSettingsRequestBody(s domain.ArvanCloudCachingSettings) map[string]any {
	body := map[string]any{
		"cache_developer_mode":    s.CacheDeveloperMode,
		"cache_consistent_uptime": s.CacheConsistentUptime,
		"cache_scheme":            s.CacheScheme,
		"cache_ignore_sc":         s.CacheIgnoreSC,
		"cache_args":              s.CacheArgs,
	}
	if s.CacheMaxSizeBytes > 0 {
		body["cache_max_size"] = s.CacheMaxSizeBytes
	}
	if s.CacheStatus != "" {
		body["cache_status"] = string(s.CacheStatus)
	}
	if s.CachePage200 != "" {
		body["cache_page_200"] = string(s.CachePage200)
	}
	if s.CachePageAny != "" {
		body["cache_page_any"] = string(s.CachePageAny)
	}
	if s.CacheBrowser != "" {
		body["cache_browser"] = string(s.CacheBrowser)
	}
	if s.CacheCookie != "" {
		body["cache_cookie"] = s.CacheCookie
	}
	if s.CacheArg != "" {
		body["cache_arg"] = s.CacheArg
	}
	return body
}

// GetArvanCloudCachingSettings returns a domain's current cache-behavior
// settings (caching.index).
func (p *Provider) GetArvanCloudCachingSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudCachingSettings, error) {
	var wire cachingSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, cachingPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud caching settings for domain %q: %w", domainName, err)
	}
	settings := toCachingSettingsDomain(wire)
	return &settings, nil
}

// UpdateArvanCloudCachingSettings changes a domain's cache-behavior settings
// (caching.update). The endpoint's response carries no data, only a
// confirmation message.
func (p *Provider) UpdateArvanCloudCachingSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudCachingSettings) error {
	body := cachingSettingsRequestBody(settings)
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, cachingPath(domainName), body, nil); err != nil {
		return fmt.Errorf("updating arvancloud caching settings for domain %q: %w", domainName, err)
	}
	return nil
}

// cachePurgeRequestBody builds the JSON body for POST
// /domains/{domain}/caching/purge (caching.purge, the CachingPurge schema).
// Deliberately the ONLY purge path this adapter builds a request for — the
// deprecated DELETE /domains/{domain}/caching (caching.deprecated_purge) has
// no corresponding method anywhere in this package, per the spec's own
// deprecation notice and issue #72's explicit scope note. See
// TestPurgeArvanCloudCacheNeverCallsDeprecatedEndpoint.
func cachePurgeRequestBody(purge domain.ArvanCloudCachePurgeRequest) map[string]any {
	body := map[string]any{"purge": string(purge.Mode)}
	if len(purge.URLs) > 0 {
		body["purge_urls"] = purge.URLs
	}
	if len(purge.Tags) > 0 {
		body["purge_tags"] = purge.Tags
	}
	return body
}

// PurgeArvanCloudCache purges a domain's cached content (caching.purge). The
// endpoint's response carries no data, only a confirmation message — see
// ports.ArvanCloudProvider's own doc comment for why this is treated as a
// fast operation despite the response wording ("Successfully queued
// purge").
func (p *Provider) PurgeArvanCloudCache(ctx context.Context, creds domain.ProviderCredentials, domainName string, purge domain.ArvanCloudCachePurgeRequest) error {
	body := cachePurgeRequestBody(purge)
	if err := p.client.doJSON(ctx, creds, http.MethodPost, cachingPurgePath(domainName), body, nil); err != nil {
		return fmt.Errorf("purging arvancloud cache for domain %q: %w", domainName, err)
	}
	return nil
}

// domainPurgeTagsWire mirrors DomainPurgeTags, the response of
// purge_tags.index: one object wrapping the whole tag list, not a per-tag
// array — see domain.ArvanCloudPurgeTag's own doc comment for how this
// adapter denormalizes it.
type domainPurgeTagsWire struct {
	DomainID  string   `json:"domain_id"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
}

// ListArvanCloudPurgeTags returns a domain's previously-purged cache tag
// history (purge_tags.index, deprecated by the spec but still implemented
// per issue #72's own scope note).
func (p *Provider) ListArvanCloudPurgeTags(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudPurgeTag, error) {
	var wire domainPurgeTagsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, purgeTagsPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("listing arvancloud purge tags for domain %q: %w", domainName, err)
	}
	tags := make([]domain.ArvanCloudPurgeTag, len(wire.Tags))
	for i, t := range wire.Tags {
		tags[i] = domain.ArvanCloudPurgeTag{Tag: t, CreatedAt: wire.CreatedAt}
	}
	return tags, nil
}

// DeleteArvanCloudPurgeTag removes one tag from the purge-tag history by its
// value (purge_tags.destroy, deprecated by the spec but still implemented
// per issue #72's own scope note). The tag is a required query parameter,
// not a path segment or JSON body.
func (p *Provider) DeleteArvanCloudPurgeTag(ctx context.Context, creds domain.ProviderCredentials, domainName, tag string) error {
	path := purgeTagsPath(domainName) + "?" + url.Values{"tag": {tag}}.Encode()
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud purge tag %q for domain %q: %w", tag, domainName, err)
	}
	return nil
}

// --- Image Resize ------------------------------------------------------

// imageResizeSettingsWire mirrors ImageResize (the standalone endpoint's own
// schema — distinct from imageResizeWire in rules.go, which mirrors
// PageRuleImageResize, an allOf-widened variant with a different Status
// value set; see domain.ArvanCloudImageResizeStatus's own doc comment for
// why these stay two Go types).
type imageResizeSettingsWire struct {
	Status    string `json:"status,omitempty"`
	HeightBy  string `json:"height_by,omitempty"`
	WidthBy   string `json:"width_by,omitempty"`
	Mode      string `json:"mode,omitempty"`
	ModeBy    string `json:"mode_by,omitempty"`
	QualityBy string `json:"quality_by,omitempty"`
}

func toImageResizeSettingsDomain(w imageResizeSettingsWire) domain.ArvanCloudImageResizeSettings {
	return domain.ArvanCloudImageResizeSettings{
		Status:    domain.ArvanCloudImageResizeStatus(w.Status),
		HeightBy:  w.HeightBy,
		WidthBy:   w.WidthBy,
		Mode:      domain.ArvanCloudImageResizeMode(w.Mode),
		ModeBy:    w.ModeBy,
		QualityBy: w.QualityBy,
	}
}

func imageResizeSettingsRequestBody(s domain.ArvanCloudImageResizeSettings) map[string]any {
	return map[string]any{
		"status":     string(s.Status),
		"height_by":  s.HeightBy,
		"width_by":   s.WidthBy,
		"mode":       string(s.Mode),
		"mode_by":    s.ModeBy,
		"quality_by": s.QualityBy,
	}
}

// GetArvanCloudImageResizeSettings returns a domain's image-resize settings
// (image-resize.show).
func (p *Provider) GetArvanCloudImageResizeSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudImageResizeSettings, error) {
	var wire imageResizeSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, imageResizePath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud image-resize settings for domain %q: %w", domainName, err)
	}
	settings := toImageResizeSettingsDomain(wire)
	return &settings, nil
}

// UpdateArvanCloudImageResizeSettings changes a domain's image-resize
// settings and returns them as stored afterward (image-resize.update).
func (p *Provider) UpdateArvanCloudImageResizeSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudImageResizeSettings) (*domain.ArvanCloudImageResizeSettings, error) {
	body := imageResizeSettingsRequestBody(settings)
	var wire imageResizeSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, imageResizePath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud image-resize settings for domain %q: %w", domainName, err)
	}
	updated := toImageResizeSettingsDomain(wire)
	return &updated, nil
}

// --- Acceleration --------------------------------------------------------
//
// Reuses accelerationWire/toAccelerationDomain/accelerationRequestBody from
// rules.go — same package, same Acceleration schema (see
// domain.ArvanCloudAccelerationSettings' own doc comment).

// GetArvanCloudAccelerationSettings returns a domain's acceleration settings
// (acceleration.show).
func (p *Provider) GetArvanCloudAccelerationSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudAccelerationSettings, error) {
	var wire accelerationWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, accelerationPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud acceleration settings for domain %q: %w", domainName, err)
	}
	settings := toAccelerationDomain(&wire)
	return &settings, nil
}

// UpdateArvanCloudAccelerationSettings changes a domain's acceleration
// settings and returns them as stored afterward (acceleration.update). Per
// the spec's AccelerationUpdate schema, the provider only accepts "on"/"off"
// for settings.Status here (not "inherit", which is meaningful only in a
// PageRule context) — this adapter does not pre-check that client-side, the
// same choice rules.go's own accelerationRequestBody already made for
// PageRule.acceleration/PageRuleDiff.acceleration.
func (p *Provider) UpdateArvanCloudAccelerationSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudAccelerationSettings) (*domain.ArvanCloudAccelerationSettings, error) {
	body := accelerationRequestBody(settings)
	var wire accelerationWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, accelerationPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud acceleration settings for domain %q: %w", domainName, err)
	}
	updated := toAccelerationDomain(&wire)
	return &updated, nil
}

// --- Custom Pages ----------------------------------------------------------

// customPageFileWire mirrors CustomPageFile (the item shape in
// CustomPage.file, no "value").
type customPageFileWire struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

func toCustomPageFileDomain(w customPageFileWire) domain.ArvanCloudCustomPageFile {
	return domain.ArvanCloudCustomPageFile{ID: w.ID, Name: w.Name, Active: w.Active}
}

// customPageWire mirrors CustomPage.
type customPageWire struct {
	StatusCode int                  `json:"status_code"`
	Type       string               `json:"type"`
	URL        string               `json:"url,omitempty"`
	File       []customPageFileWire `json:"file,omitempty"`
}

func toCustomPageDomain(w customPageWire) domain.ArvanCloudCustomPage {
	files := make([]domain.ArvanCloudCustomPageFile, len(w.File))
	for i, f := range w.File {
		files[i] = toCustomPageFileDomain(f)
	}
	return domain.ArvanCloudCustomPage{
		StatusCode: domain.ArvanCloudCustomPageStatusCode(w.StatusCode),
		Type:       domain.ArvanCloudCustomPageType(w.Type),
		URL:        w.URL,
		Files:      files,
	}
}

// customPagesWire mirrors CustomPages: the domain's nine named slots.
type customPagesWire struct {
	UnderConstruction customPageWire `json:"under_construction"`
	FirewallError     customPageWire `json:"firewall_error"`
	WAFProtection     customPageWire `json:"waf_protection"`
	RateLimitExceeded customPageWire `json:"rate_limit_exceeded"`
	SecureLinkExpired customPageWire `json:"secure_link_expired"`
	SecureLinkInvalid customPageWire `json:"secure_link_invalid"`
	Error500          customPageWire `json:"error_500"`
	DdosJS            customPageWire `json:"ddos_js"`
	DdosCaptcha       customPageWire `json:"ddos_captcha"`
}

func toCustomPagesDomain(w customPagesWire) domain.ArvanCloudCustomPages {
	return domain.ArvanCloudCustomPages{
		UnderConstruction: toCustomPageDomain(w.UnderConstruction),
		FirewallError:     toCustomPageDomain(w.FirewallError),
		WAFProtection:     toCustomPageDomain(w.WAFProtection),
		RateLimitExceeded: toCustomPageDomain(w.RateLimitExceeded),
		SecureLinkExpired: toCustomPageDomain(w.SecureLinkExpired),
		SecureLinkInvalid: toCustomPageDomain(w.SecureLinkInvalid),
		Error500:          toCustomPageDomain(w.Error500),
		DdosJS:            toCustomPageDomain(w.DdosJS),
		DdosCaptcha:       toCustomPageDomain(w.DdosCaptcha),
	}
}

// ListArvanCloudCustomPages returns a domain's whole named-slot custom-pages
// object (custom-pages.show — despite the operationId, the response shape
// is a list/index; see domain.ArvanCloudCustomPages' own doc comment).
func (p *Provider) ListArvanCloudCustomPages(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudCustomPages, error) {
	var wire customPagesWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, customPagesPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("listing arvancloud custom pages for domain %q: %w", domainName, err)
	}
	pages := toCustomPagesDomain(wire)
	return &pages, nil
}

// customPageUpdateFileField is the multipart field name CustomPageUpdate
// declares for its file part (custom-pages.update).
const customPageUpdateFileField = "file"

// UpdateArvanCloudCustomPage updates exactly one named custom-page slot
// (custom-pages.update, multipart/form-data — see
// domain.ArvanCloudCustomPageUpdate's own doc comment for why a POST to the
// /custom-pages collection endpoint is per-slot, and for why this is not
// JSON). The endpoint's response carries no data, only a confirmation
// message.
func (p *Provider) UpdateArvanCloudCustomPage(ctx context.Context, creds domain.ProviderCredentials, domainName string, update domain.ArvanCloudCustomPageUpdate) error {
	fields := map[string]string{"page": string(update.Page)}
	if update.Type != "" {
		fields["type"] = string(update.Type)
	}
	if update.URL != "" {
		fields["url"] = update.URL
	}

	var file *multipartFormFile
	if len(update.FileContent) > 0 {
		fileName := update.FileName
		if fileName == "" {
			fileName = string(update.Page) + ".html"
		}
		file = &multipartFormFile{FieldName: customPageUpdateFileField, FileName: fileName, Content: update.FileContent}
	}

	if err := p.client.doMultipartForm(ctx, creds, http.MethodPost, customPagesPath(domainName), fields, file, nil); err != nil {
		return fmt.Errorf("updating arvancloud custom page %q for domain %q: %w", update.Page, domainName, err)
	}
	return nil
}

// customPageFileDataWire mirrors CustomPageFileData's inline data shape —
// CustomPageFile plus the file's HTML content in "value", which the plain
// CustomPageFile schema embedded in CustomPage.file does not carry (see
// domain.ArvanCloudCustomPageFile.Value's own doc comment).
type customPageFileDataWire struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Value  string `json:"value"`
}

// GetArvanCloudCustomPageFile returns one custom-page file's details,
// including its HTML content (custom-pages.file.show).
func (p *Provider) GetArvanCloudCustomPageFile(ctx context.Context, creds domain.ProviderCredentials, domainName, fileID string) (*domain.ArvanCloudCustomPageFile, error) {
	var wire customPageFileDataWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, customPageFilePath(domainName, fileID), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud custom page file %q for domain %q: %w", fileID, domainName, err)
	}
	found := domain.ArvanCloudCustomPageFile{ID: wire.ID, Name: wire.Name, Active: wire.Active, Value: wire.Value}
	return &found, nil
}

// UpdateArvanCloudCustomPageFile updates one already-uploaded custom-page
// file entry in place (custom-pages.file.update, multipart/form-data). The
// endpoint's response carries no data, only a confirmation message.
func (p *Provider) UpdateArvanCloudCustomPageFile(ctx context.Context, creds domain.ProviderCredentials, domainName, fileID string, active *bool, fileName string, fileContent []byte) error {
	fields := map[string]string{}
	if active != nil {
		fields["active"] = boolToWire(*active)
	}

	var file *multipartFormFile
	if len(fileContent) > 0 {
		name := fileName
		if name == "" {
			name = fileID + ".html"
		}
		file = &multipartFormFile{FieldName: customPageUpdateFileField, FileName: name, Content: fileContent}
	}

	if err := p.client.doMultipartForm(ctx, creds, http.MethodPost, customPageFilePath(domainName, fileID), fields, file, nil); err != nil {
		return fmt.Errorf("updating arvancloud custom page file %q for domain %q: %w", fileID, domainName, err)
	}
	return nil
}

// DeleteArvanCloudCustomPageFile removes one custom-page file entry by id
// (custom-pages.file.delete). The provider rejects deleting the
// currently-active file for a slot (a 400) — propagated as-is, not
// pre-checked client-side, since which file is active can change between a
// caller's read and delete.
func (p *Provider) DeleteArvanCloudCustomPageFile(ctx context.Context, creds domain.ProviderCredentials, domainName, fileID string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, customPageFilePath(domainName, fileID), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud custom page file %q for domain %q: %w", fileID, domainName, err)
	}
	return nil
}

// boolToWire renders b as the multipart/form-data string value ArvanCloud
// expects for a boolean form field, mirroring how encoding/json would
// stringify it — multipart fields are always plain strings, never a typed
// JSON boolean.
func boolToWire(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
