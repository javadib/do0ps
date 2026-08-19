package parspack_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestGetCDNModSecStatusSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/mod-security" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/firewalls/mod-security", got)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"standards":[{"id":"lbr2yJAk","name":"docs-standard-1","selected":false}],
			"customs":[{"id":"l3Ad770V","name":"docs-custom-1","selected":true}]
		}}`))
	})

	status, err := c.GetCDNModSecStatus(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNModSecStatus: %v", err)
	}
	if len(status.Standards) != 1 || status.Standards[0].ID != "lbr2yJAk" || status.Standards[0].Selected {
		t.Errorf("standards = %+v, want one unselected lbr2yJAk", status.Standards)
	}
	if len(status.Customs) != 1 || !status.Customs[0].Selected {
		t.Errorf("customs = %+v, want one selected entry", status.Customs)
	}
}

func TestGetCDNModSecStatusNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNModSecStatus(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNModSecStatusSuccess(t *testing.T) {
	calls := 0
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/mod-security" {
				t.Errorf("path = %s, want /external/api/v1/zones/zone-1/firewalls/mod-security", got)
			}
			_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
			return
		}
		// The second call is UpdateCDNModSecStatus's re-fetch via GetCDNModSecStatus.
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET on refetch", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"standards":[{"id":"AbCdEfGh","name":"docs-standard-1","selected":true}],
			"customs":[]
		}}`))
	})

	status, err := c.UpdateCDNModSecStatus(context.Background(), creds, "zone-1", []string{"AbCdEfGh"})
	if err != nil {
		t.Fatalf("UpdateCDNModSecStatus: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (update + refetch)", calls)
	}
	if len(status.Standards) != 1 || !status.Standards[0].Selected {
		t.Errorf("standards = %+v, want one selected entry from the refetch", status.Standards)
	}
}

func TestListCDNModSecDataSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/mod-security/data" {
			t.Errorf("path = %s, want .../data", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[{"id":"eNrxeaO2","name":"docs-check"}]}`))
	})

	data, err := c.ListCDNModSecData(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListCDNModSecData: %v", err)
	}
	if len(data) != 1 || data[0].ID != "eNrxeaO2" || data[0].Name != "docs-check" {
		t.Errorf("data = %+v, want a single docs-check entry", data)
	}
	if data[0].Value != "" {
		t.Errorf("Value = %q, want empty on the list endpoint", data[0].Value)
	}
}

func TestCreateCDNModSecDataSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/mod-security/data" {
			t.Errorf("path = %s, want .../data", got)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	data, err := c.CreateCDNModSecData(context.Background(), creds, "zone-1", domain.CDNModSecData{Name: "sample-data", Value: "sample-data"})
	if err != nil {
		t.Fatalf("CreateCDNModSecData: %v", err)
	}
	if data.Name != "sample-data" || data.Value != "sample-data" {
		t.Errorf("data = %+v, want the echoed input", data)
	}
	if data.ID != "" {
		t.Errorf("ID = %q, want empty since the provider never returns it on create", data.ID)
	}
}

func TestGetCDNModSecDataSuccess(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("sample-data"))
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/mod-security/data/eNrxeaO2" {
			t.Errorf("path = %s, want .../data/eNrxeaO2", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"id":"eNrxeaO2","name":"docs-check","value":"` + encoded + `"}}`))
	})

	data, err := c.GetCDNModSecData(context.Background(), creds, "zone-1", "eNrxeaO2")
	if err != nil {
		t.Fatalf("GetCDNModSecData: %v", err)
	}
	if data.Value != "sample-data" {
		t.Errorf("Value = %q, want decoded sample-data", data.Value)
	}
}

func TestGetCDNModSecDataNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNModSecData(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNModSecDataSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	data, err := c.UpdateCDNModSecData(context.Background(), creds, "zone-1", "eNrxeaO2", domain.CDNModSecData{Name: "renamed", Value: "new-value"})
	if err != nil {
		t.Fatalf("UpdateCDNModSecData: %v", err)
	}
	if data.ID != "eNrxeaO2" || data.Name != "renamed" || data.Value != "new-value" {
		t.Errorf("data = %+v, want id eNrxeaO2 with the updated fields", data)
	}
}

func TestDeleteCDNModSecDataSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNModSecData(context.Background(), creds, "zone-1", "eNrxeaO2"); err != nil {
		t.Fatalf("DeleteCDNModSecData: %v", err)
	}
}

func TestListCDNModSecRulesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/mod-security/rules" {
			t.Errorf("path = %s, want .../rules", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[{"id":"l3Ad770V","name":"docs-custom-1","status":"verified"}]}`))
	})

	rules, err := c.ListCDNModSecRules(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListCDNModSecRules: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != "l3Ad770V" || rules[0].Status != "verified" {
		t.Errorf("rules = %+v, want a single verified l3Ad770V entry", rules)
	}
}

func TestCreateCDNModSecRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/mod-security/rules" {
			t.Errorf("path = %s, want .../rules", got)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rule, err := c.CreateCDNModSecRule(context.Background(), creds, "zone-1", domain.CDNModSecRule{
		Name: "sample-rule", RuleValue: "sample-rule", ModSecDataIDs: []string{"AbCdEfGh"},
	})
	if err != nil {
		t.Fatalf("CreateCDNModSecRule: %v", err)
	}
	if rule.Name != "sample-rule" || rule.RuleValue != "sample-rule" {
		t.Errorf("rule = %+v, want the echoed input", rule)
	}
	if rule.ID != "" {
		t.Errorf("ID = %q, want empty since the provider never returns it on create", rule.ID)
	}
}

func TestGetCDNModSecRuleSuccess(t *testing.T) {
	ruleEncoded := base64.StdEncoding.EncodeToString([]byte("sample-custom"))
	dataEncoded := base64.StdEncoding.EncodeToString([]byte("sample-data"))
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/mod-security/rules/l3Ad770V" {
			t.Errorf("path = %s, want .../rules/l3Ad770V", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"id":"l3Ad770V","rule_value":"` + ruleEncoded + `",
			"mod_sec_data":[{"id":"eNrxeaO2","name":"docs-check","value":"` + dataEncoded + `"}]
		}}`))
	})

	rule, err := c.GetCDNModSecRule(context.Background(), creds, "zone-1", "l3Ad770V")
	if err != nil {
		t.Fatalf("GetCDNModSecRule: %v", err)
	}
	if rule.RuleValue != "sample-custom" {
		t.Errorf("RuleValue = %q, want decoded sample-custom", rule.RuleValue)
	}
	if len(rule.ModSecData) != 1 || rule.ModSecData[0].Value != "sample-data" {
		t.Errorf("ModSecData = %+v, want one decoded entry", rule.ModSecData)
	}
	if rule.Name != "" || rule.Status != "" {
		t.Errorf("Name/Status = %q/%q, want both empty since the show endpoint omits them", rule.Name, rule.Status)
	}
}

func TestGetCDNModSecRuleNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNModSecRule(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNModSecRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rule, err := c.UpdateCDNModSecRule(context.Background(), creds, "zone-1", "l3Ad770V", domain.CDNModSecRule{
		Name: "renamed-rule", RuleValue: "new-value", ModSecDataIDs: []string{"AbCdEfGh"},
	})
	if err != nil {
		t.Fatalf("UpdateCDNModSecRule: %v", err)
	}
	if rule.ID != "l3Ad770V" || rule.Name != "renamed-rule" {
		t.Errorf("rule = %+v, want id l3Ad770V with the updated fields", rule)
	}
}

func TestDeleteCDNModSecRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNModSecRule(context.Background(), creds, "zone-1", "l3Ad770V"); err != nil {
		t.Fatalf("DeleteCDNModSecRule: %v", err)
	}
}

func TestDeleteCDNModSecRuleNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	err := c.DeleteCDNModSecRule(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}
