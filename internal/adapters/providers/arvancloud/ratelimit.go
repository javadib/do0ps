package arvancloud

import (
	"context"
	"fmt"
	"net/http"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Rate Limiting (issue #68), wired to the real CDN API: per-domain
// request-rate settings plus a rule engine that throttles or blocks traffic
// exceeding a configured rate — distinct from the CDN edge Firewall
// (firewall.go), the WAF managed rule-set engine (waf.go) and the dedicated
// DDoS challenge engine (ddos.go). Base paths are confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Rate Limiting" tag, relative to
// domainPath (defined in domain.go) — i.e.
// https://napi.arvancloud.ir/cdn/4.0/domains/{domain}/rate-limit/... .
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real ArvanCloud responses correctly. Nothing above
// the adapter boundary ever sees them — every method here translates to/from
// internal/core/domain types.

const (
	rateLimitSettingsPathSuffix     = "/rate-limit/settings"
	rateLimitRulesPathSuffix        = "/rate-limit/rules"
	rateLimitReprioritizePathSuffix = "/rate-limit/actions/reprioritize"
)

func rateLimitSettingsPath(domainName string) string {
	return domainPath(domainName) + rateLimitSettingsPathSuffix
}
func rateLimitRulesPath(domainName string) string {
	return domainPath(domainName) + rateLimitRulesPathSuffix
}
func rateLimitRulePath(domainName, id string) string {
	return rateLimitRulesPath(domainName) + "/" + id
}

// --- Per-domain rate-limit settings ---------------------------------------

// rateLimitSettingsWire mirrors RateLimitSettings.
type rateLimitSettingsWire struct {
	DDoSDetection  bool     `json:"ddos_detection"`
	ExcludeSources []string `json:"exclude_sources,omitempty"`
}

func toRateLimitSettingsDomain(w rateLimitSettingsWire) domain.ArvanCloudRateLimitSettings {
	return domain.ArvanCloudRateLimitSettings{
		DDoSDetection:  w.DDoSDetection,
		ExcludeSources: w.ExcludeSources,
	}
}

// rateLimitSettingsRequestBody builds the JSON body for a rate-limit
// settings PATCH as a plain map — the same reason ddosSettingsRequestBody
// does: an explicit `false` on ddos_detection must reach the provider rather
// than being dropped by encoding/json's omitempty.
func rateLimitSettingsRequestBody(settings domain.ArvanCloudRateLimitSettings) map[string]any {
	body := map[string]any{
		"ddos_detection": settings.DDoSDetection,
	}
	if len(settings.ExcludeSources) > 0 {
		body["exclude_sources"] = settings.ExcludeSources
	}
	return body
}

// GetArvanCloudRateLimitSettings returns domainName's rate-limiting
// configuration.
func (p *Provider) GetArvanCloudRateLimitSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudRateLimitSettings, error) {
	var wire rateLimitSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, rateLimitSettingsPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud rate limit settings for domain %q: %w", domainName, err)
	}
	settings := toRateLimitSettingsDomain(wire)
	return &settings, nil
}

// UpdateArvanCloudRateLimitSettings changes domainName's rate-limiting
// configuration and returns it as stored afterward.
func (p *Provider) UpdateArvanCloudRateLimitSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudRateLimitSettings) (*domain.ArvanCloudRateLimitSettings, error) {
	body := rateLimitSettingsRequestBody(settings)
	var wire rateLimitSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, rateLimitSettingsPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud rate limit settings for domain %q: %w", domainName, err)
	}
	updated := toRateLimitSettingsDomain(wire)
	return &updated, nil
}

// --- Per-domain rate-limit rules -------------------------------------------

// challengeActionWire mirrors ChallengeAction, shared by request and
// response.
type challengeActionWire struct {
	Mode      int  `json:"mode,omitempty"`
	TTL       int  `json:"ttl,omitempty"`
	HTTPSOnly bool `json:"https_only,omitempty"`
}

func toChallengeActionDomain(w *challengeActionWire) domain.ArvanCloudChallengeAction {
	if w == nil {
		return domain.ArvanCloudChallengeAction{}
	}
	return domain.ArvanCloudChallengeAction{
		Mode:      domain.ArvanCloudChallengeMode(w.Mode),
		TTL:       w.TTL,
		HTTPSOnly: w.HTTPSOnly,
	}
}

// challengeActionRequestBody builds action_details' request value, or nil
// (the field omitted entirely) when a is the exact zero value — the same
// "omit the whole optional sub-object when nothing was given" choice
// ddosPreflightRequestBody makes for DDoS's preflight.
func challengeActionRequestBody(a domain.ArvanCloudChallengeAction) map[string]any {
	if a.Mode == 0 && a.TTL == 0 && !a.HTTPSOnly {
		return nil
	}
	body := map[string]any{}
	if a.Mode != 0 {
		body["mode"] = int(a.Mode)
	}
	if a.TTL != 0 {
		body["ttl"] = a.TTL
	}
	body["https_only"] = a.HTTPSOnly
	return body
}

// rateLimitRuleWire mirrors BaseRateLimitRule/RateLimitRuleView, decode-only
// like ddosRuleWire: a request body is built separately
// (rateLimitRuleRequestBody) as a plain map, so explicit zero values
// (is_enabled: false) reach the provider rather than being dropped by
// encoding/json's omitempty.
type rateLimitRuleWire struct {
	ID             string               `json:"id,omitempty"`
	Action         string               `json:"action,omitempty"`
	IsEnabled      *bool                `json:"is_enabled,omitempty"`
	URLPattern     string               `json:"url_pattern,omitempty"`
	Description    string               `json:"description,omitempty"`
	ExcludeSources []string             `json:"exclude_sources,omitempty"`
	Rate           int                  `json:"rate,omitempty"`
	Burst          int                  `json:"burst,omitempty"`
	BlockDuration  int                  `json:"block_duration,omitempty"`
	TimeDuration   int                  `json:"time_duration,omitempty"`
	AllowedMethods []string             `json:"allowed_methods,omitempty"`
	ActionDetails  *challengeActionWire `json:"action_details,omitempty"`
}

func toRateLimitRuleDomain(w rateLimitRuleWire) domain.ArvanCloudRateLimitRule {
	rule := domain.ArvanCloudRateLimitRule{
		ID:             w.ID,
		Action:         domain.ArvanCloudRateLimitAction(w.Action),
		URLPattern:     w.URLPattern,
		Description:    w.Description,
		ExcludeSources: w.ExcludeSources,
		Rate:           w.Rate,
		Burst:          w.Burst,
		BlockDuration:  w.BlockDuration,
		TimeDuration:   w.TimeDuration,
		AllowedMethods: w.AllowedMethods,
		ActionDetails:  toChallengeActionDomain(w.ActionDetails),
	}
	if w.IsEnabled != nil {
		rule.IsEnabled = *w.IsEnabled
	}
	return rule
}

// rateLimitRuleRequestBody builds the JSON body for a rate-limit rule
// create/update, sharing the same field set for both since BaseRateLimitRule
// underlies both RateLimitRule (create) and RateLimitRuleUpdate. Rate and
// TimeDuration are always sent (the use case validates both are positive
// before this is ever called — see
// app.validateArvanCloudRateLimitRuleInput). Burst is sent only when
// positive: the spec sets minimum: 1 on it, so a zero value the caller never
// set would fail validation if sent. BlockDuration's minimum is 0, but is
// likewise sent only when positive, so an unset value lets the provider
// apply its own default rather than forcing an explicit 0.
func rateLimitRuleRequestBody(rule domain.ArvanCloudRateLimitRule) map[string]any {
	body := map[string]any{
		"url_pattern":   rule.URLPattern,
		"action":        string(rule.Action),
		"is_enabled":    rule.IsEnabled,
		"rate":          rule.Rate,
		"time_duration": rule.TimeDuration,
	}
	if rule.Description != "" {
		body["description"] = rule.Description
	}
	if len(rule.ExcludeSources) > 0 {
		body["exclude_sources"] = rule.ExcludeSources
	}
	if rule.Burst > 0 {
		body["burst"] = rule.Burst
	}
	if rule.BlockDuration > 0 {
		body["block_duration"] = rule.BlockDuration
	}
	if len(rule.AllowedMethods) > 0 {
		body["allowed_methods"] = rule.AllowedMethods
	}
	// action_details is meaningful only when Action is "challenge" — the
	// same "only send captcha fields for protection_mode == captcha" choice
	// ddosSettingsRequestBody makes.
	if rule.Action == domain.ArvanCloudRateLimitActionChallenge {
		if details := challengeActionRequestBody(rule.ActionDetails); details != nil {
			body["action_details"] = details
		}
	}
	return body
}

// ListArvanCloudRateLimitRules returns every rate-limit rule configured for
// domainName, unfiltered — matching ListArvanCloudDdosRules' own convention.
func (p *Provider) ListArvanCloudRateLimitRules(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudRateLimitRule, error) {
	var items []rateLimitRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, rateLimitRulesPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud rate limit rules of domain %q: %w", domainName, err)
	}
	rules := make([]domain.ArvanCloudRateLimitRule, len(items))
	for i := range items {
		rules[i] = toRateLimitRuleDomain(items[i])
	}
	return rules, nil
}

// CreateArvanCloudRateLimitRule creates a new rate-limit rule.
func (p *Provider) CreateArvanCloudRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, domainName string, rule domain.ArvanCloudRateLimitRule) (*domain.ArvanCloudRateLimitRule, error) {
	body := rateLimitRuleRequestBody(rule)
	var wire rateLimitRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, rateLimitRulesPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud rate limit rule on domain %q: %w", domainName, err)
	}
	created := toRateLimitRuleDomain(wire)
	return &created, nil
}

// GetArvanCloudRateLimitRule returns a single rate-limit rule by id.
func (p *Provider) GetArvanCloudRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudRateLimitRule, error) {
	var wire rateLimitRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, rateLimitRulePath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud rate limit rule %q on domain %q: %w", id, domainName, err)
	}
	found := toRateLimitRuleDomain(wire)
	return &found, nil
}

// UpdateArvanCloudRateLimitRule updates a rate-limit rule and returns it as
// stored afterward.
func (p *Provider) UpdateArvanCloudRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rule domain.ArvanCloudRateLimitRule) (*domain.ArvanCloudRateLimitRule, error) {
	body := rateLimitRuleRequestBody(rule)
	var wire rateLimitRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, rateLimitRulePath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud rate limit rule %q on domain %q: %w", id, domainName, err)
	}
	updated := toRateLimitRuleDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudRateLimitRule removes a rate-limit rule by id.
func (p *Provider) DeleteArvanCloudRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, rateLimitRulePath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud rate limit rule %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// ReprioritizeArvanCloudRateLimitRules moves ruleID relative to
// afterRuleID/beforeRuleID within domainName's rate-limit rule set. Reuses
// reprioritizeRuleRequestWire from firewall.go: the spec's
// ReprioritizeRuleRequest schema is identical across Firewall, WAF, DDoS and
// Rate Limiting.
func (p *Provider) ReprioritizeArvanCloudRateLimitRules(ctx context.Context, creds domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error {
	body := reprioritizeRuleRequestWire{RuleID: ruleID, AfterRuleID: afterRuleID, BeforeRuleID: beforeRuleID}
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+rateLimitReprioritizePathSuffix, body, nil); err != nil {
		return fmt.Errorf("reprioritizing arvancloud rate limit rule %q on domain %q: %w", ruleID, domainName, err)
	}
	return nil
}
