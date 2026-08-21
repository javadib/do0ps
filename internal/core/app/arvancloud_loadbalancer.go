package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the Load Balancing use cases for ArvanCloud (issue #69):
// CDN edge-level traffic distribution across origin pools — a 3-level
// resource hierarchy (load balancer -> pools -> origins), see
// domain/arvancloud_loadbalancer.go's package comment for the naming
// collision warning against domain.LoadBalancer. Every one of them is a
// fast operation (ports.ArvanCloudProvider, AGENTS.md 4.3): each dispatches
// onto the queue and blocks for the result within the same tool call.
// Creating a load balancer configures routing rules, it does not provision
// new infrastructure — unlike Parspack's cloud-server ProvisionLoadBalancer
// (a long operation), do not copy that polling pattern here.
//
// What IS validated client-side, per issue #69's acceptance criteria:
//
//   - A load balancer's Method against domain.ValidArvanCloudLoadBalancerMethod
//     ("failover"/"cluster_rr"/"cluster_chash").
//   - A pool's Method against the narrower domain.ValidArvanCloudPoolMethod
//     ("cluster_rr"/"cluster_chash" only, no "failover" — see
//     domain.ArvanCloudPoolMethod's doc comment for the confirmed asymmetry).
//   - An origin's Address/Port/Weight/Protocol, mirroring
//     validateArvanCloudRateLimitRuleInput's "catch it before an opaque 422"
//     reasoning.
//   - A reprioritize call's "exactly one of after/before" contract — its own
//     validator, validateArvanCloudLBPoolPrioritize, since the field names
//     (pool_id/after_pool_id/before_pool_id) differ from
//     validateArvanCloudReprioritize's rule_id/after_rule_id/before_rule_id
//     (arvancloud_firewall.go), even though the shape is the same idea.

// validateArvanCloudLBPoolPrioritize checks the shared shape of a pool
// reprioritize call: poolID is always required (PrioritizePoolAfter/
// PrioritizePoolBefore's own required pool_id), and
// afterPoolID/beforePoolID must not both be given — the spec's PrioritizePool
// schema is an anyOf of the two, each carrying exactly one relative-position
// field.
func validateArvanCloudLBPoolPrioritize(poolID, afterPoolID, beforePoolID string) error {
	if poolID == "" {
		return fmt.Errorf("pool_id is required: %w", domain.ErrInvalidInput)
	}
	if afterPoolID != "" && beforePoolID != "" {
		return fmt.Errorf("only one of after_pool_id or before_pool_id may be given, not both: %w", domain.ErrInvalidInput)
	}
	if afterPoolID == "" && beforePoolID == "" {
		return fmt.Errorf("one of after_pool_id or before_pool_id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// --- Regions ----------------------------------------------------------------

// ListArvanCloudLBRegionsInput carries only credentials: the global region
// list is account-independent.
type ListArvanCloudLBRegionsInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudLBRegions is a fast operation.
type ListArvanCloudLBRegions struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudLBRegions builds the use case from its ports.
func NewListArvanCloudLBRegions(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudLBRegions {
	return &ListArvanCloudLBRegions{queue: queue, provider: provider}
}

// Execute returns every region load-balancer pools can be scoped to,
// account-independent.
func (uc *ListArvanCloudLBRegions) Execute(ctx context.Context, in ListArvanCloudLBRegionsInput) ([]domain.ArvanCloudLoadBalancerRegion, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		regions, err := uc.provider.ListArvanCloudLBRegions(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud load balancer regions: %w", err)
		}
		return json.Marshal(regions)
	})
	if err != nil {
		return nil, err
	}

	var regions []domain.ArvanCloudLoadBalancerRegion
	if err := json.Unmarshal(raw, &regions); err != nil {
		return nil, fmt.Errorf("decoding arvancloud load balancer region list: %w", err)
	}
	return regions, nil
}

// arvanCloudLBDomainInput is embedded by every use case below that is scoped
// to exactly one domain by name and needs nothing else.
type arvanCloudLBDomainInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

func (in arvanCloudLBDomainInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// ListArvanCloudDomainLBRegionsInput identifies the domain whose
// load-balancer regions to list.
type ListArvanCloudDomainLBRegionsInput = arvanCloudLBDomainInput

// ListArvanCloudDomainLBRegions is a fast operation.
type ListArvanCloudDomainLBRegions struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudDomainLBRegions builds the use case from its ports.
func NewListArvanCloudDomainLBRegions(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudDomainLBRegions {
	return &ListArvanCloudDomainLBRegions{queue: queue, provider: provider}
}

// Execute returns the regions load-balancer pools on the domain can be
// scoped to.
func (uc *ListArvanCloudDomainLBRegions) Execute(ctx context.Context, in ListArvanCloudDomainLBRegionsInput) ([]domain.ArvanCloudLoadBalancerRegion, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		regions, err := uc.provider.ListArvanCloudDomainLBRegions(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud load balancer regions for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(regions)
	})
	if err != nil {
		return nil, err
	}

	var regions []domain.ArvanCloudLoadBalancerRegion
	if err := json.Unmarshal(raw, &regions); err != nil {
		return nil, fmt.Errorf("decoding arvancloud load balancer region list: %w", err)
	}
	return regions, nil
}

// --- Settings -----------------------------------------------------------

// validateArvanCloudLBSettingsInput checks the enum fields of a load
// balancer settings update, each only when the caller actually set it (every
// field on LoadBalancerSetting is optional).
func validateArvanCloudLBSettingsInput(s domain.ArvanCloudLoadBalancerSettings) error {
	if s.Method != "" && !domain.ValidArvanCloudPoolMethod(string(s.Method)) {
		return fmt.Errorf("method %q is not one of \"cluster_rr\" or \"cluster_chash\": %w", s.Method, domain.ErrInvalidInput)
	}
	if s.NextUpstreamTCP != "" && !domain.ValidArvanCloudLBToggle(string(s.NextUpstreamTCP)) {
		return fmt.Errorf("next_upstream_tcp %q is not \"on\" or \"off\": %w", s.NextUpstreamTCP, domain.ErrInvalidInput)
	}
	if s.Keepalive != "" && !domain.ValidArvanCloudLBToggle(string(s.Keepalive)) {
		return fmt.Errorf("keepalive %q is not \"on\" or \"off\": %w", s.Keepalive, domain.ErrInvalidInput)
	}
	if s.Protocol != "" && !domain.ValidArvanCloudOriginProtocol(string(s.Protocol)) {
		return fmt.Errorf("protocol %q is not one of \"auto\", \"http\" or \"https\": %w", s.Protocol, domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudLBSettingsInput identifies the domain whose load-balancing
// settings to look up.
type GetArvanCloudLBSettingsInput = arvanCloudLBDomainInput

// GetArvanCloudLBSettings is a fast operation.
type GetArvanCloudLBSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudLBSettings builds the use case from its ports.
func NewGetArvanCloudLBSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudLBSettings {
	return &GetArvanCloudLBSettings{queue: queue, provider: provider}
}

// Execute returns the domain's current load-balancing defaults.
func (uc *GetArvanCloudLBSettings) Execute(ctx context.Context, in GetArvanCloudLBSettingsInput) (*domain.ArvanCloudLoadBalancerSettings, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudLBSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancerSettings, error) {
		found, err := uc.provider.GetArvanCloudLBSettings(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud load balancer settings for domain %q: %w", in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudLBSettingsInput identifies the domain and its new
// load-balancing defaults.
type UpdateArvanCloudLBSettingsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Settings    domain.ArvanCloudLoadBalancerSettings
}

// UpdateArvanCloudLBSettings changes a domain's load-balancing defaults.
// This is a fast operation.
type UpdateArvanCloudLBSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudLBSettings builds the use case from its ports.
func NewUpdateArvanCloudLBSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudLBSettings {
	return &UpdateArvanCloudLBSettings{queue: queue, provider: provider}
}

// Execute updates the settings, returning them as stored afterward.
func (uc *UpdateArvanCloudLBSettings) Execute(ctx context.Context, in UpdateArvanCloudLBSettingsInput) (*domain.ArvanCloudLoadBalancerSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudLBSettingsInput(in.Settings); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLBSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancerSettings, error) {
		updated, err := uc.provider.UpdateArvanCloudLBSettings(ctx, in.Credentials, in.Domain, in.Settings)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud load balancer settings for domain %q: %w", in.Domain, err)
		}
		return updated, nil
	})
}

// --- Load balancers -------------------------------------------------------

// validateArvanCloudLoadBalancerInput checks the fields every create/update
// load balancer call shares: name and a method from
// domain.ValidArvanCloudLoadBalancerMethod's set.
func validateArvanCloudLoadBalancerInput(lb domain.ArvanCloudLoadBalancer) error {
	if lb.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudLoadBalancerMethod(string(lb.Method)) {
		return fmt.Errorf("method %q is not one of \"failover\", \"cluster_rr\" or \"cluster_chash\": %w", lb.Method, domain.ErrInvalidInput)
	}
	return nil
}

// ListArvanCloudLoadBalancersInput identifies the domain whose load
// balancers to list.
type ListArvanCloudLoadBalancersInput = arvanCloudLBDomainInput

// ListArvanCloudLoadBalancers is a fast operation.
type ListArvanCloudLoadBalancers struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudLoadBalancers builds the use case from its ports.
func NewListArvanCloudLoadBalancers(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudLoadBalancers {
	return &ListArvanCloudLoadBalancers{queue: queue, provider: provider}
}

// Execute returns every load balancer configured for the domain.
func (uc *ListArvanCloudLoadBalancers) Execute(ctx context.Context, in ListArvanCloudLoadBalancersInput) ([]domain.ArvanCloudLoadBalancer, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		lbs, err := uc.provider.ListArvanCloudLoadBalancers(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud load balancers for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(lbs)
	})
	if err != nil {
		return nil, err
	}

	var lbs []domain.ArvanCloudLoadBalancer
	if err := json.Unmarshal(raw, &lbs); err != nil {
		return nil, fmt.Errorf("decoding arvancloud load balancer list: %w", err)
	}
	return lbs, nil
}

// CreateArvanCloudLoadBalancerInput is the normalized form of a
// create_arvancloud_load_balancer tool call.
type CreateArvanCloudLoadBalancerInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	LB          domain.ArvanCloudLoadBalancer
}

// CreateArvanCloudLoadBalancer creates a new load balancer. This is a fast
// operation: creating a load balancer configures routing rules, it does not
// provision new infrastructure — unlike Parspack's cloud-server
// ProvisionLoadBalancer (a long operation, AGENTS.md 4.3), so this use case
// dispatches onto the queue and returns the created load balancer within the
// same tool call, with no operation_id to poll afterward.
type CreateArvanCloudLoadBalancer struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudLoadBalancer builds the use case from its ports.
func NewCreateArvanCloudLoadBalancer(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudLoadBalancer {
	return &CreateArvanCloudLoadBalancer{queue: queue, provider: provider}
}

// Execute validates the request and creates the load balancer, returning it
// as stored.
func (uc *CreateArvanCloudLoadBalancer) Execute(ctx context.Context, in CreateArvanCloudLoadBalancerInput) (*domain.ArvanCloudLoadBalancer, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudLoadBalancerInput(in.LB); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLoadBalancer(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancer, error) {
		created, err := uc.provider.CreateArvanCloudLoadBalancer(ctx, in.Credentials, in.Domain, in.LB)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud load balancer on domain %q: %w", in.Domain, err)
		}
		return created, nil
	})
}

// arvanCloudLBIDInput is embedded by every use case below that is scoped to
// exactly one load balancer by domain + id.
type arvanCloudLBIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
}

func (in arvanCloudLBIDInput) validate() error {
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

// GetArvanCloudLoadBalancerInput identifies the load balancer to look up.
type GetArvanCloudLoadBalancerInput = arvanCloudLBIDInput

// GetArvanCloudLoadBalancer is a fast operation.
type GetArvanCloudLoadBalancer struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudLoadBalancer builds the use case from its ports.
func NewGetArvanCloudLoadBalancer(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudLoadBalancer {
	return &GetArvanCloudLoadBalancer{queue: queue, provider: provider}
}

// Execute returns the current state of one load balancer.
func (uc *GetArvanCloudLoadBalancer) Execute(ctx context.Context, in GetArvanCloudLoadBalancerInput) (*domain.ArvanCloudLoadBalancer, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudLoadBalancer(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancer, error) {
		found, err := uc.provider.GetArvanCloudLoadBalancer(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud load balancer %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudLoadBalancerInput identifies the load balancer to update
// and its new field values.
type UpdateArvanCloudLoadBalancerInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	LB          domain.ArvanCloudLoadBalancer
}

// UpdateArvanCloudLoadBalancer changes a load balancer. This is a fast
// operation.
type UpdateArvanCloudLoadBalancer struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudLoadBalancer builds the use case from its ports.
func NewUpdateArvanCloudLoadBalancer(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudLoadBalancer {
	return &UpdateArvanCloudLoadBalancer{queue: queue, provider: provider}
}

// Execute updates the load balancer and returns it as stored afterward.
func (uc *UpdateArvanCloudLoadBalancer) Execute(ctx context.Context, in UpdateArvanCloudLoadBalancerInput) (*domain.ArvanCloudLoadBalancer, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudLoadBalancerInput(in.LB); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLoadBalancer(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancer, error) {
		updated, err := uc.provider.UpdateArvanCloudLoadBalancer(ctx, in.Credentials, in.Domain, in.ID, in.LB)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud load balancer %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudLoadBalancerInput identifies the load balancer to remove.
type DeleteArvanCloudLoadBalancerInput = arvanCloudLBIDInput

// DeleteArvanCloudLoadBalancer is a fast operation. Deleting a load balancer
// the provider no longer has is treated as already done rather than an
// error, matching DeleteArvanCloudRateLimitRule's tolerant-delete contract.
type DeleteArvanCloudLoadBalancer struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudLoadBalancer builds the use case from its ports.
func NewDeleteArvanCloudLoadBalancer(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudLoadBalancer {
	return &DeleteArvanCloudLoadBalancer{queue: queue, provider: provider}
}

// Execute deletes the load balancer, tolerating one that is already gone.
func (uc *DeleteArvanCloudLoadBalancer) Execute(ctx context.Context, in DeleteArvanCloudLoadBalancerInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudLoadBalancer(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud load balancer %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Pools ------------------------------------------------------------------

// validateArvanCloudLBPoolInput checks the fields every create/update pool
// call shares: name and a method from domain.ValidArvanCloudPoolMethod's
// (narrower) set.
func validateArvanCloudLBPoolInput(pool domain.ArvanCloudLoadBalancerPool) error {
	if pool.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudPoolMethod(string(pool.Method)) {
		return fmt.Errorf("method %q is not one of \"cluster_rr\" or \"cluster_chash\": %w", pool.Method, domain.ErrInvalidInput)
	}
	for i, origin := range pool.Origins {
		if err := validateArvanCloudLBOriginInput(origin); err != nil {
			return fmt.Errorf("origins[%d]: %w", i, err)
		}
	}
	return nil
}

// arvanCloudLBPoolListInput is embedded by every use case below that lists or
// creates a pool, scoped to exactly one load balancer by domain + id.
type arvanCloudLBPoolListInput struct {
	Credentials    domain.ProviderCredentials
	Domain         string
	LoadBalancerID string
}

func (in arvanCloudLBPoolListInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.LoadBalancerID == "" {
		return fmt.Errorf("load_balancer_id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// ListArvanCloudLBPoolsInput identifies the load balancer whose pools to
// list.
type ListArvanCloudLBPoolsInput = arvanCloudLBPoolListInput

// ListArvanCloudLBPools is a fast operation.
type ListArvanCloudLBPools struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudLBPools builds the use case from its ports.
func NewListArvanCloudLBPools(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudLBPools {
	return &ListArvanCloudLBPools{queue: queue, provider: provider}
}

// Execute returns every pool of the load balancer.
func (uc *ListArvanCloudLBPools) Execute(ctx context.Context, in ListArvanCloudLBPoolsInput) ([]domain.ArvanCloudLoadBalancerPool, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		pools, err := uc.provider.ListArvanCloudLBPools(ctx, in.Credentials, in.Domain, in.LoadBalancerID)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud load balancer pools (domain %q, lb %q): %w", in.Domain, in.LoadBalancerID, err)
		}
		return json.Marshal(pools)
	})
	if err != nil {
		return nil, err
	}

	var pools []domain.ArvanCloudLoadBalancerPool
	if err := json.Unmarshal(raw, &pools); err != nil {
		return nil, fmt.Errorf("decoding arvancloud load balancer pool list: %w", err)
	}
	return pools, nil
}

// CreateArvanCloudLBPoolInput is the normalized form of a
// create_arvancloud_lb_pool tool call.
type CreateArvanCloudLBPoolInput struct {
	Credentials    domain.ProviderCredentials
	Domain         string
	LoadBalancerID string
	Pool           domain.ArvanCloudLoadBalancerPool
}

// CreateArvanCloudLBPool creates a new pool, optionally with its initial set
// of origins in the same call. This is a fast operation.
type CreateArvanCloudLBPool struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudLBPool builds the use case from its ports.
func NewCreateArvanCloudLBPool(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudLBPool {
	return &CreateArvanCloudLBPool{queue: queue, provider: provider}
}

// Execute validates the request and creates the pool, returning it as
// stored.
func (uc *CreateArvanCloudLBPool) Execute(ctx context.Context, in CreateArvanCloudLBPoolInput) (*domain.ArvanCloudLoadBalancerPool, error) {
	if err := (arvanCloudLBPoolListInput{Credentials: in.Credentials, Domain: in.Domain, LoadBalancerID: in.LoadBalancerID}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudLBPoolInput(in.Pool); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLBPool(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancerPool, error) {
		created, err := uc.provider.CreateArvanCloudLBPool(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.Pool)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud load balancer pool (domain %q, lb %q): %w", in.Domain, in.LoadBalancerID, err)
		}
		return created, nil
	})
}

// ReprioritizeArvanCloudLBPoolInput identifies the load balancer, the pool
// to move, and exactly one of AfterPoolID/BeforePoolID to move it relative
// to.
type ReprioritizeArvanCloudLBPoolInput struct {
	Credentials    domain.ProviderCredentials
	Domain         string
	LoadBalancerID string
	PoolID         string
	AfterPoolID    string
	BeforePoolID   string
}

// ReprioritizeArvanCloudLBPool is a fast operation. Unlike the rule-engine
// reprioritize use cases elsewhere in this package (e.g.
// ReprioritizeArvanCloudFirewallRules), this returns the load balancer as
// stored afterward — load-balancers.prioritize_pool's response carries the
// updated resource, not just a confirmation.
type ReprioritizeArvanCloudLBPool struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewReprioritizeArvanCloudLBPool builds the use case from its ports.
func NewReprioritizeArvanCloudLBPool(queue ports.Queue, provider ports.ArvanCloudProvider) *ReprioritizeArvanCloudLBPool {
	return &ReprioritizeArvanCloudLBPool{queue: queue, provider: provider}
}

// Execute moves PoolID to its new position and returns the load balancer as
// stored afterward.
func (uc *ReprioritizeArvanCloudLBPool) Execute(ctx context.Context, in ReprioritizeArvanCloudLBPoolInput) (*domain.ArvanCloudLoadBalancer, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.LoadBalancerID == "" {
		return nil, fmt.Errorf("load_balancer_id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudLBPoolPrioritize(in.PoolID, in.AfterPoolID, in.BeforePoolID); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLoadBalancer(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancer, error) {
		updated, err := uc.provider.ReprioritizeArvanCloudLBPool(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.PoolID, in.AfterPoolID, in.BeforePoolID)
		if err != nil {
			return nil, fmt.Errorf("reprioritizing arvancloud load balancer pool %q (domain %q, lb %q): %w", in.PoolID, in.Domain, in.LoadBalancerID, err)
		}
		return updated, nil
	})
}

// arvanCloudLBPoolIDInput is embedded by every use case below that is scoped
// to exactly one pool by domain + load balancer + pool id.
type arvanCloudLBPoolIDInput struct {
	Credentials    domain.ProviderCredentials
	Domain         string
	LoadBalancerID string
	PoolID         string
}

func (in arvanCloudLBPoolIDInput) validate() error {
	if err := (arvanCloudLBPoolListInput{Credentials: in.Credentials, Domain: in.Domain, LoadBalancerID: in.LoadBalancerID}).validate(); err != nil {
		return err
	}
	if in.PoolID == "" {
		return fmt.Errorf("pool_id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudLBPoolInput identifies the pool to look up.
type GetArvanCloudLBPoolInput = arvanCloudLBPoolIDInput

// GetArvanCloudLBPool is a fast operation.
type GetArvanCloudLBPool struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudLBPool builds the use case from its ports.
func NewGetArvanCloudLBPool(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudLBPool {
	return &GetArvanCloudLBPool{queue: queue, provider: provider}
}

// Execute returns the current state of one pool.
func (uc *GetArvanCloudLBPool) Execute(ctx context.Context, in GetArvanCloudLBPoolInput) (*domain.ArvanCloudLoadBalancerPool, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudLBPool(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancerPool, error) {
		found, err := uc.provider.GetArvanCloudLBPool(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.PoolID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud load balancer pool %q (domain %q, lb %q): %w", in.PoolID, in.Domain, in.LoadBalancerID, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudLBPoolInput identifies the pool to update and its new
// field values, shared by both ReplaceArvanCloudLBPoolWithOrigins (PUT) and
// UpdateArvanCloudLBPoolSettings (PATCH) below — the two share the same
// input shape, differing only in whether Pool.Origins is honored.
type UpdateArvanCloudLBPoolInput struct {
	Credentials    domain.ProviderCredentials
	Domain         string
	LoadBalancerID string
	PoolID         string
	Pool           domain.ArvanCloudLoadBalancerPool
}

// ReplaceArvanCloudLBPoolWithOrigins (PUT) replaces the pool AND its full
// set of origins in one call — any existing origin not included in
// Pool.Origins is removed. This is a fast operation.
//
// See UpdateArvanCloudLBPoolSettings's doc comment for why these are two
// distinct use cases rather than one with a flag: issue #69's acceptance
// criteria calls the PUT-vs-PATCH distinction the highest-risk part of this
// capability.
type ReplaceArvanCloudLBPoolWithOrigins struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewReplaceArvanCloudLBPoolWithOrigins builds the use case from its ports.
func NewReplaceArvanCloudLBPoolWithOrigins(queue ports.Queue, provider ports.ArvanCloudProvider) *ReplaceArvanCloudLBPoolWithOrigins {
	return &ReplaceArvanCloudLBPoolWithOrigins{queue: queue, provider: provider}
}

// Execute replaces the pool and its origins, returning it as stored
// afterward.
func (uc *ReplaceArvanCloudLBPoolWithOrigins) Execute(ctx context.Context, in UpdateArvanCloudLBPoolInput) (*domain.ArvanCloudLoadBalancerPool, error) {
	if err := (arvanCloudLBPoolIDInput{Credentials: in.Credentials, Domain: in.Domain, LoadBalancerID: in.LoadBalancerID, PoolID: in.PoolID}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudLBPoolInput(in.Pool); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLBPool(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancerPool, error) {
		updated, err := uc.provider.UpdateArvanCloudLBPoolWithOrigins(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.PoolID, in.Pool)
		if err != nil {
			return nil, fmt.Errorf("replacing arvancloud load balancer pool %q with origins (domain %q, lb %q): %w", in.PoolID, in.Domain, in.LoadBalancerID, err)
		}
		return updated, nil
	})
}

// UpdateArvanCloudLBPoolSettings (PATCH) updates the pool's own settings
// only and leaves its existing origins untouched — Pool.Origins on the
// input is ignored. This is a fast operation.
//
// This is deliberately a separate use case from
// ReplaceArvanCloudLBPoolWithOrigins above, mirroring the port's own
// UpdateArvanCloudLBPoolWithOrigins/UpdateArvanCloudLBPoolSettings split:
// silently picking the wrong one either wipes origins the caller didn't
// intend to touch, or fails to apply origin changes the caller expected —
// see ports.ArvanCloudProvider's doc comment. A future reader must not
// collapse these back into one method with a boolean flag.
type UpdateArvanCloudLBPoolSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudLBPoolSettings builds the use case from its ports.
func NewUpdateArvanCloudLBPoolSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudLBPoolSettings {
	return &UpdateArvanCloudLBPoolSettings{queue: queue, provider: provider}
}

// Execute updates the pool's settings, returning it as stored afterward.
// Origins are never touched by this call.
func (uc *UpdateArvanCloudLBPoolSettings) Execute(ctx context.Context, in UpdateArvanCloudLBPoolInput) (*domain.ArvanCloudLoadBalancerPool, error) {
	if err := (arvanCloudLBPoolIDInput{Credentials: in.Credentials, Domain: in.Domain, LoadBalancerID: in.LoadBalancerID, PoolID: in.PoolID}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudLBPoolInput(in.Pool); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLBPool(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancerPool, error) {
		updated, err := uc.provider.UpdateArvanCloudLBPoolSettings(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.PoolID, in.Pool)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud load balancer pool %q settings (domain %q, lb %q): %w", in.PoolID, in.Domain, in.LoadBalancerID, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudLBPoolInput identifies the pool to remove.
type DeleteArvanCloudLBPoolInput = arvanCloudLBPoolIDInput

// DeleteArvanCloudLBPool is a fast operation. Deleting a pool the provider
// no longer has is treated as already done rather than an error, matching
// DeleteArvanCloudLoadBalancer's tolerant-delete contract.
type DeleteArvanCloudLBPool struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudLBPool builds the use case from its ports.
func NewDeleteArvanCloudLBPool(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudLBPool {
	return &DeleteArvanCloudLBPool{queue: queue, provider: provider}
}

// Execute deletes the pool, tolerating one that is already gone.
func (uc *DeleteArvanCloudLBPool) Execute(ctx context.Context, in DeleteArvanCloudLBPoolInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudLBPool(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.PoolID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud load balancer pool %q (domain %q, lb %q): %w", in.PoolID, in.Domain, in.LoadBalancerID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Origins ------------------------------------------------------------

// validateArvanCloudLBOriginInput checks the fields every create/update
// origin call shares: address, port and weight in range, and a protocol
// from domain.ValidArvanCloudOriginProtocol's set.
func validateArvanCloudLBOriginInput(origin domain.ArvanCloudLoadBalancerOrigin) error {
	if origin.Address == "" {
		return fmt.Errorf("address is required: %w", domain.ErrInvalidInput)
	}
	if origin.Port < 1 || origin.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d: %w", origin.Port, domain.ErrInvalidInput)
	}
	if origin.Weight < 1 || origin.Weight > 1000 {
		return fmt.Errorf("weight must be between 1 and 1000, got %d: %w", origin.Weight, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudOriginProtocol(string(origin.Protocol)) {
		return fmt.Errorf("protocol %q is not one of \"auto\", \"http\" or \"https\": %w", origin.Protocol, domain.ErrInvalidInput)
	}
	return nil
}

// arvanCloudLBPoolOriginListInput is embedded by every use case below that
// lists or creates an origin, scoped to exactly one pool by domain + load
// balancer + pool id.
type arvanCloudLBPoolOriginListInput struct {
	Credentials    domain.ProviderCredentials
	Domain         string
	LoadBalancerID string
	PoolID         string
}

func (in arvanCloudLBPoolOriginListInput) validate() error {
	return (arvanCloudLBPoolIDInput(in)).validate()
}

// ListArvanCloudLBPoolOriginsInput identifies the pool whose origins to
// list.
type ListArvanCloudLBPoolOriginsInput = arvanCloudLBPoolOriginListInput

// ListArvanCloudLBPoolOrigins is a fast operation.
type ListArvanCloudLBPoolOrigins struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudLBPoolOrigins builds the use case from its ports.
func NewListArvanCloudLBPoolOrigins(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudLBPoolOrigins {
	return &ListArvanCloudLBPoolOrigins{queue: queue, provider: provider}
}

// Execute returns every origin in the pool.
func (uc *ListArvanCloudLBPoolOrigins) Execute(ctx context.Context, in ListArvanCloudLBPoolOriginsInput) ([]domain.ArvanCloudLoadBalancerOrigin, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		origins, err := uc.provider.ListArvanCloudLBPoolOrigins(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.PoolID)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud load balancer pool origins (domain %q, lb %q, pool %q): %w", in.Domain, in.LoadBalancerID, in.PoolID, err)
		}
		return json.Marshal(origins)
	})
	if err != nil {
		return nil, err
	}

	var origins []domain.ArvanCloudLoadBalancerOrigin
	if err := json.Unmarshal(raw, &origins); err != nil {
		return nil, fmt.Errorf("decoding arvancloud load balancer pool origin list: %w", err)
	}
	return origins, nil
}

// CreateArvanCloudLBPoolOriginInput is the normalized form of a
// create_arvancloud_lb_pool_origin tool call.
type CreateArvanCloudLBPoolOriginInput struct {
	Credentials    domain.ProviderCredentials
	Domain         string
	LoadBalancerID string
	PoolID         string
	Origin         domain.ArvanCloudLoadBalancerOrigin
}

// CreateArvanCloudLBPoolOrigin creates a new origin in a pool. This is a
// fast operation.
type CreateArvanCloudLBPoolOrigin struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudLBPoolOrigin builds the use case from its ports.
func NewCreateArvanCloudLBPoolOrigin(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudLBPoolOrigin {
	return &CreateArvanCloudLBPoolOrigin{queue: queue, provider: provider}
}

// Execute validates the request and creates the origin, returning it as
// stored.
func (uc *CreateArvanCloudLBPoolOrigin) Execute(ctx context.Context, in CreateArvanCloudLBPoolOriginInput) (*domain.ArvanCloudLoadBalancerOrigin, error) {
	if err := (arvanCloudLBPoolOriginListInput{Credentials: in.Credentials, Domain: in.Domain, LoadBalancerID: in.LoadBalancerID, PoolID: in.PoolID}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudLBOriginInput(in.Origin); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLBOrigin(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancerOrigin, error) {
		created, err := uc.provider.CreateArvanCloudLBPoolOrigin(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.PoolID, in.Origin)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud load balancer pool origin (domain %q, lb %q, pool %q): %w", in.Domain, in.LoadBalancerID, in.PoolID, err)
		}
		return created, nil
	})
}

// arvanCloudLBPoolOriginIDInput is embedded by every use case below that is
// scoped to exactly one origin by domain + load balancer + pool + origin id.
type arvanCloudLBPoolOriginIDInput struct {
	Credentials    domain.ProviderCredentials
	Domain         string
	LoadBalancerID string
	PoolID         string
	OriginID       string
}

func (in arvanCloudLBPoolOriginIDInput) validate() error {
	if err := (arvanCloudLBPoolIDInput{Credentials: in.Credentials, Domain: in.Domain, LoadBalancerID: in.LoadBalancerID, PoolID: in.PoolID}).validate(); err != nil {
		return err
	}
	if in.OriginID == "" {
		return fmt.Errorf("origin_id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudLBPoolOriginInput identifies the origin to look up.
type GetArvanCloudLBPoolOriginInput = arvanCloudLBPoolOriginIDInput

// GetArvanCloudLBPoolOrigin is a fast operation.
type GetArvanCloudLBPoolOrigin struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudLBPoolOrigin builds the use case from its ports.
func NewGetArvanCloudLBPoolOrigin(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudLBPoolOrigin {
	return &GetArvanCloudLBPoolOrigin{queue: queue, provider: provider}
}

// Execute returns the current state of one origin.
func (uc *GetArvanCloudLBPoolOrigin) Execute(ctx context.Context, in GetArvanCloudLBPoolOriginInput) (*domain.ArvanCloudLoadBalancerOrigin, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudLBOrigin(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancerOrigin, error) {
		found, err := uc.provider.GetArvanCloudLBPoolOrigin(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.PoolID, in.OriginID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud load balancer pool origin %q (domain %q, lb %q, pool %q): %w", in.OriginID, in.Domain, in.LoadBalancerID, in.PoolID, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudLBPoolOriginInput identifies the origin to update and its
// new field values.
type UpdateArvanCloudLBPoolOriginInput struct {
	Credentials    domain.ProviderCredentials
	Domain         string
	LoadBalancerID string
	PoolID         string
	OriginID       string
	Origin         domain.ArvanCloudLoadBalancerOrigin
}

// UpdateArvanCloudLBPoolOrigin changes an origin. This is a fast operation.
type UpdateArvanCloudLBPoolOrigin struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudLBPoolOrigin builds the use case from its ports.
func NewUpdateArvanCloudLBPoolOrigin(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudLBPoolOrigin {
	return &UpdateArvanCloudLBPoolOrigin{queue: queue, provider: provider}
}

// Execute updates the origin and returns it as stored afterward.
func (uc *UpdateArvanCloudLBPoolOrigin) Execute(ctx context.Context, in UpdateArvanCloudLBPoolOriginInput) (*domain.ArvanCloudLoadBalancerOrigin, error) {
	if err := (arvanCloudLBPoolOriginIDInput{
		Credentials: in.Credentials, Domain: in.Domain, LoadBalancerID: in.LoadBalancerID, PoolID: in.PoolID, OriginID: in.OriginID,
	}).validate(); err != nil {
		return nil, err
	}
	if err := validateArvanCloudLBOriginInput(in.Origin); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLBOrigin(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLoadBalancerOrigin, error) {
		updated, err := uc.provider.UpdateArvanCloudLBPoolOrigin(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.PoolID, in.OriginID, in.Origin)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud load balancer pool origin %q (domain %q, lb %q, pool %q): %w", in.OriginID, in.Domain, in.LoadBalancerID, in.PoolID, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudLBPoolOriginInput identifies the origin to remove.
type DeleteArvanCloudLBPoolOriginInput = arvanCloudLBPoolOriginIDInput

// DeleteArvanCloudLBPoolOrigin is a fast operation. Deleting an origin the
// provider no longer has is treated as already done rather than an error,
// matching DeleteArvanCloudLBPool's tolerant-delete contract.
type DeleteArvanCloudLBPoolOrigin struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudLBPoolOrigin builds the use case from its ports.
func NewDeleteArvanCloudLBPoolOrigin(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudLBPoolOrigin {
	return &DeleteArvanCloudLBPoolOrigin{queue: queue, provider: provider}
}

// Execute deletes the origin, tolerating one that is already gone.
func (uc *DeleteArvanCloudLBPoolOrigin) Execute(ctx context.Context, in DeleteArvanCloudLBPoolOriginInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudLBPoolOrigin(ctx, in.Credentials, in.Domain, in.LoadBalancerID, in.PoolID, in.OriginID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud load balancer pool origin %q (domain %q, lb %q, pool %q): %w", in.OriginID, in.Domain, in.LoadBalancerID, in.PoolID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Dispatch helpers -------------------------------------------------

// dispatchArvanCloudLBSettings runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudLoadBalancerSettings.
func dispatchArvanCloudLBSettings(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudLoadBalancerSettings, error),
) (*domain.ArvanCloudLoadBalancerSettings, error) {
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

	var result domain.ArvanCloudLoadBalancerSettings
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud load balancer settings: %w", err)
	}
	return &result, nil
}

// dispatchArvanCloudLoadBalancer runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudLoadBalancer.
func dispatchArvanCloudLoadBalancer(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudLoadBalancer, error),
) (*domain.ArvanCloudLoadBalancer, error) {
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

	var result domain.ArvanCloudLoadBalancer
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud load balancer: %w", err)
	}
	return &result, nil
}

// dispatchArvanCloudLBPool runs fn on the queue and decodes its result back
// into a *domain.ArvanCloudLoadBalancerPool.
func dispatchArvanCloudLBPool(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudLoadBalancerPool, error),
) (*domain.ArvanCloudLoadBalancerPool, error) {
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

	var result domain.ArvanCloudLoadBalancerPool
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud load balancer pool: %w", err)
	}
	return &result, nil
}

// dispatchArvanCloudLBOrigin runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudLoadBalancerOrigin.
func dispatchArvanCloudLBOrigin(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudLoadBalancerOrigin, error),
) (*domain.ArvanCloudLoadBalancerOrigin, error) {
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

	var result domain.ArvanCloudLoadBalancerOrigin
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud load balancer origin: %w", err)
	}
	return &result, nil
}
