package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// rateLimitRuleIDProperty is shared by every tool that operates on an
// existing rate limit rule.
func rateLimitRuleIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The rate limit rule's provider ID, as returned by list_cdn_rate_limit_rules or get_cdn_rate_limit_rule.",
	}
}

// rateLimitRuleFieldProperties are the mutable fields of a rate limit rule,
// shared by create_cdn_rate_limit_rule and update_cdn_rate_limit_rule.
// includeEnabled controls whether "enabled" is added: the provider's create
// endpoint does not accept it (the rule starts in the provider's default
// state), but update requires it.
func rateLimitRuleFieldProperties(props map[string]any, includeEnabled bool) {
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Human-readable name for the rule, e.g. \"api-burst-guard\". Max 255 characters.",
	}
	props["value"] = map[string]any{
		"type":        "string",
		"description": "The URL pattern (glob) this rule matches against, e.g. \"https://example.com/api/*\".",
	}
	if includeEnabled {
		props["enabled"] = map[string]any{
			"type":        "boolean",
			"description": "Whether the rule is active.",
		}
	}
	props["static_interval_type"] = map[string]any{
		"type":        "string",
		"enum":        []string{"second", "minute"},
		"description": "Unit for static_interval, e.g. \"second\".",
	}
	props["static_interval"] = map[string]any{
		"type":        "integer",
		"minimum":     0,
		"description": "Length of the static rate-limit window in static_interval_type units, e.g. 10 for a 10-second window.",
	}
	props["dynamic_interval_type"] = map[string]any{
		"type":        "string",
		"enum":        []string{"second", "minute", "hour", "day"},
		"description": "Unit for dynamic_interval, e.g. \"day\".",
	}
	props["dynamic_interval"] = map[string]any{
		"type":        "integer",
		"minimum":     0,
		"description": "Length of the dynamic (traffic-pattern-aware) rate-limit window in dynamic_interval_type units.",
	}
	props["ip_reputation_enabled"] = map[string]any{
		"type":        "boolean",
		"description": "Factor the requesting IP's reputation score into the dynamic rate limit decision.",
	}
	props["challenge"] = map[string]any{
		"type":        "string",
		"enum":        []string{"js", "block", "captcha", "allow", "bypass"},
		"description": "Action taken against a client that exceeds the rule's limits, e.g. \"captcha\" to require a captcha, \"block\" to reject outright.",
	}
	props["trust_time"] = map[string]any{
		"type":        "integer",
		"minimum":     0,
		"description": "Seconds a client that passes the challenge is trusted before being re-challenged, e.g. 3600 for one hour.",
	}
	props["attack_ban_time"] = map[string]any{
		"type":        "integer",
		"minimum":     0,
		"description": "Seconds a client that fails the challenge is banned for, e.g. 900 for 15 minutes.",
	}
	props["white_list_ips"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "IP addresses exempt from this rule. Omit for no exemptions.",
	}
}

// rateLimitRuleFieldArgs carries the mutable fields shared by
// create_cdn_rate_limit_rule and update_cdn_rate_limit_rule.
type rateLimitRuleFieldArgs struct {
	Name                string   `json:"name"`
	Value               string   `json:"value"`
	Enabled             bool     `json:"enabled"`
	StaticIntervalType  string   `json:"static_interval_type"`
	StaticInterval      int      `json:"static_interval"`
	DynamicIntervalType string   `json:"dynamic_interval_type"`
	DynamicInterval     int      `json:"dynamic_interval"`
	IPReputationEnabled bool     `json:"ip_reputation_enabled"`
	Challenge           string   `json:"challenge"`
	TrustTime           int      `json:"trust_time"`
	AttackBanTime       int      `json:"attack_ban_time"`
	WhiteListIPs        []string `json:"white_list_ips"`
}

func (a rateLimitRuleFieldArgs) toDomainRule() domain.CDNRateLimitRule {
	rule := domain.CDNRateLimitRule{
		Name: a.Name, Value: a.Value, Enabled: a.Enabled,
		StaticIntervalType: a.StaticIntervalType, StaticInterval: a.StaticInterval,
		DynamicIntervalType: a.DynamicIntervalType, DynamicInterval: a.DynamicInterval,
		IPReputationEnabled: a.IPReputationEnabled,
		Challenge:           a.Challenge, TrustTime: a.TrustTime, AttackBanTime: a.AttackBanTime,
	}
	for _, ip := range a.WhiteListIPs {
		rule.WhitelistIPs = append(rule.WhitelistIPs, domain.CDNRateLimitWhitelistIP{IP: ip})
	}
	return rule
}

// rateLimitRuleToMap renders a domain.CDNRateLimitRule the way every
// rule-returning tool reports it back to the caller.
func rateLimitRuleToMap(rule domain.CDNRateLimitRule) map[string]any {
	ips := make([]string, len(rule.WhitelistIPs))
	for i, ip := range rule.WhitelistIPs {
		ips[i] = ip.IP
	}
	return map[string]any{
		"id":                    rule.ID,
		"name":                  rule.Name,
		"value":                 rule.Value,
		"enabled":               rule.Enabled,
		"priority":              rule.Priority,
		"static_interval_type":  rule.StaticIntervalType,
		"static_interval":       rule.StaticInterval,
		"static_requests":       rule.StaticRequests,
		"dynamic_interval_type": rule.DynamicIntervalType,
		"dynamic_interval":      rule.DynamicInterval,
		"dynamic_requests":      rule.DynamicRequests,
		"ip_reputation_enabled": rule.IPReputationEnabled,
		"challenge":             rule.Challenge,
		"trust_time":            rule.TrustTime,
		"attack_ban_time":       rule.AttackBanTime,
		"white_list_ips":        ips,
	}
}

func listCDNRateLimitRulesTool(uc *app.ListCDNRateLimitRules) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_cdn_rate_limit_rules",
		Description: "List every rate limit rule configured on a Parspack CDN zone's edge firewall. This is a fast " +
			"operation: the list is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args zoneUUIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rules, err := uc.Execute(ctx, app.ListCDNRateLimitRulesInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, rule := range rules {
				out[i] = rateLimitRuleToMap(rule)
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "rules": out}, nil
		},
	}
}

type createCDNRateLimitRuleArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	rateLimitRuleFieldArgs
}

func createCDNRateLimitRuleTool(uc *app.CreateCDNRateLimitRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	rateLimitRuleFieldProperties(props, false)

	return Tool{
		Name: "create_cdn_rate_limit_rule",
		Description: "Create a new rate limit rule on a Parspack CDN zone's edge firewall. This is a fast operation. " +
			"The provider does not return the new rule's ID in its response — call list_cdn_rate_limit_rules " +
			"afterward to discover it.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required": []string{
				"api_key", "zone_uuid", "name", "value", "static_interval_type", "static_interval",
				"dynamic_interval_type", "dynamic_interval", "challenge", "trust_time", "attack_ban_time",
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createCDNRateLimitRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateCDNRateLimitRuleInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				Rule:        args.toDomainRule(),
			})
			if err != nil {
				return nil, err
			}
			return rateLimitRuleToMap(*created), nil
		},
	}
}

type rateLimitRuleIDArgs struct {
	credentialArgs
	ZoneUUID        string `json:"zone_uuid"`
	RateLimitRuleID string `json:"rate_limit_rule_id"`
}

func getCDNRateLimitRuleTool(uc *app.GetCDNRateLimitRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["rate_limit_rule_id"] = rateLimitRuleIDProperty()

	return Tool{
		Name: "get_cdn_rate_limit_rule",
		Description: "Get the current state of one rate limit rule on a Parspack CDN zone's edge firewall by its ID. " +
			"This is a fast operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "rate_limit_rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args rateLimitRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.GetCDNRateLimitRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RateLimitRuleID,
			})
			if err != nil {
				return nil, err
			}
			return rateLimitRuleToMap(*rule), nil
		},
	}
}

type updateCDNRateLimitRuleArgs struct {
	credentialArgs
	ZoneUUID        string `json:"zone_uuid"`
	RateLimitRuleID string `json:"rate_limit_rule_id"`
	rateLimitRuleFieldArgs
}

func updateCDNRateLimitRuleTool(uc *app.UpdateCDNRateLimitRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["rate_limit_rule_id"] = rateLimitRuleIDProperty()
	rateLimitRuleFieldProperties(props, true)

	return Tool{
		Name: "update_cdn_rate_limit_rule",
		Description: "Replace the configuration of an existing rate limit rule on a Parspack CDN zone's edge " +
			"firewall by its ID. This is a fast operation: the updated rule is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required": []string{
				"api_key", "zone_uuid", "rate_limit_rule_id", "name", "value", "enabled", "static_interval_type",
				"static_interval", "dynamic_interval_type", "dynamic_interval", "challenge", "trust_time",
				"attack_ban_time",
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNRateLimitRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateCDNRateLimitRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RateLimitRuleID,
				Rule: args.toDomainRule(),
			})
			if err != nil {
				return nil, err
			}
			return rateLimitRuleToMap(*updated), nil
		},
	}
}

func deleteCDNRateLimitRuleTool(uc *app.DeleteCDNRateLimitRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["rate_limit_rule_id"] = rateLimitRuleIDProperty()

	return Tool{
		Name: "delete_cdn_rate_limit_rule",
		Description: "Permanently delete a rate limit rule from a Parspack CDN zone's edge firewall by its ID. This " +
			"is a fast operation and cannot be undone. Deleting a rule that no longer exists is treated as already " +
			"done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "rate_limit_rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args rateLimitRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteCDNRateLimitRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RateLimitRuleID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID, "rate_limit_rule_id": args.RateLimitRuleID}, nil
		},
	}
}

type updateCDNRateLimitRulePriorityArgs struct {
	credentialArgs
	ZoneUUID        string `json:"zone_uuid"`
	RateLimitRuleID string `json:"rate_limit_rule_id"`
	Priority        int    `json:"priority"`
}

func updateCDNRateLimitRulePriorityTool(uc *app.UpdateCDNRateLimitRulePriority) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["rate_limit_rule_id"] = rateLimitRuleIDProperty()
	props["priority"] = map[string]any{
		"type":        "integer",
		"minimum":     1,
		"description": "New evaluation priority for the rule relative to the zone's other rate limit rules. Lower values are evaluated first, e.g. 1 for highest priority.",
	}

	return Tool{
		Name: "update_cdn_rate_limit_rule_priority",
		Description: "Reorder the evaluation priority of a rate limit rule on a Parspack CDN zone's edge firewall. " +
			"This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "rate_limit_rule_id", "priority"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNRateLimitRulePriorityArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.UpdateCDNRateLimitRulePriorityInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RateLimitRuleID, Priority: args.Priority,
			}); err != nil {
				return nil, err
			}
			return map[string]any{
				"updated": true, "zone_uuid": args.ZoneUUID, "rate_limit_rule_id": args.RateLimitRuleID, "priority": args.Priority,
			}, nil
		},
	}
}

func upstreamErrorsToMap(zoneUUID string, settings domain.CDNUpstreamErrorSettings) map[string]any {
	return map[string]any{"zone_uuid": zoneUUID, "enabled": settings.Enabled}
}

func getCDNUpstreamErrorsTool(uc *app.GetCDNUpstreamErrors) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_upstream_errors",
		Description: "Get whether Parspack's \"upstream errors\" handling is enabled for a CDN zone. This is a fast " +
			"operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args zoneUUIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			settings, err := uc.Execute(ctx, app.GetCDNUpstreamErrorsInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return upstreamErrorsToMap(args.ZoneUUID, *settings), nil
		},
	}
}

type updateCDNUpstreamErrorsArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Enabled  bool   `json:"enabled"`
}

func updateCDNUpstreamErrorsTool(uc *app.UpdateCDNUpstreamErrors) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["enabled"] = map[string]any{
		"type":        "boolean",
		"description": "Whether to enable Parspack's \"upstream errors\" handling for this zone.",
	}

	return Tool{
		Name: "update_cdn_upstream_errors",
		Description: "Enable or disable Parspack's \"upstream errors\" handling for a CDN zone. This is a fast " +
			"operation: the updated setting is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "enabled"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNUpstreamErrorsArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			settings, err := uc.Execute(ctx, app.UpdateCDNUpstreamErrorsInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, Enabled: args.Enabled,
			})
			if err != nil {
				return nil, err
			}
			return upstreamErrorsToMap(args.ZoneUUID, *settings), nil
		},
	}
}
