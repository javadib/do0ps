package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the CDN Apps marketplace use cases for ArvanCloud (issue
// #77): a catalog of installable edge add-ons a domain owner can browse,
// install, and uninstall — see domain/arvancloud_apps.go's package comment.
// Every use case is a fast operation (ports.ArvanCloudProvider, AGENTS.md 4.3):
// each dispatches onto the queue and blocks for the result within the same
// tool call.

// --- Catalog operations ------------------------------------------------

// ListArvanCloudCdnAppsInput identifies the catalog filter. When
// IncludeDraft is false (the default), the use case filters out draft
// apps before returning — matching the issue's requirement that
// list_arvancloud_apps should not surface unpublished apps by default.
type ListArvanCloudCdnAppsInput struct {
	Credentials  domain.ProviderCredentials
	IncludeDraft bool
}

// ListArvanCloudCdnApps is a fast operation.
type ListArvanCloudCdnApps struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudCdnApps builds the use case from its ports.
func NewListArvanCloudCdnApps(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudCdnApps {
	return &ListArvanCloudCdnApps{queue: queue, provider: provider}
}

// Execute returns every published (or all, if IncludeDraft) marketplace app.
func (uc *ListArvanCloudCdnApps) Execute(ctx context.Context, in ListArvanCloudCdnAppsInput) ([]domain.ArvanCloudCdnApp, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		apps, err := uc.provider.ListArvanCloudCdnApps(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud cdn apps: %w", err)
		}
		return json.Marshal(apps)
	})
	if err != nil {
		return nil, err
	}

	var all []domain.ArvanCloudCdnApp
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, fmt.Errorf("decoding arvancloud cdn app list: %w", err)
	}

	if in.IncludeDraft {
		return all, nil
	}

	// Filter out draft apps.
	filtered := make([]domain.ArvanCloudCdnApp, 0, len(all))
	for _, app := range all {
		if app.Status == domain.ArvanCloudCdnAppStatusPublished {
			filtered = append(filtered, app)
		}
	}
	return filtered, nil
}

// GetArvanCloudCdnAppInput identifies the app to look up.
type GetArvanCloudCdnAppInput struct {
	Credentials domain.ProviderCredentials
	AppID       string
}

// GetArvanCloudCdnApp is a fast operation.
type GetArvanCloudCdnApp struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudCdnApp builds the use case from its ports.
func NewGetArvanCloudCdnApp(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudCdnApp {
	return &GetArvanCloudCdnApp{queue: queue, provider: provider}
}

// Execute returns the marketplace listing for one app.
func (uc *GetArvanCloudCdnApp) Execute(ctx context.Context, in GetArvanCloudCdnAppInput) (*domain.ArvanCloudCdnApp, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.AppID == "" {
		return nil, fmt.Errorf("app_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		found, err := uc.provider.GetArvanCloudCdnApp(ctx, in.Credentials, in.AppID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud cdn app %q: %w", in.AppID, err)
		}
		return json.Marshal(found)
	})
	if err != nil {
		return nil, err
	}

	var app domain.ArvanCloudCdnApp
	if err := json.Unmarshal(raw, &app); err != nil {
		return nil, fmt.Errorf("decoding arvancloud cdn app: %w", err)
	}
	return &app, nil
}

// LikeArvanCloudCdnAppInput carries the vote. Like=nil retracts a
// previous vote; like=true likes; like=false dislikes.
type LikeArvanCloudCdnAppInput struct {
	Credentials domain.ProviderCredentials
	AppID       string
	Like        *bool
}

// LikeArvanCloudCdnApp is a fast operation.
type LikeArvanCloudCdnApp struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewLikeArvanCloudCdnApp builds the use case from its ports.
func NewLikeArvanCloudCdnApp(queue ports.Queue, provider ports.ArvanCloudProvider) *LikeArvanCloudCdnApp {
	return &LikeArvanCloudCdnApp{queue: queue, provider: provider}
}

// Execute expresses a like/dislike/retraction and returns the updated
// aggregate like stats.
func (uc *LikeArvanCloudCdnApp) Execute(ctx context.Context, in LikeArvanCloudCdnAppInput) (*domain.ArvanCloudCdnAppLikeStats, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.AppID == "" {
		return nil, fmt.Errorf("app_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		stats, err := uc.provider.LikeArvanCloudCdnApp(ctx, in.Credentials, in.AppID, in.Like)
		if err != nil {
			return nil, fmt.Errorf("liking arvancloud cdn app %q: %w", in.AppID, err)
		}
		return json.Marshal(stats)
	})
	if err != nil {
		return nil, err
	}

	var stats domain.ArvanCloudCdnAppLikeStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		return nil, fmt.Errorf("decoding arvancloud cdn app like stats: %w", err)
	}
	return &stats, nil
}

// --- Categories --------------------------------------------------------

// ListArvanCloudCdnAppCategoriesInput carries only credentials: this
// endpoint is account-independent.
type ListArvanCloudCdnAppCategoriesInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudCdnAppCategories is a fast operation.
type ListArvanCloudCdnAppCategories struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudCdnAppCategories builds the use case from its ports.
func NewListArvanCloudCdnAppCategories(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudCdnAppCategories {
	return &ListArvanCloudCdnAppCategories{queue: queue, provider: provider}
}

// Execute returns every marketplace category.
func (uc *ListArvanCloudCdnAppCategories) Execute(ctx context.Context, in ListArvanCloudCdnAppCategoriesInput) ([]domain.ArvanCloudAppCategory, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		cats, err := uc.provider.ListArvanCloudCdnAppCategories(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud cdn app categories: %w", err)
		}
		return json.Marshal(cats)
	})
	if err != nil {
		return nil, err
	}

	var cats []domain.ArvanCloudAppCategory
	if err := json.Unmarshal(raw, &cats); err != nil {
		return nil, fmt.Errorf("decoding arvancloud cdn app category list: %w", err)
	}
	return cats, nil
}

// GetArvanCloudCdnAppCategoryInput identifies the category to look up.
type GetArvanCloudCdnAppCategoryInput struct {
	Credentials domain.ProviderCredentials
	CategoryID  string
}

// GetArvanCloudCdnAppCategory is a fast operation.
type GetArvanCloudCdnAppCategory struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudCdnAppCategory builds the use case from its ports.
func NewGetArvanCloudCdnAppCategory(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudCdnAppCategory {
	return &GetArvanCloudCdnAppCategory{queue: queue, provider: provider}
}

// Execute returns one marketplace category.
func (uc *GetArvanCloudCdnAppCategory) Execute(ctx context.Context, in GetArvanCloudCdnAppCategoryInput) (*domain.ArvanCloudAppCategory, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.CategoryID == "" {
		return nil, fmt.Errorf("category_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		found, err := uc.provider.GetArvanCloudCdnAppCategory(ctx, in.Credentials, in.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud cdn app category %q: %w", in.CategoryID, err)
		}
		return json.Marshal(found)
	})
	if err != nil {
		return nil, err
	}

	var cat domain.ArvanCloudAppCategory
	if err := json.Unmarshal(raw, &cat); err != nil {
		return nil, fmt.Errorf("decoding arvancloud cdn app category: %w", err)
	}
	return &cat, nil
}

// --- Domain-scoped operations ------------------------------------------

// arvanCloudCdnAppDomainInput is embedded by every domain-scoped use case
// below that identifies exactly one domain.
type arvanCloudCdnAppDomainInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

func (in arvanCloudCdnAppDomainInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// ListArvanCloudDomainCdnAppsInput identifies the domain whose installed
// apps to list.
type ListArvanCloudDomainCdnAppsInput = arvanCloudCdnAppDomainInput

// ListArvanCloudDomainCdnApps is a fast operation.
type ListArvanCloudDomainCdnApps struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudDomainCdnApps builds the use case from its ports.
func NewListArvanCloudDomainCdnApps(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudDomainCdnApps {
	return &ListArvanCloudDomainCdnApps{queue: queue, provider: provider}
}

// Execute returns every app installed on the domain.
func (uc *ListArvanCloudDomainCdnApps) Execute(ctx context.Context, in ListArvanCloudDomainCdnAppsInput) ([]domain.ArvanCloudCdnApp, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		apps, err := uc.provider.ListArvanCloudDomainCdnApps(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud cdn apps of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(apps)
	})
	if err != nil {
		return nil, err
	}

	var apps []domain.ArvanCloudCdnApp
	if err := json.Unmarshal(raw, &apps); err != nil {
		return nil, fmt.Errorf("decoding arvancloud cdn app list: %w", err)
	}
	return apps, nil
}

// CheckArvanCloudCdnAppInstalledInput identifies the domain and app.
type CheckArvanCloudCdnAppInstalledInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	AppID       string
}

// CheckArvanCloudCdnAppInstalled is a fast operation.
type CheckArvanCloudCdnAppInstalled struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCheckArvanCloudCdnAppInstalled builds the use case from its ports.
func NewCheckArvanCloudCdnAppInstalled(queue ports.Queue, provider ports.ArvanCloudProvider) *CheckArvanCloudCdnAppInstalled {
	return &CheckArvanCloudCdnAppInstalled{queue: queue, provider: provider}
}

// Execute reports whether the app is installed on the domain.
func (uc *CheckArvanCloudCdnAppInstalled) Execute(ctx context.Context, in CheckArvanCloudCdnAppInstalledInput) (bool, error) {
	if err := in.Credentials.Validate(); err != nil {
		return false, err
	}
	if in.Domain == "" {
		return false, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.AppID == "" {
		return false, fmt.Errorf("app_id is required: %w", domain.ErrInvalidInput)
	}

	queueResult, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		isInstalled, err := uc.provider.CheckArvanCloudCdnAppInstalled(ctx, in.Credentials, in.Domain, in.AppID)
		if err != nil {
			return nil, fmt.Errorf("checking arvancloud cdn app %q installation on domain %q: %w", in.AppID, in.Domain, err)
		}
		type installCheckResult struct {
			Installed bool `json:"installed"`
		}
		return json.Marshal(installCheckResult{Installed: isInstalled})
	})
	if err != nil {
		return false, err
	}

	var installCheck struct {
		Installed bool `json:"installed"`
	}
	if err := json.Unmarshal(queueResult, &installCheck); err != nil {
		return false, fmt.Errorf("decoding arvancloud cdn app install check: %w", err)
	}
	return installCheck.Installed, nil
}

// InstallArvanCloudCdnAppInput identifies the domain and app, plus
// optional configuration options.
type InstallArvanCloudCdnAppInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	AppID       string
	Options     map[string]any
}

// InstallArvanCloudCdnApp is a fast operation.
type InstallArvanCloudCdnApp struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewInstallArvanCloudCdnApp builds the use case from its ports.
func NewInstallArvanCloudCdnApp(queue ports.Queue, provider ports.ArvanCloudProvider) *InstallArvanCloudCdnApp {
	return &InstallArvanCloudCdnApp{queue: queue, provider: provider}
}

// Execute installs the app and returns the installation record.
func (uc *InstallArvanCloudCdnApp) Execute(ctx context.Context, in InstallArvanCloudCdnAppInput) (*domain.ArvanCloudDomainCdnApp, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.AppID == "" {
		return nil, fmt.Errorf("app_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		installed, err := uc.provider.InstallArvanCloudCdnApp(ctx, in.Credentials, in.Domain, in.AppID, in.Options)
		if err != nil {
			return nil, fmt.Errorf("installing arvancloud cdn app %q on domain %q: %w", in.AppID, in.Domain, err)
		}
		return json.Marshal(installed)
	})
	if err != nil {
		return nil, err
	}

	var installed domain.ArvanCloudDomainCdnApp
	if err := json.Unmarshal(raw, &installed); err != nil {
		return nil, fmt.Errorf("decoding arvancloud cdn app installation: %w", err)
	}
	return &installed, nil
}

// UninstallArvanCloudCdnAppInput identifies the domain and app to remove.
type UninstallArvanCloudCdnAppInput = CheckArvanCloudCdnAppInstalledInput

// UninstallArvanCloudCdnApp is a fast operation. Uninstalling an app that
// is not installed is treated as already done rather than an error.
type UninstallArvanCloudCdnApp struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUninstallArvanCloudCdnApp builds the use case from its ports.
func NewUninstallArvanCloudCdnApp(queue ports.Queue, provider ports.ArvanCloudProvider) *UninstallArvanCloudCdnApp {
	return &UninstallArvanCloudCdnApp{queue: queue, provider: provider}
}

// Execute removes the app from the domain, tolerating one that is not
// installed.
func (uc *UninstallArvanCloudCdnApp) Execute(ctx context.Context, in UninstallArvanCloudCdnAppInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.AppID == "" {
		return fmt.Errorf("app_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UninstallArvanCloudCdnApp(ctx, in.Credentials, in.Domain, in.AppID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("uninstalling arvancloud cdn app %q from domain %q: %w", in.AppID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// TriggerArvanCloudCdnAppWebhookInput identifies the domain, app, and
// event to trigger, plus optional event options.
type TriggerArvanCloudCdnAppWebhookInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	AppID       string
	Event       domain.ArvanCloudTriggerWebhookEvent
	Options     map[string]any
}

// TriggerArvanCloudCdnAppWebhook is a fast operation.
type TriggerArvanCloudCdnAppWebhook struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewTriggerArvanCloudCdnAppWebhook builds the use case from its ports.
func NewTriggerArvanCloudCdnAppWebhook(queue ports.Queue, provider ports.ArvanCloudProvider) *TriggerArvanCloudCdnAppWebhook {
	return &TriggerArvanCloudCdnAppWebhook{queue: queue, provider: provider}
}

// Execute fires the webhook event.
func (uc *TriggerArvanCloudCdnAppWebhook) Execute(ctx context.Context, in TriggerArvanCloudCdnAppWebhookInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.AppID == "" {
		return fmt.Errorf("app_id is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudTriggerWebhookEvent(string(in.Event)) {
		return fmt.Errorf("event %q is not one of before-new-install/new-install: %w", in.Event, domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.TriggerArvanCloudCdnAppWebhook(ctx, in.Credentials, in.Domain, in.AppID, in.Event, in.Options); err != nil {
			return nil, fmt.Errorf("triggering arvancloud cdn app %q webhook on domain %q: %w", in.AppID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
