package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// AssignIPToServerInput names the address and the server it should attach to.
type AssignIPToServerInput struct {
	Credentials domain.ProviderCredentials
	IPAddress   string
	ServerID    string
}

// AssignIPToServer is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call. The address itself is
// untouched — only its attachment changes, so it can be reassigned later.
type AssignIPToServer struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewAssignIPToServer builds the use case from its ports.
func NewAssignIPToServer(queue ports.Queue, provider ports.ParspackProvider) *AssignIPToServer {
	return &AssignIPToServer{queue: queue, provider: provider}
}

// Execute attaches the reserved IP to the server and returns the address's
// updated state.
func (uc *AssignIPToServer) Execute(ctx context.Context, in AssignIPToServerInput) (*domain.ReservedIP, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.IPAddress == "" {
		return nil, fmt.Errorf("ip_address is required: %w", domain.ErrInvalidInput)
	}
	if in.ServerID == "" {
		return nil, fmt.Errorf("server_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		ip, err := uc.provider.AssignIPToServer(ctx, in.Credentials, in.IPAddress, in.ServerID)
		if err != nil {
			return nil, fmt.Errorf("assigning reserved IP %s to server %s: %w", in.IPAddress, in.ServerID, err)
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
