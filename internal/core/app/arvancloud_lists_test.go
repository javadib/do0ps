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

// fakeArvanCloudListsProvider embeds the port so a test only needs to
// override the methods it actually exercises (the same pattern as
// fakeArvanCloudProvider, for the domain onboarding use cases).
type fakeArvanCloudListsProvider struct {
	ports.ArvanCloudProvider

	fields []domain.ArvanCloudDynamicField

	createdField domain.ArvanCloudDynamicField
	createErr    error

	updatedID          string
	updatedDescription string
	updatedType        domain.ArvanCloudDynamicFieldType

	deletedID string
	deleteErr error

	addedID     string
	addedValues []domain.ArvanCloudDynamicFieldValue

	removedID     string
	removedItemID string
	removeErr     error
}

func (p *fakeArvanCloudListsProvider) ListArvanCloudDynamicFields(context.Context, domain.ProviderCredentials) ([]domain.ArvanCloudDynamicField, error) {
	return p.fields, nil
}

func (p *fakeArvanCloudListsProvider) CreateArvanCloudDynamicField(_ context.Context, _ domain.ProviderCredentials, field domain.ArvanCloudDynamicField) (*domain.ArvanCloudDynamicField, error) {
	if p.createErr != nil {
		return nil, p.createErr
	}
	p.createdField = field
	created := field
	created.ID = "list-1"
	created.Scope = domain.ArvanCloudDynamicFieldScopePrivate
	return &created, nil
}

func (p *fakeArvanCloudListsProvider) GetArvanCloudDynamicField(_ context.Context, _ domain.ProviderCredentials, id string) (*domain.ArvanCloudDynamicField, error) {
	for i := range p.fields {
		if p.fields[i].ID == id {
			return &p.fields[i], nil
		}
	}
	return nil, fmt.Errorf("dynamic field %q: %w", id, domain.ErrNotFound)
}

func (p *fakeArvanCloudListsProvider) UpdateArvanCloudDynamicField(_ context.Context, _ domain.ProviderCredentials, id, description string, fieldType domain.ArvanCloudDynamicFieldType) (*domain.ArvanCloudDynamicField, error) {
	p.updatedID = id
	p.updatedDescription = description
	p.updatedType = fieldType
	return &domain.ArvanCloudDynamicField{ID: id, Description: description, Type: fieldType}, nil
}

func (p *fakeArvanCloudListsProvider) DeleteArvanCloudDynamicField(_ context.Context, _ domain.ProviderCredentials, id string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedID = id
	return nil
}

func (p *fakeArvanCloudListsProvider) AddArvanCloudDynamicFieldItems(_ context.Context, _ domain.ProviderCredentials, id string, values []domain.ArvanCloudDynamicFieldValue) error {
	p.addedID = id
	p.addedValues = values
	return nil
}

func (p *fakeArvanCloudListsProvider) RemoveArvanCloudDynamicFieldItem(_ context.Context, _ domain.ProviderCredentials, id, itemID string) error {
	if p.removeErr != nil {
		return p.removeErr
	}
	p.removedID = id
	p.removedItemID = itemID
	return nil
}

func TestListArvanCloudDynamicFields(t *testing.T) {
	provider := &fakeArvanCloudListsProvider{fields: []domain.ArvanCloudDynamicField{
		{ID: "list-1", Name: "bad-ips", Type: domain.ArvanCloudDynamicFieldTypeIP},
		{ID: "list-2", Name: "asns", Type: domain.ArvanCloudDynamicFieldTypeNumber},
	}}
	uc := app.NewListArvanCloudDynamicFields(&inlineQueue{}, provider)

	fields, err := uc.Execute(context.Background(), app.ListArvanCloudDynamicFieldsInput{Credentials: validArvanCloudCreds()})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("len(fields) = %d, want 2", len(fields))
	}
}

func TestListArvanCloudDynamicFieldsMissingCredentials(t *testing.T) {
	uc := app.NewListArvanCloudDynamicFields(&inlineQueue{}, &fakeArvanCloudListsProvider{})
	if _, err := uc.Execute(context.Background(), app.ListArvanCloudDynamicFieldsInput{}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCreateArvanCloudDynamicFieldSuccess(t *testing.T) {
	provider := &fakeArvanCloudListsProvider{}
	uc := app.NewCreateArvanCloudDynamicField(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudDynamicFieldInput{
		Credentials: validArvanCloudCreds(),
		Field:       domain.ArvanCloudDynamicField{Name: "bad-ips", Type: domain.ArvanCloudDynamicFieldTypeIP},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "list-1" || created.Name != "bad-ips" {
		t.Errorf("created = %+v, want the fake provider's result", created)
	}
	if provider.createdField.Type != domain.ArvanCloudDynamicFieldTypeIP {
		t.Errorf("provider received type %q, want %q", provider.createdField.Type, domain.ArvanCloudDynamicFieldTypeIP)
	}
}

func TestCreateArvanCloudDynamicFieldValidation(t *testing.T) {
	uc := app.NewCreateArvanCloudDynamicField(&inlineQueue{}, &fakeArvanCloudListsProvider{})

	tests := []struct {
		name  string
		field domain.ArvanCloudDynamicField
	}{
		{"missing name", domain.ArvanCloudDynamicField{Type: domain.ArvanCloudDynamicFieldTypeIP}},
		{"missing type", domain.ArvanCloudDynamicField{Name: "bad-ips"}},
		{"bad type", domain.ArvanCloudDynamicField{Name: "bad-ips", Type: "string"}},
		{"spec's list-filter typo is not a real type", domain.ArvanCloudDynamicField{Name: "x", Type: "bytes"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), app.CreateArvanCloudDynamicFieldInput{Credentials: validArvanCloudCreds(), Field: tc.field})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

func TestGetArvanCloudDynamicFieldNotFound(t *testing.T) {
	provider := &fakeArvanCloudListsProvider{}
	uc := app.NewGetArvanCloudDynamicField(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetArvanCloudDynamicFieldInput{Credentials: validArvanCloudCreds(), ID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetArvanCloudDynamicFieldMissingID(t *testing.T) {
	uc := app.NewGetArvanCloudDynamicField(&inlineQueue{}, &fakeArvanCloudListsProvider{})
	if _, err := uc.Execute(context.Background(), app.GetArvanCloudDynamicFieldInput{Credentials: validArvanCloudCreds()}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateArvanCloudDynamicFieldSuccess(t *testing.T) {
	provider := &fakeArvanCloudListsProvider{}
	uc := app.NewUpdateArvanCloudDynamicField(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudDynamicFieldInput{
		Credentials: validArvanCloudCreds(), ID: "list-1", Description: "known scanners", Type: domain.ArvanCloudDynamicFieldTypeIP,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.Description != "known scanners" {
		t.Errorf("updated.Description = %q, want %q", updated.Description, "known scanners")
	}
	if provider.updatedID != "list-1" || provider.updatedType != domain.ArvanCloudDynamicFieldTypeIP {
		t.Errorf("provider received id=%q type=%q, want list-1/ip", provider.updatedID, provider.updatedType)
	}
}

func TestUpdateArvanCloudDynamicFieldValidation(t *testing.T) {
	uc := app.NewUpdateArvanCloudDynamicField(&inlineQueue{}, &fakeArvanCloudListsProvider{})

	tests := []struct {
		name string
		in   app.UpdateArvanCloudDynamicFieldInput
	}{
		{"missing id", app.UpdateArvanCloudDynamicFieldInput{Credentials: validArvanCloudCreds(), Type: domain.ArvanCloudDynamicFieldTypeIP}},
		{"missing type", app.UpdateArvanCloudDynamicFieldInput{Credentials: validArvanCloudCreds(), ID: "list-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := uc.Execute(context.Background(), tc.in); !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

func TestDeleteArvanCloudDynamicFieldTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudListsProvider{deleteErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudDynamicField(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudDynamicFieldInput{Credentials: validArvanCloudCreds(), ID: "gone"}); err != nil {
		t.Fatalf("Execute() error = %v, want nil (already-absent list tolerated)", err)
	}
}

func TestDeleteArvanCloudDynamicFieldSuccess(t *testing.T) {
	provider := &fakeArvanCloudListsProvider{}
	uc := app.NewDeleteArvanCloudDynamicField(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudDynamicFieldInput{Credentials: validArvanCloudCreds(), ID: "list-1"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedID != "list-1" {
		t.Errorf("provider.deletedID = %q, want %q", provider.deletedID, "list-1")
	}
}

func TestAddArvanCloudDynamicFieldItemsSuccess(t *testing.T) {
	provider := &fakeArvanCloudListsProvider{}
	uc := app.NewAddArvanCloudDynamicFieldItems(&inlineQueue{}, provider)

	values := []domain.ArvanCloudDynamicFieldValue{{Value: "203.0.113.5"}}
	if err := uc.Execute(context.Background(), app.AddArvanCloudDynamicFieldItemsInput{
		Credentials: validArvanCloudCreds(), ID: "list-1", Values: values,
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.addedID != "list-1" || len(provider.addedValues) != 1 {
		t.Errorf("provider received id=%q values=%+v, want list-1 with 1 value", provider.addedID, provider.addedValues)
	}
}

func TestAddArvanCloudDynamicFieldItemsValidation(t *testing.T) {
	uc := app.NewAddArvanCloudDynamicFieldItems(&inlineQueue{}, &fakeArvanCloudListsProvider{})

	tests := []struct {
		name string
		in   app.AddArvanCloudDynamicFieldItemsInput
	}{
		{"missing id", app.AddArvanCloudDynamicFieldItemsInput{Credentials: validArvanCloudCreds(), Values: []domain.ArvanCloudDynamicFieldValue{{Value: "1.2.3.4"}}}},
		{"empty values", app.AddArvanCloudDynamicFieldItemsInput{Credentials: validArvanCloudCreds(), ID: "list-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := uc.Execute(context.Background(), tc.in); !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

func TestRemoveArvanCloudDynamicFieldItemSuccess(t *testing.T) {
	provider := &fakeArvanCloudListsProvider{}
	uc := app.NewRemoveArvanCloudDynamicFieldItem(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.RemoveArvanCloudDynamicFieldItemInput{
		Credentials: validArvanCloudCreds(), ID: "list-1", ItemID: "item-1",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.removedID != "list-1" || provider.removedItemID != "item-1" {
		t.Errorf("provider received id=%q item_id=%q, want list-1/item-1", provider.removedID, provider.removedItemID)
	}
}

func TestRemoveArvanCloudDynamicFieldItemTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudListsProvider{removeErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewRemoveArvanCloudDynamicFieldItem(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.RemoveArvanCloudDynamicFieldItemInput{
		Credentials: validArvanCloudCreds(), ID: "list-1", ItemID: "gone",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil (already-absent item tolerated)", err)
	}
}

func TestRemoveArvanCloudDynamicFieldItemValidation(t *testing.T) {
	uc := app.NewRemoveArvanCloudDynamicFieldItem(&inlineQueue{}, &fakeArvanCloudListsProvider{})

	tests := []struct {
		name string
		in   app.RemoveArvanCloudDynamicFieldItemInput
	}{
		{"missing id", app.RemoveArvanCloudDynamicFieldItemInput{Credentials: validArvanCloudCreds(), ItemID: "item-1"}},
		{"missing item_id", app.RemoveArvanCloudDynamicFieldItemInput{Credentials: validArvanCloudCreds(), ID: "list-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := uc.Execute(context.Background(), tc.in); !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}
