package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ListLoadBalancersInput carries the credentials needed to list an account's
// load balancers. There is nothing else to specify: listing is unscoped.
type ListLoadBalancersInput struct {
	Credentials domain.ProviderCredentials
}

// ListLoadBalancers is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type ListLoadBalancers struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListLoadBalancers builds the use case from its ports.
func NewListLoadBalancers(queue ports.Queue, provider ports.ParspackProvider) *ListLoadBalancers {
	return &ListLoadBalancers{queue: queue, provider: provider}
}

// Execute returns every load balancer visible to the given credentials.
func (uc *ListLoadBalancers) Execute(ctx context.Context, in ListLoadBalancersInput) ([]domain.LoadBalancer, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		balancers, err := uc.provider.ListLoadBalancers(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing load balancers: %w", err)
		}
		return json.Marshal(balancers)
	})
	if err != nil {
		return nil, err
	}

	var balancers []domain.LoadBalancer
	if err := json.Unmarshal(raw, &balancers); err != nil {
		return nil, fmt.Errorf("decoding load balancer list: %w", err)
	}
	return balancers, nil
}
