package domain

// The types below model ArvanCloud's CDN edge-level L7 Firewall (issue #65):
// per-domain `filter_expr` rules evaluated at the edge, plus a
// domain-selectable variant of the same rule shape scoped to the account
// instead of one domain. Confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Firewall" and "Account Level
// Firewall" tags (the firewall.* and account.firewall_rules.* operationIds)
// and the FirewallRule/AccountFirewallRule/FirewallSettings schemas.
//
// Naming collision warning (AGENTS.md 4.1/4.5, same shape as Parspack's):
// this is ArvanCloud's CDN edge-level firewall, unrelated to any future
// ArvanCloud IaaS/cloud-server firewall (a separate product, a separate
// OpenAPI spec, out of scope for phase 2 entirely — see issue #65's context
// note). Do not let a future IaaS FirewallRule name collide with this one.

// ArvanCloudFirewallAction is the action a domain-level or account-level
// firewall rule takes when its filter_expr matches a request. Confirmed
// against BaseFirewallRule's "action" enum. Note this has one fewer value
// than ArvanCloudFirewallDefaultAction below — "drop" is valid only as a
// firewall settings default_action, never as a per-rule action — so the two
// enums are validated independently (issue #65's acceptance criteria).
type ArvanCloudFirewallAction string

const (
	ArvanCloudFirewallActionAllow     ArvanCloudFirewallAction = "allow"
	ArvanCloudFirewallActionDeny      ArvanCloudFirewallAction = "deny"
	ArvanCloudFirewallActionBypass    ArvanCloudFirewallAction = "bypass"
	ArvanCloudFirewallActionChallenge ArvanCloudFirewallAction = "challenge"
)

var arvanCloudFirewallActions = []string{
	string(ArvanCloudFirewallActionAllow),
	string(ArvanCloudFirewallActionDeny),
	string(ArvanCloudFirewallActionBypass),
	string(ArvanCloudFirewallActionChallenge),
}

// ValidArvanCloudFirewallAction reports whether s is one of the actions a
// domain-level or account-level firewall rule accepts. "drop" is
// deliberately not included here — see ArvanCloudFirewallDefaultAction.
func ValidArvanCloudFirewallAction(s string) bool { return contains(arvanCloudFirewallActions, s) }

// ArvanCloudFirewallDefaultAction is the fallback action a domain's firewall
// settings apply to a request that no rule matched. Confirmed against
// BaseFirewallSettings' "default_action" enum: the same four values as
// ArvanCloudFirewallAction, plus "drop" (silently discard the request,
// without even the response a "deny" gives) — valid here but not as a
// per-rule action.
type ArvanCloudFirewallDefaultAction string

const (
	ArvanCloudFirewallDefaultActionAllow     ArvanCloudFirewallDefaultAction = "allow"
	ArvanCloudFirewallDefaultActionDeny      ArvanCloudFirewallDefaultAction = "deny"
	ArvanCloudFirewallDefaultActionDrop      ArvanCloudFirewallDefaultAction = "drop"
	ArvanCloudFirewallDefaultActionBypass    ArvanCloudFirewallDefaultAction = "bypass"
	ArvanCloudFirewallDefaultActionChallenge ArvanCloudFirewallDefaultAction = "challenge"
)

var arvanCloudFirewallDefaultActions = []string{
	string(ArvanCloudFirewallDefaultActionAllow),
	string(ArvanCloudFirewallDefaultActionDeny),
	string(ArvanCloudFirewallDefaultActionDrop),
	string(ArvanCloudFirewallDefaultActionBypass),
	string(ArvanCloudFirewallDefaultActionChallenge),
}

// ValidArvanCloudFirewallDefaultAction reports whether s is one of the
// actions firewall settings' default_action accepts — independent of
// ValidArvanCloudFirewallAction, per this file's package comment.
func ValidArvanCloudFirewallDefaultAction(s string) bool {
	return contains(arvanCloudFirewallDefaultActions, s)
}

// ArvanCloudDomainSelectionType controls which of an account's domains an
// account-level firewall rule applies to. Confirmed against
// AccountFirewallDomainSelection's enum and description.
type ArvanCloudDomainSelectionType string

const (
	// ArvanCloudDomainSelectionAll applies the rule to every enterprise
	// domain on the account. DomainIDs is not required in this case.
	ArvanCloudDomainSelectionAll ArvanCloudDomainSelectionType = "all"
	// ArvanCloudDomainSelectionInclude applies the rule only to the domains
	// listed in DomainIDs, which must be non-empty.
	ArvanCloudDomainSelectionInclude ArvanCloudDomainSelectionType = "include"
	// ArvanCloudDomainSelectionExclude applies the rule to every enterprise
	// domain on the account except those listed in DomainIDs, which must be
	// non-empty.
	ArvanCloudDomainSelectionExclude ArvanCloudDomainSelectionType = "exclude"
)

var arvanCloudDomainSelectionTypes = []string{
	string(ArvanCloudDomainSelectionAll),
	string(ArvanCloudDomainSelectionInclude),
	string(ArvanCloudDomainSelectionExclude),
}

// ValidArvanCloudDomainSelectionType reports whether s is one of the domain
// selection modes an account-level firewall rule accepts.
func ValidArvanCloudDomainSelectionType(s string) bool {
	return contains(arvanCloudDomainSelectionTypes, s)
}

// ArvanCloudFirewallActionDetails carries the extra configuration a "bypass"
// or "challenge" action needs, flattened onto one struct the same way
// ArvanCloudDNSRecordValue flattens its 13-type union (see that type's doc
// comment for the rationale): the spec's FirewallActionDetails is a oneOf of
// BypassAction and ChallengeAction, and which half of these fields is
// meaningful is determined entirely by the containing rule's Action (or, for
// firewall settings, its DefaultAction) — never both at once, and never
// meaningful at all for "allow" or "deny". The zero value means "no
// bypass/challenge configuration given", matching every other flattened
// optional struct in this package (e.g. ArvanCloudDNSRecordIPFilterMode).
type ArvanCloudFirewallActionDetails struct {
	// BypassRateLimit is BypassAction's "rlimit": whether matching requests
	// skip rate limiting. Meaningful only when Action/DefaultAction is
	// "bypass".
	BypassRateLimit bool
	// BypassChallengeCheck is BypassAction's "challenge": whether matching
	// requests skip any active challenge. Meaningful only for "bypass".
	BypassChallengeCheck bool
	// BypassWAF is BypassAction's "waf": whether matching requests skip WAF
	// inspection. Meaningful only for "bypass".
	BypassWAF bool

	// ChallengeMode is ChallengeAction's "mode": 1 (cookie), 2 (javascript)
	// or 3 (captcha). Meaningful only when Action/DefaultAction is
	// "challenge".
	ChallengeMode int
	// ChallengeTTL is ChallengeAction's "ttl", in seconds (10-31536000): how
	// long a passed challenge is remembered before it is asked again.
	// Meaningful only for "challenge".
	ChallengeTTL int
	// ChallengeHTTPSOnly is ChallengeAction's "https_only". Meaningful only
	// for "challenge".
	ChallengeHTTPSOnly bool
}

// ArvanCloudFirewallRule is a domain-level firewall rule
// (/domains/{domain}/firewall/rules[/{id}], the FirewallRule/
// FirewallRuleUpdate/FirewallRuleView schemas). filter_expr is a
// Wireshark-like filter expression evaluated at the edge — free-form, and
// deliberately not validated client-side (see FilterExpr's doc comment).
type ArvanCloudFirewallRule struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string

	// Name is a caller-chosen label for the rule. Required on create.
	Name string

	// FilterExpr is the Wireshark-like filter expression this rule matches
	// requests against, e.g. `ip.geoip.country in {"IR" "TH" "US"} and ssl`.
	// Required on create, 3-5000 chars per the spec. This is intentionally
	// not validated for syntax here: the expression language is free-form
	// and provider-defined, so a malformed one is left to surface as
	// whatever 422 the provider itself returns (issue #65's scope note,
	// mirroring how DNS's filter_expr-adjacent free-form fields are
	// handled).
	FilterExpr string

	// Action is what happens to a matching request. Required on create; must
	// be one of ValidArvanCloudFirewallAction's values ("drop" is not
	// allowed here — see ArvanCloudFirewallDefaultAction).
	Action ArvanCloudFirewallAction

	// Priority orders rule evaluation, lower first. Assigned by the provider
	// on create unless reordered via ReprioritizeArvanCloudFirewallRules.
	Priority int

	// IsEnabled toggles the rule without deleting it.
	IsEnabled bool

	// Note is a caller-supplied comment about the rule, empty when none was
	// given.
	Note string

	// IsAccountLevel is read-only: true when this rule (as returned by a
	// listing call) actually originates from the account-level firewall
	// rather than this domain's own rule set. Never sent on create/update.
	IsAccountLevel bool

	// ActionDetails carries the extra configuration a "bypass" or
	// "challenge" Action needs. Zero value for "allow"/"deny".
	ActionDetails ArvanCloudFirewallActionDetails
}

// ArvanCloudFirewallSettings is a domain's firewall-wide configuration
// (/domains/{domain}/firewall/settings, the FirewallSettings/
// FirewallSettingsView schemas): the fallback behavior applied to a request
// no rule matched, plus a few firewall-wide toggles.
type ArvanCloudFirewallSettings struct {
	// IsEnabled is read-only: whether the domain's firewall is active at
	// all.
	IsEnabled bool

	// DefaultAction is applied to a request no rule matched. Must be one of
	// ValidArvanCloudFirewallDefaultAction's values — unlike a per-rule
	// Action, "drop" is allowed here.
	DefaultAction ArvanCloudFirewallDefaultAction

	// VerifySNI, when true, requires a TLS request's SNI and Host header to
	// match. Defaults to true per the spec.
	VerifySNI bool

	// SkipGlobalWhitelist, when true, skips ArvanCloud's own global
	// whitelist for this domain.
	SkipGlobalWhitelist bool

	// SkipGlobalFirewall, when true, skips ArvanCloud's own global firewall
	// rules for this domain.
	SkipGlobalFirewall bool

	// DefaultActionDetails carries the extra configuration DefaultAction
	// needs when it is "bypass" or "challenge". Zero value otherwise.
	DefaultActionDetails ArvanCloudFirewallActionDetails
}

// ArvanCloudAccountFirewallRule is an account-level firewall rule
// (/account/firewall-rules[/{accountFirewallRule}], the AccountFirewallRule/
// FirewallRuleUpdate/FirewallRuleView schemas): the same rule shape as
// ArvanCloudFirewallRule, but applied across a settable subset of the
// account's domains instead of one domain picked by the URL path.
type ArvanCloudAccountFirewallRule struct {
	// ID is the provider-assigned UUID. Read-only; empty until created.
	ID string

	// Name is a caller-chosen label for the rule. Required on create.
	Name string

	// FilterExpr is the Wireshark-like filter expression this rule matches
	// requests against. Required on create; see
	// ArvanCloudFirewallRule.FilterExpr's doc comment — the same "not
	// validated client-side" rule applies here.
	FilterExpr string

	// Action is what happens to a matching request. Required on create; see
	// ArvanCloudFirewallRule.Action — "drop" is not allowed here either.
	Action ArvanCloudFirewallAction

	// Priority orders rule evaluation, lower first.
	Priority int

	// IsEnabled toggles the rule without deleting it.
	IsEnabled bool

	// Note is a caller-supplied comment about the rule, empty when none was
	// given.
	Note string

	// IsAccountLevel is read-only: expected true for every rule returned
	// through this port's account-level methods. Never sent on
	// create/update.
	IsAccountLevel bool

	// ActionDetails carries the extra configuration a "bypass" or
	// "challenge" Action needs. Zero value for "allow"/"deny".
	ActionDetails ArvanCloudFirewallActionDetails

	// DomainSelectionType and DomainIDs together determine which of the
	// account's domains this rule applies to. Both are required on create
	// (AccountFirewallRule's required list). When DomainSelectionType is
	// "include" or "exclude", DomainIDs must be non-empty — validated
	// client-side before this ever reaches the provider (issue #65's
	// acceptance criteria) — since an empty DomainIDs under either mode is
	// either a no-op the caller almost certainly did not intend
	// ("include" with nothing listed matches no domain) or ambiguous
	// ("exclude" with nothing listed is indistinguishable from "all", but
	// spelled a confusing way).
	DomainSelectionType ArvanCloudDomainSelectionType
	DomainIDs           []string
}

// ArvanCloudAccountFirewallValidDomain is one entry of
// ListArvanCloudAccountFirewallValidDomains' result: an active enterprise
// domain on the account that account-level firewall rules may target.
// Confirmed against the AccountFirewallRuleValidDomain schema.
type ArvanCloudAccountFirewallValidDomain struct {
	// ID is the domain's UUID — what an ArvanCloudAccountFirewallRule's
	// DomainIDs entries reference.
	ID string
	// Name is the domain's hostname, e.g. "example.com".
	Name string
}
