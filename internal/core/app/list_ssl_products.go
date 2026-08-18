package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ListSSLProductsInput carries the credentials needed to list an account's
// available SSL products.
type ListSSLProductsInput struct {
	Credentials domain.ProviderCredentials
}

// ListSSLProducts is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type ListSSLProducts struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListSSLProducts builds the use case from its ports.
func NewListSSLProducts(queue ports.Queue, provider ports.ParspackProvider) *ListSSLProducts {
	return &ListSSLProducts{queue: queue, provider: provider}
}

// Execute returns every SSL product available to order, e.g. to populate
// product_slug before calling CreateSSLOrder.
func (uc *ListSSLProducts) Execute(ctx context.Context, in ListSSLProductsInput) ([]domain.SSLProduct, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		products, err := uc.provider.ListSSLProducts(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing SSL products: %w", err)
		}
		return json.Marshal(products)
	})
	if err != nil {
		return nil, err
	}

	var products []domain.SSLProduct
	if err := json.Unmarshal(raw, &products); err != nil {
		return nil, fmt.Errorf("decoding SSL product list: %w", err)
	}
	return products, nil
}
