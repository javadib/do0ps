package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// CDN zone-level settings/toggles beyond issue #19's zone/order/DNS scope
// (issue #24): antivirus, DNSSEC, asset optimization, developer mode,
// maintenance mode, query-string caching behavior, and origin-offline
// handling. Every use case here is a fast operation (AGENTS.md 4.3): each
// is a single provider HTTP round trip with no further polling.
//
// Each use case's provider field is typed as a small local interface,
// deliberately NOT ports.ParspackProvider — see this package's cdn_zone_settings
// integration notes for why (ports.ParspackProvider is being extended
// centrally once every parallel issue #24 slice lands; these local
// interfaces are written so that swap is mechanical: method signatures
// match what a ports.ParspackProvider extension would look like). Concrete
// *parspack.Client already satisfies each of them by structural typing.
//
// Developer mode, maintenance mode, query-string and origin-offline only
// have a documented PUT endpoint (docs/api-specs/parspack-cdn.openapi.yaml,
// "Service Settings" tag) — there is no dedicated single-setting GET for
// any of them, so this file only defines their Update use case, not a Get
// one. See internal/adapters/providers/parspack/cdn_zone_settings.go's
// top-level comment for the full explanation.

// validateZoneUUID (shared by every CDN use case that scopes to one zone) is
// defined once in cdn_cache.go.

// --- Antivirus ---------------------------------------------------------

// GetCDNAntivirusStatus needs.

// GetCDNAntivirusStatusInput identifies the zone whose antivirus status to
// read.
type GetCDNAntivirusStatusInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNAntivirusStatus is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNAntivirusStatus struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNAntivirusStatus builds the use case from its ports.
func NewGetCDNAntivirusStatus(queue ports.Queue, provider ports.ParspackProvider) *GetCDNAntivirusStatus {
	return &GetCDNAntivirusStatus{queue: queue, provider: provider}
}

// Execute returns whether antivirus scanning is enabled for the zone.
func (uc *GetCDNAntivirusStatus) Execute(ctx context.Context, in GetCDNAntivirusStatusInput) (bool, error) {
	if err := in.Credentials.Validate(); err != nil {
		return false, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return false, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		enabled, err := uc.provider.GetCDNAntivirusStatus(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting CDN antivirus status of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(enabled)
	})
	if err != nil {
		return false, err
	}

	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, fmt.Errorf("decoding CDN antivirus status: %w", err)
	}
	return enabled, nil
}

// UpdateCDNAntivirusStatus needs.

// UpdateCDNAntivirusStatusInput is the normalized form of an
// update_cdn_antivirus_status tool call.
type UpdateCDNAntivirusStatusInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Enabled     bool
}

// UpdateCDNAntivirusStatus is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNAntivirusStatus struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNAntivirusStatus builds the use case from its ports.
func NewUpdateCDNAntivirusStatus(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNAntivirusStatus {
	return &UpdateCDNAntivirusStatus{queue: queue, provider: provider}
}

// Execute enables or disables antivirus scanning for the zone, returning
// the applied value.
func (uc *UpdateCDNAntivirusStatus) Execute(ctx context.Context, in UpdateCDNAntivirusStatusInput) (bool, error) {
	if err := in.Credentials.Validate(); err != nil {
		return false, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return false, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		enabled, err := uc.provider.UpdateCDNAntivirusStatus(ctx, in.Credentials, in.ZoneUUID, in.Enabled)
		if err != nil {
			return nil, fmt.Errorf("updating CDN antivirus status of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(enabled)
	})
	if err != nil {
		return false, err
	}

	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, fmt.Errorf("decoding updated CDN antivirus status: %w", err)
	}
	return enabled, nil
}

// --- DNS Sec -------------------------------------------------------------

// GetCDNDNSSecStatus needs.

// GetCDNDNSSecStatusInput identifies the zone whose DNSSEC status to read.
type GetCDNDNSSecStatusInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNDNSSecStatus is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNDNSSecStatus struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNDNSSecStatus builds the use case from its ports.
func NewGetCDNDNSSecStatus(queue ports.Queue, provider ports.ParspackProvider) *GetCDNDNSSecStatus {
	return &GetCDNDNSSecStatus{queue: queue, provider: provider}
}

// Execute returns the zone's current DNSSEC status, including the DS
// record when DNSSEC is enabled.
func (uc *GetCDNDNSSecStatus) Execute(ctx context.Context, in GetCDNDNSSecStatusInput) (*domain.CDNDNSSecStatus, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		status, err := uc.provider.GetCDNDNSSecStatus(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting CDN DNSSEC status of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(status)
	})
	if err != nil {
		return nil, err
	}

	var status domain.CDNDNSSecStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("decoding CDN DNSSEC status: %w", err)
	}
	return &status, nil
}

// UpdateCDNDNSSecStatus needs.

// UpdateCDNDNSSecStatusInput is the normalized form of an
// update_cdn_dnssec_status tool call.
type UpdateCDNDNSSecStatusInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Enabled     bool
}

// UpdateCDNDNSSecStatus is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNDNSSecStatus struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNDNSSecStatus builds the use case from its ports.
func NewUpdateCDNDNSSecStatus(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNDNSSecStatus {
	return &UpdateCDNDNSSecStatus{queue: queue, provider: provider}
}

// Execute enables or disables DNSSEC for the zone, returning the
// resulting status.
func (uc *UpdateCDNDNSSecStatus) Execute(ctx context.Context, in UpdateCDNDNSSecStatusInput) (*domain.CDNDNSSecStatus, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		status, err := uc.provider.UpdateCDNDNSSecStatus(ctx, in.Credentials, in.ZoneUUID, in.Enabled)
		if err != nil {
			return nil, fmt.Errorf("updating CDN DNSSEC status of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(status)
	})
	if err != nil {
		return nil, err
	}

	var status domain.CDNDNSSecStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("decoding updated CDN DNSSEC status: %w", err)
	}
	return &status, nil
}

// --- Optimization --------------------------------------------------------

// GetCDNOptimizationStatus needs.

// GetCDNOptimizationStatusInput identifies the zone whose optimization
// configuration to read.
type GetCDNOptimizationStatusInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNOptimizationStatus is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNOptimizationStatus struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNOptimizationStatus builds the use case from its ports.
func NewGetCDNOptimizationStatus(queue ports.Queue, provider ports.ParspackProvider) *GetCDNOptimizationStatus {
	return &GetCDNOptimizationStatus{queue: queue, provider: provider}
}

// Execute returns the zone's current asset optimization configuration.
func (uc *GetCDNOptimizationStatus) Execute(ctx context.Context, in GetCDNOptimizationStatusInput) (*domain.CDNOptimizationStatus, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		status, err := uc.provider.GetCDNOptimizationStatus(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting CDN optimization status of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(status)
	})
	if err != nil {
		return nil, err
	}

	var status domain.CDNOptimizationStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("decoding CDN optimization status: %w", err)
	}
	return &status, nil
}

// UpdateCDNOptimization needs.

// UpdateCDNOptimizationInput is the normalized form of an
// update_cdn_optimization tool call.
type UpdateCDNOptimizationInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Status      domain.CDNOptimizationStatus
}

// UpdateCDNOptimization is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNOptimization struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNOptimization builds the use case from its ports.
func NewUpdateCDNOptimization(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNOptimization {
	return &UpdateCDNOptimization{queue: queue, provider: provider}
}

// Execute replaces the zone's asset optimization configuration, returning
// the applied configuration.
func (uc *UpdateCDNOptimization) Execute(ctx context.Context, in UpdateCDNOptimizationInput) (*domain.CDNOptimizationStatus, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		status, err := uc.provider.UpdateCDNOptimization(ctx, in.Credentials, in.ZoneUUID, in.Status)
		if err != nil {
			return nil, fmt.Errorf("updating CDN optimization status of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(status)
	})
	if err != nil {
		return nil, err
	}

	var status domain.CDNOptimizationStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("decoding updated CDN optimization status: %w", err)
	}
	return &status, nil
}

// --- Developer mode --------------------------------------------------------

// UpdateCDNDeveloperMode needs.

// UpdateCDNDeveloperModeInput is the normalized form of an
// update_cdn_developer_mode tool call.
type UpdateCDNDeveloperModeInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Enabled     bool
}

// UpdateCDNDeveloperMode is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNDeveloperMode struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNDeveloperMode builds the use case from its ports.
func NewUpdateCDNDeveloperMode(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNDeveloperMode {
	return &UpdateCDNDeveloperMode{queue: queue, provider: provider}
}

// Execute enables or disables developer mode for the zone, returning the
// applied value.
func (uc *UpdateCDNDeveloperMode) Execute(ctx context.Context, in UpdateCDNDeveloperModeInput) (bool, error) {
	if err := in.Credentials.Validate(); err != nil {
		return false, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return false, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		enabled, err := uc.provider.UpdateCDNDeveloperMode(ctx, in.Credentials, in.ZoneUUID, in.Enabled)
		if err != nil {
			return nil, fmt.Errorf("updating CDN developer mode of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(enabled)
	})
	if err != nil {
		return false, err
	}

	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, fmt.Errorf("decoding updated CDN developer mode: %w", err)
	}
	return enabled, nil
}

// --- Maintenance mode ------------------------------------------------------

// UpdateCDNMaintenanceMode needs.

// UpdateCDNMaintenanceModeInput is the normalized form of an
// update_cdn_maintenance_mode tool call.
type UpdateCDNMaintenanceModeInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Enabled     bool
}

// UpdateCDNMaintenanceMode is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNMaintenanceMode struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNMaintenanceMode builds the use case from its ports.
func NewUpdateCDNMaintenanceMode(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNMaintenanceMode {
	return &UpdateCDNMaintenanceMode{queue: queue, provider: provider}
}

// Execute enables or disables maintenance mode for the zone, returning the
// applied value.
func (uc *UpdateCDNMaintenanceMode) Execute(ctx context.Context, in UpdateCDNMaintenanceModeInput) (bool, error) {
	if err := in.Credentials.Validate(); err != nil {
		return false, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return false, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		enabled, err := uc.provider.UpdateCDNMaintenanceMode(ctx, in.Credentials, in.ZoneUUID, in.Enabled)
		if err != nil {
			return nil, fmt.Errorf("updating CDN maintenance mode of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(enabled)
	})
	if err != nil {
		return false, err
	}

	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, fmt.Errorf("decoding updated CDN maintenance mode: %w", err)
	}
	return enabled, nil
}

// --- Query string ----------------------------------------------------------

// UpdateCDNQueryStringSettingInput is the normalized form of an
// update_cdn_query_string_setting tool call.
type UpdateCDNQueryStringSettingInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Enabled     bool
}

// UpdateCDNQueryStringSetting is a fast operation: it runs on a worker but
// the caller waits for the result inside the same tool call.
type UpdateCDNQueryStringSetting struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNQueryStringSetting builds the use case from its ports.
func NewUpdateCDNQueryStringSetting(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNQueryStringSetting {
	return &UpdateCDNQueryStringSetting{queue: queue, provider: provider}
}

// Execute enables or disables "ignore query string" caching behavior for
// the zone, returning the applied value.
func (uc *UpdateCDNQueryStringSetting) Execute(ctx context.Context, in UpdateCDNQueryStringSettingInput) (bool, error) {
	if err := in.Credentials.Validate(); err != nil {
		return false, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return false, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		enabled, err := uc.provider.UpdateCDNQueryStringSetting(ctx, in.Credentials, in.ZoneUUID, in.Enabled)
		if err != nil {
			return nil, fmt.Errorf("updating CDN query string setting of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(enabled)
	})
	if err != nil {
		return false, err
	}

	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, fmt.Errorf("decoding updated CDN query string setting: %w", err)
	}
	return enabled, nil
}

// --- Origin offline ----------------------------------------------------------

// UpdateCDNOriginOffline needs.

// UpdateCDNOriginOfflineInput is the normalized form of an
// update_cdn_origin_offline tool call.
type UpdateCDNOriginOfflineInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Enabled     bool
}

// UpdateCDNOriginOffline is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNOriginOffline struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNOriginOffline builds the use case from its ports.
func NewUpdateCDNOriginOffline(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNOriginOffline {
	return &UpdateCDNOriginOffline{queue: queue, provider: provider}
}

// Execute enables or disables origin-offline handling for the zone,
// returning the applied value.
func (uc *UpdateCDNOriginOffline) Execute(ctx context.Context, in UpdateCDNOriginOfflineInput) (bool, error) {
	if err := in.Credentials.Validate(); err != nil {
		return false, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return false, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		enabled, err := uc.provider.UpdateCDNOriginOffline(ctx, in.Credentials, in.ZoneUUID, in.Enabled)
		if err != nil {
			return nil, fmt.Errorf("updating CDN origin offline setting of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(enabled)
	})
	if err != nil {
		return false, err
	}

	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, fmt.Errorf("decoding updated CDN origin offline setting: %w", err)
	}
	return enabled, nil
}
