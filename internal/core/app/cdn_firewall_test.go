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

// fakeCDNFirewallProvider implements only the small local interface
// cdn_firewall.go's use cases depend on (app.cdnFirewallProvider is
// unexported, so this satisfies it structurally without naming it). It is
// deliberately its own type rather than an extension of the shared
// fakeProvider in app_test.go, per house style: app_test.go is a file other
// groups touch in parallel and must not be edited here.
type fakeCDNFirewallProvider struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	listRulesOut           []domain.CDNAccessRule
	listRulesErr           error

	getRuleOut *domain.CDNAccessRule
	getRuleErr error

	createRuleIn  domain.CDNAccessRule
	createRuleErr error

	updateRuleIn  domain.CDNAccessRule
	updateRuleErr error

	deleteRuleZoneUUID string
	deleteRuleID       string
	deleteRuleErr      error

	ipReputationOut *domain.CDNIPReputationSettings
	getIPRepErr     error

	updateIPRepIn  domain.CDNIPReputationSettings
	updateIPRepErr error

	ddosOut    *domain.CDNDDoSActionSettings
	getDDoSErr error

	updateDDoSIn  domain.CDNDDoSActionSettings
	updateDDoSErr error
}

func (p *fakeCDNFirewallProvider) ListCDNAccessRules(context.Context, domain.ProviderCredentials, string) ([]domain.CDNAccessRule, error) {
	return p.listRulesOut, p.listRulesErr
}

func (p *fakeCDNFirewallProvider) CreateCDNAccessRule(_ context.Context, _ domain.ProviderCredentials, zoneUUID string, rule domain.CDNAccessRule) (*domain.CDNAccessRule, error) {
	if p.createRuleErr != nil {
		return nil, p.createRuleErr
	}
	p.createRuleIn = rule
	rule.ZoneUUID = zoneUUID
	return &rule, nil
}

func (p *fakeCDNFirewallProvider) GetCDNAccessRule(context.Context, domain.ProviderCredentials, string, string) (*domain.CDNAccessRule, error) {
	return p.getRuleOut, p.getRuleErr
}

func (p *fakeCDNFirewallProvider) UpdateCDNAccessRule(_ context.Context, _ domain.ProviderCredentials, zoneUUID string, rule domain.CDNAccessRule) (*domain.CDNAccessRule, error) {
	if p.updateRuleErr != nil {
		return nil, p.updateRuleErr
	}
	p.updateRuleIn = rule
	rule.ZoneUUID = zoneUUID
	return &rule, nil
}

func (p *fakeCDNFirewallProvider) DeleteCDNAccessRule(_ context.Context, _ domain.ProviderCredentials, zoneUUID, ruleID string) error {
	if p.deleteRuleErr != nil {
		return p.deleteRuleErr
	}
	p.deleteRuleZoneUUID = zoneUUID
	p.deleteRuleID = ruleID
	return nil
}

func (p *fakeCDNFirewallProvider) GetCDNIPReputation(context.Context, domain.ProviderCredentials, string) (*domain.CDNIPReputationSettings, error) {
	return p.ipReputationOut, p.getIPRepErr
}

func (p *fakeCDNFirewallProvider) UpdateCDNIPReputation(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.CDNIPReputationSettings) (*domain.CDNIPReputationSettings, error) {
	if p.updateIPRepErr != nil {
		return nil, p.updateIPRepErr
	}
	p.updateIPRepIn = settings
	return &settings, nil
}

func (p *fakeCDNFirewallProvider) GetCDNDDoSActions(context.Context, domain.ProviderCredentials, string) (*domain.CDNDDoSActionSettings, error) {
	return p.ddosOut, p.getDDoSErr
}

func (p *fakeCDNFirewallProvider) UpdateCDNDDoSActions(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.CDNDDoSActionSettings) (*domain.CDNDDoSActionSettings, error) {
	if p.updateDDoSErr != nil {
		return nil, p.updateDDoSErr
	}
	p.updateDDoSIn = settings
	return &settings, nil
}

func TestListCDNAccessRulesReturnsProviderResult(t *testing.T) {
	provider := &fakeCDNFirewallProvider{listRulesOut: []domain.CDNAccessRule{{ID: "r1"}, {ID: "r2"}}}
	uc := app.NewListCDNAccessRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListCDNAccessRulesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
}

func TestListCDNAccessRulesRequiresZoneUUID(t *testing.T) {
	uc := app.NewListCDNAccessRules(&inlineQueue{}, &fakeCDNFirewallProvider{})

	_, err := uc.Execute(context.Background(), app.ListCDNAccessRulesInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNAccessRuleReturnsProviderResult(t *testing.T) {
	provider := &fakeCDNFirewallProvider{getRuleOut: &domain.CDNAccessRule{ID: "r1", Action: "block"}}
	uc := app.NewGetCDNAccessRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.GetCDNAccessRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", RuleID: "r1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.ID != "r1" || rule.Action != "block" {
		t.Errorf("rule = %+v, unexpected", rule)
	}
}

func TestGetCDNAccessRuleRequiresRuleID(t *testing.T) {
	uc := app.NewGetCDNAccessRule(&inlineQueue{}, &fakeCDNFirewallProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNAccessRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCreateCDNAccessRuleSuccess(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewCreateCDNAccessRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.CreateCDNAccessRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Rule: domain.CDNAccessRule{Type: "ip", Value: "1.2.3.4", Action: "block"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.ZoneUUID != "z1" {
		t.Errorf("ZoneUUID = %q, want z1", rule.ZoneUUID)
	}
	if provider.createRuleIn.Value != "1.2.3.4" {
		t.Errorf("provider saw value %q, want 1.2.3.4", provider.createRuleIn.Value)
	}
}

// TestCreateCDNAccessRuleRejectsMissingValueAndBulklist proves the spec's
// "value is required unless bulklist_id is present" rule is enforced before
// the provider is ever called.
func TestCreateCDNAccessRuleRejectsMissingValueAndBulklist(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewCreateCDNAccessRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNAccessRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Rule: domain.CDNAccessRule{Type: "ip", Action: "block"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createRuleIn.Type != "" {
		t.Error("provider was called with an invalid rule")
	}
}

func TestCreateCDNAccessRuleRejectsInvalidType(t *testing.T) {
	uc := app.NewCreateCDNAccessRule(&inlineQueue{}, &fakeCDNFirewallProvider{})

	_, err := uc.Execute(context.Background(), app.CreateCDNAccessRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Rule: domain.CDNAccessRule{Type: "not-a-type", Value: "1.2.3.4", Action: "block"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNAccessRuleSuccess(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewUpdateCDNAccessRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.UpdateCDNAccessRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Rule: domain.CDNAccessRule{ID: "r1", Value: "1.2.3.4", Action: "allow", Status: true},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.Action != "allow" || !rule.Status {
		t.Errorf("rule = %+v, want action allow, status true", rule)
	}
}

func TestUpdateCDNAccessRuleRequiresRuleID(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewUpdateCDNAccessRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateCDNAccessRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Rule: domain.CDNAccessRule{Value: "1.2.3.4", Action: "allow", Status: true},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteCDNAccessRuleCallsProvider(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewDeleteCDNAccessRule(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNAccessRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", RuleID: "r1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deleteRuleZoneUUID != "z1" || provider.deleteRuleID != "r1" {
		t.Errorf("deleteRuleZoneUUID/ID = %q/%q, want z1/r1", provider.deleteRuleZoneUUID, provider.deleteRuleID)
	}
}

// TestDeleteCDNAccessRuleTreatsAlreadyGoneAsSuccess proves
// delete_cdn_access_rule can be called more than once safely.
func TestDeleteCDNAccessRuleTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeCDNFirewallProvider{deleteRuleErr: fmt.Errorf("rule r1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteCDNAccessRule(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNAccessRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", RuleID: "r1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted rule", err)
	}
}

func TestGetCDNIPReputationReturnsProviderResult(t *testing.T) {
	provider := &fakeCDNFirewallProvider{ipReputationOut: &domain.CDNIPReputationSettings{TreatScore: "medium"}}
	uc := app.NewGetCDNIPReputation(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetCDNIPReputationInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if settings.TreatScore != "medium" {
		t.Errorf("TreatScore = %q, want medium", settings.TreatScore)
	}
}

func TestUpdateCDNIPReputationSuccess(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewUpdateCDNIPReputation(&inlineQueue{}, provider)

	in := domain.CDNIPReputationSettings{
		Enabled: true, TrustTime: 1800, TreatScore: "high", Challenge: "js", AttackBanTime: 600,
	}
	settings, err := uc.Execute(context.Background(), app.UpdateCDNIPReputationInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", Settings: in,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if *settings != in {
		t.Errorf("settings = %+v, want %+v", settings, in)
	}
}

func TestUpdateCDNIPReputationRejectsInvalidTreatScore(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewUpdateCDNIPReputation(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateCDNIPReputationInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Settings: domain.CDNIPReputationSettings{TreatScore: "extreme", Challenge: "js"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNIPReputationRejectsInvalidChallenge(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewUpdateCDNIPReputation(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateCDNIPReputationInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Settings: domain.CDNIPReputationSettings{TreatScore: "medium", Challenge: "captcha"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNDDoSActionsReturnsProviderResult(t *testing.T) {
	provider := &fakeCDNFirewallProvider{ddosOut: &domain.CDNDDoSActionSettings{Action: "js", TrustTime: 10, BanTime: 10}}
	uc := app.NewGetCDNDDoSActions(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetCDNDDoSActionsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if settings.Action != "js" {
		t.Errorf("Action = %q, want js", settings.Action)
	}
}

func TestUpdateCDNDDoSActionsSuccess(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewUpdateCDNDDoSActions(&inlineQueue{}, provider)

	in := domain.CDNDDoSActionSettings{Action: "block", TrustTime: 3600, BanTime: 900}
	settings, err := uc.Execute(context.Background(), app.UpdateCDNDDoSActionsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", Settings: in,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if *settings != in {
		t.Errorf("settings = %+v, want %+v", settings, in)
	}
}

func TestUpdateCDNDDoSActionsRejectsInvalidAction(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewUpdateCDNDDoSActions(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateCDNDDoSActionsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Settings: domain.CDNDDoSActionSettings{Action: "captcha"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNDDoSActionsRejectsOutOfRangeTrustTime(t *testing.T) {
	provider := &fakeCDNFirewallProvider{}
	uc := app.NewUpdateCDNDDoSActions(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateCDNDDoSActionsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Settings: domain.CDNDDoSActionSettings{Action: "block", TrustTime: 999999},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
