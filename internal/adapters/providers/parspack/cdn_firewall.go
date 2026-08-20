package parspack

import (
	"context"
	"fmt"
	"strconv"

	"github.com/javadib/do0ps/internal/core/domain"
)

// CDN-edge firewall: access-management rules, IP-reputation blocking and
// DDoS mitigation actions, wired to the real CDN API (issue #24). Base paths
// are confirmed against docs/api-specs/parspack-cdn.openapi.yaml lines
// 3634-5027 (the "Firewall" tag), relative to Client.cdnBaseURL.
//
// These are entirely distinct from the cloud-server/VM-network-level
// Firewall methods elsewhere in this package (issue #11) — every method and
// wire type here is scoped to a CDN zone_uuid instead of a server/network.

func cdnAccessManagementBasePath(zoneUUID string) string {
	return "external/api/v1/zones/" + zoneUUID + "/firewalls/access-management"
}

func cdnIPReputationPath(zoneUUID string) string {
	return "external/api/v1/zones/" + zoneUUID + "/firewalls/ip-reputation"
}

func cdnDDoSActionsPath(zoneUUID string) string {
	return "external/api/v1/zones/" + zoneUUID + "/ddos-actions"
}

// cdnValueDetailWire mirrors an access-management rule's "value_detail"
// field. The spec's schema types it as a nullable string but every example
// with a non-null value shows an object with a "name" (e.g. a resolved
// country name) — decoding into a pointer struct handles both: JSON null
// leaves it nil, and a present object decodes normally.
type cdnValueDetailWire struct {
	Name string `json:"name"`
}

// cdnBulklistItemWire is one entry of an access-management rule's
// "bulklist.items" array.
type cdnBulklistItemWire struct {
	Value       string              `json:"value"`
	ValueDetail *cdnValueDetailWire `json:"value_detail"`
}

// cdnBulklistWire mirrors the "bulklist" object nested in an
// access-management rule, present when the rule references a managed list of
// values instead of a single Value.
type cdnBulklistWire struct {
	ID    int                   `json:"id"`
	Name  string                `json:"name"`
	Items []cdnBulklistItemWire `json:"items"`
}

// cdnAccessRuleWire mirrors GET .../access-management and
// GET .../access-management/{id}'s "data" object.
type cdnAccessRuleWire struct {
	ID          string              `json:"id"`
	Type        string              `json:"type"`
	Value       string              `json:"value"`
	Action      string              `json:"action"`
	Status      bool                `json:"status"`
	Priority    int                 `json:"priority"`
	ValueDetail *cdnValueDetailWire `json:"value_detail"`
	Bulklist    *cdnBulklistWire    `json:"bulklist"`
}

func toDomainAccessRule(zoneUUID string, w cdnAccessRuleWire) domain.CDNAccessRule {
	rule := domain.CDNAccessRule{
		ID: w.ID, ZoneUUID: zoneUUID, Type: w.Type, Value: w.Value,
		Action: w.Action, Status: w.Status, Priority: w.Priority,
	}
	if w.Bulklist != nil {
		rule.BulklistID = strconv.Itoa(w.Bulklist.ID)
		rule.BulklistName = w.Bulklist.Name
	}
	return rule
}

// ListCDNAccessRules returns every access-management rule of a CDN zone.
func (c *Client) ListCDNAccessRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNAccessRule, error) {
	var items []cdnAccessRuleWire
	if err := c.doCDNJSON(ctx, creds, "GET", cdnAccessManagementBasePath(zoneUUID), nil, &items); err != nil {
		return nil, fmt.Errorf("list CDN access rules of zone %s: %w", zoneUUID, err)
	}

	rules := make([]domain.CDNAccessRule, len(items))
	for i := range items {
		rules[i] = toDomainAccessRule(zoneUUID, items[i])
	}
	return rules, nil
}

// GetCDNAccessRule returns one access-management rule by ID.
func (c *Client) GetCDNAccessRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNAccessRule, error) {
	var w cdnAccessRuleWire
	if err := c.doCDNJSON(ctx, creds, "GET", cdnAccessManagementBasePath(zoneUUID)+"/"+ruleID, nil, &w); err != nil {
		return nil, fmt.Errorf("get CDN access rule %s of zone %s: %w", ruleID, zoneUUID, err)
	}
	rule := toDomainAccessRule(zoneUUID, w)
	return &rule, nil
}

// cdnAccessRuleCreateRequest is the body of POST .../access-management.
type cdnAccessRuleCreateRequest struct {
	Action     string `json:"action"`
	Type       string `json:"type"`
	Value      string `json:"value,omitempty"`
	BulklistID string `json:"bulklist_id,omitempty"`
}

// CreateCDNAccessRule adds a new access-management rule to a zone. The
// provider's create endpoint returns only a success message with an empty
// body (confirmed against the spec's 201 example) — no ID or priority is
// reported — so, like CreateDNSRecord, this echoes back the rule that was
// requested. Call ListCDNAccessRules afterward to learn the assigned ID and
// priority.
func (c *Client) CreateCDNAccessRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNAccessRule) (*domain.CDNAccessRule, error) {
	reqBody := cdnAccessRuleCreateRequest{
		Action: rule.Action, Type: rule.Type, Value: rule.Value, BulklistID: rule.BulklistID,
	}

	if err := c.doCDNJSON(ctx, creds, "POST", cdnAccessManagementBasePath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("create CDN access rule in zone %s: %w", zoneUUID, err)
	}

	created := rule
	created.ZoneUUID = zoneUUID
	return &created, nil
}

// cdnAccessRuleUpdateRequest is the body of PUT .../access-management/{id}.
type cdnAccessRuleUpdateRequest struct {
	Value      string `json:"value,omitempty"`
	Action     string `json:"action"`
	Status     bool   `json:"status"`
	BulklistID string `json:"bulklist_id,omitempty"`
}

// UpdateCDNAccessRule updates an existing rule identified by rule.ID. As with
// create, the provider's response carries no body, so this echoes back the
// rule that was requested.
func (c *Client) UpdateCDNAccessRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNAccessRule) (*domain.CDNAccessRule, error) {
	if rule.ID == "" {
		return nil, fmt.Errorf("update CDN access rule in zone %s: rule ID is required: %w", zoneUUID, domain.ErrInvalidInput)
	}

	reqBody := cdnAccessRuleUpdateRequest{
		Value: rule.Value, Action: rule.Action, Status: rule.Status, BulklistID: rule.BulklistID,
	}

	if err := c.doCDNJSON(ctx, creds, "PUT", cdnAccessManagementBasePath(zoneUUID)+"/"+rule.ID, reqBody, nil); err != nil {
		return nil, fmt.Errorf("update CDN access rule %s in zone %s: %w", rule.ID, zoneUUID, err)
	}

	updated := rule
	updated.ZoneUUID = zoneUUID
	return &updated, nil
}

// DeleteCDNAccessRule removes a rule by ID.
func (c *Client) DeleteCDNAccessRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", cdnAccessManagementBasePath(zoneUUID)+"/"+ruleID, nil, nil); err != nil {
		return fmt.Errorf("delete CDN access rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	return nil
}

// cdnIPReputationWire mirrors GET/PUT .../firewalls/ip-reputation's "data"
// object.
type cdnIPReputationWire struct {
	Enabled       bool   `json:"ip_reputation_enabled"`
	TrustTime     int    `json:"ip_reputation_trust_time"`
	TreatScore    string `json:"ip_reputation_treat_score"`
	Challenge     string `json:"ip_reputation_challenge"`
	AttackBanTime int    `json:"attack_ban_time"`
}

func toDomainIPReputation(w cdnIPReputationWire) domain.CDNIPReputationSettings {
	return domain.CDNIPReputationSettings{
		Enabled: w.Enabled, TrustTime: w.TrustTime, TreatScore: w.TreatScore,
		Challenge: w.Challenge, AttackBanTime: w.AttackBanTime,
	}
}

func toWireIPReputation(s domain.CDNIPReputationSettings) cdnIPReputationWire {
	return cdnIPReputationWire{
		Enabled: s.Enabled, TrustTime: s.TrustTime, TreatScore: s.TreatScore,
		Challenge: s.Challenge, AttackBanTime: s.AttackBanTime,
	}
}

// GetCDNIPReputation returns the current IP-reputation blocking settings for
// a zone.
func (c *Client) GetCDNIPReputation(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNIPReputationSettings, error) {
	var w cdnIPReputationWire
	if err := c.doCDNJSON(ctx, creds, "GET", cdnIPReputationPath(zoneUUID), nil, &w); err != nil {
		return nil, fmt.Errorf("get CDN IP reputation settings for zone %s: %w", zoneUUID, err)
	}
	settings := toDomainIPReputation(w)
	return &settings, nil
}

// UpdateCDNIPReputation replaces a zone's IP-reputation settings; the
// provider requires every field. Its response carries no body, so this
// echoes back the settings that were requested.
func (c *Client) UpdateCDNIPReputation(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, settings domain.CDNIPReputationSettings) (*domain.CDNIPReputationSettings, error) {
	reqBody := toWireIPReputation(settings)
	if err := c.doCDNJSON(ctx, creds, "PUT", cdnIPReputationPath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("update CDN IP reputation settings for zone %s: %w", zoneUUID, err)
	}
	updated := settings
	return &updated, nil
}

// cdnDDoSActionWire mirrors GET .../ddos-actions's "data" object.
type cdnDDoSActionWire struct {
	Action    string `json:"action"`
	TrustTime int    `json:"trust_time"`
	BanTime   int    `json:"ban_time"`
}

func toDomainDDoSActions(w cdnDDoSActionWire) domain.CDNDDoSActionSettings {
	return domain.CDNDDoSActionSettings{Action: w.Action, TrustTime: w.TrustTime, BanTime: w.BanTime}
}

// GetCDNDDoSActions returns the current DDoS mitigation action settings for a
// zone.
func (c *Client) GetCDNDDoSActions(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNDDoSActionSettings, error) {
	var w cdnDDoSActionWire
	if err := c.doCDNJSON(ctx, creds, "GET", cdnDDoSActionsPath(zoneUUID), nil, &w); err != nil {
		return nil, fmt.Errorf("get CDN DDoS action settings for zone %s: %w", zoneUUID, err)
	}
	settings := toDomainDDoSActions(w)
	return &settings, nil
}

// cdnDDoSActionUpdateRequest is the body of PUT .../ddos-actions. TrustTime
// and BanTime are optional (the provider defaults them to 3600 and 900).
type cdnDDoSActionUpdateRequest struct {
	Action    string `json:"action"`
	TrustTime int    `json:"trust_time,omitempty"`
	BanTime   int    `json:"ban_time,omitempty"`
}

// UpdateCDNDDoSActions updates a zone's DDoS mitigation action. The
// provider's response carries no body, so this echoes back the settings that
// were requested.
func (c *Client) UpdateCDNDDoSActions(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, settings domain.CDNDDoSActionSettings) (*domain.CDNDDoSActionSettings, error) {
	reqBody := cdnDDoSActionUpdateRequest{Action: settings.Action, TrustTime: settings.TrustTime, BanTime: settings.BanTime}
	if err := c.doCDNJSON(ctx, creds, "PUT", cdnDDoSActionsPath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("update CDN DDoS action settings for zone %s: %w", zoneUUID, err)
	}
	updated := settings
	return &updated, nil
}
