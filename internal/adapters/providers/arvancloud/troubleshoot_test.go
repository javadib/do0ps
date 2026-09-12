package arvancloud

import (
	"context"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// TestListArvanCloudTroubleshoots pins GET /domains/{domain}/troubleshoots.
func TestListArvanCloudTroubleshoots(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"tr-1","details":[
				{"id":"root_dns_record","status":"safe","details":"Root DNS record exists"},
				{"id":"https_redirection","status":"troubled","details":"HTTPS redirect not configured"}
			],"created_at":"2026-08-01T10:00:00Z"},
			{"id":"tr-2","details":[
				{"id":"active_certificate","status":"safe","details":"Certificate is valid"}
			],"created_at":"2026-08-02T12:00:00Z"}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	list, err := provider.ListArvanCloudTroubleshoots(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudTroubleshoots() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/troubleshoots" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/troubleshoots", records)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
	if list[0].ID != "tr-1" || list[0].CreatedAt != "2026-08-01T10:00:00Z" {
		t.Errorf("list[0] = %+v, want id=tr-1", list[0])
	}
	if len(list[0].Details) != 2 {
		t.Fatalf("len(list[0].Details) = %d, want 2", len(list[0].Details))
	}
	if list[0].Details[0].ID != domain.ArvanCloudTroubleshootDetailRootDNSRecord || list[0].Details[0].Status != domain.ArvanCloudTroubleshootDetailStatusSafe {
		t.Errorf("list[0].Details[0] = %+v, want root_dns_record/safe", list[0].Details[0])
	}
	if list[0].Details[1].ID != domain.ArvanCloudTroubleshootDetailHTTPSRedirection || list[0].Details[1].Status != domain.ArvanCloudTroubleshootDetailStatusTroubled {
		t.Errorf("list[0].Details[1] = %+v, want https_redirection/troubled", list[0].Details[1])
	}
}

// TestRunArvanCloudTroubleshoot pins POST /domains/{domain}/troubleshoots
// and confirms the result is returned synchronously (fast operation, not long).
func TestRunArvanCloudTroubleshoot(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(r *http.Request) []byte {
		return []byte(`{"data":{"id":"tr-new","details":[
			{"id":"domain_active_status","status":"safe","details":"Domain is active"},
			{"id":"cloud_icon","status":"troubled","details":"Cloud icon is disabled"}
		],"created_at":"2026-08-10T14:00:00Z"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	result, err := provider.RunArvanCloudTroubleshoot(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("RunArvanCloudTroubleshoot() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/troubleshoots" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/troubleshoots", records)
	}
	if result == nil {
		t.Fatal("result = nil, want a troubleshoot result")
	}
	if result.ID != "tr-new" || result.CreatedAt != "2026-08-10T14:00:00Z" {
		t.Errorf("result = %+v, want id=tr-new", result)
	}
	if len(result.Details) != 2 {
		t.Fatalf("len(result.Details) = %d, want 2", len(result.Details))
	}
	if result.Details[0].ID != domain.ArvanCloudTroubleshootDetailDomainActiveStatus {
		t.Errorf("result.Details[0].ID = %q, want domain_active_status", result.Details[0].ID)
	}
	if result.Details[1].Status != domain.ArvanCloudTroubleshootDetailStatusTroubled {
		t.Errorf("result.Details[1].Status = %q, want troubled", result.Details[1].Status)
	}
}

// TestRunArvanCloudTroubleshoot_error proves error propagation.
func TestRunArvanCloudTroubleshoot_error(t *testing.T) {
	srv := recordingServer(t, http.StatusNotFound, func(*http.Request) []byte {
		return []byte(`{"message":"domain not found"}`)
	}, nil)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	_, err := provider.RunArvanCloudTroubleshoot(context.Background(), creds(), "nonexistent.com")
	if err == nil {
		t.Fatal("expected error for nonexistent domain")
	}
}

// TestGetLatestArvanCloudTroubleshoot pins GET /domains/{domain}/troubleshoots/latest.
func TestGetLatestArvanCloudTroubleshoot(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"tr-latest","details":[
			{"id":"mx_dns_record","status":"safe","details":"MX record exists"}
		],"created_at":"2026-08-12T09:00:00Z"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	result, err := provider.GetLatestArvanCloudTroubleshoot(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetLatestArvanCloudTroubleshoot() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/troubleshoots/latest" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/troubleshoots/latest", records)
	}
	if result == nil {
		t.Fatal("result = nil, want a troubleshoot result")
	}
	if result.ID != "tr-latest" {
		t.Errorf("result.ID = %q, want tr-latest", result.ID)
	}
	if len(result.Details) != 1 || result.Details[0].ID != domain.ArvanCloudTroubleshootDetailMxDNSRecord {
		t.Errorf("result.Details[0] = %+v, want mx_dns_record", result.Details[0])
	}
}

// TestGetLatestArvanCloudTroubleshoot_error proves error propagation.
func TestGetLatestArvanCloudTroubleshoot_error(t *testing.T) {
	srv := recordingServer(t, http.StatusNotFound, func(*http.Request) []byte {
		return []byte(`{"message":"no troubleshoots yet"}`)
	}, nil)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	_, err := provider.GetLatestArvanCloudTroubleshoot(context.Background(), creds(), "example.com")
	if err == nil {
		t.Fatal("expected error when no troubleshoots exist")
	}
}

// TestListArvanCloudTroubleshoots_empty proves an empty result is handled.
func TestListArvanCloudTroubleshoots_empty(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	list, err := provider.ListArvanCloudTroubleshoots(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudTroubleshoots() error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("len(list) = %d, want 0", len(list))
	}
}

// TestTroubleshootDetailIDValidation proves all 9 detail IDs are valid and
// unknown IDs are rejected.
func TestTroubleshootDetailIDValidation(t *testing.T) {
	valid := []string{
		"root_dns_record", "www_dns_record", "mx_dns_record",
		"https_redirection", "domain_active_status", "active_certificate",
		"cloud_icon", "domain_expiration_days", "origin_ssl_port",
	}
	for _, id := range valid {
		if !domain.ValidArvanCloudTroubleshootDetailID(id) {
			t.Errorf("ValidArvanCloudTroubleshootDetailID(%q) = false, want true", id)
		}
	}
	if domain.ValidArvanCloudTroubleshootDetailID("unknown_check") {
		t.Error("ValidArvanCloudTroubleshootDetailID(\"unknown_check\") = true, want false")
	}
}

// TestTroubleshootDetailStatusValidation proves both statuses are valid and
// unknown statuses are rejected.
func TestTroubleshootDetailStatusValidation(t *testing.T) {
	if !domain.ValidArvanCloudTroubleshootDetailStatus("safe") {
		t.Error("ValidArvanCloudTroubleshootDetailStatus(\"safe\") = false, want true")
	}
	if !domain.ValidArvanCloudTroubleshootDetailStatus("troubled") {
		t.Error("ValidArvanCloudTroubleshootDetailStatus(\"troubled\") = false, want true")
	}
	if domain.ValidArvanCloudTroubleshootDetailStatus("unknown") {
		t.Error("ValidArvanCloudTroubleshootDetailStatus(\"unknown\") = true, want false")
	}
}
