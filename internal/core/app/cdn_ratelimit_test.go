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

// cdnRateLimitFakeProvider is a package-local fake implementing exactly the
// provider methods app.cdnRateLimitProvider (an unexported interface local
// to internal/core/app) needs. It is declared here, not in app_test.go
// (that file is shared across several concurrently developed slices of
// issue #24 and must not be edited from this file), and is separate from
// the shared fakeProvider for the same reason. Go interfaces are satisfied
// structurally, so this type does not need to name the interface: passing
// it to the app.NewX constructors below is enough for the compiler to check
// it implements what each use case needs.
type cdnRateLimitFakeProvider struct {
	ports.ParspackProvider // embedded nil; only the methods below are overridden
	rules                  []domain.CDNRateLimitRule
	rulesErr               error

	createdRule domain.CDNRateLimitRule
	createErr   error

	updatedZoneUUID string
	updatedRuleID   string
	updatedRule     domain.CDNRateLimitRule
	updateErr       error

	deletedRuleID string
	deleteErr     error

	priorityRuleID string
	priorityValue  int
	priorityErr    error

	upstreamErrors    domain.CDNUpstreamErrorSettings
	getUpstreamErrErr error

	updatedUpstreamEnabled bool
	updateUpstreamErr      error
}

func (p *cdnRateLimitFakeProvider) ListCDNRateLimitRules(context.Context, domain.ProviderCredentials, string) ([]domain.CDNRateLimitRule, error) {
	if p.rulesErr != nil {
		return nil, p.rulesErr
	}
	return p.rules, nil
}

func (p *cdnRateLimitFakeProvider) CreateCDNRateLimitRule(_ context.Context, _ domain.ProviderCredentials, _ string, rule domain.CDNRateLimitRule) (*domain.CDNRateLimitRule, error) {
	if p.createErr != nil {
		return nil, p.createErr
	}
	p.createdRule = rule
	return &rule, nil
}

func (p *cdnRateLimitFakeProvider) GetCDNRateLimitRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string) (*domain.CDNRateLimitRule, error) {
	for i := range p.rules {
		if p.rules[i].ID == ruleID {
			return &p.rules[i], nil
		}
	}
	return nil, fmt.Errorf("rule %q: %w", ruleID, domain.ErrNotFound)
}

func (p *cdnRateLimitFakeProvider) UpdateCDNRateLimitRule(_ context.Context, _ domain.ProviderCredentials, zoneUUID, ruleID string, rule domain.CDNRateLimitRule) (*domain.CDNRateLimitRule, error) {
	if p.updateErr != nil {
		return nil, p.updateErr
	}
	p.updatedZoneUUID = zoneUUID
	p.updatedRuleID = ruleID
	rule.ID = ruleID
	p.updatedRule = rule
	return &rule, nil
}

func (p *cdnRateLimitFakeProvider) DeleteCDNRateLimitRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedRuleID = ruleID
	return nil
}

func (p *cdnRateLimitFakeProvider) UpdateCDNRateLimitRulePriority(_ context.Context, _ domain.ProviderCredentials, _, ruleID string, priority int) error {
	if p.priorityErr != nil {
		return p.priorityErr
	}
	p.priorityRuleID = ruleID
	p.priorityValue = priority
	return nil
}

func (p *cdnRateLimitFakeProvider) GetCDNUpstreamErrors(context.Context, domain.ProviderCredentials, string) (*domain.CDNUpstreamErrorSettings, error) {
	if p.getUpstreamErrErr != nil {
		return nil, p.getUpstreamErrErr
	}
	settings := p.upstreamErrors
	return &settings, nil
}

func (p *cdnRateLimitFakeProvider) UpdateCDNUpstreamErrors(_ context.Context, _ domain.ProviderCredentials, _ string, enabled bool) (*domain.CDNUpstreamErrorSettings, error) {
	if p.updateUpstreamErr != nil {
		return nil, p.updateUpstreamErr
	}
	p.updatedUpstreamEnabled = enabled
	return &domain.CDNUpstreamErrorSettings{Enabled: enabled}, nil
}

func validRateLimitRule() domain.CDNRateLimitRule {
	return domain.CDNRateLimitRule{
		Name: "sample-rule", Value: "https://example.com/*",
		StaticIntervalType: "second", StaticInterval: 1,
		DynamicIntervalType: "day", DynamicInterval: 1,
		Challenge: "js", TrustTime: 1, AttackBanTime: 1,
	}
}

func TestListCDNRateLimitRulesReturnsProviderResult(t *testing.T) {
	provider := &cdnRateLimitFakeProvider{rules: []domain.CDNRateLimitRule{{ID: "r1", Name: "one"}, {ID: "r2", Name: "two"}}}
	uc := app.NewListCDNRateLimitRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListCDNRateLimitRulesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 2 {
		t.Errorf("len(rules) = %d, want 2", len(rules))
	}
}

func TestListCDNRateLimitRulesRequiresZoneUUID(t *testing.T) {
	uc := app.NewListCDNRateLimitRules(&inlineQueue{}, &cdnRateLimitFakeProvider{})

	_, err := uc.Execute(context.Background(), app.ListCDNRateLimitRulesInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestCreateCDNRateLimitRuleSuccess(t *testing.T) {
	provider := &cdnRateLimitFakeProvider{}
	uc := app.NewCreateCDNRateLimitRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.CreateCDNRateLimitRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Rule: validRateLimitRule(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.Name != "sample-rule" {
		t.Errorf("Name = %q, want sample-rule", rule.Name)
	}
	if provider.createdRule.Value != "https://example.com/*" {
		t.Errorf("provider was called with value %q", provider.createdRule.Value)
	}
}

func TestCreateCDNRateLimitRuleRejectsBadChallenge(t *testing.T) {
	provider := &cdnRateLimitFakeProvider{}
	uc := app.NewCreateCDNRateLimitRule(&inlineQueue{}, provider)

	rule := validRateLimitRule()
	rule.Challenge = "deny-everyone"

	_, err := uc.Execute(context.Background(), app.CreateCDNRateLimitRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Rule: rule,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdRule.Name != "" {
		t.Error("provider was called with an invalid challenge")
	}
}

func TestCreateCDNRateLimitRuleRejectsBadStaticIntervalType(t *testing.T) {
	provider := &cdnRateLimitFakeProvider{}
	uc := app.NewCreateCDNRateLimitRule(&inlineQueue{}, provider)

	rule := validRateLimitRule()
	rule.StaticIntervalType = "fortnight"

	_, err := uc.Execute(context.Background(), app.CreateCDNRateLimitRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Rule: rule,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNRateLimitRuleSuccess(t *testing.T) {
	provider := &cdnRateLimitFakeProvider{rules: []domain.CDNRateLimitRule{{ID: "r1", Name: "one"}}}
	uc := app.NewGetCDNRateLimitRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.GetCDNRateLimitRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "r1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.Name != "one" {
		t.Errorf("Name = %q, want one", rule.Name)
	}
}

func TestGetCDNRateLimitRuleNotFound(t *testing.T) {
	uc := app.NewGetCDNRateLimitRule(&inlineQueue{}, &cdnRateLimitFakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNRateLimitRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNRateLimitRuleSuccess(t *testing.T) {
	provider := &cdnRateLimitFakeProvider{}
	uc := app.NewUpdateCDNRateLimitRule(&inlineQueue{}, provider)

	rule := validRateLimitRule()
	rule.Enabled = true

	updated, err := uc.Execute(context.Background(), app.UpdateCDNRateLimitRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "r1", Rule: rule,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.ID != "r1" || !updated.Enabled {
		t.Errorf("updated = %+v, want id r1 and enabled true", updated)
	}
	if provider.updatedZoneUUID != "zone-1" || provider.updatedRuleID != "r1" {
		t.Errorf("provider called with zone %q rule %q, want zone-1/r1", provider.updatedZoneUUID, provider.updatedRuleID)
	}
}

func TestDeleteCDNRateLimitRuleToleratesNotFound(t *testing.T) {
	provider := &cdnRateLimitFakeProvider{deleteErr: fmt.Errorf("rule r1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteCDNRateLimitRule(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNRateLimitRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "r1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil (already-gone tolerated)", err)
	}
}

func TestDeleteCDNRateLimitRuleRequiresRuleID(t *testing.T) {
	uc := app.NewDeleteCDNRateLimitRule(&inlineQueue{}, &cdnRateLimitFakeProvider{})

	err := uc.Execute(context.Background(), app.DeleteCDNRateLimitRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNRateLimitRulePrioritySuccess(t *testing.T) {
	provider := &cdnRateLimitFakeProvider{}
	uc := app.NewUpdateCDNRateLimitRulePriority(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.UpdateCDNRateLimitRulePriorityInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "r1", Priority: 2,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.priorityRuleID != "r1" || provider.priorityValue != 2 {
		t.Errorf("provider called with rule %q priority %d, want r1/2", provider.priorityRuleID, provider.priorityValue)
	}
}

func TestUpdateCDNRateLimitRulePriorityRejectsZero(t *testing.T) {
	uc := app.NewUpdateCDNRateLimitRulePriority(&inlineQueue{}, &cdnRateLimitFakeProvider{})

	err := uc.Execute(context.Background(), app.UpdateCDNRateLimitRulePriorityInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "r1", Priority: 0,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestGetCDNUpstreamErrorsSuccess(t *testing.T) {
	provider := &cdnRateLimitFakeProvider{upstreamErrors: domain.CDNUpstreamErrorSettings{Enabled: true}}
	uc := app.NewGetCDNUpstreamErrors(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetCDNUpstreamErrorsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !settings.Enabled {
		t.Errorf("Enabled = false, want true")
	}
}

func TestGetCDNUpstreamErrorsRequiresZoneUUID(t *testing.T) {
	uc := app.NewGetCDNUpstreamErrors(&inlineQueue{}, &cdnRateLimitFakeProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNUpstreamErrorsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNUpstreamErrorsSuccess(t *testing.T) {
	provider := &cdnRateLimitFakeProvider{}
	uc := app.NewUpdateCDNUpstreamErrors(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.UpdateCDNUpstreamErrorsInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", Enabled: false,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if settings.Enabled {
		t.Errorf("Enabled = true, want false")
	}
	if provider.updatedUpstreamEnabled {
		t.Error("provider was called with enabled=true, want false")
	}
}

func TestUpdateCDNUpstreamErrorsRequiresCredentials(t *testing.T) {
	uc := app.NewUpdateCDNUpstreamErrors(&inlineQueue{}, &cdnRateLimitFakeProvider{})

	_, err := uc.Execute(context.Background(), app.UpdateCDNUpstreamErrorsInput{ZoneUUID: "zone-1", Enabled: true})
	if err == nil {
		t.Fatal("Execute: want error for missing credentials")
	}
}
