package parspack

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// CDN rule engines beyond Zone/DNS (issue #24): Origin Rules, Page Rules and
// Transform Rules. Base paths are confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml's "Origin Rules", "Page Rules" and
// "Transform Rules" tags, relative to Client.cdnBaseURL and nested under the
// same zonesBasePath ("external/api/v1/zones") cdn.go's zone endpoints use.
//
// All operations here are fast (single HTTP round trip each) — there is no
// long-running provisioning state for a rule the way there is for a server.
//
// A create or update call's response body carries an empty "data" array per
// the spec's documented examples — the provider does not echo the created
// resource (and, for create, does not return its new ID either). So, like
// CreateDNSRecord/UpdateDNSRecord in cdn.go, Create* here returns the input
// spec back to the caller (with no ID) and Update* returns it with the ID
// the caller already supplied on the path.

const (
	originRulesSuffix    = "origin-rules"
	pageRulesSuffix      = "page-rules"
	transformRulesSuffix = "transform-rules"
	toggleStatusSuffix   = "toggle-status"
)

func originRulesPath(zoneUUID string) string {
	return zonesBasePath + "/" + zoneUUID + "/" + originRulesSuffix
}

func originRulePath(zoneUUID, ruleID string) string {
	return originRulesPath(zoneUUID) + "/" + ruleID
}

func pageRulesPath(zoneUUID string) string {
	return zonesBasePath + "/" + zoneUUID + "/" + pageRulesSuffix
}

func pageRulePath(zoneUUID, ruleID string) string {
	return pageRulesPath(zoneUUID) + "/" + ruleID
}

func transformRulesPath(zoneUUID string) string {
	return zonesBasePath + "/" + zoneUUID + "/" + transformRulesSuffix
}

func transformRulePath(zoneUUID, ruleID string) string {
	return transformRulesPath(zoneUUID) + "/" + ruleID
}

// toggleRuleRequest is the body of every rule engine's PUT .../toggle-status
// endpoint (Origin Rules and Transform Rules — Page Rules has no such
// endpoint, per the spec).
type toggleRuleRequest struct {
	Enabled bool `json:"enabled"`
}

// ruleConditionWire mirrors one entry of a "rule_items" condition group,
// shared by Origin Rules and Transform Rules. Outgoing write requests only
// ever set Field/Operation/Value — ValueDetail and Bulklist are read-only,
// populated by the provider on get/list.
type ruleConditionWire struct {
	Field       string           `json:"field"`
	Operation   string           `json:"operation"`
	Value       json.RawMessage  `json:"value,omitempty"`
	ValueDetail json.RawMessage  `json:"value_detail,omitempty"`
	Bulklist    *bulklistRefWire `json:"bulklist,omitempty"`
}

// bulklistRefWire is the object a condition's "bulklist" field holds when
// Operation is "from_list".
type bulklistRefWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func toDomainCondition(w ruleConditionWire) domain.CDNRuleCondition {
	c := domain.CDNRuleCondition{
		Field: w.Field, Operation: w.Operation, Value: w.Value, ValueDetail: w.ValueDetail,
	}
	if w.Bulklist != nil {
		c.BulklistID, c.BulklistName, c.BulklistType = w.Bulklist.ID, w.Bulklist.Name, w.Bulklist.Type
	}
	return c
}

func toWireCondition(c domain.CDNRuleCondition) ruleConditionWire {
	return ruleConditionWire{Field: c.Field, Operation: c.Operation, Value: c.Value}
}

func toDomainConditionGroups(wire [][]ruleConditionWire) [][]domain.CDNRuleCondition {
	if wire == nil {
		return nil
	}
	groups := make([][]domain.CDNRuleCondition, len(wire))
	for i, group := range wire {
		conditions := make([]domain.CDNRuleCondition, len(group))
		for j, w := range group {
			conditions[j] = toDomainCondition(w)
		}
		groups[i] = conditions
	}
	return groups
}

func toWireConditionGroups(groups [][]domain.CDNRuleCondition) [][]ruleConditionWire {
	wire := make([][]ruleConditionWire, len(groups))
	for i, group := range groups {
		conditions := make([]ruleConditionWire, len(group))
		for j, c := range group {
			conditions[j] = toWireCondition(c)
		}
		wire[i] = conditions
	}
	return wire
}

// ---------------------------------------------------------------------------
// Origin Rules — route matching traffic in a zone to a specific origin.

// originRuleLoadBalanceRefWire is the object an origin rule's "load_balance"
// field holds on read, when Type is "load_balance".
type originRuleLoadBalanceRefWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// originRuleListItemWire is one entry of GET .../origin-rules.
type originRuleListItemWire struct {
	ID          string                        `json:"id"`
	Name        string                        `json:"name"`
	Type        string                        `json:"type"`
	UpstreamIP  string                        `json:"upstream_ip"`
	Port        int                           `json:"port"`
	LoadBalance *originRuleLoadBalanceRefWire `json:"load_balance"`
	Enabled     bool                          `json:"enabled"`
	Priority    int                           `json:"priority"`
}

// originRuleDetailWire is the response of GET .../origin-rules/{origin_rule},
// which additionally carries the rule's condition tree.
type originRuleDetailWire struct {
	originRuleListItemWire
	RuleItems [][]ruleConditionWire `json:"rule_items"`
}

func toDomainOriginRuleFromList(w originRuleListItemWire) domain.CDNOriginRule {
	r := domain.CDNOriginRule{
		ID: w.ID, Name: w.Name, Type: w.Type, UpstreamIP: w.UpstreamIP, Port: w.Port,
		Enabled: w.Enabled, Priority: w.Priority,
	}
	if w.LoadBalance != nil {
		r.LoadBalanceID, r.LoadBalanceName = w.LoadBalance.ID, w.LoadBalance.Name
	}
	return r
}

func toDomainOriginRuleFromDetail(w originRuleDetailWire) domain.CDNOriginRule {
	r := toDomainOriginRuleFromList(w.originRuleListItemWire)
	r.RuleItems = toDomainConditionGroups(w.RuleItems)
	return r
}

// originRuleWriteRequest is the body of both POST .../origin-rules and
// PUT .../origin-rules/{origin_rule} — the spec documents an identical shape
// for create and update.
type originRuleWriteRequest struct {
	Name          string                `json:"name"`
	Type          string                `json:"type,omitempty"`
	UpstreamIP    string                `json:"upstream_ip,omitempty"`
	Port          int                   `json:"port,omitempty"`
	LoadBalanceID string                `json:"load_balance_id,omitempty"`
	RuleItems     [][]ruleConditionWire `json:"rule_items"`
}

func toOriginRuleWriteRequest(rule domain.CDNOriginRule) originRuleWriteRequest {
	return originRuleWriteRequest{
		Name: rule.Name, Type: rule.Type, UpstreamIP: rule.UpstreamIP, Port: rule.Port,
		LoadBalanceID: rule.LoadBalanceID, RuleItems: toWireConditionGroups(rule.RuleItems),
	}
}

// ListCDNOriginRules returns every origin rule of one zone.
func (c *Client) ListCDNOriginRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNOriginRule, error) {
	var items []originRuleListItemWire
	if err := c.doCDNJSON(ctx, creds, "GET", originRulesPath(zoneUUID), nil, &items); err != nil {
		return nil, fmt.Errorf("list origin rules of zone %s: %w", zoneUUID, err)
	}

	rules := make([]domain.CDNOriginRule, len(items))
	for i := range items {
		rules[i] = toDomainOriginRuleFromList(items[i])
	}
	return rules, nil
}

// GetCDNOriginRule returns a single origin rule of a zone by ID.
func (c *Client) GetCDNOriginRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNOriginRule, error) {
	var detail originRuleDetailWire
	if err := c.doCDNJSON(ctx, creds, "GET", originRulePath(zoneUUID, ruleID), nil, &detail); err != nil {
		return nil, fmt.Errorf("get origin rule %s of zone %s: %w", ruleID, zoneUUID, err)
	}
	rule := toDomainOriginRuleFromDetail(detail)
	return &rule, nil
}

// CreateCDNOriginRule adds a new origin rule to a zone. The provider's
// create endpoint does not echo the created rule or its new ID (the spec
// documents an empty "data" array), so the returned rule mirrors the input
// spec with no ID — call ListCDNOriginRules to discover the assigned ID.
func (c *Client) CreateCDNOriginRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNOriginRule) (*domain.CDNOriginRule, error) {
	reqBody := toOriginRuleWriteRequest(rule)
	if err := c.doCDNJSON(ctx, creds, "POST", originRulesPath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("creating origin rule %q in zone %s: %w", rule.Name, zoneUUID, err)
	}
	created := rule
	return &created, nil
}

// UpdateCDNOriginRule replaces the configuration of an existing origin rule.
func (c *Client) UpdateCDNOriginRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNOriginRule) (*domain.CDNOriginRule, error) {
	reqBody := toOriginRuleWriteRequest(rule)
	if err := c.doCDNJSON(ctx, creds, "PUT", originRulePath(zoneUUID, ruleID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("updating origin rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	updated := rule
	updated.ID = ruleID
	return &updated, nil
}

// DeleteCDNOriginRule removes an origin rule from a zone by ID.
func (c *Client) DeleteCDNOriginRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", originRulePath(zoneUUID, ruleID), nil, nil); err != nil {
		return fmt.Errorf("deleting origin rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	return nil
}

// ToggleCDNOriginRule enables or disables an origin rule without deleting it.
func (c *Client) ToggleCDNOriginRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, enabled bool) error {
	reqBody := toggleRuleRequest{Enabled: enabled}
	if err := c.doCDNJSON(ctx, creds, "PUT", originRulePath(zoneUUID, ruleID)+"/"+toggleStatusSuffix, reqBody, nil); err != nil {
		return fmt.Errorf("toggling origin rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Page Rules — apply per-URL/user-agent settings overrides to matching
// traffic in a zone. No toggle-status endpoint exists for this rule engine.

type pageRuleRedirectWire struct {
	HTTPCode int    `json:"http_code,omitempty"`
	URL      string `json:"url,omitempty"`
}

type pagePortDestinationWire struct {
	HTTP  int `json:"http,omitempty"`
	HTTPS int `json:"https,omitempty"`
}

type pageCookieWire struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	ExpireDate int    `json:"expire_date,omitempty"`
	Path       string `json:"path,omitempty"`
	HTTPOnly   bool   `json:"http_only,omitempty"`
	Secure     bool   `json:"secure,omitempty"`
}

// pageRuleListItemWire is one entry of GET .../page-rules.
type pageRuleListItemWire struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Priority int    `json:"priority"`
}

// pageRuleDetailWire is the response of GET .../page-rules/{page_rule},
// which additionally carries the settings overrides the rule applies.
type pageRuleDetailWire struct {
	pageRuleListItemWire
	Minify            bool                     `json:"minify"`
	CachePolicy       string                   `json:"cache_policy"`
	CacheTTL          int                      `json:"cache_ttl"`
	FirewallStatus    bool                     `json:"firewall_status"`
	FirewallExceptIPs []string                 `json:"firewall_except_ips"`
	WAFIsActive       bool                     `json:"waf_is_active"`
	RemovableHeaders  []string                 `json:"removable_headers"`
	URLRedirection    *pageRuleRedirectWire    `json:"url_redirection"`
	AdditionHeaders   map[string]string        `json:"addition_headers"`
	PortDestination   *pagePortDestinationWire `json:"port_destination"`
	AdditionCookies   []pageCookieWire         `json:"addition_cookies"`
}

func toDomainPageRuleFromList(w pageRuleListItemWire) domain.CDNPageRule {
	return domain.CDNPageRule{
		ID: w.ID, Name: w.Name, Type: w.Type, Operator: w.Operator, Value: w.Value, Priority: w.Priority,
	}
}

func toDomainPageRuleFromDetail(w pageRuleDetailWire) domain.CDNPageRule {
	r := toDomainPageRuleFromList(w.pageRuleListItemWire)
	r.Minify = w.Minify
	r.CachePolicy, _ = domain.ParseDNSRecordProxy(w.CachePolicy)
	r.CacheTTLSeconds = w.CacheTTL
	r.FirewallStatus = w.FirewallStatus
	r.FirewallExceptIPs = w.FirewallExceptIPs
	r.WAFIsActive = w.WAFIsActive
	r.RemovableHeaders = w.RemovableHeaders
	r.AdditionHeaders = w.AdditionHeaders
	if w.URLRedirection != nil {
		r.URLRedirection = &domain.CDNPageRuleRedirect{HTTPCode: w.URLRedirection.HTTPCode, URL: w.URLRedirection.URL}
	}
	if w.PortDestination != nil {
		r.PortDestination = &domain.CDNPagePortDestination{HTTP: w.PortDestination.HTTP, HTTPS: w.PortDestination.HTTPS}
	}
	for _, ck := range w.AdditionCookies {
		r.AdditionCookies = append(r.AdditionCookies, domain.CDNPageRuleCookie{
			Key: ck.Key, Value: ck.Value, ExpireSeconds: ck.ExpireDate, Path: ck.Path,
			HTTPOnly: ck.HTTPOnly, Secure: ck.Secure,
		})
	}
	return r
}

// pageRuleWriteRequest is the body of both POST .../page-rules and
// PUT .../page-rules/{page_rule} — the spec documents an identical shape for
// create and update. FirewallStatus has no omitempty: it is a required field
// even though false is its zero value.
type pageRuleWriteRequest struct {
	Name              string                   `json:"name"`
	Type              string                   `json:"type,omitempty"`
	Operator          string                   `json:"operator,omitempty"`
	Value             string                   `json:"value"`
	Minify            bool                     `json:"minify,omitempty"`
	CachePolicy       string                   `json:"cache_policy,omitempty"`
	CacheTTL          int                      `json:"cache_ttl,omitempty"`
	FirewallStatus    bool                     `json:"firewall_status"`
	FirewallExceptIPs []string                 `json:"firewall_except_ips,omitempty"`
	WAFIsActive       bool                     `json:"waf_is_active,omitempty"`
	RemovableHeaders  []string                 `json:"removable_headers,omitempty"`
	URLRedirection    *pageRuleRedirectWire    `json:"url_redirection,omitempty"`
	AdditionHeaders   map[string]string        `json:"addition_headers,omitempty"`
	PortDestination   *pagePortDestinationWire `json:"port_destination,omitempty"`
	AdditionCookies   []pageCookieWire         `json:"addition_cookies,omitempty"`
}

func toPageRuleWriteRequest(rule domain.CDNPageRule) pageRuleWriteRequest {
	req := pageRuleWriteRequest{
		Name: rule.Name, Type: rule.Type, Operator: rule.Operator, Value: rule.Value, Minify: rule.Minify,
		CacheTTL: rule.CacheTTLSeconds, FirewallStatus: rule.FirewallStatus, FirewallExceptIPs: rule.FirewallExceptIPs,
		WAFIsActive: rule.WAFIsActive, RemovableHeaders: rule.RemovableHeaders, AdditionHeaders: rule.AdditionHeaders,
	}
	if rule.CachePolicy != domain.DNSRecordProxyUnknown {
		req.CachePolicy = rule.CachePolicy.String()
	}
	if rule.URLRedirection != nil {
		req.URLRedirection = &pageRuleRedirectWire{HTTPCode: rule.URLRedirection.HTTPCode, URL: rule.URLRedirection.URL}
	}
	if rule.PortDestination != nil {
		req.PortDestination = &pagePortDestinationWire{HTTP: rule.PortDestination.HTTP, HTTPS: rule.PortDestination.HTTPS}
	}
	for _, ck := range rule.AdditionCookies {
		req.AdditionCookies = append(req.AdditionCookies, pageCookieWire{
			Key: ck.Key, Value: ck.Value, ExpireDate: ck.ExpireSeconds, Path: ck.Path,
			HTTPOnly: ck.HTTPOnly, Secure: ck.Secure,
		})
	}
	return req
}

// ListCDNPageRules returns every page rule of one zone.
func (c *Client) ListCDNPageRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNPageRule, error) {
	var items []pageRuleListItemWire
	if err := c.doCDNJSON(ctx, creds, "GET", pageRulesPath(zoneUUID), nil, &items); err != nil {
		return nil, fmt.Errorf("list page rules of zone %s: %w", zoneUUID, err)
	}

	rules := make([]domain.CDNPageRule, len(items))
	for i := range items {
		rules[i] = toDomainPageRuleFromList(items[i])
	}
	return rules, nil
}

// GetCDNPageRule returns a single page rule of a zone by ID.
func (c *Client) GetCDNPageRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNPageRule, error) {
	var detail pageRuleDetailWire
	if err := c.doCDNJSON(ctx, creds, "GET", pageRulePath(zoneUUID, ruleID), nil, &detail); err != nil {
		return nil, fmt.Errorf("get page rule %s of zone %s: %w", ruleID, zoneUUID, err)
	}
	rule := toDomainPageRuleFromDetail(detail)
	return &rule, nil
}

// CreateCDNPageRule adds a new page rule to a zone. As with origin rules,
// the provider's create endpoint does not echo the created rule or its new
// ID, so the returned rule mirrors the input spec with no ID — call
// ListCDNPageRules to discover the assigned ID.
func (c *Client) CreateCDNPageRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNPageRule) (*domain.CDNPageRule, error) {
	reqBody := toPageRuleWriteRequest(rule)
	if err := c.doCDNJSON(ctx, creds, "POST", pageRulesPath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("creating page rule %q in zone %s: %w", rule.Name, zoneUUID, err)
	}
	created := rule
	return &created, nil
}

// UpdateCDNPageRule replaces the configuration of an existing page rule.
func (c *Client) UpdateCDNPageRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNPageRule) (*domain.CDNPageRule, error) {
	reqBody := toPageRuleWriteRequest(rule)
	if err := c.doCDNJSON(ctx, creds, "PUT", pageRulePath(zoneUUID, ruleID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("updating page rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	updated := rule
	updated.ID = ruleID
	return &updated, nil
}

// DeleteCDNPageRule removes a page rule from a zone by ID.
func (c *Client) DeleteCDNPageRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", pageRulePath(zoneUUID, ruleID), nil, nil); err != nil {
		return fmt.Errorf("deleting page rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Transform Rules — rewrite request/response headers on matching traffic in
// a zone.

type transformHeaderActionWire struct {
	HeaderName  string `json:"header_name"`
	HeaderValue string `json:"header_value,omitempty"`
	Action      string `json:"action"`
}

type transformRuleValuesWire struct {
	RequestHeaders  []transformHeaderActionWire `json:"request_headers"`
	ResponseHeaders []transformHeaderActionWire `json:"response_headers"`
}

func toDomainHeaderActions(wire []transformHeaderActionWire) []domain.CDNTransformHeaderAction {
	if wire == nil {
		return nil
	}
	out := make([]domain.CDNTransformHeaderAction, len(wire))
	for i, w := range wire {
		out[i] = domain.CDNTransformHeaderAction{HeaderName: w.HeaderName, HeaderValue: w.HeaderValue, Action: w.Action}
	}
	return out
}

func toWireHeaderActions(actions []domain.CDNTransformHeaderAction) []transformHeaderActionWire {
	wire := make([]transformHeaderActionWire, len(actions))
	for i, a := range actions {
		wire[i] = transformHeaderActionWire{HeaderName: a.HeaderName, HeaderValue: a.HeaderValue, Action: a.Action}
	}
	return wire
}

// transformRuleListItemWire is one entry of GET .../transform-rules.
type transformRuleListItemWire struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// transformRuleDetailWire is the response of
// GET .../transform-rules/{transform_rule}, which additionally carries the
// header rewrites and the rule's condition tree.
type transformRuleDetailWire struct {
	transformRuleListItemWire
	TransformRuleValues transformRuleValuesWire `json:"transform_rule_values"`
	RuleItems           [][]ruleConditionWire   `json:"rule_items"`
}

func toDomainTransformRuleFromList(w transformRuleListItemWire) domain.CDNTransformRule {
	return domain.CDNTransformRule{ID: w.ID, Name: w.Name, Enabled: w.Enabled, Priority: w.Priority}
}

func toDomainTransformRuleFromDetail(w transformRuleDetailWire) domain.CDNTransformRule {
	r := toDomainTransformRuleFromList(w.transformRuleListItemWire)
	r.RequestHeaders = toDomainHeaderActions(w.TransformRuleValues.RequestHeaders)
	r.ResponseHeaders = toDomainHeaderActions(w.TransformRuleValues.ResponseHeaders)
	r.RuleItems = toDomainConditionGroups(w.RuleItems)
	return r
}

// transformRuleWriteRequest is the body of both POST .../transform-rules and
// PUT .../transform-rules/{transform_rule} — the spec documents an identical
// shape for create and update.
type transformRuleWriteRequest struct {
	Name                string                  `json:"name"`
	TransformRuleValues transformRuleValuesWire `json:"transform_rule_values"`
	RuleItems           [][]ruleConditionWire   `json:"rule_items"`
}

func toTransformRuleWriteRequest(rule domain.CDNTransformRule) transformRuleWriteRequest {
	return transformRuleWriteRequest{
		Name: rule.Name,
		TransformRuleValues: transformRuleValuesWire{
			RequestHeaders:  toWireHeaderActions(rule.RequestHeaders),
			ResponseHeaders: toWireHeaderActions(rule.ResponseHeaders),
		},
		RuleItems: toWireConditionGroups(rule.RuleItems),
	}
}

// ListCDNTransformRules returns every transform rule of one zone.
func (c *Client) ListCDNTransformRules(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNTransformRule, error) {
	var items []transformRuleListItemWire
	if err := c.doCDNJSON(ctx, creds, "GET", transformRulesPath(zoneUUID), nil, &items); err != nil {
		return nil, fmt.Errorf("list transform rules of zone %s: %w", zoneUUID, err)
	}

	rules := make([]domain.CDNTransformRule, len(items))
	for i := range items {
		rules[i] = toDomainTransformRuleFromList(items[i])
	}
	return rules, nil
}

// GetCDNTransformRule returns a single transform rule of a zone by ID.
func (c *Client) GetCDNTransformRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) (*domain.CDNTransformRule, error) {
	var detail transformRuleDetailWire
	if err := c.doCDNJSON(ctx, creds, "GET", transformRulePath(zoneUUID, ruleID), nil, &detail); err != nil {
		return nil, fmt.Errorf("get transform rule %s of zone %s: %w", ruleID, zoneUUID, err)
	}
	rule := toDomainTransformRuleFromDetail(detail)
	return &rule, nil
}

// CreateCDNTransformRule adds a new transform rule to a zone. As with origin
// rules, the provider's create endpoint does not echo the created rule or
// its new ID, so the returned rule mirrors the input spec with no ID — call
// ListCDNTransformRules to discover the assigned ID.
func (c *Client) CreateCDNTransformRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, rule domain.CDNTransformRule) (*domain.CDNTransformRule, error) {
	reqBody := toTransformRuleWriteRequest(rule)
	if err := c.doCDNJSON(ctx, creds, "POST", transformRulesPath(zoneUUID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("creating transform rule %q in zone %s: %w", rule.Name, zoneUUID, err)
	}
	created := rule
	return &created, nil
}

// UpdateCDNTransformRule replaces the configuration of an existing transform
// rule.
func (c *Client) UpdateCDNTransformRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNTransformRule) (*domain.CDNTransformRule, error) {
	reqBody := toTransformRuleWriteRequest(rule)
	if err := c.doCDNJSON(ctx, creds, "PUT", transformRulePath(zoneUUID, ruleID), reqBody, nil); err != nil {
		return nil, fmt.Errorf("updating transform rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	updated := rule
	updated.ID = ruleID
	return &updated, nil
}

// DeleteCDNTransformRule removes a transform rule from a zone by ID.
func (c *Client) DeleteCDNTransformRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string) error {
	if err := c.doCDNJSON(ctx, creds, "DELETE", transformRulePath(zoneUUID, ruleID), nil, nil); err != nil {
		return fmt.Errorf("deleting transform rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	return nil
}

// ToggleCDNTransformRule enables or disables a transform rule without
// deleting it.
func (c *Client) ToggleCDNTransformRule(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, ruleID string, enabled bool) error {
	reqBody := toggleRuleRequest{Enabled: enabled}
	if err := c.doCDNJSON(ctx, creds, "PUT", transformRulePath(zoneUUID, ruleID)+"/"+toggleStatusSuffix, reqBody, nil); err != nil {
		return fmt.Errorf("toggling transform rule %s in zone %s: %w", ruleID, zoneUUID, err)
	}
	return nil
}
