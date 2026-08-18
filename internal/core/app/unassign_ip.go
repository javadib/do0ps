package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// UnassignIPInput identifies the address to detach from its server.
type UnassignIPInput struct {
	Credentials domain.ProviderCredentials
	IPAddress   string
}

// UnassignIP is a fast operation: it runs on a worker but the caller waits for
// the result inside the same tool call. The address stays reserved and billed;
// only the attachment to a server is removed.
type UnassignIP struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUnassignIP builds the use case from its ports.
func NewUnassignIP(queue ports.Queue, provider ports.ParspackProvider) *UnassignIP {
	return &UnassignIP{queue: queue, provider: provider}
}

// Execute detaches the reserved IP from its server and returns the address's
// updated state.
func (uc *UnassignIP) Execute(ctx context.Context, in UnassignIPInput) (*domain.ReservedIP, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.IPAddress == "" {
		return nil, fmt.Errorf("ip_address is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		ip, err := uc.provider.UnassignIP(ctx, in.Credentials, in.IPAddress)
		if err != nil {
			return nil, fmt.Errorf("unassigning reserved IP %s: %w", in.IPAddress, err)
		}
		return json.Marshal(ip)
	})
	if err != nil {
		return nil, err
	}

	var ip domain.ReservedIP
	if err := json.Unmarshal(raw, &ip); err != nil {
		return nil, fmt.Errorf("decoding reserved IP: %w", err)
	}
	return &ip, nil
}
