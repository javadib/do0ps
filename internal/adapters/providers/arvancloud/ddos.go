package arvancloud

import (
	"context"
	"fmt"
	"net/http"

	"github.com/javadib/do0ps/internal/core/domain"
)

// DDoS Protection (issue #67), wired to the real CDN API: a per-domain
// challenge engine (cookie/JavaScript/CAPTCHA-based), distinct from both the
// CDN edge Firewall in firewall.go and the WAF managed rule-set engine in
// waf.go — see domain/arvancloud_ddos.go's package comment for the naming-
// collision warning. Base paths are confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "DDoS" tag, relative to domainPath
// (defined in domain.go) — i.e.
// https://napi.arvancloud.ir/cdn/4.0/domains/{domain}/ddos/... .
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real ArvanCloud responses correctly. Nothing above
// the adapter boundary ever sees them — every method here translates to/from
// internal/core/domain types.
//
// secret_key handling: DdosSettings.secret_key is caller-supplied CAPTCHA
// provider material (domain.ArvanCloudDdosSettings.SecretKey's doc comment),
// treated as sensitive the same way domain.ProviderCredentials is. This
// adapter never logs a request body — the shared client's debug log
// (client.go's roundTrip) only ever logs the method, URL and redacted
// headers, matching the redaction client.go already applies to the API key
// — and no method below embeds settings.SecretKey in any fmt.Errorf message.
// See ddos_test.go's TestUpdateArvanCloudDdosSettingsNeverLogsSecretKey for
// the guard.

const (
	ddosSettingsPathSuffix     = "/ddos/settings"
	ddosRulesPathSuffix        = "/ddos/rules"
	ddosReprioritizePathSuffix = "/ddos/actions/reprioritize"
)

func ddosSettingsPath(domainName string) string {
	return domainPath(domainName) + ddosSettingsPathSuffix
}
func ddosRulesPath(domainName string) string { return domainPath(domainName) + ddosRulesPathSuffix }
func ddosRulePath(domainName, id string) string {
	return ddosRulesPath(domainName) + "/" + id
}

// --- Per-domain DDoS settings -------------------------------------------

// ddosPreflightWire mirrors DdosPreflight.
type ddosPreflightWire struct {
	AccessOrigin        string   `json:"access_origin,omitempty"`
	AccessCredentials   string   `json:"access_credentials,omitempty"`
	AccessMethods       []string `json:"access_methods,omitempty"`
	AccessHeaders       string   `json:"access_headers,omitempty"`
	AccessExposeHeaders string   `json:"access_expose_headers,omitempty"`
}

func toDdosPreflightDomain(w *ddosPreflightWire) domain.ArvanCloudDdosPreflight {
	if w == nil {
		return domain.ArvanCloudDdosPreflight{}
	}
	return domain.ArvanCloudDdosPreflight{
		AccessOrigin:        w.AccessOrigin,
		AccessCredentials:   w.AccessCredentials,
		AccessMethods:       w.AccessMethods,
		AccessHeaders:       w.AccessHeaders,
		AccessExposeHeaders: w.AccessExposeHeaders,
	}
}

// ddosPreflightRequestBody builds preflight's request value, or nil (field
// omitted, letting the provider keep whatever it already has) when p is the
// exact zero value — the same "omit the whole optional sub-object when
// nothing was given" choice wafLogRedactionRequestBody makes for
// log_redaction.
func ddosPreflightRequestBody(p domain.ArvanCloudDdosPreflight) map[string]any {
	if p.AccessOrigin == "" && p.AccessCredentials == "" && len(p.AccessMethods) == 0 &&
		p.AccessHeaders == "" && p.AccessExposeHeaders == "" {
		return nil
	}
	body := map[string]any{}
	if p.AccessOrigin != "" {
		body["access_origin"] = p.AccessOrigin
	}
	if p.AccessCredentials != "" {
		body["access_credentials"] = p.AccessCredentials
	}
	if len(p.AccessMethods) > 0 {
		body["access_methods"] = p.AccessMethods
	}
	if p.AccessHeaders != "" {
		body["access_headers"] = p.AccessHeaders
	}
	if p.AccessExposeHeaders != "" {
		body["access_expose_headers"] = p.AccessExposeHeaders
	}
	return body
}

// ddosSettingsWire mirrors DdosSettings, decode-only for the same reason as
// wafSettingsWire: a PATCH request is built separately
// (ddosSettingsRequestBody) as a plain map, so an explicit `false` on a
// boolean toggle (e.g. https_only) reaches the provider rather than being
// dropped by encoding/json's omitempty.
type ddosSettingsWire struct {
	IsEnabled      bool               `json:"is_enabled"`
	ProtectionMode string             `json:"protection_mode"`
	CaptchaService string             `json:"captcha_service,omitempty"`
	SiteKey        string             `json:"site_key,omitempty"`
	SecretKey      string             `json:"secret_key,omitempty"`
	TTL            int                `json:"ttl"`
	HTTPSOnly      bool               `json:"https_only"`
	Preflight      *ddosPreflightWire `json:"preflight,omitempty"`
}

func toDdosSettingsDomain(w ddosSettingsWire) domain.ArvanCloudDdosSettings {
	return domain.ArvanCloudDdosSettings{
		IsEnabled:      w.IsEnabled,
		ProtectionMode: domain.ArvanCloudDdosProtectionMode(w.ProtectionMode),
		CaptchaService: domain.ArvanCloudCaptchaService(w.CaptchaService),
		SiteKey:        w.SiteKey,
		SecretKey:      w.SecretKey,
		TTL:            w.TTL,
		HTTPSOnly:      w.HTTPSOnly,
		Preflight:      toDdosPreflightDomain(w.Preflight),
	}
}

// ddosSettingsRequestBody builds the JSON body for a DDoS settings PATCH.
// is_enabled is readOnly on DdosSettings (domain.ArvanCloudDdosSettings's own
// doc comment), so it is never part of a request body. captcha_service,
// site_key and secret_key are included only when non-empty, matching the
// spec's own framing that they are meaningful only for
// protection_mode == "captcha" (the use case validates this before the
// request ever reaches here — see app.UpdateArvanCloudDdosSettings).
func ddosSettingsRequestBody(settings domain.ArvanCloudDdosSettings) map[string]any {
	body := map[string]any{
		"protection_mode": string(settings.ProtectionMode),
		"ttl":             settings.TTL,
		"https_only":      settings.HTTPSOnly,
	}
	// captcha_service/site_key/secret_key are sent only when ProtectionMode
	// is actually "captcha" — matching domain.ArvanCloudDdosSettings's own
	// doc comment that these three are meaningful only in that case. This
	// also means switching ProtectionMode away from "captcha" never
	// re-submits a stale CaptchaService/SecretKey the caller may still have
	// set on the input struct from a previous read.
	if settings.ProtectionMode == domain.ArvanCloudDdosProtectionModeCaptcha {
		if settings.CaptchaService != "" {
			body["captcha_service"] = string(settings.CaptchaService)
		}
		if settings.SiteKey != "" {
			body["site_key"] = settings.SiteKey
		}
		if settings.SecretKey != "" {
			body["secret_key"] = settings.SecretKey
		}
	}
	if preflight := ddosPreflightRequestBody(settings.Preflight); preflight != nil {
		body["preflight"] = preflight
	}
	return body
}

// GetArvanCloudDdosSettings returns domainName's DDoS protection
// configuration.
func (p *Provider) GetArvanCloudDdosSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDdosSettings, error) {
	var wire ddosSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, ddosSettingsPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud ddos settings for domain %q: %w", domainName, err)
	}
	settings := toDdosSettingsDomain(wire)
	return &settings, nil
}

// UpdateArvanCloudDdosSettings changes domainName's DDoS protection
// configuration and returns it as stored afterward. See this file's package
// comment for how settings.SecretKey is kept out of logs and error messages.
func (p *Provider) UpdateArvanCloudDdosSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudDdosSettings) (*domain.ArvanCloudDdosSettings, error) {
	body := ddosSettingsRequestBody(settings)
	var wire ddosSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, ddosSettingsPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud ddos settings for domain %q: %w", domainName, err)
	}
	updated := toDdosSettingsDomain(wire)
	return &updated, nil
}

// --- Per-domain DDoS rules -----------------------------------------------

// ddosRuleWire mirrors DdosRule, shared by request and response. This struct
// is decode-only, matching wafRuleWire's own convention.
type ddosRuleWire struct {
	ID          string   `json:"id,omitempty"`
	URLPattern  string   `json:"url_pattern,omitempty"`
	Sources     []string `json:"sources,omitempty"`
	Description string   `json:"description,omitempty"`
	Action      string   `json:"action,omitempty"`
	IsEnabled   *bool    `json:"is_enabled,omitempty"`
}

func toDdosRuleDomain(w ddosRuleWire) domain.ArvanCloudDdosRule {
	rule := domain.ArvanCloudDdosRule{
		ID:          w.ID,
		URLPattern:  w.URLPattern,
		Sources:     w.Sources,
		Description: w.Description,
		Action:      domain.ArvanCloudDdosRuleAction(w.Action),
	}
	if w.IsEnabled != nil {
		rule.IsEnabled = *w.IsEnabled
	}
	return rule
}

// ddosRuleRequestBody builds the JSON body for a DDoS rule create/update,
// sharing the same field set for both since DdosRule is used as the request
// schema for both ddos.rules.store and ddos.rules.update.
func ddosRuleRequestBody(rule domain.ArvanCloudDdosRule) map[string]any {
	body := map[string]any{
		"url_pattern": rule.URLPattern,
		"sources":     rule.Sources,
		"action":      string(rule.Action),
		"is_enabled":  rule.IsEnabled,
	}
	if rule.Description != "" {
		body["description"] = rule.Description
	}
	return body
}

// ListArvanCloudDdosRules returns every DDoS rule configured for domainName,
// unfiltered — matching ListArvanCloudWafRules' own convention.
func (p *Provider) ListArvanCloudDdosRules(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudDdosRule, error) {
	var items []ddosRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, ddosRulesPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud ddos rules of domain %q: %w", domainName, err)
	}
	rules := make([]domain.ArvanCloudDdosRule, len(items))
	for i := range items {
		rules[i] = toDdosRuleDomain(items[i])
	}
	return rules, nil
}

// CreateArvanCloudDdosRule creates a new DDoS rule.
func (p *Provider) CreateArvanCloudDdosRule(ctx context.Context, creds domain.ProviderCredentials, domainName string, rule domain.ArvanCloudDdosRule) (*domain.ArvanCloudDdosRule, error) {
	body := ddosRuleRequestBody(rule)
	var wire ddosRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, ddosRulesPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud ddos rule on domain %q: %w", domainName, err)
	}
	created := toDdosRuleDomain(wire)
	return &created, nil
}

// GetArvanCloudDdosRule returns a single DDoS rule by id.
func (p *Provider) GetArvanCloudDdosRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudDdosRule, error) {
	var wire ddosRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, ddosRulePath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud ddos rule %q on domain %q: %w", id, domainName, err)
	}
	found := toDdosRuleDomain(wire)
	return &found, nil
}

// UpdateArvanCloudDdosRule updates a DDoS rule and returns it as stored
// afterward.
func (p *Provider) UpdateArvanCloudDdosRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rule domain.ArvanCloudDdosRule) (*domain.ArvanCloudDdosRule, error) {
	body := ddosRuleRequestBody(rule)
	var wire ddosRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, ddosRulePath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud ddos rule %q on domain %q: %w", id, domainName, err)
	}
	updated := toDdosRuleDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudDdosRule removes a DDoS rule by id.
func (p *Provider) DeleteArvanCloudDdosRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, ddosRulePath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud ddos rule %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// ReprioritizeArvanCloudDdosRules moves ruleID relative to
// afterRuleID/beforeRuleID within domainName's DDoS rule set. Reuses
// reprioritizeRuleRequestWire from firewall.go: the spec's
// ReprioritizeRuleRequest schema is identical across Firewall, WAF and DDoS.
// The endpoint's response carries no data to translate.
func (p *Provider) ReprioritizeArvanCloudDdosRules(ctx context.Context, creds domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error {
	body := reprioritizeRuleRequestWire{RuleID: ruleID, AfterRuleID: afterRuleID, BeforeRuleID: beforeRuleID}
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+ddosReprioritizePathSuffix, body, nil); err != nil {
		return fmt.Errorf("reprioritizing arvancloud ddos rule %q on domain %q: %w", ruleID, domainName, err)
	}
	return nil
}
