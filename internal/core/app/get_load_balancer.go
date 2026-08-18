package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// GetLoadBalancerInput identifies the load balancer to inspect.
type GetLoadBalancerInput struct {
	Credentials    domain.ProviderCredentials
	LoadBalancerID string
}

// GetLoadBalancer is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetLoadBalancer struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetLoadBalancer builds the use case from its ports.
func NewGetLoadBalancer(queue ports.Queue, provider ports.ParspackProvider) *GetLoadBalancer {
	return &GetLoadBalancer{queue: queue, provider: provider}
}

// Execute returns the current state of one load balancer.
func (uc *GetLoadBalancer) Execute(ctx context.Context, in GetLoadBalancerInput) (*domain.LoadBalancer, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.LoadBalancerID == "" {
		return nil, fmt.Errorf("load_balancer_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		lb, err := uc.provider.GetLoadBalancer(ctx, in.Credentials, in.LoadBalancerID)
		if err != nil {
			return nil, fmt.Errorf("getting load balancer %s: %w", in.LoadBalancerID, err)
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
