package parspack_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestListCDNRateLimitRulesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/rate-limit" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/firewalls/rate-limit", got)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"6DAbXaog","value":"https://example.local/api/*","enabled":true,"name":"Rate Limit Rule 1",
			 "priority":1,"static_interval_type":"second","static_interval":10,"static_requests":5,
			 "dynamic_interval_type":"second","dynamic_interval":10,"dynamic_requests":6,
			 "challenge":"captcha","trust_time":3600,"attack_ban_time":900,
			 "white_list_ips":[{"id":1399,"ip":"1.1.1.1","rate_limit_rule_id":29}]}
		]}`))
	})

	rules, err := c.ListCDNRateLimitRules(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListCDNRateLimitRules: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != "6DAbXaog" || rules[0].Name != "Rate Limit Rule 1" {
		t.Errorf("rules = %+v, want a single rule 6DAbXaog named 'Rate Limit Rule 1'", rules)
	}
	if len(rules[0].WhitelistIPs) != 1 || rules[0].WhitelistIPs[0].IP != "1.1.1.1" {
		t.Errorf("whitelist = %+v, want a single entry 1.1.1.1", rules[0].WhitelistIPs)
	}
}

func TestListCDNRateLimitRulesNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.ListCDNRateLimitRules(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetCDNRateLimitRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/rate-limit/l3AdK0VQ" {
			t.Errorf("path = %s, want .../firewalls/rate-limit/l3AdK0VQ", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":
			{"id":"l3AdK0VQ","value":"*.example.local/new/assets/**","enabled":true,"name":"new-added rulerw234ra",
			 "priority":1,"static_interval_type":"second","static_interval":10,"static_requests":10,
			 "dynamic_interval_type":"second","dynamic_interval":10,"dynamic_requests":6,
			 "challenge":"captcha","trust_time":3600,"attack_ban_time":900,"white_list_ips":[]}
		}`))
	})

	rule, err := c.GetCDNRateLimitRule(context.Background(), creds, "zone-1", "l3AdK0VQ")
	if err != nil {
		t.Fatalf("GetCDNRateLimitRule: %v", err)
	}
	if rule.ID != "l3AdK0VQ" || rule.StaticRequests != 10 || rule.DynamicRequests != 6 {
		t.Errorf("rule = %+v, want id l3AdK0VQ, static_requests 10, dynamic_requests 6", rule)
	}
}

func TestGetCDNRateLimitRuleNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNRateLimitRule(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestCreateCDNRateLimitRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/rate-limit" {
			t.Errorf("path = %s, want .../firewalls/rate-limit", got)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rule, err := c.CreateCDNRateLimitRule(context.Background(), creds, "zone-1", domain.CDNRateLimitRule{
		Name: "sample-rule", Value: "https://example.com/*",
		StaticIntervalType: "second", StaticInterval: 1,
		DynamicIntervalType: "day", DynamicInterval: 1,
		Challenge: "js", TrustTime: 1, AttackBanTime: 1,
	})
	if err != nil {
		t.Fatalf("CreateCDNRateLimitRule: %v", err)
	}
	if rule.Name != "sample-rule" || rule.Value != "https://example.com/*" {
		t.Errorf("rule = %+v, want the request echoed back", rule)
	}
}

func TestCreateCDNRateLimitRuleInvalidInput(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The name field is required.","errors":{}}`))
	})

	_, err := c.CreateCDNRateLimitRule(context.Background(), creds, "zone-1", domain.CDNRateLimitRule{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNRateLimitRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/rate-limit/rule-1" {
			t.Errorf("path = %s, want .../firewalls/rate-limit/rule-1", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rule, err := c.UpdateCDNRateLimitRule(context.Background(), creds, "zone-1", "rule-1", domain.CDNRateLimitRule{
		Name: "sample-rule", Value: "https://example.com/*", Enabled: true,
		StaticIntervalType: "second", StaticInterval: 1,
		DynamicIntervalType: "day", DynamicInterval: 1,
		Challenge: "js", TrustTime: 1, AttackBanTime: 1,
	})
	if err != nil {
		t.Fatalf("UpdateCDNRateLimitRule: %v", err)
	}
	if rule.ID != "rule-1" || !rule.Enabled {
		t.Errorf("rule = %+v, want id rule-1 and enabled true", rule)
	}
}

func TestDeleteCDNRateLimitRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/rate-limit/rule-1" {
			t.Errorf("path = %s, want .../firewalls/rate-limit/rule-1", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNRateLimitRule(context.Background(), creds, "zone-1", "rule-1"); err != nil {
		t.Fatalf("DeleteCDNRateLimitRule: %v", err)
	}
}

func TestDeleteCDNRateLimitRuleNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	err := c.DeleteCDNRateLimitRule(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNRateLimitRulePrioritySuccess(t *testing.T) {
	var gotBody string
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/rate-limit/rule-1/update-priority" {
			t.Errorf("path = %s, want .../firewalls/rate-limit/rule-1/update-priority", got)
		}
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.UpdateCDNRateLimitRulePriority(context.Background(), creds, "zone-1", "rule-1", 3); err != nil {
		t.Fatalf("UpdateCDNRateLimitRulePriority: %v", err)
	}
	if gotBody != `{"priority":3}` {
		t.Errorf("request body = %s, want {\"priority\":3}", gotBody)
	}
}

func TestGetCDNUpstreamErrorsSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/upstream-errors" {
			t.Errorf("path = %s, want .../zones/zone-1/upstream-errors", got)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"enabled":true}}`))
	})

	settings, err := c.GetCDNUpstreamErrors(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNUpstreamErrors: %v", err)
	}
	if !settings.Enabled {
		t.Errorf("settings = %+v, want enabled true", settings)
	}
}

func TestGetCDNUpstreamErrorsNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNUpstreamErrors(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNUpstreamErrorsSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/upstream-errors" {
			t.Errorf("path = %s, want .../zones/zone-1/upstream-errors", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	settings, err := c.UpdateCDNUpstreamErrors(context.Background(), creds, "zone-1", false)
	if err != nil {
		t.Fatalf("UpdateCDNUpstreamErrors: %v", err)
	}
	if settings.Enabled {
		t.Errorf("settings = %+v, want enabled false", settings)
	}
}

func TestUpdateCDNUpstreamErrorsValidationError(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The enabled parameter is required.","errors":{}}`))
	})

	_, err := c.UpdateCDNUpstreamErrors(context.Background(), creds, "zone-1", true)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
