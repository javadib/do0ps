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

// fakeArvanCloudProvider embeds the port so a test only needs to override the
// methods it actually exercises (the same pattern as fakeProvider above, for
// ports.ParspackProvider).
type fakeArvanCloudProvider struct {
	ports.ArvanCloudProvider

	domains []domain.ArvanCloudDomain

	createdSpec domain.ArvanCloudDomainSpec
	createErr   error

	deletedName string
	deleteErr   error

	setNSKeysCalledWith []string
	clonedFrom          string
	held, unheld        string
	regenerated         string
}

func (p *fakeArvanCloudProvider) ListDomains(context.Context, domain.ProviderCredentials) ([]domain.ArvanCloudDomain, error) {
	return p.domains, nil
}

func (p *fakeArvanCloudProvider) CreateDomain(_ context.Context, _ domain.ProviderCredentials, spec domain.ArvanCloudDomainSpec) (*domain.ArvanCloudDomain, error) {
	if p.createErr != nil {
		return nil, p.createErr
	}
	p.createdSpec = spec
	created := domain.ArvanCloudDomain{ID: "uuid-1", Name: spec.Name, PlanLevel: spec.PlanLevel, Type: spec.DomainType, Status: "initializing"}
	return &created, nil
}

func (p *fakeArvanCloudProvider) GetDomain(_ context.Context, _ domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	for i := range p.domains {
		if p.domains[i].Name == domainName {
			return &p.domains[i], nil
		}
	}
	return nil, fmt.Errorf("domain %q: %w", domainName, domain.ErrNotFound)
}

func (p *fakeArvanCloudProvider) DeleteDomain(_ context.Context, _ domain.ProviderCredentials, domainName string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedName = domainName
	return nil
}

func (p *fakeArvanCloudProvider) SetNSKeys(_ context.Context, _ domain.ProviderCredentials, domainName string, nsKeys []string) (*domain.ArvanCloudDomain, error) {
	p.setNSKeysCalledWith = nsKeys
	return &domain.ArvanCloudDomain{Name: domainName, NSKeys: nsKeys}, nil
}

func (p *fakeArvanCloudProvider) ResetNSKeys(_ context.Context, _ domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	return &domain.ArvanCloudDomain{Name: domainName, NSKeys: []string{"ns1.arvancloud.ir", "ns2.arvancloud.ir"}}, nil
}

func (p *fakeArvanCloudProvider) CheckNSStatus(_ context.Context, _ domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	return &domain.ArvanCloudDomain{Name: domainName, NSKeys: []string{"h.ns.arvancloud.ir"}, CurrentNS: []string{"ns1.registrar.com"}}, nil
}

func (p *fakeArvanCloudProvider) UseOptionalNSKeys(_ context.Context, _ domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	return &domain.ArvanCloudDomain{Name: domainName, NSKeys: []string{"alt1.arvancloud.ir", "alt2.arvancloud.ir"}}, nil
}

func (p *fakeArvanCloudProvider) SetCnameTarget(_ context.Context, _ domain.ProviderCredentials, domainName, address string) (*domain.ArvanCloudDomain, error) {
	return &domain.ArvanCloudDomain{Name: domainName, Type: domain.ArvanCloudDomainTypePartial, CustomCname: address}, nil
}

func (p *fakeArvanCloudProvider) ResetCnameTarget(_ context.Context, _ domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	return &domain.ArvanCloudDomain{Name: domainName, Type: domain.ArvanCloudDomainTypePartial, CnameTarget: "cdn.arvancloud.ir"}, nil
}

func (p *fakeArvanCloudProvider) ConvertToCnameSetup(_ context.Context, _ domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	return &domain.ArvanCloudDomain{Name: domainName, Type: domain.ArvanCloudDomainTypePartial}, nil
}

func (p *fakeArvanCloudProvider) CheckCnameStatus(_ context.Context, _ domain.ProviderCredentials, domainName string) (*domain.ArvanCloudDomain, error) {
	return &domain.ArvanCloudDomain{Name: domainName, Type: domain.ArvanCloudDomainTypePartial, Status: "active"}, nil
}

func (p *fakeArvanCloudProvider) CloneDomainConfig(_ context.Context, _ domain.ProviderCredentials, _, fromDomain string) error {
	p.clonedFrom = fromDomain
	return nil
}

func (p *fakeArvanCloudProvider) RegenerateDomainConfig(_ context.Context, _ domain.ProviderCredentials, domainName string) error {
	p.regenerated = domainName
	return nil
}

func (p *fakeArvanCloudProvider) HoldDomain(_ context.Context, _ domain.ProviderCredentials, domainName string) error {
	p.held = domainName
	return nil
}

func (p *fakeArvanCloudProvider) UnholdDomain(_ context.Context, _ domain.ProviderCredentials, domainName string) error {
	p.unheld = domainName
	return nil
}

func validArvanCloudCreds() domain.ProviderCredentials {
	return domain.ProviderCredentials{APIKey: "arvan-key"}
}

func TestCreateArvanCloudDomainSuccess(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	uc := app.NewCreateArvanCloudDomain(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudDomainInput{
		Credentials: validArvanCloudCreds(),
		Spec: domain.ArvanCloudDomainSpec{
			Name: "example.com", DomainType: domain.ArvanCloudDomainTypeFull,
			PlanLevel: domain.ArvanCloudPlanGrowth, ImportDNSRecords: true,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.Name != "example.com" || created.Status != "initializing" {
		t.Errorf("created = %+v, want the fake provider's result", created)
	}
	if !provider.createdSpec.ImportDNSRecords {
		t.Error("provider received ImportDNSRecords = false, want the true this test set")
	}
}

func TestCreateArvanCloudDomainValidation(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	uc := app.NewCreateArvanCloudDomain(&inlineQueue{}, provider)

	tests := []struct {
		name string
		spec domain.ArvanCloudDomainSpec
	}{
		{"missing domain", domain.ArvanCloudDomainSpec{}},
		{"bad domain_type", domain.ArvanCloudDomainSpec{Name: "example.com", DomainType: "nope"}},
		{"bad plan_level", domain.ArvanCloudDomainSpec{Name: "example.com", PlanLevel: 99}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), app.CreateArvanCloudDomainInput{Credentials: validArvanCloudCreds(), Spec: tc.spec})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

func TestCreateArvanCloudDomainMissingCredentials(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	uc := app.NewCreateArvanCloudDomain(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudDomainInput{
		Spec: domain.ArvanCloudDomainSpec{Name: "example.com"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestListArvanCloudDomains(t *testing.T) {
	provider := &fakeArvanCloudProvider{domains: []domain.ArvanCloudDomain{
		{Name: "a.com", Status: "active"},
		{Name: "b.com", Status: "pending"},
	}}
	uc := app.NewListArvanCloudDomains(&inlineQueue{}, provider)

	domains, err := uc.Execute(context.Background(), app.ListArvanCloudDomainsInput{Credentials: validArvanCloudCreds()})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(domains) != 2 {
		t.Fatalf("len(domains) = %d, want 2", len(domains))
	}
}

func TestGetArvanCloudDomainNotFound(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	uc := app.NewGetArvanCloudDomain(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetArvanCloudDomainInput{Credentials: validArvanCloudCreds(), DomainName: "missing.com"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

// TestDeleteArvanCloudDomainTolerantOfNotFound is the use-case half of the
// AC #62 pin: the port's DeleteDomain reports domain.ErrNotFound for an
// already-absent domain, and the use case here must treat that as already
// done, exactly like DeleteCDNZone.Execute (internal/core/app/cdn_zone.go).
func TestDeleteArvanCloudDomainTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudProvider{deleteErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudDomain(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudDomainInput{Credentials: validArvanCloudCreds(), DomainName: "gone.com"}); err != nil {
		t.Fatalf("Execute() error = %v, want nil (already-absent domain tolerated)", err)
	}
}

func TestDeleteArvanCloudDomainSuccess(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	uc := app.NewDeleteArvanCloudDomain(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudDomainInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedName != "example.com" {
		t.Errorf("provider.deletedName = %q, want %q", provider.deletedName, "example.com")
	}
}

func TestSetArvanCloudNSKeysValidatesCount(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	uc := app.NewSetArvanCloudNSKeys(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.SetArvanCloudNSKeysInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", NSKeys: []string{"only-one.arvancloud.ir"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestSetArvanCloudNSKeysSuccess(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	uc := app.NewSetArvanCloudNSKeys(&inlineQueue{}, provider)

	keys := []string{"h.ns.arvancloud.ir", "s.ns.arvancloud.ir"}
	updated, err := uc.Execute(context.Background(), app.SetArvanCloudNSKeysInput{
		Credentials: validArvanCloudCreds(), DomainName: "example.com", NSKeys: keys,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(updated.NSKeys) != 2 {
		t.Errorf("updated.NSKeys = %v, want 2 entries", updated.NSKeys)
	}
	if len(provider.setNSKeysCalledWith) != 2 {
		t.Errorf("provider received %v, want the 2 keys passed in", provider.setNSKeysCalledWith)
	}
}

func TestSetArvanCloudCnameTargetValidation(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	uc := app.NewSetArvanCloudCnameTarget(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.SetArvanCloudCnameTargetInput{
		Credentials: validArvanCloudCreds(), DomainName: "sub.example.com", Address: "",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCloneArvanCloudDomainConfig(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	uc := app.NewCloneArvanCloudDomainConfig(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.CloneArvanCloudDomainConfigInput{
		Credentials: validArvanCloudCreds(), DomainName: "new.com", FromDomain: "template.com",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.clonedFrom != "template.com" {
		t.Errorf("provider.clonedFrom = %q, want %q", provider.clonedFrom, "template.com")
	}
}

func TestHoldAndUnholdArvanCloudDomain(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	hold := app.NewHoldArvanCloudDomain(&inlineQueue{}, provider)
	unhold := app.NewUnholdArvanCloudDomain(&inlineQueue{}, provider)

	if err := hold.Execute(context.Background(), app.HoldArvanCloudDomainInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"}); err != nil {
		t.Fatalf("hold Execute: %v", err)
	}
	if provider.held != "example.com" {
		t.Errorf("provider.held = %q, want %q", provider.held, "example.com")
	}

	if err := unhold.Execute(context.Background(), app.UnholdArvanCloudDomainInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"}); err != nil {
		t.Fatalf("unhold Execute: %v", err)
	}
	if provider.unheld != "example.com" {
		t.Errorf("provider.unheld = %q, want %q", provider.unheld, "example.com")
	}
}

func TestRegenerateArvanCloudDomainConfig(t *testing.T) {
	provider := &fakeArvanCloudProvider{}
	uc := app.NewRegenerateArvanCloudDomainConfig(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.RegenerateArvanCloudDomainConfigInput{Credentials: validArvanCloudCreds(), DomainName: "example.com"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.regenerated != "example.com" {
		t.Errorf("provider.regenerated = %q, want %q", provider.regenerated, "example.com")
	}
}
