package parspack

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// ModSec (Web Application Firewall custom rules), scoped to a CDN zone same
// as DNS records (AGENTS.md 4.1). Confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml lines 6677-8223 (issue #24's
// ModSec tag): a zone selects some mix of provider-managed "standard" rule
// sets and its own "custom" rules; custom rules can reference reusable
// "data" values.
//
// The wire types below mirror that spec's request/response shapes exactly
// (field names and JSON tags). Two response quirks the spec documents, that
// this adapter works around rather than papering over:
//   - The create/update endpoints for both ModSecData and ModSecRule return
//     an empty "data" array — the provider never reports the created
//     resource's assigned id in that response. CreateCDNModSecData and
//     CreateCDNModSecRule echo the input back instead of the (absent)
//     response; callers must call the matching List method afterward to
//     discover the new id.
//   - The custom-rule Show endpoint's response omits name/status for the
//     rule itself, unlike its List endpoint — GetCDNModSecRule leaves
//     domain.CDNModSecRule.Name and .Status at their zero value.
//   - Similarly, POST .../firewalls/mod-security (the zone's overall ModSec
//     selection) returns no data on success, so UpdateCDNModSecStatus
//     re-fetches via GetCDNModSecStatus to give the caller a real result.
//
// modSecBasePath/modSecDataBasePath/modSecRulesBasePath build on zonesBasePath
// (declared in cdn.go, same package) rather than redeclaring it.

// modSecPathSegment is the path segment ModSec status, data and rules all
// nest under, relative to a zone.
const modSecPathSegment = "firewalls/mod-security"

func modSecBasePath(zoneUUID string) string {
	return zonesBasePath + "/" + zoneUUID + "/" + modSecPathSegment
}

func modSecDataBasePath(zoneUUID string) string {
	return modSecBasePath(zoneUUID) + "/data"
}

func modSecRulesBasePath(zoneUUID string) string {
	return modSecBasePath(zoneUUID) + "/rules"
}

// nonNilStrings ensures a required array field is sent as [] rather than
// null (encoding/json marshals a nil slice as null) when the caller supplied
// nothing — the CDN API's modsec_rules and modsec_data_ids fields are
// required even when empty.
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// --- ModSec status (zone-level rule-set selection) -------------------------

// modSecRuleSetItemWire mirrors one entry of "standards"/"customs" in
// GET .../firewalls/mod-security.
type modSecRuleSetItemWire struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Selected bool   `json:"selected"`
}

// modSecStatusWire mirrors the "data" object of GET .../firewalls/mod-security.
type modSecStatusWire struct {
	Standards []modSecRuleSetItemWire `json:"standards"`
	Customs   []modSecRuleSetItemWire `json:"customs"`
}

func toDomainModSecRuleSetItems(items []modSecRuleSetItemWire) []domain.CDNModSecRuleSetItem {
	out := make([]domain.CDNModSecRuleSetItem, len(items))
	for i, it := range items {
		out[i] = domain.CDNModSecRuleSetItem{ID: it.ID, Name: it.Name, Selected: it.Selected}
	}
	return out
}

func toDomainModSecStatus(w modSecStatusWire) domain.CDNModSecStatus {
	return domain.CDNModSecStatus{
		Standards: toDomainModSecRuleSetItems(w.Standards),
		Customs:   toDomainModSecRuleSetItems(w.Customs),
	}
}

// GetCDNModSecStatus returns the zone's ModSec rule-set selection: which
// standard rule sets Parspack offers and which of the zone's custom rules
// exist, each flagged with whether it is currently selected.
func (c *Client) GetCDNModSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNModSecStatus, error) {
	var wire modSecStatusWire
	if err := c.doCDNJSON(ctx, creds, "GET", modSecBasePath(zoneUUID), nil, &wire); err != nil {
		return nil, fmt.Errorf("get modsec status for zone %s: %w", zoneUUID, err)
	}
	status := toDomainModSecStatus(wire)
	return &status, nil
}

// modSecStatusUpdateRequest is the body of POST .../firewalls/mod-security.
type modSecStatusUpdateRequest struct {
	ModSecRules []string `json:"modsec_rules"`
}

// UpdateCDNModSecStatus replaces the set of selected ModSec rule-set ids
// (standard and/or custom) for the zone; pass an empty slice to clear every
// selection. The provider's response carries no data, so this re-fetches and
// returns the zone's status afterward via GetCDNModSecStatus.
func (c *Client) UpdateCDNModSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, selectedRuleIDs []string) (*domain.CDNModSecStatus, error) {
	reqBody := modSecStatusUpdateRequest{ModSecRules: nonNilStrings(selectedRuleIDs)}
	if err := c.doCDNJSON(ctx, creds, "POST", modSecBasePath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("update modsec status for zone %s: %w", zoneUUID, err)
	}
	status, err := c.GetCDNModSecStatus(ctx, creds, zoneUUID)
	if err != nil {
		return nil, fmt.Errorf("update modsec status for zone %s: refetching after update: %w", zoneUUID, err)
	}
	return status, nil
}

// --- ModSec data (reusable values custom rules reference) ------------------

// modSecDataListItemWire mirrors one entry of GET .../firewalls/mod-security/data
// — the list endpoint omits "value".
type modSecDataListItemWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// modSecDataWire mirrors GET .../firewalls/mod-security/data/{id}, and the
// request bodies of dataStore/dataUpdate — Value is base64-encoded on the
// wire in every direction (spec: "should be base64 encoded").
type modSecDataWire struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

func toDomainModSecData(w modSecDataWire) (domain.CDNModSecData, error) {
	decoded, err := base64.StdEncoding.DecodeString(w.Value)
	if err != nil {
		return domain.CDNModSecData{}, fmt.Errorf("decoding modsec data %s value: %w", w.ID, err)
	}
	return domain.CDNModSecData{ID: w.ID, Name: w.Name, Value: string(decoded)}, nil
}

// ListCDNModSecData returns every reusable ModSec data value defined on the
// zone. Only id and name are reported here — call GetCDNModSecData for a
// specific entry's decoded value.
func (c *Client) ListCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNModSecData, error) {
	var items []modSecDataListItemWire
	if err := c.doCDNJSON(ctx, creds, "GET", modSecDataBasePath(zoneUUID), nil, &items); err != nil {
		return nil, fmt.Errorf("list modsec data for zone %s: %w", zoneUUID, err)
	}
	data := make([]domain.CDNModSecData, len(items))
	for i, it := range items {
		data[i] = domain.CDNModSecData{ID: it.ID, Name: it.Name}
	}
	return data, nil
}

// CreateCDNModSecData adds a new reusable ModSec data value to the zone. The
// provider's create response carries no data (not even the new id) — this
// echoes the input back; call ListCDNModSecData afterward to discover the
// assigned id.
func (c *Client) CreateCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, data domain.CDNModSecData) (*domain.CDNModSecData, error) {
	reqBody := modSecDataWire{Name: data.Name, Value: base64.StdEncoding.EncodeToString([]byte(data.Value))}
	if err := c.doCDNJSON(ctx, creds, "POST", modSecDataBasePath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("create modsec data %q for zone %s: %w", data.Name, zoneUUID, err)
	}
	created := domain.CDNModSecData{Name: data.Name, Value: data.Value}
	return &created, nil
}

// GetCDNModSecData returns one ModSec data value by id, decoded from its
// base64 wire form.
func (c *Client) GetCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNModSecData, error) {
	var wire modSecDataWire
	if err := c.doCDNJSON(ctx, creds, "GET", modSecDataBasePath(zoneUUID)+"/"+id, nil, &wire); err != nil {
		return nil, fmt.Errorf("get modsec data %s for zone %s: %w", id, zoneUUID, err)
	}
	data, err := toDomainModSecData(wire)
	if err != nil {
		return nil, fmt.Errorf("get modsec data %s for zone %s: %w", id, zoneUUID, err)
	}
	return &data, nil
}

// UpdateCDNModSecData replaces an existing ModSec data value's name and
// value by id. The provider's response carries no data, so this echoes back
// the id and the input that was sent.
func (c *Client) UpdateCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string, data domain.CDNModSecData) (*domain.CDNModSecData, error) {
	reqBody := modSecDataWire{Name: data.Name, Value: base64.StdEncoding.EncodeToString([]byte(data.Value))}
	if err := c.doCDNJSON(ctx, creds, "PUT", modSecDataBasePath(zoneUUID)+"/"+id, reqBody, nil); err != nil {
		return nil, fmt.Errorf("update modsec data %s for zone %s: %w", id, zoneUUID, err)
	}
	updated := domain.CDNModSecData{ID: id, Name: data.Name, Value: data.Value}
	return &updated, nil
}

// DeleteCDNModSecData removes a ModSec data value by id.
func (c *Client) DeleteCDNModSecData(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", modSecDataBasePath(zoneUUID)+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete modsec data %s for zone %s: %w", id, zoneUUID, err)
	}
	return nil
}

// --- ModSec custom rules -----------------------------------------------

// modSecRuleListItemWire mirrors one entry of GET .../firewalls/mod-security/rules
// — the list endpoint reports name/status but not rule_value or mod_sec_data.
type modSecRuleListItemWire struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// modSecRuleShowWire mirrors GET .../firewalls/mod-security/rules/{mod_sec_rule}
// — unlike the list endpoint, it omits name/status for the rule itself but
// expands the referenced data entries.
type modSecRuleShowWire struct {
	ID         string           `json:"id"`
	RuleValue  string           `json:"rule_value"`
	ModSecData []modSecDataWire `json:"mod_sec_data"`
}

// modSecRuleWriteRequest is the body of POST .../rules and
// PUT .../rules/{mod_sec_rule}.
type modSecRuleWriteRequest struct {
	Name          string   `json:"name"`
	RuleValue     string   `json:"rule_value"`
	ModSecDataIDs []string `json:"modsec_data_ids"`
}

func toDomainModSecRuleShow(w modSecRuleShowWire) (domain.CDNModSecRule, error) {
	decoded, err := base64.StdEncoding.DecodeString(w.RuleValue)
	if err != nil {
		return domain.CDNModSecRule{}, fmt.Errorf("decoding modsec rule %s rule_value: %w", w.ID, err)
	}
	data := make([]domain.CDNModSecData, len(w.ModSecData))
	for i, d := range w.ModSecData {
		item, err := toDomainModSecData(d)
		if err != nil {
			return domain.CDNModSecRule{}, err
		}
		data[i] = item
	}
	return domain.CDNModSecRule{ID: w.ID, RuleValue: string(decoded), ModSecData: data}, nil
}

// ListCDNModSecRules returns every custom ModSec rule defined on the zone.
// Only id, name and status are reported here — call GetCDNModSecRule for a
// specific rule's decoded value and referenced data.
func (c *Client) ListCDNModSecRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNModSecRule, error) {
	var items []modSecRuleListItemWire
	if err := c.doCDNJSON(ctx, creds, "GET", modSecRulesBasePath(zoneUUID), nil, &items); err != nil {
		return nil, fmt.Errorf("list modsec rules for zone %s: %w", zoneUUID, err)
	}
	rules := make([]domain.CDNModSecRule, len(items))
	for i, it := range items {
		rules[i] = domain.CDNModSecRule{ID: it.ID, Name: it.Name, Status: it.Status}
	}
	return rules, nil
}

// CreateCDNModSecRule adds a new custom ModSec rule to the zone, optionally
// referencing existing ModSec data by id. The provider's create response
// carries no data (not even the new id) — this echoes the input back; call
// ListCDNModSecRules afterward to discover the assigned id.
func (c *Client) CreateCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNModSecRule) (*domain.CDNModSecRule, error) {
	reqBody := modSecRuleWriteRequest{
		Name:          rule.Name,
		RuleValue:     base64.StdEncoding.EncodeToString([]byte(rule.RuleValue)),
		ModSecDataIDs: nonNilStrings(rule.ModSecDataIDs),
	}
	if err := c.doCDNJSON(ctx, creds, "POST", modSecRulesBasePath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("create modsec rule %q for zone %s: %w", rule.Name, zoneUUID, err)
	}
	created := domain.CDNModSecRule{Name: rule.Name, RuleValue: rule.RuleValue, ModSecDataIDs: rule.ModSecDataIDs}
	return &created, nil
}

// GetCDNModSecRule returns one custom ModSec rule by id, with its rule value
// decoded and its referenced ModSec data expanded. The provider's response
// does not include the rule's name or status (unlike the list endpoint) —
// those fields are left at their zero value here.
func (c *Client) GetCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNModSecRule, error) {
	var wire modSecRuleShowWire
	if err := c.doCDNJSON(ctx, creds, "GET", modSecRulesBasePath(zoneUUID)+"/"+ruleID, nil, &wire); err != nil {
		return nil, fmt.Errorf("get modsec rule %s for zone %s: %w", ruleID, zoneUUID, err)
	}
	rule, err := toDomainModSecRuleShow(wire)
	if err != nil {
		return nil, fmt.Errorf("get modsec rule %s for zone %s: %w", ruleID, zoneUUID, err)
	}
	return &rule, nil
}

// UpdateCDNModSecRule replaces an existing custom ModSec rule's name, value
// and referenced data ids by id. The provider's response carries no data, so
// this echoes back the id and the input that was sent.
func (c *Client) UpdateCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNModSecRule) (*domain.CDNModSecRule, error) {
	reqBody := modSecRuleWriteRequest{
		Name:          rule.Name,
		RuleValue:     base64.StdEncoding.EncodeToString([]byte(rule.RuleValue)),
		ModSecDataIDs: nonNilStrings(rule.ModSecDataIDs),
	}
	if err := c.doCDNJSON(ctx, creds, "PUT", modSecRulesBasePath(zoneUUID)+"/"+ruleID, reqBody, nil); err != nil {
		return nil, fmt.Errorf("update modsec rule %s for zone %s: %w", ruleID, zoneUUID, err)
	}
	updated := domain.CDNModSecRule{ID: ruleID, Name: rule.Name, RuleValue: rule.RuleValue, ModSecDataIDs: rule.ModSecDataIDs}
	return &updated, nil
}

// DeleteCDNModSecRule removes a custom ModSec rule by id.
func (c *Client) DeleteCDNModSecRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", modSecRulesBasePath(zoneUUID)+"/"+ruleID, nil, nil); err != nil {
		return fmt.Errorf("delete modsec rule %s for zone %s: %w", ruleID, zoneUUID, err)
	}
	return nil
}
