package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// DeleteServerInput identifies the server to remove.
type DeleteServerInput struct {
	Credentials domain.ProviderCredentials
	ServerID    string
}

// DeleteServer is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call. Deleting an ID the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely (ports.ParspackProvider.DeleteServer's
// contract).
type DeleteServer struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteServer builds the use case from its ports.
func NewDeleteServer(queue ports.Queue, provider ports.ParspackProvider) *DeleteServer {
	return &DeleteServer{queue: queue, provider: provider}
}

// Execute deletes the server, tolerating one that is already gone.
func (uc *DeleteServer) Execute(ctx context.Context, in DeleteServerInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.ServerID == "" {
		return fmt.Errorf("server_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteServer(ctx, in.Credentials, in.ServerID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting server %s: %w", in.ServerID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
