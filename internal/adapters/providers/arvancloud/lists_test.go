package arvancloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// TestListArvanCloudDynamicFields pins the request shape and response
// parsing of GET /dynamic-fields.
func TestListArvanCloudDynamicFields(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"list-1","name":"bad-ips","type":"ip","scope":"private","values":[
				{"id":"item-1","value":"203.0.113.5","desc":"scanner"}
			]},
			{"id":"list-2","name":"asns","type":"number","scope":"public","values":[]}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	fields, err := provider.ListArvanCloudDynamicFields(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudDynamicFields() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/dynamic-fields" {
		t.Fatalf("request = %+v, want a single GET /dynamic-fields", records)
	}
	if len(fields) != 2 {
		t.Fatalf("len(fields) = %d, want 2", len(fields))
	}
	if fields[0].ID != "list-1" || fields[0].Name != "bad-ips" ||
		fields[0].Type != domain.ArvanCloudDynamicFieldTypeIP ||
		fields[0].Scope != domain.ArvanCloudDynamicFieldScopePrivate {
		t.Errorf("fields[0] = %+v, want the parsed first entry", fields[0])
	}
	if len(fields[0].Values) != 1 || fields[0].Values[0].ID != "item-1" ||
		fields[0].Values[0].Value != "203.0.113.5" || fields[0].Values[0].Desc != "scanner" {
		t.Errorf("fields[0].Values = %+v, want one parsed item", fields[0].Values)
	}
	if fields[1].Type != domain.ArvanCloudDynamicFieldTypeNumber || len(fields[1].Values) != 0 {
		t.Errorf("fields[1] = %+v, want type number with no values", fields[1])
	}
}

// TestCreateArvanCloudDynamicField pins the request body of POST
// /dynamic-fields, including that an empty Values slice is still sent as
// "values": [] rather than omitted.
func TestCreateArvanCloudDynamicField(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusCreated, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"list-1","name":"bad-ips","type":"ip","scope":"private","values":[]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	created, err := provider.CreateArvanCloudDynamicField(context.Background(), creds(), domain.ArvanCloudDynamicField{
		Name: "bad-ips",
		Type: domain.ArvanCloudDynamicFieldTypeIP,
	})
	if err != nil {
		t.Fatalf("CreateArvanCloudDynamicField() error = %v", err)
	}
	if created.ID != "list-1" || created.Name != "bad-ips" {
		t.Errorf("created = %+v, want the parsed response", created)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/dynamic-fields" {
		t.Fatalf("request = %+v, want a single POST /dynamic-fields", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["name"] != "bad-ips" || body["type"] != "ip" {
		t.Errorf("request body = %+v, want name=bad-ips type=ip", body)
	}
	values, ok := body["values"].([]any)
	if !ok || len(values) != 0 {
		t.Errorf("request body values = %#v, want an empty array, not omitted or null", body["values"])
	}
}

// TestGetArvanCloudDynamicField pins the request shape and response parsing
// of GET /dynamic-fields/{id}.
func TestGetArvanCloudDynamicField(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"list-1","name":"bad-ips","type":"ip","scope":"private","allowed_plans":[2,3,4],"values":[]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudDynamicField(context.Background(), creds(), "list-1")
	if err != nil {
		t.Fatalf("GetArvanCloudDynamicField() error = %v", err)
	}
	if len(records) != 1 || records[0].path != "/dynamic-fields/list-1" {
		t.Fatalf("request = %+v, want GET /dynamic-fields/list-1", records)
	}
	if found.ID != "list-1" || len(found.AllowedPlans) != 3 {
		t.Errorf("found = %+v, want the parsed response with 3 allowed plans", found)
	}
}

// TestUpdateArvanCloudDynamicField pins the request body of PATCH
// /dynamic-fields/{id}: only description and type, per
// DynamicFieldUpdateRequest.
func TestUpdateArvanCloudDynamicField(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"list-1","name":"bad-ips","description":"known scanners","type":"ip","scope":"private","values":[]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.UpdateArvanCloudDynamicField(context.Background(), creds(), "list-1", "known scanners", domain.ArvanCloudDynamicFieldTypeIP)
	if err != nil {
		t.Fatalf("UpdateArvanCloudDynamicField() error = %v", err)
	}
	if updated.Description != "known scanners" {
		t.Errorf("updated.Description = %q, want %q", updated.Description, "known scanners")
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/dynamic-fields/list-1" {
		t.Fatalf("request = %+v, want a single PATCH /dynamic-fields/list-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["description"] != "known scanners" || body["type"] != "ip" {
		t.Errorf("request body = %+v, want description and type only", body)
	}
	if _, hasName := body["name"]; hasName {
		t.Errorf("request body = %+v, must not include name — the update endpoint cannot rename a list", body)
	}
}

// TestDeleteArvanCloudDynamicField pins the request shape of DELETE
// /dynamic-fields/{id}.
func TestDeleteArvanCloudDynamicField(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Deleted successfully"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudDynamicField(context.Background(), creds(), "list-1"); err != nil {
		t.Fatalf("DeleteArvanCloudDynamicField() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/dynamic-fields/list-1" {
		t.Fatalf("request = %+v, want a single DELETE /dynamic-fields/list-1", records)
	}
}

// TestDeleteArvanCloudDynamicFieldNotFound proves a 404 surfaces as
// domain.ErrNotFound, consistent with DeleteDomain's tolerant-delete
// contract at the use-case layer.
func TestDeleteArvanCloudDynamicFieldNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudDynamicField(context.Background(), creds(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteArvanCloudDynamicField() error = %v, want domain.ErrNotFound", err)
	}
}

// TestAddArvanCloudDynamicFieldItems pins the request body of POST
// /dynamic-fields/{id}/items, and that a response carrying no "data" (only a
// confirmation message) is not treated as an error.
func TestAddArvanCloudDynamicFieldItems(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusCreated, func(*http.Request) []byte {
		return []byte(`{"message":"Added successfully"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.AddArvanCloudDynamicFieldItems(context.Background(), creds(), "list-1", []domain.ArvanCloudDynamicFieldValue{
		{Value: "203.0.113.5", Desc: "scanner"},
		{Value: "203.0.113.6"},
	})
	if err != nil {
		t.Fatalf("AddArvanCloudDynamicFieldItems() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/dynamic-fields/list-1/items" {
		t.Fatalf("request = %+v, want a single POST /dynamic-fields/list-1/items", records)
	}
	var body struct {
		Values []map[string]any `json:"values"`
	}
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if len(body.Values) != 2 || body.Values[0]["value"] != "203.0.113.5" || body.Values[0]["desc"] != "scanner" {
		t.Errorf("request body values = %+v, want the two submitted items", body.Values)
	}
	if _, hasID := body.Values[0]["id"]; hasID {
		t.Errorf("request body values[0] = %+v, must not send an id for a new item", body.Values[0])
	}
}

// TestRemoveArvanCloudDynamicFieldItem pins the request shape of DELETE
// /dynamic-fields/{id}/items/{item_id}.
func TestRemoveArvanCloudDynamicFieldItem(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Deleted successfully"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.RemoveArvanCloudDynamicFieldItem(context.Background(), creds(), "list-1", "item-1"); err != nil {
		t.Fatalf("RemoveArvanCloudDynamicFieldItem() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/dynamic-fields/list-1/items/item-1" {
		t.Fatalf("request = %+v, want a single DELETE /dynamic-fields/list-1/items/item-1", records)
	}
}
