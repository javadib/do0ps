package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// fakeModSecProvider is a package-local fake, separate from the shared
// fakeProvider in app_test.go (house style for issue #24's slices: the
// use cases in cdn_modsec.go are typed against small local interfaces, not
// ports.ParspackProvider directly, so this fake only needs to implement the
// methods those local interfaces declare — no embedding trick required).
type fakeModSecProvider struct {
	status    *domain.CDNModSecStatus
	statusErr error

	modSecData    []domain.CDNModSecData
	getData       *domain.CDNModSecData
	getDataErr    error
	createdData   *domain.CDNModSecData
	updatedData   *domain.CDNModSecData
	deletedDataID string
	deleteDataErr error

	modSecRules   []domain.CDNModSecRule
	getRule       *domain.CDNModSecRule
	getRuleErr    error
	createdRule   *domain.CDNModSecRule
	updatedRule   *domain.CDNModSecRule
	deletedRuleID string
	deleteRuleErr error

	lastSelectedRuleIDs []string
}

func (p *fakeModSecProvider) GetCDNModSecStatus(context.Context, domain.ProviderCredentials, string) (*domain.CDNModSecStatus, error) {
	if p.statusErr != nil {
		return nil, p.statusErr
	}
	return p.status, nil
}

func (p *fakeModSecProvider) UpdateCDNModSecStatus(_ context.Context, _ domain.ProviderCredentials, _ string, selectedRuleIDs []string) (*domain.CDNModSecStatus, error) {
	if p.statusErr != nil {
		return nil, p.statusErr
	}
	p.lastSelectedRuleIDs = selectedRuleIDs
	return p.status, nil
}

func (p *fakeModSecProvider) ListCDNModSecData(context.Context, domain.ProviderCredentials, string) ([]domain.CDNModSecData, error) {
	return p.modSecData, nil
}

func (p *fakeModSecProvider) CreateCDNModSecData(_ context.Context, _ domain.ProviderCredentials, _ string, data domain.CDNModSecData) (*domain.CDNModSecData, error) {
	p.createdData = &data
	return &data, nil
}

func (p *fakeModSecProvider) GetCDNModSecData(_ context.Context, _ domain.ProviderCredentials, _, _ string) (*domain.CDNModSecData, error) {
	if p.getDataErr != nil {
		return nil, p.getDataErr
	}
	return p.getData, nil
}

func (p *fakeModSecProvider) UpdateCDNModSecData(_ context.Context, _ domain.ProviderCredentials, _, id string, data domain.CDNModSecData) (*domain.CDNModSecData, error) {
	data.ID = id
	p.updatedData = &data
	return &data, nil
}

func (p *fakeModSecProvider) DeleteCDNModSecData(_ context.Context, _ domain.ProviderCredentials, _, id string) error {
	if p.deleteDataErr != nil {
		return p.deleteDataErr
	}
	p.deletedDataID = id
	return nil
}

func (p *fakeModSecProvider) ListCDNModSecRules(context.Context, domain.ProviderCredentials, string) ([]domain.CDNModSecRule, error) {
	return p.modSecRules, nil
}

func (p *fakeModSecProvider) CreateCDNModSecRule(_ context.Context, _ domain.ProviderCredentials, _ string, rule domain.CDNModSecRule) (*domain.CDNModSecRule, error) {
	p.createdRule = &rule
	return &rule, nil
}

func (p *fakeModSecProvider) GetCDNModSecRule(_ context.Context, _ domain.ProviderCredentials, _, _ string) (*domain.CDNModSecRule, error) {
	if p.getRuleErr != nil {
		return nil, p.getRuleErr
	}
	return p.getRule, nil
}

func (p *fakeModSecProvider) UpdateCDNModSecRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string, rule domain.CDNModSecRule) (*domain.CDNModSecRule, error) {
	rule.ID = ruleID
	p.updatedRule = &rule
	return &rule, nil
}

func (p *fakeModSecProvider) DeleteCDNModSecRule(_ context.Context, _ domain.ProviderCredentials, _, ruleID string) error {
	if p.deleteRuleErr != nil {
		return p.deleteRuleErr
	}
	p.deletedRuleID = ruleID
	return nil
}

func TestGetCDNModSecStatusReturnsProviderResult(t *testing.T) {
	provider := &fakeModSecProvider{status: &domain.CDNModSecStatus{
		Standards: []domain.CDNModSecRuleSetItem{{ID: "s1", Name: "standard-1", Selected: true}},
	}}
	uc := app.NewGetCDNModSecStatus(&inlineQueue{}, provider)

	status, err := uc.Execute(context.Background(), app.GetCDNModSecStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(status.Standards) != 1 || !status.Standards[0].Selected {
		t.Errorf("status = %+v, want one selected standard", status)
	}
}

func TestGetCDNModSecStatusRequiresZoneUUID(t *testing.T) {
	uc := app.NewGetCDNModSecStatus(&inlineQueue{}, &fakeModSecProvider{})

	_, err := uc.Execute(context.Background(), app.GetCDNModSecStatusInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateCDNModSecStatusPassesSelectedIDs(t *testing.T) {
	provider := &fakeModSecProvider{status: &domain.CDNModSecStatus{}}
	uc := app.NewUpdateCDNModSecStatus(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateCDNModSecStatusInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		SelectedRuleIDs: []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(provider.lastSelectedRuleIDs) != 2 {
		t.Errorf("lastSelectedRuleIDs = %v, want [a b]", provider.lastSelectedRuleIDs)
	}
}

func TestListCDNModSecDataReturnsProviderResult(t *testing.T) {
	provider := &fakeModSecProvider{modSecData: []domain.CDNModSecData{{ID: "d1", Name: "check-1"}}}
	uc := app.NewListCDNModSecData(&inlineQueue{}, provider)

	data, err := uc.Execute(context.Background(), app.ListCDNModSecDataInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(data) != 1 || data[0].ID != "d1" {
		t.Errorf("data = %+v, want a single d1 entry", data)
	}
}

func TestCreateCDNModSecDataSuccess(t *testing.T) {
	provider := &fakeModSecProvider{}
	uc := app.NewCreateCDNModSecData(&inlineQueue{}, provider)

	data, err := uc.Execute(context.Background(), app.CreateCDNModSecDataInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Data: domain.CDNModSecData{Name: "check-1", Value: "abc"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if data.Name != "check-1" {
		t.Errorf("Name = %q, want check-1", data.Name)
	}
	if provider.createdData == nil || provider.createdData.Value != "abc" {
		t.Error("provider was not called with the expected data")
	}
}

func TestCreateCDNModSecDataRejectsMissingValue(t *testing.T) {
	provider := &fakeModSecProvider{}
	uc := app.NewCreateCDNModSecData(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNModSecDataInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Data: domain.CDNModSecData{Name: "check-1"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdData != nil {
		t.Error("provider was called with an invalid data value")
	}
}

func TestGetCDNModSecDataReturnsProviderResult(t *testing.T) {
	provider := &fakeModSecProvider{getData: &domain.CDNModSecData{ID: "d1", Name: "check-1", Value: "abc"}}
	uc := app.NewGetCDNModSecData(&inlineQueue{}, provider)

	data, err := uc.Execute(context.Background(), app.GetCDNModSecDataInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", ID: "d1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if data.Value != "abc" {
		t.Errorf("Value = %q, want abc", data.Value)
	}
}

func TestGetCDNModSecDataUnknownID(t *testing.T) {
	provider := &fakeModSecProvider{getDataErr: fmt.Errorf("data missing: %w", domain.ErrNotFound)}
	uc := app.NewGetCDNModSecData(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetCDNModSecDataInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", ID: "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNModSecDataSuccess(t *testing.T) {
	provider := &fakeModSecProvider{}
	uc := app.NewUpdateCDNModSecData(&inlineQueue{}, provider)

	data, err := uc.Execute(context.Background(), app.UpdateCDNModSecDataInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", ID: "d1",
		Data: domain.CDNModSecData{Name: "renamed", Value: "new"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if data.ID != "d1" || data.Name != "renamed" {
		t.Errorf("data = %+v, want id d1 with the updated name", data)
	}
}

func TestDeleteCDNModSecDataCallsProvider(t *testing.T) {
	provider := &fakeModSecProvider{}
	uc := app.NewDeleteCDNModSecData(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNModSecDataInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", ID: "d1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedDataID != "d1" {
		t.Errorf("deletedDataID = %q, want d1", provider.deletedDataID)
	}
}

// TestDeleteCDNModSecDataTreatsAlreadyGoneAsSuccess proves
// delete_cdn_modsec_data can be called more than once safely.
func TestDeleteCDNModSecDataTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeModSecProvider{deleteDataErr: fmt.Errorf("data d1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteCDNModSecData(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNModSecDataInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", ID: "d1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted data value", err)
	}
}

func TestListCDNModSecRulesReturnsProviderResult(t *testing.T) {
	provider := &fakeModSecProvider{modSecRules: []domain.CDNModSecRule{{ID: "r1", Name: "rule-1", Status: "verified"}}}
	uc := app.NewListCDNModSecRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListCDNModSecRulesInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 1 || rules[0].Status != "verified" {
		t.Errorf("rules = %+v, want a single verified rule", rules)
	}
}

func TestCreateCDNModSecRuleSuccess(t *testing.T) {
	provider := &fakeModSecProvider{}
	uc := app.NewCreateCDNModSecRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.CreateCDNModSecRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Rule: domain.CDNModSecRule{Name: "rule-1", RuleValue: "abc", ModSecDataIDs: []string{"d1"}},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.Name != "rule-1" {
		t.Errorf("Name = %q, want rule-1", rule.Name)
	}
	if provider.createdRule == nil || provider.createdRule.RuleValue != "abc" {
		t.Error("provider was not called with the expected rule")
	}
}

func TestCreateCDNModSecRuleRejectsMissingRuleValue(t *testing.T) {
	provider := &fakeModSecProvider{}
	uc := app.NewCreateCDNModSecRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.CreateCDNModSecRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1",
		Rule: domain.CDNModSecRule{Name: "rule-1"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.createdRule != nil {
		t.Error("provider was called with an invalid rule")
	}
}

func TestGetCDNModSecRuleReturnsProviderResult(t *testing.T) {
	provider := &fakeModSecProvider{getRule: &domain.CDNModSecRule{ID: "r1", RuleValue: "abc"}}
	uc := app.NewGetCDNModSecRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.GetCDNModSecRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", RuleID: "r1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.RuleValue != "abc" {
		t.Errorf("RuleValue = %q, want abc", rule.RuleValue)
	}
}

func TestGetCDNModSecRuleUnknownID(t *testing.T) {
	provider := &fakeModSecProvider{getRuleErr: fmt.Errorf("rule missing: %w", domain.ErrNotFound)}
	uc := app.NewGetCDNModSecRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetCDNModSecRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", RuleID: "missing",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNModSecRuleSuccess(t *testing.T) {
	provider := &fakeModSecProvider{}
	uc := app.NewUpdateCDNModSecRule(&inlineQueue{}, provider)

	rule, err := uc.Execute(context.Background(), app.UpdateCDNModSecRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", RuleID: "r1",
		Rule: domain.CDNModSecRule{Name: "renamed", RuleValue: "new"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rule.ID != "r1" || rule.Name != "renamed" {
		t.Errorf("rule = %+v, want id r1 with the updated name", rule)
	}
}

func TestUpdateCDNModSecRuleRejectsMissingName(t *testing.T) {
	provider := &fakeModSecProvider{}
	uc := app.NewUpdateCDNModSecRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateCDNModSecRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", RuleID: "r1",
		Rule: domain.CDNModSecRule{RuleValue: "new"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
	if provider.updatedRule != nil {
		t.Error("provider was called with an invalid rule")
	}
}

func TestDeleteCDNModSecRuleCallsProvider(t *testing.T) {
	provider := &fakeModSecProvider{}
	uc := app.NewDeleteCDNModSecRule(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNModSecRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", RuleID: "r1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedRuleID != "r1" {
		t.Errorf("deletedRuleID = %q, want r1", provider.deletedRuleID)
	}
}

// TestDeleteCDNModSecRuleTreatsAlreadyGoneAsSuccess proves
// delete_cdn_modsec_rule can be called more than once safely.
func TestDeleteCDNModSecRuleTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeModSecProvider{deleteRuleErr: fmt.Errorf("rule r1: %w", domain.ErrNotFound)}
	uc := app.NewDeleteCDNModSecRule(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteCDNModSecRuleInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"}, ZoneUUID: "z1", RuleID: "r1",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted rule", err)
	}
}
