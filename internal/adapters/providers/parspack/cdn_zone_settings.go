package parspack

import (
	"context"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// CDN zone-level settings/toggles beyond issue #19's zone/order/DNS scope
// (issue #24): antivirus, DNSSEC, asset optimization, developer mode,
// maintenance mode, query-string caching behavior, and origin-offline
// handling. Every endpoint here is scoped to a single zone
// (zonesBasePath+"/"+zoneUUID+"/...", zonesBasePath is defined in cdn.go)
// and, other than dns-sec, lives under the "settings" or "optimization"
// sub-path, confirmed against docs/api-specs/parspack-cdn.openapi.yaml's
// Antivirus (lines 80-365), Dns (lines 2213-2477), Optimization (lines
// 9279-9574) and "Service Settings" (lines 16649-17285) tags.
//
// Developer mode, maintenance mode, query-string and origin-offline only
// have a PUT endpoint documented in those line ranges — there is no
// dedicated single-setting GET for any of them. The only GET that reports
// their current value is the combined
// GET /external/api/v1/zones/{zone_uuid}/cache/settings endpoint (line 1588,
// "Cache Management" tag), which is outside this file's assigned scope and
// likely owned by a parallel issue #24 slice covering cache settings. To
// avoid duplicating that group's work (and possibly two adapters hitting the
// same endpoint with different wire shapes), this file deliberately does
// NOT implement GetCDNDeveloperMode/GetCDNMaintenanceMode/
// GetCDNQueryStringSetting/GetCDNOriginOffline — only their Update
// counterparts, which are fully specified on their own. See the "Get CDN
// Developer/Maintenance/QueryString/OriginOffline" gap noted in this
// package's issue #24 integration notes for the maintainer to reconcile
// once the cache-settings slice lands.

// antivirusStatusWire mirrors both the get- and update-antivirus-status
// response's "data" object: {"enabled": bool}.
type antivirusStatusWire struct {
	Enabled bool `json:"enabled"`
}

// GetCDNAntivirusStatus returns whether antivirus scanning is enabled for
// the zone.
func (c *Client) GetCDNAntivirusStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (bool, error) {
	var wire antivirusStatusWire
	path := zonesBasePath + "/" + zoneUUID + "/settings/get-antivirus-status"
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &wire); err != nil {
		return false, fmt.Errorf("get CDN antivirus status for zone %s: %w", zoneUUID, err)
	}
	return wire.Enabled, nil
}

// UpdateCDNAntivirusStatus enables or disables antivirus scanning for the
// zone, returning the applied value.
func (c *Client) UpdateCDNAntivirusStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error) {
	reqBody := antivirusStatusWire{Enabled: enabled}
	path := zonesBasePath + "/" + zoneUUID + "/settings/update-antivirus-status"
	if err := c.doCDNJSON(ctx, creds, "PUT", path, reqBody, nil); err != nil {
		return false, fmt.Errorf("update CDN antivirus status for zone %s: %w", zoneUUID, err)
	}
	return enabled, nil
}

// dnsSecStatusWire mirrors GET .../dns-sec's "data" object: {"enabled",
// "value"}. PUT .../dns-sec's response only carries "value" (the DS record),
// so this same shape is reused there with Enabled left at its zero value —
// UpdateCDNDNSSecStatus fills Enabled from the request instead of the
// response.
type dnsSecStatusWire struct {
	Enabled bool   `json:"enabled"`
	Value   string `json:"value"`
}

func toDomainDNSSecStatus(w dnsSecStatusWire) domain.CDNDNSSecStatus {
	return domain.CDNDNSSecStatus{Enabled: w.Enabled, Value: w.Value}
}

// GetCDNDNSSecStatus returns the zone's current DNSSEC status, including the
// DS record when DNSSEC is enabled.
func (c *Client) GetCDNDNSSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNDNSSecStatus, error) {
	var wire dnsSecStatusWire
	path := zonesBasePath + "/" + zoneUUID + "/dns-sec"
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get CDN DNSSEC status for zone %s: %w", zoneUUID, err)
	}
	status := toDomainDNSSecStatus(wire)
	return &status, nil
}

// dnsSecUpdateRequest is the body of PUT .../dns-sec.
type dnsSecUpdateRequest struct {
	Enabled bool `json:"enabled"`
}

// UpdateCDNDNSSecStatus enables or disables DNSSEC for the zone, returning
// the resulting status (including the DS record when the response carries
// one).
func (c *Client) UpdateCDNDNSSecStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (*domain.CDNDNSSecStatus, error) {
	reqBody := dnsSecUpdateRequest{Enabled: enabled}
	var wire dnsSecStatusWire
	path := zonesBasePath + "/" + zoneUUID + "/dns-sec"
	if err := c.doCDNJSON(ctx, creds, "PUT", path, reqBody, &wire); err != nil {
		return nil, fmt.Errorf("update CDN DNSSEC status for zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNDNSSecStatus{Enabled: enabled, Value: wire.Value}, nil
}

// websiteMinificationWire mirrors the "website_minification" object nested
// in the optimization status get/update payloads.
type websiteMinificationWire struct {
	HTML bool `json:"html"`
	CSS  bool `json:"css"`
	JS   bool `json:"js"`
}

// optimizationStatusWire mirrors GET .../optimization/status's "data" object
// and PUT .../optimization/update's request body — both share this exact
// shape per the spec.
type optimizationStatusWire struct {
	ImageMinificationStatus bool                    `json:"image_minification_status"`
	WebPConversionStatus    bool                    `json:"webp_conversion_status"`
	WebsiteMinification     websiteMinificationWire `json:"website_minification"`
}

func toDomainOptimizationStatus(w optimizationStatusWire) domain.CDNOptimizationStatus {
	return domain.CDNOptimizationStatus{
		ImageMinification: w.ImageMinificationStatus,
		WebPConversion:    w.WebPConversionStatus,
		MinifyHTML:        w.WebsiteMinification.HTML,
		MinifyCSS:         w.WebsiteMinification.CSS,
		MinifyJS:          w.WebsiteMinification.JS,
	}
}

func toWireOptimizationStatus(s domain.CDNOptimizationStatus) optimizationStatusWire {
	return optimizationStatusWire{
		ImageMinificationStatus: s.ImageMinification,
		WebPConversionStatus:    s.WebPConversion,
		WebsiteMinification: websiteMinificationWire{
			HTML: s.MinifyHTML, CSS: s.MinifyCSS, JS: s.MinifyJS,
		},
	}
}

// GetCDNOptimizationStatus returns the zone's current asset optimization
// configuration.
func (c *Client) GetCDNOptimizationStatus(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNOptimizationStatus, error) {
	var wire optimizationStatusWire
	path := zonesBasePath + "/" + zoneUUID + "/optimization/status"
	if err := c.doCDNJSON(ctx, creds, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get CDN optimization status for zone %s: %w", zoneUUID, err)
	}
	status := toDomainOptimizationStatus(wire)
	return &status, nil
}

// UpdateCDNOptimization replaces the zone's asset optimization configuration
// (image minification, WebP conversion, and per-asset-type website
// minification), returning the applied configuration.
func (c *Client) UpdateCDNOptimization(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, status domain.CDNOptimizationStatus) (*domain.CDNOptimizationStatus, error) {
	reqBody := toWireOptimizationStatus(status)
	path := zonesBasePath + "/" + zoneUUID + "/optimization/update"
	if err := c.doCDNJSON(ctx, creds, "PUT", path, reqBody, nil); err != nil {
		return nil, fmt.Errorf("update CDN optimization status for zone %s: %w", zoneUUID, err)
	}
	return &status, nil
}

// zoneToggleUpdateRequest is the {"enabled": bool} request body shared by
// developer mode, maintenance mode, query-string and origin-offline updates
// — all four take and return nothing but this one flag per the spec.
type zoneToggleUpdateRequest struct {
	Enabled bool `json:"enabled"`
}

// UpdateCDNDeveloperMode enables or disables developer mode for the zone
// (bypasses caching for troubleshooting), returning the applied value. See
// this file's top-level comment for why there is no GetCDNDeveloperMode: the
// spec documents no dedicated single-setting GET for it.
func (c *Client) UpdateCDNDeveloperMode(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error) {
	reqBody := zoneToggleUpdateRequest{Enabled: enabled}
	path := zonesBasePath + "/" + zoneUUID + "/settings/developer-mode"
	if err := c.doCDNJSON(ctx, creds, "PUT", path, reqBody, nil); err != nil {
		return false, fmt.Errorf("update CDN developer mode for zone %s: %w", zoneUUID, err)
	}
	return enabled, nil
}

// UpdateCDNMaintenanceMode enables or disables maintenance mode for the
// zone, returning the applied value. See this file's top-level comment for
// why there is no GetCDNMaintenanceMode.
func (c *Client) UpdateCDNMaintenanceMode(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error) {
	reqBody := zoneToggleUpdateRequest{Enabled: enabled}
	path := zonesBasePath + "/" + zoneUUID + "/settings/maintenance-mode"
	if err := c.doCDNJSON(ctx, creds, "PUT", path, reqBody, nil); err != nil {
		return false, fmt.Errorf("update CDN maintenance mode for zone %s: %w", zoneUUID, err)
	}
	return enabled, nil
}

// UpdateCDNQueryStringSetting enables or disables "ignore query string"
// caching behavior for the zone (when enabled, the CDN caches a URL's
// response once regardless of its query string), returning the applied
// value. See this file's top-level comment for why there is no
// GetCDNQueryStringSetting.
func (c *Client) UpdateCDNQueryStringSetting(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error) {
	reqBody := zoneToggleUpdateRequest{Enabled: enabled}
	path := zonesBasePath + "/" + zoneUUID + "/settings/query-string"
	if err := c.doCDNJSON(ctx, creds, "PUT", path, reqBody, nil); err != nil {
		return false, fmt.Errorf("update CDN query string setting for zone %s: %w", zoneUUID, err)
	}
	return enabled, nil
}

// UpdateCDNOriginOffline enables or disables origin-offline handling for the
// zone (serving cached content when the origin is unreachable), returning
// the applied value. See this file's top-level comment for why there is no
// GetCDNOriginOffline.
func (c *Client) UpdateCDNOriginOffline(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, enabled bool) (bool, error) {
	reqBody := zoneToggleUpdateRequest{Enabled: enabled}
	path := zonesBasePath + "/" + zoneUUID + "/settings/origin-offline"
	if err := c.doCDNJSON(ctx, creds, "PUT", path, reqBody, nil); err != nil {
		return false, fmt.Errorf("update CDN origin offline setting for zone %s: %w", zoneUUID, err)
	}
	return enabled, nil
}
