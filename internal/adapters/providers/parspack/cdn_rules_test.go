package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// ---------------------------------------------------------------------------
// Origin Rules.

func TestListCDNOriginRulesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/origin-rules" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/origin-rules", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"19rEvwaq","name":"upstream-rule","type":"upstream","upstream_ip":"1.1.1.1","port":null,"load_balance":null,"enabled":true,"priority":1},
			{"id":"19rEEerq","name":"load-balance-rule","type":"load_balance","upstream_ip":null,"port":null,"load_balance":{"id":"AbCdEfGh","name":"primary-lb"},"enabled":true,"priority":3}
		]}`))
	})

	rules, err := c.ListCDNOriginRules(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListCDNOriginRules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
	if rules[0].Type != "upstream" || rules[0].UpstreamIP != "1.1.1.1" {
		t.Errorf("rules[0] = %+v, want type upstream with upstream_ip 1.1.1.1", rules[0])
	}
	if rules[1].LoadBalanceID != "AbCdEfGh" || rules[1].LoadBalanceName != "primary-lb" {
		t.Errorf("rules[1] = %+v, want load_balance AbCdEfGh/primary-lb", rules[1])
	}
}

func TestGetCDNOriginRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/origin-rules/19rEvwaq" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/origin-rules/19rEvwaq", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"id":"19rEvwaq","name":"upstream-rule","type":"load_balance","upstream_ip":null,"port":null,
			"load_balance":{"id":"wg0ljPrK","name":"primary-lb"},"enabled":true,"priority":1,
			"rule_items":[
				[{"field":"country_code","operation":"equals","value":353,"value_detail":"Sudan","bulklist":null}],
				[{"field":"country_code","operation":"from_list","value":"","value_detail":"","bulklist":{"id":"AbCdEfGh","name":"blocked-countries","type":"country"}}]
			]
		}}`))
	})

	rule, err := c.GetCDNOriginRule(context.Background(), creds, "zone-1", "19rEvwaq")
	if err != nil {
		t.Fatalf("GetCDNOriginRule: %v", err)
	}
	if rule.LoadBalanceID != "wg0ljPrK" {
		t.Errorf("LoadBalanceID = %q, want wg0ljPrK", rule.LoadBalanceID)
	}
	if len(rule.RuleItems) != 2 {
		t.Fatalf("len(RuleItems) = %d, want 2", len(rule.RuleItems))
	}
	if rule.RuleItems[0][0].Field != "country_code" || string(rule.RuleItems[0][0].Value) != "353" {
		t.Errorf("RuleItems[0][0] = %+v, want field country_code, value 353", rule.RuleItems[0][0])
	}
	if rule.RuleItems[1][0].BulklistID != "AbCdEfGh" || rule.RuleItems[1][0].BulklistName != "blocked-countries" {
		t.Errorf("RuleItems[1][0] = %+v, want bulklist AbCdEfGh/blocked-countries", rule.RuleItems[1][0])
	}
}

func TestGetCDNOriginRuleNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNOriginRule(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestCreateCDNOriginRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/origin-rules" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/origin-rules", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "upstream-rule" || body["type"] != "upstream" || body["upstream_ip"] != "1.1.1.1" {
			t.Errorf("body = %+v, want name/type/upstream_ip upstream-rule/upstream/1.1.1.1", body)
		}
		ruleItems, ok := body["rule_items"].([]any)
		if !ok || len(ruleItems) != 1 {
			t.Errorf("body.rule_items = %+v, want a single condition group", body["rule_items"])
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rule, err := c.CreateCDNOriginRule(context.Background(), creds, "zone-1", domain.CDNOriginRule{
		Name: "upstream-rule", Type: "upstream", UpstreamIP: "1.1.1.1",
		RuleItems: [][]domain.CDNRuleCondition{{{Field: "full_url", Operation: "contains", Value: json.RawMessage(`"https://example.local/*"`)}}},
	})
	if err != nil {
		t.Fatalf("CreateCDNOriginRule: %v", err)
	}
	if rule.ID != "" {
		t.Errorf("ID = %q, want empty (provider does not echo an ID on create)", rule.ID)
	}
	if rule.Name != "upstream-rule" {
		t.Errorf("Name = %q, want upstream-rule", rule.Name)
	}
}

func TestUpdateCDNOriginRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/origin-rules/19rEvwaq" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/origin-rules/19rEvwaq", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rule, err := c.UpdateCDNOriginRule(context.Background(), creds, "zone-1", "19rEvwaq", domain.CDNOriginRule{
		Name: "renamed", Type: "port", Port: 8080,
	})
	if err != nil {
		t.Fatalf("UpdateCDNOriginRule: %v", err)
	}
	if rule.ID != "19rEvwaq" {
		t.Errorf("ID = %q, want 19rEvwaq", rule.ID)
	}
}

func TestDeleteCDNOriginRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNOriginRule(context.Background(), creds, "zone-1", "19rEvwaq"); err != nil {
		t.Fatalf("DeleteCDNOriginRule: %v", err)
	}
}

func TestToggleCDNOriginRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/origin-rules/19rEvwaq/toggle-status" {
			t.Errorf("path = %s, want .../origin-rules/19rEvwaq/toggle-status", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["enabled"] != false {
			t.Errorf("body.enabled = %v, want false", body["enabled"])
		}

		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.ToggleCDNOriginRule(context.Background(), creds, "zone-1", "19rEvwaq", false); err != nil {
		t.Fatalf("ToggleCDNOriginRule: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Page Rules.

func TestListCDNPageRulesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/page-rules" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/page-rules", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"19rEvwaq","name":"default-rule","type":"url","operator":"pattern","value":"*example.local/**","priority":1}
		]}`))
	})

	rules, err := c.ListCDNPageRules(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListCDNPageRules: %v", err)
	}
	if len(rules) != 1 || rules[0].Value != "*example.local/**" {
		t.Errorf("rules = %+v, want a single rule with value *example.local/**", rules)
	}
}

func TestGetCDNPageRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/page-rules/19rEvwaq" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/page-rules/19rEvwaq", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"id":"19rEvwaq","type":"url","operator":"pattern","name":"assets-cache","value":"*example.local/assets/**",
			"minify":false,"cache_policy":"cdn-smart-caching","cache_ttl":3600,
			"firewall_except_ips":["192.168.0.1"],"waf_is_active":false,"priority":1,
			"addition_headers":{"access-control-allow-origin":"*"},"removable_headers":["X-Powered-By"],
			"addition_cookies":[{"key":"session_id","value":"abc123","expire_date":3600,"path":"/","http_only":false,"secure":false}],
			"port_destination":{"http":80,"https":443},
			"url_redirection":{"http_code":301,"url":"https://example.local/new-path"},
			"firewall_status":true
		}}`))
	})

	rule, err := c.GetCDNPageRule(context.Background(), creds, "zone-1", "19rEvwaq")
	if err != nil {
		t.Fatalf("GetCDNPageRule: %v", err)
	}
	if rule.CachePolicy != domain.DNSRecordProxyCDNSmartCaching {
		t.Errorf("CachePolicy = %v, want DNSRecordProxyCDNSmartCaching", rule.CachePolicy)
	}
	if rule.CacheTTLSeconds != 3600 {
		t.Errorf("CacheTTLSeconds = %d, want 3600", rule.CacheTTLSeconds)
	}
	if !rule.FirewallStatus {
		t.Error("FirewallStatus = false, want true")
	}
	if rule.URLRedirection == nil || rule.URLRedirection.HTTPCode != 301 {
		t.Errorf("URLRedirection = %+v, want http_code 301", rule.URLRedirection)
	}
	if rule.PortDestination == nil || rule.PortDestination.HTTP != 80 || rule.PortDestination.HTTPS != 443 {
		t.Errorf("PortDestination = %+v, want http/https 80/443", rule.PortDestination)
	}
	if len(rule.AdditionCookies) != 1 || rule.AdditionCookies[0].Key != "session_id" {
		t.Errorf("AdditionCookies = %+v, want a single session_id cookie", rule.AdditionCookies)
	}
}

func TestCreateCDNPageRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/page-rules" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/page-rules", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "cache-assets" || body["value"] != "*example.com/assets/**" || body["firewall_status"] != true {
			t.Errorf("body = %+v, want name/value/firewall_status cache-assets/*example.com/assets/**/true", body)
		}
		if body["cache_policy"] != "cdn-smart-caching" {
			t.Errorf("body.cache_policy = %v, want cdn-smart-caching", body["cache_policy"])
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rule, err := c.CreateCDNPageRule(context.Background(), creds, "zone-1", domain.CDNPageRule{
		Name: "cache-assets", Type: "url", Operator: "pattern", Value: "*example.com/assets/**",
		FirewallStatus: true, CachePolicy: domain.DNSRecordProxyCDNSmartCaching,
	})
	if err != nil {
		t.Fatalf("CreateCDNPageRule: %v", err)
	}
	if rule.ID != "" {
		t.Errorf("ID = %q, want empty (provider does not echo an ID on create)", rule.ID)
	}
}

func TestUpdateCDNPageRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rule, err := c.UpdateCDNPageRule(context.Background(), creds, "zone-1", "19rEvwaq", domain.CDNPageRule{
		Name: "renamed", Value: "*example.com/**", FirewallStatus: true,
	})
	if err != nil {
		t.Fatalf("UpdateCDNPageRule: %v", err)
	}
	if rule.ID != "19rEvwaq" {
		t.Errorf("ID = %q, want 19rEvwaq", rule.ID)
	}
}

func TestDeleteCDNPageRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNPageRule(context.Background(), creds, "zone-1", "19rEvwaq"); err != nil {
		t.Fatalf("DeleteCDNPageRule: %v", err)
	}
}

func TestDeleteCDNPageRuleNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	err := c.DeleteCDNPageRule(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// Transform Rules.

func TestListCDNTransformRulesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/transform-rules" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/transform-rules", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"19rEvwaq","name":"header-modify-rule","enabled":true,"priority":1},
			{"id":"wg0ljPrK","name":"response-header-rule","enabled":false,"priority":2}
		]}`))
	})

	rules, err := c.ListCDNTransformRules(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListCDNTransformRules: %v", err)
	}
	if len(rules) != 2 || rules[0].Enabled != true || rules[1].Enabled != false {
		t.Errorf("rules = %+v, want two rules enabled true/false", rules)
	}
}

func TestGetCDNTransformRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/transform-rules/19rEvwaq" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/transform-rules/19rEvwaq", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"id":"19rEvwaq","name":"header-modify-rule","enabled":true,"priority":1,
			"transform_rule_values":{
				"request_headers":[{"header_name":"X-Example-Header-1","header_value":"Example-Value-1","action":"modify"}],
				"response_headers":[{"header_name":"X-Example-Header-3","header_value":"Example-Value-3","action":"modify"}]
			},
			"rule_items":[
				[{"field":"country_code","operation":"list","value":[353,840],"value_detail":["Sudan","United Kingdom"],"bulklist":null}]
			]
		}}`))
	})

	rule, err := c.GetCDNTransformRule(context.Background(), creds, "zone-1", "19rEvwaq")
	if err != nil {
		t.Fatalf("GetCDNTransformRule: %v", err)
	}
	if len(rule.RequestHeaders) != 1 || rule.RequestHeaders[0].HeaderName != "X-Example-Header-1" {
		t.Errorf("RequestHeaders = %+v, want a single X-Example-Header-1 entry", rule.RequestHeaders)
	}
	if len(rule.ResponseHeaders) != 1 || rule.ResponseHeaders[0].Action != "modify" {
		t.Errorf("ResponseHeaders = %+v, want a single modify entry", rule.ResponseHeaders)
	}
	if len(rule.RuleItems) != 1 || rule.RuleItems[0][0].Field != "country_code" {
		t.Errorf("RuleItems = %+v, want a single country_code condition group", rule.RuleItems)
	}
}

func TestCreateCDNTransformRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/transform-rules" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/transform-rules", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "header-modify-rule" {
			t.Errorf("body.name = %v, want header-modify-rule", body["name"])
		}
		values, ok := body["transform_rule_values"].(map[string]any)
		if !ok {
			t.Fatalf("body.transform_rule_values = %+v, want an object", body["transform_rule_values"])
		}
		reqHeaders, ok := values["request_headers"].([]any)
		if !ok || len(reqHeaders) != 1 {
			t.Errorf("body.transform_rule_values.request_headers = %+v, want a single entry", values["request_headers"])
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rule, err := c.CreateCDNTransformRule(context.Background(), creds, "zone-1", domain.CDNTransformRule{
		Name: "header-modify-rule",
		RequestHeaders: []domain.CDNTransformHeaderAction{
			{HeaderName: "X-Request-Id", HeaderValue: "abc123", Action: "modify"},
		},
		RuleItems: [][]domain.CDNRuleCondition{{{Field: "full_url", Operation: "contains", Value: json.RawMessage(`"https://example.local/*"`)}}},
	})
	if err != nil {
		t.Fatalf("CreateCDNTransformRule: %v", err)
	}
	if rule.ID != "" {
		t.Errorf("ID = %q, want empty (provider does not echo an ID on create)", rule.ID)
	}
}

func TestUpdateCDNTransformRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rule, err := c.UpdateCDNTransformRule(context.Background(), creds, "zone-1", "19rEvwaq", domain.CDNTransformRule{
		Name: "renamed",
	})
	if err != nil {
		t.Fatalf("UpdateCDNTransformRule: %v", err)
	}
	if rule.ID != "19rEvwaq" {
		t.Errorf("ID = %q, want 19rEvwaq", rule.ID)
	}
}

func TestDeleteCDNTransformRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNTransformRule(context.Background(), creds, "zone-1", "19rEvwaq"); err != nil {
		t.Fatalf("DeleteCDNTransformRule: %v", err)
	}
}

func TestToggleCDNTransformRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/transform-rules/19rEvwaq/toggle-status" {
			t.Errorf("path = %s, want .../transform-rules/19rEvwaq/toggle-status", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["enabled"] != true {
			t.Errorf("body.enabled = %v, want true", body["enabled"])
		}

		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.ToggleCDNTransformRule(context.Background(), creds, "zone-1", "19rEvwaq", true); err != nil {
		t.Fatalf("ToggleCDNTransformRule: %v", err)
	}
}
