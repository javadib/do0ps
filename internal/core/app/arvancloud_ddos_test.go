package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// fakeArvanCloudDdosProvider embeds the port so a test only needs to
// override the methods it actually exercises, the same pattern as
// fakeArvanCloudWafProvider.
type fakeArvanCloudDdosProvider struct {
	ports.ArvanCloudProvider

	settings        domain.ArvanCloudDdosSettings
	updatedSettings domain.ArvanCloudDdosSettings

	rules []domain.ArvanCloudDdosRule

	createdRule       domain.ArvanCloudDdosRule
	createdRuleDomain string

	updatedRuleID string
	updatedRule   domain.ArvanCloudDdosRule

	deleteRuleErr error

	reprioritizedDomain string
	reprioritizedRuleID string
	reprioritizedAfter  string
	reprioritizedBefore string
}

func (p *fakeArvanCloudDdosProvider) GetArvanCloudDdosSettings(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudDdosSettings, error) {
	settings := p.settings
	return &settings, nil
}

func (p *fakeArvanCloudDdosProvider) UpdateArvanCloudDdosSettings(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.ArvanCloudDdosSettings) (*domain.ArvanCloudDdosSettings, error) {
	p.updatedSettings = settings
	return &settings, nil
}

func (p *fakeArvanCloudDdosProvider) ListArvanCloudDdosRules(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudDdosRule, error) {
	return p.rules, nil
}

func (p *fakeArvanCloudDdosProvider) CreateArvanCloudDdosRule(_ context.Context, _ domain.ProviderCredentials, domainName string, rule domain.ArvanCloudDdosRule) (*domain.ArvanCloudDdosRule, error) {
	p.createdRule = rule
	p.createdRuleDomain = domainName
	created := rule
	created.ID = "rule-1"
	return &created, nil
}

func (p *fakeArvanCloudDdosProvider) GetArvanCloudDdosRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudDdosRule, error) {
	for i := range p.rules {
		if p.rules[i].ID == id {
			return &p.rules[i], nil
		}
	}
	return nil, fmt.Errorf("ddos rule %q: %w", id, domain.ErrNotFound)
}

func (p *fakeArvanCloudDdosProvider) UpdateArvanCloudDdosRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string, rule domain.ArvanCloudDdosRule) (*domain.ArvanCloudDdosRule, error) {
	p.updatedRuleID = id
	p.updatedRule = rule
	updated := rule
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudDdosProvider) DeleteArvanCloudDdosRule(_ context.Context, _ domain.ProviderCredentials, _ string, _ string) error {
	return p.deleteRuleErr
}

func (p *fakeArvanCloudDdosProvider) ReprioritizeArvanCloudDdosRules(_ context.Context, _ domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error {
	p.reprioritizedDomain = domainName
	p.reprioritizedRuleID = ruleID
	p.reprioritizedAfter = afterRuleID
	p.reprioritizedBefore = beforeRuleID
	return nil
}

// --- Per-domain DDoS settings ------------------------------------------------

func TestGetArvanCloudDdosSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudDdosProvider{settings: domain.ArvanCloudDdosSettings{
		IsEnabled: true, ProtectionMode: domain.ArvanCloudDdosProtectionModeCookie,
	}}
	uc := app.NewGetArvanCloudDdosSettings(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetArvanCloudDdosSettingsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if settings.ProtectionMode != domain.ArvanCloudDdosProtectionModeCookie {
		t.Errorf("settings.ProtectionMode = %q, want %q", settings.ProtectionMode, domain.ArvanCloudDdosProtectionModeCookie)
	}
}

func TestGetArvanCloudDdosSettingsMissingDomain(t *testing.T) {
	uc := app.NewGetArvanCloudDdosSettings(&inlineQueue{}, &fakeArvanCloudDdosProvider{})
	if _, err := uc.Execute(context.Background(), app.GetArvanCloudDdosSettingsInput{Credentials: validArvanCloudCreds()}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateArvanCloudDdosSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudDdosProvider{}
	uc := app.NewUpdateArvanCloudDdosSettings(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudDdosSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudDdosSettings{ProtectionMode: domain.ArvanCloudDdosProtectionModeJavaScript},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.updatedSettings.ProtectionMode != domain.ArvanCloudDdosProtectionModeJavaScript {
		t.Errorf("provider received protection_mode %q, want %q", provider.updatedSettings.ProtectionMode, domain.ArvanCloudDdosProtectionModeJavaScript)
	}
}

// TestUpdateArvanCloudDdosSettingsRejectsInvalidProtectionMode proves a mode
// outside "off"/"cookie"/"javascript"/"captcha" — including the string
// "false", which issue #67 warned might be the wire encoding but which the
// spec does not actually use here (see ArvanCloudDdosProtectionMode's doc
// comment) — is rejected client-side.
func TestUpdateArvanCloudDdosSettingsRejectsInvalidProtectionMode(t *testing.T) {
	uc := app.NewUpdateArvanCloudDdosSettings(&inlineQueue{}, &fakeArvanCloudDdosProvider{})

	for _, mode := range []string{"false", "on", "Captcha", ""} {
		t.Run(mode, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), app.UpdateArvanCloudDdosSettingsInput{
				Credentials: validArvanCloudCreds(), Domain: "example.com",
				Settings: domain.ArvanCloudDdosSettings{ProtectionMode: domain.ArvanCloudDdosProtectionMode(mode)},
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

// TestUpdateArvanCloudDdosSettingsCaptchaServiceOnlyRequiredForCaptchaMode is
// issue #67's explicit acceptance criterion: captcha_service is validated
// only when protection_mode is "captcha" — any (or no) value is accepted for
// the other three modes, but "captcha" mode requires one of
// domain.ValidArvanCloudCaptchaService's three values.
func TestUpdateArvanCloudDdosSettingsCaptchaServiceOnlyRequiredForCaptchaMode(t *testing.T) {
	tests := []struct {
		name           string
		protectionMode domain.ArvanCloudDdosProtectionMode
		captchaService domain.ArvanCloudCaptchaService
		wantErr        bool
	}{
		{"off mode, empty captcha_service is fine", domain.ArvanCloudDdosProtectionModeOff, "", false},
		{"cookie mode, empty captcha_service is fine", domain.ArvanCloudDdosProtectionModeCookie, "", false},
		{"javascript mode, garbage captcha_service is still fine (ignored)", domain.ArvanCloudDdosProtectionModeJavaScript, "not-a-real-service", false},
		{"captcha mode, empty captcha_service is rejected", domain.ArvanCloudDdosProtectionModeCaptcha, "", true},
		{"captcha mode, invalid captcha_service is rejected", domain.ArvanCloudDdosProtectionModeCaptcha, "not-a-real-service", true},
		{"captcha mode, recaptcha is accepted", domain.ArvanCloudDdosProtectionModeCaptcha, domain.ArvanCloudCaptchaServiceRecaptcha, false},
		{"captcha mode, arcaptcha is accepted", domain.ArvanCloudDdosProtectionModeCaptcha, domain.ArvanCloudCaptchaServiceArcaptcha, false},
		{"captcha mode, hcaptcha is accepted", domain.ArvanCloudDdosProtectionModeCaptcha, domain.ArvanCloudCaptchaServiceHcaptcha, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := app.NewUpdateArvanCloudDdosSettings(&inlineQueue{}, &fakeArvanCloudDdosProvider{})
			_, err := uc.Execute(context.Background(), app.UpdateArvanCloudDdosSettingsInput{
				Credentials: validArvanCloudCreds(), Domain: "example.com",
				Settings: domain.ArvanCloudDdosSettings{ProtectionMode: tc.protectionMode, CaptchaService: tc.captchaService},
			})
			if tc.wantErr && !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Execute() error = %v, want nil", err)
			}
		})
	}
}

// --- Per-domain DDoS rules ---------------------------------------------------

func TestListArvanCloudDdosRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudDdosProvider{rules: []domain.ArvanCloudDdosRule{
		{ID: "rule-1", URLPattern: "/wp-admin/**", Action: domain.ArvanCloudDdosRuleActionPassthrough},
	}}
	uc := app.NewListArvanCloudDdosRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListArvanCloudDdosRulesInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
}

func TestCreateArvanCloudDdosRuleSuccess(t *testing.T) {
	provider := &fakeArvanCloudDdosProvider{}
	uc := app.NewCreateArvanCloudDdosRule(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudDdosRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Rule: domain.ArvanCloudDdosRule{URLPattern: "/wp-admin/**", Action: domain.ArvanCloudDdosRuleActionPassthrough},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "rule-1" || provider.createdRuleDomain != "example.com" {
		t.Errorf("created = %+v, provider domain = %q, want rule-1/example.com", created, provider.createdRuleDomain)
	}
}

// TestCreateArvanCloudDdosRuleValidation covers issue #67's acceptance
// criteria: url_pattern is required, action must be one of "protect"/
// "passthrough", and sources may not exceed the spec's 20-entry maxItems.
func TestCreateArvanCloudDdosRuleValidation(t *testing.T) {
	uc := app.NewCreateArvanCloudDdosRule(&inlineQueue{}, &fakeArvanCloudDdosProvider{})

	tooManySources := make([]string, 21)
	for i := range tooManySources {
		tooManySources[i] = "203.0.113.0/32"
	}

	tests := []struct {
		name string
		in   app.CreateArvanCloudDdosRuleInput
	}{
		{"missing domain", app.CreateArvanCloudDdosRuleInput{
			Credentials: validArvanCloudCreds(),
			Rule:        domain.ArvanCloudDdosRule{URLPattern: "/x/**", Action: domain.ArvanCloudDdosRuleActionProtect},
		}},
		{"missing url_pattern", app.CreateArvanCloudDdosRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudDdosRule{Action: domain.ArvanCloudDdosRuleActionProtect},
		}},
		{"missing action", app.CreateArvanCloudDdosRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudDdosRule{URLPattern: "/x/**"},
		}},
		{"waf action does not leak into ddos's own enum", app.CreateArvanCloudDdosRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudDdosRule{URLPattern: "/x/**", Action: domain.ArvanCloudDdosRuleAction("allow")},
		}},
		{"too many sources", app.CreateArvanCloudDdosRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudDdosRule{URLPattern: "/x/**", Action: domain.ArvanCloudDdosRuleActionProtect, Sources: tooManySources},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := uc.Execute(context.Background(), tc.in); !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

func TestGetArvanCloudDdosRuleNotFound(t *testing.T) {
	provider := &fakeArvanCloudDdosProvider{}
	uc := app.NewGetArvanCloudDdosRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetArvanCloudDdosRuleInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateArvanCloudDdosRuleSuccess(t *testing.T) {
	provider := &fakeArvanCloudDdosProvider{}
	uc := app.NewUpdateArvanCloudDdosRule(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudDdosRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "rule-1",
		Rule: domain.ArvanCloudDdosRule{URLPattern: "/api/v2/**", Action: domain.ArvanCloudDdosRuleActionProtect},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.URLPattern != "/api/v2/**" || provider.updatedRuleID != "rule-1" {
		t.Errorf("updated = %+v, provider.updatedRuleID = %q, want /api/v2/**/rule-1", updated, provider.updatedRuleID)
	}
}

func TestDeleteArvanCloudDdosRuleTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudDdosProvider{deleteRuleErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudDdosRule(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudDdosRuleInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "gone"}); err != nil {
		t.Fatalf("Execute() error = %v, want nil (already-absent rule tolerated)", err)
	}
}

func TestReprioritizeArvanCloudDdosRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudDdosProvider{}
	uc := app.NewReprioritizeArvanCloudDdosRules(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudDdosRulesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", RuleID: "rule-1", AfterRuleID: "rule-2",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.reprioritizedRuleID != "rule-1" || provider.reprioritizedAfter != "rule-2" {
		t.Errorf("provider received rule_id=%q after=%q, want rule-1/rule-2", provider.reprioritizedRuleID, provider.reprioritizedAfter)
	}
}

func TestReprioritizeArvanCloudDdosRulesRejectsBothAfterAndBefore(t *testing.T) {
	uc := app.NewReprioritizeArvanCloudDdosRules(&inlineQueue{}, &fakeArvanCloudDdosProvider{})

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudDdosRulesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", RuleID: "rule-1", AfterRuleID: "rule-2", BeforeRuleID: "rule-3",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}
