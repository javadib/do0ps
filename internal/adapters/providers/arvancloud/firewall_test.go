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

// --- Firewall Settings (domain-level) --------------------------------------

// TestGetArvanCloudFirewallSettings pins the request shape and response
// parsing of GET /domains/{domain}/firewall/settings.
func TestGetArvanCloudFirewallSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"is_enabled":true,"default_action":"deny","verify_sni":true,
			"skip_global_whitelist":false,"skip_global_firewall":false,"default_action_details":null}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings, err := provider.GetArvanCloudFirewallSettings(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudFirewallSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/firewall/settings" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/firewall/settings", records)
	}
	if !settings.IsEnabled || settings.DefaultAction != domain.ArvanCloudFirewallDefaultActionDeny || !settings.VerifySNI {
		t.Errorf("settings = %+v, want the parsed response", settings)
	}
}

// TestUpdateArvanCloudFirewallSettings pins the request body of PATCH
// /domains/{domain}/firewall/settings, including that a "challenge"
// default_action sends only the challenge-shaped action_details.
func TestUpdateArvanCloudFirewallSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"is_enabled":true,"default_action":"challenge","verify_sni":true,
			"skip_global_whitelist":false,"skip_global_firewall":false,
			"default_action_details":{"mode":2,"ttl":3600,"https_only":true}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.UpdateArvanCloudFirewallSettings(context.Background(), creds(), "example.com", domain.ArvanCloudFirewallSettings{
		DefaultAction: domain.ArvanCloudFirewallDefaultActionChallenge,
		VerifySNI:     true,
		DefaultActionDetails: domain.ArvanCloudFirewallActionDetails{
			ChallengeMode: 2, ChallengeTTL: 3600, ChallengeHTTPSOnly: true,
		},
	})
	if err != nil {
		t.Fatalf("UpdateArvanCloudFirewallSettings() error = %v", err)
	}
	if updated.DefaultAction != domain.ArvanCloudFirewallDefaultActionChallenge || updated.DefaultActionDetails.ChallengeMode != 2 {
		t.Errorf("updated = %+v, want the parsed response", updated)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/firewall/settings" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/firewall/settings", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["default_action"] != "challenge" {
		t.Errorf(`request body["default_action"] = %v, want "challenge"`, body["default_action"])
	}
	details, ok := body["default_action_details"].(map[string]any)
	if !ok {
		t.Fatalf("request body[\"default_action_details\"] = %#v, want an object", body["default_action_details"])
	}
	if details["mode"] != float64(2) || details["ttl"] != float64(3600) || details["https_only"] != true {
		t.Errorf("request body default_action_details = %+v, want the challenge fields only", details)
	}
	if _, hasRlimit := details["rlimit"]; hasRlimit {
		t.Errorf("request body default_action_details = %+v, must not carry bypass-only fields for a challenge action", details)
	}
}

// --- Firewall Rules (domain-level) -----------------------------------------

// TestListArvanCloudFirewallRules pins the request shape and response
// parsing of GET /domains/{domain}/firewall/rules.
func TestListArvanCloudFirewallRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"rule-1","name":"block-ir","filter_expr":"ip.geoip.country in {\"IR\"}","action":"deny","priority":1,"is_enabled":true},
			{"id":"rule-2","name":"allow-office","filter_expr":"ip.src eq 203.0.113.1","action":"allow","priority":2,"is_enabled":false}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rules, err := provider.ListArvanCloudFirewallRules(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudFirewallRules() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/firewall/rules" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/firewall/rules", records)
	}
	if len(rules) != 2 || rules[0].ID != "rule-1" || rules[0].Action != domain.ArvanCloudFirewallActionDeny {
		t.Errorf("rules = %+v, want the two parsed entries", rules)
	}
}

// TestCreateArvanCloudFirewallRule pins the request body of POST
// /domains/{domain}/firewall/rules, including that a "bypass" action sends
// only the bypass-shaped action_details.
func TestCreateArvanCloudFirewallRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusCreated, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","name":"trust-office","filter_expr":"ip.src eq 203.0.113.1","action":"bypass",
			"priority":1,"is_enabled":true,"action_details":{"rlimit":true,"challenge":false,"waf":true}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	created, err := provider.CreateArvanCloudFirewallRule(context.Background(), creds(), "example.com", domain.ArvanCloudFirewallRule{
		Name:       "trust-office",
		FilterExpr: "ip.src eq 203.0.113.1",
		Action:     domain.ArvanCloudFirewallActionBypass,
		IsEnabled:  true,
		ActionDetails: domain.ArvanCloudFirewallActionDetails{
			BypassRateLimit: true, BypassWAF: true,
		},
	})
	if err != nil {
		t.Fatalf("CreateArvanCloudFirewallRule() error = %v", err)
	}
	if created.ID != "rule-1" || !created.ActionDetails.BypassRateLimit || !created.ActionDetails.BypassWAF {
		t.Errorf("created = %+v, want the parsed response", created)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/firewall/rules" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/firewall/rules", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["name"] != "trust-office" || body["action"] != "bypass" {
		t.Errorf("request body = %+v, want name=trust-office action=bypass", body)
	}
	details, ok := body["action_details"].(map[string]any)
	if !ok {
		t.Fatalf("request body[\"action_details\"] = %#v, want an object", body["action_details"])
	}
	if details["rlimit"] != true || details["waf"] != true {
		t.Errorf("request body action_details = %+v, want rlimit=true waf=true", details)
	}
	if _, hasMode := details["mode"]; hasMode {
		t.Errorf("request body action_details = %+v, must not carry challenge-only fields for a bypass action", details)
	}
}

// TestCreateArvanCloudFirewallRuleAllowOmitsActionDetails proves an "allow"
// rule sends no action_details field at all — it is meaningless for that
// action.
func TestCreateArvanCloudFirewallRuleAllowOmitsActionDetails(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusCreated, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","name":"allow-all","filter_expr":"true","action":"allow","is_enabled":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if _, err := provider.CreateArvanCloudFirewallRule(context.Background(), creds(), "example.com", domain.ArvanCloudFirewallRule{
		Name: "allow-all", FilterExpr: "true", Action: domain.ArvanCloudFirewallActionAllow, IsEnabled: true,
	}); err != nil {
		t.Fatalf("CreateArvanCloudFirewallRule() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if _, has := body["action_details"]; has {
		t.Errorf(`request body = %+v, must not carry "action_details" for an "allow" rule`, body)
	}
}

// TestGetArvanCloudFirewallRule pins the request shape of GET
// /domains/{domain}/firewall/rules/{id}.
func TestGetArvanCloudFirewallRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","name":"block-ir","filter_expr":"ip.geoip.country in {\"IR\"}","action":"deny","priority":1,"is_enabled":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudFirewallRule(context.Background(), creds(), "example.com", "rule-1")
	if err != nil {
		t.Fatalf("GetArvanCloudFirewallRule() error = %v", err)
	}
	if len(records) != 1 || records[0].path != "/domains/example.com/firewall/rules/rule-1" {
		t.Fatalf("request = %+v, want GET /domains/example.com/firewall/rules/rule-1", records)
	}
	if found.ID != "rule-1" || found.Priority != 1 {
		t.Errorf("found = %+v, want the parsed response", found)
	}
}

// TestUpdateArvanCloudFirewallRule pins the request shape of PATCH
// /domains/{domain}/firewall/rules/{id}.
func TestUpdateArvanCloudFirewallRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","name":"block-ir-th","filter_expr":"ip.geoip.country in {\"IR\" \"TH\"}","action":"deny","priority":1,"is_enabled":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.UpdateArvanCloudFirewallRule(context.Background(), creds(), "example.com", "rule-1", domain.ArvanCloudFirewallRule{
		Name: "block-ir-th", FilterExpr: `ip.geoip.country in {"IR" "TH"}`, Action: domain.ArvanCloudFirewallActionDeny, IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("UpdateArvanCloudFirewallRule() error = %v", err)
	}
	if updated.Name != "block-ir-th" {
		t.Errorf("updated.Name = %q, want %q", updated.Name, "block-ir-th")
	}
	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/firewall/rules/rule-1" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/firewall/rules/rule-1", records)
	}
}

// TestDeleteArvanCloudFirewallRule pins the request shape of DELETE
// /domains/{domain}/firewall/rules/{id}.
func TestDeleteArvanCloudFirewallRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Deleted successfully"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudFirewallRule(context.Background(), creds(), "example.com", "rule-1"); err != nil {
		t.Fatalf("DeleteArvanCloudFirewallRule() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/firewall/rules/rule-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/firewall/rules/rule-1", records)
	}
}

// TestDeleteArvanCloudFirewallRuleNotFound proves a 404 surfaces as
// domain.ErrNotFound, consistent with the tolerant-delete contract at the
// use-case layer.
func TestDeleteArvanCloudFirewallRuleNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudFirewallRule(context.Background(), creds(), "example.com", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteArvanCloudFirewallRule() error = %v, want domain.ErrNotFound", err)
	}
}

// TestReprioritizeArvanCloudFirewallRules pins the request body of POST
// /domains/{domain}/firewall/actions/reprioritize.
func TestReprioritizeArvanCloudFirewallRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Successfully changed the priority of the rule."}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.ReprioritizeArvanCloudFirewallRules(context.Background(), creds(), "example.com", "rule-1", "rule-2", ""); err != nil {
		t.Fatalf("ReprioritizeArvanCloudFirewallRules() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/firewall/actions/reprioritize" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/firewall/actions/reprioritize", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["rule_id"] != "rule-1" || body["after_rule_id"] != "rule-2" {
		t.Errorf("request body = %+v, want rule_id=rule-1 after_rule_id=rule-2", body)
	}
	if _, has := body["before_rule_id"]; has {
		t.Errorf("request body = %+v, must not carry before_rule_id when it was not given", body)
	}
}

// --- Firewall Rules (account-level) -----------------------------------------

// TestListArvanCloudAccountFirewallValidDomains pins the request shape and
// response parsing of GET /account/firewall-rules/domains.
func TestListArvanCloudAccountFirewallValidDomains(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"domain-uuid-1","name":"example.com"},{"id":"domain-uuid-2","name":"shop.example.com"}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	domains, err := provider.ListArvanCloudAccountFirewallValidDomains(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudAccountFirewallValidDomains() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/account/firewall-rules/domains" {
		t.Fatalf("request = %+v, want a single GET /account/firewall-rules/domains", records)
	}
	if len(domains) != 2 || domains[0].ID != "domain-uuid-1" || domains[0].Name != "example.com" {
		t.Errorf("domains = %+v, want the two parsed entries", domains)
	}
}

// TestListArvanCloudAccountFirewallRules pins the request shape and response
// parsing of GET /account/firewall-rules.
func TestListArvanCloudAccountFirewallRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"rule-1","name":"block-ir-everywhere","filter_expr":"ip.geoip.country in {\"IR\"}",
			"action":"deny","is_enabled":true,"is_account_level":true,"domain_selection_type":"all"}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rules, err := provider.ListArvanCloudAccountFirewallRules(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudAccountFirewallRules() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/account/firewall-rules" {
		t.Fatalf("request = %+v, want a single GET /account/firewall-rules", records)
	}
	if len(rules) != 1 || !rules[0].IsAccountLevel || rules[0].DomainSelectionType != domain.ArvanCloudDomainSelectionAll {
		t.Errorf("rules = %+v, want one parsed account-level rule", rules)
	}
}

// TestCreateArvanCloudAccountFirewallRule pins the request body of POST
// /account/firewall-rules, including domain_selection_type and domain_ids.
func TestCreateArvanCloudAccountFirewallRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusCreated, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","name":"block-scanners","filter_expr":"ip.src in {\"203.0.113.5\"}",
			"action":"deny","is_enabled":true,"domain_selection_type":"include","domain_ids":["domain-uuid-1"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	created, err := provider.CreateArvanCloudAccountFirewallRule(context.Background(), creds(), domain.ArvanCloudAccountFirewallRule{
		Name: "block-scanners", FilterExpr: `ip.src in {"203.0.113.5"}`, Action: domain.ArvanCloudFirewallActionDeny,
		IsEnabled: true, DomainSelectionType: domain.ArvanCloudDomainSelectionInclude, DomainIDs: []string{"domain-uuid-1"},
	})
	if err != nil {
		t.Fatalf("CreateArvanCloudAccountFirewallRule() error = %v", err)
	}
	if created.ID != "rule-1" || len(created.DomainIDs) != 1 {
		t.Errorf("created = %+v, want the parsed response", created)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/account/firewall-rules" {
		t.Fatalf("request = %+v, want a single POST /account/firewall-rules", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["domain_selection_type"] != "include" {
		t.Errorf(`request body["domain_selection_type"] = %v, want "include"`, body["domain_selection_type"])
	}
	ids, ok := body["domain_ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "domain-uuid-1" {
		t.Errorf("request body[\"domain_ids\"] = %#v, want [\"domain-uuid-1\"]", body["domain_ids"])
	}
}

// TestGetArvanCloudAccountFirewallRule pins the request shape of GET
// /account/firewall-rules/{accountFirewallRule}.
func TestGetArvanCloudAccountFirewallRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","name":"block-scanners","filter_expr":"ip.src in {\"203.0.113.5\"}","action":"deny","domain_selection_type":"all"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudAccountFirewallRule(context.Background(), creds(), "rule-1")
	if err != nil {
		t.Fatalf("GetArvanCloudAccountFirewallRule() error = %v", err)
	}
	if len(records) != 1 || records[0].path != "/account/firewall-rules/rule-1" {
		t.Fatalf("request = %+v, want GET /account/firewall-rules/rule-1", records)
	}
	if found.ID != "rule-1" {
		t.Errorf("found.ID = %q, want %q", found.ID, "rule-1")
	}
}

// TestUpdateArvanCloudAccountFirewallRule pins the request shape of PATCH
// /account/firewall-rules/{accountFirewallRule}.
func TestUpdateArvanCloudAccountFirewallRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","name":"block-scanners-v2","filter_expr":"ip.src in {\"203.0.113.5\"}","action":"deny","domain_selection_type":"all"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.UpdateArvanCloudAccountFirewallRule(context.Background(), creds(), "rule-1", domain.ArvanCloudAccountFirewallRule{
		Name: "block-scanners-v2", FilterExpr: `ip.src in {"203.0.113.5"}`, Action: domain.ArvanCloudFirewallActionDeny,
		DomainSelectionType: domain.ArvanCloudDomainSelectionAll,
	})
	if err != nil {
		t.Fatalf("UpdateArvanCloudAccountFirewallRule() error = %v", err)
	}
	if updated.Name != "block-scanners-v2" {
		t.Errorf("updated.Name = %q, want %q", updated.Name, "block-scanners-v2")
	}
	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/account/firewall-rules/rule-1" {
		t.Fatalf("request = %+v, want a single PATCH /account/firewall-rules/rule-1", records)
	}
}

// TestDeleteArvanCloudAccountFirewallRule pins the request shape of DELETE
// /account/firewall-rules/{accountFirewallRule}.
func TestDeleteArvanCloudAccountFirewallRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Deleted successfully"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudAccountFirewallRule(context.Background(), creds(), "rule-1"); err != nil {
		t.Fatalf("DeleteArvanCloudAccountFirewallRule() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/account/firewall-rules/rule-1" {
		t.Fatalf("request = %+v, want a single DELETE /account/firewall-rules/rule-1", records)
	}
}

// TestDeleteArvanCloudAccountFirewallRuleNotFound proves a 404 surfaces as
// domain.ErrNotFound.
func TestDeleteArvanCloudAccountFirewallRuleNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudAccountFirewallRule(context.Background(), creds(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteArvanCloudAccountFirewallRule() error = %v, want domain.ErrNotFound", err)
	}
}

// TestAttachArvanCloudAccountFirewallDomains pins the request body of POST
// /account/firewall-rules/{accountFirewallRule}/domains.
func TestAttachArvanCloudAccountFirewallDomains(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","name":"block-scanners","filter_expr":"x","action":"deny",
			"domain_selection_type":"include","domain_ids":["domain-uuid-1","domain-uuid-2"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.AttachArvanCloudAccountFirewallDomains(context.Background(), creds(), "rule-1", []string{"domain-uuid-2"})
	if err != nil {
		t.Fatalf("AttachArvanCloudAccountFirewallDomains() error = %v", err)
	}
	if len(updated.DomainIDs) != 2 {
		t.Errorf("updated.DomainIDs = %v, want the parsed response's 2 entries", updated.DomainIDs)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/account/firewall-rules/rule-1/domains" {
		t.Fatalf("request = %+v, want a single POST /account/firewall-rules/rule-1/domains", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	ids, ok := body["domain_ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "domain-uuid-2" {
		t.Errorf("request body[\"domain_ids\"] = %#v, want [\"domain-uuid-2\"]", body["domain_ids"])
	}
}

// TestDetachArvanCloudAccountFirewallDomains pins the request shape of
// DELETE /account/firewall-rules/{accountFirewallRule}/domains, including
// that the DELETE carries a JSON body (unusual for the verb, but that's the
// spec's declared shape for this endpoint).
func TestDetachArvanCloudAccountFirewallDomains(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","name":"block-scanners","filter_expr":"x","action":"deny",
			"domain_selection_type":"include","domain_ids":[]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if _, err := provider.DetachArvanCloudAccountFirewallDomains(context.Background(), creds(), "rule-1", []string{"domain-uuid-1"}); err != nil {
		t.Fatalf("DetachArvanCloudAccountFirewallDomains() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/account/firewall-rules/rule-1/domains" {
		t.Fatalf("request = %+v, want a single DELETE /account/firewall-rules/rule-1/domains", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	ids, ok := body["domain_ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "domain-uuid-1" {
		t.Errorf("request body[\"domain_ids\"] = %#v, want [\"domain-uuid-1\"]", body["domain_ids"])
	}
}

// TestReprioritizeArvanCloudAccountFirewallRules pins the request body of
// POST /account/firewall-rules/reprioritize.
func TestReprioritizeArvanCloudAccountFirewallRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Successfully changed the priority of the rule."}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.ReprioritizeArvanCloudAccountFirewallRules(context.Background(), creds(), "rule-1", "", "rule-2"); err != nil {
		t.Fatalf("ReprioritizeArvanCloudAccountFirewallRules() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/account/firewall-rules/reprioritize" {
		t.Fatalf("request = %+v, want a single POST /account/firewall-rules/reprioritize", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["rule_id"] != "rule-1" || body["before_rule_id"] != "rule-2" {
		t.Errorf("request body = %+v, want rule_id=rule-1 before_rule_id=rule-2", body)
	}
	if _, has := body["after_rule_id"]; has {
		t.Errorf("request body = %+v, must not carry after_rule_id when it was not given", body)
	}
}
