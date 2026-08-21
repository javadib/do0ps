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

// fakeArvanCloudRateLimitProvider embeds the port so a test only needs to
// override the methods it actually exercises, the same pattern as
// fakeArvanCloudDdosProvider.
type fakeArvanCloudRateLimitProvider struct {
	ports.ArvanCloudProvider

	settings        domain.ArvanCloudRateLimitSettings
	updatedSettings domain.ArvanCloudRateLimitSettings

	rules []domain.ArvanCloudRateLimitRule

	createdRule       domain.ArvanCloudRateLimitRule
	createdRuleDomain string

	updatedRuleID string
	updatedRule   domain.ArvanCloudRateLimitRule

	deleteRuleErr error

	reprioritizedDomain string
	reprioritizedRuleID string
	reprioritizedAfter  string
	reprioritizedBefore string
}

func (p *fakeArvanCloudRateLimitProvider) GetArvanCloudRateLimitSettings(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudRateLimitSettings, error) {
	settings := p.settings
	return &settings, nil
}

func (p *fakeArvanCloudRateLimitProvider) UpdateArvanCloudRateLimitSettings(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.ArvanCloudRateLimitSettings) (*domain.ArvanCloudRateLimitSettings, error) {
	p.updatedSettings = settings
	return &settings, nil
}

func (p *fakeArvanCloudRateLimitProvider) ListArvanCloudRateLimitRules(context.Context, domain.ProviderCredentials, string) ([]domain.ArvanCloudRateLimitRule, error) {
	return p.rules, nil
}

func (p *fakeArvanCloudRateLimitProvider) CreateArvanCloudRateLimitRule(_ context.Context, _ domain.ProviderCredentials, domainName string, rule domain.ArvanCloudRateLimitRule) (*domain.ArvanCloudRateLimitRule, error) {
	p.createdRule = rule
	p.createdRuleDomain = domainName
	created := rule
	created.ID = "rule-1"
	return &created, nil
}

func (p *fakeArvanCloudRateLimitProvider) GetArvanCloudRateLimitRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudRateLimitRule, error) {
	for i := range p.rules {
		if p.rules[i].ID == id {
			return &p.rules[i], nil
		}
	}
	return nil, fmt.Errorf("rate limit rule %q: %w", id, domain.ErrNotFound)
}

func (p *fakeArvanCloudRateLimitProvider) UpdateArvanCloudRateLimitRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string, rule domain.ArvanCloudRateLimitRule) (*domain.ArvanCloudRateLimitRule, error) {
	p.updatedRuleID = id
	p.updatedRule = rule
	updated := rule
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudRateLimitProvider) DeleteArvanCloudRateLimitRule(_ context.Context, _ domain.ProviderCredentials, _ string, _ string) error {
	return p.deleteRuleErr
}

func (p *fakeArvanCloudRateLimitProvider) ReprioritizeArvanCloudRateLimitRules(_ context.Context, _ domain.ProviderCredentials, domainName, ruleID, afterRuleID, beforeRuleID string) error {
	p.reprioritizedDomain = domainName
	p.reprioritizedRuleID = ruleID
	p.reprioritizedAfter = afterRuleID
	p.reprioritizedBefore = beforeRuleID
	return nil
}

// --- Per-domain rate-limit settings -----------------------------------------

func TestGetArvanCloudRateLimitSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudRateLimitProvider{settings: domain.ArvanCloudRateLimitSettings{
		DDoSDetection: true, ExcludeSources: []string{"203.0.113.0/24"},
	}}
	uc := app.NewGetArvanCloudRateLimitSettings(&inlineQueue{}, provider)

	settings, err := uc.Execute(context.Background(), app.GetArvanCloudRateLimitSettingsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !settings.DDoSDetection {
		t.Errorf("settings.DDoSDetection = %v, want true", settings.DDoSDetection)
	}
}

func TestGetArvanCloudRateLimitSettingsMissingDomain(t *testing.T) {
	uc := app.NewGetArvanCloudRateLimitSettings(&inlineQueue{}, &fakeArvanCloudRateLimitProvider{})
	if _, err := uc.Execute(context.Background(), app.GetArvanCloudRateLimitSettingsInput{Credentials: validArvanCloudCreds()}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateArvanCloudRateLimitSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudRateLimitProvider{}
	uc := app.NewUpdateArvanCloudRateLimitSettings(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudRateLimitSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com",
		Settings: domain.ArvanCloudRateLimitSettings{DDoSDetection: true},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !provider.updatedSettings.DDoSDetection {
		t.Errorf("provider.updatedSettings.DDoSDetection = %v, want true", provider.updatedSettings.DDoSDetection)
	}
}

// --- Per-domain rate-limit rules ---------------------------------------------

func validArvanCloudRateLimitRule() domain.ArvanCloudRateLimitRule {
	return domain.ArvanCloudRateLimitRule{
		URLPattern:   "/api/**",
		Action:       domain.ArvanCloudRateLimitActionBlock,
		Rate:         100,
		TimeDuration: 60,
	}
}

func TestCreateArvanCloudRateLimitRuleSuccess(t *testing.T) {
	provider := &fakeArvanCloudRateLimitProvider{}
	uc := app.NewCreateArvanCloudRateLimitRule(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudRateLimitRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Rule: validArvanCloudRateLimitRule(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "rule-1" {
		t.Errorf("created.ID = %q, want %q", created.ID, "rule-1")
	}
}

// TestCreateArvanCloudRateLimitRuleRejectsNonPositiveRateOrTimeDuration
// covers issue #68's acceptance criteria: rate and time_duration — the
// spec's two required numeric fields — must be rejected client-side when
// zero or negative, since a rate limit of 0-per-0-seconds is meaningless and
// better caught before the call than surfaced as an opaque 422.
func TestCreateArvanCloudRateLimitRuleRejectsNonPositiveRateOrTimeDuration(t *testing.T) {
	tests := []struct {
		name string
		rule domain.ArvanCloudRateLimitRule
	}{
		{"zero rate", func() domain.ArvanCloudRateLimitRule { r := validArvanCloudRateLimitRule(); r.Rate = 0; return r }()},
		{"negative rate", func() domain.ArvanCloudRateLimitRule { r := validArvanCloudRateLimitRule(); r.Rate = -1; return r }()},
		{"zero time_duration", func() domain.ArvanCloudRateLimitRule {
			r := validArvanCloudRateLimitRule()
			r.TimeDuration = 0
			return r
		}()},
		{"negative time_duration", func() domain.ArvanCloudRateLimitRule {
			r := validArvanCloudRateLimitRule()
			r.TimeDuration = -1
			return r
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &fakeArvanCloudRateLimitProvider{}
			uc := app.NewCreateArvanCloudRateLimitRule(&inlineQueue{}, provider)

			_, err := uc.Execute(context.Background(), app.CreateArvanCloudRateLimitRuleInput{
				Credentials: validArvanCloudCreds(), Domain: "example.com", Rule: tc.rule,
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
			if provider.createdRuleDomain != "" {
				t.Errorf("provider was called with an invalid rule, want validation to reject it first")
			}
		})
	}
}

func TestCreateArvanCloudRateLimitRuleInvalidAction(t *testing.T) {
	uc := app.NewCreateArvanCloudRateLimitRule(&inlineQueue{}, &fakeArvanCloudRateLimitProvider{})
	rule := validArvanCloudRateLimitRule()
	rule.Action = "throttle"

	_, err := uc.Execute(context.Background(), app.CreateArvanCloudRateLimitRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Rule: rule,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// TestCreateArvanCloudRateLimitRuleChallengeRequiresValidMode mirrors
// TestUpdateArvanCloudDdosSettingsCaptchaServiceOnlyRequiredForCaptchaMode's
// table-driven "only required for the matching branch" shape: action_details
// .mode is validated only when action is "challenge".
func TestCreateArvanCloudRateLimitRuleChallengeRequiresValidMode(t *testing.T) {
	tests := []struct {
		name    string
		rule    domain.ArvanCloudRateLimitRule
		wantErr bool
	}{
		{
			name: "block action, no mode set, is fine",
			rule: func() domain.ArvanCloudRateLimitRule {
				r := validArvanCloudRateLimitRule()
				r.Action = domain.ArvanCloudRateLimitActionBlock
				return r
			}(),
			wantErr: false,
		},
		{
			name: "challenge action, valid mode",
			rule: func() domain.ArvanCloudRateLimitRule {
				r := validArvanCloudRateLimitRule()
				r.Action = domain.ArvanCloudRateLimitActionChallenge
				r.ActionDetails = domain.ArvanCloudChallengeAction{Mode: domain.ArvanCloudChallengeModeCaptcha, TTL: 600}
				return r
			}(),
			wantErr: false,
		},
		{
			name: "challenge action, missing mode",
			rule: func() domain.ArvanCloudRateLimitRule {
				r := validArvanCloudRateLimitRule()
				r.Action = domain.ArvanCloudRateLimitActionChallenge
				return r
			}(),
			wantErr: true,
		},
		{
			name: "challenge action, out-of-range mode",
			rule: func() domain.ArvanCloudRateLimitRule {
				r := validArvanCloudRateLimitRule()
				r.Action = domain.ArvanCloudRateLimitActionChallenge
				r.ActionDetails = domain.ArvanCloudChallengeAction{Mode: 9}
				return r
			}(),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := app.NewCreateArvanCloudRateLimitRule(&inlineQueue{}, &fakeArvanCloudRateLimitProvider{})
			_, err := uc.Execute(context.Background(), app.CreateArvanCloudRateLimitRuleInput{
				Credentials: validArvanCloudCreds(), Domain: "example.com", Rule: tc.rule,
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

func TestListArvanCloudRateLimitRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudRateLimitProvider{rules: []domain.ArvanCloudRateLimitRule{validArvanCloudRateLimitRule()}}
	uc := app.NewListArvanCloudRateLimitRules(&inlineQueue{}, provider)

	rules, err := uc.Execute(context.Background(), app.ListArvanCloudRateLimitRulesInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("len(rules) = %d, want 1", len(rules))
	}
}

func TestGetArvanCloudRateLimitRuleNotFound(t *testing.T) {
	provider := &fakeArvanCloudRateLimitProvider{}
	uc := app.NewGetArvanCloudRateLimitRule(&inlineQueue{}, provider)

	_, err := uc.Execute(context.Background(), app.GetArvanCloudRateLimitRuleInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "missing"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Execute() error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateArvanCloudRateLimitRuleRejectsNonPositiveRate(t *testing.T) {
	uc := app.NewUpdateArvanCloudRateLimitRule(&inlineQueue{}, &fakeArvanCloudRateLimitProvider{})
	rule := validArvanCloudRateLimitRule()
	rule.Rate = 0

	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudRateLimitRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "rule-1", Rule: rule,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteArvanCloudRateLimitRuleTolerantOfNotFound(t *testing.T) {
	provider := &fakeArvanCloudRateLimitProvider{deleteRuleErr: fmt.Errorf("gone: %w", domain.ErrNotFound)}
	uc := app.NewDeleteArvanCloudRateLimitRule(&inlineQueue{}, provider)

	if err := uc.Execute(context.Background(), app.DeleteArvanCloudRateLimitRuleInput{Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "rule-1"}); err != nil {
		t.Errorf("Execute() error = %v, want nil (tolerant delete)", err)
	}
}

func TestReprioritizeArvanCloudRateLimitRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudRateLimitProvider{}
	uc := app.NewReprioritizeArvanCloudRateLimitRules(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudRateLimitRulesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", RuleID: "rule-1", AfterRuleID: "rule-2",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.reprioritizedRuleID != "rule-1" || provider.reprioritizedAfter != "rule-2" {
		t.Errorf("provider reprioritize call = ruleID:%q after:%q, want rule-1/rule-2", provider.reprioritizedRuleID, provider.reprioritizedAfter)
	}
}

func TestReprioritizeArvanCloudRateLimitRulesRequiresExactlyOneOfAfterBefore(t *testing.T) {
	uc := app.NewReprioritizeArvanCloudRateLimitRules(&inlineQueue{}, &fakeArvanCloudRateLimitProvider{})

	err := uc.Execute(context.Background(), app.ReprioritizeArvanCloudRateLimitRulesInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", RuleID: "rule-1",
		AfterRuleID: "rule-2", BeforeRuleID: "rule-3",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}
