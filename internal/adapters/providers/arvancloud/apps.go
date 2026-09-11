package arvancloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/javadib/do0ps/internal/core/domain"
)

// CDN Apps marketplace (issue #77), wired to the real CDN API: a catalog of
// installable edge add-ons a domain owner can browse, install, and uninstall.
// Base paths are confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "CDN Apps" tag, relative to
// Client.baseURL — i.e. https://napi.arvancloud.ir/cdn/4.0/apps and
// /domains/{domain}/apps.
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real ArvanCloud responses correctly. Nothing above
// the adapter boundary ever sees them — every method here translates
// to/from internal/core/domain types.

const (
	appsBasePath = "apps"
)

func appsPath() string { return appsBasePath }
func appPath(id string) string { return appsBasePath + "/" + id }
func appLikePath(id string) string { return appsBasePath + "/" + id }
func appCategoriesPath() string { return appsBasePath + "/category" }
func appCategoryPath(categoryID string) string { return appsBasePath + "/category/" + categoryID }
func domainAppsPath(domainName string) string { return domainPath(domainName) + "/apps" }
func domainAppPath(domainName, appID string) string { return domainPath(domainName) + "/apps/" + appID }
func domainAppWebhookPath(domainName, appID string) string { return domainAppPath(domainName, appID) + "/actions/trigger_webhook" }

// --- wire types ---------------------------------------------------------

// cdnAppCategoryWire mirrors ApplicationCategory (the components schema,
// not the path parameter with the same name).
type cdnAppCategoryWire struct {
	ID              string                                    `json:"id"`
	Name            string                                    `json:"name"`
	Active          bool                                      `json:"active"`
	Order           int                                       `json:"order"`
	NameTranslation map[string]cdnAppCategoryTranslationWire `json:"name_translation,omitempty"`
}

// cdnAppCategoryTranslationWire mirrors one language entry inside
// ApplicationCategory.name_translation.
type cdnAppCategoryTranslationWire struct {
	Name string `json:"name"`
}

func toAppCategoryDomain(w cdnAppCategoryWire) domain.ArvanCloudAppCategory {
	out := domain.ArvanCloudAppCategory{
		ID:     w.ID,
		Name:   w.Name,
		Active: w.Active,
		Order:  w.Order,
	}
	if len(w.NameTranslation) > 0 {
		out.NameTranslation = make(map[string]domain.ArvanCloudAppCategoryTranslation, len(w.NameTranslation))
		for lang, t := range w.NameTranslation {
			out.NameTranslation[lang] = domain.ArvanCloudAppCategoryTranslation{Name: t.Name}
		}
	}
	return out
}

// cdnAppLikeStatsWire mirrors CdnAppLikeStats.
type cdnAppLikeStatsWire struct {
	LikesCount   int `json:"likes_count"`
	DislikesCount int `json:"dislikes_count"`
}

func toLikeStatsDomain(w cdnAppLikeStatsWire) domain.ArvanCloudCdnAppLikeStats {
	return domain.ArvanCloudCdnAppLikeStats{
		LikesCount:    w.LikesCount,
		DislikesCount: w.DislikesCount,
	}
}

// cdnAppWire mirrors CdnApp (used by both catalog and domain-scoped
// list endpoints).
type cdnAppWire struct {
	ID               string                  `json:"id"`
	Categories       []cdnAppCategoryWire    `json:"categories,omitempty"`
	Rank             float64                 `json:"rank"`
	Name             string                  `json:"name"`
	Slug             string                  `json:"slug"`
	ShortDescription string                  `json:"short_description"`
	Description      string                  `json:"description"`
	Logo             string                  `json:"logo"`
	Pictures         []string                `json:"pictures,omitempty"`
	Vendor           string                  `json:"vendor"`
	SupportEmail     string                  `json:"support_email"`
	InstallJSON      map[string]any          `json:"install_json,omitempty"`
	Status           string                  `json:"status"`
	LikeStats        cdnAppLikeStatsWire     `json:"like_stats"`
	LikeByAccount    *bool                   `json:"like_by_account"`
	CreatedAt        string                  `json:"created_at"`
	UpdatedAt        string                  `json:"updated_at"`
}

func toCdnAppDomain(w cdnAppWire) domain.ArvanCloudCdnApp {
	categories := make([]domain.ArvanCloudAppCategory, len(w.Categories))
	for i, c := range w.Categories {
		categories[i] = toAppCategoryDomain(c)
	}
	return domain.ArvanCloudCdnApp{
		ID:               w.ID,
		Categories:       categories,
		Rank:             w.Rank,
		Name:             w.Name,
		Slug:             w.Slug,
		ShortDescription: w.ShortDescription,
		Description:      w.Description,
		Logo:             w.Logo,
		Pictures:         w.Pictures,
		Vendor:           w.Vendor,
		SupportEmail:     w.SupportEmail,
		InstallJSON:      w.InstallJSON,
		Status:           domain.ArvanCloudCdnAppStatus(w.Status),
		LikeStats:        toLikeStatsDomain(w.LikeStats),
		LikeByAccount:    w.LikeByAccount,
		CreatedAt:        w.CreatedAt,
		UpdatedAt:        w.UpdatedAt,
	}
}

// domainCdnAppWire mirrors DomainCdnApp (the install response).
type domainCdnAppWire struct {
	ID            string         `json:"id"`
	DomainID      string         `json:"domain_id"`
	ApplicationID string         `json:"application_id"`
	Active        bool           `json:"active"`
	Options       map[string]any `json:"options,omitempty"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
}

func toDomainCdnAppDomain(w domainCdnAppWire) domain.ArvanCloudDomainCdnApp {
	return domain.ArvanCloudDomainCdnApp{
		ID:            w.ID,
		DomainID:      w.DomainID,
		ApplicationID: w.ApplicationID,
		Active:        w.Active,
		Options:       w.Options,
		CreatedAt:     w.CreatedAt,
		UpdatedAt:     w.UpdatedAt,
	}
}

// --- catalog operations ------------------------------------------------

// ListArvanCloudCdnApps returns every app in the marketplace catalog.
func (p *Provider) ListArvanCloudCdnApps(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudCdnApp, error) {
	var items []cdnAppWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, appsPath(), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud cdn apps: %w", err)
	}
	apps := make([]domain.ArvanCloudCdnApp, len(items))
	for i := range items {
		apps[i] = toCdnAppDomain(items[i])
	}
	return apps, nil
}

// GetArvanCloudCdnApp returns a single marketplace listing by ID.
func (p *Provider) GetArvanCloudCdnApp(ctx context.Context, creds domain.ProviderCredentials, appID string) (*domain.ArvanCloudCdnApp, error) {
	var wire cdnAppWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, appPath(appID), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud cdn app %q: %w", appID, err)
	}
	app := toCdnAppDomain(wire)
	return &app, nil
}

// cdnAppLikeRequest mirrors CdnAppLike (the POST body for apps.like).
type cdnAppLikeRequest struct {
	Like *bool `json:"like"`
}

// LikeArvanCloudCdnApp expresses a like, dislike, or vote retraction for
// an app. Returns the updated aggregate like_stats.
func (p *Provider) LikeArvanCloudCdnApp(ctx context.Context, creds domain.ProviderCredentials, appID string, like *bool) (*domain.ArvanCloudCdnAppLikeStats, error) {
	body := cdnAppLikeRequest{Like: like}
	var wire cdnAppLikeStatsWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, appLikePath(appID), body, &wire); err != nil {
		return nil, fmt.Errorf("liking arvancloud cdn app %q: %w", appID, err)
	}
	stats := toLikeStatsDomain(wire)
	return &stats, nil
}

// --- categories --------------------------------------------------------

// ListArvanCloudCdnAppCategories returns every marketplace category.
func (p *Provider) ListArvanCloudCdnAppCategories(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudAppCategory, error) {
	var items []cdnAppCategoryWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, appCategoriesPath(), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud cdn app categories: %w", err)
	}
	cats := make([]domain.ArvanCloudAppCategory, len(items))
	for i := range items {
		cats[i] = toAppCategoryDomain(items[i])
	}
	return cats, nil
}

// GetArvanCloudCdnAppCategory returns one marketplace category by ID.
func (p *Provider) GetArvanCloudCdnAppCategory(ctx context.Context, creds domain.ProviderCredentials, categoryID string) (*domain.ArvanCloudAppCategory, error) {
	var wire cdnAppCategoryWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, appCategoryPath(categoryID), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud cdn app category %q: %w", categoryID, err)
	}
	cat := toAppCategoryDomain(wire)
	return &cat, nil
}

// --- domain-scoped operations ------------------------------------------

// ListArvanCloudDomainCdnApps returns the apps installed on domainName.
func (p *Provider) ListArvanCloudDomainCdnApps(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudCdnApp, error) {
	var items []cdnAppWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, domainAppsPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud cdn apps of domain %q: %w", domainName, err)
	}
	apps := make([]domain.ArvanCloudCdnApp, len(items))
	for i := range items {
		apps[i] = toCdnAppDomain(items[i])
	}
	return apps, nil
}

// cdnAppInstallWire mirrors the CdnAppInstall schema (the GET response for
// domains.apps.installed): a simple {is_install: bool} body. Unlike most
// endpoints, this one does NOT wrap the response in a {"data":...} envelope,
// so we use doRawGET and unmarshal the body directly.
type cdnAppInstallWire struct {
	IsInstall bool `json:"is_install"`
}

// CheckArvanCloudCdnAppInstalled reports whether appID is installed on
// domainName. The spec's GET /domains/{domain}/apps/{id} returns CdnAppInstall
// with {"is_install": bool} — no data envelope, so doRawGET is used.
func (p *Provider) CheckArvanCloudCdnAppInstalled(ctx context.Context, creds domain.ProviderCredentials, domainName, appID string) (bool, error) {
	raw, err := p.client.doRawGET(ctx, creds, domainAppPath(domainName, appID), "application/json")
	if err != nil {
		return false, fmt.Errorf("checking arvancloud cdn app %q installation on domain %q: %w", appID, domainName, err)
	}
	var wire cdnAppInstallWire
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &wire); err != nil {
			return false, fmt.Errorf("decoding arvancloud cdn app install status for %q on domain %q: %w", appID, domainName, err)
		}
	}
	return wire.IsInstall, nil
}

// InstallArvanCloudCdnApp installs appID on domainName with the given
// options. The spec's POST body is AppOptions (an opaque object).
func (p *Provider) InstallArvanCloudCdnApp(ctx context.Context, creds domain.ProviderCredentials, domainName, appID string, options map[string]any) (*domain.ArvanCloudDomainCdnApp, error) {
	if options == nil {
		options = map[string]any{}
	}
	var wire domainCdnAppWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainAppPath(domainName, appID), options, &wire); err != nil {
		return nil, fmt.Errorf("installing arvancloud cdn app %q on domain %q: %w", appID, domainName, err)
	}
	installed := toDomainCdnAppDomain(wire)
	return &installed, nil
}

// UninstallArvanCloudCdnApp removes an app from a domain.
func (p *Provider) UninstallArvanCloudCdnApp(ctx context.Context, creds domain.ProviderCredentials, domainName, appID string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, domainAppPath(domainName, appID), nil, nil); err != nil {
		return fmt.Errorf("uninstalling arvancloud cdn app %q from domain %q: %w", appID, domainName, err)
	}
	return nil
}

// cdnAppTriggerWebhookRequest mirrors CdnAppTriggerWebhook.
type cdnAppTriggerWebhookRequest struct {
	Event   string         `json:"event"`
	Options map[string]any `json:"options"`
}

// TriggerArvanCloudCdnAppWebhook fires the app's webhook event on the
// domain.
func (p *Provider) TriggerArvanCloudCdnAppWebhook(ctx context.Context, creds domain.ProviderCredentials, domainName, appID string, event domain.ArvanCloudTriggerWebhookEvent, options map[string]any) error {
	body := cdnAppTriggerWebhookRequest{
		Event:   string(event),
		Options: options,
	}
	if body.Options == nil {
		body.Options = map[string]any{}
	}
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainAppWebhookPath(domainName, appID), body, nil); err != nil {
		return fmt.Errorf("triggering arvancloud cdn app %q webhook on domain %q: %w", appID, domainName, err)
	}
	return nil
}
