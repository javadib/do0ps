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

// firewallFakeProvider embeds the port so only the firewall methods a test
// needs are written.
type firewallFakeProvider struct {
	ports.ParspackProvider

	firewalls []domain.Firewall
	created   *domain.Firewall
	updated   *domain.Firewall
	deletedID string
	deleteErr error
	createErr error
}

func (p *firewallFakeProvider) CreateFirewall(_ context.Context, _ domain.ProviderCredentials, fw domain.Firewall) (*domain.Firewall, error) {
	if p.createErr != nil {
		return nil, p.createErr
	}
	fw.ID = "fw-1"
	p.created = &fw
	return &fw, nil
}

func (p *firewallFakeProvider) GetFirewall(_ context.Context, _ domain.ProviderCredentials, id string) (*domain.Firewall, error) {
	for i := range p.firewalls {
		if p.firewalls[i].ID == id {
			return &p.firewalls[i], nil
		}
	}
	return nil, fmt.Errorf("firewall %q: %w", id, domain.ErrNotFound)
}

func (p *firewallFakeProvider) ListFirewalls(context.Context, domain.ProviderCredentials) ([]domain.Firewall, error) {
	return p.firewalls, nil
}

func (p *firewallFakeProvider) UpdateFirewall(_ context.Context, _ domain.ProviderCredentials, id string, fw domain.Firewall) (*domain.Firewall, error) {
	fw.ID = id
	p.updated = &fw
	return &fw, nil
}

func (p *firewallFakeProvider) DeleteFirewall(_ context.Context, _ domain.ProviderCredentials, id string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedID = id
	return nil
}

func TestCreateFirewallCallsProvider(t *testing.T) {
	provider := &firewallFakeProvider{}
	uc := app.NewCreateFirewall(&inlineQueue{}, provider)

	fw, err := uc.Execute(context.Background(), app.CreateFirewallInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Firewall: domain.Firewall{
			Name:      "web",
			ServerIDs: []string{"vm-1"},
			InboundRules: []domain.FirewallRule{
				{Protocol: "tcp", PortRange: "22", Addresses: []string{"0.0.0.0/0"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if fw.ID != "fw-1" {
		t.Errorf("ID = %q, want fw-1", fw.ID)
	}
	if provider.created == nil || provider.created.Name != "web" {
		t.Errorf("provider.created = %+v, want the submitted firewall", provider.created)
	}
}

func TestCreateFirewallRequiresName(t *testing.T) {
	uc := app.NewCreateFirewall(&inlineQueue{}, &firewallFakeProvider{})

	_, err := uc.Execute(context.Background(), app.CreateFirewallInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCreateFirewallRequiresCredentials(t *testing.T) {
	uc := app.NewCreateFirewall(&inlineQueue{}, &firewallFakeProvider{})

	_, err := uc.Execute(context.Background(), app.CreateFirewallInput{Firewall: domain.Firewall{Name: "web"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetFirewallReturnsMatchingFirewall(t *testing.T) {
	provider := &firewallFakeProvider{firewalls: []domain.Firewall{{ID: "fw-1", Name: "web"}}}
	uc := app.NewGetFirewall(&inlineQueue{}, provider)

	fw, err := uc.Execute(context.Background(), app.GetFirewallInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		FirewallID:  "fw-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if fw.Name != "web" {
		t.Errorf("Name = %q, want web", fw.Name)
	}
}

func TestGetFirewallUnknownID(t *testing.T) {
	uc := app.NewGetFirewall(&inlineQueue{}, &firewallFakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetFirewallInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		FirewallID:  "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetFirewallRequiresFirewallID(t *testing.T) {
	uc := app.NewGetFirewall(&inlineQueue{}, &firewallFakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetFirewallInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestListFirewallsReturnsProviderResult(t *testing.T) {
	provider := &firewallFakeProvider{firewalls: []domain.Firewall{{ID: "fw-1", Name: "web"}, {ID: "fw-2", Name: "db"}}}
	uc := app.NewListFirewalls(&inlineQueue{}, provider)

	firewalls, err := uc.Execute(context.Background(), app.ListFirewallsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(firewalls) != 2 {
		t.Fatalf("len(firewalls) = %d, want 2", len(firewalls))
	}
}

func TestListFirewallsRequiresCredentials(t *testing.T) {
	uc := app.NewListFirewalls(&inlineQueue{}, &firewallFakeProvider{})

	_, err := uc.Execute(context.Background(), app.ListFirewallsInput{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateFirewallCallsProvider(t *testing.T) {
	provider := &firewallFakeProvider{}
	uc := app.NewUpdateFirewall(&inlineQueue{}, provider)

	fw, err := uc.Execute(context.Background(), app.UpdateFirewallInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		FirewallID:  "fw-1",
		Firewall:    domain.Firewall{Name: "web-renamed"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if fw.ID != "fw-1" {
		t.Errorf("ID = %q, want fw-1", fw.ID)
	}
	if provider.updated == nil || provider.updated.Name != "web-renamed" {
		t.Errorf("provider.updated = %+v, want the submitted firewall", provider.updated)
	}
}

func TestUpdateFirewallRequiresFirewallID(t *testing.T) {
	uc := app.NewUpdateFirewall(&inlineQueue{}, &firewallFakeProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateFirewallInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Firewall:    domain.Firewall{Name: "web"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteFirewallCallsProvider(t *testing.T) {
	provider := &firewallFakeProvider{}
	uc := app.NewDeleteFirewall(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteFirewallInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		FirewallID:  "fw-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedID != "fw-1" {
		t.Errorf("deletedID = %q, want fw-1", provider.deletedID)
	}
}

// TestDeleteFirewallTreatsAlreadyGoneAsSuccess proves delete_firewall can be
// called more than once safely: a not-found response from the provider is
// not surfaced as an error.
func TestDeleteFirewallTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &firewallFakeProvider{deleteErr: fmt.Errorf("firewall fw-1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteFirewall(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteFirewallInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		FirewallID:  "fw-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted firewall", err)
	}
}

func TestDeleteFirewallRequiresFirewallID(t *testing.T) {
	uc := app.NewDeleteFirewall(&inlineQueue{}, &firewallFakeProvider{})

	err := uc.Execute(context.Background(), app.DeleteFirewallInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
