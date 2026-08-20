package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ListSSHKeysInput carries the credentials needed to list an account's keys.
// There is nothing else to specify: listing is unscoped.
type ListSSHKeysInput struct {
	Credentials domain.ProviderCredentials
}

// ListSSHKeys is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type ListSSHKeys struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListSSHKeys builds the use case from its ports.
func NewListSSHKeys(queue ports.Queue, provider ports.ParspackProvider) *ListSSHKeys {
	return &ListSSHKeys{queue: queue, provider: provider}
}

// Execute returns every SSH key registered with the given credentials.
func (uc *ListSSHKeys) Execute(ctx context.Context, in ListSSHKeysInput) ([]domain.SSHKey, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		keys, err := uc.provider.ListSSHKeys(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing SSH keys: %w", err)
		}
		return json.Marshal(keys)
	})
	if err != nil {
		return nil, err
	}

	var keys []domain.SSHKey
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, fmt.Errorf("decoding SSH key list: %w", err)
	}
	return keys, nil
}
