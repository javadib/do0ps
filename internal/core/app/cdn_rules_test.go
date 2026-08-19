package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// The fakes below are package-local to this file (not the shared fakeProvider
// in app_test.go): each use case in cdn_rules.go is typed against a small
// local interface, so a fake only needs to implement the handful of methods
// that interface actually declares.

type fakeOriginRuleProvider struct {
	rules           []domain.CDNOriginRule
	createdZoneUUID string
	createdRule     *domain.CDNOriginRule
	updatedRuleID   string
	updatedRule     *domain.CDNOriginRule
	deleteErr       error
	deletedRuleID   string
	toggledRuleID   string
	toggledEnabled  bool
}

func (p *fakeOriginRuleProvider) ListCDNOriginRules(context.Context, domain.ProviderCredentials, string) ([]domain.CDNOriginRule, error) {
	return p.rules, nil
}

func (p *fakeOriginRuleProvider) CreateCDNOriginRule(_ context.Context, _ domain.ProviderCredentials, zoneUUID string, rule domain.CDNOriginRule) (*domain.CDNOriginRule, error) {
	p.createdZoneUUID = zoneUUID
	p.createdRule = &rule
	return &rule, nil
}

func (p *fakeOriginRuleProvider) GetCDNOriginRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string) (*domain.CDNOriginRule, error) {
	for i := range p.rules {
		if p.rules[i].ID == ruleID {
			return &p.rules[i], nil
		}
	}
	return nil, fmt.Errorf("origin rule %q: %w", ruleID, domain.ErrNotFound)
}

func (p *fakeOriginRuleProvider) UpdateCDNOriginRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string, rule domain.CDNOriginRule) (*domain.CDNOriginRule, error) {
	p.updatedRuleID = ruleID
	rule.ID = ruleID
	p.updatedRule = &rule
	return &rule, nil
}

func (p *fakeOriginRuleProvider) DeleteCDNOriginRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedRuleID = ruleID
	return nil
}

func (p *fakeOriginRuleProvider) ToggleCDNOriginRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string, enabled bool) error {
	p.toggledRuleID = ruleID
	p.toggledEnabled = enabled
	return nil
}

func TestListCDNOriginRulesReturnsProviderResult(t *testing.T) {
	provider := &fakeOriginRuleProvider{rules: []domain.CDNOriginRule{{ID: "r1", Name: "a"}, {ID: "r2", Name: "b"}}}
	uc := app.NewListCDNOriginRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListCDNOriginRulesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
}

func TestCreateCDNOriginRuleSuccess(t *testing.T) {
	provider := &fakeOriginRuleProvider{}
	uc := app.NewCreateCDNOriginRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.CreateCDNOriginRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		Rule:        domain.CDNOriginRule{Name: "upstream-rule", Type: "upstream", UpstreamIP: "1.1.1.1"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.Name != "upstream-rule" {
		t.Errorf("Name = %q, want upstream-rule", rule.Name)
	}
	if provider.createdZoneUUID != "zone-1" {
		t.Errorf("createdZoneUUID = %q, want zone-1", provider.createdZoneUUID)
	}
}

func TestCreateCDNOriginRuleRejectsMissingUpstreamIP(t *testing.T) {
	provider := &fakeOriginRuleProvider{}
	uc := app.NewCreateCDNOriginRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNOriginRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		Rule:        domain.CDNOriginRule{Name: "upstream-rule", Type: "upstream"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdRule != nil {
		t.Error("provider was called with a rule missing its required upstream_ip")
	}
}

func TestGetCDNOriginRuleReturnsMatchingRule(t *testing.T) {
	provider := &fakeOriginRuleProvider{rules: []domain.CDNOriginRule{{ID: "r1", Name: "a"}}}
	uc := app.NewGetCDNOriginRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.GetCDNOriginRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "r1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.Name != "a" {
		t.Errorf("Name = %q, want a", rule.Name)
	}
}

func TestGetCDNOriginRuleUnknownID(t *testing.T) {
	uc := app.NewGetCDNOriginRule(&inlineQueue{}, &fakeOriginRuleProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNOriginRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNOriginRuleSuccess(t *testing.T) {
	provider := &fakeOriginRuleProvider{}
	uc := app.NewUpdateCDNOriginRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.UpdateCDNOriginRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		RuleID:      "r1",
		Rule:        domain.CDNOriginRule{Name: "renamed", Type: "port", Port: 8080},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.ID != "r1" {
		t.Errorf("ID = %q, want r1", rule.ID)
	}
	if provider.updatedRuleID != "r1" {
		t.Errorf("updatedRuleID = %q, want r1", provider.updatedRuleID)
	}
}

func TestDeleteCDNOriginRuleTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeOriginRuleProvider{deleteErr: fmt.Errorf("rule r1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteCDNOriginRule(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNOriginRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "r1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted rule", err)
	}
}

func TestToggleCDNOriginRuleCallsProvider(t *testing.T) {
	provider := &fakeOriginRuleProvider{}
	uc := app.NewToggleCDNOriginRule(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ToggleCDNOriginRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "r1", Enabled: false,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.toggledRuleID != "r1" || provider.toggledEnabled {
		t.Errorf("toggledRuleID/Enabled = %q/%v, want r1/false", provider.toggledRuleID, provider.toggledEnabled)
	}
}

// ---------------------------------------------------------------------------

type fakePageRuleProvider struct {
	rules           []domain.CDNPageRule
	createdZoneUUID string
	createdRule     *domain.CDNPageRule
	updatedRuleID   string
	updatedRule     *domain.CDNPageRule
	deleteErr       error
	deletedRuleID   string
}

func (p *fakePageRuleProvider) ListCDNPageRules(context.Context, domain.ProviderCredentials, string) ([]domain.CDNPageRule, error) {
	return p.rules, nil
}

func (p *fakePageRuleProvider) CreateCDNPageRule(_ context.Context, _ domain.ProviderCredentials, zoneUUID string, rule domain.CDNPageRule) (*domain.CDNPageRule, error) {
	p.createdZoneUUID = zoneUUID
	p.createdRule = &rule
	return &rule, nil
}

func (p *fakePageRuleProvider) GetCDNPageRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string) (*domain.CDNPageRule, error) {
	for i := range p.rules {
		if p.rules[i].ID == ruleID {
			return &p.rules[i], nil
		}
	}
	return nil, fmt.Errorf("page rule %q: %w", ruleID, domain.ErrNotFound)
}

func (p *fakePageRuleProvider) UpdateCDNPageRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string, rule domain.CDNPageRule) (*domain.CDNPageRule, error) {
	p.updatedRuleID = ruleID
	rule.ID = ruleID
	p.updatedRule = &rule
	return &rule, nil
}

func (p *fakePageRuleProvider) DeleteCDNPageRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedRuleID = ruleID
	return nil
}

func TestListCDNPageRulesReturnsProviderResult(t *testing.T) {
	provider := &fakePageRuleProvider{rules: []domain.CDNPageRule{{ID: "p1"}, {ID: "p2"}}}
	uc := app.NewListCDNPageRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListCDNPageRulesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
}

func TestCreateCDNPageRuleSuccess(t *testing.T) {
	provider := &fakePageRuleProvider{}
	uc := app.NewCreateCDNPageRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.CreateCDNPageRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		Rule: domain.CDNPageRule{
			Name: "cache-assets", Type: "url", Operator: "pattern", Value: "*example.com/assets/**",
			FirewallStatus: true,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.Value != "*example.com/assets/**" {
		t.Errorf("Value = %q, want *example.com/assets/**", rule.Value)
	}
	if provider.createdZoneUUID != "zone-1" {
		t.Errorf("createdZoneUUID = %q, want zone-1", provider.createdZoneUUID)
	}
}

func TestCreateCDNPageRuleRejectsMissingValue(t *testing.T) {
	provider := &fakePageRuleProvider{}
	uc := app.NewCreateCDNPageRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNPageRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		Rule:        domain.CDNPageRule{Name: "cache-assets", FirewallStatus: true},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdRule != nil {
		t.Error("provider was called with a rule missing its required value")
	}
}

func TestGetCDNPageRuleUnknownID(t *testing.T) {
	uc := app.NewGetCDNPageRule(&inlineQueue{}, &fakePageRuleProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNPageRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNPageRuleSuccess(t *testing.T) {
	provider := &fakePageRuleProvider{}
	uc := app.NewUpdateCDNPageRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.UpdateCDNPageRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		RuleID:      "p1",
		Rule:        domain.CDNPageRule{Name: "renamed", Value: "*example.com/**", FirewallStatus: true},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.ID != "p1" {
		t.Errorf("ID = %q, want p1", rule.ID)
	}
}

func TestDeleteCDNPageRuleTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakePageRuleProvider{deleteErr: fmt.Errorf("rule p1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteCDNPageRule(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNPageRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "p1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted rule", err)
	}
}

// ---------------------------------------------------------------------------

type fakeTransformRuleProvider struct {
	rules           []domain.CDNTransformRule
	createdZoneUUID string
	createdRule     *domain.CDNTransformRule
	updatedRuleID   string
	updatedRule     *domain.CDNTransformRule
	deleteErr       error
	deletedRuleID   string
	toggledRuleID   string
	toggledEnabled  bool
}

func (p *fakeTransformRuleProvider) ListCDNTransformRules(context.Context, domain.ProviderCredentials, string) ([]domain.CDNTransformRule, error) {
	return p.rules, nil
}

func (p *fakeTransformRuleProvider) CreateCDNTransformRule(_ context.Context, _ domain.ProviderCredentials, zoneUUID string, rule domain.CDNTransformRule) (*domain.CDNTransformRule, error) {
	p.createdZoneUUID = zoneUUID
	p.createdRule = &rule
	return &rule, nil
}

func (p *fakeTransformRuleProvider) GetCDNTransformRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string) (*domain.CDNTransformRule, error) {
	for i := range p.rules {
		if p.rules[i].ID == ruleID {
			return &p.rules[i], nil
		}
	}
	return nil, fmt.Errorf("transform rule %q: %w", ruleID, domain.ErrNotFound)
}

func (p *fakeTransformRuleProvider) UpdateCDNTransformRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string, rule domain.CDNTransformRule) (*domain.CDNTransformRule, error) {
	p.updatedRuleID = ruleID
	rule.ID = ruleID
	p.updatedRule = &rule
	return &rule, nil
}

func (p *fakeTransformRuleProvider) DeleteCDNTransformRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deletedRuleID = ruleID
	return nil
}

func (p *fakeTransformRuleProvider) ToggleCDNTransformRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string, enabled bool) error {
	p.toggledRuleID = ruleID
	p.toggledEnabled = enabled
	return nil
}

func TestListCDNTransformRulesReturnsProviderResult(t *testing.T) {
	provider := &fakeTransformRuleProvider{rules: []domain.CDNTransformRule{{ID: "t1"}, {ID: "t2"}}}
	uc := app.NewListCDNTransformRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListCDNTransformRulesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
}

func TestCreateCDNTransformRuleSuccess(t *testing.T) {
	provider := &fakeTransformRuleProvider{}
	uc := app.NewCreateCDNTransformRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.CreateCDNTransformRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		Rule: domain.CDNTransformRule{
			Name: "header-modify-rule",
			RequestHeaders: []domain.CDNTransformHeaderAction{
				{HeaderName: "X-Request-Id", HeaderValue: "abc123", Action: "modify"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.Name != "header-modify-rule" {
		t.Errorf("Name = %q, want header-modify-rule", rule.Name)
	}
	if provider.createdZoneUUID != "zone-1" {
		t.Errorf("createdZoneUUID = %q, want zone-1", provider.createdZoneUUID)
	}
}

// TestCreateCDNTransformRuleRejectsMissingHeaderValueOnModify proves a
// "modify" header action without a header_value fails fast (issue #24).
func TestCreateCDNTransformRuleRejectsMissingHeaderValueOnModify(t *testing.T) {
	provider := &fakeTransformRuleProvider{}
	uc := app.NewCreateCDNTransformRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNTransformRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		Rule: domain.CDNTransformRule{
			Name:           "header-modify-rule",
			RequestHeaders: []domain.CDNTransformHeaderAction{{HeaderName: "X-Request-Id", Action: "modify"}},
		},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdRule != nil {
		t.Error("provider was called with a modify action missing its required header_value")
	}
}

func TestGetCDNTransformRuleUnknownID(t *testing.T) {
	uc := app.NewGetCDNTransformRule(&inlineQueue{}, &fakeTransformRuleProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNTransformRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNTransformRuleSuccess(t *testing.T) {
	provider := &fakeTransformRuleProvider{}
	uc := app.NewUpdateCDNTransformRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.UpdateCDNTransformRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ZoneUUID:    "zone-1",
		RuleID:      "t1",
		Rule:        domain.CDNTransformRule{Name: "renamed"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.ID != "t1" {
		t.Errorf("ID = %q, want t1", rule.ID)
	}
}

func TestDeleteCDNTransformRuleTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeTransformRuleProvider{deleteErr: fmt.Errorf("rule t1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteCDNTransformRule(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNTransformRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "t1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted rule", err)
	}
}

func TestToggleCDNTransformRuleCallsProvider(t *testing.T) {
	provider := &fakeTransformRuleProvider{}
	uc := app.NewToggleCDNTransformRule(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ToggleCDNTransformRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "zone-1", RuleID: "t1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.toggledRuleID != "t1" || !provider.toggledEnabled {
		t.Errorf("toggledRuleID/Enabled = %q/%v, want t1/true", provider.toggledRuleID, provider.toggledEnabled)
	}
}
