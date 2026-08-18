package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

var validContact = domain.SSLContact{
	FirstName: "John", LastName: "Doe", Country: "US", City: "NYC", Address: "1 Main St", Phone: "12025551234", Email: "a@b.com",
}

func TestListSSLProductsReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{sslProducts: []domain.SSLProduct{{Slug: "dv-ssl"}, {Slug: "wildcard-ssl"}}}
	uc := app.NewListSSLProducts(&inlineQueue{}, provider)

	products, err := uc.Execute(context.Background(), app.ListSSLProductsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(products) != 2 {
		t.Fatalf("len(products) = %d, want 2", len(products))
	}
}

func TestCreateSSLOrderSuccess(t *testing.T) {
	provider := &fakeProvider{sslOrder: &domain.SSLOrder{ID: "o1", InvoiceID: "1", InvoiceStatus: "unpaid"}}
	uc := app.NewCreateSSLOrder(&inlineQueue{}, provider)

	order, err := uc.Execute(context.Background(), app.CreateSSLOrderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Spec:        domain.SSLOrderSpec{ProductSlug: "dv-ssl", Domain: "example.com", BillingCycle: "annually"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if order.ID != "o1" {
		t.Errorf("ID = %q, want o1", order.ID)
	}
}

func TestCreateSSLOrderRejectsInvalidBillingCycle(t *testing.T) {
	uc := app.NewCreateSSLOrder(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.CreateSSLOrderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Spec:        domain.SSLOrderSpec{ProductSlug: "dv-ssl", Domain: "example.com", BillingCycle: "monthly"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCreateSSLOrderRequiresDomain(t *testing.T) {
	uc := app.NewCreateSSLOrder(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.CreateSSLOrderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		Spec:        domain.SSLOrderSpec{ProductSlug: "dv-ssl", BillingCycle: "annually"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestProcessSSLOrderPassesContactThrough(t *testing.T) {
	set := &domain.SSLChallengeSet{Methods: map[string]domain.SSLChallengeMethod{"DNS_TXT": {Method: "DNS_TXT"}}}
	provider := &fakeProvider{sslChallengeSet: set}
	uc := app.NewProcessSSLOrder(&inlineQueue{}, provider)

	got, err := uc.Execute(context.Background(), app.ProcessSSLOrderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		OrderID:     "o1",
		CSR:         "CSR-DATA",
		Contact:     validContact,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, ok := got.Methods["DNS_TXT"]; !ok {
		t.Errorf("methods = %+v, want DNS_TXT", got.Methods)
	}
	if provider.processedContact != validContact {
		t.Errorf("contact passed to provider = %+v, want %+v", provider.processedContact, validContact)
	}
}

func TestProcessSSLOrderRequiresContactFields(t *testing.T) {
	uc := app.NewProcessSSLOrder(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.ProcessSSLOrderInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		OrderID:     "o1",
		CSR:         "CSR-DATA",
		Contact:     domain.SSLContact{FirstName: "John"}, // missing everything else
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetSSLChallengeReturnsProviderResult(t *testing.T) {
	set := &domain.SSLChallengeSet{Deadline: "2025/11/17"}
	uc := app.NewGetSSLChallenge(&inlineQueue{}, &fakeProvider{sslChallengeSet: set})

	got, err := uc.Execute(context.Background(), app.GetSSLChallengeInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		OrderID:     "o1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Deadline != "2025/11/17" {
		t.Errorf("deadline = %q, want 2025/11/17", got.Deadline)
	}
}

func TestReloadSSLChallengePassesMethodThrough(t *testing.T) {
	provider := &fakeProvider{sslChallengeSet: &domain.SSLChallengeSet{}}
	uc := app.NewReloadSSLChallenge(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.ReloadSSLChallengeInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		OrderID:     "o1",
		Method:      "FILE",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.reloadedMethod != "FILE" {
		t.Errorf("reloadedMethod = %q, want FILE", provider.reloadedMethod)
	}
}

func TestVerifySSLChallengeRequiresMethod(t *testing.T) {
	uc := app.NewVerifySSLChallenge(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.VerifySSLChallengeInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		OrderID:     "o1",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestVerifySSLChallengeReturnsResult(t *testing.T) {
	result := &domain.SSLVerifyResult{Step: "final_verify", CertificateReady: true, Certificate: "CERT"}
	provider := &fakeProvider{sslVerifyResult: result}
	uc := app.NewVerifySSLChallenge(&inlineQueue{}, provider)

	got, err := uc.Execute(context.Background(), app.VerifySSLChallengeInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		OrderID:     "o1",
		Method:      "DNS_TXT",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !got.CertificateReady || got.Certificate != "CERT" {
		t.Errorf("result = %+v", got)
	}
	if provider.verifiedMethod != "DNS_TXT" {
		t.Errorf("verifiedMethod = %q, want DNS_TXT", provider.verifiedMethod)
	}
}

func TestGetSSLCertificateReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{sslCertificate: &domain.SSLCertificate{Ready: false}}
	uc := app.NewGetSSLCertificate(&inlineQueue{}, provider)

	cert, err := uc.Execute(context.Background(), app.GetSSLCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		OrderID:     "o1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if cert.Ready {
		t.Errorf("Ready = true, want false")
	}
}

func TestReissueSSLCertificatePassesCSRThrough(t *testing.T) {
	provider := &fakeProvider{sslCertificate: &domain.SSLCertificate{Ready: true, Certificate: "NEW-CERT"}}
	uc := app.NewReissueSSLCertificate(&inlineQueue{}, provider)

	cert, err := uc.Execute(context.Background(), app.ReissueSSLCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		OrderID:     "o1",
		CSR:         "NEW-CSR",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if cert.Certificate != "NEW-CERT" {
		t.Errorf("Certificate = %q, want NEW-CERT", cert.Certificate)
	}
	if provider.reissuedCSR != "NEW-CSR" {
		t.Errorf("reissuedCSR = %q, want NEW-CSR", provider.reissuedCSR)
	}
}

func TestReissueSSLCertificateRequiresCSR(t *testing.T) {
	uc := app.NewReissueSSLCertificate(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.ReissueSSLCertificateInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		OrderID:     "o1",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
