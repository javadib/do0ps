package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// DeleteVPCInput identifies the VPC to remove.
type DeleteVPCInput struct {
	Credentials domain.ProviderCredentials
	VPCID       string
}

// DeleteVPC is a fast operation: it runs on a worker but the caller waits for
// the result inside the same tool call. Deleting an ID the provider no longer
// has is treated as already-done rather than an error, so callers can call it
// more than once safely (ports.ParspackProvider.DeleteVPC's contract).
type DeleteVPC struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteVPC builds the use case from its ports.
func NewDeleteVPC(queue ports.Queue, provider ports.ParspackProvider) *DeleteVPC {
	return &DeleteVPC{queue: queue, provider: provider}
}

// Execute deletes the VPC, tolerating one that is already gone.
func (uc *DeleteVPC) Execute(ctx context.Context, in DeleteVPCInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.VPCID == "" {
		return fmt.Errorf("vpc_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteVPC(ctx, in.Credentials, in.VPCID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting VPC %s: %w", in.VPCID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
