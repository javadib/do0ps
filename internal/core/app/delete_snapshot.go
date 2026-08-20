package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// DeleteSnapshotInput identifies the snapshot to remove.
type DeleteSnapshotInput struct {
	Credentials domain.ProviderCredentials
	SnapshotID  string
}

// DeleteSnapshot is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call. Deleting an ID the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely (ports.ParspackProvider.DeleteVMSnapshot's
// contract).
type DeleteSnapshot struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteSnapshot builds the use case from its ports.
func NewDeleteSnapshot(queue ports.Queue, provider ports.ParspackProvider) *DeleteSnapshot {
	return &DeleteSnapshot{queue: queue, provider: provider}
}

// Execute deletes the snapshot, tolerating one that is already gone.
func (uc *DeleteSnapshot) Execute(ctx context.Context, in DeleteSnapshotInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.SnapshotID == "" {
		return fmt.Errorf("snapshot_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteVMSnapshot(ctx, in.Credentials, in.SnapshotID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting VM snapshot %s: %w", in.SnapshotID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
