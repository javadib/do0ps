package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud Lists tools (issue #64): a reusable, account-scoped collection
// of values other CDN capabilities (firewall, WAF, DDoS protection, rate
// limiting — AC5-AC8) reference by ID from their own filter/source fields.
// All fast operations (AGENTS.md 4.3): every tool below returns its result
// within the call, with no operation_id to poll afterward.
//
// Unlike domain/DNS tools, a list is account-scoped, not scoped to a domain
// by name — there is no "domain" argument on any tool here.

// arvanCloudDynamicFieldValueToMap renders one list item the way every
// list-returning tool reports it.
func arvanCloudDynamicFieldValueToMap(v domain.ArvanCloudDynamicFieldValue) map[string]any {
	return map[string]any{
		"id":         v.ID,
		"value":      v.Value,
		"desc":       v.Desc,
		"created_at": v.CreatedAt,
	}
}

// arvanCloudDynamicFieldToMap renders a domain.ArvanCloudDynamicField the way
// every list-returning tool reports it back to the caller.
func arvanCloudDynamicFieldToMap(f domain.ArvanCloudDynamicField) map[string]any {
	values := make([]map[string]any, len(f.Values))
	for i, v := range f.Values {
		values[i] = arvanCloudDynamicFieldValueToMap(v)
	}
	return map[string]any{
		"id":            f.ID,
		"name":          f.Name,
		"description":   f.Description,
		"namespace":     f.Namespace,
		"type":          string(f.Type),
		"scope":         string(f.Scope),
		"values":        values,
		"allowed_plans": f.AllowedPlans,
		"created_at":    f.CreatedAt,
		"updated_at":    f.UpdatedAt,
	}
}

// arvanCloudDynamicFieldTypeProperty is repeated on the tools that need to
// specify a list's value type.
func arvanCloudDynamicFieldTypeProperty(description string) map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudDynamicFieldTypeIP),
			string(domain.ArvanCloudDynamicFieldTypeNumber),
			string(domain.ArvanCloudDynamicFieldTypeByte),
		},
		"description": description,
	}
}

// arvanCloudDynamicFieldIDArgs is embedded by every tool below that is
// scoped to exactly one list by id and needs nothing else.
type arvanCloudDynamicFieldIDArgs struct {
	credentialArgs
	ID string `json:"id"`
}

func arvanCloudDynamicFieldIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The list's provider-assigned ID (a UUID), as returned by create_arvancloud_dynamic_field or list_arvancloud_dynamic_fields.",
	}
}

// arvanCloudDynamicFieldValueProperty is the JSON Schema for one item in a
// list's "values" array, shared by create and add-items.
func arvanCloudDynamicFieldValueProperty() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{
				"description": "The item's actual value: an IP address/CIDR string for an \"ip\" list, a number for a " +
					"\"number\" list, or a string for a \"byte\" list. Its shape must match the list's own type.",
			},
			"desc": map[string]any{
				"type":        "string",
				"description": "An optional note about this item, e.g. why it was added.",
			},
		},
		"required": []string{"value"},
	}
}

func listArvanCloudDynamicFieldsTool(uc *app.ListArvanCloudDynamicFields) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_arvancloud_dynamic_fields",
		Description: "List every ArvanCloud \"List\" (called \"dynamic-fields\" in the API) visible to the given " +
			"credentials — both the account's own private lists and ArvanCloud's public ones. Lists are reusable, " +
			"account-scoped collections of values (IPs, numbers, or strings) that firewall, WAF, DDoS protection and " +
			"rate-limit rules can reference by ID. This is a fast operation.",
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

			fields, err := uc.Execute(ctx, app.ListArvanCloudDynamicFieldsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(fields))
			for i, f := range fields {
				out[i] = arvanCloudDynamicFieldToMap(f)
			}
			return map[string]any{"dynamic_fields": out}, nil
		},
	}
}

func createArvanCloudDynamicFieldTool(uc *app.CreateArvanCloudDynamicField) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "A label for the new list, e.g. \"known-scanners\".",
	}
	props["description"] = map[string]any{
		"type":        "string",
		"description": "An optional note about what this list is for.",
	}
	props["type"] = arvanCloudDynamicFieldTypeProperty(
		"The kind of value every item in this list holds. Fixed once the list is created — every item added later " +
			"must match this type.")
	props["values"] = map[string]any{
		"type":        "array",
		"items":       arvanCloudDynamicFieldValueProperty(),
		"description": "Items to seed the list with, if any. Leave empty to create an empty list and add items later with add_arvancloud_dynamic_field_items.",
	}

	return Tool{
		Name: "create_arvancloud_dynamic_field",
		Description: "Create a new ArvanCloud List (\"dynamic-fields\") of a fixed value type (ip/number/byte), " +
			"optionally seeded with items. This is a fast operation: the created list, including its provider-assigned " +
			"ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name", "type"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				Name        string                    `json:"name"`
				Description string                    `json:"description"`
				Type        string                    `json:"type"`
				Values      []arvanCloudDynFieldValue `json:"values"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudDynamicFieldInput{
				Credentials: args.domain(),
				Field: domain.ArvanCloudDynamicField{
					Name:        args.Name,
					Description: args.Description,
					Type:        domain.ArvanCloudDynamicFieldType(args.Type),
					Values:      toDynamicFieldValuesDomain(args.Values),
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDynamicFieldToMap(*created), nil
		},
	}
}

func getArvanCloudDynamicFieldTool(uc *app.GetArvanCloudDynamicField) Tool {
	props := credentialProperties()
	props["id"] = arvanCloudDynamicFieldIDProperty()

	return Tool{
		Name:        "get_arvancloud_dynamic_field",
		Description: "Get the current state of one ArvanCloud List by ID, including all of its items. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDynamicFieldIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudDynamicFieldInput{Credentials: args.domain(), ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudDynamicFieldToMap(*found), nil
		},
	}
}

func updateArvanCloudDynamicFieldTool(uc *app.UpdateArvanCloudDynamicField) Tool {
	props := credentialProperties()
	props["id"] = arvanCloudDynamicFieldIDProperty()
	props["description"] = map[string]any{
		"type":        "string",
		"description": "The list's new description.",
	}
	props["type"] = arvanCloudDynamicFieldTypeProperty(
		"The list's value type. ArvanCloud's update endpoint requires this field on every call even though it " +
			"documents no way to actually change a list's type after creation — pass the list's current type here " +
			"(e.g. from get_arvancloud_dynamic_field) when only changing the description.")

	return Tool{
		Name: "update_arvancloud_dynamic_field",
		Description: "Update an ArvanCloud List's description. The underlying API call also requires the list's " +
			"type to be repeated in the request (see the type parameter); it cannot rename a list — there is no way " +
			"to change name via this or any other endpoint. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "id", "type"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDynamicFieldIDArgs
				Description string `json:"description"`
				Type        string `json:"type"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudDynamicFieldInput{
				Credentials: args.domain(), ID: args.ID, Description: args.Description,
				Type: domain.ArvanCloudDynamicFieldType(args.Type),
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDynamicFieldToMap(*updated), nil
		},
	}
}

func deleteArvanCloudDynamicFieldTool(uc *app.DeleteArvanCloudDynamicField) Tool {
	props := credentialProperties()
	props["id"] = arvanCloudDynamicFieldIDProperty()

	return Tool{
		Name: "delete_arvancloud_dynamic_field",
		Description: "Permanently delete an ArvanCloud List by ID, including all of its items. This is a fast " +
			"operation and cannot be undone. Any firewall/WAF/DDoS/rate-limit rule still referencing this list will " +
			"lose that reference — check first if unsure. Deleting a list that no longer exists is treated as " +
			"already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDynamicFieldIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudDynamicFieldInput{Credentials: args.domain(), ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "id": args.ID}, nil
		},
	}
}

func addArvanCloudDynamicFieldItemsTool(uc *app.AddArvanCloudDynamicFieldItems) Tool {
	props := credentialProperties()
	props["id"] = arvanCloudDynamicFieldIDProperty()
	props["values"] = map[string]any{
		"type":        "array",
		"items":       arvanCloudDynamicFieldValueProperty(),
		"minItems":    1,
		"description": "One or more items to add to the list. Each value's shape must match the list's own type.",
	}

	return Tool{
		Name: "add_arvancloud_dynamic_field_items",
		Description: "Add one or more items to an existing ArvanCloud List. This is a fast operation. The " +
			"provider's response confirms success but does not report the new items' assigned IDs — call " +
			"get_arvancloud_dynamic_field afterward if those are needed (e.g. to remove one later).",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "id", "values"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDynamicFieldIDArgs
				Values []arvanCloudDynFieldValue `json:"values"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.AddArvanCloudDynamicFieldItemsInput{
				Credentials: args.domain(), ID: args.ID, Values: toDynamicFieldValuesDomain(args.Values),
			}); err != nil {
				return nil, err
			}
			return map[string]any{"added": true, "id": args.ID, "count": len(args.Values)}, nil
		},
	}
}

func removeArvanCloudDynamicFieldItemTool(uc *app.RemoveArvanCloudDynamicFieldItem) Tool {
	props := credentialProperties()
	props["id"] = arvanCloudDynamicFieldIDProperty()
	props["item_id"] = map[string]any{
		"type":        "string",
		"description": "The item's own provider-assigned ID (a UUID, from get_arvancloud_dynamic_field's values) — not an index or the value itself.",
	}

	return Tool{
		Name: "remove_arvancloud_dynamic_field_item",
		Description: "Remove one item from an ArvanCloud List by the item's own ID. This is a fast operation. " +
			"Removing an item that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "id", "item_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDynamicFieldIDArgs
				ItemID string `json:"item_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.RemoveArvanCloudDynamicFieldItemInput{
				Credentials: args.domain(), ID: args.ID, ItemID: args.ItemID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"removed": true, "id": args.ID, "item_id": args.ItemID}, nil
		},
	}
}

// arvanCloudDynFieldValue is the tool-argument shape of one list item, as
// submitted by a caller on create/add-items — narrower than
// domain.ArvanCloudDynamicFieldValue, since a caller never supplies ID or
// CreatedAt (the provider assigns both).
type arvanCloudDynFieldValue struct {
	Value any    `json:"value"`
	Desc  string `json:"desc"`
}

// toDynamicFieldValuesDomain converts tool-argument items to the domain
// shape, always producing a non-nil (possibly empty) slice so a caller's
// deliberately-empty "values": [] on create is sent to the provider as an
// empty array rather than becoming a JSON null.
func toDynamicFieldValuesDomain(values []arvanCloudDynFieldValue) []domain.ArvanCloudDynamicFieldValue {
	out := make([]domain.ArvanCloudDynamicFieldValue, len(values))
	for i, v := range values {
		out[i] = domain.ArvanCloudDynamicFieldValue{Value: v.Value, Desc: v.Desc}
	}
	return out
}
