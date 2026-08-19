package parspack_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestGetCDNAntivirusStatusSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/settings/get-antivirus-status" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/settings/get-antivirus-status", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"enabled":true}}`))
	})

	enabled, err := c.GetCDNAntivirusStatus(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNAntivirusStatus: %v", err)
	}
	if !enabled {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestGetCDNAntivirusStatusNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNAntivirusStatus(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNAntivirusStatusSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/settings/update-antivirus-status" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/settings/update-antivirus-status", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	enabled, err := c.UpdateCDNAntivirusStatus(context.Background(), creds, "zone-1", true)
	if err != nil {
		t.Fatalf("UpdateCDNAntivirusStatus: %v", err)
	}
	if !enabled {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestUpdateCDNAntivirusStatusValidationError(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The enabled parameter is required.","errors":{"enabled":["The enabled field is required."]}}`))
	})

	_, err := c.UpdateCDNAntivirusStatus(context.Background(), creds, "zone-1", true)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNDNSSecStatusSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/dns-sec" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/dns-sec", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{"enabled":false,"value":null}}`))
	})

	status, err := c.GetCDNDNSSecStatus(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNDNSSecStatus: %v", err)
	}
	if status.Enabled || status.Value != "" {
		t.Errorf("status = %+v, want disabled with empty value", status)
	}
}

func TestGetCDNDNSSecStatusUnauthorized(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthenticated."}`))
	})

	_, err := c.GetCDNDNSSecStatus(context.Background(), creds, "zone-1")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestUpdateCDNDNSSecStatusSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":{"value":"example.com. 3600 IN DS 12345 8 2 ABCD"}}`))
	})

	status, err := c.UpdateCDNDNSSecStatus(context.Background(), creds, "zone-1", true)
	if err != nil {
		t.Fatalf("UpdateCDNDNSSecStatus: %v", err)
	}
	if !status.Enabled || status.Value != "example.com. 3600 IN DS 12345 8 2 ABCD" {
		t.Errorf("status = %+v, want enabled with a DS record value", status)
	}
}

func TestUpdateCDNDNSSecStatusOperationFail(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(424)
		_, _ = w.Write([]byte(`{"success":false,"message":"Operation failed!","errors":[]}`))
	})

	_, err := c.UpdateCDNDNSSecStatus(context.Background(), creds, "zone-1", true)
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}

func TestGetCDNOptimizationStatusSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/optimization/status" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/optimization/status", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"image_minification_status":true,"webp_conversion_status":true,
			"website_minification":{"html":false,"css":true,"js":false}
		}}`))
	})

	status, err := c.GetCDNOptimizationStatus(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNOptimizationStatus: %v", err)
	}
	if !status.ImageMinification || !status.WebPConversion {
		t.Errorf("status = %+v, want image minification and webp conversion both true", status)
	}
	if status.MinifyHTML || !status.MinifyCSS || status.MinifyJS {
		t.Errorf("status = %+v, want minify html=false css=true js=false", status)
	}
}

func TestGetCDNOptimizationStatusNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNOptimizationStatus(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNOptimizationSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/optimization/update" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/optimization/update", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	status, err := c.UpdateCDNOptimization(context.Background(), creds, "zone-1", domain.CDNOptimizationStatus{
		ImageMinification: true, WebPConversion: true, MinifyHTML: true, MinifyCSS: true, MinifyJS: true,
	})
	if err != nil {
		t.Fatalf("UpdateCDNOptimization: %v", err)
	}
	if !status.ImageMinification || !status.MinifyJS {
		t.Errorf("status = %+v, want the applied configuration echoed back", status)
	}
}

func TestUpdateCDNOptimizationValidationError(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The website_minification parameter is required.","errors":{}}`))
	})

	_, err := c.UpdateCDNOptimization(context.Background(), creds, "zone-1", domain.CDNOptimizationStatus{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNDeveloperModeSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/settings/developer-mode" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/settings/developer-mode", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	enabled, err := c.UpdateCDNDeveloperMode(context.Background(), creds, "zone-1", true)
	if err != nil {
		t.Fatalf("UpdateCDNDeveloperMode: %v", err)
	}
	if !enabled {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestUpdateCDNDeveloperModeNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.UpdateCDNDeveloperMode(context.Background(), creds, "missing", true)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNMaintenanceModeSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/settings/maintenance-mode" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/settings/maintenance-mode", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	enabled, err := c.UpdateCDNMaintenanceMode(context.Background(), creds, "zone-1", false)
	if err != nil {
		t.Fatalf("UpdateCDNMaintenanceMode: %v", err)
	}
	if enabled {
		t.Errorf("enabled = %v, want false", enabled)
	}
}

func TestUpdateCDNMaintenanceModeValidationError(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The enabled parameter is required.","errors":{}}`))
	})

	_, err := c.UpdateCDNMaintenanceMode(context.Background(), creds, "zone-1", true)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNQueryStringSettingSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/settings/query-string" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/settings/query-string", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	enabled, err := c.UpdateCDNQueryStringSetting(context.Background(), creds, "zone-1", true)
	if err != nil {
		t.Fatalf("UpdateCDNQueryStringSetting: %v", err)
	}
	if !enabled {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestUpdateCDNQueryStringSettingForbidden(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"success":false,"message":"Permission Denied","errors":[]}`))
	})

	_, err := c.UpdateCDNQueryStringSetting(context.Background(), creds, "zone-1", true)
	if err == nil {
		t.Fatal("UpdateCDNQueryStringSetting: want an error for 403 Forbidden")
	}
}

func TestUpdateCDNOriginOfflineSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/settings/origin-offline" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/settings/origin-offline", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"Operation finished successfully.","data":[]}`))
	})

	enabled, err := c.UpdateCDNOriginOffline(context.Background(), creds, "zone-1", true)
	if err != nil {
		t.Fatalf("UpdateCDNOriginOffline: %v", err)
	}
	if !enabled {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestUpdateCDNOriginOfflineServerError(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"Server Error"}`))
	})

	_, err := c.UpdateCDNOriginOffline(context.Background(), creds, "zone-1", true)
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}
