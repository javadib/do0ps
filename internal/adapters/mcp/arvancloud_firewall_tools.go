package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud Firewall tools (issue #65): the CDN edge-level L7 firewall,
// domain-level (/domains/{domain}/firewall/...) and account-level
// (/account/firewall-rules/...). All fast operations (AGENTS.md 4.3): every
// tool below returns its result within the call, with no operation_id to
// poll afterward.
//
// Naming collision warning (mirrors domain/arvancloud_firewall.go's own):
// this is ArvanCloud's CDN edge-level firewall, unrelated to any future
// ArvanCloud IaaS/cloud-server firewall tool set.

// arvanCloudFilterExprPrimer is repeated on every tool that accepts a
// filter_expr, so the calling chatbot has a concrete, self-contained
// reference for turning a natural-language request into a valid expression
// without needing to consult ArvanCloud's own docs mid-call.
const arvanCloudFilterExprPrimer = "filter_expr is a Wireshark-like boolean expression evaluated per-request at the edge: " +
	"field names are dotted paths into the request (ip.geoip.country, http.request.method, http.request.uri.path, " +
	"http.request.headers[\"...\"], ssl), operators include eq/ne/in/contains, string sets use {\"A\" \"B\"} syntax, and " +
	"clauses combine with and/or/not. Examples: block two countries entirely with " +
	"`ip.geoip.country in {\"KP\" \"RU\"}`; block POST requests to a specific admin endpoint with " +
	"`http.request.uri.path eq \"/wp-admin/admin-ajax.php\" and http.request.method eq \"POST\"`; challenge requests " +
	"that arrive with no User-Agent header (a common bot signature) with " +
	"`http.request.headers[\"user-agent\"] eq \"\"`. The expression itself is never validated by this server — an " +
	"invalid one is reported back as whatever error ArvanCloud's own API returns."

// arvanCloudFirewallActionDetailsArgs is the tool-argument shape of
// domain.ArvanCloudFirewallActionDetails, shared by every tool that accepts
// a rule's action or firewall settings' default_action.
type arvanCloudFirewallActionDetailsArgs struct {
	BypassRateLimit      bool `json:"bypass_rate_limit"`
	BypassChallengeCheck bool `json:"bypass_challenge_check"`
	BypassWAF            bool `json:"bypass_waf"`
	ChallengeMode        int  `json:"challenge_mode"`
	ChallengeTTL         int  `json:"challenge_ttl"`
	ChallengeHTTPSOnly   bool `json:"challenge_https_only"`
}

func (a arvanCloudFirewallActionDetailsArgs) toDomain() domain.ArvanCloudFirewallActionDetails {
	return domain.ArvanCloudFirewallActionDetails{
		BypassRateLimit:      a.BypassRateLimit,
		BypassChallengeCheck: a.BypassChallengeCheck,
		BypassWAF:            a.BypassWAF,
		ChallengeMode:        a.ChallengeMode,
		ChallengeTTL:         a.ChallengeTTL,
		ChallengeHTTPSOnly:   a.ChallengeHTTPSOnly,
	}
}

// arvanCloudFirewallActionDetailsProperty is the JSON Schema for the
// optional action_details object, meaningful only when the sibling
// action/default_action argument is "bypass" or "challenge".
func arvanCloudFirewallActionDetailsProperty() map[string]any {
	return map[string]any{
		"type": "object",
		"description": "Extra configuration for a \"bypass\" or \"challenge\" action/default_action. Ignored for " +
			"\"allow\"/\"deny\"/\"drop\". Only the fields relevant to the chosen action need to be set.",
		"properties": map[string]any{
			"bypass_rate_limit": map[string]any{
				"type": "boolean", "description": "Bypass action only: skip rate limiting for matching requests.",
			},
			"bypass_challenge_check": map[string]any{
				"type": "boolean", "description": "Bypass action only: skip any active challenge for matching requests.",
			},
			"bypass_waf": map[string]any{
				"type": "boolean", "description": "Bypass action only: skip WAF inspection for matching requests.",
			},
			"challenge_mode": map[string]any{
				"type": "integer", "enum": []int{1, 2, 3},
				"description": "Challenge action only: how the challenge is presented — 1 cookie, 2 javascript, 3 captcha.",
			},
			"challenge_ttl": map[string]any{
				"type": "integer",
				"description": "Challenge action only: how long, in seconds (10-31536000), a passed challenge is " +
					"remembered before it is asked again, e.g. 3600 for one hour.",
			},
			"challenge_https_only": map[string]any{
				"type": "boolean", "description": "Challenge action only: only issue the challenge over HTTPS.",
			},
		},
	}
}

// arvanCloudFirewallActionDetailsFromArgs decodes an optional
// action_details object from raw tool args, returning the zero value when
// the caller omitted it entirely.
func arvanCloudFirewallActionDetailsFromArgs(raw *arvanCloudFirewallActionDetailsArgs) domain.ArvanCloudFirewallActionDetails {
	if raw == nil {
		return domain.ArvanCloudFirewallActionDetails{}
	}
	return raw.toDomain()
}

// arvanCloudFirewallActionDetailsToMap renders
// domain.ArvanCloudFirewallActionDetails the way every rule/settings tool
// reports it back to the caller.
func arvanCloudFirewallActionDetailsToMap(d domain.ArvanCloudFirewallActionDetails) map[string]any {
	return map[string]any{
		"bypass_rate_limit":      d.BypassRateLimit,
		"bypass_challenge_check": d.BypassChallengeCheck,
		"bypass_waf":             d.BypassWAF,
		"challenge_mode":         d.ChallengeMode,
		"challenge_ttl":          d.ChallengeTTL,
		"challenge_https_only":   d.ChallengeHTTPSOnly,
	}
}

func arvanCloudFirewallActionProperty(description string) map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudFirewallActionAllow),
			string(domain.ArvanCloudFirewallActionDeny),
			string(domain.ArvanCloudFirewallActionBypass),
			string(domain.ArvanCloudFirewallActionChallenge),
		},
		"description": description,
	}
}

func arvanCloudFirewallDefaultActionProperty() map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudFirewallDefaultActionAllow),
			string(domain.ArvanCloudFirewallDefaultActionDeny),
			string(domain.ArvanCloudFirewallDefaultActionDrop),
			string(domain.ArvanCloudFirewallDefaultActionBypass),
			string(domain.ArvanCloudFirewallDefaultActionChallenge),
		},
		"description": "The action applied to a request no rule matched. Unlike a per-rule action, \"drop\" is " +
			"allowed here: it silently discards the request without even the response a \"deny\" gives.",
	}
}

// arvanCloudFirewallRuleToMap renders a domain.ArvanCloudFirewallRule the
// way every domain-level rule-returning tool reports it back to the caller.
func arvanCloudFirewallRuleToMap(r domain.ArvanCloudFirewallRule) map[string]any {
	return map[string]any{
		"id":               r.ID,
		"name":             r.Name,
		"filter_expr":      r.FilterExpr,
		"action":           string(r.Action),
		"priority":         r.Priority,
		"is_enabled":       r.IsEnabled,
		"note":             r.Note,
		"is_account_level": r.IsAccountLevel,
		"action_details":   arvanCloudFirewallActionDetailsToMap(r.ActionDetails),
	}
}

// arvanCloudFirewallSettingsToMap renders a
// domain.ArvanCloudFirewallSettings the way every settings-returning tool
// reports it back to the caller.
func arvanCloudFirewallSettingsToMap(s domain.ArvanCloudFirewallSettings) map[string]any {
	return map[string]any{
		"is_enabled":             s.IsEnabled,
		"default_action":         string(s.DefaultAction),
		"verify_sni":             s.VerifySNI,
		"skip_global_whitelist":  s.SkipGlobalWhitelist,
		"skip_global_firewall":   s.SkipGlobalFirewall,
		"default_action_details": arvanCloudFirewallActionDetailsToMap(s.DefaultActionDetails),
	}
}

// arvanCloudAccountFirewallRuleToMap renders a
// domain.ArvanCloudAccountFirewallRule the way every account-level
// rule-returning tool reports it back to the caller.
func arvanCloudAccountFirewallRuleToMap(r domain.ArvanCloudAccountFirewallRule) map[string]any {
	return map[string]any{
		"id":                    r.ID,
		"name":                  r.Name,
		"filter_expr":           r.FilterExpr,
		"action":                string(r.Action),
		"priority":              r.Priority,
		"is_enabled":            r.IsEnabled,
		"note":                  r.Note,
		"is_account_level":      r.IsAccountLevel,
		"action_details":        arvanCloudFirewallActionDetailsToMap(r.ActionDetails),
		"domain_selection_type": string(r.DomainSelectionType),
		"domain_ids":            r.DomainIDs,
	}
}

// arvanCloudFirewallRuleIDArgs is embedded by every domain-level rule tool
// below that is scoped to exactly one rule by domain + id.
type arvanCloudFirewallRuleIDArgs struct {
	arvanCloudDomainNameArgs
	ID string `json:"id"`
}

func arvanCloudFirewallRuleIDProperty() map[string]any {
	return map[string]any{
		"type": "string",
		"description": "The rule's provider-assigned ID (a UUID), as returned by create_arvancloud_firewall_rule or " +
			"list_arvancloud_firewall_rules.",
	}
}

// --- Firewall Settings (domain-level) --------------------------------------

func getArvanCloudFirewallSettingsTool(uc *app.GetArvanCloudFirewallSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_firewall_settings",
		Description: "Get a domain's CDN edge-level firewall configuration: whether the firewall is enabled, the " +
			"default_action applied to a request no rule matched, and a few firewall-wide toggles. This is a fast " +
			"operation.",
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

			found, err := uc.Execute(ctx, app.GetArvanCloudFirewallSettingsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudFirewallSettingsToMap(*found), nil
		},
	}
}

func updateArvanCloudFirewallSettingsTool(uc *app.UpdateArvanCloudFirewallSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["default_action"] = arvanCloudFirewallDefaultActionProperty()
	props["verify_sni"] = map[string]any{
		"type":        "boolean",
		"description": "Require a TLS request's SNI and Host header to match. ArvanCloud defaults this to true.",
	}
	props["skip_global_whitelist"] = map[string]any{
		"type":        "boolean",
		"description": "Skip ArvanCloud's own global whitelist for this domain.",
	}
	props["skip_global_firewall"] = map[string]any{
		"type":        "boolean",
		"description": "Skip ArvanCloud's own global firewall rules for this domain.",
	}
	props["default_action_details"] = arvanCloudFirewallActionDetailsProperty()

	return Tool{
		Name: "update_arvancloud_firewall_settings",
		Description: "Update a domain's CDN edge-level firewall configuration. This is a fast operation: the " +
			"updated settings are returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "default_action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				DefaultAction        string                               `json:"default_action"`
				VerifySNI            bool                                 `json:"verify_sni"`
				SkipGlobalWhitelist  bool                                 `json:"skip_global_whitelist"`
				SkipGlobalFirewall   bool                                 `json:"skip_global_firewall"`
				DefaultActionDetails *arvanCloudFirewallActionDetailsArgs `json:"default_action_details"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudFirewallSettingsInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				Settings: domain.ArvanCloudFirewallSettings{
					DefaultAction:        domain.ArvanCloudFirewallDefaultAction(args.DefaultAction),
					VerifySNI:            args.VerifySNI,
					SkipGlobalWhitelist:  args.SkipGlobalWhitelist,
					SkipGlobalFirewall:   args.SkipGlobalFirewall,
					DefaultActionDetails: arvanCloudFirewallActionDetailsFromArgs(args.DefaultActionDetails),
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudFirewallSettingsToMap(*updated), nil
		},
	}
}

// --- Firewall Rules (domain-level) -----------------------------------------

func listArvanCloudFirewallRulesTool(uc *app.ListArvanCloudFirewallRules) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_firewall_rules",
		Description: "List every domain-level CDN edge firewall rule configured for a domain, in priority order. " +
			"This is a fast operation.",
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

			rules, err := uc.Execute(ctx, app.ListArvanCloudFirewallRulesInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, r := range rules {
				out[i] = arvanCloudFirewallRuleToMap(r)
			}
			return map[string]any{"rules": out}, nil
		},
	}
}

func createArvanCloudFirewallRuleTool(uc *app.CreateArvanCloudFirewallRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "A label for the new rule, e.g. \"block-known-scanners\".",
	}
	props["filter_expr"] = map[string]any{
		"type":        "string",
		"description": arvanCloudFilterExprPrimer,
	}
	props["action"] = arvanCloudFirewallActionProperty(
		"What happens to a request this rule matches: \"allow\" lets it through, \"deny\" blocks it, \"bypass\" " +
			"skips other protections for it (see action_details), \"challenge\" makes the client prove it is human " +
			"(see action_details).")
	props["priority"] = map[string]any{
		"type":        "integer",
		"description": "Evaluation order among this domain's rules, lower first, e.g. 1 for \"check this before anything else\". Assigned by ArvanCloud when omitted.",
	}
	props["is_enabled"] = map[string]any{
		"type":        "boolean",
		"description": "Whether the rule is active. Defaults to true when omitted.",
	}
	props["note"] = map[string]any{
		"type":        "string",
		"description": "An optional comment about what this rule is for.",
	}
	props["action_details"] = arvanCloudFirewallActionDetailsProperty()

	return Tool{
		Name: "create_arvancloud_firewall_rule",
		Description: "Create a new domain-level CDN edge firewall rule. " + arvanCloudFilterExprPrimer +
			" This is a fast operation: the created rule, including its provider-assigned ID and priority, is " +
			"returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "name", "filter_expr", "action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Name          string                               `json:"name"`
				FilterExpr    string                               `json:"filter_expr"`
				Action        string                               `json:"action"`
				Priority      int                                  `json:"priority"`
				IsEnabled     *bool                                `json:"is_enabled"`
				Note          string                               `json:"note"`
				ActionDetails *arvanCloudFirewallActionDetailsArgs `json:"action_details"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			isEnabled := true
			if args.IsEnabled != nil {
				isEnabled = *args.IsEnabled
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudFirewallRuleInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				Rule: domain.ArvanCloudFirewallRule{
					Name:          args.Name,
					FilterExpr:    args.FilterExpr,
					Action:        domain.ArvanCloudFirewallAction(args.Action),
					Priority:      args.Priority,
					IsEnabled:     isEnabled,
					Note:          args.Note,
					ActionDetails: arvanCloudFirewallActionDetailsFromArgs(args.ActionDetails),
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudFirewallRuleToMap(*created), nil
		},
	}
}

func getArvanCloudFirewallRuleTool(uc *app.GetArvanCloudFirewallRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudFirewallRuleIDProperty()

	return Tool{
		Name:        "get_arvancloud_firewall_rule",
		Description: "Get the current state of one domain-level CDN edge firewall rule by ID. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudFirewallRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudFirewallRuleInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudFirewallRuleToMap(*found), nil
		},
	}
}

func updateArvanCloudFirewallRuleTool(uc *app.UpdateArvanCloudFirewallRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudFirewallRuleIDProperty()
	props["name"] = map[string]any{"type": "string", "description": "The rule's new label."}
	props["filter_expr"] = map[string]any{
		"type":        "string",
		"description": arvanCloudFilterExprPrimer,
	}
	props["action"] = arvanCloudFirewallActionProperty("The rule's new action. See create_arvancloud_firewall_rule for the full list.")
	props["priority"] = map[string]any{
		"type":        "integer",
		"description": "The rule's new evaluation order, lower first. Prefer reprioritize_arvancloud_firewall_rules to reorder relative to another rule.",
	}
	props["is_enabled"] = map[string]any{"type": "boolean", "description": "Whether the rule is active."}
	props["note"] = map[string]any{"type": "string", "description": "The rule's new comment."}
	props["action_details"] = arvanCloudFirewallActionDetailsProperty()

	return Tool{
		Name: "update_arvancloud_firewall_rule",
		Description: "Update a domain-level CDN edge firewall rule. This replaces the rule's fields with the given " +
			"values — pass every field you want to keep, not only the ones changing. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "name", "filter_expr", "action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudFirewallRuleIDArgs
				Name          string                               `json:"name"`
				FilterExpr    string                               `json:"filter_expr"`
				Action        string                               `json:"action"`
				Priority      int                                  `json:"priority"`
				IsEnabled     bool                                 `json:"is_enabled"`
				Note          string                               `json:"note"`
				ActionDetails *arvanCloudFirewallActionDetailsArgs `json:"action_details"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudFirewallRuleInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				ID:          args.ID,
				Rule: domain.ArvanCloudFirewallRule{
					Name:          args.Name,
					FilterExpr:    args.FilterExpr,
					Action:        domain.ArvanCloudFirewallAction(args.Action),
					Priority:      args.Priority,
					IsEnabled:     args.IsEnabled,
					Note:          args.Note,
					ActionDetails: arvanCloudFirewallActionDetailsFromArgs(args.ActionDetails),
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudFirewallRuleToMap(*updated), nil
		},
	}
}

func deleteArvanCloudFirewallRuleTool(uc *app.DeleteArvanCloudFirewallRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudFirewallRuleIDProperty()

	return Tool{
		Name: "delete_arvancloud_firewall_rule",
		Description: "Permanently delete a domain-level CDN edge firewall rule by ID. This is a fast operation and " +
			"cannot be undone. Deleting a rule that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudFirewallRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudFirewallRuleInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

// arvanCloudReprioritizeProperties adds the shared rule_id/after_rule_id/
// before_rule_id trio to props, used by both the domain-level and
// account-level reprioritize tools.
func arvanCloudReprioritizeProperties(props map[string]any) {
	props["rule_id"] = map[string]any{
		"type":        "string",
		"description": "The ID of the rule to move.",
	}
	props["after_rule_id"] = map[string]any{
		"type":        "string",
		"description": "Move rule_id to just after this rule (higher priority number, evaluated later). Give only one of after_rule_id/before_rule_id.",
	}
	props["before_rule_id"] = map[string]any{
		"type":        "string",
		"description": "Move rule_id to just before this rule (lower priority number, evaluated earlier). Give only one of after_rule_id/before_rule_id.",
	}
}

func reprioritizeArvanCloudFirewallRulesTool(uc *app.ReprioritizeArvanCloudFirewallRules) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReprioritizeProperties(props)

	return Tool{
		Name: "reprioritize_arvancloud_firewall_rules",
		Description: "Change the evaluation order of a domain's firewall rules by moving one rule relative to " +
			"another. Give exactly one of after_rule_id/before_rule_id, not both. This is a fast operation.",
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

			if err := uc.Execute(ctx, app.ReprioritizeArvanCloudFirewallRulesInput{
				Credentials: args.domain(), Domain: args.Domain,
				RuleID: args.RuleID, AfterRuleID: args.AfterRuleID, BeforeRuleID: args.BeforeRuleID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"reprioritized": true, "domain": args.Domain, "rule_id": args.RuleID}, nil
		},
	}
}

// --- Firewall Rules (account-level) -----------------------------------------

func listArvanCloudAccountFirewallValidDomainsTool(uc *app.ListArvanCloudAccountFirewallValidDomains) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_arvancloud_account_firewall_valid_domains",
		Description: "List the account's active enterprise domains that are eligible to be targeted by an " +
			"account-level firewall rule's domain_ids (via create_arvancloud_account_firewall_rule, " +
			"attach_arvancloud_account_firewall_domains or detach_arvancloud_account_firewall_domains). This is a " +
			"fast operation.",
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

			domains, err := uc.Execute(ctx, app.ListArvanCloudAccountFirewallValidDomainsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(domains))
			for i, d := range domains {
				out[i] = map[string]any{"id": d.ID, "domain": d.Name}
			}
			return map[string]any{"domains": out}, nil
		},
	}
}

func listArvanCloudAccountFirewallRulesTool(uc *app.ListArvanCloudAccountFirewallRules) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_arvancloud_account_firewall_rules",
		Description: "List every account-level CDN edge firewall rule — rules that apply across a chosen subset of " +
			"the account's domains rather than one domain. This is a fast operation.",
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

			rules, err := uc.Execute(ctx, app.ListArvanCloudAccountFirewallRulesInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, r := range rules {
				out[i] = arvanCloudAccountFirewallRuleToMap(r)
			}
			return map[string]any{"rules": out}, nil
		},
	}
}

// arvanCloudDomainSelectionTypeProperty is the JSON Schema for an
// account-level rule's domain_selection_type, shared by create and update.
func arvanCloudDomainSelectionTypeProperty() map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudDomainSelectionAll),
			string(domain.ArvanCloudDomainSelectionInclude),
			string(domain.ArvanCloudDomainSelectionExclude),
		},
		"description": "Which of the account's domains this rule applies to: \"all\" every enterprise domain on the " +
			"account (domain_ids not needed), \"include\" only the domains listed in domain_ids, \"exclude\" every " +
			"enterprise domain except those listed in domain_ids. domain_ids must be non-empty for \"include\" or " +
			"\"exclude\" — call list_arvancloud_account_firewall_valid_domains first to find domain IDs.",
	}
}

func arvanCloudAccountFirewallDomainIDsProperty(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": description,
	}
}

func createArvanCloudAccountFirewallRuleTool(uc *app.CreateArvanCloudAccountFirewallRule) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "A label for the new rule, e.g. \"block-known-scanners-everywhere\".",
	}
	props["filter_expr"] = map[string]any{
		"type":        "string",
		"description": arvanCloudFilterExprPrimer,
	}
	props["action"] = arvanCloudFirewallActionProperty(
		"What happens to a request this rule matches. See create_arvancloud_firewall_rule for the full list of values.")
	props["priority"] = map[string]any{
		"type":        "integer",
		"description": "Evaluation order among account-level rules, lower first. Assigned by ArvanCloud when omitted.",
	}
	props["is_enabled"] = map[string]any{
		"type":        "boolean",
		"description": "Whether the rule is active. Defaults to true when omitted.",
	}
	props["note"] = map[string]any{
		"type":        "string",
		"description": "An optional comment about what this rule is for.",
	}
	props["action_details"] = arvanCloudFirewallActionDetailsProperty()
	props["domain_selection_type"] = arvanCloudDomainSelectionTypeProperty()
	props["domain_ids"] = arvanCloudAccountFirewallDomainIDsProperty(
		"The domain IDs this rule targets or excludes, from list_arvancloud_account_firewall_valid_domains. " +
			"Required (non-empty) when domain_selection_type is \"include\" or \"exclude\"; ignored for \"all\".")

	return Tool{
		Name: "create_arvancloud_account_firewall_rule",
		Description: "Create a new account-level CDN edge firewall rule, applied across a chosen subset of the " +
			"account's domains instead of one domain. " + arvanCloudFilterExprPrimer + " This is a fast operation: " +
			"the created rule, including its provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name", "filter_expr", "action", "domain_selection_type"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				Name                string                               `json:"name"`
				FilterExpr          string                               `json:"filter_expr"`
				Action              string                               `json:"action"`
				Priority            int                                  `json:"priority"`
				IsEnabled           *bool                                `json:"is_enabled"`
				Note                string                               `json:"note"`
				ActionDetails       *arvanCloudFirewallActionDetailsArgs `json:"action_details"`
				DomainSelectionType string                               `json:"domain_selection_type"`
				DomainIDs           []string                             `json:"domain_ids"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			isEnabled := true
			if args.IsEnabled != nil {
				isEnabled = *args.IsEnabled
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudAccountFirewallRuleInput{
				Credentials: args.domain(),
				Rule: domain.ArvanCloudAccountFirewallRule{
					Name:                args.Name,
					FilterExpr:          args.FilterExpr,
					Action:              domain.ArvanCloudFirewallAction(args.Action),
					Priority:            args.Priority,
					IsEnabled:           isEnabled,
					Note:                args.Note,
					ActionDetails:       arvanCloudFirewallActionDetailsFromArgs(args.ActionDetails),
					DomainSelectionType: domain.ArvanCloudDomainSelectionType(args.DomainSelectionType),
					DomainIDs:           args.DomainIDs,
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudAccountFirewallRuleToMap(*created), nil
		},
	}
}

// arvanCloudAccountFirewallRuleIDArgs is embedded by every account-level
// rule tool below that is scoped to exactly one rule by id and needs
// nothing else.
type arvanCloudAccountFirewallRuleIDArgs struct {
	credentialArgs
	ID string `json:"id"`
}

func arvanCloudAccountFirewallRuleIDProperty() map[string]any {
	return map[string]any{
		"type": "string",
		"description": "The rule's provider-assigned ID (a UUID), as returned by create_arvancloud_account_firewall_rule " +
			"or list_arvancloud_account_firewall_rules.",
	}
}

func getArvanCloudAccountFirewallRuleTool(uc *app.GetArvanCloudAccountFirewallRule) Tool {
	props := credentialProperties()
	props["id"] = arvanCloudAccountFirewallRuleIDProperty()

	return Tool{
		Name:        "get_arvancloud_account_firewall_rule",
		Description: "Get the current state of one account-level CDN edge firewall rule by ID. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudAccountFirewallRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudAccountFirewallRuleInput{Credentials: args.domain(), ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudAccountFirewallRuleToMap(*found), nil
		},
	}
}

func updateArvanCloudAccountFirewallRuleTool(uc *app.UpdateArvanCloudAccountFirewallRule) Tool {
	props := credentialProperties()
	props["id"] = arvanCloudAccountFirewallRuleIDProperty()
	props["name"] = map[string]any{"type": "string", "description": "The rule's new label."}
	props["filter_expr"] = map[string]any{
		"type":        "string",
		"description": arvanCloudFilterExprPrimer,
	}
	props["action"] = arvanCloudFirewallActionProperty("The rule's new action. See create_arvancloud_firewall_rule for the full list.")
	props["priority"] = map[string]any{
		"type":        "integer",
		"description": "The rule's new evaluation order, lower first. Prefer reprioritize_arvancloud_account_firewall_rules to reorder relative to another rule.",
	}
	props["is_enabled"] = map[string]any{"type": "boolean", "description": "Whether the rule is active."}
	props["note"] = map[string]any{"type": "string", "description": "The rule's new comment."}
	props["action_details"] = arvanCloudFirewallActionDetailsProperty()
	props["domain_selection_type"] = arvanCloudDomainSelectionTypeProperty()
	props["domain_ids"] = arvanCloudAccountFirewallDomainIDsProperty(
		"The rule's new target domain IDs. Required (non-empty) when domain_selection_type is \"include\" or \"exclude\".")

	return Tool{
		Name: "update_arvancloud_account_firewall_rule",
		Description: "Update an account-level CDN edge firewall rule. This replaces the rule's fields with the " +
			"given values — pass every field you want to keep, not only the ones changing. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "id", "name", "filter_expr", "action", "domain_selection_type"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudAccountFirewallRuleIDArgs
				Name                string                               `json:"name"`
				FilterExpr          string                               `json:"filter_expr"`
				Action              string                               `json:"action"`
				Priority            int                                  `json:"priority"`
				IsEnabled           bool                                 `json:"is_enabled"`
				Note                string                               `json:"note"`
				ActionDetails       *arvanCloudFirewallActionDetailsArgs `json:"action_details"`
				DomainSelectionType string                               `json:"domain_selection_type"`
				DomainIDs           []string                             `json:"domain_ids"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudAccountFirewallRuleInput{
				Credentials: args.domain(),
				ID:          args.ID,
				Rule: domain.ArvanCloudAccountFirewallRule{
					Name:                args.Name,
					FilterExpr:          args.FilterExpr,
					Action:              domain.ArvanCloudFirewallAction(args.Action),
					Priority:            args.Priority,
					IsEnabled:           args.IsEnabled,
					Note:                args.Note,
					ActionDetails:       arvanCloudFirewallActionDetailsFromArgs(args.ActionDetails),
					DomainSelectionType: domain.ArvanCloudDomainSelectionType(args.DomainSelectionType),
					DomainIDs:           args.DomainIDs,
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudAccountFirewallRuleToMap(*updated), nil
		},
	}
}

func deleteArvanCloudAccountFirewallRuleTool(uc *app.DeleteArvanCloudAccountFirewallRule) Tool {
	props := credentialProperties()
	props["id"] = arvanCloudAccountFirewallRuleIDProperty()

	return Tool{
		Name: "delete_arvancloud_account_firewall_rule",
		Description: "Permanently delete an account-level CDN edge firewall rule by ID. This is a fast operation " +
			"and cannot be undone. Deleting a rule that no longer exists is treated as already done rather than an " +
			"error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudAccountFirewallRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudAccountFirewallRuleInput{Credentials: args.domain(), ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "id": args.ID}, nil
		},
	}
}

func attachArvanCloudAccountFirewallDomainsTool(uc *app.AttachArvanCloudAccountFirewallDomains) Tool {
	props := credentialProperties()
	props["id"] = arvanCloudAccountFirewallRuleIDProperty()
	props["domain_ids"] = arvanCloudAccountFirewallDomainIDsProperty(
		"One or more domain IDs to add to this rule's target set, from list_arvancloud_account_firewall_valid_domains.")

	return Tool{
		Name: "attach_arvancloud_account_firewall_domains",
		Description: "Add one or more domains to an \"include\"/\"exclude\" account-level firewall rule's target " +
			"set, without resubmitting the whole rule. This is a fast operation: the rule as stored afterward is " +
			"returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "id", "domain_ids"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudAccountFirewallRuleIDArgs
				DomainIDs []string `json:"domain_ids"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.AttachArvanCloudAccountFirewallDomainsInput{
				Credentials: args.domain(), ID: args.ID, DomainIDs: args.DomainIDs,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudAccountFirewallRuleToMap(*updated), nil
		},
	}
}

func detachArvanCloudAccountFirewallDomainsTool(uc *app.DetachArvanCloudAccountFirewallDomains) Tool {
	props := credentialProperties()
	props["id"] = arvanCloudAccountFirewallRuleIDProperty()
	props["domain_ids"] = arvanCloudAccountFirewallDomainIDsProperty(
		"One or more domain IDs to remove from this rule's target set.")

	return Tool{
		Name: "detach_arvancloud_account_firewall_domains",
		Description: "Remove one or more domains from an \"include\"/\"exclude\" account-level firewall rule's " +
			"target set, without resubmitting the whole rule. This is a fast operation: the rule as stored " +
			"afterward is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "id", "domain_ids"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudAccountFirewallRuleIDArgs
				DomainIDs []string `json:"domain_ids"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.DetachArvanCloudAccountFirewallDomainsInput{
				Credentials: args.domain(), ID: args.ID, DomainIDs: args.DomainIDs,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudAccountFirewallRuleToMap(*updated), nil
		},
	}
}

func reprioritizeArvanCloudAccountFirewallRulesTool(uc *app.ReprioritizeArvanCloudAccountFirewallRules) Tool {
	props := credentialProperties()
	arvanCloudReprioritizeProperties(props)

	return Tool{
		Name: "reprioritize_arvancloud_account_firewall_rules",
		Description: "Change the evaluation order of account-level firewall rules by moving one rule relative to " +
			"another. Give exactly one of after_rule_id/before_rule_id, not both. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				RuleID       string `json:"rule_id"`
				AfterRuleID  string `json:"after_rule_id"`
				BeforeRuleID string `json:"before_rule_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.ReprioritizeArvanCloudAccountFirewallRulesInput{
				Credentials: args.domain(), RuleID: args.RuleID, AfterRuleID: args.AfterRuleID, BeforeRuleID: args.BeforeRuleID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"reprioritized": true, "rule_id": args.RuleID}, nil
		},
	}
}
