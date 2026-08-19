package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// cdnLoadBalanceProvider is the slice of provider behavior these use cases
// need: the CDN-edge load-balance pool and load-balance-server operations of
// issue #24 (docs/api-specs/parspack-cdn.openapi.yaml's "Load Balance" tag).
//
// It is declared locally, not as an addition to ports.ParspackProvider,
// because ports.go is being integrated centrally once every issue-24 slice
// lands; adding these methods there directly would conflict with concurrent
// work on the same file. The signatures below are shaped exactly like the
// extension ports.ParspackProvider is meant to gain — see this package's
// wiring notes for the exact doc comment to add there. *parspack.Client
// already implements every method here structurally, so no adapter change
// is needed to satisfy this interface.
//
// These are a completely different resource from ports.ParspackProvider's
// existing LoadBalancer methods (cloud-server/VM-network level, issue #12):
// every method here is prefixed CDN to keep the two apart.
type cdnLoadBalanceProvider interface {
	ListCDNLoadBalances(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string) ([]domain.CDNLoadBalance, error)
	CreateCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, lb domain.CDNLoadBalance) (*domain.CDNLoadBalance, error)
	GetCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNLoadBalance, error)
	UpdateCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string, lb domain.CDNLoadBalance) (*domain.CDNLoadBalance, error)
	DeleteCDNLoadBalance(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) error

	ListCDNLoadBalanceServers(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, loadBalanceID string) ([]domain.CDNLoadBalanceServer, error)
	CreateCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID string, srv domain.CDNLoadBalanceServer) (*domain.CDNLoadBalanceServer, error)
	GetCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) (*domain.CDNLoadBalanceServer, error)
	UpdateCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string, srv domain.CDNLoadBalanceServer) (*domain.CDNLoadBalanceServer, error)
	DeleteCDNLoadBalanceServer(ctx context.Context, creds domain.ProviderCredentials, zoneUUID, id string) error
}

// validateCDNLoadBalance checks a pool's shape against the enums the CDN API
// confirms (docs/api-specs/parspack-cdn.openapi.yaml, issue #24), so a bad
// method fails fast here instead of reaching the provider and coming back as
// a 422.
func validateCDNLoadBalance(lb domain.CDNLoadBalance) error {
	if lb.Name == "" {
		return fmt.Errorf("load balance name is required: %w", domain.ErrInvalidInput)
	}
	if lb.Method != "" && !domain.ValidCDNLoadBalanceMethod(lb.Method) {
		return fmt.Errorf("method %q is not one of the methods Parspack accepts: %w", lb.Method, domain.ErrInvalidInput)
	}
	for _, srv := range lb.Servers {
		if err := validateCDNLoadBalanceServer(srv); err != nil {
			return err
		}
	}
	return nil
}

// validateCDNLoadBalanceServer checks a backend server's shape against the
// enums the CDN API confirms.
func validateCDNLoadBalanceServer(srv domain.CDNLoadBalanceServer) error {
	if srv.IP == "" {
		return fmt.Errorf("server ip is required: %w", domain.ErrInvalidInput)
	}
	if srv.Group != "" && !domain.ValidCDNLoadBalanceServerGroup(srv.Group) {
		return fmt.Errorf("group %q is not one of primary/backup: %w", srv.Group, domain.ErrInvalidInput)
	}
	return nil
}

// ListCDNLoadBalancesInput identifies the zone whose load-balance pools to
// list.
type ListCDNLoadBalancesInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// ListCDNLoadBalances is a fast operation: the CDN API's index endpoint
// returns the current pool list synchronously (confirmed against the spec's
// "Load Balance" tag — no async status field to poll, unlike the
// cloud-server LoadBalancer's create flow, AGENTS.md 4.3). It runs on a
// worker but the caller waits for the result inside the same tool call.
type ListCDNLoadBalances struct {
	queue    ports.Queue
	provider cdnLoadBalanceProvider
}

// NewListCDNLoadBalances builds the use case from its ports.
func NewListCDNLoadBalances(queue ports.Queue, provider cdnLoadBalanceProvider) *ListCDNLoadBalances {
	return &ListCDNLoadBalances{queue: queue, provider: provider}
}

// Execute returns every load-balance pool configured in the given zone.
func (uc *ListCDNLoadBalances) Execute(ctx context.Context, in ListCDNLoadBalancesInput) ([]domain.CDNLoadBalance, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		balances, err := uc.provider.ListCDNLoadBalances(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("listing CDN load balances of zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(balances)
	})
	if err != nil {
		return nil, err
	}

	var balances []domain.CDNLoadBalance
	if err := json.Unmarshal(raw, &balances); err != nil {
		return nil, fmt.Errorf("decoding CDN load balance list: %w", err)
	}
	return balances, nil
}

// CreateCDNLoadBalanceInput is the normalized form of a
// create_cdn_load_balance tool call.
type CreateCDNLoadBalanceInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	LoadBalance domain.CDNLoadBalance
}

// CreateCDNLoadBalance is a fast operation: the provider's store endpoint
// returns synchronously with no "provisioning" state to poll (its 201
// example returns an empty data array, confirmed against the spec — there is
// no status field at all, so there is nothing that could transition
// asynchronously). It runs on a worker but the caller waits for the result
// inside the same tool call. Because the store endpoint's response carries
// no generated ID, the pool returned to the caller has no ID populated; use
// list_cdn_load_balances or get it by name to learn the ID afterward.
type CreateCDNLoadBalance struct {
	queue    ports.Queue
	provider cdnLoadBalanceProvider
}

// NewCreateCDNLoadBalance builds the use case from its ports.
func NewCreateCDNLoadBalance(queue ports.Queue, provider cdnLoadBalanceProvider) *CreateCDNLoadBalance {
	return &CreateCDNLoadBalance{queue: queue, provider: provider}
}

// Execute validates the request and creates the pool, returning what the
// provider echoed back synchronously.
func (uc *CreateCDNLoadBalance) Execute(ctx context.Context, in CreateCDNLoadBalanceInput) (*domain.CDNLoadBalance, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNLoadBalance(in.LoadBalance); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		created, err := uc.provider.CreateCDNLoadBalance(ctx, in.Credentials, in.ZoneUUID, in.LoadBalance)
		if err != nil {
			return nil, fmt.Errorf("creating CDN load balance %q in zone %s: %w", in.LoadBalance.Name, in.ZoneUUID, err)
		}
		return json.Marshal(created)
	})
	if err != nil {
		return nil, err
	}

	var created domain.CDNLoadBalance
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, fmt.Errorf("decoding created CDN load balance: %w", err)
	}
	return &created, nil
}

// GetCDNLoadBalanceInput identifies the pool to look up.
type GetCDNLoadBalanceInput struct {
	Credentials   domain.ProviderCredentials
	ZoneUUID      string
	LoadBalanceID string
}

// GetCDNLoadBalance is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNLoadBalance struct {
	queue    ports.Queue
	provider cdnLoadBalanceProvider
}

// NewGetCDNLoadBalance builds the use case from its ports.
func NewGetCDNLoadBalance(queue ports.Queue, provider cdnLoadBalanceProvider) *GetCDNLoadBalance {
	return &GetCDNLoadBalance{queue: queue, provider: provider}
}

// Execute returns the current state of one load-balance pool.
func (uc *GetCDNLoadBalance) Execute(ctx context.Context, in GetCDNLoadBalanceInput) (*domain.CDNLoadBalance, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.LoadBalanceID == "" {
		return nil, fmt.Errorf("load_balance_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		lb, err := uc.provider.GetCDNLoadBalance(ctx, in.Credentials, in.ZoneUUID, in.LoadBalanceID)
		if err != nil {
			return nil, fmt.Errorf("getting CDN load balance %s in zone %s: %w", in.LoadBalanceID, in.ZoneUUID, err)
		}
		return json.Marshal(lb)
	})
	if err != nil {
		return nil, err
	}

	var lb domain.CDNLoadBalance
	if err := json.Unmarshal(raw, &lb); err != nil {
		return nil, fmt.Errorf("decoding CDN load balance: %w", err)
	}
	return &lb, nil
}

// UpdateCDNLoadBalanceInput is the normalized form of an
// update_cdn_load_balance tool call.
type UpdateCDNLoadBalanceInput struct {
	Credentials   domain.ProviderCredentials
	ZoneUUID      string
	LoadBalanceID string
	LoadBalance   domain.CDNLoadBalance
}

// UpdateCDNLoadBalance is a fast operation: the provider's update endpoint
// returns synchronously with no state to poll, same reasoning as
// CreateCDNLoadBalance. Its response also carries nothing to decode, so the
// pool returned to the caller is what was sent, with the ID from the
// request.
type UpdateCDNLoadBalance struct {
	queue    ports.Queue
	provider cdnLoadBalanceProvider
}

// NewUpdateCDNLoadBalance builds the use case from its ports.
func NewUpdateCDNLoadBalance(queue ports.Queue, provider cdnLoadBalanceProvider) *UpdateCDNLoadBalance {
	return &UpdateCDNLoadBalance{queue: queue, provider: provider}
}

// Execute validates the request and updates the pool, returning what the
// provider echoed back synchronously.
func (uc *UpdateCDNLoadBalance) Execute(ctx context.Context, in UpdateCDNLoadBalanceInput) (*domain.CDNLoadBalance, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.LoadBalanceID == "" {
		return nil, fmt.Errorf("load_balance_id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNLoadBalance(in.LoadBalance); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		updated, err := uc.provider.UpdateCDNLoadBalance(ctx, in.Credentials, in.ZoneUUID, in.LoadBalanceID, in.LoadBalance)
		if err != nil {
			return nil, fmt.Errorf("updating CDN load balance %s in zone %s: %w", in.LoadBalanceID, in.ZoneUUID, err)
		}
		return json.Marshal(updated)
	})
	if err != nil {
		return nil, err
	}

	var updated domain.CDNLoadBalance
	if err := json.Unmarshal(raw, &updated); err != nil {
		return nil, fmt.Errorf("decoding updated CDN load balance: %w", err)
	}
	return &updated, nil
}

// DeleteCDNLoadBalanceInput identifies the pool to remove.
type DeleteCDNLoadBalanceInput struct {
	Credentials   domain.ProviderCredentials
	ZoneUUID      string
	LoadBalanceID string
}

// DeleteCDNLoadBalance is a fast operation. Deleting a pool the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely (AGENTS.md 4.4).
type DeleteCDNLoadBalance struct {
	queue    ports.Queue
	provider cdnLoadBalanceProvider
}

// NewDeleteCDNLoadBalance builds the use case from its ports.
func NewDeleteCDNLoadBalance(queue ports.Queue, provider cdnLoadBalanceProvider) *DeleteCDNLoadBalance {
	return &DeleteCDNLoadBalance{queue: queue, provider: provider}
}

// Execute deletes the pool, tolerating one that is already gone.
func (uc *DeleteCDNLoadBalance) Execute(ctx context.Context, in DeleteCDNLoadBalanceInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.LoadBalanceID == "" {
		return fmt.Errorf("load_balance_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNLoadBalance(ctx, in.Credentials, in.ZoneUUID, in.LoadBalanceID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting CDN load balance %s in zone %s: %w", in.LoadBalanceID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// ListCDNLoadBalanceServersInput identifies the pool whose backend servers
// to list. LoadBalanceID is required by the provider's index endpoint (a
// query parameter, confirmed against the spec).
type ListCDNLoadBalanceServersInput struct {
	Credentials   domain.ProviderCredentials
	ZoneUUID      string
	LoadBalanceID string
}

// ListCDNLoadBalanceServers is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type ListCDNLoadBalanceServers struct {
	queue    ports.Queue
	provider cdnLoadBalanceProvider
}

// NewListCDNLoadBalanceServers builds the use case from its ports.
func NewListCDNLoadBalanceServers(queue ports.Queue, provider cdnLoadBalanceProvider) *ListCDNLoadBalanceServers {
	return &ListCDNLoadBalanceServers{queue: queue, provider: provider}
}

// Execute returns every backend server of the given pool.
func (uc *ListCDNLoadBalanceServers) Execute(ctx context.Context, in ListCDNLoadBalanceServersInput) ([]domain.CDNLoadBalanceServer, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.LoadBalanceID == "" {
		return nil, fmt.Errorf("load_balance_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		servers, err := uc.provider.ListCDNLoadBalanceServers(ctx, in.Credentials, in.ZoneUUID, in.LoadBalanceID)
		if err != nil {
			return nil, fmt.Errorf("listing CDN load balance servers of pool %s in zone %s: %w", in.LoadBalanceID, in.ZoneUUID, err)
		}
		return json.Marshal(servers)
	})
	if err != nil {
		return nil, err
	}

	var servers []domain.CDNLoadBalanceServer
	if err := json.Unmarshal(raw, &servers); err != nil {
		return nil, fmt.Errorf("decoding CDN load balance server list: %w", err)
	}
	return servers, nil
}

// CreateCDNLoadBalanceServerInput is the normalized form of a
// create_cdn_load_balance_server tool call.
type CreateCDNLoadBalanceServerInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Server      domain.CDNLoadBalanceServer
}

// CreateCDNLoadBalanceServer is a fast operation, for the same reason as
// CreateCDNLoadBalance: the store endpoint's 201 example returns an empty
// data array with no status field to poll. Because that response carries no
// generated ID, the server returned to the caller has no ID populated; use
// list_cdn_load_balance_servers to learn the ID afterward.
type CreateCDNLoadBalanceServer struct {
	queue    ports.Queue
	provider cdnLoadBalanceProvider
}

// NewCreateCDNLoadBalanceServer builds the use case from its ports.
func NewCreateCDNLoadBalanceServer(queue ports.Queue, provider cdnLoadBalanceProvider) *CreateCDNLoadBalanceServer {
	return &CreateCDNLoadBalanceServer{queue: queue, provider: provider}
}

// Execute validates the request and creates the server, returning what the
// provider echoed back synchronously.
func (uc *CreateCDNLoadBalanceServer) Execute(ctx context.Context, in CreateCDNLoadBalanceServerInput) (*domain.CDNLoadBalanceServer, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNLoadBalanceServer(in.Server); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		created, err := uc.provider.CreateCDNLoadBalanceServer(ctx, in.Credentials, in.ZoneUUID, in.Server)
		if err != nil {
			return nil, fmt.Errorf("creating CDN load balance server in zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(created)
	})
	if err != nil {
		return nil, err
	}

	var created domain.CDNLoadBalanceServer
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, fmt.Errorf("decoding created CDN load balance server: %w", err)
	}
	return &created, nil
}

// GetCDNLoadBalanceServerInput identifies the server to look up.
type GetCDNLoadBalanceServerInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	ServerID    string
}

// GetCDNLoadBalanceServer is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNLoadBalanceServer struct {
	queue    ports.Queue
	provider cdnLoadBalanceProvider
}

// NewGetCDNLoadBalanceServer builds the use case from its ports.
func NewGetCDNLoadBalanceServer(queue ports.Queue, provider cdnLoadBalanceProvider) *GetCDNLoadBalanceServer {
	return &GetCDNLoadBalanceServer{queue: queue, provider: provider}
}

// Execute returns the current state of one backend server.
func (uc *GetCDNLoadBalanceServer) Execute(ctx context.Context, in GetCDNLoadBalanceServerInput) (*domain.CDNLoadBalanceServer, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.ServerID == "" {
		return nil, fmt.Errorf("server_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		srv, err := uc.provider.GetCDNLoadBalanceServer(ctx, in.Credentials, in.ZoneUUID, in.ServerID)
		if err != nil {
			return nil, fmt.Errorf("getting CDN load balance server %s in zone %s: %w", in.ServerID, in.ZoneUUID, err)
		}
		return json.Marshal(srv)
	})
	if err != nil {
		return nil, err
	}

	var srv domain.CDNLoadBalanceServer
	if err := json.Unmarshal(raw, &srv); err != nil {
		return nil, fmt.Errorf("decoding CDN load balance server: %w", err)
	}
	return &srv, nil
}

// UpdateCDNLoadBalanceServerInput is the normalized form of an
// update_cdn_load_balance_server tool call.
type UpdateCDNLoadBalanceServerInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	ServerID    string
	Server      domain.CDNLoadBalanceServer
}

// UpdateCDNLoadBalanceServer is a fast operation, same reasoning as
// UpdateCDNLoadBalance: no state to poll, and the response carries nothing
// to decode, so the server returned to the caller is what was sent, with the
// ID from the request.
type UpdateCDNLoadBalanceServer struct {
	queue    ports.Queue
	provider cdnLoadBalanceProvider
}

// NewUpdateCDNLoadBalanceServer builds the use case from its ports.
func NewUpdateCDNLoadBalanceServer(queue ports.Queue, provider cdnLoadBalanceProvider) *UpdateCDNLoadBalanceServer {
	return &UpdateCDNLoadBalanceServer{queue: queue, provider: provider}
}

// Execute validates the request and updates the server, returning what the
// provider echoed back synchronously.
func (uc *UpdateCDNLoadBalanceServer) Execute(ctx context.Context, in UpdateCDNLoadBalanceServerInput) (*domain.CDNLoadBalanceServer, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ZoneUUID == "" {
		return nil, fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.ServerID == "" {
		return nil, fmt.Errorf("server_id is required: %w", domain.ErrInvalidInput)
	}
	if in.Server.Name == "" {
		return nil, fmt.Errorf("server name is required: %w", domain.ErrInvalidInput)
	}
	if err := validateCDNLoadBalanceServer(in.Server); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		updated, err := uc.provider.UpdateCDNLoadBalanceServer(ctx, in.Credentials, in.ZoneUUID, in.ServerID, in.Server)
		if err != nil {
			return nil, fmt.Errorf("updating CDN load balance server %s in zone %s: %w", in.ServerID, in.ZoneUUID, err)
		}
		return json.Marshal(updated)
	})
	if err != nil {
		return nil, err
	}

	var updated domain.CDNLoadBalanceServer
	if err := json.Unmarshal(raw, &updated); err != nil {
		return nil, fmt.Errorf("decoding updated CDN load balance server: %w", err)
	}
	return &updated, nil
}

// DeleteCDNLoadBalanceServerInput identifies the server to remove.
type DeleteCDNLoadBalanceServerInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	ServerID    string
}

// DeleteCDNLoadBalanceServer is a fast operation. Deleting a server the
// provider no longer has is treated as already-done rather than an error, so
// callers can call it more than once safely (AGENTS.md 4.4).
type DeleteCDNLoadBalanceServer struct {
	queue    ports.Queue
	provider cdnLoadBalanceProvider
}

// NewDeleteCDNLoadBalanceServer builds the use case from its ports.
func NewDeleteCDNLoadBalanceServer(queue ports.Queue, provider cdnLoadBalanceProvider) *DeleteCDNLoadBalanceServer {
	return &DeleteCDNLoadBalanceServer{queue: queue, provider: provider}
}

// Execute deletes the server, tolerating one that is already gone.
func (uc *DeleteCDNLoadBalanceServer) Execute(ctx context.Context, in DeleteCDNLoadBalanceServerInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ZoneUUID == "" {
		return fmt.Errorf("zone_uuid is required: %w", domain.ErrInvalidInput)
	}
	if in.ServerID == "" {
		return fmt.Errorf("server_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteCDNLoadBalanceServer(ctx, in.Credentials, in.ZoneUUID, in.ServerID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting CDN load balance server %s in zone %s: %w", in.ServerID, in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
