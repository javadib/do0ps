package parspack

import (
	"context"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Zone-level network settings (issue #24's Network tag), wired to the real
// CDN API. Base path is confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml, lines 8223-9279 — each setting
// is a simple get/update pair under zonesBasePath (defined in cdn.go),
// relative to Client.cdnBaseURL. All eight operations are single round
// trips (fast, AGENTS.md 4.3).

// httpsConvertorWire mirrors both the response of GET .../https-convertor
// and the request body of PUT .../https-convertor.
type httpsConvertorWire struct {
	Enabled bool `json:"enabled"`
}

// GetCDNHTTPSConvertor returns whether Parspack automatically rewrites HTTP
// links to HTTPS for the given zone.
func (c *Client) GetCDNHTTPSConvertor(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNHTTPSConvertorSetting, error) {
	var wire httpsConvertorWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+"/https-convertor", nil, &wire); err != nil {
		return nil, fmt.Errorf("get HTTPS convertor setting for zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNHTTPSConvertorSetting{Enabled: wire.Enabled}, nil
}

// UpdateCDNHTTPSConvertor sets whether Parspack automatically rewrites HTTP
// links to HTTPS for the given zone. The endpoint's response carries no
// useful body, so the setting sent is echoed back on success.
func (c *Client) UpdateCDNHTTPSConvertor(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNHTTPSConvertorSetting) (*domain.CDNHTTPSConvertorSetting, error) {
	reqBody := httpsConvertorWire{Enabled: setting.Enabled}
	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+"/https-convertor", reqBody, nil); err != nil {
		return nil, fmt.Errorf("update HTTPS convertor setting for zone %s: %w", zoneUUID, err)
	}
	return &setting, nil
}

// edgeToUpstreamConnectionWire mirrors both the response of GET
// .../edge-to-upstream-connection and the request body of PUT
// .../edge-to-upstream-connection.
type edgeToUpstreamConnectionWire struct {
	Type string `json:"type"`
}

// GetCDNEdgeToUpstreamConnection returns the protocol Parspack's edge nodes
// currently use when connecting to the origin server for the given zone.
func (c *Client) GetCDNEdgeToUpstreamConnection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNEdgeToUpstreamConnectionSetting, error) {
	var wire edgeToUpstreamConnectionWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+"/edge-to-upstream-connection", nil, &wire); err != nil {
		return nil, fmt.Errorf("get edge-to-upstream connection setting for zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNEdgeToUpstreamConnectionSetting{Type: wire.Type}, nil
}

// UpdateCDNEdgeToUpstreamConnection sets the protocol Parspack's edge nodes
// use when connecting to the origin server for the given zone. The
// endpoint's response carries no useful body, so the setting sent is
// echoed back on success.
func (c *Client) UpdateCDNEdgeToUpstreamConnection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNEdgeToUpstreamConnectionSetting) (*domain.CDNEdgeToUpstreamConnectionSetting, error) {
	reqBody := edgeToUpstreamConnectionWire{Type: setting.Type}
	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+"/edge-to-upstream-connection", reqBody, nil); err != nil {
		return nil, fmt.Errorf("update edge-to-upstream connection setting for zone %s: %w", zoneUUID, err)
	}
	return &setting, nil
}

// wwwRedirectionWire mirrors both the response of GET .../www-redirection
// and the request body of PUT .../www-redirection.
type wwwRedirectionWire struct {
	WWWRedirection string `json:"www_redirection"`
}

// GetCDNWWWRedirection returns the current www/non-www redirection mode for
// the given zone.
func (c *Client) GetCDNWWWRedirection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNWWWRedirectionSetting, error) {
	var wire wwwRedirectionWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+"/www-redirection", nil, &wire); err != nil {
		return nil, fmt.Errorf("get www redirection setting for zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNWWWRedirectionSetting{Mode: wire.WWWRedirection}, nil
}

// UpdateCDNWWWRedirection sets the www/non-www redirection mode for the
// given zone. The endpoint's response carries no useful body, so the
// setting sent is echoed back on success.
func (c *Client) UpdateCDNWWWRedirection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNWWWRedirectionSetting) (*domain.CDNWWWRedirectionSetting, error) {
	reqBody := wwwRedirectionWire{WWWRedirection: setting.Mode}
	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+"/www-redirection", reqBody, nil); err != nil {
		return nil, fmt.Errorf("update www redirection setting for zone %s: %w", zoneUUID, err)
	}
	return &setting, nil
}

// webSocketWire mirrors both the response of GET .../web-socket and the
// request body of PUT .../web-socket.
type webSocketWire struct {
	Enabled bool `json:"enabled"`
}

// GetCDNWebSocket returns whether WebSocket connections are currently
// allowed through the given zone.
func (c *Client) GetCDNWebSocket(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNWebSocketSetting, error) {
	var wire webSocketWire
	if err := c.doCDNJSON(ctx, creds, "GET", zonesBasePath+"/"+zoneUUID+"/web-socket", nil, &wire); err != nil {
		return nil, fmt.Errorf("get WebSocket setting for zone %s: %w", zoneUUID, err)
	}
	return &domain.CDNWebSocketSetting{Enabled: wire.Enabled}, nil
}

// UpdateCDNWebSocket sets whether WebSocket connections are allowed through
// the given zone. The endpoint's 200 response carries no body at all, so the
// setting sent is echoed back on success.
func (c *Client) UpdateCDNWebSocket(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNWebSocketSetting) (*domain.CDNWebSocketSetting, error) {
	reqBody := webSocketWire{Enabled: setting.Enabled}
	if err := c.doCDNJSON(ctx, creds, "PUT", zonesBasePath+"/"+zoneUUID+"/web-socket", reqBody, nil); err != nil {
		return nil, fmt.Errorf("update WebSocket setting for zone %s: %w", zoneUUID, err)
	}
	return &setting, nil
}
