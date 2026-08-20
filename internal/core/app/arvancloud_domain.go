package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the domain onboarding and lifecycle use cases for
// ArvanCloud (issue #62): domain lifecycle, NS Setup, CNAME Setup, and the
// remaining single-domain actions (clone, regenerate, hold/unhold). Every one
// of them is a fast operation (ports.ArvanCloudProvider, AGENTS.md 4.3): each
// dispatches onto the queue and blocks for the result within the same tool
// call — there is no operation_id to poll.

// CreateArvanCloudDomainInput is the normalized form of a
// create_arvancloud_domain tool call.
type CreateArvanCloudDomainInput struct {
	Credentials domain.ProviderCredentials
	Spec        domain.ArvanCloudDomainSpec
}

// CreateArvanCloudDomain onboards a new domain onto ArvanCloud's CDN.
type CreateArvanCloudDomain struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudDomain builds the use case from its ports.
func NewCreateArvanCloudDomain(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudDomain {
	return &CreateArvanCloudDomain{queue: queue, provider: provider}
}

// Execute validates the request and creates the domain, returning the stored
// domain synchronously.
func (uc *CreateArvanCloudDomain) Execute(ctx context.Context, in CreateArvanCloudDomainInput) (*domain.ArvanCloudDomain, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudDomainSpec(in.Spec); err != nil {
		return nil, err
	}

	return dispatchArvanCloudDomain(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDomain, error) {
		created, err := uc.provider.CreateDomain(ctx, in.Credentials, in.Spec)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud domain %q: %w", in.Spec.Name, err)
		}
		return created, nil
	})
}

func validateArvanCloudDomainSpec(spec domain.ArvanCloudDomainSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if spec.DomainType != "" && !domain.ValidArvanCloudDomainType(spec.DomainType) {
		return fmt.Errorf("domain_type %q is not one of \"full\" or \"partial\": %w", spec.DomainType, domain.ErrInvalidInput)
	}
	if !spec.PlanLevel.Valid() {
		return fmt.Errorf("plan_level %d is not a plan ArvanCloud offers (0-4): %w", spec.PlanLevel, domain.ErrInvalidInput)
	}
	return nil
}

// ListArvanCloudDomainsInput carries the credentials needed to list an
// account's ArvanCloud domains. There is nothing else to specify: listing is
// unscoped.
type ListArvanCloudDomainsInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudDomains is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type ListArvanCloudDomains struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudDomains builds the use case from its ports.
func NewListArvanCloudDomains(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudDomains {
	return &ListArvanCloudDomains{queue: queue, provider: provider}
}

// Execute returns every domain visible to the given credentials.
func (uc *ListArvanCloudDomains) Execute(ctx context.Context, in ListArvanCloudDomainsInput) ([]domain.ArvanCloudDomain, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		domains, err := uc.provider.ListDomains(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud domains: %w", err)
		}
		return json.Marshal(domains)
	})
	if err != nil {
		return nil, err
	}

	var domains []domain.ArvanCloudDomain
	if err := json.Unmarshal(raw, &domains); err != nil {
		return nil, fmt.Errorf("decoding arvancloud domain list: %w", err)
	}
	return domains, nil
}

// arvanCloudDomainNameInput is embedded by every use case below that is
// scoped to exactly one domain and needs nothing else.
type arvanCloudDomainNameInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
}

func (in arvanCloudDomainNameInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.DomainName == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudDomainInput identifies the domain to look up.
type GetArvanCloudDomainInput = arvanCloudDomainNameInput

// GetArvanCloudDomain is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetArvanCloudDomain struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudDomain builds the use case from its ports.
func NewGetArvanCloudDomain(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudDomain {
	return &GetArvanCloudDomain{queue: queue, provider: provider}
}

// Execute returns the current state of one domain.
func (uc *GetArvanCloudDomain) Execute(ctx context.Context, in GetArvanCloudDomainInput) (*domain.ArvanCloudDomain, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDomain(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDomain, error) {
		found, err := uc.provider.GetDomain(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud domain %q: %w", in.DomainName, err)
		}
		return found, nil
	})
}

// DeleteArvanCloudDomainInput identifies the domain to remove.
type DeleteArvanCloudDomainInput = arvanCloudDomainNameInput

// DeleteArvanCloudDomain is a fast operation. Deleting a domain the provider
// no longer has is treated as already done rather than an error, so callers
// can call it more than once safely (ports.ArvanCloudProvider.DeleteDomain's
// contract).
type DeleteArvanCloudDomain struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudDomain builds the use case from its ports.
func NewDeleteArvanCloudDomain(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudDomain {
	return &DeleteArvanCloudDomain{queue: queue, provider: provider}
}

// Execute deletes the domain, tolerating one that is already gone.
func (uc *DeleteArvanCloudDomain) Execute(ctx context.Context, in DeleteArvanCloudDomainInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteDomain(ctx, in.Credentials, in.DomainName); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud domain %q: %w", in.DomainName, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// SetArvanCloudNSKeysInput identifies the domain and the NS records to set.
type SetArvanCloudNSKeysInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
	NSKeys      []string
}

// SetArvanCloudNSKeys sets the custom NS records a "full" domain's registrar
// must be pointed at. This is a fast operation.
type SetArvanCloudNSKeys struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewSetArvanCloudNSKeys builds the use case from its ports.
func NewSetArvanCloudNSKeys(queue ports.Queue, provider ports.ArvanCloudProvider) *SetArvanCloudNSKeys {
	return &SetArvanCloudNSKeys{queue: queue, provider: provider}
}

// Execute sets the NS records and returns the domain's updated NS fields.
func (uc *SetArvanCloudNSKeys) Execute(ctx context.Context, in SetArvanCloudNSKeysInput) (*domain.ArvanCloudDomain, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.DomainName == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if len(in.NSKeys) != 2 {
		return nil, fmt.Errorf("ns_keys must contain exactly 2 nameservers, got %d: %w", len(in.NSKeys), domain.ErrInvalidInput)
	}

	return dispatchArvanCloudDomain(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDomain, error) {
		updated, err := uc.provider.SetNSKeys(ctx, in.Credentials, in.DomainName, in.NSKeys)
		if err != nil {
			return nil, fmt.Errorf("setting NS keys for arvancloud domain %q: %w", in.DomainName, err)
		}
		return updated, nil
	})
}

// ResetArvanCloudNSKeysInput identifies the domain whose NS records to reset.
type ResetArvanCloudNSKeysInput = arvanCloudDomainNameInput

// ResetArvanCloudNSKeys resets a "full" domain's NS records to ArvanCloud's
// defaults. This is a fast operation.
type ResetArvanCloudNSKeys struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewResetArvanCloudNSKeys builds the use case from its ports.
func NewResetArvanCloudNSKeys(queue ports.Queue, provider ports.ArvanCloudProvider) *ResetArvanCloudNSKeys {
	return &ResetArvanCloudNSKeys{queue: queue, provider: provider}
}

// Execute resets the NS records and returns the domain's default NS values.
func (uc *ResetArvanCloudNSKeys) Execute(ctx context.Context, in ResetArvanCloudNSKeysInput) (*domain.ArvanCloudDomain, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDomain(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDomain, error) {
		reset, err := uc.provider.ResetNSKeys(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("resetting NS keys for arvancloud domain %q: %w", in.DomainName, err)
		}
		return reset, nil
	})
}

// CheckArvanCloudNSStatusInput identifies the domain whose NS status to
// check.
type CheckArvanCloudNSStatusInput = arvanCloudDomainNameInput

// CheckArvanCloudNSStatus reports whether a "full" domain's registrar has
// been repointed at ArvanCloud yet. This is a fast operation.
type CheckArvanCloudNSStatus struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCheckArvanCloudNSStatus builds the use case from its ports.
func NewCheckArvanCloudNSStatus(queue ports.Queue, provider ports.ArvanCloudProvider) *CheckArvanCloudNSStatus {
	return &CheckArvanCloudNSStatus{queue: queue, provider: provider}
}

// Execute returns the domain's expected and currently-configured NS values.
func (uc *CheckArvanCloudNSStatus) Execute(ctx context.Context, in CheckArvanCloudNSStatusInput) (*domain.ArvanCloudDomain, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDomain(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDomain, error) {
		status, err := uc.provider.CheckNSStatus(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("checking NS status for arvancloud domain %q: %w", in.DomainName, err)
		}
		return status, nil
	})
}

// UseArvanCloudOptionalNSKeysInput identifies the domain to switch to
// ArvanCloud's alternate NS key set.
type UseArvanCloudOptionalNSKeysInput = arvanCloudDomainNameInput

// UseArvanCloudOptionalNSKeys switches a "full" domain to ArvanCloud's
// alternate NS key set. This is a fast operation.
type UseArvanCloudOptionalNSKeys struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUseArvanCloudOptionalNSKeys builds the use case from its ports.
func NewUseArvanCloudOptionalNSKeys(queue ports.Queue, provider ports.ArvanCloudProvider) *UseArvanCloudOptionalNSKeys {
	return &UseArvanCloudOptionalNSKeys{queue: queue, provider: provider}
}

// Execute switches the domain to the optional NS key set.
func (uc *UseArvanCloudOptionalNSKeys) Execute(ctx context.Context, in UseArvanCloudOptionalNSKeysInput) (*domain.ArvanCloudDomain, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDomain(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDomain, error) {
		updated, err := uc.provider.UseOptionalNSKeys(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("switching to optional NS keys for arvancloud domain %q: %w", in.DomainName, err)
		}
		return updated, nil
	})
}

// SetArvanCloudCnameTargetInput identifies the domain and the CNAME record it
// must resolve through.
type SetArvanCloudCnameTargetInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
	Address     string
}

// SetArvanCloudCnameTarget sets the custom CNAME record a "partial" domain
// resolves through. This is a fast operation.
type SetArvanCloudCnameTarget struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewSetArvanCloudCnameTarget builds the use case from its ports.
func NewSetArvanCloudCnameTarget(queue ports.Queue, provider ports.ArvanCloudProvider) *SetArvanCloudCnameTarget {
	return &SetArvanCloudCnameTarget{queue: queue, provider: provider}
}

// Execute sets the CNAME target and returns the domain as ArvanCloud reports
// it afterward.
func (uc *SetArvanCloudCnameTarget) Execute(ctx context.Context, in SetArvanCloudCnameTargetInput) (*domain.ArvanCloudDomain, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.DomainName == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.Address == "" {
		return nil, fmt.Errorf("address is required: %w", domain.ErrInvalidInput)
	}

	return dispatchArvanCloudDomain(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDomain, error) {
		updated, err := uc.provider.SetCnameTarget(ctx, in.Credentials, in.DomainName, in.Address)
		if err != nil {
			return nil, fmt.Errorf("setting CNAME target for arvancloud domain %q: %w", in.DomainName, err)
		}
		return updated, nil
	})
}

// ResetArvanCloudCnameTargetInput identifies the domain whose CNAME target to
// reset.
type ResetArvanCloudCnameTargetInput = arvanCloudDomainNameInput

// ResetArvanCloudCnameTarget resets a "partial" domain's CNAME record to
// ArvanCloud's default. This is a fast operation.
type ResetArvanCloudCnameTarget struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewResetArvanCloudCnameTarget builds the use case from its ports.
func NewResetArvanCloudCnameTarget(queue ports.Queue, provider ports.ArvanCloudProvider) *ResetArvanCloudCnameTarget {
	return &ResetArvanCloudCnameTarget{queue: queue, provider: provider}
}

// Execute resets the CNAME target and returns the domain as ArvanCloud
// reports it afterward.
func (uc *ResetArvanCloudCnameTarget) Execute(ctx context.Context, in ResetArvanCloudCnameTargetInput) (*domain.ArvanCloudDomain, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDomain(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDomain, error) {
		reset, err := uc.provider.ResetCnameTarget(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("resetting CNAME target for arvancloud domain %q: %w", in.DomainName, err)
		}
		return reset, nil
	})
}

// ConvertArvanCloudToCnameSetupInput identifies the domain to switch to CNAME
// Setup.
type ConvertArvanCloudToCnameSetupInput = arvanCloudDomainNameInput

// ConvertArvanCloudToCnameSetup switches a domain's onboarding mode to CNAME
// Setup ("partial") — the mode for a caller who says something like "my
// domain's DNS is hosted elsewhere, I just want the CDN on a subdomain". This
// is a fast operation.
type ConvertArvanCloudToCnameSetup struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewConvertArvanCloudToCnameSetup builds the use case from its ports.
func NewConvertArvanCloudToCnameSetup(queue ports.Queue, provider ports.ArvanCloudProvider) *ConvertArvanCloudToCnameSetup {
	return &ConvertArvanCloudToCnameSetup{queue: queue, provider: provider}
}

// Execute converts the domain and returns it as ArvanCloud reports it
// afterward.
func (uc *ConvertArvanCloudToCnameSetup) Execute(ctx context.Context, in ConvertArvanCloudToCnameSetupInput) (*domain.ArvanCloudDomain, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDomain(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDomain, error) {
		converted, err := uc.provider.ConvertToCnameSetup(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("converting arvancloud domain %q to CNAME setup: %w", in.DomainName, err)
		}
		return converted, nil
	})
}

// CheckArvanCloudCnameStatusInput identifies the domain whose CNAME status to
// check.
type CheckArvanCloudCnameStatusInput = arvanCloudDomainNameInput

// CheckArvanCloudCnameStatus reports whether a "partial" domain's CNAME has
// been activated yet. This is a fast operation.
type CheckArvanCloudCnameStatus struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCheckArvanCloudCnameStatus builds the use case from its ports.
func NewCheckArvanCloudCnameStatus(queue ports.Queue, provider ports.ArvanCloudProvider) *CheckArvanCloudCnameStatus {
	return &CheckArvanCloudCnameStatus{queue: queue, provider: provider}
}

// Execute returns the domain's current CNAME Setup status.
func (uc *CheckArvanCloudCnameStatus) Execute(ctx context.Context, in CheckArvanCloudCnameStatusInput) (*domain.ArvanCloudDomain, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudDomain(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudDomain, error) {
		status, err := uc.provider.CheckCnameStatus(ctx, in.Credentials, in.DomainName)
		if err != nil {
			return nil, fmt.Errorf("checking CNAME status for arvancloud domain %q: %w", in.DomainName, err)
		}
		return status, nil
	})
}

// CloneArvanCloudDomainConfigInput identifies the domain to copy a
// configuration onto and the domain to copy it from.
type CloneArvanCloudDomainConfigInput struct {
	Credentials domain.ProviderCredentials
	DomainName  string
	FromDomain  string
}

// CloneArvanCloudDomainConfig copies another domain's CDN configuration onto
// this one. This is a fast operation.
type CloneArvanCloudDomainConfig struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCloneArvanCloudDomainConfig builds the use case from its ports.
func NewCloneArvanCloudDomainConfig(queue ports.Queue, provider ports.ArvanCloudProvider) *CloneArvanCloudDomainConfig {
	return &CloneArvanCloudDomainConfig{queue: queue, provider: provider}
}

// Execute clones fromDomain's configuration onto domainName.
func (uc *CloneArvanCloudDomainConfig) Execute(ctx context.Context, in CloneArvanCloudDomainConfigInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.DomainName == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.FromDomain == "" {
		return fmt.Errorf("from_domain is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.CloneDomainConfig(ctx, in.Credentials, in.DomainName, in.FromDomain); err != nil {
			return nil, fmt.Errorf("cloning arvancloud domain config from %q onto %q: %w", in.FromDomain, in.DomainName, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// RegenerateArvanCloudDomainConfigInput identifies the domain to
// re-publish.
type RegenerateArvanCloudDomainConfigInput = arvanCloudDomainNameInput

// RegenerateArvanCloudDomainConfig re-publishes a domain's current
// configuration to the edge servers. This is a fast operation: the call
// itself returns immediately, even though the actual propagation to edge
// servers happens asynchronously afterward, on ArvanCloud's own side, with
// nothing exposed to poll for it.
type RegenerateArvanCloudDomainConfig struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewRegenerateArvanCloudDomainConfig builds the use case from its ports.
func NewRegenerateArvanCloudDomainConfig(queue ports.Queue, provider ports.ArvanCloudProvider) *RegenerateArvanCloudDomainConfig {
	return &RegenerateArvanCloudDomainConfig{queue: queue, provider: provider}
}

// Execute triggers the regeneration.
func (uc *RegenerateArvanCloudDomainConfig) Execute(ctx context.Context, in RegenerateArvanCloudDomainConfigInput) error {
	if err := in.validate(); err != nil {
		return err
	}
	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.RegenerateDomainConfig(ctx, in.Credentials, in.DomainName); err != nil {
			return nil, fmt.Errorf("regenerating arvancloud domain config for %q: %w", in.DomainName, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// HoldArvanCloudDomainInput identifies the domain to hold.
type HoldArvanCloudDomainInput = arvanCloudDomainNameInput

// HoldArvanCloudDomain pauses CDN service for a domain. This is a fast
// operation.
type HoldArvanCloudDomain struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewHoldArvanCloudDomain builds the use case from its ports.
func NewHoldArvanCloudDomain(queue ports.Queue, provider ports.ArvanCloudProvider) *HoldArvanCloudDomain {
	return &HoldArvanCloudDomain{queue: queue, provider: provider}
}

// Execute holds the domain's CDN service.
func (uc *HoldArvanCloudDomain) Execute(ctx context.Context, in HoldArvanCloudDomainInput) error {
	if err := in.validate(); err != nil {
		return err
	}
	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.HoldDomain(ctx, in.Credentials, in.DomainName); err != nil {
			return nil, fmt.Errorf("holding arvancloud domain %q: %w", in.DomainName, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// UnholdArvanCloudDomainInput identifies the domain to resume.
type UnholdArvanCloudDomainInput = arvanCloudDomainNameInput

// UnholdArvanCloudDomain resumes CDN service for a domain previously held.
// This is a fast operation.
type UnholdArvanCloudDomain struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUnholdArvanCloudDomain builds the use case from its ports.
func NewUnholdArvanCloudDomain(queue ports.Queue, provider ports.ArvanCloudProvider) *UnholdArvanCloudDomain {
	return &UnholdArvanCloudDomain{queue: queue, provider: provider}
}

// Execute resumes the domain's CDN service.
func (uc *UnholdArvanCloudDomain) Execute(ctx context.Context, in UnholdArvanCloudDomainInput) error {
	if err := in.validate(); err != nil {
		return err
	}
	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UnholdDomain(ctx, in.Credentials, in.DomainName); err != nil {
			return nil, fmt.Errorf("unholding arvancloud domain %q: %w", in.DomainName, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// dispatchArvanCloudDomain runs fn on the queue and decodes its result back
// into a *domain.ArvanCloudDomain, the shape every use case above but the
// clone/regenerate/hold/unhold group returns.
func dispatchArvanCloudDomain(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudDomain, error),
) (*domain.ArvanCloudDomain, error) {
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

	var result domain.ArvanCloudDomain
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud domain: %w", err)
	}
	return &result, nil
}
