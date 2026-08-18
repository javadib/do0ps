package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javadib/do0ps/internal/adapters/providers/parspack"
	"github.com/javadib/do0ps/internal/core/domain"
)

func newSSLTestClient(t *testing.T, handler http.HandlerFunc) *parspack.Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := parspack.New(parspack.WithSSLBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestListSSLProductsSuccess(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/public/v1/products" {
			t.Errorf("path = %s, want /api/public/v1/products", got)
		}
		_, _ = w.Write([]byte(`{"message":"success","code":200,"status":true,"data":[
			{"title":"DV SSL","slug":"dv-ssl","product_group":"dv","wildcard":0,"multi_domain":0,"is_available":true,
			 "prices":{"annually":{"cycle":"annually","price":49.99,"setup_fee":0,"currency":"USD"}}}
		]}`))
	})

	products, err := c.ListSSLProducts(context.Background(), creds)
	if err != nil {
		t.Fatalf("ListSSLProducts: %v", err)
	}
	if len(products) != 1 || products[0].Slug != "dv-ssl" {
		t.Fatalf("products = %+v, want one dv-ssl product", products)
	}
	if products[0].Prices["annually"].Price != 49.99 {
		t.Errorf("price = %v, want 49.99", products[0].Prices["annually"].Price)
	}
}

func TestCreateSSLOrderSuccess(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/order/addOrder" {
			t.Errorf("path = %s, want /api/public/v1/order/addOrder", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["domain"] != "example.com" {
			t.Errorf("domain = %v, want example.com", body["domain"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"success","code":200,"status":true,
			"data":{"order_id":"07e0-32ff-f210-b126","invoice_id":2112,"invoice_status":"unpaid"}}`))
	})

	order, err := c.CreateSSLOrder(context.Background(), creds, domain.SSLOrderSpec{
		ProductSlug: "dv-ssl", Domain: "example.com", BillingCycle: "annually",
	})
	if err != nil {
		t.Fatalf("CreateSSLOrder: %v", err)
	}
	if order.ID != "07e0-32ff-f210-b126" || order.InvoiceStatus != "unpaid" {
		t.Errorf("order = %+v, want id 07e0-32ff-f210-b126 and status unpaid", order)
	}
}

func TestCreateSSLOrderValidationError(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The given data was invalid.","code":422,"status":false,
			"errors":{"domain":["The domain field is required."]}}`))
	})

	_, err := c.CreateSSLOrder(context.Background(), creds, domain.SSLOrderSpec{ProductSlug: "dv-ssl", BillingCycle: "annually"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestProcessSSLOrderReturnsChallenges(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/public/v1/order/07e0-32ff-f210-b126/process" {
			t.Errorf("path = %s, want .../process", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["csr"] != "CSR-DATA" {
			t.Errorf("csr = %v, want CSR-DATA", body["csr"])
		}
		_, _ = w.Write([]byte(`{"message":"ok","code":200,"status":true,"data":{
			"DNS_TXT":{"method":"DNS_TXT","name":"DNS_TXT","challenges":[
				{"id":1234,"order_id":"07e0-32ff-f210-b126","domain":"example.com","challenge":"tok-1","verify":0,"type":"DNS_TXT"}
			]},
			"allowed_challenge_methods":[{"method":"DNS_TXT","name":"DNS_TXT"},{"method":"FILE","name":"FILE"}],
			"maximum_verification_opportunity":"2025/11/17"
		}}`))
	})

	set, err := c.ProcessSSLOrder(context.Background(), creds, "07e0-32ff-f210-b126", "CSR-DATA", domain.SSLContact{
		FirstName: "John", LastName: "Doe", Country: "US", City: "NYC", Address: "1 Main St", Phone: "12025551234", Email: "a@b.com",
	})
	if err != nil {
		t.Fatalf("ProcessSSLOrder: %v", err)
	}
	if len(set.Methods) != 1 {
		t.Fatalf("methods = %+v, want exactly 1 (allowed_challenge_methods must not be treated as a method)", set.Methods)
	}
	dnsTXT, ok := set.Methods["DNS_TXT"]
	if !ok || len(dnsTXT.Challenges) != 1 || dnsTXT.Challenges[0].Token != "tok-1" {
		t.Fatalf("DNS_TXT method = %+v", dnsTXT)
	}
	if len(set.AllowedMethods) != 2 {
		t.Errorf("allowed methods = %+v, want 2", set.AllowedMethods)
	}
	if set.Deadline != "2025/11/17" {
		t.Errorf("deadline = %q, want 2025/11/17", set.Deadline)
	}
}

func TestGetSSLChallengeSuccess(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte(`{"message":"ok","code":200,"status":true,"data":{
			"FILE":{"method":"FILE","name":"FILE","challenges":[
				{"id":1,"order_id":"o1","domain":"example.com","challenge":"content","verify":0,"type":"FILE","file_name":"a.txt","file_path":"/.well-known/pki-validation/a.txt"}
			]},
			"allowed_challenge_methods":[{"method":"FILE","name":"FILE"}]
		}}`))
	})

	set, err := c.GetSSLChallenge(context.Background(), creds, "o1")
	if err != nil {
		t.Fatalf("GetSSLChallenge: %v", err)
	}
	file := set.Methods["FILE"].Challenges[0]
	if file.FileName != "a.txt" || file.FilePath != "/.well-known/pki-validation/a.txt" {
		t.Errorf("FILE challenge = %+v", file)
	}
}

func TestReloadSSLChallengeSuccess(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["method"] != "DNS_TXT" {
			t.Errorf("method = %v, want DNS_TXT", body["method"])
		}
		_, _ = w.Write([]byte(`{"message":"ok","code":200,"status":true,"data":{
			"DNS_TXT":{"method":"DNS_TXT","name":"DNS_TXT","challenges":[
				{"id":2,"order_id":"o1","domain":"example.com","challenge":"new-token","verify":0,"type":"DNS_TXT"}
			]},
			"allowed_challenge_methods":[{"method":"DNS_TXT","name":"DNS_TXT"},{"method":"FILE","name":"FILE"}]
		}}`))
	})

	set, err := c.ReloadSSLChallenge(context.Background(), creds, "o1", "DNS_TXT", "")
	if err != nil {
		t.Fatalf("ReloadSSLChallenge: %v", err)
	}
	if set.Methods["DNS_TXT"].Challenges[0].Token != "new-token" {
		t.Errorf("token = %q, want new-token", set.Methods["DNS_TXT"].Challenges[0].Token)
	}
}

func TestVerifySSLChallengeCertificateReady(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"success","code":200,"status":true,"data":{
			"step":"final_verify","certificate_ready":true,
			"certificate":"-----BEGIN CERTIFICATE-----\nAAA\n-----END CERTIFICATE-----",
			"ca_bundle":"-----BEGIN CERTIFICATE-----\nBBB\n-----END CERTIFICATE-----"
		}}`))
	})

	result, err := c.VerifySSLChallenge(context.Background(), creds, "o1", "DNS_TXT")
	if err != nil {
		t.Fatalf("VerifySSLChallenge: %v", err)
	}
	if !result.CertificateReady || result.Certificate == "" {
		t.Fatalf("result = %+v, want a ready certificate", result)
	}
}

func TestVerifySSLChallengeCertificatePending(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message":"pending","code":202,"status":true,"data":{"step":"final_verify","certificate_ready":false}}`))
	})

	result, err := c.VerifySSLChallenge(context.Background(), creds, "o1", "DNS_TXT")
	if err != nil {
		t.Fatalf("VerifySSLChallenge: %v (a 202 must not be treated as an error)", err)
	}
	if result.CertificateReady {
		t.Errorf("CertificateReady = true, want false")
	}
}

func TestVerifySSLChallengeFailed(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Domain verification failed.","code":400,"response_code":"VERIFICATION_FAILED","status":false}`))
	})

	_, err := c.VerifySSLChallenge(context.Background(), creds, "o1", "DNS_TXT")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetSSLCertificateNotReady(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/public/v1/order/o1/certificate" {
			t.Errorf("path = %s, want .../certificate", got)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message":"not ready","code":202,"status":true,"data":{"certificate_ready":false}}`))
	})

	cert, err := c.GetSSLCertificate(context.Background(), creds, "o1")
	if err != nil {
		t.Fatalf("GetSSLCertificate: %v", err)
	}
	if cert.Ready {
		t.Errorf("Ready = true, want false")
	}
}

func TestGetSSLCertificateInvalidOrderStatus(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Order is not in correct status for certificate retrieval.","code":403,"response_code":"INVALID_ORDER_STATUS","status":false}`))
	})

	// The SSL API's 403 means "wrong order state", not "bad credentials" —
	// it must not be reported as domain.ErrInvalidCredentials.
	_, err := c.GetSSLCertificate(context.Background(), creds, "o1")
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("error = %v, must not be domain.ErrInvalidCredentials for a 403 order-status error", err)
	}
}

func TestReissueSSLCertificateSuccess(t *testing.T) {
	c := newSSLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/public/v1/order/o1/reissue" {
			t.Errorf("path = %s, want .../reissue", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["csr"] != "NEW-CSR" {
			t.Errorf("csr = %v, want NEW-CSR", body["csr"])
		}
		_, _ = w.Write([]byte(`{"message":"success","code":200,"status":true,"data":{"certificate_ready":true,"certificate":"CERT","ca_bundle":"BUNDLE"}}`))
	})

	cert, err := c.ReissueSSLCertificate(context.Background(), creds, "o1", "NEW-CSR")
	if err != nil {
		t.Fatalf("ReissueSSLCertificate: %v", err)
	}
	if !cert.Ready || cert.Certificate != "CERT" {
		t.Fatalf("cert = %+v", cert)
	}
}

// TestSSLOrderFullWorkflow walks create order → process → verify challenge →
// get certificate against one mocked transport, proving the whole chain
// works together (issue #18's acceptance criteria).
func TestSSLOrderFullWorkflow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/public/v1/order/addOrder", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":"success","code":200,"status":true,"data":{"order_id":"o1","invoice_id":1,"invoice_status":"unpaid"}}`))
	})
	mux.HandleFunc("/api/public/v1/order/o1/process", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":"ok","code":200,"status":true,"data":{
			"DNS_TXT":{"method":"DNS_TXT","name":"DNS_TXT","challenges":[{"id":1,"order_id":"o1","domain":"example.com","challenge":"tok","verify":0,"type":"DNS_TXT"}]},
			"allowed_challenge_methods":[{"method":"DNS_TXT","name":"DNS_TXT"}]
		}}`))
	})
	mux.HandleFunc("/api/public/v1/order/o1/verifyChallenge", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":"success","code":200,"status":true,"data":{"step":"final_verify","certificate_ready":true,"certificate":"CERT","ca_bundle":"BUNDLE"}}`))
	})
	mux.HandleFunc("/api/public/v1/order/o1/certificate", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":"success","code":200,"status":true,"data":{"certificate_ready":true,"certificate":"CERT","ca_bundle":"BUNDLE"}}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, err := parspack.New(parspack.WithSSLBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	order, err := c.CreateSSLOrder(ctx, creds, domain.SSLOrderSpec{ProductSlug: "dv-ssl", Domain: "example.com", BillingCycle: "annually"})
	if err != nil {
		t.Fatalf("CreateSSLOrder: %v", err)
	}

	set, err := c.ProcessSSLOrder(ctx, creds, order.ID, "CSR-DATA", domain.SSLContact{
		FirstName: "John", LastName: "Doe", Country: "US", City: "NYC", Address: "1 Main St", Phone: "12025551234", Email: "a@b.com",
	})
	if err != nil {
		t.Fatalf("ProcessSSLOrder: %v", err)
	}
	if _, ok := set.Methods["DNS_TXT"]; !ok {
		t.Fatal("expected a DNS_TXT challenge from process")
	}

	verify, err := c.VerifySSLChallenge(ctx, creds, order.ID, "DNS_TXT")
	if err != nil {
		t.Fatalf("VerifySSLChallenge: %v", err)
	}
	if !verify.CertificateReady {
		t.Fatal("expected the certificate to be ready after verification")
	}

	cert, err := c.GetSSLCertificate(ctx, creds, order.ID)
	if err != nil {
		t.Fatalf("GetSSLCertificate: %v", err)
	}
	if !cert.Ready || cert.Certificate != "CERT" {
		t.Fatalf("cert = %+v", cert)
	}
}
