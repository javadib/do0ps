package parspack

import (
	"context"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// CDN Bulklist and Country reference list (issue #24), wired to the real CDN
// API. Base paths are confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml's Bulklist and Country tags,
// relative to Client.cdnBaseURL.
//
// Bulklist is a user-level resource (no zone_uuid on the path, unlike zones
// and DNS records); Country is zone-scoped and reuses zonesBasePath, already
// declared in cdn.go.

const (
	bulklistsBasePath         = "external/api/v1/user/bulklists"
	firewallCountriesPathTail = "firewalls/countries"
)

// bulklistItemWire mirrors one entry of a bulklist's "items" array as
// reported by the list/get endpoints.
type bulklistItemWire struct {
	Value       string `json:"value"`
	ValueDetail string `json:"value_detail"`
}

// bulklistWire is the response shape of the list and get bulklist endpoints.
type bulklistWire struct {
	ID    string             `json:"id"`
	Name  string             `json:"name"`
	Type  string             `json:"type"`
	Items []bulklistItemWire `json:"items"`
}

func toDomainBulklist(w bulklistWire) domain.CDNBulklist {
	items := make([]domain.CDNBulklistItem, len(w.Items))
	for i, it := range w.Items {
		items[i] = domain.CDNBulklistItem{Value: it.Value, ValueDetail: it.ValueDetail}
	}
	return domain.CDNBulklist{ID: w.ID, Name: w.Name, Type: w.Type, Items: items}
}

// bulklistFromSpec builds the domain result for create/update, whose
// responses do not echo the resource (the spec's documented "data" is an
// empty array), so the returned bulklist reflects what was sent rather than
// what was decoded.
func bulklistFromSpec(id string, spec domain.CDNBulklistSpec) *domain.CDNBulklist {
	items := make([]domain.CDNBulklistItem, len(spec.Items))
	for i, v := range spec.Items {
		items[i] = domain.CDNBulklistItem{Value: v}
	}
	return &domain.CDNBulklist{ID: id, Name: spec.Name, Type: spec.Type, Items: items}
}

// ListCDNBulklists returns every bulklist belonging to the account behind
// the given credentials.
func (c *Client) ListCDNBulklists(ctx context.Context, creds domain.ProviderCredentials) ([]domain.CDNBulklist, error) {
	var items []bulklistWire
	if err := c.doCDNJSON(ctx, creds, "GET", bulklistsBasePath, nil, &items); err != nil {
		return nil, fmt.Errorf("list CDN bulklists: %w", err)
	}

	lists := make([]domain.CDNBulklist, len(items))
	for i := range items {
		lists[i] = toDomainBulklist(items[i])
	}
	return lists, nil
}

// bulklistWriteRequest is the body of both the create and update bulklist
// endpoints.
type bulklistWriteRequest struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Items []string `json:"items"`
}

// CreateCDNBulklist creates a new bulklist for the account.
func (c *Client) CreateCDNBulklist(ctx context.Context, creds domain.ProviderCredentials, spec domain.CDNBulklistSpec) (*domain.CDNBulklist, error) {
	reqBody := bulklistWriteRequest{Name: spec.Name, Type: spec.Type, Items: spec.Items}
	if err := c.doCDNJSON(ctx, creds, "POST", bulklistsBasePath, reqBody, nil); err != nil {
		return nil, fmt.Errorf("creating CDN bulklist %q: %w", spec.Name, err)
	}
	return bulklistFromSpec("", spec), nil
}

// GetCDNBulklist returns a single bulklist by ID.
func (c *Client) GetCDNBulklist(ctx context.Context, creds domain.ProviderCredentials, bulklistID string) (*domain.CDNBulklist, error) {
	var wire bulklistWire
	if err := c.doCDNJSON(ctx, creds, "GET", bulklistsBasePath+"/"+bulklistID, nil, &wire); err != nil {
		return nil, fmt.Errorf("get CDN bulklist %s: %w", bulklistID, err)
	}
	list := toDomainBulklist(wire)
	return &list, nil
}

// UpdateCDNBulklist replaces the name, type and items of an existing
// bulklist.
func (c *Client) UpdateCDNBulklist(ctx context.Context, creds domain.ProviderCredentials, bulklistID string, spec domain.CDNBulklistSpec) (*domain.CDNBulklist, error) {
	reqBody := bulklistWriteRequest{Name: spec.Name, Type: spec.Type, Items: spec.Items}
	if err := c.doCDNJSON(ctx, creds, "PUT", bulklistsBasePath+"/"+bulklistID, reqBody, nil); err != nil {
		return nil, fmt.Errorf("updating CDN bulklist %s: %w", bulklistID, err)
	}
	return bulklistFromSpec(bulklistID, spec), nil
}

// DeleteCDNBulklist removes a bulklist by ID.
func (c *Client) DeleteCDNBulklist(ctx context.Context, creds domain.ProviderCredentials, bulklistID string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", bulklistsBasePath+"/"+bulklistID, nil, nil); err != nil {
		return fmt.Errorf("deleting CDN bulklist %s: %w", bulklistID, err)
	}
	return nil
}

// countryWire mirrors one entry of GET .../firewalls/countries.
type countryWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListCDNFirewallCountries returns the reference list of countries usable
// when configuring country-based firewall rules for a zone.
func (c *Client) ListCDNFirewallCountries(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNCountry, error) {
	var items []countryWire
	path := zonesBasePath + "/" + zoneUUID + "/" + firewallCountriesPathTail
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &items); err != nil {
		return nil, fmt.Errorf("list CDN firewall countries for zone %s: %w", zoneUUID, err)
	}

	countries := make([]domain.CDNCountry, len(items))
	for i, it := range items {
		countries[i] = domain.CDNCountry{Code: it.ID, Name: it.Name}
	}
	return countries, nil
}
