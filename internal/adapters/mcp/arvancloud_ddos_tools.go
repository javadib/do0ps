package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud DDoS Protection tools (issue #67): a per-domain challenge engine
// (cookie/JavaScript/CAPTCHA-based). All fast operations (AGENTS.md 4.3):
// every tool below returns its result within the call, with no operation_id
// to poll afterward.
//
// Naming collision warning, repeated on every tool description below, same
// shape as the WAF/Firewall warning: DDoS here means ArvanCloud's per-domain
// challenge engine — completely distinct from both the CDN edge Firewall's
// hand-written filter_expr rules (create_arvancloud_firewall_rule and
// friends) and the WAF managed rule-set engine (create_arvancloud_waf_rule
// and friends), even though DDoS rules use the same "protect"/"passthrough"
// action spelling as WAF's custom rules.
const arvanCloudDdosVsWafFirewallNote = "DDoS Protection here means ArvanCloud's per-domain challenge engine (cookie/JavaScript/CAPTCHA-based) — completely distinct from the CDN edge Firewall's hand-written filter_expr rules (create_arvancloud_firewall_rule and friends) and the WAF managed rule-set engine (create_arvancloud_waf_rule and friends)."

// arvanCloudDdosPreflightToMap renders a domain.ArvanCloudDdosPreflight.
func arvanCloudDdosPreflightToMap(p domain.ArvanCloudDdosPreflight) map[string]any {
	return map[string]any{
		"access_origin":         p.AccessOrigin,
		"access_credentials":    p.AccessCredentials,
		"access_methods":        p.AccessMethods,
		"access_headers":        p.AccessHeaders,
		"access_expose_headers": p.AccessExposeHeaders,
	}
}

// arvanCloudDdosSettingsToMap renders a domain.ArvanCloudDdosSettings the way
// get/update_arvancloud_ddos_settings report it back to the caller.
// secret_key is included because the provider's own GET response already
// returns it in cleartext to the account that owns it (the same caller
// supplying it on every call, per AGENTS.md 4.2) — the sensitivity this
// package guards against is a log line or error message leaking it to
// somewhere OTHER than the caller who already holds it, not the tool
// response itself. See ddos.go's package comment and ddos_test.go's
// TestUpdateArvanCloudDdosSettingsNeverLogsSecretKey.
func arvanCloudDdosSettingsToMap(s domain.ArvanCloudDdosSettings) map[string]any {
	return map[string]any{
		"is_enabled":      s.IsEnabled,
		"protection_mode": string(s.ProtectionMode),
		"captcha_service": string(s.CaptchaService),
		"site_key":        s.SiteKey,
		"secret_key":      s.SecretKey,
		"ttl":             s.TTL,
		"https_only":      s.HTTPSOnly,
		"preflight":       arvanCloudDdosPreflightToMap(s.Preflight),
	}
}

// arvanCloudDdosRuleToMap renders a domain.ArvanCloudDdosRule the way every
// DDoS-rule-returning tool reports it back to the caller.
func arvanCloudDdosRuleToMap(r domain.ArvanCloudDdosRule) map[string]any {
	return map[string]any{
		"id":          r.ID,
		"url_pattern": r.URLPattern,
		"sources":     r.Sources,
		"description": r.Description,
		"action":      string(r.Action),
		"is_enabled":  r.IsEnabled,
	}
}

// --- Per-domain DDoS settings ------------------------------------------------

func getArvanCloudDdosSettingsTool(uc *app.GetArvanCloudDdosSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_ddos_settings",
		Description: "Get a domain's DDoS protection configuration: whether it is enabled, its challenge mechanism " +
			"(off/cookie/javascript/captcha), CAPTCHA vendor configuration when applicable, cookie TTL and CORS " +
			"preflight settings. " + arvanCloudDdosVsWafFirewallNote + " This is a fast operation.",
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

			found, err := uc.Execute(ctx, app.GetArvanCloudDdosSettingsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudDdosSettingsToMap(*found), nil
		},
	}
}

func arvanCloudDdosProtectionModeProperty() map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudDdosProtectionModeOff),
			string(domain.ArvanCloudDdosProtectionModeCookie),
			string(domain.ArvanCloudDdosProtectionModeJavaScript),
			string(domain.ArvanCloudDdosProtectionModeCaptcha),
		},
		"description": "The domain's DDoS challenge mechanism: \"off\" disables DDoS protection entirely, " +
			"\"cookie\" challenges suspicious traffic with a cookie-based check, \"javascript\" with a " +
			"JavaScript-execution check, \"captcha\" with a CAPTCHA (requires captcha_service and, typically, " +
			"captcha_site_key/captcha_secret_key).",
	}
}

func arvanCloudDdosPreflightProperty() map[string]any {
	return map[string]any{
		"type": "object",
		"description": "The CORS-preflight response emitted alongside a DDoS challenge, so a browser-based API " +
			"client is not blocked by the challenge's own cross-origin response. Leave unset to keep ArvanCloud's " +
			"own defaults.",
		"properties": map[string]any{
			"access_origin":         map[string]any{"type": "string", "description": "Value for Access-Control-Allow-Origin."},
			"access_credentials":    map[string]any{"type": "string", "description": "Value for Access-Control-Allow-Credentials."},
			"access_methods":        map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}}, "description": "HTTP methods to allow."},
			"access_headers":        map[string]any{"type": "string", "description": "Value for Access-Control-Allow-Headers."},
			"access_expose_headers": map[string]any{"type": "string", "description": "Value for Access-Control-Expose-Headers."},
		},
	}
}

type arvanCloudDdosPreflightArgs struct {
	AccessOrigin        string   `json:"access_origin"`
	AccessCredentials   string   `json:"access_credentials"`
	AccessMethods       []string `json:"access_methods"`
	AccessHeaders       string   `json:"access_headers"`
	AccessExposeHeaders string   `json:"access_expose_headers"`
}

func (a *arvanCloudDdosPreflightArgs) toDomain() domain.ArvanCloudDdosPreflight {
	if a == nil {
		return domain.ArvanCloudDdosPreflight{}
	}
	return domain.ArvanCloudDdosPreflight{
		AccessOrigin:        a.AccessOrigin,
		AccessCredentials:   a.AccessCredentials,
		AccessMethods:       a.AccessMethods,
		AccessHeaders:       a.AccessHeaders,
		AccessExposeHeaders: a.AccessExposeHeaders,
	}
}

func updateArvanCloudDdosSettingsTool(uc *app.UpdateArvanCloudDdosSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["protection_mode"] = arvanCloudDdosProtectionModeProperty()
	props["captcha_service"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudCaptchaServiceRecaptcha), string(domain.ArvanCloudCaptchaServiceArcaptcha), string(domain.ArvanCloudCaptchaServiceHcaptcha)},
		"description": "The CAPTCHA vendor to challenge with. REQUIRED when protection_mode is \"captcha\"; ignored otherwise.",
	}
	props["captcha_site_key"] = map[string]any{
		"type": "string",
		"description": "The CAPTCHA provider's public site key, registered by the domain owner directly with the " +
			"CAPTCHA vendor (reCAPTCHA/hCAPTCHA/ArCaptcha) — this is NOT an ArvanCloud credential. Only meaningful " +
			"when protection_mode is \"captcha\".",
	}
	props["captcha_secret_key"] = map[string]any{
		"type": "string",
		"description": "The CAPTCHA provider's secret key, from the same reCAPTCHA/hCAPTCHA/ArCaptcha registration " +
			"as captcha_site_key — this is NOT an ArvanCloud credential (do not confuse it with this tool's own " +
			"api_key/secret_key). It is caller-supplied material passed straight through to ArvanCloud: never " +
			"generate, guess, or invent a value for this field — only send a value the caller (the domain owner) " +
			"actually provided. Only meaningful when protection_mode is \"captcha\".",
	}
	props["ttl"] = map[string]any{"type": "integer", "description": "Challenge cookie max-age, in seconds, e.g. 3600 for one hour."}
	props["https_only"] = map[string]any{"type": "boolean", "description": "Add \"SameSite=None; Secure\" to the challenge's set-cookie header."}
	props["preflight"] = arvanCloudDdosPreflightProperty()

	return Tool{
		Name: "update_arvancloud_ddos_settings",
		Description: "Update a domain's DDoS protection configuration (challenge mechanism, CAPTCHA vendor " +
			"configuration, cookie TTL, CORS preflight). " + arvanCloudDdosVsWafFirewallNote +
			" IMPORTANT: captcha_secret_key is caller-supplied CAPTCHA provider material (the domain owner's own " +
			"reCAPTCHA/hCAPTCHA/ArCaptcha secret) passed straight through to ArvanCloud — never generate, guess, " +
			"or invent this value; only send one the caller actually provided, and omit it entirely if they did " +
			"not give one. This is a fast operation: the updated settings are returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "protection_mode"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ProtectionMode   string                       `json:"protection_mode"`
				CaptchaService   string                       `json:"captcha_service"`
				CaptchaSiteKey   string                       `json:"captcha_site_key"`
				CaptchaSecretKey string                       `json:"captcha_secret_key"`
				TTL              int                          `json:"ttl"`
				HTTPSOnly        bool                         `json:"https_only"`
				Preflight        *arvanCloudDdosPreflightArgs `json:"preflight"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudDdosSettingsInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				Settings: domain.ArvanCloudDdosSettings{
					ProtectionMode: domain.ArvanCloudDdosProtectionMode(args.ProtectionMode),
					CaptchaService: domain.ArvanCloudCaptchaService(args.CaptchaService),
					SiteKey:        args.CaptchaSiteKey,
					SecretKey:      args.CaptchaSecretKey,
					TTL:            args.TTL,
					HTTPSOnly:      args.HTTPSOnly,
					Preflight:      args.Preflight.toDomain(),
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDdosSettingsToMap(*updated), nil
		},
	}
}

// --- Per-domain DDoS rules ----------------------------------------------------

// arvanCloudDdosRuleIDArgs is embedded by every DDoS-rule tool below that is
// scoped to exactly one rule by domain + id.
type arvanCloudDdosRuleIDArgs struct {
	arvanCloudDomainNameArgs
	ID string `json:"id"`
}

func arvanCloudDdosRuleIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The DDoS rule's provider-assigned ID (a UUID), as returned by create_arvancloud_ddos_rule or list_arvancloud_ddos_rules.",
	}
}

func arvanCloudDdosRuleActionProperty(description string) map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudDdosRuleActionProtect),
			string(domain.ArvanCloudDdosRuleActionPassthrough),
		},
		"description": description,
	}
}

func listArvanCloudDdosRulesTool(uc *app.ListArvanCloudDdosRules) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_ddos_rules",
		Description: "List every DDoS protection rule configured for a domain. " + arvanCloudDdosVsWafFirewallNote +
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

			rules, err := uc.Execute(ctx, app.ListArvanCloudDdosRulesInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, r := range rules {
				out[i] = arvanCloudDdosRuleToMap(r)
			}
			return map[string]any{"rules": out}, nil
		},
	}
}

func createArvanCloudDdosRuleTool(uc *app.CreateArvanCloudDdosRule) Tool {
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
	props["description"] = map[string]any{"type": "string", "description": "An optional note about what this rule is for."}
	props["action"] = arvanCloudDdosRuleActionProperty(
		"What happens to a request this rule matches: \"protect\" actively applies the domain's DDoS challenge, \"passthrough\" exempts the request from the DDoS challenge entirely.")
	props["is_enabled"] = map[string]any{"type": "boolean", "description": "Whether the rule is active. Defaults to true when omitted."}

	return Tool{
		Name: "create_arvancloud_ddos_rule",
		Description: "Create a new DDoS protection rule: a caller-authored exemption or enforcement layered on top " +
			"of a domain's DDoS challenge settings (e.g. \"exempt this source range from the DDoS challenge\"). " +
			arvanCloudDdosVsWafFirewallNote + " This is a fast operation: the created rule, including its " +
			"provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "url_pattern", "action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				URLPattern  string   `json:"url_pattern"`
				Sources     []string `json:"sources"`
				Description string   `json:"description"`
				Action      string   `json:"action"`
				IsEnabled   *bool    `json:"is_enabled"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			isEnabled := true
			if args.IsEnabled != nil {
				isEnabled = *args.IsEnabled
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudDdosRuleInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				Rule: domain.ArvanCloudDdosRule{
					URLPattern:  args.URLPattern,
					Sources:     args.Sources,
					Description: args.Description,
					Action:      domain.ArvanCloudDdosRuleAction(args.Action),
					IsEnabled:   isEnabled,
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDdosRuleToMap(*created), nil
		},
	}
}

func getArvanCloudDdosRuleTool(uc *app.GetArvanCloudDdosRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudDdosRuleIDProperty()

	return Tool{
		Name:        "get_arvancloud_ddos_rule",
		Description: "Get the current state of one DDoS protection rule by ID. " + arvanCloudDdosVsWafFirewallNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDdosRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudDdosRuleInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudDdosRuleToMap(*found), nil
		},
	}
}

func updateArvanCloudDdosRuleTool(uc *app.UpdateArvanCloudDdosRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudDdosRuleIDProperty()
	props["url_pattern"] = map[string]any{"type": "string", "description": "The rule's new glob pattern. See create_arvancloud_ddos_rule for the pattern syntax."}
	props["sources"] = map[string]any{
		"type": "array", "items": map[string]any{"type": "string"},
		"description": "The rule's new CIDR-format source addresses. At most 20 entries.",
	}
	props["description"] = map[string]any{"type": "string", "description": "The rule's new note."}
	props["action"] = arvanCloudDdosRuleActionProperty("The rule's new action. See create_arvancloud_ddos_rule for the full list.")
	props["is_enabled"] = map[string]any{"type": "boolean", "description": "Whether the rule is active."}

	return Tool{
		Name: "update_arvancloud_ddos_rule",
		Description: "Update a DDoS protection rule. This replaces the rule's fields with the given values — pass " +
			"every field you want to keep, not only the ones changing. " + arvanCloudDdosVsWafFirewallNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "url_pattern", "action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDdosRuleIDArgs
				URLPattern  string   `json:"url_pattern"`
				Sources     []string `json:"sources"`
				Description string   `json:"description"`
				Action      string   `json:"action"`
				IsEnabled   bool     `json:"is_enabled"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudDdosRuleInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				ID:          args.ID,
				Rule: domain.ArvanCloudDdosRule{
					URLPattern:  args.URLPattern,
					Sources:     args.Sources,
					Description: args.Description,
					Action:      domain.ArvanCloudDdosRuleAction(args.Action),
					IsEnabled:   args.IsEnabled,
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDdosRuleToMap(*updated), nil
		},
	}
}

func deleteArvanCloudDdosRuleTool(uc *app.DeleteArvanCloudDdosRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudDdosRuleIDProperty()

	return Tool{
		Name: "delete_arvancloud_ddos_rule",
		Description: "Permanently delete a DDoS protection rule by ID. " + arvanCloudDdosVsWafFirewallNote +
			" This is a fast operation and cannot be undone. Deleting a rule that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDdosRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudDdosRuleInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

func reprioritizeArvanCloudDdosRulesTool(uc *app.ReprioritizeArvanCloudDdosRules) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReprioritizeProperties(props)

	return Tool{
		Name: "reprioritize_arvancloud_ddos_rules",
		Description: "Change the evaluation order of a domain's DDoS protection rules by moving one rule relative " +
			"to another. " + arvanCloudDdosVsWafFirewallNote + " Give exactly one of after_rule_id/before_rule_id, " +
			"not both. This is a fast operation.",
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

			if err := uc.Execute(ctx, app.ReprioritizeArvanCloudDdosRulesInput{
				Credentials: args.domain(), Domain: args.Domain,
				RuleID: args.RuleID, AfterRuleID: args.AfterRuleID, BeforeRuleID: args.BeforeRuleID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"reprioritized": true, "domain": args.Domain, "rule_id": args.RuleID}, nil
		},
	}
}
