package domain

// The types below model ArvanCloud CDN "domain edge-behavior settings" that
// don't fit the rule-engine pattern of AC5-AC8 (Firewall/WAF/RateLimit/Lists)
// or the routing-rule pattern of AC11 (issue #71's Page Rules): mostly
// single-object GET/PATCH settings resources, plus one file-upload-ish
// resource (Custom Pages). Confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Caching", "Acceleration" and
// "Custom Pages" tags and the CacheSettings/CacheSettingsData, ImageResize,
// Acceleration, CustomPage/CustomPages schemas (issue #72/AC12).
//
// Acceleration note: the standalone GET/PATCH /domains/{domain}/acceleration
// endpoint (acceleration.show/acceleration.update) reuses
// domain.ArvanCloudAccelerationSettings from arvancloud_acceleration.go
// rather than defining a second type here — see that file's own doc comment,
// which already documents this exact reuse relationship (it was written when
// AC11 landed first, anticipating AC12).

// --- Caching -----------------------------------------------------------

// ArvanCloudCachingSettings is a domain's cache-behavior configuration
// (/domains/{domain}/caching, the CacheSettings/CacheSettingsData schemas).
// Every field is optional in the spec (no "required" list); an empty/zero
// field left unset on an update lets the provider keep applying its own
// default rather than this adapter guessing one client-side.
type ArvanCloudCachingSettings struct {
	// CacheDeveloperMode disables caching for the requesting IP only, useful
	// while developing against a cached origin.
	CacheDeveloperMode bool
	// CacheConsistentUptime is the provider's "consistent uptime" caching
	// mode toggle.
	CacheConsistentUptime bool
	// CacheMaxSizeBytes is the maximum size of cacheable content, in BYTES.
	// Defaults to 104857600 (100 MB) when left unset (zero); the provider's
	// own maximum is 2147483648 (2 GB).
	CacheMaxSizeBytes int64
	// CacheStatus is the domain-wide cache mode (cache_status). Reuses
	// ArvanCloudPageRuleCacheLevel's off/uri/query_string enum
	// (ValidArvanCloudPageRuleCacheLevel) — the spec declares the exact same
	// three values for this field as for PageRule.cache_level, so this type
	// avoids a near-duplicate enum. Empty means provider default.
	CacheStatus ArvanCloudPageRuleCacheLevel
	// CachePage200/CachePageAny are the shared TTL enum (see
	// arvancloud_rules.go's package comment for the full 29-value list):
	// how long to cache a 200 response, and any response, respectively. Must
	// satisfy ValidArvanCloudCacheTTL. Empty means provider default.
	CachePage200 ArvanCloudCacheTTL
	CachePageAny ArvanCloudCacheTTL
	// CacheBrowser is the browser-facing Cache-Control TTL (cache_browser).
	// Must satisfy ValidArvanCloudCacheBrowserTTL, which additionally allows
	// the sentinel "default". Empty means provider default.
	CacheBrowser ArvanCloudCacheTTL
	// CacheScheme is deprecated by the provider but still a real, settable
	// field: whether to consider the request scheme (HTTP/HTTPS) in caching.
	CacheScheme bool
	// CacheIgnoreSC ignores the default Set-Cookie-header caching behavior.
	CacheIgnoreSC bool
	// CacheCookie is a comma-separated list of cookie names to vary the
	// cache on. Empty means none.
	CacheCookie string
	// CacheArgs varies the cache by query-string arguments.
	CacheArgs bool
	// CacheArg is a "&"-separated list of specific query-string argument
	// names to vary the cache on, e.g. "filter&sort". Empty means none/all,
	// depending on CacheArgs. Default "".
	CacheArg string
}

// --- Cache purge ---------------------------------------------------------

// ArvanCloudCachePurgeMode is ArvanCloudCachePurgeRequest.Mode's enum
// (CachingPurge.purge).
type ArvanCloudCachePurgeMode string

const (
	// ArvanCloudCachePurgeAll purges every cached object for the domain.
	ArvanCloudCachePurgeAll ArvanCloudCachePurgeMode = "all"
	// ArvanCloudCachePurgeIndividual purges only the URLs listed in
	// ArvanCloudCachePurgeRequest.URLs.
	ArvanCloudCachePurgeIndividual ArvanCloudCachePurgeMode = "individual"
	// ArvanCloudCachePurgeTags purges only the tags listed in
	// ArvanCloudCachePurgeRequest.Tags. Deprecated by the provider (the spec
	// marks "tags is deprecated" on this value directly) but still a real,
	// accepted value, so it is still modeled here — only available for
	// domains with a Professional plan or higher per the spec's own
	// description; this adapter does not pre-check the plan client-side.
	ArvanCloudCachePurgeTags ArvanCloudCachePurgeMode = "tags"
)

var arvanCloudCachePurgeModes = []string{
	string(ArvanCloudCachePurgeAll), string(ArvanCloudCachePurgeIndividual), string(ArvanCloudCachePurgeTags),
}

// ValidArvanCloudCachePurgeMode reports whether s is one of
// CachingPurge.purge's three values.
func ValidArvanCloudCachePurgeMode(s string) bool { return contains(arvanCloudCachePurgeModes, s) }

// ArvanCloudCachePurgeRequest is the request body of PurgeArvanCloudCache
// (POST /domains/{domain}/caching/purge, the CachingPurge schema —
// caching.purge). This is the endpoint the spec's own deprecation notice on
// DELETE /domains/{domain}/caching (caching.deprecated_purge) points callers
// to; that deprecated endpoint is deliberately not implemented anywhere in
// this adapter (issue #72's own scope note; see
// TestPurgeArvanCloudCacheNeverCallsDeprecatedEndpoint).
type ArvanCloudCachePurgeRequest struct {
	// Mode must satisfy ValidArvanCloudCachePurgeMode; required.
	Mode ArvanCloudCachePurgeMode
	// URLs is required, 1-50 entries, when Mode is "individual"; ignored
	// otherwise.
	URLs []string
	// Tags is required, 1-100 entries (each at most 32 ASCII characters per
	// the spec), when Mode is "tags"; ignored otherwise. Deprecated, see
	// ArvanCloudCachePurgeTags.
	Tags []string
}

// ArvanCloudPurgeTag is one entry of a domain's previously-purged cache tag
// history (/domains/{domain}/purge-tags, the DomainPurgeTags schema —
// purge_tags.index, deprecated by the spec but still implemented per issue
// #72's own scope, matching the same "implement it anyway" choice this
// package already makes elsewhere for a deprecated-but-still-real field,
// e.g. PageRule.cluster_status).
//
// The provider returns the whole set as one object — {domain_id, tags:
// []string, created_at} — with one created_at shared by every tag in the
// response, not a per-tag timestamp. This type denormalizes that response
// into one row per tag so ListArvanCloudPurgeTags can return a plain slice
// (matching the list_arvancloud_purge_tags tool name and every other List*
// method on this port), at the cost of repeating CreatedAt on every entry.
type ArvanCloudPurgeTag struct {
	// Tag is one previously-purged cache tag value.
	Tag string
	// CreatedAt is when this batch of purge tags was recorded, as reported
	// by the provider. Shared across every entry from the same call.
	CreatedAt string
}

// --- Image Resize --------------------------------------------------------

// ArvanCloudImageResizeStatus is the base ImageResize.status enum
// ("on"/"off" only), used by the standalone /domains/{domain}/image-resize
// endpoint. NOT the same Go type as ArvanCloudPageRuleImageResizeStatus
// (arvancloud_rules.go): that one is PageRuleImageResize's allOf-widened
// override of this same field, which additionally accepts "inherit" — a
// value the standalone endpoint's plain ImageResize schema does not declare.
// Keeping two Go types here (rather than reusing the wider one everywhere,
// the choice ArvanCloudAccelerationSettings.Status made for the analogous
// Acceleration case) avoids letting a caller of the standalone endpoint send
// "inherit", which has no meaning outside a Page Rule context.
type ArvanCloudImageResizeStatus string

const (
	ArvanCloudImageResizeStatusOn  ArvanCloudImageResizeStatus = "on"
	ArvanCloudImageResizeStatusOff ArvanCloudImageResizeStatus = "off"
)

var arvanCloudImageResizeStatuses = []string{
	string(ArvanCloudImageResizeStatusOn), string(ArvanCloudImageResizeStatusOff),
}

// ValidArvanCloudImageResizeStatus reports whether s is one of
// ImageResize.status's two values, or empty (provider default "off"
// applies).
func ValidArvanCloudImageResizeStatus(s string) bool {
	if s == "" {
		return true
	}
	return contains(arvanCloudImageResizeStatuses, s)
}

// ArvanCloudImageResizeSettings is a domain's image-resize configuration
// (/domains/{domain}/image-resize, the ImageResize schema — image-
// resize.show/image-resize.update).
type ArvanCloudImageResizeSettings struct {
	// Status must satisfy ValidArvanCloudImageResizeStatus. Defaults to
	// "off" when left unset.
	Status ArvanCloudImageResizeStatus
	// HeightBy/WidthBy are the query-string argument names read for the
	// target height/width, e.g. "height"/"width". Pattern: [a-z0-9_]+, max
	// 20 chars. Default "height"/"width" when left unset.
	HeightBy string
	WidthBy  string
	// Mode must satisfy ValidArvanCloudImageResizeMode (arvancloud_rules.go
	// — reused directly, per that type's own doc comment: "used by both
	// PageRule.image_resize and the standalone image-resize endpoint... same
	// schema"). Empty means provider default "freely".
	Mode ArvanCloudImageResizeMode
	// ModeBy is the query-string variable name for overriding Mode per
	// request. Acceptable request-time values for that variable are "f"
	// (freely), "s" (short-side), "l" (long-side) per the spec's own
	// description. Pattern: [a-z0-9_]+, max 20 chars. Default "".
	ModeBy string
	// QualityBy is the query-string variable name for setting image quality
	// per request (acceptable request-time values: 1-100). Pattern:
	// [a-z0-9_]+, max 20 chars. Default "".
	QualityBy string
}

// --- Custom Pages ----------------------------------------------------------

// ArvanCloudCustomPageStatusCode is CustomPage.status_code's enum: a FIXED
// 8-value set of integer codes, confirmed directly against
// docs/api-specs/arvancloud-cdn-4.0.yml (`enum: [200, 302, 481, 403, 482,
// 483, 484, 500]`) — these are ArvanCloud's own internal status codes for
// specific error/response conditions (e.g. a WAF block, a rate-limit block,
// an expired/invalid secure link), NOT a generic HTTP status code list.
// Notably, 404 is genuinely absent from this set despite being an otherwise
// unremarkable HTTP status — confirmed directly against the spec rather than
// assumed to be an omission worth "fixing": see
// TestValidArvanCloudCustomPageStatusCodeRejects404.
type ArvanCloudCustomPageStatusCode int

const (
	ArvanCloudCustomPageStatus200 ArvanCloudCustomPageStatusCode = 200
	ArvanCloudCustomPageStatus302 ArvanCloudCustomPageStatusCode = 302
	ArvanCloudCustomPageStatus481 ArvanCloudCustomPageStatusCode = 481
	ArvanCloudCustomPageStatus403 ArvanCloudCustomPageStatusCode = 403
	ArvanCloudCustomPageStatus482 ArvanCloudCustomPageStatusCode = 482
	ArvanCloudCustomPageStatus483 ArvanCloudCustomPageStatusCode = 483
	ArvanCloudCustomPageStatus484 ArvanCloudCustomPageStatusCode = 484
	ArvanCloudCustomPageStatus500 ArvanCloudCustomPageStatusCode = 500
)

var arvanCloudCustomPageStatusCodes = []ArvanCloudCustomPageStatusCode{
	ArvanCloudCustomPageStatus200, ArvanCloudCustomPageStatus302, ArvanCloudCustomPageStatus481,
	ArvanCloudCustomPageStatus403, ArvanCloudCustomPageStatus482, ArvanCloudCustomPageStatus483,
	ArvanCloudCustomPageStatus484, ArvanCloudCustomPageStatus500,
}

// ValidArvanCloudCustomPageStatusCode reports whether code is one of
// CustomPage.status_code's exact 8 values, or zero (the field is optional on
// read; the provider reports its own default when a page has not been
// configured). Any other HTTP status code — including the otherwise
// unremarkable 404 — is rejected.
func ValidArvanCloudCustomPageStatusCode(code ArvanCloudCustomPageStatusCode) bool {
	if code == 0 {
		return true
	}
	for _, v := range arvanCloudCustomPageStatusCodes {
		if v == code {
			return true
		}
	}
	return false
}

// ArvanCloudCustomPageType is CustomPage.type's enum: confirmed directly
// against the spec as the plain string `off`/`url`/`file`
// (`enum: [off, url, file]`) — NOT a literal boolean `false` mixed into a
// string enum, despite issue #72's own review text raising that as a thing
// to confirm (drawing the parallel to WAF's/DDoS's mode/protection_mode
// fields, which genuinely are plain strings with no literal boolean). This
// field turns out to already match that same "plain string, no literal
// JSON boolean" pattern on its own: the disabled value is spelled "off",
// exactly like Acceleration.status and DdosSettings.protection_mode.
type ArvanCloudCustomPageType string

const (
	// ArvanCloudCustomPageTypeOff disables this custom page slot (the
	// provider's own default error content is served instead).
	ArvanCloudCustomPageTypeOff ArvanCloudCustomPageType = "off"
	// ArvanCloudCustomPageTypeURL redirects to an external URL
	// (ArvanCloudCustomPage.URL / ArvanCloudCustomPageUpdate.URL).
	ArvanCloudCustomPageTypeURL ArvanCloudCustomPageType = "url"
	// ArvanCloudCustomPageTypeFile serves an uploaded HTML file
	// (ArvanCloudCustomPage.Files / ArvanCloudCustomPageUpdate's file
	// upload).
	ArvanCloudCustomPageTypeFile ArvanCloudCustomPageType = "file"
)

var arvanCloudCustomPageTypes = []string{
	string(ArvanCloudCustomPageTypeOff), string(ArvanCloudCustomPageTypeURL), string(ArvanCloudCustomPageTypeFile),
}

// ValidArvanCloudCustomPageType reports whether s is one of CustomPage.type's
// three values, or empty (optional on read).
func ValidArvanCloudCustomPageType(s string) bool {
	if s == "" {
		return true
	}
	return contains(arvanCloudCustomPageTypes, s)
}

// ArvanCloudCustomPageName is CustomPageUpdate.page's enum: which of a
// domain's nine named custom-page slots an UpdateArvanCloudCustomPage call
// targets (custom-pages.update is POST to the /custom-pages COLLECTION
// endpoint, but always updates exactly one named slot selected by this
// field — see ArvanCloudCustomPageUpdate's own doc comment for why the
// update_arvancloud_custom_pages tool is per-slot despite its plural name).
type ArvanCloudCustomPageName string

const (
	ArvanCloudCustomPageUnderConstruction ArvanCloudCustomPageName = "under_construction"
	ArvanCloudCustomPageFirewallError     ArvanCloudCustomPageName = "firewall_error"
	ArvanCloudCustomPageWAFProtection     ArvanCloudCustomPageName = "waf_protection"
	ArvanCloudCustomPageRateLimitExceeded ArvanCloudCustomPageName = "rate_limit_exceeded"
	ArvanCloudCustomPageSecureLinkExpired ArvanCloudCustomPageName = "secure_link_expired"
	ArvanCloudCustomPageSecureLinkInvalid ArvanCloudCustomPageName = "secure_link_invalid"
	ArvanCloudCustomPageError500          ArvanCloudCustomPageName = "error_500"
	// ArvanCloudCustomPageDdosJS and ArvanCloudCustomPageDdosCaptcha can only
	// be used with ArvanCloudCustomPageTypeFile, per the spec's own
	// description on CustomPageUpdate.page — this adapter does not enforce
	// that pairing client-side; the provider reports a 422 on a mismatch.
	ArvanCloudCustomPageDdosJS      ArvanCloudCustomPageName = "ddos_js"
	ArvanCloudCustomPageDdosCaptcha ArvanCloudCustomPageName = "ddos_captcha"
)

var arvanCloudCustomPageNames = []string{
	string(ArvanCloudCustomPageUnderConstruction), string(ArvanCloudCustomPageFirewallError),
	string(ArvanCloudCustomPageWAFProtection), string(ArvanCloudCustomPageRateLimitExceeded),
	string(ArvanCloudCustomPageSecureLinkExpired), string(ArvanCloudCustomPageSecureLinkInvalid),
	string(ArvanCloudCustomPageError500), string(ArvanCloudCustomPageDdosJS), string(ArvanCloudCustomPageDdosCaptcha),
}

// ValidArvanCloudCustomPageName reports whether s is one of
// CustomPageUpdate.page's nine values.
func ValidArvanCloudCustomPageName(s string) bool { return contains(arvanCloudCustomPageNames, s) }

// ArvanCloudCustomPageFile is one uploaded file entry of a custom page slot
// (the CustomPageFile schema): ArvanCloudCustomPage.Files' item shape, and
// the shape GetArvanCloudCustomPageFile returns for a single file by id.
// Every upload to a "file"-type slot creates a NEW entry here (the spec's
// own description on CustomPageUpdate.file/CustomPageFileUpdate.file), so a
// slot can accumulate multiple files over time with only one marked Active.
type ArvanCloudCustomPageFile struct {
	// ID is the provider-assigned UUID, the path segment every
	// custom-pages.file.* endpoint addresses this file by.
	ID string
	// Name is the uploaded file's name.
	Name string
	// Active reports whether this is the file currently served for its slot.
	Active bool
	// Value is the file's HTML content. Only populated by
	// GetArvanCloudCustomPageFile (custom-pages.file.show, the
	// CustomPageFileData schema, which adds this field on top of the base
	// CustomPageFile shape) — always empty on an entry read from
	// ArvanCloudCustomPage.Files (ListArvanCloudCustomPages), since the
	// spec's plain CustomPageFile schema used there has no "value" field.
	Value string
}

// ArvanCloudCustomPage is one of a domain's nine named custom-page slots
// (the CustomPage schema): what is served instead of ArvanCloud's own
// default content for a specific condition (a WAF block, a rate-limit
// block, an expired secure link, ...).
type ArvanCloudCustomPage struct {
	// StatusCode must satisfy ValidArvanCloudCustomPageStatusCode — the
	// fixed 8-value set, see that type's own doc comment.
	StatusCode ArvanCloudCustomPageStatusCode
	// Type must satisfy ValidArvanCloudCustomPageType.
	Type ArvanCloudCustomPageType
	// URL is the redirect target when Type is ArvanCloudCustomPageTypeURL.
	URL string
	// Files lists every uploaded file for this slot when Type is
	// ArvanCloudCustomPageTypeFile (present/relevant only in that case per
	// the spec's own description on CustomPage.file). Only one entry is
	// normally Active at a time.
	Files []ArvanCloudCustomPageFile
}

// ArvanCloudCustomPages is the full set of a domain's nine named custom-page
// slots (/domains/{domain}/custom-pages, the CustomPages schema —
// custom-pages.show/ListArvanCloudCustomPages). Despite the operationId
// literally saying "show", the response is this whole named-slot object,
// not a single CustomPage — issue #72's own scope note flags this exact
// operationId-vs-response-shape mismatch, and this type is modeled on the
// confirmed response shape rather than the operationId's name.
type ArvanCloudCustomPages struct {
	UnderConstruction ArvanCloudCustomPage
	FirewallError     ArvanCloudCustomPage
	WAFProtection     ArvanCloudCustomPage
	RateLimitExceeded ArvanCloudCustomPage
	SecureLinkExpired ArvanCloudCustomPage
	SecureLinkInvalid ArvanCloudCustomPage
	Error500          ArvanCloudCustomPage
	DdosJS            ArvanCloudCustomPage
	DdosCaptcha       ArvanCloudCustomPage
}

// ArvanCloudCustomPageUpdate is the request for UpdateArvanCloudCustomPage
// (POST /domains/{domain}/custom-pages, the CustomPageUpdate schema —
// custom-pages.update). Despite being POST to the /custom-pages collection
// endpoint (matching custom-pages.show's own path), this updates exactly
// ONE named slot per call, selected by Page — confirmed directly against
// the spec's CustomPageUpdate request schema, which has no way to address
// more than one slot in a single request. The request is multipart/
// form-data, not JSON (the spec declares FileContent's field as
// `type: string, format: binary`), because Type can be "file", in which
// case FileContent carries a new HTML file to upload.
type ArvanCloudCustomPageUpdate struct {
	// Page selects which of the domain's nine slots this call updates. Must
	// satisfy ValidArvanCloudCustomPageName; required.
	Page ArvanCloudCustomPageName
	// Type must satisfy ValidArvanCloudCustomPageType.
	Type ArvanCloudCustomPageType
	// URL is sent when Type is ArvanCloudCustomPageTypeURL.
	URL string
	// FileName/FileContent carry a new HTML file to upload when Type is
	// ArvanCloudCustomPageTypeFile (required in that case per the spec's own
	// description — "HTML file to upload (required when type is file). Each
	// upload creates a new file entry."). FileName need only be
	// human-readable; the provider does not require any particular
	// extension.
	FileName    string
	FileContent []byte
}
