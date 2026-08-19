package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// The use cases in this file cover Parspack's ModSec (WAF custom rules) CDN
// capability, scoped to a zone same as DNS records (AGENTS.md 4.1) — issue
// #24. Every operation here is FAST: it is synchronous rule/settings CRUD
// against the CDN API with no async job pattern evident in the spec
// (docs/api-specs/parspack-cdn.openapi.yaml lines 6677-8223), so each use
// case calls ports.Queue.Dispatch and blocks on the result, same as the CDN
// zone/DNS record use cases in cdn_zone.go.
//
// ports.ParspackProvider is integrated centrally later (house style for
// issue #24's slices): each use case below is typed against a small local
// interface declared next to it, containing only the provider method(s) that
// use case calls, with the signature shaped exactly like the eventual
// ports.ParspackProvider method. *parspack.Client already satisfies every one
// of these structurally, with no adapter changes required once the
// signatures are copied onto ports.ParspackProvider.

// --- ModSec status (zone-level rule-set selection) -------------------------

// GetCDNModSecStatusInput identifies the zone whose ModSec status to look up.
type GetCDNModSecStatusInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// getCDNModSecStatusProvider is the one provider method GetCDNModSecStatus needs.
type getCDNModSecStatusProvider interface {
	GetCDNModSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNModSecStatus, error)
}

// GetCDNModSecStatus is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNModSecStatus struct {
	queue    ports.Queue
	provider getCDNModSecStatusProvider
}

// NewGetCDNModSecStatus builds the use case from its ports.
func NewGetCDNModSecStatus(queue ports.Queue, provider getCDNModSecStatusProvider) *GetCDNModSecStatus {
	return &GetCDNModSecStatus{queue: queue, provider: provider}
}

// Execute returns the zone's current ModSec rule-set selection.
func (uc *GetCDNModSecStatus) Execute(ctx context.Context, in GetCDNModSecStatusInput) (*domain.CDNModSecStatus, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		status, err := uc.provider.GetCDNModSecStatus(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting modsec status for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(status)
	})
	if err != nil {
		return nil, err
	}

	var status domain.CDNModSecStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("decoding modsec status: %w", err)
	}
	return &status, nil
}

// UpdateCDNModSecStatusInput identifies the zone and the full set of ModSec
// rule-set ids (standard and/or custom) that should be selected.
type UpdateCDNModSecStatusInput struct {
	Credentials     domain.ProviderCredentials
	ZoneUUID        string
	SelectedRuleIDs []string
}

// updateCDNModSecStatusProvider is the one provider method
// UpdateCDNModSecStatus needs.
type updateCDNModSecStatusProvider interface {
	UpdateCDNModSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, selectedRuleIDs []string) (*domain.CDNModSecStatus, error)
}

// UpdateCDNModSecStatus is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNModSecStatus struct {
	queue    ports.Queue
	provider updateCDNModSecStatusProvider
}

// NewUpdateCDNModSecStatus builds the use case from its ports.
func NewUpdateCDNModSecStatus(queue ports.Queue, provider updateCDNModSecStatusProvider) *UpdateCDNModSecStatus {
	return &UpdateCDNModSecStatus{queue: queue, provider: provider}
}

// Execute replaces the zone's selected ModSec rule sets and returns the
// resulting status. Passing an empty SelectedRuleIDs clears every selection.
func (uc *UpdateCDNModSecStatus) Execute(ctx context.Context, in UpdateCDNModSecStatusInput) (*domain.CDNModSecStatus, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		status, err := uc.provider.UpdateCDNModSecStatus(ctx, in.Credentials, in.ZoneUUID, in.SelectedRuleIDs)
		if err != nil {
			return nil, fmt.Errorf("updating modsec status for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(status)
	})
	if err != nil {
		return nil, err
	}

	var status domain.CDNModSecStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("decoding modsec status: %w", err)
	}
	return &status, nil
}

// --- ModSec data (reusable values custom rules reference) ------------------

// ListCDNModSecDataInput identifies the zone whose ModSec data to list.
type ListCDNModSecDataInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// listCDNModSecDataProvider is the one provider method ListCDNModSecData needs.
type listCDNModSecDataProvider interface {
	ListCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNModSecData, error)
}

// ListCDNModSecData is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type ListCDNModSecData struct {
	queue    ports.Queue
	provider listCDNModSecDataProvider
}

// NewListCDNModSecData builds the use case from its ports.
func NewListCDNModSecData(queue ports.Queue, provider listCDNModSecDataProvider) *ListCDNModSecData {
	return &ListCDNModSecData{queue: queue, provider: provider}
}

// Execute returns every ModSec data value defined on the zone (id and name
// only — call GetCDNModSecData for a specific entry's value).
func (uc *ListCDNModSecData) Execute(ctx context.Context, in ListCDNModSecDataInput) ([]domain.CDNModSecData, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		data, err := uc.provider.ListCDNModSecData(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing modsec data for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(data)
	})
	if err != nil {
		return nil, err
	}

	var data []domain.CDNModSecData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decoding modsec data list: %w", err)
	}
	return data, nil
}

// CreateCDNModSecDataInput is the normalized form of a
// create_cdn_modsec_data tool call.
type CreateCDNModSecDataInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Data        domain.CDNModSecData
}

// createCDNModSecDataProvider is the one provider method CreateCDNModSecData needs.
type createCDNModSecDataProvider interface {
	CreateCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, data domain.CDNModSecData) (*domain.CDNModSecData, error)
}

// CreateCDNModSecData is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type CreateCDNModSecData struct {
	queue    ports.Queue
	provider createCDNModSecDataProvider
}

// NewCreateCDNModSecData builds the use case from its ports.
func NewCreateCDNModSecData(queue ports.Queue, provider createCDNModSecDataProvider) *CreateCDNModSecData {
	return &CreateCDNModSecData{queue: queue, provider: provider}
}

// Execute validates the request and creates the ModSec data value. The
// provider's create response carries no id (ports.ParspackProvider's
// CreateCDNModSecData contract) — call ListCDNModSecData afterward to learn
// the assigned id.
func (uc *CreateCDNModSecData) Execute(ctx context.Context, in CreateCDNModSecDataInput) (*domain.CDNModSecData, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNModSecData(in.Data); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		data, err := uc.provider.CreateCDNModSecData(ctx, in.Credentials, in.ZoneUUID, in.Data)
		if err != nil {
			return nil, fmt.Errorf("creating modsec data %q for zone %s: %w", in.Data.Name, in.ZoneUUID, err)
		}
		return json.Marshal(data)
	})
	if err != nil {
		return nil, err
	}

	var data domain.CDNModSecData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decoding created modsec data: %w", err)
	}
	return &data, nil
}

func validateCDNModSecData(data domain.CDNModSecData) error {
	if data.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if data.Value == "" {
		return fmt.Errorf("value is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetCDNModSecDataInput identifies the ModSec data value to look up.
type GetCDNModSecDataInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	ID          string
}

// getCDNModSecDataProvider is the one provider method GetCDNModSecData needs.
type getCDNModSecDataProvider interface {
	GetCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNModSecData, error)
}

// GetCDNModSecData is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNModSecData struct {
	queue    ports.Queue
	provider getCDNModSecDataProvider
}

// NewGetCDNModSecData builds the use case from its ports.
func NewGetCDNModSecData(queue ports.Queue, provider getCDNModSecDataProvider) *GetCDNModSecData {
	return &GetCDNModSecData{queue: queue, provider: provider}
}

// Execute returns one ModSec data value by id, decoded value included.
func (uc *GetCDNModSecData) Execute(ctx context.Context, in GetCDNModSecDataInput) (*domain.CDNModSecData, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		data, err := uc.provider.GetCDNModSecData(ctx, in.Credentials, in.ZoneUUID, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting modsec data %s for zone %s: %w", in.ID, in.ZoneUUID, err)
		}
		return json.Marshal(data)
	})
	if err != nil {
		return nil, err
	}

	var data domain.CDNModSecData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decoding modsec data: %w", err)
	}
	return &data, nil
}

// UpdateCDNModSecDataInput is the normalized form of an
// update_cdn_modsec_data tool call.
type UpdateCDNModSecDataInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	ID          string
	Data        domain.CDNModSecData
}

// updateCDNModSecDataProvider is the one provider method UpdateCDNModSecData needs.
type updateCDNModSecDataProvider interface {
	UpdateCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string, data domain.CDNModSecData) (*domain.CDNModSecData, error)
}

// UpdateCDNModSecData is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNModSecData struct {
	queue    ports.Queue
	provider updateCDNModSecDataProvider
}

// NewUpdateCDNModSecData builds the use case from its ports.
func NewUpdateCDNModSecData(queue ports.Queue, provider updateCDNModSecDataProvider) *UpdateCDNModSecData {
	return &UpdateCDNModSecData{queue: queue, provider: provider}
}

// Execute validates the request and replaces an existing ModSec data value's
// name and value by id.
func (uc *UpdateCDNModSecData) Execute(ctx context.Context, in UpdateCDNModSecDataInput) (*domain.CDNModSecData, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNModSecData(in.Data); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		data, err := uc.provider.UpdateCDNModSecData(ctx, in.Credentials, in.ZoneUUID, in.ID, in.Data)
		if err != nil {
			return nil, fmt.Errorf("updating modsec data %s for zone %s: %w", in.ID, in.ZoneUUID, err)
		}
		return json.Marshal(data)
	})
	if err != nil {
		return nil, err
	}

	var data domain.CDNModSecData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decoding updated modsec data: %w", err)
	}
	return &data, nil
}

// DeleteCDNModSecDataInput identifies the ModSec data value to remove.
type DeleteCDNModSecDataInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	ID          string
}

// deleteCDNModSecDataProvider is the one provider method DeleteCDNModSecData needs.
type deleteCDNModSecDataProvider interface {
	DeleteCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) error
}

// DeleteCDNModSecData is a fast operation. Deleting a data value the
// provider no longer has is treated as already-done rather than an error, so
// callers can call it more than once safely.
type DeleteCDNModSecData struct {
	queue    ports.Queue
	provider deleteCDNModSecDataProvider
}

// NewDeleteCDNModSecData builds the use case from its ports.
func NewDeleteCDNModSecData(queue ports.Queue, provider deleteCDNModSecDataProvider) *DeleteCDNModSecData {
	return &DeleteCDNModSecData{queue: queue, provider: provider}
}

// Execute deletes the ModSec data value, tolerating one that is already gone.
func (uc *DeleteCDNModSecData) Execute(ctx context.Context, in DeleteCDNModSecDataInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNModSecData(ctx, in.Credentials, in.ZoneUUID, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting modsec data %s for zone %s: %w", in.ID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- ModSec custom rules -----------------------------------------------

// ListCDNModSecRulesInput identifies the zone whose custom ModSec rules to
// list.
type ListCDNModSecRulesInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// listCDNModSecRulesProvider is the one provider method ListCDNModSecRules needs.
type listCDNModSecRulesProvider interface {
	ListCDNModSecRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNModSecRule, error)
}

// ListCDNModSecRules is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type ListCDNModSecRules struct {
	queue    ports.Queue
	provider listCDNModSecRulesProvider
}

// NewListCDNModSecRules builds the use case from its ports.
func NewListCDNModSecRules(queue ports.Queue, provider listCDNModSecRulesProvider) *ListCDNModSecRules {
	return &ListCDNModSecRules{queue: queue, provider: provider}
}

// Execute returns every custom ModSec rule defined on the zone (id, name and
// status only — call GetCDNModSecRule for a specific rule's value and data).
func (uc *ListCDNModSecRules) Execute(ctx context.Context, in ListCDNModSecRulesInput) ([]domain.CDNModSecRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, err := uc.provider.ListCDNModSecRules(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing modsec rules for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(rules)
	})
	if err != nil {
		return nil, err
	}

	var rules []domain.CDNModSecRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("decoding modsec rule list: %w", err)
	}
	return rules, nil
}

// CreateCDNModSecRuleInput is the normalized form of a
// create_cdn_modsec_rule tool call.
type CreateCDNModSecRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Rule        domain.CDNModSecRule
}

// createCDNModSecRuleProvider is the one provider method CreateCDNModSecRule needs.
type createCDNModSecRuleProvider interface {
	CreateCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNModSecRule) (*domain.CDNModSecRule, error)
}

// CreateCDNModSecRule is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type CreateCDNModSecRule struct {
	queue    ports.Queue
	provider createCDNModSecRuleProvider
}

// NewCreateCDNModSecRule builds the use case from its ports.
func NewCreateCDNModSecRule(queue ports.Queue, provider createCDNModSecRuleProvider) *CreateCDNModSecRule {
	return &CreateCDNModSecRule{queue: queue, provider: provider}
}

// Execute validates the request and creates the custom ModSec rule. The
// provider's create response carries no id (ports.ParspackProvider's
// CreateCDNModSecRule contract) — call ListCDNModSecRules afterward to learn
// the assigned id.
func (uc *CreateCDNModSecRule) Execute(ctx context.Context, in CreateCDNModSecRuleInput) (*domain.CDNModSecRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNModSecRuleCreate(in.Rule); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rule, err := uc.provider.CreateCDNModSecRule(ctx, in.Credentials, in.ZoneUUID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating modsec rule %q for zone %s: %w", in.Rule.Name, in.ZoneUUID, err)
		}
		return json.Marshal(rule)
	})
	if err != nil {
		return nil, err
	}

	var rule domain.CDNModSecRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, fmt.Errorf("decoding created modsec rule: %w", err)
	}
	return &rule, nil
}

func validateCDNModSecRuleCreate(rule domain.CDNModSecRule) error {
	if rule.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if rule.RuleValue == "" {
		return fmt.Errorf("rule_value is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetCDNModSecRuleInput identifies the custom ModSec rule to look up.
type GetCDNModSecRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// getCDNModSecRuleProvider is the one provider method GetCDNModSecRule needs.
type getCDNModSecRuleProvider interface {
	GetCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNModSecRule, error)
}

// GetCDNModSecRule is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNModSecRule struct {
	queue    ports.Queue
	provider getCDNModSecRuleProvider
}

// NewGetCDNModSecRule builds the use case from its ports.
func NewGetCDNModSecRule(queue ports.Queue, provider getCDNModSecRuleProvider) *GetCDNModSecRule {
	return &GetCDNModSecRule{queue: queue, provider: provider}
}

// Execute returns one custom ModSec rule by id, decoded value and expanded
// referenced data included. The provider does not report the rule's name or
// status on this endpoint (ports.ParspackProvider's GetCDNModSecRule
// contract) — those fields come back empty; use ListCDNModSecRules for them.
func (uc *GetCDNModSecRule) Execute(ctx context.Context, in GetCDNModSecRuleInput) (*domain.CDNModSecRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("rule_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rule, err := uc.provider.GetCDNModSecRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID)
		if err != nil {
			return nil, fmt.Errorf("getting modsec rule %s for zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(rule)
	})
	if err != nil {
		return nil, err
	}

	var rule domain.CDNModSecRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, fmt.Errorf("decoding modsec rule: %w", err)
	}
	return &rule, nil
}

// UpdateCDNModSecRuleInput is the normalized form of an
// update_cdn_modsec_rule tool call.
type UpdateCDNModSecRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
	Rule        domain.CDNModSecRule
}

// updateCDNModSecRuleProvider is the one provider method UpdateCDNModSecRule needs.
type updateCDNModSecRuleProvider interface {
	UpdateCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNModSecRule) (*domain.CDNModSecRule, error)
}

// UpdateCDNModSecRule is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNModSecRule struct {
	queue    ports.Queue
	provider updateCDNModSecRuleProvider
}

// NewUpdateCDNModSecRule builds the use case from its ports.
func NewUpdateCDNModSecRule(queue ports.Queue, provider updateCDNModSecRuleProvider) *UpdateCDNModSecRule {
	return &UpdateCDNModSecRule{queue: queue, provider: provider}
}

// Execute validates the request and replaces an existing custom ModSec
// rule's name, value and referenced data ids by id. Unlike create, rule_value
// is optional here — the provider's spec marks it nullable on update, e.g. to
// leave the rule value unchanged while only updating referenced data ids.
func (uc *UpdateCDNModSecRule) Execute(ctx context.Context, in UpdateCDNModSecRuleInput) (*domain.CDNModSecRule, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return nil, fmt.Errorf("rule_id is required: %w", domain.ErrInvalidInput)
	}
	if in.Rule.Name == "" {
		return nil, fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rule, err := uc.provider.UpdateCDNModSecRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating modsec rule %s for zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.Marshal(rule)
	})
	if err != nil {
		return nil, err
	}

	var rule domain.CDNModSecRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, fmt.Errorf("decoding updated modsec rule: %w", err)
	}
	return &rule, nil
}

// DeleteCDNModSecRuleInput identifies the custom ModSec rule to remove.
type DeleteCDNModSecRuleInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	RuleID      string
}

// deleteCDNModSecRuleProvider is the one provider method DeleteCDNModSecRule needs.
type deleteCDNModSecRuleProvider interface {
	DeleteCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error
}

// DeleteCDNModSecRule is a fast operation. Deleting a rule the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely.
type DeleteCDNModSecRule struct {
	queue    ports.Queue
	provider deleteCDNModSecRuleProvider
}

// NewDeleteCDNModSecRule builds the use case from its ports.
func NewDeleteCDNModSecRule(queue ports.Queue, provider deleteCDNModSecRuleProvider) *DeleteCDNModSecRule {
	return &DeleteCDNModSecRule{queue: queue, provider: provider}
}

// Execute deletes the custom ModSec rule, tolerating one that is already
// gone.
func (uc *DeleteCDNModSecRule) Execute(ctx context.Context, in DeleteCDNModSecRuleInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.RuleID == "" {
		return fmt.Errorf("rule_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNModSecRule(ctx, in.Credentials, in.ZoneUUID, in.RuleID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting modsec rule %s for zone %s: %w", in.RuleID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
