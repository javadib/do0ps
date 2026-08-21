package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// fakeArvanCloudLBProvider embeds the port so a test only needs to override
// the methods it actually exercises, the same pattern as
// fakeArvanCloudRateLimitProvider.
type fakeArvanCloudLBProvider struct {
	ports.ArvanCloudProvider

	regions       []domain.ArvanCloudLoadBalancerRegion
	domainRegions []domain.ArvanCloudLoadBalancerRegion

	settings        domain.ArvanCloudLoadBalancerSettings
	updatedSettings domain.ArvanCloudLoadBalancerSettings

	lbs []domain.ArvanCloudLoadBalancer

	createdLB       domain.ArvanCloudLoadBalancer
	createdLBDomain string

	updatedLBID string
	updatedLB   domain.ArvanCloudLoadBalancer

	deleteLBErr error

	pools []domain.ArvanCloudLoadBalancerPool

	createdPool             domain.ArvanCloudLoadBalancerPool
	createdPoolLoadBalancer string

	reprioritizedLoadBalancerID string
	reprioritizedPoolID         string
	reprioritizedAfterPoolID    string
	reprioritizedBeforePoolID   string
	reprioritizeResult          domain.ArvanCloudLoadBalancer

	updatedWithOriginsPool domain.ArvanCloudLoadBalancerPool
	updatedSettingsPool    domain.ArvanCloudLoadBalancerPool

	deletePoolErr error

	origins []domain.ArvanCloudLoadBalancerOrigin

	createdOrigin       domain.ArvanCloudLoadBalancerOrigin
	createdOriginPoolID string

	updatedOriginID string
	updatedOrigin   domain.ArvanCloudLoadBalancerOrigin

	deleteOriginErr error
}

func (p *fakeArvanCloudLBProvider) ListArvanCloudLBRegions(context.Context, domain.ProviderCredentials) ([]domain.ArvanCloudLoadBalancerRegion, error) {
	return p.regions, nil
}

func (p *fakeArvanCloudLBProvider) ListArvanCloudDomainLBRegions(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudLoadBalancerRegion, error) {
	return p.domainRegions, nil
}

func (p *fakeArvanCloudLBProvider) GetArvanCloudLBSettings(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudLoadBalancerSettings, error) {
	settings := p.settings
	return &settings, nil
}

func (p *fakeArvanCloudLBProvider) UpdateArvanCloudLBSettings(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.ArvanCloudLoadBalancerSettings) (*domain.ArvanCloudLoadBalancerSettings, error) {
	p.updatedSettings = settings
	return &settings, nil
}

func (p *fakeArvanCloudLBProvider) ListArvanCloudLoadBalancers(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudLoadBalancer, error) {
	return p.lbs, nil
}

func (p *fakeArvanCloudLBProvider) CreateArvanCloudLoadBalancer(_ context.Context, _ domain.ProviderCredentials, domainName string, lb domain.ArvanCloudLoadBalancer) (*domain.ArvanCloudLoadBalancer, error) {
	p.createdLB = lb
	p.createdLBDomain = domainName
	created := lb
	created.ID = "lb-1"
	return &created, nil
}

func (p *fakeArvanCloudLBProvider) GetArvanCloudLoadBalancer(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudLoadBalancer, error) {
	for i := range p.lbs {
		if p.lbs[i].ID == id {
			return &p.lbs[i], nil
		}
	}
	return nil, fmt.Errorf("load balancer %q: %w", id, domain.ErrNotFound)
}

func (p *fakeArvanCloudLBProvider) UpdateArvanCloudLoadBalancer(_ context.Context, _ domain.ProviderCredentials, _ string, id string, lb domain.ArvanCloudLoadBalancer) (*domain.ArvanCloudLoadBalancer, error) {
	p.updatedLBID = id
	p.updatedLB = lb
	updated := lb
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudLBProvider) DeleteArvanCloudLoadBalancer(context.Context, domain.ProviderCredentials, string, string) error {
	return p.deleteLBErr
}

func (p *fakeArvanCloudLBProvider) ListArvanCloudLBPools(context.Context, domain.ProviderCredentials, string, string) ([]domain.ArvanCloudLoadBalancerPool, error) {
	return p.pools, nil
}

func (p *fakeArvanCloudLBProvider) CreateArvanCloudLBPool(_ context.Context, _ domain.ProviderCredentials, _ string, loadBalancerID string, pool domain.ArvanCloudLoadBalancerPool) (*domain.ArvanCloudLoadBalancerPool, error) {
	p.createdPool = pool
	p.createdPoolLoadBalancer = loadBalancerID
	created := pool
	created.ID = "pool-1"
	return &created, nil
}

func (p *fakeArvanCloudLBProvider) ReprioritizeArvanCloudLBPool(_ context.Context, _ domain.ProviderCredentials, _ string, loadBalancerID, poolID, afterPoolID, beforePoolID string) (*domain.ArvanCloudLoadBalancer, error) {
	p.reprioritizedLoadBalancerID = loadBalancerID
	p.reprioritizedPoolID = poolID
	p.reprioritizedAfterPoolID = afterPoolID
	p.reprioritizedBeforePoolID = beforePoolID
	result := p.reprioritizeResult
	return &result, nil
}

func (p *fakeArvanCloudLBProvider) GetArvanCloudLBPool(_ context.Context, _ domain.ProviderCredentials, _ string, _ string, poolID string) (*domain.ArvanCloudLoadBalancerPool, error) {
	for i := range p.pools {
		if p.pools[i].ID == poolID {
			return &p.pools[i], nil
		}
	}
	return nil, fmt.Errorf("pool %q: %w", poolID, domain.ErrNotFound)
}

func (p *fakeArvanCloudLBProvider) UpdateArvanCloudLBPoolWithOrigins(_ context.Context, _ domain.ProviderCredentials, _ string, _ string, poolID string, pool domain.ArvanCloudLoadBalancerPool) (*domain.ArvanCloudLoadBalancerPool, error) {
	p.updatedWithOriginsPool = pool
	updated := pool
	updated.ID = poolID
	return &updated, nil
}

func (p *fakeArvanCloudLBProvider) UpdateArvanCloudLBPoolSettings(_ context.Context, _ domain.ProviderCredentials, _ string, _ string, poolID string, pool domain.ArvanCloudLoadBalancerPool) (*domain.ArvanCloudLoadBalancerPool, error) {
	p.updatedSettingsPool = pool
	updated := pool
	updated.ID = poolID
	return &updated, nil
}

func (p *fakeArvanCloudLBProvider) DeleteArvanCloudLBPool(context.Context, domain.ProviderCredentials, string, string, string) error {
	return p.deletePoolErr
}

func (p *fakeArvanCloudLBProvider) ListArvanCloudLBPoolOrigins(context.Context, domain.ProviderCredentials, string, string, string) ([]domain.ArvanCloudLoadBalancerOrigin, error) {
	return p.origins, nil
}

func (p *fakeArvanCloudLBProvider) CreateArvanCloudLBPoolOrigin(_ context.Context, _ domain.ProviderCredentials, _ string, _ string, poolID string, origin domain.ArvanCloudLoadBalancerOrigin) (*domain.ArvanCloudLoadBalancerOrigin, error) {
	p.createdOrigin = origin
	p.createdOriginPoolID = poolID
	created := origin
	created.ID = "origin-1"
	return &created, nil
}

func (p *fakeArvanCloudLBProvider) GetArvanCloudLBPoolOrigin(_ context.Context, _ domain.ProviderCredentials, _ string, _ string, _ string, originID string) (*domain.ArvanCloudLoadBalancerOrigin, error) {
	for i := range p.origins {
		if p.origins[i].ID == originID {
			return &p.origins[i], nil
		}
	}
	return nil, fmt.Errorf("origin %q: %w", originID, domain.ErrNotFound)
}

func (p *fakeArvanCloudLBProvider) UpdateArvanCloudLBPoolOrigin(_ context.Context, _ domain.ProviderCredentials, _ string, _ string, _ string, originID string, origin domain.ArvanCloudLoadBalancerOrigin) (*domain.ArvanCloudLoadBalancerOrigin, error) {
	p.updatedOriginID = originID
	p.updatedOrigin = origin
	updated := origin
	updated.ID = originID
	return &updated, nil
}

func (p *fakeArvanCloudLBProvider) DeleteArvanCloudLBPoolOrigin(context.Context, domain.ProviderCredentials, string, string, string, string) error {
	return p.deleteOriginErr
}

// --- Regions ----------------------------------------------------------------

func TestListArvanCloudLBRegionsSuccess(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{regions: []domain.ArvanCloudLoadBalancerRegion{{ID: "r1", Region: "LAH"}}}
	uc := app.NewListArvanCloudLBRegions(&inlineQueue{}, provider)

	regions, err := uc.Execute(context.Background(), app.ListArvanCloudLBRegionsInput{Credentials: validArvanCloudCreds()})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(regions) != 1 || regions[0].Region != "LAH" {
		t.Errorf("regions = %+v, want one LAH region", regions)
	}
}

func TestListArvanCloudDomainLBRegionsSuccess(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{domainRegions: []domain.ArvanCloudLoadBalancerRegion{{ID: "r1", Region: "THR"}}}
	uc := app.NewListArvanCloudDomainLBRegions(&inlineQueue{}, provider)

	regions, err := uc.Execute(context.Background(), app.ListArvanCloudDomainLBRegionsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(regions) != 1 || regions[0].Region != "THR" {
		t.Errorf("regions = %+v, want one THR region", regions)
	}
}

// --- Settings -----------------------------------------------------------

func TestGetArvanCloudLBSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{settings: domain.ArvanCloudLoadBalancerSettings{GRPCStatus: true}}
	uc := app.NewGetArvanCloudLBSettings(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetArvanCloudLBSettingsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !settings.GRPCStatus {
		t.Errorf("settings.GRPCStatus = %v, want true", settings.GRPCStatus)
	}
}

func TestUpdateArvanCloudLBSettingsRejectsInvalidMethod(t *testing.T) {
	uc := app.NewUpdateArvanCloudLBSettings(&inlineQueue{}, &fakeArvanCloudLBProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudLBSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudLoadBalancerSettings{Method: "failover"}, // pool-level method has no failover
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateArvanCloudLBSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{}
	uc := app.NewUpdateArvanCloudLBSettings(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudLBSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudLoadBalancerSettings{Method: domain.ArvanCloudPoolMethodClusterRR, GRPCStatus: false},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.updatedSettings.Method != domain.ArvanCloudPoolMethodClusterRR {
		t.Errorf("provider.updatedSettings.Method = %q, want %q", provider.updatedSettings.Method, domain.ArvanCloudPoolMethodClusterRR)
	}
}

// --- Load balancers -------------------------------------------------------

func validArvanCloudLoadBalancer() domain.ArvanCloudLoadBalancer {
	return domain.ArvanCloudLoadBalancer{
		Name:   "lb1",
		Status: true,
		Method: domain.ArvanCloudLoadBalancerMethodFailover,
	}
}

func TestCreateArvanCloudLoadBalancerSuccess(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{}
	uc := app.NewCreateArvanCloudLoadBalancer(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudLoadBalancerInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LB: validArvanCloudLoadBalancer(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "lb-1" {
		t.Errorf("created.ID = %q, want %q", created.ID, "lb-1")
	}
}

func TestCreateArvanCloudLoadBalancerRejectsMissingName(t *testing.T) {
	uc := app.NewCreateArvanCloudLoadBalancer(&inlineQueue{}, &fakeArvanCloudLBProvider{})
	lb := validArvanCloudLoadBalancer()
	lb.Name = ""

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudLoadBalancerInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LB: lb,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCreateArvanCloudLoadBalancerRejectsInvalidMethod(t *testing.T) {
	uc := app.NewCreateArvanCloudLoadBalancer(&inlineQueue{}, &fakeArvanCloudLBProvider{})
	lb := validArvanCloudLoadBalancer()
	lb.Method = "round_robin"

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudLoadBalancerInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LB: lb,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetArvanCloudLoadBalancerNotFound(t *testing.T) {
	uc := app.NewGetArvanCloudLoadBalancer(&inlineQueue{}, &fakeArvanCloudLBProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudLoadBalancerInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateArvanCloudLoadBalancerSuccess(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{}
	uc := app.NewUpdateArvanCloudLoadBalancer(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudLoadBalancerInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "lb-1", LB: validArvanCloudLoadBalancer(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.ID != "lb-1" || provider.updatedLBID != "lb-1" {
		t.Errorf("updated = %+v, provider.updatedLBID = %q, want lb-1", updated, provider.updatedLBID)
	}
}

func TestDeleteArvanCloudLoadBalancerTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{deleteLBErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudLoadBalancer(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudLoadBalancerInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "lb-1"}); err != nil {
		t.Errorf("Execute() error = %v, want nil (tolerant delete)", err)
	}
}

// --- Pools ------------------------------------------------------------------

func validArvanCloudLBPool() domain.ArvanCloudLoadBalancerPool {
	return domain.ArvanCloudLoadBalancerPool{
		Name:      "pool1",
		Status:    true,
		Method:    domain.ArvanCloudPoolMethodClusterRR,
		Keepalive: domain.ArvanCloudLBOn,
	}
}

func TestCreateArvanCloudLBPoolSuccess(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{}
	uc := app.NewCreateArvanCloudLBPool(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudLBPoolInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", Pool: validArvanCloudLBPool(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "pool-1" || provider.createdPoolLoadBalancer != "lb-1" {
		t.Errorf("created = %+v, provider.createdPoolLoadBalancer = %q, want pool-1/lb-1", created, provider.createdPoolLoadBalancer)
	}
}

// TestCreateArvanCloudLBPoolRejectsFailoverMethod covers the confirmed
// asymmetry issue #69 flags explicitly: a pool's method enum has no
// "failover" value, unlike the parent load balancer's — sending it must be
// rejected client-side, not left to surface as an opaque 422.
func TestCreateArvanCloudLBPoolRejectsFailoverMethod(t *testing.T) {
	uc := app.NewCreateArvanCloudLBPool(&inlineQueue{}, &fakeArvanCloudLBProvider{})
	pool := validArvanCloudLBPool()
	pool.Method = domain.ArvanCloudPoolMethod(domain.ArvanCloudLoadBalancerMethodFailover)

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudLBPoolInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", Pool: pool,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCreateArvanCloudLBPoolRejectsInvalidOrigin(t *testing.T) {
	uc := app.NewCreateArvanCloudLBPool(&inlineQueue{}, &fakeArvanCloudLBProvider{})
	pool := validArvanCloudLBPool()
	pool.Origins = []domain.ArvanCloudLoadBalancerOrigin{{Address: "203.0.113.10", Port: 0, Weight: 100, Protocol: domain.ArvanCloudOriginProtocolAuto}}

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudLBPoolInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", Pool: pool,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput (invalid port)", err)
	}
}

// TestReprioritizeArvanCloudLBPoolReturnsLoadBalancer proves the use case
// returns the load balancer resource (not a bare confirmation), matching
// load-balancers.prioritize_pool's documented response shape — unlike the
// rule-engine reprioritize use cases elsewhere in this package.
func TestReprioritizeArvanCloudLBPoolReturnsLoadBalancer(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{reprioritizeResult: domain.ArvanCloudLoadBalancer{ID: "lb-1", Name: "lb1"}}
	uc := app.NewReprioritizeArvanCloudLBPool(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudLBPoolInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1",
		PoolID: "pool-1", BeforePoolID: "pool-2",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.ID != "lb-1" {
		t.Errorf("updated.ID = %q, want %q", updated.ID, "lb-1")
	}
	if provider.reprioritizedPoolID != "pool-1" || provider.reprioritizedBeforePoolID != "pool-2" {
		t.Errorf("provider reprioritize call = pool:%q before:%q, want pool-1/pool-2", provider.reprioritizedPoolID, provider.reprioritizedBeforePoolID)
	}
}

// TestReprioritizeArvanCloudLBPoolRequiresExactlyOneOfAfterBefore covers
// validateArvanCloudLBPoolPrioritize's "exactly one of after/before"
// contract, including that omitting BOTH is also rejected (the
// PrioritizePool schema is an anyOf of two schemas, each requiring its own
// field — unlike the rule-engine reprioritize contract, where both may be
// omitted for a firewall-style rule since that shape allows it).
func TestReprioritizeArvanCloudLBPoolRequiresExactlyOneOfAfterBefore(t *testing.T) {
	tests := []struct {
		name         string
		afterPoolID  string
		beforePoolID string
	}{
		{"both given", "pool-2", "pool-3"},
		{"neither given", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := app.NewReprioritizeArvanCloudLBPool(&inlineQueue{}, &fakeArvanCloudLBProvider{})
			_, err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudLBPoolInput{
				Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1",
				PoolID: "pool-1", AfterPoolID: tc.afterPoolID, BeforePoolID: tc.beforePoolID,
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

func TestGetArvanCloudLBPoolNotFound(t *testing.T) {
	uc := app.NewGetArvanCloudLBPool(&inlineQueue{}, &fakeArvanCloudLBProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudLBPoolInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", PoolID: "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

// TestReplaceArvanCloudLBPoolWithOriginsSendsOrigins and
// TestUpdateArvanCloudLBPoolSettingsNeverSendsOrigins are the use-case-layer
// counterpart to the adapter-layer regression tests
// (TestUpdateArvanCloudLBPoolWithOriginsReplacesOrigins/
// TestUpdateArvanCloudLBPoolSettingsLeavesOriginsUntouched in
// internal/adapters/providers/arvancloud/loadbalancer_test.go): proving the
// two distinct use cases call the two distinct port methods with the
// pool value the caller gave them, unmodified — issue #69's acceptance
// criteria calls this the highest-risk part of this capability.
func TestReplaceArvanCloudLBPoolWithOriginsSendsOrigins(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{}
	uc := app.NewReplaceArvanCloudLBPoolWithOrigins(&inlineQueue{}, provider)

	pool := validArvanCloudLBPool()
	pool.Origins = []domain.ArvanCloudLoadBalancerOrigin{
		{Address: "203.0.113.10", Port: 443, Weight: 100, Protocol: domain.ArvanCloudOriginProtocolHTTPS},
	}

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudLBPoolInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", PoolID: "pool-1", Pool: pool,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(provider.updatedWithOriginsPool.Origins) != 1 {
		t.Errorf("provider.updatedWithOriginsPool.Origins = %+v, want the one origin passed through", provider.updatedWithOriginsPool.Origins)
	}
	if updated.ID != "pool-1" {
		t.Errorf("updated.ID = %q, want %q", updated.ID, "pool-1")
	}
}

func TestUpdateArvanCloudLBPoolSettingsNeverSendsOrigins(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{}
	uc := app.NewUpdateArvanCloudLBPoolSettings(&inlineQueue{}, provider)

	pool := validArvanCloudLBPool() // no Origins set, mirroring a settings-only caller

	if _, err := uc.Execute(context.Background(), app.UpdateArvanCloudLBPoolInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", PoolID: "pool-1", Pool: pool,
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.updatedSettingsPool.Origins != nil {
		t.Errorf("provider.updatedSettingsPool.Origins = %+v, want nil — this call must never touch origins", provider.updatedSettingsPool.Origins)
	}
}

func TestDeleteArvanCloudLBPoolTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{deletePoolErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudLBPool(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudLBPoolInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", PoolID: "pool-1",
	}); err != nil {
		t.Errorf("Execute() error = %v, want nil (tolerant delete)", err)
	}
}

// --- Origins ------------------------------------------------------------

func validArvanCloudLBOrigin() domain.ArvanCloudLoadBalancerOrigin {
	return domain.ArvanCloudLoadBalancerOrigin{
		Address:  "203.0.113.10",
		Port:     443,
		Weight:   100,
		Protocol: domain.ArvanCloudOriginProtocolHTTPS,
		Status:   true,
	}
}

func TestCreateArvanCloudLBPoolOriginSuccess(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{}
	uc := app.NewCreateArvanCloudLBPoolOrigin(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudLBPoolOriginInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", PoolID: "pool-1", Origin: validArvanCloudLBOrigin(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "origin-1" || provider.createdOriginPoolID != "pool-1" {
		t.Errorf("created = %+v, provider.createdOriginPoolID = %q, want origin-1/pool-1", created, provider.createdOriginPoolID)
	}
}

// TestCreateArvanCloudLBPoolOriginRejectsOutOfRangeFields covers issue #69's
// acceptance criteria: address/port/weight/protocol are validated
// client-side rather than surfaced as an opaque 422.
func TestCreateArvanCloudLBPoolOriginRejectsOutOfRangeFields(t *testing.T) {
	tests := []struct {
		name   string
		origin domain.ArvanCloudLoadBalancerOrigin
	}{
		{"missing address", func() domain.ArvanCloudLoadBalancerOrigin { o := validArvanCloudLBOrigin(); o.Address = ""; return o }()},
		{"port too low", func() domain.ArvanCloudLoadBalancerOrigin { o := validArvanCloudLBOrigin(); o.Port = 0; return o }()},
		{"port too high", func() domain.ArvanCloudLoadBalancerOrigin { o := validArvanCloudLBOrigin(); o.Port = 70000; return o }()},
		{"weight too low", func() domain.ArvanCloudLoadBalancerOrigin { o := validArvanCloudLBOrigin(); o.Weight = 0; return o }()},
		{"weight too high", func() domain.ArvanCloudLoadBalancerOrigin { o := validArvanCloudLBOrigin(); o.Weight = 1001; return o }()},
		{"invalid protocol", func() domain.ArvanCloudLoadBalancerOrigin {
			o := validArvanCloudLBOrigin()
			o.Protocol = "ftp"
			return o
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := app.NewCreateArvanCloudLBPoolOrigin(&inlineQueue{}, &fakeArvanCloudLBProvider{})
			_, err := uc.Execute(context.Background(), app.CreateArvanCloudLBPoolOriginInput{
				Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", PoolID: "pool-1", Origin: tc.origin,
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

func TestGetArvanCloudLBPoolOriginNotFound(t *testing.T) {
	uc := app.NewGetArvanCloudLBPoolOrigin(&inlineQueue{}, &fakeArvanCloudLBProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudLBPoolOriginInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", PoolID: "pool-1", OriginID: "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateArvanCloudLBPoolOriginSuccess(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{}
	uc := app.NewUpdateArvanCloudLBPoolOrigin(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudLBPoolOriginInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", PoolID: "pool-1",
		OriginID: "origin-1", Origin: validArvanCloudLBOrigin(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.ID != "origin-1" || provider.updatedOriginID != "origin-1" {
		t.Errorf("updated = %+v, provider.updatedOriginID = %q, want origin-1", updated, provider.updatedOriginID)
	}
}

func TestDeleteArvanCloudLBPoolOriginTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudLBProvider{deleteOriginErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudLBPoolOrigin(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudLBPoolOriginInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", LoadBalancerID: "lb-1", PoolID: "pool-1", OriginID: "origin-1",
	}); err != nil {
		t.Errorf("Execute() error = %v, want nil (tolerant delete)", err)
	}
}
