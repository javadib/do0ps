package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// fakeArvanCloudAccountSSLProvider embeds the port so a test only needs to
// override the methods it actually exercises (the same pattern as
// fakeArvanCloudSSLProvider in issue_arvancloud_managed_certificate_test.go,
// for ports.ArvanCloudProvider).
type fakeArvanCloudAccountSSLProvider struct {
	ports.ArvanCloudProvider

	products    []domain.ArvanCloudCertificateProduct
	productsErr error

	issueCalls int
	issueErr   error
	issued     domain.ArvanCloudAccountCertificateOrder

	orders    []domain.ArvanCloudAccountCertificateOrder
	ordersErr error

	getErr            error
	gotKeepPrivateKey *bool
	revokedOrderID    string
	revokeErr         error
	reissuedOrderID   string
	reissueErr        error
	installedOrderID  string
	installErr        error
	installResult     domain.ArvanCloudCertificateInstallResult
}

func (p *fakeArvanCloudAccountSSLProvider) ListArvanCloudCertificateProducts(context.Context, domain.ProviderCredentials) ([]domain.ArvanCloudCertificateProduct, error) {
	if p.productsErr != nil {
		return nil, p.productsErr
	}
	return p.products, nil
}

func (p *fakeArvanCloudAccountSSLProvider) IssueArvanCloudAccountCertificate(_ context.Context, _ domain.ProviderCredentials, req domain.ArvanCloudCertificateOrderIssueRequest) (*domain.ArvanCloudAccountCertificateOrder, error) {
	p.issueCalls++
	if p.issueErr != nil {
		return nil, p.issueErr
	}
	order := p.issued
	if order.DomainNames == nil {
		order.DomainNames = requestDomainNamesForTest(req)
	}
	return &order, nil
}

func (p *fakeArvanCloudAccountSSLProvider) ListArvanCloudAccountCertificateOrders(context.Context, domain.ProviderCredentials) ([]domain.ArvanCloudAccountCertificateOrder, error) {
	if p.ordersErr != nil {
		return nil, p.ordersErr
	}
	return p.orders, nil
}

func (p *fakeArvanCloudAccountSSLProvider) GetArvanCloudAccountCertificateOrder(_ context.Context, _ domain.ProviderCredentials, orderID string, keepPrivateKey *bool) (*domain.ArvanCloudAccountCertificateOrder, error) {
	p.gotKeepPrivateKey = keepPrivateKey
	if p.getErr != nil {
		return nil, p.getErr
	}
	for i := range p.orders {
		if p.orders[i].ID == orderID {
			return &p.orders[i], nil
		}
	}
	return nil, domain.ErrNotFound
}

func (p *fakeArvanCloudAccountSSLProvider) RevokeArvanCloudAccountCertificate(_ context.Context, _ domain.ProviderCredentials, orderID string) (*domain.ArvanCloudAccountCertificateOrder, error) {
	p.revokedOrderID = orderID
	if p.revokeErr != nil {
		return nil, p.revokeErr
	}
	return &domain.ArvanCloudAccountCertificateOrder{ID: orderID, IsRevoked: true}, nil
}

func (p *fakeArvanCloudAccountSSLProvider) ReissueArvanCloudAccountCertificate(_ context.Context, _ domain.ProviderCredentials, orderID string) (*domain.ArvanCloudAccountCertificateOrder, error) {
	p.reissuedOrderID = orderID
	if p.reissueErr != nil {
		return nil, p.reissueErr
	}
	return &domain.ArvanCloudAccountCertificateOrder{ID: orderID, Status: domain.ArvanCloudAccountCertificateOrderStatus("pending")}, nil
}

func (p *fakeArvanCloudAccountSSLProvider) InstallArvanCloudAccountCertificate(_ context.Context, _ domain.ProviderCredentials, orderID string) (*domain.ArvanCloudCertificateInstallResult, error) {
	p.installedOrderID = orderID
	if p.installErr != nil {
		return nil, p.installErr
	}
	result := p.installResult
	return &result, nil
}

func requestDomainNamesForTest(req domain.ArvanCloudCertificateOrderIssueRequest) []string {
	var names []string
	for _, d := range req.Domains {
		names = append(names, d.DomainNames...)
	}
	return names
}

func TestListArvanCloudCertificateProductsUseCase(t *testing.T) {
	provider := &fakeArvanCloudAccountSSLProvider{products: []domain.ArvanCloudCertificateProduct{{ID: "prod-1", Name: "Certum Basic", Price: 9.99}}}
	uc := app.NewListArvanCloudCertificateProducts(&inlineQueue{}, provider)

	products, err := uc.Execute(context.Background(), app.ListArvanCloudCertificateProductsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(products) != 1 || products[0].ID != "prod-1" {
		t.Errorf("products = %+v, want the one fake product", products)
	}
}

func TestListArvanCloudCertificateProductsUseCaseRejectsMissingCredentials(t *testing.T) {
	uc := app.NewListArvanCloudCertificateProducts(&inlineQueue{}, &fakeArvanCloudAccountSSLProvider{})
	if _, err := uc.Execute(context.Background(), app.ListArvanCloudCertificateProductsInput{}); err == nil {
		t.Fatal("Execute() error = nil, want a validation error for missing credentials")
	}
}

func TestListArvanCloudAccountCertificateOrdersUseCase(t *testing.T) {
	provider := &fakeArvanCloudAccountSSLProvider{orders: []domain.ArvanCloudAccountCertificateOrder{{ID: "order-1", Status: domain.ArvanCloudAccountCertificateOrderStatusValid}}}
	uc := app.NewListArvanCloudAccountCertificateOrders(&inlineQueue{}, provider)

	orders, err := uc.Execute(context.Background(), app.ListArvanCloudAccountCertificateOrdersInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(orders) != 1 || orders[0].ID != "order-1" {
		t.Errorf("orders = %+v, want the one fake order", orders)
	}
}

func TestGetArvanCloudAccountCertificateOrderUseCase(t *testing.T) {
	provider := &fakeArvanCloudAccountSSLProvider{orders: []domain.ArvanCloudAccountCertificateOrder{{ID: "order-1", Status: domain.ArvanCloudAccountCertificateOrderStatusValid}}}
	uc := app.NewGetArvanCloudAccountCertificateOrder(&inlineQueue{}, provider)

	keep := false
	found, err := uc.Execute(context.Background(), app.GetArvanCloudAccountCertificateOrderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, OrderID: "order-1", KeepPrivateKey: &keep,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if found.ID != "order-1" {
		t.Errorf("found.ID = %q, want order-1", found.ID)
	}
	if provider.gotKeepPrivateKey == nil || *provider.gotKeepPrivateKey != false {
		t.Errorf("gotKeepPrivateKey = %v, want a pointer to false forwarded through", provider.gotKeepPrivateKey)
	}
}

func TestGetArvanCloudAccountCertificateOrderUseCaseRejectsMissingOrderID(t *testing.T) {
	uc := app.NewGetArvanCloudAccountCertificateOrder(&inlineQueue{}, &fakeArvanCloudAccountSSLProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudAccountCertificateOrderInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput for the missing order_id", err)
	}
}

func TestRevokeArvanCloudAccountCertificateUseCase(t *testing.T) {
	provider := &fakeArvanCloudAccountSSLProvider{}
	uc := app.NewRevokeArvanCloudAccountCertificate(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.RevokeArvanCloudAccountCertificateInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, OrderID: "order-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !updated.IsRevoked || provider.revokedOrderID != "order-1" {
		t.Errorf("updated = %+v, revokedOrderID = %q, want order-1 revoked", updated, provider.revokedOrderID)
	}
}

func TestReissueArvanCloudAccountCertificateUseCase(t *testing.T) {
	provider := &fakeArvanCloudAccountSSLProvider{}
	uc := app.NewReissueArvanCloudAccountCertificate(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.ReissueArvanCloudAccountCertificateInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, OrderID: "order-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.reissuedOrderID != "order-1" || updated.Status != domain.ArvanCloudAccountCertificateOrderStatus("pending") {
		t.Errorf("updated = %+v, reissuedOrderID = %q, want order-1 reissued", updated, provider.reissuedOrderID)
	}
}

func TestInstallArvanCloudAccountCertificateUseCase(t *testing.T) {
	provider := &fakeArvanCloudAccountSSLProvider{
		installResult: domain.ArvanCloudCertificateInstallResult{Success: true, Message: "installed"},
	}
	uc := app.NewInstallArvanCloudAccountCertificate(&inlineQueue{}, provider)

	result, err := uc.Execute(context.Background(), app.InstallArvanCloudAccountCertificateInput{Credentials: domain.ProviderCredentials{APIKey: "k"}, OrderID: "order-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success || provider.installedOrderID != "order-1" {
		t.Errorf("result = %+v, installedOrderID = %q, want order-1 installed successfully", result, provider.installedOrderID)
	}
}
