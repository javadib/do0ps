package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// GetFirewallInput identifies the firewall to inspect.
type GetFirewallInput struct {
	Credentials domain.ProviderCredentials
	FirewallID  string
}

// GetFirewall is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type GetFirewall struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetFirewall builds the use case from its ports.
func NewGetFirewall(queue ports.Queue, provider ports.ParspackProvider) *GetFirewall {
	return &GetFirewall{queue: queue, provider: provider}
}

// Execute returns the current state of one firewall.
func (uc *GetFirewall) Execute(ctx context.Context, in GetFirewallInput) (*domain.Firewall, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.FirewallID == "" {
		return nil, fmt.Errorf("firewall_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		fw, err := uc.provider.GetFirewall(ctx, in.Credentials, in.FirewallID)
		if err != nil {
			return nil, fmt.Errorf("getting firewall %s: %w", in.FirewallID, err)
		}
		return json.Marshal(fw)
	})
	if err != nil {
		return nil, err
	}

	var fw domain.Firewall
	if err := json.Unmarshal(raw, &fw); err != nil {
		return nil, fmt.Errorf("decoding firewall: %w", err)
	}
	return &fw, nil
}
