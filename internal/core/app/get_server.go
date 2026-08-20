package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// GetServerInput identifies the server to inspect.
type GetServerInput struct {
	Credentials domain.ProviderCredentials
	ServerID    string
}

// GetServer is a fast operation: it runs on a worker but the caller waits for
// the result inside the same tool call.
type GetServer struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetServer builds the use case from its ports.
func NewGetServer(queue ports.Queue, provider ports.ParspackProvider) *GetServer {
	return &GetServer{queue: queue, provider: provider}
}

// Execute returns the current state of one server.
func (uc *GetServer) Execute(ctx context.Context, in GetServerInput) (*domain.Server, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.ServerID == "" {
		return nil, fmt.Errorf("server_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		srv, err := uc.provider.GetServer(ctx, in.Credentials, in.ServerID)
		if err != nil {
			return nil, fmt.Errorf("getting server %s: %w", in.ServerID, err)
		}
		return json.Marshal(srv)
	})
	if err != nil {
		return nil, err
	}

	var srv domain.Server
	if err := json.Unmarshal(raw, &srv); err != nil {
		return nil, fmt.Errorf("decoding server: %w", err)
	}
	return &srv, nil
}
