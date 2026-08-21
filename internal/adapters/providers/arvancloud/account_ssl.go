package arvancloud

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Account-level Certum certificate ordering (issue #74/AC14), wired to the
// real CDN API: a paid, CA-issued certificate product purchased against the
// account and installed onto one or more domains. Base paths are confirmed
// against docs/api-specs/arvancloud-cdn-4.0.yml's "SSL/TLS" tag, relative to
// Client.baseURL directly — i.e. https://napi.arvancloud.ir/cdn/4.0/certificate-products
// and https://napi.arvancloud.ir/cdn/4.0/certificates/... — NOT under
// domainPath (defined in domain.go), unlike every other capability this
// adapter implements: this is an account-wide purchase, not a
// /domains/{domain}/... operation.
//
// Deliberately NOT the domain-scoped SSL/TLS settings and managed/uploaded
// certificate workflow (issue #73, ssl.go) — see
// domain/arvancloud_account_ssl.go's package comment.
const (
	certificateProductsPath = "certificate-products"
	certificatesIssuePath   = "certificates/issue"
	certificateOrdersPath   = "certificates/orders"
)

func certificateOrderPath(orderID string) string { return certificateOrdersPath + "/" + orderID }
func certificateOrderRevokePath(orderID string) string {
	return certificateOrderPath(orderID) + "/revoke"
}
func certificateOrderReissuePath(orderID string) string {
	return certificateOrderPath(orderID) + "/reissue"
}
func certificateOrderInstallPath(orderID string) string {
	return certificateOrderPath(orderID) + "/install"
}

// --- Wire types ------------------------------------------------------------

// certificateProductWire mirrors the CertificateProduct schema. Decode-only.
type certificateProductWire struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	IsMulti     bool    `json:"is_multi"`
	HasWildcard bool    `json:"has_wildcard"`
	Limit       int     `json:"limit"`
	Price       float64 `json:"price"`
}

func toCertificateProductDomain(w certificateProductWire) domain.ArvanCloudCertificateProduct {
	return domain.ArvanCloudCertificateProduct{
		ID:          w.ID,
		Name:        w.Name,
		IsMulti:     w.IsMulti,
		HasWildcard: w.HasWildcard,
		Limit:       w.Limit,
		Price:       w.Price,
	}
}

// certificateIssueDomainWire mirrors one entry of
// CertificateOrderIssueRequest.domains. Encode-only.
type certificateIssueDomainWire struct {
	Domain      string   `json:"domain"`
	DomainNames []string `json:"domain_names"`
}

// certificateOrderIssueRequestWire mirrors CertificateOrderIssueRequest.
// Encode-only.
type certificateOrderIssueRequestWire struct {
	Domains        []certificateIssueDomainWire `json:"domains"`
	ProductID      string                       `json:"product_id"`
	CommonName     string                       `json:"common_name,omitempty"`
	PrivateKeySize int                          `json:"private_key_size,omitempty"`
}

func toCertificateOrderIssueRequestWire(req domain.ArvanCloudCertificateOrderIssueRequest) certificateOrderIssueRequestWire {
	domains := make([]certificateIssueDomainWire, len(req.Domains))
	for i, d := range req.Domains {
		domains[i] = certificateIssueDomainWire{Domain: d.DomainID, DomainNames: d.DomainNames}
	}
	return certificateOrderIssueRequestWire{
		Domains:        domains,
		ProductID:      req.ProductID,
		CommonName:     req.CommonName,
		PrivateKeySize: req.PrivateKeySize,
	}
}

// accountCertificateOrderWire mirrors the AccountCertificateOrder schema.
// Decode-only. See domain.ArvanCloudAccountCertificateOrder's own doc
// comment for why OrderID/Errors/Product are NOT decoded the same way as
// issue #73's certificateOrderWire.
type accountCertificateOrderWire struct {
	ID          string         `json:"id"`
	OrderID     string         `json:"order_id"`
	Status      string         `json:"status"`
	DomainNames []string       `json:"domain_names"`
	Product     string         `json:"product"`
	Errors      map[string]any `json:"errors"`
	ExpiryDate  string         `json:"expiry_date"`
	IsRevoked   bool           `json:"is_revoked"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

func toAccountCertificateOrderDomain(w accountCertificateOrderWire) domain.ArvanCloudAccountCertificateOrder {
	return domain.ArvanCloudAccountCertificateOrder{
		ID:          w.ID,
		OrderID:     w.OrderID,
		Status:      domain.ArvanCloudAccountCertificateOrderStatus(w.Status),
		DomainNames: w.DomainNames,
		Product:     w.Product,
		Errors:      w.Errors,
		ExpiryDate:  w.ExpiryDate,
		IsRevoked:   w.IsRevoked,
		CreatedAt:   w.CreatedAt,
		UpdatedAt:   w.UpdatedAt,
	}
}

// certificateInstallWarningWire mirrors one entry of the install endpoint's
// data.warnings array. Decode-only.
type certificateInstallWarningWire struct {
	Domain string `json:"domain"`
	Reason string `json:"reason"`
}

// certificateInstallDataWire mirrors the install endpoint's "data" object —
// confirmed against the spec as a bespoke shape, distinct from every other
// operation's envelope.
type certificateInstallDataWire struct {
	Warnings []certificateInstallWarningWire `json:"warnings"`
}

// certificateInstallResponseWire mirrors the install endpoint's full
// response body: success/message are SIBLINGS of data, not nested under it
// — the one response in this file client.doJSON's generic {"data": ...}
// envelope unwrapping does not fit, so this method decodes the raw envelope
// itself instead (see InstallArvanCloudAccountCertificate).
type certificateInstallResponseWire struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Data    certificateInstallDataWire `json:"data"`
}

func toCertificateInstallResultDomain(w certificateInstallResponseWire) domain.ArvanCloudCertificateInstallResult {
	warnings := make([]domain.ArvanCloudCertificateInstallWarning, len(w.Data.Warnings))
	for i, wa := range w.Data.Warnings {
		warnings[i] = domain.ArvanCloudCertificateInstallWarning{Domain: wa.Domain, Reason: wa.Reason}
	}
	return domain.ArvanCloudCertificateInstallResult{Success: w.Success, Message: w.Message, Warnings: warnings}
}

// --- Certificate products ---------------------------------------------

// ListArvanCloudCertificateProducts returns every purchasable Certum
// certificate product, unfiltered.
func (p *Provider) ListArvanCloudCertificateProducts(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudCertificateProduct, error) {
	var items []certificateProductWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, certificateProductsPath, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud certificate products: %w", err)
	}
	products := make([]domain.ArvanCloudCertificateProduct, len(items))
	for i := range items {
		products[i] = toCertificateProductDomain(items[i])
	}
	return products, nil
}

// --- Account certificate orders -----------------------------------------

// IssueArvanCloudAccountCertificate purchases and requests issuance of a
// Certum certificate and returns the order it started. See
// ports.ArvanCloudProvider's own doc comment: this is a long operation, and
// the caller (app.IssueArvanCloudAccountCertificate) is responsible for not
// calling this a second time while an order is already in flight.
func (p *Provider) IssueArvanCloudAccountCertificate(ctx context.Context, creds domain.ProviderCredentials, req domain.ArvanCloudCertificateOrderIssueRequest) (*domain.ArvanCloudAccountCertificateOrder, error) {
	body := toCertificateOrderIssueRequestWire(req)
	var wire accountCertificateOrderWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, certificatesIssuePath, body, &wire); err != nil {
		return nil, fmt.Errorf("issuing arvancloud account certificate: %w", err)
	}
	order := toAccountCertificateOrderDomain(wire)
	return &order, nil
}

// ListArvanCloudAccountCertificateOrders returns the account's full Certum
// order history, unfiltered — there is no per-domain filter on this
// endpoint (see ports.ArvanCloudProvider's own doc comment).
func (p *Provider) ListArvanCloudAccountCertificateOrders(ctx context.Context, creds domain.ProviderCredentials) ([]domain.ArvanCloudAccountCertificateOrder, error) {
	var items []accountCertificateOrderWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, certificateOrdersPath, nil, &items); err != nil {
		return nil, fmt.Errorf("listing arvancloud account certificate orders: %w", err)
	}
	orders := make([]domain.ArvanCloudAccountCertificateOrder, len(items))
	for i := range items {
		orders[i] = toAccountCertificateOrderDomain(items[i])
	}
	return orders, nil
}

// GetArvanCloudAccountCertificateOrder returns one order by id. See
// ports.ArvanCloudProvider's own doc comment for keepPrivateKey.
func (p *Provider) GetArvanCloudAccountCertificateOrder(ctx context.Context, creds domain.ProviderCredentials, orderID string, keepPrivateKey *bool) (*domain.ArvanCloudAccountCertificateOrder, error) {
	path := certificateOrderPath(orderID)
	if keepPrivateKey != nil {
		values := url.Values{}
		values.Set("keep_private_key", fmt.Sprintf("%t", *keepPrivateKey))
		path += "?" + values.Encode()
	}
	var wire accountCertificateOrderWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, path, nil, &wire); err != nil {
		return nil, fmt.Errorf("getting arvancloud account certificate order %q: %w", orderID, err)
	}
	order := toAccountCertificateOrderDomain(wire)
	return &order, nil
}

// RevokeArvanCloudAccountCertificate revokes an order's certificate and
// returns it as updated.
func (p *Provider) RevokeArvanCloudAccountCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.ArvanCloudAccountCertificateOrder, error) {
	var wire accountCertificateOrderWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, certificateOrderRevokePath(orderID), nil, &wire); err != nil {
		return nil, fmt.Errorf("revoking arvancloud account certificate order %q: %w", orderID, err)
	}
	order := toAccountCertificateOrderDomain(wire)
	return &order, nil
}

// ReissueArvanCloudAccountCertificate reissues an order's certificate and
// returns it as updated.
func (p *Provider) ReissueArvanCloudAccountCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.ArvanCloudAccountCertificateOrder, error) {
	var wire accountCertificateOrderWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, certificateOrderReissuePath(orderID), nil, &wire); err != nil {
		return nil, fmt.Errorf("reissuing arvancloud account certificate order %q: %w", orderID, err)
	}
	order := toAccountCertificateOrderDomain(wire)
	return &order, nil
}

// InstallArvanCloudAccountCertificate installs an issued certificate onto
// edge servers. Unlike every other method in this file, the response
// carries success/message as siblings of data (see
// certificateInstallResponseWire's own doc comment), so this bypasses
// client.doJSON's generic envelope unwrapping and decodes the raw response
// itself via doRawJSON.
func (p *Provider) InstallArvanCloudAccountCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.ArvanCloudCertificateInstallResult, error) {
	var wire certificateInstallResponseWire
	if err := p.client.doRawJSON(ctx, creds, http.MethodPost, certificateOrderInstallPath(orderID), nil, &wire); err != nil {
		return nil, fmt.Errorf("installing arvancloud account certificate order %q: %w", orderID, err)
	}
	result := toCertificateInstallResultDomain(wire)
	return &result, nil
}
