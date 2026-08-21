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

// --- Per-domain rate-limit settings -----------------------------------------

// TestGetArvanCloudRateLimitSettings pins the request shape and response
// parsing of GET /domains/{domain}/rate-limit/settings.
func TestGetArvanCloudRateLimitSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"ddos_detection":true,"exclude_sources":["203.0.113.0/24"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings, err := provider.GetArvanCloudRateLimitSettings(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudRateLimitSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/rate-limit/settings" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/rate-limit/settings", records)
	}
	if !settings.DDoSDetection || len(settings.ExcludeSources) != 1 || settings.ExcludeSources[0] != "203.0.113.0/24" {
		t.Errorf("settings = %+v, want the parsed settings", settings)
	}
}

// TestUpdateArvanCloudRateLimitSettings pins the request body of PATCH
// /domains/{domain}/rate-limit/settings, including that ddos_detection is
// sent explicitly even when false.
func TestUpdateArvanCloudRateLimitSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"ddos_detection":false,"exclude_sources":["198.51.100.0/24"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings := domain.ArvanCloudRateLimitSettings{
		DDoSDetection:  false,
		ExcludeSources: []string{"198.51.100.0/24"},
	}
	updated, err := provider.UpdateArvanCloudRateLimitSettings(context.Background(), creds(), "example.com", settings)
	if err != nil {
		t.Fatalf("UpdateArvanCloudRateLimitSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/rate-limit/settings" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/rate-limit/settings", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	ddosDetection, hasDdosDetection := body["ddos_detection"]
	if !hasDdosDetection || ddosDetection != false {
		t.Errorf(`request body["ddos_detection"] = %#v, hasKey=%v, want an explicit false`, ddosDetection, hasDdosDetection)
	}
	if updated.ExcludeSources[0] != "198.51.100.0/24" {
		t.Errorf("updated = %+v, want the parsed settings echoed back", updated)
	}
}

// --- Per-domain rate-limit rules ---------------------------------------------

// TestListArvanCloudRateLimitRules pins the request shape and response
// parsing of GET /domains/{domain}/rate-limit/rules.
func TestListArvanCloudRateLimitRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"rule-1","url_pattern":"/api/**","action":"block","is_enabled":true,"rate":100,"time_duration":60},
			{"id":"rule-2","url_pattern":"/login","action":"challenge","is_enabled":true,"rate":5,"time_duration":60,
				"action_details":{"mode":3,"ttl":600,"https_only":true}}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rules, err := provider.ListArvanCloudRateLimitRules(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudRateLimitRules() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/rate-limit/rules" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/rate-limit/rules", records)
	}
	if len(rules) != 2 || rules[0].ID != "rule-1" || rules[0].Action != domain.ArvanCloudRateLimitActionBlock {
		t.Errorf("rules = %+v, want the two parsed entries", rules)
	}
	if rules[1].ActionDetails.Mode != domain.ArvanCloudChallengeModeCaptcha || rules[1].ActionDetails.TTL != 600 {
		t.Errorf("rules[1].ActionDetails = %+v, want the parsed challenge action", rules[1].ActionDetails)
	}
}

// TestCreateArvanCloudRateLimitRule pins the request body of POST
// /domains/{domain}/rate-limit/rules, including that action_details is sent
// only when action is "challenge".
func TestCreateArvanCloudRateLimitRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","url_pattern":"/login","action":"challenge","is_enabled":true,
			"rate":5,"time_duration":60,"action_details":{"mode":3,"ttl":600,"https_only":true}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rule := domain.ArvanCloudRateLimitRule{
		URLPattern:   "/login",
		Action:       domain.ArvanCloudRateLimitActionChallenge,
		IsEnabled:    true,
		Rate:         5,
		TimeDuration: 60,
		ActionDetails: domain.ArvanCloudChallengeAction{
			Mode: domain.ArvanCloudChallengeModeCaptcha, TTL: 600, HTTPSOnly: true,
		},
	}
	created, err := provider.CreateArvanCloudRateLimitRule(context.Background(), creds(), "example.com", rule)
	if err != nil {
		t.Fatalf("CreateArvanCloudRateLimitRule() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/rate-limit/rules" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/rate-limit/rules", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["url_pattern"] != "/login" || body["action"] != "challenge" || body["rate"] != float64(5) {
		t.Errorf("request body = %+v, want url_pattern/action/rate sent", body)
	}
	details, ok := body["action_details"].(map[string]any)
	if !ok || details["mode"] != float64(3) {
		t.Errorf("request body[action_details] = %+v, want the challenge action sent", body["action_details"])
	}
	if created.ID != "rule-1" {
		t.Errorf("created.ID = %q, want %q", created.ID, "rule-1")
	}
}

// TestCreateArvanCloudRateLimitRuleOmitsActionDetailsWhenBlock proves a
// "block" action does not send action_details at all, even if the caller
// left a stale value set on the input struct.
func TestCreateArvanCloudRateLimitRuleOmitsActionDetailsWhenBlock(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","url_pattern":"/api/**","action":"block","is_enabled":true,"rate":100,"time_duration":60}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rule := domain.ArvanCloudRateLimitRule{
		URLPattern:   "/api/**",
		Action:       domain.ArvanCloudRateLimitActionBlock,
		Rate:         100,
		TimeDuration: 60,
		ActionDetails: domain.ArvanCloudChallengeAction{ // stale, should not be sent
			Mode: domain.ArvanCloudChallengeModeCaptcha, TTL: 600,
		},
	}
	if _, err := provider.CreateArvanCloudRateLimitRule(context.Background(), creds(), "example.com", rule); err != nil {
		t.Fatalf("CreateArvanCloudRateLimitRule() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if _, has := body["action_details"]; has {
		t.Errorf("request body = %+v, must omit action_details when action is \"block\"", body)
	}
}

// TestGetArvanCloudRateLimitRule pins the request shape and response parsing
// of GET /domains/{domain}/rate-limit/rules/{id}.
func TestGetArvanCloudRateLimitRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","url_pattern":"/api/**","action":"block","is_enabled":true,"rate":100,"time_duration":60}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rule, err := provider.GetArvanCloudRateLimitRule(context.Background(), creds(), "example.com", "rule-1")
	if err != nil {
		t.Fatalf("GetArvanCloudRateLimitRule() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/rate-limit/rules/rule-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/rate-limit/rules/rule-1", records)
	}
	if rule.ID != "rule-1" || rule.Rate != 100 || rule.TimeDuration != 60 {
		t.Errorf("rule = %+v, want the parsed rule", rule)
	}
}

// TestUpdateArvanCloudRateLimitRule pins the request body of PATCH
// /domains/{domain}/rate-limit/rules/{id}.
func TestUpdateArvanCloudRateLimitRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","url_pattern":"/api/**","action":"block","is_enabled":false,"rate":200,"time_duration":30}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rule := domain.ArvanCloudRateLimitRule{
		URLPattern:   "/api/**",
		Action:       domain.ArvanCloudRateLimitActionBlock,
		IsEnabled:    false,
		Rate:         200,
		TimeDuration: 30,
	}
	updated, err := provider.UpdateArvanCloudRateLimitRule(context.Background(), creds(), "example.com", "rule-1", rule)
	if err != nil {
		t.Fatalf("UpdateArvanCloudRateLimitRule() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/rate-limit/rules/rule-1" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/rate-limit/rules/rule-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	isEnabled, hasIsEnabled := body["is_enabled"]
	if !hasIsEnabled || isEnabled != false {
		t.Errorf(`request body["is_enabled"] = %#v, hasKey=%v, want an explicit false`, isEnabled, hasIsEnabled)
	}
	if updated.Rate != 200 {
		t.Errorf("updated.Rate = %d, want %d", updated.Rate, 200)
	}
}

// TestDeleteArvanCloudRateLimitRule pins the request shape of DELETE
// /domains/{domain}/rate-limit/rules/{id}.
func TestDeleteArvanCloudRateLimitRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Deleted successfully"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudRateLimitRule(context.Background(), creds(), "example.com", "rule-1"); err != nil {
		t.Fatalf("DeleteArvanCloudRateLimitRule() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/rate-limit/rules/rule-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/rate-limit/rules/rule-1", records)
	}
}

// TestDeleteArvanCloudRateLimitRuleNotFound proves a 404 surfaces as
// domain.ErrNotFound, consistent with the tolerant-delete contract at the
// use-case layer (app.DeleteArvanCloudRateLimitRule).
func TestDeleteArvanCloudRateLimitRuleNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudRateLimitRule(context.Background(), creds(), "example.com", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteArvanCloudRateLimitRule() error = %v, want domain.ErrNotFound", err)
	}
}

// TestReprioritizeArvanCloudRateLimitRules pins the request body of POST
// /domains/{domain}/rate-limit/actions/reprioritize.
func TestReprioritizeArvanCloudRateLimitRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"OK"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.ReprioritizeArvanCloudRateLimitRules(context.Background(), creds(), "example.com", "rule-1", "", "rule-2"); err != nil {
		t.Fatalf("ReprioritizeArvanCloudRateLimitRules() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/rate-limit/actions/reprioritize" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/rate-limit/actions/reprioritize", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["rule_id"] != "rule-1" || body["before_rule_id"] != "rule-2" {
		t.Errorf("request body = %+v, want rule_id/before_rule_id set", body)
	}
	if _, hasAfter := body["after_rule_id"]; hasAfter {
		t.Errorf("request body = %+v, must omit after_rule_id when unset", body)
	}
}
