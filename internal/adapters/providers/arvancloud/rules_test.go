package arvancloud

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// TestArvanCloudCacheTTLSharedType proves cache_200, cache_any and
// cache_browser on domain.ArvanCloudPageRuleCaching all use the SAME Go type
// (domain.ArvanCloudCacheTTL) and the same underlying validator, rather than
// three separate near-identical enums — issue #71's own acceptance
// criterion. A compile-time assignment across the three fields with a single
// typed constant proves the type identity; the validator calls prove the
// enum values themselves are shared, with cache_browser's one extra
// "default" sentinel layered on top of the same list.
func TestArvanCloudCacheTTLSharedType(t *testing.T) {
	const ttl domain.ArvanCloudCacheTTL = domain.ArvanCloudCacheTTL1h
	caching := domain.ArvanCloudPageRuleCaching{Cache200: ttl, CacheAny: ttl, CacheBrowser: ttl}
	if caching.Cache200 != caching.CacheAny || caching.CacheAny != caching.CacheBrowser {
		t.Fatalf("Cache200/CacheAny/CacheBrowser diverged: %+v", caching)
	}

	if !domain.ValidArvanCloudCacheTTL("1h") || !domain.ValidArvanCloudCacheTTL("30d") {
		t.Errorf("ValidArvanCloudCacheTTL rejected a valid TTL")
	}
	if domain.ValidArvanCloudCacheTTL("default") {
		t.Errorf("ValidArvanCloudCacheTTL accepted \"default\", which only cache_browser accepts")
	}
	if !domain.ValidArvanCloudCacheBrowserTTL("default") {
		t.Errorf("ValidArvanCloudCacheBrowserTTL rejected \"default\", cache_browser's own extra sentinel")
	}
	if !domain.ValidArvanCloudCacheBrowserTTL("1h") {
		t.Errorf("ValidArvanCloudCacheBrowserTTL rejected \"1h\", part of the shared base enum")
	}
	if domain.ValidArvanCloudCacheTTL("31d") || domain.ValidArvanCloudCacheBrowserTTL("31d") {
		t.Errorf("an undeclared TTL value was accepted")
	}
}

// validArvanCloudPageRule returns a page rule touching at least one field
// from every logical group (matching/caching/security/routing/headers/
// other), for the create/update tests below (issue #71's acceptance
// criterion).
func validArvanCloudPageRule() domain.ArvanCloudPageRule {
	return domain.ArvanCloudPageRule{
		Matching: domain.ArvanCloudPageRuleMatching{
			URLType: domain.ArvanCloudPageRuleURLTypeDefault, URL: "/api/*", Seq: 1, Status: true,
		},
		Caching: domain.ArvanCloudPageRuleCaching{
			CacheLevel: domain.ArvanCloudPageRuleCacheLevelURI,
			Cache200:   domain.ArvanCloudCacheTTL30m, CacheAny: domain.ArvanCloudCacheTTL0s, CacheBrowser: domain.ArvanCloudCacheTTLDefault,
			CacheArgs: true,
		},
		Security: domain.ArvanCloudPageRuleSecurity{WAFStatus: true, SlinkStatus: false},
		Routing: domain.ArvanCloudPageRuleRouting{
			LoadBalancer: "lb-1",
			Redirect:     domain.ArvanCloudPageRuleRedirect{Enable: false},
		},
		Headers: domain.ArvanCloudPageRuleHeaders{
			CORSHeader:       "*",
			ReqCustomHeaders: []domain.ArvanCloudPageRuleHeaderEntry{{Name: "X-Test", Value: "1"}},
		},
		Other: domain.ArvanCloudPageRuleOther{
			Acceleration: domain.ArvanCloudAccelerationSettings{
				Status: domain.ArvanCloudAccelerationOn, Extensions: []domain.ArvanCloudAccelerationExtension{domain.ArvanCloudAccelerationExtensionJS},
			},
			UpstreamTimeout: domain.ArvanCloudUpstreamTimeout{ConnectTimeoutSeconds: 15},
		},
	}
}

// pageRuleWireResponse is a minimal but complete page-rule JSON response
// body, one field per logical group, used by the tests below.
const pageRuleWireResponse = `{"data":{
	"id":"pr-1","domain_id":"dom-1","seq":1,"url_type":"default","url":"/api/*","status":true,
	"cache_level":"uri","cache_200":"30m","cache_any":"0s","cache_browser":"default","cache_args":true,
	"waf_status":true,"slink_status":false,
	"load_balancer":"lb-1","redirect":{"enable":false,"status_code":301},
	"cors_header":"*","req_custom_headers":[{"name":"X-Test","value":"1"}],
	"acceleration":{"status":"on","extensions":["js"]},"upstream_timeout":{"connect_timeout":15,"read_timeout":100,"send_timeout":300}
}}`

// assertPageRuleGroups checks one representative field from every logical
// group is present on rule, proving a regression in any group would be
// caught (issue #71's acceptance criterion).
func assertPageRuleGroups(t *testing.T, rule domain.ArvanCloudPageRule) {
	t.Helper()
	if rule.Matching.URL != "/api/*" || rule.Matching.Seq != 1 {
		t.Errorf("Matching group = %+v, want URL=/api/* Seq=1", rule.Matching)
	}
	if rule.Caching.CacheLevel != domain.ArvanCloudPageRuleCacheLevelURI || rule.Caching.Cache200 != domain.ArvanCloudCacheTTL30m {
		t.Errorf("Caching group = %+v, want CacheLevel=uri Cache200=30m", rule.Caching)
	}
	if !rule.Security.WAFStatus {
		t.Errorf("Security group = %+v, want WAFStatus=true", rule.Security)
	}
	if rule.Routing.LoadBalancer != "lb-1" {
		t.Errorf("Routing group = %+v, want LoadBalancer=lb-1", rule.Routing)
	}
	if rule.Headers.CORSHeader != "*" || len(rule.Headers.ReqCustomHeaders) != 1 || rule.Headers.ReqCustomHeaders[0].Name != "X-Test" {
		t.Errorf("Headers group = %+v, want CORSHeader=* and one req_custom_headers entry", rule.Headers)
	}
	if rule.Other.Acceleration.Status != domain.ArvanCloudAccelerationOn || rule.Other.UpstreamTimeout.ConnectTimeoutSeconds != 15 {
		t.Errorf("Other group = %+v, want Acceleration.Status=on UpstreamTimeout.ConnectTimeoutSeconds=15", rule.Other)
	}
}

// TestCreateArvanCloudPageRule pins the request body and response parsing of
// POST /domains/{domain}/page-rules, covering one field from every logical
// group (issue #71's acceptance criterion).
func TestCreateArvanCloudPageRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(pageRuleWireResponse) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	created, err := provider.CreateArvanCloudPageRule(context.Background(), creds(), "example.com", validArvanCloudPageRule())
	if err != nil {
		t.Fatalf("CreateArvanCloudPageRule() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/page-rules" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/page-rules", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["url"] != "/api/*" || body["seq"] != float64(1) {
		t.Errorf("request body (matching) = url=%v seq=%v, want /api/* and 1", body["url"], body["seq"])
	}
	if body["cache_level"] != "uri" || body["cache_200"] != "30m" {
		t.Errorf("request body (caching) = cache_level=%v cache_200=%v, want uri and 30m", body["cache_level"], body["cache_200"])
	}
	if body["waf_status"] != true {
		t.Errorf("request body (security) = waf_status=%v, want true", body["waf_status"])
	}
	if body["load_balancer"] != "lb-1" {
		t.Errorf("request body (routing) = load_balancer=%v, want lb-1", body["load_balancer"])
	}
	if body["cors_header"] != "*" {
		t.Errorf("request body (headers) = cors_header=%v, want *", body["cors_header"])
	}
	acceleration, ok := body["acceleration"].(map[string]any)
	if !ok || acceleration["status"] != "on" {
		t.Errorf("request body (other) = acceleration=%v, want status=on", body["acceleration"])
	}

	assertPageRuleGroups(t, *created)
}

// TestUpdateArvanCloudPageRule pins the request method/path and response
// parsing of PUT /domains/{domain}/page-rules/{id}, covering the same field
// groups as TestCreateArvanCloudPageRule.
func TestUpdateArvanCloudPageRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(pageRuleWireResponse) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.UpdateArvanCloudPageRule(context.Background(), creds(), "example.com", "pr-1", validArvanCloudPageRule())
	if err != nil {
		t.Fatalf("UpdateArvanCloudPageRule() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/example.com/page-rules/pr-1" {
		t.Fatalf("request = %+v, want a single PUT /domains/example.com/page-rules/pr-1", records)
	}
	assertPageRuleGroups(t, *updated)
}

// TestGetArvanCloudPageRule pins GET /domains/{domain}/page-rules/{id}.
func TestGetArvanCloudPageRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(pageRuleWireResponse) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudPageRule(context.Background(), creds(), "example.com", "pr-1")
	if err != nil {
		t.Fatalf("GetArvanCloudPageRule() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/page-rules/pr-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/page-rules/pr-1", records)
	}
	assertPageRuleGroups(t, *found)
}

// TestListArvanCloudPageRules pins GET /domains/{domain}/page-rules,
// including the query string and pagination meta.
func TestListArvanCloudPageRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"pr-1","seq":1,"url":"/api/*","status":true,"cache_level":"uri"}],
			"meta":{"current_page":1,"from":1,"last_page":1,"per_page":25,"to":1,"total":1}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rules, meta, err := provider.ListArvanCloudPageRules(context.Background(), creds(), "example.com", domain.ArvanCloudPageRuleListQuery{Search: "api", PerPage: 25, Page: 1, Order: "asc"})
	if err != nil {
		t.Fatalf("ListArvanCloudPageRules() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/page-rules" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/page-rules", records)
	}
	if records[0].query != "order=asc&page=1&per_page=25&search=api" {
		t.Errorf("request query = %q, want order=asc&page=1&per_page=25&search=api", records[0].query)
	}
	if len(rules) != 1 || rules[0].ID != "pr-1" || rules[0].URL != "/api/*" {
		t.Errorf("rules = %+v, want one rule with id=pr-1 url=/api/*", rules)
	}
	if meta.Total != 1 {
		t.Errorf("meta = %+v, want Total=1", meta)
	}
}

// TestSetArvanCloudPageRuleStatus pins PATCH /domains/{domain}/page-rules/{id}.
func TestSetArvanCloudPageRuleStatus(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"ok"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.SetArvanCloudPageRuleStatus(context.Background(), creds(), "example.com", "pr-1", false); err != nil {
		t.Fatalf("SetArvanCloudPageRuleStatus() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/page-rules/pr-1" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/page-rules/pr-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["status"] != false {
		t.Errorf("request body[status] = %v, want false", body["status"])
	}
}

// TestDeleteArvanCloudPageRule pins DELETE /domains/{domain}/page-rules/{id}.
func TestDeleteArvanCloudPageRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"ok"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudPageRule(context.Background(), creds(), "example.com", "pr-1"); err != nil {
		t.Fatalf("DeleteArvanCloudPageRule() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/page-rules/pr-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/page-rules/pr-1", records)
	}
}

// TestPurgeArvanCloudPageRuleCache pins
// DELETE /domains/{domain}/page-rules/{id}/purge.
func TestPurgeArvanCloudPageRuleCache(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"ok"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.PurgeArvanCloudPageRuleCache(context.Background(), creds(), "example.com", "pr-1"); err != nil {
		t.Fatalf("PurgeArvanCloudPageRuleCache() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/page-rules/pr-1/purge" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/page-rules/pr-1/purge", records)
	}
}

// TestGetArvanCloudPageRuleExceptions pins
// GET /domains/{domain}/page-rules/{id}/diff.
func TestGetArvanCloudPageRuleExceptions(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"url":"/override","cache_level":"uri","waf_status":true,"load_balancer":"lb-2",
			"cors_header":"*","acceleration":{"status":"off"}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	exceptions, err := provider.GetArvanCloudPageRuleExceptions(context.Background(), creds(), "example.com", "pr-1")
	if err != nil {
		t.Fatalf("GetArvanCloudPageRuleExceptions() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/page-rules/pr-1/diff" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/page-rules/pr-1/diff", records)
	}
	if exceptions.URL != "/override" || exceptions.LoadBalancer != "lb-2" {
		t.Errorf("exceptions = %+v, want URL=/override LoadBalancer=lb-2", exceptions)
	}
}

// TestUpdateArvanCloudPageRuleExceptions pins
// PATCH /domains/{domain}/page-rules/{id}/diff and its request body shape.
func TestUpdateArvanCloudPageRuleExceptions(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"url":"/override","cache_level":"uri"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	exceptions := domain.ArvanCloudPageRuleExceptions{
		URL: "/override", CacheLevel: domain.ArvanCloudPageRuleCacheLevelURI,
		ReqCustomHeaders: []domain.ArvanCloudPageRuleExceptionHeaderEntry{{Name: "X-A", Value: "1", IsVar: "true"}},
	}
	updated, err := provider.UpdateArvanCloudPageRuleExceptions(context.Background(), creds(), "example.com", "pr-1", exceptions)
	if err != nil {
		t.Fatalf("UpdateArvanCloudPageRuleExceptions() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/page-rules/pr-1/diff" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/page-rules/pr-1/diff", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["url"] != "/override" {
		t.Errorf("request body[url] = %v, want /override", body["url"])
	}
	headers, ok := body["req_custom_headers"].([]any)
	if !ok || len(headers) != 1 {
		t.Fatalf("request body[req_custom_headers] = %#v, want one entry", body["req_custom_headers"])
	}
	entry, ok := headers[0].(map[string]any)
	if !ok || entry["is_var"] != "true" {
		t.Errorf("request body[req_custom_headers][0] = %#v, want is_var=true", headers[0])
	}
	if updated.URL != "/override" {
		t.Errorf("updated = %+v, want URL=/override", updated)
	}
}

// --- Response Transforms ---------------------------------------------------

// TestListArvanCloudResponseTransforms pins GET
// /domains/{domain}/response-transforms and pagination meta.
func TestListArvanCloudResponseTransforms(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"rt-1","name":"cors-preset","transforms":[{"condition":"http.request.method == \"GET\"",
			"actions":[{"type":"add_header","mode":"set","key":"Access-Control-Allow-Origin","value":"to_string(\"*\")"}]}]}],
			"meta":{"current_page":1,"from":1,"last_page":1,"per_page":25,"to":1,"total":1}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	transforms, meta, err := provider.ListArvanCloudResponseTransforms(context.Background(), creds(), "example.com", domain.ArvanCloudResponseTransformListQuery{Name: "cors"})
	if err != nil {
		t.Fatalf("ListArvanCloudResponseTransforms() error = %v", err)
	}
	if len(records) != 1 || records[0].path != "/domains/example.com/response-transforms" || records[0].query != "name=cors" {
		t.Fatalf("request = %+v, want GET /domains/example.com/response-transforms?name=cors", records)
	}
	if len(transforms) != 1 || transforms[0].ID != "rt-1" || len(transforms[0].Transforms) != 1 {
		t.Fatalf("transforms = %+v, want one preset with one step", transforms)
	}
	step := transforms[0].Transforms[0]
	if len(step.Actions) != 1 || step.Actions[0].Key != domain.ArvanCloudResponseTransformHeaderAllowOrigin {
		t.Errorf("step.Actions = %+v, want one Access-Control-Allow-Origin action", step.Actions)
	}
	if meta.Total != 1 {
		t.Errorf("meta = %+v, want Total=1", meta)
	}
}

// TestCreateArvanCloudResponseTransform pins the request body of POST
// /domains/{domain}/response-transforms.
func TestCreateArvanCloudResponseTransform(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rt-1","name":"cors-preset","transforms":[{"condition":"true","actions":[
			{"type":"add_header","mode":"set","key":"Access-Control-Allow-Origin","value":"to_string(\"*\")"}]}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rt := domain.ArvanCloudResponseTransform{
		Name: "cors-preset",
		Transforms: []domain.ArvanCloudResponseTransformStep{{
			Condition: "true",
			Actions: []domain.ArvanCloudResponseTransformAction{{
				Type: domain.ArvanCloudResponseTransformActionAddHeader, Mode: domain.ArvanCloudResponseTransformModeSet,
				Key: domain.ArvanCloudResponseTransformHeaderAllowOrigin, Value: `to_string("*")`,
			}},
		}},
	}
	created, err := provider.CreateArvanCloudResponseTransform(context.Background(), creds(), "example.com", rt)
	if err != nil {
		t.Fatalf("CreateArvanCloudResponseTransform() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/response-transforms" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/response-transforms", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["name"] != "cors-preset" {
		t.Errorf("request body[name] = %v, want cors-preset", body["name"])
	}
	if created.ID != "rt-1" {
		t.Errorf("created.ID = %q, want rt-1", created.ID)
	}
}

// TestGetArvanCloudResponseTransform pins GET
// /domains/{domain}/response-transforms/{id}.
func TestGetArvanCloudResponseTransform(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rt-1","name":"cors-preset"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudResponseTransform(context.Background(), creds(), "example.com", "rt-1")
	if err != nil {
		t.Fatalf("GetArvanCloudResponseTransform() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/response-transforms/rt-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/response-transforms/rt-1", records)
	}
	if found.ID != "rt-1" {
		t.Errorf("found.ID = %q, want rt-1", found.ID)
	}
}

// TestUpdateArvanCloudResponseTransform pins PATCH
// /domains/{domain}/response-transforms/{id}.
func TestUpdateArvanCloudResponseTransform(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rt-1","name":"cors-preset-renamed"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rt := domain.ArvanCloudResponseTransform{Name: "cors-preset-renamed"}
	updated, err := provider.UpdateArvanCloudResponseTransform(context.Background(), creds(), "example.com", "rt-1", rt)
	if err != nil {
		t.Fatalf("UpdateArvanCloudResponseTransform() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/response-transforms/rt-1" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/response-transforms/rt-1", records)
	}
	if updated.Name != "cors-preset-renamed" {
		t.Errorf("updated.Name = %q, want cors-preset-renamed", updated.Name)
	}
}

// TestDeleteArvanCloudResponseTransform pins DELETE
// /domains/{domain}/response-transforms/{id}.
func TestDeleteArvanCloudResponseTransform(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"ok"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudResponseTransform(context.Background(), creds(), "example.com", "rt-1"); err != nil {
		t.Fatalf("DeleteArvanCloudResponseTransform() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/response-transforms/rt-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/response-transforms/rt-1", records)
	}
}

// --- Redirect (www-redirect) ------------------------------------------------

// TestGetArvanCloudWWWRedirect pins GET
// /domains/{domain}/settings/www-redirect.
func TestGetArvanCloudWWWRedirect(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"data":{"f_redirect_to_www":"www"}}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings, err := provider.GetArvanCloudWWWRedirect(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudWWWRedirect() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/settings/www-redirect" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/settings/www-redirect", records)
	}
	if settings.Mode != domain.ArvanCloudWWWRedirectToWWW {
		t.Errorf("settings.Mode = %q, want %q", settings.Mode, domain.ArvanCloudWWWRedirectToWWW)
	}
}

// TestUpdateArvanCloudWWWRedirect pins PUT
// /domains/{domain}/settings/www-redirect and its request body.
func TestUpdateArvanCloudWWWRedirect(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"ok"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.UpdateArvanCloudWWWRedirect(context.Background(), creds(), "example.com", domain.ArvanCloudWWWRedirectSettings{Mode: domain.ArvanCloudWWWRedirectToRoot})
	if err != nil {
		t.Fatalf("UpdateArvanCloudWWWRedirect() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/example.com/settings/www-redirect" {
		t.Fatalf("request = %+v, want a single PUT /domains/example.com/settings/www-redirect", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["f_redirect_to_www"] != "root" {
		t.Errorf("request body[f_redirect_to_www] = %v, want root", body["f_redirect_to_www"])
	}
}

// --- Host Header Whitelist --------------------------------------------------

// TestGetArvanCloudHostHeaderWhitelist pins GET
// /domains/{domain}/host-header-whitelists.
func TestGetArvanCloudHostHeaderWhitelist(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"target_accounts":["acct-1","acct-2"],"globally_whitelisted":false}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	whitelist, err := provider.GetArvanCloudHostHeaderWhitelist(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudHostHeaderWhitelist() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/host-header-whitelists" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/host-header-whitelists", records)
	}
	if len(whitelist.TargetAccounts) != 2 || whitelist.GloballyWhitelisted {
		t.Errorf("whitelist = %+v, want two target accounts and globally_whitelisted=false", whitelist)
	}
}

// TestAddArvanCloudHostHeaderWhitelistEntry pins POST
// /domains/{domain}/host-header-whitelists and its request body.
func TestAddArvanCloudHostHeaderWhitelistEntry(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"target_accounts":["acct-1"],"globally_whitelisted":false}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	whitelist, err := provider.AddArvanCloudHostHeaderWhitelistEntry(context.Background(), creds(), "example.com", "acct-1")
	if err != nil {
		t.Fatalf("AddArvanCloudHostHeaderWhitelistEntry() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/host-header-whitelists" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/host-header-whitelists", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["target_account"] != "acct-1" {
		t.Errorf("request body[target_account] = %v, want acct-1", body["target_account"])
	}
	if len(whitelist.TargetAccounts) != 1 {
		t.Errorf("whitelist = %+v, want one target account", whitelist)
	}
}

// TestSetArvanCloudHostHeaderWhitelistSettings pins PUT
// /domains/{domain}/host-header-whitelists/settings and its request body.
func TestSetArvanCloudHostHeaderWhitelistSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"target_accounts":[],"globally_whitelisted":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	whitelist, err := provider.SetArvanCloudHostHeaderWhitelistSettings(context.Background(), creds(), "example.com", true)
	if err != nil {
		t.Fatalf("SetArvanCloudHostHeaderWhitelistSettings() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/example.com/host-header-whitelists/settings" {
		t.Fatalf("request = %+v, want a single PUT /domains/example.com/host-header-whitelists/settings", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["global"] != true {
		t.Errorf("request body[global] = %v, want true", body["global"])
	}
	if !whitelist.GloballyWhitelisted {
		t.Errorf("whitelist = %+v, want globally_whitelisted=true", whitelist)
	}
}

// TestRemoveArvanCloudHostHeaderWhitelistEntry pins DELETE
// /domains/{domain}/host-header-whitelists/{target_account}.
func TestRemoveArvanCloudHostHeaderWhitelistEntry(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"target_accounts":[],"globally_whitelisted":false}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	whitelist, err := provider.RemoveArvanCloudHostHeaderWhitelistEntry(context.Background(), creds(), "example.com", "acct-1")
	if err != nil {
		t.Fatalf("RemoveArvanCloudHostHeaderWhitelistEntry() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/host-header-whitelists/acct-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/host-header-whitelists/acct-1", records)
	}
	if len(whitelist.TargetAccounts) != 0 {
		t.Errorf("whitelist = %+v, want zero target accounts", whitelist)
	}
}
