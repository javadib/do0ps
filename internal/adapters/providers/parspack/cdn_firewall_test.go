package parspack_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestListCDNAccessRulesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/access-management" {
			t.Errorf("path = %s, want .../zone-1/firewalls/access-management", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"vpaO2roj","type":"ip","value":"192.168.1.10","action":"block","status":true,"priority":1,"value_detail":null,"bulklist":null},
			{"id":"7B0NZ0YZ","type":"ip","value":null,"action":"block","status":true,"priority":3,"value_detail":null,
			 "bulklist":{"id":1,"name":"blocked-ips","items":[{"value":"192.168.1.20","value_detail":null}]}}
		]}`))
	})

	rules, err := c.ListCDNAccessRules(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListCDNAccessRules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
	if rules[0].ID != "vpaO2roj" || rules[0].Value != "192.168.1.10" || rules[0].Action != "block" || rules[0].Priority != 1 {
		t.Errorf("rules[0] = %+v, unexpected", rules[0])
	}
	if rules[1].BulklistID != "1" || rules[1].BulklistName != "blocked-ips" {
		t.Errorf("rules[1] = %+v, want bulklist id 1 name blocked-ips", rules[1])
	}
	if rules[0].ZoneUUID != "zone-1" {
		t.Errorf("ZoneUUID = %q, want zone-1", rules[0].ZoneUUID)
	}
}

func TestGetCDNAccessRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/access-management/rule-1" {
			t.Errorf("path = %s, want .../access-management/rule-1", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":
			{"id":"rule-1","type":"country","value":"1","action":"block","status":true,"priority":2,
			 "value_detail":{"name":"Afghanistan"},"bulklist":null}
		}`))
	})

	rule, err := c.GetCDNAccessRule(context.Background(), creds, "zone-1", "rule-1")
	if err != nil {
		t.Fatalf("GetCDNAccessRule: %v", err)
	}
	if rule.ID != "rule-1" || rule.Type != "country" || rule.Value != "1" || rule.Priority != 2 {
		t.Errorf("rule = %+v, unexpected", rule)
	}
}

func TestGetCDNAccessRuleNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNAccessRule(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestCreateCDNAccessRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	rule := domain.CDNAccessRule{Type: "ip", Value: "1.2.3.4", Action: "block"}
	created, err := c.CreateCDNAccessRule(context.Background(), creds, "zone-1", rule)
	if err != nil {
		t.Fatalf("CreateCDNAccessRule: %v", err)
	}
	if created.ZoneUUID != "zone-1" || created.Type != "ip" || created.Value != "1.2.3.4" || created.Action != "block" {
		t.Errorf("created = %+v, unexpected", created)
	}
}

func TestCreateCDNAccessRuleBadRequest(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"success":false,"message":"Operation failed!","errors":[]}`))
	})

	_, err := c.CreateCDNAccessRule(context.Background(), creds, "zone-1", domain.CDNAccessRule{Type: "ip", Value: "1.2.3.4", Action: "block"})
	if err == nil {
		t.Fatal("CreateCDNAccessRule: want error, got nil")
	}
}

func TestUpdateCDNAccessRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/access-management/rule-1" {
			t.Errorf("path = %s, want .../access-management/rule-1", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	rule := domain.CDNAccessRule{ID: "rule-1", Value: "1.2.3.4", Action: "allow", Status: true}
	updated, err := c.UpdateCDNAccessRule(context.Background(), creds, "zone-1", rule)
	if err != nil {
		t.Fatalf("UpdateCDNAccessRule: %v", err)
	}
	if updated.Action != "allow" || !updated.Status {
		t.Errorf("updated = %+v, want action allow, status true", updated)
	}
}

func TestUpdateCDNAccessRuleMissingID(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("provider should not be called when rule ID is missing")
	})

	_, err := c.UpdateCDNAccessRule(context.Background(), creds, "zone-1", domain.CDNAccessRule{Action: "block", Status: true})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteCDNAccessRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNAccessRule(context.Background(), creds, "zone-1", "rule-1"); err != nil {
		t.Fatalf("DeleteCDNAccessRule: %v", err)
	}
}

func TestDeleteCDNAccessRuleNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	err := c.DeleteCDNAccessRule(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetCDNIPReputationSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/ip-reputation" {
			t.Errorf("path = %s, want .../ip-reputation", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"ip_reputation_enabled":false,"ip_reputation_trust_time":3600,
			"ip_reputation_treat_score":"medium","ip_reputation_challenge":"recaptcha","attack_ban_time":900
		}}`))
	})

	settings, err := c.GetCDNIPReputation(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNIPReputation: %v", err)
	}
	if settings.Enabled || settings.TrustTime != 3600 || settings.TreatScore != "medium" || settings.Challenge != "recaptcha" || settings.AttackBanTime != 900 {
		t.Errorf("settings = %+v, unexpected", settings)
	}
}

func TestUpdateCDNIPReputationSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	in := domain.CDNIPReputationSettings{
		Enabled: true, TrustTime: 1800, TreatScore: "high", Challenge: "js", AttackBanTime: 600,
	}
	updated, err := c.UpdateCDNIPReputation(context.Background(), creds, "zone-1", in)
	if err != nil {
		t.Fatalf("UpdateCDNIPReputation: %v", err)
	}
	if *updated != in {
		t.Errorf("updated = %+v, want %+v", updated, in)
	}
}

func TestGetCDNDDoSActionsSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/ddos-actions" {
			t.Errorf("path = %s, want .../ddos-actions", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"action":"js","trust_time":10,"ban_time":10}}`))
	})

	settings, err := c.GetCDNDDoSActions(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNDDoSActions: %v", err)
	}
	if settings.Action != "js" || settings.TrustTime != 10 || settings.BanTime != 10 {
		t.Errorf("settings = %+v, unexpected", settings)
	}
}

func TestUpdateCDNDDoSActionsSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	in := domain.CDNDDoSActionSettings{Action: "block", TrustTime: 3600, BanTime: 900}
	updated, err := c.UpdateCDNDDoSActions(context.Background(), creds, "zone-1", in)
	if err != nil {
		t.Fatalf("UpdateCDNDDoSActions: %v", err)
	}
	if *updated != in {
		t.Errorf("updated = %+v, want %+v", updated, in)
	}
}

func TestUpdateCDNDDoSActionsServerError(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"Server Error"}`))
	})

	_, err := c.UpdateCDNDDoSActions(context.Background(), creds, "zone-1", domain.CDNDDoSActionSettings{Action: "none"})
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}
