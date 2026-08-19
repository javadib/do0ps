package parspack

import (
	"context"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Rate Limit Rules and Upstream Errors, two more corners of the CDN edge
// firewall (issue #24). Both are scoped under a zone, relative to
// Client.cdnBaseURL and zonesBasePath (defined in cdn.go), i.e.
// https://my.parspack.com/cdnapi/external/api/v1/zones/{zone_uuid}/... —
// confirmed against docs/api-specs/parspack-cdn.openapi.yaml's "Rate Limit
// Rule" and "Upstream Errors" tags.
//
// The wire types below mirror those tags' request/response shapes exactly.
// Nothing above the adapter boundary ever sees these — every method here
// translates to/from internal/core/domain types.

const (
	rateLimitRulesPathSegment = "firewalls/rate-limit"
	upstreamErrorsPathSegment = "upstream-errors"
)

// rateLimitRulesPath is the collection path for a zone's rate limit rules.
func rateLimitRulesPath(zoneUUID string) string {
	return zonesBasePath + "/" + zoneUUID + "/" + rateLimitRulesPathSegment
}

// rateLimitRulePath is the single-resource path for one rate limit rule.
func rateLimitRulePath(zoneUUID, ruleID string) string {
	return rateLimitRulesPath(zoneUUID) + "/" + ruleID
}

// upstreamErrorsPath is the single-resource path for a zone's upstream
// errors setting.
func upstreamErrorsPath(zoneUUID string) string {
	return zonesBasePath + "/" + zoneUUID + "/" + upstreamErrorsPathSegment
}

// rateLimitWhitelistIPWire mirrors one entry of a rate limit rule's
// "white_list_ips" as reported on list/show. A store/update request instead
// sends this field as a plain array of IP strings (see
// rateLimitRuleCreateRequest/rateLimitRuleUpdateRequest), so this shape is
// read-only.
type rateLimitWhitelistIPWire struct {
	ID              int    `json:"id"`
	IP              string `json:"ip"`
	RateLimitRuleID int    `json:"rate_limit_rule_id"`
}

// rateLimitRuleWire mirrors the object returned by index/show rate limit
// rule endpoints.
type rateLimitRuleWire struct {
	ID                  string                     `json:"id"`
	Value               string                     `json:"value"`
	Enabled             bool                       `json:"enabled"`
	Name                string                     `json:"name"`
	Priority            int                        `json:"priority"`
	StaticIntervalType  string                     `json:"static_interval_type"`
	StaticInterval      int                        `json:"static_interval"`
	StaticRequests      int                        `json:"static_requests"`
	DynamicIntervalType string                     `json:"dynamic_interval_type"`
	DynamicInterval     int                        `json:"dynamic_interval"`
	DynamicRequests     int                        `json:"dynamic_requests"`
	Challenge           string                     `json:"challenge"`
	TrustTime           int                        `json:"trust_time"`
	AttackBanTime       int                        `json:"attack_ban_time"`
	WhiteListIPs        []rateLimitWhitelistIPWire `json:"white_list_ips"`
}

func toDomainRateLimitRule(w rateLimitRuleWire) domain.CDNRateLimitRule {
	rule := domain.CDNRateLimitRule{
		ID:                  w.ID,
		Value:               w.Value,
		Enabled:             w.Enabled,
		Name:                w.Name,
		Priority:            w.Priority,
		StaticIntervalType:  w.StaticIntervalType,
		StaticInterval:      w.StaticInterval,
		StaticRequests:      w.StaticRequests,
		DynamicIntervalType: w.DynamicIntervalType,
		DynamicInterval:     w.DynamicInterval,
		DynamicRequests:     w.DynamicRequests,
		Challenge:           w.Challenge,
		TrustTime:           w.TrustTime,
		AttackBanTime:       w.AttackBanTime,
	}
	for _, ip := range w.WhiteListIPs {
		rule.WhitelistIPs = append(rule.WhitelistIPs, domain.CDNRateLimitWhitelistIP{
			ID: ip.ID, IP: ip.IP, RateLimitRuleID: ip.RateLimitRuleID,
		})
	}
	return rule
}

// whitelistIPsToWire flattens a rule's whitelist down to the plain IP
// strings a store/update request accepts (AGENTS.md: provider-confirmed
// shape, see rateLimitWhitelistIPWire's doc comment). A nil/empty slice
// stays nil so it is omitted from the request rather than sent as [].
func whitelistIPsToWire(ips []domain.CDNRateLimitWhitelistIP) []string {
	if len(ips) == 0 {
		return nil
	}
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = ip.IP
	}
	return out
}

// ListCDNRateLimitRules returns every rate limit rule configured on a zone.
func (c *Client) ListCDNRateLimitRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNRateLimitRule, error) {
	var items []rateLimitRuleWire
	if err := c.doCDNJSON(ctx, creds, "GET", rateLimitRulesPath(zoneUUID), nil, &items); err != nil {
		return nil, fmt.Errorf("list rate limit rules of zone %s: %w", zoneUUID, err)
	}

	rules := make([]domain.CDNRateLimitRule, len(items))
	for i := range items {
		rules[i] = toDomainRateLimitRule(items[i])
	}
	return rules, nil
}

// GetCDNRateLimitRule returns a single rate limit rule by ID.
func (c *Client) GetCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNRateLimitRule, error) {
	var wire rateLimitRuleWire
	if err := c.doCDNJSON(ctx, creds, "GET", rateLimitRulePath(zoneUUID, ruleID), nil, &wire); err != nil {
		return nil, fmt.Errorf("get rate limit rule %s of zone %s: %w", ruleID, zoneUUID, err)
	}
	rule := toDomainRateLimitRule(wire)
	return &rule, nil
}

// rateLimitRuleCreateRequest is the body of POST .../firewalls/rate-limit.
// Unlike rateLimitRuleWire, "enabled" is not accepted here (the provider
// decides the initial state) and the whitelist is a plain IP-string array.
type rateLimitRuleCreateRequest struct {
	Name                string   `json:"name"`
	Value               string   `json:"value"`
	StaticIntervalType  string   `json:"static_interval_type"`
	StaticInterval      int      `json:"static_interval"`
	DynamicIntervalType string   `json:"dynamic_interval_type"`
	DynamicInterval     int      `json:"dynamic_interval"`
	IPReputationEnabled bool     `json:"ip_reputation_enabled,omitempty"`
	Challenge           string   `json:"challenge"`
	TrustTime           int      `json:"trust_time"`
	AttackBanTime       int      `json:"attack_ban_time"`
	WhiteListIPs        []string `json:"white_list_ips,omitempty"`
}

// CreateCDNRateLimitRule adds a new rate limit rule to a zone. The store
// endpoint's response carries no resource body (per the spec's documented
// "data": [] example) — unlike CreateCDNZone there is no rule ID to report
// back, so the returned rule echoes the request with ZoneUUID left for the
// caller to track; call ListCDNRateLimitRules afterward to discover the
// provider-assigned ID.
func (c *Client) CreateCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNRateLimitRule) (*domain.CDNRateLimitRule, error) {
	reqBody := rateLimitRuleCreateRequest{
		Name: rule.Name, Value: rule.Value,
		StaticIntervalType: rule.StaticIntervalType, StaticInterval: rule.StaticInterval,
		DynamicIntervalType: rule.DynamicIntervalType, DynamicInterval: rule.DynamicInterval,
		IPReputationEnabled: rule.IPReputationEnabled,
		Challenge:           rule.Challenge, TrustTime: rule.TrustTime, AttackBanTime: rule.AttackBanTime,
		WhiteListIPs: whitelistIPsToWire(rule.WhitelistIPs),
	}

	if err := c.doCDNJSON(ctx, creds, "POST", rateLimitRulesPath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("creating rate limit rule %q in zone %s: %w", rule.Name, zoneUUID, err)
	}

	created := rule
	return &created, nil
}

// rateLimitRuleUpdateRequest is the body of PUT
// .../firewalls/rate-limit/{id}. Unlike create, "enabled" is required here.
type rateLimitRuleUpdateRequest struct {
	Name                string   `json:"name"`
	Value               string   `json:"value"`
	Enabled             bool     `json:"enabled"`
	StaticIntervalType  string   `json:"static_interval_type"`
	StaticInterval      int      `json:"static_interval"`
	DynamicIntervalType string   `json:"dynamic_interval_type"`
	DynamicInterval     int      `json:"dynamic_interval"`
	IPReputationEnabled bool     `json:"ip_reputation_enabled,omitempty"`
	Challenge           string   `json:"challenge"`
	TrustTime           int      `json:"trust_time"`
	AttackBanTime       int      `json:"attack_ban_time"`
	WhiteListIPs        []string `json:"white_list_ips,omitempty"`
}

// UpdateCDNRateLimitRule replaces a rate limit rule's configuration by ID.
// As with CreateCDNRateLimitRule, the update endpoint's response carries no
// resource body, so the returned rule echoes the request.
func (c *Client) UpdateCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNRateLimitRule) (*domain.CDNRateLimitRule, error) {
	reqBody := rateLimitRuleUpdateRequest{
		Name: rule.Name, Value: rule.Value, Enabled: rule.Enabled,
		StaticIntervalType: rule.StaticIntervalType, StaticInterval: rule.StaticInterval,
		DynamicIntervalType: rule.DynamicIntervalType, DynamicInterval: rule.DynamicInterval,
		IPReputationEnabled: rule.IPReputationEnabled,
		Challenge:           rule.Challenge, TrustTime: rule.TrustTime, AttackBanTime: rule.AttackBanTime,
		WhiteListIPs: whitelistIPsToWire(rule.WhitelistIPs),
	}

	if err := c.doCDNJSON(ctx, creds, "PUT", rateLimitRulePath(zoneUUID, ruleID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("updating rate limit rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}

	updated := rule
	updated.ID = ruleID
	return &updated, nil
}

// DeleteCDNRateLimitRule removes a rate limit rule by ID.
func (c *Client) DeleteCDNRateLimitRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", rateLimitRulePath(zoneUUID, ruleID), nil, nil); err != nil {
		return fmt.Errorf("deleting rate limit rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	return nil
}

// rateLimitRulePriorityRequest is the body of PUT
// .../firewalls/rate-limit/{id}/update-priority.
type rateLimitRulePriorityRequest struct {
	Priority int `json:"priority"`
}

// UpdateCDNRateLimitRulePriority reorders a rule's evaluation priority
// relative to the zone's other rate limit rules. Lower values are evaluated
// first, per the spec's minimum: 1 on the priority field.
func (c *Client) UpdateCDNRateLimitRulePriority(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, priority int) error {
	reqBody := rateLimitRulePriorityRequest{Priority: priority}
	path := rateLimitRulePath(zoneUUID, ruleID) + "/update-priority"
	if err := c.doCDNJSON(ctx, creds, "PUT", path, reqBody, nil); err != nil {
		return fmt.Errorf("updating priority of rate limit rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	return nil
}

// upstreamErrorsWire mirrors GET .../upstream-errors' single-field payload.
type upstreamErrorsWire struct {
	Enabled bool `json:"enabled"`
}

// GetCDNUpstreamErrors returns a zone's current upstream errors setting.
func (c *Client) GetCDNUpstreamErrors(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNUpstreamErrorSettings, error) {
	var wire upstreamErrorsWire
	if err := c.doCDNJSON(ctx, creds, "GET", upstreamErrorsPath(zoneUUID), nil, &wire); err != nil {
		return nil, fmt.Errorf("get upstream errors setting of zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNUpstreamErrorSettings{Enabled: wire.Enabled}, nil
}

// UpdateCDNUpstreamErrors sets a zone's upstream errors flag. As with the
// rate limit rule's update endpoint, the response carries no resource body,
// so the returned setting echoes the requested value.
func (c *Client) UpdateCDNUpstreamErrors(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (*domain.CDNUpstreamErrorSettings, error) {
	reqBody := upstreamErrorsWire{Enabled: enabled}
	if err := c.doCDNJSON(ctx, creds, "PUT", upstreamErrorsPath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("updating upstream errors setting of zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNUpstreamErrorSettings{Enabled: enabled}, nil
}
