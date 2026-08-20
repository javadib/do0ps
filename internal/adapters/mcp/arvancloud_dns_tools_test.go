package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/adapters/providers/arvancloud"
	"github.com/javadib/do0ps/internal/core/app"
)

// newArvanCloudDNSProvider wires an arvancloud.Provider against a fake CDN
// API, mirroring newArvanCloudCreateDomainTool in
// arvancloud_domain_tools_test.go.
func newArvanCloudDNSProvider(t *testing.T, providerSrv *httptest.Server) *arvancloud.Provider {
	t.Helper()
	client, err := arvancloud.New(arvancloud.WithBaseURL(providerSrv.URL))
	if err != nil {
		t.Fatalf("arvancloud.New() error = %v", err)
	}
	provider, err := arvancloud.NewProvider(client)
	if err != nil {
		t.Fatalf("arvancloud.NewProvider() error = %v", err)
	}
	return provider
}

// TestCreateArvanCloudDNSRecordToolRoundTrip exercises
// create_arvancloud_dns_record end to end: tool args -> use case -> real
// adapter -> fake CDN API -> back through to the tool result, for a CAA
// record (the type with the most caller-visible required fields).
func TestCreateArvanCloudDNSRecordToolRoundTrip(t *testing.T) {
	var body []byte
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"uuid-1","name":"@","type":"caa","ttl":3600,"value":{"value":"letsencrypt.org","tag":"issue"}}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDNSProvider(t, providerSrv)
	tool := createArvanCloudDNSRecordTool(app.NewCreateArvanCloudDNSRecord(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
		"name":    "@",
		"type":    "caa",
		"ttl":     3600,
		"values":  []map[string]any{{"value": "letsencrypt.org", "tag": "issue"}},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["type"] != "caa" {
		t.Errorf(`request body["type"] = %v, want "caa"`, sent["type"])
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["id"] != "uuid-1" || out["type"] != "caa" {
		t.Errorf("result = %v, want id/type from the parsed response", out)
	}
}

// TestCreateArvanCloudDNSRecordToolRejectsInvalidTTL proves the use case's
// client-side validation runs before any request reaches the provider, at
// the tool boundary too.
func TestCreateArvanCloudDNSRecordToolRejectsInvalidTTL(t *testing.T) {
	var called bool
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDNSProvider(t, providerSrv)
	tool := createArvanCloudDNSRecordTool(app.NewCreateArvanCloudDNSRecord(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
		"name":    "www",
		"type":    "a",
		"ttl":     121, // not one of ArvanCloud's fixed TTL values
		"values":  []map[string]any{{"ip": "198.51.100.1"}},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal("Handler() error = nil, want a validation error for ttl 121")
	}
	if called {
		t.Error("the provider was called despite the invalid TTL; validation must happen before dispatch")
	}
}

// TestImportArvanCloudDNSRecordsToolContentType is the AC #63 pin at the
// tool boundary: a zone_file string argument must reach the provider as a
// multipart/form-data upload, not JSON.
func TestImportArvanCloudDNSRecordsToolContentType(t *testing.T) {
	var (
		contentType string
		fileContent []byte
	)
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		file, _, err := r.FormFile("f_zone_file")
		if err != nil {
			t.Fatalf("FormFile(f_zone_file): %v", err)
		}
		defer file.Close()
		fileContent, _ = io.ReadAll(file)
		_, _ = w.Write([]byte(`{"message":"Successfully imported DNS records"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDNSProvider(t, providerSrv)
	tool := importArvanCloudDNSRecordsTool(app.NewImportArvanCloudDNSRecords(inlineQueue{}, provider))

	zoneFile := "www IN A 198.51.100.1\n"
	args, err := json.Marshal(map[string]any{
		"api_key":   "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":    "example.com",
		"zone_file": zoneFile,
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data", contentType)
	}
	if string(fileContent) != zoneFile {
		t.Errorf("uploaded file content = %q, want %q", fileContent, zoneFile)
	}
}

// TestExportArvanCloudDNSRecordsToolContentType is the AC #63 pin at the
// tool boundary: the tool must return the provider's text/plain response
// verbatim as the "zone_file" result field, not fail trying to parse it as
// JSON.
func TestExportArvanCloudDNSRecordsToolContentType(t *testing.T) {
	const zoneFile = "$ORIGIN example.com.\nwww IN A 198.51.100.1\n"
	var accept string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(zoneFile))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDNSProvider(t, providerSrv)
	tool := exportArvanCloudDNSRecordsTool(app.NewExportArvanCloudDNSRecords(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if accept != "text/plain" {
		t.Errorf("Accept = %q, want text/plain", accept)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["zone_file"] != zoneFile {
		t.Errorf(`result["zone_file"] = %v, want the raw exported content %q`, out["zone_file"], zoneFile)
	}
}

// TestDeleteArvanCloudDNSRecordToolTolerantOfNotFound proves the tool
// surfaces the use case's tolerant-delete contract: deleting an already-gone
// record reports success, not an error.
func TestDeleteArvanCloudDNSRecordToolTolerantOfNotFound(t *testing.T) {
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Record not found."}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDNSProvider(t, providerSrv)
	tool := deleteArvanCloudDNSRecordTool(app.NewDeleteArvanCloudDNSRecord(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
		"id":      "gone",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v, want nil for an already-gone record", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["deleted"] != true {
		t.Errorf("result = %#v, want deleted=true", result)
	}
}

// TestSetAndRemoveArvanCloudSecondaryDNSTools smoke-tests the Secondary DNS
// set/remove pair through their tools.
func TestSetAndRemoveArvanCloudSecondaryDNSTools(t *testing.T) {
	var lastMethod string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		switch r.Method {
		case http.MethodPost:
			_, _ = w.Write([]byte(`{"data":{"status":true,"nameserver":"ns1.example.com"}}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDNSProvider(t, providerSrv)
	setTool := setArvanCloudSecondaryDNSTool(app.NewSetArvanCloudSecondaryDNS(inlineQueue{}, provider))
	removeTool := removeArvanCloudSecondaryDNSTool(app.NewRemoveArvanCloudSecondaryDNS(inlineQueue{}, provider))

	setArgs, _ := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d", "domain": "example.com",
		"status": true, "nameserver": "ns1.example.com",
	})
	setResult, err := setTool.Handler(context.Background(), setArgs)
	if err != nil {
		t.Fatalf("set Handler() error = %v", err)
	}
	if out, ok := setResult.(map[string]any); !ok || out["nameserver"] != "ns1.example.com" {
		t.Errorf("set result = %#v, want nameserver ns1.example.com", setResult)
	}
	if lastMethod != http.MethodPost {
		t.Errorf("last method = %q, want POST", lastMethod)
	}

	removeArgs, _ := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d", "domain": "example.com",
	})
	removeResult, err := removeTool.Handler(context.Background(), removeArgs)
	if err != nil {
		t.Fatalf("remove Handler() error = %v", err)
	}
	if out, ok := removeResult.(map[string]any); !ok || out["removed"] != true {
		t.Errorf("remove result = %#v, want removed=true", removeResult)
	}
	if lastMethod != http.MethodDelete {
		t.Errorf("last method = %q, want DELETE", lastMethod)
	}
}
