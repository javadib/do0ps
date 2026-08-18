package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ListVPCsInput carries the credentials needed to list an account's VPCs.
// There is nothing else to specify: listing is unscoped.
type ListVPCsInput struct {
	Credentials domain.ProviderCredentials
}

// ListVPCs is a fast operation: it runs on a worker but the caller waits for
// the result inside the same tool call.
type ListVPCs struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListVPCs builds the use case from its ports.
func NewListVPCs(queue ports.Queue, provider ports.ParspackProvider) *ListVPCs {
	return &ListVPCs{queue: queue, provider: provider}
}

// Execute returns every VPC visible to the given credentials.
func (uc *ListVPCs) Execute(ctx context.Context, in ListVPCsInput) ([]domain.VPC, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		vpcs, err := uc.provider.ListVPCs(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing VPCs: %w", err)
		}
		return json.Marshal(vpcs)
	})
	if err != nil {
		return nil, err
	}

	var vpcs []domain.VPC
	if err := json.Unmarshal(raw, &vpcs); err != nil {
		return nil, fmt.Errorf("decoding VPC list: %w", err)
	}
	return vpcs, nil
}
