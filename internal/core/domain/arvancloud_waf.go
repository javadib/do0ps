package domain

// The types below model ArvanCloud's WAF (Web Application Firewall, issue
// #66): a managed rule-set engine (OWASP-style packages/presets a domain
// subscribes to) with its own custom-rule layer (waf.rules.*) layered on top
// of those packages. Confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "WAF" tag (the global.waf.*,
// waf.settings.*, waf.rules.*, waf.packages.*, waf.reconfigure,
// waf.reprioritize and waf.package.reprioritize operationIds) and the
// WafSettings/WafRule/WafRuleOutput/WafPackage/DomainWafPackage/WafPreset
// schemas.
//
// Naming collision warning (same shape as the Firewall/#65 warning in
// arvancloud_firewall.go): this is unrelated to the CDN edge-level L7
// Firewall (issue #65, filter_expr rules evaluated per-request at the edge).
// Both capabilities use the word "rule", but they are different products with
// different schemas: Firewall's FirewallRule.filter_expr is a free-form
// Wireshark-like expression the caller writes themselves, while WAF's
// protection comes primarily from subscribing a domain to a managed,
// OWASP-style package/preset (ReconfigureArvanCloudWaf) — WafRule below is
// only WAF's own thin custom-rule layer on top of those packages, narrower
// than a Firewall rule (url_pattern + sources only, no arbitrary boolean
// expression). A natural-language request like "block SQL injection
// attempts" should route to WAF (subscribing to an OWASP-style preset/package
// via ReconfigureArvanCloudWaf or InstallArvanCloudWafPackage), not to a
// hand-written Firewall filter_expr rule.

// ArvanCloudWafMode is a domain's WAF operating mode
// (WafSettings.mode).
//
// Wire-encoding finding (the riskiest ambiguity this issue calls out —
// confirmed by reading the spec text directly rather than guessing):
// docs/api-specs/arvancloud-cdn-4.0.yml line 9612-9614 declares
//
//	mode:
//	  type: string
//	  enum: [off, detect, protect]
//
// i.e. `mode` is a plain STRING enum of three values — "off", "detect",
// "protect" — with NO literal JSON boolean `false` appearing anywhere in the
// schema for this field (grep across the whole spec file confirms
// `enum: [off, detect, protect]` occurs exactly once, only here). The issue
// text's warning that "false is a literal boolean value in this enum, not
// the string \"false\"" does not match what the spec actually declares for
// WafSettings.mode: there is no boolean form of this field in the OpenAPI
// document at all, only the string "off". This type and its JSON tag
// therefore encode Mode as a plain Go string, sent/received verbatim as
// "off"/"detect"/"protect" — no special-casing for a boolean zero value is
// needed or would be correct here. If a future API revision is observed on
// the wire to actually emit a JSON `false` for this field despite what the
// committed spec says, that would be a runtime discovery to handle in the
// adapter's decode step (internal/adapters/providers/arvancloud/waf.go), not
// a reason to change this type's shape speculatively now.
type ArvanCloudWafMode string

const (
	// ArvanCloudWafModeOff disables WAF inspection for the domain entirely.
	ArvanCloudWafModeOff ArvanCloudWafMode = "off"
	// ArvanCloudWafModeDetect runs WAF inspection and logs matches without
	// blocking any request ("log-only").
	ArvanCloudWafModeDetect ArvanCloudWafMode = "detect"
	// ArvanCloudWafModeProtect runs WAF inspection and actively blocks
	// matching requests.
	ArvanCloudWafModeProtect ArvanCloudWafMode = "protect"
)

var arvanCloudWafModes = []string{
	string(ArvanCloudWafModeOff),
	string(ArvanCloudWafModeDetect),
	string(ArvanCloudWafModeProtect),
}

// ValidArvanCloudWafMode reports whether s is one of WafSettings.mode's three
// values.
func ValidArvanCloudWafMode(s string) bool { return contains(arvanCloudWafModes, s) }

// ArvanCloudWafLogRedactionReplacement is
// WafSettings.log_redaction.replacement_string's enum. Confirmed against the
// spec: the empty string is itself one of the four allowed values (meaning
// "replace with nothing", i.e. drop the redacted value entirely), not a
// stand-in for "field not set" the way an empty string is treated elsewhere
// in this codebase — so ValidArvanCloudWafLogRedactionReplacement accepts ""
// deliberately.
type ArvanCloudWafLogRedactionReplacement string

const (
	ArvanCloudWafLogRedactionReplacementStars  ArvanCloudWafLogRedactionReplacement = "****"
	ArvanCloudWafLogRedactionReplacementHashes ArvanCloudWafLogRedactionReplacement = "####"
	ArvanCloudWafLogRedactionReplacementLabel  ArvanCloudWafLogRedactionReplacement = "--REDACTED--"
	ArvanCloudWafLogRedactionReplacementEmpty  ArvanCloudWafLogRedactionReplacement = ""
)

var arvanCloudWafLogRedactionReplacements = []string{
	string(ArvanCloudWafLogRedactionReplacementStars),
	string(ArvanCloudWafLogRedactionReplacementHashes),
	string(ArvanCloudWafLogRedactionReplacementLabel),
	string(ArvanCloudWafLogRedactionReplacementEmpty),
}

// ValidArvanCloudWafLogRedactionReplacement reports whether s is one of
// log_redaction.replacement_string's four values, "" included.
func ValidArvanCloudWafLogRedactionReplacement(s string) bool {
	return contains(arvanCloudWafLogRedactionReplacements, s)
}

// ArvanCloudWafLogRedaction is WafSettings.log_redaction: which parts of a
// domain's WAF logs are redacted to protect sensitive information. Every
// field is optional on the wire (the spec gives each a default), so the zero
// value here means "use ArvanCloud's own defaults" on a create/update rather
// than "redact nothing" — see UpdateArvanCloudWafSettings's doc comment for
// how a caller distinguishes the two.
type ArvanCloudWafLogRedaction struct {
	// Cookies is a list of cookie names to redact. Defaults to empty (no
	// cookies redacted by name) per the spec.
	Cookies []string
	// Headers is a list of header names to redact. Defaults to
	// ["authorization", "proxy-authorization", "cookie"] per the spec.
	Headers []string
	// AllHeaders, when true, redacts every header regardless of Headers.
	AllHeaders bool
	// Body, when true, redacts the request body. Defaults to true per the
	// spec.
	Body bool
	// Records, when true, redacts specific log entries or entire log
	// records.
	Records bool
	// ReplacementString is what a redacted value is replaced with. Must be
	// one of ValidArvanCloudWafLogRedactionReplacement's values; defaults to
	// "--REDACTED--" per the spec.
	ReplacementString ArvanCloudWafLogRedactionReplacement
}

// ArvanCloudWafProvider identifies the vendor behind a WAF package
// (WafPackage.provider / WafPreset's nested package summary): a display name
// and a logo URL, both read-only.
type ArvanCloudWafProvider struct {
	Name string
	Logo string
}

// ArvanCloudWafPackage is a WAF managed rule bundle — either the global
// reference-data shape (WafPackage / WafPackageDetails, from
// GetArvanCloudWafPackage) or a domain's installed instance of one
// (DomainWafPackage / DomainWafPackageDetails, from the per-domain package
// subscription methods below), which the spec models as WafPackage plus two
// extra fields (Params, IsEnabled). Both shapes are flattened onto this one
// struct — the same "share one struct across a spec-level allOf" choice
// ArvanCloudFirewallRule's siblings make — since which fields are meaningful
// depends only on which method returned the value: Params and IsEnabled are
// always zero-valued/absent for a global (non-domain) package, and
// Rulesets is populated only by the "*Details" show endpoints
// (GetArvanCloudWafPackage, GetArvanCloudWafDomainPackage), not by the list
// endpoints (ListArvanCloudWafPresets' Packages, ListArvanCloudWafDomainPackages).
type ArvanCloudWafPackage struct {
	// ID is the package's provider-assigned identifier. For the global
	// package endpoints this is the packageId path segment; for a
	// domain-installed instance this is also what
	// GetArvanCloudWafDomainPackage/UpdateArvanCloudWafDomainPackage/
	// UninstallArvanCloudWafPackage address it by. Read-only.
	ID string
	// Name is the package's display name. Read-only.
	Name string
	// Provider is the vendor behind this package (e.g. an OWASP core rule
	// set provider). Read-only.
	Provider ArvanCloudWafProvider
	// ParamsSchema is the JSON Schema describing what Params accepts,
	// reported by the provider. Read-only.
	ParamsSchema map[string]any
	// DisabledRules are the package's own rule IDs (UUIDs) turned off within
	// it. Per the spec, when omitted on install this is filled with the
	// package's default disabled rules rather than left empty.
	DisabledRules []string
	// DisabledRulesets are the package's own ruleset IDs (UUIDs) turned off
	// within it, same "filled with the default when omitted" behavior as
	// DisabledRules.
	DisabledRulesets []string
	// Params holds a domain-installed instance's configuration values,
	// validated against ParamsSchema. Meaningful only for a domain-installed
	// package (DomainWafPackage); always empty for the global reference-data
	// shape.
	Params map[string]any
	// IsEnabled toggles a domain-installed package without uninstalling it.
	// Meaningful only for a domain-installed package; defaults to true per
	// the spec. Always false (zero value) for the global reference-data
	// shape — callers must not read IsEnabled off a value returned by
	// GetArvanCloudWafPackage/ListArvanCloudWafPresets as if it meant
	// anything.
	IsEnabled bool
	// Rulesets is the package's own rule groupings, populated only by the
	// "*Details" show endpoints (GetArvanCloudWafPackage,
	// GetArvanCloudWafDomainPackage) — nil from every list/install/update
	// call, which the spec does not echo it on.
	Rulesets []ArvanCloudWafRuleset
}

// ArvanCloudWafRulesetRule is one rule inside a ArvanCloudWafRuleset, as
// reported by a WAF package's rulesets detail.
type ArvanCloudWafRulesetRule struct {
	ID     string
	Name   string
	Params map[string]any
}

// ArvanCloudWafRuleset is one named grouping of rules inside a WAF package
// (WafRuleset), e.g. a single OWASP Core Rule Set category.
type ArvanCloudWafRuleset struct {
	ID    string
	Name  string
	Rules []ArvanCloudWafRulesetRule
}

// ArvanCloudWafPackageRule is one entry of GetArvanCloudWafPackageRules'
// result: a rule within a global WAF package. Confirmed against
// global.waf.show_package_rules' response shape — deliberately lighter than
// ArvanCloudWafRulesetRule (id and name only, no params), matching what that
// endpoint's schema actually declares.
type ArvanCloudWafPackageRule struct {
	ID   string
	Name string
}

// ArvanCloudWafPresetPackage is one entry of ArvanCloudWafPreset.Packages: a
// summary of a package a preset installs, confirmed against WafPreset's
// nested "packages" array (name + provider only, no package id — a preset
// entry is resolved to an installable package id via
// ArvanCloudWafPresetsAndPackages.Packages, matched by Name).
type ArvanCloudWafPresetPackage struct {
	Name     string
	Provider ArvanCloudWafProvider
}

// ArvanCloudWafPreset is one of ArvanCloud's predefined WAF configurations
// (WafPreset): a named bundle of packages a domain can be switched to in one
// call via ReconfigureArvanCloudWaf, which replaces every package currently
// installed on the domain with the preset's own set. This is the natural
// target for a request like "block SQL injection attempts" or "turn on
// OWASP-style protection" — see this file's package comment.
type ArvanCloudWafPreset struct {
	// ID is what ReconfigureArvanCloudWaf's presetID parameter identifies
	// this preset by.
	ID          string
	Name        string
	Description string
	Packages    []ArvanCloudWafPresetPackage
}

// ArvanCloudWafPresetsAndPackages is the result of ListArvanCloudWafPresets:
// the account's confirmed presets plus the full catalog of global WAF
// packages they draw from, matching WafPresets' own two-field shape
// (presets, packages).
type ArvanCloudWafPresetsAndPackages struct {
	Presets  []ArvanCloudWafPreset
	Packages []ArvanCloudWafPackage
}

// ArvanCloudWafSettings is a domain's WAF-wide configuration
// (/domains/{domain}/waf/settings, the WafSettings schema).
type ArvanCloudWafSettings struct {
	// IsEnabled is read-only: whether the domain's WAF is active at all
	// (distinct from Mode — the spec marks this readOnly, so it is reported
	// but never sent on update).
	IsEnabled bool
	// Mode is the domain's WAF operating mode. See ArvanCloudWafMode's doc
	// comment for the confirmed wire encoding.
	Mode ArvanCloudWafMode
	// LogRedaction controls which parts of the WAF logs are redacted.
	LogRedaction ArvanCloudWafLogRedaction
	// Packages is read-only: the packages currently configuring this
	// domain's WAF, as reported alongside settings. Managed independently
	// via the per-domain package subscription methods
	// (ListArvanCloudWafDomainPackages and friends) or in bulk via
	// ReconfigureArvanCloudWaf — never by sending Packages back on an update.
	Packages []ArvanCloudWafPackage
}

// ArvanCloudWafRuleAction is the action a WAF custom rule takes when its
// url_pattern/sources match a request. Confirmed against WafRule's "action"
// enum: exactly two values, narrower than
// ArvanCloudFirewallAction's four — do not conflate the two enums (this
// file's package comment).
type ArvanCloudWafRuleAction string

const (
	// ArvanCloudWafRuleActionProtect actively enforces WAF protection for a
	// matching request.
	ArvanCloudWafRuleActionProtect ArvanCloudWafRuleAction = "protect"
	// ArvanCloudWafRuleActionPassthrough exempts a matching request from WAF
	// inspection.
	ArvanCloudWafRuleActionPassthrough ArvanCloudWafRuleAction = "passthrough"
)

var arvanCloudWafRuleActions = []string{
	string(ArvanCloudWafRuleActionProtect),
	string(ArvanCloudWafRuleActionPassthrough),
}

// ValidArvanCloudWafRuleAction reports whether s is one of WafRule.action's
// two values. Anything else — including Firewall's "allow"/"deny"/"bypass"/
// "challenge" — is rejected.
func ValidArvanCloudWafRuleAction(s string) bool { return contains(arvanCloudWafRuleActions, s) }

// ArvanCloudWafRuleException is one entry of a WafRule's "exceptions": an
// exemption of specific numbered rules within one WAF package from this
// custom rule's effect. Confirmed against WafRuleExceptions (request side)
// and WafRuleExceptionsResponse (response side) — the two differ only in the
// shape of each id (a plain integer on write, an {id, name} object on read),
// which this struct flattens: RuleIDs is always populated (from either
// side), RuleNames is populated only when decoding a response and left nil
// when building a request.
type ArvanCloudWafRuleException struct {
	// Package is the name of the WAF package (e.g. a package's own Name)
	// whose numbered rules this exception applies to.
	Package string
	// RuleIDs are the package's own numbered rule ids to exempt. Required
	// content on both the request and response side.
	RuleIDs []int
	// RuleNames are the same rules' human-readable names, index-aligned with
	// RuleIDs. Populated only when decoding a response
	// (WafRuleExceptionsResponse); always nil when building a request, since
	// the API does not accept names on write.
	RuleNames []string
}

// ArvanCloudWafRule is a domain's WAF custom rule
// (/domains/{domain}/waf/rules[/{id}], the WafRule/WafRuleOutput schemas): a
// thin layer of caller-authored exceptions on top of the managed
// packages/presets a domain subscribes to — see this file's package comment
// for how this differs from the CDN edge Firewall's filter_expr rules.
type ArvanCloudWafRule struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string

	// URLPattern is a glob pattern (not a regex — see GlobPattern in the
	// spec: `?`/`*`/`**`/`[...]`/`[!...]`) this rule matches a request's URL
	// against, e.g. "/wp-admin/**" or "/api/v?/users". Confirmed against
	// WafRule.url_pattern's $ref to the spec's GlobPattern schema.
	URLPattern string

	// Sources are the CIDR-format source addresses this rule matches
	// against, e.g. "203.0.113.0/24" — at most 20 entries per the spec
	// (maxItems: 20). Confirmed against WafRule.sources: a plain array of
	// CIDR-format strings ($ref: CIDR), NOT a mix of raw IPs/countries/list
	// references as the issue speculated — this WAF rule shape has no
	// Dynamic Field ("Lists", issue #64) reference field and no country-code
	// item, unlike what a superficial reading of the issue's "read the exact
	// item shape from the spec" note might suggest. If a future WAF-adjacent
	// capability (DDoS, Rate Limiting) is found to accept a Dynamic Field
	// reference in its own sources-like field, that is a separate finding
	// for that capability's own issue, not evidence this field's shape is
	// wrong.
	Sources []string

	// Action is what happens to a matching request: "protect" (enforce WAF)
	// or "passthrough" (exempt from it). Must be one of
	// ValidArvanCloudWafRuleAction's values.
	Action ArvanCloudWafRuleAction

	// Description is a caller-supplied note about the rule.
	Description string

	// IsEnabled toggles the rule without deleting it.
	IsEnabled bool

	// Exceptions exempts specific numbered rules within named WAF packages
	// from this rule's effect.
	Exceptions []ArvanCloudWafRuleException
}
