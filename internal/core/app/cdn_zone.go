package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// CreateCDNZoneInput is the normalized form of a create_cdn_zone tool call.
type CreateCDNZoneInput struct {
	Credentials domain.ProviderCredentials
	Spec        domain.CDNZoneSpec
}

// CreateCDNZone onboards a new domain onto Parspack's CDN. It is a fast
// operation: the provider's order endpoint returns a final zone_id and
// status synchronously, so — unlike ProvisionServer — there is no
// operation_id to poll (ports.ParspackProvider.CreateCDNZone, AGENTS.md 4.3).
type CreateCDNZone struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewCreateCDNZone builds the use case from its ports.
func NewCreateCDNZone(queue ports.Queue, provider ports.ParspackProvider) *CreateCDNZone {
	return &CreateCDNZone{queue: queue, provider: provider}
}

// Execute validates the request and creates the zone, returning the stored
// zone synchronously.
func (uc *CreateCDNZone) Execute(ctx context.Context, in CreateCDNZoneInput) (*domain.CDNZone, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateCDNZoneSpec(in.Spec); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		zone, err := uc.provider.CreateCDNZone(ctx, in.Credentials, in.Spec)
		if err != nil {
			return nil, fmt.Errorf("creating CDN zone %q: %w", in.Spec.Domain, err)
		}
		return json.Marshal(zone)
	})
	if err != nil {
		return nil, err
	}

	var zone domain.CDNZone
	if err := json.Unmarshal(raw, &zone); err != nil {
		return nil, fmt.Errorf("decoding created CDN zone: %w", err)
	}
	return &zone, nil
}

func validateCDNZoneSpec(spec domain.CDNZoneSpec) error {
	if spec.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidCDNZonePlan(spec.Plan) {
		return fmt.Errorf("plan %q is not one of the plans Parspack offers: %w", spec.Plan, domain.ErrInvalidInput)
	}
	if !domain.ValidCDNBillingCycle(spec.BillingCycle) {
		return fmt.Errorf("billing_cycle %q is not one of the cycles Parspack offers: %w", spec.BillingCycle, domain.ErrInvalidInput)
	}
	for _, rec := range spec.Records {
		if err := validateDNSRecord(rec); err != nil {
			return err
		}
	}
	return nil
}

// ListCDNZonesInput carries the credentials needed to list an account's CDN
// zones. There is nothing else to specify: listing is unscoped.
type ListCDNZonesInput struct {
	Credentials domain.ProviderCredentials
}

// ListCDNZones is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type ListCDNZones struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListCDNZones builds the use case from its ports.
func NewListCDNZones(queue ports.Queue, provider ports.ParspackProvider) *ListCDNZones {
	return &ListCDNZones{queue: queue, provider: provider}
}

// Execute returns every CDN zone visible to the given credentials.
func (uc *ListCDNZones) Execute(ctx context.Context, in ListCDNZonesInput) ([]domain.CDNZone, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		zones, err := uc.provider.ListCDNZones(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing CDN zones: %w", err)
		}
		return json.Marshal(zones)
	})
	if err != nil {
		return nil, err
	}

	var zones []domain.CDNZone
	if err := json.Unmarshal(raw, &zones); err != nil {
		return nil, fmt.Errorf("decoding CDN zone list: %w", err)
	}
	return zones, nil
}

// GetCDNZoneInput identifies the zone to look up.
type GetCDNZoneInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNZone is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type GetCDNZone struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNZone builds the use case from its ports.
func NewGetCDNZone(queue ports.Queue, provider ports.ParspackProvider) *GetCDNZone {
	return &GetCDNZone{queue: queue, provider: provider}
}

// Execute returns the current state of one zone.
func (uc *GetCDNZone) Execute(ctx context.Context, in GetCDNZoneInput) (*domain.CDNZone, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		zone, err := uc.provider.GetCDNZone(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting CDN zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(zone)
	})
	if err != nil {
		return nil, err
	}

	var zone domain.CDNZone
	if err := json.Unmarshal(raw, &zone); err != nil {
		return nil, fmt.Errorf("decoding CDN zone: %w", err)
	}
	return &zone, nil
}

// DeleteCDNZoneInput identifies the zone to remove.
type DeleteCDNZoneInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// DeleteCDNZone is a fast operation. Deleting a zone the provider no longer
// has is treated as already-done rather than an error, so callers can call
// it more than once safely (ports.ParspackProvider.DeleteCDNZone's contract).
type DeleteCDNZone struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteCDNZone builds the use case from its ports.
func NewDeleteCDNZone(queue ports.Queue, provider ports.ParspackProvider) *DeleteCDNZone {
	return &DeleteCDNZone{queue: queue, provider: provider}
}

// Execute deletes the zone, tolerating one that is already gone.
func (uc *DeleteCDNZone) Execute(ctx context.Context, in DeleteCDNZoneInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNZone(ctx, in.Credentials, in.ZoneUUID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting CDN zone %s: %w", in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ListCDNZonePlansInput carries the credentials needed to list available
// plans and pricing.
type ListCDNZonePlansInput struct {
	Credentials domain.ProviderCredentials
}

// ListCDNZonePlans is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type ListCDNZonePlans struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListCDNZonePlans builds the use case from its ports.
func NewListCDNZonePlans(queue ports.Queue, provider ports.ParspackProvider) *ListCDNZonePlans {
	return &ListCDNZonePlans{queue: queue, provider: provider}
}

// Execute returns the plans and pricing available for a new zone.
func (uc *ListCDNZonePlans) Execute(ctx context.Context, in ListCDNZonePlansInput) ([]domain.CDNZonePlanPricing, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		plans, err := uc.provider.ListCDNZonePlans(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing CDN zone plans: %w", err)
		}
		return json.Marshal(plans)
	})
	if err != nil {
		return nil, err
	}

	var plans []domain.CDNZonePlanPricing
	if err := json.Unmarshal(raw, &plans); err != nil {
		return nil, fmt.Errorf("decoding CDN zone plans: %w", err)
	}
	return plans, nil
}

// GetNameserverRecordsInput identifies the zone whose nameservers to look up.
type GetNameserverRecordsInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetNameserverRecords is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetNameserverRecords struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetNameserverRecords builds the use case from its ports.
func NewGetNameserverRecords(queue ports.Queue, provider ports.ParspackProvider) *GetNameserverRecords {
	return &GetNameserverRecords{queue: queue, provider: provider}
}

// Execute returns the nameservers a zone's domain registrar must be pointed
// at.
func (uc *GetNameserverRecords) Execute(ctx context.Context, in GetNameserverRecordsInput) (*domain.NameserverRecords, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		ns, err := uc.provider.GetNameserverRecords(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting nameserver records for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(ns)
	})
	if err != nil {
		return nil, err
	}

	var ns domain.NameserverRecords
	if err := json.Unmarshal(raw, &ns); err != nil {
		return nil, fmt.Errorf("decoding nameserver records: %w", err)
	}
	return &ns, nil
}
