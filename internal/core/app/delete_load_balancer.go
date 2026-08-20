package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// DeleteLoadBalancerInput identifies the load balancer to remove.
type DeleteLoadBalancerInput struct {
	Credentials    domain.ProviderCredentials
	LoadBalancerID string
}

// DeleteLoadBalancer is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call. Deleting an ID the provider
// no longer has is treated as already-done rather than an error, so callers
// can call it more than once safely.
type DeleteLoadBalancer struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteLoadBalancer builds the use case from its ports.
func NewDeleteLoadBalancer(queue ports.Queue, provider ports.ParspackProvider) *DeleteLoadBalancer {
	return &DeleteLoadBalancer{queue: queue, provider: provider}
}

// Execute deletes the load balancer, tolerating one that is already gone.
func (uc *DeleteLoadBalancer) Execute(ctx context.Context, in DeleteLoadBalancerInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.LoadBalancerID == "" {
		return fmt.Errorf("load_balancer_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteLoadBalancer(ctx, in.Credentials, in.LoadBalancerID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting load balancer %s: %w", in.LoadBalancerID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
