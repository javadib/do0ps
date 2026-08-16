package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// SetupDNSInput is the normalized form of a create_dns_record tool call.
type SetupDNSInput struct {
	Credentials domain.ProviderCredentials
	ZoneName    string
	Record      domain.DNSRecord
}

// SetupDNS is a fast operation: it runs on a worker but the caller waits for
// the result inside the same tool call and never learns a queue exists.
type SetupDNS struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewSetupDNS builds the use case from its ports.
func NewSetupDNS(queue ports.Queue, provider ports.ParspackProvider) *SetupDNS {
	return &SetupDNS{queue: queue, provider: provider}
}

// Execute resolves the zone by name and creates the record, returning the
// stored record synchronously.
func (uc *SetupDNS) Execute(ctx context.Context, in SetupDNSInput) (*domain.DNSRecord, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneName == "" {
		return nil, fmt.Errorf("zone name is required: %w", domain.ErrInvalidInput)
	}
	if in.Record.Type == domain.DNSRecordTypeUnknown {
		return nil, fmt.Errorf("record type is required: %w", domain.ErrInvalidInput)
	}
	if in.Record.Value == "" {
		return nil, fmt.Errorf("record value is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		zone, err := uc.findZone(ctx, in.Credentials, in.ZoneName)
		if err != nil {
			return nil, err
		}

		rec := in.Record
		rec.ZoneID = zone.ID

		created, err := uc.provider.CreateDNSRecord(ctx, in.Credentials, rec)
		if err != nil {
			return nil, fmt.Errorf("creating %s record %q: %w", rec.Type, rec.Name, err)
		}
		return json.Marshal(created)
	})
	if err != nil {
		return nil, err
	}

	var created domain.DNSRecord
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, fmt.Errorf("decoding created DNS record: %w", err)
	}
	return &created, nil
}

// findZone looks the zone up by name, as callers speak in domain names rather
// than provider zone IDs.
func (uc *SetupDNS) findZone(ctx context.Context, creds domain.ProviderCredentials, name string) (*domain.DNSZone, error) {
	zones, err := uc.provider.ListDNSZones(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("listing DNS zones: %w", err)
	}
	for i := range zones {
		if zones[i].Name == name {
			return &zones[i], nil
		}
	}
	return nil, fmt.Errorf("zone %q: %w", name, domain.ErrNotFound)
}

func isNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
