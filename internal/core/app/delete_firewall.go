package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// DeleteFirewallInput identifies the firewall to remove.
type DeleteFirewallInput struct {
	Credentials domain.ProviderCredentials
	FirewallID  string
}

// DeleteFirewall is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call. Deleting an ID the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely (ports.ParspackProvider.DeleteFirewall's
// contract).
type DeleteFirewall struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteFirewall builds the use case from its ports.
func NewDeleteFirewall(queue ports.Queue, provider ports.ParspackProvider) *DeleteFirewall {
	return &DeleteFirewall{queue: queue, provider: provider}
}

// Execute deletes the firewall, tolerating one that is already gone.
func (uc *DeleteFirewall) Execute(ctx context.Context, in DeleteFirewallInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.FirewallID == "" {
		return fmt.Errorf("firewall_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteFirewall(ctx, in.Credentials, in.FirewallID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting firewall %s: %w", in.FirewallID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
