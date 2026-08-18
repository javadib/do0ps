package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ListServersInput carries the credentials needed to list an account's
// servers. There is nothing else to specify: listing is unscoped.
type ListServersInput struct {
	Credentials domain.ProviderCredentials
}

// ListServers is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type ListServers struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListServers builds the use case from its ports.
func NewListServers(queue ports.Queue, provider ports.ParspackProvider) *ListServers {
	return &ListServers{queue: queue, provider: provider}
}

// Execute returns every server visible to the given credentials.
func (uc *ListServers) Execute(ctx context.Context, in ListServersInput) ([]domain.Server, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		servers, err := uc.provider.ListServers(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing servers: %w", err)
		}
		return json.Marshal(servers)
	})
	if err != nil {
		return nil, err
	}

	var servers []domain.Server
	if err := json.Unmarshal(raw, &servers); err != nil {
		return nil, fmt.Errorf("decoding server list: %w", err)
	}
	return servers, nil
}
