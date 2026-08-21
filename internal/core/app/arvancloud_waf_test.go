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

// fakeArvanCloudWafProvider embeds the port so a test only needs to override
// the methods it actually exercises (the same pattern as
// fakeArvanCloudFirewallProvider).
type fakeArvanCloudWafProvider struct {
	ports.ArvanCloudProvider

	presets  domain.ArvanCloudWafPresetsAndPackages
	packages []domain.ArvanCloudWafPackageRule

	globalPackage domain.ArvanCloudWafPackage

	settings        domain.ArvanCloudWafSettings
	updatedSettings domain.ArvanCloudWafSettings

	reconfiguredDomain string
	reconfiguredPreset string

	reprioritizedRulesDomain string
	reprioritizedRuleID      string
	reprioritizedRuleAfter   string
	reprioritizedRuleBefore  string

	reprioritizedPackagesDomain string
	reprioritizedPackageID      string
	reprioritizedPackageAfter   string
	reprioritizedPackageBefore  string

	rules []domain.ArvanCloudWafRule

	createdRule       domain.ArvanCloudWafRule
	createdRuleDomain string

	updatedRuleID string
	updatedRule   domain.ArvanCloudWafRule

	deletedRuleID string
	deleteRuleErr error

	domainPackages []domain.ArvanCloudWafPackage

	installedPackageID     string
	installedPackageDomain string

	updatedDomainPackageID string
	updatedDomainPackage   domain.ArvanCloudWafPackage

	uninstalledPackageID string
	uninstallErr         error
}

func (p *fakeArvanCloudWafProvider) ListArvanCloudWafPresets(context.Context, domain.ProviderCredentials) (*domain.ArvanCloudWafPresetsAndPackages, error) {
	result := p.presets
	return &result, nil
}

func (p *fakeArvanCloudWafProvider) GetArvanCloudWafPackage(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudWafPackage, error) {
	pkg := p.globalPackage
	return &pkg, nil
}

func (p *fakeArvanCloudWafProvider) GetArvanCloudWafPackageRules(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudWafPackageRule, error) {
	return p.packages, nil
}

func (p *fakeArvanCloudWafProvider) GetArvanCloudWafSettings(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudWafSettings, error) {
	settings := p.settings
	return &settings, nil
}

func (p *fakeArvanCloudWafProvider) UpdateArvanCloudWafSettings(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.ArvanCloudWafSettings) (*domain.ArvanCloudWafSettings, error) {
	p.updatedSettings = settings
	return &settings, nil
}

func (p *fakeArvanCloudWafProvider) ReconfigureArvanCloudWaf(_ context.Context, _ domain.ProviderCredentials, domainName, presetID string) error {
	p.reconfiguredDomain = domainName
	p.reconfiguredPreset = presetID
	return nil
}

func (p *fakeArvanCloudWafProvider) ReprioritizeArvanCloudWafRules(_ context.Context, _ domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error {
	p.reprioritizedRulesDomain = domainName
	p.reprioritizedRuleID = ruleID
	p.reprioritizedRuleAfter = afterRuleID
	p.reprioritizedRuleBefore = beforeRuleID
	return nil
}

func (p *fakeArvanCloudWafProvider) ReprioritizeArvanCloudWafPackages(_ context.Context, _ domain.ProviderCredentials, domainName, packageID, afterPackageID, beforePackageID string) error {
	p.reprioritizedPackagesDomain = domainName
	p.reprioritizedPackageID = packageID
	p.reprioritizedPackageAfter = afterPackageID
	p.reprioritizedPackageBefore = beforePackageID
	return nil
}

func (p *fakeArvanCloudWafProvider) ListArvanCloudWafRules(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudWafRule, error) {
	return p.rules, nil
}

func (p *fakeArvanCloudWafProvider) CreateArvanCloudWafRule(_ context.Context, _ domain.ProviderCredentials, domainName string, rule domain.ArvanCloudWafRule) (*domain.ArvanCloudWafRule, error) {
	p.createdRule = rule
	p.createdRuleDomain = domainName
	created := rule
	created.ID = "rule-1"
	return &created, nil
}

func (p *fakeArvanCloudWafProvider) GetArvanCloudWafRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudWafRule, error) {
	for i := range p.rules {
		if p.rules[i].ID == id {
			return &p.rules[i], nil
		}
	}
	return nil, fmt.Errorf("waf rule %q: %w", id, domain.ErrNotFound)
}

func (p *fakeArvanCloudWafProvider) UpdateArvanCloudWafRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string, rule domain.ArvanCloudWafRule) (*domain.ArvanCloudWafRule, error) {
	p.updatedRuleID = id
	p.updatedRule = rule
	updated := rule
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudWafProvider) DeleteArvanCloudWafRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string) error {
	if p.deleteRuleErr != nil {
		return p.deleteRuleErr
	}
	p.deletedRuleID = id
	return nil
}

func (p *fakeArvanCloudWafProvider) ListArvanCloudWafDomainPackages(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudWafPackage, error) {
	return p.domainPackages, nil
}

func (p *fakeArvanCloudWafProvider) InstallArvanCloudWafPackage(_ context.Context, _ domain.ProviderCredentials, domainName, packageID string) (*domain.ArvanCloudWafPackage, error) {
	p.installedPackageDomain = domainName
	p.installedPackageID = packageID
	return &domain.ArvanCloudWafPackage{ID: packageID}, nil
}

func (p *fakeArvanCloudWafProvider) GetArvanCloudWafDomainPackage(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudWafPackage, error) {
	for i := range p.domainPackages {
		if p.domainPackages[i].ID == id {
			return &p.domainPackages[i], nil
		}
	}
	return nil, fmt.Errorf("waf package %q: %w", id, domain.ErrNotFound)
}

func (p *fakeArvanCloudWafProvider) UpdateArvanCloudWafDomainPackage(_ context.Context, _ domain.ProviderCredentials, _ string, id string, pkg domain.ArvanCloudWafPackage) (*domain.ArvanCloudWafPackage, error) {
	p.updatedDomainPackageID = id
	p.updatedDomainPackage = pkg
	updated := pkg
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudWafProvider) UninstallArvanCloudWafPackage(_ context.Context, _ domain.ProviderCredentials, _ string, id string) error {
	if p.uninstallErr != nil {
		return p.uninstallErr
	}
	p.uninstalledPackageID = id
	return nil
}

// --- Global (account-independent reference data) --------------------------

func TestListArvanCloudWafPresetsSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{presets: domain.ArvanCloudWafPresetsAndPackages{
		Presets:  []domain.ArvanCloudWafPreset{{ID: "preset-1", Name: "OWASP Basic"}},
		Packages: []domain.ArvanCloudWafPackage{{ID: "pkg-1", Name: "core-rules"}},
	}}
	uc := app.NewListArvanCloudWafPresets(&inlineQueue{}, provider)

	result, err := uc.Execute(context.Background(), app.ListArvanCloudWafPresetsInput{Credentials: validArvanCloudCreds()})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Presets) != 1 || len(result.Packages) != 1 {
		t.Errorf("result = %+v, want one preset and one package", result)
	}
}

func TestGetArvanCloudWafPackageSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{globalPackage: domain.ArvanCloudWafPackage{ID: "pkg-1", Name: "core-rules"}}
	uc := app.NewGetArvanCloudWafPackage(&inlineQueue{}, provider)

	found, err := uc.Execute(context.Background(), app.GetArvanCloudWafPackageInput{Credentials: validArvanCloudCreds(), PackageID: "pkg-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if found.ID != "pkg-1" {
		t.Errorf("found.ID = %q, want %q", found.ID, "pkg-1")
	}
}

func TestGetArvanCloudWafPackageMissingPackageID(t *testing.T) {
	uc := app.NewGetArvanCloudWafPackage(&inlineQueue{}, &fakeArvanCloudWafProvider{})
	if _, err := uc.Execute(context.Background(), app.GetArvanCloudWafPackageInput{Credentials: validArvanCloudCreds()}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetArvanCloudWafPackageRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{packages: []domain.ArvanCloudWafPackageRule{{ID: "941100", Name: "SQL Injection Attack"}}}
	uc := app.NewGetArvanCloudWafPackageRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.GetArvanCloudWafPackageRulesInput{Credentials: validArvanCloudCreds(), PackageID: "pkg-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 1 || rules[0].Name != "SQL Injection Attack" {
		t.Errorf("rules = %+v, want the one parsed entry", rules)
	}
}

// --- Per-domain WAF configuration ------------------------------------------

func TestGetArvanCloudWafSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{settings: domain.ArvanCloudWafSettings{IsEnabled: true, Mode: domain.ArvanCloudWafModeProtect}}
	uc := app.NewGetArvanCloudWafSettings(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetArvanCloudWafSettingsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if settings.Mode != domain.ArvanCloudWafModeProtect {
		t.Errorf("settings.Mode = %q, want %q", settings.Mode, domain.ArvanCloudWafModeProtect)
	}
}

func TestGetArvanCloudWafSettingsMissingDomain(t *testing.T) {
	uc := app.NewGetArvanCloudWafSettings(&inlineQueue{}, &fakeArvanCloudWafProvider{})
	if _, err := uc.Execute(context.Background(), app.GetArvanCloudWafSettingsInput{Credentials: validArvanCloudCreds()}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateArvanCloudWafSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{}
	uc := app.NewUpdateArvanCloudWafSettings(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudWafSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudWafSettings{Mode: domain.ArvanCloudWafModeOff},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.updatedSettings.Mode != domain.ArvanCloudWafModeOff {
		t.Errorf("provider received mode %q, want %q", provider.updatedSettings.Mode, domain.ArvanCloudWafModeOff)
	}
}

// TestUpdateArvanCloudWafSettingsRejectsInvalidMode proves a mode outside
// "off"/"detect"/"protect" — including the string "false", which the issue
// text warned about but the spec does not actually use here (see
// ArvanCloudWafMode's doc comment) — is rejected client-side.
func TestUpdateArvanCloudWafSettingsRejectsInvalidMode(t *testing.T) {
	uc := app.NewUpdateArvanCloudWafSettings(&inlineQueue{}, &fakeArvanCloudWafProvider{})

	for _, mode := range []string{"false", "on", "Protect", ""} {
		t.Run(mode, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), app.UpdateArvanCloudWafSettingsInput{
				Credentials: validArvanCloudCreds(), Domain: "example.com",
				Settings: domain.ArvanCloudWafSettings{Mode: domain.ArvanCloudWafMode(mode)},
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

func TestUpdateArvanCloudWafSettingsRejectsInvalidReplacementString(t *testing.T) {
	uc := app.NewUpdateArvanCloudWafSettings(&inlineQueue{}, &fakeArvanCloudWafProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudWafSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudWafSettings{
			Mode:         domain.ArvanCloudWafModeDetect,
			LogRedaction: domain.ArvanCloudWafLogRedaction{ReplacementString: "REDACTED"},
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// --- WAF actions -------------------------------------------------------

func TestReconfigureArvanCloudWafSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{}
	uc := app.NewReconfigureArvanCloudWaf(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ReconfigureArvanCloudWafInput{Credentials: validArvanCloudCreds(), Domain: "example.com", PresetID: "preset-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.reconfiguredDomain != "example.com" || provider.reconfiguredPreset != "preset-1" {
		t.Errorf("provider received domain=%q preset=%q, want example.com/preset-1", provider.reconfiguredDomain, provider.reconfiguredPreset)
	}
}

func TestReconfigureArvanCloudWafMissingPresetID(t *testing.T) {
	uc := app.NewReconfigureArvanCloudWaf(&inlineQueue{}, &fakeArvanCloudWafProvider{})
	err := uc.Execute(context.Background(), app.ReconfigureArvanCloudWafInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestReprioritizeArvanCloudWafRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{}
	uc := app.NewReprioritizeArvanCloudWafRules(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudWafRulesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", RuleID: "rule-1", AfterRuleID: "rule-2",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.reprioritizedRuleID != "rule-1" || provider.reprioritizedRuleAfter != "rule-2" {
		t.Errorf("provider received rule_id=%q after=%q, want rule-1/rule-2", provider.reprioritizedRuleID, provider.reprioritizedRuleAfter)
	}
}

func TestReprioritizeArvanCloudWafRulesRejectsBothAfterAndBefore(t *testing.T) {
	uc := app.NewReprioritizeArvanCloudWafRules(&inlineQueue{}, &fakeArvanCloudWafProvider{})

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudWafRulesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", RuleID: "rule-1", AfterRuleID: "rule-2", BeforeRuleID: "rule-3",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestReprioritizeArvanCloudWafPackagesSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{}
	uc := app.NewReprioritizeArvanCloudWafPackages(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudWafPackagesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", PackageID: "pkg-1", BeforePackageID: "pkg-2",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.reprioritizedPackageID != "pkg-1" || provider.reprioritizedPackageBefore != "pkg-2" {
		t.Errorf("provider received package_id=%q before=%q, want pkg-1/pkg-2", provider.reprioritizedPackageID, provider.reprioritizedPackageBefore)
	}
}

func TestReprioritizeArvanCloudWafPackagesRejectsBothAfterAndBefore(t *testing.T) {
	uc := app.NewReprioritizeArvanCloudWafPackages(&inlineQueue{}, &fakeArvanCloudWafProvider{})

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudWafPackagesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", PackageID: "pkg-1", AfterPackageID: "pkg-2", BeforePackageID: "pkg-3",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestReprioritizeArvanCloudWafPackagesMissingPackageID(t *testing.T) {
	uc := app.NewReprioritizeArvanCloudWafPackages(&inlineQueue{}, &fakeArvanCloudWafProvider{})

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudWafPackagesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// --- Per-domain WAF custom rules --------------------------------------------

func TestListArvanCloudWafRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{rules: []domain.ArvanCloudWafRule{
		{ID: "rule-1", URLPattern: "/wp-admin/**", Action: domain.ArvanCloudWafRuleActionPassthrough},
	}}
	uc := app.NewListArvanCloudWafRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListArvanCloudWafRulesInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
}

func TestCreateArvanCloudWafRuleSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{}
	uc := app.NewCreateArvanCloudWafRule(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudWafRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Rule: domain.ArvanCloudWafRule{URLPattern: "/wp-admin/**", Action: domain.ArvanCloudWafRuleActionPassthrough},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "rule-1" || provider.createdRuleDomain != "example.com" {
		t.Errorf("created = %+v, provider domain = %q, want rule-1/example.com", created, provider.createdRuleDomain)
	}
}

// TestCreateArvanCloudWafRuleValidation covers issue #66's acceptance
// criteria: url_pattern is required, action must be one of "protect"/
// "passthrough" (rejecting the CDN edge Firewall's own action values), and
// sources may not exceed the spec's 20-entry maxItems.
func TestCreateArvanCloudWafRuleValidation(t *testing.T) {
	uc := app.NewCreateArvanCloudWafRule(&inlineQueue{}, &fakeArvanCloudWafProvider{})

	tooManySources := make([]string, 21)
	for i := range tooManySources {
		tooManySources[i] = "203.0.113.0/32"
	}

	tests := []struct {
		name string
		in   app.CreateArvanCloudWafRuleInput
	}{
		{"missing domain", app.CreateArvanCloudWafRuleInput{
			Credentials: validArvanCloudCreds(),
			Rule:        domain.ArvanCloudWafRule{URLPattern: "/x/**", Action: domain.ArvanCloudWafRuleActionProtect},
		}},
		{"missing url_pattern", app.CreateArvanCloudWafRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudWafRule{Action: domain.ArvanCloudWafRuleActionProtect},
		}},
		{"missing action", app.CreateArvanCloudWafRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudWafRule{URLPattern: "/x/**"},
		}},
		{"firewall action does not leak into waf's narrower enum", app.CreateArvanCloudWafRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudWafRule{URLPattern: "/x/**", Action: domain.ArvanCloudWafRuleAction("allow")},
		}},
		{"too many sources", app.CreateArvanCloudWafRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudWafRule{URLPattern: "/x/**", Action: domain.ArvanCloudWafRuleActionProtect, Sources: tooManySources},
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

func TestGetArvanCloudWafRuleNotFound(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{}
	uc := app.NewGetArvanCloudWafRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetArvanCloudWafRuleInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateArvanCloudWafRuleSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{}
	uc := app.NewUpdateArvanCloudWafRule(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudWafRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "rule-1",
		Rule: domain.ArvanCloudWafRule{URLPattern: "/api/v2/**", Action: domain.ArvanCloudWafRuleActionProtect},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.URLPattern != "/api/v2/**" || provider.updatedRuleID != "rule-1" {
		t.Errorf("updated = %+v, provider.updatedRuleID = %q, want /api/v2/**/rule-1", updated, provider.updatedRuleID)
	}
}

func TestDeleteArvanCloudWafRuleTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{deleteRuleErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudWafRule(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudWafRuleInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "gone"}); err != nil {
		t.Fatalf("Execute() error = %v, want nil (already-absent rule tolerated)", err)
	}
}

// --- Per-domain WAF package subscriptions -----------------------------------

func TestListArvanCloudWafDomainPackagesSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{domainPackages: []domain.ArvanCloudWafPackage{{ID: "pkg-1", Name: "core-rules"}}}
	uc := app.NewListArvanCloudWafDomainPackages(&inlineQueue{}, provider)

	packages, err := uc.Execute(context.Background(), app.ListArvanCloudWafDomainPackagesInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(packages) != 1 {
		t.Fatalf("len(packages) = %d, want 1", len(packages))
	}
}

func TestInstallArvanCloudWafPackageSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{}
	uc := app.NewInstallArvanCloudWafPackage(&inlineQueue{}, provider)

	installed, err := uc.Execute(context.Background(), app.InstallArvanCloudWafPackageInput{Credentials: validArvanCloudCreds(), Domain: "example.com", PackageID: "pkg-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if installed.ID != "pkg-1" || provider.installedPackageDomain != "example.com" {
		t.Errorf("installed = %+v, provider domain = %q, want pkg-1/example.com", installed, provider.installedPackageDomain)
	}
}

func TestInstallArvanCloudWafPackageMissingPackageID(t *testing.T) {
	uc := app.NewInstallArvanCloudWafPackage(&inlineQueue{}, &fakeArvanCloudWafProvider{})
	_, err := uc.Execute(context.Background(), app.InstallArvanCloudWafPackageInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetArvanCloudWafDomainPackageNotFound(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{}
	uc := app.NewGetArvanCloudWafDomainPackage(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetArvanCloudWafDomainPackageInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateArvanCloudWafDomainPackageSuccess(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{}
	uc := app.NewUpdateArvanCloudWafDomainPackage(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudWafDomainPackageInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "pkg-1",
		Package: domain.ArvanCloudWafPackage{IsEnabled: false, DisabledRules: []string{"941100"}},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.ID != "pkg-1" || provider.updatedDomainPackageID != "pkg-1" || provider.updatedDomainPackage.IsEnabled {
		t.Errorf("updated = %+v, provider received %+v, want id pkg-1 and IsEnabled false", updated, provider.updatedDomainPackage)
	}
}

func TestUninstallArvanCloudWafPackageTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudWafProvider{uninstallErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewUninstallArvanCloudWafPackage(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.UninstallArvanCloudWafPackageInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "gone"}); err != nil {
		t.Fatalf("Execute() error = %v, want nil (already-uninstalled package tolerated)", err)
	}
}
