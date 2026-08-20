package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// Tools for Parspack's ModSec (WAF custom rules) CDN capability, scoped to a
// zone same as DNS records (AGENTS.md 4.1) — issue #24. Every tool here is a
// fast operation: synchronous rule/settings CRUD against the CDN API, no
// operation_id to poll.

// --- ModSec status (zone-level rule-set selection) -------------------------

// modSecRuleSetItemToMap renders a domain.CDNModSecRuleSetItem the way
// get/update_cdn_modsec_status report each standard/custom entry.
func modSecRuleSetItemToMap(item domain.CDNModSecRuleSetItem) map[string]any {
	return map[string]any{"id": item.ID, "name": item.Name, "selected": item.Selected}
}

// modSecStatusToMap renders a domain.CDNModSecStatus the way
// get/update_cdn_modsec_status report it back to the caller.
func modSecStatusToMap(status domain.CDNModSecStatus) map[string]any {
	standards := make([]map[string]any, len(status.Standards))
	for i, it := range status.Standards {
		standards[i] = modSecRuleSetItemToMap(it)
	}
	customs := make([]map[string]any, len(status.Customs))
	for i, it := range status.Customs {
		customs[i] = modSecRuleSetItemToMap(it)
	}
	return map[string]any{"standards": standards, "customs": customs}
}

func getCDNModSecStatusTool(uc *app.GetCDNModSecStatus) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "get_cdn_modsec_status",
		Description: "Get a CDN zone's ModSec (WAF) rule-set selection: the standard rule sets Parspack offers and " +
			"the zone's own custom rules, each flagged with whether it is currently selected/enabled. This is a fast " +
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

			status, err := uc.Execute(ctx, app.GetCDNModSecStatusInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}
			return modSecStatusToMap(*status), nil
		},
	}
}

type updateCDNModSecStatusArgs struct {
	credentialArgs
	ZoneUUID        string   `json:"zone_uuid"`
	SelectedRuleIDs []string `json:"selected_rule_ids"`
}

func updateCDNModSecStatusTool(uc *app.UpdateCDNModSecStatus) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["selected_rule_ids"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Complete replacement set of ModSec rule-set ids (standard and/or custom, from get_cdn_modsec_status or list_cdn_modsec_rules) that should be selected/enabled on the zone. Pass an empty array to clear every selection.",
	}

	return Tool{
		Name: "update_cdn_modsec_status",
		Description: "Replace which ModSec (WAF) rule sets are selected/enabled on a CDN zone (standard rule sets " +
			"and/or the zone's own custom rules). This is a fast operation: the resulting status is returned within " +
			"this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "selected_rule_ids"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNModSecStatusArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			status, err := uc.Execute(ctx, app.UpdateCDNModSecStatusInput{
				Credentials:     args.domain(),
				ZoneUUID:        args.ZoneUUID,
				SelectedRuleIDs: args.SelectedRuleIDs,
			})
			if err != nil {
				return nil, err
			}
			return modSecStatusToMap(*status), nil
		},
	}
}

// --- ModSec data (reusable values custom rules reference) ------------------

// modSecDataToMap renders a domain.CDNModSecData the way every
// data-returning tool reports it back to the caller. Value is empty when the
// caller only asked for a list (the CDN API's list endpoint does not report it).
func modSecDataToMap(data domain.CDNModSecData) map[string]any {
	return map[string]any{"id": data.ID, "name": data.Name, "value": data.Value}
}

func listCDNModSecDataTool(uc *app.ListCDNModSecData) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_cdn_modsec_data",
		Description: "List the reusable ModSec data values (e.g. shared regex/allow-list content) defined on a CDN " +
			"zone that custom WAF rules can reference. Only id and name are returned here — use " +
			"get_cdn_modsec_data for a specific entry's value. This is a fast operation.",
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

			data, err := uc.Execute(ctx, app.ListCDNModSecDataInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(data))
			for i, d := range data {
				out[i] = modSecDataToMap(d)
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "data": out}, nil
		},
	}
}

// modSecDataWriteArgs is shared by create_cdn_modsec_data and
// update_cdn_modsec_data.
type modSecDataWriteArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

func modSecDataValueProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The data value's raw content, e.g. a regex pattern or allow-listed value. Sent as-is; base64 encoding for the wire is handled automatically.",
	}
}

func createCDNModSecDataTool(uc *app.CreateCDNModSecData) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Human-readable name for the data value, e.g. \"blocked-user-agents\". Max 255 characters.",
	}
	props["value"] = modSecDataValueProperty()

	return Tool{
		Name: "create_cdn_modsec_data",
		Description: "Create a new reusable ModSec data value on a CDN zone for custom WAF rules to reference. This " +
			"is a fast operation: the provider does not return the new value's id in its response, so the created " +
			"data is echoed back without an id — call list_cdn_modsec_data afterward to discover it.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "name", "value"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args modSecDataWriteArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			data, err := uc.Execute(ctx, app.CreateCDNModSecDataInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				Data:        domain.CDNModSecData{Name: args.Name, Value: args.Value},
			})
			if err != nil {
				return nil, err
			}
			return modSecDataToMap(*data), nil
		},
	}
}

type modSecDataIDArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	ID       string `json:"id"`
}

func modSecDataIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The ModSec data value's id, as returned by list_cdn_modsec_data.",
	}
}

func getCDNModSecDataTool(uc *app.GetCDNModSecData) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["id"] = modSecDataIDProperty()

	return Tool{
		Name: "get_cdn_modsec_data",
		Description: "Get one ModSec data value by id, including its decoded content. This is a fast operation: the " +
			"result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args modSecDataIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			data, err := uc.Execute(ctx, app.GetCDNModSecDataInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return modSecDataToMap(*data), nil
		},
	}
}

type updateCDNModSecDataArgs struct {
	modSecDataWriteArgs
	ID string `json:"id"`
}

func updateCDNModSecDataTool(uc *app.UpdateCDNModSecData) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["id"] = modSecDataIDProperty()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "New human-readable name for the data value. Max 255 characters.",
	}
	props["value"] = modSecDataValueProperty()

	return Tool{
		Name: "update_cdn_modsec_data",
		Description: "Replace an existing ModSec data value's name and content by id. This is a fast operation: the " +
			"updated data is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "id", "name", "value"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNModSecDataArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			data, err := uc.Execute(ctx, app.UpdateCDNModSecDataInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				ID:          args.ID,
				Data:        domain.CDNModSecData{Name: args.Name, Value: args.Value},
			})
			if err != nil {
				return nil, err
			}
			return modSecDataToMap(*data), nil
		},
	}
}

func deleteCDNModSecDataTool(uc *app.DeleteCDNModSecData) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["id"] = modSecDataIDProperty()

	return Tool{
		Name: "delete_cdn_modsec_data",
		Description: "Permanently delete a ModSec data value from a CDN zone by id. This is a fast operation and " +
			"cannot be undone. Deleting a value that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args modSecDataIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteCDNModSecDataInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID, "id": args.ID}, nil
		},
	}
}

// --- ModSec custom rules -----------------------------------------------

// modSecRuleToMap renders a domain.CDNModSecRule the way every
// rule-returning tool reports it back to the caller. Different fields are
// populated depending on which provider endpoint produced the value (see
// domain.CDNModSecRule's doc comment): list_cdn_modsec_rules fills name/
// status but not rule_value/mod_sec_data, get_cdn_modsec_rule is the reverse.
func modSecRuleToMap(rule domain.CDNModSecRule) map[string]any {
	data := make([]map[string]any, len(rule.ModSecData))
	for i, d := range rule.ModSecData {
		data[i] = modSecDataToMap(d)
	}
	return map[string]any{
		"id":              rule.ID,
		"name":            rule.Name,
		"status":          rule.Status,
		"rule_value":      rule.RuleValue,
		"modsec_data_ids": rule.ModSecDataIDs,
		"mod_sec_data":    data,
	}
}

func listCDNModSecRulesTool(uc *app.ListCDNModSecRules) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_cdn_modsec_rules",
		Description: "List the custom ModSec (WAF) rules defined on a CDN zone. Only id, name and status are " +
			"returned here — use get_cdn_modsec_rule for a specific rule's value and referenced data. This is a fast " +
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

			rules, err := uc.Execute(ctx, app.ListCDNModSecRulesInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(rules))
			for i, r := range rules {
				out[i] = modSecRuleToMap(r)
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "rules": out}, nil
		},
	}
}

func modSecRuleNameProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "Human-readable name for the custom rule, e.g. \"block-sqli-in-query\". Max 255 characters.",
	}
}

func modSecRuleValueProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The rule's raw ModSecRule directive text. Sent as-is; base64 encoding for the wire is handled automatically.",
	}
}

func modSecDataIDsProperty() map[string]any {
	return map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "ids of ModSec data values (from list_cdn_modsec_data) this rule references. Pass an empty array if the rule references no data.",
	}
}

type createCDNModSecRuleArgs struct {
	credentialArgs
	ZoneUUID      string   `json:"zone_uuid"`
	Name          string   `json:"name"`
	RuleValue     string   `json:"rule_value"`
	ModSecDataIDs []string `json:"modsec_data_ids"`
}

func createCDNModSecRuleTool(uc *app.CreateCDNModSecRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["name"] = modSecRuleNameProperty()
	props["rule_value"] = modSecRuleValueProperty()
	props["modsec_data_ids"] = modSecDataIDsProperty()

	return Tool{
		Name: "create_cdn_modsec_rule",
		Description: "Create a new custom ModSec (WAF) rule on a CDN zone, optionally referencing existing ModSec " +
			"data values. This is a fast operation: the provider does not return the new rule's id in its response, " +
			"so the created rule is echoed back without an id — call list_cdn_modsec_rules afterward to discover it.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "name", "rule_value", "modsec_data_ids"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createCDNModSecRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.CreateCDNModSecRuleInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				Rule: domain.CDNModSecRule{
					Name: args.Name, RuleValue: args.RuleValue, ModSecDataIDs: args.ModSecDataIDs,
				},
			})
			if err != nil {
				return nil, err
			}
			return modSecRuleToMap(*rule), nil
		},
	}
}

type modSecRuleIDArgs struct {
	credentialArgs
	ZoneUUID string `json:"zone_uuid"`
	RuleID   string `json:"rule_id"`
}

func modSecRuleIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The custom ModSec rule's id, as returned by list_cdn_modsec_rules.",
	}
}

func getCDNModSecRuleTool(uc *app.GetCDNModSecRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["rule_id"] = modSecRuleIDProperty()

	return Tool{
		Name: "get_cdn_modsec_rule",
		Description: "Get one custom ModSec rule by id, including its decoded rule value and expanded referenced " +
			"data. The provider does not report the rule's name or status on this endpoint (unlike " +
			"list_cdn_modsec_rules) — those fields come back empty. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args modSecRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.GetCDNModSecRuleInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID})
			if err != nil {
				return nil, err
			}
			return modSecRuleToMap(*rule), nil
		},
	}
}

type updateCDNModSecRuleArgs struct {
	credentialArgs
	ZoneUUID      string   `json:"zone_uuid"`
	RuleID        string   `json:"rule_id"`
	Name          string   `json:"name"`
	RuleValue     string   `json:"rule_value"`
	ModSecDataIDs []string `json:"modsec_data_ids"`
}

func updateCDNModSecRuleTool(uc *app.UpdateCDNModSecRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["rule_id"] = modSecRuleIDProperty()
	props["name"] = modSecRuleNameProperty()
	props["rule_value"] = map[string]any{
		"type":        "string",
		"description": "New raw ModSecRule directive text. Omit to leave the current rule value unchanged. Sent as-is; base64 encoding for the wire is handled automatically.",
	}
	props["modsec_data_ids"] = modSecDataIDsProperty()

	return Tool{
		Name: "update_cdn_modsec_rule",
		Description: "Replace an existing custom ModSec rule's name, value and referenced data ids by id. This is a " +
			"fast operation: the updated rule is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "rule_id", "name", "modsec_data_ids"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNModSecRuleArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			rule, err := uc.Execute(ctx, app.UpdateCDNModSecRuleInput{
				Credentials: args.domain(),
				ZoneUUID:    args.ZoneUUID,
				RuleID:      args.RuleID,
				Rule: domain.CDNModSecRule{
					Name: args.Name, RuleValue: args.RuleValue, ModSecDataIDs: args.ModSecDataIDs,
				},
			})
			if err != nil {
				return nil, err
			}
			return modSecRuleToMap(*rule), nil
		},
	}
}

func deleteCDNModSecRuleTool(uc *app.DeleteCDNModSecRule) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()
	props["rule_id"] = modSecRuleIDProperty()

	return Tool{
		Name: "delete_cdn_modsec_rule",
		Description: "Permanently delete a custom ModSec rule from a CDN zone by id. This is a fast operation and " +
			"cannot be undone. Deleting a rule that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "zone_uuid", "rule_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args modSecRuleIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteCDNModSecRuleInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID, RuleID: args.RuleID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "zone_uuid": args.ZoneUUID, "rule_id": args.RuleID}, nil
		},
	}
}
