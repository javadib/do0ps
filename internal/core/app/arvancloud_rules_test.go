package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// fakeArvanCloudRulesProvider embeds the port so a test only needs to
// override the methods it actually exercises, the same pattern
// fakeArvanCloudHealthCheckProvider uses.
type fakeArvanCloudRulesProvider struct {
	ports.ArvanCloudProvider

	pageRules []domain.ArvanCloudPageRuleSummary
	pageMeta  domain.ArvanCloudPageRulePageMeta

	createdRule       domain.ArvanCloudPageRule
	createdRuleDomain string

	updatedRuleID string
	updatedRule   domain.ArvanCloudPageRule

	statusRuleID string
	statusValue  bool

	deleteRuleErr error
	purgeRuleErr  error

	exceptions       domain.ArvanCloudPageRuleExceptions
	updatedExceptID  string
	updatedException domain.ArvanCloudPageRuleExceptions

	transforms    []domain.ArvanCloudResponseTransform
	transformMeta domain.ArvanCloudResponseTransformPageMeta

	createdTransform domain.ArvanCloudResponseTransform
	deleteRTErr      error

	wwwRedirect        domain.ArvanCloudWWWRedirectSettings
	updatedWWWRedirect domain.ArvanCloudWWWRedirectSettings

	whitelist            domain.ArvanCloudHostHeaderWhitelist
	addedTargetAccount   string
	removedTargetAccount string
	settingsGlobal       bool
}

func (p *fakeArvanCloudRulesProvider) ListArvanCloudPageRules(context.Context, domain.ProviderCredentials, string, domain.ArvanCloudPageRuleListQuery) ([]domain.ArvanCloudPageRuleSummary, domain.ArvanCloudPageRulePageMeta, error) {
	return p.pageRules, p.pageMeta, nil
}

func (p *fakeArvanCloudRulesProvider) CreateArvanCloudPageRule(_ context.Context, _ domain.ProviderCredentials, domainName string, rule domain.ArvanCloudPageRule) (*domain.ArvanCloudPageRule, error) {
	p.createdRule = rule
	p.createdRuleDomain = domainName
	created := rule
	created.ID = "pr-1"
	return &created, nil
}

func (p *fakeArvanCloudRulesProvider) GetArvanCloudPageRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudPageRule, error) {
	return &domain.ArvanCloudPageRule{ID: id, Matching: domain.ArvanCloudPageRuleMatching{URL: "/api/*"}}, nil
}

func (p *fakeArvanCloudRulesProvider) UpdateArvanCloudPageRule(_ context.Context, _ domain.ProviderCredentials, _ string, id string, rule domain.ArvanCloudPageRule) (*domain.ArvanCloudPageRule, error) {
	p.updatedRuleID = id
	p.updatedRule = rule
	updated := rule
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudRulesProvider) SetArvanCloudPageRuleStatus(_ context.Context, _ domain.ProviderCredentials, _ string, id string, status bool) error {
	p.statusRuleID = id
	p.statusValue = status
	return nil
}

func (p *fakeArvanCloudRulesProvider) DeleteArvanCloudPageRule(context.Context, domain.ProviderCredentials, string, string) error {
	return p.deleteRuleErr
}

func (p *fakeArvanCloudRulesProvider) PurgeArvanCloudPageRuleCache(context.Context, domain.ProviderCredentials, string, string) error {
	return p.purgeRuleErr
}

func (p *fakeArvanCloudRulesProvider) GetArvanCloudPageRuleExceptions(context.Context, domain.ProviderCredentials, string, string) (*domain.ArvanCloudPageRuleExceptions, error) {
	return &p.exceptions, nil
}

func (p *fakeArvanCloudRulesProvider) UpdateArvanCloudPageRuleExceptions(_ context.Context, _ domain.ProviderCredentials, _ string, id string, exceptions domain.ArvanCloudPageRuleExceptions) (*domain.ArvanCloudPageRuleExceptions, error) {
	p.updatedExceptID = id
	p.updatedException = exceptions
	return &exceptions, nil
}

func (p *fakeArvanCloudRulesProvider) ListArvanCloudResponseTransforms(context.Context, domain.ProviderCredentials, string, domain.ArvanCloudResponseTransformListQuery) ([]domain.ArvanCloudResponseTransform, domain.ArvanCloudResponseTransformPageMeta, error) {
	return p.transforms, p.transformMeta, nil
}

func (p *fakeArvanCloudRulesProvider) CreateArvanCloudResponseTransform(_ context.Context, _ domain.ProviderCredentials, _ string, rt domain.ArvanCloudResponseTransform) (*domain.ArvanCloudResponseTransform, error) {
	p.createdTransform = rt
	created := rt
	created.ID = "rt-1"
	return &created, nil
}

func (p *fakeArvanCloudRulesProvider) GetArvanCloudResponseTransform(_ context.Context, _ domain.ProviderCredentials, _ string, id string) (*domain.ArvanCloudResponseTransform, error) {
	return &domain.ArvanCloudResponseTransform{ID: id, Name: "preset"}, nil
}

func (p *fakeArvanCloudRulesProvider) UpdateArvanCloudResponseTransform(_ context.Context, _ domain.ProviderCredentials, _ string, id string, rt domain.ArvanCloudResponseTransform) (*domain.ArvanCloudResponseTransform, error) {
	updated := rt
	updated.ID = id
	return &updated, nil
}

func (p *fakeArvanCloudRulesProvider) DeleteArvanCloudResponseTransform(context.Context, domain.ProviderCredentials, string, string) error {
	return p.deleteRTErr
}

func (p *fakeArvanCloudRulesProvider) GetArvanCloudWWWRedirect(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudWWWRedirectSettings, error) {
	return &p.wwwRedirect, nil
}

func (p *fakeArvanCloudRulesProvider) UpdateArvanCloudWWWRedirect(_ context.Context, _ domain.ProviderCredentials, _ string, settings domain.ArvanCloudWWWRedirectSettings) error {
	p.updatedWWWRedirect = settings
	return nil
}

func (p *fakeArvanCloudRulesProvider) GetArvanCloudHostHeaderWhitelist(context.Context, domain.ProviderCredentials, string) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	return &p.whitelist, nil
}

func (p *fakeArvanCloudRulesProvider) AddArvanCloudHostHeaderWhitelistEntry(_ context.Context, _ domain.ProviderCredentials, _ string, targetAccount string) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	p.addedTargetAccount = targetAccount
	return &p.whitelist, nil
}

func (p *fakeArvanCloudRulesProvider) SetArvanCloudHostHeaderWhitelistSettings(_ context.Context, _ domain.ProviderCredentials, _ string, global bool) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	p.settingsGlobal = global
	return &p.whitelist, nil
}

func (p *fakeArvanCloudRulesProvider) RemoveArvanCloudHostHeaderWhitelistEntry(_ context.Context, _ domain.ProviderCredentials, _ string, targetAccount string) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	p.removedTargetAccount = targetAccount
	return &p.whitelist, nil
}

// validArvanCloudPageRule returns a page rule with one field from every
// logical group set, valid enough to pass validateArvanCloudPageRuleInput.
func validArvanCloudPageRule() domain.ArvanCloudPageRule {
	return domain.ArvanCloudPageRule{
		Matching: domain.ArvanCloudPageRuleMatching{URL: "/api/*", Status: true},
		Caching: domain.ArvanCloudPageRuleCaching{
			CacheLevel: domain.ArvanCloudPageRuleCacheLevelURI, Cache200: domain.ArvanCloudCacheTTL30m,
			CacheAny: domain.ArvanCloudCacheTTL0s, CacheBrowser: domain.ArvanCloudCacheTTLDefault,
		},
		Security: domain.ArvanCloudPageRuleSecurity{WAFStatus: true},
		Routing:  domain.ArvanCloudPageRuleRouting{LoadBalancer: "lb-1"},
		Headers:  domain.ArvanCloudPageRuleHeaders{CORSHeader: "*"},
		Other: domain.ArvanCloudPageRuleOther{
			Acceleration: domain.ArvanCloudAccelerationSettings{Status: domain.ArvanCloudAccelerationOn},
		},
	}
}

func TestCreateArvanCloudPageRuleSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{}
	uc := app.NewCreateArvanCloudPageRule(&inlineQueue{}, provider)

	created, err := uc.Execute(context.Background(), app.CreateArvanCloudPageRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Rule: validArvanCloudPageRule(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "pr-1" || provider.createdRuleDomain != "example.com" {
		t.Errorf("created = %+v, provider.createdRuleDomain = %q, want id=pr-1 domain=example.com", created, provider.createdRuleDomain)
	}
}

// TestCreateArvanCloudPageRuleRejectsMissingURL proves url (Matching group)
// is required.
func TestCreateArvanCloudPageRuleRejectsMissingURL(t *testing.T) {
	rule := validArvanCloudPageRule()
	rule.Matching.URL = ""
	uc := app.NewCreateArvanCloudPageRule(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	_, err := uc.Execute(context.Background(), app.CreateArvanCloudPageRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Rule: rule,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// TestCreateArvanCloudPageRuleRejectsBadCacheTTL proves cache_200, cache_any
// and cache_browser are all validated through the same shared TTL enum
// (issue #71's acceptance criterion) — an invalid value on any of the three
// is rejected.
func TestCreateArvanCloudPageRuleRejectsBadCacheTTL(t *testing.T) {
	cases := []struct {
		name  string
		apply func(*domain.ArvanCloudPageRule)
	}{
		{"cache_200", func(r *domain.ArvanCloudPageRule) { r.Caching.Cache200 = "31d" }},
		{"cache_any", func(r *domain.ArvanCloudPageRule) { r.Caching.CacheAny = "2 minutes" }},
		{"cache_browser", func(r *domain.ArvanCloudPageRule) { r.Caching.CacheBrowser = "sometimes" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := validArvanCloudPageRule()
			tc.apply(&rule)
			uc := app.NewCreateArvanCloudPageRule(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
			_, err := uc.Execute(context.Background(), app.CreateArvanCloudPageRuleInput{
				Credentials: validArvanCloudCreds(), Domain: "example.com", Rule: rule,
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
			}
		})
	}
}

// TestCreateArvanCloudPageRuleAcceptsCacheBrowserDefault proves
// cache_browser's own extra "default" sentinel is accepted even though
// cache_200/cache_any do not declare it.
func TestCreateArvanCloudPageRuleAcceptsCacheBrowserDefault(t *testing.T) {
	rule := validArvanCloudPageRule()
	rule.Caching.CacheBrowser = domain.ArvanCloudCacheTTLDefault
	uc := app.NewCreateArvanCloudPageRule(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	if _, err := uc.Execute(context.Background(), app.CreateArvanCloudPageRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Rule: rule,
	}); err != nil {
		t.Errorf("Execute() error = %v, want success for cache_browser=default", err)
	}
}

// TestCreateArvanCloudPageRuleRejectsBadRedirectStatusCode proves the
// Routing group's redirect.status_code is validated.
func TestCreateArvanCloudPageRuleRejectsBadRedirectStatusCode(t *testing.T) {
	rule := validArvanCloudPageRule()
	rule.Routing.Redirect = domain.ArvanCloudPageRuleRedirect{Enable: true, StatusCode: 418, URL: "https://example.com"}
	uc := app.NewCreateArvanCloudPageRule(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	_, err := uc.Execute(context.Background(), app.CreateArvanCloudPageRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Rule: rule,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput for status_code=418", err)
	}
}

func TestGetArvanCloudPageRuleMissingID(t *testing.T) {
	uc := app.NewGetArvanCloudPageRule(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	_, err := uc.Execute(context.Background(), app.GetArvanCloudPageRuleInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestListArvanCloudPageRulesSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{
		pageRules: []domain.ArvanCloudPageRuleSummary{{ID: "pr-1", URL: "/api/*"}},
		pageMeta:  domain.ArvanCloudPageRulePageMeta{Total: 1},
	}
	uc := app.NewListArvanCloudPageRules(&inlineQueue{}, provider)
	result, err := uc.Execute(context.Background(), app.ListArvanCloudPageRulesInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Rules) != 1 || result.Rules[0].ID != "pr-1" || result.Page.Total != 1 {
		t.Errorf("result = %+v, want one rule id=pr-1 and Page.Total=1", result)
	}
}

func TestSetArvanCloudPageRuleStatusSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{}
	uc := app.NewSetArvanCloudPageRuleStatus(&inlineQueue{}, provider)
	err := uc.Execute(context.Background(), app.SetArvanCloudPageRuleStatusInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "pr-1", Status: false,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.statusRuleID != "pr-1" || provider.statusValue != false {
		t.Errorf("provider.statusRuleID = %q, provider.statusValue = %v, want pr-1 and false", provider.statusRuleID, provider.statusValue)
	}
}

func TestDeleteArvanCloudPageRuleToleratesNotFound(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{deleteRuleErr: domain.ErrNotFound}
	uc := app.NewDeleteArvanCloudPageRule(&inlineQueue{}, provider)
	err := uc.Execute(context.Background(), app.DeleteArvanCloudPageRuleInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "missing",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil (tolerant delete)", err)
	}
}

func TestPurgeArvanCloudPageRuleCacheSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{}
	uc := app.NewPurgeArvanCloudPageRuleCache(&inlineQueue{}, provider)
	err := uc.Execute(context.Background(), app.PurgeArvanCloudPageRuleCacheInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "pr-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestGetArvanCloudPageRuleExceptionsSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{exceptions: domain.ArvanCloudPageRuleExceptions{URL: "/override"}}
	uc := app.NewGetArvanCloudPageRuleExceptions(&inlineQueue{}, provider)
	exceptions, err := uc.Execute(context.Background(), app.GetArvanCloudPageRuleExceptionsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "pr-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if exceptions.URL != "/override" {
		t.Errorf("exceptions.URL = %q, want /override", exceptions.URL)
	}
}

// TestUpdateArvanCloudPageRuleExceptionsRejectsBadIsVar proves the exception
// header entries' is_var field is validated against its "true"/"false"/empty
// enum.
func TestUpdateArvanCloudPageRuleExceptionsRejectsBadIsVar(t *testing.T) {
	exceptions := domain.ArvanCloudPageRuleExceptions{
		URL:              "/override",
		ReqCustomHeaders: []domain.ArvanCloudPageRuleExceptionHeaderEntry{{Name: "X-A", Value: "1", IsVar: "yes"}},
	}
	uc := app.NewUpdateArvanCloudPageRuleExceptions(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	_, err := uc.Execute(context.Background(), app.UpdateArvanCloudPageRuleExceptionsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "pr-1", Exceptions: exceptions,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput for is_var=yes", err)
	}
}

func TestUpdateArvanCloudPageRuleExceptionsSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{}
	uc := app.NewUpdateArvanCloudPageRuleExceptions(&inlineQueue{}, provider)
	exceptions := domain.ArvanCloudPageRuleExceptions{URL: "/override", CacheLevel: domain.ArvanCloudPageRuleCacheLevelURI}
	updated, err := uc.Execute(context.Background(), app.UpdateArvanCloudPageRuleExceptionsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "pr-1", Exceptions: exceptions,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.URL != "/override" || provider.updatedExceptID != "pr-1" {
		t.Errorf("updated = %+v, provider.updatedExceptID = %q, want URL=/override id=pr-1", updated, provider.updatedExceptID)
	}
}

// --- Response Transforms ---------------------------------------------------

func validArvanCloudResponseTransform() domain.ArvanCloudResponseTransform {
	return domain.ArvanCloudResponseTransform{
		Name: "cors-preset",
		Transforms: []domain.ArvanCloudResponseTransformStep{{
			Condition: `http.request.method == "GET"`,
			Actions: []domain.ArvanCloudResponseTransformAction{{
				Key: domain.ArvanCloudResponseTransformHeaderAllowOrigin, Value: `to_string("*")`,
			}},
		}},
	}
}

func TestCreateArvanCloudResponseTransformSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{}
	uc := app.NewCreateArvanCloudResponseTransform(&inlineQueue{}, provider)
	created, err := uc.Execute(context.Background(), app.CreateArvanCloudResponseTransformInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Transform: validArvanCloudResponseTransform(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if created.ID != "rt-1" {
		t.Errorf("created.ID = %q, want rt-1", created.ID)
	}
}

// TestCreateArvanCloudResponseTransformRejectsMissingName proves name is
// required.
func TestCreateArvanCloudResponseTransformRejectsMissingName(t *testing.T) {
	rt := validArvanCloudResponseTransform()
	rt.Name = ""
	uc := app.NewCreateArvanCloudResponseTransform(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	_, err := uc.Execute(context.Background(), app.CreateArvanCloudResponseTransformInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Transform: rt,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// TestCreateArvanCloudResponseTransformRejectsEmptyTransforms proves at
// least one step is required.
func TestCreateArvanCloudResponseTransformRejectsEmptyTransforms(t *testing.T) {
	rt := validArvanCloudResponseTransform()
	rt.Transforms = nil
	uc := app.NewCreateArvanCloudResponseTransform(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	_, err := uc.Execute(context.Background(), app.CreateArvanCloudResponseTransformInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Transform: rt,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

// TestCreateArvanCloudResponseTransformRejectsBadKey proves each action's
// key is validated against the allowed CORS header enum.
func TestCreateArvanCloudResponseTransformRejectsBadKey(t *testing.T) {
	rt := validArvanCloudResponseTransform()
	rt.Transforms[0].Actions[0].Key = "X-Custom-Header"
	uc := app.NewCreateArvanCloudResponseTransform(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	_, err := uc.Execute(context.Background(), app.CreateArvanCloudResponseTransformInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Transform: rt,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteArvanCloudResponseTransformToleratesNotFound(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{deleteRTErr: domain.ErrNotFound}
	uc := app.NewDeleteArvanCloudResponseTransform(&inlineQueue{}, provider)
	err := uc.Execute(context.Background(), app.DeleteArvanCloudResponseTransformInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", ID: "missing",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil (tolerant delete)", err)
	}
}

func TestListArvanCloudResponseTransformsSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{
		transforms:    []domain.ArvanCloudResponseTransform{{ID: "rt-1", Name: "cors-preset"}},
		transformMeta: domain.ArvanCloudResponseTransformPageMeta{Total: 1},
	}
	uc := app.NewListArvanCloudResponseTransforms(&inlineQueue{}, provider)
	result, err := uc.Execute(context.Background(), app.ListArvanCloudResponseTransformsInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Transforms) != 1 || result.Transforms[0].ID != "rt-1" {
		t.Errorf("result = %+v, want one transform id=rt-1", result)
	}
}

// --- Redirect (www-redirect) ------------------------------------------------

func TestGetArvanCloudWWWRedirectSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{wwwRedirect: domain.ArvanCloudWWWRedirectSettings{Mode: domain.ArvanCloudWWWRedirectToWWW}}
	uc := app.NewGetArvanCloudWWWRedirect(&inlineQueue{}, provider)
	settings, err := uc.Execute(context.Background(), app.GetArvanCloudWWWRedirectInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if settings.Mode != domain.ArvanCloudWWWRedirectToWWW {
		t.Errorf("settings.Mode = %q, want %q", settings.Mode, domain.ArvanCloudWWWRedirectToWWW)
	}
}

// TestUpdateArvanCloudWWWRedirectRejectsBadMode proves mode is validated
// against its enum.
func TestUpdateArvanCloudWWWRedirectRejectsBadMode(t *testing.T) {
	uc := app.NewUpdateArvanCloudWWWRedirect(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	err := uc.Execute(context.Background(), app.UpdateArvanCloudWWWRedirectInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Settings: domain.ArvanCloudWWWRedirectSettings{Mode: "sometimes"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateArvanCloudWWWRedirectSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{}
	uc := app.NewUpdateArvanCloudWWWRedirect(&inlineQueue{}, provider)
	err := uc.Execute(context.Background(), app.UpdateArvanCloudWWWRedirectInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Settings: domain.ArvanCloudWWWRedirectSettings{Mode: domain.ArvanCloudWWWRedirectToRoot},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.updatedWWWRedirect.Mode != domain.ArvanCloudWWWRedirectToRoot {
		t.Errorf("provider.updatedWWWRedirect.Mode = %q, want %q", provider.updatedWWWRedirect.Mode, domain.ArvanCloudWWWRedirectToRoot)
	}
}

// --- Host Header Whitelist --------------------------------------------------

func TestGetArvanCloudHostHeaderWhitelistSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{whitelist: domain.ArvanCloudHostHeaderWhitelist{GloballyWhitelisted: true}}
	uc := app.NewGetArvanCloudHostHeaderWhitelist(&inlineQueue{}, provider)
	whitelist, err := uc.Execute(context.Background(), app.GetArvanCloudHostHeaderWhitelistInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !whitelist.GloballyWhitelisted {
		t.Errorf("whitelist.GloballyWhitelisted = false, want true")
	}
}

// TestAddArvanCloudHostHeaderWhitelistEntryMissingTargetAccount proves
// target_account is required.
func TestAddArvanCloudHostHeaderWhitelistEntryMissingTargetAccount(t *testing.T) {
	uc := app.NewAddArvanCloudHostHeaderWhitelistEntry(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	_, err := uc.Execute(context.Background(), app.AddArvanCloudHostHeaderWhitelistEntryInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestAddArvanCloudHostHeaderWhitelistEntrySuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{}
	uc := app.NewAddArvanCloudHostHeaderWhitelistEntry(&inlineQueue{}, provider)
	_, err := uc.Execute(context.Background(), app.AddArvanCloudHostHeaderWhitelistEntryInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", TargetAccount: "acct-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.addedTargetAccount != "acct-1" {
		t.Errorf("provider.addedTargetAccount = %q, want acct-1", provider.addedTargetAccount)
	}
}

func TestSetArvanCloudHostHeaderWhitelistSettingsSuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{}
	uc := app.NewSetArvanCloudHostHeaderWhitelistSettings(&inlineQueue{}, provider)
	_, err := uc.Execute(context.Background(), app.SetArvanCloudHostHeaderWhitelistSettingsInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", Global: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !provider.settingsGlobal {
		t.Errorf("provider.settingsGlobal = false, want true")
	}
}

func TestRemoveArvanCloudHostHeaderWhitelistEntryMissingTargetAccount(t *testing.T) {
	uc := app.NewRemoveArvanCloudHostHeaderWhitelistEntry(&inlineQueue{}, &fakeArvanCloudRulesProvider{})
	_, err := uc.Execute(context.Background(), app.RemoveArvanCloudHostHeaderWhitelistEntryInput{Credentials: validArvanCloudCreds(), Domain: "example.com"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Execute() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestRemoveArvanCloudHostHeaderWhitelistEntrySuccess(t *testing.T) {
	provider := &fakeArvanCloudRulesProvider{}
	uc := app.NewRemoveArvanCloudHostHeaderWhitelistEntry(&inlineQueue{}, provider)
	_, err := uc.Execute(context.Background(), app.RemoveArvanCloudHostHeaderWhitelistEntryInput{
		Credentials: validArvanCloudCreds(), Domain: "example.com", TargetAccount: "acct-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.removedTargetAccount != "acct-1" {
		t.Errorf("provider.removedTargetAccount = %q, want acct-1", provider.removedTargetAccount)
	}
}
