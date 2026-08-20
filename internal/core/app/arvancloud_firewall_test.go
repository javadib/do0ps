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

// fakeArvanCloudFirewallProvider embeds the port so a test only needs to
// override the methods it actually exercises (the same pattern as
// fakeArvanCloudListsProvider).
type fakeArvanCloudFirewallProvider struct {
	ports.ArvanCloudProvider

	settings        domain.ArvanCloudFirewallSettings
	updatedSettings domain.ArvanCloudFirewallSettings

	rules []domain.ArvanCloudFirewallRule

	createdRule       domain.ArvanCloudFirewallRule
	createdRuleDomain string

	updatedRuleID string
	updatedRule   domain.ArvanCloudFirewallRule

	deletedRuleID string
	deleteErr     error

	reprioritizedDomain string
	reprioritizedRuleID string
	reprioritizedAfter  string
	reprioritizedBefore string

	accountRules       []domain.ArvanCloudAccountFirewallRule
	validDomains       []domain.ArvanCloudAccountFirewallValidDomain
	createdAccountRule domain.ArvanCloudAccountFirewallRule

	updatedAccountRuleID string
	updatedAccountRule   domain.ArvanCloudAccountFirewallRule

	deletedAccountRuleID string
	deleteAccountErr     error

	attachedID        string
	attachedDomainIDs []string

	detachedID        string
	detachedDomainIDs []string
}

func (p *fakeArvanCloudFirewallProvider) GetArvanCloudFirewallSettings(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudFirewallSettings, error) {
	settings := p.settings
	return &settings, nil
}

func (p *fakeArvanCloudFirewallProvider) UpdateArvanCloudFirewallSettings(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.ArvanCloudFirewallSettings) (*domain.ArvanCloudFirewallSettings, error) {
	p.updatedSettings = settings
	return &settings, nil
}

func (p *fakeArvanCloudFirewallProvider) ListArvanCloudFirewallRules(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudFirewallRule, error) {
	return p.rules, nil
}

func (p *fakeArvanCloudFirewallProvider) CreateArvanCloudFirewallRule(_ context.Context, _ domain.ProviderCredentials, domainName string, rule domain.ArvanCloudFirewallRule) (*domain.ArvanCloudFirewallRule, error) {
	p.createdRule = rule
	p.createdRuleDomain = domainName
	created := rule
	created.ID = "rule-1"
	return &created, nil
}

func (p *fakeArvanCloudFirewallProvider) GetArvanCloudFirewallRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudFirewallRule, error) {
	for i := range p.rules {
		if p.rules[i].ID == id {
			return &p.rules[i], nil
		}
	}
	return nil, fmt.Errorf("firewall rule %q: %w", id, domain.ErrNotFound)
}

func (p *fakeArvanCloudFirewallProvider) UpdateArvanCloudFirewallRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string, rule domain.ArvanCloudFirewallRule) (*domain.ArvanCloudFirewallRule, error) {
	p.updatedRuleID = id
	p.updatedRule = rule
	updated := rule
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudFirewallProvider) DeleteArvanCloudFirewallRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedRuleID = id
	return nil
}

func (p *fakeArvanCloudFirewallProvider) ReprioritizeArvanCloudFirewallRules(_ context.Context, _ domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error {
	p.reprioritizedDomain = domainName
	p.reprioritizedRuleID = ruleID
	p.reprioritizedAfter = afterRuleID
	p.reprioritizedBefore = beforeRuleID
	return nil
}

func (p *fakeArvanCloudFirewallProvider) ListArvanCloudAccountFirewallValidDomains(context.Context, domain.ProviderCredentials) ([]domain.ArvanCloudAccountFirewallValidDomain, error) {
	return p.validDomains, nil
}

func (p *fakeArvanCloudFirewallProvider) ListArvanCloudAccountFirewallRules(context.Context, domain.ProviderCredentials) ([]domain.ArvanCloudAccountFirewallRule, error) {
	return p.accountRules, nil
}

func (p *fakeArvanCloudFirewallProvider) CreateArvanCloudAccountFirewallRule(_ context.Context, _ domain.ProviderCredentials, rule domain.ArvanCloudAccountFirewallRule) (*domain.ArvanCloudAccountFirewallRule, error) {
	p.createdAccountRule = rule
	created := rule
	created.ID = "rule-1"
	return &created, nil
}

func (p *fakeArvanCloudFirewallProvider) GetArvanCloudAccountFirewallRule(_ context.Context, _ domain.ProviderCredentials, id string) (*domain.ArvanCloudAccountFirewallRule, error) {
	for i := range p.accountRules {
		if p.accountRules[i].ID == id {
			return &p.accountRules[i], nil
		}
	}
	return nil, fmt.Errorf("account firewall rule %q: %w", id, domain.ErrNotFound)
}

func (p *fakeArvanCloudFirewallProvider) UpdateArvanCloudAccountFirewallRule(_ context.Context, _ domain.ProviderCredentials, id string, rule domain.ArvanCloudAccountFirewallRule) (*domain.ArvanCloudAccountFirewallRule, error) {
	p.updatedAccountRuleID = id
	p.updatedAccountRule = rule
	updated := rule
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudFirewallProvider) DeleteArvanCloudAccountFirewallRule(_ context.Context, _ domain.ProviderCredentials, id string) error {
	if p.deleteAccountErr != nil {
		return p.deleteAccountErr
	}
	p.deletedAccountRuleID = id
	return nil
}

func (p *fakeArvanCloudFirewallProvider) AttachArvanCloudAccountFirewallDomains(_ context.Context, _ domain.ProviderCredentials, id string, domainIDs []string) (*domain.ArvanCloudAccountFirewallRule, error) {
	p.attachedID = id
	p.attachedDomainIDs = domainIDs
	return &domain.ArvanCloudAccountFirewallRule{ID: id, DomainIDs: domainIDs}, nil
}

func (p *fakeArvanCloudFirewallProvider) DetachArvanCloudAccountFirewallDomains(_ context.Context, _ domain.ProviderCredentials, id string, domainIDs []string) (*domain.ArvanCloudAccountFirewallRule, error) {
	p.detachedID = id
	p.detachedDomainIDs = domainIDs
	return &domain.ArvanCloudAccountFirewallRule{ID: id}, nil
}

func (p *fakeArvanCloudFirewallProvider) ReprioritizeArvanCloudAccountFirewallRules(_ context.Context, _ domain.ProviderCredentials, ruleID, afterRuleID, beforeRuleID string) error {
	p.reprioritizedRuleID = ruleID
	p.reprioritizedAfter = afterRuleID
	p.reprioritizedBefore = beforeRuleID
	return nil
}

// --- Firewall Settings (domain-level) --------------------------------------

func TestGetArvanCloudFirewallSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{settings: domain.ArvanCloudFirewallSettings{
		IsEnabled: true, DefaultAction: domain.ArvanCloudFirewallDefaultActionDeny,
	}}
	uc := app.NewGetArvanCloudFirewallSettings(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetArvanCloudFirewallSettingsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if settings.DefaultAction != domain.ArvanCloudFirewallDefaultActionDeny {
		t.Errorf("settings.DefaultAction = %q, want %q", settings.DefaultAction, domain.ArvanCloudFirewallDefaultActionDeny)
	}
}

func TestGetArvanCloudFirewallSettingsMissingDomain(t *testing.T) {
	uc := app.NewGetArvanCloudFirewallSettings(&inlineQueue{}, &fakeArvanCloudFirewallProvider{})
	if _, err := uc.Execute(context.Background(), app.GetArvanCloudFirewallSettingsInput{Credentials: validArvanCloudCreds()}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateArvanCloudFirewallSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewUpdateArvanCloudFirewallSettings(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudFirewallSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudFirewallSettings{DefaultAction: domain.ArvanCloudFirewallDefaultActionDrop},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.updatedSettings.DefaultAction != domain.ArvanCloudFirewallDefaultActionDrop {
		t.Errorf("provider received default_action %q, want %q", provider.updatedSettings.DefaultAction, domain.ArvanCloudFirewallDefaultActionDrop)
	}
}

// TestUpdateArvanCloudFirewallSettingsAcceptsDrop proves "drop" — invalid
// for a per-rule action — is accepted as a settings default_action (issue
// #65's acceptance criteria).
func TestUpdateArvanCloudFirewallSettingsAcceptsDrop(t *testing.T) {
	uc := app.NewUpdateArvanCloudFirewallSettings(&inlineQueue{}, &fakeArvanCloudFirewallProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudFirewallSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudFirewallSettings{DefaultAction: domain.ArvanCloudFirewallDefaultActionDrop},
	})
	if err != nil {
		t.Errorf(`Execute() error = %v, want nil ("drop" is a valid default_action)`, err)
	}
}

func TestUpdateArvanCloudFirewallSettingsRejectsInvalidDefaultAction(t *testing.T) {
	uc := app.NewUpdateArvanCloudFirewallSettings(&inlineQueue{}, &fakeArvanCloudFirewallProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudFirewallSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudFirewallSettings{DefaultAction: "block"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// --- Firewall Rules (domain-level) ------------------------------------------

func TestListArvanCloudFirewallRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{rules: []domain.ArvanCloudFirewallRule{
		{ID: "rule-1", Name: "block-ir", Action: domain.ArvanCloudFirewallActionDeny},
	}}
	uc := app.NewListArvanCloudFirewallRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListArvanCloudFirewallRulesInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
}

func TestCreateArvanCloudFirewallRuleSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewCreateArvanCloudFirewallRule(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudFirewallRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Rule: domain.ArvanCloudFirewallRule{Name: "block-ir", FilterExpr: `ip.geoip.country in {"IR"}`, Action: domain.ArvanCloudFirewallActionDeny},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "rule-1" || provider.createdRuleDomain != "example.com" {
		t.Errorf("created = %+v, provider domain = %q, want rule-1/example.com", created, provider.createdRuleDomain)
	}
}

func TestCreateArvanCloudFirewallRuleValidation(t *testing.T) {
	uc := app.NewCreateArvanCloudFirewallRule(&inlineQueue{}, &fakeArvanCloudFirewallProvider{})

	tests := []struct {
		name string
		in   app.CreateArvanCloudFirewallRuleInput
	}{
		{"missing domain", app.CreateArvanCloudFirewallRuleInput{
			Credentials: validArvanCloudCreds(),
			Rule:        domain.ArvanCloudFirewallRule{Name: "x", FilterExpr: "ssl", Action: domain.ArvanCloudFirewallActionAllow},
		}},
		{"missing name", app.CreateArvanCloudFirewallRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudFirewallRule{FilterExpr: "ssl", Action: domain.ArvanCloudFirewallActionAllow},
		}},
		{"missing filter_expr", app.CreateArvanCloudFirewallRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudFirewallRule{Name: "x", Action: domain.ArvanCloudFirewallActionAllow},
		}},
		{"missing action", app.CreateArvanCloudFirewallRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudFirewallRule{Name: "x", FilterExpr: "ssl"},
		}},
		{"drop is not a valid per-rule action", app.CreateArvanCloudFirewallRuleInput{
			Credentials: validArvanCloudCreds(), Domain: "example.com",
			Rule: domain.ArvanCloudFirewallRule{Name: "x", FilterExpr: "ssl", Action: domain.ArvanCloudFirewallAction("drop")},
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

func TestGetArvanCloudFirewallRuleNotFound(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewGetArvanCloudFirewallRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetArvanCloudFirewallRuleInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateArvanCloudFirewallRuleSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewUpdateArvanCloudFirewallRule(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudFirewallRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "rule-1",
		Rule: domain.ArvanCloudFirewallRule{Name: "block-ir-th", FilterExpr: `ip.geoip.country in {"IR" "TH"}`, Action: domain.ArvanCloudFirewallActionDeny},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.Name != "block-ir-th" || provider.updatedRuleID != "rule-1" {
		t.Errorf("updated = %+v, provider.updatedRuleID = %q, want block-ir-th/rule-1", updated, provider.updatedRuleID)
	}
}

func TestDeleteArvanCloudFirewallRuleTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{deleteErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudFirewallRule(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudFirewallRuleInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "gone"}); err != nil {
		t.Fatalf("Execute() error = %v, want nil (already-absent rule tolerated)", err)
	}
}

func TestReprioritizeArvanCloudFirewallRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewReprioritizeArvanCloudFirewallRules(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudFirewallRulesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", RuleID: "rule-1", AfterRuleID: "rule-2",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.reprioritizedRuleID != "rule-1" || provider.reprioritizedAfter != "rule-2" {
		t.Errorf("provider received rule_id=%q after=%q, want rule-1/rule-2", provider.reprioritizedRuleID, provider.reprioritizedAfter)
	}
}

func TestReprioritizeArvanCloudFirewallRulesRejectsBothAfterAndBefore(t *testing.T) {
	uc := app.NewReprioritizeArvanCloudFirewallRules(&inlineQueue{}, &fakeArvanCloudFirewallProvider{})

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudFirewallRulesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", RuleID: "rule-1", AfterRuleID: "rule-2", BeforeRuleID: "rule-3",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// --- Firewall Rules (account-level) -----------------------------------------

func TestListArvanCloudAccountFirewallValidDomainsSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{validDomains: []domain.ArvanCloudAccountFirewallValidDomain{
		{ID: "domain-uuid-1", Name: "example.com"},
	}}
	uc := app.NewListArvanCloudAccountFirewallValidDomains(&inlineQueue{}, provider)

	domains, err := uc.Execute(context.Background(), app.ListArvanCloudAccountFirewallValidDomainsInput{Credentials: validArvanCloudCreds()})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(domains) != 1 {
		t.Fatalf("len(domains) = %d, want 1", len(domains))
	}
}

func TestListArvanCloudAccountFirewallRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{accountRules: []domain.ArvanCloudAccountFirewallRule{
		{ID: "rule-1", Name: "block-ir-everywhere"},
	}}
	uc := app.NewListArvanCloudAccountFirewallRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListArvanCloudAccountFirewallRulesInput{Credentials: validArvanCloudCreds()})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
}

func TestCreateArvanCloudAccountFirewallRuleSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewCreateArvanCloudAccountFirewallRule(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudAccountFirewallRuleInput{
		Credentials: validArvanCloudCreds(),
		Rule: domain.ArvanCloudAccountFirewallRule{
			Name: "block-scanners", FilterExpr: `ip.src in {"203.0.113.5"}`, Action: domain.ArvanCloudFirewallActionDeny,
			DomainSelectionType: domain.ArvanCloudDomainSelectionAll,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "rule-1" {
		t.Errorf("created.ID = %q, want %q", created.ID, "rule-1")
	}
}

// TestCreateArvanCloudAccountFirewallRuleRejectsEmptyDomainIDsForInclude is
// issue #65's headline acceptance criterion: domain_selection_type=
// "include" with empty domain_ids must be rejected client-side, before the
// provider is ever called.
func TestCreateArvanCloudAccountFirewallRuleRejectsEmptyDomainIDsForInclude(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewCreateArvanCloudAccountFirewallRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudAccountFirewallRuleInput{
		Credentials: validArvanCloudCreds(),
		Rule: domain.ArvanCloudAccountFirewallRule{
			Name: "x", FilterExpr: "ssl", Action: domain.ArvanCloudFirewallActionDeny,
			DomainSelectionType: domain.ArvanCloudDomainSelectionInclude, // DomainIDs deliberately left empty
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdAccountRule.Name != "" {
		t.Error("provider was called despite empty domain_ids under domain_selection_type=include")
	}
}

// TestCreateArvanCloudAccountFirewallRuleRejectsEmptyDomainIDsForExclude
// mirrors the "include" case above for "exclude".
func TestCreateArvanCloudAccountFirewallRuleRejectsEmptyDomainIDsForExclude(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewCreateArvanCloudAccountFirewallRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudAccountFirewallRuleInput{
		Credentials: validArvanCloudCreds(),
		Rule: domain.ArvanCloudAccountFirewallRule{
			Name: "x", FilterExpr: "ssl", Action: domain.ArvanCloudFirewallActionDeny,
			DomainSelectionType: domain.ArvanCloudDomainSelectionExclude,
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdAccountRule.Name != "" {
		t.Error("provider was called despite empty domain_ids under domain_selection_type=exclude")
	}
}

// TestCreateArvanCloudAccountFirewallRuleAllowsEmptyDomainIDsForAll proves
// "all" needs no domain_ids at all.
func TestCreateArvanCloudAccountFirewallRuleAllowsEmptyDomainIDsForAll(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewCreateArvanCloudAccountFirewallRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudAccountFirewallRuleInput{
		Credentials: validArvanCloudCreds(),
		Rule: domain.ArvanCloudAccountFirewallRule{
			Name: "x", FilterExpr: "ssl", Action: domain.ArvanCloudFirewallActionDeny,
			DomainSelectionType: domain.ArvanCloudDomainSelectionAll,
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil (domain_ids is not required for \"all\")", err)
	}
}

func TestCreateArvanCloudAccountFirewallRuleRejectsBadDomainSelectionType(t *testing.T) {
	uc := app.NewCreateArvanCloudAccountFirewallRule(&inlineQueue{}, &fakeArvanCloudFirewallProvider{})

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudAccountFirewallRuleInput{
		Credentials: validArvanCloudCreds(),
		Rule: domain.ArvanCloudAccountFirewallRule{
			Name: "x", FilterExpr: "ssl", Action: domain.ArvanCloudFirewallActionDeny,
			DomainSelectionType: "only",
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetArvanCloudAccountFirewallRuleNotFound(t *testing.T) {
	uc := app.NewGetArvanCloudAccountFirewallRule(&inlineQueue{}, &fakeArvanCloudFirewallProvider{})

	_, err := uc.Execute(context.Background(), app.GetArvanCloudAccountFirewallRuleInput{Credentials: validArvanCloudCreds(), ID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateArvanCloudAccountFirewallRuleSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewUpdateArvanCloudAccountFirewallRule(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudAccountFirewallRuleInput{
		Credentials: validArvanCloudCreds(), ID: "rule-1",
		Rule: domain.ArvanCloudAccountFirewallRule{
			Name: "block-scanners-v2", FilterExpr: "ssl", Action: domain.ArvanCloudFirewallActionDeny,
			DomainSelectionType: domain.ArvanCloudDomainSelectionAll,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.Name != "block-scanners-v2" || provider.updatedAccountRuleID != "rule-1" {
		t.Errorf("updated = %+v, provider.updatedAccountRuleID = %q, want block-scanners-v2/rule-1", updated, provider.updatedAccountRuleID)
	}
}

func TestUpdateArvanCloudAccountFirewallRuleRejectsEmptyDomainIDsForInclude(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewUpdateArvanCloudAccountFirewallRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudAccountFirewallRuleInput{
		Credentials: validArvanCloudCreds(), ID: "rule-1",
		Rule: domain.ArvanCloudAccountFirewallRule{
			Name: "x", FilterExpr: "ssl", Action: domain.ArvanCloudFirewallActionDeny,
			DomainSelectionType: domain.ArvanCloudDomainSelectionInclude,
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteArvanCloudAccountFirewallRuleTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{deleteAccountErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudAccountFirewallRule(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudAccountFirewallRuleInput{Credentials: validArvanCloudCreds(), ID: "gone"}); err != nil {
		t.Fatalf("Execute() error = %v, want nil (already-absent rule tolerated)", err)
	}
}

func TestAttachArvanCloudAccountFirewallDomainsSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewAttachArvanCloudAccountFirewallDomains(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.AttachArvanCloudAccountFirewallDomainsInput{
		Credentials: validArvanCloudCreds(), ID: "rule-1", DomainIDs: []string{"domain-uuid-1"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.ID != "rule-1" || provider.attachedID != "rule-1" || len(provider.attachedDomainIDs) != 1 {
		t.Errorf("updated = %+v, provider attached %q/%v, want rule-1", updated, provider.attachedID, provider.attachedDomainIDs)
	}
}

func TestAttachArvanCloudAccountFirewallDomainsValidation(t *testing.T) {
	uc := app.NewAttachArvanCloudAccountFirewallDomains(&inlineQueue{}, &fakeArvanCloudFirewallProvider{})

	tests := []struct {
		name string
		in   app.AttachArvanCloudAccountFirewallDomainsInput
	}{
		{"missing id", app.AttachArvanCloudAccountFirewallDomainsInput{Credentials: validArvanCloudCreds(), DomainIDs: []string{"d-1"}}},
		{"empty domain_ids", app.AttachArvanCloudAccountFirewallDomainsInput{Credentials: validArvanCloudCreds(), ID: "rule-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := uc.Execute(context.Background(), tc.in); !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

func TestDetachArvanCloudAccountFirewallDomainsSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewDetachArvanCloudAccountFirewallDomains(&inlineQueue{}, provider)

	updated, err := uc.Execute(context.Background(), app.DetachArvanCloudAccountFirewallDomainsInput{
		Credentials: validArvanCloudCreds(), ID: "rule-1", DomainIDs: []string{"domain-uuid-1"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.ID != "rule-1" || provider.detachedID != "rule-1" || len(provider.detachedDomainIDs) != 1 {
		t.Errorf("updated = %+v, provider detached %q/%v, want rule-1", updated, provider.detachedID, provider.detachedDomainIDs)
	}
}

func TestReprioritizeArvanCloudAccountFirewallRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudFirewallProvider{}
	uc := app.NewReprioritizeArvanCloudAccountFirewallRules(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudAccountFirewallRulesInput{
		Credentials: validArvanCloudCreds(), RuleID: "rule-1", BeforeRuleID: "rule-2",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.reprioritizedRuleID != "rule-1" || provider.reprioritizedBefore != "rule-2" {
		t.Errorf("provider received rule_id=%q before=%q, want rule-1/rule-2", provider.reprioritizedRuleID, provider.reprioritizedBefore)
	}
}

func TestReprioritizeArvanCloudAccountFirewallRulesRejectsBothAfterAndBefore(t *testing.T) {
	uc := app.NewReprioritizeArvanCloudAccountFirewallRules(&inlineQueue{}, &fakeArvanCloudFirewallProvider{})

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudAccountFirewallRulesInput{
		Credentials: validArvanCloudCreds(), RuleID: "rule-1", AfterRuleID: "rule-2", BeforeRuleID: "rule-3",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestReprioritizeArvanCloudAccountFirewallRulesMissingRuleID(t *testing.T) {
	uc := app.NewReprioritizeArvanCloudAccountFirewallRules(&inlineQueue{}, &fakeArvanCloudFirewallProvider{})

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudAccountFirewallRulesInput{
		Credentials: validArvanCloudCreds(), AfterRuleID: "rule-2",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}
