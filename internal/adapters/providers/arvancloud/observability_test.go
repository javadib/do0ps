package arvancloud

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// --- Log Forwarders ---------------------------------------------------------

// TestListArvanCloudLogForwarders pins the request shape and response
// parsing of GET /domains/{domain}/log-forwarders, including that the
// top-level data+meta envelope (PaginatedResponse, no extra "data" nesting)
// decodes correctly and that filter parameters are sent.
func TestListArvanCloudLogForwarders(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"lf-1","type":"access","connection_type":"arvan_s3","name":"s3 forwarder","description":"desc","status":true},
			{"id":"lf-2","type":"waf","connection_type":"datadog","name":"dd forwarder","description":"desc2","status":false}
		],"meta":{"current_page":1,"last_page":1,"per_page":20,"total":2}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	forwarders, page, err := provider.ListArvanCloudLogForwarders(context.Background(), creds(), "example.com",
		domain.ArvanCloudLogForwarderListQuery{Name: "s3", Types: []string{"access", "waf"}, Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("ListArvanCloudLogForwarders() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/log-forwarders" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/log-forwarders", records)
	}
	if !strings.Contains(records[0].query, "name=s3") || !strings.Contains(records[0].query, "types=access") ||
		!strings.Contains(records[0].query, "types=waf") || !strings.Contains(records[0].query, "per_page=20") {
		t.Errorf("query = %q, want name/types/per_page/page set", records[0].query)
	}
	if len(forwarders) != 2 || forwarders[0].ID != "lf-1" || forwarders[0].ConnectionType != domain.ArvanCloudConnectionTypeArvanS3 {
		t.Errorf("forwarders = %+v, want the two parsed entries", forwarders)
	}
	if forwarders[1].Type != domain.ArvanCloudLogForwarderTypeWAF || forwarders[1].Status {
		t.Errorf("forwarders[1] = %+v, want type waf and status false", forwarders[1])
	}
	if page.Total != 2 || page.PerPage != 20 {
		t.Errorf("page = %+v, want total=2, per_page=20", page)
	}
}

// TestCreateArvanCloudLogForwarder pins the request body of POST
// /domains/{domain}/log-forwarders for an S3-family connection type,
// including that settings/data_fields/mask_rules are all sent.
func TestCreateArvanCloudLogForwarder(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{
			"id":"lf-1","name":"s3 forwarder","description":"ships access logs to s3","type":"access",
			"connection_type":"arvan_s3","data_format_expr":"Method == 'GET'","status":true,
			"settings":{"s3_endpoint":"s3.example.com","access_key":"AKIA123","secret_key":"shh","bucket_name":"logs"}
		}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	forwarder := domain.ArvanCloudLogForwarder{
		Name: "s3 forwarder", Description: "ships access logs to s3",
		Type: domain.ArvanCloudLogForwarderTypeAccess, ConnectionType: domain.ArvanCloudConnectionTypeArvanS3,
		DataFormatExpr: "Method == 'GET'",
		DataFields:     domain.ArvanCloudLogForwarderDataFields{Method: true, Status: true},
		MaskRules:      []domain.ArvanCloudMaskRule{{Pattern: "(cookie=.*?_).*?(&)", Replace: "${1}*****${2}"}},
		Settings: domain.ArvanCloudLogForwarderSettings{S3: &domain.ArvanCloudLogForwarderS3Settings{
			S3Endpoint: "s3.example.com", AccessKey: "AKIA123", SecretKey: "shh", BucketName: "logs",
		}},
		Status: true,
	}
	created, err := provider.CreateArvanCloudLogForwarder(context.Background(), creds(), "example.com", forwarder)
	if err != nil {
		t.Fatalf("CreateArvanCloudLogForwarder() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/log-forwarders" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/log-forwarders", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["type"] != "access" || body["connection_type"] != "arvan_s3" {
		t.Errorf("request body type/connection_type = %+v, want access/arvan_s3", body)
	}
	settings, ok := body["settings"].(map[string]any)
	if !ok || settings["access_key"] != "AKIA123" || settings["secret_key"] != "shh" || settings["bucket_name"] != "logs" {
		t.Errorf("request body settings = %+v, want the S3 fields sent", body["settings"])
	}
	dataFields, ok := body["data_fields"].(map[string]any)
	if !ok || dataFields["method"] != true || dataFields["status"] != true {
		t.Errorf("request body data_fields = %+v, want method/status true", body["data_fields"])
	}
	maskRules, ok := body["mask_rules"].([]any)
	if !ok || len(maskRules) != 1 {
		t.Errorf("request body mask_rules = %+v, want one entry", body["mask_rules"])
	}

	if created.ID != "lf-1" || created.ConnectionType != domain.ArvanCloudConnectionTypeArvanS3 {
		t.Errorf("created = %+v, want id lf-1 and connection_type arvan_s3", created)
	}
	if created.Settings.S3 == nil || created.Settings.S3.SecretKey != "shh" {
		t.Errorf("created.Settings.S3 = %+v, want the parsed S3 settings", created.Settings.S3)
	}
	// LogForwarderGeneric never echoes data_fields/mask_rules back — see
	// observability.go's package comment.
	if created.DataFields != (domain.ArvanCloudLogForwarderDataFields{}) {
		t.Errorf("created.DataFields = %+v, want the zero value (not echoed by the provider)", created.DataFields)
	}
	if len(created.MaskRules) != 0 {
		t.Errorf("created.MaskRules = %+v, want empty (not echoed by the provider)", created.MaskRules)
	}
}

// TestCreateArvanCloudLogForwarderDatadog pins the request body for a
// Datadog connection type, proving the settings union selects the right
// branch by connection_type.
func TestCreateArvanCloudLogForwarderDatadog(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{
			"id":"lf-2","name":"dd forwarder","description":"ships waf logs to datadog","type":"waf",
			"connection_type":"datadog","status":true,
			"settings":{"url":"https://http-intake.logs.datadoghq.com","api_key":"dd-api-key"}
		}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	forwarder := domain.ArvanCloudLogForwarder{
		Name: "dd forwarder", Description: "ships waf logs to datadog",
		Type: domain.ArvanCloudLogForwarderTypeWAF, ConnectionType: domain.ArvanCloudConnectionTypeDatadog,
		Settings: domain.ArvanCloudLogForwarderSettings{Datadog: &domain.ArvanCloudLogForwarderDatadogSettings{
			URL: "https://http-intake.logs.datadoghq.com", APIKey: "dd-api-key",
		}},
		Status: true,
	}
	created, err := provider.CreateArvanCloudLogForwarder(context.Background(), creds(), "example.com", forwarder)
	if err != nil {
		t.Fatalf("CreateArvanCloudLogForwarder() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	settings, ok := body["settings"].(map[string]any)
	if !ok || settings["api_key"] != "dd-api-key" || settings["url"] != "https://http-intake.logs.datadoghq.com" {
		t.Errorf("request body settings = %+v, want the Datadog fields sent", body["settings"])
	}
	if created.Settings.Datadog == nil || created.Settings.Datadog.APIKey != "dd-api-key" {
		t.Errorf("created.Settings.Datadog = %+v, want the parsed Datadog settings", created.Settings.Datadog)
	}
	if created.Settings.S3 != nil {
		t.Errorf("created.Settings.S3 = %+v, want nil for a datadog connection_type", created.Settings.S3)
	}
}

// TestGetArvanCloudLogForwarder pins the request shape and response parsing
// of GET /domains/{domain}/log-forwarders/{id}.
func TestGetArvanCloudLogForwarder(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{
			"id":"lf-1","name":"s3 forwarder","description":"desc","type":"access","connection_type":"amazon_s3",
			"status":true,"settings":{"s3_endpoint":"s3.amazonaws.com","access_key":"AKIA","secret_key":"s3cret","bucket_name":"logs"}
		}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudLogForwarder(context.Background(), creds(), "example.com", "lf-1")
	if err != nil {
		t.Fatalf("GetArvanCloudLogForwarder() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/log-forwarders/lf-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/log-forwarders/lf-1", records)
	}
	if found.ID != "lf-1" || found.ConnectionType != domain.ArvanCloudConnectionTypeAmazonS3 {
		t.Errorf("found = %+v, want id lf-1 and connection_type amazon_s3", found)
	}
}

// TestUpdateArvanCloudLogForwarder pins the request method/path of PUT
// /domains/{domain}/log-forwarders/{id}.
func TestUpdateArvanCloudLogForwarder(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"lf-1","name":"renamed","description":"desc","type":"access","connection_type":"arvan_logs","status":false,"settings":{}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	forwarder := domain.ArvanCloudLogForwarder{
		Name: "renamed", Description: "desc", Type: domain.ArvanCloudLogForwarderTypeAccess,
		ConnectionType: domain.ArvanCloudConnectionTypeArvanLogs,
		Settings:       domain.ArvanCloudLogForwarderSettings{ArvanLogs: &domain.ArvanCloudLogForwarderArvanLogsSettings{}},
		Status:         false,
	}
	updated, err := provider.UpdateArvanCloudLogForwarder(context.Background(), creds(), "example.com", "lf-1", forwarder)
	if err != nil {
		t.Fatalf("UpdateArvanCloudLogForwarder() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/example.com/log-forwarders/lf-1" {
		t.Fatalf("request = %+v, want a single PUT /domains/example.com/log-forwarders/lf-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["status"] != false {
		t.Errorf(`request body["status"] = %#v, want explicit false`, body["status"])
	}
	if updated.Name != "renamed" || updated.Status {
		t.Errorf("updated = %+v, want name=renamed, status=false", updated)
	}
}

// TestDeleteArvanCloudLogForwarder pins the request shape of DELETE
// /domains/{domain}/log-forwarders/{id}.
func TestDeleteArvanCloudLogForwarder(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"Deleted successfully"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudLogForwarder(context.Background(), creds(), "example.com", "lf-1"); err != nil {
		t.Fatalf("DeleteArvanCloudLogForwarder() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/log-forwarders/lf-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/log-forwarders/lf-1", records)
	}
}

// TestSetArvanCloudLogForwarderStatus pins the request shape and body of
// PATCH /domains/{domain}/log-forwarders/{id}/status.
func TestSetArvanCloudLogForwarderStatus(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"lf-1","name":"s3 forwarder","description":"desc","type":"access","connection_type":"arvan_s3","status":false,"settings":{}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.SetArvanCloudLogForwarderStatus(context.Background(), creds(), "example.com", "lf-1", false)
	if err != nil {
		t.Fatalf("SetArvanCloudLogForwarderStatus() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/log-forwarders/lf-1/status" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/log-forwarders/lf-1/status", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["status"] != false {
		t.Errorf(`request body = %+v, want {"status": false}`, body)
	}
	if updated.Status {
		t.Errorf("updated.Status = true, want false")
	}
}

// TestArvanCloudLogForwarderSettingsNeverLogged proves that no field under
// LogForwarder.settings (S3's access_key/secret_key exercised here) ever
// appears in a debug log line, mirroring
// TestUpdateArvanCloudDdosSettingsNeverLogsSecretKey's style (ddos_test.go)
// — the AC #76 redaction requirement.
func TestArvanCloudLogForwarderSettingsNeverLogged(t *testing.T) {
	const accessKey = "AKIA-SHOULD-NEVER-LEAK"
	const secretKey = "super-secret-s3-key-should-never-leak"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The provider echoes the settings back, credentials included — the
		// realistic shape of log-forwarders.store's response.
		_, _ = w.Write([]byte(`{"data":{
			"id":"lf-1","name":"s3 forwarder","description":"desc","type":"access","connection_type":"arvan_s3","status":true,
			"settings":{"s3_endpoint":"s3.example.com","access_key":"` + accessKey + `","secret_key":"` + secretKey + `","bucket_name":"logs"}
		}}`))
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := newTestClient(t, srv, WithLogger(logger))
	provider, err := NewProvider(client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	forwarder := domain.ArvanCloudLogForwarder{
		Name: "s3 forwarder", Description: "desc", Type: domain.ArvanCloudLogForwarderTypeAccess,
		ConnectionType: domain.ArvanCloudConnectionTypeArvanS3,
		Settings: domain.ArvanCloudLogForwarderSettings{S3: &domain.ArvanCloudLogForwarderS3Settings{
			S3Endpoint: "s3.example.com", AccessKey: accessKey, SecretKey: secretKey, BucketName: "logs",
		}},
		Status: true,
	}
	created, err := provider.CreateArvanCloudLogForwarder(context.Background(), creds(), "example.com", forwarder)
	if err != nil {
		t.Fatalf("CreateArvanCloudLogForwarder() error = %v", err)
	}
	// The tool response reaching the same caller who supplied the
	// credentials is fine (see arvanCloudLogForwarderSettingsToMap's own
	// comment) — this test is only about the log line.
	if created.Settings.S3 == nil || created.Settings.S3.SecretKey != secretKey {
		t.Errorf("created.Settings.S3 = %+v, want it decoded from the response", created.Settings.S3)
	}

	if strings.Contains(logBuf.String(), accessKey) || strings.Contains(logBuf.String(), secretKey) {
		t.Errorf("debug log contains S3 settings verbatim:\n%s", logBuf.String())
	}
}

// TestArvanCloudLogForwarderSettingsErrorNeverContainsSettings proves a
// failed create's error message does not contain any settings field either,
// even when the provider's error response happens to echo the submitted
// settings back.
func TestArvanCloudLogForwarderSettingsErrorNeverContainsSettings(t *testing.T) {
	const secretKey = "super-secret-s3-key-should-never-leak"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		// An error response shape this adapter does not recognize (no
		// "message"/"errors" fields), so mapErrorResponse falls back to the
		// raw body excerpt — the path TestArvanCloudLogForwarderSettingsNeverLogged
		// does not exercise.
		_, _ = w.Write([]byte(`{"submitted":{"connection_type":"arvan_s3","secret_key":"` + secretKey + `"}}`))
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := newTestClient(t, srv, WithLogger(logger))
	provider, err := NewProvider(client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	forwarder := domain.ArvanCloudLogForwarder{
		Name: "s3 forwarder", Description: "desc", Type: domain.ArvanCloudLogForwarderTypeAccess,
		ConnectionType: domain.ArvanCloudConnectionTypeArvanS3,
		Settings: domain.ArvanCloudLogForwarderSettings{S3: &domain.ArvanCloudLogForwarderS3Settings{
			S3Endpoint: "s3.example.com", AccessKey: "AKIA", SecretKey: secretKey, BucketName: "logs",
		}},
		Status: true,
	}
	_, err = provider.CreateArvanCloudLogForwarder(context.Background(), creds(), "example.com", forwarder)
	if err == nil {
		t.Fatal("CreateArvanCloudLogForwarder() error = nil, want the 422 to surface")
	}

	if strings.Contains(err.Error(), secretKey) {
		t.Errorf("error message contains the S3 secret_key verbatim: %v", err)
	}
	if strings.Contains(logBuf.String(), secretKey) {
		t.Errorf("debug log contains the S3 secret_key verbatim:\n%s", logBuf.String())
	}
}

// --- Metric Exporters --------------------------------------------------------

// TestListArvanCloudMetricExporters pins the request shape (account-wide,
// no /domains/{domain} prefix) and response parsing of GET
// /metric-exporters, including the per-item "domain" field that resolves
// the list-vs-CRUD scoping asymmetry.
func TestListArvanCloudMetricExporters(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"me-1","type":"access","interval":"30s","name":"exporter one","domain":"example.com","url":"https://metrics.example/one","status":true},
			{"id":"me-2","type":"dns","interval":"60s","name":"exporter two","domain":"other.com","url":"https://metrics.example/two","status":false}
		],"meta":{"current_page":1,"last_page":1,"per_page":20,"total":2}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	exporters, page, err := provider.ListArvanCloudMetricExporters(context.Background(), creds(),
		domain.ArvanCloudMetricExporterListQuery{Types: []string{"access", "dns"}, PerPage: 20, Page: 1})
	if err != nil {
		t.Fatalf("ListArvanCloudMetricExporters() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/metric-exporters" {
		t.Fatalf("request = %+v, want a single GET /metric-exporters (account-wide, no /domains prefix)", records)
	}
	if len(exporters) != 2 || exporters[0].Domain != "example.com" || exporters[1].Domain != "other.com" {
		t.Errorf("exporters = %+v, want each entry's own \"domain\" field parsed", exporters)
	}
	if page.Total != 2 {
		t.Errorf("page = %+v, want total=2", page)
	}
}

// TestListArvanCloudMetricExporterTypes pins the request shape and response
// parsing of GET /metric-exporters/metrics.
func TestListArvanCloudMetricExporterTypes(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"metric":"access","items":[{"name":"requests_total","description":"total requests"}]},
			{"metric":"dns","items":[{"name":"queries_total","description":"total dns queries"}]}
		],"message":"ok"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	metrics, err := provider.ListArvanCloudMetricExporterTypes(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudMetricExporterTypes() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/metric-exporters/metrics" {
		t.Fatalf("request = %+v, want a single GET /metric-exporters/metrics", records)
	}
	if len(metrics.Groups) != 2 || metrics.Groups[0].Metric != "access" || len(metrics.Groups[0].Items) != 1 {
		t.Errorf("metrics.Groups = %+v, want the two parsed groups", metrics.Groups)
	}
	if metrics.Message != "ok" {
		t.Errorf("metrics.Message = %q, want %q", metrics.Message, "ok")
	}
}

// TestCreateArvanCloudMetricExporter pins the request body of POST
// /domains/{domain}/metric-exporters.
func TestCreateArvanCloudMetricExporter(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"me-1","name":"exporter one","type":"access","interval":"30s","url":"https://metrics.example/one","status":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	exporter := domain.ArvanCloudMetricExporter{
		Name: "exporter one", Type: domain.ArvanCloudMetricExporterTypeAccess,
		Interval: domain.ArvanCloudMetricExporterInterval30s, Status: true,
	}
	created, err := provider.CreateArvanCloudMetricExporter(context.Background(), creds(), "example.com", exporter)
	if err != nil {
		t.Fatalf("CreateArvanCloudMetricExporter() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/metric-exporters" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/metric-exporters", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["type"] != "access" || body["interval"] != "30s" {
		t.Errorf("request body = %+v, want type=access, interval=30s", body)
	}
	if created.ID != "me-1" || created.URL != "https://metrics.example/one" {
		t.Errorf("created = %+v, want id me-1 and the parsed url", created)
	}
	if created.Domain != "" {
		t.Errorf("created.Domain = %q, want empty — MetricExporterResponse carries no \"domain\" field", created.Domain)
	}
}

// TestGetArvanCloudMetricExporter pins the request shape and response
// parsing of GET /domains/{domain}/metric-exporters/{id}.
func TestGetArvanCloudMetricExporter(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"me-1","name":"exporter one","type":"access","interval":"30s","url":"https://metrics.example/one","status":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetArvanCloudMetricExporter(context.Background(), creds(), "example.com", "me-1")
	if err != nil {
		t.Fatalf("GetArvanCloudMetricExporter() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/metric-exporters/me-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/metric-exporters/me-1", records)
	}
	if found.ID != "me-1" || found.Interval != domain.ArvanCloudMetricExporterInterval30s {
		t.Errorf("found = %+v, want id me-1 and interval 30s", found)
	}
}

// TestUpdateArvanCloudMetricExporter pins the request method/path of PUT
// /domains/{domain}/metric-exporters/{id}.
func TestUpdateArvanCloudMetricExporter(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"me-1","name":"renamed","type":"error","interval":"60s","status":false}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	exporter := domain.ArvanCloudMetricExporter{
		Name: "renamed", Type: domain.ArvanCloudMetricExporterTypeError,
		Interval: domain.ArvanCloudMetricExporterInterval60s, Status: false,
	}
	updated, err := provider.UpdateArvanCloudMetricExporter(context.Background(), creds(), "example.com", "me-1", exporter)
	if err != nil {
		t.Fatalf("UpdateArvanCloudMetricExporter() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/example.com/metric-exporters/me-1" {
		t.Fatalf("request = %+v, want a single PUT /domains/example.com/metric-exporters/me-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["status"] != false {
		t.Errorf(`request body["status"] = %#v, want explicit false`, body["status"])
	}
	if updated.Name != "renamed" || updated.Status {
		t.Errorf("updated = %+v, want name=renamed, status=false", updated)
	}
}

// TestDeleteArvanCloudMetricExporter pins the request shape of DELETE
// /domains/{domain}/metric-exporters/{id}.
func TestDeleteArvanCloudMetricExporter(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"Deleted successfully"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudMetricExporter(context.Background(), creds(), "example.com", "me-1"); err != nil {
		t.Fatalf("DeleteArvanCloudMetricExporter() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/metric-exporters/me-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/metric-exporters/me-1", records)
	}
}

// TestSetArvanCloudMetricExporterStatus pins the request shape and body of
// PATCH /domains/{domain}/metric-exporters/{id}/status.
func TestSetArvanCloudMetricExporterStatus(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"me-1","name":"exporter one","type":"access","interval":"30s","status":false}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.SetArvanCloudMetricExporterStatus(context.Background(), creds(), "example.com", "me-1", false)
	if err != nil {
		t.Fatalf("SetArvanCloudMetricExporterStatus() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/metric-exporters/me-1/status" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/metric-exporters/me-1/status", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["status"] != false {
		t.Errorf(`request body = %+v, want {"status": false}`, body)
	}
	if updated.Status {
		t.Errorf("updated.Status = true, want false")
	}
}
