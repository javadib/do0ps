package parspack_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestUpdateCDNCacheTTLSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/cache/ttl" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/cache/ttl", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	setting, err := c.UpdateCDNCacheTTL(context.Background(), creds, "zone-1", 10800)
	if err != nil {
		t.Fatalf("UpdateCDNCacheTTL: %v", err)
	}
	if setting.EdgeCacheTTLSeconds != 10800 {
		t.Errorf("EdgeCacheTTLSeconds = %d, want 10800", setting.EdgeCacheTTLSeconds)
	}
}

func TestUpdateCDNCacheTTLNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.UpdateCDNCacheTTL(context.Background(), creds, "missing", 3600)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNCacheRuleSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/cache/rule" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/cache/rule", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	setting, err := c.UpdateCDNCacheRule(context.Background(), creds, "zone-1", "cdn-smart-caching")
	if err != nil {
		t.Fatalf("UpdateCDNCacheRule: %v", err)
	}
	if setting.CacheRule != "cdn-smart-caching" {
		t.Errorf("CacheRule = %q, want cdn-smart-caching", setting.CacheRule)
	}
}

func TestUpdateCDNCacheRuleValidationError(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The cache_rule parameter is required.","errors":{"cache_rule":["The cache_rule is required."]}}`))
	})

	_, err := c.UpdateCDNCacheRule(context.Background(), creds, "zone-1", "")
	if err == nil {
		t.Fatal("UpdateCDNCacheRule: want error for a 422 response, got nil")
	}
}

func TestUpdateCDNCacheUserAgentSettingSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/cache/user-agent" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/cache/user-agent", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	setting, err := c.UpdateCDNCacheUserAgentSetting(context.Background(), creds, "zone-1", true)
	if err != nil {
		t.Fatalf("UpdateCDNCacheUserAgentSetting: %v", err)
	}
	if !setting.Enabled {
		t.Errorf("Enabled = %v, want true", setting.Enabled)
	}
}

func TestGetCDNCacheSettingsSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/cache/settings" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/cache/settings", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"developer_mode":false,"maintenance_mode":false,"ignore_query_string":false,
			"cache_rule":"cdn-static-caching","edge_cache_ttl":3600,"origin_offline":true,
			"enable_cache_per_user_agent":true
		}}`))
	})

	settings, err := c.GetCDNCacheSettings(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNCacheSettings: %v", err)
	}
	if settings.CacheRule != "cdn-static-caching" || settings.EdgeCacheTTLSeconds != 3600 {
		t.Errorf("settings = %+v, want cache_rule cdn-static-caching, edge_cache_ttl 3600", settings)
	}
	if !settings.OriginOffline || !settings.EnableCachePerUserAgent {
		t.Errorf("settings = %+v, want origin_offline and enable_cache_per_user_agent both true", settings)
	}
}

func TestGetCDNCacheSettingsNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNCacheSettings(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestListCDNCacheEntriesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/cache" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/cache", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"OaB1AVVr","operation":"Purge All","status":"none","created_time":"2023-07-17 08:48:15","success_progress":0}
		]}`))
	})

	entries, err := c.ListCDNCacheEntries(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListCDNCacheEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "OaB1AVVr" || entries[0].Operation != "Purge All" {
		t.Errorf("entries = %+v, want a single Purge All entry with id OaB1AVVr", entries)
	}
}

func TestPurgeCDNCacheSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/cache" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/cache", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	if err := c.PurgeCDNCache(context.Background(), creds, "zone-1"); err != nil {
		t.Fatalf("PurgeCDNCache: %v", err)
	}
}

func TestPurgeCDNCacheNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	err := c.PurgeCDNCache(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetCDNCacheEntrySuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/cache/AbCdEfGh" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/cache/AbCdEfGh", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"id":"AbCdEfGh","operation":"Purge All","status":"none","created_time":"2023-07-17 08:48:15","success_progress":0
		}}`))
	})

	entry, err := c.GetCDNCacheEntry(context.Background(), creds, "zone-1", "AbCdEfGh")
	if err != nil {
		t.Fatalf("GetCDNCacheEntry: %v", err)
	}
	if entry.ID != "AbCdEfGh" || entry.Operation != "Purge All" {
		t.Errorf("entry = %+v, want id AbCdEfGh, operation Purge All", entry)
	}
}

func TestGetCDNCacheEntryNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNCacheEntry(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}
