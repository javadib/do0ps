package domain

// The types below model ArvanCloud's CDN Apps marketplace (issue #77): a
// catalog of installable edge add-ons (analytics snippets, third-party
// integrations, etc.) a domain owner can browse, install, and uninstall.
// Confirmed against docs/api-specs/arvancloud-cdn-4.0.yml's "CDN Apps" tag
// (the apps.* and domains.apps.* operationIds) and the
// CdnApp/CdnAppLikeStats/DomainCdnApp/ApplicationCategory schemas.
//
// Unlike every other CDN capability on this port, CDN Apps are not
// domain-scoped at the catalog level: GET /apps returns the full account-wide
// marketplace, while GET /domains/{domain}/apps returns only the apps
// installed on that domain. The domain-scoped endpoints
// (domains.apps.*) take both domain and app ID.

// ArvanCloudCdnAppStatus is the publication status of a marketplace listing
// (CdnApp.status). Only "published" apps are intended for end-user
// installation; "draft" apps are internal/testing and should be filtered
// from chatbot-facing results by default (see
// ListArvanCloudCdnApps.IncludeDraft).
type ArvanCloudCdnAppStatus string

const (
	ArvanCloudCdnAppStatusPublished ArvanCloudCdnAppStatus = "published"
	ArvanCloudCdnAppStatusDraft    ArvanCloudCdnAppStatus = "draft"
)

// ArvanCloudCdnApp is a marketplace listing (CdnApp schema) — the shape
// returned by both the catalog (GET /apps) and the domain-scoped installed
// list (GET /domains/{domain}/apps).
type ArvanCloudCdnApp struct {
	// ID is the app's provider-assigned UUID.
	ID string
	// Categories lists the app's marketplace categories.
	Categories []ArvanCloudAppCategory
	// Rank is the app's display ranking (higher = more prominent).
	Rank float64
	// Name is the app's human-readable display name.
	Name string
	// Slug is the URL-friendly identifier.
	Slug string
	// ShortDescription is a one-line summary.
	ShortDescription string
	// Description is the full marketing description.
	Description string
	// Logo is an absolute URL to the app's logo image.
	Logo string
	// Pictures is a list of absolute URLs to the app's screenshot/gallery
	// images.
	Pictures []string
	// Vendor is the app's publisher.
	Vendor string
	// SupportEmail is the vendor's support contact (email format).
	SupportEmail string
	// InstallJSON is the app's configurable options schema (an opaque
	// object from the spec's perspective — the shape is app-defined).
	// A nil or empty map means the app has no configurable options.
	InstallJSON map[string]any
	// Status is whether the app is published or still a draft.
	Status ArvanCloudCdnAppStatus
	// LikeStats carries the aggregate like/dislike counts.
	LikeStats ArvanCloudCdnAppLikeStats
	// LikeByAccount is the caller's own vote: true=liked, false=disliked,
	// nil=not voted.
	LikeByAccount *bool
	// CreatedAt is the marketplace listing creation timestamp (ISO 8601).
	CreatedAt string
	// UpdatedAt is the last-update timestamp (ISO 8601).
	UpdatedAt string
}

// ArvanCloudCdnAppLikeStats carries the aggregate like/dislike counts
// (CdnAppLikeStats).
type ArvanCloudCdnAppLikeStats struct {
	LikesCount   int
	DislikesCount int
}

// ArvanCloudAppCategory is a marketplace category (ApplicationCategory
// schema): a simple id/name pair, optionally with translated names.
type ArvanCloudAppCategory struct {
	ID              string
	Name            string
	Active          bool
	Order           int
	NameTranslation map[string]ArvanCloudAppCategoryTranslation
}

// ArvanCloudAppCategoryTranslation is one language's translated name
// (the "en"/"fa" entries inside ApplicationCategory.name_translation).
type ArvanCloudAppCategoryTranslation struct {
	Name string
}

// ArvanCloudDomainCdnApp is an installed instance of a CDN app on a domain
// (DomainCdnApp schema) — the shape returned by POST /domains/{domain}/apps/{id}
// (install). It carries the installation's own ID and the active/options
// configuration.
type ArvanCloudDomainCdnApp struct {
	// ID is the installation's provider-assigned UUID.
	ID string
	// DomainID is the UUID of the domain this app is installed on.
	DomainID string
	// ApplicationID is the UUID of the marketplace app that was installed.
	ApplicationID string
	// Active indicates whether this installed app instance is currently
	// enabled.
	Active bool
	// Options is the concrete configuration values applied at install time,
	// matching the app's install_json schema.
	Options map[string]any
	// CreatedAt is the installation timestamp (ISO 8601).
	CreatedAt string
	// UpdatedAt is the last-update timestamp (ISO 8601).
	UpdatedAt string
}

// ArvanCloudTriggerWebhookEvent is the event type for trigger_webhook
// (CdnAppTriggerWebhook.event). Confirmed against the spec: two values.
type ArvanCloudTriggerWebhookEvent string

const (
	ArvanCloudTriggerWebhookBeforeNewInstall ArvanCloudTriggerWebhookEvent = "before-new-install"
	ArvanCloudTriggerWebhookNewInstall       ArvanCloudTriggerWebhookEvent = "new-install"
)

var arvanCloudTriggerWebhookEvents = []string{
	string(ArvanCloudTriggerWebhookBeforeNewInstall),
	string(ArvanCloudTriggerWebhookNewInstall),
}

// ValidArvanCloudTriggerWebhookEvent reports whether s is one of
// CdnAppTriggerWebhook.event's two values.
func ValidArvanCloudTriggerWebhookEvent(s string) bool {
	return contains(arvanCloudTriggerWebhookEvents, s)
}
