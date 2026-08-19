package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// fakeCDNLoadBalanceProvider is a package-local fake satisfying app's
// unexported cdnLoadBalanceProvider interface by structural typing. It is
// deliberately separate from the shared fakeProvider in app_test.go (a file
// other concurrent work also touches): only the CDN load-balance methods
// these use cases actually call need to exist here.
type fakeCDNLoadBalanceProvider struct {
	loadBalances []domain.CDNLoadBalance
	servers      []domain.CDNLoadBalanceServer

	createdLoadBalance *domain.CDNLoadBalance
	updatedLoadBalance *domain.CDNLoadBalance
	deletedLoadBalance string

	createdServer *domain.CDNLoadBalanceServer
	updatedServer *domain.CDNLoadBalanceServer
	deletedServer string

	createLoadBalanceErr error
	getLoadBalanceErr    error
	deleteLoadBalanceErr error
	createServerErr      error
	getServerErr         error
	deleteServerErr      error
}

func (p *fakeCDNLoadBalanceProvider) ListCDNLoadBalances(context.Context, domain.ProviderCredentials, string) ([]domain.CDNLoadBalance, error) {
	return p.loadBalances, nil
}

func (p *fakeCDNLoadBalanceProvider) CreateCDNLoadBalance(_ context.Context, _ domain.ProviderCredentials, _ string, lb domain.CDNLoadBalance) (*domain.CDNLoadBalance, error) {
	if p.createLoadBalanceErr != nil {
		return nil, p.createLoadBalanceErr
	}
	p.createdLoadBalance = &lb
	return &lb, nil
}

func (p *fakeCDNLoadBalanceProvider) GetCDNLoadBalance(_ context.Context, _ domain.ProviderCredentials, _, id string) (*domain.CDNLoadBalance, error) {
	if p.getLoadBalanceErr != nil {
		return nil, p.getLoadBalanceErr
	}
	for i := range p.loadBalances {
		if p.loadBalances[i].ID == id {
			return &p.loadBalances[i], nil
		}
	}
	return nil, fmt.Errorf("load balance %q: %w", id, domain.ErrNotFound)
}

func (p *fakeCDNLoadBalanceProvider) UpdateCDNLoadBalance(_ context.Context, _ domain.ProviderCredentials, _, id string, lb domain.CDNLoadBalance) (*domain.CDNLoadBalance, error) {
	lb.ID = id
	p.updatedLoadBalance = &lb
	return &lb, nil
}

func (p *fakeCDNLoadBalanceProvider) DeleteCDNLoadBalance(_ context.Context, _ domain.ProviderCredentials, _, id string) error {
	if p.deleteLoadBalanceErr != nil {
		return p.deleteLoadBalanceErr
	}
	p.deletedLoadBalance = id
	return nil
}

func (p *fakeCDNLoadBalanceProvider) ListCDNLoadBalanceServers(context.Context, domain.ProviderCredentials, string, string) ([]domain.CDNLoadBalanceServer, error) {
	return p.servers, nil
}

func (p *fakeCDNLoadBalanceProvider) CreateCDNLoadBalanceServer(_ context.Context, _ domain.ProviderCredentials, _ string, srv domain.CDNLoadBalanceServer) (*domain.CDNLoadBalanceServer, error) {
	if p.createServerErr != nil {
		return nil, p.createServerErr
	}
	p.createdServer = &srv
	return &srv, nil
}

func (p *fakeCDNLoadBalanceProvider) GetCDNLoadBalanceServer(_ context.Context, _ domain.ProviderCredentials, _, id string) (*domain.CDNLoadBalanceServer, error) {
	if p.getServerErr != nil {
		return nil, p.getServerErr
	}
	for i := range p.servers {
		if p.servers[i].ID == id {
			return &p.servers[i], nil
		}
	}
	return nil, fmt.Errorf("server %q: %w", id, domain.ErrNotFound)
}

func (p *fakeCDNLoadBalanceProvider) UpdateCDNLoadBalanceServer(_ context.Context, _ domain.ProviderCredentials, _, id string, srv domain.CDNLoadBalanceServer) (*domain.CDNLoadBalanceServer, error) {
	srv.ID = id
	p.updatedServer = &srv
	return &srv, nil
}

func (p *fakeCDNLoadBalanceProvider) DeleteCDNLoadBalanceServer(_ context.Context, _ domain.ProviderCredentials, _, id string) error {
	if p.deleteServerErr != nil {
		return p.deleteServerErr
	}
	p.deletedServer = id
	return nil
}

func TestListCDNLoadBalancesReturnsProviderResult(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{loadBalances: []domain.CDNLoadBalance{{ID: "lb1", Name: "pool-1"}}}
	uc := app.NewListCDNLoadBalances(&inlineQueue{}, provider)

	balances, err := uc.Execute(context.Background(), app.ListCDNLoadBalancesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(balances) != 1 || balances[0].ID != "lb1" {
		t.Errorf("balances = %+v, want a single lb1 pool", balances)
	}
}

func TestListCDNLoadBalancesRejectsMissingZone(t *testing.T) {
	uc := app.NewListCDNLoadBalances(&inlineQueue{}, &fakeCDNLoadBalanceProvider{})

	_, err := uc.Execute(context.Background(), app.ListCDNLoadBalancesInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCreateCDNLoadBalanceSuccess(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{}
	uc := app.NewCreateCDNLoadBalance(&inlineQueue{}, provider)

	lb, err := uc.Execute(context.Background(), app.CreateCDNLoadBalanceInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		LoadBalance: domain.CDNLoadBalance{Name: "pool-1", Enabled: true, Method: "round_robin"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if lb.Name != "pool-1" {
		t.Errorf("lb.Name = %q, want pool-1", lb.Name)
	}
	if provider.createdLoadBalance == nil || provider.createdLoadBalance.Name != "pool-1" {
		t.Errorf("provider.createdLoadBalance = %+v, want pool-1", provider.createdLoadBalance)
	}
}

func TestCreateCDNLoadBalanceRejectsMissingName(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{}
	uc := app.NewCreateCDNLoadBalance(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNLoadBalanceInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		LoadBalance: domain.CDNLoadBalance{Enabled: true},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdLoadBalance != nil {
		t.Error("provider was called with a load balance missing a name")
	}
}

func TestCreateCDNLoadBalanceRejectsInvalidMethod(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{}
	uc := app.NewCreateCDNLoadBalance(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNLoadBalanceInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		LoadBalance: domain.CDNLoadBalance{Name: "pool-1", Method: "weighted-magic"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCreateCDNLoadBalanceRejectsInvalidServerGroup(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{}
	uc := app.NewCreateCDNLoadBalance(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNLoadBalanceInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		LoadBalance: domain.CDNLoadBalance{
			Name:    "pool-1",
			Servers: []domain.CDNLoadBalanceServer{{IP: "1.1.1.1", Group: "tertiary"}},
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNLoadBalanceSuccess(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{loadBalances: []domain.CDNLoadBalance{{ID: "lb1", Name: "pool-1"}}}
	uc := app.NewGetCDNLoadBalance(&inlineQueue{}, provider)

	lb, err := uc.Execute(context.Background(), app.GetCDNLoadBalanceInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", LoadBalanceID: "lb1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if lb.Name != "pool-1" {
		t.Errorf("lb.Name = %q, want pool-1", lb.Name)
	}
}

func TestGetCDNLoadBalanceNotFound(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{}
	uc := app.NewGetCDNLoadBalance(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetCDNLoadBalanceInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", LoadBalanceID: "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNLoadBalanceSuccess(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{}
	uc := app.NewUpdateCDNLoadBalance(&inlineQueue{}, provider)

	lb, err := uc.Execute(context.Background(), app.UpdateCDNLoadBalanceInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", LoadBalanceID: "lb1",
		LoadBalance: domain.CDNLoadBalance{Name: "pool-1-renamed", Enabled: false},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if lb.ID != "lb1" || lb.Name != "pool-1-renamed" {
		t.Errorf("lb = %+v, want ID lb1 and name pool-1-renamed", lb)
	}
}

func TestDeleteCDNLoadBalanceToleratesNotFound(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{deleteLoadBalanceErr: domain.ErrNotFound}
	uc := app.NewDeleteCDNLoadBalance(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNLoadBalanceInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", LoadBalanceID: "lb1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil (already-absent pool treated as done)", err)
	}
}

func TestListCDNLoadBalanceServersRejectsMissingLoadBalanceID(t *testing.T) {
	uc := app.NewListCDNLoadBalanceServers(&inlineQueue{}, &fakeCDNLoadBalanceProvider{})

	_, err := uc.Execute(context.Background(), app.ListCDNLoadBalanceServersInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestListCDNLoadBalanceServersReturnsProviderResult(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{servers: []domain.CDNLoadBalanceServer{{ID: "s1", IP: "1.1.1.1"}}}
	uc := app.NewListCDNLoadBalanceServers(&inlineQueue{}, provider)

	servers, err := uc.Execute(context.Background(), app.ListCDNLoadBalanceServersInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", LoadBalanceID: "lb1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(servers) != 1 || servers[0].ID != "s1" {
		t.Errorf("servers = %+v, want a single s1 server", servers)
	}
}

func TestCreateCDNLoadBalanceServerSuccess(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{}
	uc := app.NewCreateCDNLoadBalanceServer(&inlineQueue{}, provider)

	srv, err := uc.Execute(context.Background(), app.CreateCDNLoadBalanceServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		Server: domain.CDNLoadBalanceServer{IP: "1.1.1.1", Group: "primary", Active: true},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if srv.IP != "1.1.1.1" {
		t.Errorf("srv.IP = %q, want 1.1.1.1", srv.IP)
	}
}

func TestCreateCDNLoadBalanceServerRejectsMissingIP(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{}
	uc := app.NewCreateCDNLoadBalanceServer(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNLoadBalanceServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		Server: domain.CDNLoadBalanceServer{Active: true},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdServer != nil {
		t.Error("provider was called with a server missing an IP")
	}
}

func TestGetCDNLoadBalanceServerSuccess(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{servers: []domain.CDNLoadBalanceServer{{ID: "s1", Name: "srv-1"}}}
	uc := app.NewGetCDNLoadBalanceServer(&inlineQueue{}, provider)

	srv, err := uc.Execute(context.Background(), app.GetCDNLoadBalanceServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", ServerID: "s1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if srv.Name != "srv-1" {
		t.Errorf("srv.Name = %q, want srv-1", srv.Name)
	}
}

func TestUpdateCDNLoadBalanceServerSuccess(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{}
	uc := app.NewUpdateCDNLoadBalanceServer(&inlineQueue{}, provider)

	srv, err := uc.Execute(context.Background(), app.UpdateCDNLoadBalanceServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", ServerID: "s1",
		Server: domain.CDNLoadBalanceServer{Name: "srv-1-renamed", IP: "1.1.1.1", Group: "backup", Active: true},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if srv.ID != "s1" || srv.Name != "srv-1-renamed" {
		t.Errorf("srv = %+v, want ID s1 and name srv-1-renamed", srv)
	}
}

func TestUpdateCDNLoadBalanceServerRejectsMissingName(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{}
	uc := app.NewUpdateCDNLoadBalanceServer(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateCDNLoadBalanceServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", ServerID: "s1",
		Server: domain.CDNLoadBalanceServer{IP: "1.1.1.1", Group: "backup", Active: true},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteCDNLoadBalanceServerToleratesNotFound(t *testing.T) {
	provider := &fakeCDNLoadBalanceProvider{deleteServerErr: domain.ErrNotFound}
	uc := app.NewDeleteCDNLoadBalanceServer(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNLoadBalanceServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", ServerID: "s1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil (already-absent server treated as done)", err)
	}
}
