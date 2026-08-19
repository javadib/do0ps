package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// CDN-edge firewall tools (issue #24): access-management rules, IP-reputation
// blocking and DDoS mitigation actions, all scoped to a CDN zone. All are
// fast operations — see each tool's Description.
//
// These are entirely distinct from the cloud-server/VM-network-level
// firewall tools elsewhere in this package (issue #11); every tool here
// takes a zone_uuid, never a server/network id.

func cdnAccessRuleToMap(rule domain.CDNAccessRule) map[string]any {
	return map[string]any{
		"id":            rule.ID,
		"zone_uuid":     rule.ZoneUUID,
		"type":          rule.Type,
		"value":         rule.Value,
		"action":        rule.Action,
		"status":        rule.Status,
		"priority":      rule.Priority,
		"bulklist_id":   rule.BulklistID,
		"bulklist_name": rule.BulklistName,
	}
}

func cdnIPReputationToMap(s domain.CDNIPReputationSettings) map[string]any {
	return map[string]any{
		"ip_reputation_enabled":     s.Enabled,
		"ip_reputation_trust_time":  s.TrustTime,
		"ip_reputation_treat_score": s.TreatScore,
		"ip_reputation_challenge":   s.Challenge,
		"attack_ban_time":           s.AttackBanTime,
	}
}

func cdnDDoSActionsToMap(s domain.CDNDDoSActionSettings) map[string]any {
	return map[string]any{
		"action":     s.Action,
		"trust_time": s.TrustTime,
		"ban_time":   s.BanTime,
	}
}

// accessManagementIDProperty is shared by every tool that identifies one
// access-management rule.
func accessManagementIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The access-management rule's ID, as returned by list_cdn_access_rules or get_cdn_access_rule, e.g. \"vpaO2roj\".",
	}
}

func cdnAccessRuleTypeProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{"ip", "user_agent", "country", "zone", "referral"},
		"description": "What the rule matches on: an IP address, a user agent string, a country, the whole zone, or an HTTP referral value.",
	}
}

func cdnAccessRuleActionProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{"dynamic", "block", "captcha", "allow", "bypass"},
		"description": "Action to take on matching traffic: \"block\" denies it, \"allow\" permits it, \"captcha\" challenges it, \"bypass\" skips other CDN processing, \"dynamic\" applies the zone's adaptive handling.",
	}
}

func cdnAccessRuleValueProperties(props map[string]any) {
	props["value"] = map[string]any{
		"type": "string",
		"description": "The value to match against `type`, e.g. an IPv4/IPv6 address for type \"ip\", or a country id for " +
			"type \"country\". Required unless bulklist_id is set.",
	}
	props["bulklist_id"] = map[string]any{
		"type": "string",
		"description": "ID of a Parspack bulk list of values to match instead of a single value. When set, `value` is " +
			"ignored. Required unless value is set.",
	}
}

type zoneUUIDAndRuleIDArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	RuleID   string `json:"access_management_id"`
}

func listCDNAccessRulesTool(uc *app.ListCDNAccessRules) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_cdn_access_rules",
		Description: "List every CDN-edge access-management (firewall) rule of a Parspack CDN zone: allow/block/challenge " +
			"rules by IP, user agent, country, referral or the whole zone. This is a fast operation: the list is " +
			"returned within this call.",
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

			rules, err := uc.Execute(ctx, app.ListCDNAccessRulesInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, rule := range rules {
				out[i] = cdnAccessRuleToMap(rule)
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "rules": out}, nil
		},
	}
}

func getCDNAccessRuleTool(uc *app.GetCDNAccessRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["access_management_id"] = accessManagementIDProperty()

	return Tool{
		Name: "get_cdn_access_rule",
		Description: "Get one CDN-edge access-management (firewall) rule of a Parspack CDN zone by its ID. This is a " +
			"fast operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "access_management_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args zoneUUIDAndRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.GetCDNAccessRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID,
			})
			if err != nil {
				return nil, err
			}
			return cdnAccessRuleToMap(*rule), nil
		},
	}
}

type createCDNAccessRuleArgs struct {
	credentialArgs
	ZoneUUID   string `json:"zone_uuid"`
	Type       string `json:"type"`
	Value      string `json:"value"`
	Action     string `json:"action"`
	BulklistID string `json:"bulklist_id"`
}

func createCDNAccessRuleTool(uc *app.CreateCDNAccessRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["type"] = cdnAccessRuleTypeProperty()
	props["action"] = cdnAccessRuleActionProperty()
	cdnAccessRuleValueProperties(props)

	return Tool{
		Name: "create_cdn_access_rule",
		Description: "Create a new CDN-edge access-management (firewall) rule for a Parspack CDN zone. This is a fast " +
			"operation: the requested rule is echoed back within this call, but the provider does not report an ID or " +
			"priority synchronously — call list_cdn_access_rules afterward to learn them.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "type", "action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createCDNAccessRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.CreateCDNAccessRuleInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				Rule: domain.CDNAccessRule{
					Type: args.Type, Value: args.Value, Action: args.Action, BulklistID: args.BulklistID,
				},
			})
			if err != nil {
				return nil, err
			}
			return cdnAccessRuleToMap(*rule), nil
		},
	}
}

type updateCDNAccessRuleArgs struct {
	credentialArgs
	ZoneUUID   string `json:"zone_uuid"`
	RuleID     string `json:"access_management_id"`
	Value      string `json:"value"`
	Action     string `json:"action"`
	Status     bool   `json:"status"`
	BulklistID string `json:"bulklist_id"`
}

func updateCDNAccessRuleTool(uc *app.UpdateCDNAccessRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["access_management_id"] = accessManagementIDProperty()
	props["action"] = cdnAccessRuleActionProperty()
	props["status"] = map[string]any{
		"type":        "boolean",
		"description": "Whether the rule is enabled. Set false to keep the rule without enforcing it.",
	}
	cdnAccessRuleValueProperties(props)

	return Tool{
		Name: "update_cdn_access_rule",
		Description: "Update an existing CDN-edge access-management (firewall) rule of a Parspack CDN zone by ID: " +
			"value/bulklist, action and status can change, but the match type cannot. This is a fast operation: the " +
			"requested update is echoed back within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "access_management_id", "action", "status"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNAccessRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.UpdateCDNAccessRuleInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				Rule: domain.CDNAccessRule{
					ID: args.RuleID, Value: args.Value, Action: args.Action, Status: args.Status, BulklistID: args.BulklistID,
				},
			})
			if err != nil {
				return nil, err
			}
			return cdnAccessRuleToMap(*rule), nil
		},
	}
}

func deleteCDNAccessRuleTool(uc *app.DeleteCDNAccessRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["access_management_id"] = accessManagementIDProperty()

	return Tool{
		Name: "delete_cdn_access_rule",
		Description: "Delete a CDN-edge access-management (firewall) rule of a Parspack CDN zone by ID. This is a fast " +
			"operation and cannot be undone. Deleting a rule that no longer exists is treated as already done rather " +
			"than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "access_management_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args zoneUUIDAndRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteCDNAccessRuleInput{
				Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID, "access_management_id": args.RuleID}, nil
		},
	}
}

func getCDNIPReputationTool(uc *app.GetCDNIPReputation) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_ip_reputation",
		Description: "Get the current IP-reputation-based blocking settings for a Parspack CDN zone. This is a fast " +
			"operation.",
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

			settings, err := uc.Execute(ctx, app.GetCDNIPReputationInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return cdnIPReputationToMap(*settings), nil
		},
	}
}

type updateCDNIPReputationArgs struct {
	credentialArgs
	ZoneUUID      string `json:"zone_uuid"`
	Enabled       bool   `json:"ip_reputation_enabled"`
	TrustTime     int    `json:"ip_reputation_trust_time"`
	TreatScore    string `json:"ip_reputation_treat_score"`
	Challenge     string `json:"ip_reputation_challenge"`
	AttackBanTime int    `json:"attack_ban_time"`
}

func updateCDNIPReputationTool(uc *app.UpdateCDNIPReputation) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["ip_reputation_enabled"] = map[string]any{
		"type":        "boolean",
		"description": "Whether IP-reputation-based blocking is enabled for the zone.",
	}
	props["ip_reputation_trust_time"] = map[string]any{
		"type":        "integer",
		"description": "Seconds a challenged IP is trusted after passing the challenge, e.g. 3600 for one hour.",
	}
	props["ip_reputation_treat_score"] = map[string]any{
		"type":        "string",
		"enum":        []string{"very-low", "low", "medium", "high", "very-high"},
		"description": "Sensitivity threshold for treating an IP as suspicious.",
	}
	props["ip_reputation_challenge"] = map[string]any{
		"type":        "string",
		"enum":        []string{"js", "block", "recaptcha"},
		"description": "Action applied to IPs flagged by reputation: a JS challenge, an outright block, or reCAPTCHA.",
	}
	props["attack_ban_time"] = map[string]any{
		"type":        "integer",
		"description": "Seconds an IP that trips the reputation threshold is banned for, e.g. 900 for 15 minutes.",
	}

	return Tool{
		Name: "update_cdn_ip_reputation",
		Description: "Replace the IP-reputation-based blocking settings for a Parspack CDN zone. The provider requires " +
			"every field on this call. This is a fast operation: the updated settings are echoed back within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required": []string{
				"api_key", "zone_uuid", "ip_reputation_enabled", "ip_reputation_trust_time",
				"ip_reputation_treat_score", "ip_reputation_challenge", "attack_ban_time",
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNIPReputationArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			settings, err := uc.Execute(ctx, app.UpdateCDNIPReputationInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				Settings: domain.CDNIPReputationSettings{
					Enabled: args.Enabled, TrustTime: args.TrustTime, TreatScore: args.TreatScore,
					Challenge: args.Challenge, AttackBanTime: args.AttackBanTime,
				},
			})
			if err != nil {
				return nil, err
			}
			return cdnIPReputationToMap(*settings), nil
		},
	}
}

func getCDNDDoSActionsTool(uc *app.GetCDNDDoSActions) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name:        "get_cdn_ddos_actions",
		Description: "Get the current DDoS mitigation action settings for a Parspack CDN zone. This is a fast operation.",
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

			settings, err := uc.Execute(ctx, app.GetCDNDDoSActionsInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return cdnDDoSActionsToMap(*settings), nil
		},
	}
}

type updateCDNDDoSActionsArgs struct {
	credentialArgs
	ZoneUUID  string `json:"zone_uuid"`
	Action    string `json:"action"`
	TrustTime int    `json:"trust_time"`
	BanTime   int    `json:"ban_time"`
}

func updateCDNDDoSActionsTool(uc *app.UpdateCDNDDoSActions) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["action"] = map[string]any{
		"type":        "string",
		"enum":        []string{"none", "js", "recaptcha", "block"},
		"description": "DDoS mitigation action to apply to suspected attack traffic.",
	}
	props["trust_time"] = map[string]any{
		"type":        "integer",
		"description": "Seconds a client is trusted after passing the DDoS challenge, e.g. 3600. Omit to keep the provider's default (3600).",
	}
	props["ban_time"] = map[string]any{
		"type":        "integer",
		"description": "Seconds a client identified as an attacker is banned for, e.g. 900. Omit to keep the provider's default (900).",
	}

	return Tool{
		Name: "update_cdn_ddos_actions",
		Description: "Update the DDoS mitigation action for a Parspack CDN zone. This is a fast operation: the updated " +
			"settings are echoed back within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNDDoSActionsArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			settings, err := uc.Execute(ctx, app.UpdateCDNDDoSActionsInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				Settings:    domain.CDNDDoSActionSettings{Action: args.Action, TrustTime: args.TrustTime, BanTime: args.BanTime},
			})
			if err != nil {
				return nil, err
			}
			return cdnDDoSActionsToMap(*settings), nil
		},
	}
}
