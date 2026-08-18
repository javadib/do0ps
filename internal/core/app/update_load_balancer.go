package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// UpdateLoadBalancerInput identifies the load balancer to replace and the new
// configuration to apply to it.
type UpdateLoadBalancerInput struct {
	Credentials    domain.ProviderCredentials
	LoadBalancerID string
	LoadBalancer   domain.LoadBalancer
}

// UpdateLoadBalancer is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type UpdateLoadBalancer struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateLoadBalancer builds the use case from its ports.
func NewUpdateLoadBalancer(queue ports.Queue, provider ports.ParspackProvider) *UpdateLoadBalancer {
	return &UpdateLoadBalancer{queue: queue, provider: provider}
}

// Execute replaces the load balancer's configuration and returns its new
// state.
func (uc *UpdateLoadBalancer) Execute(ctx context.Context, in UpdateLoadBalancerInput) (*domain.LoadBalancer, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.LoadBalancerID == "" {
		return nil, fmt.Errorf("load_balancer_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		lb, err := uc.provider.UpdateLoadBalancer(ctx, in.Credentials, in.LoadBalancerID, in.LoadBalancer)
		if err != nil {
			return nil, fmt.Errorf("updating load balancer %s: %w", in.LoadBalancerID, err)
		}
		return json.Marshal(lb)
	})
	if err != nil {
		return nil, err
	}

	var lb domain.LoadBalancer
	if err := json.Unmarshal(raw, &lb); err != nil {
		return nil, fmt.Errorf("decoding load balancer: %w", err)
	}
	return &lb, nil
}
