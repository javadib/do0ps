package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// validateDNSRecord checks a record's shape against the enums the CDN API
// confirms (AGENTS.md 4.5, issue #19), so a bad TTL or missing content fails
// fast here instead of reaching the provider and coming back as a 422.
func validateDNSRecord(rec domain.DNSRecord) error {
	if rec.Host == "" {
		return fmt.Errorf("record host is required: %w", domain.ErrInvalidInput)
	}
	if rec.Type == domain.DNSRecordTypeUnknown {
		return fmt.Errorf("record type is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidDNSRecordTTL(rec.TTL) {
		return fmt.Errorf("ttl %d is not one of the values Parspack accepts: %w", rec.TTL, domain.ErrInvalidInput)
	}
	if rec.Proxy == domain.DNSRecordProxyUnknown {
		return fmt.Errorf("proxy mode is required: %w", domain.ErrInvalidInput)
	}
	if len(rec.Values) != 1 || rec.Values[0].Content == "" {
		return fmt.Errorf("record content is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// ListDNSRecordsInput identifies the zone whose records to list.
type ListDNSRecordsInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// ListDNSRecords is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type ListDNSRecords struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListDNSRecords builds the use case from its ports.
func NewListDNSRecords(queue ports.Queue, provider ports.ParspackProvider) *ListDNSRecords {
	return &ListDNSRecords{queue: queue, provider: provider}
}

// Execute returns every record of the given zone.
func (uc *ListDNSRecords) Execute(ctx context.Context, in ListDNSRecordsInput) ([]domain.DNSRecord, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		records, err := uc.provider.ListDNSRecords(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing DNS records of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(records)
	})
	if err != nil {
		return nil, err
	}

	var records []domain.DNSRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("decoding DNS record list: %w", err)
	}
	return records, nil
}

// CreateDNSRecordInput is the normalized form of a create_dns_record tool
// call.
type CreateDNSRecordInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Record      domain.DNSRecord
}

// CreateDNSRecord is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type CreateDNSRecord struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewCreateDNSRecord builds the use case from its ports.
func NewCreateDNSRecord(queue ports.Queue, provider ports.ParspackProvider) *CreateDNSRecord {
	return &CreateDNSRecord{queue: queue, provider: provider}
}

// Execute validates the record and creates it, returning the stored record
// synchronously.
func (uc *CreateDNSRecord) Execute(ctx context.Context, in CreateDNSRecordInput) (*domain.DNSRecord, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateDNSRecord(in.Record); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		created, err := uc.provider.CreateDNSRecord(ctx, in.Credentials, in.ZoneUUID, in.Record)
		if err != nil {
			return nil, fmt.Errorf("creating %s record %q in zone %s: %w", in.Record.Type, in.Record.Host, in.ZoneUUID, err)
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

// UpdateDNSRecordInput is the normalized form of an update_dns_record tool
// call.
type UpdateDNSRecordInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Record      domain.DNSRecord
}

// UpdateDNSRecord is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type UpdateDNSRecord struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateDNSRecord builds the use case from its ports.
func NewUpdateDNSRecord(queue ports.Queue, provider ports.ParspackProvider) *UpdateDNSRecord {
	return &UpdateDNSRecord{queue: queue, provider: provider}
}

// Execute validates the record and updates it, returning the stored record
// synchronously.
func (uc *UpdateDNSRecord) Execute(ctx context.Context, in UpdateDNSRecordInput) (*domain.DNSRecord, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateDNSRecord(in.Record); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		updated, err := uc.provider.UpdateDNSRecord(ctx, in.Credentials, in.ZoneUUID, in.Record)
		if err != nil {
			return nil, fmt.Errorf("updating %s record %q in zone %s: %w", in.Record.Type, in.Record.Host, in.ZoneUUID, err)
		}
		return json.Marshal(updated)
	})
	if err != nil {
		return nil, err
	}

	var updated domain.DNSRecord
	if err := json.Unmarshal(raw, &updated); err != nil {
		return nil, fmt.Errorf("decoding updated DNS record: %w", err)
	}
	return &updated, nil
}

// DeleteDNSRecordInput identifies the record (or single value of it) to
// remove. Content is optional: when empty, every value under Host+Type is
// deleted; otherwise only the value matching Content is removed.
type DeleteDNSRecordInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Host        string
	Type        domain.DNSRecordType
	Content     string
}

// DeleteDNSRecord is a fast operation. Deleting a record the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely (ports.ParspackProvider.DeleteDNSRecord's
// contract).
type DeleteDNSRecord struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteDNSRecord builds the use case from its ports.
func NewDeleteDNSRecord(queue ports.Queue, provider ports.ParspackProvider) *DeleteDNSRecord {
	return &DeleteDNSRecord{queue: queue, provider: provider}
}

// Execute deletes the record, tolerating one that is already gone.
func (uc *DeleteDNSRecord) Execute(ctx context.Context, in DeleteDNSRecordInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.Host == "" {
		return fmt.Errorf("host is required: %w", domain.ErrInvalidInput)
	}
	if in.Type == domain.DNSRecordTypeUnknown {
		return fmt.Errorf("record type is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteDNSRecord(ctx, in.Credentials, in.ZoneUUID, in.Host, in.Type, in.Content); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting %s record %q in zone %s: %w", in.Type, in.Host, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
