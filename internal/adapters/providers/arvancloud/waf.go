package arvancloud

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/javadib/do0ps/internal/core/domain"
)

// WAF (issue #66), wired to the real CDN API: ArvanCloud's managed rule-set
// engine, distinct from the CDN edge Firewall in firewall.go — see
// domain/arvancloud_waf.go's package comment for the naming-collision
// warning. Base paths are confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "WAF" tag: the global endpoints are
// relative to Client.baseURL (/waf and /waf/packages/{packageId}...), the
// per-domain ones relative to domainPath (/domains/{domain}/waf/...).
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real ArvanCloud responses correctly. Nothing above the
// adapter boundary ever sees them — every method here translates to/from
// internal/core/domain types.

const (
	wafGlobalBasePath         = "waf"
	wafGlobalPackagesBasePath = wafGlobalBasePath + "/packages"

	wafSettingsPathSuffix             = "/waf/settings"
	wafRulesPathSuffix                = "/waf/rules"
	wafPackagesPathSuffix             = "/waf/packages"
	wafReconfigurePathSuffix          = "/waf/actions/reconfigure"
	wafReprioritizeRulesPathSuffix    = "/waf/actions/reprioritize"
	wafReprioritizePackagesPathSuffix = "/waf/actions/reprioritize-package"
)

func wafGlobalPackagePath(packageID string) string {
	return wafGlobalPackagesBasePath + "/" + packageID
}
func wafGlobalPackageRulesPath(packageID string) string {
	return wafGlobalPackagePath(packageID) + "/rules"
}

func wafSettingsPath(domainName string) string { return domainPath(domainName) + wafSettingsPathSuffix }
func wafRulesPath(domainName string) string    { return domainPath(domainName) + wafRulesPathSuffix }
func wafRulePath(domainName, id string) string { return wafRulesPath(domainName) + "/" + id }
func wafPackagesPath(domainName string) string { return domainPath(domainName) + wafPackagesPathSuffix }
func wafPackagePath(domainName, id string) string {
	return wafPackagesPath(domainName) + "/" + id
}

// --- shared wire shapes ------------------------------------------------

// wafProviderWire mirrors the "provider" object nested in WafPackage and
// WafPreset's package summaries.
type wafProviderWire struct {
	Name string `json:"name,omitempty"`
	Logo string `json:"logo,omitempty"`
}

func toWafProviderDomain(w *wafProviderWire) domain.ArvanCloudWafProvider {
	if w == nil {
		return domain.ArvanCloudWafProvider{}
	}
	return domain.ArvanCloudWafProvider{Name: w.Name, Logo: w.Logo}
}

// wafRulesetRuleWire mirrors one entry of WafRuleset.rules.
type wafRulesetRuleWire struct {
	ID     string         `json:"id,omitempty"`
	Name   string         `json:"name,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

// wafRulesetWire mirrors WafRuleset.
type wafRulesetWire struct {
	ID    string               `json:"id,omitempty"`
	Name  string               `json:"name,omitempty"`
	Rules []wafRulesetRuleWire `json:"rules,omitempty"`
}

func toWafRulesetsDomain(items []wafRulesetWire) []domain.ArvanCloudWafRuleset {
	if len(items) == 0 {
		return nil
	}
	rulesets := make([]domain.ArvanCloudWafRuleset, len(items))
	for i, w := range items {
		rules := make([]domain.ArvanCloudWafRulesetRule, len(w.Rules))
		for j, r := range w.Rules {
			rules[j] = domain.ArvanCloudWafRulesetRule{ID: r.ID, Name: r.Name, Params: r.Params}
		}
		rulesets[i] = domain.ArvanCloudWafRuleset{ID: w.ID, Name: w.Name, Rules: rules}
	}
	return rulesets
}

// wafPackageWire mirrors WafPackage/WafPackageDetails (global) and
// DomainWafPackage/DomainWafPackageDetails (per-domain instance) in one
// shape, decode-only: Params and IsEnabled simply come back zero/absent from
// a global endpoint, and Rulesets comes back nil from every endpoint but the
// "*Details" show ones — matching domain.ArvanCloudWafPackage's own doc
// comment on sharing one struct across both spec-level shapes.
type wafPackageWire struct {
	ID               string           `json:"id,omitempty"`
	Name             string           `json:"name,omitempty"`
	Provider         *wafProviderWire `json:"provider,omitempty"`
	ParamsSchema     map[string]any   `json:"params_schema,omitempty"`
	DisabledRules    []string         `json:"disabled_rules,omitempty"`
	DisabledRulesets []string         `json:"disabled_rulesets,omitempty"`
	Params           map[string]any   `json:"params,omitempty"`
	IsEnabled        *bool            `json:"is_enabled,omitempty"`
	Rulesets         []wafRulesetWire `json:"rulesets,omitempty"`
}

func toWafPackageDomain(w wafPackageWire) domain.ArvanCloudWafPackage {
	pkg := domain.ArvanCloudWafPackage{
		ID:               w.ID,
		Name:             w.Name,
		Provider:         toWafProviderDomain(w.Provider),
		ParamsSchema:     w.ParamsSchema,
		DisabledRules:    w.DisabledRules,
		DisabledRulesets: w.DisabledRulesets,
		Params:           w.Params,
		Rulesets:         toWafRulesetsDomain(w.Rulesets),
	}
	if w.IsEnabled != nil {
		pkg.IsEnabled = *w.IsEnabled
	}
	return pkg
}

// --- Global (account-independent reference data) -----------------------

// wafPresetPackageWire mirrors one entry of WafPreset.packages: a lighter
// summary (name + provider only, no id) than wafPackageWire.
type wafPresetPackageWire struct {
	Name     string           `json:"name,omitempty"`
	Provider *wafProviderWire `json:"provider,omitempty"`
}

// wafPresetWire mirrors WafPreset.
type wafPresetWire struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Packages    []wafPresetPackageWire `json:"packages,omitempty"`
}

// wafPresetsWire mirrors WafPresets: WafPresetsData's "data" object.
type wafPresetsWire struct {
	Presets  []wafPresetWire  `json:"presets"`
	Packages []wafPackageWire `json:"packages"`
}

// ListArvanCloudWafPresets returns ArvanCloud's predefined WAF configurations
// and the full catalog of global WAF packages they draw from.
func (p *Provider) ListArvanCloudWafPresets(ctx context.Context, creds domain.ProviderCredentials) (*domain.ArvanCloudWafPresetsAndPackages, error) {
	var wire wafPresetsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, wafGlobalBasePath, nil, &wire); err != nil {
		return nil, fmt.Errorf("listing arvancloud waf presets: %w", err)
	}

	presets := make([]domain.ArvanCloudWafPreset, len(wire.Presets))
	for i, w := range wire.Presets {
		packages := make([]domain.ArvanCloudWafPresetPackage, len(w.Packages))
		for j, pw := range w.Packages {
			packages[j] = domain.ArvanCloudWafPresetPackage{Name: pw.Name, Provider: toWafProviderDomain(pw.Provider)}
		}
		presets[i] = domain.ArvanCloudWafPreset{ID: w.ID, Name: w.Name, Description: w.Description, Packages: packages}
	}

	packages := make([]domain.ArvanCloudWafPackage, len(wire.Packages))
	for i := range wire.Packages {
		packages[i] = toWafPackageDomain(wire.Packages[i])
	}

	return &domain.ArvanCloudWafPresetsAndPackages{Presets: presets, Packages: packages}, nil
}

// GetArvanCloudWafPackage returns a global WAF package's details, including
// its Rulesets.
func (p *Provider) GetArvanCloudWafPackage(ctx context.Context, creds domain.ProviderCredentials, packageID string) (*domain.ArvanCloudWafPackage, error) {
	var wire wafPackageWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, wafGlobalPackagePath(packageID), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud waf package %q: %w", packageID, err)
	}
	found := toWafPackageDomain(wire)
	return &found, nil
}

// wafPackageRuleWire mirrors global.waf.show_package_rules' response item
// shape: {id, name} only, lighter than wafRulesetRuleWire (no params) —
// confirmed against that endpoint's own inline schema rather than WafRuleset.
type wafPackageRuleWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetArvanCloudWafPackageRules returns a global WAF package's rule details.
// The endpoint's response is paginated (PaginatedResponse), but this method
// keeps the same "return everything, unfiltered" convention as every other
// listing method on this port (e.g. ListArvanCloudDynamicFields): it neither
// exposes the spec's optional id/per_page/page query parameters nor exposes
// pagination metadata, relying on the provider's own default page size.
func (p *Provider) GetArvanCloudWafPackageRules(ctx context.Context, creds domain.ProviderCredentials, packageID string) ([]domain.ArvanCloudWafPackageRule, error) {
	var items []wafPackageRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, wafGlobalPackageRulesPath(packageID), nil, &items); err != nil {
		return nil, fmt.Errorf("getting arvancloud waf package %q rules: %w", packageID, err)
	}
	rules := make([]domain.ArvanCloudWafPackageRule, len(items))
	for i, w := range items {
		rules[i] = domain.ArvanCloudWafPackageRule{ID: w.ID, Name: w.Name}
	}
	return rules, nil
}

// --- Per-domain WAF configuration ---------------------------------------

// wafLogRedactionWire mirrors WafSettings.log_redaction.
type wafLogRedactionWire struct {
	Cookies           []string `json:"cookies,omitempty"`
	Headers           []string `json:"headers,omitempty"`
	AllHeaders        bool     `json:"all_headers"`
	Body              bool     `json:"body"`
	Records           bool     `json:"records"`
	ReplacementString string   `json:"replacement_string,omitempty"`
}

func toWafLogRedactionDomain(w *wafLogRedactionWire) domain.ArvanCloudWafLogRedaction {
	if w == nil {
		return domain.ArvanCloudWafLogRedaction{}
	}
	return domain.ArvanCloudWafLogRedaction{
		Cookies:           w.Cookies,
		Headers:           w.Headers,
		AllHeaders:        w.AllHeaders,
		Body:              w.Body,
		Records:           w.Records,
		ReplacementString: domain.ArvanCloudWafLogRedactionReplacement(w.ReplacementString),
	}
}

// wafLogRedactionRequestBody builds log_redaction's request value, or nil
// (field omitted from the request entirely, letting the provider apply its
// own defaults) when lr is the exact zero value — see
// domain.ArvanCloudWafLogRedaction's doc comment. This means a caller who
// deliberately wants ReplacementString "" (one of the four valid values,
// meaning "replace with nothing") cannot express that alone without also
// setting some other field away from its zero, a known, narrow edge case:
// every other field of log_redaction already defaults to a caller-visible
// non-empty value (e.g. Body defaults true, Headers defaults to a non-empty
// list) on ArvanCloud's own side, so in practice a caller wanting only the
// replacement string changed passes at least one other field alongside it.
func wafLogRedactionRequestBody(lr domain.ArvanCloudWafLogRedaction) map[string]any {
	if len(lr.Cookies) == 0 && len(lr.Headers) == 0 && !lr.AllHeaders && !lr.Body && !lr.Records && lr.ReplacementString == "" {
		return nil
	}
	body := map[string]any{
		"all_headers": lr.AllHeaders,
		"body":        lr.Body,
		"records":     lr.Records,
	}
	if len(lr.Cookies) > 0 {
		body["cookies"] = lr.Cookies
	}
	if len(lr.Headers) > 0 {
		body["headers"] = lr.Headers
	}
	if lr.ReplacementString != "" {
		body["replacement_string"] = string(lr.ReplacementString)
	}
	return body
}

// wafSettingsWire mirrors WafSettings, decode-only for the same reason as
// firewallSettingsWire: a PATCH request is built separately
// (wafSettingsRequestBody) as a plain map so an explicit value on a field
// like log_redaction.all_headers reaches the provider rather than being
// dropped by encoding/json's omitempty.
type wafSettingsWire struct {
	IsEnabled    bool                 `json:"is_enabled"`
	Mode         string               `json:"mode"`
	LogRedaction *wafLogRedactionWire `json:"log_redaction"`
	Packages     []wafPackageWire     `json:"packages"`
}

func toWafSettingsDomain(w wafSettingsWire) domain.ArvanCloudWafSettings {
	packages := make([]domain.ArvanCloudWafPackage, len(w.Packages))
	for i := range w.Packages {
		packages[i] = toWafPackageDomain(w.Packages[i])
	}
	return domain.ArvanCloudWafSettings{
		IsEnabled:    w.IsEnabled,
		Mode:         domain.ArvanCloudWafMode(w.Mode),
		LogRedaction: toWafLogRedactionDomain(w.LogRedaction),
		Packages:     packages,
	}
}

// wafSettingsRequestBody builds the JSON body for a WAF settings PATCH. Only
// mode and log_redaction are sent: is_enabled and packages are readOnly on
// WafSettings (see domain.ArvanCloudWafSettings's doc comment), so they are
// never part of a request body, only ever reported back.
func wafSettingsRequestBody(settings domain.ArvanCloudWafSettings) map[string]any {
	body := map[string]any{"mode": string(settings.Mode)}
	if lr := wafLogRedactionRequestBody(settings.LogRedaction); lr != nil {
		body["log_redaction"] = lr
	}
	return body
}

// GetArvanCloudWafSettings returns domainName's WAF configuration.
func (p *Provider) GetArvanCloudWafSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudWafSettings, error) {
	var wire wafSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, wafSettingsPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud waf settings for domain %q: %w", domainName, err)
	}
	settings := toWafSettingsDomain(wire)
	return &settings, nil
}

// UpdateArvanCloudWafSettings changes domainName's WAF configuration and
// returns it as stored afterward.
func (p *Provider) UpdateArvanCloudWafSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudWafSettings) (*domain.ArvanCloudWafSettings, error) {
	body := wafSettingsRequestBody(settings)
	var wire wafSettingsWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, wafSettingsPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud waf settings for domain %q: %w", domainName, err)
	}
	updated := toWafSettingsDomain(wire)
	return &updated, nil
}

// wafReconfigureWire mirrors WafReconfigure.
type wafReconfigureWire struct {
	PresetID string `json:"preset_id"`
}

// ReconfigureArvanCloudWaf applies presetID to domainName in one call. The
// endpoint's response carries no data to translate — only a confirmation
// message — so there is nothing for this method to return but the error.
func (p *Provider) ReconfigureArvanCloudWaf(ctx context.Context, creds domain.ProviderCredentials, domainName, presetID string) error {
	body := wafReconfigureWire{PresetID: presetID}
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+wafReconfigurePathSuffix, body, nil); err != nil {
		return fmt.Errorf("reconfiguring arvancloud waf for domain %q with preset %q: %w", domainName, presetID, err)
	}
	return nil
}

// ReprioritizeArvanCloudWafRules moves ruleID relative to
// afterRuleID/beforeRuleID within domainName's WAF custom rule set. Reuses
// reprioritizeRuleRequestWire from firewall.go: the spec's
// ReprioritizeRuleRequest schema is identical for both capabilities. The
// endpoint's response carries no data to translate.
func (p *Provider) ReprioritizeArvanCloudWafRules(ctx context.Context, creds domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error {
	body := reprioritizeRuleRequestWire{RuleID: ruleID, AfterRuleID: afterRuleID, BeforeRuleID: beforeRuleID}
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+wafReprioritizeRulesPathSuffix, body, nil); err != nil {
		return fmt.Errorf("reprioritizing arvancloud waf rule %q on domain %q: %w", ruleID, domainName, err)
	}
	return nil
}

// wafReprioritizePackageWire mirrors WafReprioritize.
type wafReprioritizePackageWire struct {
	PackageID       string `json:"package_id"`
	AfterPackageID  string `json:"after_package_id,omitempty"`
	BeforePackageID string `json:"before_package_id,omitempty"`
}

// ReprioritizeArvanCloudWafPackages moves packageID relative to
// afterPackageID/beforePackageID within domainName's installed WAF package
// set. The endpoint's response carries no data to translate.
func (p *Provider) ReprioritizeArvanCloudWafPackages(ctx context.Context, creds domain.ProviderCredentials, domainName, packageID, afterPackageID, beforePackageID string) error {
	body := wafReprioritizePackageWire{PackageID: packageID, AfterPackageID: afterPackageID, BeforePackageID: beforePackageID}
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+wafReprioritizePackagesPathSuffix, body, nil); err != nil {
		return fmt.Errorf("reprioritizing arvancloud waf package %q on domain %q: %w", packageID, domainName, err)
	}
	return nil
}

// --- Per-domain WAF custom rules -----------------------------------------

// wafRuleExceptionIDReadWire mirrors one entry of
// WafRuleExceptionsResponse[].exceptions.ids: {id, name}. Spec quirk worth
// flagging (same documentation habit as lists.go's ip/bytes note): the id
// here is typed as a STRING on read, even though the matching write-side
// field (WafRuleExceptions[].exceptions.ids) is typed as a plain integer —
// see wafRuleExceptionWriteWire below and
// domain.ArvanCloudWafRuleException's doc comment. This adapter parses the
// read-side string back into an int for RuleIDs, tolerating (as 0, rather
// than failing the whole decode) an id that turns out not to be numeric.
type wafRuleExceptionIDReadWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type wafRuleExceptionReadWire struct {
	Package    string `json:"package"`
	Exceptions struct {
		IDs []wafRuleExceptionIDReadWire `json:"ids"`
	} `json:"exceptions"`
}

func toWafRuleExceptionsDomain(items []wafRuleExceptionReadWire) []domain.ArvanCloudWafRuleException {
	if len(items) == 0 {
		return nil
	}
	exceptions := make([]domain.ArvanCloudWafRuleException, len(items))
	for i, w := range items {
		ids := make([]int, len(w.Exceptions.IDs))
		names := make([]string, len(w.Exceptions.IDs))
		for j, idw := range w.Exceptions.IDs {
			// A non-numeric id is tolerated as 0 rather than failing the
			// whole decode — see this file's wafRuleExceptionIDReadWire doc
			// comment.
			id, _ := strconv.Atoi(idw.ID)
			ids[j] = id
			names[j] = idw.Name
		}
		exceptions[i] = domain.ArvanCloudWafRuleException{Package: w.Package, RuleIDs: ids, RuleNames: names}
	}
	return exceptions
}

// wafRuleExceptionWriteWire mirrors one entry of
// WafRuleExceptions[].exceptions.ids: a plain integer array — see
// wafRuleExceptionIDReadWire's doc comment for the read/write type mismatch
// this adapter bridges.
type wafRuleExceptionWriteWire struct {
	Package    string `json:"package"`
	Exceptions struct {
		IDs []int `json:"ids"`
	} `json:"exceptions"`
}

func toWafRuleExceptionsWire(exceptions []domain.ArvanCloudWafRuleException) []wafRuleExceptionWriteWire {
	items := make([]wafRuleExceptionWriteWire, len(exceptions))
	for i, e := range exceptions {
		items[i].Package = e.Package
		items[i].Exceptions.IDs = e.RuleIDs
	}
	return items
}

// wafRuleWire mirrors WafRule (request) and WafRuleOutput (response) — the
// two share every field but exceptions' item shape, which is handled
// separately by wafRuleExceptionReadWire/wafRuleExceptionWriteWire above.
// This struct is decode-only, matching firewallRuleWire's own convention.
type wafRuleWire struct {
	ID          string                     `json:"id,omitempty"`
	URLPattern  string                     `json:"url_pattern,omitempty"`
	Sources     []string                   `json:"sources,omitempty"`
	Action      string                     `json:"action,omitempty"`
	Description string                     `json:"description,omitempty"`
	IsEnabled   *bool                      `json:"is_enabled,omitempty"`
	Exceptions  []wafRuleExceptionReadWire `json:"exceptions,omitempty"`
}

func toWafRuleDomain(w wafRuleWire) domain.ArvanCloudWafRule {
	rule := domain.ArvanCloudWafRule{
		ID:          w.ID,
		URLPattern:  w.URLPattern,
		Sources:     w.Sources,
		Action:      domain.ArvanCloudWafRuleAction(w.Action),
		Description: w.Description,
		Exceptions:  toWafRuleExceptionsDomain(w.Exceptions),
	}
	if w.IsEnabled != nil {
		rule.IsEnabled = *w.IsEnabled
	}
	return rule
}

// wafRuleRequestBody builds the JSON body for a WAF rule create/update,
// sharing the same field set for both since WafRule is used as the request
// schema for both waf.rules.store and waf.rules.update.
func wafRuleRequestBody(rule domain.ArvanCloudWafRule) map[string]any {
	body := map[string]any{
		"url_pattern": rule.URLPattern,
		"sources":     rule.Sources,
		"action":      string(rule.Action),
		"is_enabled":  rule.IsEnabled,
	}
	if rule.Description != "" {
		body["description"] = rule.Description
	}
	if len(rule.Exceptions) > 0 {
		body["exceptions"] = toWafRuleExceptionsWire(rule.Exceptions)
	}
	return body
}

// ListArvanCloudWafRules returns every WAF custom rule configured for
// domainName. The endpoint's response schema
// (docs/api-specs/arvancloud-cdn-4.0.yml around waf.rules.index) nests a
// PaginatedResponse's "data" items under $ref: WafRuleResponse — the
// single-resource {message, data: WafRuleOutput} envelope shape, rather than
// WafRuleOutput directly. That combination does not describe a coherent list
// item (a list entry double-wrapped in its own {message, data} envelope), so
// it is read here as a spec authoring slip (the same kind already flagged in
// lists.go's ip/bytes note) and this method decodes each list entry directly
// as a WafRuleOutput (wafRuleWire) instead.
func (p *Provider) ListArvanCloudWafRules(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudWafRule, error) {
	var items []wafRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, wafRulesPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud waf rules of domain %q: %w", domainName, err)
	}
	rules := make([]domain.ArvanCloudWafRule, len(items))
	for i := range items {
		rules[i] = toWafRuleDomain(items[i])
	}
	return rules, nil
}

// CreateArvanCloudWafRule creates a new WAF custom rule.
func (p *Provider) CreateArvanCloudWafRule(ctx context.Context, creds domain.ProviderCredentials, domainName string, rule domain.ArvanCloudWafRule) (*domain.ArvanCloudWafRule, error) {
	body := wafRuleRequestBody(rule)
	var wire wafRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, wafRulesPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud waf rule on domain %q: %w", domainName, err)
	}
	created := toWafRuleDomain(wire)
	return &created, nil
}

// GetArvanCloudWafRule returns a single WAF custom rule by id.
func (p *Provider) GetArvanCloudWafRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudWafRule, error) {
	var wire wafRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, wafRulePath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud waf rule %q on domain %q: %w", id, domainName, err)
	}
	found := toWafRuleDomain(wire)
	return &found, nil
}

// UpdateArvanCloudWafRule updates a WAF custom rule and returns it as stored
// afterward.
func (p *Provider) UpdateArvanCloudWafRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rule domain.ArvanCloudWafRule) (*domain.ArvanCloudWafRule, error) {
	body := wafRuleRequestBody(rule)
	var wire wafRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, wafRulePath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud waf rule %q on domain %q: %w", id, domainName, err)
	}
	updated := toWafRuleDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudWafRule removes a WAF custom rule by id.
func (p *Provider) DeleteArvanCloudWafRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, wafRulePath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud waf rule %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// --- Per-domain WAF package subscriptions ---------------------------------

// ListArvanCloudWafDomainPackages returns the WAF packages currently
// installed on domainName. Unlike waf.packages.index's optional `available`
// query parameter (which would instead list packages available but not yet
// installed), this method always lists installed packages only — issue #66's
// scope explicitly describes this endpoint as "list installed packages".
func (p *Provider) ListArvanCloudWafDomainPackages(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudWafPackage, error) {
	var items []wafPackageWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, wafPackagesPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud waf packages installed on domain %q: %w", domainName, err)
	}
	packages := make([]domain.ArvanCloudWafPackage, len(items))
	for i := range items {
		packages[i] = toWafPackageDomain(items[i])
	}
	return packages, nil
}

// domainWafPackageStoreWire mirrors DomainWafPackageStore.
type domainWafPackageStoreWire struct {
	ID string `json:"id"`
}

// InstallArvanCloudWafPackage subscribes domainName to the global package
// identified by packageID.
func (p *Provider) InstallArvanCloudWafPackage(ctx context.Context, creds domain.ProviderCredentials, domainName, packageID string) (*domain.ArvanCloudWafPackage, error) {
	body := domainWafPackageStoreWire{ID: packageID}
	var wire wafPackageWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, wafPackagesPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("installing arvancloud waf package %q on domain %q: %w", packageID, domainName, err)
	}
	installed := toWafPackageDomain(wire)
	return &installed, nil
}

// GetArvanCloudWafDomainPackage returns one installed package's details,
// including its Rulesets.
func (p *Provider) GetArvanCloudWafDomainPackage(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudWafPackage, error) {
	var wire wafPackageWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, wafPackagePath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud waf package %q on domain %q: %w", id, domainName, err)
	}
	found := toWafPackageDomain(wire)
	return &found, nil
}

// wafDomainPackageUpdateRequestBody builds the JSON body for a
// waf.packages.update PATCH: only the fields DomainWafPackage actually lets
// a caller change (params, is_enabled, disabled_rules, disabled_rulesets).
// id/name/provider/params_schema/rulesets are read-only and never sent.
func wafDomainPackageUpdateRequestBody(pkg domain.ArvanCloudWafPackage) map[string]any {
	body := map[string]any{"is_enabled": pkg.IsEnabled}
	if pkg.Params != nil {
		body["params"] = pkg.Params
	}
	if pkg.DisabledRules != nil {
		body["disabled_rules"] = pkg.DisabledRules
	}
	if pkg.DisabledRulesets != nil {
		body["disabled_rulesets"] = pkg.DisabledRulesets
	}
	return body
}

// UpdateArvanCloudWafDomainPackage changes an installed package's own
// configuration and returns it as stored afterward.
func (p *Provider) UpdateArvanCloudWafDomainPackage(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, pkg domain.ArvanCloudWafPackage) (*domain.ArvanCloudWafPackage, error) {
	body := wafDomainPackageUpdateRequestBody(pkg)
	var wire wafPackageWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, wafPackagePath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud waf package %q on domain %q: %w", id, domainName, err)
	}
	updated := toWafPackageDomain(wire)
	return &updated, nil
}

// UninstallArvanCloudWafPackage removes an installed package from the domain
// by id.
func (p *Provider) UninstallArvanCloudWafPackage(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, wafPackagePath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("uninstalling arvancloud waf package %q from domain %q: %w", id, domainName, err)
	}
	return nil
}
