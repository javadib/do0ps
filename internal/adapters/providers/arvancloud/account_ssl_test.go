package arvancloud

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// TestListArvanCloudCertificateProducts pins the request shape and response
// parsing of GET /certificate-products.
func TestListArvanCloudCertificateProducts(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"prod-1","name":"Certum Basic","is_multi":false,"has_wildcard":false,"limit":1,"price":9.99},
			{"id":"prod-2","name":"Certum Wildcard","is_multi":true,"has_wildcard":true,"limit":10,"price":49.99}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	products, err := provider.ListArvanCloudCertificateProducts(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudCertificateProducts() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/certificate-products" {
		t.Fatalf("request = %+v, want a single GET /certificate-products", records)
	}
	if len(products) != 2 || products[0].ID != "prod-1" || !products[1].HasWildcard || products[1].Price != 49.99 {
		t.Errorf("products = %+v, want the two parsed entries", products)
	}
}

// TestIssueArvanCloudAccountCertificate pins the request body and response
// parsing of POST /certificates/issue.
func TestIssueArvanCloudAccountCertificate(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"order-1","order_id":"ord-100","status":"pending","domain_names":["example.com"],"product":"prod-1"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	req := domain.ArvanCloudCertificateOrderIssueRequest{
		Domains: []domain.ArvanCloudCertificateIssueDomain{
			{DomainID: "domain-uuid-1", DomainNames: []string{"example.com", "www.example.com"}},
		},
		ProductID:      "prod-1",
		CommonName:     "example.com",
		PrivateKeySize: 2048,
	}
	order, err := provider.IssueArvanCloudAccountCertificate(context.Background(), creds(), req)
	if err != nil {
		t.Fatalf("IssueArvanCloudAccountCertificate() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/certificates/issue" {
		t.Fatalf("request = %+v, want a single POST /certificates/issue", records)
	}
	body := string(records[0].body)
	if !strings.Contains(body, `"domain":"domain-uuid-1"`) || !strings.Contains(body, `"product_id":"prod-1"`) {
		t.Errorf("request body = %s, want the domain id and product_id sent", body)
	}
	if !strings.Contains(body, `"private_key_size":2048`) {
		t.Errorf("request body = %s, want private_key_size sent", body)
	}
	if order.ID != "order-1" || order.OrderID != "ord-100" || order.Status != domain.ArvanCloudAccountCertificateOrderStatus("pending") {
		t.Errorf("order = %+v, want the parsed non-terminal order", order)
	}
}

// TestListArvanCloudAccountCertificateOrders pins the request shape and
// response parsing of GET /certificates/orders.
func TestListArvanCloudAccountCertificateOrders(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"order-1","order_id":"ord-1","status":"killed","domain_names":["example.com"],"errors":{"reason":"validation failed"}},
			{"id":"order-2","order_id":"ord-2","status":"valid","domain_names":["example.com"]}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	orders, err := provider.ListArvanCloudAccountCertificateOrders(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudAccountCertificateOrders() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/certificates/orders" {
		t.Fatalf("request = %+v, want a single GET /certificates/orders", records)
	}
	if len(orders) != 2 || orders[0].Status != domain.ArvanCloudAccountCertificateOrderStatusKilled {
		t.Errorf("orders = %+v, want the two parsed entries", orders)
	}
	if orders[0].Errors["reason"] != "validation failed" {
		t.Errorf("orders[0].Errors = %+v, want the generic error map preserved", orders[0].Errors)
	}
	if orders[1].Status != domain.ArvanCloudAccountCertificateOrderStatusValid {
		t.Errorf("orders[1].Status = %q, want valid", orders[1].Status)
	}
}

// TestGetArvanCloudAccountCertificateOrder pins the request shape (no
// keep_private_key query param when left nil) and response parsing of GET
// /certificates/orders/{id}.
func TestGetArvanCloudAccountCertificateOrder(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"order-1","order_id":"ord-1","status":"valid","domain_names":["example.com"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	order, err := provider.GetArvanCloudAccountCertificateOrder(context.Background(), creds(), "order-1", nil)
	if err != nil {
		t.Fatalf("GetArvanCloudAccountCertificateOrder() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/certificates/orders/order-1" {
		t.Fatalf("request = %+v, want a single GET /certificates/orders/order-1", records)
	}
	if records[0].query != "" {
		t.Errorf("query = %q, want empty when keepPrivateKey is nil", records[0].query)
	}
	if order.Status != domain.ArvanCloudAccountCertificateOrderStatusValid {
		t.Errorf("order.Status = %q, want valid", order.Status)
	}
}

// TestGetArvanCloudAccountCertificateOrderKeepPrivateKeyFalse pins the
// keep_private_key=false query parameter when explicitly requested.
func TestGetArvanCloudAccountCertificateOrderKeepPrivateKeyFalse(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"order-1","order_id":"ord-1","status":"valid"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	keep := false
	if _, err := provider.GetArvanCloudAccountCertificateOrder(context.Background(), creds(), "order-1", &keep); err != nil {
		t.Fatalf("GetArvanCloudAccountCertificateOrder() error = %v", err)
	}

	if len(records) != 1 || records[0].query != "keep_private_key=false" {
		t.Fatalf("query = %q, want keep_private_key=false", records[0].query)
	}
}

// TestGetArvanCloudAccountCertificateOrderNotFound proves a 404 is reported
// as domain.ErrNotFound, the signal reconciliation relies on.
func TestGetArvanCloudAccountCertificateOrderNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	_, err := provider.GetArvanCloudAccountCertificateOrder(context.Background(), creds(), "missing", nil)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetArvanCloudAccountCertificateOrder() error = %v, want domain.ErrNotFound", err)
	}
}

// TestRevokeArvanCloudAccountCertificate pins the request shape of POST
// /certificates/orders/{id}/revoke.
func TestRevokeArvanCloudAccountCertificate(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"order-1","order_id":"ord-1","status":"valid","is_revoked":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	order, err := provider.RevokeArvanCloudAccountCertificate(context.Background(), creds(), "order-1")
	if err != nil {
		t.Fatalf("RevokeArvanCloudAccountCertificate() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/certificates/orders/order-1/revoke" {
		t.Fatalf("request = %+v, want a single POST /certificates/orders/order-1/revoke", records)
	}
	if !order.IsRevoked {
		t.Errorf("order.IsRevoked = false, want true")
	}
}

// TestReissueArvanCloudAccountCertificate pins the request shape of POST
// /certificates/orders/{id}/reissue.
func TestReissueArvanCloudAccountCertificate(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"order-1","order_id":"ord-1","status":"pending"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	order, err := provider.ReissueArvanCloudAccountCertificate(context.Background(), creds(), "order-1")
	if err != nil {
		t.Fatalf("ReissueArvanCloudAccountCertificate() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/certificates/orders/order-1/reissue" {
		t.Fatalf("request = %+v, want a single POST /certificates/orders/order-1/reissue", records)
	}
	if order.Status != domain.ArvanCloudAccountCertificateOrderStatus("pending") {
		t.Errorf("order.Status = %q, want pending", order.Status)
	}
}

// TestInstallArvanCloudAccountCertificate pins the request shape of POST
// /certificates/orders/{id}/install and the response's bespoke
// success/message/data.warnings shape (siblings, not nested under "data" the
// way every other endpoint in this file works — see
// certificateInstallResponseWire's own doc comment).
func TestInstallArvanCloudAccountCertificate(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"success":true,"message":"installed","data":{"warnings":[{"domain":"example.com","reason":"propagation delay"}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	result, err := provider.InstallArvanCloudAccountCertificate(context.Background(), creds(), "order-1")
	if err != nil {
		t.Fatalf("InstallArvanCloudAccountCertificate() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/certificates/orders/order-1/install" {
		t.Fatalf("request = %+v, want a single POST /certificates/orders/order-1/install", records)
	}
	if !result.Success || result.Message != "installed" {
		t.Errorf("result = %+v, want success=true and the message decoded", result)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Domain != "example.com" || result.Warnings[0].Reason != "propagation delay" {
		t.Errorf("result.Warnings = %+v, want the one parsed warning", result.Warnings)
	}
}
