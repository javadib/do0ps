package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the Lists ("dynamic-fields") use cases for ArvanCloud
// (issue #64): a reusable, account-scoped collection of values other CDN
// capabilities (firewall, WAF, DDoS protection, rate limiting — AC5-AC8)
// reference by ID. Every one of them is a fast operation
// (ports.ArvanCloudProvider, AGENTS.md 4.3): each dispatches onto the queue
// and blocks for the result within the same tool call.

// ListArvanCloudDynamicFieldsInput carries the credentials needed to list an
// account's ArvanCloud lists. There is nothing else to specify: listing is
// unscoped, matching ListArvanCloudDomains' own choice.
type ListArvanCloudDynamicFieldsInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudDynamicFields is a fast operation: it runs on a worker but
// the caller waits for the result inside the same tool call.
type ListArvanCloudDynamicFields struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudDynamicFields builds the use case from its ports.
func NewListArvanCloudDynamicFields(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudDynamicFields {
	return &ListArvanCloudDynamicFields{queue: queue, provider: provider}
}

// Execute returns every list visible to the given credentials.
func (uc *ListArvanCloudDynamicFields) Execute(ctx context.Context, in ListArvanCloudDynamicFieldsInput) ([]domain.ArvanCloudDynamicField, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		fields, err := uc.provider.ListArvanCloudDynamicFields(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud dynamic fields: %w", err)
		}
		return json.Marshal(fields)
	})
	if err != nil {
		return nil, err
	}

	var fields []domain.ArvanCloudDynamicField
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("decoding arvancloud dynamic field list: %w", err)
	}
	return fields, nil
}

// CreateArvanCloudDynamicFieldInput is the normalized form of a
// create_arvancloud_dynamic_field tool call.
type CreateArvanCloudDynamicFieldInput struct {
	Credentials domain.ProviderCredentials
	Field       domain.ArvanCloudDynamicField
}

// CreateArvanCloudDynamicField creates a new list.
type CreateArvanCloudDynamicField struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudDynamicField builds the use case from its ports.
func NewCreateArvanCloudDynamicField(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudDynamicField {
	return &CreateArvanCloudDynamicField{queue: queue, provider: provider}
}

// Execute validates the request and creates the list, returning it as stored.
func (uc *CreateArvanCloudDynamicField) Execute(ctx context.Context, in CreateArvanCloudDynamicFieldInput) (*domain.ArvanCloudDynamicField, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Field.Name == "" {
		return nil, fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudDynamicFieldType(string(in.Field.Type)) {
		return nil, fmt.Errorf("type %q is not one of \"ip\", \"number\" or \"byte\": %w", in.Field.Type, domain.ErrInvalidInput)
	}

	return dispatchArvanCloudDynamicField(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDynamicField, error) {
		created, err := uc.provider.CreateArvanCloudDynamicField(ctx, in.Credentials, in.Field)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud dynamic field %q: %w", in.Field.Name, err)
		}
		return created, nil
	})
}

// arvanCloudDynamicFieldIDInput is embedded by every use case below that is
// scoped to exactly one list by id and needs nothing else.
type arvanCloudDynamicFieldIDInput struct {
	Credentials domain.ProviderCredentials
	ID          string
}

func (in arvanCloudDynamicFieldIDInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudDynamicFieldInput identifies the list to look up.
type GetArvanCloudDynamicFieldInput = arvanCloudDynamicFieldIDInput

// GetArvanCloudDynamicField is a fast operation.
type GetArvanCloudDynamicField struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudDynamicField builds the use case from its ports.
func NewGetArvanCloudDynamicField(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudDynamicField {
	return &GetArvanCloudDynamicField{queue: queue, provider: provider}
}

// Execute returns the current state of one list.
func (uc *GetArvanCloudDynamicField) Execute(ctx context.Context, in GetArvanCloudDynamicFieldInput) (*domain.ArvanCloudDynamicField, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDynamicField(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDynamicField, error) {
		found, err := uc.provider.GetArvanCloudDynamicField(ctx, in.Credentials, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud dynamic field %q: %w", in.ID, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudDynamicFieldInput identifies the list to update and its new
// description and/or type.
//
// Type is required even when the caller only means to change Description:
// the CDN API's update endpoint (DynamicFieldUpdateRequest) requires "type"
// in every request body, despite documenting no way to actually change a
// list's value type after creation — a caller changing only the description
// passes the list's current, unchanged Type here (e.g. fetched via
// GetArvanCloudDynamicField first).
type UpdateArvanCloudDynamicFieldInput struct {
	Credentials domain.ProviderCredentials
	ID          string
	Description string
	Type        domain.ArvanCloudDynamicFieldType
}

// UpdateArvanCloudDynamicField changes a list's description and/or type. This
// is a fast operation.
type UpdateArvanCloudDynamicField struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudDynamicField builds the use case from its ports.
func NewUpdateArvanCloudDynamicField(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudDynamicField {
	return &UpdateArvanCloudDynamicField{queue: queue, provider: provider}
}

// Execute updates the list and returns it as stored afterward.
func (uc *UpdateArvanCloudDynamicField) Execute(ctx context.Context, in UpdateArvanCloudDynamicFieldInput) (*domain.ArvanCloudDynamicField, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudDynamicFieldType(string(in.Type)) {
		return nil, fmt.Errorf("type %q is not one of \"ip\", \"number\" or \"byte\": %w", in.Type, domain.ErrInvalidInput)
	}

	return dispatchArvanCloudDynamicField(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDynamicField, error) {
		updated, err := uc.provider.UpdateArvanCloudDynamicField(ctx, in.Credentials, in.ID, in.Description, in.Type)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud dynamic field %q: %w", in.ID, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudDynamicFieldInput identifies the list to remove.
type DeleteArvanCloudDynamicFieldInput = arvanCloudDynamicFieldIDInput

// DeleteArvanCloudDynamicField is a fast operation. Deleting a list the
// provider no longer has is treated as already done rather than an error,
// matching DeleteArvanCloudDomain's tolerant-delete contract.
type DeleteArvanCloudDynamicField struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudDynamicField builds the use case from its ports.
func NewDeleteArvanCloudDynamicField(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudDynamicField {
	return &DeleteArvanCloudDynamicField{queue: queue, provider: provider}
}

// Execute deletes the list, tolerating one that is already gone.
func (uc *DeleteArvanCloudDynamicField) Execute(ctx context.Context, in DeleteArvanCloudDynamicFieldInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudDynamicField(ctx, in.Credentials, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud dynamic field %q: %w", in.ID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// AddArvanCloudDynamicFieldItemsInput identifies the list and the items to
// append to it.
type AddArvanCloudDynamicFieldItemsInput struct {
	Credentials domain.ProviderCredentials
	ID          string
	Values      []domain.ArvanCloudDynamicFieldValue
}

// AddArvanCloudDynamicFieldItems appends items to a list. This is a fast
// operation.
type AddArvanCloudDynamicFieldItems struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewAddArvanCloudDynamicFieldItems builds the use case from its ports.
func NewAddArvanCloudDynamicFieldItems(queue ports.Queue, provider ports.ArvanCloudProvider) *AddArvanCloudDynamicFieldItems {
	return &AddArvanCloudDynamicFieldItems{queue: queue, provider: provider}
}

// Execute appends the items. The provider's response carries no data (see
// ports.ArvanCloudProvider.AddArvanCloudDynamicFieldItems's doc comment), so
// a caller that needs the newly-assigned item IDs calls
// GetArvanCloudDynamicField afterward.
func (uc *AddArvanCloudDynamicFieldItems) Execute(ctx context.Context, in AddArvanCloudDynamicFieldItemsInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if len(in.Values) == 0 {
		return fmt.Errorf("values must contain at least one item: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.AddArvanCloudDynamicFieldItems(ctx, in.Credentials, in.ID, in.Values); err != nil {
			return nil, fmt.Errorf("adding items to arvancloud dynamic field %q: %w", in.ID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// RemoveArvanCloudDynamicFieldItemInput identifies the list and the item to
// remove from it.
type RemoveArvanCloudDynamicFieldItemInput struct {
	Credentials domain.ProviderCredentials
	ID          string
	ItemID      string
}

// RemoveArvanCloudDynamicFieldItem removes one item from a list. This is a
// fast operation.
type RemoveArvanCloudDynamicFieldItem struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewRemoveArvanCloudDynamicFieldItem builds the use case from its ports.
func NewRemoveArvanCloudDynamicFieldItem(queue ports.Queue, provider ports.ArvanCloudProvider) *RemoveArvanCloudDynamicFieldItem {
	return &RemoveArvanCloudDynamicFieldItem{queue: queue, provider: provider}
}

// Execute removes the item, tolerating one that is already gone (matching
// DeleteArvanCloudDynamicField's tolerant-delete contract).
func (uc *RemoveArvanCloudDynamicFieldItem) Execute(ctx context.Context, in RemoveArvanCloudDynamicFieldItemInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if in.ItemID == "" {
		return fmt.Errorf("item_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.RemoveArvanCloudDynamicFieldItem(ctx, in.Credentials, in.ID, in.ItemID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("removing item %q from arvancloud dynamic field %q: %w", in.ItemID, in.ID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// dispatchArvanCloudDynamicField runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudDynamicField, the shape every use case above
// but delete/add-items/remove-item returns.
func dispatchArvanCloudDynamicField(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudDynamicField, error),
) (*domain.ArvanCloudDynamicField, error) {
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

	var result domain.ArvanCloudDynamicField
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud dynamic field: %w", err)
	}
	return &result, nil
}
