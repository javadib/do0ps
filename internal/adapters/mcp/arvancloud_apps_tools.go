package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud CDN Apps marketplace tools (issue #77): installable edge
// add-ons a domain owner can browse, install, and uninstall. All fast
// operations (AGENTS.md 4.3): every tool below returns its result within
// the call, with no operation_id to poll afterward.

// arvanCloudCdnAppIDProperty describes the app ID parameter shared by
// domain-scoped app tools.
func arvanCloudCdnAppIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The CDN app's provider-assigned ID (a UUID), as returned by list_arvancloud_apps or get_arvancloud_app.",
	}
}

// arvanCloudCdnAppToMap renders a domain.ArvanCloudCdnApp the way every
// app-returning tool reports it back to the caller.
func arvanCloudCdnAppToMap(app domain.ArvanCloudCdnApp) map[string]any {
	categories := make([]map[string]any, len(app.Categories))
	for i, c := range app.Categories {
		categories[i] = map[string]any{
			"id":     c.ID,
			"name":   c.Name,
			"active": c.Active,
			"order":  c.Order,
		}
	}
	return map[string]any{
		"id":                app.ID,
		"categories":        categories,
		"rank":              app.Rank,
		"name":              app.Name,
		"slug":              app.Slug,
		"short_description": app.ShortDescription,
		"description":       app.Description,
		"logo":              app.Logo,
		"pictures":          app.Pictures,
		"vendor":            app.Vendor,
		"support_email":     app.SupportEmail,
		"install_json":      app.InstallJSON,
		"status":            string(app.Status),
		"like_stats": map[string]any{
			"likes_count":    app.LikeStats.LikesCount,
			"dislikes_count": app.LikeStats.DislikesCount,
		},
		"like_by_account": app.LikeByAccount,
		"created_at":      app.CreatedAt,
		"updated_at":      app.UpdatedAt,
	}
}

// arvanCloudCdnAppCategoryToMap renders a domain.ArvanCloudAppCategory.
func arvanCloudCdnAppCategoryToMap(cat domain.ArvanCloudAppCategory) map[string]any {
	out := map[string]any{
		"id":     cat.ID,
		"name":   cat.Name,
		"active": cat.Active,
		"order":  cat.Order,
	}
	if len(cat.NameTranslation) > 0 {
		translation := make(map[string]any, len(cat.NameTranslation))
		for lang, t := range cat.NameTranslation {
			translation[lang] = map[string]any{"name": t.Name}
		}
		out["name_translation"] = translation
	}
	return out
}

// --- Catalog tools -----------------------------------------------------

func listArvanCloudCdnAppsTool(uc *app.ListArvanCloudCdnApps) Tool {
	props := credentialProperties()
	props["include_draft"] = map[string]any{
		"type":        "boolean",
		"description": "Include unpublished (draft) apps in the result. Defaults to false, so only published apps are shown to end users.",
	}

	return Tool{
		Name: "list_arvancloud_apps",
		Description: "List all available CDN apps in the ArvanCloud marketplace. By default only published apps are returned " +
			"(draft/internal-testing apps are filtered out). Pass include_draft=true to see all apps. " +
			"This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				IncludeDraft *bool `json:"include_draft"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			includeDraft := false
			if args.IncludeDraft != nil {
				includeDraft = *args.IncludeDraft
			}

			apps, err := uc.Execute(ctx, app.ListArvanCloudCdnAppsInput{
				Credentials:  args.domain(),
				IncludeDraft: includeDraft,
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(apps))
			for i, a := range apps {
				out[i] = arvanCloudCdnAppToMap(a)
			}
			return map[string]any{"apps": out}, nil
		},
	}
}

func getArvanCloudCdnAppTool(uc *app.GetArvanCloudCdnApp) Tool {
	props := credentialProperties()
	props["app_id"] = arvanCloudCdnAppIDProperty()

	return Tool{
		Name:        "get_arvancloud_app",
		Description: "Get one CDN app's marketplace listing by ID. Returns the full app metadata including description, vendor, and configurable options schema. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "app_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				AppID string `json:"app_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudCdnAppInput{Credentials: args.domain(), AppID: args.AppID})
			if err != nil {
				return nil, err
			}
			return arvanCloudCdnAppToMap(*found), nil
		},
	}
}

func likeArvanCloudCdnAppTool(uc *app.LikeArvanCloudCdnApp) Tool {
	props := credentialProperties()
	props["app_id"] = arvanCloudCdnAppIDProperty()
	props["like"] = map[string]any{
		"type":        "boolean",
		"nullable":    true,
		"description": "True to like, False to dislike, or null to retract a previous vote.",
	}

	return Tool{
		Name: "like_arvancloud_app",
		Description: "Express a like, dislike, or vote retraction for a CDN app in the marketplace. " +
			"Returns the updated aggregate like/dislike counts. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "app_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				AppID string `json:"app_id"`
				Like  *bool `json:"like"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			stats, err := uc.Execute(ctx, app.LikeArvanCloudCdnAppInput{
				Credentials: args.domain(), AppID: args.AppID, Like: args.Like,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"likes_count":    stats.LikesCount,
				"dislikes_count": stats.DislikesCount,
			}, nil
		},
	}
}

func listArvanCloudCdnAppCategoriesTool(uc *app.ListArvanCloudCdnAppCategories) Tool {
	props := credentialProperties()

	return Tool{
		Name:        "list_arvancloud_app_categories",
		Description: "List all CDN app marketplace categories. Each category groups related apps (e.g. \"Analytics\", \"Security\"). This is a fast operation.",
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

			cats, err := uc.Execute(ctx, app.ListArvanCloudCdnAppCategoriesInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(cats))
			for i, c := range cats {
				out[i] = arvanCloudCdnAppCategoryToMap(c)
			}
			return map[string]any{"categories": out}, nil
		},
	}
}

func getArvanCloudCdnAppCategoryTool(uc *app.GetArvanCloudCdnAppCategory) Tool {
	props := credentialProperties()
	props["category_id"] = map[string]any{
		"type":        "string",
		"description": "The category's ID, as returned by list_arvancloud_app_categories.",
	}

	return Tool{
		Name:        "get_arvancloud_app_category",
		Description: "Get one CDN app marketplace category by ID. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "category_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				CategoryID string `json:"category_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			cat, err := uc.Execute(ctx, app.GetArvanCloudCdnAppCategoryInput{
				Credentials: args.domain(), CategoryID: args.CategoryID,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudCdnAppCategoryToMap(*cat), nil
		},
	}
}

// --- Domain-scoped tools -----------------------------------------------

func listArvanCloudDomainCdnAppsTool(uc *app.ListArvanCloudDomainCdnApps) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_domain_apps",
		Description: "List all CDN apps currently installed on a domain. This is the domain-scoped counterpart of list_arvancloud_apps (which shows the full catalog). " +
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

			apps, err := uc.Execute(ctx, app.ListArvanCloudDomainCdnAppsInput{
				Credentials: args.domain(), Domain: args.Domain,
			})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(apps))
			for i, a := range apps {
				out[i] = arvanCloudCdnAppToMap(a)
			}
			return map[string]any{"apps": out}, nil
		},
	}
}

func checkArvanCloudCdnAppInstalledTool(uc *app.CheckArvanCloudCdnAppInstalled) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["app_id"] = arvanCloudCdnAppIDProperty()

	return Tool{
		Name: "check_arvancloud_app_installed",
		Description: "Check whether a specific CDN app is installed on a domain. Returns {\"installed\": true/false}. " +
			"This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "app_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				AppID string `json:"app_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			installed, err := uc.Execute(ctx, app.CheckArvanCloudCdnAppInstalledInput{
				Credentials: args.domain(), Domain: args.Domain, AppID: args.AppID,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"installed": installed}, nil
		},
	}
}

func installArvanCloudCdnAppTool(uc *app.InstallArvanCloudCdnApp) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["app_id"] = arvanCloudCdnAppIDProperty()
	props["options"] = map[string]any{
		"type":        "object",
		"description": "App-specific configuration options to supply at install time. The shape depends on the app — check the app's install_json field via get_arvancloud_app to see what options it accepts. Omit or pass {} if the app has no configurable options.",
	}

	return Tool{
		Name: "install_arvancloud_app",
		Description: "Install a CDN app on a domain. Check get_arvancloud_app's install_json field to see what options the app accepts. " +
			"This is a fast operation: the installation record, including its provider-assigned ID, is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "app_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				AppID   string         `json:"app_id"`
				Options map[string]any `json:"options"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			installed, err := uc.Execute(ctx, app.InstallArvanCloudCdnAppInput{
				Credentials: args.domain(), Domain: args.Domain, AppID: args.AppID, Options: args.Options,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"id":             installed.ID,
				"domain_id":      installed.DomainID,
				"application_id": installed.ApplicationID,
				"active":         installed.Active,
				"options":        installed.Options,
				"created_at":     installed.CreatedAt,
				"updated_at":     installed.UpdatedAt,
			}, nil
		},
	}
}

func uninstallArvanCloudCdnAppTool(uc *app.UninstallArvanCloudCdnApp) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["app_id"] = arvanCloudCdnAppIDProperty()

	return Tool{
		Name: "uninstall_arvancloud_app",
		Description: "Uninstall a CDN app from a domain. This is a fast operation and cannot be undone. " +
			"Uninstalling an app that is not installed is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "app_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				AppID string `json:"app_id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.UninstallArvanCloudCdnAppInput{
				Credentials: args.domain(), Domain: args.Domain, AppID: args.AppID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "app_id": args.AppID}, nil
		},
	}
}

func triggerArvanCloudCdnAppWebhookTool(uc *app.TriggerArvanCloudCdnAppWebhook) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["app_id"] = arvanCloudCdnAppIDProperty()
	props["event"] = map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudTriggerWebhookBeforeNewInstall), string(domain.ArvanCloudTriggerWebhookNewInstall)},
		"description": "The webhook event to trigger: \"before-new-install\" or \"new-install\".",
	}
	props["options"] = map[string]any{
		"type":        "object",
		"nullable":    true,
		"description": "Optional event-specific configuration. Check the app's documentation for supported options. May be omitted.",
	}

	return Tool{
		Name: "trigger_arvancloud_app_webhook",
		Description: "Manually trigger a CDN app's webhook event on a domain. Use this to test or re-fire an app's event handler. " +
			"This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "app_id", "event"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				AppID   string         `json:"app_id"`
				Event   string         `json:"event"`
				Options map[string]any `json:"options"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.TriggerArvanCloudCdnAppWebhookInput{
				Credentials: args.domain(), Domain: args.Domain, AppID: args.AppID,
				Event: domain.ArvanCloudTriggerWebhookEvent(args.Event), Options: args.Options,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"triggered": true, "domain": args.Domain, "app_id": args.AppID, "event": args.Event}, nil
		},
	}
}
