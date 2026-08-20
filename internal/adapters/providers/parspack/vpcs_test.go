package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestCreateVPCSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/vpcs" {
			t.Errorf("path = %s, want /api/public/v1/vpcs", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "web-net" {
			t.Errorf("name = %v, want web-net", body["name"])
		}
		if body["region"] != "tehran" {
			t.Errorf("region = %v, want tehran", body["region"])
		}
		if body["ip_range"] != "10.10.10.0/24" {
			t.Errorf("ip_range = %v, want 10.10.10.0/24", body["ip_range"])
		}

		_, _ = w.Write([]byte(`{"vpc":{"id":"vpc-1","name":"web-net","region":"tehran",
			"description":"web tier","ip_range":"10.10.10.0/24","default":false,
			"created_at":"2026-08-17T12:00:00Z"}}`))
	})

	vpc, err := c.CreateVPC(context.Background(), creds, domain.VPC{
		Name:        "web-net",
		Region:      "tehran",
		Description: "web tier",
		IPRange:     "10.10.10.0/24",
	})
	if err != nil {
		t.Fatalf("CreateVPC: %v", err)
	}
	if vpc.ID != "vpc-1" || vpc.Name != "web-net" {
		t.Errorf("vpc = %+v, want id vpc-1 and name web-net", vpc)
	}
	if vpc.IPRange != "10.10.10.0/24" || vpc.Default {
		t.Errorf("vpc = %+v, want ip_range 10.10.10.0/24 and default=false", vpc)
	}
	if vpc.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero, want a parsed timestamp")
	}
}

func TestCreateVPCMapsUnauthorized(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	})

	_, err := c.CreateVPC(context.Background(), creds, domain.VPC{Name: "web-net", Region: "tehran"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestGetVPCSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/vpcs/vpc-1" {
			t.Errorf("path = %s, want /api/public/v1/vpcs/vpc-1", got)
		}
		_, _ = w.Write([]byte(`{"vpc":{"id":"vpc-1","name":"web-net","region":"tehran","default":true}}`))
	})

	vpc, err := c.GetVPC(context.Background(), creds, "vpc-1")
	if err != nil {
		t.Fatalf("GetVPC: %v", err)
	}
	if vpc.ID != "vpc-1" || !vpc.Default {
		t.Errorf("vpc = %+v, want id vpc-1 and default=true", vpc)
	}
}

func TestGetVPCNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	})

	_, err := c.GetVPC(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestListVPCsSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"vpcs":[{"id":"vpc-1","name":"web-net","region":"tehran"},
			{"id":"vpc-2","name":"db-net","region":"tehran"}]}`))
	})

	vpcs, err := c.ListVPCs(context.Background(), creds)
	if err != nil {
		t.Fatalf("ListVPCs: %v", err)
	}
	if len(vpcs) != 2 {
		t.Fatalf("len(vpcs) = %d, want 2", len(vpcs))
	}
	if vpcs[1].Name != "db-net" {
		t.Errorf("vpcs[1].Name = %q, want db-net", vpcs[1].Name)
	}
}

func TestListVPCsProviderUnavailable(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})

	_, err := c.ListVPCs(context.Background(), creds)
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}

func TestDeleteVPCSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/vpcs/vpc-1" {
			t.Errorf("path = %s, want /api/public/v1/vpcs/vpc-1", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.DeleteVPC(context.Background(), creds, "vpc-1"); err != nil {
		t.Fatalf("DeleteVPC: %v", err)
	}
}

func TestDeleteVPCNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.DeleteVPC(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}
