package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the use cases for ArvanCloud's Page Rules, Response
// Transforms, Redirect (www-redirect) and Host Header Whitelist (issue #71)
// — see domain/arvancloud_rules.go's package comment for what each resource
// is. Every one of them is a fast operation (ports.ArvanCloudProvider,
// AGENTS.md 4.3): each dispatches onto the queue and blocks for the result
// within the same tool call.
//
// What IS validated client-side, per issue #71's acceptance criteria:
//
//   - Page Rule: Matching.URL required; Matching.URLType,
//     Caching.CacheLevel, Caching.Cache200/CacheAny (the shared TTL enum,
//     validated once via validateArvanCloudCacheTTLGroup and reused for both
//     fields) and Caching.CacheBrowser (the same shared enum plus its extra
//     "default" sentinel), Security.SlinkMD5 entries,
//     Other.ImageResize.Status/Mode and Routing.Redirect.StatusCode are all
//     checked against their respective domain.ValidArvanCloud... enums.
//   - Page Rule exceptions: the same field-group validation as Page Rule,
//     applied to domain.ArvanCloudPageRuleExceptions' overlapping fields.
//   - Response Transform: Name and at least one Transforms step required;
//     each step's Condition length and each action's Type/Mode/Key against
//     their enums.
//   - Redirect: Mode against domain.ValidArvanCloudWWWRedirectMode.
//   - Host Header Whitelist: TargetAccount required for add/remove.

// arvanCloudDomainInput is embedded by every use case below that is scoped
// to exactly one domain by name and needs nothing else. Named distinctly
// from arvanCloudHealthCheckDomainInput (arvancloud_healthcheck.go) even
// though the shape is identical, matching this package's existing
// per-resource-family convention rather than sharing one input type across
// unrelated resources.
type arvanCloudDomainInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

func (in arvanCloudDomainInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// arvanCloudRuleIDInput is embedded by every use case below that is scoped
// to exactly one resource by domain + id.
type arvanCloudRuleIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
}

func (in arvanCloudRuleIDInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// --- Page Rules ---------------------------------------------------------

// validateArvanCloudCacheTTLGroup checks cache_200/cache_any/cache_browser
// against the ONE shared TTL enum (domain.ArvanCloudCacheTTL): Cache200 and
// CacheAny satisfy domain.ValidArvanCloudCacheTTL, CacheBrowser satisfies
// domain.ValidArvanCloudCacheBrowserTTL (the same list, plus its own extra
// "default" sentinel) — see this file's package comment. Extracted into its
// own function so both validateArvanCloudPageRuleInput and
// validateArvanCloudPageRuleExceptionsInput call the exact same check rather
// than duplicating it.
func validateArvanCloudCacheTTLGroup(cache200, cacheAny, cacheBrowser domain.ArvanCloudCacheTTL) error {
	if cache200 != "" && !domain.ValidArvanCloudCacheTTL(string(cache200)) {
		return fmt.Errorf("cache_200 %q is not a valid cache TTL: %w", cache200, domain.ErrInvalidInput)
	}
	if cacheAny != "" && !domain.ValidArvanCloudCacheTTL(string(cacheAny)) {
		return fmt.Errorf("cache_any %q is not a valid cache TTL: %w", cacheAny, domain.ErrInvalidInput)
	}
	if cacheBrowser != "" && !domain.ValidArvanCloudCacheBrowserTTL(string(cacheBrowser)) {
		return fmt.Errorf("cache_browser %q is not a valid cache TTL (or \"default\"): %w", cacheBrowser, domain.ErrInvalidInput)
	}
	return nil
}

// validateArvanCloudPageRuleInput checks the fields every create/update page
// rule call shares, one per logical group at minimum (issue #71's own
// acceptance criterion).
func validateArvanCloudPageRuleInput(pr domain.ArvanCloudPageRule) error {
	// Matching.
	if pr.Matching.URL == "" {
		return fmt.Errorf("url is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudPageRuleURLType(string(pr.Matching.URLType)) {
		return fmt.Errorf("url_type %q is not one of default/index/directory/extension/page/regex: %w", pr.Matching.URLType, domain.ErrInvalidInput)
	}

	// Caching.
	if !domain.ValidArvanCloudPageRuleCacheLevel(string(pr.Caching.CacheLevel)) {
		return fmt.Errorf("cache_level %q is not one of off/uri/query_string: %w", pr.Caching.CacheLevel, domain.ErrInvalidInput)
	}
	if err := validateArvanCloudCacheTTLGroup(pr.Caching.Cache200, pr.Caching.CacheAny, pr.Caching.CacheBrowser); err != nil {
		return err
	}

	// Security.
	for _, field := range pr.Security.SlinkMD5 {
		if !domain.ValidArvanCloudPageRuleSlinkMD5Field(string(field)) {
			return fmt.Errorf("slink_md5 entry %q is not one of remote_addr/file/expires/url/uri: %w", field, domain.ErrInvalidInput)
		}
	}

	// Routing.
	if !domain.ValidArvanCloudPageRuleRedirectStatusCode(pr.Routing.Redirect.StatusCode) {
		return fmt.Errorf("redirect.status_code %d is not one of 301/302/307: %w", pr.Routing.Redirect.StatusCode, domain.ErrInvalidInput)
	}

	// Other.
	if pr.Other.Acceleration.Status != "" && !domain.ValidArvanCloudAccelerationStatus(string(pr.Other.Acceleration.Status)) {
		return fmt.Errorf("acceleration.status %q is not one of inherit/on/off: %w", pr.Other.Acceleration.Status, domain.ErrInvalidInput)
	}
	for _, ext := range pr.Other.Acceleration.Extensions {
		if !domain.ValidArvanCloudAccelerationExtension(string(ext)) {
			return fmt.Errorf("acceleration.extensions entry %q is not one of css/gif/jpeg/js/png: %w", ext, domain.ErrInvalidInput)
		}
	}
	if !domain.ValidArvanCloudPageRuleImageResizeStatus(string(pr.Other.ImageResize.Status)) {
		return fmt.Errorf("image_resize.status %q is not one of on/off/inherit: %w", pr.Other.ImageResize.Status, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudImageResizeMode(string(pr.Other.ImageResize.Mode)) {
		return fmt.Errorf("image_resize.mode %q is not one of freely/short-side/long-side: %w", pr.Other.ImageResize.Mode, domain.ErrInvalidInput)
	}
	return nil
}

// ListArvanCloudPageRulesInput identifies the domain and query for
// ListArvanCloudPageRules.
type ListArvanCloudPageRulesInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Query       domain.ArvanCloudPageRuleListQuery
}

// ListArvanCloudPageRulesResult pairs one page of page rules with its
// pagination info, the return shape ListArvanCloudPageRules' Execute needs
// since the queue's Dispatch carries exactly one JSON value.
type ListArvanCloudPageRulesResult struct {
	Rules []domain.ArvanCloudPageRuleSummary
	Page  domain.ArvanCloudPageRulePageMeta
}

// ListArvanCloudPageRules is a fast operation.
type ListArvanCloudPageRules struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudPageRules builds the use case from its ports.
func NewListArvanCloudPageRules(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudPageRules {
	return &ListArvanCloudPageRules{queue: queue, provider: provider}
}

// Execute returns one page of the domain's page rules.
func (uc *ListArvanCloudPageRules) Execute(ctx context.Context, in ListArvanCloudPageRulesInput) (*ListArvanCloudPageRulesResult, error) {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		rules, page, err := uc.provider.ListArvanCloudPageRules(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud page rules of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(ListArvanCloudPageRulesResult{Rules: rules, Page: page})
	})
	if err != nil {
		return nil, err
	}

	var result ListArvanCloudPageRulesResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud page rule list: %w", err)
	}
	return &result, nil
}

// CreateArvanCloudPageRuleInput identifies the domain and the new rule's
// field values.
type CreateArvanCloudPageRuleInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Rule        domain.ArvanCloudPageRule
}

// CreateArvanCloudPageRule creates a new page rule. This is a fast
// operation.
type CreateArvanCloudPageRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudPageRule builds the use case from its ports.
func NewCreateArvanCloudPageRule(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudPageRule {
	return &CreateArvanCloudPageRule{queue: queue, provider: provider}
}

// Execute validates the request and creates the rule, returning it as
// stored.
func (uc *CreateArvanCloudPageRule) Execute(ctx context.Context, in CreateArvanCloudPageRuleInput) (*domain.ArvanCloudPageRule, error) {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudPageRuleInput(in.Rule); err != nil {
		return nil, err
	}

	return dispatchArvanCloudPageRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudPageRule, error) {
		created, err := uc.provider.CreateArvanCloudPageRule(ctx, in.Credentials, in.Domain, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud page rule on domain %q: %w", in.Domain, err)
		}
		return created, nil
	})
}

// GetArvanCloudPageRuleInput identifies the page rule to look up.
type GetArvanCloudPageRuleInput = arvanCloudRuleIDInput

// GetArvanCloudPageRule is a fast operation.
type GetArvanCloudPageRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudPageRule builds the use case from its ports.
func NewGetArvanCloudPageRule(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudPageRule {
	return &GetArvanCloudPageRule{queue: queue, provider: provider}
}

// Execute returns the current state of one page rule.
func (uc *GetArvanCloudPageRule) Execute(ctx context.Context, in GetArvanCloudPageRuleInput) (*domain.ArvanCloudPageRule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudPageRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudPageRule, error) {
		found, err := uc.provider.GetArvanCloudPageRule(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud page rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudPageRuleInput identifies the page rule to update and its
// new field values.
type UpdateArvanCloudPageRuleInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Rule        domain.ArvanCloudPageRule
}

// UpdateArvanCloudPageRule replaces a page rule's fields via PUT. This is a
// fast operation.
type UpdateArvanCloudPageRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudPageRule builds the use case from its ports.
func NewUpdateArvanCloudPageRule(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudPageRule {
	return &UpdateArvanCloudPageRule{queue: queue, provider: provider}
}

// Execute updates the rule and returns it as stored afterward.
func (uc *UpdateArvanCloudPageRule) Execute(ctx context.Context, in UpdateArvanCloudPageRuleInput) (*domain.ArvanCloudPageRule, error) {
	if err := (arvanCloudRuleIDInput{Credentials: in.Credentials, Domain: in.Domain, ID: in.ID}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudPageRuleInput(in.Rule); err != nil {
		return nil, err
	}

	return dispatchArvanCloudPageRule(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudPageRule, error) {
		updated, err := uc.provider.UpdateArvanCloudPageRule(ctx, in.Credentials, in.Domain, in.ID, in.Rule)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud page rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// SetArvanCloudPageRuleStatusInput identifies the page rule and the new
// status.
type SetArvanCloudPageRuleStatusInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Status      bool
}

// SetArvanCloudPageRuleStatus toggles only a page rule's status via PATCH —
// a separate, narrower operation from UpdateArvanCloudPageRule's full
// replace (ports.ArvanCloudProvider's own doc comment). This is a fast
// operation.
type SetArvanCloudPageRuleStatus struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewSetArvanCloudPageRuleStatus builds the use case from its ports.
func NewSetArvanCloudPageRuleStatus(queue ports.Queue, provider ports.ArvanCloudProvider) *SetArvanCloudPageRuleStatus {
	return &SetArvanCloudPageRuleStatus{queue: queue, provider: provider}
}

// Execute toggles the rule's status. The endpoint carries no response data,
// so Execute returns only an error; a caller wanting the rule's full state
// afterward calls GetArvanCloudPageRule.
func (uc *SetArvanCloudPageRuleStatus) Execute(ctx context.Context, in SetArvanCloudPageRuleStatusInput) error {
	if err := (arvanCloudRuleIDInput{Credentials: in.Credentials, Domain: in.Domain, ID: in.ID}).validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.SetArvanCloudPageRuleStatus(ctx, in.Credentials, in.Domain, in.ID, in.Status); err != nil {
			return nil, fmt.Errorf("setting arvancloud page rule %q status on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// DeleteArvanCloudPageRuleInput identifies the page rule to remove.
type DeleteArvanCloudPageRuleInput = arvanCloudRuleIDInput

// DeleteArvanCloudPageRule is a fast operation. Deleting a rule the provider
// no longer has is treated as already done rather than an error, matching
// DeleteArvanCloudHealthCheck's own tolerant-delete contract.
type DeleteArvanCloudPageRule struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudPageRule builds the use case from its ports.
func NewDeleteArvanCloudPageRule(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudPageRule {
	return &DeleteArvanCloudPageRule{queue: queue, provider: provider}
}

// Execute deletes the rule, tolerating one that is already gone.
func (uc *DeleteArvanCloudPageRule) Execute(ctx context.Context, in DeleteArvanCloudPageRuleInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudPageRule(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud page rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// PurgeArvanCloudPageRuleCacheInput identifies the page rule whose matching
// cached content to purge.
type PurgeArvanCloudPageRuleCacheInput = arvanCloudRuleIDInput

// PurgeArvanCloudPageRuleCache is a fast operation.
type PurgeArvanCloudPageRuleCache struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewPurgeArvanCloudPageRuleCache builds the use case from its ports.
func NewPurgeArvanCloudPageRuleCache(queue ports.Queue, provider ports.ArvanCloudProvider) *PurgeArvanCloudPageRuleCache {
	return &PurgeArvanCloudPageRuleCache{queue: queue, provider: provider}
}

// Execute purges cached content for URLs matching the rule. The endpoint
// carries no response data, so Execute returns only an error.
func (uc *PurgeArvanCloudPageRuleCache) Execute(ctx context.Context, in PurgeArvanCloudPageRuleCacheInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.PurgeArvanCloudPageRuleCache(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			return nil, fmt.Errorf("purging cache for arvancloud page rule %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Page Rule exceptions ("diff") -----------------------------------------

// validateArvanCloudPageRuleExceptionsInput checks
// domain.ArvanCloudPageRuleExceptions' overlapping fields with
// validateArvanCloudPageRuleInput, reusing the same shared-TTL-enum and enum
// checks rather than duplicating them under different names.
func validateArvanCloudPageRuleExceptionsInput(e domain.ArvanCloudPageRuleExceptions) error {
	if !domain.ValidArvanCloudPageRuleCacheLevel(string(e.CacheLevel)) {
		return fmt.Errorf("cache_level %q is not one of off/uri/query_string: %w", e.CacheLevel, domain.ErrInvalidInput)
	}
	if err := validateArvanCloudCacheTTLGroup(e.Cache200, e.CacheAny, e.CacheBrowser); err != nil {
		return err
	}
	for _, field := range e.SlinkMD5 {
		if !domain.ValidArvanCloudPageRuleSlinkMD5Field(string(field)) {
			return fmt.Errorf("slink_md5 entry %q is not one of remote_addr/file/expires/url/uri: %w", field, domain.ErrInvalidInput)
		}
	}
	if !domain.ValidArvanCloudPageRuleRedirectStatusCode(e.Redirect.StatusCode) {
		return fmt.Errorf("redirect.status_code %d is not one of 301/302/307: %w", e.Redirect.StatusCode, domain.ErrInvalidInput)
	}
	if e.Acceleration.Status != "" && !domain.ValidArvanCloudAccelerationStatus(string(e.Acceleration.Status)) {
		return fmt.Errorf("acceleration.status %q is not one of inherit/on/off: %w", e.Acceleration.Status, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudPageRuleImageResizeStatus(string(e.ImageResize.Status)) {
		return fmt.Errorf("image_resize.status %q is not one of on/off/inherit: %w", e.ImageResize.Status, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudImageResizeMode(string(e.ImageResize.Mode)) {
		return fmt.Errorf("image_resize.mode %q is not one of freely/short-side/long-side: %w", e.ImageResize.Mode, domain.ErrInvalidInput)
	}
	for _, h := range e.ReqCustomHeaders {
		if !domain.ValidArvanCloudPageRuleExceptionIsVar(h.IsVar) {
			return fmt.Errorf("req_custom_headers[].is_var %q is not \"true\", \"false\" or empty: %w", h.IsVar, domain.ErrInvalidInput)
		}
	}
	for _, h := range e.ResCustomHeaders {
		if !domain.ValidArvanCloudPageRuleExceptionIsVar(h.IsVar) {
			return fmt.Errorf("res_custom_headers[].is_var %q is not \"true\", \"false\" or empty: %w", h.IsVar, domain.ErrInvalidInput)
		}
	}
	return nil
}

// GetArvanCloudPageRuleExceptionsInput identifies the page rule whose
// exceptions to look up.
type GetArvanCloudPageRuleExceptionsInput = arvanCloudRuleIDInput

// GetArvanCloudPageRuleExceptions is a fast operation.
type GetArvanCloudPageRuleExceptions struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudPageRuleExceptions builds the use case from its ports.
func NewGetArvanCloudPageRuleExceptions(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudPageRuleExceptions {
	return &GetArvanCloudPageRuleExceptions{queue: queue, provider: provider}
}

// Execute returns the rule's exceptions.
func (uc *GetArvanCloudPageRuleExceptions) Execute(ctx context.Context, in GetArvanCloudPageRuleExceptionsInput) (*domain.ArvanCloudPageRuleExceptions, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		exceptions, err := uc.provider.GetArvanCloudPageRuleExceptions(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud page rule %q exceptions on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.Marshal(exceptions)
	})
	if err != nil {
		return nil, err
	}

	var exceptions domain.ArvanCloudPageRuleExceptions
	if err := json.Unmarshal(raw, &exceptions); err != nil {
		return nil, fmt.Errorf("decoding arvancloud page rule exceptions: %w", err)
	}
	return &exceptions, nil
}

// UpdateArvanCloudPageRuleExceptionsInput identifies the page rule and the
// new exceptions to apply.
type UpdateArvanCloudPageRuleExceptionsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Exceptions  domain.ArvanCloudPageRuleExceptions
}

// UpdateArvanCloudPageRuleExceptions is a fast operation.
type UpdateArvanCloudPageRuleExceptions struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudPageRuleExceptions builds the use case from its ports.
func NewUpdateArvanCloudPageRuleExceptions(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudPageRuleExceptions {
	return &UpdateArvanCloudPageRuleExceptions{queue: queue, provider: provider}
}

// Execute updates the rule's exceptions and returns them as stored
// afterward.
func (uc *UpdateArvanCloudPageRuleExceptions) Execute(ctx context.Context, in UpdateArvanCloudPageRuleExceptionsInput) (*domain.ArvanCloudPageRuleExceptions, error) {
	if err := (arvanCloudRuleIDInput{Credentials: in.Credentials, Domain: in.Domain, ID: in.ID}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudPageRuleExceptionsInput(in.Exceptions); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		updated, err := uc.provider.UpdateArvanCloudPageRuleExceptions(ctx, in.Credentials, in.Domain, in.ID, in.Exceptions)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud page rule %q exceptions on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.Marshal(updated)
	})
	if err != nil {
		return nil, err
	}

	var updated domain.ArvanCloudPageRuleExceptions
	if err := json.Unmarshal(raw, &updated); err != nil {
		return nil, fmt.Errorf("decoding arvancloud page rule exceptions: %w", err)
	}
	return &updated, nil
}

// --- Dispatch helper ----------------------------------------------------

// dispatchArvanCloudPageRule runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudPageRule, the shape every page rule use case
// above but list/delete/purge/status/exceptions returns.
func dispatchArvanCloudPageRule(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudPageRule, error),
) (*domain.ArvanCloudPageRule, error) {
	raw, err := queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudPageRule
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud page rule: %w", err)
	}
	return &result, nil
}

// --- Response Transforms ---------------------------------------------------

// validateArvanCloudResponseTransformInput checks the fields every
// create/update response-transform call shares.
func validateArvanCloudResponseTransformInput(rt domain.ArvanCloudResponseTransform) error {
	if rt.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if len(rt.Transforms) == 0 {
		return fmt.Errorf("transforms must have at least one step: %w", domain.ErrInvalidInput)
	}
	for i, step := range rt.Transforms {
		if len(step.Condition) < 3 || len(step.Condition) > 5000 {
			return fmt.Errorf("transforms[%d].condition must be 3-5000 characters: %w", i, domain.ErrInvalidInput)
		}
		if len(step.Actions) == 0 {
			return fmt.Errorf("transforms[%d].actions must have at least one action: %w", i, domain.ErrInvalidInput)
		}
		for j, action := range step.Actions {
			actionType := action.Type
			if actionType == "" {
				actionType = domain.ArvanCloudResponseTransformActionAddHeader
			}
			if !domain.ValidArvanCloudResponseTransformActionType(string(actionType)) {
				return fmt.Errorf("transforms[%d].actions[%d].type %q is not \"add_header\": %w", i, j, actionType, domain.ErrInvalidInput)
			}
			mode := action.Mode
			if mode == "" {
				mode = domain.ArvanCloudResponseTransformModeSet
			}
			if !domain.ValidArvanCloudResponseTransformActionMode(string(mode)) {
				return fmt.Errorf("transforms[%d].actions[%d].mode %q is not \"set\": %w", i, j, mode, domain.ErrInvalidInput)
			}
			if !domain.ValidArvanCloudResponseTransformHeaderKey(string(action.Key)) {
				return fmt.Errorf("transforms[%d].actions[%d].key %q is not one of the allowed CORS response headers: %w", i, j, action.Key, domain.ErrInvalidInput)
			}
			if action.Value == "" {
				return fmt.Errorf("transforms[%d].actions[%d].value is required: %w", i, j, domain.ErrInvalidInput)
			}
		}
	}
	return nil
}

// ListArvanCloudResponseTransformsInput identifies the domain and query for
// ListArvanCloudResponseTransforms.
type ListArvanCloudResponseTransformsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Query       domain.ArvanCloudResponseTransformListQuery
}

// ListArvanCloudResponseTransformsResult pairs one page of presets with its
// pagination info.
type ListArvanCloudResponseTransformsResult struct {
	Transforms []domain.ArvanCloudResponseTransform
	Page       domain.ArvanCloudResponseTransformPageMeta
}

// ListArvanCloudResponseTransforms is a fast operation.
type ListArvanCloudResponseTransforms struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudResponseTransforms builds the use case from its ports.
func NewListArvanCloudResponseTransforms(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudResponseTransforms {
	return &ListArvanCloudResponseTransforms{queue: queue, provider: provider}
}

// Execute returns one page of the domain's response-transform presets.
func (uc *ListArvanCloudResponseTransforms) Execute(ctx context.Context, in ListArvanCloudResponseTransformsInput) (*ListArvanCloudResponseTransformsResult, error) {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		transforms, page, err := uc.provider.ListArvanCloudResponseTransforms(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud response transforms of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(ListArvanCloudResponseTransformsResult{Transforms: transforms, Page: page})
	})
	if err != nil {
		return nil, err
	}

	var result ListArvanCloudResponseTransformsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud response transform list: %w", err)
	}
	return &result, nil
}

// CreateArvanCloudResponseTransformInput identifies the domain and the new
// preset's field values.
type CreateArvanCloudResponseTransformInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Transform   domain.ArvanCloudResponseTransform
}

// CreateArvanCloudResponseTransform is a fast operation.
type CreateArvanCloudResponseTransform struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudResponseTransform builds the use case from its ports.
func NewCreateArvanCloudResponseTransform(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudResponseTransform {
	return &CreateArvanCloudResponseTransform{queue: queue, provider: provider}
}

// Execute validates the request and creates the preset, returning it as
// stored.
func (uc *CreateArvanCloudResponseTransform) Execute(ctx context.Context, in CreateArvanCloudResponseTransformInput) (*domain.ArvanCloudResponseTransform, error) {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudResponseTransformInput(in.Transform); err != nil {
		return nil, err
	}

	return dispatchArvanCloudResponseTransform(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudResponseTransform, error) {
		created, err := uc.provider.CreateArvanCloudResponseTransform(ctx, in.Credentials, in.Domain, in.Transform)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud response transform on domain %q: %w", in.Domain, err)
		}
		return created, nil
	})
}

// GetArvanCloudResponseTransformInput identifies the preset to look up.
type GetArvanCloudResponseTransformInput = arvanCloudRuleIDInput

// GetArvanCloudResponseTransform is a fast operation.
type GetArvanCloudResponseTransform struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudResponseTransform builds the use case from its ports.
func NewGetArvanCloudResponseTransform(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudResponseTransform {
	return &GetArvanCloudResponseTransform{queue: queue, provider: provider}
}

// Execute returns the current state of one preset.
func (uc *GetArvanCloudResponseTransform) Execute(ctx context.Context, in GetArvanCloudResponseTransformInput) (*domain.ArvanCloudResponseTransform, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudResponseTransform(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudResponseTransform, error) {
		found, err := uc.provider.GetArvanCloudResponseTransform(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud response transform %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudResponseTransformInput identifies the preset to update and
// its new field values.
type UpdateArvanCloudResponseTransformInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Transform   domain.ArvanCloudResponseTransform
}

// UpdateArvanCloudResponseTransform updates a preset via PATCH. This is a
// fast operation.
type UpdateArvanCloudResponseTransform struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudResponseTransform builds the use case from its ports.
func NewUpdateArvanCloudResponseTransform(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudResponseTransform {
	return &UpdateArvanCloudResponseTransform{queue: queue, provider: provider}
}

// Execute updates the preset and returns it as stored afterward.
func (uc *UpdateArvanCloudResponseTransform) Execute(ctx context.Context, in UpdateArvanCloudResponseTransformInput) (*domain.ArvanCloudResponseTransform, error) {
	if err := (arvanCloudRuleIDInput{Credentials: in.Credentials, Domain: in.Domain, ID: in.ID}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudResponseTransformInput(in.Transform); err != nil {
		return nil, err
	}

	return dispatchArvanCloudResponseTransform(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudResponseTransform, error) {
		updated, err := uc.provider.UpdateArvanCloudResponseTransform(ctx, in.Credentials, in.Domain, in.ID, in.Transform)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud response transform %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudResponseTransformInput identifies the preset to remove.
type DeleteArvanCloudResponseTransformInput = arvanCloudRuleIDInput

// DeleteArvanCloudResponseTransform is a fast operation. Deleting a preset
// the provider no longer has is treated as already done rather than an
// error, matching DeleteArvanCloudPageRule's own tolerant-delete contract.
type DeleteArvanCloudResponseTransform struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudResponseTransform builds the use case from its ports.
func NewDeleteArvanCloudResponseTransform(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudResponseTransform {
	return &DeleteArvanCloudResponseTransform{queue: queue, provider: provider}
}

// Execute deletes the preset, tolerating one that is already gone.
func (uc *DeleteArvanCloudResponseTransform) Execute(ctx context.Context, in DeleteArvanCloudResponseTransformInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudResponseTransform(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud response transform %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// dispatchArvanCloudResponseTransform runs fn on the queue and decodes its
// result back into a *domain.ArvanCloudResponseTransform, the shape every
// response-transform use case above but list/delete returns.
func dispatchArvanCloudResponseTransform(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudResponseTransform, error),
) (*domain.ArvanCloudResponseTransform, error) {
	raw, err := queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudResponseTransform
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud response transform: %w", err)
	}
	return &result, nil
}

// --- Redirect (www-redirect) ------------------------------------------------

// GetArvanCloudWWWRedirectInput identifies the domain whose www-redirect
// setting to look up.
type GetArvanCloudWWWRedirectInput = arvanCloudDomainInput

// GetArvanCloudWWWRedirect is a fast operation.
type GetArvanCloudWWWRedirect struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudWWWRedirect builds the use case from its ports.
func NewGetArvanCloudWWWRedirect(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudWWWRedirect {
	return &GetArvanCloudWWWRedirect{queue: queue, provider: provider}
}

// Execute returns the domain's current www-redirect setting.
func (uc *GetArvanCloudWWWRedirect) Execute(ctx context.Context, in GetArvanCloudWWWRedirectInput) (*domain.ArvanCloudWWWRedirectSettings, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		settings, err := uc.provider.GetArvanCloudWWWRedirect(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud www-redirect setting of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(settings)
	})
	if err != nil {
		return nil, err
	}

	var settings domain.ArvanCloudWWWRedirectSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decoding arvancloud www-redirect setting: %w", err)
	}
	return &settings, nil
}

// UpdateArvanCloudWWWRedirectInput identifies the domain and the new
// www-redirect setting.
type UpdateArvanCloudWWWRedirectInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Settings    domain.ArvanCloudWWWRedirectSettings
}

// UpdateArvanCloudWWWRedirect is a fast operation.
type UpdateArvanCloudWWWRedirect struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudWWWRedirect builds the use case from its ports.
func NewUpdateArvanCloudWWWRedirect(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudWWWRedirect {
	return &UpdateArvanCloudWWWRedirect{queue: queue, provider: provider}
}

// Execute sets the domain's www-redirect setting. The endpoint carries no
// response data, so Execute returns only an error; a caller wanting the
// setting's state afterward calls GetArvanCloudWWWRedirect.
func (uc *UpdateArvanCloudWWWRedirect) Execute(ctx context.Context, in UpdateArvanCloudWWWRedirectInput) error {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return err
	}
	if !domain.ValidArvanCloudWWWRedirectMode(string(in.Settings.Mode)) {
		return fmt.Errorf("mode %q is not one of off/www/root: %w", in.Settings.Mode, domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UpdateArvanCloudWWWRedirect(ctx, in.Credentials, in.Domain, in.Settings); err != nil {
			return nil, fmt.Errorf("updating arvancloud www-redirect setting of domain %q: %w", in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Host Header Whitelist --------------------------------------------------

// GetArvanCloudHostHeaderWhitelistInput identifies the domain whose Host
// header whitelist state to look up.
type GetArvanCloudHostHeaderWhitelistInput = arvanCloudDomainInput

// GetArvanCloudHostHeaderWhitelist is a fast operation.
type GetArvanCloudHostHeaderWhitelist struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudHostHeaderWhitelist builds the use case from its ports.
func NewGetArvanCloudHostHeaderWhitelist(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudHostHeaderWhitelist {
	return &GetArvanCloudHostHeaderWhitelist{queue: queue, provider: provider}
}

// Execute returns the domain's Host header whitelist state.
func (uc *GetArvanCloudHostHeaderWhitelist) Execute(ctx context.Context, in GetArvanCloudHostHeaderWhitelistInput) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudHostHeaderWhitelist(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudHostHeaderWhitelist, error) {
		found, err := uc.provider.GetArvanCloudHostHeaderWhitelist(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud host header whitelist of domain %q: %w", in.Domain, err)
		}
		return found, nil
	})
}

// AddArvanCloudHostHeaderWhitelistEntryInput identifies the domain and the
// target account to add.
type AddArvanCloudHostHeaderWhitelistEntryInput struct {
	Credentials   domain.ProviderCredentials
	Domain        string
	TargetAccount string
}

// AddArvanCloudHostHeaderWhitelistEntry is a fast operation.
type AddArvanCloudHostHeaderWhitelistEntry struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewAddArvanCloudHostHeaderWhitelistEntry builds the use case from its
// ports.
func NewAddArvanCloudHostHeaderWhitelistEntry(queue ports.Queue, provider ports.ArvanCloudProvider) *AddArvanCloudHostHeaderWhitelistEntry {
	return &AddArvanCloudHostHeaderWhitelistEntry{queue: queue, provider: provider}
}

// Execute adds the target account and returns the whitelist state
// afterward.
func (uc *AddArvanCloudHostHeaderWhitelistEntry) Execute(ctx context.Context, in AddArvanCloudHostHeaderWhitelistEntryInput) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return nil, err
	}
	if in.TargetAccount == "" {
		return nil, fmt.Errorf("target_account is required: %w", domain.ErrInvalidInput)
	}

	return dispatchArvanCloudHostHeaderWhitelist(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudHostHeaderWhitelist, error) {
		updated, err := uc.provider.AddArvanCloudHostHeaderWhitelistEntry(ctx, in.Credentials, in.Domain, in.TargetAccount)
		if err != nil {
			return nil, fmt.Errorf("adding arvancloud host header whitelist entry %q on domain %q: %w", in.TargetAccount, in.Domain, err)
		}
		return updated, nil
	})
}

// SetArvanCloudHostHeaderWhitelistSettingsInput identifies the domain and the
// new global-allowlist setting.
type SetArvanCloudHostHeaderWhitelistSettingsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Global      bool
}

// SetArvanCloudHostHeaderWhitelistSettings is a fast operation.
type SetArvanCloudHostHeaderWhitelistSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewSetArvanCloudHostHeaderWhitelistSettings builds the use case from its
// ports.
func NewSetArvanCloudHostHeaderWhitelistSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *SetArvanCloudHostHeaderWhitelistSettings {
	return &SetArvanCloudHostHeaderWhitelistSettings{queue: queue, provider: provider}
}

// Execute sets or clears the domain's global Host allowlist entry and
// returns the whitelist state afterward.
func (uc *SetArvanCloudHostHeaderWhitelistSettings) Execute(ctx context.Context, in SetArvanCloudHostHeaderWhitelistSettingsInput) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return nil, err
	}

	return dispatchArvanCloudHostHeaderWhitelist(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudHostHeaderWhitelist, error) {
		updated, err := uc.provider.SetArvanCloudHostHeaderWhitelistSettings(ctx, in.Credentials, in.Domain, in.Global)
		if err != nil {
			return nil, fmt.Errorf("setting arvancloud host header whitelist global setting on domain %q: %w", in.Domain, err)
		}
		return updated, nil
	})
}

// RemoveArvanCloudHostHeaderWhitelistEntryInput identifies the domain and the
// target account to remove.
type RemoveArvanCloudHostHeaderWhitelistEntryInput struct {
	Credentials   domain.ProviderCredentials
	Domain        string
	TargetAccount string
}

// RemoveArvanCloudHostHeaderWhitelistEntry is a fast operation. Unlike this
// package's other Delete use cases, a missing row is NOT tolerated here —
// see ports.ArvanCloudProvider.RemoveArvanCloudHostHeaderWhitelistEntry's own
// doc comment for why (the provider's 404 shape is ambiguous between "domain
// not found" and "row not found").
type RemoveArvanCloudHostHeaderWhitelistEntry struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewRemoveArvanCloudHostHeaderWhitelistEntry builds the use case from its
// ports.
func NewRemoveArvanCloudHostHeaderWhitelistEntry(queue ports.Queue, provider ports.ArvanCloudProvider) *RemoveArvanCloudHostHeaderWhitelistEntry {
	return &RemoveArvanCloudHostHeaderWhitelistEntry{queue: queue, provider: provider}
}

// Execute removes the target account and returns the whitelist state
// afterward.
func (uc *RemoveArvanCloudHostHeaderWhitelistEntry) Execute(ctx context.Context, in RemoveArvanCloudHostHeaderWhitelistEntryInput) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	if err := (arvanCloudDomainInput{Credentials: in.Credentials, Domain: in.Domain}).validate(); err != nil {
		return nil, err
	}
	if in.TargetAccount == "" {
		return nil, fmt.Errorf("target_account is required: %w", domain.ErrInvalidInput)
	}

	return dispatchArvanCloudHostHeaderWhitelist(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudHostHeaderWhitelist, error) {
		updated, err := uc.provider.RemoveArvanCloudHostHeaderWhitelistEntry(ctx, in.Credentials, in.Domain, in.TargetAccount)
		if err != nil {
			return nil, fmt.Errorf("removing arvancloud host header whitelist entry %q on domain %q: %w", in.TargetAccount, in.Domain, err)
		}
		return updated, nil
	})
}

// dispatchArvanCloudHostHeaderWhitelist runs fn on the queue and decodes its
// result back into a *domain.ArvanCloudHostHeaderWhitelist, the shape every
// host-header-whitelist use case above returns.
func dispatchArvanCloudHostHeaderWhitelist(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudHostHeaderWhitelist, error),
) (*domain.ArvanCloudHostHeaderWhitelist, error) {
	raw, err := queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudHostHeaderWhitelist
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud host header whitelist: %w", err)
	}
	return &result, nil
}
