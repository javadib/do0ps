package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ReleaseIPInput identifies the address to release back to the provider.
type ReleaseIPInput struct {
	Credentials domain.ProviderCredentials
	IPAddress   string
}

// ReleaseIP is a fast operation: it runs on a worker but the caller waits for
// the result inside the same tool call. Releasing an address the provider no
// longer has is treated as already done rather than an error, so callers can
// call it more than once safely.
type ReleaseIP struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewReleaseIP builds the use case from its ports.
func NewReleaseIP(queue ports.Queue, provider ports.ParspackProvider) *ReleaseIP {
	return &ReleaseIP{queue: queue, provider: provider}
}

// Execute releases the reserved IP, tolerating one that is already gone.
func (uc *ReleaseIP) Execute(ctx context.Context, in ReleaseIPInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.IPAddress == "" {
		return fmt.Errorf("ip_address is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.ReleaseIP(ctx, in.Credentials, in.IPAddress); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("releasing reserved IP %s: %w", in.IPAddress, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
