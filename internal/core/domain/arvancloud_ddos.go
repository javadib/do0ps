package domain

// The types below model ArvanCloud's DDoS Protection (issue #67): a
// per-domain challenge engine (cookie/JavaScript/CAPTCHA-based) with its own
// rule layer (ddos.rules.*) that exempts or enforces specific traffic,
// layered independently of the CDN edge Firewall (issue #65) and the WAF
// managed rule-set engine (issue #66). Confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "DDoS" tag (the ddos.settings.*,
// ddos.rules.* and ddos.reprioritize operationIds) and the
// DdosSettings/DdosRule schemas.
//
// Naming collision warning, same shape as the Firewall/#65 and WAF/#66
// warnings: DDoS here shares the word "rule" and the "protect"/"passthrough"
// action spelling with WAF's custom rules, but is a fully independent
// product with its own settings shape (a challenge mechanism, not a managed
// rule-set subscription) and its own Go types — nothing here is unified with
// domain.ArvanCloudWafRule/domain.ArvanCloudWafRuleAction, and nothing should
// be, absent a spec change that actually merges the two.

// ArvanCloudDdosProtectionMode is a domain's DDoS protection mechanism
// (DdosSettings.protection_mode).
//
// Wire-encoding finding (checked directly against the spec, per issue #67's
// explicit warning not to assume this mirrors WAF's mode resolution):
// docs/api-specs/arvancloud-cdn-4.0.yml line 9847-9849 declares
//
//	protection_mode:
//	  type: string
//	  enum: [off, cookie, javascript, captcha]
//
// i.e. a plain STRING enum of FOUR values — "off" plus the three actual
// challenge mechanisms (cookie/javascript/captcha) — again with no literal
// JSON boolean `false` anywhere in the schema for this field. This differs
// from WAF's protection_mode-shaped field (WafSettings.mode) in value count
// (four here vs. three there) and in meaning (a challenge mechanism choice
// here, an inspect/block posture there), even though both happen to use
// "off" as their disabled value and both are plain string enums on the wire.
// DDoS does NOT genuinely differ from WAF's finding on the boolean-false
// question — both are confirmed plain strings — but it does differ on the
// value set, which is why this is its own type rather than reusing
// ArvanCloudWafMode.
type ArvanCloudDdosProtectionMode string

const (
	// ArvanCloudDdosProtectionModeOff disables DDoS protection for the domain
	// entirely.
	ArvanCloudDdosProtectionModeOff ArvanCloudDdosProtectionMode = "off"
	// ArvanCloudDdosProtectionModeCookie challenges suspicious traffic with a
	// cookie-based check.
	ArvanCloudDdosProtectionModeCookie ArvanCloudDdosProtectionMode = "cookie"
	// ArvanCloudDdosProtectionModeJavaScript challenges suspicious traffic
	// with a JavaScript-execution check.
	ArvanCloudDdosProtectionModeJavaScript ArvanCloudDdosProtectionMode = "javascript"
	// ArvanCloudDdosProtectionModeCaptcha challenges suspicious traffic with a
	// CAPTCHA, using CaptchaService/SiteKey/SecretKey below.
	ArvanCloudDdosProtectionModeCaptcha ArvanCloudDdosProtectionMode = "captcha"
)

var arvanCloudDdosProtectionModes = []string{
	string(ArvanCloudDdosProtectionModeOff),
	string(ArvanCloudDdosProtectionModeCookie),
	string(ArvanCloudDdosProtectionModeJavaScript),
	string(ArvanCloudDdosProtectionModeCaptcha),
}

// ValidArvanCloudDdosProtectionMode reports whether s is one of
// DdosSettings.protection_mode's four values.
func ValidArvanCloudDdosProtectionMode(s string) bool {
	return contains(arvanCloudDdosProtectionModes, s)
}

// ArvanCloudCaptchaService is the CAPTCHA vendor a domain's DDoS settings use
// when ProtectionMode is ArvanCloudDdosProtectionModeCaptcha
// (DdosSettings.captcha_service).
type ArvanCloudCaptchaService string

const (
	ArvanCloudCaptchaServiceRecaptcha ArvanCloudCaptchaService = "recaptcha"
	ArvanCloudCaptchaServiceArcaptcha ArvanCloudCaptchaService = "arcaptcha"
	ArvanCloudCaptchaServiceHcaptcha  ArvanCloudCaptchaService = "hcaptcha"
)

var arvanCloudCaptchaServices = []string{
	string(ArvanCloudCaptchaServiceRecaptcha),
	string(ArvanCloudCaptchaServiceArcaptcha),
	string(ArvanCloudCaptchaServiceHcaptcha),
}

// ValidArvanCloudCaptchaService reports whether s is one of
// DdosSettings.captcha_service's three values.
func ValidArvanCloudCaptchaService(s string) bool { return contains(arvanCloudCaptchaServices, s) }

// ArvanCloudDdosPreflight is DdosSettings.preflight: the CORS-preflight
// response ArvanCloud emits alongside a DDoS challenge, so a browser-based
// API client is not blocked by the challenge's own cross-origin response.
type ArvanCloudDdosPreflight struct {
	AccessOrigin        string
	AccessCredentials   string
	AccessMethods       []string
	AccessHeaders       string
	AccessExposeHeaders string
}

// ArvanCloudDdosSettings is a domain's DDoS protection configuration
// (/domains/{domain}/ddos/settings, the DdosSettings schema).
//
// SecretKey is the CAPTCHA provider's own secret key — a credential the
// domain owner registers directly with reCAPTCHA/hCAPTCHA/ArCaptcha, NOT an
// ArvanCloud credential (unrelated to ProviderCredentials.APIKey/SecretKey
// above). It is treated as sensitive the same way ProviderCredentials is:
// never logged, never embedded in an error message, per AGENTS.md 7's
// no-secrets rule extended to runtime logging (see this package's provider
// adapter, internal/adapters/providers/arvancloud/ddos.go, and its tests).
//
// Spec quirk worth flagging: docs/api-specs/arvancloud-cdn-4.0.yml's own
// description text for both site_key and secret_key literally reads "it can
// be configured when the captcha_service is set to arcaptcha" — narrower
// than what the field's purpose actually implies (a CAPTCHA site/secret key
// pair is meaningful for any of the three captcha_service values, not only
// arcaptcha). This reads as a copy-paste artifact in the spec's authored
// description rather than an intentional restriction, so this type and its
// validation (see the update use case) treat SiteKey/SecretKey as meaningful
// whenever ProtectionMode is ArvanCloudDdosProtectionModeCaptcha, matching
// issue #67's own framing ("only meaningful when protection_mode is
// captcha"), not the narrower arcaptcha-only text.
type ArvanCloudDdosSettings struct {
	// IsEnabled is read-only: whether the domain's DDoS protection is active
	// at all (the spec marks this readOnly, so it is reported but never sent
	// on update).
	IsEnabled bool

	// ProtectionMode is the domain's DDoS challenge mechanism. Must be one of
	// ValidArvanCloudDdosProtectionMode's values.
	ProtectionMode ArvanCloudDdosProtectionMode

	// CaptchaService selects the CAPTCHA vendor. Meaningful — and required by
	// UpdateArvanCloudDdosSettings's own validation — only when ProtectionMode
	// is ArvanCloudDdosProtectionModeCaptcha; ignored otherwise.
	CaptchaService ArvanCloudCaptchaService

	// SiteKey is the CAPTCHA provider's public site key, registered by the
	// domain owner directly with the CAPTCHA vendor. Meaningful only when
	// ProtectionMode is ArvanCloudDdosProtectionModeCaptcha.
	SiteKey string

	// SecretKey is the CAPTCHA provider's secret key — sensitive, see this
	// type's doc comment above. Meaningful only when ProtectionMode is
	// ArvanCloudDdosProtectionModeCaptcha.
	SecretKey string

	// TTL is the challenge cookie's max-age, in seconds.
	TTL int

	// HTTPSOnly, when true, adds "SameSite=None; Secure" to the challenge's
	// set-cookie header.
	HTTPSOnly bool

	// Preflight controls the CORS-preflight response emitted alongside a
	// challenge.
	Preflight ArvanCloudDdosPreflight
}

// ArvanCloudDdosRuleAction is the action a DDoS rule takes when its
// url_pattern/sources match a request. Confirmed against DdosRule's "action"
// enum: exactly two values — "passthrough" and "protect" — the same spelling
// as domain.ArvanCloudWafRuleAction's two values, but a genuinely separate
// enum (see this file's package comment): "protect" here means "apply this
// domain's DDoS challenge (ProtectionMode) to matching requests", while
// "passthrough" means "exempt matching requests from the DDoS challenge
// entirely" — analogous in shape to WAF's protect/passthrough (apply vs.
// exempt from that capability's own protection), but gating a different
// mechanism (a challenge, not managed rule inspection). Do not conflate this
// type with domain.ArvanCloudWafRuleAction: nothing in the spec unifies them,
// and a future divergence in either enum's value set would silently break
// callers that assumed they were interchangeable.
type ArvanCloudDdosRuleAction string

const (
	// ArvanCloudDdosRuleActionProtect actively applies the domain's DDoS
	// challenge (ProtectionMode) to a matching request.
	ArvanCloudDdosRuleActionProtect ArvanCloudDdosRuleAction = "protect"
	// ArvanCloudDdosRuleActionPassthrough exempts a matching request from the
	// DDoS challenge entirely.
	ArvanCloudDdosRuleActionPassthrough ArvanCloudDdosRuleAction = "passthrough"
)

var arvanCloudDdosRuleActions = []string{
	string(ArvanCloudDdosRuleActionProtect),
	string(ArvanCloudDdosRuleActionPassthrough),
}

// ValidArvanCloudDdosRuleAction reports whether s is one of DdosRule.action's
// two values.
func ValidArvanCloudDdosRuleAction(s string) bool { return contains(arvanCloudDdosRuleActions, s) }

// ArvanCloudDdosRule is a domain's DDoS protection rule
// (/domains/{domain}/ddos/rules[/{id}], the DdosRule schema): a
// caller-authored exemption or enforcement layered on top of the domain's
// DDoS challenge settings.
type ArvanCloudDdosRule struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string

	// URLPattern is a glob pattern (not a regex — see GlobPattern in the
	// spec: `?`/`*`/`**`/`[...]`/`[!...]`) this rule matches a request's URL
	// against, e.g. "/wp-admin/**" or "/api/v?/users". Confirmed against
	// DdosRule.url_pattern's $ref to the spec's GlobPattern schema — the same
	// type WAF's url_pattern and Firewall-adjacent glob fields use.
	URLPattern string

	// Sources are the CIDR-format source addresses this rule matches
	// against, e.g. "203.0.113.0/24" — at most 20 entries per the spec
	// (maxItems: 20). Confirmed against DdosRule.sources: a plain array of
	// CIDR-format strings ($ref: CIDR), the SAME shape as
	// domain.ArvanCloudWafRule.Sources — not a Lists/#64 Dynamic Field
	// reference, matching (rather than diverging from) WAF's own confirmed
	// finding for its sources field.
	Sources []string

	// Description is a caller-supplied note about the rule.
	Description string

	// Action is what happens to a matching request: "protect" (apply the
	// domain's DDoS challenge) or "passthrough" (exempt from it). Must be one
	// of ValidArvanCloudDdosRuleAction's values.
	Action ArvanCloudDdosRuleAction

	// IsEnabled toggles the rule without deleting it.
	IsEnabled bool
}
