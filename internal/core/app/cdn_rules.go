package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// Origin Rule, Page Rule and Transform Rule use cases (issue #24). All of
// them are fast operations: each is a single provider HTTP round trip with
// no long-running provisioning state, so every use case here dispatches
// through ports.Queue and returns synchronously, exactly like the existing
// CDN zone and DNS record use cases (cdn_zone.go, dns_record.go).
//
// ports.ParspackProvider does not yet declare these provider methods (it is
// being integrated centrally as part of issue #24's rollup), so each use
// case below is typed against a small local interface listing only the
// provider method(s) it actually calls. *parspack.Client satisfies every one
// of these automatically via structural typing; once ports.ParspackProvider
// gains the matching methods, these use cases keep working unchanged against
// that wider interface too.

// ---------------------------------------------------------------------------
// Validation helpers.

// validateCDNOriginRule checks a rule's shape against the enums and
// type-specific required fields the CDN API confirms (issue #24), so a bad
// request fails fast here instead of reaching the provider and coming back
// as a 422.
func validateCDNOriginRule(rule domain.CDNOriginRule) error {
	if rule.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if rule.Type != "" && !domain.ValidCDNOriginRuleType(rule.Type) {
		return fmt.Errorf("type %q is not one of the origin rule types Parspack offers: %w", rule.Type, domain.ErrInvalidInput)
	}
	switch rule.Type {
	case "upstream":
		if rule.UpstreamIP == "" {
			return fmt.Errorf("upstream_ip is required when type is \"upstream\": %w", domain.ErrInvalidInput)
		}
	case "port":
		if rule.Port == 0 {
			return fmt.Errorf("port is required when type is \"port\": %w", domain.ErrInvalidInput)
		}
	case "load_balance":
		if rule.LoadBalanceID == "" {
			return fmt.Errorf("load_balance_id is required when type is \"load_balance\": %w", domain.ErrInvalidInput)
		}
	}
	return nil
}

// validateCDNPageRule checks a page rule's shape against the enums the CDN
// API confirms (issue #24).
func validateCDNPageRule(rule domain.CDNPageRule) error {
	if rule.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if rule.Value == "" {
		return fmt.Errorf("value is required: %w", domain.ErrInvalidInput)
	}
	if rule.Type != "" && !domain.ValidCDNPageRuleType(rule.Type) {
		return fmt.Errorf("type %q is not one of the page rule types Parspack offers: %w", rule.Type, domain.ErrInvalidInput)
	}
	if rule.Operator != "" && !domain.ValidCDNPageRuleOperator(rule.Operator) {
		return fmt.Errorf("operator %q is not one of the page rule operators Parspack offers: %w", rule.Operator, domain.ErrInvalidInput)
	}
	if rule.URLRedirection != nil && !domain.ValidCDNPageRuleRedirectCode(rule.URLRedirection.HTTPCode) {
		return fmt.Errorf("url_redirection.http_code %d is not one of the redirect codes Parspack offers: %w", rule.URLRedirection.HTTPCode, domain.ErrInvalidInput)
	}
	return nil
}

// validateCDNTransformRule checks a transform rule's shape against the
// enums the CDN API confirms (issue #24).
func validateCDNTransformRule(rule domain.CDNTransformRule) error {
	if rule.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	for _, headers := range [][]domain.CDNTransformHeaderAction{rule.RequestHeaders, rule.ResponseHeaders} {
		for _, h := range headers {
			if h.HeaderName == "" {
				return fmt.Errorf("header_name is required for every request/response header entry: %w", domain.ErrInvalidInput)
			}
			if !domain.ValidCDNTransformHeaderAction(h.Action) {
				return fmt.Errorf("action %q is not one of \"modify\" or \"delete\": %w", h.Action, domain.ErrInvalidInput)
			}
			if h.Action == "modify" && h.HeaderValue == "" {
				return fmt.Errorf("header_value is required for header %q when action is \"modify\": %w", h.HeaderName, domain.ErrInvalidInput)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Origin Rules.

// ListCDNOriginRulesInput identifies the zone whose origin rules to list.
type ListCDNOriginRulesInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// ListCDNOriginRules is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type ListCDNOriginRules struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListCDNOriginRules builds the use case from its ports.
func NewListCDNOriginRules(queue ports.Queue, provider ports.ParspackProvider) *ListCDNOriginRules {
	return &ListCDNOriginRules{queue: queue, provider: provider}
}

// Execute returns every origin rule of the given zone.
func (uc *ListCDNOriginRules) Execute(ctx context.Context, in ListCDNOriginRulesInput) ([]domain.CDNOriginRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListCDNOriginRules(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing origin rules of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.CDNOriginRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding origin rule list: %w", err)
	}
	return rules, nil
}

// CreateCDNOriginRuleInput is the normalized form of a
// create_cdn_origin_rule tool call.
type CreateCDNOriginRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Rule        domain.CDNOriginRule
}

// CreateCDNOriginRule is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type CreateCDNOriginRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewCreateCDNOriginRule builds the use case from its ports.
func NewCreateCDNOriginRule(queue ports.Queue, provider ports.ParspackProvider) *CreateCDNOriginRule {
	return &CreateCDNOriginRule{queue: queue, provider: provider}
}

// Execute validates the rule and creates it. The provider does not echo the
// created rule's ID (see cdn_rules.go's adapter doc comment), so the caller
// should follow up with ListCDNOriginRules to discover it.
func (uc *CreateCDNOriginRule) Execute(ctx context.Context, in CreateCDNOriginRuleInput) (*domain.CDNOriginRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNOriginRule(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		created, err := uc.provider.CreateCDNOriginRule(ctx, in.Credentials, in.ZoneUUID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating origin rule %q in zone %s: %w", in.Rule.Name, in.ZoneUUID, err)
		}
		return json.Marshal(created)
	})
	if err != nil {
		return nil, err
	}

	var created domain.CDNOriginRule
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, fmt.Errorf("decoding created origin rule: %w", err)
	}
	return &created, nil
}

// GetCDNOriginRuleInput identifies the origin rule to look up.
type GetCDNOriginRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// GetCDNOriginRule is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNOriginRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNOriginRule builds the use case from its ports.
func NewGetCDNOriginRule(queue ports.Queue, provider ports.ParspackProvider) *GetCDNOriginRule {
	return &GetCDNOriginRule{queue: queue, provider: provider}
}

// Execute returns the current state of one origin rule.
func (uc *GetCDNOriginRule) Execute(ctx context.Context, in GetCDNOriginRuleInput) (*domain.CDNOriginRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("origin_rule_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rule, err := uc.provider.GetCDNOriginRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID)
		if err != nil {
			return nil, fmt.Errorf("getting origin rule %s of zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(rule)
	})
	if err != nil {
		return nil, err
	}

	var rule domain.CDNOriginRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, fmt.Errorf("decoding origin rule: %w", err)
	}
	return &rule, nil
}

// UpdateCDNOriginRuleInput is the normalized form of an
// update_cdn_origin_rule tool call.
type UpdateCDNOriginRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
	Rule        domain.CDNOriginRule
}

// UpdateCDNOriginRule is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNOriginRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNOriginRule builds the use case from its ports.
func NewUpdateCDNOriginRule(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNOriginRule {
	return &UpdateCDNOriginRule{queue: queue, provider: provider}
}

// Execute validates the rule and replaces its configuration, returning the
// stored rule synchronously.
func (uc *UpdateCDNOriginRule) Execute(ctx context.Context, in UpdateCDNOriginRuleInput) (*domain.CDNOriginRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("origin_rule_id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNOriginRule(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		updated, err := uc.provider.UpdateCDNOriginRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating origin rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(updated)
	})
	if err != nil {
		return nil, err
	}

	var updated domain.CDNOriginRule
	if err := json.Unmarshal(raw, &updated); err != nil {
		return nil, fmt.Errorf("decoding updated origin rule: %w", err)
	}
	return &updated, nil
}

// DeleteCDNOriginRuleInput identifies the origin rule to remove.
type DeleteCDNOriginRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// DeleteCDNOriginRule is a fast operation. Deleting a rule the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely.
type DeleteCDNOriginRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteCDNOriginRule builds the use case from its ports.
func NewDeleteCDNOriginRule(queue ports.Queue, provider ports.ParspackProvider) *DeleteCDNOriginRule {
	return &DeleteCDNOriginRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteCDNOriginRule) Execute(ctx context.Context, in DeleteCDNOriginRuleInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return fmt.Errorf("origin_rule_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNOriginRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting origin rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ToggleCDNOriginRuleInput identifies the origin rule to enable or disable.
type ToggleCDNOriginRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
	Enabled     bool
}

// ToggleCDNOriginRule is a fast operation: it enables or disables a rule
// without deleting it.
type ToggleCDNOriginRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewToggleCDNOriginRule builds the use case from its ports.
func NewToggleCDNOriginRule(queue ports.Queue, provider ports.ParspackProvider) *ToggleCDNOriginRule {
	return &ToggleCDNOriginRule{queue: queue, provider: provider}
}

// Execute enables or disables the rule.
func (uc *ToggleCDNOriginRule) Execute(ctx context.Context, in ToggleCDNOriginRuleInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return fmt.Errorf("origin_rule_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.ToggleCDNOriginRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID, in.Enabled); err != nil {
			return nil, fmt.Errorf("toggling origin rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ---------------------------------------------------------------------------
// Page Rules. No toggle-status endpoint exists for this rule engine (the
// spec confirms only list/create/get/update/delete), so there is no
// ToggleCDNPageRule use case.

// ListCDNPageRulesInput identifies the zone whose page rules to list.
type ListCDNPageRulesInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// ListCDNPageRules is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type ListCDNPageRules struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListCDNPageRules builds the use case from its ports.
func NewListCDNPageRules(queue ports.Queue, provider ports.ParspackProvider) *ListCDNPageRules {
	return &ListCDNPageRules{queue: queue, provider: provider}
}

// Execute returns every page rule of the given zone.
func (uc *ListCDNPageRules) Execute(ctx context.Context, in ListCDNPageRulesInput) ([]domain.CDNPageRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListCDNPageRules(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing page rules of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.CDNPageRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding page rule list: %w", err)
	}
	return rules, nil
}

// CreateCDNPageRuleInput is the normalized form of a create_cdn_page_rule
// tool call.
type CreateCDNPageRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Rule        domain.CDNPageRule
}

// CreateCDNPageRule is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type CreateCDNPageRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewCreateCDNPageRule builds the use case from its ports.
func NewCreateCDNPageRule(queue ports.Queue, provider ports.ParspackProvider) *CreateCDNPageRule {
	return &CreateCDNPageRule{queue: queue, provider: provider}
}

// Execute validates the rule and creates it. The provider does not echo the
// created rule's ID (see cdn_rules.go's adapter doc comment), so the caller
// should follow up with ListCDNPageRules to discover it.
func (uc *CreateCDNPageRule) Execute(ctx context.Context, in CreateCDNPageRuleInput) (*domain.CDNPageRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNPageRule(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		created, err := uc.provider.CreateCDNPageRule(ctx, in.Credentials, in.ZoneUUID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating page rule %q in zone %s: %w", in.Rule.Name, in.ZoneUUID, err)
		}
		return json.Marshal(created)
	})
	if err != nil {
		return nil, err
	}

	var created domain.CDNPageRule
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, fmt.Errorf("decoding created page rule: %w", err)
	}
	return &created, nil
}

// GetCDNPageRuleInput identifies the page rule to look up.
type GetCDNPageRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// GetCDNPageRule is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNPageRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNPageRule builds the use case from its ports.
func NewGetCDNPageRule(queue ports.Queue, provider ports.ParspackProvider) *GetCDNPageRule {
	return &GetCDNPageRule{queue: queue, provider: provider}
}

// Execute returns the current state of one page rule.
func (uc *GetCDNPageRule) Execute(ctx context.Context, in GetCDNPageRuleInput) (*domain.CDNPageRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("page_rule_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rule, err := uc.provider.GetCDNPageRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID)
		if err != nil {
			return nil, fmt.Errorf("getting page rule %s of zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(rule)
	})
	if err != nil {
		return nil, err
	}

	var rule domain.CDNPageRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, fmt.Errorf("decoding page rule: %w", err)
	}
	return &rule, nil
}

// UpdateCDNPageRuleInput is the normalized form of an update_cdn_page_rule
// tool call.
type UpdateCDNPageRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
	Rule        domain.CDNPageRule
}

// UpdateCDNPageRule is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type UpdateCDNPageRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNPageRule builds the use case from its ports.
func NewUpdateCDNPageRule(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNPageRule {
	return &UpdateCDNPageRule{queue: queue, provider: provider}
}

// Execute validates the rule and replaces its configuration, returning the
// stored rule synchronously.
func (uc *UpdateCDNPageRule) Execute(ctx context.Context, in UpdateCDNPageRuleInput) (*domain.CDNPageRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("page_rule_id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNPageRule(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		updated, err := uc.provider.UpdateCDNPageRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating page rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(updated)
	})
	if err != nil {
		return nil, err
	}

	var updated domain.CDNPageRule
	if err := json.Unmarshal(raw, &updated); err != nil {
		return nil, fmt.Errorf("decoding updated page rule: %w", err)
	}
	return &updated, nil
}

// DeleteCDNPageRuleInput identifies the page rule to remove.
type DeleteCDNPageRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// DeleteCDNPageRule is a fast operation. Deleting a rule the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely.
type DeleteCDNPageRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteCDNPageRule builds the use case from its ports.
func NewDeleteCDNPageRule(queue ports.Queue, provider ports.ParspackProvider) *DeleteCDNPageRule {
	return &DeleteCDNPageRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteCDNPageRule) Execute(ctx context.Context, in DeleteCDNPageRuleInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return fmt.Errorf("page_rule_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNPageRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting page rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ---------------------------------------------------------------------------
// Transform Rules.

// ListCDNTransformRulesInput identifies the zone whose transform rules to
// list.
type ListCDNTransformRulesInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// ListCDNTransformRules is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type ListCDNTransformRules struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListCDNTransformRules builds the use case from its ports.
func NewListCDNTransformRules(queue ports.Queue, provider ports.ParspackProvider) *ListCDNTransformRules {
	return &ListCDNTransformRules{queue: queue, provider: provider}
}

// Execute returns every transform rule of the given zone.
func (uc *ListCDNTransformRules) Execute(ctx context.Context, in ListCDNTransformRulesInput) ([]domain.CDNTransformRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListCDNTransformRules(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing transform rules of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.CDNTransformRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding transform rule list: %w", err)
	}
	return rules, nil
}

// CreateCDNTransformRuleInput is the normalized form of a
// create_cdn_transform_rule tool call.
type CreateCDNTransformRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Rule        domain.CDNTransformRule
}

// CreateCDNTransformRule is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type CreateCDNTransformRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewCreateCDNTransformRule builds the use case from its ports.
func NewCreateCDNTransformRule(queue ports.Queue, provider ports.ParspackProvider) *CreateCDNTransformRule {
	return &CreateCDNTransformRule{queue: queue, provider: provider}
}

// Execute validates the rule and creates it. The provider does not echo the
// created rule's ID (see cdn_rules.go's adapter doc comment), so the caller
// should follow up with ListCDNTransformRules to discover it.
func (uc *CreateCDNTransformRule) Execute(ctx context.Context, in CreateCDNTransformRuleInput) (*domain.CDNTransformRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNTransformRule(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		created, err := uc.provider.CreateCDNTransformRule(ctx, in.Credentials, in.ZoneUUID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating transform rule %q in zone %s: %w", in.Rule.Name, in.ZoneUUID, err)
		}
		return json.Marshal(created)
	})
	if err != nil {
		return nil, err
	}

	var created domain.CDNTransformRule
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, fmt.Errorf("decoding created transform rule: %w", err)
	}
	return &created, nil
}

// GetCDNTransformRuleInput identifies the transform rule to look up.
type GetCDNTransformRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// GetCDNTransformRule is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNTransformRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNTransformRule builds the use case from its ports.
func NewGetCDNTransformRule(queue ports.Queue, provider ports.ParspackProvider) *GetCDNTransformRule {
	return &GetCDNTransformRule{queue: queue, provider: provider}
}

// Execute returns the current state of one transform rule.
func (uc *GetCDNTransformRule) Execute(ctx context.Context, in GetCDNTransformRuleInput) (*domain.CDNTransformRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("transform_rule_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rule, err := uc.provider.GetCDNTransformRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID)
		if err != nil {
			return nil, fmt.Errorf("getting transform rule %s of zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(rule)
	})
	if err != nil {
		return nil, err
	}

	var rule domain.CDNTransformRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, fmt.Errorf("decoding transform rule: %w", err)
	}
	return &rule, nil
}

// UpdateCDNTransformRuleInput is the normalized form of an
// update_cdn_transform_rule tool call.
type UpdateCDNTransformRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
	Rule        domain.CDNTransformRule
}

// UpdateCDNTransformRule is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNTransformRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNTransformRule builds the use case from its ports.
func NewUpdateCDNTransformRule(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNTransformRule {
	return &UpdateCDNTransformRule{queue: queue, provider: provider}
}

// Execute validates the rule and replaces its configuration, returning the
// stored rule synchronously.
func (uc *UpdateCDNTransformRule) Execute(ctx context.Context, in UpdateCDNTransformRuleInput) (*domain.CDNTransformRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("transform_rule_id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNTransformRule(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		updated, err := uc.provider.UpdateCDNTransformRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating transform rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(updated)
	})
	if err != nil {
		return nil, err
	}

	var updated domain.CDNTransformRule
	if err := json.Unmarshal(raw, &updated); err != nil {
		return nil, fmt.Errorf("decoding updated transform rule: %w", err)
	}
	return &updated, nil
}

// DeleteCDNTransformRuleInput identifies the transform rule to remove.
type DeleteCDNTransformRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// DeleteCDNTransformRule is a fast operation. Deleting a rule the provider
// no longer has is treated as already-done rather than an error, so callers
// can call it more than once safely.
type DeleteCDNTransformRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteCDNTransformRule builds the use case from its ports.
func NewDeleteCDNTransformRule(queue ports.Queue, provider ports.ParspackProvider) *DeleteCDNTransformRule {
	return &DeleteCDNTransformRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteCDNTransformRule) Execute(ctx context.Context, in DeleteCDNTransformRuleInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return fmt.Errorf("transform_rule_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNTransformRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting transform rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ToggleCDNTransformRuleInput identifies the transform rule to enable or
// disable.
type ToggleCDNTransformRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
	Enabled     bool
}

// ToggleCDNTransformRule is a fast operation: it enables or disables a rule
// without deleting it.
type ToggleCDNTransformRule struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewToggleCDNTransformRule builds the use case from its ports.
func NewToggleCDNTransformRule(queue ports.Queue, provider ports.ParspackProvider) *ToggleCDNTransformRule {
	return &ToggleCDNTransformRule{queue: queue, provider: provider}
}

// Execute enables or disables the rule.
func (uc *ToggleCDNTransformRule) Execute(ctx context.Context, in ToggleCDNTransformRuleInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return fmt.Errorf("transform_rule_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.ToggleCDNTransformRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID, in.Enabled); err != nil {
			return nil, fmt.Errorf("toggling transform rule %s in zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
