package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// UpdateFirewallInput identifies the firewall to replace and the new
// configuration to apply to it.
type UpdateFirewallInput struct {
	Credentials domain.ProviderCredentials
	FirewallID  string
	Firewall    domain.Firewall
}

// UpdateFirewall is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type UpdateFirewall struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateFirewall builds the use case from its ports.
func NewUpdateFirewall(queue ports.Queue, provider ports.ParspackProvider) *UpdateFirewall {
	return &UpdateFirewall{queue: queue, provider: provider}
}

// Execute replaces the firewall's configuration and returns its new state.
func (uc *UpdateFirewall) Execute(ctx context.Context, in UpdateFirewallInput) (*domain.Firewall, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.FirewallID == "" {
		return nil, fmt.Errorf("firewall_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		fw, err := uc.provider.UpdateFirewall(ctx, in.Credentials, in.FirewallID, in.Firewall)
		if err != nil {
			return nil, fmt.Errorf("updating firewall %s: %w", in.FirewallID, err)
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
