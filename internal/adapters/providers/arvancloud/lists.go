package arvancloud

import (
	"context"
	"fmt"
	"net/http"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Lists ("dynamic-fields", issue #64), wired to the real CDN API. Unlike
// every other capability in this package, a list is account-scoped, not
// scoped to a domain by name — the path is /dynamic-fields, not
// /domains/{domain}/dynamic-fields. Base path is confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "List" tag (the lists.*
// operationIds).

const dynamicFieldsBasePath = "dynamic-fields"

func dynamicFieldPath(id string) string {
	return dynamicFieldsBasePath + "/" + id
}

// dynamicFieldValueWire mirrors DynamicFieldValue.
type dynamicFieldValueWire struct {
	ID        string `json:"id,omitempty"`
	Value     any    `json:"value"`
	Desc      string `json:"desc,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func toDynamicFieldValueDomain(w dynamicFieldValueWire) domain.ArvanCloudDynamicFieldValue {
	return domain.ArvanCloudDynamicFieldValue{
		ID:        w.ID,
		Value:     w.Value,
		Desc:      w.Desc,
		CreatedAt: w.CreatedAt,
	}
}

func toDynamicFieldValueWire(v domain.ArvanCloudDynamicFieldValue) dynamicFieldValueWire {
	return dynamicFieldValueWire{ID: v.ID, Value: v.Value, Desc: v.Desc, CreatedAt: v.CreatedAt}
}

// dynamicFieldWire mirrors the DynamicField schema.
type dynamicFieldWire struct {
	ID           string                  `json:"id,omitempty"`
	Name         string                  `json:"name"`
	Description  string                  `json:"description,omitempty"`
	Namespace    string                  `json:"namespace,omitempty"`
	Type         string                  `json:"type"`
	Scope        string                  `json:"scope,omitempty"`
	Values       []dynamicFieldValueWire `json:"values"`
	AllowedPlans []int                   `json:"allowed_plans,omitempty"`
	CreatedAt    string                  `json:"created_at,omitempty"`
	UpdatedAt    string                  `json:"updated_at,omitempty"`
}

func toDynamicFieldDomain(w dynamicFieldWire) domain.ArvanCloudDynamicField {
	values := make([]domain.ArvanCloudDynamicFieldValue, len(w.Values))
	for i := range w.Values {
		values[i] = toDynamicFieldValueDomain(w.Values[i])
	}
	return domain.ArvanCloudDynamicField{
		ID:           w.ID,
		Name:         w.Name,
		Description:  w.Description,
		Namespace:    w.Namespace,
		Type:         domain.ArvanCloudDynamicFieldType(w.Type),
		Scope:        domain.ArvanCloudDynamicFieldScope(w.Scope),
		Values:       values,
		AllowedPlans: w.AllowedPlans,
		CreatedAt:    w.CreatedAt,
		UpdatedAt:    w.UpdatedAt,
	}
}

// ListArvanCloudDynamicFields returns every list visible to the given
// credentials, unfiltered.
func (p *Provider) ListArvanCloudDynamicFields(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudDynamicField, error) {
	var items []dynamicFieldWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, dynamicFieldsBasePath, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud dynamic fields: %w", err)
	}

	fields := make([]domain.ArvanCloudDynamicField, len(items))
	for i := range items {
		fields[i] = toDynamicFieldDomain(items[i])
	}
	return fields, nil
}

// CreateArvanCloudDynamicField creates a new list. field.Values may be nil or
// empty — the store endpoint accepts an empty items array.
func (p *Provider) CreateArvanCloudDynamicField(ctx context.Context, creds domain.ProviderCredentials, field domain.ArvanCloudDynamicField) (*domain.ArvanCloudDynamicField, error) {
	values := make([]dynamicFieldValueWire, len(field.Values))
	for i := range field.Values {
		values[i] = toDynamicFieldValueWire(field.Values[i])
	}
	body := dynamicFieldWire{
		Name:        field.Name,
		Description: field.Description,
		Type:        string(field.Type),
		Values:      values,
	}

	var wire dynamicFieldWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, dynamicFieldsBasePath, body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud dynamic field %q: %w", field.Name, err)
	}
	created := toDynamicFieldDomain(wire)
	return &created, nil
}

// GetArvanCloudDynamicField returns a single list by id.
func (p *Provider) GetArvanCloudDynamicField(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.ArvanCloudDynamicField, error) {
	var wire dynamicFieldWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, dynamicFieldPath(id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud dynamic field %q: %w", id, err)
	}
	found := toDynamicFieldDomain(wire)
	return &found, nil
}

// dynamicFieldUpdateWire mirrors DynamicFieldUpdateRequest: the only two
// fields the (deprecated but still live) update endpoint accepts.
type dynamicFieldUpdateWire struct {
	Description *string `json:"description,omitempty"`
	Type        string  `json:"type"`
}

// UpdateArvanCloudDynamicField changes a list's description and/or type. Per
// DynamicFieldUpdateRequest, type is a required part of the request body
// even though the CDN API documents no way for a list's value type to
// actually change after creation in practice — this adapter always sends the
// caller-supplied fieldType, which ports.ArvanCloudProvider's use case
// callers pass as the list's current type when they only mean to change the
// description.
func (p *Provider) UpdateArvanCloudDynamicField(ctx context.Context, creds domain.ProviderCredentials, id string, description string, fieldType domain.ArvanCloudDynamicFieldType) (*domain.ArvanCloudDynamicField, error) {
	body := dynamicFieldUpdateWire{Type: string(fieldType)}
	if description != "" {
		body.Description = &description
	}

	var wire dynamicFieldWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, dynamicFieldPath(id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud dynamic field %q: %w", id, err)
	}
	updated := toDynamicFieldDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudDynamicField removes a list by id.
func (p *Provider) DeleteArvanCloudDynamicField(ctx context.Context, creds domain.ProviderCredentials, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, dynamicFieldPath(id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud dynamic field %q: %w", id, err)
	}
	return nil
}

// addDynamicFieldItemsWire mirrors the request body of lists.items.store:
// {"values": [DynamicFieldValue, ...]}.
type addDynamicFieldItemsWire struct {
	Values []dynamicFieldValueWire `json:"values"`
}

// AddArvanCloudDynamicFieldItems appends items to a list. The response
// carries no data — only a confirmation message (the spec's "OK" response) —
// so there is nothing to translate back but the error.
func (p *Provider) AddArvanCloudDynamicFieldItems(ctx context.Context, creds domain.ProviderCredentials, id string, values []domain.ArvanCloudDynamicFieldValue) error {
	wireValues := make([]dynamicFieldValueWire, len(values))
	for i := range values {
		wireValues[i] = toDynamicFieldValueWire(values[i])
	}
	body := addDynamicFieldItemsWire{Values: wireValues}

	if err := p.client.doJSON(ctx, creds, http.MethodPost, dynamicFieldPath(id)+"/items", body, nil); err != nil {
		return fmt.Errorf("adding items to arvancloud dynamic field %q: %w", id, err)
	}
	return nil
}

// RemoveArvanCloudDynamicFieldItem removes one item from a list by the
// item's own id.
func (p *Provider) RemoveArvanCloudDynamicFieldItem(ctx context.Context, creds domain.ProviderCredentials, id, itemID string) error {
	path := dynamicFieldPath(id) + "/items/" + itemID
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("removing item %q from arvancloud dynamic field %q: %w", itemID, id, err)
	}
	return nil
}
