package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestCreateFirewallSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/firewalls" {
			t.Errorf("path = %s, want /api/public/v1/firewalls", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "web" {
			t.Errorf("name = %v, want web", body["name"])
		}
		if ids, ok := body["vm_ids"].([]any); !ok || len(ids) != 1 || ids[0] != "vm-1" {
			t.Errorf("vm_ids = %v, want [vm-1]", body["vm_ids"])
		}
		inbound, ok := body["inbound_rules"].([]any)
		if !ok || len(inbound) != 1 {
			t.Fatalf("inbound_rules = %v, want one rule", body["inbound_rules"])
		}
		rule := inbound[0].(map[string]any)
		if rule["protocol"] != "tcp" || rule["ports"] != "22" {
			t.Errorf("rule = %v, want protocol tcp ports 22", rule)
		}
		sources, ok := rule["sources"].(map[string]any)
		if !ok || sources["addresses"] == nil {
			t.Errorf("rule sources = %v, want addresses", rule["sources"])
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"firewall":{"id":"fw-1","name":"web","status":"succeeded","vm_ids":["vm-1"],
			"inbound_rules":[{"protocol":"tcp","ports":"22","sources":{"addresses":["0.0.0.0/0"]}}],
			"created_at":"2026-08-17T12:00:00Z"}}`))
	})

	fw, err := c.CreateFirewall(context.Background(), creds, domain.Firewall{
		Name:      "web",
		ServerIDs: []string{"vm-1"},
		InboundRules: []domain.FirewallRule{
			{Protocol: "tcp", PortRange: "22", Addresses: []string{"0.0.0.0/0"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateFirewall: %v", err)
	}
	if fw.ID != "fw-1" || fw.Status != "succeeded" {
		t.Errorf("firewall = %+v, want id fw-1 and status succeeded", fw)
	}
	if len(fw.InboundRules) != 1 || fw.InboundRules[0].Protocol != "tcp" {
		t.Errorf("inbound rules = %+v, want one tcp rule", fw.InboundRules)
	}
	if got := fw.InboundRules[0].Addresses; len(got) != 1 || got[0] != "0.0.0.0/0" {
		t.Errorf("rule addresses = %v, want [0.0.0.0/0]", got)
	}
}

func TestCreateFirewallMapsUnauthorized(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	})

	_, err := c.CreateFirewall(context.Background(), creds, domain.Firewall{Name: "web"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestGetFirewallSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/public/v1/firewalls/fw-1" {
			t.Errorf("path = %s, want /api/public/v1/firewalls/fw-1", got)
		}
		_, _ = w.Write([]byte(`{"firewall":{"id":"fw-1","name":"web","status":"succeeded",
			"outbound_rules":[{"protocol":"udp","ports":"53","destinations":{"addresses":["0.0.0.0/0"]}}]}}`))
	})

	fw, err := c.GetFirewall(context.Background(), creds, "fw-1")
	if err != nil {
		t.Fatalf("GetFirewall: %v", err)
	}
	if fw.ID != "fw-1" || fw.Status != "succeeded" {
		t.Errorf("firewall = %+v, want id fw-1 and status succeeded", fw)
	}
	if len(fw.OutboundRules) != 1 || fw.OutboundRules[0].PortRange != "53" {
		t.Errorf("outbound rules = %+v, want one udp/53 rule", fw.OutboundRules)
	}
	if got := fw.OutboundRules[0].Addresses; len(got) != 1 || got[0] != "0.0.0.0/0" {
		t.Errorf("destination addresses = %v, want [0.0.0.0/0]", got)
	}
}

func TestGetFirewallNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	})

	_, err := c.GetFirewall(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestListFirewallsSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"firewalls":[{"id":"fw-1","name":"web","status":"succeeded"},{"id":"fw-2","name":"db","status":"waiting"}]}`))
	})

	firewalls, err := c.ListFirewalls(context.Background(), creds)
	if err != nil {
		t.Fatalf("ListFirewalls: %v", err)
	}
	if len(firewalls) != 2 {
		t.Fatalf("len(firewalls) = %d, want 2", len(firewalls))
	}
	if firewalls[1].Name != "db" || firewalls[1].Status != "waiting" {
		t.Errorf("firewalls[1] = %+v, want name db and status waiting", firewalls[1])
	}
}

func TestListFirewallsProviderUnavailable(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})

	_, err := c.ListFirewalls(context.Background(), creds)
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}

func TestUpdateFirewallSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/firewalls/fw-1" {
			t.Errorf("path = %s, want /api/public/v1/firewalls/fw-1", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "web-renamed" {
			t.Errorf("name = %v, want web-renamed", body["name"])
		}
		if _, ok := body["inbound_rules"]; ok {
			t.Errorf("inbound_rules should be omitted when none are sent, got %v", body["inbound_rules"])
		}

		_, _ = w.Write([]byte(`{"firewall":{"id":"fw-1","name":"web-renamed","status":"succeeded"}}`))
	})

	fw, err := c.UpdateFirewall(context.Background(), creds, "fw-1", domain.Firewall{Name: "web-renamed"})
	if err != nil {
		t.Fatalf("UpdateFirewall: %v", err)
	}
	if fw.Name != "web-renamed" {
		t.Errorf("Name = %q, want web-renamed", fw.Name)
	}
}

func TestDeleteFirewallSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/firewalls/fw-1" {
			t.Errorf("path = %s, want /api/public/v1/firewalls/fw-1", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.DeleteFirewall(context.Background(), creds, "fw-1"); err != nil {
		t.Fatalf("DeleteFirewall: %v", err)
	}
}

func TestDeleteFirewallNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.DeleteFirewall(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}
