package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the DNS record, DNSSEC and Secondary DNS use cases for
// ArvanCloud (issue #63). Every one of them is a fast operation
// (ports.ArvanCloudProvider, AGENTS.md 4.3): each dispatches onto the queue
// and blocks for the result within the same tool call — there is no
// operation_id to poll.

// arvanCloudDomainScopedInput identifies the domain (and credentials) every
// use case below needs at minimum, mirroring arvanCloudDomainNameInput in
// arvancloud_domain.go.
type arvanCloudDomainScopedInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
}

func (in arvanCloudDomainScopedInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.DomainName == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// validateArvanCloudDNSRecord checks a record's shape against the enums and
// per-type required value fields the CDN API confirms (issue #63's
// "Confirmed record shape" section), so a bad TTL, an unknown type, or a
// value missing that type's required sub-fields (e.g. a CAA record without
// tag) fails fast here instead of reaching the provider and coming back as a
// 422.
func validateArvanCloudDNSRecord(rec domain.ArvanCloudDNSRecord) error {
	if rec.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if rec.Type == domain.ArvanCloudDNSRecordTypeUnknown {
		return fmt.Errorf("type is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudDNSRecordTTL(rec.TTL) {
		return fmt.Errorf("ttl %d is not one of the values ArvanCloud accepts: %w", rec.TTL, domain.ErrInvalidInput)
	}
	if rec.UpstreamHTTPS != "" && !domain.ValidArvanCloudUpstreamHTTPS(rec.UpstreamHTTPS) {
		return fmt.Errorf("upstream_https %q is not one of the values ArvanCloud accepts: %w", rec.UpstreamHTTPS, domain.ErrInvalidInput)
	}
	if err := validateArvanCloudIPFilterMode(rec.IPFilterMode); err != nil {
		return err
	}
	return validateArvanCloudDNSRecordValues(rec.Type, rec.Values)
}

func validateArvanCloudIPFilterMode(m domain.ArvanCloudDNSRecordIPFilterMode) error {
	if m.Count != "" && !domain.ValidArvanCloudIPFilterCount(m.Count) {
		return fmt.Errorf("ip_filter_mode.count %q is not one of the values ArvanCloud accepts: %w", m.Count, domain.ErrInvalidInput)
	}
	if m.Order != "" && !domain.ValidArvanCloudIPFilterOrder(m.Order) {
		return fmt.Errorf("ip_filter_mode.order %q is not one of the values ArvanCloud accepts: %w", m.Order, domain.ErrInvalidInput)
	}
	if m.GeoFilter != "" && !domain.ValidArvanCloudIPFilterGeo(m.GeoFilter) {
		return fmt.Errorf("ip_filter_mode.geo_filter %q is not one of the values ArvanCloud accepts: %w", m.GeoFilter, domain.ErrInvalidInput)
	}
	return nil
}

// singleArvanCloudDNSRecordValue requires values to carry exactly one entry
// — every record type but A/AAAA carries exactly one value.
func singleArvanCloudDNSRecordValue(t domain.ArvanCloudDNSRecordType, values []domain.ArvanCloudDNSRecordValue) (domain.ArvanCloudDNSRecordValue, error) {
	if len(values) != 1 {
		return domain.ArvanCloudDNSRecordValue{}, fmt.Errorf(
			"exactly one value is required for a(n) %s record, got %d: %w", t, len(values), domain.ErrInvalidInput)
	}
	return values[0], nil
}

// validateArvanCloudDNSRecordValues checks that values carries the
// sub-fields the given record type requires, per issue #63's "Confirmed
// record shape" section.
func validateArvanCloudDNSRecordValues(t domain.ArvanCloudDNSRecordType, values []domain.ArvanCloudDNSRecordValue) error {
	switch t {
	case domain.ArvanCloudDNSRecordTypeA, domain.ArvanCloudDNSRecordTypeAAAA:
		if len(values) == 0 {
			return fmt.Errorf("at least one value is required for a(n) %s record: %w", t, domain.ErrInvalidInput)
		}
		for i, v := range values {
			if v.IP == "" {
				return fmt.Errorf("value[%d].ip is required for a(n) %s record: %w", i, t, domain.ErrInvalidInput)
			}
		}
		return nil

	case domain.ArvanCloudDNSRecordTypeCNAME:
		v, err := singleArvanCloudDNSRecordValue(t, values)
		if err != nil {
			return err
		}
		if v.Host == "" {
			return fmt.Errorf("value.host is required for a CNAME record: %w", domain.ErrInvalidInput)
		}
		if v.HostHeader != "" && !domain.ValidArvanCloudHostHeader(v.HostHeader) {
			return fmt.Errorf("value.host_header %q is not \"source\" or \"dest\": %w", v.HostHeader, domain.ErrInvalidInput)
		}
		return nil

	case domain.ArvanCloudDNSRecordTypeANAME:
		v, err := singleArvanCloudDNSRecordValue(t, values)
		if err != nil {
			return err
		}
		if v.Location == "" {
			return fmt.Errorf("value.location is required for an ANAME record: %w", domain.ErrInvalidInput)
		}
		if v.HostHeader != "" && !domain.ValidArvanCloudHostHeader(v.HostHeader) {
			return fmt.Errorf("value.host_header %q is not \"source\" or \"dest\": %w", v.HostHeader, domain.ErrInvalidInput)
		}
		return nil

	case domain.ArvanCloudDNSRecordTypeMX:
		v, err := singleArvanCloudDNSRecordValue(t, values)
		if err != nil {
			return err
		}
		if v.Host == "" {
			return fmt.Errorf("value.host is required for an MX record: %w", domain.ErrInvalidInput)
		}
		if v.Priority < 0 || v.Priority > 9999 {
			return fmt.Errorf("value.priority %d is out of range 0-9999 for an MX record: %w", v.Priority, domain.ErrInvalidInput)
		}
		return nil

	case domain.ArvanCloudDNSRecordTypeSRV:
		v, err := singleArvanCloudDNSRecordValue(t, values)
		if err != nil {
			return err
		}
		if v.Target == "" {
			return fmt.Errorf("value.target is required for an SRV record: %w", domain.ErrInvalidInput)
		}
		if v.Port < 1 || v.Port > 65535 {
			return fmt.Errorf("value.port %d is out of range 1-65535 for an SRV record: %w", v.Port, domain.ErrInvalidInput)
		}
		return nil

	case domain.ArvanCloudDNSRecordTypeTXT, domain.ArvanCloudDNSRecordTypeSPF, domain.ArvanCloudDNSRecordTypeDKIM:
		v, err := singleArvanCloudDNSRecordValue(t, values)
		if err != nil {
			return err
		}
		if v.Text == "" {
			return fmt.Errorf("value.text is required for a(n) %s record: %w", t, domain.ErrInvalidInput)
		}
		if len(v.Text) > 500 {
			return fmt.Errorf("value.text exceeds 500 characters for a(n) %s record: %w", t, domain.ErrInvalidInput)
		}
		return nil

	case domain.ArvanCloudDNSRecordTypeNS:
		v, err := singleArvanCloudDNSRecordValue(t, values)
		if err != nil {
			return err
		}
		if v.Host == "" {
			return fmt.Errorf("value.host is required for an NS record: %w", domain.ErrInvalidInput)
		}
		return nil

	case domain.ArvanCloudDNSRecordTypePTR:
		// No required value sub-fields per the spec; only the value count
		// itself is checked.
		_, err := singleArvanCloudDNSRecordValue(t, values)
		return err

	case domain.ArvanCloudDNSRecordTypeTLSA:
		v, err := singleArvanCloudDNSRecordValue(t, values)
		if err != nil {
			return err
		}
		if v.Usage == "" || v.Selector == "" || v.MatchingType == "" || v.Certificate == "" {
			return fmt.Errorf(
				"value.usage, value.selector, value.matching_type and value.certificate are all required for a TLSA record: %w",
				domain.ErrInvalidInput)
		}
		return nil

	case domain.ArvanCloudDNSRecordTypeCAA:
		v, err := singleArvanCloudDNSRecordValue(t, values)
		if err != nil {
			return err
		}
		if v.CAAValue == "" {
			return fmt.Errorf("value.value is required for a CAA record: %w", domain.ErrInvalidInput)
		}
		if v.Tag == "" {
			return fmt.Errorf("value.tag is required for a CAA record: %w", domain.ErrInvalidInput)
		}
		if !domain.ValidArvanCloudCAATag(v.Tag) {
			return fmt.Errorf("value.tag %q is not one of \"issue\", \"issuewild\" or \"iodef\": %w", v.Tag, domain.ErrInvalidInput)
		}
		return nil

	default:
		return fmt.Errorf("record type %v is not one of the 13 ArvanCloud accepts: %w", t, domain.ErrInvalidInput)
	}
}

// dispatchArvanCloudDNSRecord runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudDNSRecord, the shape every DNS record use
// case below but Delete/Import returns.
func dispatchArvanCloudDNSRecord(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudDNSRecord, error),
) (*domain.ArvanCloudDNSRecord, error) {
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

	var result domain.ArvanCloudDNSRecord
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud dns record: %w", err)
	}
	return &result, nil
}

// --- DNS records -------------------------------------------------------

// ListArvanCloudDNSRecordsInput identifies the domain whose records to list.
type ListArvanCloudDNSRecordsInput = arvanCloudDomainScopedInput

// ListArvanCloudDNSRecords is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type ListArvanCloudDNSRecords struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudDNSRecords builds the use case from its ports.
func NewListArvanCloudDNSRecords(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudDNSRecords {
	return &ListArvanCloudDNSRecords{queue: queue, provider: provider}
}

// Execute returns every DNS record of the given domain.
func (uc *ListArvanCloudDNSRecords) Execute(ctx context.Context, in ListArvanCloudDNSRecordsInput) ([]domain.ArvanCloudDNSRecord, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		records, err := uc.provider.ListArvanCloudDNSRecords(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud dns records of domain %q: %w", in.DomainName, err)
		}
		return json.Marshal(records)
	})
	if err != nil {
		return nil, err
	}

	var records []domain.ArvanCloudDNSRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("decoding arvancloud dns record list: %w", err)
	}
	return records, nil
}

// CreateArvanCloudDNSRecordInput is the normalized form of a
// create_arvancloud_dns_record tool call.
type CreateArvanCloudDNSRecordInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
	Record      domain.ArvanCloudDNSRecord
}

// CreateArvanCloudDNSRecord is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type CreateArvanCloudDNSRecord struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudDNSRecord builds the use case from its ports.
func NewCreateArvanCloudDNSRecord(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudDNSRecord {
	return &CreateArvanCloudDNSRecord{queue: queue, provider: provider}
}

// Execute validates the record and creates it, returning the stored record
// synchronously.
func (uc *CreateArvanCloudDNSRecord) Execute(ctx context.Context, in CreateArvanCloudDNSRecordInput) (*domain.ArvanCloudDNSRecord, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.DomainName == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudDNSRecord(in.Record); err != nil {
		return nil, err
	}

	return dispatchArvanCloudDNSRecord(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDNSRecord, error) {
		created, err := uc.provider.CreateArvanCloudDNSRecord(ctx, in.Credentials, in.DomainName, in.Record)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud dns record %q in domain %q: %w", in.Record.Name, in.DomainName, err)
		}
		return created, nil
	})
}

// arvanCloudDNSRecordIDInput identifies a single record within a domain,
// embedded by every use case below that operates on exactly one existing
// record and needs nothing else.
type arvanCloudDNSRecordIDInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
	ID          string
}

func (in arvanCloudDNSRecordIDInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.DomainName == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudDNSRecordInput identifies the record to look up.
type GetArvanCloudDNSRecordInput = arvanCloudDNSRecordIDInput

// GetArvanCloudDNSRecord is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetArvanCloudDNSRecord struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudDNSRecord builds the use case from its ports.
func NewGetArvanCloudDNSRecord(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudDNSRecord {
	return &GetArvanCloudDNSRecord{queue: queue, provider: provider}
}

// Execute returns a single DNS record by id.
func (uc *GetArvanCloudDNSRecord) Execute(ctx context.Context, in GetArvanCloudDNSRecordInput) (*domain.ArvanCloudDNSRecord, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDNSRecord(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDNSRecord, error) {
		found, err := uc.provider.GetArvanCloudDNSRecord(ctx, in.Credentials, in.DomainName, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud dns record %q of domain %q: %w", in.ID, in.DomainName, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudDNSRecordInput is the normalized form of an
// update_arvancloud_dns_record tool call.
type UpdateArvanCloudDNSRecordInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
	ID          string
	Record      domain.ArvanCloudDNSRecord
}

// UpdateArvanCloudDNSRecord is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateArvanCloudDNSRecord struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudDNSRecord builds the use case from its ports.
func NewUpdateArvanCloudDNSRecord(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudDNSRecord {
	return &UpdateArvanCloudDNSRecord{queue: queue, provider: provider}
}

// Execute validates the record and updates it, returning the stored record
// synchronously.
func (uc *UpdateArvanCloudDNSRecord) Execute(ctx context.Context, in UpdateArvanCloudDNSRecordInput) (*domain.ArvanCloudDNSRecord, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.DomainName == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudDNSRecord(in.Record); err != nil {
		return nil, err
	}

	return dispatchArvanCloudDNSRecord(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDNSRecord, error) {
		updated, err := uc.provider.UpdateArvanCloudDNSRecord(ctx, in.Credentials, in.DomainName, in.ID, in.Record)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud dns record %q of domain %q: %w", in.ID, in.DomainName, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudDNSRecordInput identifies the record to remove.
type DeleteArvanCloudDNSRecordInput = arvanCloudDNSRecordIDInput

// DeleteArvanCloudDNSRecord is a fast operation. Deleting a record the
// provider no longer has is treated as already done rather than an error, so
// callers can call it more than once safely
// (ports.ArvanCloudProvider.DeleteArvanCloudDNSRecord's contract). A record
// the provider refuses to delete because it is protected or still in use is
// not pre-checked here; that refusal is propagated as whatever error the
// provider returns.
type DeleteArvanCloudDNSRecord struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudDNSRecord builds the use case from its ports.
func NewDeleteArvanCloudDNSRecord(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudDNSRecord {
	return &DeleteArvanCloudDNSRecord{queue: queue, provider: provider}
}

// Execute deletes the record, tolerating one that is already gone.
func (uc *DeleteArvanCloudDNSRecord) Execute(ctx context.Context, in DeleteArvanCloudDNSRecordInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudDNSRecord(ctx, in.Credentials, in.DomainName, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud dns record %q of domain %q: %w", in.ID, in.DomainName, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ToggleArvanCloudDNSRecordCloudInput identifies the record whose CDN-proxy
// status to flip and the value to set it to.
type ToggleArvanCloudDNSRecordCloudInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
	ID          string
	Cloud       bool
}

// ToggleArvanCloudDNSRecordCloud is a fast operation.
type ToggleArvanCloudDNSRecordCloud struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewToggleArvanCloudDNSRecordCloud builds the use case from its ports.
func NewToggleArvanCloudDNSRecordCloud(queue ports.Queue, provider ports.ArvanCloudProvider) *ToggleArvanCloudDNSRecordCloud {
	return &ToggleArvanCloudDNSRecordCloud{queue: queue, provider: provider}
}

// Execute flips the record's cloud (CDN-proxy) status.
func (uc *ToggleArvanCloudDNSRecordCloud) Execute(ctx context.Context, in ToggleArvanCloudDNSRecordCloudInput) (*domain.ArvanCloudDNSRecord, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.DomainName == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}

	return dispatchArvanCloudDNSRecord(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDNSRecord, error) {
		updated, err := uc.provider.ToggleArvanCloudDNSRecordCloud(ctx, in.Credentials, in.DomainName, in.ID, in.Cloud)
		if err != nil {
			return nil, fmt.Errorf("toggling cloud status of arvancloud dns record %q of domain %q: %w", in.ID, in.DomainName, err)
		}
		return updated, nil
	})
}

// ImportArvanCloudDNSRecordsInput identifies the domain to import into and
// carries the BIND zone file's raw content.
type ImportArvanCloudDNSRecordsInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
	ZoneFile    []byte
}

// ImportArvanCloudDNSRecords is a fast operation: the endpoint bulk-creates
// records from a BIND zone file in one synchronous call.
type ImportArvanCloudDNSRecords struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewImportArvanCloudDNSRecords builds the use case from its ports.
func NewImportArvanCloudDNSRecords(queue ports.Queue, provider ports.ArvanCloudProvider) *ImportArvanCloudDNSRecords {
	return &ImportArvanCloudDNSRecords{queue: queue, provider: provider}
}

// Execute imports the given zone file's records into the domain.
func (uc *ImportArvanCloudDNSRecords) Execute(ctx context.Context, in ImportArvanCloudDNSRecordsInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.DomainName == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if len(in.ZoneFile) == 0 {
		return fmt.Errorf("zone_file is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.ImportArvanCloudDNSRecords(ctx, in.Credentials, in.DomainName, in.ZoneFile); err != nil {
			return nil, fmt.Errorf("importing arvancloud dns records for domain %q: %w", in.DomainName, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ExportArvanCloudDNSRecordsInput identifies the domain to export.
type ExportArvanCloudDNSRecordsInput = arvanCloudDomainScopedInput

// ExportArvanCloudDNSRecords is a fast operation.
type ExportArvanCloudDNSRecords struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewExportArvanCloudDNSRecords builds the use case from its ports.
func NewExportArvanCloudDNSRecords(queue ports.Queue, provider ports.ArvanCloudProvider) *ExportArvanCloudDNSRecords {
	return &ExportArvanCloudDNSRecords{queue: queue, provider: provider}
}

// Execute returns the domain's DNS records as a BIND zone file's raw text.
func (uc *ExportArvanCloudDNSRecords) Execute(ctx context.Context, in ExportArvanCloudDNSRecordsInput) (string, error) {
	if err := in.validate(); err != nil {
		return "", err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		content, err := uc.provider.ExportArvanCloudDNSRecords(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("exporting arvancloud dns records for domain %q: %w", in.DomainName, err)
		}
		return json.Marshal(content)
	})
	if err != nil {
		return "", err
	}

	var content string
	if err := json.Unmarshal(raw, &content); err != nil {
		return "", fmt.Errorf("decoding exported arvancloud dns records: %w", err)
	}
	return content, nil
}

// --- DNSSEC --------------------------------------------------------------

// GetArvanCloudDNSSecStatusInput identifies the domain whose DNSSEC status
// to look up.
type GetArvanCloudDNSSecStatusInput = arvanCloudDomainScopedInput

// GetArvanCloudDNSSecStatus is a fast operation.
type GetArvanCloudDNSSecStatus struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudDNSSecStatus builds the use case from its ports.
func NewGetArvanCloudDNSSecStatus(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudDNSSecStatus {
	return &GetArvanCloudDNSSecStatus{queue: queue, provider: provider}
}

// Execute returns the domain's current DNSSEC status.
func (uc *GetArvanCloudDNSSecStatus) Execute(ctx context.Context, in GetArvanCloudDNSSecStatusInput) (*domain.ArvanCloudDNSSecStatus, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		status, err := uc.provider.GetArvanCloudDNSSecStatus(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("getting DNSSEC status for arvancloud domain %q: %w", in.DomainName, err)
		}
		return json.Marshal(status)
	})
	if err != nil {
		return nil, err
	}

	var status domain.ArvanCloudDNSSecStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("decoding arvancloud DNSSEC status: %w", err)
	}
	return &status, nil
}

// UpdateArvanCloudDNSSecStatusInput identifies the domain and the DNSSEC
// state to set.
type UpdateArvanCloudDNSSecStatusInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
	Enable      bool
	Rotate      bool
}

// UpdateArvanCloudDNSSecStatus is a fast operation.
type UpdateArvanCloudDNSSecStatus struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudDNSSecStatus builds the use case from its ports.
func NewUpdateArvanCloudDNSSecStatus(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudDNSSecStatus {
	return &UpdateArvanCloudDNSSecStatus{queue: queue, provider: provider}
}

// Execute enables or disables DNSSEC for the domain, optionally rotating its
// keys.
func (uc *UpdateArvanCloudDNSSecStatus) Execute(ctx context.Context, in UpdateArvanCloudDNSSecStatusInput) (*domain.ArvanCloudDNSSecStatus, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.DomainName == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		status, err := uc.provider.UpdateArvanCloudDNSSecStatus(ctx, in.Credentials, in.DomainName, in.Enable, in.Rotate)
		if err != nil {
			return nil, fmt.Errorf("updating DNSSEC status for arvancloud domain %q: %w", in.DomainName, err)
		}
		return json.Marshal(status)
	})
	if err != nil {
		return nil, err
	}

	var status domain.ArvanCloudDNSSecStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("decoding arvancloud DNSSEC status: %w", err)
	}
	return &status, nil
}

// --- Secondary DNS ---------------------------------------------------------

// GetArvanCloudSecondaryDNSInput identifies the domain whose Secondary DNS
// config to look up.
type GetArvanCloudSecondaryDNSInput = arvanCloudDomainScopedInput

// GetArvanCloudSecondaryDNS is a fast operation.
type GetArvanCloudSecondaryDNS struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudSecondaryDNS builds the use case from its ports.
func NewGetArvanCloudSecondaryDNS(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudSecondaryDNS {
	return &GetArvanCloudSecondaryDNS{queue: queue, provider: provider}
}

// Execute returns the domain's current Secondary DNS configuration.
func (uc *GetArvanCloudSecondaryDNS) Execute(ctx context.Context, in GetArvanCloudSecondaryDNSInput) (*domain.ArvanCloudSecondaryDNSConfig, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		cfg, err := uc.provider.GetArvanCloudSecondaryDNS(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("getting secondary DNS config for arvancloud domain %q: %w", in.DomainName, err)
		}
		return json.Marshal(cfg)
	})
	if err != nil {
		return nil, err
	}

	var cfg domain.ArvanCloudSecondaryDNSConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decoding arvancloud secondary DNS config: %w", err)
	}
	return &cfg, nil
}

// SetArvanCloudSecondaryDNSInput identifies the domain and the config to set.
type SetArvanCloudSecondaryDNSInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
	Config      domain.ArvanCloudSecondaryDNSConfig
}

// SetArvanCloudSecondaryDNS is a fast operation.
type SetArvanCloudSecondaryDNS struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewSetArvanCloudSecondaryDNS builds the use case from its ports.
func NewSetArvanCloudSecondaryDNS(queue ports.Queue, provider ports.ArvanCloudProvider) *SetArvanCloudSecondaryDNS {
	return &SetArvanCloudSecondaryDNS{queue: queue, provider: provider}
}

// Execute creates or replaces the domain's Secondary DNS configuration.
func (uc *SetArvanCloudSecondaryDNS) Execute(ctx context.Context, in SetArvanCloudSecondaryDNSInput) (*domain.ArvanCloudSecondaryDNSConfig, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.DomainName == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		cfg, err := uc.provider.SetArvanCloudSecondaryDNS(ctx, in.Credentials, in.DomainName, in.Config)
		if err != nil {
			return nil, fmt.Errorf("setting secondary DNS config for arvancloud domain %q: %w", in.DomainName, err)
		}
		return json.Marshal(cfg)
	})
	if err != nil {
		return nil, err
	}

	var cfg domain.ArvanCloudSecondaryDNSConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decoding arvancloud secondary DNS config: %w", err)
	}
	return &cfg, nil
}

// RemoveArvanCloudSecondaryDNSInput identifies the domain whose Secondary
// DNS config to remove.
type RemoveArvanCloudSecondaryDNSInput = arvanCloudDomainScopedInput

// RemoveArvanCloudSecondaryDNS is a fast operation. Removing a config the
// provider no longer has is treated as already done rather than an error,
// the same tolerant-delete contract as DeleteArvanCloudDomain.
type RemoveArvanCloudSecondaryDNS struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewRemoveArvanCloudSecondaryDNS builds the use case from its ports.
func NewRemoveArvanCloudSecondaryDNS(queue ports.Queue, provider ports.ArvanCloudProvider) *RemoveArvanCloudSecondaryDNS {
	return &RemoveArvanCloudSecondaryDNS{queue: queue, provider: provider}
}

// Execute removes the domain's Secondary DNS configuration, tolerating one
// that is already gone.
func (uc *RemoveArvanCloudSecondaryDNS) Execute(ctx context.Context, in RemoveArvanCloudSecondaryDNSInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.RemoveArvanCloudSecondaryDNS(ctx, in.Credentials, in.DomainName); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("removing secondary DNS config for arvancloud domain %q: %w", in.DomainName, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
