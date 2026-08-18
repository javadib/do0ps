package parspack

import (
	"context"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// CDN zone management and DNS records, wired to the real CDN API (issue #19).
// Base path is confirmed against docs/api-specs/cdn-api.openapi's
// Service/Order/Dns tags, relative to Client.cdnBaseURL, i.e.
// https://my.parspack.com/cdnapi/external/api/v1/zones and friends.
//
// The wire types below mirror that spec's request/response shapes exactly
// (field names and JSON tags) so this adapter decodes real Parspack CDN
// responses correctly. Nothing above the adapter boundary ever sees these —
// every method here translates to/from internal/core/domain types.

const (
	zonesBasePath      = "external/api/v1/zones"
	ordersBasePath     = "external/api/v1/orders"
	createOrderPath    = "external/api/v2/orders"
	dnsRecordsBasePath = "external/api/v2/zones"
)

// zoneListItemWire is one entry of GET /zones.
type zoneListItemWire struct {
	ID           string `json:"id"`
	UUID         string `json:"uuid"`
	TargetDomain string `json:"target_domain"`
	Status       string `json:"status"`
	Plan         string `json:"plan"`
	ExpireAt     string `json:"expire_at"`
}

// zoneDetailWire is the response of GET /zones/{zone_uuid}. It has no
// id/uuid of its own in the payload — the caller already supplied zone_uuid
// on the path — so toDomainZone is given it separately.
type zoneDetailWire struct {
	TargetDomain  string `json:"target_domain"`
	Status        string `json:"status"`
	Plan          string `json:"plan"`
	Proxy         bool   `json:"proxy"`
	NSStatus      bool   `json:"ns_status"`
	BillingCycle  string `json:"billing_cycle"`
	RemainingDays int    `json:"remaining_days"`
}

func toDomainZoneFromList(w zoneListItemWire) domain.CDNZone {
	return domain.CDNZone{
		UUID:     w.UUID,
		ID:       w.ID,
		Domain:   w.TargetDomain,
		Status:   w.Status,
		Plan:     w.Plan,
		ExpireAt: w.ExpireAt,
	}
}

func toDomainZoneFromDetail(zoneUUID string, w zoneDetailWire) domain.CDNZone {
	return domain.CDNZone{
		UUID:          zoneUUID,
		Domain:        w.TargetDomain,
		Status:        w.Status,
		Plan:          w.Plan,
		BillingCycle:  w.BillingCycle,
		Proxy:         w.Proxy,
		NSStatus:      w.NSStatus,
		RemainingDays: w.RemainingDays,
	}
}

// ListCDNZones returns every CDN zone visible to the credentials.
func (c *Client) ListCDNZones(ctx context.Context, creds domain.ProviderCredentials) ([]domain.CDNZone, error) {
	var items []zoneListItemWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath, nil, &items); err != nil {
		return nil, fmt.Errorf("list CDN zones: %w", err)
	}

	zones := make([]domain.CDNZone, len(items))
	for i := range items {
		zones[i] = toDomainZoneFromList(items[i])
	}
	return zones, nil
}

// GetCDNZone returns a single zone by UUID.
func (c *Client) GetCDNZone(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNZone, error) {
	var detail zoneDetailWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID, nil, &detail); err != nil {
		return nil, fmt.Errorf("get CDN zone %s: %w", zoneUUID, err)
	}
	zone := toDomainZoneFromDetail(zoneUUID, detail)
	return &zone, nil
}

// DeleteCDNZone removes a zone by UUID.
func (c *Client) DeleteCDNZone(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", zonesBasePath+"/"+zoneUUID, nil, nil); err != nil {
		return fmt.Errorf("delete CDN zone %s: %w", zoneUUID, err)
	}
	return nil
}

// packagePeriodWire and packageWire mirror GET /orders/packages.
type packagePeriodWire struct {
	Title string `json:"title"`
}

type packagePricesWire struct {
	Currency     string `json:"currency"`
	Monthly      int    `json:"monthly"`
	Quarterly    int    `json:"quarterly"`
	Semiannually int    `json:"semiannually"`
	Annually     int    `json:"annually"`
}

type packageWire struct {
	Plan   string            `json:"plan"`
	Prices packagePricesWire `json:"prices"`
}

type orderPackagesWire struct {
	PackagePeriods []packagePeriodWire `json:"package_periods"`
	Packages       []packageWire       `json:"packages"`
}

// ListCDNZonePlans returns the plans and pricing available for a new zone.
func (c *Client) ListCDNZonePlans(ctx context.Context, creds domain.ProviderCredentials) ([]domain.CDNZonePlanPricing, error) {
	var wire orderPackagesWire
	if err := c.doCDNJSON(ctx, creds, "GET", ordersBasePath+"/packages", nil, &wire); err != nil {
		return nil, fmt.Errorf("list CDN zone plans: %w", err)
	}

	plans := make([]domain.CDNZonePlanPricing, len(wire.Packages))
	for i, p := range wire.Packages {
		plans[i] = domain.CDNZonePlanPricing{
			Plan:         p.Plan,
			Currency:     p.Prices.Currency,
			Monthly:      p.Prices.Monthly,
			Quarterly:    p.Prices.Quarterly,
			Semiannually: p.Prices.Semiannually,
			Annually:     p.Prices.Annually,
		}
	}
	return plans, nil
}

// nsRecordsWire mirrors GET /orders/{zone_uuid}/ns-records and the
// "ns_records" field of the order-create response.
type nsRecordsWire struct {
	NS1       string   `json:"ns1"`
	NS2       string   `json:"ns2"`
	NS3       string   `json:"ns3"`
	NS4       string   `json:"ns4"`
	CurrentNS []string `json:"current_ns"`
}

func toDomainNameservers(w nsRecordsWire) domain.NameserverRecords {
	return domain.NameserverRecords{
		NS1: w.NS1, NS2: w.NS2, NS3: w.NS3, NS4: w.NS4,
		CurrentNS: w.CurrentNS,
	}
}

// GetNameserverRecords returns the nameservers the zone's domain registrar
// must be pointed at.
func (c *Client) GetNameserverRecords(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.NameserverRecords, error) {
	var wire nsRecordsWire
	if err := c.doCDNJSON(ctx, creds, "GET", ordersBasePath+"/"+zoneUUID+"/ns-records", nil, &wire); err != nil {
		return nil, fmt.Errorf("get nameserver records for zone %s: %w", zoneUUID, err)
	}
	ns := toDomainNameservers(wire)
	return &ns, nil
}

// dnsRecordValueWire mirrors the objects nested under a dns-records
// request's "record"/"records" field and a list response's "records" array.
type dnsRecordValueWire struct {
	Content  string `json:"content"`
	Port     int    `json:"port,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Flags    int    `json:"flags,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

func toWireValues(values []domain.DNSRecordValue) []dnsRecordValueWire {
	wire := make([]dnsRecordValueWire, len(values))
	for i, v := range values {
		wire[i] = dnsRecordValueWire{
			Content: v.Content, Port: v.Port, Weight: v.Weight,
			Priority: v.Priority, Flags: v.Flags, Tag: v.Tag, Disabled: v.Disabled,
		}
	}
	return wire
}

func toDomainValues(wire []dnsRecordValueWire) []domain.DNSRecordValue {
	values := make([]domain.DNSRecordValue, len(wire))
	for i, w := range wire {
		values[i] = domain.DNSRecordValue{
			Content: w.Content, Port: w.Port, Weight: w.Weight,
			Priority: w.Priority, Flags: w.Flags, Tag: w.Tag, Disabled: w.Disabled,
		}
	}
	return values
}

// dnsRecordListItemWire is one entry of GET .../dns-records — grouped by
// host+type, so a single entry can hold more than one value (e.g. multiple
// NS records for the zone apex).
type dnsRecordListItemWire struct {
	Zone    string               `json:"zone"`
	Host    string               `json:"host"`
	Type    string               `json:"type"`
	TTL     int                  `json:"ttl"`
	Proxy   string               `json:"proxy"`
	Records []dnsRecordValueWire `json:"records"`
}

func toDomainRecord(zoneUUID string, w dnsRecordListItemWire) domain.DNSRecord {
	recType, _ := domain.ParseDNSRecordType(w.Type)
	proxy, _ := domain.ParseDNSRecordProxy(w.Proxy)
	return domain.DNSRecord{
		ZoneUUID: zoneUUID,
		Host:     w.Host,
		Type:     recType,
		TTL:      w.TTL,
		Proxy:    proxy,
		Values:   toDomainValues(w.Records),
	}
}

// ListDNSRecords returns every record of one zone.
func (c *Client) ListDNSRecords(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.DNSRecord, error) {
	var items []dnsRecordListItemWire
	if err := c.doCDNJSON(ctx, creds, "GET", dnsRecordsBasePath+"/"+zoneUUID+"/dns-records", nil, &items); err != nil {
		return nil, fmt.Errorf("list DNS records of zone %s: %w", zoneUUID, err)
	}

	records := make([]domain.DNSRecord, len(items))
	for i := range items {
		records[i] = toDomainRecord(zoneUUID, items[i])
	}
	return records, nil
}

// dnsRecordCreateRequest is the body of POST .../dns-records. "record" (a
// single value object) is confirmed by the spec; a create with the same
// host+type as an existing record appends to it rather than erroring.
type dnsRecordCreateRequest struct {
	Host          string             `json:"host"`
	Type          string             `json:"type"`
	TTL           int                `json:"ttl"`
	Proxy         string             `json:"proxy"`
	LoadBalanceID string             `json:"load_balance_id,omitempty"`
	Record        dnsRecordValueWire `json:"record"`
}

// CreateDNSRecord adds one value to a zone under a host+type. The provider's
// create endpoint accepts exactly one value per call — send CreateDNSRecord
// again with the same host+type to add another (e.g. a second NS record).
func (c *Client) CreateDNSRecord(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rec domain.DNSRecord) (*domain.DNSRecord, error) {
	if len(rec.Values) != 1 {
		return nil, fmt.Errorf("create %s record %q: exactly one value is required per call: %w", rec.Type, rec.Host, domain.ErrInvalidInput)
	}

	reqBody := dnsRecordCreateRequest{
		Host: rec.Host, Type: rec.Type.String(), TTL: rec.TTL, Proxy: rec.Proxy.String(),
		LoadBalanceID: rec.LoadBalanceID, Record: toWireValues(rec.Values)[0],
	}

	if err := c.doCDNJSON(ctx, creds, "POST", dnsRecordsBasePath+"/"+zoneUUID+"/dns-records", reqBody, nil); err != nil {
		return nil, fmt.Errorf("create %s record %q in zone %s: %w", rec.Type, rec.Host, zoneUUID, err)
	}

	created := rec
	created.ZoneUUID = zoneUUID
	return &created, nil
}

// dnsRecordUpdateRequest is the body of PUT .../dns-records. The spec
// documents "records" as the same value-object shape as create's "record",
// scoped by host+type — content, TTL, proxy and load balancer can change,
// but records cannot be added or removed this way.
type dnsRecordUpdateRequest struct {
	Host          string             `json:"host"`
	Type          string             `json:"type"`
	TTL           int                `json:"ttl"`
	Proxy         string             `json:"proxy"`
	LoadBalanceID string             `json:"load_balance_id,omitempty"`
	Records       dnsRecordValueWire `json:"records"`
}

// UpdateDNSRecord updates the single value under a zone's host+type.
func (c *Client) UpdateDNSRecord(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rec domain.DNSRecord) (*domain.DNSRecord, error) {
	if len(rec.Values) != 1 {
		return nil, fmt.Errorf("update %s record %q: exactly one value is required per call: %w", rec.Type, rec.Host, domain.ErrInvalidInput)
	}

	reqBody := dnsRecordUpdateRequest{
		Host: rec.Host, Type: rec.Type.String(), TTL: rec.TTL, Proxy: rec.Proxy.String(),
		LoadBalanceID: rec.LoadBalanceID, Records: toWireValues(rec.Values)[0],
	}

	if err := c.doCDNJSON(ctx, creds, "PUT", dnsRecordsBasePath+"/"+zoneUUID+"/dns-records", reqBody, nil); err != nil {
		return nil, fmt.Errorf("update %s record %q in zone %s: %w", rec.Type, rec.Host, zoneUUID, err)
	}

	updated := rec
	updated.ZoneUUID = zoneUUID
	return &updated, nil
}

// dnsRecordDeleteRequest is the body of DELETE .../dns-records. A nil Record
// deletes every value under the host+type; a set one deletes only the
// matching value (a host+type can hold more than one, e.g. multiple NS
// records).
type dnsRecordDeleteRequest struct {
	Host   string              `json:"host"`
	Type   string              `json:"type"`
	Record *dnsRecordValueWire `json:"record,omitempty"`
}

// DeleteDNSRecord removes a record (or one of its values) from a zone.
func (c *Client) DeleteDNSRecord(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, host string, recordType domain.DNSRecordType, content string) error {
	reqBody := dnsRecordDeleteRequest{Host: host, Type: recordType.String()}
	if content != "" {
		reqBody.Record = &dnsRecordValueWire{Content: content}
	}

	if err := c.doCDNJSON(ctx, creds, "DELETE", dnsRecordsBasePath+"/"+zoneUUID+"/dns-records", reqBody, nil); err != nil {
		return fmt.Errorf("delete %s record %q in zone %s: %w", recordType, host, zoneUUID, err)
	}
	return nil
}

// orderCreateRequest is the body of POST /external/api/v2/orders.
type orderCreateRequest struct {
	Domain        string                   `json:"domain"`
	Plan          string                   `json:"plan"`
	BillingCycle  string                   `json:"billing_cycle"`
	PromotionCode string                   `json:"promotion_code,omitempty"`
	Records       []dnsRecordOrderItemWire `json:"records"`
}

// dnsRecordOrderItemWire mirrors POST /orders' nested "records" array item,
// which (unlike a standalone create-record call) accepts multiple values at
// once under "records" — one zone-creation call can seed several values for
// the same host+type in one shot.
type dnsRecordOrderItemWire struct {
	Host    string               `json:"host"`
	Type    string               `json:"type"`
	TTL     int                  `json:"ttl"`
	Proxy   string               `json:"proxy"`
	Records []dnsRecordValueWire `json:"records"`
}

// orderCreateResponse is the "data" object of a successful order response.
// Status differs by outcome (active/payment_pending/setup_required, per the
// spec's three documented 201 examples); NSRecords is only present when the
// zone activated immediately.
type orderCreateResponse struct {
	ZoneID    string         `json:"zone_id"`
	Status    string         `json:"status"`
	NSRecords *nsRecordsWire `json:"ns_records,omitempty"`
	Warnings  []string       `json:"warnings"`
}

// CreateCDNZone onboards a new domain onto the CDN. This is a single
// synchronous call: the order endpoint returns a final zone_id and status in
// its response body, so unlike CreateServer there is no further
// "provisioning" state for this adapter to poll (ports.ParspackProvider,
// AGENTS.md 4.3).
func (c *Client) CreateCDNZone(ctx context.Context, creds domain.ProviderCredentials, spec domain.CDNZoneSpec) (*domain.CDNZone, error) {
	// make(..., len(spec.Records)) is never nil, even for zero records: the
	// "records" field is required by the order endpoint, so an empty slice
	// (not an omitted/null field) must always be sent.
	records := make([]dnsRecordOrderItemWire, len(spec.Records))
	for i, rec := range spec.Records {
		records[i] = dnsRecordOrderItemWire{
			Host: rec.Host, Type: rec.Type.String(), TTL: rec.TTL, Proxy: rec.Proxy.String(),
			Records: toWireValues(rec.Values),
		}
	}

	reqBody := orderCreateRequest{
		Domain: spec.Domain, Plan: spec.Plan, BillingCycle: spec.BillingCycle,
		PromotionCode: spec.PromotionCode, Records: records,
	}

	var resp orderCreateResponse
	if err := c.doCDNJSON(ctx, creds, "POST", createOrderPath, reqBody, &resp); err != nil {
		return nil, fmt.Errorf("creating CDN zone %q: %w", spec.Domain, err)
	}

	return &domain.CDNZone{
		UUID:         resp.ZoneID,
		Domain:       spec.Domain,
		Status:       resp.Status,
		Plan:         spec.Plan,
		BillingCycle: spec.BillingCycle,
	}, nil
}
