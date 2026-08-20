package arvancloud

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Domain lifecycle, NS Setup, CNAME Setup and the remaining single-domain
// actions (issue #62), wired to the real CDN API. Base path is confirmed
// against docs/api-specs/arvancloud-cdn-4.0.yml's Domain tag, relative to
// Client.baseURL, i.e. https://napi.arvancloud.ir/cdn/4.0/domains and
// friends.
//
// The wire types below mirror that spec's request/response shapes exactly
// (field names and JSON tags) so this adapter decodes real ArvanCloud
// responses correctly. Nothing above the adapter boundary ever sees these —
// every method here translates to/from internal/core/domain types.

const domainsBasePath = "domains"

// domainPath builds the /domains/{domain}... path segment every sub-resource
// method below starts from. ArvanCloud addresses a domain by its name, not a
// UUID (ports.ArvanCloudProvider), so domainName is inserted as-is: it is a
// DNS hostname, never a value that needs URL escaping to stay a valid path
// segment.
func domainPath(domainName string) string {
	return domainsBasePath + "/" + domainName
}

// domainWire mirrors the Domain schema: the shape returned by list, create,
// get, and every CNAME Setup endpoint, all of which echo the full resource.
type domainWire struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"` // deprecated fallback, see domain.ArvanCloudDomain's doc comment

	PlanLevel   int      `json:"plan_level"`
	Type        string   `json:"type"`
	Status      string   `json:"status"`
	NSKeys      []string `json:"ns_keys"`
	CurrentNS   []string `json:"current_ns"`
	CnameTarget string   `json:"cname_target"`
	CustomCname string   `json:"custom_cname"`
	DNSCloud    bool     `json:"dns_cloud"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// toDomainDomain translates a full Domain payload into the port's domain
// type. domainName is the name the caller already knows (the path segment
// every method here is scoped to); it fills the result only when the
// response carries neither "name" nor its deprecated "domain" alias, which
// the spec allows for endpoints that echo just a partial resource.
func toDomainDomain(domainName string, w domainWire) domain.ArvanCloudDomain {
	name := w.Name
	if name == "" {
		name = w.Domain
	}
	if name == "" {
		name = domainName
	}
	return domain.ArvanCloudDomain{
		ID:          w.ID,
		Name:        name,
		PlanLevel:   domain.ArvanCloudPlanLevel(w.PlanLevel),
		Type:        w.Type,
		Status:      w.Status,
		NSKeys:      w.NSKeys,
		CurrentNS:   w.CurrentNS,
		CnameTarget: w.CnameTarget,
		CustomCname: w.CustomCname,
		DNSCloud:    w.DNSCloud,
		CreatedAt:   w.CreatedAt,
		UpdatedAt:   w.UpdatedAt,
	}
}

// ListDomains returns every domain onboarded onto the CDN under the given
// credentials, unfiltered — the spec's optional search/plan/status query
// parameters are not exposed by this port; a caller that needs to narrow the
// list filters the result itself.
func (p *Provider) ListDomains(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudDomain, error) {
	var items []domainWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, domainsBasePath, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud domains: %w", err)
	}

	domains := make([]domain.ArvanCloudDomain, len(items))
	for i := range items {
		domains[i] = toDomainDomain("", items[i])
	}
	return domains, nil
}

// domainStoreWire is the body of POST /domains/dns-service. domain_type is
// omitted when spec left it unset, so the CDN API's own documented default
// ("full") applies instead of this adapter guessing at one. plan_level is
// always sent: domain.ArvanCloudPlanLevel's zero value is ArvanCloudPlanTraffic,
// itself a real selectable plan, so there is no "unset" state to distinguish
// it from at this layer. import_dns_records is likewise always sent
// explicitly: ports.ArvanCloudProvider.CreateDomain always receives a
// resolved true/false (see domain.ArvanCloudDomainSpec), so there is nothing
// left here to default either.
type domainStoreWire struct {
	Domain           string `json:"domain"`
	DomainType       string `json:"domain_type,omitempty"`
	PlanLevel        int    `json:"plan_level"`
	ImportDNSRecords bool   `json:"import_dns_records"`
}

// CreateDomain onboards a new domain onto ArvanCloud's CDN. This is a single
// synchronous call: the store endpoint returns the created domain resource
// directly, so there is no further "provisioning" state for this adapter to
// poll (ports.ArvanCloudProvider, AGENTS.md 4.3).
func (p *Provider) CreateDomain(ctx context.Context, creds domain.ProviderCredentials, spec domain.ArvanCloudDomainSpec) (*domain.ArvanCloudDomain, error) {
	body := domainStoreWire{
		Domain:           spec.Name,
		DomainType:       spec.DomainType,
		PlanLevel:        int(spec.PlanLevel),
		ImportDNSRecords: spec.ImportDNSRecords,
	}

	var wire domainWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainsBasePath+"/dns-service", body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud domain %q: %w", spec.Name, err)
	}

	created := toDomainDomain(spec.Name, wire)
	return &created, nil
}

// GetDomain returns a single domain by name.
func (p *Provider) GetDomain(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	var wire domainWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, domainPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud domain %q: %w", domainName, err)
	}
	found := toDomainDomain(domainName, wire)
	return &found, nil
}

// DeleteDomain removes a domain by name. The destroy endpoint requires the
// domain's account-side UUID as a query parameter in addition to the name
// already in the path (DELETE /domains/{domain}?id=..., a required parameter
// per the spec), so this method resolves the id via GetDomain first. That
// lookup has a convenient side effect: a domain the provider no longer has is
// discovered here and reported as domain.ErrNotFound before any DELETE is
// attempted, giving DeleteDomain the same tolerant-delete contract as
// DeleteServer/DeleteCDNZone (ports.ArvanCloudProvider, AGENTS.md 4.4).
func (p *Provider) DeleteDomain(ctx context.Context, creds domain.ProviderCredentials, domainName string) error {
	existing, err := p.GetDomain(ctx, creds, domainName)
	if err != nil {
		return fmt.Errorf("deleting arvancloud domain %q: %w", domainName, err)
	}

	path := domainPath(domainName) + "?id=" + url.QueryEscape(existing.ID)
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud domain %q: %w", domainName, err)
	}
	return nil
}

// nsKeysWire mirrors NsKeys (the request body of the set endpoint) and
// NsKeysResponse's "data" object (the response of set, reset and
// use-optional-keys) — both are shaped as {"ns_keys": [...]}.
type nsKeysWire struct {
	NSKeys []string `json:"ns_keys"`
}

// SetNSKeys sets the custom NS records a "full" domain's registrar must be
// pointed at.
func (p *Provider) SetNSKeys(ctx context.Context, creds domain.ProviderCredentials, domainName string, nsKeys []string) (*domain.ArvanCloudDomain, error) {
	var wire nsKeysWire
	body := nsKeysWire{NSKeys: nsKeys}
	if err := p.client.doJSON(ctx, creds, http.MethodPut, domainPath(domainName)+"/ns-keys", body, &wire); err != nil {
		return nil, fmt.Errorf("setting NS keys for arvancloud domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudDomain{Name: domainName, NSKeys: wire.NSKeys}, nil
}

// ResetNSKeys resets a "full" domain's NS records to ArvanCloud's defaults.
func (p *Provider) ResetNSKeys(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	var wire nsKeysWire
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, domainPath(domainName)+"/ns-keys", nil, &wire); err != nil {
		return nil, fmt.Errorf("resetting NS keys for arvancloud domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudDomain{Name: domainName, NSKeys: wire.NSKeys}, nil
}

// nsCheckWire mirrors NsDomain: the response of the NS check endpoint.
// NSDomain is what the registrar currently has configured; NSKeys is what
// ArvanCloud expects it to be.
type nsCheckWire struct {
	NSDomain []string `json:"ns_domain"`
	NSKeys   []string `json:"ns_keys"`
}

// CheckNSStatus reports whether the registrar has been repointed at
// ArvanCloud yet, for a "full" domain.
func (p *Provider) CheckNSStatus(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	var wire nsCheckWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, domainPath(domainName)+"/ns-keys/check", nil, &wire); err != nil {
		return nil, fmt.Errorf("checking NS status for arvancloud domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudDomain{Name: domainName, NSKeys: wire.NSKeys, CurrentNS: wire.NSDomain}, nil
}

// UseOptionalNSKeys switches a "full" domain to ArvanCloud's alternate NS key
// set.
func (p *Provider) UseOptionalNSKeys(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	var wire nsKeysWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+"/ns-keys/use-optional-keys", nil, &wire); err != nil {
		return nil, fmt.Errorf("switching to optional NS keys for arvancloud domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudDomain{Name: domainName, NSKeys: wire.NSKeys}, nil
}

// customCnameWire mirrors CustomCname, the request body of the CNAME Setup
// set endpoint.
type customCnameWire struct {
	Address string `json:"address"`
}

// SetCnameTarget sets the custom CNAME record a "partial" domain resolves
// through.
func (p *Provider) SetCnameTarget(ctx context.Context, creds domain.ProviderCredentials, domainName, address string) (*domain.ArvanCloudDomain, error) {
	var wire domainWire
	body := customCnameWire{Address: address}
	if err := p.client.doJSON(ctx, creds, http.MethodPut, domainPath(domainName)+"/cname-setup/custom", body, &wire); err != nil {
		return nil, fmt.Errorf("setting CNAME target for arvancloud domain %q: %w", domainName, err)
	}
	result := toDomainDomain(domainName, wire)
	return &result, nil
}

// ResetCnameTarget resets a "partial" domain's CNAME record to ArvanCloud's
// default.
func (p *Provider) ResetCnameTarget(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	var wire domainWire
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, domainPath(domainName)+"/cname-setup/custom", nil, &wire); err != nil {
		return nil, fmt.Errorf("resetting CNAME target for arvancloud domain %q: %w", domainName, err)
	}
	result := toDomainDomain(domainName, wire)
	return &result, nil
}

// ConvertToCnameSetup switches a domain's onboarding mode to CNAME Setup
// ("partial").
func (p *Provider) ConvertToCnameSetup(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	var wire domainWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+"/cname-setup/convert", nil, &wire); err != nil {
		return nil, fmt.Errorf("converting arvancloud domain %q to CNAME setup: %w", domainName, err)
	}
	result := toDomainDomain(domainName, wire)
	return &result, nil
}

// CheckCnameStatus reports whether a "partial" domain's CNAME has been
// activated yet.
func (p *Provider) CheckCnameStatus(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	var wire domainWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, domainPath(domainName)+"/cname-setup/check", nil, &wire); err != nil {
		return nil, fmt.Errorf("checking CNAME status for arvancloud domain %q: %w", domainName, err)
	}
	result := toDomainDomain(domainName, wire)
	return &result, nil
}

// cloneDomainWire mirrors CloneDomain, the request body of the clone
// endpoint.
type cloneDomainWire struct {
	From string `json:"from"`
}

// CloneDomainConfig copies fromDomain's CDN configuration onto domainName.
// The endpoint's response carries no data to translate — only a confirmation
// message — so there is nothing for this method to return but the error.
func (p *Provider) CloneDomainConfig(ctx context.Context, creds domain.ProviderCredentials, domainName, fromDomain string) error {
	body := cloneDomainWire{From: fromDomain}
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+"/clone", body, nil); err != nil {
		return fmt.Errorf("cloning arvancloud domain config from %q onto %q: %w", fromDomain, domainName, err)
	}
	return nil
}

// RegenerateDomainConfig re-publishes domainName's current configuration to
// the edge servers.
func (p *Provider) RegenerateDomainConfig(ctx context.Context, creds domain.ProviderCredentials, domainName string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+"/regenerate", nil, nil); err != nil {
		return fmt.Errorf("regenerating arvancloud domain config for %q: %w", domainName, err)
	}
	return nil
}

// HoldDomain pauses CDN service for domainName.
func (p *Provider) HoldDomain(ctx context.Context, creds domain.ProviderCredentials, domainName string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+"/hold", nil, nil); err != nil {
		return fmt.Errorf("holding arvancloud domain %q: %w", domainName, err)
	}
	return nil
}

// UnholdDomain resumes CDN service for domainName.
func (p *Provider) UnholdDomain(ctx context.Context, creds domain.ProviderCredentials, domainName string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodPost, domainPath(domainName)+"/unhold", nil, nil); err != nil {
		return fmt.Errorf("unholding arvancloud domain %q: %w", domainName, err)
	}
	return nil
}
