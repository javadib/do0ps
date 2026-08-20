package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javadib/do0ps/internal/adapters/providers/arvancloud"
	"github.com/javadib/do0ps/internal/core/app"
)

// newArvanCloudCreateDomainTool wires create_arvancloud_domain onto a real
// arvancloud.Provider pointed at a fake CDN API, run inline (no worker pool)
// so the test can assert directly on what left the process.
func newArvanCloudCreateDomainTool(t *testing.T, providerSrv *httptest.Server) Tool {
	t.Helper()

	client, err := arvancloud.New(arvancloud.WithBaseURL(providerSrv.URL))
	if err != nil {
		t.Fatalf("arvancloud.New() error = %v", err)
	}
	provider, err := arvancloud.NewProvider(client)
	if err != nil {
		t.Fatalf("arvancloud.NewProvider() error = %v", err)
	}

	return createArvanCloudDomainTool(app.NewCreateArvanCloudDomain(inlineQueue{}, provider))
}

// TestCreateArvanCloudDomainImportDNSRecordsDefaultsToTrue is the AC #62 pin:
// a create_arvancloud_domain call that omits import_dns_records entirely must
// still reach ArvanCloud with the field explicitly set to true — its
// documented default — rather than sending the field's Go zero value
// (false) or leaving it out and hoping the provider agrees.
func TestCreateArvanCloudDomainImportDNSRecordsDefaultsToTrue(t *testing.T) {
	var body []byte
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"uuid-1","name":"example.com"}}`))
	}))
	defer providerSrv.Close()

	tool := newArvanCloudCreateDomainTool(t, providerSrv)

	// Deliberately no "import_dns_records" key at all — this is what a tool
	// caller who never mentioned it sends.
	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	got, ok := sent["import_dns_records"].(bool)
	if !ok {
		t.Fatalf(`request body["import_dns_records"] = %v, want an explicit bool`, sent["import_dns_records"])
	}
	if !got {
		t.Errorf("import_dns_records = false, want true: omitting the tool argument must default to ArvanCloud's own documented default (true)")
	}
}

// TestCreateArvanCloudDomainImportDNSRecordsExplicitFalseIsRespected is the
// other half of the contract: a caller who explicitly opts out must not be
// silently overridden by the default above.
func TestCreateArvanCloudDomainImportDNSRecordsExplicitFalseIsRespected(t *testing.T) {
	var body []byte
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"uuid-1","name":"example.com"}}`))
	}))
	defer providerSrv.Close()

	tool := newArvanCloudCreateDomainTool(t, providerSrv)

	args, err := json.Marshal(map[string]any{
		"api_key":            "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":             "example.com",
		"import_dns_records": false,
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if got, ok := sent["import_dns_records"].(bool); !ok || got {
		t.Errorf(`import_dns_records = %v, want explicit false to be forwarded as-is`, sent["import_dns_records"])
	}
}
