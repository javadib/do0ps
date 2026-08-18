package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ReserveIPInput carries the region a new static address should live in.
type ReserveIPInput struct {
	Credentials domain.ProviderCredentials
	Region      string
}

// ReserveIP is a fast operation: it runs on a worker but the caller waits for
// the result inside the same tool call. The address is reserved but not
// attached to any server; attach it later with AssignIPToServer.
type ReserveIP struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewReserveIP builds the use case from its ports.
func NewReserveIP(queue ports.Queue, provider ports.ParspackProvider) *ReserveIP {
	return &ReserveIP{queue: queue, provider: provider}
}

// Execute reserves a static public IPv4 address in the given region.
func (uc *ReserveIP) Execute(ctx context.Context, in ReserveIPInput) (*domain.ReservedIP, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Region == "" {
		return nil, fmt.Errorf("region is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		ip, err := uc.provider.ReserveIP(ctx, in.Credentials, in.Region)
		if err != nil {
			return nil, fmt.Errorf("reserving IP in region %s: %w", in.Region, err)
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
