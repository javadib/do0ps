package arvancloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// --- Per-domain DDoS settings ------------------------------------------------

// TestGetArvanCloudDdosSettings pins the request shape and response parsing
// of GET /domains/{domain}/ddos/settings.
func TestGetArvanCloudDdosSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{
			"is_enabled":true,"protection_mode":"javascript","ttl":3600,"https_only":true,
			"preflight":{"access_origin":"*","access_methods":["GET","POST"]}
		}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings, err := provider.GetArvanCloudDdosSettings(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudDdosSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/ddos/settings" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/ddos/settings", records)
	}
	if !settings.IsEnabled || settings.ProtectionMode != domain.ArvanCloudDdosProtectionModeJavaScript || settings.TTL != 3600 {
		t.Errorf("settings = %+v, want the parsed settings", settings)
	}
	if len(settings.Preflight.AccessMethods) != 2 || settings.Preflight.AccessOrigin != "*" {
		t.Errorf("preflight = %+v, want the parsed preflight", settings.Preflight)
	}
}

// TestUpdateArvanCloudDdosSettings pins the request body of PATCH
// /domains/{domain}/ddos/settings, including that protection_mode is sent as
// the plain JSON string and captcha fields are included when given.
func TestUpdateArvanCloudDdosSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"is_enabled":true,"protection_mode":"captcha","captcha_service":"recaptcha",
			"site_key":"site-123","secret_key":"secret-abc","ttl":600,"https_only":false}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings := domain.ArvanCloudDdosSettings{
		ProtectionMode: domain.ArvanCloudDdosProtectionModeCaptcha,
		CaptchaService: domain.ArvanCloudCaptchaServiceRecaptcha,
		SiteKey:        "site-123",
		SecretKey:      "secret-abc",
		TTL:            600,
		HTTPSOnly:      false,
	}
	updated, err := provider.UpdateArvanCloudDdosSettings(context.Background(), creds(), "example.com", settings)
	if err != nil {
		t.Fatalf("UpdateArvanCloudDdosSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/ddos/settings" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/ddos/settings", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["protection_mode"] != "captcha" {
		t.Errorf(`request body["protection_mode"] = %#v, want the string "captcha"`, body["protection_mode"])
	}
	if body["captcha_service"] != "recaptcha" || body["site_key"] != "site-123" || body["secret_key"] != "secret-abc" {
		t.Errorf("request body = %+v, want captcha_service/site_key/secret_key sent", body)
	}
	if _, hasIsEnabled := body["is_enabled"]; hasIsEnabled {
		t.Errorf("request body = %+v, must omit is_enabled (readOnly)", body)
	}
	if updated.CaptchaService != domain.ArvanCloudCaptchaServiceRecaptcha || updated.SecretKey != "secret-abc" {
		t.Errorf("updated = %+v, want the parsed settings echoed back", updated)
	}
}

// TestUpdateArvanCloudDdosSettingsOmitsCaptchaFieldsWhenNotCaptcha proves a
// non-captcha protection_mode does not send captcha_service/site_key/
// secret_key at all, even if the caller left stale values set on the input
// struct.
func TestUpdateArvanCloudDdosSettingsOmitsCaptchaFieldsWhenNotCaptcha(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"is_enabled":true,"protection_mode":"cookie","ttl":3600,"https_only":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings := domain.ArvanCloudDdosSettings{
		ProtectionMode: domain.ArvanCloudDdosProtectionModeCookie,
		CaptchaService: domain.ArvanCloudCaptchaServiceRecaptcha, // stale, should not be sent
		SecretKey:      "leftover-secret",                        // stale, should not be sent
		TTL:            3600,
		HTTPSOnly:      true,
	}
	if _, err := provider.UpdateArvanCloudDdosSettings(context.Background(), creds(), "example.com", settings); err != nil {
		t.Fatalf("UpdateArvanCloudDdosSettings() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if _, has := body["captcha_service"]; has {
		t.Errorf("request body = %+v, must omit captcha_service when protection_mode is not captcha", body)
	}
	if _, has := body["secret_key"]; has {
		t.Errorf("request body = %+v, must omit secret_key when protection_mode is not captcha", body)
	}
}

// TestUpdateArvanCloudDdosSettingsNeverLogsSecretKey guards the debug log and
// any resulting error message: secret_key must never appear verbatim,
// mirroring how TestRedactedHeadersHideTheKey guards the API key
// (client_test.go) and matching this file's own package comment.
func TestUpdateArvanCloudDdosSettingsNeverLogsSecretKey(t *testing.T) {
	const captchaSecret = "super-secret-recaptcha-key-should-never-leak"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The provider echoes the settings back, secret_key included — the
		// realistic shape of ddos.settings.update's response.
		_, _ = w.Write([]byte(`{"data":{"is_enabled":true,"protection_mode":"captcha","captcha_service":"recaptcha",
			"site_key":"site-123","secret_key":"` + captchaSecret + `","ttl":600,"https_only":false}}`))
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := newTestClient(t, srv, WithLogger(logger))
	provider, err := NewProvider(client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	settings := domain.ArvanCloudDdosSettings{
		ProtectionMode: domain.ArvanCloudDdosProtectionModeCaptcha,
		CaptchaService: domain.ArvanCloudCaptchaServiceRecaptcha,
		SiteKey:        "site-123",
		SecretKey:      captchaSecret,
		TTL:            600,
	}
	updated, err := provider.UpdateArvanCloudDdosSettings(context.Background(), creds(), "example.com", settings)
	if err != nil {
		t.Fatalf("UpdateArvanCloudDdosSettings() error = %v", err)
	}
	// The tool response reaching the same caller who supplied the secret is
	// fine (see arvancloud_ddos_tools.go's arvanCloudDdosSettingsToMap
	// comment) — this test is only about the log line.
	if updated.SecretKey != captchaSecret {
		t.Errorf("updated.SecretKey = %q, want it decoded from the response", updated.SecretKey)
	}

	if strings.Contains(logBuf.String(), captchaSecret) {
		t.Errorf("debug log contains the CAPTCHA secret_key verbatim:\n%s", logBuf.String())
	}
}

// TestUpdateArvanCloudDdosSettingsErrorNeverContainsSecretKey proves a failed
// update's error message does not contain secret_key either, even when the
// provider's error response happens to echo the submitted settings back.
func TestUpdateArvanCloudDdosSettingsErrorNeverContainsSecretKey(t *testing.T) {
	const captchaSecret = "super-secret-recaptcha-key-should-never-leak"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		// An error response shape this adapter does not recognize (no
		// "message"/"errors" fields), so mapErrorResponse falls back to the
		// raw body excerpt — the path TestUpdateArvanCloudDdosSettingsNeverLogsSecretKey
		// does not exercise.
		_, _ = w.Write([]byte(`{"submitted":{"protection_mode":"captcha","secret_key":"` + captchaSecret + `"}}`))
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := newTestClient(t, srv, WithLogger(logger))
	provider, err := NewProvider(client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	settings := domain.ArvanCloudDdosSettings{
		ProtectionMode: domain.ArvanCloudDdosProtectionModeCaptcha,
		CaptchaService: domain.ArvanCloudCaptchaServiceRecaptcha,
		SecretKey:      captchaSecret,
	}
	_, err = provider.UpdateArvanCloudDdosSettings(context.Background(), creds(), "example.com", settings)
	if err == nil {
		t.Fatal("UpdateArvanCloudDdosSettings() error = nil, want the 422 to surface")
	}

	if strings.Contains(err.Error(), captchaSecret) {
		t.Errorf("error message contains the CAPTCHA secret_key verbatim: %v", err)
	}
	if strings.Contains(logBuf.String(), captchaSecret) {
		t.Errorf("debug log contains the CAPTCHA secret_key verbatim:\n%s", logBuf.String())
	}
}

// --- Per-domain DDoS rules ----------------------------------------------------

// TestListArvanCloudDdosRules pins the request shape and response parsing of
// GET /domains/{domain}/ddos/rules.
func TestListArvanCloudDdosRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"rule-1","url_pattern":"/wp-admin/**","sources":["203.0.113.0/24"],"action":"passthrough","description":"trust office","is_enabled":true},
			{"id":"rule-2","url_pattern":"/api/**","sources":[],"action":"protect","is_enabled":true}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rules, err := provider.ListArvanCloudDdosRules(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudDdosRules() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/ddos/rules" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/ddos/rules", records)
	}
	if len(rules) != 2 || rules[0].ID != "rule-1" || rules[0].Action != domain.ArvanCloudDdosRuleActionPassthrough {
		t.Errorf("rules = %+v, want the two parsed entries", rules)
	}
}

// TestCreateArvanCloudDdosRule pins the request body of POST
// /domains/{domain}/ddos/rules.
func TestCreateArvanCloudDdosRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","url_pattern":"/wp-admin/**","sources":["203.0.113.0/24"],"action":"protect","is_enabled":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rule := domain.ArvanCloudDdosRule{
		URLPattern: "/wp-admin/**",
		Sources:    []string{"203.0.113.0/24"},
		Action:     domain.ArvanCloudDdosRuleActionProtect,
		IsEnabled:  true,
	}
	created, err := provider.CreateArvanCloudDdosRule(context.Background(), creds(), "example.com", rule)
	if err != nil {
		t.Fatalf("CreateArvanCloudDdosRule() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/ddos/rules" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/ddos/rules", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["url_pattern"] != "/wp-admin/**" || body["action"] != "protect" {
		t.Errorf("request body = %+v, want url_pattern/action sent", body)
	}
	if created.ID != "rule-1" {
		t.Errorf("created.ID = %q, want %q", created.ID, "rule-1")
	}
}

// TestGetArvanCloudDdosRule pins the request shape and response parsing of
// GET /domains/{domain}/ddos/rules/{id}.
func TestGetArvanCloudDdosRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","url_pattern":"/wp-admin/**","sources":[],"action":"protect","is_enabled":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rule, err := provider.GetArvanCloudDdosRule(context.Background(), creds(), "example.com", "rule-1")
	if err != nil {
		t.Fatalf("GetArvanCloudDdosRule() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/ddos/rules/rule-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/ddos/rules/rule-1", records)
	}
	if rule.ID != "rule-1" || rule.Action != domain.ArvanCloudDdosRuleActionProtect {
		t.Errorf("rule = %+v, want the parsed rule", rule)
	}
}

// TestUpdateArvanCloudDdosRule pins the request body of PATCH
// /domains/{domain}/ddos/rules/{id}.
func TestUpdateArvanCloudDdosRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","url_pattern":"/api/**","sources":[],"action":"passthrough","is_enabled":false}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rule := domain.ArvanCloudDdosRule{
		URLPattern: "/api/**",
		Action:     domain.ArvanCloudDdosRuleActionPassthrough,
		IsEnabled:  false,
	}
	updated, err := provider.UpdateArvanCloudDdosRule(context.Background(), creds(), "example.com", "rule-1", rule)
	if err != nil {
		t.Fatalf("UpdateArvanCloudDdosRule() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/ddos/rules/rule-1" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/ddos/rules/rule-1", records)
	}
	if updated.Action != domain.ArvanCloudDdosRuleActionPassthrough {
		t.Errorf("updated.Action = %q, want %q", updated.Action, domain.ArvanCloudDdosRuleActionPassthrough)
	}
}

// TestDeleteArvanCloudDdosRule pins the request shape of DELETE
// /domains/{domain}/ddos/rules/{id}.
func TestDeleteArvanCloudDdosRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Deleted successfully"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudDdosRule(context.Background(), creds(), "example.com", "rule-1"); err != nil {
		t.Fatalf("DeleteArvanCloudDdosRule() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/ddos/rules/rule-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/ddos/rules/rule-1", records)
	}
}

// TestDeleteArvanCloudDdosRuleNotFound proves a 404 surfaces as
// domain.ErrNotFound, consistent with the tolerant-delete contract at the
// use-case layer (app.DeleteArvanCloudDdosRule).
func TestDeleteArvanCloudDdosRuleNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudDdosRule(context.Background(), creds(), "example.com", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteArvanCloudDdosRule() error = %v, want domain.ErrNotFound", err)
	}
}

// TestReprioritizeArvanCloudDdosRules pins the request body of POST
// /domains/{domain}/ddos/actions/reprioritize.
func TestReprioritizeArvanCloudDdosRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"OK"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.ReprioritizeArvanCloudDdosRules(context.Background(), creds(), "example.com", "rule-1", "rule-2", ""); err != nil {
		t.Fatalf("ReprioritizeArvanCloudDdosRules() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/ddos/actions/reprioritize" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/ddos/actions/reprioritize", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["rule_id"] != "rule-1" || body["after_rule_id"] != "rule-2" {
		t.Errorf("request body = %+v, want rule_id/after_rule_id set", body)
	}
	if _, hasBefore := body["before_rule_id"]; hasBefore {
		t.Errorf("request body = %+v, must omit before_rule_id when unset", body)
	}
}
