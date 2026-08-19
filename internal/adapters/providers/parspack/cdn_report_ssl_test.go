package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestGetCDNAccessLogSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/report/access-log" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/report/access-log", got)
		}
		if got := r.URL.Query().Get("wcdn_state"); got != "miss" {
			t.Errorf("wcdn_state query = %q, want miss", got)
		}
		if got := r.URL.Query().Get("step"); got != "25" {
			t.Errorf("step query = %q, want 25", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"records":[{"_id":"rec-1","date":"1714290509","status_code":200,"uri":"/","method":"GET","host":"example.local"}],
			"meta":{"current_page":1,"last_page":3,"per_page":25,"total":60}
		}}`))
	})

	page, err := c.GetCDNAccessLog(context.Background(), creds, "zone-1", domain.CDNLogQuery{Step: 25, WCDNState: "miss"})
	if err != nil {
		t.Fatalf("GetCDNAccessLog: %v", err)
	}
	if len(page.Records) != 1 || page.Records[0].ID != "rec-1" || page.Records[0].StatusCode != 200 {
		t.Errorf("records = %+v, want a single 200 record with id rec-1", page.Records)
	}
	if page.Meta.LastPage != 3 || page.Meta.Total != 60 {
		t.Errorf("meta = %+v, want last_page 3 and total 60", page.Meta)
	}
}

func TestGetCDNSecurityLogSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/report/security-log" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/report/security-log", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"records":[{"_id":"rec-2","status_code":403,"security_message":"Access denied by WAF protection!","security_type":"modsec_waf"}],
			"meta":{"current_page":1,"last_page":1,"per_page":10,"total":1}
		}}`))
	})

	page, err := c.GetCDNSecurityLog(context.Background(), creds, "zone-1", domain.CDNLogQuery{})
	if err != nil {
		t.Fatalf("GetCDNSecurityLog: %v", err)
	}
	if len(page.Records) != 1 || page.Records[0].SecurityType != "modsec_waf" {
		t.Errorf("records = %+v, want a single modsec_waf record", page.Records)
	}
}

func TestGetCDNErrorLogSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/report/error-log" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/report/error-log", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"records":[{"_id":"rec-3","status_code":499,"error_type":"upstream"}],
			"meta":{"current_page":1,"last_page":1,"per_page":10,"total":1}
		}}`))
	})

	page, err := c.GetCDNErrorLog(context.Background(), creds, "zone-1", domain.CDNLogQuery{})
	if err != nil {
		t.Fatalf("GetCDNErrorLog: %v", err)
	}
	if len(page.Records) != 1 || page.Records[0].ErrorType != "upstream" {
		t.Errorf("records = %+v, want a single upstream record", page.Records)
	}
}

func TestGetCDNWAFLogSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/report/waf-log" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/report/waf-log", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"records":[{"_id":"rec-4","status_code":403,"additional_logs":[{"message":"GET or HEAD Request with Body Content","details":{"match":"Matched","data":"2"}}]}],
			"meta":{"current_page":1,"last_page":1,"per_page":10,"total":1}
		}}`))
	})

	page, err := c.GetCDNWAFLog(context.Background(), creds, "zone-1", domain.CDNLogQuery{})
	if err != nil {
		t.Fatalf("GetCDNWAFLog: %v", err)
	}
	if len(page.Records) != 1 || len(page.Records[0].AdditionalLogs) != 1 {
		t.Fatalf("records = %+v, want a single record with one additional log", page.Records)
	}
	if page.Records[0].AdditionalLogs[0].Match != "Matched" {
		t.Errorf("match = %q, want Matched", page.Records[0].AdditionalLogs[0].Match)
	}
}

func TestGetCDNTopVisitorsSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/analytics/top-visitors" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/analytics/top-visitors", got)
		}
		if got := r.URL.Query().Get("start"); got != "2023-05-20" {
			t.Errorf("start query = %q, want 2023-05-20", got)
		}
		if got := r.URL.Query().Get("end"); got != "2023-05-25" {
			t.Errorf("end query = %q, want 2023-05-25", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"ip":"185.8.172.10","count":1055},{"ip":"185.13.230.46","count":278}
		]}`))
	})

	visitors, err := c.GetCDNTopVisitors(context.Background(), creds, "zone-1", "2023-05-20", "2023-05-25")
	if err != nil {
		t.Fatalf("GetCDNTopVisitors: %v", err)
	}
	if len(visitors) != 2 || visitors[0].IP != "185.8.172.10" || visitors[0].Count != 1055 {
		t.Errorf("visitors = %+v, want a first entry of 185.8.172.10 with count 1055", visitors)
	}
}

func TestGetCDNMonthlyTrafficUsageSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/analytics/monthly-traffic-usage" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/analytics/monthly-traffic-usage", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"receive":12345,"traffic_limit":"53687091200"}}`))
	})

	usage, err := c.GetCDNMonthlyTrafficUsage(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNMonthlyTrafficUsage: %v", err)
	}
	if usage.ReceivedBytes != 12345 || usage.TrafficLimit != "53687091200" {
		t.Errorf("usage = %+v, want receive 12345 and limit 53687091200", usage)
	}
}

func TestGetCDNMinTLSVersionSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/ssl/min-tls-version" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/ssl/min-tls-version", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"min_tls_version":"1.2"}}`))
	})

	version, err := c.GetCDNMinTLSVersion(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNMinTLSVersion: %v", err)
	}
	if version != domain.CDNMinTLSVersion12 {
		t.Errorf("version = %q, want 1.2", version)
	}
}

func TestUpdateCDNMinTLSVersionSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["min_tls_version"] != "1.3" {
			t.Errorf("body = %+v, want min_tls_version 1.3", body)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.UpdateCDNMinTLSVersion(context.Background(), creds, "zone-1", domain.CDNMinTLSVersion13); err != nil {
		t.Fatalf("UpdateCDNMinTLSVersion: %v", err)
	}
}

func TestListCDNCertificatesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/ssl/certificates" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/ssl/certificates", got)
		}
		if got := r.URL.Query().Get("domain_filter"); got != "api" {
			t.Errorf("domain_filter query = %q, want api", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"domain":"example.com","status":"ok","ssl_type":"letsencrypt","ssl":{"letsencrypt":{"expiration_time":"2023-05-02 13:33:23","active":true,"credentials":{"certificate":"Y2VydA==","private_key":"a2V5","ca_bundle":"Y2E="}},"custom":null}}
		]}`))
	})

	certs, err := c.ListCDNCertificates(context.Background(), creds, "zone-1", 0, 0, "api")
	if err != nil {
		t.Fatalf("ListCDNCertificates: %v", err)
	}
	if len(certs) != 1 || certs[0].Domain != "example.com" {
		t.Fatalf("certs = %+v, want a single example.com certificate", certs)
	}
	if certs[0].LetsEncrypt == nil || !certs[0].LetsEncrypt.Active || certs[0].LetsEncrypt.Certificate != "Y2VydA==" {
		t.Errorf("LetsEncrypt = %+v, want an active cert with base64 certificate Y2VydA==", certs[0].LetsEncrypt)
	}
	if certs[0].Custom != nil {
		t.Errorf("Custom = %+v, want nil", certs[0].Custom)
	}
}

func TestGetCDNHSTSSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/hsts" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/hsts", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"enabled":false,"max_age":60}}`))
	})

	settings, err := c.GetCDNHSTS(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNHSTS: %v", err)
	}
	if settings.Enabled || settings.MaxAgeSeconds != 60 {
		t.Errorf("settings = %+v, want disabled with max_age 60", settings)
	}
}

func TestUpdateCDNHSTSSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["enabled"] != true || body["max_age"] != float64(3600) {
			t.Errorf("body = %+v, want enabled true and max_age 3600", body)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	err := c.UpdateCDNHSTS(context.Background(), creds, "zone-1", domain.CDNHSTSSettings{Enabled: true, MaxAgeSeconds: 3600})
	if err != nil {
		t.Fatalf("UpdateCDNHSTS: %v", err)
	}
}

func TestGetCDNAccessLogNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNAccessLog(context.Background(), creds, "missing", domain.CDNLogQuery{})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}
