package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// RegisterSSHKeyInput carries the credentials and the key to register.
type RegisterSSHKeyInput struct {
	Credentials domain.ProviderCredentials
	Key         domain.SSHKey
}

// RegisterSSHKey is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type RegisterSSHKey struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewRegisterSSHKey builds the use case from its ports.
func NewRegisterSSHKey(queue ports.Queue, provider ports.ParspackProvider) *RegisterSSHKey {
	return &RegisterSSHKey{queue: queue, provider: provider}
}

// Execute registers the key and returns the provider-assigned copy, which
// carries the ID and fingerprint.
func (uc *RegisterSSHKey) Execute(ctx context.Context, in RegisterSSHKeyInput) (*domain.SSHKey, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Key.Name == "" {
		return nil, fmt.Errorf("SSH key name is required: %w", domain.ErrInvalidInput)
	}
	if in.Key.PublicKey == "" {
		return nil, fmt.Errorf("SSH key public_key is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		key, err := uc.provider.CreateSSHKey(ctx, in.Credentials, in.Key)
		if err != nil {
			return nil, fmt.Errorf("registering SSH key %q: %w", in.Key.Name, err)
		}
		return json.Marshal(key)
	})
	if err != nil {
		return nil, err
	}

	var key domain.SSHKey
	if err := json.Unmarshal(raw, &key); err != nil {
		return nil, fmt.Errorf("decoding SSH key: %w", err)
	}
	return &key, nil
}
