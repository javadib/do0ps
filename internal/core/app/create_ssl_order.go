package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// CreateSSLOrderInput is the normalized form of a create_ssl_order tool call.
type CreateSSLOrderInput struct {
	Credentials domain.ProviderCredentials
	Spec        domain.SSLOrderSpec
}

// CreateSSLOrder is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call. It only places the order
// and returns its invoice; the order is not processed (CSR + contact
// details submitted) until ProcessSSLOrder runs, once the invoice is paid.
type CreateSSLOrder struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewCreateSSLOrder builds the use case from its ports.
func NewCreateSSLOrder(queue ports.Queue, provider ports.ParspackProvider) *CreateSSLOrder {
	return &CreateSSLOrder{queue: queue, provider: provider}
}

// Execute validates the request and creates the order.
func (uc *CreateSSLOrder) Execute(ctx context.Context, in CreateSSLOrderInput) (*domain.SSLOrder, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Spec.ProductSlug == "" {
		return nil, fmt.Errorf("product_slug is required: %w", domain.ErrInvalidInput)
	}
	if in.Spec.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	switch in.Spec.BillingCycle {
	case "annually", "biennially", "triennially":
	default:
		return nil, fmt.Errorf("billing_cycle must be one of annually, biennially, triennially, got %q: %w", in.Spec.BillingCycle, domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		order, err := uc.provider.CreateSSLOrder(ctx, in.Credentials, in.Spec)
		if err != nil {
			return nil, fmt.Errorf("creating SSL order for %q: %w", in.Spec.Domain, err)
		}
		return json.Marshal(order)
	})
	if err != nil {
		return nil, err
	}

	var order domain.SSLOrder
	if err := json.Unmarshal(raw, &order); err != nil {
		return nil, fmt.Errorf("decoding created SSL order: %w", err)
	}
	return &order, nil
}
