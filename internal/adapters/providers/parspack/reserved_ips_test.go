package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestReserveIPSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/reserved_ips" {
			t.Errorf("path = %s, want /api/public/v1/reserved_ips", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["region"] != "tehran" {
			t.Errorf("region = %v, want tehran", body["region"])
		}

		_, _ = w.Write([]byte(`{"reserved_ip":{"ip":"203.0.113.10","region":{"slug":"tehran","name":"Tehran"},"locked":false}}`))
	})

	ip, err := c.ReserveIP(context.Background(), creds, "tehran")
	if err != nil {
		t.Fatalf("ReserveIP: %v", err)
	}
	if ip.IPAddress != "203.0.113.10" {
		t.Errorf("IPAddress = %q, want 203.0.113.10", ip.IPAddress)
	}
	if ip.Region != "tehran" {
		t.Errorf("Region = %q, want tehran", ip.Region)
	}
	if ip.ServerID != "" {
		t.Errorf("ServerID = %q, want empty — a newly reserved IP is unassigned", ip.ServerID)
	}
	if ip.URN != "do:reservedip:203.0.113.10" {
		t.Errorf("URN = %q, want do:reservedip:203.0.113.10", ip.URN)
	}
}

func TestReserveIPMapsNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"no such region"}`))
	})

	_, err := c.ReserveIP(context.Background(), creds, "nowhere")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestReleaseIPSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/reserved_ips/203.0.113.10" {
			t.Errorf("path = %s, want /api/public/v1/reserved_ips/203.0.113.10", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.ReleaseIP(context.Background(), creds, "203.0.113.10"); err != nil {
		t.Fatalf("ReleaseIP: %v", err)
	}
}

func TestReleaseIPNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.ReleaseIP(context.Background(), creds, "203.0.113.99")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

// TestAssignIPToServer posts the assign action with the target vm_id and then
// reads the reserved IP back so the caller sees its updated attachment.
func TestAssignIPToServerSuccess(t *testing.T) {
	var calls []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/public/v1/reserved_ips/203.0.113.10/actions":
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding request body: %v", err)
			}
			if body["type"] != "assign" {
				t.Errorf("type = %v, want assign", body["type"])
			}
			if body["vm_id"] != "vm-1" {
				t.Errorf("vm_id = %v, want vm-1", body["vm_id"])
			}
			_, _ = w.Write([]byte(`{"action":{"id":1,"status":"in-progress","type":"assign"}}`))
		case "/api/public/v1/reserved_ips/203.0.113.10":
			_, _ = w.Write([]byte(`{"reserved_ip":{"ip":"203.0.113.10","region":{"slug":"tehran"},"vm":{"id":"vm-1","name":"web-01","status":"active"}}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	ip, err := c.AssignIPToServer(context.Background(), creds, "203.0.113.10", "vm-1")
	if err != nil {
		t.Fatalf("AssignIPToServer: %v", err)
	}
	if ip.ServerID != "vm-1" {
		t.Errorf("ServerID = %q, want vm-1", ip.ServerID)
	}
	if len(calls) != 2 {
		t.Errorf("made %d calls, want 2 (action then read-back)", len(calls))
	}
}

func TestAssignIPToServerMapsUnprocessable(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"server not in the reserved IP's region"}`))
	})

	_, err := c.AssignIPToServer(context.Background(), creds, "203.0.113.10", "vm-2")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

// TestUnassignIPSuccess posts the unassign action (no vm_id) and reads the
// address back with its server attachment cleared.
func TestUnassignIPSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/public/v1/reserved_ips/203.0.113.10/actions":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding request body: %v", err)
			}
			if body["type"] != "unassign" {
				t.Errorf("type = %v, want unassign", body["type"])
			}
			if _, sent := body["vm_id"]; sent {
				t.Errorf("unassign request must not carry vm_id, got %v", body["vm_id"])
			}
			_, _ = w.Write([]byte(`{"action":{"id":2,"status":"in-progress","type":"unassign"}}`))
		case "/api/public/v1/reserved_ips/203.0.113.10":
			_, _ = w.Write([]byte(`{"reserved_ip":{"ip":"203.0.113.10","region":{"slug":"tehran"}}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	ip, err := c.UnassignIP(context.Background(), creds, "203.0.113.10")
	if err != nil {
		t.Fatalf("UnassignIP: %v", err)
	}
	if ip.ServerID != "" {
		t.Errorf("ServerID = %q, want empty after unassign", ip.ServerID)
	}
}
