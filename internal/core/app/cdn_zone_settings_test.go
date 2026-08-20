package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// The fakes below implement only the single-method local provider interface
// each cdn_zone_settings.go use case declares for itself (see that file's
// top comment for why they are not ports.ParspackProvider), so each fake
// covers exactly one use case under test.

type fakeAntivirusGetter struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	enabled                bool
	err                    error
}

func (f fakeAntivirusGetter) GetCDNAntivirusStatus(context.Context, domain.ProviderCredentials, string) (bool, error) {
	return f.enabled, f.err
}

type fakeAntivirusUpdater struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	enabled                bool
	err                    error
}

func (f fakeAntivirusUpdater) UpdateCDNAntivirusStatus(context.Context, domain.ProviderCredentials, string, bool) (bool, error) {
	return f.enabled, f.err
}

func TestGetCDNAntivirusStatusExecute(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewGetCDNAntivirusStatus(q, fakeAntivirusGetter{enabled: true})

	enabled, err := uc.Execute(context.Background(), app.GetCDNAntivirusStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !enabled {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestGetCDNAntivirusStatusExecuteMissingZoneUUID(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewGetCDNAntivirusStatus(q, fakeAntivirusGetter{})

	_, err := uc.Execute(context.Background(), app.GetCDNAntivirusStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNAntivirusStatusExecuteProviderError(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewGetCDNAntivirusStatus(q, fakeAntivirusGetter{err: domain.ErrProviderUnavailable})

	_, err := uc.Execute(context.Background(), app.GetCDNAntivirusStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}

func TestUpdateCDNAntivirusStatusExecute(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNAntivirusStatus(q, fakeAntivirusUpdater{enabled: true})

	enabled, err := uc.Execute(context.Background(), app.UpdateCDNAntivirusStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !enabled {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestUpdateCDNAntivirusStatusExecuteMissingCredentials(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNAntivirusStatus(q, fakeAntivirusUpdater{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNAntivirusStatusInput{ZoneUUID: "zone-1"})
	if err == nil {
		t.Fatal("Execute: want an error for missing credentials")
	}
}

type fakeDNSSecGetter struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	status                 *domain.CDNDNSSecStatus
	err                    error
}

func (f fakeDNSSecGetter) GetCDNDNSSecStatus(context.Context, domain.ProviderCredentials, string) (*domain.CDNDNSSecStatus, error) {
	return f.status, f.err
}

type fakeDNSSecUpdater struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	status                 *domain.CDNDNSSecStatus
	err                    error
}

func (f fakeDNSSecUpdater) UpdateCDNDNSSecStatus(context.Context, domain.ProviderCredentials, string, bool) (*domain.CDNDNSSecStatus, error) {
	return f.status, f.err
}

func TestGetCDNDNSSecStatusExecute(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewGetCDNDNSSecStatus(q, fakeDNSSecGetter{status: &domain.CDNDNSSecStatus{Enabled: true, Value: "ds-record"}})

	status, err := uc.Execute(context.Background(), app.GetCDNDNSSecStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !status.Enabled || status.Value != "ds-record" {
		t.Errorf("status = %+v, want enabled with ds-record", status)
	}
}

func TestGetCDNDNSSecStatusExecuteMissingZoneUUID(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewGetCDNDNSSecStatus(q, fakeDNSSecGetter{})

	_, err := uc.Execute(context.Background(), app.GetCDNDNSSecStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNDNSSecStatusExecute(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNDNSSecStatus(q, fakeDNSSecUpdater{status: &domain.CDNDNSSecStatus{Enabled: true, Value: "ds-record"}})

	status, err := uc.Execute(context.Background(), app.UpdateCDNDNSSecStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !status.Enabled || status.Value != "ds-record" {
		t.Errorf("status = %+v, want enabled with ds-record", status)
	}
}

func TestUpdateCDNDNSSecStatusExecuteProviderError(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNDNSSecStatus(q, fakeDNSSecUpdater{err: domain.ErrProviderUnavailable})

	_, err := uc.Execute(context.Background(), app.UpdateCDNDNSSecStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Enabled: true,
	})
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}

type fakeOptimizationGetter struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	status                 *domain.CDNOptimizationStatus
	err                    error
}

func (f fakeOptimizationGetter) GetCDNOptimizationStatus(context.Context, domain.ProviderCredentials, string) (*domain.CDNOptimizationStatus, error) {
	return f.status, f.err
}

type fakeOptimizationUpdater struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	status                 *domain.CDNOptimizationStatus
	err                    error
}

func (f fakeOptimizationUpdater) UpdateCDNOptimization(context.Context, domain.ProviderCredentials, string, domain.CDNOptimizationStatus) (*domain.CDNOptimizationStatus, error) {
	return f.status, f.err
}

func TestGetCDNOptimizationStatusExecute(t *testing.T) {
	q := &inlineQueue{}
	want := &domain.CDNOptimizationStatus{ImageMinification: true, WebPConversion: true, MinifyCSS: true}
	uc := app.NewGetCDNOptimizationStatus(q, fakeOptimizationGetter{status: want})

	status, err := uc.Execute(context.Background(), app.GetCDNOptimizationStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !status.ImageMinification || !status.WebPConversion || !status.MinifyCSS {
		t.Errorf("status = %+v, want image minification, webp conversion and minify css all true", status)
	}
}

func TestGetCDNOptimizationStatusExecuteMissingZoneUUID(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewGetCDNOptimizationStatus(q, fakeOptimizationGetter{})

	_, err := uc.Execute(context.Background(), app.GetCDNOptimizationStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNOptimizationExecute(t *testing.T) {
	q := &inlineQueue{}
	want := &domain.CDNOptimizationStatus{ImageMinification: true, MinifyJS: true}
	uc := app.NewUpdateCDNOptimization(q, fakeOptimizationUpdater{status: want})

	status, err := uc.Execute(context.Background(), app.UpdateCDNOptimizationInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
		Status: domain.CDNOptimizationStatus{ImageMinification: true, MinifyJS: true},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !status.ImageMinification || !status.MinifyJS {
		t.Errorf("status = %+v, want image minification and minify js both true", status)
	}
}

func TestUpdateCDNOptimizationExecuteProviderError(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNOptimization(q, fakeOptimizationUpdater{err: domain.ErrInvalidInput})

	_, err := uc.Execute(context.Background(), app.UpdateCDNOptimizationInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

type fakeDeveloperModeUpdater struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	enabled                bool
	err                    error
}

func (f fakeDeveloperModeUpdater) UpdateCDNDeveloperMode(context.Context, domain.ProviderCredentials, string, bool) (bool, error) {
	return f.enabled, f.err
}

func TestUpdateCDNDeveloperModeExecute(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNDeveloperMode(q, fakeDeveloperModeUpdater{enabled: true})

	enabled, err := uc.Execute(context.Background(), app.UpdateCDNDeveloperModeInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !enabled {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestUpdateCDNDeveloperModeExecuteMissingZoneUUID(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNDeveloperMode(q, fakeDeveloperModeUpdater{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNDeveloperModeInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

type fakeMaintenanceModeUpdater struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	enabled                bool
	err                    error
}

func (f fakeMaintenanceModeUpdater) UpdateCDNMaintenanceMode(context.Context, domain.ProviderCredentials, string, bool) (bool, error) {
	return f.enabled, f.err
}

func TestUpdateCDNMaintenanceModeExecute(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNMaintenanceMode(q, fakeMaintenanceModeUpdater{enabled: false})

	enabled, err := uc.Execute(context.Background(), app.UpdateCDNMaintenanceModeInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Enabled: false,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if enabled {
		t.Errorf("enabled = %v, want false", enabled)
	}
}

func TestUpdateCDNMaintenanceModeExecuteProviderError(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNMaintenanceMode(q, fakeMaintenanceModeUpdater{err: domain.ErrProviderUnavailable})

	_, err := uc.Execute(context.Background(), app.UpdateCDNMaintenanceModeInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Enabled: true,
	})
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}

type fakeQueryStringUpdater struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	enabled                bool
	err                    error
}

func (f fakeQueryStringUpdater) UpdateCDNQueryStringSetting(context.Context, domain.ProviderCredentials, string, bool) (bool, error) {
	return f.enabled, f.err
}

func TestUpdateCDNQueryStringSettingExecute(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNQueryStringSetting(q, fakeQueryStringUpdater{enabled: true})

	enabled, err := uc.Execute(context.Background(), app.UpdateCDNQueryStringSettingInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !enabled {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestUpdateCDNQueryStringSettingExecuteMissingCredentials(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNQueryStringSetting(q, fakeQueryStringUpdater{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNQueryStringSettingInput{ZoneUUID: "zone-1"})
	if err == nil {
		t.Fatal("Execute: want an error for missing credentials")
	}
}

type fakeOriginOfflineUpdater struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	enabled                bool
	err                    error
}

func (f fakeOriginOfflineUpdater) UpdateCDNOriginOffline(context.Context, domain.ProviderCredentials, string, bool) (bool, error) {
	return f.enabled, f.err
}

func TestUpdateCDNOriginOfflineExecute(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNOriginOffline(q, fakeOriginOfflineUpdater{enabled: true})

	enabled, err := uc.Execute(context.Background(), app.UpdateCDNOriginOfflineInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !enabled {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestUpdateCDNOriginOfflineExecuteMissingZoneUUID(t *testing.T) {
	q := &inlineQueue{}
	uc := app.NewUpdateCDNOriginOffline(q, fakeOriginOfflineUpdater{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNOriginOfflineInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
