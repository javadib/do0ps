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

// newArvanCloudListsProvider wires an arvancloud.Provider onto a fake CDN API
// for the tests below.
func newArvanCloudListsProvider(t *testing.T, providerSrv *httptest.Server) *arvancloud.Provider {
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

// TestCreateArvanCloudDynamicFieldTool is an end-to-end tool -> use case ->
// real adapter -> fake HTTP server round trip: the create_arvancloud_dynamic_
// field tool sends the request the CDN API expects and returns the parsed
// result.
func TestCreateArvanCloudDynamicFieldTool(t *testing.T) {
	var body []byte
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"list-1","name":"bad-ips","type":"ip","scope":"private","values":[{"id":"item-1","value":"203.0.113.5"}]}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudListsProvider(t, providerSrv)
	tool := createArvanCloudDynamicFieldTool(app.NewCreateArvanCloudDynamicField(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"name":    "bad-ips",
		"type":    "ip",
		"values":  []map[string]any{{"value": "203.0.113.5"}},
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
	if sent["name"] != "bad-ips" || sent["type"] != "ip" {
		t.Errorf("request body = %+v, want name=bad-ips type=ip", sent)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["id"] != "list-1" {
		t.Errorf("result[id] = %v, want list-1", out["id"])
	}
}

// TestCreateArvanCloudDynamicFieldToolEmptyValuesSentAsEmptyArray proves that
// omitting "values" entirely still reaches the provider as an empty array,
// not a JSON null — the store endpoint's "values" field is required.
func TestCreateArvanCloudDynamicFieldToolEmptyValuesSentAsEmptyArray(t *testing.T) {
	var body []byte
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"list-1","name":"bad-ips","type":"ip","scope":"private","values":[]}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudListsProvider(t, providerSrv)
	tool := createArvanCloudDynamicFieldTool(app.NewCreateArvanCloudDynamicField(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"name":    "bad-ips",
		"type":    "ip",
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
	values, ok := sent["values"].([]any)
	if !ok || len(values) != 0 {
		t.Errorf(`request body["values"] = %#v, want an empty array, not omitted or null`, sent["values"])
	}
}

// TestCreateArvanCloudDynamicFieldToolRejectsBadType proves an invalid type
// never reaches the provider.
func TestCreateArvanCloudDynamicFieldToolRejectsBadType(t *testing.T) {
	called := false
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer providerSrv.Close()

	provider := newArvanCloudListsProvider(t, providerSrv)
	tool := createArvanCloudDynamicFieldTool(app.NewCreateArvanCloudDynamicField(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"name":    "bad-ips",
		"type":    "string",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal("Handler() error = nil, want a validation error for an unrecognized type")
	}
	if called {
		t.Error("provider was called despite invalid input")
	}
}

// TestDeleteArvanCloudDynamicFieldToolTolerantOfNotFound proves the tool
// reports a missing list as already deleted rather than an error.
func TestDeleteArvanCloudDynamicFieldToolTolerantOfNotFound(t *testing.T) {
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudListsProvider(t, providerSrv)
	tool := deleteArvanCloudDynamicFieldTool(app.NewDeleteArvanCloudDynamicField(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"id":      "missing",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v, want nil (already-absent list tolerated)", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["deleted"] != true {
		t.Errorf("result = %#v, want deleted=true", result)
	}
}

// TestAddArvanCloudDynamicFieldItemsTool pins the request body sent to
// lists.items.store and that a response with no "data" is still a success.
func TestAddArvanCloudDynamicFieldItemsTool(t *testing.T) {
	var body []byte
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message":"Added successfully"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudListsProvider(t, providerSrv)
	tool := addArvanCloudDynamicFieldItemsTool(app.NewAddArvanCloudDynamicFieldItems(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"id":      "list-1",
		"values":  []map[string]any{{"value": "203.0.113.5", "desc": "scanner"}},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/dynamic-fields/list-1/items" {
		t.Errorf("request path = %q, want /dynamic-fields/list-1/items", path)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	values, ok := sent["values"].([]any)
	if !ok || len(values) != 1 {
		t.Fatalf("request body values = %#v, want one item", sent["values"])
	}

	out, ok := result.(map[string]any)
	if !ok || out["added"] != true || out["count"] != 1 {
		t.Errorf("result = %#v, want added=true count=1", result)
	}
}

// TestRemoveArvanCloudDynamicFieldItemTool pins the request path sent to
// lists.items.destroy.
func TestRemoveArvanCloudDynamicFieldItemTool(t *testing.T) {
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"Deleted successfully"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudListsProvider(t, providerSrv)
	tool := removeArvanCloudDynamicFieldItemTool(app.NewRemoveArvanCloudDynamicFieldItem(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"id":      "list-1",
		"item_id": "item-1",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/dynamic-fields/list-1/items/item-1" {
		t.Errorf("request path = %q, want /dynamic-fields/list-1/items/item-1", path)
	}
}
