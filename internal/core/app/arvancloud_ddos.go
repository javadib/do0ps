package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the DDoS Protection use cases for ArvanCloud (issue #67):
// a per-domain challenge engine (cookie/JavaScript/CAPTCHA-based), distinct
// from both the CDN edge Firewall (arvancloud_firewall.go) and the WAF
// managed rule-set engine (arvancloud_waf.go) — see
// domain/arvancloud_ddos.go's package comment for the naming-collision
// warning. Every one of them is a fast operation (ports.ArvanCloudProvider,
// AGENTS.md 4.3): each dispatches onto the queue and blocks for the result
// within the same tool call.
//
// What IS validated client-side, per issue #67's acceptance criteria:
//
//   - ProtectionMode against domain.ValidArvanCloudDdosProtectionMode
//     ("off"/"cookie"/"javascript"/"captcha" — see
//     domain.ArvanCloudDdosProtectionMode's doc comment for the confirmed
//     wire encoding).
//   - CaptchaService against domain.ValidArvanCloudCaptchaService, but ONLY
//     when ProtectionMode is "captcha" — see
//     validateArvanCloudDdosSettingsInput below and its test,
//     TestUpdateArvanCloudDdosSettingsCaptchaServiceOnlyRequiredForCaptchaMode.
//   - A rule's Action against domain.ValidArvanCloudDdosRuleAction
//     ("protect"/"passthrough" only).
//   - A reprioritize call's "exactly one of after/before" contract, reusing
//     validateArvanCloudReprioritize (arvancloud_firewall.go): the spec's
//     ReprioritizeRuleRequest schema is identical across Firewall, WAF and
//     DDoS.

// validateArvanCloudDdosSettingsInput checks the fields an update DDoS
// settings call must satisfy: ProtectionMode against
// domain.ValidArvanCloudDdosProtectionMode, and — only when ProtectionMode
// is "captcha" — CaptchaService against domain.ValidArvanCloudCaptchaService.
// A non-captcha ProtectionMode leaves CaptchaService/SiteKey/SecretKey
// unvalidated and unused; the adapter never sends them in that case (see
// ddosSettingsRequestBody).
func validateArvanCloudDdosSettingsInput(settings domain.ArvanCloudDdosSettings) error {
	if !domain.ValidArvanCloudDdosProtectionMode(string(settings.ProtectionMode)) {
		return fmt.Errorf(
			"protection_mode %q is not one of \"off\", \"cookie\", \"javascript\" or \"captcha\": %w",
			settings.ProtectionMode, domain.ErrInvalidInput)
	}
	if settings.ProtectionMode == domain.ArvanCloudDdosProtectionModeCaptcha {
		if !domain.ValidArvanCloudCaptchaService(string(settings.CaptchaService)) {
			return fmt.Errorf(
				"captcha_service %q is not one of \"recaptcha\", \"arcaptcha\" or \"hcaptcha\" (required when protection_mode is \"captcha\"): %w",
				settings.CaptchaService, domain.ErrInvalidInput)
		}
	}
	return nil
}

// --- Per-domain DDoS settings -----------------------------------------------

// arvanCloudDdosDomainInput is embedded by every use case below that is
// scoped to exactly one domain by name and needs nothing else.
type arvanCloudDdosDomainInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

func (in arvanCloudDdosDomainInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudDdosSettingsInput identifies the domain whose DDoS settings
// to look up.
type GetArvanCloudDdosSettingsInput = arvanCloudDdosDomainInput

// GetArvanCloudDdosSettings is a fast operation.
type GetArvanCloudDdosSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudDdosSettings builds the use case from its ports.
func NewGetArvanCloudDdosSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudDdosSettings {
	return &GetArvanCloudDdosSettings{queue: queue, provider: provider}
}

// Execute returns the domain's current DDoS protection settings.
func (uc *GetArvanCloudDdosSettings) Execute(ctx context.Context, in GetArvanCloudDdosSettingsInput) (*domain.ArvanCloudDdosSettings, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDdosSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDdosSettings, error) {
		found, err := uc.provider.GetArvanCloudDdosSettings(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud ddos settings for domain %q: %w", in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudDdosSettingsInput identifies the domain and its new DDoS
// protection configuration.
type UpdateArvanCloudDdosSettingsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Settings    domain.ArvanCloudDdosSettings
}

// UpdateArvanCloudDdosSettings changes a domain's DDoS protection
// configuration. This is a fast operation.
type UpdateArvanCloudDdosSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudDdosSettings builds the use case from its ports.
func NewUpdateArvanCloudDdosSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudDdosSettings {
	return &UpdateArvanCloudDdosSettings{queue: queue, provider: provider}
}

// Execute validates the request — see validateArvanCloudDdosSettingsInput —
// and updates the settings, returning them as stored afterward.
func (uc *UpdateArvanCloudDdosSettings) Execute(ctx context.Context, in UpdateArvanCloudDdosSettingsInput) (*domain.ArvanCloudDdosSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudDdosSettingsInput(in.Settings); err != nil {
		return nil, err
	}

	return dispatchArvanCloudDdosSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDdosSettings, error) {
		updated, err := uc.provider.UpdateArvanCloudDdosSettings(ctx, in.Credentials, in.Domain, in.Settings)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud ddos settings for domain %q: %w", in.Domain, err)
		}
		return updated, nil
	})
}

// --- Per-domain DDoS rules ---------------------------------------------------

// validateArvanCloudDdosRuleInput checks the fields every create/update DDoS
// rule call shares: url_pattern, a non-oversized sources list (the spec's
// maxItems: 20), and an action from domain.ValidArvanCloudDdosRuleAction's
// set.
func validateArvanCloudDdosRuleInput(urlPattern string, sources []string, action domain.ArvanCloudDdosRuleAction) error {
	if urlPattern == "" {
		return fmt.Errorf("url_pattern is required: %w", domain.ErrInvalidInput)
	}
	if len(sources) > 20 {
		return fmt.Errorf("sources may contain at most 20 entries, got %d: %w", len(sources), domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudDdosRuleAction(string(action)) {
		return fmt.Errorf("action %q is not one of \"protect\" or \"passthrough\": %w", action, domain.ErrInvalidInput)
	}
	return nil
}

// ListArvanCloudDdosRulesInput identifies the domain whose DDoS rules to
// list.
type ListArvanCloudDdosRulesInput = arvanCloudDdosDomainInput

// ListArvanCloudDdosRules is a fast operation.
type ListArvanCloudDdosRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudDdosRules builds the use case from its ports.
func NewListArvanCloudDdosRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudDdosRules {
	return &ListArvanCloudDdosRules{queue: queue, provider: provider}
}

// Execute returns every DDoS rule configured for the domain.
func (uc *ListArvanCloudDdosRules) Execute(ctx context.Context, in ListArvanCloudDdosRulesInput) ([]domain.ArvanCloudDdosRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListArvanCloudDdosRules(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud ddos rules of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.ArvanCloudDdosRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding arvancloud ddos rule list: %w", err)
	}
	return rules, nil
}

// CreateArvanCloudDdosRuleInput is the normalized form of a
// create_arvancloud_ddos_rule tool call.
type CreateArvanCloudDdosRuleInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Rule        domain.ArvanCloudDdosRule
}

// CreateArvanCloudDdosRule creates a new DDoS rule.
type CreateArvanCloudDdosRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudDdosRule builds the use case from its ports.
func NewCreateArvanCloudDdosRule(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudDdosRule {
	return &CreateArvanCloudDdosRule{queue: queue, provider: provider}
}

// Execute validates the request and creates the rule, returning it as
// stored.
func (uc *CreateArvanCloudDdosRule) Execute(ctx context.Context, in CreateArvanCloudDdosRuleInput) (*domain.ArvanCloudDdosRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudDdosRuleInput(in.Rule.URLPattern, in.Rule.Sources, in.Rule.Action); err != nil {
		return nil, err
	}

	return dispatchArvanCloudDdosRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDdosRule, error) {
		created, err := uc.provider.CreateArvanCloudDdosRule(ctx, in.Credentials, in.Domain, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud ddos rule on domain %q: %w", in.Domain, err)
		}
		return created, nil
	})
}

// arvanCloudDdosRuleIDInput is embedded by every use case below that is
// scoped to exactly one DDoS rule by domain + id.
type arvanCloudDdosRuleIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
}

func (in arvanCloudDdosRuleIDInput) validate() error {
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

// GetArvanCloudDdosRuleInput identifies the DDoS rule to look up.
type GetArvanCloudDdosRuleInput = arvanCloudDdosRuleIDInput

// GetArvanCloudDdosRule is a fast operation.
type GetArvanCloudDdosRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudDdosRule builds the use case from its ports.
func NewGetArvanCloudDdosRule(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudDdosRule {
	return &GetArvanCloudDdosRule{queue: queue, provider: provider}
}

// Execute returns the current state of one DDoS rule.
func (uc *GetArvanCloudDdosRule) Execute(ctx context.Context, in GetArvanCloudDdosRuleInput) (*domain.ArvanCloudDdosRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDdosRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDdosRule, error) {
		found, err := uc.provider.GetArvanCloudDdosRule(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud ddos rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudDdosRuleInput identifies the DDoS rule to update and its
// new field values.
type UpdateArvanCloudDdosRuleInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Rule        domain.ArvanCloudDdosRule
}

// UpdateArvanCloudDdosRule changes a DDoS rule. This is a fast operation.
type UpdateArvanCloudDdosRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudDdosRule builds the use case from its ports.
func NewUpdateArvanCloudDdosRule(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudDdosRule {
	return &UpdateArvanCloudDdosRule{queue: queue, provider: provider}
}

// Execute updates the rule and returns it as stored afterward.
func (uc *UpdateArvanCloudDdosRule) Execute(ctx context.Context, in UpdateArvanCloudDdosRuleInput) (*domain.ArvanCloudDdosRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudDdosRuleInput(in.Rule.URLPattern, in.Rule.Sources, in.Rule.Action); err != nil {
		return nil, err
	}

	return dispatchArvanCloudDdosRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDdosRule, error) {
		updated, err := uc.provider.UpdateArvanCloudDdosRule(ctx, in.Credentials, in.Domain, in.ID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud ddos rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudDdosRuleInput identifies the DDoS rule to remove.
type DeleteArvanCloudDdosRuleInput = arvanCloudDdosRuleIDInput

// DeleteArvanCloudDdosRule is a fast operation. Deleting a rule the provider
// no longer has is treated as already done rather than an error, matching
// DeleteArvanCloudWafRule's tolerant-delete contract.
type DeleteArvanCloudDdosRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudDdosRule builds the use case from its ports.
func NewDeleteArvanCloudDdosRule(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudDdosRule {
	return &DeleteArvanCloudDdosRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteArvanCloudDdosRule) Execute(ctx context.Context, in DeleteArvanCloudDdosRuleInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudDdosRule(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud ddos rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ReprioritizeArvanCloudDdosRulesInput identifies the domain, the rule to
// move, and exactly one of AfterRuleID/BeforeRuleID to move it relative to.
type ReprioritizeArvanCloudDdosRulesInput struct {
	Credentials  domain.ProviderCredentials
	Domain       string
	RuleID       string
	AfterRuleID  string
	BeforeRuleID string
}

// ReprioritizeArvanCloudDdosRules is a fast operation.
type ReprioritizeArvanCloudDdosRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewReprioritizeArvanCloudDdosRules builds the use case from its ports.
func NewReprioritizeArvanCloudDdosRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ReprioritizeArvanCloudDdosRules {
	return &ReprioritizeArvanCloudDdosRules{queue: queue, provider: provider}
}

// Execute moves RuleID to its new position. Reuses
// validateArvanCloudReprioritize (arvancloud_firewall.go): the spec's
// ReprioritizeRuleRequest schema is identical across Firewall, WAF and DDoS.
func (uc *ReprioritizeArvanCloudDdosRules) Execute(ctx context.Context, in ReprioritizeArvanCloudDdosRulesInput) error {
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
		if err := uc.provider.ReprioritizeArvanCloudDdosRules(ctx, in.Credentials, in.Domain, in.RuleID, in.AfterRuleID, in.BeforeRuleID); err != nil {
			return nil, fmt.Errorf("reprioritizing arvancloud ddos rule %q on domain %q: %w", in.RuleID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Dispatch helpers -------------------------------------------------

// dispatchArvanCloudDdosSettings runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudDdosSettings.
func dispatchArvanCloudDdosSettings(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudDdosSettings, error),
) (*domain.ArvanCloudDdosSettings, error) {
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

	var result domain.ArvanCloudDdosSettings
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud ddos settings: %w", err)
	}
	return &result, nil
}

// dispatchArvanCloudDdosRule runs fn on the queue and decodes its result back
// into a *domain.ArvanCloudDdosRule, the shape every DDoS rule use case above
// but list/delete/reprioritize returns.
func dispatchArvanCloudDdosRule(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudDdosRule, error),
) (*domain.ArvanCloudDdosRule, error) {
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

	var result domain.ArvanCloudDdosRule
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud ddos rule: %w", err)
	}
	return &result, nil
}
