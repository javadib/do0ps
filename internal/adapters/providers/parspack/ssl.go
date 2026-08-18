package parspack

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// sslEnvelope is the JSON envelope every Parspack SSL API response uses:
// {"message": ..., "code": ..., "data": ..., "status": true/false}, with an
// "errors" object added on 422 validation failures. Confirmed against
// docs/api-specs/ssl-api.openapi (AGENTS.md 4.5) — distinct from the
// cloud-server surface's bare "vm"/"vms" wrapper.
type sslEnvelope struct {
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Status  bool            `json:"status"`
}

// callSSL sends a request to the SSL surface and returns the decoded
// envelope's Data field for the caller to unmarshal into the specific shape
// it expects.
func (c *Client) callSSL(ctx context.Context, creds domain.ProviderCredentials, method, path string, body any) (json.RawMessage, error) {
	var env sslEnvelope
	if err := c.doJSONSSL(ctx, creds, method, path, body, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// ListSSLProducts returns every SSL product available to order.
func (c *Client) ListSSLProducts(ctx context.Context, creds domain.ProviderCredentials) ([]domain.SSLProduct, error) {
	data, err := c.callSSL(ctx, creds, "GET", "api/public/v1/products", nil)
	if err != nil {
		return nil, fmt.Errorf("listing SSL products: %w", err)
	}

	var wire []productWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decoding SSL products: %w", err)
	}

	products := make([]domain.SSLProduct, len(wire))
	for i := range wire {
		products[i] = wire[i].toDomain()
	}
	return products, nil
}

// addOrderRequest mirrors the confirmed POST /order/addOrder body.
type addOrderRequest struct {
	ProductSlug  string   `json:"product_slug"`
	Domain       string   `json:"domain"`
	BillingCycle string   `json:"billing_cycle"`
	WWW          bool     `json:"www"`
	PromoCode    string   `json:"promocode,omitempty"`
	SANs         []sanReq `json:"sans,omitempty"`
}

type sanReq struct {
	Name string `json:"name"`
	IsIP bool   `json:"is_ip"`
	WWW  int    `json:"www"` // 0 or 1, per the confirmed schema
}

type addOrderResponse struct {
	OrderID       string `json:"order_id"`
	InvoiceID     int    `json:"invoice_id"`
	InvoiceStatus string `json:"invoice_status"`
}

// CreateSSLOrder places a new SSL order. The order is unpaid until the
// returned invoice is settled — ProcessSSLOrder will fail until then.
func (c *Client) CreateSSLOrder(ctx context.Context, creds domain.ProviderCredentials, spec domain.SSLOrderSpec) (*domain.SSLOrder, error) {
	req := addOrderRequest{
		ProductSlug:  spec.ProductSlug,
		Domain:       spec.Domain,
		BillingCycle: spec.BillingCycle,
		WWW:          spec.WWW,
		PromoCode:    spec.PromoCode,
	}
	for _, san := range spec.SANs {
		www := 0
		if san.WWW {
			www = 1
		}
		req.SANs = append(req.SANs, sanReq{Name: san.Name, IsIP: san.IsIP, WWW: www})
	}

	data, err := c.callSSL(ctx, creds, "POST", "api/public/v1/order/addOrder", req)
	if err != nil {
		return nil, fmt.Errorf("creating SSL order for %q: %w", spec.Domain, err)
	}

	var wire addOrderResponse
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decoding created SSL order: %w", err)
	}
	return &domain.SSLOrder{
		ID:            wire.OrderID,
		InvoiceID:     fmt.Sprintf("%d", wire.InvoiceID),
		InvoiceStatus: wire.InvoiceStatus,
	}, nil
}

// processOrderRequest mirrors the confirmed POST /order/{id}/process body.
type processOrderRequest struct {
	CSR                 string `json:"csr"`
	FirstName           string `json:"first_name"`
	LastName            string `json:"last_name"`
	Country             string `json:"country"`
	City                string `json:"city"`
	Address             string `json:"address"`
	Phone               string `json:"phone"`
	Email               string `json:"email"`
	PostalCode          string `json:"postal_code,omitempty"`
	JurisdictionCountry string `json:"joisocn,omitempty"`
	BusinessCategory    string `json:"business_category,omitempty"`
	RegistrationNumber  string `json:"registration_number,omitempty"`
}

// ProcessSSLOrder submits the CSR and contact details for a paid order and
// returns the domain-ownership challenges to complete next.
func (c *Client) ProcessSSLOrder(ctx context.Context, creds domain.ProviderCredentials, orderID, csr string, contact domain.SSLContact) (*domain.SSLChallengeSet, error) {
	req := processOrderRequest{
		CSR:                 csr,
		FirstName:           contact.FirstName,
		LastName:            contact.LastName,
		Country:             contact.Country,
		City:                contact.City,
		Address:             contact.Address,
		Phone:               contact.Phone,
		Email:               contact.Email,
		PostalCode:          contact.PostalCode,
		JurisdictionCountry: contact.JurisdictionCountry,
		BusinessCategory:    contact.BusinessCategory,
		RegistrationNumber:  contact.RegistrationNumber,
	}

	data, err := c.callSSL(ctx, creds, "POST", "api/public/v1/order/"+orderID+"/process", req)
	if err != nil {
		return nil, fmt.Errorf("processing SSL order %s: %w", orderID, err)
	}
	set, err := decodeChallengeSet(data)
	if err != nil {
		return nil, fmt.Errorf("decoding challenges for SSL order %s: %w", orderID, err)
	}
	return set, nil
}

// GetSSLChallenge re-shows the challenges of an already-processed order.
func (c *Client) GetSSLChallenge(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.SSLChallengeSet, error) {
	data, err := c.callSSL(ctx, creds, "GET", "api/public/v1/order/"+orderID+"/challenge", nil)
	if err != nil {
		return nil, fmt.Errorf("getting challenges for SSL order %s: %w", orderID, err)
	}
	set, err := decodeChallengeSet(data)
	if err != nil {
		return nil, fmt.Errorf("decoding challenges for SSL order %s: %w", orderID, err)
	}
	return set, nil
}

type reloadChallengeRequest struct {
	Method      string `json:"method,omitempty"`
	EmailPrefix string `json:"email_prefix,omitempty"`
}

// ReloadSSLChallenge switches the verification method, invalidating any
// previously shown challenge tokens.
func (c *Client) ReloadSSLChallenge(ctx context.Context, creds domain.ProviderCredentials, orderID, method, emailPrefix string) (*domain.SSLChallengeSet, error) {
	req := reloadChallengeRequest{Method: method, EmailPrefix: emailPrefix}

	data, err := c.callSSL(ctx, creds, "POST", "api/public/v1/order/"+orderID+"/reloadChallenge", req)
	if err != nil {
		return nil, fmt.Errorf("reloading challenge for SSL order %s: %w", orderID, err)
	}
	set, err := decodeChallengeSet(data)
	if err != nil {
		return nil, fmt.Errorf("decoding reloaded challenges for SSL order %s: %w", orderID, err)
	}
	return set, nil
}

type verifyChallengeRequest struct {
	Method string `json:"method"`
}

type verifyChallengeResponse struct {
	Step              string `json:"step"`
	CertificateReady  bool   `json:"certificate_ready"`
	Certificate       string `json:"certificate"`
	CABundle          string `json:"ca_bundle"`
	RequiresDocuments bool   `json:"requires_documents"`
	ProductGroup      string `json:"product_group"`
}

// VerifySSLChallenge checks the completed challenge for method and, on
// success, returns the certificate if it is ready immediately. A 202
// response ("verified, certificate pending") is not an error: it still
// decodes into this same shape with CertificateReady false.
func (c *Client) VerifySSLChallenge(ctx context.Context, creds domain.ProviderCredentials, orderID, method string) (*domain.SSLVerifyResult, error) {
	var env sslEnvelope
	if err := c.doJSONSSL(ctx, creds, "POST", "api/public/v1/order/"+orderID+"/verifyChallenge", verifyChallengeRequest{Method: method}, &env); err != nil {
		return nil, fmt.Errorf("verifying challenge for SSL order %s: %w", orderID, err)
	}

	var wire verifyChallengeResponse
	if err := json.Unmarshal(env.Data, &wire); err != nil {
		return nil, fmt.Errorf("decoding verify result for SSL order %s: %w", orderID, err)
	}
	return &domain.SSLVerifyResult{
		Step:              wire.Step,
		CertificateReady:  wire.CertificateReady,
		Certificate:       wire.Certificate,
		CABundle:          wire.CABundle,
		RequiresDocuments: wire.RequiresDocuments,
		ProductGroup:      wire.ProductGroup,
		Message:           env.Message,
	}, nil
}

type certificateResponse struct {
	CertificateReady bool   `json:"certificate_ready"`
	Certificate      string `json:"certificate"`
	CABundle         string `json:"ca_bundle"`
}

// GetSSLCertificate downloads the issued certificate, or reports that it is
// not ready yet (a 202 response, not an error).
func (c *Client) GetSSLCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.SSLCertificate, error) {
	data, err := c.callSSL(ctx, creds, "GET", "api/public/v1/order/"+orderID+"/certificate", nil)
	if err != nil {
		return nil, fmt.Errorf("getting certificate for SSL order %s: %w", orderID, err)
	}

	var wire certificateResponse
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decoding certificate for SSL order %s: %w", orderID, err)
	}
	return &domain.SSLCertificate{Ready: wire.CertificateReady, Certificate: wire.Certificate, CABundle: wire.CABundle}, nil
}

type reissueRequest struct {
	CSR string `json:"csr"`
}

// ReissueSSLCertificate requests a new certificate for an already-issued
// order using a new CSR.
func (c *Client) ReissueSSLCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID, csr string) (*domain.SSLCertificate, error) {
	data, err := c.callSSL(ctx, creds, "POST", "api/public/v1/order/"+orderID+"/reissue", reissueRequest{CSR: csr})
	if err != nil {
		return nil, fmt.Errorf("reissuing certificate for SSL order %s: %w", orderID, err)
	}

	var wire certificateResponse
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decoding reissued certificate for SSL order %s: %w", orderID, err)
	}
	return &domain.SSLCertificate{Ready: wire.CertificateReady, Certificate: wire.Certificate, CABundle: wire.CABundle}, nil
}

// The wire types below mirror docs/api-specs/ssl-api.openapi exactly (field
// names and JSON tags), translated into internal/core/domain types at the
// adapter boundary.

type priceWire struct {
	Cycle    string  `json:"cycle"`
	Price    float64 `json:"price"`
	SetupFee float64 `json:"setup_fee"`
	Currency string  `json:"currency"`
}

type productWire struct {
	Title        string               `json:"title"`
	Slug         string               `json:"slug"`
	Description  string               `json:"description"`
	ProductGroup string               `json:"product_group"`
	Wildcard     int                  `json:"wildcard"`
	MultiDomain  int                  `json:"multi_domain"`
	Prices       map[string]priceWire `json:"prices"`
	IsAvailable  bool                 `json:"is_available"`
}

func (p productWire) toDomain() domain.SSLProduct {
	prices := make(map[string]domain.SSLPrice, len(p.Prices))
	for cycle, price := range p.Prices {
		prices[cycle] = domain.SSLPrice{Cycle: price.Cycle, Price: price.Price, SetupFee: price.SetupFee, Currency: price.Currency}
	}
	return domain.SSLProduct{
		Slug:         p.Slug,
		Title:        p.Title,
		Description:  p.Description,
		ProductGroup: p.ProductGroup,
		Wildcard:     p.Wildcard != 0,
		MultiDomain:  p.MultiDomain != 0,
		Available:    p.IsAvailable,
		Prices:       prices,
	}
}

type challengeWire struct {
	ID        int    `json:"id"`
	OrderID   string `json:"order_id"`
	Domain    string `json:"domain"`
	Challenge string `json:"challenge"`
	Verify    int    `json:"verify"`
	Type      string `json:"type"`
	FileName  string `json:"file_name,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
}

type challengeMethodWire struct {
	Method     string          `json:"method"`
	Name       string          `json:"name"`
	Challenges []challengeWire `json:"challenges"`
}

type allowedMethodWire struct {
	Method   string   `json:"method"`
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes,omitempty"`
}

// decodeChallengeSet decodes the "data" object shared by process, challenge
// and reloadChallenge: a set of keys dynamically named after verification
// methods (DNS_TXT, FILE, ...) sitting alongside two fixed keys,
// allowed_challenge_methods and maximum_verification_opportunity. The fixed
// keys are pulled out first; everything else is treated as a challenge
// method entry.
func decodeChallengeSet(data json.RawMessage) (*domain.SSLChallengeSet, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	set := &domain.SSLChallengeSet{Methods: make(map[string]domain.SSLChallengeMethod)}

	if allowedRaw, ok := raw["allowed_challenge_methods"]; ok {
		var allowed []allowedMethodWire
		if err := json.Unmarshal(allowedRaw, &allowed); err != nil {
			return nil, fmt.Errorf("decoding allowed_challenge_methods: %w", err)
		}
		for _, a := range allowed {
			set.AllowedMethods = append(set.AllowedMethods, domain.SSLAllowedMethod{Method: a.Method, Name: a.Name, Prefixes: a.Prefixes})
		}
		delete(raw, "allowed_challenge_methods")
	}

	if deadlineRaw, ok := raw["maximum_verification_opportunity"]; ok {
		var deadline string
		if err := json.Unmarshal(deadlineRaw, &deadline); err == nil {
			set.Deadline = deadline
		}
		delete(raw, "maximum_verification_opportunity")
	}

	for key, methodRaw := range raw {
		var m challengeMethodWire
		if err := json.Unmarshal(methodRaw, &m); err != nil {
			return nil, fmt.Errorf("decoding challenge method %q: %w", key, err)
		}
		challenges := make([]domain.SSLChallenge, len(m.Challenges))
		for i, c := range m.Challenges {
			challenges[i] = domain.SSLChallenge{
				ID:       c.ID,
				OrderID:  c.OrderID,
				Domain:   c.Domain,
				Token:    c.Challenge,
				Verified: c.Verify != 0,
				Type:     c.Type,
				FileName: c.FileName,
				FilePath: c.FilePath,
			}
		}
		set.Methods[key] = domain.SSLChallengeMethod{Method: m.Method, Name: m.Name, Challenges: challenges}
	}

	return set, nil
}
