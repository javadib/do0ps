package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// fakeCDNNetworkProvider implements the local cdnNetworkProvider interface
// declared in internal/core/app/cdn_network.go. It is kept in its own file,
// separate from the shared fakeProvider in app_test.go, because that
// interface is local to cdn_network.go rather than part of
// ports.ParspackProvider (see that file's doc comment for why).
type fakeCDNNetworkProvider struct {
	httpsConvertor *domain.CDNHTTPSConvertorSetting
	getHTTPSErr    error
	updatedHTTPS   *domain.CDNHTTPSConvertorSetting
	updateHTTPSErr error

	edgeConnection *domain.CDNEdgeToUpstreamConnectionSetting
	getEdgeErr     error
	updatedEdge    *domain.CDNEdgeToUpstreamConnectionSetting
	updateEdgeErr  error

	wwwRedirection *domain.CDNWWWRedirectionSetting
	getWWWErr      error
	updatedWWW     *domain.CDNWWWRedirectionSetting
	updateWWWErr   error

	webSocket        *domain.CDNWebSocketSetting
	getWebSocketErr  error
	updatedWebSocket *domain.CDNWebSocketSetting
	updateWSErr      error
}

func (p *fakeCDNNetworkProvider) GetCDNHTTPSConvertor(context.Context, domain.ProviderCredentials, string) (*domain.CDNHTTPSConvertorSetting, error) {
	if p.getHTTPSErr != nil {
		return nil, p.getHTTPSErr
	}
	return p.httpsConvertor, nil
}

func (p *fakeCDNNetworkProvider) UpdateCDNHTTPSConvertor(_ context.Context, _ domain.ProviderCredentials, _ string, setting domain.CDNHTTPSConvertorSetting) (*domain.CDNHTTPSConvertorSetting, error) {
	if p.updateHTTPSErr != nil {
		return nil, p.updateHTTPSErr
	}
	p.updatedHTTPS = &setting
	return &setting, nil
}

func (p *fakeCDNNetworkProvider) GetCDNEdgeToUpstreamConnection(context.Context, domain.ProviderCredentials, string) (*domain.CDNEdgeToUpstreamConnectionSetting, error) {
	if p.getEdgeErr != nil {
		return nil, p.getEdgeErr
	}
	return p.edgeConnection, nil
}

func (p *fakeCDNNetworkProvider) UpdateCDNEdgeToUpstreamConnection(_ context.Context, _ domain.ProviderCredentials, _ string, setting domain.CDNEdgeToUpstreamConnectionSetting) (*domain.CDNEdgeToUpstreamConnectionSetting, error) {
	if p.updateEdgeErr != nil {
		return nil, p.updateEdgeErr
	}
	p.updatedEdge = &setting
	return &setting, nil
}

func (p *fakeCDNNetworkProvider) GetCDNWWWRedirection(context.Context, domain.ProviderCredentials, string) (*domain.CDNWWWRedirectionSetting, error) {
	if p.getWWWErr != nil {
		return nil, p.getWWWErr
	}
	return p.wwwRedirection, nil
}

func (p *fakeCDNNetworkProvider) UpdateCDNWWWRedirection(_ context.Context, _ domain.ProviderCredentials, _ string, setting domain.CDNWWWRedirectionSetting) (*domain.CDNWWWRedirectionSetting, error) {
	if p.updateWWWErr != nil {
		return nil, p.updateWWWErr
	}
	p.updatedWWW = &setting
	return &setting, nil
}

func (p *fakeCDNNetworkProvider) GetCDNWebSocket(context.Context, domain.ProviderCredentials, string) (*domain.CDNWebSocketSetting, error) {
	if p.getWebSocketErr != nil {
		return nil, p.getWebSocketErr
	}
	return p.webSocket, nil
}

func (p *fakeCDNNetworkProvider) UpdateCDNWebSocket(_ context.Context, _ domain.ProviderCredentials, _ string, setting domain.CDNWebSocketSetting) (*domain.CDNWebSocketSetting, error) {
	if p.updateWSErr != nil {
		return nil, p.updateWSErr
	}
	p.updatedWebSocket = &setting
	return &setting, nil
}

// --- HTTPS convertor -------------------------------------------------------

func TestGetCDNHTTPSConvertorReturnsProviderResult(t *testing.T) {
	provider := &fakeCDNNetworkProvider{httpsConvertor: &domain.CDNHTTPSConvertorSetting{Enabled: true}}
	uc := app.NewGetCDNHTTPSConvertor(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.GetCDNHTTPSConvertorInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !setting.Enabled {
		t.Errorf("Enabled = %v, want true", setting.Enabled)
	}
}

func TestGetCDNHTTPSConvertorRequiresZoneUUID(t *testing.T) {
	uc := app.NewGetCDNHTTPSConvertor(&inlineQueue{}, &fakeCDNNetworkProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNHTTPSConvertorInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNHTTPSConvertorSuccess(t *testing.T) {
	provider := &fakeCDNNetworkProvider{}
	uc := app.NewUpdateCDNHTTPSConvertor(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.UpdateCDNHTTPSConvertorInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !setting.Enabled {
		t.Errorf("Enabled = %v, want true", setting.Enabled)
	}
	if !provider.updatedHTTPS.Enabled {
		t.Error("provider was not called with enabled=true")
	}
}

// --- Edge-to-upstream connection --------------------------------------------

func TestGetCDNEdgeToUpstreamConnectionReturnsProviderResult(t *testing.T) {
	provider := &fakeCDNNetworkProvider{edgeConnection: &domain.CDNEdgeToUpstreamConnectionSetting{Type: "auto"}}
	uc := app.NewGetCDNEdgeToUpstreamConnection(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.GetCDNEdgeToUpstreamConnectionInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if setting.Type != "auto" {
		t.Errorf("Type = %q, want auto", setting.Type)
	}
}

func TestUpdateCDNEdgeToUpstreamConnectionSuccess(t *testing.T) {
	provider := &fakeCDNNetworkProvider{}
	uc := app.NewUpdateCDNEdgeToUpstreamConnection(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.UpdateCDNEdgeToUpstreamConnectionInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", Type: "https",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if setting.Type != "https" {
		t.Errorf("Type = %q, want https", setting.Type)
	}
}

func TestUpdateCDNEdgeToUpstreamConnectionRejectsInvalidType(t *testing.T) {
	provider := &fakeCDNNetworkProvider{}
	uc := app.NewUpdateCDNEdgeToUpstreamConnection(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateCDNEdgeToUpstreamConnectionInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", Type: "bogus",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.updatedEdge != nil {
		t.Error("provider was called with an invalid connection type")
	}
}

// --- WWW redirection ---------------------------------------------------

func TestGetCDNWWWRedirectionReturnsProviderResult(t *testing.T) {
	provider := &fakeCDNNetworkProvider{wwwRedirection: &domain.CDNWWWRedirectionSetting{Mode: "none"}}
	uc := app.NewGetCDNWWWRedirection(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.GetCDNWWWRedirectionInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if setting.Mode != "none" {
		t.Errorf("Mode = %q, want none", setting.Mode)
	}
}

func TestUpdateCDNWWWRedirectionSuccess(t *testing.T) {
	provider := &fakeCDNNetworkProvider{}
	uc := app.NewUpdateCDNWWWRedirection(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.UpdateCDNWWWRedirectionInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", Mode: "redirect-to-www",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if setting.Mode != "redirect-to-www" {
		t.Errorf("Mode = %q, want redirect-to-www", setting.Mode)
	}
}

func TestUpdateCDNWWWRedirectionRejectsInvalidMode(t *testing.T) {
	provider := &fakeCDNNetworkProvider{}
	uc := app.NewUpdateCDNWWWRedirection(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateCDNWWWRedirectionInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", Mode: "bogus",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.updatedWWW != nil {
		t.Error("provider was called with an invalid redirection mode")
	}
}

// --- WebSocket -----------------------------------------------------------

func TestGetCDNWebSocketReturnsProviderResult(t *testing.T) {
	provider := &fakeCDNNetworkProvider{webSocket: &domain.CDNWebSocketSetting{Enabled: false}}
	uc := app.NewGetCDNWebSocket(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.GetCDNWebSocketInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if setting.Enabled {
		t.Errorf("Enabled = %v, want false", setting.Enabled)
	}
}

func TestUpdateCDNWebSocketSuccess(t *testing.T) {
	provider := &fakeCDNNetworkProvider{}
	uc := app.NewUpdateCDNWebSocket(&inlineQueue{}, provider)

	setting, err := uc.Execute(context.Background(), app.UpdateCDNWebSocketInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !setting.Enabled {
		t.Errorf("Enabled = %v, want true", setting.Enabled)
	}
}

func TestUpdateCDNWebSocketRequiresZoneUUID(t *testing.T) {
	uc := app.NewUpdateCDNWebSocket(&inlineQueue{}, &fakeCDNNetworkProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNWebSocketInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, Enabled: true})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
