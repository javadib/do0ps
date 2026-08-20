package arvancloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Domain-level and account-level Firewall (issue #65), wired to the real CDN
// API. Domain-level base path is confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Firewall" tag, relative to
// domainPath (defined in domain.go) — i.e.
// https://napi.arvancloud.ir/cdn/4.0/domains/{domain}/firewall/... .
// Account-level base path is confirmed against the "Account Level Firewall"
// tag, relative to Client.baseURL — i.e.
// https://napi.arvancloud.ir/cdn/4.0/account/firewall-rules... .
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real ArvanCloud responses correctly. Nothing above
// the adapter boundary ever sees them — every method here translates to/from
// internal/core/domain types.

const (
	firewallSettingsPathSuffix = "/firewall/settings"
	firewallRulesPathSuffix    = "/firewall/rules"
	firewallReprioritizePath   = "/firewall/actions/reprioritize"

	accountFirewallRulesBasePath         = "account/firewall-rules"
	accountFirewallValidDomainsPath      = accountFirewallRulesBasePath + "/domains"
	accountFirewallRulesReprioritizePath = accountFirewallRulesBasePath + "/reprioritize"
)

func firewallSettingsPath(domainName string) string {
	return domainPath(domainName) + firewallSettingsPathSuffix
}

func firewallRulesPath(domainName string) string {
	return domainPath(domainName) + firewallRulesPathSuffix
}

func firewallRulePath(domainName, id string) string {
	return firewallRulesPath(domainName) + "/" + id
}

func accountFirewallRulePath(id string) string {
	return accountFirewallRulesBasePath + "/" + id
}

func accountFirewallRuleDomainsPath(id string) string {
	return accountFirewallRulePath(id) + "/domains"
}

// --- action_details wire shapes --------------------------------------------
//
// The spec's FirewallActionDetails is a oneOf of BypassAction and
// ChallengeAction, discriminated not by a field of its own but by whichever
// action the containing rule/settings resource carries. Encoding a request
// therefore picks one of the two narrow wire shapes below based on the
// rule's own action, so a "bypass" rule never sends challenge-only fields
// (mode/ttl/https_only) and vice versa. Decoding a response is tolerant: the
// combined struct only has the fields either shape can produce, and an
// absent field simply decodes to its zero value — which
// domain.ArvanCloudFirewallActionDetails already treats as "not meaningful
// for this action" (see that type's doc comment).

// bypassActionDetailsWire mirrors BypassAction, sent only when the
// containing rule's action is "bypass".
type bypassActionDetailsWire struct {
	Rlimit    bool `json:"rlimit,omitempty"`
	Challenge bool `json:"challenge,omitempty"`
	Waf       bool `json:"waf,omitempty"`
}

// challengeActionDetailsWire mirrors ChallengeAction, sent only when the
// containing rule's action is "challenge".
type challengeActionDetailsWire struct {
	Mode      int  `json:"mode,omitempty"`
	TTL       int  `json:"ttl,omitempty"`
	HTTPSOnly bool `json:"https_only,omitempty"`
}

// actionDetailsWireForRequest builds the action_details value to send on a
// create/update request, based on the rule's own action — nil (field
// omitted) for any action other than "bypass"/"challenge", since
// FirewallActionDetails is nullable and meaningless for "allow"/"deny".
func actionDetailsWireForRequest(action string, d domain.ArvanCloudFirewallActionDetails) any {
	switch action {
	case string(domain.ArvanCloudFirewallActionBypass):
		return bypassActionDetailsWire{Rlimit: d.BypassRateLimit, Challenge: d.BypassChallengeCheck, Waf: d.BypassWAF}
	case string(domain.ArvanCloudFirewallActionChallenge):
		return challengeActionDetailsWire{Mode: d.ChallengeMode, TTL: d.ChallengeTTL, HTTPSOnly: d.ChallengeHTTPSOnly}
	default:
		return nil
	}
}

// firewallActionDetailsResponseWire is the tolerant combined shape used to
// decode a response's action_details, whichever of the two request shapes
// produced it (see this file's action_details section comment).
type firewallActionDetailsResponseWire struct {
	Rlimit    bool `json:"rlimit"`
	Challenge bool `json:"challenge"`
	Waf       bool `json:"waf"`
	Mode      int  `json:"mode"`
	TTL       int  `json:"ttl"`
	HTTPSOnly bool `json:"https_only"`
}

func toActionDetailsDomain(raw json.RawMessage) domain.ArvanCloudFirewallActionDetails {
	if len(raw) == 0 || string(raw) == "null" {
		return domain.ArvanCloudFirewallActionDetails{}
	}
	var w firewallActionDetailsResponseWire
	if json.Unmarshal(raw, &w) != nil {
		return domain.ArvanCloudFirewallActionDetails{}
	}
	return domain.ArvanCloudFirewallActionDetails{
		BypassRateLimit:      w.Rlimit,
		BypassChallengeCheck: w.Challenge,
		BypassWAF:            w.Waf,
		ChallengeMode:        w.Mode,
		ChallengeTTL:         w.TTL,
		ChallengeHTTPSOnly:   w.HTTPSOnly,
	}
}

// --- Firewall Settings -------------------------------------------------

// firewallSettingsWire mirrors BaseFirewallSettings plus
// default_action_details, decode-only: a PATCH request is built separately
// (firewallSettingsRequestBody) as a plain map, since an explicit `false` on
// a boolean toggle must reach the provider rather than being dropped by
// encoding/json's omitempty.
type firewallSettingsWire struct {
	IsEnabled            bool            `json:"is_enabled"`
	DefaultAction        string          `json:"default_action"`
	VerifySNI            bool            `json:"verify_sni"`
	SkipGlobalWhitelist  bool            `json:"skip_global_whitelist"`
	SkipGlobalFirewall   bool            `json:"skip_global_firewall"`
	DefaultActionDetails json.RawMessage `json:"default_action_details"`
}

func toFirewallSettingsDomain(w firewallSettingsWire) domain.ArvanCloudFirewallSettings {
	return domain.ArvanCloudFirewallSettings{
		IsEnabled:            w.IsEnabled,
		DefaultAction:        domain.ArvanCloudFirewallDefaultAction(w.DefaultAction),
		VerifySNI:            w.VerifySNI,
		SkipGlobalWhitelist:  w.SkipGlobalWhitelist,
		SkipGlobalFirewall:   w.SkipGlobalFirewall,
		DefaultActionDetails: toActionDetailsDomain(w.DefaultActionDetails),
	}
}

// firewallSettingsRequestBody builds the JSON body for a firewall settings
// PATCH.
func firewallSettingsRequestBody(settings domain.ArvanCloudFirewallSettings) map[string]any {
	body := map[string]any{
		"default_action":        string(settings.DefaultAction),
		"verify_sni":            settings.VerifySNI,
		"skip_global_whitelist": settings.SkipGlobalWhitelist,
		"skip_global_firewall":  settings.SkipGlobalFirewall,
	}
	if details := actionDetailsWireForRequest(string(settings.DefaultAction), settings.DefaultActionDetails); details != nil {
		body["default_action_details"] = details
	}
	return body
}

// GetArvanCloudFirewallSettings returns domainName's firewall configuration.
func (p *Provider) GetArvanCloudFirewallSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudFirewallSettings, error) {
	var wire firewallSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, firewallSettingsPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud firewall settings for domain %q: %w", domainName, err)
	}
	settings := toFirewallSettingsDomain(wire)
	return &settings, nil
}

// UpdateArvanCloudFirewallSettings changes domainName's firewall
// configuration and returns it as stored afterward.
func (p *Provider) UpdateArvanCloudFirewallSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudFirewallSettings) (*domain.ArvanCloudFirewallSettings, error) {
	body := firewallSettingsRequestBody(settings)
	var wire firewallSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, firewallSettingsPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud firewall settings for domain %q: %w", domainName, err)
	}
	updated := toFirewallSettingsDomain(wire)
	return &updated, nil
}

// --- Firewall Rules (domain-level) --------------------------------------

// firewallRuleWire mirrors BaseFirewallRule plus action_details, shared by
// FirewallRule/FirewallRuleUpdate (request) and FirewallRuleView (response).
type firewallRuleWire struct {
	ID             string          `json:"id,omitempty"`
	Name           string          `json:"name,omitempty"`
	FilterExpr     string          `json:"filter_expr,omitempty"`
	Action         string          `json:"action,omitempty"`
	Priority       int             `json:"priority,omitempty"`
	IsEnabled      *bool           `json:"is_enabled,omitempty"`
	Note           string          `json:"note,omitempty"`
	IsAccountLevel bool            `json:"is_account_level,omitempty"`
	ActionDetails  json.RawMessage `json:"action_details,omitempty"`
}

func toFirewallRuleDomain(w firewallRuleWire) domain.ArvanCloudFirewallRule {
	rule := domain.ArvanCloudFirewallRule{
		ID:             w.ID,
		Name:           w.Name,
		FilterExpr:     w.FilterExpr,
		Action:         domain.ArvanCloudFirewallAction(w.Action),
		Priority:       w.Priority,
		Note:           w.Note,
		IsAccountLevel: w.IsAccountLevel,
		ActionDetails:  toActionDetailsDomain(w.ActionDetails),
	}
	if w.IsEnabled != nil {
		rule.IsEnabled = *w.IsEnabled
	}
	return rule
}

// firewallRuleRequestBody builds the JSON body for a firewall rule
// create/update, sharing the same field set for both since FirewallRule and
// FirewallRuleUpdate declare identical properties (the only difference is
// which fields the spec marks required, already enforced at the app layer).
func firewallRuleRequestBody(rule domain.ArvanCloudFirewallRule) map[string]any {
	body := map[string]any{
		"name":        rule.Name,
		"filter_expr": rule.FilterExpr,
		"action":      string(rule.Action),
		"is_enabled":  rule.IsEnabled,
	}
	if rule.Priority > 0 {
		body["priority"] = rule.Priority
	}
	if rule.Note != "" {
		body["note"] = rule.Note
	}
	if details := actionDetailsWireForRequest(string(rule.Action), rule.ActionDetails); details != nil {
		body["action_details"] = details
	}
	return body
}

// ListArvanCloudFirewallRules returns every domain-level rule configured for
// domainName, unfiltered — matching ListArvanCloudDynamicFields' own choice
// to keep listing simple (issue #64).
func (p *Provider) ListArvanCloudFirewallRules(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudFirewallRule, error) {
	var items []firewallRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, firewallRulesPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud firewall rules of domain %q: %w", domainName, err)
	}
	rules := make([]domain.ArvanCloudFirewallRule, len(items))
	for i := range items {
		rules[i] = toFirewallRuleDomain(items[i])
	}
	return rules, nil
}

// CreateArvanCloudFirewallRule creates a new domain-level rule.
func (p *Provider) CreateArvanCloudFirewallRule(ctx context.Context, creds domain.ProviderCredentials, domainName string, rule domain.ArvanCloudFirewallRule) (*domain.ArvanCloudFirewallRule, error) {
	body := firewallRuleRequestBody(rule)
	var wire firewallRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, firewallRulesPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud firewall rule %q on domain %q: %w", rule.Name, domainName, err)
	}
	created := toFirewallRuleDomain(wire)
	return &created, nil
}

// GetArvanCloudFirewallRule returns a single domain-level rule by id.
func (p *Provider) GetArvanCloudFirewallRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudFirewallRule, error) {
	var wire firewallRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, firewallRulePath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud firewall rule %q on domain %q: %w", id, domainName, err)
	}
	found := toFirewallRuleDomain(wire)
	return &found, nil
}

// UpdateArvanCloudFirewallRule updates a domain-level rule and returns it as
// stored afterward.
func (p *Provider) UpdateArvanCloudFirewallRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rule domain.ArvanCloudFirewallRule) (*domain.ArvanCloudFirewallRule, error) {
	body := firewallRuleRequestBody(rule)
	var wire firewallRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, firewallRulePath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud firewall rule %q on domain %q: %w", id, domainName, err)
	}
	updated := toFirewallRuleDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudFirewallRule removes a domain-level rule by id.
func (p *Provider) DeleteArvanCloudFirewallRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, firewallRulePath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud firewall rule %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// reprioritizeRuleRequestWire mirrors ReprioritizeRuleRequest.
type reprioritizeRuleRequestWire struct {
	RuleID       string `json:"rule_id"`
	AfterRuleID  string `json:"after_rule_id,omitempty"`
	BeforeRuleID string `json:"before_rule_id,omitempty"`
}

// ReprioritizeArvanCloudFirewallRules moves ruleID relative to
// afterRuleID/beforeRuleID within domainName's rule set. The endpoint's
// response carries no data to translate — only a confirmation message — so
// there is nothing for this method to return but the error.
func (p *Provider) ReprioritizeArvanCloudFirewallRules(ctx context.Context, creds domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error {
	body := reprioritizeRuleRequestWire{RuleID: ruleID, AfterRuleID: afterRuleID, BeforeRuleID: beforeRuleID}
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+firewallReprioritizePath, body, nil); err != nil {
		return fmt.Errorf("reprioritizing arvancloud firewall rule %q on domain %q: %w", ruleID, domainName, err)
	}
	return nil
}

// --- Firewall Rules (account-level) --------------------------------------

// accountFirewallRuleWire mirrors BaseFirewallRule plus action_details,
// domain_selection_type and domain_ids — the AccountFirewallRule/
// FirewallRuleUpdate/FirewallRuleView shapes for account-level rules.
type accountFirewallRuleWire struct {
	ID                  string          `json:"id,omitempty"`
	Name                string          `json:"name,omitempty"`
	FilterExpr          string          `json:"filter_expr,omitempty"`
	Action              string          `json:"action,omitempty"`
	Priority            int             `json:"priority,omitempty"`
	IsEnabled           *bool           `json:"is_enabled,omitempty"`
	Note                string          `json:"note,omitempty"`
	IsAccountLevel      bool            `json:"is_account_level,omitempty"`
	ActionDetails       json.RawMessage `json:"action_details,omitempty"`
	DomainSelectionType string          `json:"domain_selection_type,omitempty"`
	DomainIDs           []string        `json:"domain_ids,omitempty"`
}

func toAccountFirewallRuleDomain(w accountFirewallRuleWire) domain.ArvanCloudAccountFirewallRule {
	rule := domain.ArvanCloudAccountFirewallRule{
		ID:                  w.ID,
		Name:                w.Name,
		FilterExpr:          w.FilterExpr,
		Action:              domain.ArvanCloudFirewallAction(w.Action),
		Priority:            w.Priority,
		Note:                w.Note,
		IsAccountLevel:      w.IsAccountLevel,
		ActionDetails:       toActionDetailsDomain(w.ActionDetails),
		DomainSelectionType: domain.ArvanCloudDomainSelectionType(w.DomainSelectionType),
		DomainIDs:           w.DomainIDs,
	}
	if w.IsEnabled != nil {
		rule.IsEnabled = *w.IsEnabled
	}
	return rule
}

func accountFirewallRuleRequestBody(rule domain.ArvanCloudAccountFirewallRule) map[string]any {
	body := map[string]any{
		"name":                  rule.Name,
		"filter_expr":           rule.FilterExpr,
		"action":                string(rule.Action),
		"is_enabled":            rule.IsEnabled,
		"domain_selection_type": string(rule.DomainSelectionType),
	}
	if rule.Priority > 0 {
		body["priority"] = rule.Priority
	}
	if rule.Note != "" {
		body["note"] = rule.Note
	}
	if len(rule.DomainIDs) > 0 {
		body["domain_ids"] = rule.DomainIDs
	}
	if details := actionDetailsWireForRequest(string(rule.Action), rule.ActionDetails); details != nil {
		body["action_details"] = details
	}
	return body
}

// accountFirewallValidDomainWire mirrors AccountFirewallRuleValidDomain.
type accountFirewallValidDomainWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListArvanCloudAccountFirewallValidDomains returns the account's active
// enterprise domains eligible to be targeted by an account-level rule.
func (p *Provider) ListArvanCloudAccountFirewallValidDomains(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudAccountFirewallValidDomain, error) {
	var items []accountFirewallValidDomainWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, accountFirewallValidDomainsPath, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud account firewall valid domains: %w", err)
	}
	domains := make([]domain.ArvanCloudAccountFirewallValidDomain, len(items))
	for i, w := range items {
		domains[i] = domain.ArvanCloudAccountFirewallValidDomain{ID: w.ID, Name: w.Name}
	}
	return domains, nil
}

// ListArvanCloudAccountFirewallRules returns every account-level rule,
// unfiltered.
func (p *Provider) ListArvanCloudAccountFirewallRules(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudAccountFirewallRule, error) {
	var items []accountFirewallRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, accountFirewallRulesBasePath, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud account firewall rules: %w", err)
	}
	rules := make([]domain.ArvanCloudAccountFirewallRule, len(items))
	for i := range items {
		rules[i] = toAccountFirewallRuleDomain(items[i])
	}
	return rules, nil
}

// CreateArvanCloudAccountFirewallRule creates a new account-level rule.
func (p *Provider) CreateArvanCloudAccountFirewallRule(ctx context.Context, creds domain.ProviderCredentials, rule domain.ArvanCloudAccountFirewallRule) (*domain.ArvanCloudAccountFirewallRule, error) {
	body := accountFirewallRuleRequestBody(rule)
	var wire accountFirewallRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, accountFirewallRulesBasePath, body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud account firewall rule %q: %w", rule.Name, err)
	}
	created := toAccountFirewallRuleDomain(wire)
	return &created, nil
}

// GetArvanCloudAccountFirewallRule returns a single account-level rule by id.
func (p *Provider) GetArvanCloudAccountFirewallRule(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.ArvanCloudAccountFirewallRule, error) {
	var wire accountFirewallRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, accountFirewallRulePath(id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud account firewall rule %q: %w", id, err)
	}
	found := toAccountFirewallRuleDomain(wire)
	return &found, nil
}

// UpdateArvanCloudAccountFirewallRule updates an account-level rule and
// returns it as stored afterward.
func (p *Provider) UpdateArvanCloudAccountFirewallRule(ctx context.Context, creds domain.ProviderCredentials, id string, rule domain.ArvanCloudAccountFirewallRule) (*domain.ArvanCloudAccountFirewallRule, error) {
	body := accountFirewallRuleRequestBody(rule)
	var wire accountFirewallRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, accountFirewallRulePath(id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud account firewall rule %q: %w", id, err)
	}
	updated := toAccountFirewallRuleDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudAccountFirewallRule removes an account-level rule by id.
func (p *Provider) DeleteArvanCloudAccountFirewallRule(ctx context.Context, creds domain.ProviderCredentials, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, accountFirewallRulePath(id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud account firewall rule %q: %w", id, err)
	}
	return nil
}

// accountFirewallRuleDomainsRequestWire mirrors
// AccountFirewallRuleDomainsRequest, shared by attach and detach.
type accountFirewallRuleDomainsRequestWire struct {
	DomainIDs []string `json:"domain_ids"`
}

// AttachArvanCloudAccountFirewallDomains adds domainIDs to rule id's target
// set.
func (p *Provider) AttachArvanCloudAccountFirewallDomains(ctx context.Context, creds domain.ProviderCredentials, id string, domainIDs []string) (*domain.ArvanCloudAccountFirewallRule, error) {
	body := accountFirewallRuleDomainsRequestWire{DomainIDs: domainIDs}
	var wire accountFirewallRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, accountFirewallRuleDomainsPath(id), body, &wire); err != nil {
		return nil, fmt.Errorf("attaching domains to arvancloud account firewall rule %q: %w", id, err)
	}
	updated := toAccountFirewallRuleDomain(wire)
	return &updated, nil
}

// DetachArvanCloudAccountFirewallDomains removes domainIDs from rule id's
// target set.
func (p *Provider) DetachArvanCloudAccountFirewallDomains(ctx context.Context, creds domain.ProviderCredentials, id string, domainIDs []string) (*domain.ArvanCloudAccountFirewallRule, error) {
	body := accountFirewallRuleDomainsRequestWire{DomainIDs: domainIDs}
	var wire accountFirewallRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, accountFirewallRuleDomainsPath(id), body, &wire); err != nil {
		return nil, fmt.Errorf("detaching domains from arvancloud account firewall rule %q: %w", id, err)
	}
	updated := toAccountFirewallRuleDomain(wire)
	return &updated, nil
}

// ReprioritizeArvanCloudAccountFirewallRules moves ruleID relative to
// afterRuleID/beforeRuleID within the account-level rule set. Same
// no-response-data shape as ReprioritizeArvanCloudFirewallRules.
func (p *Provider) ReprioritizeArvanCloudAccountFirewallRules(ctx context.Context, creds domain.ProviderCredentials, ruleID, afterRuleID, beforeRuleID string) error {
	body := reprioritizeRuleRequestWire{RuleID: ruleID, AfterRuleID: afterRuleID, BeforeRuleID: beforeRuleID}
	if err := p.client.doJSON(ctx, creds, http.MethodPost, accountFirewallRulesReprioritizePath, body, nil); err != nil {
		return fmt.Errorf("reprioritizing arvancloud account firewall rule %q: %w", ruleID, err)
	}
	return nil
}
