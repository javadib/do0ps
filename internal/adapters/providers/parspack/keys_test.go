package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestCreateSSHKeySuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/account/keys" {
			t.Errorf("path = %s, want /api/public/v1/account/keys", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "laptop" {
			t.Errorf("name = %v, want laptop", body["name"])
		}
		if got := body["public_key"]; got == "" {
			t.Errorf("public_key = %v, want it to be sent", got)
		}

		_, _ = w.Write([]byte(`{"ssh_key":{"id":101,"name":"laptop","fingerprint":"aa:bb:cc","public_key":"ssh-ed25519 AAAAC3..."}}`))
	})

	key, err := c.CreateSSHKey(context.Background(), creds, domain.SSHKey{Name: "laptop", PublicKey: "ssh-ed25519 AAAAC3..."})
	if err != nil {
		t.Fatalf("CreateSSHKey: %v", err)
	}
	if key.ID != "101" {
		t.Errorf("ID = %q, want 101", key.ID)
	}
	if key.Fingerprint != "aa:bb:cc" {
		t.Errorf("Fingerprint = %q, want aa:bb:cc", key.Fingerprint)
	}
}

func TestCreateSSHKeyMapsUnauthorized(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	})

	_, err := c.CreateSSHKey(context.Background(), creds, domain.SSHKey{Name: "laptop", PublicKey: "ssh-ed25519 AAAAC3..."})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestCreateSSHKeyEmptyResponse(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})

	_, err := c.CreateSSHKey(context.Background(), creds, domain.SSHKey{Name: "laptop", PublicKey: "ssh-ed25519 AAAAC3..."})
	if err == nil {
		t.Fatal("expected an error for a key-less response, got nil")
	}
}

func TestListSSHKeysSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"ssh_keys":[
			{"id":101,"name":"laptop","fingerprint":"aa:bb","public_key":"ssh-ed25519 AAAA..."},
			{"id":102,"name":"ci-runner","fingerprint":"cc:dd","public_key":"ssh-rsa BBBB..."}
		]}`))
	})

	keys, err := c.ListSSHKeys(context.Background(), creds)
	if err != nil {
		t.Fatalf("ListSSHKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("len(keys) = %d, want 2", len(keys))
	}
	if keys[0].ID != "101" || keys[1].Name != "ci-runner" {
		t.Errorf("keys = %+v, want id 101 and name ci-runner", keys)
	}
}

func TestListSSHKeysProviderUnavailable(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})

	_, err := c.ListSSHKeys(context.Background(), creds)
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}

func TestDeleteSSHKeySuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/account/keys/101" {
			t.Errorf("path = %s, want /api/public/v1/account/keys/101", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.DeleteSSHKey(context.Background(), creds, "101"); err != nil {
		t.Fatalf("DeleteSSHKey: %v", err)
	}
}

func TestDeleteSSHKeyNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.DeleteSSHKey(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}
