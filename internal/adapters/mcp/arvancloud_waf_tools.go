package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud WAF tools (issue #66): the managed rule-set engine (OWASP-style
// packages/presets a domain subscribes to). All fast operations (AGENTS.md
// 4.3): every tool below returns its result within the call, with no
// operation_id to poll afterward.
//
// Naming collision warning, repeated on every tool description below because
// both capabilities use the word "rule": WAF here is ArvanCloud's managed
// rule-set engine (this file), completely distinct from the free-form
// filter_expr CDN edge Firewall in arvancloud_firewall_tools.go. A
// natural-language request like "block SQL injection attempts" or "turn on
// OWASP protection" should route to a WAF tool — typically
// reconfigure_arvancloud_waf with an OWASP-style preset, or
// install_arvancloud_waf_package — NOT to create_arvancloud_firewall_rule.
// The Firewall's filter_expr is a hand-written Wireshark-like expression the
// caller authors themselves; WAF's protection comes from subscribing to
// vendor-maintained rule packages, with only a thin caller-authored
// exception layer (the "rules" in this file's tool names) on top of them.

const arvanCloudWafVsFirewallNote = "WAF here means ArvanCloud's managed rule-set engine (OWASP-style packages/presets a domain subscribes to) — completely distinct from the CDN edge Firewall's hand-written filter_expr rules (create_arvancloud_firewall_rule and friends). A request like \"block SQL injection attempts\" or \"turn on OWASP protection\" should use a WAF tool (reconfigure_arvancloud_waf with a preset, or install_arvancloud_waf_package), not the Firewall."

// arvanCloudWafProviderToMap renders a domain.ArvanCloudWafProvider.
func arvanCloudWafProviderToMap(p domain.ArvanCloudWafProvider) map[string]any {
	return map[string]any{"name": p.Name, "logo": p.Logo}
}

// arvanCloudWafRulesetsToSlice renders []domain.ArvanCloudWafRuleset, nil
// (omitted by the JSON encoder as a null) when the calling endpoint never
// populates rulesets (every method but the "*Details" show ones — see
// domain.ArvanCloudWafPackage's doc comment).
func arvanCloudWafRulesetsToSlice(rulesets []domain.ArvanCloudWafRuleset) []map[string]any {
	if rulesets == nil {
		return nil
	}
	out := make([]map[string]any, len(rulesets))
	for i, rs := range rulesets {
		rules := make([]map[string]any, len(rs.Rules))
		for j, r := range rs.Rules {
			rules[j] = map[string]any{"id": r.ID, "name": r.Name, "params": r.Params}
		}
		out[i] = map[string]any{"id": rs.ID, "name": rs.Name, "rules": rules}
	}
	return out
}

// arvanCloudWafPackageToMap renders a domain.ArvanCloudWafPackage the way
// every package-returning tool reports it back to the caller. is_enabled and
// params are meaningful only for a domain-installed package instance — see
// that type's own doc comment — but are still reported here (as
// false/empty) for a global package, since a caller inspecting the raw
// response can already tell the two shapes apart by which tool it called.
func arvanCloudWafPackageToMap(p domain.ArvanCloudWafPackage) map[string]any {
	return map[string]any{
		"id":                p.ID,
		"name":              p.Name,
		"provider":          arvanCloudWafProviderToMap(p.Provider),
		"params_schema":     p.ParamsSchema,
		"disabled_rules":    p.DisabledRules,
		"disabled_rulesets": p.DisabledRulesets,
		"params":            p.Params,
		"is_enabled":        p.IsEnabled,
		"rulesets":          arvanCloudWafRulesetsToSlice(p.Rulesets),
	}
}

// arvanCloudWafPresetToMap renders a domain.ArvanCloudWafPreset.
func arvanCloudWafPresetToMap(preset domain.ArvanCloudWafPreset) map[string]any {
	packages := make([]map[string]any, len(preset.Packages))
	for i, pkg := range preset.Packages {
		packages[i] = map[string]any{"name": pkg.Name, "provider": arvanCloudWafProviderToMap(pkg.Provider)}
	}
	return map[string]any{
		"id":          preset.ID,
		"name":        preset.Name,
		"description": preset.Description,
		"packages":    packages,
	}
}

// arvanCloudWafLogRedactionToMap renders a domain.ArvanCloudWafLogRedaction.
func arvanCloudWafLogRedactionToMap(lr domain.ArvanCloudWafLogRedaction) map[string]any {
	return map[string]any{
		"cookies":            lr.Cookies,
		"headers":            lr.Headers,
		"all_headers":        lr.AllHeaders,
		"body":               lr.Body,
		"records":            lr.Records,
		"replacement_string": string(lr.ReplacementString),
	}
}

// arvanCloudWafSettingsToMap renders a domain.ArvanCloudWafSettings.
func arvanCloudWafSettingsToMap(s domain.ArvanCloudWafSettings) map[string]any {
	packages := make([]map[string]any, len(s.Packages))
	for i, pkg := range s.Packages {
		packages[i] = arvanCloudWafPackageToMap(pkg)
	}
	return map[string]any{
		"is_enabled":    s.IsEnabled,
		"mode":          string(s.Mode),
		"log_redaction": arvanCloudWafLogRedactionToMap(s.LogRedaction),
		"packages":      packages,
	}
}

// arvanCloudWafRuleExceptionsToSlice renders []domain.ArvanCloudWafRuleException.
func arvanCloudWafRuleExceptionsToSlice(exceptions []domain.ArvanCloudWafRuleException) []map[string]any {
	out := make([]map[string]any, len(exceptions))
	for i, e := range exceptions {
		out[i] = map[string]any{"package": e.Package, "rule_ids": e.RuleIDs, "rule_names": e.RuleNames}
	}
	return out
}

// arvanCloudWafRuleToMap renders a domain.ArvanCloudWafRule the way every
// WAF custom-rule-returning tool reports it back to the caller.
func arvanCloudWafRuleToMap(r domain.ArvanCloudWafRule) map[string]any {
	return map[string]any{
		"id":          r.ID,
		"url_pattern": r.URLPattern,
		"sources":     r.Sources,
		"action":      string(r.Action),
		"description": r.Description,
		"is_enabled":  r.IsEnabled,
		"exceptions":  arvanCloudWafRuleExceptionsToSlice(r.Exceptions),
	}
}

// arvanCloudWafRuleExceptionArgs is the tool-argument shape of one
// domain.ArvanCloudWafRuleException.
type arvanCloudWafRuleExceptionArgs struct {
	Package string `json:"package"`
	RuleIDs []int  `json:"rule_ids"`
}

func arvanCloudWafRuleExceptionsFromArgs(raw []arvanCloudWafRuleExceptionArgs) []domain.ArvanCloudWafRuleException {
	if len(raw) == 0 {
		return nil
	}
	out := make([]domain.ArvanCloudWafRuleException, len(raw))
	for i, e := range raw {
		out[i] = domain.ArvanCloudWafRuleException{Package: e.Package, RuleIDs: e.RuleIDs}
	}
	return out
}

func arvanCloudWafRuleExceptionsProperty() map[string]any {
	return map[string]any{
		"type": "array",
		"description": "Exempt specific numbered rules within named WAF packages from this custom rule's effect. " +
			"Usually left empty; only needed to carve out a narrow exception within an otherwise-active package.",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"package":  map[string]any{"type": "string", "description": "The WAF package name whose rules to exempt."},
				"rule_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "The package's own numbered rule IDs to exempt, e.g. from get_arvancloud_waf_package_rules."},
			},
		},
	}
}

// --- Global (account-independent reference data) --------------------------

func listArvanCloudWafPresetsTool(uc *app.ListArvanCloudWafPresets) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_arvancloud_waf_presets",
		Description: "List ArvanCloud's predefined WAF configurations (presets) and the full catalog of global WAF " +
			"packages they draw from. " + arvanCloudWafVsFirewallNote + " Call this before reconfigure_arvancloud_waf " +
			"to find a preset_id, or before install_arvancloud_waf_package to find a package id. This is a fast " +
			"operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args credentialArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			result, err := uc.Execute(ctx, app.ListArvanCloudWafPresetsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			presets := make([]map[string]any, len(result.Presets))
			for i, preset := range result.Presets {
				presets[i] = arvanCloudWafPresetToMap(preset)
			}
			packages := make([]map[string]any, len(result.Packages))
			for i, pkg := range result.Packages {
				packages[i] = arvanCloudWafPackageToMap(pkg)
			}
			return map[string]any{"presets": presets, "packages": packages}, nil
		},
	}
}

// arvanCloudWafPackageIDArgs is embedded by every global-package tool below
// that is scoped to exactly one package by id.
type arvanCloudWafPackageIDArgs struct {
	credentialArgs
	PackageID string `json:"package_id"`
}

func arvanCloudWafPackageIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The global WAF package's ID, from list_arvancloud_waf_presets.",
	}
}

func getArvanCloudWafPackageTool(uc *app.GetArvanCloudWafPackage) Tool {
	props := credentialProperties()
	props["package_id"] = arvanCloudWafPackageIDProperty()

	return Tool{
		Name: "get_arvancloud_waf_package",
		Description: "Get a global WAF package's details, including its rulesets. " + arvanCloudWafVsFirewallNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "package_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudWafPackageIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudWafPackageInput{Credentials: args.domain(), PackageID: args.PackageID})
			if err != nil {
				return nil, err
			}
			return arvanCloudWafPackageToMap(*found), nil
		},
	}
}

func getArvanCloudWafPackageRulesTool(uc *app.GetArvanCloudWafPackageRules) Tool {
	props := credentialProperties()
	props["package_id"] = arvanCloudWafPackageIDProperty()

	return Tool{
		Name: "get_arvancloud_waf_package_rules",
		Description: "Get a global WAF package's individual rule details (id and name of each rule the package " +
			"contains). " + arvanCloudWafVsFirewallNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "package_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudWafPackageIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rules, err := uc.Execute(ctx, app.GetArvanCloudWafPackageRulesInput{Credentials: args.domain(), PackageID: args.PackageID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, r := range rules {
				out[i] = map[string]any{"id": r.ID, "name": r.Name}
			}
			return map[string]any{"rules": out}, nil
		},
	}
}

// --- Per-domain WAF configuration ------------------------------------------

func getArvanCloudWafSettingsTool(uc *app.GetArvanCloudWafSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_waf_settings",
		Description: "Get a domain's WAF configuration: whether it is enabled, its mode (off/detect/protect), log " +
			"redaction settings, and the packages currently configuring it. " + arvanCloudWafVsFirewallNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudWafSettingsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudWafSettingsToMap(*found), nil
		},
	}
}

func arvanCloudWafModeProperty() map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudWafModeOff),
			string(domain.ArvanCloudWafModeDetect),
			string(domain.ArvanCloudWafModeProtect),
		},
		"description": "The domain's WAF operating mode: \"off\" disables WAF inspection entirely, \"detect\" " +
			"inspects and logs matches without blocking anything (log-only), \"protect\" inspects and actively " +
			"blocks matching requests.",
	}
}

func arvanCloudWafLogRedactionProperty() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Which parts of this domain's WAF logs are redacted to protect sensitive information. Leave every field unset to keep ArvanCloud's own defaults.",
		"properties": map[string]any{
			"cookies":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Cookie names to redact."},
			"headers":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Header names to redact. Defaults to authorization/proxy-authorization/cookie."},
			"all_headers": map[string]any{"type": "boolean", "description": "Redact every header, regardless of the headers list."},
			"body":        map[string]any{"type": "boolean", "description": "Redact the request body. Defaults to true."},
			"records":     map[string]any{"type": "boolean", "description": "Redact specific log entries or entire log records."},
			"replacement_string": map[string]any{
				"type": "string", "enum": []string{"****", "####", "--REDACTED--", ""},
				"description": "What a redacted value is replaced with. Defaults to \"--REDACTED--\"; \"\" means replace with nothing.",
			},
		},
	}
}

type arvanCloudWafLogRedactionArgs struct {
	Cookies           []string `json:"cookies"`
	Headers           []string `json:"headers"`
	AllHeaders        bool     `json:"all_headers"`
	Body              bool     `json:"body"`
	Records           bool     `json:"records"`
	ReplacementString string   `json:"replacement_string"`
}

func (a *arvanCloudWafLogRedactionArgs) toDomain() domain.ArvanCloudWafLogRedaction {
	if a == nil {
		return domain.ArvanCloudWafLogRedaction{}
	}
	return domain.ArvanCloudWafLogRedaction{
		Cookies: a.Cookies, Headers: a.Headers, AllHeaders: a.AllHeaders, Body: a.Body, Records: a.Records,
		ReplacementString: domain.ArvanCloudWafLogRedactionReplacement(a.ReplacementString),
	}
}

func updateArvanCloudWafSettingsTool(uc *app.UpdateArvanCloudWafSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["mode"] = arvanCloudWafModeProperty()
	props["log_redaction"] = arvanCloudWafLogRedactionProperty()

	return Tool{
		Name: "update_arvancloud_waf_settings",
		Description: "Update a domain's WAF configuration (mode and log redaction). " + arvanCloudWafVsFirewallNote +
			" To turn on protection with a ready-made rule set (e.g. \"block SQL injection attempts\"), prefer " +
			"reconfigure_arvancloud_waf with a preset — this tool alone changes only the mode, not which packages " +
			"are installed. This is a fast operation: the updated settings are returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "mode"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Mode         string                         `json:"mode"`
				LogRedaction *arvanCloudWafLogRedactionArgs `json:"log_redaction"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudWafSettingsInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				Settings: domain.ArvanCloudWafSettings{
					Mode:         domain.ArvanCloudWafMode(args.Mode),
					LogRedaction: args.LogRedaction.toDomain(),
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudWafSettingsToMap(*updated), nil
		},
	}
}

func reconfigureArvanCloudWafTool(uc *app.ReconfigureArvanCloudWaf) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["preset_id"] = map[string]any{
		"type":        "string",
		"description": "The preset to apply, from list_arvancloud_waf_presets. Applying a preset REMOVES every WAF package currently installed on the domain and replaces it with the preset's own set.",
	}

	return Tool{
		Name: "reconfigure_arvancloud_waf",
		Description: "Apply a WAF preset to a domain in one call: this is the tool for a request like \"turn on " +
			"OWASP-style protection\" or \"block SQL injection attempts\". " + arvanCloudWafVsFirewallNote +
			" Applying a preset removes every WAF package currently installed on the domain and replaces it with " +
			"the preset's own set — call list_arvancloud_waf_presets first to find a preset_id and confirm what it " +
			"installs. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "preset_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				PresetID string `json:"preset_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.ReconfigureArvanCloudWafInput{Credentials: args.domain(), Domain: args.Domain, PresetID: args.PresetID}); err != nil {
				return nil, err
			}
			return map[string]any{"reconfigured": true, "domain": args.Domain, "preset_id": args.PresetID}, nil
		},
	}
}

func reprioritizeArvanCloudWafRulesTool(uc *app.ReprioritizeArvanCloudWafRules) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReprioritizeProperties(props)

	return Tool{
		Name: "reprioritize_arvancloud_waf_rules",
		Description: "Change the evaluation order of a domain's WAF custom rules by moving one rule relative to " +
			"another. " + arvanCloudWafVsFirewallNote + " Give exactly one of after_rule_id/before_rule_id, not " +
			"both. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				RuleID       string `json:"rule_id"`
				AfterRuleID  string `json:"after_rule_id"`
				BeforeRuleID string `json:"before_rule_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.ReprioritizeArvanCloudWafRulesInput{
				Credentials: args.domain(), Domain: args.Domain,
				RuleID: args.RuleID, AfterRuleID: args.AfterRuleID, BeforeRuleID: args.BeforeRuleID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"reprioritized": true, "domain": args.Domain, "rule_id": args.RuleID}, nil
		},
	}
}

func reprioritizeArvanCloudWafPackagesTool(uc *app.ReprioritizeArvanCloudWafPackages) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["package_id"] = map[string]any{"type": "string", "description": "The ID of the installed package to move, from list_arvancloud_waf_domain_packages."}
	props["after_package_id"] = map[string]any{"type": "string", "description": "Move package_id to just after this package. Give only one of after_package_id/before_package_id."}
	props["before_package_id"] = map[string]any{"type": "string", "description": "Move package_id to just before this package. Give only one of after_package_id/before_package_id."}

	return Tool{
		Name: "reprioritize_arvancloud_waf_packages",
		Description: "Change the evaluation order of a domain's installed WAF packages by moving one package " +
			"relative to another. " + arvanCloudWafVsFirewallNote + " Give exactly one of after_package_id/" +
			"before_package_id, not both. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "package_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				PackageID       string `json:"package_id"`
				AfterPackageID  string `json:"after_package_id"`
				BeforePackageID string `json:"before_package_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.ReprioritizeArvanCloudWafPackagesInput{
				Credentials: args.domain(), Domain: args.Domain,
				PackageID: args.PackageID, AfterPackageID: args.AfterPackageID, BeforePackageID: args.BeforePackageID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"reprioritized": true, "domain": args.Domain, "package_id": args.PackageID}, nil
		},
	}
}

// --- Per-domain WAF custom rules --------------------------------------------

// arvanCloudWafRuleIDArgs is embedded by every WAF custom-rule tool below
// that is scoped to exactly one rule by domain + id.
type arvanCloudWafRuleIDArgs struct {
	arvanCloudDomainNameArgs
	ID string `json:"id"`
}

func arvanCloudWafRuleIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The WAF custom rule's provider-assigned ID (a UUID), as returned by create_arvancloud_waf_rule or list_arvancloud_waf_rules.",
	}
}

func arvanCloudWafRuleActionProperty(description string) map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudWafRuleActionProtect),
			string(domain.ArvanCloudWafRuleActionPassthrough),
		},
		"description": description,
	}
}

func listArvanCloudWafRulesTool(uc *app.ListArvanCloudWafRules) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_waf_rules",
		Description: "List every WAF custom rule configured for a domain — the thin caller-authored exception " +
			"layer on top of the domain's installed WAF packages. " + arvanCloudWafVsFirewallNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rules, err := uc.Execute(ctx, app.ListArvanCloudWafRulesInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, r := range rules {
				out[i] = arvanCloudWafRuleToMap(r)
			}
			return map[string]any{"rules": out}, nil
		},
	}
}

func createArvanCloudWafRuleTool(uc *app.CreateArvanCloudWafRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["url_pattern"] = map[string]any{
		"type": "string",
		"description": "A glob pattern (not a regex) this rule matches a request's URL against: `?` matches any " +
			"single character, `*` matches any sequence of characters, `**` matches across path segments, " +
			"`[...]`/`[!...]` match/exclude a character class. Example: \"/wp-admin/**\" or \"/api/v?/users\".",
	}
	props["sources"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "CIDR-format source addresses this rule matches against, e.g. \"203.0.113.0/24\". At most 20 entries.",
	}
	props["action"] = arvanCloudWafRuleActionProperty(
		"What happens to a request this rule matches: \"protect\" actively enforces WAF protection, \"passthrough\" exempts the request from WAF inspection entirely.")
	props["description"] = map[string]any{"type": "string", "description": "An optional note about what this rule is for."}
	props["is_enabled"] = map[string]any{"type": "boolean", "description": "Whether the rule is active. Defaults to true when omitted."}
	props["exceptions"] = arvanCloudWafRuleExceptionsProperty()

	return Tool{
		Name: "create_arvancloud_waf_rule",
		Description: "Create a new WAF custom rule: a caller-authored exception layered on top of a domain's " +
			"installed WAF packages (e.g. \"exempt this specific source range from WAF inspection\"). " +
			arvanCloudWafVsFirewallNote + " This is a fast operation: the created rule, including its " +
			"provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "url_pattern", "action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				URLPattern  string                           `json:"url_pattern"`
				Sources     []string                         `json:"sources"`
				Action      string                           `json:"action"`
				Description string                           `json:"description"`
				IsEnabled   *bool                            `json:"is_enabled"`
				Exceptions  []arvanCloudWafRuleExceptionArgs `json:"exceptions"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			isEnabled := true
			if args.IsEnabled != nil {
				isEnabled = *args.IsEnabled
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudWafRuleInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				Rule: domain.ArvanCloudWafRule{
					URLPattern:  args.URLPattern,
					Sources:     args.Sources,
					Action:      domain.ArvanCloudWafRuleAction(args.Action),
					Description: args.Description,
					IsEnabled:   isEnabled,
					Exceptions:  arvanCloudWafRuleExceptionsFromArgs(args.Exceptions),
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudWafRuleToMap(*created), nil
		},
	}
}

func getArvanCloudWafRuleTool(uc *app.GetArvanCloudWafRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudWafRuleIDProperty()

	return Tool{
		Name:        "get_arvancloud_waf_rule",
		Description: "Get the current state of one WAF custom rule by ID. " + arvanCloudWafVsFirewallNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudWafRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudWafRuleInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudWafRuleToMap(*found), nil
		},
	}
}

func updateArvanCloudWafRuleTool(uc *app.UpdateArvanCloudWafRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudWafRuleIDProperty()
	props["url_pattern"] = map[string]any{"type": "string", "description": "The rule's new glob pattern. See create_arvancloud_waf_rule for the pattern syntax."}
	props["sources"] = map[string]any{
		"type": "array", "items": map[string]any{"type": "string"},
		"description": "The rule's new CIDR-format source addresses. At most 20 entries.",
	}
	props["action"] = arvanCloudWafRuleActionProperty("The rule's new action. See create_arvancloud_waf_rule for the full list.")
	props["description"] = map[string]any{"type": "string", "description": "The rule's new note."}
	props["is_enabled"] = map[string]any{"type": "boolean", "description": "Whether the rule is active."}
	props["exceptions"] = arvanCloudWafRuleExceptionsProperty()

	return Tool{
		Name: "update_arvancloud_waf_rule",
		Description: "Update a WAF custom rule. This replaces the rule's fields with the given values — pass every " +
			"field you want to keep, not only the ones changing. " + arvanCloudWafVsFirewallNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "url_pattern", "action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudWafRuleIDArgs
				URLPattern  string                           `json:"url_pattern"`
				Sources     []string                         `json:"sources"`
				Action      string                           `json:"action"`
				Description string                           `json:"description"`
				IsEnabled   bool                             `json:"is_enabled"`
				Exceptions  []arvanCloudWafRuleExceptionArgs `json:"exceptions"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudWafRuleInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				ID:          args.ID,
				Rule: domain.ArvanCloudWafRule{
					URLPattern:  args.URLPattern,
					Sources:     args.Sources,
					Action:      domain.ArvanCloudWafRuleAction(args.Action),
					Description: args.Description,
					IsEnabled:   args.IsEnabled,
					Exceptions:  arvanCloudWafRuleExceptionsFromArgs(args.Exceptions),
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudWafRuleToMap(*updated), nil
		},
	}
}

func deleteArvanCloudWafRuleTool(uc *app.DeleteArvanCloudWafRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudWafRuleIDProperty()

	return Tool{
		Name: "delete_arvancloud_waf_rule",
		Description: "Permanently delete a WAF custom rule by ID. " + arvanCloudWafVsFirewallNote +
			" This is a fast operation and cannot be undone. Deleting a rule that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudWafRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudWafRuleInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

// --- Per-domain WAF package subscriptions -----------------------------------

func listArvanCloudWafDomainPackagesTool(uc *app.ListArvanCloudWafDomainPackages) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_waf_domain_packages",
		Description: "List the WAF packages currently installed on a domain. " + arvanCloudWafVsFirewallNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			packages, err := uc.Execute(ctx, app.ListArvanCloudWafDomainPackagesInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(packages))
			for i, pkg := range packages {
				out[i] = arvanCloudWafPackageToMap(pkg)
			}
			return map[string]any{"packages": out}, nil
		},
	}
}

func installArvanCloudWafPackageTool(uc *app.InstallArvanCloudWafPackage) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["package_id"] = map[string]any{
		"type":        "string",
		"description": "The global WAF package to subscribe the domain to, from list_arvancloud_waf_presets.",
	}

	return Tool{
		Name: "install_arvancloud_waf_package",
		Description: "Subscribe a domain to one global WAF package, adding it to whatever is already installed " +
			"(unlike reconfigure_arvancloud_waf, which replaces the whole set). " + arvanCloudWafVsFirewallNote +
			" This is a fast operation: the installed package is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "package_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				PackageID string `json:"package_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			installed, err := uc.Execute(ctx, app.InstallArvanCloudWafPackageInput{Credentials: args.domain(), Domain: args.Domain, PackageID: args.PackageID})
			if err != nil {
				return nil, err
			}
			return arvanCloudWafPackageToMap(*installed), nil
		},
	}
}

// arvanCloudWafDomainPackageIDArgs is embedded by every installed-package
// tool below that is scoped to exactly one installed package by domain + id.
type arvanCloudWafDomainPackageIDArgs struct {
	arvanCloudDomainNameArgs
	ID string `json:"id"`
}

func arvanCloudWafDomainPackageIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The installed package's ID, from list_arvancloud_waf_domain_packages or install_arvancloud_waf_package.",
	}
}

func getArvanCloudWafDomainPackageTool(uc *app.GetArvanCloudWafDomainPackage) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudWafDomainPackageIDProperty()

	return Tool{
		Name: "get_arvancloud_waf_domain_package",
		Description: "Get one installed WAF package's details, including its rulesets. " + arvanCloudWafVsFirewallNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudWafDomainPackageIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudWafDomainPackageInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudWafPackageToMap(*found), nil
		},
	}
}

func updateArvanCloudWafDomainPackageTool(uc *app.UpdateArvanCloudWafDomainPackage) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudWafDomainPackageIDProperty()
	props["is_enabled"] = map[string]any{"type": "boolean", "description": "Whether the installed package is active, without uninstalling it. Defaults to true when omitted."}
	props["disabled_rules"] = map[string]any{
		"type": "array", "items": map[string]any{"type": "string"},
		"description": "Rule IDs (UUIDs) within this package to turn off individually, from get_arvancloud_waf_package_rules.",
	}
	props["disabled_rulesets"] = map[string]any{
		"type": "array", "items": map[string]any{"type": "string"},
		"description": "Ruleset IDs (UUIDs) within this package to turn off individually.",
	}
	props["params"] = map[string]any{
		"type":        "object",
		"description": "Package-specific configuration values, validated against the package's own params_schema (see get_arvancloud_waf_package).",
	}

	return Tool{
		Name: "update_arvancloud_waf_domain_package",
		Description: "Update an installed WAF package's own configuration: toggle it, or selectively disable " +
			"individual rules/rulesets within it without uninstalling the whole package. " + arvanCloudWafVsFirewallNote +
			" This is a fast operation: the package as stored afterward is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudWafDomainPackageIDArgs
				IsEnabled        *bool          `json:"is_enabled"`
				DisabledRules    []string       `json:"disabled_rules"`
				DisabledRulesets []string       `json:"disabled_rulesets"`
				Params           map[string]any `json:"params"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			isEnabled := true
			if args.IsEnabled != nil {
				isEnabled = *args.IsEnabled
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudWafDomainPackageInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				ID:          args.ID,
				Package: domain.ArvanCloudWafPackage{
					DisabledRules:    args.DisabledRules,
					DisabledRulesets: args.DisabledRulesets,
					Params:           args.Params,
					IsEnabled:        isEnabled,
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudWafPackageToMap(*updated), nil
		},
	}
}

func uninstallArvanCloudWafPackageTool(uc *app.UninstallArvanCloudWafPackage) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudWafDomainPackageIDProperty()

	return Tool{
		Name: "uninstall_arvancloud_waf_package",
		Description: "Remove an installed WAF package from a domain. " + arvanCloudWafVsFirewallNote +
			" This is a fast operation and cannot be undone. Uninstalling a package that is no longer installed is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudWafDomainPackageIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.UninstallArvanCloudWafPackageInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"uninstalled": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}
