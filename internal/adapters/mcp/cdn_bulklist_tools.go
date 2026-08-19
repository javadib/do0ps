package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// bulklistToMap renders a domain.CDNBulklist the way every bulklist-returning
// tool reports it back to the caller.
func bulklistToMap(list domain.CDNBulklist) map[string]any {
	items := make([]map[string]any, len(list.Items))
	for i, it := range list.Items {
		items[i] = map[string]any{"value": it.Value, "value_detail": it.ValueDetail}
	}
	return map[string]any{
		"bulklist_id": list.ID,
		"name":        list.Name,
		"type":        list.Type,
		"items":       items,
	}
}

func bulklistTypeProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{"ip", "user_agent", "country", "referral"},
		"description": "The kind of value this bulklist holds.",
	}
}

func bulklistItemsProperty() map[string]any {
	return map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Values in the bulklist, e.g. [\"192.168.0.1\", \"203.0.113.0/24\"] for type \"ip\". At least one value is required.",
	}
}

func bulklistIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The bulklist's ID, as returned by create_cdn_bulklist or list_cdn_bulklists.",
	}
}

func listCDNBulklistsTool(uc *app.ListCDNBulklists) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_cdn_bulklists",
		Description: "List every bulklist (reusable IP/country/user-agent/referral list) on the account behind the " +
			"given Parspack credentials. Bulklists are account-level, not tied to a specific CDN zone; other CDN " +
			"features such as firewall rules reference them by ID. This is a fast operation.",
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

			lists, err := uc.Execute(ctx, app.ListCDNBulklistsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(lists))
			for i, list := range lists {
				out[i] = bulklistToMap(list)
			}
			return map[string]any{"bulklists": out}, nil
		},
	}
}

type createCDNBulklistArgs struct {
	credentialArgs
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Items []string `json:"items"`
}

func createCDNBulklistTool(uc *app.CreateCDNBulklist) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Name of the bulklist, e.g. \"Blocked IPs\".",
	}
	props["type"] = bulklistTypeProperty()
	props["items"] = bulklistItemsProperty()

	return Tool{
		Name: "create_cdn_bulklist",
		Description: "Create a new bulklist (reusable IP/country/user-agent/referral list) on the account behind the " +
			"given Parspack credentials, for other CDN features (e.g. firewall rules) to reference by ID. This is a " +
			"fast operation: the created bulklist is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name", "type", "items"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createCDNBulklistArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			list, err := uc.Execute(ctx, app.CreateCDNBulklistInput{
				Credentials: args.domain(),
				Spec:        domain.CDNBulklistSpec{Name: args.Name, Type: args.Type, Items: args.Items},
			})
			if err != nil {
				return nil, err
			}
			return bulklistToMap(*list), nil
		},
	}
}

type bulklistIDArgs struct {
	credentialArgs
	BulklistID string `json:"bulklist_id"`
}

func getCDNBulklistTool(uc *app.GetCDNBulklist) Tool {
	props := credentialProperties()
	props["bulklist_id"] = bulklistIDProperty()

	return Tool{
		Name: "get_cdn_bulklist",
		Description: "Get the current state of one bulklist by its ID. This is a fast operation: the result is " +
			"returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "bulklist_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args bulklistIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			list, err := uc.Execute(ctx, app.GetCDNBulklistInput{Credentials: args.domain(), BulklistID: args.BulklistID})
			if err != nil {
				return nil, err
			}
			return bulklistToMap(*list), nil
		},
	}
}

type updateCDNBulklistArgs struct {
	credentialArgs
	BulklistID string   `json:"bulklist_id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Items      []string `json:"items"`
}

func updateCDNBulklistTool(uc *app.UpdateCDNBulklist) Tool {
	props := credentialProperties()
	props["bulklist_id"] = bulklistIDProperty()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "New name of the bulklist, e.g. \"Blocked IPs\".",
	}
	props["type"] = bulklistTypeProperty()
	props["items"] = bulklistItemsProperty()

	return Tool{
		Name: "update_cdn_bulklist",
		Description: "Replace the name, type and items of an existing bulklist by its ID. This is a fast operation: " +
			"the updated bulklist is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "bulklist_id", "name", "type", "items"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateCDNBulklistArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			list, err := uc.Execute(ctx, app.UpdateCDNBulklistInput{
				Credentials: args.domain(),
				BulklistID:  args.BulklistID,
				Spec:        domain.CDNBulklistSpec{Name: args.Name, Type: args.Type, Items: args.Items},
			})
			if err != nil {
				return nil, err
			}
			return bulklistToMap(*list), nil
		},
	}
}

func deleteCDNBulklistTool(uc *app.DeleteCDNBulklist) Tool {
	props := credentialProperties()
	props["bulklist_id"] = bulklistIDProperty()

	return Tool{
		Name: "delete_cdn_bulklist",
		Description: "Permanently delete a bulklist by its ID. This is a fast operation and cannot be undone. " +
			"Deleting a bulklist that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "bulklist_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args bulklistIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteCDNBulklistInput{Credentials: args.domain(), BulklistID: args.BulklistID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "bulklist_id": args.BulklistID}, nil
		},
	}
}

func listCDNFirewallCountriesTool(uc *app.ListCDNFirewallCountries) Tool {
	props := credentialProperties()
	props["zone_uuid"] = zoneUUIDProperty()

	return Tool{
		Name: "list_cdn_firewall_countries",
		Description: "List the reference table of countries usable when configuring country-based firewall rules " +
			"for a Parspack CDN zone. This is a fast operation.",
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

			countries, err := uc.Execute(ctx, app.ListCDNFirewallCountriesInput{Credentials: args.domain(), ZoneUUID: args.ZoneUUID})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(countries))
			for i, c := range countries {
				out[i] = map[string]any{"code": c.Code, "name": c.Name}
			}
			return map[string]any{"zone_uuid": args.ZoneUUID, "countries": out}, nil
		},
	}
}
