package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the WAF use cases for ArvanCloud (issue #66): the managed
// rule-set engine (OWASP-style packages/presets a domain subscribes to),
// distinct from the CDN edge Firewall in arvancloud_firewall.go — see
// domain/arvancloud_waf.go's package comment for the naming-collision
// warning. Every one of them is a fast operation (ports.ArvanCloudProvider,
// AGENTS.md 4.3): each dispatches onto the queue and blocks for the result
// within the same tool call.
//
// What IS validated client-side, per issue #66's acceptance criteria:
//
//   - Mode against domain.ValidArvanCloudWafMode ("off"/"detect"/"protect" —
//     see domain.ArvanCloudWafMode's doc comment for the confirmed wire
//     encoding).
//   - A custom rule's Action against domain.ValidArvanCloudWafRuleAction
//     ("protect"/"passthrough" only — narrower than, and independent of, the
//     CDN edge Firewall's four-value action enum).
//   - A reprioritize call's "exactly one of after/before" contract, for both
//     rules and packages.

// --- Global (account-independent reference data) --------------------------

// ListArvanCloudWafPresetsInput carries the credentials needed to list
// ArvanCloud's predefined WAF configurations. There is nothing else to
// specify: this is global, account-independent reference data.
type ListArvanCloudWafPresetsInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudWafPresets is a fast operation.
type ListArvanCloudWafPresets struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudWafPresets builds the use case from its ports.
func NewListArvanCloudWafPresets(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudWafPresets {
	return &ListArvanCloudWafPresets{queue: queue, provider: provider}
}

// Execute returns ArvanCloud's predefined WAF configurations and the global
// package catalog they draw from.
func (uc *ListArvanCloudWafPresets) Execute(ctx context.Context, in ListArvanCloudWafPresetsInput) (*domain.ArvanCloudWafPresetsAndPackages, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := uc.provider.ListArvanCloudWafPresets(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud waf presets: %w", err)
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudWafPresetsAndPackages
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud waf presets: %w", err)
	}
	return &result, nil
}

// arvanCloudWafPackageIDOnlyInput is embedded by every global-package use
// case below that is scoped to exactly one package by id and needs nothing
// else.
type arvanCloudWafPackageIDOnlyInput struct {
	Credentials domain.ProviderCredentials
	PackageID   string
}

func (in arvanCloudWafPackageIDOnlyInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.PackageID == "" {
		return fmt.Errorf("package_id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudWafPackageInput identifies the global package to look up.
type GetArvanCloudWafPackageInput = arvanCloudWafPackageIDOnlyInput

// GetArvanCloudWafPackage is a fast operation.
type GetArvanCloudWafPackage struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudWafPackage builds the use case from its ports.
func NewGetArvanCloudWafPackage(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudWafPackage {
	return &GetArvanCloudWafPackage{queue: queue, provider: provider}
}

// Execute returns one global WAF package's details, including its rulesets.
func (uc *GetArvanCloudWafPackage) Execute(ctx context.Context, in GetArvanCloudWafPackageInput) (*domain.ArvanCloudWafPackage, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudWafPackage(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudWafPackage, error) {
		found, err := uc.provider.GetArvanCloudWafPackage(ctx, in.Credentials, in.PackageID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud waf package %q: %w", in.PackageID, err)
		}
		return found, nil
	})
}

// GetArvanCloudWafPackageRulesInput identifies the global package whose
// rules to look up.
type GetArvanCloudWafPackageRulesInput = arvanCloudWafPackageIDOnlyInput

// GetArvanCloudWafPackageRules is a fast operation.
type GetArvanCloudWafPackageRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudWafPackageRules builds the use case from its ports.
func NewGetArvanCloudWafPackageRules(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudWafPackageRules {
	return &GetArvanCloudWafPackageRules{queue: queue, provider: provider}
}

// Execute returns one global WAF package's rule details.
func (uc *GetArvanCloudWafPackageRules) Execute(ctx context.Context, in GetArvanCloudWafPackageRulesInput) ([]domain.ArvanCloudWafPackageRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.GetArvanCloudWafPackageRules(ctx, in.Credentials, in.PackageID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud waf package %q rules: %w", in.PackageID, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.ArvanCloudWafPackageRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding arvancloud waf package rule list: %w", err)
	}
	return rules, nil
}

// --- Per-domain WAF configuration ------------------------------------------

// arvanCloudWafDomainInput is embedded by every use case below that is
// scoped to exactly one domain by name and needs nothing else.
type arvanCloudWafDomainInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

func (in arvanCloudWafDomainInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudWafSettingsInput identifies the domain whose WAF settings to
// look up.
type GetArvanCloudWafSettingsInput = arvanCloudWafDomainInput

// GetArvanCloudWafSettings is a fast operation.
type GetArvanCloudWafSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudWafSettings builds the use case from its ports.
func NewGetArvanCloudWafSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudWafSettings {
	return &GetArvanCloudWafSettings{queue: queue, provider: provider}
}

// Execute returns the domain's current WAF settings.
func (uc *GetArvanCloudWafSettings) Execute(ctx context.Context, in GetArvanCloudWafSettingsInput) (*domain.ArvanCloudWafSettings, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudWafSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudWafSettings, error) {
		found, err := uc.provider.GetArvanCloudWafSettings(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud waf settings for domain %q: %w", in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudWafSettingsInput identifies the domain and its new WAF
// configuration.
type UpdateArvanCloudWafSettingsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Settings    domain.ArvanCloudWafSettings
}

// UpdateArvanCloudWafSettings changes a domain's WAF configuration. This is
// a fast operation.
type UpdateArvanCloudWafSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudWafSettings builds the use case from its ports.
func NewUpdateArvanCloudWafSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudWafSettings {
	return &UpdateArvanCloudWafSettings{queue: queue, provider: provider}
}

// Execute validates the request and updates the settings, returning them as
// stored afterward.
func (uc *UpdateArvanCloudWafSettings) Execute(ctx context.Context, in UpdateArvanCloudWafSettingsInput) (*domain.ArvanCloudWafSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudWafMode(string(in.Settings.Mode)) {
		return nil, fmt.Errorf("mode %q is not one of \"off\", \"detect\" or \"protect\": %w", in.Settings.Mode, domain.ErrInvalidInput)
	}
	if replacement := in.Settings.LogRedaction.ReplacementString; replacement != "" && !domain.ValidArvanCloudWafLogRedactionReplacement(string(replacement)) {
		return nil, fmt.Errorf(
			"log_redaction.replacement_string %q is not one of \"****\", \"####\", \"--REDACTED--\" or \"\": %w",
			replacement, domain.ErrInvalidInput)
	}

	return dispatchArvanCloudWafSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudWafSettings, error) {
		updated, err := uc.provider.UpdateArvanCloudWafSettings(ctx, in.Credentials, in.Domain, in.Settings)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud waf settings for domain %q: %w", in.Domain, err)
		}
		return updated, nil
	})
}

// --- WAF actions: reconfigure / reprioritize / reprioritize-package -------

// ReconfigureArvanCloudWafInput identifies the domain and the preset to
// apply to it.
type ReconfigureArvanCloudWafInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	PresetID    string
}

// ReconfigureArvanCloudWaf is a fast operation. Applying a preset removes
// every WAF package currently installed on the domain and replaces it with
// the preset's own set (ports.ArvanCloudProvider.ReconfigureArvanCloudWaf's
// doc comment) — this is the use case for a request like "turn on
// OWASP-style protection" or "block SQL injection attempts".
type ReconfigureArvanCloudWaf struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewReconfigureArvanCloudWaf builds the use case from its ports.
func NewReconfigureArvanCloudWaf(queue ports.Queue, provider ports.ArvanCloudProvider) *ReconfigureArvanCloudWaf {
	return &ReconfigureArvanCloudWaf{queue: queue, provider: provider}
}

// Execute applies PresetID to Domain.
func (uc *ReconfigureArvanCloudWaf) Execute(ctx context.Context, in ReconfigureArvanCloudWafInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.PresetID == "" {
		return fmt.Errorf("preset_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.ReconfigureArvanCloudWaf(ctx, in.Credentials, in.Domain, in.PresetID); err != nil {
			return nil, fmt.Errorf("reconfiguring arvancloud waf for domain %q with preset %q: %w", in.Domain, in.PresetID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ReprioritizeArvanCloudWafRulesInput identifies the domain, the custom rule
// to move, and exactly one of AfterRuleID/BeforeRuleID to move it relative
// to.
type ReprioritizeArvanCloudWafRulesInput struct {
	Credentials  domain.ProviderCredentials
	Domain       string
	RuleID       string
	AfterRuleID  string
	BeforeRuleID string
}

// ReprioritizeArvanCloudWafRules is a fast operation.
type ReprioritizeArvanCloudWafRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewReprioritizeArvanCloudWafRules builds the use case from its ports.
func NewReprioritizeArvanCloudWafRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ReprioritizeArvanCloudWafRules {
	return &ReprioritizeArvanCloudWafRules{queue: queue, provider: provider}
}

// Execute moves RuleID to its new position. Reuses
// validateArvanCloudReprioritize (arvancloud_firewall.go): the spec's
// ReprioritizeRuleRequest schema is identical for both capabilities.
func (uc *ReprioritizeArvanCloudWafRules) Execute(ctx context.Context, in ReprioritizeArvanCloudWafRulesInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudReprioritize(in.RuleID, in.AfterRuleID, in.BeforeRuleID); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.ReprioritizeArvanCloudWafRules(ctx, in.Credentials, in.Domain, in.RuleID, in.AfterRuleID, in.BeforeRuleID); err != nil {
			return nil, fmt.Errorf("reprioritizing arvancloud waf rule %q on domain %q: %w", in.RuleID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// validateArvanCloudWafReprioritizePackage checks the shared shape of a
// package-reprioritize call: package_id is always required
// (WafReprioritize's own required field), and after_package_id/
// before_package_id must not both be given — same "only one of the two"
// contract as validateArvanCloudReprioritize, spelled out separately here
// because WafReprioritize is its own schema (package_id/after_package_id/
// before_package_id), not a reuse of ReprioritizeRuleRequest.
func validateArvanCloudWafReprioritizePackage(packageID, afterPackageID, beforePackageID string) error {
	if packageID == "" {
		return fmt.Errorf("package_id is required: %w", domain.ErrInvalidInput)
	}
	if afterPackageID != "" && beforePackageID != "" {
		return fmt.Errorf("only one of after_package_id or before_package_id may be given, not both: %w", domain.ErrInvalidInput)
	}
	return nil
}

// ReprioritizeArvanCloudWafPackagesInput identifies the domain, the
// installed package to move, and exactly one of
// AfterPackageID/BeforePackageID to move it relative to.
type ReprioritizeArvanCloudWafPackagesInput struct {
	Credentials     domain.ProviderCredentials
	Domain          string
	PackageID       string
	AfterPackageID  string
	BeforePackageID string
}

// ReprioritizeArvanCloudWafPackages is a fast operation.
type ReprioritizeArvanCloudWafPackages struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewReprioritizeArvanCloudWafPackages builds the use case from its ports.
func NewReprioritizeArvanCloudWafPackages(queue ports.Queue, provider ports.ArvanCloudProvider) *ReprioritizeArvanCloudWafPackages {
	return &ReprioritizeArvanCloudWafPackages{queue: queue, provider: provider}
}

// Execute moves PackageID to its new position.
func (uc *ReprioritizeArvanCloudWafPackages) Execute(ctx context.Context, in ReprioritizeArvanCloudWafPackagesInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudWafReprioritizePackage(in.PackageID, in.AfterPackageID, in.BeforePackageID); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.ReprioritizeArvanCloudWafPackages(ctx, in.Credentials, in.Domain, in.PackageID, in.AfterPackageID, in.BeforePackageID); err != nil {
			return nil, fmt.Errorf("reprioritizing arvancloud waf package %q on domain %q: %w", in.PackageID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Per-domain WAF custom rules --------------------------------------------

// validateArvanCloudWafRuleInput checks the fields every create/update WAF
// custom rule call shares: url_pattern, a non-oversized sources list (the
// spec's maxItems: 20), and an action from
// domain.ValidArvanCloudWafRuleAction's set.
func validateArvanCloudWafRuleInput(urlPattern string, sources []string, action domain.ArvanCloudWafRuleAction) error {
	if urlPattern == "" {
		return fmt.Errorf("url_pattern is required: %w", domain.ErrInvalidInput)
	}
	if len(sources) > 20 {
		return fmt.Errorf("sources may contain at most 20 entries, got %d: %w", len(sources), domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudWafRuleAction(string(action)) {
		return fmt.Errorf("action %q is not one of \"protect\" or \"passthrough\": %w", action, domain.ErrInvalidInput)
	}
	return nil
}

// ListArvanCloudWafRulesInput identifies the domain whose WAF custom rules
// to list.
type ListArvanCloudWafRulesInput = arvanCloudWafDomainInput

// ListArvanCloudWafRules is a fast operation.
type ListArvanCloudWafRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudWafRules builds the use case from its ports.
func NewListArvanCloudWafRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudWafRules {
	return &ListArvanCloudWafRules{queue: queue, provider: provider}
}

// Execute returns every WAF custom rule configured for the domain.
func (uc *ListArvanCloudWafRules) Execute(ctx context.Context, in ListArvanCloudWafRulesInput) ([]domain.ArvanCloudWafRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListArvanCloudWafRules(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud waf rules of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.ArvanCloudWafRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding arvancloud waf rule list: %w", err)
	}
	return rules, nil
}

// CreateArvanCloudWafRuleInput is the normalized form of a
// create_arvancloud_waf_rule tool call.
type CreateArvanCloudWafRuleInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Rule        domain.ArvanCloudWafRule
}

// CreateArvanCloudWafRule creates a new WAF custom rule.
type CreateArvanCloudWafRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudWafRule builds the use case from its ports.
func NewCreateArvanCloudWafRule(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudWafRule {
	return &CreateArvanCloudWafRule{queue: queue, provider: provider}
}

// Execute validates the request and creates the rule, returning it as
// stored.
func (uc *CreateArvanCloudWafRule) Execute(ctx context.Context, in CreateArvanCloudWafRuleInput) (*domain.ArvanCloudWafRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudWafRuleInput(in.Rule.URLPattern, in.Rule.Sources, in.Rule.Action); err != nil {
		return nil, err
	}

	return dispatchArvanCloudWafRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudWafRule, error) {
		created, err := uc.provider.CreateArvanCloudWafRule(ctx, in.Credentials, in.Domain, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud waf rule on domain %q: %w", in.Domain, err)
		}
		return created, nil
	})
}

// arvanCloudWafRuleIDInput is embedded by every use case below that is
// scoped to exactly one WAF custom rule by domain + id.
type arvanCloudWafRuleIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
}

func (in arvanCloudWafRuleIDInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudWafRuleInput identifies the WAF custom rule to look up.
type GetArvanCloudWafRuleInput = arvanCloudWafRuleIDInput

// GetArvanCloudWafRule is a fast operation.
type GetArvanCloudWafRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudWafRule builds the use case from its ports.
func NewGetArvanCloudWafRule(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudWafRule {
	return &GetArvanCloudWafRule{queue: queue, provider: provider}
}

// Execute returns the current state of one WAF custom rule.
func (uc *GetArvanCloudWafRule) Execute(ctx context.Context, in GetArvanCloudWafRuleInput) (*domain.ArvanCloudWafRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudWafRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudWafRule, error) {
		found, err := uc.provider.GetArvanCloudWafRule(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud waf rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudWafRuleInput identifies the WAF custom rule to update and
// its new field values.
type UpdateArvanCloudWafRuleInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Rule        domain.ArvanCloudWafRule
}

// UpdateArvanCloudWafRule changes a WAF custom rule. This is a fast
// operation.
type UpdateArvanCloudWafRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudWafRule builds the use case from its ports.
func NewUpdateArvanCloudWafRule(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudWafRule {
	return &UpdateArvanCloudWafRule{queue: queue, provider: provider}
}

// Execute updates the rule and returns it as stored afterward.
func (uc *UpdateArvanCloudWafRule) Execute(ctx context.Context, in UpdateArvanCloudWafRuleInput) (*domain.ArvanCloudWafRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudWafRuleInput(in.Rule.URLPattern, in.Rule.Sources, in.Rule.Action); err != nil {
		return nil, err
	}

	return dispatchArvanCloudWafRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudWafRule, error) {
		updated, err := uc.provider.UpdateArvanCloudWafRule(ctx, in.Credentials, in.Domain, in.ID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud waf rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudWafRuleInput identifies the WAF custom rule to remove.
type DeleteArvanCloudWafRuleInput = arvanCloudWafRuleIDInput

// DeleteArvanCloudWafRule is a fast operation. Deleting a rule the provider
// no longer has is treated as already done rather than an error, matching
// DeleteArvanCloudFirewallRule's tolerant-delete contract.
type DeleteArvanCloudWafRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudWafRule builds the use case from its ports.
func NewDeleteArvanCloudWafRule(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudWafRule {
	return &DeleteArvanCloudWafRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteArvanCloudWafRule) Execute(ctx context.Context, in DeleteArvanCloudWafRuleInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudWafRule(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud waf rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Per-domain WAF package subscriptions -----------------------------------

// ListArvanCloudWafDomainPackagesInput identifies the domain whose installed
// WAF packages to list.
type ListArvanCloudWafDomainPackagesInput = arvanCloudWafDomainInput

// ListArvanCloudWafDomainPackages is a fast operation.
type ListArvanCloudWafDomainPackages struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudWafDomainPackages builds the use case from its ports.
func NewListArvanCloudWafDomainPackages(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudWafDomainPackages {
	return &ListArvanCloudWafDomainPackages{queue: queue, provider: provider}
}

// Execute returns every WAF package currently installed on the domain.
func (uc *ListArvanCloudWafDomainPackages) Execute(ctx context.Context, in ListArvanCloudWafDomainPackagesInput) ([]domain.ArvanCloudWafPackage, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		packages, err := uc.provider.ListArvanCloudWafDomainPackages(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud waf packages installed on domain %q: %w", in.Domain, err)
		}
		return json.Marshal(packages)
	})
	if err != nil {
		return nil, err
	}

	var packages []domain.ArvanCloudWafPackage
	if err := json.Unmarshal(raw, &packages); err != nil {
		return nil, fmt.Errorf("decoding arvancloud waf domain package list: %w", err)
	}
	return packages, nil
}

// InstallArvanCloudWafPackageInput identifies the domain and the global
// package to subscribe it to.
type InstallArvanCloudWafPackageInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	PackageID   string
}

// InstallArvanCloudWafPackage is a fast operation. Not assumed idempotent
// (ports.ArvanCloudProvider's own doc comment); a caller that must not
// duplicate an install on retry checks first, e.g. via
// ListArvanCloudWafDomainPackages.
type InstallArvanCloudWafPackage struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewInstallArvanCloudWafPackage builds the use case from its ports.
func NewInstallArvanCloudWafPackage(queue ports.Queue, provider ports.ArvanCloudProvider) *InstallArvanCloudWafPackage {
	return &InstallArvanCloudWafPackage{queue: queue, provider: provider}
}

// Execute installs the package and returns it as stored.
func (uc *InstallArvanCloudWafPackage) Execute(ctx context.Context, in InstallArvanCloudWafPackageInput) (*domain.ArvanCloudWafPackage, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.PackageID == "" {
		return nil, fmt.Errorf("package_id is required: %w", domain.ErrInvalidInput)
	}

	return dispatchArvanCloudWafPackage(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudWafPackage, error) {
		installed, err := uc.provider.InstallArvanCloudWafPackage(ctx, in.Credentials, in.Domain, in.PackageID)
		if err != nil {
			return nil, fmt.Errorf("installing arvancloud waf package %q on domain %q: %w", in.PackageID, in.Domain, err)
		}
		return installed, nil
	})
}

// arvanCloudWafDomainPackageIDInput is embedded by every use case below that
// is scoped to exactly one installed WAF package by domain + id.
type arvanCloudWafDomainPackageIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
}

func (in arvanCloudWafDomainPackageIDInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudWafDomainPackageInput identifies the installed WAF package to
// look up.
type GetArvanCloudWafDomainPackageInput = arvanCloudWafDomainPackageIDInput

// GetArvanCloudWafDomainPackage is a fast operation.
type GetArvanCloudWafDomainPackage struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudWafDomainPackage builds the use case from its ports.
func NewGetArvanCloudWafDomainPackage(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudWafDomainPackage {
	return &GetArvanCloudWafDomainPackage{queue: queue, provider: provider}
}

// Execute returns one installed WAF package's details, including its
// rulesets.
func (uc *GetArvanCloudWafDomainPackage) Execute(ctx context.Context, in GetArvanCloudWafDomainPackageInput) (*domain.ArvanCloudWafPackage, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudWafPackage(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudWafPackage, error) {
		found, err := uc.provider.GetArvanCloudWafDomainPackage(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud waf package %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudWafDomainPackageInput identifies the installed WAF package
// to update and its new configuration (IsEnabled, DisabledRules,
// DisabledRulesets, Params).
type UpdateArvanCloudWafDomainPackageInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Package     domain.ArvanCloudWafPackage
}

// UpdateArvanCloudWafDomainPackage changes an installed package's own
// configuration, e.g. toggling it or selectively disabling its rules/
// rulesets. This is a fast operation.
type UpdateArvanCloudWafDomainPackage struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudWafDomainPackage builds the use case from its ports.
func NewUpdateArvanCloudWafDomainPackage(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudWafDomainPackage {
	return &UpdateArvanCloudWafDomainPackage{queue: queue, provider: provider}
}

// Execute updates the installed package and returns it as stored afterward.
func (uc *UpdateArvanCloudWafDomainPackage) Execute(ctx context.Context, in UpdateArvanCloudWafDomainPackageInput) (*domain.ArvanCloudWafPackage, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}

	return dispatchArvanCloudWafPackage(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudWafPackage, error) {
		updated, err := uc.provider.UpdateArvanCloudWafDomainPackage(ctx, in.Credentials, in.Domain, in.ID, in.Package)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud waf package %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// UninstallArvanCloudWafPackageInput identifies the installed WAF package to
// remove.
type UninstallArvanCloudWafPackageInput = arvanCloudWafDomainPackageIDInput

// UninstallArvanCloudWafPackage is a fast operation. Uninstalling a package
// the provider no longer has installed is treated as already done rather
// than an error, matching DeleteArvanCloudWafRule's tolerant-delete
// contract.
type UninstallArvanCloudWafPackage struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUninstallArvanCloudWafPackage builds the use case from its ports.
func NewUninstallArvanCloudWafPackage(queue ports.Queue, provider ports.ArvanCloudProvider) *UninstallArvanCloudWafPackage {
	return &UninstallArvanCloudWafPackage{queue: queue, provider: provider}
}

// Execute uninstalls the package, tolerating one that is already gone.
func (uc *UninstallArvanCloudWafPackage) Execute(ctx context.Context, in UninstallArvanCloudWafPackageInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UninstallArvanCloudWafPackage(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("uninstalling arvancloud waf package %q from domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Dispatch helpers -------------------------------------------------

// dispatchArvanCloudWafSettings runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudWafSettings.
func dispatchArvanCloudWafSettings(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudWafSettings, error),
) (*domain.ArvanCloudWafSettings, error) {
	raw, err := queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudWafSettings
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud waf settings: %w", err)
	}
	return &result, nil
}

// dispatchArvanCloudWafRule runs fn on the queue and decodes its result back
// into a *domain.ArvanCloudWafRule, the shape every WAF custom rule use case
// above but list/delete returns.
func dispatchArvanCloudWafRule(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudWafRule, error),
) (*domain.ArvanCloudWafRule, error) {
	raw, err := queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudWafRule
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud waf rule: %w", err)
	}
	return &result, nil
}

// dispatchArvanCloudWafPackage runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudWafPackage, the shape every WAF package use
// case above but list returns — shared by both the global and per-domain
// package methods, since both report the same domain.ArvanCloudWafPackage
// shape (domain/arvancloud_waf.go's doc comment).
func dispatchArvanCloudWafPackage(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudWafPackage, error),
) (*domain.ArvanCloudWafPackage, error) {
	raw, err := queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudWafPackage
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud waf package: %w", err)
	}
	return &result, nil
}
