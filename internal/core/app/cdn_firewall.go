package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// CreateFirewall/GetFirewall/ListFirewalls/UpdateFirewall/DeleteFirewall
// methods, which are the cloud-server/VM-network-level firewall (issue #11).

func validateCDNAccessRuleForCreate(rule domain.CDNAccessRule) error {
	if !domain.ValidCDNAccessRuleType(rule.Type) {
		return fmt.Errorf("type %q is not one of the match types Parspack accepts: %w", rule.Type, domain.ErrInvalidInput)
	}
	if !domain.ValidCDNAccessRuleAction(rule.Action) {
		return fmt.Errorf("action %q is not one of the actions Parspack accepts: %w", rule.Action, domain.ErrInvalidInput)
	}
	if rule.Value == "" && rule.BulklistID == "" {
		return fmt.Errorf("either value or bulklist_id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

func validateCDNAccessRuleForUpdate(rule domain.CDNAccessRule) error {
	if rule.ID == "" {
		return fmt.Errorf("access_management_id is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidCDNAccessRuleAction(rule.Action) {
		return fmt.Errorf("action %q is not one of the actions Parspack accepts: %w", rule.Action, domain.ErrInvalidInput)
	}
	if rule.Value == "" && rule.BulklistID == "" {
		return fmt.Errorf("either value or bulklist_id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// ListCDNAccessRulesInput identifies the zone whose rules to list.
type ListCDNAccessRulesInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// ListCDNAccessRules is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type ListCDNAccessRules struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListCDNAccessRules builds the use case from its ports.
func NewListCDNAccessRules(queue ports.Queue, provider ports.ParspackProvider) *ListCDNAccessRules {
	return &ListCDNAccessRules{queue: queue, provider: provider}
}

// Execute returns every CDN-edge access-management rule of one zone.
func (uc *ListCDNAccessRules) Execute(ctx context.Context, in ListCDNAccessRulesInput) ([]domain.CDNAccessRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListCDNAccessRules(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing CDN access rules of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.CDNAccessRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding CDN access rule list: %w", err)
	}
	return rules, nil
}

// GetCDNAccessRuleInput identifies the rule to look up.
type GetCDNAccessRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// GetCDNAccessRule is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNAccessRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNAccessRule builds the use case from its ports.
func NewGetCDNAccessRule(queue ports.Queue, provider ports.ParspackProvider) *GetCDNAccessRule {
	return &GetCDNAccessRule{queue: queue, provider: provider}
}

// Execute returns one access-management rule by ID.
func (uc *GetCDNAccessRule) Execute(ctx context.Context, in GetCDNAccessRuleInput) (*domain.CDNAccessRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("access_management_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rule, err := uc.provider.GetCDNAccessRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID)
		if err != nil {
			return nil, fmt.Errorf("getting CDN access rule %s of zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(rule)
	})
	if err != nil {
		return nil, err
	}

	var rule domain.CDNAccessRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, fmt.Errorf("decoding CDN access rule: %w", err)
	}
	return &rule, nil
}

// CreateCDNAccessRuleInput is the normalized form of a
// create_cdn_access_rule tool call.
type CreateCDNAccessRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Rule        domain.CDNAccessRule
}

// CreateCDNAccessRule is a fast operation: the created rule is returned
// within the same tool call, though without an ID or priority — the
// provider's create endpoint does not report them synchronously (see
// parspack.Client.CreateCDNAccessRule).
type CreateCDNAccessRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewCreateCDNAccessRule builds the use case from its ports.
func NewCreateCDNAccessRule(queue ports.Queue, provider ports.ParspackProvider) *CreateCDNAccessRule {
	return &CreateCDNAccessRule{queue: queue, provider: provider}
}

// Execute validates and creates a new access-management rule.
func (uc *CreateCDNAccessRule) Execute(ctx context.Context, in CreateCDNAccessRuleInput) (*domain.CDNAccessRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNAccessRuleForCreate(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rule, err := uc.provider.CreateCDNAccessRule(ctx, in.Credentials, in.ZoneUUID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating CDN access rule in zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(rule)
	})
	if err != nil {
		return nil, err
	}

	var rule domain.CDNAccessRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, fmt.Errorf("decoding created CDN access rule: %w", err)
	}
	return &rule, nil
}

// UpdateCDNAccessRuleInput is the normalized form of an
// update_cdn_access_rule tool call.
type UpdateCDNAccessRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Rule        domain.CDNAccessRule // Rule.ID selects which rule to update
}

// UpdateCDNAccessRule is a fast operation: the updated rule is returned
// within the same tool call (echoed back, since the provider's update
// endpoint reports no body — see parspack.Client.UpdateCDNAccessRule).
type UpdateCDNAccessRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNAccessRule builds the use case from its ports.
func NewUpdateCDNAccessRule(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNAccessRule {
	return &UpdateCDNAccessRule{queue: queue, provider: provider}
}

// Execute validates and updates an existing access-management rule.
func (uc *UpdateCDNAccessRule) Execute(ctx context.Context, in UpdateCDNAccessRuleInput) (*domain.CDNAccessRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNAccessRuleForUpdate(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rule, err := uc.provider.UpdateCDNAccessRule(ctx, in.Credentials, in.ZoneUUID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating CDN access rule %s in zone %s: %w", in.Rule.ID, in.ZoneUUID, err)
		}
		return json.Marshal(rule)
	})
	if err != nil {
		return nil, err
	}

	var rule domain.CDNAccessRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, fmt.Errorf("decoding updated CDN access rule: %w", err)
	}
	return &rule, nil
}

// DeleteCDNAccessRuleInput identifies the rule to remove.
type DeleteCDNAccessRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// DeleteCDNAccessRule is a fast operation. Deleting a rule the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely.
type DeleteCDNAccessRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteCDNAccessRule builds the use case from its ports.
func NewDeleteCDNAccessRule(queue ports.Queue, provider ports.ParspackProvider) *DeleteCDNAccessRule {
	return &DeleteCDNAccessRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteCDNAccessRule) Execute(ctx context.Context, in DeleteCDNAccessRuleInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return fmt.Errorf("access_management_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNAccessRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting CDN access rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// GetCDNIPReputationInput identifies the zone whose IP-reputation settings
// to look up.
type GetCDNIPReputationInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNIPReputation is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNIPReputation struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNIPReputation builds the use case from its ports.
func NewGetCDNIPReputation(queue ports.Queue, provider ports.ParspackProvider) *GetCDNIPReputation {
	return &GetCDNIPReputation{queue: queue, provider: provider}
}

// Execute returns the current IP-reputation blocking settings for a zone.
func (uc *GetCDNIPReputation) Execute(ctx context.Context, in GetCDNIPReputationInput) (*domain.CDNIPReputationSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		settings, err := uc.provider.GetCDNIPReputation(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting CDN IP reputation settings for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(settings)
	})
	if err != nil {
		return nil, err
	}

	var settings domain.CDNIPReputationSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decoding CDN IP reputation settings: %w", err)
	}
	return &settings, nil
}

// UpdateCDNIPReputationInput is the normalized form of an
// update_cdn_ip_reputation tool call. The provider requires every field on
// update, so all of these are mandatory.
type UpdateCDNIPReputationInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Settings    domain.CDNIPReputationSettings
}

// UpdateCDNIPReputation is a fast operation: the updated settings are
// returned within the same tool call.
type UpdateCDNIPReputation struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNIPReputation builds the use case from its ports.
func NewUpdateCDNIPReputation(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNIPReputation {
	return &UpdateCDNIPReputation{queue: queue, provider: provider}
}

// Execute validates and replaces a zone's IP-reputation settings.
func (uc *UpdateCDNIPReputation) Execute(ctx context.Context, in UpdateCDNIPReputationInput) (*domain.CDNIPReputationSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidCDNIPReputationTreatScore(in.Settings.TreatScore) {
		return nil, fmt.Errorf("ip_reputation_treat_score %q is not one of the thresholds Parspack accepts: %w",
			in.Settings.TreatScore, domain.ErrInvalidInput)
	}
	if !domain.ValidCDNIPReputationChallenge(in.Settings.Challenge) {
		return nil, fmt.Errorf("ip_reputation_challenge %q is not one of the challenges Parspack accepts: %w",
			in.Settings.Challenge, domain.ErrInvalidInput)
	}
	if in.Settings.TrustTime < 0 {
		return nil, fmt.Errorf("ip_reputation_trust_time must not be negative: %w", domain.ErrInvalidInput)
	}
	if in.Settings.AttackBanTime < 0 {
		return nil, fmt.Errorf("attack_ban_time must not be negative: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		settings, err := uc.provider.UpdateCDNIPReputation(ctx, in.Credentials, in.ZoneUUID, in.Settings)
		if err != nil {
			return nil, fmt.Errorf("updating CDN IP reputation settings for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(settings)
	})
	if err != nil {
		return nil, err
	}

	var settings domain.CDNIPReputationSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decoding updated CDN IP reputation settings: %w", err)
	}
	return &settings, nil
}

// GetCDNDDoSActionsInput identifies the zone whose DDoS action settings to
// look up.
type GetCDNDDoSActionsInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNDDoSActions is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNDDoSActions struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNDDoSActions builds the use case from its ports.
func NewGetCDNDDoSActions(queue ports.Queue, provider ports.ParspackProvider) *GetCDNDDoSActions {
	return &GetCDNDDoSActions{queue: queue, provider: provider}
}

// Execute returns the current DDoS mitigation action settings for a zone.
func (uc *GetCDNDDoSActions) Execute(ctx context.Context, in GetCDNDDoSActionsInput) (*domain.CDNDDoSActionSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		settings, err := uc.provider.GetCDNDDoSActions(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting CDN DDoS action settings for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(settings)
	})
	if err != nil {
		return nil, err
	}

	var settings domain.CDNDDoSActionSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decoding CDN DDoS action settings: %w", err)
	}
	return &settings, nil
}

// UpdateCDNDDoSActionsInput is the normalized form of an
// update_cdn_ddos_actions tool call. TrustTime and BanTime are optional —
// leave them zero to keep the provider's default (3600 and 900 respectively).
type UpdateCDNDDoSActionsInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Settings    domain.CDNDDoSActionSettings
}

// UpdateCDNDDoSActions is a fast operation: the updated settings are
// returned within the same tool call.
type UpdateCDNDDoSActions struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNDDoSActions builds the use case from its ports.
func NewUpdateCDNDDoSActions(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNDDoSActions {
	return &UpdateCDNDDoSActions{queue: queue, provider: provider}
}

// Execute validates and updates a zone's DDoS mitigation action.
func (uc *UpdateCDNDDoSActions) Execute(ctx context.Context, in UpdateCDNDDoSActionsInput) (*domain.CDNDDoSActionSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidCDNDDoSAction(in.Settings.Action) {
		return nil, fmt.Errorf("action %q is not one of the actions Parspack accepts: %w", in.Settings.Action, domain.ErrInvalidInput)
	}
	if in.Settings.TrustTime < 0 || in.Settings.TrustTime > 86400 {
		return nil, fmt.Errorf("trust_time must be between 0 and 86400 seconds: %w", domain.ErrInvalidInput)
	}
	if in.Settings.BanTime < 0 || in.Settings.BanTime > 86400 {
		return nil, fmt.Errorf("ban_time must be between 0 and 86400 seconds: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		settings, err := uc.provider.UpdateCDNDDoSActions(ctx, in.Credentials, in.ZoneUUID, in.Settings)
		if err != nil {
			return nil, fmt.Errorf("updating CDN DDoS action settings for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(settings)
	})
	if err != nil {
		return nil, err
	}

	var settings domain.CDNDDoSActionSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decoding updated CDN DDoS action settings: %w", err)
	}
	return &settings, nil
}
