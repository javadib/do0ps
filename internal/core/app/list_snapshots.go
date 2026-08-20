package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ListSnapshotsInput carries the credentials needed to list an account's VM
// snapshots. There is nothing else to specify: listing is unscoped.
type ListSnapshotsInput struct {
	Credentials domain.ProviderCredentials
}

// ListSnapshots is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type ListSnapshots struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListSnapshots builds the use case from its ports.
func NewListSnapshots(queue ports.Queue, provider ports.ParspackProvider) *ListSnapshots {
	return &ListSnapshots{queue: queue, provider: provider}
}

// Execute returns every VM snapshot visible to the given credentials.
func (uc *ListSnapshots) Execute(ctx context.Context, in ListSnapshotsInput) ([]domain.VMSnapshot, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		snapshots, err := uc.provider.ListVMSnapshots(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing VM snapshots: %w", err)
		}
		return json.Marshal(snapshots)
	})
	if err != nil {
		return nil, err
	}

	var snapshots []domain.VMSnapshot
	if err := json.Unmarshal(raw, &snapshots); err != nil {
		return nil, fmt.Errorf("decoding snapshot list: %w", err)
	}
	return snapshots, nil
}
