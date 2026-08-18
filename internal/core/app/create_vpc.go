package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// CreateVPCInput carries the credentials and the VPC to create. Only Name,
// Region, Description and IPRange are meaningful on input.
type CreateVPCInput struct {
	Credentials domain.ProviderCredentials
	VPC         domain.VPC
}

// CreateVPC is a fast operation: it runs on a worker but the caller waits for
// the result inside the same tool call.
type CreateVPC struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewCreateVPC builds the use case from its ports.
func NewCreateVPC(queue ports.Queue, provider ports.ParspackProvider) *CreateVPC {
	return &CreateVPC{queue: queue, provider: provider}
}

// Execute creates the VPC and returns the provider-assigned copy, which
// carries the ID and default flag.
func (uc *CreateVPC) Execute(ctx context.Context, in CreateVPCInput) (*domain.VPC, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.VPC.Name == "" {
		return nil, fmt.Errorf("VPC name is required: %w", domain.ErrInvalidInput)
	}
	if in.VPC.Region == "" {
		return nil, fmt.Errorf("VPC region is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		vpc, err := uc.provider.CreateVPC(ctx, in.Credentials, in.VPC)
		if err != nil {
			return nil, fmt.Errorf("creating VPC %q: %w", in.VPC.Name, err)
		}
		return json.Marshal(vpc)
	})
	if err != nil {
		return nil, err
	}

	var vpc domain.VPC
	if err := json.Unmarshal(raw, &vpc); err != nil {
		return nil, fmt.Errorf("decoding VPC: %w", err)
	}
	return &vpc, nil
}
