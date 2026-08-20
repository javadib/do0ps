package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the domain-level and account-level Firewall use cases for
// ArvanCloud (issue #65): the CDN edge-level L7 firewall, evaluated
// per-request against each rule's filter_expr. Every one of them is a fast
// operation (ports.ArvanCloudProvider, AGENTS.md 4.3): each dispatches onto
// the queue and blocks for the result within the same tool call.
//
// filter_expr itself is never validated here — it is a free-form,
// provider-defined expression language, so a malformed one is left to
// surface as whatever 422 the provider returns (see
// domain.ArvanCloudFirewallRule.FilterExpr's doc comment). What IS validated
// client-side, per issue #65's acceptance criteria:
//
//   - The two action enums independently: ValidArvanCloudFirewallAction for
//     a per-rule action (rejects "drop"), ValidArvanCloudFirewallDefaultAction
//     for firewall settings' default_action (accepts "drop").
//   - An account-level rule's domain_selection_type of "include"/"exclude"
//     requires a non-empty domain_ids, since an empty list under either mode
//     is either a silent no-op or ambiguous with "all" — see
//     domain.ArvanCloudAccountFirewallRule.DomainSelectionType's doc comment.

// validateArvanCloudFirewallRuleInput checks the fields every
// create/update firewall rule call shares, both domain-level and
// account-level: name, filter_expr and a per-rule action from
// ValidArvanCloudFirewallAction's set (never "drop" — see
// domain.ArvanCloudFirewallDefaultAction's doc comment).
func validateArvanCloudFirewallRuleInput(name, filterExpr string, action domain.ArvanCloudFirewallAction) error {
	if name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if filterExpr == "" {
		return fmt.Errorf("filter_expr is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudFirewallAction(string(action)) {
		return fmt.Errorf("action %q is not one of \"allow\", \"deny\", \"bypass\" or \"challenge\": %w", action, domain.ErrInvalidInput)
	}
	return nil
}

// validateArvanCloudReprioritize checks the shared shape of a reprioritize
// call: ruleID is always required (ReprioritizeRuleRequest's own required
// field), and afterRuleID/beforeRuleID must not both be given — the spec's
// description for both firewall.reprioritize and
// account.firewall_rules.reprioritize states only one of the two should be
// supplied at a time.
func validateArvanCloudReprioritize(ruleID, afterRuleID, beforeRuleID string) error {
	if ruleID == "" {
		return fmt.Errorf("rule_id is required: %w", domain.ErrInvalidInput)
	}
	if afterRuleID != "" && beforeRuleID != "" {
		return fmt.Errorf("only one of after_rule_id or before_rule_id may be given, not both: %w", domain.ErrInvalidInput)
	}
	return nil
}

// --- Firewall Settings (domain-level) -------------------------------------

// arvanCloudFirewallDomainInput is embedded by every use case below that is
// scoped to exactly one domain by name and needs nothing else.
type arvanCloudFirewallDomainInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

func (in arvanCloudFirewallDomainInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudFirewallSettingsInput identifies the domain whose firewall
// settings to look up.
type GetArvanCloudFirewallSettingsInput = arvanCloudFirewallDomainInput

// GetArvanCloudFirewallSettings is a fast operation.
type GetArvanCloudFirewallSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudFirewallSettings builds the use case from its ports.
func NewGetArvanCloudFirewallSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudFirewallSettings {
	return &GetArvanCloudFirewallSettings{queue: queue, provider: provider}
}

// Execute returns the domain's current firewall settings.
func (uc *GetArvanCloudFirewallSettings) Execute(ctx context.Context, in GetArvanCloudFirewallSettingsInput) (*domain.ArvanCloudFirewallSettings, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudFirewallSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudFirewallSettings, error) {
		found, err := uc.provider.GetArvanCloudFirewallSettings(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud firewall settings for domain %q: %w", in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudFirewallSettingsInput identifies the domain and its new
// firewall configuration.
type UpdateArvanCloudFirewallSettingsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Settings    domain.ArvanCloudFirewallSettings
}

// UpdateArvanCloudFirewallSettings changes a domain's firewall
// configuration. This is a fast operation.
type UpdateArvanCloudFirewallSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudFirewallSettings builds the use case from its ports.
func NewUpdateArvanCloudFirewallSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudFirewallSettings {
	return &UpdateArvanCloudFirewallSettings{queue: queue, provider: provider}
}

// Execute validates the request and updates the settings, returning them as
// stored afterward. DefaultAction is validated against
// ValidArvanCloudFirewallDefaultAction — independently of
// ValidArvanCloudFirewallAction used by the rule use cases below, so "drop"
// is accepted here even though it is rejected for a per-rule action.
func (uc *UpdateArvanCloudFirewallSettings) Execute(ctx context.Context, in UpdateArvanCloudFirewallSettingsInput) (*domain.ArvanCloudFirewallSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudFirewallDefaultAction(string(in.Settings.DefaultAction)) {
		return nil, fmt.Errorf(
			"default_action %q is not one of \"allow\", \"deny\", \"drop\", \"bypass\" or \"challenge\": %w",
			in.Settings.DefaultAction, domain.ErrInvalidInput)
	}

	return dispatchArvanCloudFirewallSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudFirewallSettings, error) {
		updated, err := uc.provider.UpdateArvanCloudFirewallSettings(ctx, in.Credentials, in.Domain, in.Settings)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud firewall settings for domain %q: %w", in.Domain, err)
		}
		return updated, nil
	})
}

// --- Firewall Rules (domain-level) ----------------------------------------

// ListArvanCloudFirewallRulesInput identifies the domain whose rules to
// list.
type ListArvanCloudFirewallRulesInput = arvanCloudFirewallDomainInput

// ListArvanCloudFirewallRules is a fast operation.
type ListArvanCloudFirewallRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudFirewallRules builds the use case from its ports.
func NewListArvanCloudFirewallRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudFirewallRules {
	return &ListArvanCloudFirewallRules{queue: queue, provider: provider}
}

// Execute returns every domain-level rule configured for the domain.
func (uc *ListArvanCloudFirewallRules) Execute(ctx context.Context, in ListArvanCloudFirewallRulesInput) ([]domain.ArvanCloudFirewallRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListArvanCloudFirewallRules(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud firewall rules of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.ArvanCloudFirewallRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding arvancloud firewall rule list: %w", err)
	}
	return rules, nil
}

// CreateArvanCloudFirewallRuleInput is the normalized form of a
// create_arvancloud_firewall_rule tool call.
type CreateArvanCloudFirewallRuleInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Rule        domain.ArvanCloudFirewallRule
}

// CreateArvanCloudFirewallRule creates a new domain-level rule.
type CreateArvanCloudFirewallRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudFirewallRule builds the use case from its ports.
func NewCreateArvanCloudFirewallRule(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudFirewallRule {
	return &CreateArvanCloudFirewallRule{queue: queue, provider: provider}
}

// Execute validates the request and creates the rule, returning it as
// stored.
func (uc *CreateArvanCloudFirewallRule) Execute(ctx context.Context, in CreateArvanCloudFirewallRuleInput) (*domain.ArvanCloudFirewallRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudFirewallRuleInput(in.Rule.Name, in.Rule.FilterExpr, in.Rule.Action); err != nil {
		return nil, err
	}

	return dispatchArvanCloudFirewallRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudFirewallRule, error) {
		created, err := uc.provider.CreateArvanCloudFirewallRule(ctx, in.Credentials, in.Domain, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud firewall rule %q on domain %q: %w", in.Rule.Name, in.Domain, err)
		}
		return created, nil
	})
}

// arvanCloudFirewallRuleIDInput is embedded by every use case below that is
// scoped to exactly one domain-level rule by domain + id.
type arvanCloudFirewallRuleIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
}

func (in arvanCloudFirewallRuleIDInput) validate() error {
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

// GetArvanCloudFirewallRuleInput identifies the domain-level rule to look
// up.
type GetArvanCloudFirewallRuleInput = arvanCloudFirewallRuleIDInput

// GetArvanCloudFirewallRule is a fast operation.
type GetArvanCloudFirewallRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudFirewallRule builds the use case from its ports.
func NewGetArvanCloudFirewallRule(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudFirewallRule {
	return &GetArvanCloudFirewallRule{queue: queue, provider: provider}
}

// Execute returns the current state of one domain-level rule.
func (uc *GetArvanCloudFirewallRule) Execute(ctx context.Context, in GetArvanCloudFirewallRuleInput) (*domain.ArvanCloudFirewallRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudFirewallRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudFirewallRule, error) {
		found, err := uc.provider.GetArvanCloudFirewallRule(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud firewall rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudFirewallRuleInput identifies the domain-level rule to
// update and its new field values.
type UpdateArvanCloudFirewallRuleInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Rule        domain.ArvanCloudFirewallRule
}

// UpdateArvanCloudFirewallRule changes a domain-level rule. This is a fast
// operation.
type UpdateArvanCloudFirewallRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudFirewallRule builds the use case from its ports.
func NewUpdateArvanCloudFirewallRule(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudFirewallRule {
	return &UpdateArvanCloudFirewallRule{queue: queue, provider: provider}
}

// Execute updates the rule and returns it as stored afterward.
func (uc *UpdateArvanCloudFirewallRule) Execute(ctx context.Context, in UpdateArvanCloudFirewallRuleInput) (*domain.ArvanCloudFirewallRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudFirewallRuleInput(in.Rule.Name, in.Rule.FilterExpr, in.Rule.Action); err != nil {
		return nil, err
	}

	return dispatchArvanCloudFirewallRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudFirewallRule, error) {
		updated, err := uc.provider.UpdateArvanCloudFirewallRule(ctx, in.Credentials, in.Domain, in.ID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud firewall rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudFirewallRuleInput identifies the domain-level rule to
// remove.
type DeleteArvanCloudFirewallRuleInput = arvanCloudFirewallRuleIDInput

// DeleteArvanCloudFirewallRule is a fast operation. Deleting a rule the
// provider no longer has is treated as already done rather than an error,
// matching DeleteArvanCloudDomain's tolerant-delete contract.
type DeleteArvanCloudFirewallRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudFirewallRule builds the use case from its ports.
func NewDeleteArvanCloudFirewallRule(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudFirewallRule {
	return &DeleteArvanCloudFirewallRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteArvanCloudFirewallRule) Execute(ctx context.Context, in DeleteArvanCloudFirewallRuleInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudFirewallRule(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud firewall rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ReprioritizeArvanCloudFirewallRulesInput identifies the domain, the rule
// to move, and exactly one of AfterRuleID/BeforeRuleID to move it relative
// to.
type ReprioritizeArvanCloudFirewallRulesInput struct {
	Credentials  domain.ProviderCredentials
	Domain       string
	RuleID       string
	AfterRuleID  string
	BeforeRuleID string
}

// ReprioritizeArvanCloudFirewallRules is a fast operation.
type ReprioritizeArvanCloudFirewallRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewReprioritizeArvanCloudFirewallRules builds the use case from its ports.
func NewReprioritizeArvanCloudFirewallRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ReprioritizeArvanCloudFirewallRules {
	return &ReprioritizeArvanCloudFirewallRules{queue: queue, provider: provider}
}

// Execute moves RuleID to its new position.
func (uc *ReprioritizeArvanCloudFirewallRules) Execute(ctx context.Context, in ReprioritizeArvanCloudFirewallRulesInput) error {
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
		if err := uc.provider.ReprioritizeArvanCloudFirewallRules(ctx, in.Credentials, in.Domain, in.RuleID, in.AfterRuleID, in.BeforeRuleID); err != nil {
			return nil, fmt.Errorf("reprioritizing arvancloud firewall rule %q on domain %q: %w", in.RuleID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Firewall Rules (account-level) ---------------------------------------

// ListArvanCloudAccountFirewallValidDomainsInput carries the credentials
// needed to list the account's domains eligible for account-level firewall
// rules. There is nothing else to specify, matching
// ListArvanCloudDynamicFieldsInput's own choice to keep listing unscoped.
type ListArvanCloudAccountFirewallValidDomainsInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudAccountFirewallValidDomains is a fast operation.
type ListArvanCloudAccountFirewallValidDomains struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudAccountFirewallValidDomains builds the use case from its
// ports.
func NewListArvanCloudAccountFirewallValidDomains(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudAccountFirewallValidDomains {
	return &ListArvanCloudAccountFirewallValidDomains{queue: queue, provider: provider}
}

// Execute returns the account's domains eligible to be targeted by an
// account-level rule's domain_ids.
func (uc *ListArvanCloudAccountFirewallValidDomains) Execute(ctx context.Context, in ListArvanCloudAccountFirewallValidDomainsInput) ([]domain.ArvanCloudAccountFirewallValidDomain, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		domains, err := uc.provider.ListArvanCloudAccountFirewallValidDomains(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud account firewall valid domains: %w", err)
		}
		return json.Marshal(domains)
	})
	if err != nil {
		return nil, err
	}

	var domains []domain.ArvanCloudAccountFirewallValidDomain
	if err := json.Unmarshal(raw, &domains); err != nil {
		return nil, fmt.Errorf("decoding arvancloud account firewall valid domain list: %w", err)
	}
	return domains, nil
}

// ListArvanCloudAccountFirewallRulesInput carries the credentials needed to
// list the account's rules. Unscoped, like the valid-domains listing above.
type ListArvanCloudAccountFirewallRulesInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudAccountFirewallRules is a fast operation.
type ListArvanCloudAccountFirewallRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudAccountFirewallRules builds the use case from its ports.
func NewListArvanCloudAccountFirewallRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudAccountFirewallRules {
	return &ListArvanCloudAccountFirewallRules{queue: queue, provider: provider}
}

// Execute returns every account-level rule.
func (uc *ListArvanCloudAccountFirewallRules) Execute(ctx context.Context, in ListArvanCloudAccountFirewallRulesInput) ([]domain.ArvanCloudAccountFirewallRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListArvanCloudAccountFirewallRules(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud account firewall rules: %w", err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.ArvanCloudAccountFirewallRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding arvancloud account firewall rule list: %w", err)
	}
	return rules, nil
}

// validateArvanCloudAccountFirewallDomainSelection checks
// domain_selection_type against ValidArvanCloudDomainSelectionType, and,
// when it is "include" or "exclude", that domain_ids is non-empty — issue
// #65's acceptance criteria: an empty domain_ids under either mode is
// rejected here, before this ever reaches the provider.
func validateArvanCloudAccountFirewallDomainSelection(selectionType domain.ArvanCloudDomainSelectionType, domainIDs []string) error {
	if !domain.ValidArvanCloudDomainSelectionType(string(selectionType)) {
		return fmt.Errorf("domain_selection_type %q is not one of \"all\", \"include\" or \"exclude\": %w", selectionType, domain.ErrInvalidInput)
	}
	if (selectionType == domain.ArvanCloudDomainSelectionInclude || selectionType == domain.ArvanCloudDomainSelectionExclude) && len(domainIDs) == 0 {
		return fmt.Errorf("domain_ids must be non-empty when domain_selection_type is %q: %w", selectionType, domain.ErrInvalidInput)
	}
	return nil
}

// CreateArvanCloudAccountFirewallRuleInput is the normalized form of a
// create_arvancloud_account_firewall_rule tool call.
type CreateArvanCloudAccountFirewallRuleInput struct {
	Credentials domain.ProviderCredentials
	Rule        domain.ArvanCloudAccountFirewallRule
}

// CreateArvanCloudAccountFirewallRule creates a new account-level rule.
type CreateArvanCloudAccountFirewallRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudAccountFirewallRule builds the use case from its ports.
func NewCreateArvanCloudAccountFirewallRule(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudAccountFirewallRule {
	return &CreateArvanCloudAccountFirewallRule{queue: queue, provider: provider}
}

// Execute validates the request — including the domain_selection_type /
// domain_ids check documented on validateArvanCloudAccountFirewallDomainSelection
// — and creates the rule, returning it as stored.
func (uc *CreateArvanCloudAccountFirewallRule) Execute(ctx context.Context, in CreateArvanCloudAccountFirewallRuleInput) (*domain.ArvanCloudAccountFirewallRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudFirewallRuleInput(in.Rule.Name, in.Rule.FilterExpr, in.Rule.Action); err != nil {
		return nil, err
	}
	if err := validateArvanCloudAccountFirewallDomainSelection(in.Rule.DomainSelectionType, in.Rule.DomainIDs); err != nil {
		return nil, err
	}

	return dispatchArvanCloudAccountFirewallRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudAccountFirewallRule, error) {
		created, err := uc.provider.CreateArvanCloudAccountFirewallRule(ctx, in.Credentials, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud account firewall rule %q: %w", in.Rule.Name, err)
		}
		return created, nil
	})
}

// arvanCloudAccountFirewallRuleIDInput is embedded by every use case below
// that is scoped to exactly one account-level rule by id and needs nothing
// else.
type arvanCloudAccountFirewallRuleIDInput struct {
	Credentials domain.ProviderCredentials
	ID          string
}

func (in arvanCloudAccountFirewallRuleIDInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudAccountFirewallRuleInput identifies the account-level rule to
// look up.
type GetArvanCloudAccountFirewallRuleInput = arvanCloudAccountFirewallRuleIDInput

// GetArvanCloudAccountFirewallRule is a fast operation.
type GetArvanCloudAccountFirewallRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudAccountFirewallRule builds the use case from its ports.
func NewGetArvanCloudAccountFirewallRule(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudAccountFirewallRule {
	return &GetArvanCloudAccountFirewallRule{queue: queue, provider: provider}
}

// Execute returns the current state of one account-level rule.
func (uc *GetArvanCloudAccountFirewallRule) Execute(ctx context.Context, in GetArvanCloudAccountFirewallRuleInput) (*domain.ArvanCloudAccountFirewallRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudAccountFirewallRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudAccountFirewallRule, error) {
		found, err := uc.provider.GetArvanCloudAccountFirewallRule(ctx, in.Credentials, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud account firewall rule %q: %w", in.ID, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudAccountFirewallRuleInput identifies the account-level rule
// to update and its new field values.
type UpdateArvanCloudAccountFirewallRuleInput struct {
	Credentials domain.ProviderCredentials
	ID          string
	Rule        domain.ArvanCloudAccountFirewallRule
}

// UpdateArvanCloudAccountFirewallRule changes an account-level rule. This is
// a fast operation.
type UpdateArvanCloudAccountFirewallRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudAccountFirewallRule builds the use case from its ports.
func NewUpdateArvanCloudAccountFirewallRule(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudAccountFirewallRule {
	return &UpdateArvanCloudAccountFirewallRule{queue: queue, provider: provider}
}

// Execute updates the rule — with the same validation as
// CreateArvanCloudAccountFirewallRule — and returns it as stored afterward.
func (uc *UpdateArvanCloudAccountFirewallRule) Execute(ctx context.Context, in UpdateArvanCloudAccountFirewallRuleInput) (*domain.ArvanCloudAccountFirewallRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudFirewallRuleInput(in.Rule.Name, in.Rule.FilterExpr, in.Rule.Action); err != nil {
		return nil, err
	}
	if err := validateArvanCloudAccountFirewallDomainSelection(in.Rule.DomainSelectionType, in.Rule.DomainIDs); err != nil {
		return nil, err
	}

	return dispatchArvanCloudAccountFirewallRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudAccountFirewallRule, error) {
		updated, err := uc.provider.UpdateArvanCloudAccountFirewallRule(ctx, in.Credentials, in.ID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud account firewall rule %q: %w", in.ID, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudAccountFirewallRuleInput identifies the account-level rule
// to remove.
type DeleteArvanCloudAccountFirewallRuleInput = arvanCloudAccountFirewallRuleIDInput

// DeleteArvanCloudAccountFirewallRule is a fast operation. Deleting a rule
// the provider no longer has is treated as already done rather than an
// error, matching DeleteArvanCloudDomain's tolerant-delete contract.
type DeleteArvanCloudAccountFirewallRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudAccountFirewallRule builds the use case from its ports.
func NewDeleteArvanCloudAccountFirewallRule(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudAccountFirewallRule {
	return &DeleteArvanCloudAccountFirewallRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteArvanCloudAccountFirewallRule) Execute(ctx context.Context, in DeleteArvanCloudAccountFirewallRuleInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudAccountFirewallRule(ctx, in.Credentials, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud account firewall rule %q: %w", in.ID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// arvanCloudAccountFirewallDomainsInput is embedded by
// Attach/DetachArvanCloudAccountFirewallDomains: a rule id plus a non-empty
// set of domain ids to add to or remove from its target set.
type arvanCloudAccountFirewallDomainsInput struct {
	Credentials domain.ProviderCredentials
	ID          string
	DomainIDs   []string
}

func (in arvanCloudAccountFirewallDomainsInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if len(in.DomainIDs) == 0 {
		return fmt.Errorf("domain_ids must contain at least one id: %w", domain.ErrInvalidInput)
	}
	return nil
}

// AttachArvanCloudAccountFirewallDomainsInput identifies the rule and the
// domains to add to its target set.
type AttachArvanCloudAccountFirewallDomainsInput = arvanCloudAccountFirewallDomainsInput

// AttachArvanCloudAccountFirewallDomains is a fast operation.
type AttachArvanCloudAccountFirewallDomains struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewAttachArvanCloudAccountFirewallDomains builds the use case from its
// ports.
func NewAttachArvanCloudAccountFirewallDomains(queue ports.Queue, provider ports.ArvanCloudProvider) *AttachArvanCloudAccountFirewallDomains {
	return &AttachArvanCloudAccountFirewallDomains{queue: queue, provider: provider}
}

// Execute attaches the domains and returns the rule as stored afterward.
func (uc *AttachArvanCloudAccountFirewallDomains) Execute(ctx context.Context, in AttachArvanCloudAccountFirewallDomainsInput) (*domain.ArvanCloudAccountFirewallRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudAccountFirewallRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudAccountFirewallRule, error) {
		updated, err := uc.provider.AttachArvanCloudAccountFirewallDomains(ctx, in.Credentials, in.ID, in.DomainIDs)
		if err != nil {
			return nil, fmt.Errorf("attaching domains to arvancloud account firewall rule %q: %w", in.ID, err)
		}
		return updated, nil
	})
}

// DetachArvanCloudAccountFirewallDomainsInput identifies the rule and the
// domains to remove from its target set.
type DetachArvanCloudAccountFirewallDomainsInput = arvanCloudAccountFirewallDomainsInput

// DetachArvanCloudAccountFirewallDomains is a fast operation.
type DetachArvanCloudAccountFirewallDomains struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDetachArvanCloudAccountFirewallDomains builds the use case from its
// ports.
func NewDetachArvanCloudAccountFirewallDomains(queue ports.Queue, provider ports.ArvanCloudProvider) *DetachArvanCloudAccountFirewallDomains {
	return &DetachArvanCloudAccountFirewallDomains{queue: queue, provider: provider}
}

// Execute detaches the domains and returns the rule as stored afterward.
func (uc *DetachArvanCloudAccountFirewallDomains) Execute(ctx context.Context, in DetachArvanCloudAccountFirewallDomainsInput) (*domain.ArvanCloudAccountFirewallRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudAccountFirewallRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudAccountFirewallRule, error) {
		updated, err := uc.provider.DetachArvanCloudAccountFirewallDomains(ctx, in.Credentials, in.ID, in.DomainIDs)
		if err != nil {
			return nil, fmt.Errorf("detaching domains from arvancloud account firewall rule %q: %w", in.ID, err)
		}
		return updated, nil
	})
}

// ReprioritizeArvanCloudAccountFirewallRulesInput identifies the rule to
// move and exactly one of AfterRuleID/BeforeRuleID to move it relative to.
type ReprioritizeArvanCloudAccountFirewallRulesInput struct {
	Credentials  domain.ProviderCredentials
	RuleID       string
	AfterRuleID  string
	BeforeRuleID string
}

// ReprioritizeArvanCloudAccountFirewallRules is a fast operation.
type ReprioritizeArvanCloudAccountFirewallRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewReprioritizeArvanCloudAccountFirewallRules builds the use case from its
// ports.
func NewReprioritizeArvanCloudAccountFirewallRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ReprioritizeArvanCloudAccountFirewallRules {
	return &ReprioritizeArvanCloudAccountFirewallRules{queue: queue, provider: provider}
}

// Execute moves RuleID to its new position.
func (uc *ReprioritizeArvanCloudAccountFirewallRules) Execute(ctx context.Context, in ReprioritizeArvanCloudAccountFirewallRulesInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if err := validateArvanCloudReprioritize(in.RuleID, in.AfterRuleID, in.BeforeRuleID); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.ReprioritizeArvanCloudAccountFirewallRules(ctx, in.Credentials, in.RuleID, in.AfterRuleID, in.BeforeRuleID); err != nil {
			return nil, fmt.Errorf("reprioritizing arvancloud account firewall rule %q: %w", in.RuleID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Dispatch helpers -------------------------------------------------

// dispatchArvanCloudFirewallSettings runs fn on the queue and decodes its
// result back into a *domain.ArvanCloudFirewallSettings.
func dispatchArvanCloudFirewallSettings(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudFirewallSettings, error),
) (*domain.ArvanCloudFirewallSettings, error) {
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

	var result domain.ArvanCloudFirewallSettings
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud firewall settings: %w", err)
	}
	return &result, nil
}

// dispatchArvanCloudFirewallRule runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudFirewallRule, the shape every domain-level
// rule use case above but list/delete/reprioritize returns.
func dispatchArvanCloudFirewallRule(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudFirewallRule, error),
) (*domain.ArvanCloudFirewallRule, error) {
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

	var result domain.ArvanCloudFirewallRule
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud firewall rule: %w", err)
	}
	return &result, nil
}

// dispatchArvanCloudAccountFirewallRule runs fn on the queue and decodes its
// result back into a *domain.ArvanCloudAccountFirewallRule, the shape every
// account-level rule use case above but list/delete/reprioritize returns.
func dispatchArvanCloudAccountFirewallRule(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudAccountFirewallRule, error),
) (*domain.ArvanCloudAccountFirewallRule, error) {
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

	var result domain.ArvanCloudAccountFirewallRule
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud account firewall rule: %w", err)
	}
	return &result, nil
}
