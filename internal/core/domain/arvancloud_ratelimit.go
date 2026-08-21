package domain

// The types below model ArvanCloud's Rate Limiting capability (issue #68):
// per-domain request-rate settings plus a rule engine (rate-limiting.rules.*)
// that throttles or blocks traffic exceeding a configured rate — distinct
// from the CDN edge Firewall (issue #65), the WAF managed rule-set engine
// (issue #66) and the dedicated DDoS challenge engine (issue #67), same
// naming-collision caution as domain/arvancloud_ddos.go's package comment.
// Confirmed against docs/api-specs/arvancloud-cdn-4.0.yml's "Rate Limiting"
// tag (the rate-limiting.settings.*, rate-limiting.rules.* and
// rate-limiting.actions.reprioritize operationIds) and the
// RateLimitSettings/BaseRateLimitRule/RateLimitRule/ChallengeAction schemas.

// ArvanCloudRateLimitAction is the action a rate-limit rule takes once a
// source exceeds Rate requests per TimeDuration (BaseRateLimitRule.action).
// Confirmed against the spec: a plain string enum of exactly two values,
// default "block" — NOT the same value set as
// domain.ArvanCloudDdosRuleAction's "protect"/"passthrough" or
// domain.ArvanCloudWafRuleAction's equivalents, so this is its own type.
type ArvanCloudRateLimitAction string

const (
	// ArvanCloudRateLimitActionChallenge issues a challenge (per a rule's
	// ActionDetails) to a source once it exceeds the configured rate, rather
	// than blocking it outright.
	ArvanCloudRateLimitActionChallenge ArvanCloudRateLimitAction = "challenge"
	// ArvanCloudRateLimitActionBlock blocks a source outright, for
	// BlockDuration seconds, once it exceeds the configured rate. This is the
	// spec's own default when action is omitted.
	ArvanCloudRateLimitActionBlock ArvanCloudRateLimitAction = "block"
)

var arvanCloudRateLimitActions = []string{
	string(ArvanCloudRateLimitActionChallenge),
	string(ArvanCloudRateLimitActionBlock),
}

// ValidArvanCloudRateLimitAction reports whether s is one of
// BaseRateLimitRule.action's two values.
func ValidArvanCloudRateLimitAction(s string) bool { return contains(arvanCloudRateLimitActions, s) }

// ArvanCloudChallengeMode selects the challenge mechanism a rate-limit rule
// issues when its Action is ArvanCloudRateLimitActionChallenge
// (ChallengeAction.mode).
//
// Wire-encoding finding (checked directly against the spec, not assumed to
// mirror ArvanCloudDdosProtectionMode's string encoding):
// docs/api-specs/arvancloud-cdn-4.0.yml's ChallengeAction schema declares
//
//	mode:
//	  type: integer
//	  enum: [1, 2, 3]
//	  description: "The mode of mitigation (1: Cookie, 2: Javascript, 3: Captcha)"
//
// i.e. a plain INTEGER enum, not a string — unlike DDoS's protection_mode/
// ArvanCloudDdosProtectionMode, which is a plain string enum over the same
// three underlying mechanisms (plus its own "off" value). Do not conflate
// the two: sending "cookie" here instead of 1 would fail validation against
// the real API.
type ArvanCloudChallengeMode int

const (
	// ArvanCloudChallengeModeCookie challenges with a cookie-based check.
	ArvanCloudChallengeModeCookie ArvanCloudChallengeMode = 1
	// ArvanCloudChallengeModeJavaScript challenges with a JavaScript-execution
	// check.
	ArvanCloudChallengeModeJavaScript ArvanCloudChallengeMode = 2
	// ArvanCloudChallengeModeCaptcha challenges with a CAPTCHA.
	ArvanCloudChallengeModeCaptcha ArvanCloudChallengeMode = 3
)

// ValidArvanCloudChallengeMode reports whether m is one of
// ChallengeAction.mode's three values.
func ValidArvanCloudChallengeMode(m ArvanCloudChallengeMode) bool {
	return m == ArvanCloudChallengeModeCookie || m == ArvanCloudChallengeModeJavaScript || m == ArvanCloudChallengeModeCaptcha
}

// ArvanCloudChallengeAction is a rate-limit rule's challenge configuration
// (BaseRateLimitRule.action_details, the ChallengeAction schema), meaningful
// only when the rule's Action is ArvanCloudRateLimitActionChallenge.
type ArvanCloudChallengeAction struct {
	// Mode selects the challenge mechanism. Must be one of
	// ValidArvanCloudChallengeMode's values.
	Mode ArvanCloudChallengeMode

	// TTL is the challenge's max-age, in seconds (spec: 10-31536000).
	TTL int

	// HTTPSOnly, when true, adds "SameSite=None; Secure" to the challenge's
	// set-cookie header — the same meaning as
	// ArvanCloudDdosSettings.HTTPSOnly.
	HTTPSOnly bool
}

// ArvanCloudRateLimitSettings is a domain's rate-limiting configuration
// (/domains/{domain}/rate-limit/settings, the RateLimitSettings schema).
type ArvanCloudRateLimitSettings struct {
	// DDoSDetection turns on automatic rate-limit-based DDoS detection —
	// distinct from the dedicated DDoS protection module
	// (ArvanCloudDdosSettings), which has its own, independent challenge
	// engine and on/off switch.
	DDoSDetection bool

	// ExcludeSources are CIDR-format source addresses globally exempted from
	// every rate-limit rule on the domain, e.g. trusted IPs or load
	// balancers.
	ExcludeSources []string
}

// ArvanCloudRateLimitRule is a domain's rate-limiting rule
// (/domains/{domain}/rate-limit/rules[/{id}], the BaseRateLimitRule/
// RateLimitRule/RateLimitRuleView schemas): throttles or blocks traffic
// matching URLPattern once a source exceeds Rate requests per TimeDuration
// seconds.
type ArvanCloudRateLimitRule struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string

	// Action is what happens to a source once it exceeds the configured
	// rate: "challenge" (issue ActionDetails' challenge) or "block" (block
	// outright for BlockDuration seconds). Must be one of
	// ValidArvanCloudRateLimitAction's values.
	Action ArvanCloudRateLimitAction

	// IsEnabled toggles the rule without deleting it.
	IsEnabled bool

	// URLPattern is a glob pattern (not a regex — see GlobPattern in the
	// spec, the same type WAF/DDoS use) this rule matches a request's URL
	// against, e.g. "/api/**".
	URLPattern string

	// Description is a caller-supplied note about the rule.
	Description string

	// ExcludeSources are CIDR-format source addresses exempted from this
	// specific rule, layered on top of
	// ArvanCloudRateLimitSettings.ExcludeSources.
	ExcludeSources []string

	// Rate is the number of requests allowed per TimeDuration seconds.
	// Required; must be positive (spec: 1-4000000).
	Rate int

	// Burst is the number of requests allowed to briefly exceed Rate before
	// this rule takes effect (spec: 1-4000000). Optional — zero leaves it
	// unset on create/update, letting the provider apply its own default.
	Burst int

	// BlockDuration is how long, in seconds, a violating source is blocked
	// for when Action is ArvanCloudRateLimitActionBlock (spec: 0-86400).
	// Optional — zero leaves it unset, letting the provider apply its own
	// default.
	BlockDuration int

	// TimeDuration is the window, in seconds, Rate is measured over.
	// Required; must be positive (spec: 1-2592000).
	TimeDuration int

	// AllowedMethods restricts the rule to specific HTTP methods, e.g. only
	// rate-limit "POST". Empty means every method is covered.
	AllowedMethods []string

	// ActionDetails is this rule's challenge configuration. Meaningful only
	// when Action is ArvanCloudRateLimitActionChallenge; ignored otherwise.
	ActionDetails ArvanCloudChallengeAction
}
