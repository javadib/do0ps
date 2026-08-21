package arvancloud

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// --- Global (account-independent reference data) --------------------------

// TestListArvanCloudWafPresets pins the request shape and response parsing
// of GET /waf.
func TestListArvanCloudWafPresets(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{
			"presets":[{"id":"preset-1","name":"OWASP Basic","description":"Basic OWASP protection",
				"packages":[{"name":"core-rules","provider":{"name":"OWASP","logo":"https://example.com/logo.png"}}]}],
			"packages":[{"id":"pkg-1","name":"core-rules","provider":{"name":"OWASP","logo":"https://example.com/logo.png"},
				"disabled_rules":[],"disabled_rulesets":[]}]
		}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	result, err := provider.ListArvanCloudWafPresets(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudWafPresets() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/waf" {
		t.Fatalf("request = %+v, want a single GET /waf", records)
	}
	if len(result.Presets) != 1 || result.Presets[0].ID != "preset-1" || result.Presets[0].Packages[0].Name != "core-rules" {
		t.Errorf("presets = %+v, want the parsed preset", result.Presets)
	}
	if len(result.Packages) != 1 || result.Packages[0].ID != "pkg-1" || result.Packages[0].Provider.Name != "OWASP" {
		t.Errorf("packages = %+v, want the parsed package", result.Packages)
	}
}

// TestGetArvanCloudWafPackage pins the request shape and response parsing of
// GET /waf/packages/{packageId}, including that rulesets are decoded.
func TestGetArvanCloudWafPackage(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"pkg-1","name":"core-rules","provider":{"name":"OWASP"},
			"disabled_rules":["rule-uuid-1"],"disabled_rulesets":[],
			"rulesets":[{"id":"rs-1","name":"SQLi","rules":[{"id":"941100","name":"SQL Injection Attack","params":{"score":8}}]}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	pkg, err := provider.GetArvanCloudWafPackage(context.Background(), creds(), "pkg-1")
	if err != nil {
		t.Fatalf("GetArvanCloudWafPackage() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/waf/packages/pkg-1" {
		t.Fatalf("request = %+v, want a single GET /waf/packages/pkg-1", records)
	}
	if pkg.ID != "pkg-1" || len(pkg.DisabledRules) != 1 {
		t.Errorf("pkg = %+v, want the parsed package", pkg)
	}
	if len(pkg.Rulesets) != 1 || pkg.Rulesets[0].ID != "rs-1" || len(pkg.Rulesets[0].Rules) != 1 || pkg.Rulesets[0].Rules[0].Name != "SQL Injection Attack" {
		t.Errorf("rulesets = %+v, want the parsed ruleset", pkg.Rulesets)
	}
}

// TestGetArvanCloudWafPackageRules pins the request shape and response
// parsing of GET /waf/packages/{packageId}/rules.
func TestGetArvanCloudWafPackageRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"941100","name":"SQL Injection Attack"},{"id":"941110","name":"XSS Filter"}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rules, err := provider.GetArvanCloudWafPackageRules(context.Background(), creds(), "pkg-1")
	if err != nil {
		t.Fatalf("GetArvanCloudWafPackageRules() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/waf/packages/pkg-1/rules" {
		t.Fatalf("request = %+v, want a single GET /waf/packages/pkg-1/rules", records)
	}
	if len(rules) != 2 || rules[0].ID != "941100" || rules[0].Name != "SQL Injection Attack" {
		t.Errorf("rules = %+v, want the two parsed entries", rules)
	}
}

// --- Per-domain WAF configuration -------------------------------------------

// TestGetArvanCloudWafSettings pins the request shape and response parsing
// of GET /domains/{domain}/waf/settings, including that mode decodes as the
// plain string "protect" (not a boolean) — the wire-encoding finding this
// issue calls out.
func TestGetArvanCloudWafSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"is_enabled":true,"mode":"protect",
			"log_redaction":{"cookies":[],"headers":["authorization"],"all_headers":false,"body":true,"records":false,"replacement_string":"--REDACTED--"},
			"packages":[{"id":"pkg-1","name":"core-rules","is_enabled":true}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings, err := provider.GetArvanCloudWafSettings(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudWafSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/waf/settings" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/waf/settings", records)
	}
	if !settings.IsEnabled || settings.Mode != domain.ArvanCloudWafModeProtect {
		t.Errorf("settings = %+v, want mode %q", settings, domain.ArvanCloudWafModeProtect)
	}
	if len(settings.LogRedaction.Headers) != 1 || settings.LogRedaction.Headers[0] != "authorization" || !settings.LogRedaction.Body {
		t.Errorf("log_redaction = %+v, want the parsed redaction config", settings.LogRedaction)
	}
	if len(settings.Packages) != 1 || settings.Packages[0].ID != "pkg-1" || !settings.Packages[0].IsEnabled {
		t.Errorf("packages = %+v, want the parsed installed package", settings.Packages)
	}
}

// TestUpdateArvanCloudWafSettings pins the request body of PATCH
// /domains/{domain}/waf/settings: mode is sent as the plain string "off",
// and is_enabled/packages (both readOnly) are never part of the request
// body.
func TestUpdateArvanCloudWafSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"is_enabled":false,"mode":"off","log_redaction":null,"packages":[]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.UpdateArvanCloudWafSettings(context.Background(), creds(), "example.com", domain.ArvanCloudWafSettings{
		Mode: domain.ArvanCloudWafModeOff,
	})
	if err != nil {
		t.Fatalf("UpdateArvanCloudWafSettings() error = %v", err)
	}
	if updated.Mode != domain.ArvanCloudWafModeOff {
		t.Errorf("updated.Mode = %q, want %q", updated.Mode, domain.ArvanCloudWafModeOff)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/waf/settings" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/waf/settings", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	// The riskiest wire-encoding detail this issue calls out: mode must be
	// sent as the plain JSON string "off", never a JSON boolean false.
	if got, ok := body["mode"].(string); !ok || got != "off" {
		t.Errorf(`request body["mode"] = %#v (type %T), want the string "off"`, body["mode"], body["mode"])
	}
	if _, hasIsEnabled := body["is_enabled"]; hasIsEnabled {
		t.Errorf("request body = %+v, must not send is_enabled (readOnly)", body)
	}
	if _, hasPackages := body["packages"]; hasPackages {
		t.Errorf("request body = %+v, must not send packages (readOnly)", body)
	}
	if _, hasLogRedaction := body["log_redaction"]; hasLogRedaction {
		t.Errorf("request body = %+v, must omit log_redaction entirely for a zero-value LogRedaction", body)
	}
}

// TestUpdateArvanCloudWafSettingsWithLogRedaction pins that a non-zero
// LogRedaction is sent as an object alongside mode.
func TestUpdateArvanCloudWafSettingsWithLogRedaction(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"is_enabled":true,"mode":"detect","log_redaction":null,"packages":[]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	_, err := provider.UpdateArvanCloudWafSettings(context.Background(), creds(), "example.com", domain.ArvanCloudWafSettings{
		Mode: domain.ArvanCloudWafModeDetect,
		LogRedaction: domain.ArvanCloudWafLogRedaction{
			AllHeaders: true,
			Body:       true,
		},
	})
	if err != nil {
		t.Fatalf("UpdateArvanCloudWafSettings() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	lr, ok := body["log_redaction"].(map[string]any)
	if !ok {
		t.Fatalf("request body[\"log_redaction\"] = %#v, want an object", body["log_redaction"])
	}
	if lr["all_headers"] != true || lr["body"] != true {
		t.Errorf("log_redaction = %+v, want all_headers/body true", lr)
	}
}

// TestReconfigureArvanCloudWaf pins the request body of POST
// /domains/{domain}/waf/actions/reconfigure.
func TestReconfigureArvanCloudWaf(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"OK"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.ReconfigureArvanCloudWaf(context.Background(), creds(), "example.com", "preset-1"); err != nil {
		t.Fatalf("ReconfigureArvanCloudWaf() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/waf/actions/reconfigure" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/waf/actions/reconfigure", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["preset_id"] != "preset-1" {
		t.Errorf(`request body["preset_id"] = %v, want "preset-1"`, body["preset_id"])
	}
}

// TestReprioritizeArvanCloudWafRules pins the request body of POST
// /domains/{domain}/waf/actions/reprioritize.
func TestReprioritizeArvanCloudWafRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"OK"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.ReprioritizeArvanCloudWafRules(context.Background(), creds(), "example.com", "rule-1", "rule-2", ""); err != nil {
		t.Fatalf("ReprioritizeArvanCloudWafRules() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/waf/actions/reprioritize" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/waf/actions/reprioritize", records)
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

// TestReprioritizeArvanCloudWafPackages pins the request body of POST
// /domains/{domain}/waf/actions/reprioritize-package.
func TestReprioritizeArvanCloudWafPackages(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"OK"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.ReprioritizeArvanCloudWafPackages(context.Background(), creds(), "example.com", "pkg-1", "", "pkg-2"); err != nil {
		t.Fatalf("ReprioritizeArvanCloudWafPackages() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/waf/actions/reprioritize-package" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/waf/actions/reprioritize-package", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["package_id"] != "pkg-1" || body["before_package_id"] != "pkg-2" {
		t.Errorf("request body = %+v, want package_id/before_package_id set", body)
	}
	if _, hasAfter := body["after_package_id"]; hasAfter {
		t.Errorf("request body = %+v, must omit after_package_id when unset", body)
	}
}

// --- Per-domain WAF custom rules ---------------------------------------------

// TestListArvanCloudWafRules pins the request shape and response parsing of
// GET /domains/{domain}/waf/rules.
func TestListArvanCloudWafRules(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"rule-1","url_pattern":"/wp-admin/**","sources":["203.0.113.0/24"],"action":"passthrough","description":"trust office","is_enabled":true},
			{"id":"rule-2","url_pattern":"/api/**","sources":[],"action":"protect","is_enabled":true}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rules, err := provider.ListArvanCloudWafRules(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudWafRules() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/waf/rules" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/waf/rules", records)
	}
	if len(rules) != 2 || rules[0].ID != "rule-1" || rules[0].Action != domain.ArvanCloudWafRuleActionPassthrough {
		t.Errorf("rules = %+v, want the two parsed entries", rules)
	}
}

// TestCreateArvanCloudWafRule pins the request body of POST
// /domains/{domain}/waf/rules, including that exceptions are sent as
// integer rule IDs (the write-side shape), not the {id,name} read-side
// shape.
func TestCreateArvanCloudWafRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusCreated, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","url_pattern":"/wp-admin/**","sources":["203.0.113.0/24"],
			"action":"passthrough","description":"trust office","is_enabled":true,
			"exceptions":[{"package":"core-rules","exceptions":{"ids":[{"id":"941100","name":"SQL Injection Attack"}]}}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	created, err := provider.CreateArvanCloudWafRule(context.Background(), creds(), "example.com", domain.ArvanCloudWafRule{
		URLPattern:  "/wp-admin/**",
		Sources:     []string{"203.0.113.0/24"},
		Action:      domain.ArvanCloudWafRuleActionPassthrough,
		Description: "trust office",
		IsEnabled:   true,
		Exceptions: []domain.ArvanCloudWafRuleException{
			{Package: "core-rules", RuleIDs: []int{941100}},
		},
	})
	if err != nil {
		t.Fatalf("CreateArvanCloudWafRule() error = %v", err)
	}
	if created.ID != "rule-1" || created.Action != domain.ArvanCloudWafRuleActionPassthrough {
		t.Errorf("created = %+v, want the parsed response", created)
	}
	if len(created.Exceptions) != 1 || len(created.Exceptions[0].RuleIDs) != 1 || created.Exceptions[0].RuleIDs[0] != 941100 || created.Exceptions[0].RuleNames[0] != "SQL Injection Attack" {
		t.Errorf("created.Exceptions = %+v, want the parsed exception with numeric id 941100", created.Exceptions)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/waf/rules" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/waf/rules", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["url_pattern"] != "/wp-admin/**" || body["action"] != "passthrough" {
		t.Errorf("request body = %+v, want url_pattern/action set", body)
	}
	exceptions, ok := body["exceptions"].([]any)
	if !ok || len(exceptions) != 1 {
		t.Fatalf("request body[\"exceptions\"] = %#v, want a single-entry array", body["exceptions"])
	}
	exception, ok := exceptions[0].(map[string]any)
	if !ok || exception["package"] != "core-rules" {
		t.Fatalf("exceptions[0] = %#v, want package \"core-rules\"", exceptions[0])
	}
	innerExceptions, ok := exception["exceptions"].(map[string]any)
	if !ok {
		t.Fatalf("exceptions[0][\"exceptions\"] = %#v, want an object", exception["exceptions"])
	}
	ids, ok := innerExceptions["ids"].([]any)
	if !ok || len(ids) != 1 {
		t.Fatalf("exceptions[0][\"exceptions\"][\"ids\"] = %#v, want a single-entry array", innerExceptions["ids"])
	}
	// The write-side shape is a plain integer, not the {id,name} object the
	// read side uses.
	if _, isNumber := ids[0].(float64); !isNumber {
		t.Errorf("exceptions[0][\"exceptions\"][\"ids\"][0] = %#v (type %T), want a JSON number", ids[0], ids[0])
	}
}

// TestGetArvanCloudWafRule pins the request shape and response parsing of
// GET /domains/{domain}/waf/rules/{id}.
func TestGetArvanCloudWafRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","url_pattern":"/api/**","sources":[],"action":"protect","is_enabled":false}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudWafRule(context.Background(), creds(), "example.com", "rule-1")
	if err != nil {
		t.Fatalf("GetArvanCloudWafRule() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/waf/rules/rule-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/waf/rules/rule-1", records)
	}
	if found.ID != "rule-1" || found.IsEnabled {
		t.Errorf("found = %+v, want the parsed response", found)
	}
}

// TestUpdateArvanCloudWafRule pins the request body of PATCH
// /domains/{domain}/waf/rules/{id}.
func TestUpdateArvanCloudWafRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"rule-1","url_pattern":"/api/v2/**","sources":[],"action":"protect","is_enabled":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.UpdateArvanCloudWafRule(context.Background(), creds(), "example.com", "rule-1", domain.ArvanCloudWafRule{
		URLPattern: "/api/v2/**",
		Action:     domain.ArvanCloudWafRuleActionProtect,
		IsEnabled:  true,
	})
	if err != nil {
		t.Fatalf("UpdateArvanCloudWafRule() error = %v", err)
	}
	if updated.URLPattern != "/api/v2/**" {
		t.Errorf("updated.URLPattern = %q, want \"/api/v2/**\"", updated.URLPattern)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/waf/rules/rule-1" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/waf/rules/rule-1", records)
	}
}

// TestDeleteArvanCloudWafRule pins the request shape of DELETE
// /domains/{domain}/waf/rules/{id}.
func TestDeleteArvanCloudWafRule(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"OK"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudWafRule(context.Background(), creds(), "example.com", "rule-1"); err != nil {
		t.Fatalf("DeleteArvanCloudWafRule() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/waf/rules/rule-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/waf/rules/rule-1", records)
	}
}

// --- Per-domain WAF package subscriptions -----------------------------------

// TestListArvanCloudWafDomainPackages pins the request shape and response
// parsing of GET /domains/{domain}/waf/packages.
func TestListArvanCloudWafDomainPackages(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"pkg-1","name":"core-rules","is_enabled":true,"params":{"sensitivity":"high"}}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	packages, err := provider.ListArvanCloudWafDomainPackages(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudWafDomainPackages() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/waf/packages" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/waf/packages", records)
	}
	if len(packages) != 1 || packages[0].ID != "pkg-1" || !packages[0].IsEnabled || packages[0].Params["sensitivity"] != "high" {
		t.Errorf("packages = %+v, want the parsed installed package", packages)
	}
}

// TestInstallArvanCloudWafPackage pins the request body of POST
// /domains/{domain}/waf/packages.
func TestInstallArvanCloudWafPackage(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"pkg-1","name":"core-rules","is_enabled":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	installed, err := provider.InstallArvanCloudWafPackage(context.Background(), creds(), "example.com", "pkg-1")
	if err != nil {
		t.Fatalf("InstallArvanCloudWafPackage() error = %v", err)
	}
	if installed.ID != "pkg-1" {
		t.Errorf("installed = %+v, want the parsed response", installed)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/waf/packages" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/waf/packages", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["id"] != "pkg-1" {
		t.Errorf(`request body["id"] = %v, want "pkg-1"`, body["id"])
	}
}

// TestGetArvanCloudWafDomainPackage pins the request shape and response
// parsing of GET /domains/{domain}/waf/packages/{id}.
func TestGetArvanCloudWafDomainPackage(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"pkg-1","name":"core-rules","is_enabled":true,
			"rulesets":[{"id":"rs-1","name":"SQLi","rules":[{"id":"941100","name":"SQL Injection Attack"}]}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudWafDomainPackage(context.Background(), creds(), "example.com", "pkg-1")
	if err != nil {
		t.Fatalf("GetArvanCloudWafDomainPackage() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/waf/packages/pkg-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/waf/packages/pkg-1", records)
	}
	if found.ID != "pkg-1" || len(found.Rulesets) != 1 {
		t.Errorf("found = %+v, want the parsed response with rulesets", found)
	}
}

// TestUpdateArvanCloudWafDomainPackage pins the request body of PATCH
// /domains/{domain}/waf/packages/{id}: only params/is_enabled/
// disabled_rules/disabled_rulesets are sent, never read-only fields like
// name or provider.
func TestUpdateArvanCloudWafDomainPackage(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"pkg-1","name":"core-rules","is_enabled":false,"disabled_rules":["941100"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.UpdateArvanCloudWafDomainPackage(context.Background(), creds(), "example.com", "pkg-1", domain.ArvanCloudWafPackage{
		IsEnabled:     false,
		DisabledRules: []string{"941100"},
	})
	if err != nil {
		t.Fatalf("UpdateArvanCloudWafDomainPackage() error = %v", err)
	}
	if updated.IsEnabled {
		t.Errorf("updated.IsEnabled = true, want false")
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/waf/packages/pkg-1" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/waf/packages/pkg-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["is_enabled"] != false {
		t.Errorf(`request body["is_enabled"] = %v, want false`, body["is_enabled"])
	}
	if _, hasName := body["name"]; hasName {
		t.Errorf("request body = %+v, must not send name (read-only)", body)
	}
	if _, hasProvider := body["provider"]; hasProvider {
		t.Errorf("request body = %+v, must not send provider (read-only)", body)
	}
}

// TestUninstallArvanCloudWafPackage pins the request shape of DELETE
// /domains/{domain}/waf/packages/{id}.
func TestUninstallArvanCloudWafPackage(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"OK"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.UninstallArvanCloudWafPackage(context.Background(), creds(), "example.com", "pkg-1"); err != nil {
		t.Fatalf("UninstallArvanCloudWafPackage() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/waf/packages/pkg-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/waf/packages/pkg-1", records)
	}
}
