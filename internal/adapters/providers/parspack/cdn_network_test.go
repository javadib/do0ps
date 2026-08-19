package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestGetCDNHTTPSConvertorSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/https-convertor" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/https-convertor", got)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"enabled":true}}`))
	})

	setting, err := c.GetCDNHTTPSConvertor(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNHTTPSConvertor: %v", err)
	}
	if !setting.Enabled {
		t.Errorf("Enabled = %v, want true", setting.Enabled)
	}
}

func TestGetCDNHTTPSConvertorNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNHTTPSConvertor(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNHTTPSConvertorSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["enabled"] != true {
			t.Errorf("body = %+v, want enabled true", body)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	setting, err := c.UpdateCDNHTTPSConvertor(context.Background(), creds, "zone-1", domain.CDNHTTPSConvertorSetting{Enabled: true})
	if err != nil {
		t.Fatalf("UpdateCDNHTTPSConvertor: %v", err)
	}
	if !setting.Enabled {
		t.Errorf("Enabled = %v, want true", setting.Enabled)
	}
}

func TestUpdateCDNHTTPSConvertorUnauthorized(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthenticated."}`))
	})

	_, err := c.UpdateCDNHTTPSConvertor(context.Background(), creds, "zone-1", domain.CDNHTTPSConvertorSetting{Enabled: true})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestGetCDNEdgeToUpstreamConnectionSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/edge-to-upstream-connection" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/edge-to-upstream-connection", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"type":"auto"}}`))
	})

	setting, err := c.GetCDNEdgeToUpstreamConnection(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNEdgeToUpstreamConnection: %v", err)
	}
	if setting.Type != "auto" {
		t.Errorf("Type = %q, want auto", setting.Type)
	}
}

func TestUpdateCDNEdgeToUpstreamConnectionSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["type"] != "https" {
			t.Errorf("body = %+v, want type https", body)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	setting, err := c.UpdateCDNEdgeToUpstreamConnection(context.Background(), creds, "zone-1", domain.CDNEdgeToUpstreamConnectionSetting{Type: "https"})
	if err != nil {
		t.Fatalf("UpdateCDNEdgeToUpstreamConnection: %v", err)
	}
	if setting.Type != "https" {
		t.Errorf("Type = %q, want https", setting.Type)
	}
}

func TestGetCDNWWWRedirectionSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/www-redirection" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/www-redirection", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"www_redirection":"none"}}`))
	})

	setting, err := c.GetCDNWWWRedirection(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNWWWRedirection: %v", err)
	}
	if setting.Mode != "none" {
		t.Errorf("Mode = %q, want none", setting.Mode)
	}
}

func TestUpdateCDNWWWRedirectionSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["www_redirection"] != "redirect-to-www" {
			t.Errorf("body = %+v, want www_redirection redirect-to-www", body)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	setting, err := c.UpdateCDNWWWRedirection(context.Background(), creds, "zone-1", domain.CDNWWWRedirectionSetting{Mode: "redirect-to-www"})
	if err != nil {
		t.Fatalf("UpdateCDNWWWRedirection: %v", err)
	}
	if setting.Mode != "redirect-to-www" {
		t.Errorf("Mode = %q, want redirect-to-www", setting.Mode)
	}
}

func TestUpdateCDNWWWRedirectionValidationError(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The www_redirection parameter is required.","errors":{"www_redirection":["The www_redirection is required."]}}`))
	})

	_, err := c.UpdateCDNWWWRedirection(context.Background(), creds, "zone-1", domain.CDNWWWRedirectionSetting{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNWebSocketSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/web-socket" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/web-socket", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"enabled":false}}`))
	})

	setting, err := c.GetCDNWebSocket(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNWebSocket: %v", err)
	}
	if setting.Enabled {
		t.Errorf("Enabled = %v, want false", setting.Enabled)
	}
}

func TestUpdateCDNWebSocketSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["enabled"] != true {
			t.Errorf("body = %+v, want enabled true", body)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done"}`))
	})

	setting, err := c.UpdateCDNWebSocket(context.Background(), creds, "zone-1", domain.CDNWebSocketSetting{Enabled: true})
	if err != nil {
		t.Fatalf("UpdateCDNWebSocket: %v", err)
	}
	if !setting.Enabled {
		t.Errorf("Enabled = %v, want true", setting.Enabled)
	}
}

func TestUpdateCDNWebSocketNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.UpdateCDNWebSocket(context.Background(), creds, "missing", domain.CDNWebSocketSetting{Enabled: true})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}
