package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// cdnNetworkProvider is the subset of provider operations these use cases
// need for zone-level network settings (issue #24's Network tag: HTTPS
// convertor, edge-to-upstream connection protocol, www redirection, and
// WebSocket support). It is declared locally, rather than added to
// ports.ParspackProvider directly, because that port is being integrated
// centrally later across several concurrently-developed slices of issue
// #24 — *parspack.Client already satisfies this interface structurally
// (Go structural typing), and this method set is intended to be merged
// into ports.ParspackProvider verbatim once that integration happens.
type cdnNetworkProvider interface {
	GetCDNHTTPSConvertor(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNHTTPSConvertorSetting, error)
	UpdateCDNHTTPSConvertor(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNHTTPSConvertorSetting) (*domain.CDNHTTPSConvertorSetting, error)

	GetCDNEdgeToUpstreamConnection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNEdgeToUpstreamConnectionSetting, error)
	UpdateCDNEdgeToUpstreamConnection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNEdgeToUpstreamConnectionSetting) (*domain.CDNEdgeToUpstreamConnectionSetting, error)

	GetCDNWWWRedirection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNWWWRedirectionSetting, error)
	UpdateCDNWWWRedirection(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNWWWRedirectionSetting) (*domain.CDNWWWRedirectionSetting, error)

	GetCDNWebSocket(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) (*domain.CDNWebSocketSetting, error)
	UpdateCDNWebSocket(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, setting domain.CDNWebSocketSetting) (*domain.CDNWebSocketSetting, error)
}

// --- HTTPS convertor -------------------------------------------------------

// GetCDNHTTPSConvertorInput identifies the zone whose HTTPS convertor
// setting to look up.
type GetCDNHTTPSConvertorInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNHTTPSConvertor is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNHTTPSConvertor struct {
	queue    ports.Queue
	provider cdnNetworkProvider
}

// NewGetCDNHTTPSConvertor builds the use case from its ports.
func NewGetCDNHTTPSConvertor(queue ports.Queue, provider cdnNetworkProvider) *GetCDNHTTPSConvertor {
	return &GetCDNHTTPSConvertor{queue: queue, provider: provider}
}

// Execute returns the current HTTPS convertor setting of one zone.
func (uc *GetCDNHTTPSConvertor) Execute(ctx context.Context, in GetCDNHTTPSConvertorInput) (*domain.CDNHTTPSConvertorSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.GetCDNHTTPSConvertor(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting HTTPS convertor setting for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNHTTPSConvertorSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding HTTPS convertor setting: %w", err)
	}
	return &setting, nil
}

// UpdateCDNHTTPSConvertorInput carries the new HTTPS convertor setting for a
// zone.
type UpdateCDNHTTPSConvertorInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Enabled     bool
}

// UpdateCDNHTTPSConvertor is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNHTTPSConvertor struct {
	queue    ports.Queue
	provider cdnNetworkProvider
}

// NewUpdateCDNHTTPSConvertor builds the use case from its ports.
func NewUpdateCDNHTTPSConvertor(queue ports.Queue, provider cdnNetworkProvider) *UpdateCDNHTTPSConvertor {
	return &UpdateCDNHTTPSConvertor{queue: queue, provider: provider}
}

// Execute updates the HTTPS convertor setting of one zone, returning the
// setting now in effect.
func (uc *UpdateCDNHTTPSConvertor) Execute(ctx context.Context, in UpdateCDNHTTPSConvertorInput) (*domain.CDNHTTPSConvertorSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.UpdateCDNHTTPSConvertor(ctx, in.Credentials, in.ZoneUUID, domain.CDNHTTPSConvertorSetting{Enabled: in.Enabled})
		if err != nil {
			return nil, fmt.Errorf("updating HTTPS convertor setting for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNHTTPSConvertorSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding HTTPS convertor setting: %w", err)
	}
	return &setting, nil
}

// --- Edge-to-upstream connection --------------------------------------------

// GetCDNEdgeToUpstreamConnectionInput identifies the zone whose
// edge-to-upstream connection setting to look up.
type GetCDNEdgeToUpstreamConnectionInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNEdgeToUpstreamConnection is a fast operation: it runs on a worker
// but the caller waits for the result inside the same tool call.
type GetCDNEdgeToUpstreamConnection struct {
	queue    ports.Queue
	provider cdnNetworkProvider
}

// NewGetCDNEdgeToUpstreamConnection builds the use case from its ports.
func NewGetCDNEdgeToUpstreamConnection(queue ports.Queue, provider cdnNetworkProvider) *GetCDNEdgeToUpstreamConnection {
	return &GetCDNEdgeToUpstreamConnection{queue: queue, provider: provider}
}

// Execute returns the current edge-to-upstream connection setting of one
// zone.
func (uc *GetCDNEdgeToUpstreamConnection) Execute(ctx context.Context, in GetCDNEdgeToUpstreamConnectionInput) (*domain.CDNEdgeToUpstreamConnectionSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.GetCDNEdgeToUpstreamConnection(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting edge-to-upstream connection setting for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNEdgeToUpstreamConnectionSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding edge-to-upstream connection setting: %w", err)
	}
	return &setting, nil
}

// UpdateCDNEdgeToUpstreamConnectionInput carries the new edge-to-upstream
// connection setting for a zone.
type UpdateCDNEdgeToUpstreamConnectionInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Type        string
}

// UpdateCDNEdgeToUpstreamConnection is a fast operation: it runs on a
// worker but the caller waits for the result inside the same tool call.
type UpdateCDNEdgeToUpstreamConnection struct {
	queue    ports.Queue
	provider cdnNetworkProvider
}

// NewUpdateCDNEdgeToUpstreamConnection builds the use case from its ports.
func NewUpdateCDNEdgeToUpstreamConnection(queue ports.Queue, provider cdnNetworkProvider) *UpdateCDNEdgeToUpstreamConnection {
	return &UpdateCDNEdgeToUpstreamConnection{queue: queue, provider: provider}
}

// Execute updates the edge-to-upstream connection setting of one zone,
// returning the setting now in effect.
func (uc *UpdateCDNEdgeToUpstreamConnection) Execute(ctx context.Context, in UpdateCDNEdgeToUpstreamConnectionInput) (*domain.CDNEdgeToUpstreamConnectionSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidCDNEdgeToUpstreamConnectionType(in.Type) {
		return nil, fmt.Errorf("type %q is not one of the connection types Parspack accepts: %w", in.Type, domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.UpdateCDNEdgeToUpstreamConnection(ctx, in.Credentials, in.ZoneUUID, domain.CDNEdgeToUpstreamConnectionSetting{Type: in.Type})
		if err != nil {
			return nil, fmt.Errorf("updating edge-to-upstream connection setting for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNEdgeToUpstreamConnectionSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding edge-to-upstream connection setting: %w", err)
	}
	return &setting, nil
}

// --- WWW redirection ---------------------------------------------------

// GetCDNWWWRedirectionInput identifies the zone whose www redirection
// setting to look up.
type GetCDNWWWRedirectionInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNWWWRedirection is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNWWWRedirection struct {
	queue    ports.Queue
	provider cdnNetworkProvider
}

// NewGetCDNWWWRedirection builds the use case from its ports.
func NewGetCDNWWWRedirection(queue ports.Queue, provider cdnNetworkProvider) *GetCDNWWWRedirection {
	return &GetCDNWWWRedirection{queue: queue, provider: provider}
}

// Execute returns the current www redirection setting of one zone.
func (uc *GetCDNWWWRedirection) Execute(ctx context.Context, in GetCDNWWWRedirectionInput) (*domain.CDNWWWRedirectionSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.GetCDNWWWRedirection(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting www redirection setting for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNWWWRedirectionSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding www redirection setting: %w", err)
	}
	return &setting, nil
}

// UpdateCDNWWWRedirectionInput carries the new www redirection setting for a
// zone.
type UpdateCDNWWWRedirectionInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Mode        string
}

// UpdateCDNWWWRedirection is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNWWWRedirection struct {
	queue    ports.Queue
	provider cdnNetworkProvider
}

// NewUpdateCDNWWWRedirection builds the use case from its ports.
func NewUpdateCDNWWWRedirection(queue ports.Queue, provider cdnNetworkProvider) *UpdateCDNWWWRedirection {
	return &UpdateCDNWWWRedirection{queue: queue, provider: provider}
}

// Execute updates the www redirection setting of one zone, returning the
// setting now in effect.
func (uc *UpdateCDNWWWRedirection) Execute(ctx context.Context, in UpdateCDNWWWRedirectionInput) (*domain.CDNWWWRedirectionSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidCDNWWWRedirectionMode(in.Mode) {
		return nil, fmt.Errorf("www_redirection %q is not one of the modes Parspack accepts: %w", in.Mode, domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.UpdateCDNWWWRedirection(ctx, in.Credentials, in.ZoneUUID, domain.CDNWWWRedirectionSetting{Mode: in.Mode})
		if err != nil {
			return nil, fmt.Errorf("updating www redirection setting for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNWWWRedirectionSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding www redirection setting: %w", err)
	}
	return &setting, nil
}

// --- WebSocket -----------------------------------------------------------

// GetCDNWebSocketInput identifies the zone whose WebSocket setting to look
// up.
type GetCDNWebSocketInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNWebSocket is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNWebSocket struct {
	queue    ports.Queue
	provider cdnNetworkProvider
}

// NewGetCDNWebSocket builds the use case from its ports.
func NewGetCDNWebSocket(queue ports.Queue, provider cdnNetworkProvider) *GetCDNWebSocket {
	return &GetCDNWebSocket{queue: queue, provider: provider}
}

// Execute returns the current WebSocket setting of one zone.
func (uc *GetCDNWebSocket) Execute(ctx context.Context, in GetCDNWebSocketInput) (*domain.CDNWebSocketSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.GetCDNWebSocket(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting WebSocket setting for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNWebSocketSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding WebSocket setting: %w", err)
	}
	return &setting, nil
}

// UpdateCDNWebSocketInput carries the new WebSocket setting for a zone.
type UpdateCDNWebSocketInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Enabled     bool
}

// UpdateCDNWebSocket is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type UpdateCDNWebSocket struct {
	queue    ports.Queue
	provider cdnNetworkProvider
}

// NewUpdateCDNWebSocket builds the use case from its ports.
func NewUpdateCDNWebSocket(queue ports.Queue, provider cdnNetworkProvider) *UpdateCDNWebSocket {
	return &UpdateCDNWebSocket{queue: queue, provider: provider}
}

// Execute updates the WebSocket setting of one zone, returning the setting
// now in effect.
func (uc *UpdateCDNWebSocket) Execute(ctx context.Context, in UpdateCDNWebSocketInput) (*domain.CDNWebSocketSetting, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		setting, err := uc.provider.UpdateCDNWebSocket(ctx, in.Credentials, in.ZoneUUID, domain.CDNWebSocketSetting{Enabled: in.Enabled})
		if err != nil {
			return nil, fmt.Errorf("updating WebSocket setting for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(setting)
	})
	if err != nil {
		return nil, err
	}

	var setting domain.CDNWebSocketSetting
	if err := json.Unmarshal(raw, &setting); err != nil {
		return nil, fmt.Errorf("decoding WebSocket setting: %w", err)
	}
	return &setting, nil
}
