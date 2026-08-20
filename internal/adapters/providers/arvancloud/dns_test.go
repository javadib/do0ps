package arvancloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// recordTypeRoundTripCase pins one of the 13 record types' create request
// shape and response decoding — the highest-risk part of issue #63, per its
// own acceptance criteria: a test per type, not one generic test with a
// placeholder type.
type recordTypeRoundTripCase struct {
	name string
	rec  domain.ArvanCloudDNSRecord
	// wantValueJSON is the exact "value" JSON the create request must carry.
	wantValueJSON string
	// responseValueJSON is what the fake server answers with for "value";
	// checkValues asserts the decoded record's Values against it.
	responseValueJSON string
	checkValues       func(t *testing.T, values []domain.ArvanCloudDNSRecordValue)
}

func TestRecordTypeRoundTrip(t *testing.T) {
	cases := []recordTypeRoundTripCase{
		{
			name: "A",
			rec: domain.ArvanCloudDNSRecord{
				Name: "www", Type: domain.ArvanCloudDNSRecordTypeA, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{IP: "198.51.100.42", Port: 8080, Weight: 500, Country: "US"}},
			},
			wantValueJSON:     `[{"ip":"198.51.100.42","port":8080,"weight":500,"country":"US"}]`,
			responseValueJSON: `[{"ip":"198.51.100.42","port":8080,"weight":500,"country":"US"}]`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].IP != "198.51.100.42" || values[0].Port != 8080 ||
					values[0].Weight != 500 || values[0].Country != "US" {
					t.Errorf("values = %+v, want the parsed A value", values)
				}
			},
		},
		{
			name: "AAAA",
			rec: domain.ArvanCloudDNSRecord{
				Name: "www", Type: domain.ArvanCloudDNSRecordTypeAAAA, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{IP: "2001:db8::1"}},
			},
			wantValueJSON:     `[{"ip":"2001:db8::1"}]`,
			responseValueJSON: `[{"ip":"2001:db8::1","original_weight":100}]`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].IP != "2001:db8::1" || values[0].OriginalWeight != 100 {
					t.Errorf("values = %+v, want the parsed AAAA value with original_weight", values)
				}
			},
		},
		{
			name: "CNAME",
			rec: domain.ArvanCloudDNSRecord{
				Name: "shop", Type: domain.ArvanCloudDNSRecordTypeCNAME, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Host: "cdn.example.net", HostHeader: domain.ArvanCloudHostHeaderDest, Port: 443}},
			},
			wantValueJSON:     `{"host":"cdn.example.net","host_header":"dest","port":443}`,
			responseValueJSON: `{"host":"cdn.example.net","host_header":"dest","port":443}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].Host != "cdn.example.net" || values[0].HostHeader != "dest" || values[0].Port != 443 {
					t.Errorf("values = %+v, want the parsed CNAME value", values)
				}
			},
		},
		{
			name: "ANAME",
			rec: domain.ArvanCloudDNSRecord{
				Name: "@", Type: domain.ArvanCloudDNSRecordTypeANAME, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Location: "cdn.example.com", HostHeader: domain.ArvanCloudHostHeaderSource}},
			},
			wantValueJSON:     `{"location":"cdn.example.com","host_header":"source"}`,
			responseValueJSON: `{"location":"cdn.example.com","host_header":"source"}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].Location != "cdn.example.com" || values[0].HostHeader != "source" {
					t.Errorf("values = %+v, want the parsed ANAME value", values)
				}
			},
		},
		{
			name: "MX",
			rec: domain.ArvanCloudDNSRecord{
				Name: "@", Type: domain.ArvanCloudDNSRecordTypeMX, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Host: "mail.example.com", Priority: 10}},
			},
			wantValueJSON:     `{"host":"mail.example.com","priority":10}`,
			responseValueJSON: `{"host":"mail.example.com","priority":10}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].Host != "mail.example.com" || values[0].Priority != 10 {
					t.Errorf("values = %+v, want the parsed MX value", values)
				}
			},
		},
		{
			name: "SRV",
			rec: domain.ArvanCloudDNSRecord{
				Name: "_sip._tcp", Type: domain.ArvanCloudDNSRecordTypeSRV, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Target: "sip.example.com", Port: 5060, Weight: 10, Priority: 20}},
			},
			wantValueJSON:     `{"target":"sip.example.com","port":5060,"weight":10,"priority":20}`,
			responseValueJSON: `{"target":"sip.example.com","port":5060,"weight":10,"priority":20}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].Target != "sip.example.com" || values[0].Port != 5060 ||
					values[0].Weight != 10 || values[0].Priority != 20 {
					t.Errorf("values = %+v, want the parsed SRV value", values)
				}
			},
		},
		{
			name: "TXT",
			rec: domain.ArvanCloudDNSRecord{
				Name: "@", Type: domain.ArvanCloudDNSRecordTypeTXT, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Text: "hello world"}},
			},
			wantValueJSON:     `{"text":"hello world"}`,
			responseValueJSON: `{"text":"hello world"}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].Text != "hello world" {
					t.Errorf("values = %+v, want the parsed TXT value", values)
				}
			},
		},
		{
			name: "SPF",
			rec: domain.ArvanCloudDNSRecord{
				Name: "@", Type: domain.ArvanCloudDNSRecordTypeSPF, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Text: "v=spf1 -all"}},
			},
			wantValueJSON:     `{"text":"v=spf1 -all"}`,
			responseValueJSON: `{"text":"v=spf1 -all"}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].Text != "v=spf1 -all" {
					t.Errorf("values = %+v, want the parsed SPF value", values)
				}
			},
		},
		{
			name: "DKIM",
			rec: domain.ArvanCloudDNSRecord{
				Name: "default._domainkey", Type: domain.ArvanCloudDNSRecordTypeDKIM, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Text: "v=DKIM1; k=rsa; p=abc123"}},
			},
			wantValueJSON:     `{"text":"v=DKIM1; k=rsa; p=abc123"}`,
			responseValueJSON: `{"text":"v=DKIM1; k=rsa; p=abc123"}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].Text != "v=DKIM1; k=rsa; p=abc123" {
					t.Errorf("values = %+v, want the parsed DKIM value", values)
				}
			},
		},
		{
			name: "NS",
			rec: domain.ArvanCloudDNSRecord{
				Name: "sub", Type: domain.ArvanCloudDNSRecordTypeNS, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Host: "ns1.example.com"}},
			},
			wantValueJSON:     `{"host":"ns1.example.com"}`,
			responseValueJSON: `{"host":"ns1.example.com"}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].Host != "ns1.example.com" {
					t.Errorf("values = %+v, want the parsed NS value", values)
				}
			},
		},
		{
			name: "PTR",
			rec: domain.ArvanCloudDNSRecord{
				Name: "42.100.51.198.in-addr.arpa", Type: domain.ArvanCloudDNSRecordTypePTR, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Domain: "host.example.com"}},
			},
			wantValueJSON:     `{"domain":"host.example.com"}`,
			responseValueJSON: `{"domain":"host.example.com"}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].Domain != "host.example.com" {
					t.Errorf("values = %+v, want the parsed PTR value", values)
				}
			},
		},
		{
			name: "TLSA",
			rec: domain.ArvanCloudDNSRecord{
				Name: "_443._tcp", Type: domain.ArvanCloudDNSRecordTypeTLSA, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{Usage: "3", Selector: "1", MatchingType: "1", Certificate: "1SKJND4KSID7OS9KJ21LSJ"}},
			},
			wantValueJSON:     `{"usage":"3","selector":"1","matching_type":"1","certificate":"1SKJND4KSID7OS9KJ21LSJ"}`,
			responseValueJSON: `{"usage":"3","selector":"1","matching_type":"1","certificate":"1SKJND4KSID7OS9KJ21LSJ"}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].Usage != "3" || values[0].Selector != "1" ||
					values[0].MatchingType != "1" || values[0].Certificate != "1SKJND4KSID7OS9KJ21LSJ" {
					t.Errorf("values = %+v, want the parsed TLSA value", values)
				}
			},
		},
		{
			name: "CAA",
			rec: domain.ArvanCloudDNSRecord{
				Name: "@", Type: domain.ArvanCloudDNSRecordTypeCAA, TTL: 3600,
				Values: []domain.ArvanCloudDNSRecordValue{{CAAValue: "letsencrypt.org", Tag: domain.ArvanCloudCAATagIssue}},
			},
			wantValueJSON:     `{"value":"letsencrypt.org","tag":"issue"}`,
			responseValueJSON: `{"value":"letsencrypt.org","tag":"issue"}`,
			checkValues: func(t *testing.T, values []domain.ArvanCloudDNSRecordValue) {
				if len(values) != 1 || values[0].CAAValue != "letsencrypt.org" || values[0].Tag != "issue" {
					t.Errorf("values = %+v, want the parsed CAA value", values)
				}
			},
		},
	}

	if len(cases) != 13 {
		t.Fatalf("len(cases) = %d, want 13 — one per ArvanCloud DNS record type", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var records []requestRecord
			wireType := tc.rec.Type.String()
			responseBody := fmt.Sprintf(
				`{"data":{"id":"uuid-1","name":%q,"type":%q,"ttl":3600,"value":%s}}`,
				tc.rec.Name, wireType, tc.responseValueJSON,
			)
			srv := recordingServer(t, http.StatusCreated, func(*http.Request) []byte {
				return []byte(responseBody)
			}, &records)
			defer srv.Close()

			provider := newTestProvider(t, srv)
			created, err := provider.CreateArvanCloudDNSRecord(context.Background(), creds(), "example.com", tc.rec)
			if err != nil {
				t.Fatalf("CreateArvanCloudDNSRecord() error = %v", err)
			}

			if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/dns-records" {
				t.Fatalf("request = %+v, want a single POST /domains/example.com/dns-records", records)
			}

			var sent map[string]json.RawMessage
			if err := json.Unmarshal(records[0].body, &sent); err != nil {
				t.Fatalf("decoding request body: %v", err)
			}
			var sentType string
			if err := json.Unmarshal(sent["type"], &sentType); err != nil || sentType != wireType {
				t.Errorf(`request body["type"] = %s, want %q`, sent["type"], wireType)
			}
			if !jsonEqual(t, sent["value"], []byte(tc.wantValueJSON)) {
				t.Errorf(`request body["value"] = %s, want %s`, sent["value"], tc.wantValueJSON)
			}

			if created.ID != "uuid-1" || created.Type != tc.rec.Type {
				t.Errorf("created = %+v, want id/type from the parsed response", created)
			}
			tc.checkValues(t, created.Values)
		})
	}
}

// jsonEqual compares two JSON documents for semantic equality (ignoring key
// order), so wantValueJSON above does not have to match Go's map key
// ordering exactly.
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("decoding %s: %v", a, err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("decoding %s: %v", b, err)
	}
	aj, _ := json.Marshal(av)
	bj, _ := json.Marshal(bv)
	return string(aj) == string(bj)
}

// TestListArvanCloudDNSRecords pins the request shape and response parsing
// of GET /domains/{domain}/dns-records.
func TestListArvanCloudDNSRecords(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"uuid-1","name":"www","type":"a","ttl":3600,"cloud":true,"value":[{"ip":"198.51.100.1"}]},
			{"id":"uuid-2","name":"@","type":"mx","ttl":3600,"value":{"host":"mail.example.com","priority":10}}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	list, err := provider.ListArvanCloudDNSRecords(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudDNSRecords() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/dns-records" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/dns-records", records)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
	if list[0].Type != domain.ArvanCloudDNSRecordTypeA || !list[0].Cloud || len(list[0].Values) != 1 || list[0].Values[0].IP != "198.51.100.1" {
		t.Errorf("list[0] = %+v, want the parsed A record", list[0])
	}
	if list[1].Type != domain.ArvanCloudDNSRecordTypeMX || len(list[1].Values) != 1 || list[1].Values[0].Priority != 10 {
		t.Errorf("list[1] = %+v, want the parsed MX record", list[1])
	}
}

// TestGetArvanCloudDNSRecord pins the request shape of GET
// /domains/{domain}/dns-records/{id}.
func TestGetArvanCloudDNSRecord(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"uuid-1","name":"www","type":"a","ttl":3600,"is_protected":true,"usage":["certificate-issuance"],"value":[{"ip":"198.51.100.1"}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudDNSRecord(context.Background(), creds(), "example.com", "uuid-1")
	if err != nil {
		t.Fatalf("GetArvanCloudDNSRecord() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/dns-records/uuid-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/dns-records/uuid-1", records)
	}
	if !found.IsProtected || len(found.Usage) != 1 || found.Usage[0] != "certificate-issuance" {
		t.Errorf("found = %+v, want is_protected and usage parsed", found)
	}
}

// TestUpdateArvanCloudDNSRecord pins the request shape of PUT
// /domains/{domain}/dns-records/{id}.
func TestUpdateArvanCloudDNSRecord(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"uuid-1","name":"www","type":"a","ttl":7200,"value":[{"ip":"198.51.100.9"}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	rec := domain.ArvanCloudDNSRecord{
		Name: "www", Type: domain.ArvanCloudDNSRecordTypeA, TTL: 7200,
		Values: []domain.ArvanCloudDNSRecordValue{{IP: "198.51.100.9"}},
	}
	updated, err := provider.UpdateArvanCloudDNSRecord(context.Background(), creds(), "example.com", "uuid-1", rec)
	if err != nil {
		t.Fatalf("UpdateArvanCloudDNSRecord() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/example.com/dns-records/uuid-1" {
		t.Fatalf("request = %+v, want a single PUT /domains/example.com/dns-records/uuid-1", records)
	}
	if updated.TTL != 7200 || len(updated.Values) != 1 || updated.Values[0].IP != "198.51.100.9" {
		t.Errorf("updated = %+v, want the parsed response", updated)
	}
}

// TestDeleteArvanCloudDNSRecord pins the request shape of DELETE
// /domains/{domain}/dns-records/{id}.
func TestDeleteArvanCloudDNSRecord(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"The record was deleted successfully."}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudDNSRecord(context.Background(), creds(), "example.com", "uuid-1"); err != nil {
		t.Fatalf("DeleteArvanCloudDNSRecord() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/dns-records/uuid-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/dns-records/uuid-1", records)
	}
}

// TestDeleteArvanCloudDNSRecordNotFound proves a 404 reaches the caller as
// domain.ErrNotFound, the same tolerant-delete contract as every other
// delete-style method on this port.
func TestDeleteArvanCloudDNSRecordNotFound(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusNotFound, func(*http.Request) []byte {
		return []byte(`{"message":"Record not found."}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudDNSRecord(context.Background(), creds(), "example.com", "gone")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteArvanCloudDNSRecord() error = %v, want domain.ErrNotFound", err)
	}
}

// TestToggleArvanCloudDNSRecordCloud pins the request body of PUT
// /domains/{domain}/dns-records/{id}/cloud.
func TestToggleArvanCloudDNSRecordCloud(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"uuid-1","name":"www","type":"a","ttl":3600,"cloud":true,"value":[{"ip":"198.51.100.1"}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.ToggleArvanCloudDNSRecordCloud(context.Background(), creds(), "example.com", "uuid-1", true)
	if err != nil {
		t.Fatalf("ToggleArvanCloudDNSRecordCloud() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/example.com/dns-records/uuid-1/cloud" {
		t.Fatalf("request = %+v, want a single PUT .../dns-records/uuid-1/cloud", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	if cloud, ok := body["cloud"].(bool); !ok || !cloud {
		t.Errorf(`request body["cloud"] = %v, want true`, body["cloud"])
	}
	if !updated.Cloud {
		t.Errorf("updated.Cloud = false, want true")
	}
}

// TestImportArvanCloudDNSRecordsContentType is the AC #63 pin: the import
// request must be multipart/form-data (verified against the spec's
// DnsRecordImport requestBody, not application/json like every other method
// on this port), carrying the zone file under the "f_zone_file" field.
func TestImportArvanCloudDNSRecordsContentType(t *testing.T) {
	var (
		contentType string
		path        string
		method      string
		fieldName   string
		fileName    string
		fileContent []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		contentType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		file, header, err := r.FormFile("f_zone_file")
		if err != nil {
			t.Fatalf("FormFile(f_zone_file): %v", err)
		}
		defer file.Close()
		fieldName = "f_zone_file"
		fileName = header.Filename
		fileContent, _ = io.ReadAll(file)
		_, _ = w.Write([]byte(`{"message":"Successfully imported DNS records"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	zoneFile := []byte("www IN A 198.51.100.1\n")
	if err := provider.ImportArvanCloudDNSRecords(context.Background(), creds(), "example.com", zoneFile); err != nil {
		t.Fatalf("ImportArvanCloudDNSRecords() error = %v", err)
	}

	if method != http.MethodPost || path != "/domains/example.com/dns-records/import" {
		t.Errorf("request = %s %s, want POST /domains/example.com/dns-records/import", method, path)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want a multipart/form-data request per the spec's DnsRecordImport schema", contentType)
	}
	if fieldName != "f_zone_file" {
		t.Errorf("form field = %q, want %q", fieldName, "f_zone_file")
	}
	if fileName == "" {
		t.Error("uploaded file name is empty")
	}
	if string(fileContent) != string(zoneFile) {
		t.Errorf("uploaded file content = %q, want %q", fileContent, zoneFile)
	}
}

// TestExportArvanCloudDNSRecordsContentType is the AC #63 pin: the export
// response is declared text/plain in the spec (a BIND zone file), not
// application/json like every other method on this port, and this adapter
// must return it verbatim rather than trying to decode it as JSON.
func TestExportArvanCloudDNSRecordsContentType(t *testing.T) {
	var (
		accept string
		path   string
		method string
	)
	const zoneFile = "$ORIGIN example.com.\nwww IN A 198.51.100.1\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		accept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(zoneFile))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	got, err := provider.ExportArvanCloudDNSRecords(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ExportArvanCloudDNSRecords() error = %v", err)
	}

	if method != http.MethodGet || path != "/domains/example.com/dns-records/export" {
		t.Errorf("request = %s %s, want GET /domains/example.com/dns-records/export", method, path)
	}
	if accept != "text/plain" {
		t.Errorf("Accept = %q, want text/plain per the spec's declared response content-type", accept)
	}
	if got != zoneFile {
		t.Errorf("exported content = %q, want the raw zone file body %q, unparsed as JSON", got, zoneFile)
	}
}

// TestGetArvanCloudDNSSecStatus pins the request shape of GET
// /domains/{domain}/dns-records/dnssec.
func TestGetArvanCloudDNSSecStatus(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"enabled":true,"ds":"example. IN DS 12345 13 2 abcd"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	status, err := provider.GetArvanCloudDNSSecStatus(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudDNSSecStatus() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/dns-records/dnssec" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/dns-records/dnssec", records)
	}
	if !status.Enabled || status.DS == "" {
		t.Errorf("status = %+v, want the parsed response", status)
	}
}

// TestUpdateArvanCloudDNSSecStatus pins the request body of PUT
// /domains/{domain}/dns-records/dnssec/actions.
func TestUpdateArvanCloudDNSSecStatus(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"enabled":true,"ds":"example. IN DS 12345 13 2 abcd"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	status, err := provider.UpdateArvanCloudDNSSecStatus(context.Background(), creds(), "example.com", true, true)
	if err != nil {
		t.Fatalf("UpdateArvanCloudDNSSecStatus() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/example.com/dns-records/dnssec/actions" {
		t.Fatalf("request = %+v, want a single PUT .../dns-records/dnssec/actions", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	if enable, ok := body["enable"].(bool); !ok || !enable {
		t.Errorf(`request body["enable"] = %v, want true`, body["enable"])
	}
	if rotate, ok := body["rotate"].(bool); !ok || !rotate {
		t.Errorf(`request body["rotate"] = %v, want true`, body["rotate"])
	}
	if !status.Enabled {
		t.Errorf("status.Enabled = false, want true")
	}
}

// TestGetArvanCloudSecondaryDNS pins the request shape of GET
// /domains/{domain}/secondary-dns.
func TestGetArvanCloudSecondaryDNS(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"status":true,"nameserver":"ns1.example.com","soa_serial":"2024010100"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	cfg, err := provider.GetArvanCloudSecondaryDNS(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudSecondaryDNS() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/secondary-dns" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/secondary-dns", records)
	}
	if !cfg.Status || cfg.Nameserver != "ns1.example.com" || cfg.SOASerial != "2024010100" {
		t.Errorf("cfg = %+v, want the parsed response", cfg)
	}
}

// TestGetArvanCloudSecondaryDNSWithErrors pins the read-only "errors" object
// parsing, the one nested shape SecondaryDNSData carries.
func TestGetArvanCloudSecondaryDNSWithErrors(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"status":true,"nameserver":"ns1.example.com",
			"errors":{"error":"partial transfer","skipped_records":[{"name":"www","type":"a","value":"198.51.100.1"}]}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	cfg, err := provider.GetArvanCloudSecondaryDNS(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudSecondaryDNS() error = %v", err)
	}
	if cfg.ErrorMessage != "partial transfer" {
		t.Errorf("cfg.ErrorMessage = %q, want %q", cfg.ErrorMessage, "partial transfer")
	}
	if len(cfg.SkippedRecords) != 1 || cfg.SkippedRecords[0].Name != "www" {
		t.Errorf("cfg.SkippedRecords = %+v, want one skipped record parsed", cfg.SkippedRecords)
	}
}

// TestSetArvanCloudSecondaryDNS pins the request body of POST
// /domains/{domain}/secondary-dns.
func TestSetArvanCloudSecondaryDNS(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"status":true,"nameserver":"ns1.example.com"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	cfg, err := provider.SetArvanCloudSecondaryDNS(context.Background(), creds(), "example.com",
		domain.ArvanCloudSecondaryDNSConfig{Status: true, Nameserver: "ns1.example.com"})
	if err != nil {
		t.Fatalf("SetArvanCloudSecondaryDNS() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/secondary-dns" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/secondary-dns", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	if body["nameserver"] != "ns1.example.com" {
		t.Errorf(`request body["nameserver"] = %v, want "ns1.example.com"`, body["nameserver"])
	}
	if !cfg.Status {
		t.Errorf("cfg.Status = false, want true")
	}
}

// TestRemoveArvanCloudSecondaryDNS pins the request shape of DELETE
// /domains/{domain}/secondary-dns, whose declared successful response is 204
// No Content.
func TestRemoveArvanCloudSecondaryDNS(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusNoContent, nil, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.RemoveArvanCloudSecondaryDNS(context.Background(), creds(), "example.com"); err != nil {
		t.Fatalf("RemoveArvanCloudSecondaryDNS() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/secondary-dns" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/secondary-dns", records)
	}
}

// TestRemoveArvanCloudSecondaryDNSNotFound proves a 404 reaches the caller as
// domain.ErrNotFound.
func TestRemoveArvanCloudSecondaryDNSNotFound(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusNotFound, func(*http.Request) []byte {
		return []byte(`{"message":"Secondary DNS config not found."}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.RemoveArvanCloudSecondaryDNS(context.Background(), creds(), "example.com")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("RemoveArvanCloudSecondaryDNS() error = %v, want domain.ErrNotFound", err)
	}
}

// TestInvalidRecordTypeIsRejectedByAdapter proves that an unrecognized
// record type never reaches the wire — it fails in this adapter's own
// encoding step, wrapping domain.ErrInvalidInput. Client-side validation in
// internal/core/app/arvancloud_dns.go is the primary defense (tested there);
// this pins the adapter's own defense-in-depth for a caller that bypasses
// it.
func TestInvalidRecordTypeIsRejectedByAdapter(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, nil, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	_, err := provider.CreateArvanCloudDNSRecord(context.Background(), creds(), "example.com",
		domain.ArvanCloudDNSRecord{Name: "www", Type: domain.ArvanCloudDNSRecordType(999), TTL: 3600})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("CreateArvanCloudDNSRecord() error = %v, want domain.ErrInvalidInput", err)
	}
	if len(records) != 0 {
		t.Errorf("requests = %+v, want no request sent for an invalid record type", records)
	}
}
