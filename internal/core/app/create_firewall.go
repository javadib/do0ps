package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// CreateFirewallInput is the normalized form of a create_firewall tool call.
type CreateFirewallInput struct {
	Credentials domain.ProviderCredentials
	Firewall    domain.Firewall
}

// CreateFirewall is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type CreateFirewall struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewCreateFirewall builds the use case from its ports.
func NewCreateFirewall(queue ports.Queue, provider ports.ParspackProvider) *CreateFirewall {
	return &CreateFirewall{queue: queue, provider: provider}
}

// Execute creates the firewall and returns its provider-assigned state.
func (uc *CreateFirewall) Execute(ctx context.Context, in CreateFirewallInput) (*domain.Firewall, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Firewall.Name == "" {
		return nil, fmt.Errorf("firewall name is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		fw, err := uc.provider.CreateFirewall(ctx, in.Credentials, in.Firewall)
		if err != nil {
			return nil, fmt.Errorf("creating firewall %q: %w", in.Firewall.Name, err)
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
