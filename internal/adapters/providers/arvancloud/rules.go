package arvancloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Page Rules, Response Transforms, Redirect (www-redirect) and Host Header
// Whitelist (issue #71), wired to the real CDN API. Base paths are confirmed
// against docs/api-specs/arvancloud-cdn-4.0.yml's "Page Rule", "Response
// Transforms", "Redirect" and "Host header whitelist" tags, relative to
// domainPath (defined in domain.go).
//
// The wire types below mirror the spec's request/response shapes exactly so
// this adapter decodes real ArvanCloud responses correctly. Nothing above
// the adapter boundary ever sees them — every method here translates to/from
// internal/core/domain types. See domain/arvancloud_rules.go's package
// comment for why PageRule's flat wire shape is translated into a
// logically-grouped domain.ArvanCloudPageRule here rather than being modeled
// flat all the way through.

const (
	pageRulesPathSuffix        = "/page-rules"
	responseTransformsSuffix   = "/response-transforms"
	wwwRedirectPathSuffix      = "/settings/www-redirect"
	hostHeaderWhitelistsSuffix = "/host-header-whitelists"
)

func pageRulesPath(domainName string) string         { return domainPath(domainName) + pageRulesPathSuffix }
func pageRulePath(domainName, id string) string      { return pageRulesPath(domainName) + "/" + id }
func pageRulePurgePath(domainName, id string) string { return pageRulePath(domainName, id) + "/purge" }
func pageRuleDiffPath(domainName, id string) string  { return pageRulePath(domainName, id) + "/diff" }

func responseTransformsPath(domainName string) string {
	return domainPath(domainName) + responseTransformsSuffix
}
func responseTransformPath(domainName, id string) string {
	return responseTransformsPath(domainName) + "/" + id
}

func wwwRedirectPath(domainName string) string { return domainPath(domainName) + wwwRedirectPathSuffix }

func hostHeaderWhitelistsPath(domainName string) string {
	return domainPath(domainName) + hostHeaderWhitelistsSuffix
}
func hostHeaderWhitelistSettingsPath(domainName string) string {
	return hostHeaderWhitelistsPath(domainName) + "/settings"
}
func hostHeaderWhitelistEntryPath(domainName, targetAccount string) string {
	return hostHeaderWhitelistsPath(domainName) + "/" + targetAccount
}

// boolOrDefault dereferences p, or returns def when p is nil (the field was
// absent from the response). Used throughout this file's *bool wire fields,
// the same "response may omit a field with its own provider-side default"
// handling healthCheckWire.Status and loadBalancerWire's own Status fields
// use, just factored into one helper given how many boolean fields PageRule
// has.
func boolOrDefault(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}

// --- shared sub-object wire types ------------------------------------------

// accelerationWire mirrors the Acceleration schema, shared by
// PageRule.acceleration and PageRuleDiff.acceleration (and, later, AC12's
// standalone acceleration endpoint — see
// domain.ArvanCloudAccelerationSettings' own doc comment).
type accelerationWire struct {
	Status     string   `json:"status,omitempty"`
	Extensions []string `json:"extensions,omitempty"`
}

func toAccelerationDomain(w *accelerationWire) domain.ArvanCloudAccelerationSettings {
	if w == nil {
		return domain.ArvanCloudAccelerationSettings{}
	}
	extensions := make([]domain.ArvanCloudAccelerationExtension, len(w.Extensions))
	for i, e := range w.Extensions {
		extensions[i] = domain.ArvanCloudAccelerationExtension(e)
	}
	return domain.ArvanCloudAccelerationSettings{Status: domain.ArvanCloudAccelerationStatus(w.Status), Extensions: extensions}
}

func accelerationRequestBody(a domain.ArvanCloudAccelerationSettings) map[string]any {
	extensions := make([]string, len(a.Extensions))
	for i, e := range a.Extensions {
		extensions[i] = string(e)
	}
	return map[string]any{"status": string(a.Status), "extensions": extensions}
}

// imageResizeWire mirrors PageRuleImageResize (also reused, field-for-field,
// by PageRuleDiff's inline image_resize object).
type imageResizeWire struct {
	Status   string `json:"status,omitempty"`
	HeightBy string `json:"height_by,omitempty"`
	WidthBy  string `json:"width_by,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

func toImageResizeDomain(w *imageResizeWire) domain.ArvanCloudPageRuleImageResize {
	if w == nil {
		return domain.ArvanCloudPageRuleImageResize{}
	}
	return domain.ArvanCloudPageRuleImageResize{
		Status: domain.ArvanCloudPageRuleImageResizeStatus(w.Status), HeightBy: w.HeightBy, WidthBy: w.WidthBy,
		Mode: domain.ArvanCloudImageResizeMode(w.Mode),
	}
}

func imageResizeRequestBody(r domain.ArvanCloudPageRuleImageResize) map[string]any {
	return map[string]any{
		"status": string(r.Status), "height_by": r.HeightBy, "width_by": r.WidthBy, "mode": string(r.Mode),
	}
}

// upstreamTimeoutWire mirrors UpstreamTimeout, all fields in seconds.
type upstreamTimeoutWire struct {
	ConnectTimeout int `json:"connect_timeout,omitempty"`
	ReadTimeout    int `json:"read_timeout,omitempty"`
	SendTimeout    int `json:"send_timeout,omitempty"`
}

func toUpstreamTimeoutDomain(w *upstreamTimeoutWire) domain.ArvanCloudUpstreamTimeout {
	if w == nil {
		return domain.ArvanCloudUpstreamTimeout{}
	}
	return domain.ArvanCloudUpstreamTimeout{
		ConnectTimeoutSeconds: w.ConnectTimeout, ReadTimeoutSeconds: w.ReadTimeout, SendTimeoutSeconds: w.SendTimeout,
	}
}

func upstreamTimeoutRequestBody(t domain.ArvanCloudUpstreamTimeout) map[string]any {
	return map[string]any{
		"connect_timeout": t.ConnectTimeoutSeconds, "read_timeout": t.ReadTimeoutSeconds, "send_timeout": t.SendTimeoutSeconds,
	}
}

// pageRuleRedirectWire mirrors PageRuleRedirect (also reused, field-for-field,
// by PageRuleDiff's inline redirect object).
type pageRuleRedirectWire struct {
	Enable     *bool  `json:"enable,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	URL        string `json:"url,omitempty"`
}

func toPageRuleRedirectDomain(w *pageRuleRedirectWire) domain.ArvanCloudPageRuleRedirect {
	if w == nil {
		return domain.ArvanCloudPageRuleRedirect{}
	}
	return domain.ArvanCloudPageRuleRedirect{Enable: boolOrDefault(w.Enable, false), StatusCode: w.StatusCode, URL: w.URL}
}

func pageRuleRedirectRequestBody(r domain.ArvanCloudPageRuleRedirect) map[string]any {
	return map[string]any{"enable": r.Enable, "status_code": r.StatusCode, "url": r.URL}
}

// headerEntryWire mirrors one req_custom_headers/res_custom_headers entry on
// PageRule (name/value only — the PageRuleDiff variant additionally carries
// is_var, modeled separately as headerExceptionEntryWire below).
type headerEntryWire struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

func toHeaderEntriesDomain(items []headerEntryWire) []domain.ArvanCloudPageRuleHeaderEntry {
	if len(items) == 0 {
		return nil
	}
	out := make([]domain.ArvanCloudPageRuleHeaderEntry, len(items))
	for i, h := range items {
		out[i] = domain.ArvanCloudPageRuleHeaderEntry{Name: h.Name, Value: h.Value}
	}
	return out
}

func headerEntriesRequestBody(items []domain.ArvanCloudPageRuleHeaderEntry) []map[string]any {
	out := make([]map[string]any, len(items))
	for i, h := range items {
		out[i] = map[string]any{"name": h.Name, "value": h.Value}
	}
	return out
}

// headerExceptionEntryWire mirrors one PageRuleDiff.req_custom_headers /
// .res_custom_headers entry: like headerEntryWire, plus is_var (a
// string-typed "true"/"false" enum per the spec — see
// domain.ArvanCloudPageRuleExceptionHeaderEntry's own doc comment).
type headerExceptionEntryWire struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
	IsVar string `json:"is_var,omitempty"`
}

func toHeaderExceptionEntriesDomain(items []headerExceptionEntryWire) []domain.ArvanCloudPageRuleExceptionHeaderEntry {
	if len(items) == 0 {
		return nil
	}
	out := make([]domain.ArvanCloudPageRuleExceptionHeaderEntry, len(items))
	for i, h := range items {
		out[i] = domain.ArvanCloudPageRuleExceptionHeaderEntry{Name: h.Name, Value: h.Value, IsVar: h.IsVar}
	}
	return out
}

func headerExceptionEntriesRequestBody(items []domain.ArvanCloudPageRuleExceptionHeaderEntry) []map[string]any {
	out := make([]map[string]any, len(items))
	for i, h := range items {
		entry := map[string]any{"name": h.Name, "value": h.Value}
		if h.IsVar != "" {
			entry["is_var"] = h.IsVar
		}
		out[i] = entry
	}
	return out
}

// --- Page Rules -------------------------------------------------------

// pageRuleWire mirrors the flat PageRule schema (an allOf of PageRuleSummary
// plus roughly 30 more top-level properties — see domain/arvancloud_rules.go's
// package comment). Covers both the create/update request body and the
// get/create/update response.
type pageRuleWire struct {
	ID          string `json:"id,omitempty"`
	DomainID    string `json:"domain_id,omitempty"`
	Seq         int    `json:"seq"`
	URLType     string `json:"url_type,omitempty"`
	IsProtected *bool  `json:"is_protected,omitempty"`
	URL         string `json:"url,omitempty"`
	CacheLevel  string `json:"cache_level,omitempty"`
	WAFStatus   *bool  `json:"waf_status,omitempty"`
	FWStatus    *bool  `json:"fw_status,omitempty"`

	Acceleration *accelerationWire `json:"acceleration,omitempty"`
	SlinkStatus  *bool             `json:"slink_status,omitempty"`
	Status       *bool             `json:"status,omitempty"`
	CreatedAt    string            `json:"created_at,omitempty"`
	UpdatedAt    string            `json:"updated_at,omitempty"`

	Cache200        string `json:"cache_200,omitempty"`
	CacheAny        string `json:"cache_any,omitempty"`
	CacheCookie     string `json:"cache_cookie,omitempty"`
	CacheDeviceType *bool  `json:"cache_device_type,omitempty"`
	CacheArgs       *bool  `json:"cache_args,omitempty"`
	CacheArg        string `json:"cache_arg,omitempty"`
	CacheScheme     *bool  `json:"cache_scheme,omitempty"`
	CacheBrowser    string `json:"cache_browser,omitempty"`
	CacheIgnoreSC   *bool  `json:"cache_ignore_sc,omitempty"`
	CacheIgnoreVary *bool  `json:"cache_ignore_vary,omitempty"`
	CacheIgnoreCC   *bool  `json:"cache_ignore_cc,omitempty"`

	CORSHeader  string   `json:"cors_header,omitempty"`
	RewriteURL  string   `json:"rewrite_url,omitempty"`
	SlinkSecret string   `json:"slink_secret,omitempty"`
	SlinkMD5    []string `json:"slink_md5,omitempty"`

	LoadBalancer  *string `json:"load_balancer,omitempty"`
	EdgeComputeID *string `json:"edge_compute_id,omitempty"`
	ClusterStatus *bool   `json:"cluster_status,omitempty"`
	ClusterID     *string `json:"cluster_id,omitempty"`

	ImageResize     *imageResizeWire     `json:"image_resize,omitempty"`
	UpstreamTimeout *upstreamTimeoutWire `json:"upstream_timeout,omitempty"`

	ReqCustomHeaders []headerEntryWire `json:"req_custom_headers,omitempty"`
	ResCustomHeaders []headerEntryWire `json:"res_custom_headers,omitempty"`
	ReqHideHeaders   []string          `json:"req_hide_headers,omitempty"`
	ResHideHeaders   []string          `json:"res_hide_headers,omitempty"`
	CustomHostHeader string            `json:"custom_host_header,omitempty"`

	Redirect *pageRuleRedirectWire `json:"redirect,omitempty"`
}

func strOrEmpty(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

func toPageRuleDomain(w pageRuleWire) domain.ArvanCloudPageRule {
	slinkMD5 := make([]domain.ArvanCloudPageRuleSlinkMD5Field, len(w.SlinkMD5))
	for i, v := range w.SlinkMD5 {
		slinkMD5[i] = domain.ArvanCloudPageRuleSlinkMD5Field(v)
	}
	return domain.ArvanCloudPageRule{
		ID:       w.ID,
		DomainID: w.DomainID,
		Matching: domain.ArvanCloudPageRuleMatching{
			URLType:     domain.ArvanCloudPageRuleURLType(w.URLType),
			URL:         w.URL,
			Seq:         w.Seq,
			IsProtected: boolOrDefault(w.IsProtected, false),
			Status:      boolOrDefault(w.Status, true),
		},
		Caching: domain.ArvanCloudPageRuleCaching{
			CacheLevel:      domain.ArvanCloudPageRuleCacheLevel(w.CacheLevel),
			Cache200:        domain.ArvanCloudCacheTTL(w.Cache200),
			CacheAny:        domain.ArvanCloudCacheTTL(w.CacheAny),
			CacheBrowser:    domain.ArvanCloudCacheTTL(w.CacheBrowser),
			CacheCookie:     w.CacheCookie,
			CacheDeviceType: boolOrDefault(w.CacheDeviceType, false),
			CacheArgs:       boolOrDefault(w.CacheArgs, true),
			CacheArg:        w.CacheArg,
			CacheScheme:     boolOrDefault(w.CacheScheme, true),
			CacheIgnoreSC:   boolOrDefault(w.CacheIgnoreSC, false),
			CacheIgnoreVary: boolOrDefault(w.CacheIgnoreVary, true),
			CacheIgnoreCC:   boolOrDefault(w.CacheIgnoreCC, true),
		},
		Security: domain.ArvanCloudPageRuleSecurity{
			WAFStatus:   boolOrDefault(w.WAFStatus, true),
			FWStatus:    boolOrDefault(w.FWStatus, true),
			SlinkStatus: boolOrDefault(w.SlinkStatus, false),
			SlinkSecret: w.SlinkSecret,
			SlinkMD5:    slinkMD5,
		},
		Routing: domain.ArvanCloudPageRuleRouting{
			Redirect:      toPageRuleRedirectDomain(w.Redirect),
			RewriteURL:    w.RewriteURL,
			LoadBalancer:  strOrEmpty(w.LoadBalancer),
			EdgeComputeID: strOrEmpty(w.EdgeComputeID),
			ClusterStatus: boolOrDefault(w.ClusterStatus, false),
			ClusterID:     strOrEmpty(w.ClusterID),
		},
		Headers: domain.ArvanCloudPageRuleHeaders{
			CORSHeader:       w.CORSHeader,
			ReqCustomHeaders: toHeaderEntriesDomain(w.ReqCustomHeaders),
			ResCustomHeaders: toHeaderEntriesDomain(w.ResCustomHeaders),
			ReqHideHeaders:   w.ReqHideHeaders,
			ResHideHeaders:   w.ResHideHeaders,
			CustomHostHeader: w.CustomHostHeader,
		},
		Other: domain.ArvanCloudPageRuleOther{
			Acceleration:    toAccelerationDomain(w.Acceleration),
			ImageResize:     toImageResizeDomain(w.ImageResize),
			UpstreamTimeout: toUpstreamTimeoutDomain(w.UpstreamTimeout),
		},
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	}
}

// pageRuleRequestBody builds the JSON body for a page-rule create/update as a
// plain map. Two different omission rules apply, matched to the spec's own
// field-by-field defaults:
//
//   - Booleans always reach the provider explicitly, the same "explicit
//     false must reach the provider" reasoning healthCheckRequestBody
//     documents: several PageRule booleans (waf_status, cache_args, ...)
//     default to true, so an explicit false disabling one must not be
//     dropped by encoding/json's omitempty.
//   - Optional string/enum fields whose spec-side default is a real,
//     non-empty value (cache_200's "30m", url_type's "default", cors_header's
//     "-", ...) are OMITTED entirely when left unset (Go zero value ""),
//     rather than sent as a literal empty string — an empty string is not a
//     valid enum member and would fail the provider's own validation, and is
//     not the same thing as "let the provider apply its default". Nested
//     sub-objects (acceleration/image_resize/upstream_timeout/redirect) are
//     omitted the same way when every one of their own fields is left unset.
func pageRuleRequestBody(pr domain.ArvanCloudPageRule) map[string]any {
	body := map[string]any{
		"seq":                pr.Matching.Seq,
		"url":                pr.Matching.URL,
		"status":             pr.Matching.Status,
		"cache_cookie":       pr.Caching.CacheCookie,
		"cache_device_type":  pr.Caching.CacheDeviceType,
		"cache_args":         pr.Caching.CacheArgs,
		"cache_arg":          pr.Caching.CacheArg,
		"cache_scheme":       pr.Caching.CacheScheme,
		"cache_ignore_sc":    pr.Caching.CacheIgnoreSC,
		"cache_ignore_vary":  pr.Caching.CacheIgnoreVary,
		"cache_ignore_cc":    pr.Caching.CacheIgnoreCC,
		"waf_status":         pr.Security.WAFStatus,
		"fw_status":          pr.Security.FWStatus,
		"slink_status":       pr.Security.SlinkStatus,
		"load_balancer":      nullableString(pr.Routing.LoadBalancer),
		"edge_compute_id":    nullableString(pr.Routing.EdgeComputeID),
		"cluster_status":     pr.Routing.ClusterStatus,
		"cluster_id":         nullableString(pr.Routing.ClusterID),
		"req_custom_headers": headerEntriesRequestBody(pr.Headers.ReqCustomHeaders),
		"res_custom_headers": headerEntriesRequestBody(pr.Headers.ResCustomHeaders),
		"req_hide_headers":   pr.Headers.ReqHideHeaders,
		"res_hide_headers":   pr.Headers.ResHideHeaders,
		"custom_host_header": pr.Headers.CustomHostHeader,
	}
	setIfNonEmpty(body, "url_type", string(pr.Matching.URLType))
	setIfNonEmpty(body, "cache_level", string(pr.Caching.CacheLevel))
	setIfNonEmpty(body, "cache_200", string(pr.Caching.Cache200))
	setIfNonEmpty(body, "cache_any", string(pr.Caching.CacheAny))
	setIfNonEmpty(body, "cache_browser", string(pr.Caching.CacheBrowser))
	setIfNonEmpty(body, "cors_header", pr.Headers.CORSHeader)
	setIfNonEmpty(body, "rewrite_url", pr.Routing.RewriteURL)
	setIfNonEmpty(body, "slink_secret", pr.Security.SlinkSecret)
	if len(pr.Security.SlinkMD5) > 0 {
		body["slink_md5"] = slinkMD5RequestBody(pr.Security.SlinkMD5)
	}
	if accelerationIsSet(pr.Other.Acceleration) {
		body["acceleration"] = accelerationRequestBody(pr.Other.Acceleration)
	}
	if imageResizeIsSet(pr.Other.ImageResize) {
		body["image_resize"] = imageResizeRequestBody(pr.Other.ImageResize)
	}
	if upstreamTimeoutIsSet(pr.Other.UpstreamTimeout) {
		body["upstream_timeout"] = upstreamTimeoutRequestBody(pr.Other.UpstreamTimeout)
	}
	if redirectIsSet(pr.Routing.Redirect) {
		body["redirect"] = pageRuleRedirectRequestBody(pr.Routing.Redirect)
	}
	return body
}

// setIfNonEmpty sets body[key] = value only when value is non-empty, leaving
// the key absent (so the provider applies its own default) otherwise.
func setIfNonEmpty(body map[string]any, key, value string) {
	if value != "" {
		body[key] = value
	}
}

func slinkMD5RequestBody(fields []domain.ArvanCloudPageRuleSlinkMD5Field) []string {
	out := make([]string, len(fields))
	for i, v := range fields {
		out[i] = string(v)
	}
	return out
}

func accelerationIsSet(a domain.ArvanCloudAccelerationSettings) bool {
	return a.Status != "" || len(a.Extensions) > 0
}

func imageResizeIsSet(r domain.ArvanCloudPageRuleImageResize) bool {
	return r.Status != "" || r.HeightBy != "" || r.WidthBy != "" || r.Mode != ""
}

func upstreamTimeoutIsSet(t domain.ArvanCloudUpstreamTimeout) bool {
	return t.ConnectTimeoutSeconds != 0 || t.ReadTimeoutSeconds != 0 || t.SendTimeoutSeconds != 0
}

func redirectIsSet(r domain.ArvanCloudPageRuleRedirect) bool {
	return r.Enable || r.StatusCode != 0 || r.URL != ""
}

// nullableString renders s as nil when empty, matching the spec's nullable
// string fields (load_balancer/edge_compute_id/cluster_id) — sending "" would
// mean something different (a load balancer literally named ""), while null
// means "unset".
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// pageRuleSummaryWire mirrors PageRuleSummary, the shape page-rules.index
// actually returns (a subset of the full PageRule fields).
type pageRuleSummaryWire struct {
	ID           string            `json:"id,omitempty"`
	DomainID     string            `json:"domain_id,omitempty"`
	Seq          int               `json:"seq"`
	URLType      string            `json:"url_type,omitempty"`
	IsProtected  *bool             `json:"is_protected,omitempty"`
	URL          string            `json:"url,omitempty"`
	CacheLevel   string            `json:"cache_level,omitempty"`
	WAFStatus    *bool             `json:"waf_status,omitempty"`
	FWStatus     *bool             `json:"fw_status,omitempty"`
	Acceleration *accelerationWire `json:"acceleration,omitempty"`
	SlinkStatus  *bool             `json:"slink_status,omitempty"`
	Status       *bool             `json:"status,omitempty"`
	CreatedAt    string            `json:"created_at,omitempty"`
	UpdatedAt    string            `json:"updated_at,omitempty"`
}

func toPageRuleSummaryDomain(w pageRuleSummaryWire) domain.ArvanCloudPageRuleSummary {
	return domain.ArvanCloudPageRuleSummary{
		ID: w.ID, DomainID: w.DomainID, Seq: w.Seq,
		URLType:      domain.ArvanCloudPageRuleURLType(w.URLType),
		IsProtected:  boolOrDefault(w.IsProtected, false),
		URL:          w.URL,
		CacheLevel:   domain.ArvanCloudPageRuleCacheLevel(w.CacheLevel),
		WAFStatus:    boolOrDefault(w.WAFStatus, true),
		FWStatus:     boolOrDefault(w.FWStatus, true),
		Acceleration: toAccelerationDomain(w.Acceleration),
		SlinkStatus:  boolOrDefault(w.SlinkStatus, false),
		Status:       boolOrDefault(w.Status, true),
		CreatedAt:    w.CreatedAt,
		UpdatedAt:    w.UpdatedAt,
	}
}

// pageRuleListQueryValues builds the query string ListArvanCloudPageRules
// sends (page-rules.index's search/per_page/page/order parameters).
func pageRuleListQueryValues(q domain.ArvanCloudPageRuleListQuery) url.Values {
	values := url.Values{}
	if q.Search != "" {
		values.Set("search", q.Search)
	}
	if q.PerPage > 0 {
		values.Set("per_page", strconv.Itoa(q.PerPage))
	}
	if q.Page > 0 {
		values.Set("page", strconv.Itoa(q.Page))
	}
	if q.Order != "" {
		values.Set("order", q.Order)
	}
	return values
}

// paginatedEnvelope mirrors PaginatedResponse's top-level shape: data plus
// links/meta as SIBLINGS of data, not nested one level deeper — the same
// shape healthCheckDetailsEnvelope documents for the health-check details
// report. Reused here for both page-rules.index and
// response_transforms.index, whose data item type differs, so this type is
// generic over the raw item JSON and decoded with encoding/json's
// json.RawMessage per item... actually decoded directly against a
// caller-supplied item type via toPaginatedDomain's callers.
type paginatedEnvelope[T any] struct {
	Data []T                       `json:"data"`
	Meta paginatedResponseMetaWire `json:"meta"`
}

// decodePaginated unmarshals a paginatedEnvelope[T] from raw, tolerating an
// empty body (an unlikely but harmless response for a list endpoint) the
// same way doJSON tolerates an empty envelope.
func decodePaginated[T any](raw []byte) (paginatedEnvelope[T], error) {
	var envelope paginatedEnvelope[T]
	if len(raw) == 0 {
		return envelope, nil
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return paginatedEnvelope[T]{}, err
	}
	return envelope, nil
}

func toPageMeta(m paginatedResponseMetaWire) domain.ArvanCloudPageRulePageMeta {
	return domain.ArvanCloudPageRulePageMeta{
		CurrentPage: m.CurrentPage, From: m.From, LastPage: m.LastPage, PerPage: m.PerPage, To: m.To, Total: m.Total,
	}
}

// ListArvanCloudPageRules returns one page of domainName's page rules.
func (p *Provider) ListArvanCloudPageRules(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudPageRuleListQuery) ([]domain.ArvanCloudPageRuleSummary, domain.ArvanCloudPageRulePageMeta, error) {
	path := pageRulesPath(domainName) + "?" + pageRuleListQueryValues(query).Encode()
	raw, err := p.client.doRawGET(ctx, creds, path, "application/json")
	if err != nil {
		return nil, domain.ArvanCloudPageRulePageMeta{}, fmt.Errorf("listing arvancloud page rules of domain %q: %w", domainName, err)
	}
	envelope, err := decodePaginated[pageRuleSummaryWire](raw)
	if err != nil {
		return nil, domain.ArvanCloudPageRulePageMeta{}, fmt.Errorf("decoding arvancloud page rule list of domain %q: %w", domainName, err)
	}
	rules := make([]domain.ArvanCloudPageRuleSummary, len(envelope.Data))
	for i, w := range envelope.Data {
		rules[i] = toPageRuleSummaryDomain(w)
	}
	return rules, toPageMeta(envelope.Meta), nil
}

// CreateArvanCloudPageRule creates a new page rule.
func (p *Provider) CreateArvanCloudPageRule(ctx context.Context, creds domain.ProviderCredentials, domainName string, rule domain.ArvanCloudPageRule) (*domain.ArvanCloudPageRule, error) {
	body := pageRuleRequestBody(rule)
	var wire pageRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, pageRulesPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud page rule on domain %q: %w", domainName, err)
	}
	created := toPageRuleDomain(wire)
	return &created, nil
}

// GetArvanCloudPageRule returns a single page rule by id.
func (p *Provider) GetArvanCloudPageRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudPageRule, error) {
	var wire pageRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, pageRulePath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud page rule %q on domain %q: %w", id, domainName, err)
	}
	found := toPageRuleDomain(wire)
	return &found, nil
}

// UpdateArvanCloudPageRule replaces a page rule's fields via PUT.
func (p *Provider) UpdateArvanCloudPageRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rule domain.ArvanCloudPageRule) (*domain.ArvanCloudPageRule, error) {
	body := pageRuleRequestBody(rule)
	var wire pageRuleWire
	if err := p.client.doJSON(ctx, creds, http.MethodPut, pageRulePath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud page rule %q on domain %q: %w", id, domainName, err)
	}
	updated := toPageRuleDomain(wire)
	return &updated, nil
}

// SetArvanCloudPageRuleStatus toggles only a page rule's status via PATCH.
// The endpoint's response carries no data, only a confirmation message.
func (p *Provider) SetArvanCloudPageRuleStatus(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, status bool) error {
	body := map[string]any{"status": status}
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, pageRulePath(domainName, id), body, nil); err != nil {
		return fmt.Errorf("setting arvancloud page rule %q status on domain %q: %w", id, domainName, err)
	}
	return nil
}

// DeleteArvanCloudPageRule removes a page rule by id.
func (p *Provider) DeleteArvanCloudPageRule(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, pageRulePath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud page rule %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// PurgeArvanCloudPageRuleCache purges cached content for URLs matching this
// page rule. The endpoint's response carries no data.
func (p *Provider) PurgeArvanCloudPageRuleCache(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, pageRulePurgePath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("purging cache for arvancloud page rule %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// --- Page Rule exceptions ("diff") -----------------------------------------

// pageRuleDiffWire mirrors PageRuleDiff, used for both
// page-rules.diff.show's response and page-rules.diff.update's request body
// (see domain.ArvanCloudPageRuleExceptions' own doc comment on why the
// spec's literal PageRule requestBody $ref for diff.update is not followed
// here — this adapter encodes/decodes the diff.update request as
// PageRuleDiff, matching what the endpoint's own response and the
// diff.show endpoint both declare).
type pageRuleDiffWire struct {
	URL              string                     `json:"url,omitempty"`
	CacheLevel       string                     `json:"cache_level,omitempty"`
	WAFStatus        *bool                      `json:"waf_status,omitempty"`
	FWStatus         *bool                      `json:"fw_status,omitempty"`
	Acceleration     *accelerationWire          `json:"acceleration,omitempty"`
	SlinkStatus      *bool                      `json:"slink_status,omitempty"`
	Status           *bool                      `json:"status,omitempty"`
	Cache200         string                     `json:"cache_200,omitempty"`
	CacheAny         string                     `json:"cache_any,omitempty"`
	CacheCookie      string                     `json:"cache_cookie,omitempty"`
	CacheArgs        *bool                      `json:"cache_args,omitempty"`
	CacheArg         string                     `json:"cache_arg,omitempty"`
	CacheScheme      *bool                      `json:"cache_scheme,omitempty"`
	CacheBrowser     string                     `json:"cache_browser,omitempty"`
	CacheIgnoreSC    *bool                      `json:"cache_ignore_sc,omitempty"`
	CacheIgnoreVary  *bool                      `json:"cache_ignore_vary,omitempty"`
	CacheIgnoreCC    *bool                      `json:"cache_ignore_cc,omitempty"`
	CORSHeader       string                     `json:"cors_header,omitempty"`
	RewriteURL       string                     `json:"rewrite_url,omitempty"`
	SlinkSecret      string                     `json:"slink_secret,omitempty"`
	SlinkMD5         []string                   `json:"slink_md5,omitempty"`
	ClusterStatus    *bool                      `json:"cluster_status,omitempty"`
	ClusterID        string                     `json:"cluster_id,omitempty"`
	EdgeComputeID    string                     `json:"edge_compute_id,omitempty"`
	UpstreamTimeout  *upstreamTimeoutWire       `json:"upstream_timeout,omitempty"`
	ReqCustomHeaders []headerExceptionEntryWire `json:"req_custom_headers,omitempty"`
	CacheDeviceType  *bool                      `json:"cache_device_type,omitempty"`
	ImageResize      *imageResizeWire           `json:"image_resize,omitempty"`
	LoadBalancer     string                     `json:"load_balancer,omitempty"`
	ResCustomHeaders []headerExceptionEntryWire `json:"res_custom_headers,omitempty"`
	ReqHideHeaders   []string                   `json:"req_hide_headers,omitempty"`
	ResHideHeaders   []string                   `json:"res_hide_headers,omitempty"`
	CustomHostHeader string                     `json:"custom_host_header,omitempty"`
	Redirect         *pageRuleRedirectWire      `json:"redirect,omitempty"`
}

func toPageRuleExceptionsDomain(w pageRuleDiffWire) domain.ArvanCloudPageRuleExceptions {
	slinkMD5 := make([]domain.ArvanCloudPageRuleSlinkMD5Field, len(w.SlinkMD5))
	for i, v := range w.SlinkMD5 {
		slinkMD5[i] = domain.ArvanCloudPageRuleSlinkMD5Field(v)
	}
	return domain.ArvanCloudPageRuleExceptions{
		URL: w.URL, CacheLevel: domain.ArvanCloudPageRuleCacheLevel(w.CacheLevel),
		WAFStatus: boolOrDefault(w.WAFStatus, false), FWStatus: boolOrDefault(w.FWStatus, false),
		Acceleration: toAccelerationDomain(w.Acceleration), SlinkStatus: boolOrDefault(w.SlinkStatus, false),
		Status: boolOrDefault(w.Status, false), Cache200: domain.ArvanCloudCacheTTL(w.Cache200),
		CacheAny: domain.ArvanCloudCacheTTL(w.CacheAny), CacheCookie: w.CacheCookie,
		CacheArgs: boolOrDefault(w.CacheArgs, false), CacheArg: w.CacheArg,
		CacheScheme: boolOrDefault(w.CacheScheme, false), CacheBrowser: domain.ArvanCloudCacheTTL(w.CacheBrowser),
		CacheIgnoreSC: boolOrDefault(w.CacheIgnoreSC, false), CacheIgnoreVary: boolOrDefault(w.CacheIgnoreVary, false),
		CacheIgnoreCC: boolOrDefault(w.CacheIgnoreCC, false), CORSHeader: w.CORSHeader, RewriteURL: w.RewriteURL,
		SlinkSecret: w.SlinkSecret, SlinkMD5: slinkMD5, ClusterStatus: boolOrDefault(w.ClusterStatus, false),
		ClusterID: w.ClusterID, EdgeComputeID: w.EdgeComputeID, UpstreamTimeout: toUpstreamTimeoutDomain(w.UpstreamTimeout),
		ReqCustomHeaders: toHeaderExceptionEntriesDomain(w.ReqCustomHeaders), CacheDeviceType: boolOrDefault(w.CacheDeviceType, false),
		ImageResize: toImageResizeDomain(w.ImageResize), LoadBalancer: w.LoadBalancer,
		ResCustomHeaders: toHeaderExceptionEntriesDomain(w.ResCustomHeaders), ReqHideHeaders: w.ReqHideHeaders,
		ResHideHeaders: w.ResHideHeaders, CustomHostHeader: w.CustomHostHeader, Redirect: toPageRuleRedirectDomain(w.Redirect),
	}
}

// pageRuleExceptionsRequestBody builds the JSON body for
// page-rules.diff.update. Unlike pageRuleRequestBody's full-replace PUT
// semantics, every field here is an optional override with no non-empty
// spec-side default to fall back to (PageRuleDiff declares no "default" on
// any of its properties) — so every field, boolean fields included, is
// omitted when left at its Go zero value, rather than always sent. This does
// mean an exceptions struct cannot distinguish "explicitly override to
// false" from "no override" for a boolean field; the same limitation
// domain.ArvanCloudLoadBalancerSettings.MaxFails' own doc comment documents
// for a zero-valued int, applied here to every field of an inherently sparse
// "diff" object.
func pageRuleExceptionsRequestBody(e domain.ArvanCloudPageRuleExceptions) map[string]any {
	body := map[string]any{}
	setIfNonEmpty(body, "url", e.URL)
	setIfNonEmpty(body, "cache_level", string(e.CacheLevel))
	setIfNonEmpty(body, "cache_200", string(e.Cache200))
	setIfNonEmpty(body, "cache_any", string(e.CacheAny))
	setIfNonEmpty(body, "cache_browser", string(e.CacheBrowser))
	setIfNonEmpty(body, "cache_cookie", e.CacheCookie)
	setIfNonEmpty(body, "cache_arg", e.CacheArg)
	setIfNonEmpty(body, "cors_header", e.CORSHeader)
	setIfNonEmpty(body, "rewrite_url", e.RewriteURL)
	setIfNonEmpty(body, "slink_secret", e.SlinkSecret)
	setIfNonEmpty(body, "cluster_id", e.ClusterID)
	setIfNonEmpty(body, "edge_compute_id", e.EdgeComputeID)
	setIfNonEmpty(body, "load_balancer", e.LoadBalancer)
	setIfNonEmpty(body, "custom_host_header", e.CustomHostHeader)
	if e.WAFStatus {
		body["waf_status"] = true
	}
	if e.FWStatus {
		body["fw_status"] = true
	}
	if e.SlinkStatus {
		body["slink_status"] = true
	}
	if e.Status {
		body["status"] = true
	}
	if e.CacheArgs {
		body["cache_args"] = true
	}
	if e.CacheScheme {
		body["cache_scheme"] = true
	}
	if e.CacheIgnoreSC {
		body["cache_ignore_sc"] = true
	}
	if e.CacheIgnoreVary {
		body["cache_ignore_vary"] = true
	}
	if e.CacheIgnoreCC {
		body["cache_ignore_cc"] = true
	}
	if e.ClusterStatus {
		body["cluster_status"] = true
	}
	if e.CacheDeviceType {
		body["cache_device_type"] = true
	}
	if len(e.SlinkMD5) > 0 {
		body["slink_md5"] = slinkMD5RequestBody(e.SlinkMD5)
	}
	if len(e.ReqCustomHeaders) > 0 {
		body["req_custom_headers"] = headerExceptionEntriesRequestBody(e.ReqCustomHeaders)
	}
	if len(e.ResCustomHeaders) > 0 {
		body["res_custom_headers"] = headerExceptionEntriesRequestBody(e.ResCustomHeaders)
	}
	if len(e.ReqHideHeaders) > 0 {
		body["req_hide_headers"] = e.ReqHideHeaders
	}
	if len(e.ResHideHeaders) > 0 {
		body["res_hide_headers"] = e.ResHideHeaders
	}
	if accelerationIsSet(e.Acceleration) {
		body["acceleration"] = accelerationRequestBody(e.Acceleration)
	}
	if imageResizeIsSet(e.ImageResize) {
		body["image_resize"] = imageResizeRequestBody(e.ImageResize)
	}
	if upstreamTimeoutIsSet(e.UpstreamTimeout) {
		body["upstream_timeout"] = upstreamTimeoutRequestBody(e.UpstreamTimeout)
	}
	if redirectIsSet(e.Redirect) {
		body["redirect"] = pageRuleRedirectRequestBody(e.Redirect)
	}
	return body
}

// GetArvanCloudPageRuleExceptions returns a page rule's exceptions.
func (p *Provider) GetArvanCloudPageRuleExceptions(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudPageRuleExceptions, error) {
	var wire pageRuleDiffWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, pageRuleDiffPath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud page rule %q exceptions on domain %q: %w", id, domainName, err)
	}
	found := toPageRuleExceptionsDomain(wire)
	return &found, nil
}

// UpdateArvanCloudPageRuleExceptions replaces a page rule's exceptions.
func (p *Provider) UpdateArvanCloudPageRuleExceptions(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, exceptions domain.ArvanCloudPageRuleExceptions) (*domain.ArvanCloudPageRuleExceptions, error) {
	body := pageRuleExceptionsRequestBody(exceptions)
	var wire pageRuleDiffWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, pageRuleDiffPath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud page rule %q exceptions on domain %q: %w", id, domainName, err)
	}
	updated := toPageRuleExceptionsDomain(wire)
	return &updated, nil
}

// --- Response Transforms ---------------------------------------------------

// responseTransformActionWire mirrors ProxyResponseTransformAddHeaderAction.
type responseTransformActionWire struct {
	Type  string `json:"type,omitempty"`
	Mode  string `json:"mode,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// responseTransformStepWire mirrors ProxyResponseTransformStep.
type responseTransformStepWire struct {
	Condition string                        `json:"condition,omitempty"`
	Actions   []responseTransformActionWire `json:"actions,omitempty"`
}

// responseTransformWire mirrors ResponseTransform, covering the
// list/get/create/update response shapes.
type responseTransformWire struct {
	ID          string                      `json:"id,omitempty"`
	Name        string                      `json:"name,omitempty"`
	Description string                      `json:"description,omitempty"`
	Transforms  []responseTransformStepWire `json:"transforms,omitempty"`
	CreatedAt   string                      `json:"created_at,omitempty"`
	UpdatedAt   string                      `json:"updated_at,omitempty"`
}

func toResponseTransformDomain(w responseTransformWire) domain.ArvanCloudResponseTransform {
	steps := make([]domain.ArvanCloudResponseTransformStep, len(w.Transforms))
	for i, s := range w.Transforms {
		actions := make([]domain.ArvanCloudResponseTransformAction, len(s.Actions))
		for j, a := range s.Actions {
			actions[j] = domain.ArvanCloudResponseTransformAction{
				Type: domain.ArvanCloudResponseTransformActionType(a.Type), Mode: domain.ArvanCloudResponseTransformActionMode(a.Mode),
				Key: domain.ArvanCloudResponseTransformHeaderKey(a.Key), Value: a.Value,
			}
		}
		steps[i] = domain.ArvanCloudResponseTransformStep{Condition: s.Condition, Actions: actions}
	}
	return domain.ArvanCloudResponseTransform{
		ID: w.ID, Name: w.Name, Description: w.Description, Transforms: steps, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}
}

func responseTransformRequestBody(rt domain.ArvanCloudResponseTransform) map[string]any {
	steps := make([]map[string]any, len(rt.Transforms))
	for i, s := range rt.Transforms {
		actions := make([]map[string]any, len(s.Actions))
		for j, a := range s.Actions {
			actionType := a.Type
			if actionType == "" {
				actionType = domain.ArvanCloudResponseTransformActionAddHeader
			}
			mode := a.Mode
			if mode == "" {
				mode = domain.ArvanCloudResponseTransformModeSet
			}
			actions[j] = map[string]any{"type": string(actionType), "mode": string(mode), "key": string(a.Key), "value": a.Value}
		}
		steps[i] = map[string]any{"condition": s.Condition, "actions": actions}
	}
	body := map[string]any{"name": rt.Name, "transforms": steps}
	if rt.Description != "" {
		body["description"] = rt.Description
	}
	return body
}

func responseTransformListQueryValues(q domain.ArvanCloudResponseTransformListQuery) url.Values {
	values := url.Values{}
	if q.Name != "" {
		values.Set("name", q.Name)
	}
	if q.PerPage > 0 {
		values.Set("per_page", strconv.Itoa(q.PerPage))
	}
	if q.Page > 0 {
		values.Set("page", strconv.Itoa(q.Page))
	}
	return values
}

// ListArvanCloudResponseTransforms returns one page of domainName's
// response-transform presets.
func (p *Provider) ListArvanCloudResponseTransforms(ctx context.Context, creds domain.ProviderCredentials, domainName string, query domain.ArvanCloudResponseTransformListQuery) ([]domain.ArvanCloudResponseTransform, domain.ArvanCloudResponseTransformPageMeta, error) {
	path := responseTransformsPath(domainName) + "?" + responseTransformListQueryValues(query).Encode()
	raw, err := p.client.doRawGET(ctx, creds, path, "application/json")
	if err != nil {
		return nil, domain.ArvanCloudResponseTransformPageMeta{}, fmt.Errorf("listing arvancloud response transforms of domain %q: %w", domainName, err)
	}
	envelope, err := decodePaginated[responseTransformWire](raw)
	if err != nil {
		return nil, domain.ArvanCloudResponseTransformPageMeta{}, fmt.Errorf("decoding arvancloud response transform list of domain %q: %w", domainName, err)
	}
	out := make([]domain.ArvanCloudResponseTransform, len(envelope.Data))
	for i, w := range envelope.Data {
		out[i] = toResponseTransformDomain(w)
	}
	return out, toPageMeta(envelope.Meta), nil
}

// CreateArvanCloudResponseTransform creates a new response-transform preset.
func (p *Provider) CreateArvanCloudResponseTransform(ctx context.Context, creds domain.ProviderCredentials, domainName string, rt domain.ArvanCloudResponseTransform) (*domain.ArvanCloudResponseTransform, error) {
	body := responseTransformRequestBody(rt)
	var wire responseTransformWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, responseTransformsPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("creating arvancloud response transform on domain %q: %w", domainName, err)
	}
	created := toResponseTransformDomain(wire)
	return &created, nil
}

// GetArvanCloudResponseTransform returns a single preset by id.
func (p *Provider) GetArvanCloudResponseTransform(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) (*domain.ArvanCloudResponseTransform, error) {
	var wire responseTransformWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, responseTransformPath(domainName, id), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud response transform %q on domain %q: %w", id, domainName, err)
	}
	found := toResponseTransformDomain(wire)
	return &found, nil
}

// UpdateArvanCloudResponseTransform updates a preset via PATCH.
func (p *Provider) UpdateArvanCloudResponseTransform(ctx context.Context, creds domain.ProviderCredentials, domainName, id string, rt domain.ArvanCloudResponseTransform) (*domain.ArvanCloudResponseTransform, error) {
	body := responseTransformRequestBody(rt)
	var wire responseTransformWire
	if err := p.client.doJSON(ctx, creds, http.MethodPatch, responseTransformPath(domainName, id), body, &wire); err != nil {
		return nil, fmt.Errorf("updating arvancloud response transform %q on domain %q: %w", id, domainName, err)
	}
	updated := toResponseTransformDomain(wire)
	return &updated, nil
}

// DeleteArvanCloudResponseTransform removes a preset by id.
func (p *Provider) DeleteArvanCloudResponseTransform(ctx context.Context, creds domain.ProviderCredentials, domainName, id string) error {
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, responseTransformPath(domainName, id), nil, nil); err != nil {
		return fmt.Errorf("deleting arvancloud response transform %q on domain %q: %w", id, domainName, err)
	}
	return nil
}

// --- Redirect (www-redirect) ------------------------------------------------

// redirectWire mirrors the Redirect schema.
type redirectWire struct {
	FRedirectToWWW string `json:"f_redirect_to_www,omitempty"`
}

// GetArvanCloudWWWRedirect returns a domain's www-redirect setting.
func (p *Provider) GetArvanCloudWWWRedirect(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudWWWRedirectSettings, error) {
	var wire redirectWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, wwwRedirectPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud www-redirect setting of domain %q: %w", domainName, err)
	}
	return &domain.ArvanCloudWWWRedirectSettings{Mode: domain.ArvanCloudWWWRedirectMode(wire.FRedirectToWWW)}, nil
}

// UpdateArvanCloudWWWRedirect sets a domain's www-redirect setting. The
// endpoint's response carries no data, only a confirmation message.
func (p *Provider) UpdateArvanCloudWWWRedirect(ctx context.Context, creds domain.ProviderCredentials, domainName string, settings domain.ArvanCloudWWWRedirectSettings) error {
	body := map[string]any{"f_redirect_to_www": string(settings.Mode)}
	if err := p.client.doJSON(ctx, creds, http.MethodPut, wwwRedirectPath(domainName), body, nil); err != nil {
		return fmt.Errorf("updating arvancloud www-redirect setting of domain %q: %w", domainName, err)
	}
	return nil
}

// --- Host Header Whitelist --------------------------------------------------

// hostHeaderWhitelistWire mirrors HostHeaderWhitelistData.
type hostHeaderWhitelistWire struct {
	TargetAccounts      []string `json:"target_accounts"`
	GloballyWhitelisted bool     `json:"globally_whitelisted"`
}

func toHostHeaderWhitelistDomain(w hostHeaderWhitelistWire) domain.ArvanCloudHostHeaderWhitelist {
	return domain.ArvanCloudHostHeaderWhitelist{TargetAccounts: w.TargetAccounts, GloballyWhitelisted: w.GloballyWhitelisted}
}

// GetArvanCloudHostHeaderWhitelist returns a domain's Host header whitelist
// state.
func (p *Provider) GetArvanCloudHostHeaderWhitelist(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	var wire hostHeaderWhitelistWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, hostHeaderWhitelistsPath(domainName), nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud host header whitelist of domain %q: %w", domainName, err)
	}
	found := toHostHeaderWhitelistDomain(wire)
	return &found, nil
}

// AddArvanCloudHostHeaderWhitelistEntry adds one target-account entry.
func (p *Provider) AddArvanCloudHostHeaderWhitelistEntry(ctx context.Context, creds domain.ProviderCredentials, domainName, targetAccount string) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	body := map[string]any{"target_account": targetAccount}
	var wire hostHeaderWhitelistWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, hostHeaderWhitelistsPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("adding arvancloud host header whitelist entry %q on domain %q: %w", targetAccount, domainName, err)
	}
	updated := toHostHeaderWhitelistDomain(wire)
	return &updated, nil
}

// SetArvanCloudHostHeaderWhitelistSettings sets or clears the domain's global
// Host allowlist entry.
func (p *Provider) SetArvanCloudHostHeaderWhitelistSettings(ctx context.Context, creds domain.ProviderCredentials, domainName string, global bool) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	body := map[string]any{"global": global}
	var wire hostHeaderWhitelistWire
	if err := p.client.doJSON(ctx, creds, http.MethodPut, hostHeaderWhitelistSettingsPath(domainName), body, &wire); err != nil {
		return nil, fmt.Errorf("setting arvancloud host header whitelist global setting on domain %q: %w", domainName, err)
	}
	updated := toHostHeaderWhitelistDomain(wire)
	return &updated, nil
}

// RemoveArvanCloudHostHeaderWhitelistEntry removes one target-account entry.
// Unlike this file's other delete-style methods, a missing row is not
// normalized here — see ports.ArvanCloudProvider's own doc comment on this
// method for why.
func (p *Provider) RemoveArvanCloudHostHeaderWhitelistEntry(ctx context.Context, creds domain.ProviderCredentials, domainName, targetAccount string) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	var wire hostHeaderWhitelistWire
	if err := p.client.doJSON(ctx, creds, http.MethodDelete, hostHeaderWhitelistEntryPath(domainName, targetAccount), nil, &wire); err != nil {
		return nil, fmt.Errorf("removing arvancloud host header whitelist entry %q on domain %q: %w", targetAccount, domainName, err)
	}
	updated := toHostHeaderWhitelistDomain(wire)
	return &updated, nil
}
