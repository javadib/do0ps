package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the Rate Limiting use cases for ArvanCloud (issue #68):
// per-domain request-rate settings plus a rule engine that throttles or
// blocks traffic exceeding a configured rate — distinct from the CDN edge
// Firewall (arvancloud_firewall.go), the WAF managed rule-set engine
// (arvancloud_waf.go) and the dedicated DDoS challenge engine
// (arvancloud_ddos.go) — see domain/arvancloud_ratelimit.go's package
// comment. Every one of them is a fast operation (ports.ArvanCloudProvider,
// AGENTS.md 4.3): each dispatches onto the queue and blocks for the result
// within the same tool call.
//
// What IS validated client-side, per issue #68's acceptance criteria:
//
//   - Rate and TimeDuration must both be positive — a rate limit of
//     0-per-0-seconds is meaningless and better caught before the call than
//     surfaced as an opaque 422 (validateArvanCloudRateLimitRuleInput and
//     its test).
//   - A rule's Action against domain.ValidArvanCloudRateLimitAction
//     ("challenge"/"block" only).
//   - ActionDetails.Mode against domain.ValidArvanCloudChallengeMode, but
//     ONLY when Action is "challenge" — mirroring
//     validateArvanCloudDdosSettingsInput's captcha_service-only-for-captcha
//     gating (arvancloud_ddos.go).
//   - A reprioritize call's "exactly one of after/before" contract, reusing
//     validateArvanCloudReprioritize (arvancloud_firewall.go): the spec's
//     ReprioritizeRuleRequest schema is identical across Firewall, WAF, DDoS
//     and Rate Limiting.

// --- Per-domain rate-limit settings ----------------------------------------

// arvanCloudRateLimitDomainInput is embedded by every use case below that is
// scoped to exactly one domain by name and needs nothing else.
type arvanCloudRateLimitDomainInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

func (in arvanCloudRateLimitDomainInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudRateLimitSettingsInput identifies the domain whose
// rate-limiting settings to look up.
type GetArvanCloudRateLimitSettingsInput = arvanCloudRateLimitDomainInput

// GetArvanCloudRateLimitSettings is a fast operation.
type GetArvanCloudRateLimitSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudRateLimitSettings builds the use case from its ports.
func NewGetArvanCloudRateLimitSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudRateLimitSettings {
	return &GetArvanCloudRateLimitSettings{queue: queue, provider: provider}
}

// Execute returns the domain's current rate-limiting settings.
func (uc *GetArvanCloudRateLimitSettings) Execute(ctx context.Context, in GetArvanCloudRateLimitSettingsInput) (*domain.ArvanCloudRateLimitSettings, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudRateLimitSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudRateLimitSettings, error) {
		found, err := uc.provider.GetArvanCloudRateLimitSettings(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud rate limit settings for domain %q: %w", in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudRateLimitSettingsInput identifies the domain and its new
// rate-limiting configuration.
type UpdateArvanCloudRateLimitSettingsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Settings    domain.ArvanCloudRateLimitSettings
}

// UpdateArvanCloudRateLimitSettings changes a domain's rate-limiting
// configuration. This is a fast operation.
type UpdateArvanCloudRateLimitSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudRateLimitSettings builds the use case from its ports.
func NewUpdateArvanCloudRateLimitSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudRateLimitSettings {
	return &UpdateArvanCloudRateLimitSettings{queue: queue, provider: provider}
}

// Execute updates the settings, returning them as stored afterward. Unlike
// UpdateArvanCloudDdosSettings, RateLimitSettings has no enum field to
// validate client-side (ddos_detection is a plain bool, exclude_sources a
// plain CIDR list).
func (uc *UpdateArvanCloudRateLimitSettings) Execute(ctx context.Context, in UpdateArvanCloudRateLimitSettingsInput) (*domain.ArvanCloudRateLimitSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}

	return dispatchArvanCloudRateLimitSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudRateLimitSettings, error) {
		updated, err := uc.provider.UpdateArvanCloudRateLimitSettings(ctx, in.Credentials, in.Domain, in.Settings)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud rate limit settings for domain %q: %w", in.Domain, err)
		}
		return updated, nil
	})
}

// --- Per-domain rate-limit rules --------------------------------------------

// validateArvanCloudRateLimitRuleInput checks the fields every create/update
// rate-limit rule call shares: url_pattern, positive rate/time_duration (the
// spec's two required numeric fields — issue #68's acceptance criteria
// requires rejecting zero or negative here rather than surfacing an opaque
// 422), an action from domain.ValidArvanCloudRateLimitAction's set, and —
// only when action is "challenge" — a mode from
// domain.ValidArvanCloudChallengeMode's set.
func validateArvanCloudRateLimitRuleInput(rule domain.ArvanCloudRateLimitRule) error {
	if rule.URLPattern == "" {
		return fmt.Errorf("url_pattern is required: %w", domain.ErrInvalidInput)
	}
	if rule.Rate <= 0 {
		return fmt.Errorf("rate must be positive, got %d: %w", rule.Rate, domain.ErrInvalidInput)
	}
	if rule.TimeDuration <= 0 {
		return fmt.Errorf("time_duration must be positive, got %d: %w", rule.TimeDuration, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudRateLimitAction(string(rule.Action)) {
		return fmt.Errorf("action %q is not one of \"challenge\" or \"block\": %w", rule.Action, domain.ErrInvalidInput)
	}
	if rule.Action == domain.ArvanCloudRateLimitActionChallenge {
		if !domain.ValidArvanCloudChallengeMode(rule.ActionDetails.Mode) {
			return fmt.Errorf(
				"action_details.mode %d is not one of 1 (cookie), 2 (javascript) or 3 (captcha) (required when action is \"challenge\"): %w",
				rule.ActionDetails.Mode, domain.ErrInvalidInput)
		}
	}
	return nil
}

// ListArvanCloudRateLimitRulesInput identifies the domain whose rate-limit
// rules to list.
type ListArvanCloudRateLimitRulesInput = arvanCloudRateLimitDomainInput

// ListArvanCloudRateLimitRules is a fast operation.
type ListArvanCloudRateLimitRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudRateLimitRules builds the use case from its ports.
func NewListArvanCloudRateLimitRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudRateLimitRules {
	return &ListArvanCloudRateLimitRules{queue: queue, provider: provider}
}

// Execute returns every rate-limit rule configured for the domain.
func (uc *ListArvanCloudRateLimitRules) Execute(ctx context.Context, in ListArvanCloudRateLimitRulesInput) ([]domain.ArvanCloudRateLimitRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListArvanCloudRateLimitRules(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud rate limit rules of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.ArvanCloudRateLimitRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding arvancloud rate limit rule list: %w", err)
	}
	return rules, nil
}

// CreateArvanCloudRateLimitRuleInput is the normalized form of a
// create_arvancloud_rate_limit_rule tool call.
type CreateArvanCloudRateLimitRuleInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Rule        domain.ArvanCloudRateLimitRule
}

// CreateArvanCloudRateLimitRule creates a new rate-limit rule.
type CreateArvanCloudRateLimitRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudRateLimitRule builds the use case from its ports.
func NewCreateArvanCloudRateLimitRule(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudRateLimitRule {
	return &CreateArvanCloudRateLimitRule{queue: queue, provider: provider}
}

// Execute validates the request and creates the rule, returning it as
// stored.
func (uc *CreateArvanCloudRateLimitRule) Execute(ctx context.Context, in CreateArvanCloudRateLimitRuleInput) (*domain.ArvanCloudRateLimitRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudRateLimitRuleInput(in.Rule); err != nil {
		return nil, err
	}

	return dispatchArvanCloudRateLimitRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudRateLimitRule, error) {
		created, err := uc.provider.CreateArvanCloudRateLimitRule(ctx, in.Credentials, in.Domain, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud rate limit rule on domain %q: %w", in.Domain, err)
		}
		return created, nil
	})
}

// arvanCloudRateLimitRuleIDInput is embedded by every use case below that is
// scoped to exactly one rate-limit rule by domain + id.
type arvanCloudRateLimitRuleIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
}

func (in arvanCloudRateLimitRuleIDInput) validate() error {
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

// GetArvanCloudRateLimitRuleInput identifies the rate-limit rule to look up.
type GetArvanCloudRateLimitRuleInput = arvanCloudRateLimitRuleIDInput

// GetArvanCloudRateLimitRule is a fast operation.
type GetArvanCloudRateLimitRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudRateLimitRule builds the use case from its ports.
func NewGetArvanCloudRateLimitRule(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudRateLimitRule {
	return &GetArvanCloudRateLimitRule{queue: queue, provider: provider}
}

// Execute returns the current state of one rate-limit rule.
func (uc *GetArvanCloudRateLimitRule) Execute(ctx context.Context, in GetArvanCloudRateLimitRuleInput) (*domain.ArvanCloudRateLimitRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudRateLimitRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudRateLimitRule, error) {
		found, err := uc.provider.GetArvanCloudRateLimitRule(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud rate limit rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudRateLimitRuleInput identifies the rate-limit rule to
// update and its new field values.
type UpdateArvanCloudRateLimitRuleInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Rule        domain.ArvanCloudRateLimitRule
}

// UpdateArvanCloudRateLimitRule changes a rate-limit rule. This is a fast
// operation.
type UpdateArvanCloudRateLimitRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudRateLimitRule builds the use case from its ports.
func NewUpdateArvanCloudRateLimitRule(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudRateLimitRule {
	return &UpdateArvanCloudRateLimitRule{queue: queue, provider: provider}
}

// Execute updates the rule and returns it as stored afterward.
func (uc *UpdateArvanCloudRateLimitRule) Execute(ctx context.Context, in UpdateArvanCloudRateLimitRuleInput) (*domain.ArvanCloudRateLimitRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudRateLimitRuleInput(in.Rule); err != nil {
		return nil, err
	}

	return dispatchArvanCloudRateLimitRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudRateLimitRule, error) {
		updated, err := uc.provider.UpdateArvanCloudRateLimitRule(ctx, in.Credentials, in.Domain, in.ID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud rate limit rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudRateLimitRuleInput identifies the rate-limit rule to
// remove.
type DeleteArvanCloudRateLimitRuleInput = arvanCloudRateLimitRuleIDInput

// DeleteArvanCloudRateLimitRule is a fast operation. Deleting a rule the
// provider no longer has is treated as already done rather than an error,
// matching DeleteArvanCloudDdosRule's tolerant-delete contract.
type DeleteArvanCloudRateLimitRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudRateLimitRule builds the use case from its ports.
func NewDeleteArvanCloudRateLimitRule(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudRateLimitRule {
	return &DeleteArvanCloudRateLimitRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteArvanCloudRateLimitRule) Execute(ctx context.Context, in DeleteArvanCloudRateLimitRuleInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudRateLimitRule(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud rate limit rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ReprioritizeArvanCloudRateLimitRulesInput identifies the domain, the rule
// to move, and exactly one of AfterRuleID/BeforeRuleID to move it relative
// to.
type ReprioritizeArvanCloudRateLimitRulesInput struct {
	Credentials  domain.ProviderCredentials
	Domain       string
	RuleID       string
	AfterRuleID  string
	BeforeRuleID string
}

// ReprioritizeArvanCloudRateLimitRules is a fast operation.
type ReprioritizeArvanCloudRateLimitRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewReprioritizeArvanCloudRateLimitRules builds the use case from its
// ports.
func NewReprioritizeArvanCloudRateLimitRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ReprioritizeArvanCloudRateLimitRules {
	return &ReprioritizeArvanCloudRateLimitRules{queue: queue, provider: provider}
}

// Execute moves RuleID to its new position. Reuses
// validateArvanCloudReprioritize (arvancloud_firewall.go): the spec's
// ReprioritizeRuleRequest schema is identical across Firewall, WAF, DDoS and
// Rate Limiting.
func (uc *ReprioritizeArvanCloudRateLimitRules) Execute(ctx context.Context, in ReprioritizeArvanCloudRateLimitRulesInput) error {
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
		if err := uc.provider.ReprioritizeArvanCloudRateLimitRules(ctx, in.Credentials, in.Domain, in.RuleID, in.AfterRuleID, in.BeforeRuleID); err != nil {
			return nil, fmt.Errorf("reprioritizing arvancloud rate limit rule %q on domain %q: %w", in.RuleID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Dispatch helpers -------------------------------------------------

// dispatchArvanCloudRateLimitSettings runs fn on the queue and decodes its
// result back into a *domain.ArvanCloudRateLimitSettings.
func dispatchArvanCloudRateLimitSettings(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudRateLimitSettings, error),
) (*domain.ArvanCloudRateLimitSettings, error) {
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

	var result domain.ArvanCloudRateLimitSettings
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud rate limit settings: %w", err)
	}
	return &result, nil
}

// dispatchArvanCloudRateLimitRule runs fn on the queue and decodes its
// result back into a *domain.ArvanCloudRateLimitRule, the shape every
// rate-limit rule use case above but list/delete/reprioritize returns.
func dispatchArvanCloudRateLimitRule(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudRateLimitRule, error),
) (*domain.ArvanCloudRateLimitRule, error) {
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

	var result domain.ArvanCloudRateLimitRule
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud rate limit rule: %w", err)
	}
	return &result, nil
}
