package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// DeleteSSHKeyInput identifies the key to remove.
type DeleteSSHKeyInput struct {
	Credentials domain.ProviderCredentials
	KeyID       string
}

// DeleteSSHKey is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call. Deleting an ID the provider no
// longer has is treated as already-done rather than an error, so callers can
// call it more than once safely (ports.ParspackProvider.DeleteSSHKey's
// contract).
type DeleteSSHKey struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewDeleteSSHKey builds the use case from its ports.
func NewDeleteSSHKey(queue ports.Queue, provider ports.ParspackProvider) *DeleteSSHKey {
	return &DeleteSSHKey{queue: queue, provider: provider}
}

// Execute deletes the key, tolerating one that is already gone.
func (uc *DeleteSSHKey) Execute(ctx context.Context, in DeleteSSHKeyInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.KeyID == "" {
		return fmt.Errorf("key_id is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteSSHKey(ctx, in.Credentials, in.KeyID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting SSH key %s: %w", in.KeyID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}
