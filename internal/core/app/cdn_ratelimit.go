package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// cdnRateLimitProvider is the subset of provider operations this file's use
// cases need: CDN edge-firewall Rate Limit Rules and the zone-wide Upstream
// Errors toggle (issue #24). ports.ParspackProvider is being extended
// centrally with these exact methods elsewhere in this effort; *parspack.Client
// already implements them structurally, so it satisfies ports.ParspackProvider
// once that integration lands. Declared locally here — not added to
// ports.ParspackProvider directly — because internal/core/ports/ports.go is
// being merged from several concurrent slices of issue #24 and must not be
// edited from this file.
type cdnRateLimitProvider interface {
	ListCDNRateLimitRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNRateLimitRule, error)
	CreateCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNRateLimitRule) (*domain.CDNRateLimitRule, error)
	GetCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNRateLimitRule, error)
	UpdateCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNRateLimitRule) (*domain.CDNRateLimitRule, error)
	DeleteCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error
	UpdateCDNRateLimitRulePriority(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, priority int) error
	GetCDNUpstreamErrors(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNUpstreamErrorSettings, error)
	UpdateCDNUpstreamErrors(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (*domain.CDNUpstreamErrorSettings, error)
}

// validateCDNRateLimitRule checks a rule's shape against the enums the CDN
// API confirms (AGENTS.md 4.5, issue #24), so a bad interval type or
// challenge fails fast here instead of reaching the provider and coming
// back as a 422.
func validateCDNRateLimitRule(rule domain.CDNRateLimitRule) error {
	if rule.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if rule.Value == "" {
		return fmt.Errorf("value is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidCDNRateLimitStaticIntervalType(rule.StaticIntervalType) {
		return fmt.Errorf("static_interval_type %q is not one of the values Parspack accepts: %w", rule.StaticIntervalType, domain.ErrInvalidInput)
	}
	if rule.StaticInterval < 0 {
		return fmt.Errorf("static_interval must not be negative: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidCDNRateLimitDynamicIntervalType(rule.DynamicIntervalType) {
		return fmt.Errorf("dynamic_interval_type %q is not one of the values Parspack accepts: %w", rule.DynamicIntervalType, domain.ErrInvalidInput)
	}
	if rule.DynamicInterval < 0 {
		return fmt.Errorf("dynamic_interval must not be negative: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidCDNRateLimitChallenge(rule.Challenge) {
		return fmt.Errorf("challenge %q is not one of the values Parspack accepts: %w", rule.Challenge, domain.ErrInvalidInput)
	}
	if rule.TrustTime < 0 {
		return fmt.Errorf("trust_time must not be negative: %w", domain.ErrInvalidInput)
	}
	if rule.AttackBanTime < 0 {
		return fmt.Errorf("attack_ban_time must not be negative: %w", domain.ErrInvalidInput)
	}
	return nil
}

// ListCDNRateLimitRulesInput identifies the zone whose rate limit rules to
// list.
type ListCDNRateLimitRulesInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// ListCDNRateLimitRules is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type ListCDNRateLimitRules struct {
	queue    ports.Queue
	provider cdnRateLimitProvider
}

// NewListCDNRateLimitRules builds the use case from its ports.
func NewListCDNRateLimitRules(queue ports.Queue, provider cdnRateLimitProvider) *ListCDNRateLimitRules {
	return &ListCDNRateLimitRules{queue: queue, provider: provider}
}

// Execute returns every rate limit rule configured on the given zone.
func (uc *ListCDNRateLimitRules) Execute(ctx context.Context, in ListCDNRateLimitRulesInput) ([]domain.CDNRateLimitRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListCDNRateLimitRules(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing rate limit rules of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.CDNRateLimitRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding rate limit rule list: %w", err)
	}
	return rules, nil
}

// CreateCDNRateLimitRuleInput is the normalized form of a
// create_cdn_rate_limit_rule tool call.
type CreateCDNRateLimitRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Rule        domain.CDNRateLimitRule
}

// CreateCDNRateLimitRule is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type CreateCDNRateLimitRule struct {
	queue    ports.Queue
	provider cdnRateLimitProvider
}

// NewCreateCDNRateLimitRule builds the use case from its ports.
func NewCreateCDNRateLimitRule(queue ports.Queue, provider cdnRateLimitProvider) *CreateCDNRateLimitRule {
	return &CreateCDNRateLimitRule{queue: queue, provider: provider}
}

// Execute validates the rule and creates it. The provider's store endpoint
// does not return the created rule's ID, so the caller should follow up
// with ListCDNRateLimitRules to discover it.
func (uc *CreateCDNRateLimitRule) Execute(ctx context.Context, in CreateCDNRateLimitRuleInput) (*domain.CDNRateLimitRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNRateLimitRule(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		created, err := uc.provider.CreateCDNRateLimitRule(ctx, in.Credentials, in.ZoneUUID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating rate limit rule %q in zone %s: %w", in.Rule.Name, in.ZoneUUID, err)
		}
		return json.Marshal(created)
	})
	if err != nil {
		return nil, err
	}

	var created domain.CDNRateLimitRule
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, fmt.Errorf("decoding created rate limit rule: %w", err)
	}
	return &created, nil
}

// GetCDNRateLimitRuleInput identifies the rule to look up.
type GetCDNRateLimitRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// GetCDNRateLimitRule is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNRateLimitRule struct {
	queue    ports.Queue
	provider cdnRateLimitProvider
}

// NewGetCDNRateLimitRule builds the use case from its ports.
func NewGetCDNRateLimitRule(queue ports.Queue, provider cdnRateLimitProvider) *GetCDNRateLimitRule {
	return &GetCDNRateLimitRule{queue: queue, provider: provider}
}

// Execute returns the current state of one rate limit rule.
func (uc *GetCDNRateLimitRule) Execute(ctx context.Context, in GetCDNRateLimitRuleInput) (*domain.CDNRateLimitRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("rate_limit_rule_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rule, err := uc.provider.GetCDNRateLimitRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID)
		if err != nil {
			return nil, fmt.Errorf("getting rate limit rule %s of zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(rule)
	})
	if err != nil {
		return nil, err
	}

	var rule domain.CDNRateLimitRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, fmt.Errorf("decoding rate limit rule: %w", err)
	}
	return &rule, nil
}

// UpdateCDNRateLimitRuleInput is the normalized form of an
// update_cdn_rate_limit_rule tool call.
type UpdateCDNRateLimitRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
	Rule        domain.CDNRateLimitRule
}

// UpdateCDNRateLimitRule is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNRateLimitRule struct {
	queue    ports.Queue
	provider cdnRateLimitProvider
}

// NewUpdateCDNRateLimitRule builds the use case from its ports.
func NewUpdateCDNRateLimitRule(queue ports.Queue, provider cdnRateLimitProvider) *UpdateCDNRateLimitRule {
	return &UpdateCDNRateLimitRule{queue: queue, provider: provider}
}

// Execute validates the rule and replaces the existing rule's configuration
// by ID.
func (uc *UpdateCDNRateLimitRule) Execute(ctx context.Context, in UpdateCDNRateLimitRuleInput) (*domain.CDNRateLimitRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("rate_limit_rule_id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNRateLimitRule(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		updated, err := uc.provider.UpdateCDNRateLimitRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating rate limit rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(updated)
	})
	if err != nil {
		return nil, err
	}

	var updated domain.CDNRateLimitRule
	if err := json.Unmarshal(raw, &updated); err != nil {
		return nil, fmt.Errorf("decoding updated rate limit rule: %w", err)
	}
	return &updated, nil
}

// DeleteCDNRateLimitRuleInput identifies the rule to remove.
type DeleteCDNRateLimitRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// DeleteCDNRateLimitRule is a fast operation. Deleting a rule the provider
// no longer has is treated as already-done rather than an error, so callers
// can call it more than once safely.
type DeleteCDNRateLimitRule struct {
	queue    ports.Queue
	provider cdnRateLimitProvider
}

// NewDeleteCDNRateLimitRule builds the use case from its ports.
func NewDeleteCDNRateLimitRule(queue ports.Queue, provider cdnRateLimitProvider) *DeleteCDNRateLimitRule {
	return &DeleteCDNRateLimitRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteCDNRateLimitRule) Execute(ctx context.Context, in DeleteCDNRateLimitRuleInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return fmt.Errorf("rate_limit_rule_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNRateLimitRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting rate limit rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// UpdateCDNRateLimitRulePriorityInput identifies the rule and its new
// evaluation priority.
type UpdateCDNRateLimitRulePriorityInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
	Priority    int
}

// UpdateCDNRateLimitRulePriority is a fast operation: it runs on a worker
// but the caller waits for the result inside the same tool call.
type UpdateCDNRateLimitRulePriority struct {
	queue    ports.Queue
	provider cdnRateLimitProvider
}

// NewUpdateCDNRateLimitRulePriority builds the use case from its ports.
func NewUpdateCDNRateLimitRulePriority(queue ports.Queue, provider cdnRateLimitProvider) *UpdateCDNRateLimitRulePriority {
	return &UpdateCDNRateLimitRulePriority{queue: queue, provider: provider}
}

// Execute reorders the rule's evaluation priority among the zone's other
// rate limit rules.
func (uc *UpdateCDNRateLimitRulePriority) Execute(ctx context.Context, in UpdateCDNRateLimitRulePriorityInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return fmt.Errorf("rate_limit_rule_id is required: %w", domain.ErrInvalidInput)
	}
	if in.Priority < 1 {
		return fmt.Errorf("priority must be at least 1: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UpdateCDNRateLimitRulePriority(ctx, in.Credentials, in.ZoneUUID, in.RuleID, in.Priority); err != nil {
			return nil, fmt.Errorf("updating priority of rate limit rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// GetCDNUpstreamErrorsInput identifies the zone whose upstream errors
// setting to look up.
type GetCDNUpstreamErrorsInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNUpstreamErrors is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNUpstreamErrors struct {
	queue    ports.Queue
	provider cdnRateLimitProvider
}

// NewGetCDNUpstreamErrors builds the use case from its ports.
func NewGetCDNUpstreamErrors(queue ports.Queue, provider cdnRateLimitProvider) *GetCDNUpstreamErrors {
	return &GetCDNUpstreamErrors{queue: queue, provider: provider}
}

// Execute returns the zone's current upstream errors setting.
func (uc *GetCDNUpstreamErrors) Execute(ctx context.Context, in GetCDNUpstreamErrorsInput) (*domain.CDNUpstreamErrorSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		settings, err := uc.provider.GetCDNUpstreamErrors(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting upstream errors setting of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(settings)
	})
	if err != nil {
		return nil, err
	}

	var settings domain.CDNUpstreamErrorSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decoding upstream errors setting: %w", err)
	}
	return &settings, nil
}

// UpdateCDNUpstreamErrorsInput identifies the zone and the desired upstream
// errors setting.
type UpdateCDNUpstreamErrorsInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Enabled     bool
}

// UpdateCDNUpstreamErrors is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNUpstreamErrors struct {
	queue    ports.Queue
	provider cdnRateLimitProvider
}

// NewUpdateCDNUpstreamErrors builds the use case from its ports.
func NewUpdateCDNUpstreamErrors(queue ports.Queue, provider cdnRateLimitProvider) *UpdateCDNUpstreamErrors {
	return &UpdateCDNUpstreamErrors{queue: queue, provider: provider}
}

// Execute sets the zone's upstream errors setting.
func (uc *UpdateCDNUpstreamErrors) Execute(ctx context.Context, in UpdateCDNUpstreamErrorsInput) (*domain.CDNUpstreamErrorSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		settings, err := uc.provider.UpdateCDNUpstreamErrors(ctx, in.Credentials, in.ZoneUUID, in.Enabled)
		if err != nil {
			return nil, fmt.Errorf("updating upstream errors setting of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(settings)
	})
	if err != nil {
		return nil, err
	}

	var settings domain.CDNUpstreamErrorSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decoding updated upstream errors setting: %w", err)
	}
	return &settings, nil
}
