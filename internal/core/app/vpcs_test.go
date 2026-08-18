package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// The VPC fakes and tests live in their own file so the shared fakeProvider in
// app_test.go stays small; only the struct fields it needs are declared there.

func (p *fakeProvider) CreateVPC(_ context.Context, _ domain.ProviderCredentials, vpc domain.VPC) (*domain.VPC, error) {
	vpc.ID = "vpc-1"
	p.createdVPC = &vpc
	return &vpc, nil
}

func (p *fakeProvider) GetVPC(_ context.Context, _ domain.ProviderCredentials, id string) (*domain.VPC, error) {
	for i := range p.vpcs {
		if p.vpcs[i].ID == id {
			return &p.vpcs[i], nil
		}
	}
	return nil, fmt.Errorf("VPC %q: %w", id, domain.ErrNotFound)
}

func (p *fakeProvider) ListVPCs(context.Context, domain.ProviderCredentials) ([]domain.VPC, error) {
	return p.vpcs, nil
}

func (p *fakeProvider) DeleteVPC(_ context.Context, _ domain.ProviderCredentials, id string) error {
	if p.vpcDeleteErr != nil {
		return p.vpcDeleteErr
	}
	p.deletedVPCID = id
	return nil
}

func TestCreateVPCReturnsProviderCopy(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewCreateVPC(&inlineQueue{}, provider)

	vpc, err := uc.Execute(context.Background(), app.CreateVPCInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		VPC:         domain.VPC{Name: "web-net", Region: "tehran", Description: "web tier"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if vpc.ID != "vpc-1" || vpc.Name != "web-net" {
		t.Errorf("vpc = %+v, want id vpc-1 and name web-net", vpc)
	}
	if provider.createdVPC.Region != "tehran" {
		t.Errorf("createdVPC.Region = %q, want tehran", provider.createdVPC.Region)
	}
}

func TestCreateVPCRequiresNameAndRegion(t *testing.T) {
	uc := app.NewCreateVPC(&inlineQueue{}, &fakeProvider{})

	for name, in := range map[string]app.CreateVPCInput{
		"missing name":   {Credentials: domain.ProviderCredentials{APIKey: "k"}, VPC: domain.VPC{Region: "tehran"}},
		"missing region": {Credentials: domain.ProviderCredentials{APIKey: "k"}, VPC: domain.VPC{Name: "web-net"}},
		"missing creds":  {VPC: domain.VPC{Name: "web-net", Region: "tehran"}},
	} {
		if _, err := uc.Execute(context.Background(), in); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: error = %v, want domain.ErrInvalidInput", name, err)
		}
	}
}

func TestListVPCsReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{vpcs: []domain.VPC{{ID: "vpc-1", Name: "web-net"}, {ID: "vpc-2", Name: "db-net"}}}
	uc := app.NewListVPCs(&inlineQueue{}, provider)

	vpcs, err := uc.Execute(context.Background(), app.ListVPCsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(vpcs) != 2 {
		t.Fatalf("len(vpcs) = %d, want 2", len(vpcs))
	}
}

func TestListVPCsRequiresCredentials(t *testing.T) {
	uc := app.NewListVPCs(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.ListVPCsInput{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetVPCReturnsMatchingVPC(t *testing.T) {
	provider := &fakeProvider{vpcs: []domain.VPC{{ID: "vpc-1", Name: "web-net"}}}
	uc := app.NewGetVPC(&inlineQueue{}, provider)

	vpc, err := uc.Execute(context.Background(), app.GetVPCInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		VPCID:       "vpc-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if vpc.Name != "web-net" {
		t.Errorf("Name = %q, want web-net", vpc.Name)
	}
}

func TestGetVPCUnknownID(t *testing.T) {
	uc := app.NewGetVPC(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetVPCInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		VPCID:       "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetVPCRequiresVPCID(t *testing.T) {
	uc := app.NewGetVPC(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetVPCInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteVPCCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewDeleteVPC(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteVPCInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		VPCID:       "vpc-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedVPCID != "vpc-1" {
		t.Errorf("deletedVPCID = %q, want vpc-1", provider.deletedVPCID)
	}
}

// TestDeleteVPCTreatsAlreadyGoneAsSuccess proves delete_vpc can be called more
// than once safely: a not-found response from the provider is not surfaced as
// an error.
func TestDeleteVPCTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeProvider{vpcDeleteErr: fmt.Errorf("VPC vpc-1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteVPC(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteVPCInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		VPCID:       "vpc-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted VPC", err)
	}
}
