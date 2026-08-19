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

// fakeBulklistProvider is a package-local fake implementing exactly the
// small provider interface app.NewListCDNBulklists and friends declare
// locally (app.cdnBulklistProvider is unexported, but Go's structural typing
// means this fake satisfies it without ever naming it). Kept in its own file
// rather than added to the shared fakeProvider in app_test.go, per house
// style, since that file is touched by other concurrent work.
type fakeBulklistProvider struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	lists                  []domain.CDNBulklist

	createdSpec domain.CDNBulklistSpec
	createErr   error

	updatedID   string
	updatedSpec domain.CDNBulklistSpec
	updateErr   error

	deletedID string
	deleteErr error
}

func (p *fakeBulklistProvider) ListCDNBulklists(context.Context, domain.ProviderCredentials) ([]domain.CDNBulklist, error) {
	return p.lists, nil
}

func (p *fakeBulklistProvider) CreateCDNBulklist(_ context.Context, _ domain.ProviderCredentials, spec domain.CDNBulklistSpec) (*domain.CDNBulklist, error) {
	if p.createErr != nil {
		return nil, p.createErr
	}
	p.createdSpec = spec
	items := make([]domain.CDNBulklistItem, len(spec.Items))
	for i, v := range spec.Items {
		items[i] = domain.CDNBulklistItem{Value: v}
	}
	return &domain.CDNBulklist{ID: "bl-1", Name: spec.Name, Type: spec.Type, Items: items}, nil
}

func (p *fakeBulklistProvider) GetCDNBulklist(_ context.Context, _ domain.ProviderCredentials, bulklistID string) (*domain.CDNBulklist, error) {
	for i := range p.lists {
		if p.lists[i].ID == bulklistID {
			return &p.lists[i], nil
		}
	}
	return nil, fmt.Errorf("bulklist %q: %w", bulklistID, domain.ErrNotFound)
}

func (p *fakeBulklistProvider) UpdateCDNBulklist(_ context.Context, _ domain.ProviderCredentials, bulklistID string, spec domain.CDNBulklistSpec) (*domain.CDNBulklist, error) {
	if p.updateErr != nil {
		return nil, p.updateErr
	}
	p.updatedID = bulklistID
	p.updatedSpec = spec
	items := make([]domain.CDNBulklistItem, len(spec.Items))
	for i, v := range spec.Items {
		items[i] = domain.CDNBulklistItem{Value: v}
	}
	return &domain.CDNBulklist{ID: bulklistID, Name: spec.Name, Type: spec.Type, Items: items}, nil
}

func (p *fakeBulklistProvider) DeleteCDNBulklist(_ context.Context, _ domain.ProviderCredentials, bulklistID string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedID = bulklistID
	return nil
}

// fakeCountryProvider is the equivalent minimal fake for
// app.cdnFirewallCountryProvider.
type fakeCountryProvider struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	countries              []domain.CDNCountry
}

func (p *fakeCountryProvider) ListCDNFirewallCountries(context.Context, domain.ProviderCredentials, string) ([]domain.CDNCountry, error) {
	return p.countries, nil
}

func TestListCDNBulklistsReturnsProviderResult(t *testing.T) {
	provider := &fakeBulklistProvider{lists: []domain.CDNBulklist{{ID: "bl-1"}, {ID: "bl-2"}}}
	uc := app.NewListCDNBulklists(&inlineQueue{}, provider)

	lists, err := uc.Execute(context.Background(), app.ListCDNBulklistsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(lists) != 2 {
		t.Fatalf("len(lists) = %d, want 2", len(lists))
	}
}

func TestCreateCDNBulklistSuccess(t *testing.T) {
	provider := &fakeBulklistProvider{}
	uc := app.NewCreateCDNBulklist(&inlineQueue{}, provider)

	list, err := uc.Execute(context.Background(), app.CreateCDNBulklistInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Spec:        domain.CDNBulklistSpec{Name: "Blocked IPs", Type: "ip", Items: []string{"192.168.0.1"}},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if list.ID != "bl-1" {
		t.Errorf("ID = %q, want bl-1", list.ID)
	}
	if provider.createdSpec.Name != "Blocked IPs" {
		t.Errorf("created spec name = %q, want Blocked IPs", provider.createdSpec.Name)
	}
}

func TestCreateCDNBulklistRejectsInvalidType(t *testing.T) {
	provider := &fakeBulklistProvider{}
	uc := app.NewCreateCDNBulklist(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNBulklistInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Spec:        domain.CDNBulklistSpec{Name: "x", Type: "bogus", Items: []string{"1.2.3.4"}},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdSpec.Name != "" {
		t.Error("provider was called with an invalid type")
	}
}

func TestCreateCDNBulklistRejectsEmptyItems(t *testing.T) {
	provider := &fakeBulklistProvider{}
	uc := app.NewCreateCDNBulklist(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNBulklistInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Spec:        domain.CDNBulklistSpec{Name: "x", Type: "ip"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNBulklistReturnsMatchingList(t *testing.T) {
	provider := &fakeBulklistProvider{lists: []domain.CDNBulklist{{ID: "bl-1", Name: "list1"}}}
	uc := app.NewGetCDNBulklist(&inlineQueue{}, provider)

	list, err := uc.Execute(context.Background(), app.GetCDNBulklistInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, BulklistID: "bl-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if list.Name != "list1" {
		t.Errorf("Name = %q, want list1", list.Name)
	}
}

func TestGetCDNBulklistUnknownID(t *testing.T) {
	uc := app.NewGetCDNBulklist(&inlineQueue{}, &fakeBulklistProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNBulklistInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, BulklistID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetCDNBulklistRequiresID(t *testing.T) {
	uc := app.NewGetCDNBulklist(&inlineQueue{}, &fakeBulklistProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNBulklistInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNBulklistSuccess(t *testing.T) {
	provider := &fakeBulklistProvider{}
	uc := app.NewUpdateCDNBulklist(&inlineQueue{}, provider)

	list, err := uc.Execute(context.Background(), app.UpdateCDNBulklistInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		BulklistID:  "bl-1",
		Spec:        domain.CDNBulklistSpec{Name: "renamed", Type: "ip", Items: []string{"1.2.3.4"}},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if list.Name != "renamed" {
		t.Errorf("Name = %q, want renamed", list.Name)
	}
	if provider.updatedID != "bl-1" {
		t.Errorf("updatedID = %q, want bl-1", provider.updatedID)
	}
}

func TestDeleteCDNBulklistCallsProvider(t *testing.T) {
	provider := &fakeBulklistProvider{}
	uc := app.NewDeleteCDNBulklist(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNBulklistInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, BulklistID: "bl-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedID != "bl-1" {
		t.Errorf("deletedID = %q, want bl-1", provider.deletedID)
	}
}

// TestDeleteCDNBulklistTreatsAlreadyGoneAsSuccess proves delete_cdn_bulklist
// can be called more than once safely.
func TestDeleteCDNBulklistTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeBulklistProvider{deleteErr: fmt.Errorf("bulklist bl-1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteCDNBulklist(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNBulklistInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, BulklistID: "bl-1"})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted bulklist", err)
	}
}

func TestListCDNFirewallCountriesReturnsProviderResult(t *testing.T) {
	provider := &fakeCountryProvider{countries: []domain.CDNCountry{{Code: "1", Name: "Afghanistan"}}}
	uc := app.NewListCDNFirewallCountries(&inlineQueue{}, provider)

	countries, err := uc.Execute(context.Background(), app.ListCDNFirewallCountriesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(countries) != 1 || countries[0].Name != "Afghanistan" {
		t.Errorf("countries = %+v, want a single Afghanistan entry", countries)
	}
}

func TestListCDNFirewallCountriesRequiresZoneUUID(t *testing.T) {
	uc := app.NewListCDNFirewallCountries(&inlineQueue{}, &fakeCountryProvider{})

	_, err := uc.Execute(context.Background(), app.ListCDNFirewallCountriesInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
