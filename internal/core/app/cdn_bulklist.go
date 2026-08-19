package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

func validateCDNBulklistSpec(spec domain.CDNBulklistSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidCDNBulklistType(spec.Type) {
		return fmt.Errorf("type %q is not one of the types Parspack's bulklist API accepts: %w", spec.Type, domain.ErrInvalidInput)
	}
	if len(spec.Items) == 0 {
		return fmt.Errorf("items must contain at least one value: %w", domain.ErrInvalidInput)
	}
	return nil
}

// ListCDNBulklistsInput carries the credentials needed to list an account's
// bulklists. There is nothing else to specify: listing is unscoped.
type ListCDNBulklistsInput struct {
	Credentials domain.ProviderCredentials
}

// ListCDNBulklists is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type ListCDNBulklists struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListCDNBulklists builds the use case from its ports.
func NewListCDNBulklists(queue ports.Queue, provider ports.ParspackProvider) *ListCDNBulklists {
	return &ListCDNBulklists{queue: queue, provider: provider}
}

// Execute returns every bulklist visible to the given credentials.
func (uc *ListCDNBulklists) Execute(ctx context.Context, in ListCDNBulklistsInput) ([]domain.CDNBulklist, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		lists, err := uc.provider.ListCDNBulklists(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing CDN bulklists: %w", err)
		}
		return json.Marshal(lists)
	})
	if err != nil {
		return nil, err
	}

	var lists []domain.CDNBulklist
	if err := json.Unmarshal(raw, &lists); err != nil {
		return nil, fmt.Errorf("decoding CDN bulklist list: %w", err)
	}
	return lists, nil
}

// CreateCDNBulklistInput is the normalized form of a create_cdn_bulklist
// tool call.
type CreateCDNBulklistInput struct {
	Credentials domain.ProviderCredentials
	Spec        domain.CDNBulklistSpec
}

// CreateCDNBulklist is a fast operation: the created bulklist is returned
// within this call.
type CreateCDNBulklist struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewCreateCDNBulklist builds the use case from its ports.
func NewCreateCDNBulklist(queue ports.Queue, provider ports.ParspackProvider) *CreateCDNBulklist {
	return &CreateCDNBulklist{queue: queue, provider: provider}
}

// Execute validates the request and creates the bulklist, returning it
// synchronously.
func (uc *CreateCDNBulklist) Execute(ctx context.Context, in CreateCDNBulklistInput) (*domain.CDNBulklist, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateCDNBulklistSpec(in.Spec); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		list, err := uc.provider.CreateCDNBulklist(ctx, in.Credentials, in.Spec)
		if err != nil {
			return nil, fmt.Errorf("creating CDN bulklist %q: %w", in.Spec.Name, err)
		}
		return json.Marshal(list)
	})
	if err != nil {
		return nil, err
	}

	var list domain.CDNBulklist
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decoding created CDN bulklist: %w", err)
	}
	return &list, nil
}

// GetCDNBulklistInput identifies the bulklist to look up.
type GetCDNBulklistInput struct {
	Credentials domain.ProviderCredentials
	BulklistID  string
}

// GetCDNBulklist is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNBulklist struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNBulklist builds the use case from its ports.
func NewGetCDNBulklist(queue ports.Queue, provider ports.ParspackProvider) *GetCDNBulklist {
	return &GetCDNBulklist{queue: queue, provider: provider}
}

// Execute returns the current state of one bulklist.
func (uc *GetCDNBulklist) Execute(ctx context.Context, in GetCDNBulklistInput) (*domain.CDNBulklist, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.BulklistID == "" {
		return nil, fmt.Errorf("bulklist_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		list, err := uc.provider.GetCDNBulklist(ctx, in.Credentials, in.BulklistID)
		if err != nil {
			return nil, fmt.Errorf("getting CDN bulklist %s: %w", in.BulklistID, err)
		}
		return json.Marshal(list)
	})
	if err != nil {
		return nil, err
	}

	var list domain.CDNBulklist
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decoding CDN bulklist: %w", err)
	}
	return &list, nil
}

// UpdateCDNBulklistInput is the normalized form of an update_cdn_bulklist
// tool call.
type UpdateCDNBulklistInput struct {
	Credentials domain.ProviderCredentials
	BulklistID  string
	Spec        domain.CDNBulklistSpec
}

// UpdateCDNBulklist is a fast operation: the updated bulklist is returned
// within this call.
type UpdateCDNBulklist struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNBulklist builds the use case from its ports.
func NewUpdateCDNBulklist(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNBulklist {
	return &UpdateCDNBulklist{queue: queue, provider: provider}
}

// Execute validates the request and replaces the bulklist's name, type and
// items.
func (uc *UpdateCDNBulklist) Execute(ctx context.Context, in UpdateCDNBulklistInput) (*domain.CDNBulklist, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.BulklistID == "" {
		return nil, fmt.Errorf("bulklist_id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNBulklistSpec(in.Spec); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		list, err := uc.provider.UpdateCDNBulklist(ctx, in.Credentials, in.BulklistID, in.Spec)
		if err != nil {
			return nil, fmt.Errorf("updating CDN bulklist %s: %w", in.BulklistID, err)
		}
		return json.Marshal(list)
	})
	if err != nil {
		return nil, err
	}

	var list domain.CDNBulklist
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decoding updated CDN bulklist: %w", err)
	}
	return &list, nil
}

// DeleteCDNBulklistInput identifies the bulklist to remove.
type DeleteCDNBulklistInput struct {
	Credentials domain.ProviderCredentials
	BulklistID  string
}

// DeleteCDNBulklist is a fast operation. Deleting a bulklist the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely.
type DeleteCDNBulklist struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteCDNBulklist builds the use case from its ports.
func NewDeleteCDNBulklist(queue ports.Queue, provider ports.ParspackProvider) *DeleteCDNBulklist {
	return &DeleteCDNBulklist{queue: queue, provider: provider}
}

// Execute deletes the bulklist, tolerating one that is already gone.
func (uc *DeleteCDNBulklist) Execute(ctx context.Context, in DeleteCDNBulklistInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.BulklistID == "" {
		return fmt.Errorf("bulklist_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNBulklist(ctx, in.Credentials, in.BulklistID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting CDN bulklist %s: %w", in.BulklistID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ListCDNFirewallCountriesInput identifies the zone whose country reference
// list to fetch.
type ListCDNFirewallCountriesInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// ListCDNFirewallCountries is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type ListCDNFirewallCountries struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListCDNFirewallCountries builds the use case from its ports.
func NewListCDNFirewallCountries(queue ports.Queue, provider ports.ParspackProvider) *ListCDNFirewallCountries {
	return &ListCDNFirewallCountries{queue: queue, provider: provider}
}

// Execute returns the country reference list usable for country-based
// firewall rules on the given zone.
func (uc *ListCDNFirewallCountries) Execute(ctx context.Context, in ListCDNFirewallCountriesInput) ([]domain.CDNCountry, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		countries, err := uc.provider.ListCDNFirewallCountries(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing CDN firewall countries for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(countries)
	})
	if err != nil {
		return nil, err
	}

	var countries []domain.CDNCountry
	if err := json.Unmarshal(raw, &countries); err != nil {
		return nil, fmt.Errorf("decoding CDN firewall countries: %w", err)
	}
	return countries, nil
}
