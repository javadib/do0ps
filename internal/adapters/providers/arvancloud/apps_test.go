package arvancloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// TestListArvanCloudCdnApps pins the request shape and response parsing of
// GET /apps, including the draft-app filtering intent (the raw response
// includes drafts; the use-case layer filters them).
func TestListArvanCloudCdnApps(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"app-1","rank":5,"name":"Analytics Pro","slug":"analytics-pro",
				"short_description":"Page analytics","description":"Full analytics suite",
				"logo":"https://logo.example.com/a.png","pictures":["https://img.example.com/1.png"],
				"vendor":"Acme Corp","support_email":"help@acme.example.com",
				"status":"published","like_stats":{"likes_count":10,"dislikes_count":1},
				"like_by_account":true,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-06-01T00:00:00Z"},
			{"id":"app-2","rank":1,"name":"Draft Tool","slug":"draft-tool",
				"short_description":"Internal","description":"Internal tool",
				"status":"draft","like_stats":{"likes_count":0,"dislikes_count":0},
				"created_at":"2026-02-01T00:00:00Z","updated_at":"2026-02-01T00:00:00Z"}
		]}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	apps, err := provider.ListArvanCloudCdnApps(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudCdnApps() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/apps" {
		t.Fatalf("request = %+v, want a single GET /apps", records)
	}
	if len(apps) != 2 {
		t.Fatalf("len(apps) = %d, want 2", len(apps))
	}
	if apps[0].ID != "app-1" || apps[0].Name != "Analytics Pro" || apps[0].Status != domain.ArvanCloudCdnAppStatusPublished {
		t.Errorf("apps[0] = %+v, want the published app", apps[0])
	}
	if apps[0].LikeStats.LikesCount != 10 || apps[0].LikeStats.DislikesCount != 1 {
		t.Errorf("apps[0].LikeStats = %+v, want likes=10 dislikes=1", apps[0].LikeStats)
	}
	if apps[0].LikeByAccount == nil || !*apps[0].LikeByAccount {
		t.Errorf("apps[0].LikeByAccount = %v, want true", apps[0].LikeByAccount)
	}
	if apps[1].Status != domain.ArvanCloudCdnAppStatusDraft {
		t.Errorf("apps[1].Status = %q, want draft", apps[1].Status)
	}
}

// TestGetArvanCloudCdnApp pins GET /apps/{id}.
func TestGetArvanCloudCdnApp(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"app-1","rank":5,"name":"Analytics Pro","slug":"analytics-pro",
			"status":"published","like_stats":{"likes_count":10,"dislikes_count":1}}}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	app, err := provider.GetArvanCloudCdnApp(context.Background(), creds(), "app-1")
	if err != nil {
		t.Fatalf("GetArvanCloudCdnApp() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/apps/app-1" {
		t.Fatalf("request = %+v, want a single GET /apps/app-1", records)
	}
	if app.ID != "app-1" || app.Name != "Analytics Pro" {
		t.Errorf("app = %+v, want the parsed app", app)
	}
}

// TestLikeArvanCloudCdnApp pins POST /apps/{id} for a like action.
func TestLikeArvanCloudCdnApp(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"likes_count":11,"dislikes_count":1}}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	like := true
	stats, err := provider.LikeArvanCloudCdnApp(context.Background(), creds(), "app-1", &like)
	if err != nil {
		t.Fatalf("LikeArvanCloudCdnApp() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/apps/app-1" {
		t.Fatalf("request = %+v, want a single POST /apps/app-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["like"] != true {
		t.Errorf("request body[\"like\"] = %v, want true", body["like"])
	}
	if stats.LikesCount != 11 || stats.DislikesCount != 1 {
		t.Errorf("stats = %+v, want likes=11 dislikes=1", stats)
	}
}

// TestLikeArvanCloudCdnAppRetract pins POST /apps/{id} with like=null (vote
// retraction).
func TestLikeArvanCloudCdnAppRetract(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"likes_count":9,"dislikes_count":1}}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	stats, err := provider.LikeArvanCloudCdnApp(context.Background(), creds(), "app-1", nil)
	if err != nil {
		t.Fatalf("LikeArvanCloudCdnApp() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["like"] != nil {
		t.Errorf("request body[\"like\"] = %v, want null", body["like"])
	}
	if stats.LikesCount != 9 {
		t.Errorf("stats.LikesCount = %d, want 9", stats.LikesCount)
	}
}

// TestListArvanCloudCdnAppCategories pins GET /apps/category.
func TestListArvanCloudCdnAppCategories(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"cat-1","name":"Analytics","active":true,"order":1},
			{"id":"cat-2","name":"Security","active":true,"order":2,"name_translation":{"en":{"name":"Security"},"fa":{"name":"امنیت"}}}
		]}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	cats, err := provider.ListArvanCloudCdnAppCategories(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudCdnAppCategories() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/apps/category" {
		t.Fatalf("request = %+v, want a single GET /apps/category", records)
	}
	if len(cats) != 2 || cats[0].ID != "cat-1" || cats[0].Name != "Analytics" {
		t.Errorf("cats = %+v, want the two parsed categories", cats)
	}
	if cats[1].NameTranslation == nil || cats[1].NameTranslation["fa"].Name != "امنیت" {
		t.Errorf("cats[1].NameTranslation = %+v, want fa translation", cats[1].NameTranslation)
	}
}

// TestGetArvanCloudCdnAppCategory pins GET /apps/category/{id}.
func TestGetArvanCloudCdnAppCategory(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"cat-1","name":"Analytics","active":true,"order":1}}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	cat, err := provider.GetArvanCloudCdnAppCategory(context.Background(), creds(), "cat-1")
	if err != nil {
		t.Fatalf("GetArvanCloudCdnAppCategory() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/apps/category/cat-1" {
		t.Fatalf("request = %+v, want a single GET /apps/category/cat-1", records)
	}
	if cat.ID != "cat-1" || cat.Name != "Analytics" {
		t.Errorf("cat = %+v, want the parsed category", cat)
	}
}

// TestListArvanCloudDomainCdnApps pins GET /domains/{domain}/apps.
func TestListArvanCloudDomainCdnApps(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"app-1","name":"Analytics Pro","slug":"analytics-pro","status":"published",
			"like_stats":{"likes_count":10,"dislikes_count":1}}]}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	apps, err := provider.ListArvanCloudDomainCdnApps(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudDomainCdnApps() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/apps" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/apps", records)
	}
	if len(apps) != 1 || apps[0].ID != "app-1" {
		t.Errorf("apps = %+v, want the one installed app", apps)
	}
}

// TestCheckArvanCloudCdnAppInstalled_true pins GET /domains/{domain}/apps/{id}
// returning is_install=true.
func TestCheckArvanCloudCdnAppInstalled_true(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"is_install":true}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	installed, err := provider.CheckArvanCloudCdnAppInstalled(context.Background(), creds(), "example.com", "app-1")
	if err != nil {
		t.Fatalf("CheckArvanCloudCdnAppInstalled() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/apps/app-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/apps/app-1", records)
	}
	if !installed {
		t.Errorf("installed = false, want true")
	}
}

// TestCheckArvanCloudCdnAppInstalled_false proves is_install=false.
func TestCheckArvanCloudCdnAppInstalled_false(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"is_install":false}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	installed, err := provider.CheckArvanCloudCdnAppInstalled(context.Background(), creds(), "example.com", "app-missing")
	if err != nil {
		t.Fatalf("CheckArvanCloudCdnAppInstalled() error = %v", err)
	}
	if installed {
		t.Errorf("installed = true, want false")
	}
}

// TestInstallArvanCloudCdnApp pins POST /domains/{domain}/apps/{id} with
// an options body.
func TestInstallArvanCloudCdnApp(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"inst-1","domain_id":"dom-1","application_id":"app-1",
			"active":true,"options":{"tracking_id":"UA-12345"},
			"created_at":"2026-08-21T00:00:00Z","updated_at":"2026-08-21T00:00:00Z"}}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	options := map[string]any{"tracking_id": "UA-12345"}
	installed, err := provider.InstallArvanCloudCdnApp(context.Background(), creds(), "example.com", "app-1", options)
	if err != nil {
		t.Fatalf("InstallArvanCloudCdnApp() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/apps/app-1" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/apps/app-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["tracking_id"] != "UA-12345" {
		t.Errorf("request body = %+v, want tracking_id=UA-12345", body)
	}
	if installed.ID != "inst-1" || installed.ApplicationID != "app-1" || !installed.Active {
		t.Errorf("installed = %+v, want the parsed installation", installed)
	}
}

// TestInstallArvanCloudCdnAppEmptyOptions proves the install call works
// with nil options (the adapter sends an empty object).
func TestInstallArvanCloudCdnAppEmptyOptions(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"inst-2","domain_id":"dom-1","application_id":"app-2",
			"active":true,"created_at":"2026-08-21T00:00:00Z","updated_at":"2026-08-21T00:00:00Z"}}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	installed, err := provider.InstallArvanCloudCdnApp(context.Background(), creds(), "example.com", "app-2", nil)
	if err != nil {
		t.Fatalf("InstallArvanCloudCdnApp() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	// Empty object — the adapter converts nil to {}.
	if len(body) != 0 {
		t.Errorf("request body = %+v, want empty object", body)
	}
	if installed.ID != "inst-2" {
		t.Errorf("installed.ID = %q, want inst-2", installed.ID)
	}
}

// TestUninstallArvanCloudCdnApp pins DELETE /domains/{domain}/apps/{id}.
func TestUninstallArvanCloudCdnApp(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Deleted successfully"}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.UninstallArvanCloudCdnApp(context.Background(), creds(), "example.com", "app-1"); err != nil {
		t.Fatalf("UninstallArvanCloudCdnApp() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/apps/app-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/apps/app-1", records)
	}
}

// TestUninstallArvanCloudCdnAppNotFound proves a 404 surfaces as
// domain.ErrNotFound.
func TestUninstallArvanCloudCdnAppNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.UninstallArvanCloudCdnApp(context.Background(), creds(), "example.com", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("UninstallArvanCloudCdnApp() error = %v, want domain.ErrNotFound", err)
	}
}

// TestTriggerArvanCloudCdnAppWebhook pins POST
// /domains/{domain}/apps/{id}/actions/trigger_webhook.
func TestTriggerArvanCloudCdnAppWebhook(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Webhook triggered successfully"}`)}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	options := map[string]any{"key": "value"}
	err := provider.TriggerArvanCloudCdnAppWebhook(context.Background(), creds(), "example.com", "app-1",
		domain.ArvanCloudTriggerWebhookNewInstall, options)
	if err != nil {
		t.Fatalf("TriggerArvanCloudCdnAppWebhook() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost ||
		records[0].path != "/domains/example.com/apps/app-1/actions/trigger_webhook" {
		t.Fatalf("request = %+v, want a single POST .../trigger_webhook", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["event"] != "new-install" {
		t.Errorf("request body[\"event\"] = %v, want new-install", body["event"])
	}
	opts, ok := body["options"].(map[string]any)
	if !ok || opts["key"] != "value" {
		t.Errorf("request body[\"options\"] = %+v, want key=value", body["options"])
	}
}
