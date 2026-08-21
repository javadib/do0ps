package arvancloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// --- Caching -----------------------------------------------------------

// TestGetArvanCloudCachingSettings pins the request shape and response
// parsing of GET /domains/{domain}/caching.
func TestGetArvanCloudCachingSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{
			"cache_developer_mode":true,"cache_consistent_uptime":false,"cache_max_size":52428800,
			"cache_status":"uri","cache_page_200":"30m","cache_page_any":"0s","cache_browser":"default",
			"cache_scheme":true,"cache_ignore_sc":false,"cache_cookie":"session","cache_args":true,"cache_arg":"filter&sort"
		}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings, err := provider.GetArvanCloudCachingSettings(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudCachingSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/caching" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/caching", records)
	}
	if !settings.CacheDeveloperMode || settings.CacheMaxSizeBytes != 52428800 {
		t.Errorf("settings = %+v, want the parsed developer mode/max size", settings)
	}
	if settings.CacheStatus != domain.ArvanCloudPageRuleCacheLevelURI {
		t.Errorf("CacheStatus = %q, want %q", settings.CacheStatus, domain.ArvanCloudPageRuleCacheLevelURI)
	}
	if settings.CachePage200 != domain.ArvanCloudCacheTTL30m || settings.CacheBrowser != domain.ArvanCloudCacheTTLDefault {
		t.Errorf("settings = %+v, want the parsed TTLs", settings)
	}
	if settings.CacheArg != "filter&sort" {
		t.Errorf("CacheArg = %q, want %q", settings.CacheArg, "filter&sort")
	}
}

// TestUpdateArvanCloudCachingSettings pins the request body of PATCH
// /domains/{domain}/caching, including that boolean fields (even explicit
// false ones) are always sent, and the endpoint's message-only response is
// not treated as an error.
func TestUpdateArvanCloudCachingSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"OK"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings := domain.ArvanCloudCachingSettings{
		CacheDeveloperMode:    true,
		CacheConsistentUptime: false,
		CacheMaxSizeBytes:     10485760,
		CacheStatus:           domain.ArvanCloudPageRuleCacheLevelQueryString,
		CachePage200:          domain.ArvanCloudCacheTTL1h,
		CacheArgs:             false,
	}
	if err := provider.UpdateArvanCloudCachingSettings(context.Background(), creds(), "example.com", settings); err != nil {
		t.Fatalf("UpdateArvanCloudCachingSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/caching" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/caching", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["cache_developer_mode"] != true {
		t.Errorf("request body = %+v, want cache_developer_mode true", body)
	}
	if _, has := body["cache_consistent_uptime"]; !has || body["cache_consistent_uptime"] != false {
		t.Errorf("request body = %+v, want cache_consistent_uptime explicitly false, not omitted", body)
	}
	if _, has := body["cache_args"]; !has || body["cache_args"] != false {
		t.Errorf("request body = %+v, want cache_args explicitly false, not omitted", body)
	}
	if body["cache_status"] != "query_string" || body["cache_page_200"] != "1h" {
		t.Errorf("request body = %+v, want cache_status/cache_page_200 sent", body)
	}
	if _, has := body["cache_page_any"]; has {
		t.Errorf("request body = %+v, must omit cache_page_any when left unset", body)
	}
}

// TestPurgeArvanCloudCache pins the request body of POST
// /domains/{domain}/caching/purge.
func TestPurgeArvanCloudCache(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 201, func(*http.Request) []byte { return []byte(`{"message":"Successfully queued purge"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	purge := domain.ArvanCloudCachePurgeRequest{
		Mode: domain.ArvanCloudCachePurgeIndividual,
		URLs: []string{"https://example.com/a", "https://example.com/b"},
	}
	if err := provider.PurgeArvanCloudCache(context.Background(), creds(), "example.com", purge); err != nil {
		t.Fatalf("PurgeArvanCloudCache() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/caching/purge" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/caching/purge", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["purge"] != "individual" {
		t.Errorf(`request body["purge"] = %#v, want "individual"`, body["purge"])
	}
	urls, ok := body["purge_urls"].([]any)
	if !ok || len(urls) != 2 {
		t.Errorf("request body[\"purge_urls\"] = %+v, want the two URLs", body["purge_urls"])
	}
	if _, has := body["purge_tags"]; has {
		t.Errorf("request body = %+v, must omit purge_tags when unset", body)
	}
}

// TestPurgeArvanCloudCacheNeverCallsDeprecatedEndpoint is the issue #72
// acceptance-criteria test: no code path in this adapter calls the
// deprecated DELETE /domains/{domain}/caching endpoint
// (caching.deprecated_purge) — only POST /domains/{domain}/caching/purge
// (caching.purge) is wired. The test server fails the test outright if it
// ever sees a DELETE to the caching path.
func TestPurgeArvanCloudCacheNeverCallsDeprecatedEndpoint(t *testing.T) {
	var records []requestRecord
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/domains/example.com/caching" {
			t.Fatalf("adapter called the deprecated DELETE /domains/example.com/caching endpoint")
		}
		body, _ := io.ReadAll(r.Body)
		records = append(records, requestRecord{method: r.Method, path: r.URL.Path, body: body})
		_, _ = w.Write([]byte(`{"message":"Successfully queued purge"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	purge := domain.ArvanCloudCachePurgeRequest{Mode: domain.ArvanCloudCachePurgeAll}
	if err := provider.PurgeArvanCloudCache(context.Background(), creds(), "example.com", purge); err != nil {
		t.Fatalf("PurgeArvanCloudCache() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/caching/purge" {
		t.Fatalf("request = %+v, want exactly one POST /domains/example.com/caching/purge and nothing else", records)
	}
}

// TestListArvanCloudPurgeTags pins the request shape and the
// one-object-into-many-rows denormalization documented on
// domain.ArvanCloudPurgeTag.
func TestListArvanCloudPurgeTags(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"domain_id":"uuid-1","tags":["tag-a","tag-b"],"created_at":"2024-01-01T00:00:00Z"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	tags, err := provider.ListArvanCloudPurgeTags(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudPurgeTags() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/purge-tags" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/purge-tags", records)
	}
	if len(tags) != 2 || tags[0].Tag != "tag-a" || tags[1].Tag != "tag-b" {
		t.Fatalf("tags = %+v, want two parsed entries", tags)
	}
	if tags[0].CreatedAt != "2024-01-01T00:00:00Z" || tags[1].CreatedAt != tags[0].CreatedAt {
		t.Errorf("tags = %+v, want created_at shared across every entry", tags)
	}
}

// TestDeleteArvanCloudPurgeTag pins the request shape of DELETE
// /domains/{domain}/purge-tags, including the required "tag" query
// parameter.
func TestDeleteArvanCloudPurgeTag(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"Deleted"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudPurgeTag(context.Background(), creds(), "example.com", "tag-a"); err != nil {
		t.Fatalf("DeleteArvanCloudPurgeTag() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/purge-tags" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/purge-tags", records)
	}
	if records[0].query != "tag=tag-a" {
		t.Errorf("query = %q, want %q", records[0].query, "tag=tag-a")
	}
}

// --- Image Resize --------------------------------------------------------

// TestGetArvanCloudImageResizeSettings pins the request shape and response
// parsing of GET /domains/{domain}/image-resize.
func TestGetArvanCloudImageResizeSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"OK","data":{
			"status":"on","height_by":"h","width_by":"w","mode":"short-side","mode_by":"m","quality_by":"q"
		}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings, err := provider.GetArvanCloudImageResizeSettings(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudImageResizeSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/image-resize" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/image-resize", records)
	}
	if settings.Status != domain.ArvanCloudImageResizeStatusOn || settings.Mode != domain.ArvanCloudImageResizeModeShortSide {
		t.Errorf("settings = %+v, want the parsed status/mode", settings)
	}
	if settings.HeightBy != "h" || settings.QualityBy != "q" {
		t.Errorf("settings = %+v, want the parsed height_by/quality_by", settings)
	}
}

// TestUpdateArvanCloudImageResizeSettings pins the request body of PATCH
// /domains/{domain}/image-resize.
func TestUpdateArvanCloudImageResizeSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"OK","data":{"status":"off","height_by":"height","width_by":"width","mode":"freely"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings := domain.ArvanCloudImageResizeSettings{Status: domain.ArvanCloudImageResizeStatusOff, Mode: domain.ArvanCloudImageResizeModeFreely}
	updated, err := provider.UpdateArvanCloudImageResizeSettings(context.Background(), creds(), "example.com", settings)
	if err != nil {
		t.Fatalf("UpdateArvanCloudImageResizeSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/image-resize" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/image-resize", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["status"] != "off" || body["mode"] != "freely" {
		t.Errorf("request body = %+v, want status/mode sent", body)
	}
	if updated.Status != domain.ArvanCloudImageResizeStatusOff {
		t.Errorf("updated.Status = %q, want %q", updated.Status, domain.ArvanCloudImageResizeStatusOff)
	}
}

// --- Acceleration --------------------------------------------------------

// TestGetArvanCloudAccelerationSettings pins the request shape and response
// parsing of GET /domains/{domain}/acceleration, and that it reuses
// accelerationWire from rules.go (the parsed value round-trips through the
// same domain.ArvanCloudAccelerationSettings type AC11 already reused).
func TestGetArvanCloudAccelerationSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"OK","data":{"status":"on","extensions":["css","js"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings, err := provider.GetArvanCloudAccelerationSettings(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudAccelerationSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/acceleration" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/acceleration", records)
	}
	if settings.Status != domain.ArvanCloudAccelerationOn || len(settings.Extensions) != 2 {
		t.Errorf("settings = %+v, want the parsed status/extensions", settings)
	}
}

// TestUpdateArvanCloudAccelerationSettings pins the request body of PATCH
// /domains/{domain}/acceleration.
func TestUpdateArvanCloudAccelerationSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"OK","data":{"status":"off","extensions":[]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings := domain.ArvanCloudAccelerationSettings{Status: domain.ArvanCloudAccelerationOff}
	updated, err := provider.UpdateArvanCloudAccelerationSettings(context.Background(), creds(), "example.com", settings)
	if err != nil {
		t.Fatalf("UpdateArvanCloudAccelerationSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/acceleration" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/acceleration", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["status"] != "off" {
		t.Errorf(`request body["status"] = %#v, want "off"`, body["status"])
	}
	if updated.Status != domain.ArvanCloudAccelerationOff {
		t.Errorf("updated.Status = %q, want %q", updated.Status, domain.ArvanCloudAccelerationOff)
	}
}

// --- Custom Pages ----------------------------------------------------------

// TestListArvanCloudCustomPages pins the request shape and response parsing
// of GET /domains/{domain}/custom-pages, across all nine named slots.
func TestListArvanCloudCustomPages(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{
			"under_construction":{"status_code":200,"type":"off"},
			"firewall_error":{"status_code":403,"type":"url","url":"https://example.com/blocked"},
			"waf_protection":{"status_code":403,"type":"off"},
			"rate_limit_exceeded":{"status_code":481,"type":"off"},
			"secure_link_expired":{"status_code":482,"type":"off"},
			"secure_link_invalid":{"status_code":483,"type":"off"},
			"error_500":{"status_code":500,"type":"off"},
			"ddos_js":{"status_code":200,"type":"file","file":[{"id":"file-1","name":"page.html","active":true}]},
			"ddos_captcha":{"status_code":484,"type":"off"}
		}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	pages, err := provider.ListArvanCloudCustomPages(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudCustomPages() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/custom-pages" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/custom-pages", records)
	}
	if pages.FirewallError.Type != domain.ArvanCloudCustomPageTypeURL || pages.FirewallError.URL != "https://example.com/blocked" {
		t.Errorf("FirewallError = %+v, want the parsed url slot", pages.FirewallError)
	}
	if len(pages.DdosJS.Files) != 1 || pages.DdosJS.Files[0].ID != "file-1" || !pages.DdosJS.Files[0].Active {
		t.Errorf("DdosJS = %+v, want the parsed file entry", pages.DdosJS)
	}
	if pages.RateLimitExceeded.StatusCode != domain.ArvanCloudCustomPageStatus481 {
		t.Errorf("RateLimitExceeded.StatusCode = %v, want %v", pages.RateLimitExceeded.StatusCode, domain.ArvanCloudCustomPageStatus481)
	}
}

// TestUpdateArvanCloudCustomPageURLType pins the multipart request shape of
// POST /domains/{domain}/custom-pages for a "url"-type update: multipart/
// form-data (not JSON), with page/type/url as plain string fields and no
// file part.
func TestUpdateArvanCloudCustomPageURLType(t *testing.T) {
	var (
		method, path, contentType string
		fields                    map[string][]string
		hasFile                   bool
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		contentType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		fields = map[string][]string(r.MultipartForm.Value)
		hasFile = len(r.MultipartForm.File) > 0
		_, _ = w.Write([]byte(`{"message":"Successfully updated custom page"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	update := domain.ArvanCloudCustomPageUpdate{
		Page: domain.ArvanCloudCustomPageFirewallError,
		Type: domain.ArvanCloudCustomPageTypeURL,
		URL:  "https://example.com/blocked",
	}
	if err := provider.UpdateArvanCloudCustomPage(context.Background(), creds(), "example.com", update); err != nil {
		t.Fatalf("UpdateArvanCloudCustomPage() error = %v", err)
	}

	if method != http.MethodPost || path != "/domains/example.com/custom-pages" {
		t.Errorf("request = %s %s, want POST /domains/example.com/custom-pages", method, path)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data per the spec's CustomPageUpdate requestBody", contentType)
	}
	if got := fields["page"]; len(got) != 1 || got[0] != "firewall_error" {
		t.Errorf("page field = %v, want [firewall_error]", got)
	}
	if got := fields["type"]; len(got) != 1 || got[0] != "url" {
		t.Errorf("type field = %v, want [url]", got)
	}
	if got := fields["url"]; len(got) != 1 || got[0] != "https://example.com/blocked" {
		t.Errorf("url field = %v, want the URL", got)
	}
	if hasFile {
		t.Errorf("request carried a file part, want none for a url-type update")
	}
}

// TestUpdateArvanCloudCustomPageFileType pins the multipart request shape of
// POST /domains/{domain}/custom-pages for a "file"-type update: the file
// content must arrive as a binary form-file part under "file", per the
// spec's CustomPageUpdate.file (`type: string, format: binary`).
func TestUpdateArvanCloudCustomPageFileType(t *testing.T) {
	var (
		fields      map[string][]string
		fieldName   string
		fileContent []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		fields = map[string][]string(r.MultipartForm.Value)
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile(file): %v", err)
		}
		defer file.Close()
		fieldName = "file"
		fileContent, _ = io.ReadAll(file)
		_, _ = w.Write([]byte(`{"message":"Successfully updated custom page"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	update := domain.ArvanCloudCustomPageUpdate{
		Page:        domain.ArvanCloudCustomPageDdosJS,
		Type:        domain.ArvanCloudCustomPageTypeFile,
		FileName:    "blocked.html",
		FileContent: []byte("<html>blocked</html>"),
	}
	if err := provider.UpdateArvanCloudCustomPage(context.Background(), creds(), "example.com", update); err != nil {
		t.Fatalf("UpdateArvanCloudCustomPage() error = %v", err)
	}

	if got := fields["page"]; len(got) != 1 || got[0] != "ddos_js" {
		t.Errorf("page field = %v, want [ddos_js]", got)
	}
	if fieldName != "file" {
		t.Errorf("file field name = %q, want %q", fieldName, "file")
	}
	if string(fileContent) != "<html>blocked</html>" {
		t.Errorf("file content = %q, want the uploaded HTML", fileContent)
	}
}

// TestGetArvanCloudCustomPageFile pins the request shape and response
// parsing of GET /domains/{domain}/custom-pages/{customPageFile}, including
// the "value" HTML-content field CustomPageFileData adds on top of the base
// CustomPageFile shape.
func TestGetArvanCloudCustomPageFile(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"file-1","name":"page.html","active":true,"value":"<html>content</html>"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudCustomPageFile(context.Background(), creds(), "example.com", "file-1")
	if err != nil {
		t.Fatalf("GetArvanCloudCustomPageFile() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/custom-pages/file-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/custom-pages/file-1", records)
	}
	if found.ID != "file-1" || !found.Active || found.Value != "<html>content</html>" {
		t.Errorf("found = %+v, want the parsed file, including value", found)
	}
}

// TestUpdateArvanCloudCustomPageFile pins the multipart request shape of
// POST /domains/{domain}/custom-pages/{customPageFile}: the "active" flag
// travels as a plain string form field ("true"/"false"), not a JSON
// boolean, since the whole request is multipart/form-data.
func TestUpdateArvanCloudCustomPageFile(t *testing.T) {
	var (
		method, path string
		fields       map[string][]string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		fields = map[string][]string(r.MultipartForm.Value)
		_, _ = w.Write([]byte(`{"message":"Successfully updated custom page file"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	active := true
	if err := provider.UpdateArvanCloudCustomPageFile(context.Background(), creds(), "example.com", "file-1", &active, "", nil); err != nil {
		t.Fatalf("UpdateArvanCloudCustomPageFile() error = %v", err)
	}

	if method != http.MethodPost || path != "/domains/example.com/custom-pages/file-1" {
		t.Errorf("request = %s %s, want POST /domains/example.com/custom-pages/file-1", method, path)
	}
	if got := fields["active"]; len(got) != 1 || got[0] != "true" {
		t.Errorf("active field = %v, want [true]", got)
	}
}

// TestDeleteArvanCloudCustomPageFile pins the request shape of DELETE
// /domains/{domain}/custom-pages/{customPageFile}.
func TestDeleteArvanCloudCustomPageFile(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"Successfully deleted custom page file"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudCustomPageFile(context.Background(), creds(), "example.com", "file-1"); err != nil {
		t.Fatalf("DeleteArvanCloudCustomPageFile() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/custom-pages/file-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/custom-pages/file-1", records)
	}
}

// TestDeleteArvanCloudCustomPageFileCannotDeleteActive proves the provider's
// 400 "Cannot delete active file" response is propagated as-is rather than
// pre-checked or swallowed client-side.
func TestDeleteArvanCloudCustomPageFileCannotDeleteActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Cannot delete active file"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudCustomPageFile(context.Background(), creds(), "example.com", "file-1")
	if err == nil {
		t.Fatal("DeleteArvanCloudCustomPageFile() error = nil, want the provider's 400 to surface")
	}
	var provErr *domain.ProviderError
	if !errors.As(err, &provErr) || provErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("error = %v, want a *domain.ProviderError with StatusCode 400", err)
	}
}
