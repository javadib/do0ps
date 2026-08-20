package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ListFirewallsInput carries the credentials needed to list an account's
// firewalls. There is nothing else to specify: listing is unscoped.
type ListFirewallsInput struct {
	Credentials domain.ProviderCredentials
}

// ListFirewalls is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type ListFirewalls struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListFirewalls builds the use case from its ports.
func NewListFirewalls(queue ports.Queue, provider ports.ParspackProvider) *ListFirewalls {
	return &ListFirewalls{queue: queue, provider: provider}
}

// Execute returns every firewall visible to the given credentials.
func (uc *ListFirewalls) Execute(ctx context.Context, in ListFirewallsInput) ([]domain.Firewall, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		firewalls, err := uc.provider.ListFirewalls(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing firewalls: %w", err)
		}
		return json.Marshal(firewalls)
	})
	if err != nil {
		return nil, err
	}

	var firewalls []domain.Firewall
	if err := json.Unmarshal(raw, &firewalls); err != nil {
		return nil, fmt.Errorf("decoding firewall list: %w", err)
	}
	return firewalls, nil
}
