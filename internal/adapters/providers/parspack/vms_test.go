package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/adapters/providers/parspack"
	"github.com/javadib/do0ps/internal/core/domain"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *parspack.Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := parspack.New(parspack.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

var creds = domain.ProviderCredentials{APIKey: "test-key"}

func TestCreateServerSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/vms" {
			t.Errorf("path = %s, want /api/public/v1/vms", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["size"] != "s-1vcpu-1gb" {
			t.Errorf("size = %v, want s-1vcpu-1gb", body["size"])
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"vm":{"id":"vm-1","name":"web-01","status":"new","vcpus":1,"memory":1024,"disk":25,
			"region":{"slug":"tehran"},"image":{"slug":"ubuntu-24.04"},"size":{"slug":"s-1vcpu-1gb","price_hourly":0.01,"price_monthly":6},
			"networks":{"v4":[{"ip_address":"203.0.113.5","type":"public"}]},"created_at":"2026-08-17T12:00:00Z"}}`))
	})

	srv, err := c.CreateServer(context.Background(), creds, domain.ServerSpec{
		Name: "web-01", Region: "tehran", Image: "ubuntu-24.04", PlanID: "s-1vcpu-1gb",
	})
	if err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	if srv.ID != "vm-1" || srv.Status != domain.ServerStatusProvisioning {
		t.Errorf("server = %+v, want id vm-1 and status provisioning", srv)
	}
	if srv.IPv4 != "203.0.113.5" {
		t.Errorf("IPv4 = %q, want 203.0.113.5", srv.IPv4)
	}
	if srv.PriceMonthly != 6 {
		t.Errorf("PriceMonthly = %v, want 6", srv.PriceMonthly)
	}
}

func TestCreateServerWithoutPlanIDIsRejected(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no request should reach the transport when plan_id is missing")
	})

	_, err := c.CreateServer(context.Background(), creds, domain.ServerSpec{Name: "web-01", CPUCores: 2, RAMMB: 2048})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCreateServerMapsUnauthorized(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	})

	_, err := c.CreateServer(context.Background(), creds, domain.ServerSpec{Name: "web-01", PlanID: "s-1vcpu-1gb"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestGetServerSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/public/v1/vms/vm-1" {
			t.Errorf("path = %s, want /api/public/v1/vms/vm-1", got)
		}
		_, _ = w.Write([]byte(`{"vm":{"id":"vm-1","name":"web-01","status":"active"}}`))
	})

	srv, err := c.GetServer(context.Background(), creds, "vm-1")
	if err != nil {
		t.Fatalf("GetServer: %v", err)
	}
	if srv.Status != domain.ServerStatusRunning {
		t.Errorf("status = %s, want running", srv.Status)
	}
}

func TestGetServerNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	})

	_, err := c.GetServer(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestListServersSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"vms":[{"id":"vm-1","name":"web-01","status":"active"},{"id":"vm-2","name":"web-02","status":"off"}]}`))
	})

	servers, err := c.ListServers(context.Background(), creds)
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("len(servers) = %d, want 2", len(servers))
	}
	if servers[1].Status != domain.ServerStatusStopped {
		t.Errorf("servers[1].Status = %s, want stopped", servers[1].Status)
	}
}

func TestListServersProviderUnavailable(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})

	_, err := c.ListServers(context.Background(), creds)
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}

func TestDeleteServerSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/vms/vm-1" {
			t.Errorf("path = %s, want /api/public/v1/vms/vm-1", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.DeleteServer(context.Background(), creds, "vm-1"); err != nil {
		t.Fatalf("DeleteServer: %v", err)
	}
}

func TestDeleteServerNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.DeleteServer(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestFindServerByNameFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"vms":[{"id":"vm-1","name":"web-01","status":"active"}]}`))
	})

	srv, err := c.FindServerByName(context.Background(), creds, "web-01")
	if err != nil {
		t.Fatalf("FindServerByName: %v", err)
	}
	if srv.ID != "vm-1" {
		t.Errorf("ID = %q, want vm-1", srv.ID)
	}
}

func TestFindServerByNameNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"vms":[]}`))
	})

	_, err := c.FindServerByName(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

// TestErrorResponseMessagePropagates proves a provider error message reaches
// the caller instead of being discarded, so failures are debuggable.
func TestErrorResponseMessagePropagates(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"region is not valid"}`))
	})

	_, err := c.CreateServer(context.Background(), creds, domain.ServerSpec{Name: "web-01", PlanID: "s-1vcpu-1gb"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if got := err.Error(); !strings.Contains(got, "region is not valid") {
		t.Errorf("error = %q, want it to contain the provider message", got)
	}
}
