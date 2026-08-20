package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestCreateLoadBalancerSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/load_balancers" {
			t.Errorf("path = %s, want /api/public/v1/load_balancers", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "api-lb" {
			t.Errorf("name = %v, want api-lb", body["name"])
		}
		if body["region"] != "tehran" {
			t.Errorf("region = %v, want tehran", body["region"])
		}
		if body["redirect_http_to_https"] != true {
			t.Errorf("redirect_http_to_https = %v, want true", body["redirect_http_to_https"])
		}
		fw, ok := body["forwarding_rules"].([]any)
		if !ok || len(fw) != 1 {
			t.Fatalf("forwarding_rules = %v, want one rule", body["forwarding_rules"])
		}
		rule := fw[0].(map[string]any)
		if rule["entry_port"] != float64(80) || rule["target_port"] != float64(8080) {
			t.Errorf("rule = %v, want entry 80 -> target 8080", rule)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"load_balancer":{"id":"lb-1","name":"api-lb","status":"new","algorithm":"round_robin",
			"ip":"203.0.113.20","region":{"slug":"tehran"},"vm_ids":["vm-1","vm-2"],
			"forwarding_rules":[{"entry_protocol":"http","entry_port":80,"target_protocol":"http","target_port":8080}],
			"health_check":{"protocol":"http","port":8080,"path":"/health","check_interval_seconds":10,"response_timeout_seconds":5,"unhealthy_threshold":3,"healthy_threshold":5},
			"redirect_http_to_https":true,"vpc_uuid":"vpc-1","created_at":"2026-08-17T12:00:00Z"}}`))
	})

	lb, err := c.CreateLoadBalancer(context.Background(), creds, domain.LoadBalancer{
		Name: "api-lb", Region: "tehran",
		ForwardingRules:   []domain.ForwardingRule{{EntryProtocol: "http", EntryPort: 80, TargetProtocol: "http", TargetPort: 8080}},
		ServerIDs:         []string{"vm-1", "vm-2"},
		RedirectHTTPToTLS: true,
	})
	if err != nil {
		t.Fatalf("CreateLoadBalancer: %v", err)
	}
	if lb.ID != "lb-1" || lb.Status != "new" {
		t.Errorf("load balancer = %+v, want id lb-1 and status new", lb)
	}
	if lb.IP != "203.0.113.20" {
		t.Errorf("IP = %q, want 203.0.113.20", lb.IP)
	}
	if lb.Region != "tehran" {
		t.Errorf("Region = %q, want tehran", lb.Region)
	}
	if len(lb.ForwardingRules) != 1 || lb.ForwardingRules[0].EntryPort != 80 {
		t.Errorf("ForwardingRules = %+v, want one rule with entry port 80", lb.ForwardingRules)
	}
	if lb.HealthCheck == nil || lb.HealthCheck.Port != 8080 || lb.HealthCheck.Path != "/health" {
		t.Errorf("HealthCheck = %+v, want port 8080 and path /health", lb.HealthCheck)
	}
	if !lb.RedirectHTTPToTLS {
		t.Errorf("RedirectHTTPToTLS = false, want true")
	}
}

func TestGetLoadBalancerSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/public/v1/load_balancers/lb-1" {
			t.Errorf("path = %s, want /api/public/v1/load_balancers/lb-1", got)
		}
		_, _ = w.Write([]byte(`{"load_balancer":{"id":"lb-1","name":"api-lb","status":"active"}}`))
	})

	lb, err := c.GetLoadBalancer(context.Background(), creds, "lb-1")
	if err != nil {
		t.Fatalf("GetLoadBalancer: %v", err)
	}
	if lb.Status != "active" {
		t.Errorf("status = %q, want active", lb.Status)
	}
}

func TestGetLoadBalancerNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	})

	_, err := c.GetLoadBalancer(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestListLoadBalancersSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"load_balancers":[{"id":"lb-1","name":"api-lb","status":"active"},{"id":"lb-2","name":"db-lb","status":"new"}]}`))
	})

	balancers, err := c.ListLoadBalancers(context.Background(), creds)
	if err != nil {
		t.Fatalf("ListLoadBalancers: %v", err)
	}
	if len(balancers) != 2 {
		t.Fatalf("len(balancers) = %d, want 2", len(balancers))
	}
	if balancers[1].Name != "db-lb" {
		t.Errorf("balancers[1].Name = %q, want db-lb", balancers[1].Name)
	}
}

func TestUpdateLoadBalancerSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/load_balancers/lb-1" {
			t.Errorf("path = %s, want /api/public/v1/load_balancers/lb-1", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "api-lb" {
			t.Errorf("name = %v, want api-lb", body["name"])
		}
		_, _ = w.Write([]byte(`{"load_balancer":{"id":"lb-1","name":"api-lb","status":"active","vm_ids":["vm-3"]}}`))
	})

	lb, err := c.UpdateLoadBalancer(context.Background(), creds, "lb-1", domain.LoadBalancer{
		Name:      "api-lb",
		ServerIDs: []string{"vm-3"},
	})
	if err != nil {
		t.Fatalf("UpdateLoadBalancer: %v", err)
	}
	if len(lb.ServerIDs) != 1 || lb.ServerIDs[0] != "vm-3" {
		t.Errorf("ServerIDs = %v, want [vm-3]", lb.ServerIDs)
	}
}

func TestDeleteLoadBalancerSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/load_balancers/lb-1" {
			t.Errorf("path = %s, want /api/public/v1/load_balancers/lb-1", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.DeleteLoadBalancer(context.Background(), creds, "lb-1"); err != nil {
		t.Fatalf("DeleteLoadBalancer: %v", err)
	}
}

func TestDeleteLoadBalancerNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.DeleteLoadBalancer(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestFindLoadBalancerByNameFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"load_balancers":[{"id":"lb-1","name":"api-lb","status":"active"}]}`))
	})

	lb, err := c.FindLoadBalancerByName(context.Background(), creds, "api-lb")
	if err != nil {
		t.Fatalf("FindLoadBalancerByName: %v", err)
	}
	if lb.ID != "lb-1" {
		t.Errorf("ID = %q, want lb-1", lb.ID)
	}
}

func TestFindLoadBalancerByNameNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"load_balancers":[]}`))
	})

	_, err := c.FindLoadBalancerByName(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

// TestCreateLoadBalancerMapsUnauthorized proves error translation reuses the
// shared error mapping of the client.
func TestCreateLoadBalancerMapsUnauthorized(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	})

	_, err := c.CreateLoadBalancer(context.Background(), creds, domain.LoadBalancer{
		Name:            "api-lb",
		ForwardingRules: []domain.ForwardingRule{{EntryProtocol: "http", EntryPort: 80, TargetProtocol: "http", TargetPort: 80}},
	})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want domain.ErrInvalidCredentials", err)
	}
}
