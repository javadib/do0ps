package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud Rate Limiting tools (issue #68): per-domain request-rate
// settings plus a rule engine that throttles or blocks traffic exceeding a
// configured rate. All fast operations (AGENTS.md 4.3): every tool below
// returns its result within the call, with no operation_id to poll
// afterward.
const arvanCloudRateLimitVsDdosNote = "Rate Limiting's ddos_detection setting is automatic rate-based DDoS " +
	"detection — distinct from ArvanCloud's dedicated DDoS Protection module (get/update_arvancloud_ddos_settings " +
	"and friends), which has its own, independent challenge engine and on/off switch."

// arvanCloudChallengeActionToMap renders a domain.ArvanCloudChallengeAction.
func arvanCloudChallengeActionToMap(a domain.ArvanCloudChallengeAction) map[string]any {
	return map[string]any{
		"mode":       int(a.Mode),
		"ttl":        a.TTL,
		"https_only": a.HTTPSOnly,
	}
}

// arvanCloudRateLimitSettingsToMap renders a
// domain.ArvanCloudRateLimitSettings the way
// get/update_arvancloud_rate_limit_settings report it back to the caller.
func arvanCloudRateLimitSettingsToMap(s domain.ArvanCloudRateLimitSettings) map[string]any {
	return map[string]any{
		"ddos_detection":  s.DDoSDetection,
		"exclude_sources": s.ExcludeSources,
	}
}

// arvanCloudRateLimitRuleToMap renders a domain.ArvanCloudRateLimitRule the
// way every rate-limit-rule-returning tool reports it back to the caller.
func arvanCloudRateLimitRuleToMap(r domain.ArvanCloudRateLimitRule) map[string]any {
	return map[string]any{
		"id":              r.ID,
		"action":          string(r.Action),
		"is_enabled":      r.IsEnabled,
		"url_pattern":     r.URLPattern,
		"description":     r.Description,
		"exclude_sources": r.ExcludeSources,
		"rate":            r.Rate,
		"burst":           r.Burst,
		"block_duration":  r.BlockDuration,
		"time_duration":   r.TimeDuration,
		"allowed_methods": r.AllowedMethods,
		"action_details":  arvanCloudChallengeActionToMap(r.ActionDetails),
	}
}

// --- Per-domain rate-limit settings ----------------------------------------

func getArvanCloudRateLimitSettingsTool(uc *app.GetArvanCloudRateLimitSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_rate_limit_settings",
		Description: "Get a domain's rate-limiting configuration: whether automatic rate-based DDoS detection is " +
			"on, and which source addresses are globally exempted from every rate-limit rule. " +
			arvanCloudRateLimitVsDdosNote + " This is a fast operation.",
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

			found, err := uc.Execute(ctx, app.GetArvanCloudRateLimitSettingsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudRateLimitSettingsToMap(*found), nil
		},
	}
}

func updateArvanCloudRateLimitSettingsTool(uc *app.UpdateArvanCloudRateLimitSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["ddos_detection"] = map[string]any{
		"type":        "boolean",
		"description": "Turn on automatic rate-based DDoS detection. " + arvanCloudRateLimitVsDdosNote,
	}
	props["exclude_sources"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "CIDR-format source addresses globally exempted from every rate-limit rule on the domain, e.g. \"203.0.113.0/24\".",
	}

	return Tool{
		Name: "update_arvancloud_rate_limit_settings",
		Description: "Update a domain's rate-limiting configuration (automatic rate-based DDoS detection, globally " +
			"exempted sources). " + arvanCloudRateLimitVsDdosNote + " This is a fast operation: the updated " +
			"settings are returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				DDoSDetection  bool     `json:"ddos_detection"`
				ExcludeSources []string `json:"exclude_sources"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudRateLimitSettingsInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				Settings: domain.ArvanCloudRateLimitSettings{
					DDoSDetection:  args.DDoSDetection,
					ExcludeSources: args.ExcludeSources,
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudRateLimitSettingsToMap(*updated), nil
		},
	}
}

// --- Per-domain rate-limit rules ---------------------------------------------

// arvanCloudRateLimitRuleIDArgs is embedded by every rate-limit-rule tool
// below that is scoped to exactly one rule by domain + id.
type arvanCloudRateLimitRuleIDArgs struct {
	arvanCloudDomainNameArgs
	ID string `json:"id"`
}

func arvanCloudRateLimitRuleIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The rate-limit rule's provider-assigned ID (a UUID), as returned by create_arvancloud_rate_limit_rule or list_arvancloud_rate_limit_rules.",
	}
}

func arvanCloudRateLimitActionProperty(description string) map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudRateLimitActionChallenge),
			string(domain.ArvanCloudRateLimitActionBlock),
		},
		"description": description,
	}
}

func arvanCloudChallengeActionProperty() map[string]any {
	return map[string]any{
		"type": "object",
		"description": "The challenge to issue once a source exceeds the configured rate. REQUIRED when action is " +
			"\"challenge\"; ignored when action is \"block\".",
		"properties": map[string]any{
			"mode": map[string]any{
				"type":        "integer",
				"enum":        []int{1, 2, 3},
				"description": "The challenge mechanism: 1 = cookie-based check, 2 = JavaScript-execution check, 3 = CAPTCHA.",
			},
			"ttl":        map[string]any{"type": "integer", "description": "Challenge max-age, in seconds, e.g. 3600 for one hour."},
			"https_only": map[string]any{"type": "boolean", "description": "Add \"SameSite=None; Secure\" to the challenge's set-cookie header."},
		},
	}
}

// arvanCloudChallengeActionArgs decodes a rate-limit rule's action_details.
type arvanCloudChallengeActionArgs struct {
	Mode      int  `json:"mode"`
	TTL       int  `json:"ttl"`
	HTTPSOnly bool `json:"https_only"`
}

func (a *arvanCloudChallengeActionArgs) toDomain() domain.ArvanCloudChallengeAction {
	if a == nil {
		return domain.ArvanCloudChallengeAction{}
	}
	return domain.ArvanCloudChallengeAction{
		Mode:      domain.ArvanCloudChallengeMode(a.Mode),
		TTL:       a.TTL,
		HTTPSOnly: a.HTTPSOnly,
	}
}

// arvanCloudRateLimitRuleProperties adds the field set shared by
// create_arvancloud_rate_limit_rule and update_arvancloud_rate_limit_rule to
// props. rate/burst/time_duration/block_duration units are stated explicitly
// per AGENTS.md 5's schema quality requirement, confirmed against the spec:
// rate and burst count requests, time_duration and block_duration count
// seconds.
func arvanCloudRateLimitRuleProperties(props map[string]any) {
	props["url_pattern"] = map[string]any{
		"type": "string",
		"description": "A glob pattern (not a regex) this rule matches a request's URL against: `?` matches any " +
			"single character, `*` matches any sequence of characters, `**` matches across path segments, " +
			"`[...]`/`[!...]` match/exclude a character class. Example: \"/wp-admin/**\" or \"/api/v?/users\".",
	}
	props["description"] = map[string]any{"type": "string", "description": "An optional note about what this rule is for."}
	props["exclude_sources"] = map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
		"description": "CIDR-format source addresses exempted from this rule specifically, layered on top of the " +
			"domain's own globally exempted sources (update_arvancloud_rate_limit_settings's exclude_sources).",
	}
	props["rate"] = map[string]any{
		"type":        "integer",
		"description": "REQUIRED. The number of REQUESTS allowed per time_duration seconds, e.g. 100 for 100 requests. Must be positive.",
	}
	props["burst"] = map[string]any{
		"type":        "integer",
		"description": "The number of REQUESTS allowed to briefly exceed rate before this rule takes effect. Optional — omit to let ArvanCloud apply its own default.",
	}
	props["block_duration"] = map[string]any{
		"type":        "integer",
		"description": "How long, in SECONDS, a violating source is blocked for when action is \"block\", e.g. 300 for 5 minutes. Optional — omit to let ArvanCloud apply its own default.",
	}
	props["time_duration"] = map[string]any{
		"type":        "integer",
		"description": "REQUIRED. The window, in SECONDS, that rate is measured over, e.g. 60 for \"per minute\". Must be positive.",
	}
	props["allowed_methods"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string", "enum": []string{"POST", "GET", "PUT", "HEAD", "DELETE", "OPTIONS"}},
		"description": "Restrict this rule to specific HTTP methods, e.g. [\"POST\"] to only rate-limit POST requests. Leave empty to cover every method.",
	}
	props["action_details"] = arvanCloudChallengeActionProperty()
}

func createArvanCloudRateLimitRuleTool(uc *app.CreateArvanCloudRateLimitRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["action"] = arvanCloudRateLimitActionProperty(
		"What happens to a source once it exceeds the configured rate: \"challenge\" issues the challenge in " +
			"action_details, \"block\" blocks the source outright for block_duration seconds. Defaults to \"block\" when omitted.")
	props["is_enabled"] = map[string]any{"type": "boolean", "description": "Whether the rule is active. Defaults to true when omitted."}
	arvanCloudRateLimitRuleProperties(props)

	return Tool{
		Name: "create_arvancloud_rate_limit_rule",
		Description: "Create a new rate-limiting rule: throttle or block traffic matching a URL pattern once a " +
			"source exceeds a configured request rate. " + arvanCloudRateLimitVsDdosNote + " This is a fast " +
			"operation: the created rule, including its provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "url_pattern", "rate", "time_duration"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Action         string                         `json:"action"`
				IsEnabled      *bool                          `json:"is_enabled"`
				URLPattern     string                         `json:"url_pattern"`
				Description    string                         `json:"description"`
				ExcludeSources []string                       `json:"exclude_sources"`
				Rate           int                            `json:"rate"`
				Burst          int                            `json:"burst"`
				BlockDuration  int                            `json:"block_duration"`
				TimeDuration   int                            `json:"time_duration"`
				AllowedMethods []string                       `json:"allowed_methods"`
				ActionDetails  *arvanCloudChallengeActionArgs `json:"action_details"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			isEnabled := true
			if args.IsEnabled != nil {
				isEnabled = *args.IsEnabled
			}
			action := domain.ArvanCloudRateLimitAction(args.Action)
			if action == "" {
				action = domain.ArvanCloudRateLimitActionBlock
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudRateLimitRuleInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				Rule: domain.ArvanCloudRateLimitRule{
					Action:         action,
					IsEnabled:      isEnabled,
					URLPattern:     args.URLPattern,
					Description:    args.Description,
					ExcludeSources: args.ExcludeSources,
					Rate:           args.Rate,
					Burst:          args.Burst,
					BlockDuration:  args.BlockDuration,
					TimeDuration:   args.TimeDuration,
					AllowedMethods: args.AllowedMethods,
					ActionDetails:  args.ActionDetails.toDomain(),
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudRateLimitRuleToMap(*created), nil
		},
	}
}

func listArvanCloudRateLimitRulesTool(uc *app.ListArvanCloudRateLimitRules) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_rate_limit_rules",
		Description: "List every rate-limiting rule configured for a domain. " + arvanCloudRateLimitVsDdosNote +
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

			rules, err := uc.Execute(ctx, app.ListArvanCloudRateLimitRulesInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, r := range rules {
				out[i] = arvanCloudRateLimitRuleToMap(r)
			}
			return map[string]any{"rules": out}, nil
		},
	}
}

func getArvanCloudRateLimitRuleTool(uc *app.GetArvanCloudRateLimitRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudRateLimitRuleIDProperty()

	return Tool{
		Name:        "get_arvancloud_rate_limit_rule",
		Description: "Get the current state of one rate-limiting rule by ID. " + arvanCloudRateLimitVsDdosNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudRateLimitRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudRateLimitRuleInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudRateLimitRuleToMap(*found), nil
		},
	}
}

func updateArvanCloudRateLimitRuleTool(uc *app.UpdateArvanCloudRateLimitRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudRateLimitRuleIDProperty()
	props["action"] = arvanCloudRateLimitActionProperty("The rule's new action. See create_arvancloud_rate_limit_rule for the full list.")
	props["is_enabled"] = map[string]any{"type": "boolean", "description": "Whether the rule is active."}
	arvanCloudRateLimitRuleProperties(props)

	return Tool{
		Name: "update_arvancloud_rate_limit_rule",
		Description: "Update a rate-limiting rule. This replaces the rule's fields with the given values — pass " +
			"every field you want to keep, not only the ones changing. " + arvanCloudRateLimitVsDdosNote +
			" This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "url_pattern", "rate", "time_duration"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudRateLimitRuleIDArgs
				Action         string                         `json:"action"`
				IsEnabled      bool                           `json:"is_enabled"`
				URLPattern     string                         `json:"url_pattern"`
				Description    string                         `json:"description"`
				ExcludeSources []string                       `json:"exclude_sources"`
				Rate           int                            `json:"rate"`
				Burst          int                            `json:"burst"`
				BlockDuration  int                            `json:"block_duration"`
				TimeDuration   int                            `json:"time_duration"`
				AllowedMethods []string                       `json:"allowed_methods"`
				ActionDetails  *arvanCloudChallengeActionArgs `json:"action_details"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			action := domain.ArvanCloudRateLimitAction(args.Action)
			if action == "" {
				action = domain.ArvanCloudRateLimitActionBlock
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudRateLimitRuleInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				ID:          args.ID,
				Rule: domain.ArvanCloudRateLimitRule{
					Action:         action,
					IsEnabled:      args.IsEnabled,
					URLPattern:     args.URLPattern,
					Description:    args.Description,
					ExcludeSources: args.ExcludeSources,
					Rate:           args.Rate,
					Burst:          args.Burst,
					BlockDuration:  args.BlockDuration,
					TimeDuration:   args.TimeDuration,
					AllowedMethods: args.AllowedMethods,
					ActionDetails:  args.ActionDetails.toDomain(),
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudRateLimitRuleToMap(*updated), nil
		},
	}
}

func deleteArvanCloudRateLimitRuleTool(uc *app.DeleteArvanCloudRateLimitRule) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = arvanCloudRateLimitRuleIDProperty()

	return Tool{
		Name: "delete_arvancloud_rate_limit_rule",
		Description: "Permanently delete a rate-limiting rule by ID. " + arvanCloudRateLimitVsDdosNote +
			" This is a fast operation and cannot be undone. Deleting a rule that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudRateLimitRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudRateLimitRuleInput{Credentials: args.domain(), Domain: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

func reprioritizeArvanCloudRateLimitRulesTool(uc *app.ReprioritizeArvanCloudRateLimitRules) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudReprioritizeProperties(props)

	return Tool{
		Name: "reprioritize_arvancloud_rate_limit_rules",
		Description: "Change the evaluation order of a domain's rate-limiting rules by moving one rule relative " +
			"to another. " + arvanCloudRateLimitVsDdosNote + " Give exactly one of after_rule_id/before_rule_id, " +
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

			if err := uc.Execute(ctx, app.ReprioritizeArvanCloudRateLimitRulesInput{
				Credentials: args.domain(), Domain: args.Domain,
				RuleID: args.RuleID, AfterRuleID: args.AfterRuleID, BeforeRuleID: args.BeforeRuleID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"reprioritized": true, "domain": args.Domain, "rule_id": args.RuleID}, nil
		},
	}
}
