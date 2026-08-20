package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// The reserved-IP fakes and tests live in their own file so the shared
// fakeProvider in app_test.go stays small; only the struct fields it needs are
// declared there.

func (p *fakeProvider) ReserveIP(_ context.Context, _ domain.ProviderCredentials, region string) (*domain.ReservedIP, error) {
	if p.reserveErr != nil {
		return nil, p.reserveErr
	}
	ip := &domain.ReservedIP{IPAddress: "203.0.113.10", Region: region, URN: "do:reservedip:203.0.113.10"}
	p.reserved = append(p.reserved, *ip)
	return ip, nil
}

func (p *fakeProvider) ReleaseIP(_ context.Context, _ domain.ProviderCredentials, ip string) error {
	if p.releaseErr != nil {
		return p.releaseErr
	}
	p.released = ip
	return nil
}

func (p *fakeProvider) AssignIPToServer(_ context.Context, _ domain.ProviderCredentials, ip, serverID string) (*domain.ReservedIP, error) {
	if p.assignErr != nil {
		return nil, p.assignErr
	}
	p.assigned.ip = ip
	p.assigned.serverID = serverID
	return &domain.ReservedIP{IPAddress: ip, Region: "tehran", ServerID: serverID}, nil
}

func (p *fakeProvider) UnassignIP(_ context.Context, _ domain.ProviderCredentials, ip string) (*domain.ReservedIP, error) {
	if p.unassignErr != nil {
		return nil, p.unassignErr
	}
	p.unassigned = ip
	return &domain.ReservedIP{IPAddress: ip, Region: "tehran"}, nil
}

func TestReserveIPReturnsReservedAddress(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewReserveIP(&inlineQueue{}, provider)

	ip, err := uc.Execute(context.Background(), app.ReserveIPInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Region:      "tehran",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if ip.IPAddress != "203.0.113.10" {
		t.Errorf("IPAddress = %q, want 203.0.113.10", ip.IPAddress)
	}
	if ip.ServerID != "" {
		t.Errorf("ServerID = %q, want empty — reservation does not attach the IP", ip.ServerID)
	}
}

func TestReserveIPRequiresRegion(t *testing.T) {
	uc := app.NewReserveIP(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.ReserveIPInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestReleaseIPCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewReleaseIP(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ReleaseIPInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		IPAddress:   "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.released != "203.0.113.10" {
		t.Errorf("released = %q, want 203.0.113.10", provider.released)
	}
}

func TestReleaseIPRequiresAddress(t *testing.T) {
	uc := app.NewReleaseIP(&inlineQueue{}, &fakeProvider{})

	err := uc.Execute(context.Background(), app.ReleaseIPInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

// TestReleaseIPTreatsAlreadyGoneAsSuccess proves release_ip can be called more
// than once safely: a not-found response from the provider is not surfaced as
// an error.
func TestReleaseIPTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeProvider{releaseErr: fmt.Errorf("reserved IP 203.0.113.99: %w", domain.ErrNotFound)}
	uc := app.NewReleaseIP(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ReleaseIPInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		IPAddress:   "203.0.113.99",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-released address", err)
	}
}

func TestAssignIPToServerCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewAssignIPToServer(&inlineQueue{}, provider)

	ip, err := uc.Execute(context.Background(), app.AssignIPToServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		IPAddress:   "203.0.113.10",
		ServerID:    "vm-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.assigned.ip != "203.0.113.10" || provider.assigned.serverID != "vm-1" {
		t.Errorf("assigned = %+v, want ip 203.0.113.10 to server vm-1", provider.assigned)
	}
	if ip.ServerID != "vm-1" {
		t.Errorf("returned ServerID = %q, want vm-1", ip.ServerID)
	}
}

func TestAssignIPToServerRequiresServerID(t *testing.T) {
	uc := app.NewAssignIPToServer(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.AssignIPToServerInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		IPAddress:   "203.0.113.10",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUnassignIPCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewUnassignIP(&inlineQueue{}, provider)

	ip, err := uc.Execute(context.Background(), app.UnassignIPInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		IPAddress:   "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.unassigned != "203.0.113.10" {
		t.Errorf("unassigned = %q, want 203.0.113.10", provider.unassigned)
	}
	if ip.ServerID != "" {
		t.Errorf("returned ServerID = %q, want empty after unassign", ip.ServerID)
	}
}
