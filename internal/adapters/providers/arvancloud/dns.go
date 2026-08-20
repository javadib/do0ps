package arvancloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/javadib/do0ps/internal/core/domain"
)

// DNS records, DNSSEC and Secondary DNS (issue #63), wired to the real CDN
// API. Base paths are confirmed against docs/api-specs/arvancloud-cdn-4.0.yml's
// "DNS Management" and "Secondary DNS" tags, relative to domainPath (defined
// in domain.go) — i.e. https://napi.arvancloud.ir/cdn/4.0/domains/{domain}/...
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real ArvanCloud responses correctly. Nothing above
// the adapter boundary ever sees them — every method here translates to/from
// domain.ArvanCloudDNSRecord and friends. See
// internal/core/domain/arvancloud_dns.go's package comment for why this is a
// fresh domain type rather than a reuse of domain.DNSRecord.

const dnsRecordsPathSuffix = "/dns-records"

func dnsRecordsPath(domainName string) string {
	return domainPath(domainName) + dnsRecordsPathSuffix
}

func dnsRecordPath(domainName, id string) string {
	return dnsRecordsPath(domainName) + "/" + id
}

// --- Per-type "value" wire shapes -----------------------------------------
//
// The spec's DnsRecord request is a oneOf discriminated by "type", each
// variant pairing BaseDnsRecord with its own "value" schema (A-RecordValue,
// CNAME-RecordValue, ...). The response (DnsRecordGeneric) is looser —
// anyOf an array-valued shape (A/AAAA) or an object-valued shape (every
// other type) — so this adapter keeps "value" as raw JSON on the wire
// struct and only interprets it once the record's Type is known, in
// dnsRecordValueFromWire/dnsRecordValueToWire below.

// addressRecordValueWire mirrors A-RecordValue and AAAA-RecordValue, which
// are identical apart from the "ip" field's format (v4 vs v6) — a
// distinction JSON does not carry, so one Go struct covers both.
type addressRecordValueWire struct {
	IP             string `json:"ip"`
	Port           int    `json:"port,omitempty"`
	Weight         int    `json:"weight,omitempty"`
	OriginalWeight int    `json:"original_weight,omitempty"`
	Country        string `json:"country,omitempty"`
}

// cnameRecordValueWire mirrors CNAME-RecordValue.
type cnameRecordValueWire struct {
	Host       string `json:"host"`
	HostHeader string `json:"host_header,omitempty"`
	Port       int    `json:"port,omitempty"`
}

// anameRecordValueWire mirrors ANAME-RecordValue. Same shape as CNAME's, but
// keyed "location" instead of "host" — the one field name the two record
// types do not share.
type anameRecordValueWire struct {
	Location   string `json:"location"`
	HostHeader string `json:"host_header,omitempty"`
	Port       int    `json:"port,omitempty"`
}

// mxRecordValueWire mirrors MX-RecordValue.
type mxRecordValueWire struct {
	Host     string `json:"host"`
	Priority int    `json:"priority"`
}

// nsRecordValueWire mirrors NS-RecordValue.
type nsRecordValueWire struct {
	Host string `json:"host"`
}

// srvRecordValueWire mirrors SRV-RecordValue.
type srvRecordValueWire struct {
	Target   string `json:"target"`
	Port     int    `json:"port"`
	Weight   int    `json:"weight,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

// textRecordValueWire mirrors TXT-RecordValue, which SPF-RecordValue and
// DKIM-RecordValue are literally aliased to in the spec.
type textRecordValueWire struct {
	Text string `json:"text"`
}

// ptrRecordValueWire mirrors PTR-RecordValue — the one type with no required
// value field.
type ptrRecordValueWire struct {
	Domain string `json:"domain,omitempty"`
}

// tlsaRecordValueWire mirrors TLSA-RecordValue: all four fields are required
// per the spec despite their short declared max lengths.
type tlsaRecordValueWire struct {
	Usage        string `json:"usage"`
	Selector     string `json:"selector"`
	MatchingType string `json:"matching_type"`
	Certificate  string `json:"certificate"`
}

// caaRecordValueWire mirrors CAA-RecordValue.
type caaRecordValueWire struct {
	Value string `json:"value"`
	Tag   string `json:"tag"`
}

// firstDNSRecordValue returns values[0], or the zero value when values is
// empty. Every record type but A/AAAA carries exactly one value; app-layer
// validation (internal/core/app/arvancloud_dns.go) rejects an empty Values
// before this adapter is reached, so the zero-value fallback here only
// matters for a caller that bypasses that validation, e.g. a direct port
// test.
func firstDNSRecordValue(values []domain.ArvanCloudDNSRecordValue) domain.ArvanCloudDNSRecordValue {
	if len(values) == 0 {
		return domain.ArvanCloudDNSRecordValue{}
	}
	return values[0]
}

// dnsRecordValueToWire builds the JSON value to send for rec's "value"
// field: an array for A/AAAA, a single object for every other type.
func dnsRecordValueToWire(t domain.ArvanCloudDNSRecordType, values []domain.ArvanCloudDNSRecordValue) (any, error) {
	switch t {
	case domain.ArvanCloudDNSRecordTypeA, domain.ArvanCloudDNSRecordTypeAAAA:
		wire := make([]addressRecordValueWire, len(values))
		for i, v := range values {
			wire[i] = addressRecordValueWire{IP: v.IP, Port: v.Port, Weight: v.Weight, Country: v.Country}
		}
		return wire, nil
	case domain.ArvanCloudDNSRecordTypeCNAME:
		v := firstDNSRecordValue(values)
		return cnameRecordValueWire{Host: v.Host, HostHeader: v.HostHeader, Port: v.Port}, nil
	case domain.ArvanCloudDNSRecordTypeANAME:
		v := firstDNSRecordValue(values)
		return anameRecordValueWire{Location: v.Location, HostHeader: v.HostHeader, Port: v.Port}, nil
	case domain.ArvanCloudDNSRecordTypeMX:
		v := firstDNSRecordValue(values)
		return mxRecordValueWire{Host: v.Host, Priority: v.Priority}, nil
	case domain.ArvanCloudDNSRecordTypeNS:
		v := firstDNSRecordValue(values)
		return nsRecordValueWire{Host: v.Host}, nil
	case domain.ArvanCloudDNSRecordTypeSRV:
		v := firstDNSRecordValue(values)
		return srvRecordValueWire{Target: v.Target, Port: v.Port, Weight: v.Weight, Priority: v.Priority}, nil
	case domain.ArvanCloudDNSRecordTypeTXT, domain.ArvanCloudDNSRecordTypeSPF, domain.ArvanCloudDNSRecordTypeDKIM:
		v := firstDNSRecordValue(values)
		return textRecordValueWire{Text: v.Text}, nil
	case domain.ArvanCloudDNSRecordTypePTR:
		v := firstDNSRecordValue(values)
		return ptrRecordValueWire{Domain: v.Domain}, nil
	case domain.ArvanCloudDNSRecordTypeTLSA:
		v := firstDNSRecordValue(values)
		return tlsaRecordValueWire{Usage: v.Usage, Selector: v.Selector, MatchingType: v.MatchingType, Certificate: v.Certificate}, nil
	case domain.ArvanCloudDNSRecordTypeCAA:
		v := firstDNSRecordValue(values)
		return caaRecordValueWire{Value: v.CAAValue, Tag: v.Tag}, nil
	default:
		return nil, fmt.Errorf("record type %q is not one of the 13 ArvanCloud accepts: %w", t, domain.ErrInvalidInput)
	}
}

// dnsRecordValueFromWire parses a record's raw "value" field per its Type.
func dnsRecordValueFromWire(t domain.ArvanCloudDNSRecordType, raw json.RawMessage) ([]domain.ArvanCloudDNSRecordValue, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	switch t {
	case domain.ArvanCloudDNSRecordTypeA, domain.ArvanCloudDNSRecordTypeAAAA:
		var wire []addressRecordValueWire
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, fmt.Errorf("decoding %s record value: %w", t, err)
		}
		values := make([]domain.ArvanCloudDNSRecordValue, len(wire))
		for i, w := range wire {
			values[i] = domain.ArvanCloudDNSRecordValue{
				IP: w.IP, Port: w.Port, Weight: w.Weight, OriginalWeight: w.OriginalWeight, Country: w.Country,
			}
		}
		return values, nil
	case domain.ArvanCloudDNSRecordTypeCNAME:
		var w cnameRecordValueWire
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("decoding %s record value: %w", t, err)
		}
		return []domain.ArvanCloudDNSRecordValue{{Host: w.Host, HostHeader: w.HostHeader, Port: w.Port}}, nil
	case domain.ArvanCloudDNSRecordTypeANAME:
		var w anameRecordValueWire
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("decoding %s record value: %w", t, err)
		}
		return []domain.ArvanCloudDNSRecordValue{{Location: w.Location, HostHeader: w.HostHeader, Port: w.Port}}, nil
	case domain.ArvanCloudDNSRecordTypeMX:
		var w mxRecordValueWire
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("decoding %s record value: %w", t, err)
		}
		return []domain.ArvanCloudDNSRecordValue{{Host: w.Host, Priority: w.Priority}}, nil
	case domain.ArvanCloudDNSRecordTypeNS:
		var w nsRecordValueWire
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("decoding %s record value: %w", t, err)
		}
		return []domain.ArvanCloudDNSRecordValue{{Host: w.Host}}, nil
	case domain.ArvanCloudDNSRecordTypeSRV:
		var w srvRecordValueWire
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("decoding %s record value: %w", t, err)
		}
		return []domain.ArvanCloudDNSRecordValue{{Target: w.Target, Port: w.Port, Weight: w.Weight, Priority: w.Priority}}, nil
	case domain.ArvanCloudDNSRecordTypeTXT, domain.ArvanCloudDNSRecordTypeSPF, domain.ArvanCloudDNSRecordTypeDKIM:
		var w textRecordValueWire
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("decoding %s record value: %w", t, err)
		}
		return []domain.ArvanCloudDNSRecordValue{{Text: w.Text}}, nil
	case domain.ArvanCloudDNSRecordTypePTR:
		var w ptrRecordValueWire
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("decoding %s record value: %w", t, err)
		}
		return []domain.ArvanCloudDNSRecordValue{{Domain: w.Domain}}, nil
	case domain.ArvanCloudDNSRecordTypeTLSA:
		var w tlsaRecordValueWire
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("decoding %s record value: %w", t, err)
		}
		return []domain.ArvanCloudDNSRecordValue{{Usage: w.Usage, Selector: w.Selector, MatchingType: w.MatchingType, Certificate: w.Certificate}}, nil
	case domain.ArvanCloudDNSRecordTypeCAA:
		var w caaRecordValueWire
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("decoding %s record value: %w", t, err)
		}
		return []domain.ArvanCloudDNSRecordValue{{CAAValue: w.Value, Tag: w.Tag}}, nil
	default:
		return nil, fmt.Errorf("record type %q is not one of the 13 ArvanCloud accepts: %w", t, domain.ErrInvalidInput)
	}
}

// dnsRecordIPFilterModeWire mirrors DnsRecordIpFilterMode.
type dnsRecordIPFilterModeWire struct {
	Count     string `json:"count,omitempty"`
	Order     string `json:"order,omitempty"`
	GeoFilter string `json:"geo_filter,omitempty"`
}

// dnsRecordWire mirrors BaseDnsRecord plus the polymorphic "type"/"value"
// pair, kept raw here: only toDomainDNSRecord/toRecordWire below interpret
// "value", since its shape depends on "type" (this file's package comment).
type dnsRecordWire struct {
	ID            string                     `json:"id,omitempty"`
	Name          string                     `json:"name"`
	Type          string                     `json:"type"`
	TTL           int                        `json:"ttl,omitempty"`
	Cloud         bool                       `json:"cloud"`
	UpstreamHTTPS string                     `json:"upstream_https,omitempty"`
	IPFilterMode  *dnsRecordIPFilterModeWire `json:"ip_filter_mode,omitempty"`
	IsProtected   bool                       `json:"is_protected,omitempty"`
	Usage         []string                   `json:"usage,omitempty"`
	CreatedAt     string                     `json:"created_at,omitempty"`
	UpdatedAt     string                     `json:"updated_at,omitempty"`
	Value         json.RawMessage            `json:"value,omitempty"`
}

// toRecordWire builds the create/update request body for rec
// (dns-records.store/update, the DnsRecord oneOf schema).
func toRecordWire(rec domain.ArvanCloudDNSRecord) (dnsRecordWire, error) {
	value, err := dnsRecordValueToWire(rec.Type, rec.Values)
	if err != nil {
		return dnsRecordWire{}, err
	}
	encodedValue, err := json.Marshal(value)
	if err != nil {
		return dnsRecordWire{}, fmt.Errorf("encoding %s record value: %w", rec.Type, err)
	}

	wire := dnsRecordWire{
		Name:          rec.Name,
		Type:          rec.Type.String(),
		TTL:           rec.TTL,
		Cloud:         rec.Cloud,
		UpstreamHTTPS: rec.UpstreamHTTPS,
		Value:         encodedValue,
	}
	if rec.IPFilterMode != (domain.ArvanCloudDNSRecordIPFilterMode{}) {
		wire.IPFilterMode = &dnsRecordIPFilterModeWire{
			Count: rec.IPFilterMode.Count, Order: rec.IPFilterMode.Order, GeoFilter: rec.IPFilterMode.GeoFilter,
		}
	}
	return wire, nil
}

// toDomainDNSRecord translates a DnsRecordGeneric payload into the port's
// domain type.
func toDomainDNSRecord(w dnsRecordWire) (domain.ArvanCloudDNSRecord, error) {
	t, err := domain.ParseArvanCloudDNSRecordType(w.Type)
	if err != nil {
		return domain.ArvanCloudDNSRecord{}, fmt.Errorf("decoding dns record %q: type %q: %w", w.ID, w.Type, err)
	}
	values, err := dnsRecordValueFromWire(t, w.Value)
	if err != nil {
		return domain.ArvanCloudDNSRecord{}, fmt.Errorf("decoding dns record %q: %w", w.ID, err)
	}

	rec := domain.ArvanCloudDNSRecord{
		ID:            w.ID,
		Name:          w.Name,
		Type:          t,
		TTL:           w.TTL,
		Cloud:         w.Cloud,
		UpstreamHTTPS: w.UpstreamHTTPS,
		IsProtected:   w.IsProtected,
		Usage:         w.Usage,
		Values:        values,
		CreatedAt:     w.CreatedAt,
		UpdatedAt:     w.UpdatedAt,
	}
	if w.IPFilterMode != nil {
		rec.IPFilterMode = domain.ArvanCloudDNSRecordIPFilterMode{
			Count: w.IPFilterMode.Count, Order: w.IPFilterMode.Order, GeoFilter: w.IPFilterMode.GeoFilter,
		}
	}
	return rec, nil
}

// ListArvanCloudDNSRecords returns every DNS record of the given domain,
// unfiltered — the spec's optional search/type/page/per_page query
// parameters are not exposed by this port; a caller that needs to narrow the
// list filters the result itself, mirroring ListDomains in domain.go.
func (p *Provider) ListArvanCloudDNSRecords(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudDNSRecord, error) {
	var items []dnsRecordWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, dnsRecordsPath(domainName), nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud dns records of domain %q: %w", domainName, err)
	}

	records := make([]domain.ArvanCloudDNSRecord, len(items))
	for i, w := range items {
		rec, err := toDomainDNSRecord(w)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud dns records of domain %q: %w", domainName, err)
		}
		records[i] = rec
	}
	return records, nil
}

// CreateArvanCloudDNSRecord adds a new DNS record to the given domain.
func (p *Provider) CreateArvanCloudDNSRecord(ctx context.Context, creds domain.ProviderCredentials, domainName string, rec domain.ArvanCloudDNSRecord) (*domain.ArvanCloudDNSRecord, error) {
	body, err := toRecordWire(rec)
	if err != nil {
		return nil, fmt.Errorf("creating arvancloud dns record %q in domain %q: %w", rec.Name, domainName, err)
	}

	var wire dnsRecordWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, dnsRecordsPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud dns record %q in domain %q: %w", rec.Name, domainName, err)
	}
	created, err := toDomainDNSRecord(wire)
	if err != nil {
		return nil, fmt.Errorf("creating arvancloud dns record %q in domain %q: %w", rec.Name, domainName, err)
	}
	return &created, nil
}

// GetArvanCloudDNSRecord returns a single DNS record by its per-record UUID.
func (p *Provider) GetArvanCloudDNSRecord(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudDNSRecord, error) {
	var wire dnsRecordWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, dnsRecordPath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud dns record %q of domain %q: %w", id, domainName, err)
	}
	found, err := toDomainDNSRecord(wire)
	if err != nil {
		return nil, fmt.Errorf("getting arvancloud dns record %q of domain %q: %w", id, domainName, err)
	}
	return &found, nil
}

// UpdateArvanCloudDNSRecord replaces a DNS record's configuration by id.
func (p *Provider) UpdateArvanCloudDNSRecord(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rec domain.ArvanCloudDNSRecord) (*domain.ArvanCloudDNSRecord, error) {
	body, err := toRecordWire(rec)
	if err != nil {
		return nil, fmt.Errorf("updating arvancloud dns record %q of domain %q: %w", id, domainName, err)
	}

	var wire dnsRecordWire
	if err := p.client.doJSON(ctx, creds, http.MethodPut, dnsRecordPath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud dns record %q of domain %q: %w", id, domainName, err)
	}
	updated, err := toDomainDNSRecord(wire)
	if err != nil {
		return nil, fmt.Errorf("updating arvancloud dns record %q of domain %q: %w", id, domainName, err)
	}
	return &updated, nil
}

// DeleteArvanCloudDNSRecord removes a DNS record by id.
func (p *Provider) DeleteArvanCloudDNSRecord(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, dnsRecordPath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud dns record %q of domain %q: %w", id, domainName, err)
	}
	return nil
}

// dnsRecordCloudWire mirrors DnsRecordCloud, the request body of the cloud
// toggle endpoint.
type dnsRecordCloudWire struct {
	Cloud bool `json:"cloud"`
}

// ToggleArvanCloudDNSRecordCloud flips one record's CDN-proxy status.
func (p *Provider) ToggleArvanCloudDNSRecordCloud(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, cloud bool) (*domain.ArvanCloudDNSRecord, error) {
	body := dnsRecordCloudWire{Cloud: cloud}
	var wire dnsRecordWire
	if err := p.client.doJSON(ctx, creds, http.MethodPut, dnsRecordPath(domainName, id)+"/cloud", body, &wire); err != nil {
		return nil, fmt.Errorf("toggling cloud status of arvancloud dns record %q of domain %q: %w", id, domainName, err)
	}
	updated, err := toDomainDNSRecord(wire)
	if err != nil {
		return nil, fmt.Errorf("toggling cloud status of arvancloud dns record %q of domain %q: %w", id, domainName, err)
	}
	return &updated, nil
}

// ImportArvanCloudDNSRecords bulk-creates records from a BIND zone file
// (dns-records.import, multipart/form-data — not JSON, unlike every other
// method in this file).
func (p *Provider) ImportArvanCloudDNSRecords(ctx context.Context, creds domain.ProviderCredentials, domainName string, zoneFile []byte) error {
	if err := p.client.doMultipart(ctx, creds, http.MethodPost, dnsRecordsPath(domainName)+"/import", "zone.txt", zoneFile, nil); err != nil {
		return fmt.Errorf("importing arvancloud dns records for domain %q: %w", domainName, err)
	}
	return nil
}

// ExportArvanCloudDNSRecords returns the domain's DNS records as a BIND zone
// file (dns-records.export, text/plain — not JSON, unlike every other
// method in this file).
func (p *Provider) ExportArvanCloudDNSRecords(ctx context.Context, creds domain.ProviderCredentials, domainName string) (string, error) {
	data, err := p.client.doRawGET(ctx, creds, dnsRecordsPath(domainName)+"/export", "text/plain")
	if err != nil {
		return "", fmt.Errorf("exporting arvancloud dns records for domain %q: %w", domainName, err)
	}
	return string(data), nil
}

// dnsSecWire mirrors DnsSec, the response of both DNSSEC endpoints.
type dnsSecWire struct {
	Enabled bool   `json:"enabled"`
	DS      string `json:"ds"`
}

func toDomainDNSSecStatus(w dnsSecWire) *domain.ArvanCloudDNSSecStatus {
	return &domain.ArvanCloudDNSSecStatus{Enabled: w.Enabled, DS: w.DS}
}

// GetArvanCloudDNSSecStatus returns a domain's current DNSSEC status.
func (p *Provider) GetArvanCloudDNSSecStatus(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDNSSecStatus, error) {
	var wire dnsSecWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, dnsRecordsPath(domainName)+"/dnssec", nil, &wire); err != nil {
		return nil, fmt.Errorf("getting DNSSEC status for arvancloud domain %q: %w", domainName, err)
	}
	return toDomainDNSSecStatus(wire), nil
}

// dnsSecUpdateWire mirrors DnsSecStatus, the request body of the DNSSEC
// update endpoint.
type dnsSecUpdateWire struct {
	Enable bool `json:"enable"`
	Rotate bool `json:"rotate"`
}

// UpdateArvanCloudDNSSecStatus enables or disables DNSSEC for a domain,
// optionally rotating its keys.
func (p *Provider) UpdateArvanCloudDNSSecStatus(ctx context.Context, creds domain.ProviderCredentials, domainName string, enable, rotate bool) (*domain.ArvanCloudDNSSecStatus, error) {
	body := dnsSecUpdateWire{Enable: enable, Rotate: rotate}
	var wire dnsSecWire
	if err := p.client.doJSON(ctx, creds, http.MethodPut, dnsRecordsPath(domainName)+"/dnssec/actions", body, &wire); err != nil {
		return nil, fmt.Errorf("updating DNSSEC status for arvancloud domain %q: %w", domainName, err)
	}
	return toDomainDNSSecStatus(wire), nil
}

const secondaryDNSPathSuffix = "/secondary-dns"

// secondaryDNSSkippedRecordWire mirrors one entry of SecondaryDNSData's
// "errors.skipped_records".
type secondaryDNSSkippedRecordWire struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// secondaryDNSErrorsWire mirrors SecondaryDNSData's read-only "errors"
// object.
type secondaryDNSErrorsWire struct {
	Error          string                          `json:"error"`
	SkippedRecords []secondaryDNSSkippedRecordWire `json:"skipped_records"`
}

// secondaryDNSWire mirrors SecondaryDNSData, both the get/set request and
// response shape.
type secondaryDNSWire struct {
	Status     bool                    `json:"status"`
	Nameserver string                  `json:"nameserver"`
	SOASerial  string                  `json:"soa_serial,omitempty"`
	Errors     *secondaryDNSErrorsWire `json:"errors,omitempty"`
	CreatedAt  string                  `json:"created_at,omitempty"`
	UpdatedAt  string                  `json:"updated_at,omitempty"`
}

func toDomainSecondaryDNS(w secondaryDNSWire) domain.ArvanCloudSecondaryDNSConfig {
	cfg := domain.ArvanCloudSecondaryDNSConfig{
		Status: w.Status, Nameserver: w.Nameserver, SOASerial: w.SOASerial,
		CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}
	if w.Errors != nil {
		cfg.ErrorMessage = w.Errors.Error
		cfg.SkippedRecords = make([]domain.ArvanCloudSecondaryDNSSkippedRecord, len(w.Errors.SkippedRecords))
		for i, s := range w.Errors.SkippedRecords {
			cfg.SkippedRecords[i] = domain.ArvanCloudSecondaryDNSSkippedRecord{Name: s.Name, Type: s.Type, Value: s.Value}
		}
	}
	return cfg
}

// GetArvanCloudSecondaryDNS returns a domain's current Secondary DNS
// configuration.
func (p *Provider) GetArvanCloudSecondaryDNS(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudSecondaryDNSConfig, error) {
	var wire secondaryDNSWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, domainPath(domainName)+secondaryDNSPathSuffix, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting secondary DNS config for arvancloud domain %q: %w", domainName, err)
	}
	cfg := toDomainSecondaryDNS(wire)
	return &cfg, nil
}

// SetArvanCloudSecondaryDNS creates or replaces a domain's Secondary DNS
// configuration.
func (p *Provider) SetArvanCloudSecondaryDNS(ctx context.Context, creds domain.ProviderCredentials, domainName string, config domain.ArvanCloudSecondaryDNSConfig) (*domain.ArvanCloudSecondaryDNSConfig, error) {
	body := secondaryDNSWire{Status: config.Status, Nameserver: config.Nameserver}
	var wire secondaryDNSWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+secondaryDNSPathSuffix, body, &wire); err != nil {
		return nil, fmt.Errorf("setting secondary DNS config for arvancloud domain %q: %w", domainName, err)
	}
	cfg := toDomainSecondaryDNS(wire)
	return &cfg, nil
}

// RemoveArvanCloudSecondaryDNS deletes a domain's Secondary DNS
// configuration. The endpoint's successful response is 204 No Content, so
// there is nothing to translate but the error.
func (p *Provider) RemoveArvanCloudSecondaryDNS(ctx context.Context, creds domain.ProviderCredentials, domainName string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, domainPath(domainName)+secondaryDNSPathSuffix, nil, nil); err != nil {
		return fmt.Errorf("removing secondary DNS config for arvancloud domain %q: %w", domainName, err)
	}
	return nil
}
