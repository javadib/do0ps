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

func validateZoneUUID(zoneUUID string) error {
	if zoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// --- Antivirus ---------------------------------------------------------

// antivirusStatusProvider is the slice of ports.ParspackProvider that
// GetCDNAntivirusStatus needs.
type antivirusStatusProvider interface {
	GetCDNAntivirusStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (bool, error)
}

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
	provider antivirusStatusProvider
}

// NewGetCDNAntivirusStatus builds the use case from its ports.
func NewGetCDNAntivirusStatus(queue ports.Queue, provider antivirusStatusProvider) *GetCDNAntivirusStatus {
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

// updateAntivirusStatusProvider is the slice of ports.ParspackProvider that
// UpdateCDNAntivirusStatus needs.
type updateAntivirusStatusProvider interface {
	UpdateCDNAntivirusStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error)
}

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
	provider updateAntivirusStatusProvider
}

// NewUpdateCDNAntivirusStatus builds the use case from its ports.
func NewUpdateCDNAntivirusStatus(queue ports.Queue, provider updateAntivirusStatusProvider) *UpdateCDNAntivirusStatus {
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

// dnsSecStatusProvider is the slice of ports.ParspackProvider that
// GetCDNDNSSecStatus needs.
type dnsSecStatusProvider interface {
	GetCDNDNSSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNDNSSecStatus, error)
}

// GetCDNDNSSecStatusInput identifies the zone whose DNSSEC status to read.
type GetCDNDNSSecStatusInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNDNSSecStatus is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNDNSSecStatus struct {
	queue    ports.Queue
	provider dnsSecStatusProvider
}

// NewGetCDNDNSSecStatus builds the use case from its ports.
func NewGetCDNDNSSecStatus(queue ports.Queue, provider dnsSecStatusProvider) *GetCDNDNSSecStatus {
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

// updateDNSSecStatusProvider is the slice of ports.ParspackProvider that
// UpdateCDNDNSSecStatus needs.
type updateDNSSecStatusProvider interface {
	UpdateCDNDNSSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (*domain.CDNDNSSecStatus, error)
}

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
	provider updateDNSSecStatusProvider
}

// NewUpdateCDNDNSSecStatus builds the use case from its ports.
func NewUpdateCDNDNSSecStatus(queue ports.Queue, provider updateDNSSecStatusProvider) *UpdateCDNDNSSecStatus {
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

// optimizationStatusProvider is the slice of ports.ParspackProvider that
// GetCDNOptimizationStatus needs.
type optimizationStatusProvider interface {
	GetCDNOptimizationStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNOptimizationStatus, error)
}

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
	provider optimizationStatusProvider
}

// NewGetCDNOptimizationStatus builds the use case from its ports.
func NewGetCDNOptimizationStatus(queue ports.Queue, provider optimizationStatusProvider) *GetCDNOptimizationStatus {
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

// updateOptimizationProvider is the slice of ports.ParspackProvider that
// UpdateCDNOptimization needs.
type updateOptimizationProvider interface {
	UpdateCDNOptimization(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, status domain.CDNOptimizationStatus) (*domain.CDNOptimizationStatus, error)
}

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
	provider updateOptimizationProvider
}

// NewUpdateCDNOptimization builds the use case from its ports.
func NewUpdateCDNOptimization(queue ports.Queue, provider updateOptimizationProvider) *UpdateCDNOptimization {
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

// updateDeveloperModeProvider is the slice of ports.ParspackProvider that
// UpdateCDNDeveloperMode needs.
type updateDeveloperModeProvider interface {
	UpdateCDNDeveloperMode(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error)
}

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
	provider updateDeveloperModeProvider
}

// NewUpdateCDNDeveloperMode builds the use case from its ports.
func NewUpdateCDNDeveloperMode(queue ports.Queue, provider updateDeveloperModeProvider) *UpdateCDNDeveloperMode {
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

// updateMaintenanceModeProvider is the slice of ports.ParspackProvider that
// UpdateCDNMaintenanceMode needs.
type updateMaintenanceModeProvider interface {
	UpdateCDNMaintenanceMode(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error)
}

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
	provider updateMaintenanceModeProvider
}

// NewUpdateCDNMaintenanceMode builds the use case from its ports.
func NewUpdateCDNMaintenanceMode(queue ports.Queue, provider updateMaintenanceModeProvider) *UpdateCDNMaintenanceMode {
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

// updateQueryStringSettingProvider is the slice of ports.ParspackProvider
// that UpdateCDNQueryStringSetting needs.
type updateQueryStringSettingProvider interface {
	UpdateCDNQueryStringSetting(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error)
}

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
	provider updateQueryStringSettingProvider
}

// NewUpdateCDNQueryStringSetting builds the use case from its ports.
func NewUpdateCDNQueryStringSetting(queue ports.Queue, provider updateQueryStringSettingProvider) *UpdateCDNQueryStringSetting {
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

// updateOriginOfflineProvider is the slice of ports.ParspackProvider that
// UpdateCDNOriginOffline needs.
type updateOriginOfflineProvider interface {
	UpdateCDNOriginOffline(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error)
}

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
	provider updateOriginOfflineProvider
}

// NewUpdateCDNOriginOffline builds the use case from its ports.
func NewUpdateCDNOriginOffline(queue ports.Queue, provider updateOriginOfflineProvider) *UpdateCDNOriginOffline {
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
